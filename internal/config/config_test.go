package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	os.Unsetenv("AGENT_MEMORY_DB")
	os.Unsetenv("AGENT_MEMORY_EMBEDDING_URL")
	os.Unsetenv("AGENT_MEMORY_OLLAMA_URL")
	os.Unsetenv("AGENT_MEMORY_LOG_LEVEL")
	os.Unsetenv("AGENT_MEMORY_LAYA_URL")
	os.Unsetenv("AGENT_MEMORY_LAYA_MODEL")
	os.Unsetenv("AGENT_MEMORY_LAYA_BIN")

	cfg := LoadConfig()
	if cfg == nil {
		t.Fatal("expected cfg to be non-nil")
	}

	home, _ := os.UserHomeDir()
	expectedDBPath := filepath.Join(home, ".local/share/agent-memory/memory.db")
	if cfg.DBPath != expectedDBPath {
		t.Errorf("expected DBPath %q, got %q", expectedDBPath, cfg.DBPath)
	}
	if cfg.EmbeddingURL != "http://127.0.0.1:8088/embedding" {
		t.Errorf("unexpected EmbeddingURL: %s", cfg.EmbeddingURL)
	}
	if cfg.OllamaURL != "http://127.0.0.1:11434" {
		t.Errorf("unexpected OllamaURL: %s", cfg.OllamaURL)
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("unexpected LogLevel: %s", cfg.LogLevel)
	}
	if cfg.LayaURL != "http://127.0.0.1:8080/v1/systemone" {
		t.Errorf("unexpected LayaURL: %s", cfg.LayaURL)
	}
	if cfg.LayaModelPath != "/home/amadeus/Projects/models/laya/laya_multilingual_q8_0.gguf" {
		t.Errorf("unexpected LayaModelPath: %s", cfg.LayaModelPath)
	}
	if cfg.LayaBinPath != "/home/amadeus/Projects/bin/laya-gpu" {
		t.Errorf("unexpected LayaBinPath: %s", cfg.LayaBinPath)
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	os.Setenv("AGENT_MEMORY_DB", "/tmp/test-memory.db")
	os.Setenv("AGENT_MEMORY_EMBEDDING_URL", "http://localhost:9000/embed")
	os.Setenv("AGENT_MEMORY_OLLAMA_URL", "http://localhost:11435")
	os.Setenv("AGENT_MEMORY_LOG_LEVEL", "DEBUG")
	os.Setenv("AGENT_MEMORY_LAYA_URL", "http://localhost:8081/decision")
	os.Setenv("AGENT_MEMORY_LAYA_MODEL", "/tmp/model.gguf")
	os.Setenv("AGENT_MEMORY_LAYA_BIN", "/tmp/laya-bin")
	defer func() {
		os.Unsetenv("AGENT_MEMORY_DB")
		os.Unsetenv("AGENT_MEMORY_EMBEDDING_URL")
		os.Unsetenv("AGENT_MEMORY_OLLAMA_URL")
		os.Unsetenv("AGENT_MEMORY_LOG_LEVEL")
		os.Unsetenv("AGENT_MEMORY_LAYA_URL")
		os.Unsetenv("AGENT_MEMORY_LAYA_MODEL")
		os.Unsetenv("AGENT_MEMORY_LAYA_BIN")
	}()

	cfg := LoadConfig()
	if cfg.DBPath != "/tmp/test-memory.db" {
		t.Errorf("expected DBPath /tmp/test-memory.db, got %q", cfg.DBPath)
	}
	if cfg.EmbeddingURL != "http://localhost:9000/embed" {
		t.Errorf("unexpected EmbeddingURL: %s", cfg.EmbeddingURL)
	}
	if cfg.OllamaURL != "http://localhost:11435" {
		t.Errorf("unexpected OllamaURL: %s", cfg.OllamaURL)
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("unexpected LogLevel: %s", cfg.LogLevel)
	}
	if cfg.LayaURL != "http://localhost:8081/decision" {
		t.Errorf("unexpected LayaURL: %s", cfg.LayaURL)
	}
	if cfg.LayaModelPath != "/tmp/model.gguf" {
		t.Errorf("unexpected LayaModelPath: %s", cfg.LayaModelPath)
	}
	if cfg.LayaBinPath != "/tmp/laya-bin" {
		t.Errorf("unexpected LayaBinPath: %s", cfg.LayaBinPath)
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	tests := []struct {
		input    string
		expected string
	}{
		{"~", home},
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"/var/log", "/var/log"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandPath(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
