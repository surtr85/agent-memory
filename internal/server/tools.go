package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/surtr85/agent-memory/internal/pipeline"
)

func (s *Server) registerTools() {
	// 1. memory_get_bootstrap
	s.MCPServer.AddTool(
		mcp.NewTool("memory_get_bootstrap",
			mcp.WithDescription("Returns compact bootstrap prompt from working core memory blocks for prompt injection (< 400 tokens)."),
		),
		s.handleGetBootstrap,
	)

	// 2. memory_get_block (args: label)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_get_block",
			mcp.WithDescription("Retrieves the content of a core working memory block by label (e.g. human, persona, environment, scratchpad)."),
			mcp.WithString("label", mcp.Required(), mcp.Description("The label of the block to retrieve")),
		),
		s.handleGetBlock,
	)

	// 3. memory_set_block (args: label, content)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_set_block",
			mcp.WithDescription("Creates or overwrites a core working memory block with new content."),
			mcp.WithString("label", mcp.Required(), mcp.Description("The label of the block")),
			mcp.WithString("content", mcp.Required(), mcp.Description("The content to store in the block")),
		),
		s.handleSetBlock,
	)

	// 4. memory_replace_block (args: label, old_content, new_content)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_replace_block",
			mcp.WithDescription("Replaces a targeted substring in a core memory block with new content."),
			mcp.WithString("label", mcp.Required(), mcp.Description("The label of the block")),
			mcp.WithString("old_content", mcp.Required(), mcp.Description("The existing text substring to replace")),
			mcp.WithString("new_content", mcp.Required(), mcp.Description("The new replacement text")),
		),
		s.handleReplaceBlock,
	)

	// 5. memory_append_block (args: label, content)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_append_block",
			mcp.WithDescription("Appends content on a new line to an existing core memory block, or creates it."),
			mcp.WithString("label", mcp.Required(), mcp.Description("The label of the block")),
			mcp.WithString("content", mcp.Required(), mcp.Description("The content to append")),
		),
		s.handleAppendBlock,
	)

	// 6. memory_list_blocks
	s.MCPServer.AddTool(
		mcp.NewTool("memory_list_blocks",
			mcp.WithDescription("Lists all in-context core working memory blocks."),
		),
		s.handleListBlocks,
	)

	// 7. memory_add_fact (args: namespace, subject, predicate, object, source)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_add_fact",
			mcp.WithDescription("Records a bi-temporal atomic fact (triple) with automatic contradiction resolution."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Fact domain namespace (e.g., 'system', 'forex', 'general')")),
			mcp.WithString("subject", mcp.Required(), mcp.Description("Subject of the assertion")),
			mcp.WithString("predicate", mcp.Required(), mcp.Description("Predicate/relation of the assertion")),
			mcp.WithString("object", mcp.Required(), mcp.Description("Object value of the assertion")),
			mcp.WithString("source", mcp.Description("Optional origin or reference for this fact")),
		),
		s.handleAddFact,
	)

	// 8. memory_get_active_facts (args: namespace)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_get_active_facts",
			mcp.WithDescription("Retrieves all currently active facts (valid_until IS NULL) in a given namespace."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Domain namespace")),
		),
		s.handleGetActiveFacts,
	)

	// 9. memory_get_facts_at (args: namespace, timestamp)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_get_facts_at",
			mcp.WithDescription("Time-travel query: retrieves facts that were valid in a namespace at a specific RFC3339 timestamp."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Domain namespace")),
			mcp.WithString("timestamp", mcp.Required(), mcp.Description("RFC3339 timestamp (e.g. 2026-03-01T12:00:00Z)")),
		),
		s.handleGetFactsAt,
	)

	// 10. memory_search (args: query, namespace, top_k, include_historical)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_search",
			mcp.WithDescription("Executes 4-way hybrid search (Dense Vector + BM25 + Graph Traversal + Temporal Slicing) with RRF fusion."),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query string")),
			mcp.WithString("namespace", mcp.Description("Filter results to this namespace (optional)")),
			mcp.WithInteger("top_k", mcp.Description("Maximum number of results to return (default 5)")),
			mcp.WithBoolean("include_historical", mcp.Description("Whether to include invalidated/historical facts (default false)")),
		),
		s.handleSearch,
	)

	// 11. memory_ingest_markdown (args: namespace, title, content)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_ingest_markdown",
			mcp.WithDescription("Ingests a markdown document: splits into header sections, computes embeddings, and indexes chunks and wikilink entities."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Domain namespace")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Document title")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Markdown body text")),
		),
		s.handleIngestMarkdown,
	)

	// 12. memory_record_observation (args: category, content, namespace)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_record_observation",
			mcp.WithDescription("Records an unconsolidated streaming observation/signal during agent execution."),
			mcp.WithString("category", mcp.Required(), mcp.Description("Category (e.g. 'preference', 'tool_result', 'insight')")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Observation text")),
			mcp.WithString("namespace", mcp.Description("Namespace (defaults to 'default')")),
		),
		s.handleRecordObservation,
	)

	// 13. memory_consolidate_observations (args: namespace)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_consolidate_observations",
			mcp.WithDescription("Consolidates pending raw observations into atomic facts and core blocks."),
			mcp.WithString("namespace", mcp.Description("Namespace to consolidate (optional, consolidates all pending if omitted)")),
		),
		s.handleConsolidateObservations,
	)

	// 14. memory_health_check
	s.MCPServer.AddTool(
		mcp.NewTool("memory_health_check",
			mcp.WithDescription("Returns database connection health, table status, fact count, block count, and chunk count."),
		),
		s.handleHealthCheck,
	)

	// 15. memory_stats
	s.MCPServer.AddTool(
		mcp.NewTool("memory_stats",
			mcp.WithDescription("Returns detailed namespace, entity, and chunk breakdown statistics."),
		),
		s.handleStats,
	)

	// 16. memory_decision (args: state, preset)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_decision",
			mcp.WithDescription("Executes System-1 / Jev AI fast decision engine (router, triage, reflex)."),
			mcp.WithString("state", mcp.Required(), mcp.Description("Current state, user query, or observation text")),
			mcp.WithString("preset", mcp.Description("Decision preset (e.g. 'router', 'triage', 'reflex', default: 'router')")),
		),
		s.handleDecision,
	)
}

