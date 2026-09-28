package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/surtr85/agent-memory/internal/blocks"
	"github.com/surtr85/agent-memory/internal/config"
	"github.com/surtr85/agent-memory/internal/db"
	"github.com/surtr85/agent-memory/internal/embedding"
	"github.com/surtr85/agent-memory/internal/pipeline"
	"github.com/surtr85/agent-memory/internal/retrieval"
	"github.com/surtr85/agent-memory/internal/server"
	"github.com/surtr85/agent-memory/internal/temporal"
)

const version = "3.0.0"

func printHelp() {
	fmt.Println(`AgentMemory Universal (v3.0.0) — High-Performance Pure Go Cognitive Memory

Usage:
  agent-memory serve                             Start stdio FastMCP server
  agent-memory bootstrap                         Print compact working memory prompt for prompt injection
  agent-memory search <query> [--ns <ns>] [--top <k>] [--historical]
  agent-memory fact add <ns> <sub> <pred> <obj> [source]
  agent-memory fact list [--ns <ns>]
  agent-memory block get <label>
  agent-memory block set <label> <content>
  agent-memory block list
  agent-memory ingest <file_or_dir> [--ns <ns>]
  agent-memory stats                             Show namespace & entity breakdown
  agent-memory health                            Show database health & record counts
  agent-memory version                           Show version

Environment Variables:
  AGENT_MEMORY_DB               Database file path (default: ~/.local/share/agent-memory/memory.db)
  AGENT_MEMORY_EMBEDDING_URL    Primary BGE-M3 embedding service URL
  AGENT_MEMORY_OLLAMA_URL       Fallback Ollama service URL`)
}

