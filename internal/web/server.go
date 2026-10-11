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
	"github.com/surtr85/agent-memory/internal/decision"
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
	decision *decision.Engine
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
		decision: decision.NewEngine(cfg),
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
	mux.HandleFunc("/api/facts/invalidate", s.handleInvalidateFact)
	mux.HandleFunc("/api/facts/explain", s.handleExplainFact)
	mux.HandleFunc("/api/graph", s.handleGraph)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/decision", s.handleDecision)
	mux.HandleFunc("/api/db/optimize", s.handleOptimizeDB)
	mux.HandleFunc("/api/export", s.handleExport)

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

	type GraphFact struct {
		Predicate string `json:"predicate"`
		Object    string `json:"object"`
	}

	type GraphNode struct {
		ID        string      `json:"id"`
		Label     string      `json:"label"`
		Type      string      `json:"type"` // 'concept', 'section', 'subject', 'tool'
		Namespace string      `json:"namespace"`
		Degree    int         `json:"degree"`
		Facts     []GraphFact `json:"facts,omitempty"`
	}

	type GraphEdge struct {
		Source   string  `json:"source"`
		Target   string  `json:"target"`
		Relation string  `json:"relation"`
		Weight   float64 `json:"weight"`
	}

	nodeMap := make(map[string]GraphNode)
	nodeFacts := make(map[string][]GraphFact)
	degreeMap := make(map[string]int)
	var edges []GraphEdge

	cleanLabel := func(raw string) string {
		trimmed := strings.TrimSpace(raw)
		trimmed = strings.TrimLeft(trimmed, "# \t")
		// Strip leading numeric numbering like "1. ", "2.4 "
		for len(trimmed) > 3 && (trimmed[0] >= '0' && trimmed[0] <= '9') && (trimmed[1] == '.' || trimmed[2] == '.') {
			parts := strings.SplitN(trimmed, " ", 2)
			if len(parts) == 2 {
				trimmed = strings.TrimSpace(parts[1])
			} else {
				break
			}
		}
		if len(trimmed) > 40 {
			trimmed = trimmed[:37] + "..."
		}
		return trimmed
	}

	// 1. Load entities
	eRows, err := s.db.Query(`SELECT name, entity_type, namespace FROM entities LIMIT 300`)
	if err == nil {
		for eRows.Next() {
			var name, eType, ns string
			if err := eRows.Scan(&name, &eType, &ns); err == nil {
				clean := cleanLabel(name)
				nodeMap[name] = GraphNode{
					ID:        name,
					Label:     clean,
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
					nodeMap[src] = GraphNode{ID: src, Label: cleanLabel(src), Type: "concept", Namespace: "default"}
				}
				if _, ok := nodeMap[tgt]; !ok {
					nodeMap[tgt] = GraphNode{ID: tgt, Label: cleanLabel(tgt), Type: "concept", Namespace: "default"}
				}
				edges = append(edges, GraphEdge{
					Source:   src,
					Target:   tgt,
					Relation: rel,
					Weight:   w,
				})
				degreeMap[src]++
				degreeMap[tgt]++
			}
		}
		rRows.Close()
	}

	// 3. Load active facts:
	// If object is short (<= 35 chars) -> it's an entity node in graph.
	// If object is long (> 35 chars / paragraph) -> it's a detail attribute of the subject, NOT a standalone node!
	fRows, err := s.db.Query(`SELECT subject, predicate, object, namespace, salience FROM facts WHERE valid_until IS NULL LIMIT 250`)
	if err == nil {
		for fRows.Next() {
			var subj, pred, obj, ns string
			var salience float64
			if err := fRows.Scan(&subj, &pred, &obj, &ns, &salience); err == nil {
				cleanSubj := cleanLabel(subj)
				if _, ok := nodeMap[subj]; !ok {
					nodeMap[subj] = GraphNode{ID: subj, Label: cleanSubj, Type: "subject", Namespace: ns}
				}

				trimmedObj := strings.TrimSpace(obj)
				isLongText := len(trimmedObj) > 35 || strings.Contains(trimmedObj, "\n") || strings.Contains(trimmedObj, ". ")

				if isLongText {
					// Store as detailed fact metadata on subject node
					nodeFacts[subj] = append(nodeFacts[subj], GraphFact{
						Predicate: pred,
						Object:    trimmedObj,
					})
				} else {
					// Short object: valid graph entity
					cleanObj := cleanLabel(trimmedObj)
					if _, ok := nodeMap[trimmedObj]; !ok {
						nodeMap[trimmedObj] = GraphNode{ID: trimmedObj, Label: cleanObj, Type: "concept", Namespace: ns}
					}
					edges = append(edges, GraphEdge{
						Source:   subj,
						Target:   trimmedObj,
						Relation: pred,
						Weight:   salience,
					})
					degreeMap[subj]++
					degreeMap[trimmedObj]++
				}
			}
		}
		fRows.Close()
	}

	var nodes []GraphNode
	for id, n := range nodeMap {
		n.Degree = degreeMap[id]
		if facts, ok := nodeFacts[id]; ok {
			n.Facts = facts
		}
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

func (s *Server) handleInvalidateFact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	if err := s.temporal.InvalidateFact(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "invalidated_id": req.ID})
}

func (s *Server) handleExplainFact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id query parameter is required", http.StatusBadRequest)
		return
	}

	ev, err := s.temporal.ExplainFact(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(ev)
}

func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		State  string `json:"state"`
		Preset string `json:"preset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.State == "" {
		http.Error(w, "state is required", http.StatusBadRequest)
		return
	}
	if req.Preset == "" {
		req.Preset = "router"
	}

	start := time.Now()
	res, err := s.decision.Decide(r.Context(), req.State, req.Preset)
	elapsed := time.Since(start)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"result":     res,
		"preset":     req.Preset,
		"latency_ms": float64(elapsed.Microseconds()) / 1000.0,
		"engine":     "laya-system1-vulkan",
	})
}

func (s *Server) handleOptimizeDB(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	_, err1 := s.db.Exec(`VACUUM;`)
	_, err2 := s.db.Exec(`PRAGMA optimize;`)
	elapsed := time.Since(start)

	if err1 != nil || err2 != nil {
		http.Error(w, "optimization failed", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"status":     "optimized",
		"latency_ms": float64(elapsed.Microseconds()) / 1000.0,
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=agent-memory-%s.json", time.Now().Format("20060102-150405")))

	blocksList, _ := s.blocks.ListBlocks()
	factsList, _ := s.temporal.GetActiveFacts("")

	json.NewEncoder(w).Encode(map[string]any{
		"exported_at":  time.Now().UTC().Format(time.RFC3339),
		"version":      "3.5.0",
		"blocks":       blocksList,
		"active_facts": factsList,
	})
}
