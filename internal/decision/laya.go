package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/surtr85/agent-memory/internal/config"
)

// Engine manages Laya System-1 / Jev AI decision engine integration with fallbacks.
type Engine struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewEngine creates a new decision Engine.
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Decide executes a decision against Laya System-1:
// 1. First attempts HTTP POST to cfg.LayaURL with {"state": state, "preset": preset}.
// 2. If HTTP fails or is unreachable, checks if cfg.LayaBinPath and cfg.LayaModelPath exist on disk.
//    If yes, runs: exec.CommandContext(ctx, cfg.LayaBinPath, "decide", cfg.LayaModelPath, "--preset", preset, "--state", state, "--json") and parses JSON stdout.
// 3. If neither works, falls back to deterministic heuristic rules.
func (e *Engine) Decide(ctx context.Context, state string, preset string) (map[string]any, error) {
	// 1. Try HTTP POST
	if e.cfg.LayaURL != "" {
		res, err := e.decideHTTP(ctx, state, preset)
		if err == nil && res != nil {
			return res, nil
		}
	}

	// 2. Try CLI execution if binary and model exist on disk
	if e.cfg.LayaBinPath != "" && e.cfg.LayaModelPath != "" {
		binStat, errBin := os.Stat(e.cfg.LayaBinPath)
		modelStat, errModel := os.Stat(e.cfg.LayaModelPath)
		if errBin == nil && !binStat.IsDir() && errModel == nil && !modelStat.IsDir() {
			res, err := e.decideCLI(ctx, state, preset)
			if err == nil && res != nil {
				return res, nil
			}
		}
	}

	// 3. Fallback to deterministic heuristic rules
	return e.decideHeuristic(state, preset)
}

func (e *Engine) decideHTTP(ctx context.Context, state string, preset string) (map[string]any, error) {
	reqBody, err := json.Marshal(map[string]string{
		"state":  state,
		"preset": preset,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.LayaURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("laya http status %d: %s", resp.StatusCode, string(body))
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res, nil
}

func (e *Engine) decideCLI(ctx context.Context, state string, preset string) (map[string]any, error) {
	cmd := exec.CommandContext(ctx, e.cfg.LayaBinPath, "decide", e.cfg.LayaModelPath, "--preset", preset, "--state", state, "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Laya may output status/logging lines before the JSON object.
	// Extract the JSON object starting from the first '{' and ending at the last '}'.
	outStr := strings.TrimSpace(string(out))
	startIdx := strings.Index(outStr, "{")
	endIdx := strings.LastIndex(outStr, "}")
	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return nil, fmt.Errorf("no valid JSON object found in laya output: %s", outStr)
	}
	jsonPart := outStr[startIdx : endIdx+1]

	var res map[string]any
	if err := json.Unmarshal([]byte(jsonPart), &res); err != nil {
		return nil, err
	}
	return res, nil
}

// RouteQuery uses Decide with preset "router".
// Returns namespace ("system", "forex", "ecommerce", "literature", "ai", "general") and confidence.
func (e *Engine) RouteQuery(ctx context.Context, query string) (string, float64, error) {
	res, err := e.Decide(ctx, query, "router")
	if err != nil {
		return "general", 0.5, err
	}

	ns := ""
	confidence := 0.85

	// Check direct fields
	if val, ok := res["namespace"].(string); ok && val != "" {
		ns = val
	} else if val, ok := res["action"].(string); ok && val != "" {
		ns = val
	} else if val, ok := res["target"].(string); ok && val != "" {
		ns = val
	} else if val, ok := res["category"].(string); ok && val != "" {
		ns = val
	}

	// Check if Laya model returned answers structure (e.g. answers.domain)
	if answers, ok := res["answers"].(map[string]any); ok {
		if domainObj, ok := answers["domain"].(map[string]any); ok {
			if choice, ok := domainObj["choice"].(string); ok && choice != "" {
				switch choice {
				case "code":
					ns = "system"
				case "data_analysis":
					ns = "forex"
				case "writing":
					ns = "literature"
				case "factual_lookup":
					ns = "general"
				default:
					ns = choice
				}
			}
			if conf, ok := domainObj["confidence"].(float64); ok {
				confidence = conf
			}
		}
	}

	if val, ok := res["confidence"].(float64); ok {
		confidence = val
	}

	if ns == "" {
		ns = "general"
	}

	return ns, confidence, nil
}

// TriageObservation evaluates observation importance and category.
func (e *Engine) TriageObservation(ctx context.Context, content string) (string, bool, float64, error) {
	res, err := e.Decide(ctx, content, "triage")
	if err != nil {
		return "observation", false, 0.5, err
	}

	category := "observation"
	isNoise := false
	importance := 0.7

	if val, ok := res["category"].(string); ok && val != "" {
		category = val
	}
	if val, ok := res["is_noise"].(bool); ok {
		isNoise = val
	}
	if val, ok := res["importance"].(float64); ok {
		importance = val
	}

	return category, isNoise, importance, nil
}

// decideHeuristic provides deterministic fallback when HTTP and CLI are unavailable.
func (e *Engine) decideHeuristic(state string, preset string) (map[string]any, error) {
	lower := strings.ToLower(state)

	switch preset {
	case "router":
		ns, conf := classifyNamespace(lower)
		return map[string]any{
			"namespace":  ns,
			"confidence": conf,
			"source":     "heuristic",
		}, nil

	case "triage":
		cat, noise, imp := classifyTriage(lower)
		return map[string]any{
			"category":   cat,
			"is_noise":   noise,
			"importance": imp,
			"source":     "heuristic",
		}, nil

	default:
		// Generic fallback
		ns, conf := classifyNamespace(lower)
		return map[string]any{
			"action":     "execute",
			"target":     ns,
			"confidence": conf,
			"source":     "heuristic",
		}, nil
	}
}

func containsWord(text, word string) bool {
	pattern := `\b` + regexp.QuoteMeta(word) + `\b`
	matched, _ := regexp.MatchString(pattern, text)
	return matched
}

func matchesAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(kw, " ") {
			if strings.Contains(text, kw) {
				return true
			}
		} else {
			if containsWord(text, kw) {
				return true
			}
		}
	}
	return false
}