func initDB(cfg *config.Config) *sql.DB {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database at %s: %v\n", cfg.DBPath, err)
		os.Exit(1)
	}
	if err := db.InitSchema(database); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database schema: %v\n", err)
		os.Exit(1)
	}
	return database
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	cfg := config.LoadConfig()
	cmd := os.Args[1]

	switch cmd {
	case "version", "-v", "--version":
		fmt.Printf("agent-memory version %s (pure-go, zero-cgo)\n", version)

	case "help", "-h", "--help":
		printHelp()

	case "serve":
		database := initDB(cfg)
		defer database.Close()
		if err := server.ServeStdio(database, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server exited with error: %v\n", err)
			os.Exit(1)
		}

	case "bootstrap":
		database := initDB(cfg)
		defer database.Close()
		bm := blocks.NewManager(database)
		_ = bm.SeedDefaults()
		prompt, err := bm.GetBootstrapPrompt()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting bootstrap prompt: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(prompt)

	case "search":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: agent-memory search <query> [--ns <ns>] [--top <k>] [--historical]\n")
			os.Exit(1)
		}
		query := os.Args[2]
		ns := ""
		topK := 5
		historical := false

		for i := 3; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--ns":
				if i+1 < len(os.Args) {
					ns = os.Args[i+1]
					i++
				}
			case "--top":
				if i+1 < len(os.Args) {
					if k, err := strconv.Atoi(os.Args[i+1]); err == nil {
						topK = k
					}
					i++
				}
			case "--historical":
				historical = true
			}
		}

		database := initDB(cfg)
		defer database.Close()
		embClient := embedding.NewHTTPClient(cfg.EmbeddingURL, cfg.OllamaURL, "bge-m3", 1024)
		searcher := retrieval.NewSearcher(database, embClient)

		results, err := searcher.Search(context.Background(), query, ns, topK, historical)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("No matching results found.")
			return
		}

		fmt.Printf("Search Results (%d matches for %q):\n\n", len(results), query)
		for i, res := range results {
			fmt.Printf("[%d] Type: %s | NS: %s | Score: %.4f\n", i+1, res.Type, res.Namespace, res.Score)
			if res.Title != "" {
				fmt.Printf("    Title: %s\n", res.Title)
			}
			fmt.Printf("    Content: %s\n\n", strings.TrimSpace(res.Content))
		}

	case "fact":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: agent-memory fact [add|list] ...\n")
			os.Exit(1)
		}
		subCmd := os.Args[2]
		database := initDB(cfg)
		defer database.Close()
		reg := temporal.NewRegistry(database)

		switch subCmd {
		case "add":
			if len(os.Args) < 7 {
				fmt.Fprintf(os.Stderr, "Usage: agent-memory fact add <ns> <sub> <pred> <obj> [source]\n")
				os.Exit(1)
			}
			ns := os.Args[3]
			sub := os.Args[4]
			pred := os.Args[5]
			obj := os.Args[6]
			source := "cli"
			if len(os.Args) >= 8 {
				source = os.Args[7]
			}

			fact, err := reg.AddFact(ns, sub, pred, obj, source)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error adding fact: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Fact recorded successfully [ID: %s]\n  (%s) --[%s]--> (%s)\n  Namespace: %s | Source: %s\n",
				fact.ID, fact.Subject, fact.Predicate, fact.Object, fact.Namespace, fact.Source)

		case "list":
			ns := ""
			for i := 3; i < len(os.Args); i++ {
				if os.Args[i] == "--ns" && i+1 < len(os.Args) {
					ns = os.Args[i+1]
					i++
				}
			}

			var facts []temporal.Fact
			var err error
			if ns != "" {
				facts, err = reg.GetActiveFacts(ns)
			} else {
				// Get active facts for all namespaces
				query := `SELECT id, namespace, subject, predicate, object, confidence, source,
				          valid_from, valid_until, recorded_at, invalidated_at, superseded_by
				          FROM facts WHERE valid_until IS NULL ORDER BY recorded_at DESC`
				facts, err = reg.QueryFactsList(query)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing facts: %v\n", err)
				os.Exit(1)
			}

			if len(facts) == 0 {
				fmt.Println("No active facts found.")
				return
			}

			fmt.Printf("Active Facts (%d):\n", len(facts))
			for _, f := range facts {
				fmt.Printf("  • [%s] (%s) --[%s]--> (%s) [source: %s]\n",
					f.Namespace, f.Subject, f.Predicate, f.Object, f.Source)
			}

		default:
			fmt.Fprintf(os.Stderr, "Unknown fact command: %s. Use 'add' or 'list'.\n", subCmd)
			os.Exit(1)
		}

	case "block":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: agent-memory block [get|set|list] ...\n")
			os.Exit(1)
		}
		subCmd := os.Args[2]
		database := initDB(cfg)
		defer database.Close()
		bm := blocks.NewManager(database)
		_ = bm.SeedDefaults()

		switch subCmd {
		case "get":
			if len(os.Args) < 4 {
				fmt.Fprintf(os.Stderr, "Usage: agent-memory block get <label>\n")
				os.Exit(1)
			}
			label := os.Args[3]
			block, err := bm.GetBlock(label)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("=== Block: %s (updated: %s) ===\n%s\n", block.Label, block.UpdatedAt.Format("2006-01-02 15:04:05"), block.Content)

		case "set":
			if len(os.Args) < 5 {
				fmt.Fprintf(os.Stderr, "Usage: agent-memory block set <label> <content>\n")
				os.Exit(1)
			}
			label := os.Args[3]
			content := strings.Join(os.Args[4:], " ")
			if err := bm.SetBlock(label, content); err != nil {
				fmt.Fprintf(os.Stderr, "Error setting block: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Block %q updated successfully.\n", label)

		case "list":
			blockList, err := bm.ListBlocks()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing blocks: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Working Core Blocks (%d):\n", len(blockList))
			for _, b := range blockList {
				fmt.Printf("  • %-12s [%d chars, updated: %s]\n", b.Label, len(b.Content), b.UpdatedAt.Format("2006-01-02 15:04:05"))
			}

		default:
			fmt.Fprintf(os.Stderr, "Unknown block command: %s. Use 'get', 'set', or 'list'.\n", subCmd)
			os.Exit(1)
		}

	case "ingest":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: agent-memory ingest <file_or_dir> [--ns <ns>]\n")
			os.Exit(1)
		}
		targetPath := os.Args[2]
		ns := "documents"
		for i := 3; i < len(os.Args); i++ {
			if os.Args[i] == "--ns" && i+1 < len(os.Args) {
				ns = os.Args[i+1]
				i++
			}
		}

		database := initDB(cfg)
		defer database.Close()
		embClient := embedding.NewHTTPClient(cfg.EmbeddingURL, cfg.OllamaURL, "bge-m3", 1024)

		info, err := os.Stat(targetPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading path %q: %v\n", targetPath, err)
			os.Exit(1)
		}

		var files []string
		if info.IsDir() {
			err = filepath.Walk(targetPath, func(p string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !fi.IsDir() && (strings.HasSuffix(p, ".md") || strings.HasSuffix(p, ".txt")) {
					files = append(files, p)
				}
				return nil
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
				os.Exit(1)
			}
		} else {
			files = append(files, targetPath)
		}

		if len(files) == 0 {
			fmt.Println("No markdown (.md) or text (.txt) files found to ingest.")
			return
		}

		fmt.Printf("Ingesting %d file(s) into namespace %q...\n", len(files), ns)
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed reading %s: %v\n", f, err)
				continue
			}
			title := filepath.Base(f)
			if err := pipeline.IngestMarkdown(database, embClient, ns, title, string(data)); err != nil {
				fmt.Fprintf(os.Stderr, "Failed ingesting %s: %v\n", f, err)
			} else {
				fmt.Printf("  ✓ Ingested %s\n", title)
			}
		}

	case "stats":
		database := initDB(cfg)
		defer database.Close()

		fmt.Println("=== AgentMemory Universal Statistics ===")
		// Namespaces and fact count
		rows, err := database.Query(`SELECT namespace, COUNT(*), SUM(CASE WHEN valid_until IS NULL THEN 1 ELSE 0 END) FROM facts GROUP BY namespace`)
		if err == nil {
			fmt.Println("\nFacts by Namespace:")
			for rows.Next() {
				var ns string
				var total, active int
				_ = rows.Scan(&ns, &total, &active)
				fmt.Printf("  • %-15s total: %-4d active: %-4d\n", ns, total, active)
			}
			rows.Close()
		}

		// Chunks by namespace
		cRows, err := database.Query(`SELECT namespace, COUNT(*) FROM chunks GROUP BY namespace`)
		if err == nil {
			fmt.Println("\nChunks by Namespace:")
			for cRows.Next() {
				var ns string
				var count int
				_ = cRows.Scan(&ns, &count)
				fmt.Printf("  • %-15s chunks: %d\n", ns, count)
			}
			cRows.Close()
		}

		// Entities by type
		eRows, err := database.Query(`SELECT entity_type, COUNT(*) FROM entities GROUP BY entity_type`)
		if err == nil {
			fmt.Println("\nEntities by Type:")
			for eRows.Next() {
				var t string
				var count int
				_ = eRows.Scan(&t, &count)
				fmt.Printf("  • %-15s count: %d\n", t, count)
			}
			eRows.Close()
		}

	case "health":
		database := initDB(cfg)
		defer database.Close()

		if err := database.Ping(); err != nil {
			fmt.Printf("Database: UNHEALTHY (%v)\n", err)
			os.Exit(1)
		}

		var factsCount, blocksCount, chunksCount, entitiesCount, obsCount int
		_ = database.QueryRow("SELECT COUNT(*) FROM facts").Scan(&factsCount)
		_ = database.QueryRow("SELECT COUNT(*) FROM core_blocks").Scan(&blocksCount)
		_ = database.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&chunksCount)
		_ = database.QueryRow("SELECT COUNT(*) FROM entities").Scan(&entitiesCount)
		_ = database.QueryRow("SELECT COUNT(*) FROM observations").Scan(&obsCount)

		fmt.Println("Status:        HEALTHY")
		fmt.Println("Engine:        modernc.org/sqlite (Pure Go, Zero CGO, WAL mode)")
		fmt.Printf("Database Path: %s\n", cfg.DBPath)
		fmt.Printf("Core Blocks:   %d\n", blocksCount)
		fmt.Printf("Atomic Facts:  %d\n", factsCount)
		fmt.Printf("Chunks:        %d\n", chunksCount)
		fmt.Printf("Entities:      %d\n", entitiesCount)
		fmt.Printf("Observations:  %d\n", obsCount)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nRun 'agent-memory help' for usage.\n", cmd)
		os.Exit(1)
	}
}
