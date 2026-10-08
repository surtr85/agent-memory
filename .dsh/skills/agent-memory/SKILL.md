---
name: agent-memory
description: Access, search, and manage your local persistent second-brain cognitive memory engine (surtr85/agent-memory) via native FastMCP tools and Codemode async JavaScript (23 tools), featuring 4-way hybrid retrieval, Letta core blocks, bi-temporal atomic facts, and Laya System-1 Vulkan appraisal.
---

# AgentMemory Universal Skill (`agent-memory`)

**AgentMemory Universal (v3.2.1+)** is a zero-latency, local-first cognitive memory engine written in **Pure Go (`CGO_ENABLED=0`)** and backed by SQLite WAL mode (`~/.local/share/agent-memory/memory.db`).

## ⚠️ Invocation Protocol (MCP & Codemode Native)

**Do NOT spawn the CLI binary imperatively via bash.**
All 23 tools are registered directly as native MCP tools under `agent-memory`.
Always invoke them cleanly in `codemode` via:

```javascript
(await tools.mcp__agent_memory__) < tool_name > { ...args };
```

---

## 🏛️ Core Cognitive Architecture & Registered MCP Tools (23 Total)

### 1. In-Context Core Memory Blocks (Letta / MemGPT Paradigm)

In-context working memory blocks injected directly into agent context (< 400 tokens) with zero retrieval latency:

- `mcp__agent_memory__memory_get_bootstrap({})`: Loads all active blocks (`persona`, `human`, `environment`, `session_state`, `laya`, `konkur_1405`, etc.) + standing alignment synthesis in one shot.
- `mcp__agent_memory__memory_get_block({ label: "session_state" })`: Reads a single core block.
- `mcp__agent_memory__memory_set_block({ label: "session_state", content: "..." })`: Replaces or creates a core block.
- `mcp__agent_memory__memory_append_block({ label: "...", content: "..." })`: Appends text to an existing block.
- `mcp__agent_memory__memory_replace_block({ label: "...", old_text: "...", new_text: "..." })`: Targeted patch inside a block.
- `mcp__agent_memory__memory_list_blocks({})`: Lists all registered block labels.

### 2. Bi-Temporal Atomic Fact Registry & Citations

Knowledge triples `(Subject) --[Predicate]--> (Object)` with validity horizon and audit trail:

- `mcp__agent_memory__memory_add_fact({ namespace: "system", subject: "...", predicate: "...", object: "...", source_uri: "...", source_quote: "..." })`: Adds an immutable or superseding fact. Namespaces: `system`, `ai`, `tools`, `forex`, `literature`, `ecommerce`.
- `mcp__agent_memory__memory_get_active_facts({ namespace: "..." })`: Retrieves currently valid facts in a domain.
- `mcp__agent_memory__memory_get_facts_at({ timestamp: "...", namespace: "..." })`: Historical time-travel query.
- `mcp__agent_memory__memory_explain({ id: "..." })`: Explains a fact with source quote and supersession history.

### 3. 4-Way Hybrid Retrieval Fusion

Combines 256-d subword vectors (9.2 µs), multilingual BM25 (with Persian/Arabic normalizer & ZWNJ support), entity graph traversal, and temporal recency:

- `mcp__agent_memory__memory_search({ query: "...", namespace: "system", top_k: 5 })`: Searches memory across namespaces.
- `mcp__agent_memory__memory_ingest_markdown({ path: "...", namespace: "...", bank: "..." })`: Ingests markdown dossiers into memory banks.

### 4. Streaming Observations & Batch Consolidation

Buffers transient signals and tool edge-cases during conversation turns without polluting atomic facts:

- `mcp__agent_memory__memory_record_observation({ namespace: "...", category: "preference", content: "..." })`: Records transient signal.
- `mcp__agent_memory__memory_consolidate_observations({})`: Distills buffered observations into permanent facts and core blocks.

### 5. Nightly Dream Cycle & Memory Consolidation

- `mcp__agent_memory__memory_dream_cycle({})`: Runs the autonomous dream consolidation cycle, archives daily prose reflections, and synthesizes standing alignment guidance.

### 6. Alignment & Behavioral Repair Threads

- `mcp__agent_memory__memory_record_repair({ trigger_summary: "...", agent_adjustment: "..." })`: Records friction and behavioral correction.
- `mcp__agent_memory__memory_get_alignment({})`: Fetches active standing guidance (< 60 tokens).

### 7. 4-Stage Safe Forgetting Pipeline

- `mcp__agent_memory__memory_stage_forget({ pattern: "...", namespace: "..." })`: Stages facts for retraction with safety gate checks.
- `mcp__agent_memory__memory_execute_forget({ stage_id: "..." })`: Executes permanent cascade retraction with tombstones.

### 8. Laya System-1 Cognitive Engine & Telemetry

- `mcp__agent_memory__memory_reflect({ context: "..." })`: Sub-40ms cognitive status appraisal (`optimal`, `nominal`).
- `mcp__agent_memory__memory_decision({ state: "...", preset: "..." })`: Evaluates state through Laya System-1 engine.
- `mcp__agent_memory__memory_health_check({})`: Verifies SQLite WAL and index integrity.
- `mcp__agent_memory__memory_stats({})`: Shows namespace and entity breakdown.

---

## ⚡ Standard Agent Lifecycle Workflow

1. **Boot**: Run `memory_get_bootstrap` inside `codemode` to load working state.
2. **Observe**: When discovering key configurations or user preferences, stream with `memory_record_observation`.
3. **Assert**: Permanently assert critical architectural truths via `memory_add_fact`.
4. **Reconcile**: On task completion or user request, update `session_state` via `memory_set_block` and run `memory_dream_cycle`.
