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

// 7. memory_add_fact (args: namespace, subject, predicate, object, source, source_uri, source_quote, line_number, salience)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_add_fact",
			mcp.WithDescription("Records a bi-temporal atomic fact (triple) with automatic contradiction resolution and source citation."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Fact domain namespace (e.g., 'system', 'forex', 'general')")),
			mcp.WithString("subject", mcp.Required(), mcp.Description("Subject of the assertion")),
			mcp.WithString("predicate", mcp.Required(), mcp.Description("Predicate/relation of the assertion")),
			mcp.WithString("object", mcp.Required(), mcp.Description("Object value of the assertion")),
			mcp.WithString("source", mcp.Description("Optional origin or reference for this fact")),
			mcp.WithString("source_uri", mcp.Description("Optional source URI e.g. memory://live/human.md or chat://session/msg_42")),
			mcp.WithString("source_quote", mcp.Description("Optional exact quote evidence supporting this fact")),
			mcp.WithInteger("line_number", mcp.Description("Optional line citation")),
			mcp.WithNumber("salience", mcp.Description("Optional importance score (0.0 to 2.0)")),
		),
		s.handleAddFact,
	)

	// 8. memory_explain (args: id)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_explain",
			mcp.WithDescription("Explains a claim or fact with exact quotes, line citations, source URI, and supersession history."),
			mcp.WithString("id", mcp.Required(), mcp.Description("Fact or claim ID to explain")),
		),
		s.handleExplain,
	)

	// 9. memory_get_active_facts (args: namespace)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_get_active_facts",
			mcp.WithDescription("Retrieves all currently active facts (valid_until IS NULL) in a given namespace."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Domain namespace")),
		),
		s.handleGetActiveFacts,
	)

	// 10. memory_get_facts_at (args: namespace, timestamp)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_get_facts_at",
			mcp.WithDescription("Time-travel query: retrieves facts that were valid in a namespace at a specific RFC3339 timestamp."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Domain namespace")),
			mcp.WithString("timestamp", mcp.Required(), mcp.Description("RFC3339 timestamp (e.g. 2026-03-01T12:00:00Z)")),
		),
		s.handleGetFactsAt,
	)

	// 11. memory_search (args: query, namespace, top_k, include_historical)
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

	// 12. memory_ingest_markdown (args: namespace, title, content, bank, source_uri)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_ingest_markdown",
			mcp.WithDescription("Ingests a markdown document: splits into header sections, computes embeddings, and indexes chunks into memory banks (world, experience, opinions, reflections)."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Domain namespace")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Document title")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Markdown body text")),
			mcp.WithString("bank", mcp.Description("Memory bank (world, experience, opinions, reflections, people, groups, default: general)")),
			mcp.WithString("source_uri", mcp.Description("Source URI path")),
		),
		s.handleIngestMarkdown,
	)

	// 13. memory_record_observation (args: category, content, namespace, source_uri, line_number)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_record_observation",
			mcp.WithDescription("Records an unconsolidated streaming observation/signal during agent execution."),
			mcp.WithString("category", mcp.Required(), mcp.Description("Category (e.g. 'preference', 'tool_result', 'insight')")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Observation text")),
			mcp.WithString("namespace", mcp.Description("Namespace (defaults to 'default')")),
			mcp.WithString("source_uri", mcp.Description("Origin URI")),
			mcp.WithInteger("line_number", mcp.Description("Line citation number")),
		),
		s.handleRecordObservation,
	)

	// 14. memory_consolidate_observations (args: namespace)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_consolidate_observations",
			mcp.WithDescription("Consolidates pending raw observations into atomic facts and core blocks."),
			mcp.WithString("namespace", mcp.Description("Namespace to consolidate (optional, consolidates all pending if omitted)")),
		),
		s.handleConsolidateObservations,
	)

	// 15. memory_dream_cycle (args: date)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_dream_cycle",
			mcp.WithDescription("Executes autonomous night/consolidation dream cycle: consolidates observations, forms prose reflection, and updates alignment synthesis."),
			mcp.WithString("date", mcp.Description("Dream date in YYYY-MM-DD format (defaults to current date)")),
		),
		s.handleDreamCycle,
	)

	// 16. memory_record_repair (args: trigger_summary, agent_adjustment)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_record_repair",
			mcp.WithDescription("Records a friction/repair thread between agent and user, setting up standing alignment adjustments."),
			mcp.WithString("trigger_summary", mcp.Required(), mcp.Description("Summary of friction or error")),
			mcp.WithString("agent_adjustment", mcp.Required(), mcp.Description("How the agent should adjust future behavior")),
		),
		s.handleRecordRepair,
	)

	// 17. memory_get_alignment
	s.MCPServer.AddTool(
		mcp.NewTool("memory_get_alignment",
			mcp.WithDescription("Retrieves active standing guidance, boundaries, and open repair threads synthesis (<60 tokens)."),
		),
		s.handleGetAlignment,
	)

	// 18. memory_stage_forget (args: pattern, namespace)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_stage_forget",
			mcp.WithDescription("Stage 1 & 2 of Safe Forgetting: identifies matching facts/chunks/entities and verifies safety before retraction."),
			mcp.WithString("pattern", mcp.Required(), mcp.Description("Target pattern or entity to forget")),
			mcp.WithString("namespace", mcp.Description("Optional namespace filter")),
		),
		s.handleStageForget,
	)

	// 19. memory_execute_forget (args: stage_id)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_execute_forget",
			mcp.WithDescription("Stage 3 & 4 of Safe Forgetting: executes cascading retraction across facts, chunks, and graph, and writes permanent tombstones."),
			mcp.WithString("stage_id", mcp.Required(), mcp.Description("Approved Stage ID returned by memory_stage_forget")),
		),
		s.handleExecuteForget,
	)

	// 20. memory_health_check
	s.MCPServer.AddTool(
		mcp.NewTool("memory_health_check",
			mcp.WithDescription("Returns database connection health, table status, fact count, block count, alignment, and tombstones count."),
		),
		s.handleHealthCheck,
	)

	// 21. memory_stats
	s.MCPServer.AddTool(
		mcp.NewTool("memory_stats",
			mcp.WithDescription("Returns detailed namespace, entity, and chunk breakdown statistics."),
		),
		s.handleStats,
	)

	// 22. memory_decision (args: state, preset)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_decision",
			mcp.WithDescription("Executes System-1 / Jev AI fast decision engine (router, triage, reflex)."),
			mcp.WithString("state", mcp.Required(), mcp.Description("Current state, user query, or observation text")),
			mcp.WithString("preset", mcp.Description("Decision preset (e.g. 'router', 'triage', 'reflex', default: 'router')")),
		),
		s.handleDecision,
	)

	// 23. memory_reflect (args: context)
	s.MCPServer.AddTool(
		mcp.NewTool("memory_reflect",
			mcp.WithDescription("Runs Laya System-1 cognitive appraisal of current memory blocks, observations, or goals."),
			mcp.WithString("context", mcp.Description("Optional situational context or memory state snapshot to reflect upon")),
		),
		s.handleReflect,
	)
}

