# 🧠 AgentMemory Universal (`agent-memory`)

<p align="center">
  <img src="assets/banner.svg" alt="AgentMemory Universal Banner" width="100%">
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/surtr85/agent-memory"><img src="https://pkg.go.dev/badge/github.com/surtr85/agent-memory.svg" alt="Go Reference"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/CGO_ENABLED-0-success.svg" alt="Built with Pure Go"></a>
  <a href="https://github.com/surtr85/agent-memory/releases"><img src="https://img.shields.io/github/v/release/surtr85/agent-memory?color=purple" alt="Release"></a>
</p>

**AgentMemory Universal** is an ultra-fast, production-grade, local-first cognitive memory engine written in **Pure Go (`CGO_ENABLED=0`)**. It compiles into a single, dependency-free static binary (<15 MB) delivering sub-millisecond query latency and **zero external daemon requirements**.

It eliminates the fragility, heavy RAM overhead, and multi-second delays of legacy memory systems by unifying the world's most acclaimed cognitive paradigms:
* 🧩 **Letta (MemGPT)**: Self-editing, in-context **Working Core Memory Blocks** (`human`, `persona`, `environment`) that bootstrap instantly into agent system prompts (<400 tokens) with zero retrieval latency.
* ⚡ **Mem0 & Muse Claim Verification**: **Atomic Fact Extraction & Source Citations**, recording exact quote evidence, line citations, and URI addressing (`memory://`, `file://`, `chat://`) with full explainability (`memory_explain`).
* ⏳ **Graphiti (Zep)**: **Bi-Temporal Knowledge Modeling** (`valid_from`, `valid_until`, `recorded_at`, `invalidated_at`). Contradictions close and invalidate older facts gracefully without destructive data loss.
* 🏛️ **Muse Dossiers & Banks**: Organized memory banks (`world`, `experience`, `opinions`, `reflections`, `people`, `groups`) with automated **Nightly Dream Consolidation** (`memory_dream_cycle`).
* 🎯 **Standing Guidance & Repair Threads**: Active boundary tracking and friction repair (`alignment_state`, `repair_threads`) synthesized cleanly into bootstrap prompts (<60 tokens).
* 🛡️ **4-Stage Safe Forgetting Pipeline**: Staging, Laya System-1 safety gate check, cascade retraction (facts, chunks, entity relations), and permanent tombstone prevention against zombie re-ingestion.
* 🎯 **Hindsight**: **4-Way Hybrid Retrieval Fusion** (Native Subword Semantic Vectors + BM25 with multilingual Persian/Arabic normalizer + Entity Graph Traversal + Temporal Slicing) with Reciprocal Rank Fusion (RRF).
* ⚡ **Laya (Jev AI / TypeSafe System-1)**: Non-autoregressive System-1 decision engine running sub-15ms typed evaluations on AMD Radeon 780M Vulkan for automatic namespace routing, query classification, contradiction conflict checking, and autonomous cognitive reflection (`memory_reflect`).

---

## 💡 The Zero-Daemon Design Philosophy

Traditional agent memory setups force users to run multiple heavy services: an external vector database (Qdrant/Milvus), a graph database (Neo4j), and a separate embedding daemon (e.g. `llama-server` loading a 600MB+ BGE-M3 GGUF model in background VRAM/RAM).

**AgentMemory Universal eliminates all external embedding daemons:**
1. **Built-in Pure-Go Subword Vectorizer**:
   - Uses multi-hash feature projection with sign hashing across character 3-grams, 4-grams, and word tokens.
   - Generates calibrated 256-dimensional unit vectors in **9.2 microseconds** (~108,000 vectors/sec) with zero RAM overhead.
   - Natively captures subword stems, prefixes, and morphological variations in Persian and English.
2. **Embedded ACID SQLite Engine**:
   - Uses `modernc.org/sqlite` in WAL mode for lightning-fast concurrent reads and writes directly to `~/.local/share/agent-memory/memory.db`.
3. **Laya System-1 for Real Cognitive Judgment**:
   - Instead of wasting heavy models on rote embeddings, AI acceleration is reserved for **Laya System-1**: a non-autoregressive decision model executing typed evaluations (`choice`, `score`, `noul`) on AMD Radeon 780M Vulkan in single-pass forward time.

