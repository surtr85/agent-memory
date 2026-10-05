package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/surtr85/agent-memory/internal/config"
	"github.com/surtr85/agent-memory/internal/db"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_server.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.InitSchema(database); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	cfg := &config.Config{
		DBPath:       dbPath,
		EmbeddingURL: "http://mock-embedding",
		OllamaURL:    "http://mock-ollama",
		LogLevel:     "DEBUG",
	}

	return NewServer(database, cfg)
}

func makeCallReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func getResultText(res *mcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if textContent, ok := res.Content[0].(mcp.TextContent); ok {
		return textContent.Text
	}
	return ""
}

func TestServer_BootstrapAndBlocks(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.DB.Close()
	ctx := context.Background()

	// 1. memory_get_bootstrap
	res, err := srv.handleGetBootstrap(ctx, makeCallReq(nil))
	if err != nil {
		t.Fatalf("handleGetBootstrap returned unexpected error: %v", err)
	}
	text := getResultText(res)
	if text == "" || !res.IsError == false {
		t.Errorf("expected bootstrap content, got: %v", text)
	}

	// 2. memory_set_block
	res, err = srv.handleSetBlock(ctx, makeCallReq(map[string]any{
		"label":   "custom_block",
		"content": "Line 1: init",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleSetBlock failed: %v", getResultText(res))
	}

	// 3. memory_append_block
	res, err = srv.handleAppendBlock(ctx, makeCallReq(map[string]any{
		"label":   "custom_block",
		"content": "Line 2: appended",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleAppendBlock failed: %v", getResultText(res))
	}

	// 4. memory_replace_block
	res, err = srv.handleReplaceBlock(ctx, makeCallReq(map[string]any{
		"label":       "custom_block",
		"old_content": "Line 1: init",
		"new_content": "Line 1: modified",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleReplaceBlock failed: %v", getResultText(res))
	}

	// 5. memory_get_block
	res, err = srv.handleGetBlock(ctx, makeCallReq(map[string]any{
		"label": "custom_block",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleGetBlock failed: %v", getResultText(res))
	}
	text = getResultText(res)
	if text == "" {
		t.Fatalf("expected non-empty block content")
	}

	// 6. memory_list_blocks
	res, err = srv.handleListBlocks(ctx, makeCallReq(nil))
	if err != nil || res.IsError {
		t.Fatalf("handleListBlocks failed: %v", getResultText(res))
	}
}

func TestServer_FactsAndTemporal(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.DB.Close()
	ctx := context.Background()

	// 7. memory_add_fact
	res, err := srv.handleAddFact(ctx, makeCallReq(map[string]any{
		"namespace": "forex",
		"subject":   "EURUSD",
		"predicate": "bias",
		"object":    "bullish",
		"source":    "session-1",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleAddFact failed: %v", getResultText(res))
	}

	// 8. memory_get_active_facts
	res, err = srv.handleGetActiveFacts(ctx, makeCallReq(map[string]any{
		"namespace": "forex",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleGetActiveFacts failed: %v", getResultText(res))
	}

	// Update fact to test contradiction
	time.Sleep(10 * time.Millisecond)
	res, err = srv.handleAddFact(ctx, makeCallReq(map[string]any{
		"namespace": "forex",
		"subject":   "EURUSD",
		"predicate": "bias",
		"object":    "bearish",
		"source":    "session-2",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleAddFact contradiction update failed: %v", getResultText(res))
	}

	// 9. memory_get_facts_at
	nowStr := time.Now().UTC().Format(time.RFC3339)
	res, err = srv.handleGetFactsAt(ctx, makeCallReq(map[string]any{
		"namespace": "forex",
		"timestamp": nowStr,
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleGetFactsAt failed: %v", getResultText(res))
	}
}

func TestServer_IngestAndSearchAndObservations(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.DB.Close()
	ctx := context.Background()

	// 11. memory_ingest_markdown
	res, err := srv.handleIngestMarkdown(ctx, makeCallReq(map[string]any{
		"namespace": "system",
		"title":     "NixOS Setup",
		"content":   "# Configuration\nUsing [[Niri]] compositor with [[Waybar]].",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleIngestMarkdown failed: %v", getResultText(res))
	}

	// 10. memory_search
	res, err = srv.handleSearch(ctx, makeCallReq(map[string]any{
		"query":     "compositor",
		"namespace": "system",
		"top_k":     3,
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleSearch failed: %v", getResultText(res))
	}

	// 12. memory_record_observation
	res, err = srv.handleRecordObservation(ctx, makeCallReq(map[string]any{
		"category":  "preference",
		"content":   "Amadeus prefers concise output",
		"namespace": "agent",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleRecordObservation failed: %v", getResultText(res))
	}

	// 13. memory_consolidate_observations
	res, err = srv.handleConsolidateObservations(ctx, makeCallReq(nil))
	if err != nil || res.IsError {
		t.Fatalf("handleConsolidateObservations failed: %v", getResultText(res))
	}

	// 14. memory_health_check
	res, err = srv.handleHealthCheck(ctx, makeCallReq(nil))
	if err != nil || res.IsError {
		t.Fatalf("handleHealthCheck failed: %v", getResultText(res))
	}

	// 15. memory_stats
	res, err = srv.handleStats(ctx, makeCallReq(nil))
	if err != nil || res.IsError {
		t.Fatalf("handleStats failed: %v", getResultText(res))
	}

	// 16. memory_decision
	res, err = srv.handleDecision(ctx, makeCallReq(map[string]any{
		"state":  "How to configure Niri window manager?",
		"preset": "router",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleDecision failed: %v", getResultText(res))
	}

	// 17. memory_reflect
	res, err = srv.handleReflect(ctx, makeCallReq(map[string]any{
		"context": "Agent memory blocks configured with Niri on NixOS",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleReflect failed: %v", getResultText(res))
	}

	// Test auto-routing in search
	res, err = srv.handleSearch(ctx, makeCallReq(map[string]any{
		"query":     "How to configure Niri window manager?",
		"namespace": "auto",
		"top_k":     3,
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleSearch with auto routing failed: %v", getResultText(res))
	}

	// Test category enrichment in observation
	res, err = srv.handleRecordObservation(ctx, makeCallReq(map[string]any{
		"content": "User prefers dark mode and Neovim",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleRecordObservation with category inference failed: %v", getResultText(res))
	}

	// 18. memory_explain
	activeFacts, _ := srv.Temporal.GetActiveFacts("system")
	var targetID string
	if len(activeFacts) > 0 {
		targetID = activeFacts[0].ID
	} else {
		f, _ := srv.Temporal.AddFact("system", "Test", "is", "active", "unit")
		targetID = f.ID
	}

	res, err = srv.handleExplain(ctx, makeCallReq(map[string]any{
		"id": targetID,
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleExplain failed: %v", getResultText(res))
	}

	// 19. memory_record_repair & memory_get_alignment
	res, err = srv.handleRecordRepair(ctx, makeCallReq(map[string]any{
		"trigger_summary":  "Verbose apologies",
		"agent_adjustment": "Eliminate apologies, fix directly",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleRecordRepair failed: %v", getResultText(res))
	}

	res, err = srv.handleGetAlignment(ctx, makeCallReq(nil))
	if err != nil || res.IsError {
		t.Fatalf("handleGetAlignment failed: %v", getResultText(res))
	}

	// 20. memory_dream_cycle
	res, err = srv.handleDreamCycle(ctx, makeCallReq(map[string]any{
		"date": "2026-03-31",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleDreamCycle failed: %v", getResultText(res))
	}

	// 21. memory_stage_forget & memory_execute_forget
	res, err = srv.handleStageForget(ctx, makeCallReq(map[string]any{
		"pattern":   "Alacritty",
		"namespace": "system",
	}))
	if err != nil || res.IsError {
		t.Fatalf("handleStageForget failed: %v", getResultText(res))
	}
}