// 1. handleGetBootstrap
func (s *Server) handleGetBootstrap(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prompt, err := s.Blocks.GetBootstrapPrompt()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get bootstrap prompt: %v", err)), nil
	}

	// Append standing alignment synthesis if present (< 60 tokens)
	if s.Alignment != nil {
		synth, _ := s.Alignment.GenerateSynthesisPrompt()
		if synth != "" {
			prompt = prompt + "\n\n" + synth
		}
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
	sourceURI := req.GetString("source_uri", "")
	sourceQuote := req.GetString("source_quote", "")
	lineNo := req.GetInt("line_number", 0)
	salience := req.GetFloat("salience", 0.5)

	fact, err := s.Temporal.AddFactWithCitation(ns, subject, predicate, object, source, sourceURI, sourceQuote, lineNo, salience)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to add fact: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(fact, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 8. handleExplain
func (s *Server) handleExplain(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: id"), nil
	}

	evidence, err := s.Temporal.ExplainFact(id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("explain failed: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(evidence, "", "  ")
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

// 12. handleIngestMarkdown
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
	bank := req.GetString("bank", "general")
	sourceURI := req.GetString("source_uri", "")

	if err := pipeline.IngestMarkdownWithBank(s.DB, s.Embedding, ns, title, content, bank, sourceURI, 1); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("ingestion failed: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("successfully ingested markdown %q into namespace %q (bank: %q)", title, ns, bank)), nil
}

// 13. handleRecordObservation
func (s *Server) handleRecordObservation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	category := req.GetString("category", "")
	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: content"), nil
	}
	ns := req.GetString("namespace", "default")
	sourceURI := req.GetString("source_uri", "")
	lineNo := req.GetInt("line_number", 0)

	if category == "" && s.Decision != nil {
		if triagedCat, _, _, triageErr := s.Decision.TriageObservation(ctx, content); triageErr == nil && triagedCat != "" {
			category = triagedCat
		}
	}
	if category == "" {
		category = "observation"
	}

	if err := pipeline.RecordObservation(s.DB, category, content, ns, sourceURI, lineNo); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to record observation: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("observation recorded in namespace %q with category %q", ns, category)), nil
}

// 14. handleConsolidateObservations
func (s *Server) handleConsolidateObservations(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := pipeline.ConsolidateObservations(s.DB); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("consolidation failed: %v", err)), nil
	}

	return mcp.NewToolResultText("observations consolidated successfully"), nil
}

