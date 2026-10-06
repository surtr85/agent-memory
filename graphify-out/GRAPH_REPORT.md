# Graph Report - agent-memory  (2026-10-06)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 252 nodes · 664 edges · 15 communities (14 shown, 1 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 54 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `dbb03c5c`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Community 0
- Community 1
- Community 2
- Community 3
- Community 4
- Community 5
- Community 6
- Community 7
- Community 8
- Community 9
- Community 10
- Community 11
- Community 12
- Community 14

## God Nodes (most connected - your core abstractions)
1. `Server` - 24 edges
2. `Engine` - 17 edges
3. `Registry` - 17 edges
4. `NewRegistry()` - 16 edges
5. `Open()` - 15 edges
6. `InitSchema()` - 14 edges
7. `main()` - 13 edges
8. `Server` - 13 edges
9. `NewServer()` - 13 edges
10. `NewEngine()` - 13 edges

## Surprising Connections (you probably didn't know these)
- `initDB()` --references--> `Config`  [EXTRACTED]
  cmd/agent-memory/main.go → internal/config/config.go
- `main()` --calls--> `NewManager()`  [EXTRACTED]
  cmd/agent-memory/main.go → internal/alignment/alignment.go
- `main()` --calls--> `LoadConfig()`  [EXTRACTED]
  cmd/agent-memory/main.go → internal/config/config.go
- `main()` --calls--> `NewBuiltinVectorizer()`  [EXTRACTED]
  cmd/agent-memory/main.go → internal/embedding/client.go
- `main()` --calls--> `NewPipeline()`  [EXTRACTED]
  cmd/agent-memory/main.go → internal/forget/forget.go

## Import Cycles
- None detected.

## Communities (15 total, 1 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.16
Nodes (15): Client, DeserializeEmbedding(), SerializeEmbedding(), TestSerializeDeserializeEmbedding(), extractWikilinks(), IngestMarkdown(), IngestMarkdownWithBank(), splitMarkdownSections() (+7 more)

### Community 1 - "Community 1"
Cohesion: 0.09
Nodes (46): initDB(), main(), printHelp(), database/sql.DB, testing.T, setupTestDB(), TestAlignmentAndRepairs(), NewManager() (+38 more)

### Community 2 - "Community 2"
Cohesion: 0.14
Nodes (16): BuiltinVectorizer, HTTPClient, net/http.Client, GenerateDeterministicPseudoVector(), NewBuiltinVectorizer(), NewBuiltinVectorizerWithDim(), NewHTTPClient(), TestBuiltinVectorizer_Basic() (+8 more)

### Community 3 - "Community 3"
Cohesion: 0.25
Nodes (4): context.Context, github.com/mark3labs/mcp-go/mcp.CallToolRequest, github.com/mark3labs/mcp-go/mcp.CallToolResult, Server

### Community 4 - "Community 4"
Cohesion: 0.15
Nodes (17): main(), runBiTemporalBenchmark(), runBM25Benchmark(), runBootstrapBenchmark(), runBuiltinVectorizerBenchmark(), runHybridSearchBenchmark(), runLayaBenchmark(), runNormalizerBenchmark() (+9 more)

### Community 5 - "Community 5"
Cohesion: 0.36
Nodes (3): CoreBlock, Manager, parseTime()

### Community 6 - "Community 6"
Cohesion: 0.21
Nodes (7): appraisalMemoryState(), classifyConflict(), classifyNamespace(), classifyTriage(), containsWord(), Engine, matchesAny()

### Community 7 - "Community 7"
Cohesion: 0.28
Nodes (6): database/sql.Tx, time.Time, Registry, parseFactTime(), Fact, FactEvidence

### Community 8 - "Community 8"
Cohesion: 0.32
Nodes (5): GuidanceItem, RepairThread, Manager, NewManager(), parseTime()

### Community 9 - "Community 9"
Cohesion: 0.36
Nodes (8): AutoDiscoverLayaBin(), AutoDiscoverLayaModel(), ExpandPath(), FindFirstExisting(), LoadConfig(), TestExpandPath(), TestLoadConfig_Defaults(), TestLoadConfig_EnvOverride()

### Community 10 - "Community 10"
Cohesion: 0.20
Nodes (10): RetractionReceipt, StagedItem, StageResult, github.com/mark3labs/mcp-go/server.MCPServer, Config, Pipeline, NewPipeline(), Server (+2 more)

### Community 11 - "Community 11"
Cohesion: 0.57
Nodes (7): getResultText(), makeCallReq(), setupTestServer(), TestServer_BootstrapAndBlocks(), TestServer_FactsAndTemporal(), TestServer_IngestAndSearchAndObservations(), Server

### Community 12 - "Community 12"
Cohesion: 0.50
Nodes (3): agent-memory.nix, pkgs.buildGoModule, pkgs.mkShell

## Knowledge Gaps
- **4 isolated node(s):** `agent-memory.nix`, `pkgs.buildGoModule`, `pkgs.mkShell`, `github.com/surtr85/agent-memory`
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Engine` connect `Community 6` to `Community 1`, `Community 2`, `Community 10`, `Community 7`?**
  _High betweenness centrality (0.102) - this node is a cross-community bridge._
- **Why does `Registry` connect `Community 7` to `Community 1`, `Community 10`, `Community 6`?**
  _High betweenness centrality (0.095) - this node is a cross-community bridge._
- **Why does `Server` connect `Community 10` to `Community 0`, `Community 1`, `Community 5`, `Community 6`, `Community 7`, `Community 8`?**
  _High betweenness centrality (0.066) - this node is a cross-community bridge._
- **What connects `agent-memory.nix`, `pkgs.buildGoModule`, `pkgs.mkShell` to the rest of the system?**
  _4 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.09437386569872959 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.14 - nodes in this community are weakly interconnected._