// 1. handleGetBootstrap
func (s *Server) handleGetBootstrap(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prompt, err := s.Blocks.GetBootstrapPrompt()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get bootstrap prompt: %v", err)), nil
	}
	return mcp.NewToolResultText(prompt), nil
}

// 2. handleGetBlock
func (s *Server) handleGetBlock(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: label"), nil
	}

	block, err := s.Blocks.GetBlock(label)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	jsonBytes, err := json.MarshalIndent(block, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 3. handleSetBlock
func (s *Server) handleSetBlock(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: label"), nil
	}
	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: content"), nil
	}

	if err := s.Blocks.SetBlock(label, content); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to set block %s: %v", label, err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("block %q updated successfully", label)), nil
}

// 4. handleReplaceBlock
func (s *Server) handleReplaceBlock(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: label"), nil
	}
	oldContent, err := req.RequireString("old_content")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: old_content"), nil
	}
	newContent, err := req.RequireString("new_content")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: new_content"), nil
	}

	if err := s.Blocks.ReplaceBlock(label, oldContent, newContent); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to replace in block %s: %v", label, err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("block %q updated successfully", label)), nil
}

// 5. handleAppendBlock
func (s *Server) handleAppendBlock(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: label"), nil
	}
	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: content"), nil
	}

	if err := s.Blocks.AppendBlock(label, content); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to append to block %s: %v", label, err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("content appended to block %q successfully", label)), nil
}

// 6. handleListBlocks
func (s *Server) handleListBlocks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	blocksList, err := s.Blocks.ListBlocks()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list blocks: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(blocksList, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 7. handleAddFact
func (s *Server) handleAddFact(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ns, err := req.RequireString("namespace")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: namespace"), nil
	}
	subject, err := req.RequireString("subject")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: subject"), nil
	}
	predicate, err := req.RequireString("predicate")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: predicate"), nil
	}
	object, err := req.RequireString("object")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: object"), nil
	}
	source := req.GetString("source", "mcp")

	fact, err := s.Temporal.AddFact(ns, subject, predicate, object, source)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to add fact: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(fact, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 8. handleGetActiveFacts
func (s *Server) handleGetActiveFacts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ns, err := req.RequireString("namespace")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: namespace"), nil
	}

	facts, err := s.Temporal.GetActiveFacts(ns)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get active facts: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 9. handleGetFactsAt
func (s *Server) handleGetFactsAt(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ns, err := req.RequireString("namespace")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: namespace"), nil
	}
	tsStr, err := req.RequireString("timestamp")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: timestamp"), nil
	}

	t, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid timestamp %q: %v", tsStr, err)), nil
		}
	}

	facts, err := s.Temporal.GetFactsAt(ns, t)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get facts at timestamp: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 10. handleSearch
func (s *Server) handleSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: query"), nil
	}
	namespace := req.GetString("namespace", "")
	topK := req.GetInt("top_k", 5)
	includeHistorical := req.GetBool("include_historical", false)

	if (namespace == "" || namespace == "auto") && s.Decision != nil {
		if routedNS, _, routeErr := s.Decision.RouteQuery(ctx, query); routeErr == nil && routedNS != "" {
			namespace = routedNS
		}
	}

	results, err := s.Searcher.Search(ctx, query, namespace, topK, includeHistorical)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 11. handleIngestMarkdown