---

## 🧰 MCP Tool Suite (23 Cognitive Tools)

`agent-memory` exposes a complete **FastMCP 2.0** server over `stdio` and `sse`:

| # | Tool Name | Parameters | Capabilities |
| :---: | :--- | :--- | :--- |
| **1** | `memory_get_bootstrap` | None | Returns compact working core memory + alignment synthesis XML (<400 tokens) for prompt injection. |
| **2** | `memory_get_block` | `label` | Read content of any core block (`human`, `persona`, `environment`). |
| **3** | `memory_set_block` | `label`, `content` | Create or update a core memory block. |
| **4** | `memory_replace_block` | `label`, `old_content`, `new_content` | Precision in-context text swap without hallucinating or losing surrounding state. |
| **5** | `memory_append_block` | `label`, `content` | Append facts or new priorities to an existing block. |
| **6** | `memory_list_blocks` | None | List all registered working memory blocks and token metrics. |
| **7** | `memory_add_fact` | `namespace`, `subject`, `predicate`, `object`, `source`, `source_uri`, `source_quote`, `line_number` | Record atomic triple with **automatic contradiction resolution and citation evidence**. |
| **8** | `memory_explain` | `id` | Explain a claim or fact with exact quotes, line citations, source URI, and supersession history. |
| **9** | `memory_get_active_facts`| `namespace` | Retrieve currently valid, unexpired truth for a domain. |
| **10** | `memory_get_facts_at` | `namespace`, `timestamp` | Time-travel query: inspect exact knowledge state at any ISO8601 point in history. |
| **11** | `memory_search` | `query`, `namespace`, `top_k`, `include_historical` | 4-way hybrid search (Native Vectors + BM25 + Graph BFS + Temporal) via RRF. |
| **12** | `memory_ingest_markdown`| `namespace`, `title`, `content`, `bank`, `source_uri` | ECL pipeline: chunks markdown into memory banks (`world`, `experience`, `opinions`, `reflections`). |
| **13** | `memory_record_observation`| `category`, `content`, `namespace`, `source_uri`, `line_number` | Log real-time observations with tombstone protection. |
| **14** | `memory_consolidate_observations`| `namespace` | Distill pending stream observations into active facts or core blocks. |
| **15** | `memory_dream_cycle` | `date` | Run autonomous night consolidation: distill observations, write prose reflection, update alignment. |
| **16** | `memory_record_repair` | `trigger_summary`, `agent_adjustment` | Record friction/repair thread to permanently calibrate future agent demeanor. |
| **17** | `memory_get_alignment` | None | Get standing guidance, boundaries, and open repair threads synthesis (<60 tokens). |
| **18** | `memory_stage_forget` | `pattern`, `namespace` | Stage 1 & 2 of Safe Forgetting: identifies targets and verifies with Laya safety guard. |
| **19** | `memory_execute_forget`| `stage_id` | Stage 3 & 4 of Safe Forgetting: cascade retraction + permanent tombstone creation. |
| **20** | `memory_reflect` | `context` (optional) | Autonomous System-1 cognitive appraisal of memory state, focus, and alert level. |
| **21** | `memory_decision` | `state`, `preset` | Run sub-15ms Laya / Jev AI System-1 typed decision over arbitrary state. |
| **22** | `memory_stats` | None | Detailed breakdown of facts, chunks, banks, and entities. |
| **23** | `memory_health_check` | None | Verify SQLite WAL status, vectorizer mode, dreams, tombstones, and alignment. |

---

## ⚡ Empirical Performance Benchmarks

Evaluated on AMD Ryzen 7 PRO 7840U with AMD Radeon 780M Graphics and local SQLite database:

