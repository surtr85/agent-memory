package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Config represents runtime configuration for AgentMemory Universal.
type Config struct {
	DBPath       string
	EmbeddingURL string
	OllamaURL    string
	LogLevel     string
}

// ExpandPath expands leading ~ to user's home directory.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return homeDir
			}
			return filepath.Join(homeDir, path[2:])
		}
	}
	return path
}

// LoadConfig loads configuration from environment variables with fallback defaults.
func LoadConfig() *Config {
	dbPath := os.Getenv("AGENT_MEMORY_DB")
	if dbPath == "" {
		dbPath = "~/.local/share/agent-memory/memory.db"
	}
	dbPath = ExpandPath(dbPath)

	embeddingURL := os.Getenv("AGENT_MEMORY_EMBEDDING_URL")
	if embeddingURL == "" {
		embeddingURL = "http://127.0.0.1:8088/embedding"
	}

	ollamaURL := os.Getenv("AGENT_MEMORY_OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://127.0.0.1:11434"
	}

	logLevel := os.Getenv("AGENT_MEMORY_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}

	return &Config{
		DBPath:       dbPath,
		EmbeddingURL: embeddingURL,
		OllamaURL:    ollamaURL,
		LogLevel:     logLevel,
	}
}
