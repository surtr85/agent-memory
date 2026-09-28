# 🧠 AgentMemory Universal (v3.0 Cognitive Engine) — Master Architecture Plan (Pure Go Edition)

> **Codename**: `OmniMem` / `AgentMemory Universal`  
> **Repository Target**: `https://github.com/surtr85/agent-memory`  
> **Local Workspace**: `/home/amadeus/Projects/workspace/agent-memory`  
> **Language / Runtime**: **Pure Go (1.24+ / Zero CGO)**  
> **Author & Architect**: Sajjad & Amadeus AI Systems Architecture  
> **License**: MIT  

---

## 1. Executive Summary & Design Philosophy

**AgentMemory Universal (v3.0)** is an independent, production-grade, local-first cognitive memory engine written in **Pure Go** (`CGO_ENABLED=0`). It compiles into a single, lightning-fast static binary (<15 MB) with sub-millisecond query latency and zero external runtime dependencies.

By rejecting bloated, fragile Python dependency trees and complex external graph servers, it synthesizes the world's most effective agent memory paradigms into an ultra-reliable, embeddable system:
1. **Letta (MemGPT)**: In-context, self-editing **Working Core Memory Blocks** (`human`, `persona`, `environment`, `scratchpad`) that bootstrap instantly into agent system prompts (<400 tokens) with 0 latency.
2. **Mem0**: **Atomic Fact Extraction & Entity Linking**, decoupling memory from sprawling document blobs into discrete, verifiable assertions scoped by user, agent, and domain namespaces.
3. **Graphiti (Zep)**: **Bi-Temporal Knowledge Modeling** (`valid_from`, `valid_until`, `recorded_at`, `invalidated_at`). Contradictions close and invalidate older facts gracefully without destructive overwriting, enabling time-travel queries ("what was true last month vs now?").
4. **Cognee**: **ECL (Extract, Cognify, Load) Pipeline**, structured typed schemas, and ontology grounding.
5. **Hindsight**: **4-Way Hybrid Retrieval Fusion** (Dense Vector + BM25 with multilingual Persian normalizer + Entity Graph Traversal + Temporal Slicing) with Reciprocal Rank Fusion (RRF) in a single embedded SQLite (`modernc.org/sqlite`) database.

---

## 2. Architectural Blueprint & Layer Decomposition

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                   AGENT MEMORY UNIVERSAL (v3.0) PURE-GO ARCHITECTURE                   │
└────────────────────────────────────────────────────────────────────────────────────────┘

  ┌────────────────────────────────────────────────────────────────────────────────────┐
  │ 1. IN-CONTEXT CORE MEMORY (Working Context Injection — < 400 Tokens)                │
  │    • persona: Tone, communication style, peer demeanor (no corporate slop)         │
  │    • human: User identity, preferences, hard constraints                           │
  │    • environment: Active machine, OS, window manager, current working targets      │
  │    • Tools: core_memory_replace, core_memory_append, core_memory_set               │
  └────────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ┌────────────────────────────────────────────────────────────────────────────────────┐
  │ 2. SCOPED DOMAIN NAMESPACES                                                        │
  │    ┌──────────────┐   ┌──────────────┐   ┌──────────────┐   ┌─────────────────┐    │
  │    │    System    │   │  Trading/FX  │   │  E-Commerce  │   │   Literature    │    │
  │    │   & Coding   │   │  & Quants    │   │ (Rose Shop)  │   │   Translation   │    │
  │    └──────────────┘   └──────────────┘   └──────────────┘   └─────────────────┘    │
  └────────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ┌────────────────────────────────────────────────────────────────────────────────────┐
  │ 3. BI-TEMPORAL ATOMIC FACT REGISTRY (Pure Go SQLite ACID Storage)                  │
  │    • Triples: (Subject) --[Predicate]--> (Object)                                  │
  │    • Valid Time: valid_from, valid_until (real-world truth horizon)                │
  │    • System Time: recorded_at, invalidated_at (transaction audit trail)             │
  │    • Contradiction Auto-Resolver (Supersede rather than destroy)                   │
  └────────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ┌────────────────────────────────────────────────────────────────────────────────────┐
  │ 4. 4-WAY HYBRID RETRIEVAL ENGINE (Hindsight SOTA Architecture)                     │
  │    ┌─────────────────┬──────────────────┬─────────────────┬───────────────────┐    │
  │    │  Dense Vectors  │   Lexical BM25   │ Entity/Relation │ Temporal Slicing  │    │
  │    │ (BGE-M3/Ollama) │ (Persian Normal) │ (Multi-hop BFS) │ (Active vs Hist.) │    │
  │    └─────────────────┴──────────────────┴─────────────────┴───────────────────┘    │
  │                                     │                                              │
  │                                     ▼                                              │
  │               Reciprocal Rank Fusion (RRF) & Confidence Scorer                     │
  └────────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
  ┌────────────────────────────────────────────────────────────────────────────────────┐
  │ 5. UNIVERSAL INTERFACES & RUNTIMES                                                 │
  │    • FastMCP Go Server: stdio & SSE for Pi, Claude Code, Cursor, Antigravity       │
  │    • High-Performance CLI: `agent-memory add`, `query`, `core`, `inspect`, `serve` │
  │    • Single Static Binary (<15MB, CGO_ENABLED=0)                                   │
  │    • Standalone Packaging: Docker / Docker Compose / Nix Flake (buildGoModule)     │
  └────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Storage Engine: Pure-Go SQLite (`modernc.org/sqlite`)

