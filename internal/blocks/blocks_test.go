package blocks

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/surtr85/agent-memory/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	if err := db.InitSchema(database); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	return database
}

func TestSeedDefaults(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	mgr := NewManager(database)
	if err := mgr.SeedDefaults(); err != nil {
		t.Fatalf("SeedDefaults failed: %v", err)
	}

	blocks, err := mgr.ListBlocks()
	if err != nil {
		t.Fatalf("ListBlocks failed: %v", err)
	}

	if len(blocks) != 3 {
		t.Fatalf("expected 3 default blocks, got %d", len(blocks))
	}

	labels := map[string]bool{}
	for _, b := range blocks {
		labels[b.Label] = true
	}
	for _, expected := range []string{"persona", "human", "environment"} {
		if !labels[expected] {
			t.Errorf("missing expected block %s", expected)
		}
	}

	// Calling SeedDefaults again should be a no-op
	if err := mgr.SeedDefaults(); err != nil {
		t.Fatalf("SeedDefaults secondary call failed: %v", err)
	}
	blocksAfter, _ := mgr.ListBlocks()
	if len(blocksAfter) != 3 {
		t.Errorf("expected still 3 blocks, got %d", len(blocksAfter))
	}
}

func TestSetAndGetBlock(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	mgr := NewManager(database)

	err := mgr.SetBlock("human", "User prefers Go")
	if err != nil {
		t.Fatalf("SetBlock failed: %v", err)
	}

	blk, err := mgr.GetBlock("human")
	if err != nil {
		t.Fatalf("GetBlock failed: %v", err)
	}
	if blk.Content != "User prefers Go" {
		t.Errorf("expected content 'User prefers Go', got %q", blk.Content)
	}

	// Update existing block
	err = mgr.SetBlock("human", "User prefers Go and Nix")
	if err != nil {
		t.Fatalf("SetBlock update failed: %v", err)
	}
	blk, err = mgr.GetBlock("human")
	if err != nil {
		t.Fatalf("GetBlock failed: %v", err)
	}
	if blk.Content != "User prefers Go and Nix" {
		t.Errorf("expected content 'User prefers Go and Nix', got %q", blk.Content)
	}
}

func TestReplaceBlock(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	mgr := NewManager(database)

	if err := mgr.SetBlock("persona", "Style: friendly and casual."); err != nil {
		t.Fatalf("SetBlock failed: %v", err)
	}

	// Target not found error
	if err := mgr.ReplaceBlock("persona", "grumpy", "enthusiastic"); err == nil {
		t.Error("expected error when replacing nonexistent string, got nil")
	}

	// Successful replacement
	if err := mgr.ReplaceBlock("persona", "casual", "rigorous"); err != nil {
		t.Fatalf("ReplaceBlock failed: %v", err)
	}

	blk, err := mgr.GetBlock("persona")
	if err != nil {
		t.Fatalf("GetBlock failed: %v", err)
	}
	if blk.Content != "Style: friendly and rigorous." {
		t.Errorf("unexpected content: %q", blk.Content)
	}
}

func TestAppendBlock(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	mgr := NewManager(database)

	// Append to new block
	if err := mgr.AppendBlock("scratchpad", "line 1"); err != nil {
		t.Fatalf("AppendBlock failed: %v", err)
	}
	blk, _ := mgr.GetBlock("scratchpad")
	if blk.Content != "line 1" {
		t.Errorf("expected 'line 1', got %q", blk.Content)
	}

	// Append to existing
	if err := mgr.AppendBlock("scratchpad", "line 2"); err != nil {
		t.Fatalf("AppendBlock failed: %v", err)
	}
	blk, _ = mgr.GetBlock("scratchpad")
	expected := "line 1\nline 2"
	if blk.Content != expected {
		t.Errorf("expected %q, got %q", expected, blk.Content)
	}
}

func TestGetBootstrapPrompt(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	mgr := NewManager(database)
	prompt, err := mgr.GetBootstrapPrompt()
	if err != nil {
		t.Fatalf("GetBootstrapPrompt on empty db failed: %v", err)
	}
	if prompt != "" {
		t.Errorf("expected empty prompt for empty db, got %q", prompt)
	}

	if err := mgr.SeedDefaults(); err != nil {
		t.Fatalf("SeedDefaults failed: %v", err)
	}

	prompt, err = mgr.GetBootstrapPrompt()
	if err != nil {
		t.Fatalf("GetBootstrapPrompt failed: %v", err)
	}

	if !strings.Contains(prompt, "### Core Memory Context") {
		t.Errorf("prompt missing header: %s", prompt)
	}
	if !strings.Contains(prompt, "<human>") || !strings.Contains(prompt, "</human>") {
		t.Errorf("prompt missing <human> tag: %s", prompt)
	}
	if !strings.Contains(prompt, "<persona>") || !strings.Contains(prompt, "</persona>") {
		t.Errorf("prompt missing <persona> tag: %s", prompt)
	}
	if !strings.Contains(prompt, "<environment>") || !strings.Contains(prompt, "</environment>") {
		t.Errorf("prompt missing <environment> tag: %s", prompt)
	}

	// Add domain-specific block
	if err := mgr.SetBlock("konkur_1405", "Biology syllabus details"); err != nil {
		t.Fatalf("SetBlock failed: %v", err)
	}

	// Default bootstrap should NOT dump domain block content directly
	defaultPrompt, _ := mgr.GetBootstrapPrompt()
	if strings.Contains(defaultPrompt, "Biology syllabus details") {
		t.Errorf("default prompt should not dump domain block content directly: %s", defaultPrompt)
	}
	if !strings.Contains(defaultPrompt, "Domain blocks available on demand via memory_get_block: konkur_1405") {
		t.Errorf("default prompt should mention domain block available on demand: %s", defaultPrompt)
	}

	// Scoped with includeDomain = true should include it
	fullPrompt, _ := mgr.GetBootstrapPromptScoped(true)
	if !strings.Contains(fullPrompt, "<konkur_1405>") {
		t.Errorf("scoped prompt with includeDomain=true should include konkur_1405: %s", fullPrompt)
	}
}
