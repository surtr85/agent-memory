package pipeline

import (
	"path/filepath"
	"testing"

	"github.com/surtr85/agent-memory/internal/blocks"
	"github.com/surtr85/agent-memory/internal/db"
	"github.com/surtr85/agent-memory/internal/embedding"
	"github.com/surtr85/agent-memory/internal/temporal"
)

func TestIngestMarkdown(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ecl_test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db open failed: %v", err)
	}
	defer database.Close()
	if err := db.InitSchema(database); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	embClient := embedding.NewHTTPClient("http://127.0.0.1:1/inv", "http://127.0.0.1:1/inv", "bge-m3", 64)

	doc := `# Agent Overview
AgentMemory Universal is an embeddable cognitive architecture.

## Architecture
It integrates with [[Letta]] for core memory and [[Graphiti]] for temporal knowledge.

### Features
Fast sub-millisecond retrieval with [[SQLite]].`

	err = IngestMarkdown(database, embClient, "system", "Architecture Guide", doc)
	if err != nil {
		t.Fatalf("IngestMarkdown failed: %v", err)
	}

	// Verify chunks
	var chunkCount int
	err = database.QueryRow(`SELECT count(*) FROM chunks WHERE namespace = 'system'`).Scan(&chunkCount)
	if err != nil {
		t.Fatalf("query chunks count failed: %v", err)
	}
	if chunkCount < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", chunkCount)
	}

	// Verify vector_embeddings
	var embCount int
	err = database.QueryRow(`SELECT count(*) FROM vector_embeddings`).Scan(&embCount)
	if err != nil {
		t.Fatalf("query vector_embeddings count failed: %v", err)
	}
	if embCount != chunkCount {
		t.Fatalf("expected %d embeddings matching chunks, got %d", chunkCount, embCount)
	}

	// Verify entities extracted from [[Wikilinks]]
	var entCount int
	err = database.QueryRow(`SELECT count(*) FROM entities WHERE name IN ('Letta', 'Graphiti', 'SQLite')`).Scan(&entCount)
	if err != nil {
		t.Fatalf("query entities failed: %v", err)
	}
	if entCount != 3 {
		t.Fatalf("expected 3 extracted entities, got %d", entCount)
	}

	// Verify entity relations
	var relCount int
	err = database.QueryRow(`SELECT count(*) FROM entity_relations WHERE relation_type = 'mentions'`).Scan(&relCount)
	if err != nil {
		t.Fatalf("query entity_relations failed: %v", err)
	}
	if relCount < 3 {
		t.Fatalf("expected at least 3 relations, got %d", relCount)
	}
}

func TestRecordAndConsolidateObservations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "obs_test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db open failed: %v", err)
	}
	defer database.Close()
	if err := db.InitSchema(database); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	blockMgr := blocks.NewManager(database)
	if err := blockMgr.SeedDefaults(); err != nil {
		t.Fatalf("SeedDefaults failed: %v", err)
	}

	// 1. Record observations
	err = RecordObservation(database, "human", "User prefers concise answers", "system")
	if err != nil {
		t.Fatalf("RecordObservation 1 failed: %v", err)
	}
	err = RecordObservation(database, "fact", "User prefers_editor Neovim", "system")
	if err != nil {
		t.Fatalf("RecordObservation 2 failed: %v", err)
	}

	obsList, err := ListObservations(database, "system")
	if err != nil {
		t.Fatalf("ListObservations failed: %v", err)
	}
	if len(obsList) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obsList))
	}
	if obsList[0].Status != "unconsolidated" {
		t.Fatalf("expected status unconsolidated, got %s", obsList[0].Status)
	}

	// 2. Consolidate
	err = ConsolidateObservations(database)
	if err != nil {
		t.Fatalf("ConsolidateObservations failed: %v", err)
	}

	// Verify all observations are consolidated
	obsListAfter, err := ListObservations(database, "system")
	if err != nil {
		t.Fatalf("ListObservations after failed: %v", err)
	}
	for _, o := range obsListAfter {
		if o.Status != "consolidated" {
			t.Fatalf("expected status consolidated, got %s", o.Status)
		}
	}

	// Verify core block was updated
	humanBlock, err := blockMgr.GetBlock("human")
	if err != nil {
		t.Fatalf("GetBlock human failed: %v", err)
	}
	if humanBlock == nil || len(humanBlock.Content) == 0 {
		t.Fatal("expected human block to have content")
	}

	// Verify fact was registered in temporal registry
	reg := temporal.NewRegistry(database)
	facts, err := reg.QueryFacts("system", "User", "prefers_editor")
	if err != nil {
		t.Fatalf("QueryFacts failed: %v", err)
	}
	if len(facts) != 1 || facts[0].Object != "Neovim" {
		t.Fatalf("expected fact User prefers_editor Neovim, got: %+v", facts)
	}
}