// 15. handleDreamCycle
func (s *Server) handleDreamCycle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dateStr := req.GetString("date", "")
	res, err := pipeline.RunDreamCycle(s.DB, dateStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("dream cycle failed: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 16. handleRecordRepair
func (s *Server) handleRecordRepair(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	trigger, err := req.RequireString("trigger_summary")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: trigger_summary"), nil
	}
	adjustment, err := req.RequireString("agent_adjustment")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: agent_adjustment"), nil
	}

	thread, err := s.Alignment.RecordRepairThread(trigger, adjustment)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to record repair thread: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(thread, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 17. handleGetAlignment
func (s *Server) handleGetAlignment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	synth, err := s.Alignment.GenerateSynthesisPrompt()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed generating alignment synthesis: %v", err)), nil
	}
	return mcp.NewToolResultText(synth), nil
}

// 18. handleStageForget
func (s *Server) handleStageForget(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pattern, err := req.RequireString("pattern")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: pattern"), nil
	}
	ns := req.GetString("namespace", "")

	res, err := s.Forget.StageForget(ctx, pattern, ns)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("staging forget failed: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 19. handleExecuteForget
func (s *Server) handleExecuteForget(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stageID, err := req.RequireString("stage_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: stage_id"), nil
	}

	receipt, err := s.Forget.ExecuteForget(ctx, stageID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("execute forget failed: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// 20. handleHealthCheck
func (s *Server) handleHealthCheck(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.DB.PingContext(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("database ping failed: %v", err)), nil
	}

	var factCount, blockCount, chunkCount, entityCount, obsCount, tombCount, dreamCount, alignCount int
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM facts").Scan(&factCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM core_blocks").Scan(&blockCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM chunks").Scan(&chunkCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM entities").Scan(&entityCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM observations").Scan(&obsCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM tombstones").Scan(&tombCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM dreams").Scan(&dreamCount)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM alignment_state").Scan(&alignCount)

	health := map[string]any{
		"status":             "healthy",
		"database":           "connected (modernc.org/sqlite WAL)",
		"facts_count":        factCount,
		"blocks_count":       blockCount,
		"chunks_count":       chunkCount,
		"entities_count":     entityCount,
		"observations_count": obsCount,
		"tombstones_count":   tombCount,
		"dreams_count":       dreamCount,
		"alignment_rules":    alignCount,
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
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

// 17. handleReflect
func (s *Server) handleReflect(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.Decision == nil {
		return mcp.NewToolResultError("decision engine is not initialized"), nil
	}

	memContext := req.GetString("context", "")
	if memContext == "" {
		// Bootstrap working memory snapshot if context not explicitly provided
		prompt, err := s.Blocks.GetBootstrapPrompt()
		if err == nil && prompt != "" {
			memContext = prompt
		} else {
			memContext = "Agent idle, core blocks nominal"
		}
	}

	res, err := s.Decision.Reflect(ctx, memContext)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reflection error: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

