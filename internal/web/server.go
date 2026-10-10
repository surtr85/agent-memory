package web

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/surtr85/agent-memory/internal/blocks"
	"github.com/surtr85/agent-memory/internal/config"
	"github.com/surtr85/agent-memory/internal/retrieval"
	"github.com/surtr85/agent-memory/internal/temporal"
)

//go:embed assets/*
var assetsFS embed.FS

// Server hosts the embedded Web UI and REST API.
type Server struct {
	db       *sql.DB
	cfg      *config.Config
	blocks   *blocks.Manager
	temporal *temporal.Registry
	searcher *retrieval.Searcher
	port     int
}

// NewServer creates a new web dashboard server.
func NewServer(db *sql.DB, cfg *config.Config, searcher *retrieval.Searcher, port int) *Server {
	if port <= 0 {
		port = 3200
	}
	return &Server{
		db:       db,
		cfg:      cfg,
		blocks:   blocks.NewManager(db),
		temporal: temporal.NewRegistry(db),
		searcher: searcher,
		port:     port,
	}
}

// Start launches the HTTP server and blocks until context cancellation or error.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// REST APIs
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/blocks", s.handleBlocks)
	mux.HandleFunc("/api/facts", s.handleFacts)
	mux.HandleFunc("/api/graph", s.handleGraph)
	mux.HandleFunc("/api/search", s.handleSearch)

	// Embedded Static UI
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return fmt.Errorf("failed creating embedded sub-FS: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	fmt.Printf("AgentMemory Web UI listening on http://%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "healthy",
		"database":  "connected",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "3.5.0",
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var factsTotal, factsActive, blocksCount, chunksCount, entitiesCount, relationsCount int
	_ = s.db.QueryRow(`SELECT count(*) FROM facts`).Scan(&factsTotal)
	_ = s.db.QueryRow(`SELECT count(*) FROM facts WHERE valid_until IS NULL`).Scan(&factsActive)
	_ = s.db.QueryRow(`SELECT count(*) FROM core_blocks`).Scan(&blocksCount)
	_ = s.db.QueryRow(`SELECT count(*) FROM chunks`).Scan(&chunksCount)
	_ = s.db.QueryRow(`SELECT count(*) FROM entities`).Scan(&entitiesCount)
	_ = s.db.QueryRow(`SELECT count(*) FROM entity_relations`).Scan(&relationsCount)

	json.NewEncoder(w).Encode(map[string]any{
		"facts_total":     factsTotal,
		"facts_active":    factsActive,
		"blocks_count":    blocksCount,
		"chunks_count":    chunksCount,
		"entities_count":  entitiesCount,
		"relations_count": relationsCount,
	})
}

