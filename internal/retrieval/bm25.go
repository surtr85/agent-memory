package retrieval

import (
	"math"

	"github.com/surtr85/agent-memory/internal/normalizer"
)

// Document represents a searchable text unit in BM25.
type Document struct {
	ID      string
	Content string
}

// ScoredDoc is a result with its BM25 score.
type ScoredDoc struct {
	ID    string
	Score float64
}

// BM25Index is an in-memory Okapi BM25 search index.
type BM25Index struct {
	k1 float64
	b  float64

	docCount   int
	totalDocLen int
	avgDocLen  float64

	docLens map[string]int
	// docFrequencies: term -> number of documents containing term
	df map[string]int
	// termFrequencies: docID -> (term -> count)
	tf map[string]map[string]int
}

// NewBM25Index initializes a new BM25 index with standard parameters (k1=1.2, b=0.75).
func NewBM25Index() *BM25Index {
	return &BM25Index{
		k1:      1.2,
		b:       0.75,
		docLens: make(map[string]int),
		df:      make(map[string]int),
		tf:      make(map[string]map[string]int),
	}
}

// Index adds or re-indexes documents.
func (idx *BM25Index) Index(docs []Document) {
	for _, doc := range docs {
		tokens := normalizer.Tokenize(doc.Content)
		docLen := len(tokens)
		idx.docLens[doc.ID] = docLen
		idx.totalDocLen += docLen
		idx.docCount++

		termCounts := make(map[string]int)
		for _, t := range tokens {
			termCounts[t]++
		}
		idx.tf[doc.ID] = termCounts

		for term := range termCounts {
			idx.df[term]++
		}
	}

	if idx.docCount > 0 {
		idx.avgDocLen = float64(idx.totalDocLen) / float64(idx.docCount)
	}
}

// Search calculates BM25 scores for the given query across all indexed documents.
func (idx *BM25Index) Search(query string) []ScoredDoc {
	queryTokens := normalizer.Tokenize(query)
	if len(queryTokens) == 0 || idx.docCount == 0 {
		return nil
	}

	scores := make(map[string]float64)

	// Calculate IDF and score for each query token
	for _, qTerm := range queryTokens {
		dfVal, exists := idx.df[qTerm]
		if !exists || dfVal == 0 {
			continue
		}

		// Standard BM25 IDF: ln( (N - n + 0.5) / (n + 0.5) + 1.0 )
		N := float64(idx.docCount)
		n := float64(dfVal)
		idf := math.Log((N-n+0.5)/(n+0.5) + 1.0)
		if idf < 0 {
			idf = 0
		}

		for docID, termCounts := range idx.tf {
			freq := float64(termCounts[qTerm])
			if freq == 0 {
				continue
			}

			docLen := float64(idx.docLens[docID])
			numerator := freq * (idx.k1 + 1.0)
			denominator := freq + idx.k1*(1.0-idx.b+idx.b*(docLen/idx.avgDocLen))

			scores[docID] += idf * (numerator / denominator)
		}
	}

	results := make([]ScoredDoc, 0, len(scores))
	for id, score := range scores {
		if score > 0 {
			results = append(results, ScoredDoc{ID: id, Score: score})
		}
	}

	return results
}
