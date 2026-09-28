package db

import (
	"path/filepath"
	"testing"
)

func TestOpenAndInitSchema_InMemory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	tables := []string{
		"core_blocks",
		"facts",
		"chunks",
		"vector_embeddings",
		"entities",
		"entity_relations",
		"observations",
	}

	for _, tbl := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %s was not created: %v", tbl, err)
		}
	}
}

func TestOpen_FileWithDirs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "sub", "dir", "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open file db failed: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema file db failed: %v", err)
	}
}
