package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config represents runtime configuration for AgentMemory Universal.
type Config struct {
	DBPath        string
	EmbeddingURL  string
	OllamaURL     string
	LogLevel      string
	LayaURL       string
	LayaModelPath string
	LayaBinPath   string
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

// FindFirstExisting checks a list of candidate filepaths and returns the first one that exists.
func FindFirstExisting(paths []string) string {
	for _, p := range paths {
		expanded := ExpandPath(p)
		if fi, err := os.Stat(expanded); err == nil && !fi.IsDir() {
			return expanded
		}
	}
	return ""
}

// AutoDiscoverLayaBin searches system PATH and standard user directories for the laya binary.
func AutoDiscoverLayaBin() string {
	// 1. Check explicit environment override
	if env := os.Getenv("AGENT_MEMORY_LAYA_BIN"); env != "" {
		return ExpandPath(env)
	}

	// 2. Search system PATH for laya-gpu, laya, or laya-cli
	binNames := []string{"laya-gpu", "laya", "laya-cli", "laya-vulkan-bin"}
	for _, name := range binNames {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	// 3. Search standard local user directories
	homeDir, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(homeDir, "Projects", "bin", "laya-gpu"),
		filepath.Join(homeDir, "Projects", "bin", "laya-cli"),
		filepath.Join(homeDir, ".local", "bin", "laya-gpu"),
		filepath.Join(homeDir, ".local", "bin", "laya"),
		filepath.Join(homeDir, ".local", "bin", "laya-cli"),
		filepath.Join(homeDir, "bin", "laya-gpu"),
		filepath.Join(homeDir, "bin", "laya"),
		"/usr/local/bin/laya-gpu",
		"/usr/local/bin/laya",
	}

	return FindFirstExisting(candidates)
}

// AutoDiscoverLayaModel searches standard directories for a Laya GGUF model file.
func AutoDiscoverLayaModel() string {
	// 1. Check explicit environment override
	if env := os.Getenv("AGENT_MEMORY_LAYA_MODEL"); env != "" {
		return ExpandPath(env)
	}

	homeDir, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(homeDir, "Projects", "models", "laya", "laya_multilingual_q8_0.gguf"),
		filepath.Join(homeDir, "Projects", "models", "laya", "laya_english_q8_0.gguf"),
		filepath.Join(homeDir, ".local", "share", "laya", "models", "laya_multilingual_q8_0.gguf"),
		filepath.Join(homeDir, ".local", "share", "laya", "models", "laya_english_q8_0.gguf"),
		filepath.Join(homeDir, ".cache", "laya", "laya_multilingual_q8_0.gguf"),
		filepath.Join(homeDir, "models", "laya_multilingual_q8_0.gguf"),
		"./models/laya_multilingual_q8_0.gguf",
	}

	found := FindFirstExisting(candidates)
	if found != "" {
		return found
	}

	// Glob pattern search in ~/.local/share/laya/models/ or ~/Projects/models/laya/
	dirs := []string{
		filepath.Join(homeDir, "Projects", "models", "laya"),
		filepath.Join(homeDir, ".local", "share", "laya", "models"),
		filepath.Join(homeDir, ".cache", "laya"),
	}
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, "*.gguf"))
		if err == nil && len(matches) > 0 {
			return matches[0]
		}
	}

	return ""
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// LoadConfig loads configuration from environment variables with auto-discovery fallbacks.
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

	layaURL := os.Getenv("AGENT_MEMORY_LAYA_URL")
	if layaURL == "" {
		layaURL = "http://127.0.0.1:8080/v1/systemone"
	}

	layaModelPath := AutoDiscoverLayaModel()
	layaBinPath := AutoDiscoverLayaBin()

	return &Config{
		DBPath:        dbPath,
		EmbeddingURL:  embeddingURL,
		OllamaURL:     ollamaURL,
		LogLevel:      logLevel,
		LayaURL:       layaURL,
		LayaModelPath: layaModelPath,
		LayaBinPath:   layaBinPath,
	}
}
