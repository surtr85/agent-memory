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
    source_uri TEXT,
    source_quote TEXT,
    line_number INTEGER DEFAULT 0,
    salience REAL DEFAULT 0.5,
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
    source_uri TEXT,
    line_number INTEGER DEFAULT 0,
    bank TEXT DEFAULT 'general',
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
    source_uri TEXT,
    line_number INTEGER DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'unconsolidated',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Alignment State and Standing Guidance
CREATE TABLE IF NOT EXISTS alignment_state (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL, -- 'boundary', 'preference', 'friction', 'principle'
    guidance TEXT NOT NULL,
    salience REAL DEFAULT 1.0,
    status TEXT NOT NULL DEFAULT 'active', -- 'active', 'repaired', 'archived'
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Repair Threads for tracking recurring friction and resolution
CREATE TABLE IF NOT EXISTS repair_threads (
    id TEXT PRIMARY KEY,
    trigger_summary TEXT NOT NULL,
    agent_adjustment TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open', -- 'open', 'in_progress', 'resolved'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP
);

-- Nightly Dreams / Reflective Synthesis archive
CREATE TABLE IF NOT EXISTS dreams (
    id TEXT PRIMARY KEY,
    dream_date TEXT NOT NULL, -- YYYY-MM-DD
    prose_content TEXT NOT NULL,
    synthesis_markdown TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Safe Forgetting Tombs: prevents re-ingestion of retracted claims/facts
CREATE TABLE IF NOT EXISTS tombstones (
    hash_signature TEXT PRIMARY KEY,
    original_id TEXT NOT NULL,
    item_type TEXT NOT NULL, -- 'fact', 'chunk', 'entity'
    reason TEXT,
    retracted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Staging area for Safe Forgetting review
CREATE TABLE IF NOT EXISTS forget_staging (
    id TEXT PRIMARY KEY,
    target_pattern TEXT NOT NULL,
    item_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'approved', 'executed', 'rejected'
    staged_items_json TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indices for performance and bi-temporal / hybrid retrieval
CREATE INDEX IF NOT EXISTS idx_facts_lookup ON facts(namespace, subject, predicate, valid_until);
CREATE INDEX IF NOT EXISTS idx_facts_valid_time ON facts(valid_from, valid_until);
CREATE INDEX IF NOT EXISTS idx_facts_namespace ON facts(namespace);
CREATE INDEX IF NOT EXISTS idx_facts_source_uri ON facts(source_uri);
CREATE INDEX IF NOT EXISTS idx_chunks_namespace ON chunks(namespace);
CREATE INDEX IF NOT EXISTS idx_chunks_bank ON chunks(bank);
CREATE INDEX IF NOT EXISTS idx_chunks_source_uri ON chunks(source_uri);
CREATE INDEX IF NOT EXISTS idx_entities_namespace ON entities(namespace);
CREATE INDEX IF NOT EXISTS idx_entity_relations_src ON entity_relations(source_entity);
CREATE INDEX IF NOT EXISTS idx_entity_relations_tgt ON entity_relations(target_entity);
CREATE INDEX IF NOT EXISTS idx_observations_ns_status ON observations(namespace, status);
CREATE INDEX IF NOT EXISTS idx_alignment_status ON alignment_state(status);
CREATE INDEX IF NOT EXISTS idx_repair_status ON repair_threads(status);
CREATE INDEX IF NOT EXISTS idx_tombstones_sig ON tombstones(hash_signature);
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

// InitSchema executes all table and index creation statements and migrates columns if upgrading.
func InitSchema(db *sql.DB) error {
	// First run schema migrations for existing tables
	migrations := []string{
		"ALTER TABLE facts ADD COLUMN source_uri TEXT;",
		"ALTER TABLE facts ADD COLUMN source_quote TEXT;",
		"ALTER TABLE facts ADD COLUMN line_number INTEGER DEFAULT 0;",
		"ALTER TABLE facts ADD COLUMN salience REAL DEFAULT 0.5;",
		"ALTER TABLE chunks ADD COLUMN source_uri TEXT;",
		"ALTER TABLE chunks ADD COLUMN line_number INTEGER DEFAULT 0;",
		"ALTER TABLE chunks ADD COLUMN bank TEXT DEFAULT 'general';",
		"ALTER TABLE observations ADD COLUMN source_uri TEXT;",
		"ALTER TABLE observations ADD COLUMN line_number INTEGER DEFAULT 0;",
	}
	for _, m := range migrations {
		_, _ = db.Exec(m) // Ignore errors if columns already exist
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	return nil
}
