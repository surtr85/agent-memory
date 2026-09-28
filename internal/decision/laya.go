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
	"github.com/surtr85/agent-memory/internal/normalizer"
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

// DecideConflict evaluates if two statements or fact values contradict each other.
func (e *Engine) DecideConflict(ctx context.Context, existingFact, newFact string) (bool, float64, error) {
	state := fmt.Sprintf("Fact 1: %s\nFact 2: %s", existingFact, newFact)
	res, err := e.Decide(ctx, state, "conflict")
	if err != nil {
		return false, 0.5, err
	}

	isContradiction := false
	confidence := 0.80

	if val, ok := res["is_contradiction"].(bool); ok {
		isContradiction = val
	} else if val, ok := res["conflict"].(bool); ok {
		isContradiction = val
	} else if val, ok := res["contradiction"].(bool); ok {
		isContradiction = val
	}

	if val, ok := res["confidence"].(float64); ok {
		confidence = val
	}

	return isContradiction, confidence, nil
}

// Reflect runs System-1 cognitive appraisal of current memory blocks, observations, or goals.
func (e *Engine) Reflect(ctx context.Context, memoryState string) (map[string]any, error) {
	if strings.TrimSpace(memoryState) == "" {
		memoryState = "current agent status: idle"
	}
	return e.Decide(ctx, memoryState, "reflect")
}

// decideHeuristic provides deterministic fallback when HTTP and CLI are unavailable.
func (e *Engine) decideHeuristic(state string, preset string) (map[string]any, error) {
	norm := normalizer.Normalize(state)
	lower := strings.ToLower(norm)

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

	case "conflict":
		contradicts, conf := classifyConflict(lower)
		return map[string]any{
			"is_contradiction": contradicts,
			"conflict":         contradicts,
			"confidence":       conf,
			"source":           "heuristic",
		}, nil

	case "reflect":
		status := appraisalMemoryState(lower, state)
		return map[string]any{
			"cognitive_status": status["status"],
			"focus":            status["focus"],
			"alert_level":      status["alert_level"],
			"recommendations":  status["recommendations"],
			"timestamp":        time.Now().UTC().Format(time.RFC3339),
			"source":           "heuristic",
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
		kwNorm := strings.ToLower(normalizer.Normalize(kw))
		if strings.Contains(kwNorm, " ") {
			if strings.Contains(text, kwNorm) {
				return true
			}
		} else {
			if containsWord(text, kwNorm) || strings.Contains(text, kwNorm) {
				return true
			}
		}
	}
	return false
}

func classifyNamespace(text string) (string, float64) {
	// Literature / Fiction / Lore / Creative (Multilingual Persian & English)
	literatureKeywords := []string{
		"literature", "novel", "poetry", "poem", "fiction", "character", "prose",
		"story", "plot", "surtr", "mythology", "narrative", "chapter", "book",
		"شعر", "ادبیات", "رمان", "داستان", "اسطوره‌شناسی", "روایت", "نویسندگی",
	}
	if matchesAny(text, literatureKeywords) {
		return "literature", 0.90
	}

	// Forex / Trading (Multilingual Persian & English)
	forexKeywords := []string{
		"forex", "trading", "eurusd", "gbpusd", "usdjpy", "pip", "pips", "lot", "lots", "leverage",
		"metatrader", "mt4", "mt5", "candlestick", "stop loss", "take profit",
		"currency", "market structure", "liquidity", "order block", "ict", "smc",
		"فارکس", "معاملات", "ارز دیجیتال", "نقدینگی", "کندل", "حد ضرر", "حد سود", "تحلیل تکنیکال",
	}
	if matchesAny(text, forexKeywords) {
		return "forex", 0.92
	}

	// System / OS / Compositors / Linux (Multilingual Persian & English)
	systemKeywords := []string{
		"nixos", "nix", "linux", "systemd", "kernel", "wayland", "niri", "sway", "hyprland",
		"pipewire", "configuration.nix", "flake", "compositor", "desktop", "x11", "xorg",
		"memory.db", "agent-memory", "system", "bash", "zsh", "dotfiles",
		"لینوکس", "نیکس", "کامپوزیتور", "میزکار", "هسته", "پایپ‌وایر",
	}
	if matchesAny(text, systemKeywords) {
		return "system", 0.95
	}

	// Ecommerce (Multilingual Persian & English)
	ecommerceKeywords := []string{
		"ecommerce", "e-commerce", "shopify", "woocommerce", "product", "cart",
		"checkout", "inventory", "shipping", "sku", "payment gateway", "order fulfillment",
		"فروشگاه", "سبد خرید", "موجودی", "سفارش", "پرداخت", "ارسال", "محصول",
	}
	if matchesAny(text, ecommerceKeywords) {
		return "ecommerce", 0.90
	}

	// AI / Machine Learning / LLMs (Multilingual Persian & English)
	aiKeywords := []string{
		"ai", "llm", "embedding", "rag", "retrieval", "prompt", "transformer",
		"gpt", "claude", "ollama", "neural", "bge-m3", "fine-tuning", "vector",
		"هوش مصنوعی", "مدل زبانی", "امبدینگ", "بردار", "بازیابی", "ترانسفورمر",
	}
	if matchesAny(text, aiKeywords) {
		return "ai", 0.90
	}

	return "general", 0.70
}

