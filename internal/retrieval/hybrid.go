package retrieval

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/surtr85/agent-memory/internal/embedding"
	"github.com/surtr85/agent-memory/internal/normalizer"
)

// SearchResult represents a unified retrieved chunk or atomic fact.
type SearchResult struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"` // "chunk" or "fact"
	Namespace   string     `json:"namespace"`
	Title       string     `json:"title,omitempty"`
	Content     string     `json:"content"`
	SourceURI   string     `json:"source_uri,omitempty"`
	LineNumber  int        `json:"line_number,omitempty"`
	Score       float64    `json:"score"`
	ValidFrom   time.Time  `json:"valid_from,omitempty"`
	ValidUntil  *time.Time `json:"valid_until,omitempty"`
}

// Searcher coordinates dense vector, BM25, graph, and temporal retrieval.
type Searcher struct {
	DB        *sql.DB
	Embedding embedding.Client
}

// NewSearcher creates a new Searcher.
func NewSearcher(db *sql.DB, embClient embedding.Client) *Searcher {
	return &Searcher{
		DB:        db,
		Embedding: embClient,
	}
}

// Search performs 4-way hybrid retrieval:
// 1. Dense Vector search over chunks
// 2. Lexical BM25 search over chunks & facts
// 3. Entity Graph BFS traversal
// 4. Temporal filter (active facts vs historical)
// Combined with Reciprocal Rank Fusion (RRF k=60).
func (s *Searcher) Search(ctx context.Context, query, namespace string, topK int, includeHistorical bool) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	// 1. Generate query embedding
	qEmb, err := s.Embedding.GetEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed generating query embedding: %w", err)
	}

	// 2. Fetch chunks from DB (filtered by namespace if specified)
	chunkQuery := `SELECT c.id, c.namespace, c.title, c.content, c.source_uri, c.line_number, v.embedding 
	               FROM chunks c 
	               JOIN vector_embeddings v ON c.id = v.chunk_id`
	var chunkArgs []interface{}
	if namespace != "" {
		chunkQuery += ` WHERE c.namespace = ?`
		chunkArgs = append(chunkArgs, namespace)
	}

	cRows, err := s.DB.QueryContext(ctx, chunkQuery, chunkArgs...)
	if err != nil {
		return nil, fmt.Errorf("query chunks failed: %w", err)
	}
	defer cRows.Close()

	type chunkItem struct {
		id         string
		namespace  string
		title      string
		content    string
		sourceURI  string
		lineNumber int
		sim        float32
	}
	var chunks []chunkItem
	var bm25Docs []Document

	for cRows.Next() {
		var id, ns, title, content string
		var sourceURI sql.NullString
		var lineNo sql.NullInt64
		var embBlob []byte
		if err := cRows.Scan(&id, &ns, &title, &content, &sourceURI, &lineNo, &embBlob); err != nil {
			return nil, err
		}

		cVec, err := embedding.DeserializeEmbedding(embBlob)
		var sim float32
		if err == nil {
			sim = embedding.CosineSimilarity(qEmb, cVec)
		}

		chunks = append(chunks, chunkItem{
			id:         id,
			namespace:  ns,
			title:      title,
			content:    content,
			sourceURI:  sourceURI.String,
			lineNumber: int(lineNo.Int64),
			sim:        sim,
		})

		bm25Docs = append(bm25Docs, Document{
			ID:      id,
			Content: title + " " + content,
		})
	}

	// 3. Fetch facts from DB
	factQuery := `SELECT id, namespace, subject, predicate, object, source_uri, line_number, valid_from, valid_until, invalidated_at FROM facts`
	var factConditions []string
	var factArgs []interface{}

	if namespace != "" {
		factConditions = append(factConditions, "namespace = ?")
		factArgs = append(factArgs, namespace)
	}
	if !includeHistorical {
		factConditions = append(factConditions, "valid_until IS NULL AND invalidated_at IS NULL")
	}

	if len(factConditions) > 0 {
		factQuery += " WHERE "
		for i, cond := range factConditions {
			if i > 0 {
				factQuery += " AND "
			}
			factQuery += cond
		}
	}

	fRows, err := s.DB.QueryContext(ctx, factQuery, factArgs...)
	if err != nil {
		return nil, fmt.Errorf("query facts failed: %w", err)
	}
	defer fRows.Close()

	type factItem struct {
		id         string
		namespace  string
		subject    string
		predicate  string
		object     string
		content    string
		sourceURI  string
		lineNumber int
		validFrom  time.Time
		validUntil *time.Time
	}
	var facts []factItem

	for fRows.Next() {
		var id, ns, subj, pred, obj string
		var sourceURI sql.NullString
		var lineNo sql.NullInt64
		var validFrom time.Time
		var validUntil, invalidatedAt sql.NullTime
		if err := fRows.Scan(&id, &ns, &subj, &pred, &obj, &sourceURI, &lineNo, &validFrom, &validUntil, &invalidatedAt); err != nil {
			return nil, err
		}

		factStr := fmt.Sprintf("%s %s %s", subj, pred, obj)
		var vUntilPtr *time.Time
		if validUntil.Valid {
			vUntilPtr = &validUntil.Time
		}

		facts = append(facts, factItem{
			id:         id,
			namespace:  ns,
			subject:    subj,
			predicate:  pred,
			object:     obj,
			content:    factStr,
			sourceURI:  sourceURI.String,
			lineNumber: int(lineNo.Int64),
			validFrom:  validFrom,
			validUntil: vUntilPtr,
		})

		bm25Docs = append(bm25Docs, Document{
			ID:      id,
			Content: factStr,
		})
	}

	// Rank list 1: Vector Dense Ranking (chunks sorted by CosineSimilarity)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].sim > chunks[j].sim
	})
	var vectorRanked []string
	for _, c := range chunks {
		if c.sim > 0.05 { // mild similarity cutoff
			vectorRanked = append(vectorRanked, c.id)
		}
	}

	// Rank list 2: BM25 Lexical Ranking (over both chunks and facts)
	bm25Idx := NewBM25Index()
	bm25Idx.Index(bm25Docs)
	bm25Scores := bm25Idx.Search(query)
	sort.Slice(bm25Scores, func(i, j int) bool {
		return bm25Scores[i].Score > bm25Scores[j].Score
	})
	var bm25Ranked []string
	for _, s := range bm25Scores {
		bm25Ranked = append(bm25Ranked, s.ID)
	}

	// Rank list 3: Entity Graph traversal
	// Extract query tokens as potential start entities
	queryTokens := normalizer.Tokenize(query)
	var graphRanked []string
	if len(queryTokens) > 0 {
		var matchedEntities []string
		for _, token := range queryTokens {
			var entName string
			row := s.DB.QueryRowContext(ctx, `SELECT name FROM entities WHERE LOWER(name) = LOWER(?)`, token)
			if err := row.Scan(&entName); err == nil {
				matchedEntities = append(matchedEntities, entName)
			}
		}

		if len(matchedEntities) > 0 {
			expandedEntities, err := TraverseGraph(s.DB, matchedEntities, 2)
			if err == nil && len(expandedEntities) > 0 {
				// Find facts and chunks connected to these entities
				for _, ent := range expandedEntities {
					for _, f := range facts {
						if f.subject == ent || f.object == ent {
							graphRanked = append(graphRanked, f.id)
						}
					}
				}
			}
		}
	}

	// Reciprocal Rank Fusion of the ranked lists
	rankedLists := [][]string{vectorRanked, bm25Ranked}
	if len(graphRanked) > 0 {
		rankedLists = append(rankedLists, graphRanked)
	}

	fusedItems := ReciprocalRankFusion(rankedLists, 60)

	// Build map for fast lookup
	chunkMap := make(map[string]chunkItem)
	for _, c := range chunks {
		chunkMap[c.id] = c
	}
	factMap := make(map[string]factItem)
	for _, f := range facts {
		factMap[f.id] = f
	}

	var results []SearchResult
	for _, item := range fusedItems {
		if c, ok := chunkMap[item.ID]; ok {
			results = append(results, SearchResult{
				ID:         c.id,
				Type:       "chunk",
				Namespace:  c.namespace,
				Title:      c.title,
				Content:    c.content,
				SourceURI:  c.sourceURI,
				LineNumber: c.lineNumber,
				Score:      item.Score,
			})
		} else if f, ok := factMap[item.ID]; ok {
			results = append(results, SearchResult{
				ID:         f.id,
				Type:       "fact",
				Namespace:  f.namespace,
				Content:    f.content,
				SourceURI:  f.sourceURI,
				LineNumber: f.lineNumber,
				Score:      item.Score,
				ValidFrom:  f.validFrom,
				ValidUntil: f.validUntil,
			})
		}

		if len(results) >= topK {
			break
		}
	}

	return results, nil
}