func (s *Server) handleBlocks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		list, err := s.blocks.ListBlocks()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(list)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Label   string `json:"label"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Label == "" {
			http.Error(w, "label is required", http.StatusBadRequest)
			return
		}
		if err := s.blocks.SetBlock(req.Label, req.Content); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleFacts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		ns := r.URL.Query().Get("namespace")
		activeOnly := r.URL.Query().Get("active") != "false"

		query := `SELECT id, namespace, subject, predicate, object, confidence, source, source_uri, source_quote, line_number, salience, valid_from, valid_until, recorded_at, invalidated_at, superseded_by FROM facts`
		var conds []string
		var args []any

		if ns != "" {
			conds = append(conds, "namespace = ?")
			args = append(args, ns)
		}
		if activeOnly {
			conds = append(conds, "valid_until IS NULL")
		}
		if len(conds) > 0 {
			query += " WHERE " + strings.Join(conds, " AND ")
		}
		query += " ORDER BY recorded_at DESC LIMIT 200"

		rows, err := s.db.Query(query, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var facts []map[string]any
		for rows.Next() {
			var id, namespace, subj, pred, obj string
			var conf, salience float64
			var src, srcURI, srcQuote sql.NullString
			var lineNo int
			var validFrom, recAt time.Time
			var validUntil, inDate sql.NullTime
			var supBy sql.NullString

			if err := rows.Scan(&id, &namespace, &subj, &pred, &obj, &conf, &src, &srcURI, &srcQuote, &lineNo, &salience, &validFrom, &validUntil, &recAt, &inDate, &supBy); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			f := map[string]any{
				"id":           id,
				"namespace":    namespace,
				"subject":      subj,
				"predicate":    pred,
				"object":       obj,
				"confidence":   conf,
				"source":       src.String,
				"source_uri":   srcURI.String,
				"source_quote": srcQuote.String,
				"line_number":  lineNo,
				"salience":     salience,
				"valid_from":   validFrom.Format(time.RFC3339),
				"recorded_at":  recAt.Format(time.RFC3339),
				"is_active":    !validUntil.Valid,
			}
			if validUntil.Valid {
				f["valid_until"] = validUntil.Time.Format(time.RFC3339)
			}
			if inDate.Valid {
				f["invalidated_at"] = inDate.Time.Format(time.RFC3339)
			}
			if supBy.Valid {
				f["superseded_by"] = supBy.String
			}
			facts = append(facts, f)
		}

		json.NewEncoder(w).Encode(facts)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Namespace   string  `json:"namespace"`
			Subject     string  `json:"subject"`
			Predicate   string  `json:"predicate"`
			Object      string  `json:"object"`
			Source      string  `json:"source"`
			SourceURI   string  `json:"source_uri"`
			SourceQuote string  `json:"source_quote"`
			LineNumber  int     `json:"line_number"`
			Salience    float64 `json:"salience"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Namespace == "" || req.Subject == "" || req.Predicate == "" || req.Object == "" {
			http.Error(w, "namespace, subject, predicate, and object are required", http.StatusBadRequest)
			return
		}

		fact, err := s.temporal.AddFactWithCitation(req.Namespace, req.Subject, req.Predicate, req.Object, req.Source, req.SourceURI, req.SourceQuote, req.LineNumber, req.Salience)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(fact)
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type GraphNode struct {
		ID        string `json:"id"`
		Label     string `json:"label"`
		Type      string `json:"type"` // 'entity', 'fact_subj', 'fact_obj'
		Namespace string `json:"namespace"`
	}
	type GraphEdge struct {
		Source   string  `json:"source"`
		Target   string  `json:"target"`
		Relation string  `json:"relation"`
		Weight   float64 `json:"weight"`
	}

	nodeMap := make(map[string]GraphNode)
	var edges []GraphEdge

	// 1. Load entities
	eRows, err := s.db.Query(`SELECT name, entity_type, namespace FROM entities LIMIT 300`)
	if err == nil {
		for eRows.Next() {
			var name, eType, ns string
			if err := eRows.Scan(&name, &eType, &ns); err == nil {
				nodeMap[name] = GraphNode{
					ID:        name,
					Label:     name,
					Type:      eType,
					Namespace: ns,
				}
			}
		}
		eRows.Close()
	}

	// 2. Load entity relations
	rRows, err := s.db.Query(`SELECT source_entity, target_entity, relation_type, weight FROM entity_relations LIMIT 500`)
	if err == nil {
		for rRows.Next() {
			var src, tgt, rel string
			var w float64
			if err := rRows.Scan(&src, &tgt, &rel, &w); err == nil {
				if _, ok := nodeMap[src]; !ok {
					nodeMap[src] = GraphNode{ID: src, Label: src, Type: "concept", Namespace: "default"}
				}
				if _, ok := nodeMap[tgt]; !ok {
					nodeMap[tgt] = GraphNode{ID: tgt, Label: tgt, Type: "concept", Namespace: "default"}
				}
				edges = append(edges, GraphEdge{
					Source:   src,
					Target:   tgt,
					Relation: rel,
					Weight:   w,
				})
			}
		}
		rRows.Close()
	}

	// 3. Load active facts as graph triples (subject -> predicate -> object)
	fRows, err := s.db.Query(`SELECT subject, predicate, object, namespace, salience FROM facts WHERE valid_until IS NULL LIMIT 200`)
	if err == nil {
		for fRows.Next() {
			var subj, pred, obj, ns string
			var salience float64
			if err := fRows.Scan(&subj, &pred, &obj, &ns, &salience); err == nil {
				if _, ok := nodeMap[subj]; !ok {
					nodeMap[subj] = GraphNode{ID: subj, Label: subj, Type: "subject", Namespace: ns}
				}
				if _, ok := nodeMap[obj]; !ok {
					nodeMap[obj] = GraphNode{ID: obj, Label: obj, Type: "object", Namespace: ns}
				}
				edges = append(edges, GraphEdge{
					Source:   subj,
					Target:   obj,
					Relation: pred,
					Weight:   salience,
				})
			}
		}
		fRows.Close()
	}

	var nodes []GraphNode
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

	json.NewEncoder(w).Encode(map[string]any{
		"nodes": nodes,
		"links": edges,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query      string `json:"query"`
		Namespace  string `json:"namespace"`
		TopK       int    `json:"top_k"`
		Historical bool   `json:"historical"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}

	results, err := s.searcher.Search(r.Context(), req.Query, req.Namespace, req.TopK, req.Historical)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(results)
}