| Component / Benchmark | Latency / Op | Throughput | Methodology |
| :--- | :---: | :---: | :--- |
| **Multilingual Normalizer & Tokenizer** | **6.51 µs** (0.0065 ms) | **153,569 ops/sec** | Unicode ZWNJ, Arabic-Persian mapping, Diacritics stripping |
| **Native Pure-Go Semantic Vectorizer** | **12.11 µs** (0.0121 ms) | **82,570 vectors/sec** | Subword 3/4-gram multi-hash L2 unit projection (256-d) |
| **In-Context Working Memory Bootstrapping** | **21.46 µs** (0.0215 ms) | **46,605 prompts/sec** | Letta Core instant XML injection (< 400 tokens) |
| **BM25 Lexical Ranking Engine** | **49.97 µs** (0.0500 ms) | **20,014 queries/sec** | Okapi BM25 scored over all 373 indexed chunks |
| **Bi-Temporal Fact Assertion & Contradiction Check** | **5.78 ms** | **173.1 assertions/sec** | Full SQLite ACID transaction with index update |
| **4-Way Hybrid Search (Full RRF Fusion)** | **10.61 ms** | **94.2 searches/sec** | Vectors + BM25 + Graph BFS + Temporal Slicing |
| **Laya System-1 Cognitive Reflection (`reflect`)** | **8.90 ms** | Real-time | Non-autoregressive cognitive appraisal |

---

## 💻 CLI Commands & Examples

```bash
# 1. Health & Vectorizer Status
$ agent-memory health
Status:          HEALTHY
Database Engine: modernc.org/sqlite (Pure Go, Zero CGO, WAL mode)
Database Path:   ~/.local/share/agent-memory/memory.db
Vectorizer:      Pure Go BuiltinVectorizer (256-d subword n-gram multi-hash, zero external daemon)
Decision Engine: Laya System-1 / Jev AI (Multilingual System-1 Appraisal, Router, Triage)
Core Blocks:     3
Atomic Facts:    2
Chunks:          373
Entities:        94

# 2. Autonomous Cognitive Reflection
$ agent-memory reflect
{
  "alert_level": "nominal",
  "cognitive_status": "optimal",
  "focus": "system environment & configuration",
  "recommendations": [
    "Verify active stop loss and killzone timing before execution",
    "Ensure dotfiles and system flake remain synchronized"
  ],
  "source": "heuristic",
  "timestamp": "2026-09-28T18:17:18Z"
}

# 3. Laya System-1 Fast Routing
$ agent-memory route "How to configure Niri window manager?"
Namespace:  system
Confidence: 0.93

# 4. Instant In-Context Core Bootstrapping (< 400 tokens)
$ agent-memory bootstrap
### Core Memory Context
<environment>
NixOS Linux (x86_64), Niri Wayland scrollable compositor, Alacritty terminal.
</environment>
<human>
User is Sajjad: Systems architect, full-stack engineer, algorithmic trader.
</human>
<persona>
I am Amadeus / OmniMem, an autonomous, precise AI systems architect. Zero fluff.
</persona>

# 5. Bi-Temporal Fact Recording with Automatic Contradiction Resolution
$ agent-memory fact add system DesktopEnvironment uses_compositor Niri "Migrated to Niri"
Fact recorded successfully [ID: 197c...] # Automatically supersedes previous compositor!

# 6. 4-Way Hybrid Search
$ agent-memory search "DesktopEnvironment" --ns system
Search Results (1 matches for "DesktopEnvironment"):
[1] Type: fact | NS: system | Score: 0.0164
    Content: DesktopEnvironment uses_compositor Niri
```

---

## 🚀 Installation & Deployment

### Option 1: Go Install
```bash
go install github.com/surtr85/agent-memory/cmd/agent-memory@latest
```

### Option 2: Build From Source (Zero CGO)
```bash
git clone https://github.com/surtr85/agent-memory.git
cd agent-memory
make build
./bin/agent-memory health
```

### Option 3: Nix Flake
```bash
# Run directly without installation
nix run github.com/surtr85/agent-memory -- health

# Or enter dev shell with all tools
nix develop github.com/surtr85/agent-memory
```

### Option 4: Docker
```bash
docker run -d \
  -v ~/.local/share/agent-memory:/data \
  -e AGENT_MEMORY_DB=/data/memory.db \
  --name agent-memory \
  ghcr.io/surtr85/agent-memory:latest
```

---

## 📄 License

Licensed under the [MIT License](LICENSE).