func (s *Server) handleIngestMarkdown(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ns, err := req.RequireString("namespace")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: namespace"), nil
	}
	title, err := req.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: title"), nil
	}
	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: content"), nil
	}

	if err := pipeline.IngestMarkdown(s.DB, s.Embedding, ns, title, content); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("ingestion failed: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("successfully ingested markdown %q into namespace %q", title, ns)), nil
}

// 12. handleRecordObservation
func (s *Server) handleRecordObservation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	category := req.GetString("category", "")
	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: content"), nil
	}
	ns := req.GetString("namespace", "default")

	if category == "" && s.Decision != nil {
		if triagedCat, _, _, triageErr := s.Decision.TriageObservation(ctx, content); triageErr == nil && triagedCat != "" {
			category = triagedCat
		}
	}
	if category == "" {
		category = "observation"
	}

	if err := pipeline.RecordObservation(s.DB, category, content, ns); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to record observation: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("observation recorded in namespace %q with category %q", ns, category)), nil
}

// 13. handleConsolidateObservations
func (s *Server) handleConsolidateObservations(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := pipeline.ConsolidateObservations(s.DB); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("consolidation failed: %v", err)), nil
	}

	return mcp.NewToolResultText("observations consolidated successfully"), nil
}

// 14. handleHealthCheck
func (s *Server) handleHealthCheck(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.DB.PingContext(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("database ping failed: %v", err)), nil
	}

	var factCount, blockCount, chunkCount, entityCount, obsCount int
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM facts").Scan(&factCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM core_blocks").Scan(&blockCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM chunks").Scan(&chunkCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM entities").Scan(&entityCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM observations").Scan(&obsCount)

	health := map[string]any{
		"status":        "healthy",
		"database":      "connected (modernc.org/sqlite WAL)",
		"facts_count":   factCount,
		"blocks_count":  blockCount,
		"chunks_count":  chunkCount,
		"entities_count": entityCount,
		"observations_count": obsCount,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}

	jsonBytes, err := json.MarshalIndent(health, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 15. handleStats
func (s *Server) handleStats(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Fact breakdown by namespace
	factRows, err := s.DB.QueryContext(ctx, `SELECT namespace, COUNT(*), SUM(CASE WHEN valid_until IS NULL THEN 1 ELSE 0 END) FROM facts GROUP BY namespace`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed querying facts stats: %v", err)), nil
	}
	defer factRows.Close()

	type nsFactStat struct {
		Namespace string `json:"namespace"`
		Total     int    `json:"total"`
		Active    int    `json:"active"`
	}
	var factStats []nsFactStat
	for factRows.Next() {
		var st nsFactStat
		if err := factRows.Scan(&st.Namespace, &st.Total, &st.Active); err == nil {
			factStats = append(factStats, st)
		}
	}

	// Chunk breakdown by namespace
	chunkRows, err := s.DB.QueryContext(ctx, `SELECT namespace, COUNT(*) FROM chunks GROUP BY namespace`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed querying chunks stats: %v", err)), nil
	}
	defer chunkRows.Close()

	type nsChunkStat struct {
		Namespace string `json:"namespace"`
		Count     int    `json:"count"`
	}
	var chunkStats []nsChunkStat
	for chunkRows.Next() {
		var cs nsChunkStat
		if err := chunkRows.Scan(&cs.Namespace, &cs.Count); err == nil {
			chunkStats = append(chunkStats, cs)
		}
	}

	// Entity breakdown by type
	entRows, err := s.DB.QueryContext(ctx, `SELECT entity_type, COUNT(*) FROM entities GROUP BY entity_type`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed querying entity stats: %v", err)), nil
	}
	defer entRows.Close()

	type entityStat struct {
		Type  string `json:"type"`
		Count int    `json:"count"`
	}
	var entityStats []entityStat
	for entRows.Next() {
		var es entityStat
		if err := entRows.Scan(&es.Type, &es.Count); err == nil {
			entityStats = append(entityStats, es)
		}
	}

	stats := map[string]any{
		"namespaces_facts":  factStats,
		"namespaces_chunks": chunkStats,
		"entities_by_type":  entityStats,
		"generated_at":      time.Now().UTC().Format(time.RFC3339),
	}

	jsonBytes, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 16. handleDecision
func (s *Server) handleDecision(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state, err := req.RequireString("state")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: state"), nil
	}
	preset := req.GetString("preset", "router")

	if s.Decision == nil {
		return mcp.NewToolResultError("decision engine is not initialized"), nil
	}

	res, err := s.Decision.Decide(ctx, state, preset)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("decision error: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}
