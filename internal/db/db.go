package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schemaSQL = `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS core_blocks (
    id TEXT PRIMARY KEY,
    label TEXT UNIQUE NOT NULL,
    content TEXT NOT NULL,
    max_tokens INTEGER DEFAULT 500,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS facts (
    id TEXT PRIMARY KEY,
    namespace TEXT NOT NULL,
    subject TEXT NOT NULL,
    predicate TEXT NOT NULL,
    object TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    source TEXT,
    valid_from TIMESTAMP NOT NULL,
    valid_until TIMESTAMP,
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    invalidated_at TIMESTAMP,
    superseded_by TEXT,
    FOREIGN KEY(superseded_by) REFERENCES facts(id)
);

CREATE TABLE IF NOT EXISTS chunks (
    id TEXT PRIMARY KEY,
    namespace TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    metadata TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS vector_embeddings (
    chunk_id TEXT PRIMARY KEY,
    dimensions INTEGER NOT NULL,
    embedding BLOB NOT NULL,
    FOREIGN KEY(chunk_id) REFERENCES chunks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS entities (
    name TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    description TEXT,
    namespace TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS entity_relations (
    id TEXT PRIMARY KEY,
    source_entity TEXT NOT NULL,
    target_entity TEXT NOT NULL,
    relation_type TEXT NOT NULL,
    weight REAL DEFAULT 1.0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(source_entity) REFERENCES entities(name),
    FOREIGN KEY(target_entity) REFERENCES entities(name)
);

CREATE TABLE IF NOT EXISTS observations (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL,
    content TEXT NOT NULL,
    namespace TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unconsolidated',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indices for performance and bi-temporal / hybrid retrieval
CREATE INDEX IF NOT EXISTS idx_facts_lookup ON facts(namespace, subject, predicate, valid_until);
CREATE INDEX IF NOT EXISTS idx_facts_valid_time ON facts(valid_from, valid_until);
CREATE INDEX IF NOT EXISTS idx_facts_namespace ON facts(namespace);
CREATE INDEX IF NOT EXISTS idx_chunks_namespace ON chunks(namespace);
CREATE INDEX IF NOT EXISTS idx_entities_namespace ON entities(namespace);
CREATE INDEX IF NOT EXISTS idx_entity_relations_src ON entity_relations(source_entity);
CREATE INDEX IF NOT EXISTS idx_entity_relations_tgt ON entity_relations(target_entity);
CREATE INDEX IF NOT EXISTS idx_observations_ns_status ON observations(namespace, status);
`

// Open opens a SQLite database at dbPath, creating parent directories if needed,
// sets WAL and pragmas, and returns the *sql.DB handle.
func Open(dbPath string) (*sql.DB, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory %q: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %q: %w", dbPath, err)
	}

	// Apply critical SQLite pragmas
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to execute pragma %q: %w", pragma, err)
		}
	}

	return db, nil
}

// InitSchema executes all table and index creation statements.
func InitSchema(db *sql.DB) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	return nil
}
