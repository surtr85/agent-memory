package retrieval

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/surtr85/agent-memory/internal/db"
	"github.com/surtr85/agent-memory/internal/embedding"
	"github.com/surtr85/agent-memory/internal/temporal"
)

func TestBM25Index(t *testing.T) {
	idx := NewBM25Index()
	docs := []Document{
		{ID: "doc1", Content: "Golang concurrency with channels and goroutines"},
		{ID: "doc2", Content: "Python async and await concurrency model"},
		{ID: "doc3", Content: "Niri scrollable tiling window manager on Wayland"},
	}

	idx.Index(docs)
	results := idx.Search("golang goroutines")
	if len(results) == 0 {
		t.Fatal("expected search results, got none")
	}

	if results[0].ID != "doc1" {
		t.Fatalf("expected top doc to be doc1, got %s", results[0].ID)
	}
}

func TestReciprocalRankFusion(t *testing.T) {
	list1 := []string{"doc1", "doc2", "doc3"}
	list2 := []string{"doc2", "doc1", "doc4"}

	fused := ReciprocalRankFusion([][]string{list1, list2}, 60)
	if len(fused) == 0 {
		t.Fatal("expected fused results, got empty")
	}

	// doc1 and doc2 both appear at ranks 0 and 1, so both should have top scores
	topIDs := map[string]bool{fused[0].ID: true, fused[1].ID: true}
	if !topIDs["doc1"] || !topIDs["doc2"] {
		t.Fatalf("expected doc1 and doc2 as top results, got %v", fused)
	}
}

func TestGraphTraversal(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "graph_test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db open failed: %v", err)
	}
	defer database.Close()
	if err := db.InitSchema(database); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// Insert entities
	_, err = database.Exec(`
		INSERT INTO entities (name, entity_type, namespace) VALUES 
		('Niri', 'software', 'system'),
		('Wayland', 'protocol', 'system'),
		('Sway', 'software', 'system');
	`)
	if err != nil {
		t.Fatalf("failed inserting entities: %v", err)
	}

	// Insert relations: Niri -> Wayland, Sway -> Wayland
	_, err = database.Exec(`
		INSERT INTO entity_relations (id, source_entity, target_entity, relation_type) VALUES 
		('r1', 'Niri', 'Wayland', 'uses_protocol'),
		('r2', 'Sway', 'Wayland', 'uses_protocol');
	`)
	if err != nil {
		t.Fatalf("failed inserting relations: %v", err)
	}

	// Traverse 2 hops from Niri -> should reach Wayland and Sway
	discovered, err := TraverseGraph(database, []string{"Niri"}, 2)
	if err != nil {
		t.Fatalf("TraverseGraph failed: %v", err)
	}

	discoveredMap := make(map[string]bool)
	for _, ent := range discovered {
		discoveredMap[ent] = true
	}

	if !discoveredMap["Niri"] || !discoveredMap["Wayland"] || !discoveredMap["Sway"] {
		t.Fatalf("expected to discover Niri, Wayland, and Sway, got: %v", discovered)
	}
}

func TestHybridSearch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "hybrid_test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db open failed: %v", err)
	}
	defer database.Close()
	if err := db.InitSchema(database); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	embClient := embedding.NewHTTPClient("http://127.0.0.1:1/inv", "http://127.0.0.1:1/inv", "bge-m3", 64)
	reg := temporal.NewRegistry(database)

	// Add an atomic fact
	_, err = reg.AddFact("system", "User", "uses_compositor", "Niri", "test")
	if err != nil {
		t.Fatalf("AddFact failed: %v", err)
	}

	// Add a chunk
	chunkID := "chunk-1"
	chunkContent := "Niri is a scrollable-tiling Wayland compositor"
	cVec, _ := embClient.GetEmbedding(context.Background(), chunkContent)
	cBlob := embedding.SerializeEmbedding(cVec)

	_, err = database.Exec(`
		INSERT INTO chunks (id, namespace, title, content) VALUES (?, 'system', 'Niri Guide', ?)
	`, chunkID, chunkContent)
	if err != nil {
		t.Fatalf("chunk insert failed: %v", err)
	}

	_, err = database.Exec(`
		INSERT INTO vector_embeddings (chunk_id, dimensions, embedding) VALUES (?, 64, ?)
	`, chunkID, cBlob)
	if err != nil {
		t.Fatalf("embedding insert failed: %v", err)
	}

	searcher := NewSearcher(database, embClient)
	results, err := searcher.Search(context.Background(), "Niri compositor", "system", 5, false)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected search results, got empty")
	}

	foundChunk := false
	foundFact := false
	for _, res := range results {
		if res.Type == "chunk" && res.ID == chunkID {
			foundChunk = true
		}
		if res.Type == "fact" {
			foundFact = true
		}
	}

	if !foundChunk && !foundFact {
		t.Fatalf("expected to find either chunk or fact, results were: %+v", results)
	}
}
