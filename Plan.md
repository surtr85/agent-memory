# 🧠 AgentMemory Universal (v3.5 Cognitive Engine) — Master Architecture & Upgrade Plan

> **Codename**: `OmniMem` / `AgentMemory Universal`
> **Repository Target**: `https://github.com/surtr85/agent-memory`
> **Local Workspace**: `/home/amadeus/Projects/workspace/agent-memory`
> **Language / Runtime**: **Pure Go (1.24+ / Zero CGO)**
> **Embedding Model Target**: `embeddinggemma-2-Q8_0.gguf` (`/home/amadeus/Projects/models/embeddinggemma-2-Q8_0.gguf`)
> **Architect**: Sajjad & Amadeus AI Systems Architecture
> **License**: MIT

---

## 1. Executive Summary & Upgrade Objectives

AgentMemory Universal is transitioning from v3.0 to **v3.5 Production Grade**.
This upgrade directly addresses the architectural bottlenecks identified during code audit:

1. **True Semantic Embeddings (EmbeddingGemma-2 Q8)**:
   - Eliminate reliance solely on 256-d subword n-gram hashing by wiring native OpenAI `/v1/embeddings` and llama-server integration.
   - Maintain seamless pure Go fallback to `BuiltinVectorizer` when offline.
2. **Deterministic Token Diet for Working Memory (Letta Blocks)**:
   - Enforce hard budget (<500 tokens) on `memory_get_bootstrap` by categorizing blocks into *Core Bootstrap* (`persona`, `human`, `environment`, `session_state`, `alignment`) vs *Domain Knowledge* (`adult`, `konkur_1405`, `qrose_shop_protocols`, etc.).
3. **Multilingual & Persian Fact Extraction**:
   - Modernize the observation extraction pipeline with Unicode regex (`\p{L}`) and structured fact extraction.
4. **Embedded Visual Web Dashboard & Knowledge Graph**:
   - Introduce `agent-memory ui` command serving a zero-dependency, high-performance dark-mode web application directly from Go binary via `//go:embed`.
   - Feature an interactive Force-Directed Knowledge Graph canvas, Letta block live editor, bi-temporal fact timeline, and real-time hybrid search tester.

---

## 2. Implementation Roadmap & Todo List

- [ ] **Task 1: Embedding Client & MCP Server Wiring**
  - [ ] Add OpenAI `/v1/embeddings` format support to `internal/embedding/client.go`.
  - [ ] Support llama.cpp / llama-server embedding API responses.
  - [ ] Wire `embedding.NewHTTPClient` into `internal/server/server.go` (replacing hardcoded `BuiltinVectorizer`).
  - [ ] Fix default `internal/config/config_test.go` test assertion.

- [ ] **Task 2: Bootstrap Token Budget & Block Scoping**
  - [ ] Add `is_bootstrap` / priority flag to `core_blocks` table and manager.
  - [ ] Filter default bootstrap prompt to essential system blocks only (<500 tokens total).
  - [ ] Add `memory_get_bootstrap` parameter to allow including optional domain blocks on demand.

- [ ] **Task 3: Pipeline & Unicode Normalization**
  - [ ] Update `factPattern` in `internal/pipeline/observation.go` with Unicode support (`[\p{L}\p{N}\.\-\_]+`).
  - [ ] Add validation and structured fact ingestion rules.

- [ ] **Task 4: Zero-Dependency Web Dashboard & Visual Graph (`agent-memory ui`)**
  - [ ] Create `internal/web` package with embedded assets (`web/dist` or single-file vanilla SPA).
  - [ ] Implement REST endpoints:
    - `GET /api/health` - Server and DB status.
    - `GET /api/stats` - Entity and namespace statistics.
    - `GET /api/blocks` & `POST /api/blocks` - View and edit Letta core blocks.
    - `GET /api/facts` & `POST /api/facts` - Bi-temporal fact explorer.
    - `GET /api/graph` - Full entity graph nodes and links for D3 / HTML5 canvas.
    - `POST /api/search` - Live 4-way hybrid search sandbox.
  - [ ] Build responsive, dark-mode, high-density Web UI:
    - Interactive Force-Directed Knowledge Graph with zoom, drag, and node details panel.
    - Letta Core Block editor with token count.
    - Bi-temporal timeline visualization (Active vs Superseded facts).
  - [ ] Register CLI subcommand `agent-memory ui [--port 3200]`.

- [ ] **Task 5: Verification, Benchmarking & Commit**
  - [ ] Execute `CGO_ENABLED=0 go test ./...` with 100% pass rate.
  - [ ] Build static binary to `bin/agent-memory`.
  - [ ] Run live verification of MCP server and Web UI.
  - [ ] Git commit and push upstream.

---

## 3. Architecture Specification

### 3.1 Embedding Client Protocol
Endpoints queried in order:
1. `cfg.EmbeddingURL` (Supports both OpenAI `/v1/embeddings` and BGE `/embedding` payloads).
2. `cfg.OllamaURL` (`/api/embeddings`).
3. Pure Go `BuiltinVectorizer` (256-d subword n-gram hashing fallback).

### 3.2 Web Dashboard Architecture
- Package: `internal/web`
- Server: Standard Go `net/http` with zero external web framework dependencies.
- Frontend: Single-file, modern vanilla JS/CSS/Canvas app embedded using `embed.FS`.
- Zero Node.js build runtime required at execution time.
