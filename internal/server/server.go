package server

import (
	"database/sql"

	"github.com/mark3labs/mcp-go/server"
	"github.com/surtr85/agent-memory/internal/alignment"
	"github.com/surtr85/agent-memory/internal/blocks"
	"github.com/surtr85/agent-memory/internal/config"
	"github.com/surtr85/agent-memory/internal/decision"
	"github.com/surtr85/agent-memory/internal/embedding"
	"github.com/surtr85/agent-memory/internal/forget"
	"github.com/surtr85/agent-memory/internal/retrieval"
	"github.com/surtr85/agent-memory/internal/temporal"
)

// Server encapsulates the FastMCP server, DB handle, config, and domain services.
type Server struct {
	MCPServer *server.MCPServer
	DB        *sql.DB
	Config    *config.Config
	Blocks    *blocks.Manager
	Temporal  *temporal.Registry
	Searcher  *retrieval.Searcher
	Embedding embedding.Client
	Decision  *decision.Engine
	Alignment *alignment.Manager
	Forget    *forget.Pipeline
}

// NewServer creates a new FastMCP server instance and registers all tools.
func NewServer(db *sql.DB, cfg *config.Config) *Server {
	mcpServer := server.NewMCPServer(
		"agent-memory",
		"3.0.0",
		server.WithDescription("AgentMemory Universal (v3.0) Cognitive Engine - Pure Go Local Memory"),
	)

	// Initialize embedding client: prefer configured HTTP endpoint (llama-server / OpenAI / BGE / Ollama)
	// with automatic pure Go subword vectorizer fallback.
	var embClient embedding.Client
	if cfg.EmbeddingURL != "" || cfg.OllamaURL != "" {
		embClient = embedding.NewHTTPClient(cfg.EmbeddingURL, cfg.OllamaURL, cfg.EmbeddingModel, cfg.EmbeddingDimensions)
	} else {
		embClient = embedding.NewBuiltinVectorizer()
	}
	decEngine := decision.NewEngine(cfg)
	s := &Server{
		MCPServer: mcpServer,
		DB:        db,
		Config:    cfg,
		Blocks:    blocks.NewManager(db),
		Temporal:  temporal.NewRegistry(db).WithDecision(decEngine),
		Searcher:  retrieval.NewSearcher(db, embClient),
		Embedding: embClient,
		Decision:  decEngine,
		Alignment: alignment.NewManager(db),
		Forget:    forget.NewPipeline(db, decEngine),
	}

	// Ensure core blocks have defaults seeded
	_ = s.Blocks.SeedDefaults()

	// Register 15 MCP tools
	s.registerTools()

	return s
}

// ServeStdio starts the MCP server over standard I/O.
func ServeStdio(db *sql.DB, cfg *config.Config) error {
	srv := NewServer(db, cfg)
	return server.ServeStdio(srv.MCPServer)
}

// ServeStdio starts the MCP server over standard I/O.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.MCPServer)
}