func classifyNamespace(text string) (string, float64) {
	// Literature / Fiction / Lore / Creative
	literatureKeywords := []string{
		"literature", "novel", "poetry", "poem", "fiction", "character", "prose",
		"story", "plot", "surtr", "mythology", "narrative", "chapter", "book",
	}
	if matchesAny(text, literatureKeywords) {
		return "literature", 0.90
	}

	// Forex / Trading
	forexKeywords := []string{
		"forex", "trading", "eurusd", "gbpusd", "usdjpy", "pip", "pips", "lot", "lots", "leverage",
		"metatrader", "mt4", "mt5", "candlestick", "stop loss", "take profit",
		"currency", "market structure", "liquidity", "order block", "ict", "smc",
	}
	if matchesAny(text, forexKeywords) {
		return "forex", 0.92
	}

	// System / OS / Compositors / Linux
	systemKeywords := []string{
		"nixos", "nix", "linux", "systemd", "kernel", "wayland", "niri", "sway", "hyprland",
		"pipewire", "configuration.nix", "flake", "compositor", "desktop", "x11", "xorg",
		"memory.db", "agent-memory", "system", "bash", "zsh", "dotfiles",
	}
	if matchesAny(text, systemKeywords) {
		return "system", 0.95
	}

	// Ecommerce
	ecommerceKeywords := []string{
		"ecommerce", "e-commerce", "shopify", "woocommerce", "product", "cart",
		"checkout", "inventory", "shipping", "sku", "payment gateway", "order fulfillment",
	}
	if matchesAny(text, ecommerceKeywords) {
		return "ecommerce", 0.90
	}

	// AI / Machine Learning / LLMs
	aiKeywords := []string{
		"ai", "llm", "embedding", "rag", "retrieval", "prompt", "transformer",
		"gpt", "claude", "ollama", "neural", "bge-m3", "fine-tuning", "vector",
	}
	if matchesAny(text, aiKeywords) {
		return "ai", 0.90
	}

	return "general", 0.70
}

func classifyTriage(text string) (string, bool, float64) {
	// Noise detection
	noiseKeywords := []string{"ping", "ok", "test", "hello", "hi", "bye", "thanks", "done"}
	trimmed := strings.TrimSpace(text)
	for _, kw := range noiseKeywords {
		if trimmed == kw {
			return "noise", true, 0.1
		}
	}

	// Preference
	preferenceKeywords := []string{"prefer", "likes", "dislikes", "favorite", "always use", "never use", "wants"}
	for _, kw := range preferenceKeywords {
		if strings.Contains(text, kw) {
			return "preference", false, 0.95
		}
	}

	// Insight / Fact
	insightKeywords := []string{"found that", "learned", "insight", "solution", "fixed by", "root cause"}
	for _, kw := range insightKeywords {
		if strings.Contains(text, kw) {
			return "insight", false, 0.88
		}
	}

	// Tool Result
	toolKeywords := []string{"output:", "exit code", "error:", "returned", "stderr", "stdout"}
	for _, kw := range toolKeywords {
		if strings.Contains(text, kw) {
			return "tool_result", false, 0.75
		}
	}

	return "observation", false, 0.60
}