func classifyTriage(text string) (string, bool, float64) {
	// Noise detection
	noiseKeywords := []string{
		"ping", "ok", "test", "hello", "hi", "bye", "thanks", "done",
		"سلام", "مرسی", "ممنون", "تست", "خداحافظ",
	}
	trimmed := strings.TrimSpace(text)
	for _, kw := range noiseKeywords {
		if trimmed == kw {
			return "noise", true, 0.1
		}
	}

	// Preference
	preferenceKeywords := []string{
		"prefer", "likes", "dislikes", "favorite", "always use", "never use", "wants",
		"ترجیح می‌دهد", "علاقه‌مند", "دوست دارد", "همیشه", "هرگز",
	}
	for _, kw := range preferenceKeywords {
		if strings.Contains(text, kw) {
			return "preference", false, 0.95
		}
	}

	// Insight / Fact
	insightKeywords := []string{
		"found that", "learned", "insight", "solution", "fixed by", "root cause",
		"مشخص شد", "راه حل", "برطرف شد", "علت اصلی", "کشف شد",
	}
	for _, kw := range insightKeywords {
		if strings.Contains(text, kw) {
			return "insight", false, 0.88
		}
	}

	// Tool Result
	toolKeywords := []string{"output:", "exit code", "error:", "returned", "stderr", "stdout", "خروجی:", "خطا:"}
	for _, kw := range toolKeywords {
		if strings.Contains(text, kw) {
			return "tool_result", false, 0.75
		}
	}

	return "observation", false, 0.60
}

// classifyConflict checks if statements have contradictory values or negation patterns.
func classifyConflict(text string) (bool, float64) {
	parts := strings.Split(text, "\n")
	if len(parts) >= 2 {
		f1 := strings.TrimPrefix(strings.TrimSpace(parts[0]), "fact 1:")
		f2 := strings.TrimPrefix(strings.TrimSpace(parts[1]), "fact 2:")
		f1 = strings.TrimSpace(f1)
		f2 = strings.TrimSpace(f2)

		// Direct identical statements
		if f1 == f2 {
			return false, 0.99
		}

		// Mutually exclusive antonyms / values
		antonymPairs := [][2]string{
			{"true", "false"},
			{"enabled", "disabled"},
			{"on", "off"},
			{"active", "inactive"},
			{"yes", "no"},
			{"bullish", "bearish"},
			{"dark", "light"},
			{"خاموش", "روشن"},
			{"فعال", "غیرفعال"},
			{"درست", "نادرست"},
			{"صعودی", "نزولی"},
		}

		for _, pair := range antonymPairs {
			hasFirst1 := strings.Contains(f1, pair[0])
			hasSecond1 := strings.Contains(f1, pair[1])
			hasFirst2 := strings.Contains(f2, pair[0])
			hasSecond2 := strings.Contains(f2, pair[1])

			if (hasFirst1 && hasSecond2) || (hasSecond1 && hasFirst2) {
				return true, 0.95
			}
		}

		// Direct negation check: one has "not", "never", "نیست", "نباید" while other doesn't
		negations := []string{" not ", " never ", " doesn't ", " cannot ", " نیست ", " نباشد ", " غیر "}
		hasNeg1 := false
		for _, neg := range negations {
			if strings.Contains(" "+f1+" ", neg) {
				hasNeg1 = true
				break
			}
		}
		hasNeg2 := false
		for _, neg := range negations {
			if strings.Contains(" "+f2+" ", neg) {
				hasNeg2 = true
				break
			}
		}
		if hasNeg1 != hasNeg2 {
			// One is negated while the other is affirmative
			return true, 0.88
		}

		// If statements talk about same attribute/property with different explicit values
		if f1 != f2 && len(f1) > 0 && len(f2) > 0 {
			// e.g. "port: 8080" vs "port: 9090", "editor: vim" vs "editor: emacs"
			return true, 0.75
		}
	}

	return false, 0.50
}

func appraisalMemoryState(lowerText, rawText string) map[string]any {
	focus := "general operations"
	alertLevel := "nominal"
	var recommendations []string

	if strings.Contains(lowerText, "error") || strings.Contains(lowerText, "fail") || strings.Contains(lowerText, "خطا") {
		alertLevel = "elevated"
		focus = "troubleshooting and error resolution"
		recommendations = append(recommendations, "Inspect failing services and review recent logs")
	}

	if strings.Contains(lowerText, "trade") || strings.Contains(lowerText, "forex") || strings.Contains(lowerText, "معامله") {
		focus = "quantitative trading & risk management"
		recommendations = append(recommendations, "Verify active stop loss and killzone timing before execution")
	}

	if strings.Contains(lowerText, "nixos") || strings.Contains(lowerText, "niri") || strings.Contains(lowerText, "system") {
		focus = "system environment & configuration"
		recommendations = append(recommendations, "Ensure dotfiles and system flake remain synchronized")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Maintain regular memory consolidation and atomic fact indexing")
	}

	return map[string]any{
		"status":          "optimal",
		"focus":           focus,
		"alert_level":     alertLevel,
		"recommendations": recommendations,
	}
}