Zero CGO dependencies. Embedded directly inside the binary.

### A. Core Memory Blocks (`core_blocks`)
```sql
CREATE TABLE IF NOT EXISTS core_blocks (
    id TEXT PRIMARY KEY,
    label TEXT UNIQUE NOT NULL,      -- 'human', 'persona', 'environment', 'scratchpad'
    content TEXT NOT NULL,
    max_tokens INTEGER DEFAULT 500,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### B. Bi-Temporal Fact Registry (`facts`)
```sql
CREATE TABLE IF NOT EXISTS facts (
    id TEXT PRIMARY KEY,
    namespace TEXT NOT NULL,          -- 'system', 'forex', 'ecommerce', 'literature', 'general'
    subject TEXT NOT NULL,            -- e.g. 'User' or 'DesktopEnvironment'
    predicate TEXT NOT NULL,          -- e.g. 'uses_compositor' or 'trading_killzone'
    object TEXT NOT NULL,             -- e.g. 'Niri' or '10:30-14:30 Iran Time'
    confidence REAL DEFAULT 1.0,
    source TEXT,                      -- session reference or user assertion
    valid_from TIMESTAMP NOT NULL,
    valid_until TIMESTAMP,            -- NULL means currently active / indefinite
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    invalidated_at TIMESTAMP,         -- Timestamp when marked invalid by contradiction
    superseded_by TEXT,               -- ID of fact that replaced this
    FOREIGN KEY(superseded_by) REFERENCES facts(id)
);
CREATE INDEX IF NOT EXISTS idx_facts_lookup ON facts(namespace, subject, predicate, valid_until);
```

### C. Vectors & Chunks (`chunks` & `vector_embeddings`)
```sql
CREATE TABLE IF NOT EXISTS chunks (
    id TEXT PRIMARY KEY,
    namespace TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS vector_embeddings (
    chunk_id TEXT PRIMARY KEY,
    dimensions INTEGER NOT NULL,
    embedding BLOB NOT NULL,          -- Little-endian float32 array
    FOREIGN KEY(chunk_id) REFERENCES chunks(id) ON DELETE CASCADE
);
```

### D. Entities & Relational Graph (`entities` & `entity_relations`)
```sql
CREATE TABLE IF NOT EXISTS entities (
    name TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,        -- 'tool', 'person', 'config', 'concept'
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
```

---

## 4. Go Codebase Modular Package Layout

```
agent-memory/
├── cmd/
│   └── agent-memory/
│       └── main.go                  # Single CLI & MCP entrypoint
├── internal/
│   ├── blocks/                      # In-Context Core Memory Manager (Letta style)
│   │   ├── blocks.go
│   │   └── blocks_test.go
│   ├── config/                      # Configuration & flags
│   │   └── config.go
│   ├── db/                          # Pure-Go SQLite Driver & Migrations
│   │   ├── db.go
│   │   └── schema.sql
│   ├── embedding/                   # Local BGE-M3 / Ollama / OpenAI embeddings
│   │   ├── client.go
│   │   └── math.go                  # Fast Cosine Similarity in Go
│   ├── normalizer/                  # Multilingual & Persian/Arabic normalizer
│   │   ├── normalizer.go
│   │   └── normalizer_test.go
│   ├── pipeline/                    # ECL Pipeline & Observation Consolidator
│   │   ├── ecl.go
│   │   └── observation.go
│   ├── retrieval/                   # 4-Way Hybrid Search Engine & RRF
│   │   ├── bm25.go
│   │   ├── hybrid.go
│   │   └── rrf.go
│   ├── server/                      # FastMCP Server (mark3labs/mcp-go)
│   │   ├── server.go
│   │   └── tools.go
│   └── temporal/                    # Bi-Temporal Fact Registry (Graphiti/Zep)
│       ├── registry.go
│       └── registry_test.go
├── Dockerfile                       # Multi-stage scratch Docker build
├── docker-compose.yml
├── flake.nix                        # Hermetic Nix packaging via buildGoModule
├── go.mod
├── go.sum
├── Makefile
├── Plan.md
└── README.md
```

---

## 5. Implementation Roadmap (Subagent-Driven)

- **Phase 1**: Project bootstrap, `go.mod`, `flake.nix`, Makefile, `internal/config`, and `internal/db` with embedded SQLite.
- **Phase 2**: Implement `internal/blocks` (Letta Core memory) and `internal/temporal` (Bi-temporal fact registry with contradiction resolution).
- **Phase 3**: Implement `internal/normalizer`, `internal/embedding`, and `internal/retrieval` (BM25 + Dense Vector + Graph Traversal + RRF).
- **Phase 4**: Implement `internal/pipeline` (ECL and streaming observations) and `internal/server` (FastMCP 2.0 with 15 tools).
- **Phase 5**: Build `cmd/agent-memory/main.go` CLI, write comprehensive tests, and verify 100% test pass.
- **Phase 6**: Publish to GitHub via `gh` and wire into NixOS flake.
