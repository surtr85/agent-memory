package decision

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/surtr85/agent-memory/internal/config"
)

func TestDecide_HeuristicRouter(t *testing.T) {
	cfg := &config.Config{
		LayaURL:       "",
		LayaBinPath:   "",
		LayaModelPath: "",
	}
	engine := NewEngine(cfg)

	tests := []struct {
		query    string
		expected string
	}{
		{"How to configure Niri window manager?", "system"},
		{"NixOS flake configuration for Hyprland", "system"},
		{"EURUSD 15m order block strategy and trading liquidity", "forex"},
		{"Shopify product inventory sync checkout flow", "ecommerce"},
		{"Poetry and narrative plot about Norse mythology Surtr", "literature"},
		{"Fine-tuning BGE-M3 embedding model with RAG retrieval", "ai"},
		{"Random notes about today's grocery list", "general"},
		// Multilingual Persian tests
		{"تنظیم کامپوزیتور Niri در لینوکس نیکس", "system"},
		{"استراتژی نقدینگی و حد ضرر در بازار فارکس", "forex"},
		{"داستان و رمان اسطوره‌شناسی درباره سرتر", "literature"},
		{"پیاده‌سازی مدل زبانی و امبدینگ هوش مصنوعی", "ai"},
		{"مدیریت سبد خرید و ارسال سفارش فروشگاه", "ecommerce"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			ns, conf, err := engine.RouteQuery(context.Background(), tt.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ns != tt.expected {
				t.Errorf("RouteQuery(%q) = %q (conf %.2f); want %q", tt.query, ns, conf, tt.expected)
			}
			if conf <= 0 {
				t.Errorf("expected positive confidence, got %.2f", conf)
			}
		})
	}
}

func TestDecide_HeuristicTriage(t *testing.T) {
	cfg := &config.Config{
		LayaURL:       "",
		LayaBinPath:   "",
		LayaModelPath: "",
	}
	engine := NewEngine(cfg)

	tests := []struct {
		content          string
		expectedCategory string
		expectedNoise    bool
	}{
		{"ping", "noise", true},
		{"سلام", "noise", true},
		{"User prefers dark mode and Neovim", "preference", false},
		{"کاربر ترجیح می‌دهد از تم تاریک استفاده کند", "preference", false},
		{"We found that restarting pipewire fixed the sound issue", "insight", false},
		{"مشخص شد با ری‌استارت پایپ‌وایر صدا وصل شد", "insight", false},
		{"command returned error: exit code 1", "tool_result", false},
		{"Standard note about current context", "observation", false},
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			cat, noise, imp, err := engine.TriageObservation(context.Background(), tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cat != tt.expectedCategory {
				t.Errorf("TriageObservation(%q) category = %q; want %q", tt.content, cat, tt.expectedCategory)
			}
			if noise != tt.expectedNoise {
				t.Errorf("TriageObservation(%q) isNoise = %v; want %v", tt.content, noise, tt.expectedNoise)
			}
			if imp <= 0 {
				t.Errorf("expected positive importance, got %.2f", imp)
			}
		})
	}
}

func TestDecideConflict(t *testing.T) {
	cfg := &config.Config{
		LayaURL:       "",
		LayaBinPath:   "",
		LayaModelPath: "",
	}
	engine := NewEngine(cfg)
	ctx := context.Background()

	// Contradictory pairs
	tests := []struct {
		f1         string
		f2         string
		contradict bool
	}{
		{"theme: dark mode", "theme: light mode", true},
		{"trend is bullish", "trend is bearish", true},
		{"service is active", "service is inactive", true},
		{"سیستم روشن است", "سیستم خاموش است", true},
		{"User uses Niri", "User uses Niri", false},
	}

	for _, tc := range tests {
		t.Run(tc.f1+"_vs_"+tc.f2, func(t *testing.T) {
			contradicts, conf, err := engine.DecideConflict(ctx, tc.f1, tc.f2)
			if err != nil {
				t.Fatalf("DecideConflict error: %v", err)
			}
			if contradicts != tc.contradict {
				t.Errorf("DecideConflict(%q, %q) = %v; want %v (conf %.2f)", tc.f1, tc.f2, contradicts, tc.contradict, conf)
			}
		})
	}
}

func TestReflect(t *testing.T) {
	cfg := &config.Config{
		LayaURL:       "",
		LayaBinPath:   "",
		LayaModelPath: "",
	}
	engine := NewEngine(cfg)
	ctx := context.Background()

	res, err := engine.Reflect(ctx, "Active working blocks on NixOS and Niri compositor with Pipewire audio")
	if err != nil {
		t.Fatalf("Reflect failed: %v", err)
	}

	if res["cognitive_status"] != "optimal" {
		t.Errorf("expected optimal cognitive status, got %v", res["cognitive_status"])
	}
	if res["focus"] == "" {
		t.Errorf("expected focus to be non-empty")
	}
	if recs, ok := res["recommendations"].([]string); !ok || len(recs) == 0 {
		t.Errorf("expected non-empty recommendations: %v", res["recommendations"])
	}
}

func TestDecide_HTTPPrimary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req["preset"] == "router" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"namespace":  "forex",
				"confidence": 0.99,
				"source":     "laya-http",
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"source": "laya-http",
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		LayaURL:       server.URL,
		LayaBinPath:   "",
		LayaModelPath: "",
	}
	engine := NewEngine(cfg)

	ns, conf, err := engine.RouteQuery(context.Background(), "any query here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ns != "forex" {
		t.Errorf("expected forex from HTTP mock, got %s", ns)
	}
	if conf != 0.99 {
		t.Errorf("expected conf 0.99, got %.2f", conf)
	}
}

func TestDecide_CLIFallback(t *testing.T) {
	// Create mock script for Laya binary
	tmpDir := t.TempDir()
	mockBin := filepath.Join(tmpDir, "mock-laya")
	mockModel := filepath.Join(tmpDir, "mock-model.gguf")

	_ = os.WriteFile(mockModel, []byte("dummy-model"), 0644)
	mockScript := `#!/bin/sh
echo '{"namespace": "ai", "confidence": 0.98, "source": "cli"}'
`
	if err := os.WriteFile(mockBin, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to write mock script: %v", err)
	}

	cfg := &config.Config{
		LayaURL:       "http://127.0.0.1:54321/unreachable", // Unreachable HTTP
		LayaBinPath:   mockBin,
		LayaModelPath: mockModel,
	}
	engine := NewEngine(cfg)

	ns, conf, err := engine.RouteQuery(context.Background(), "something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ns != "ai" {
		t.Errorf("expected ai from CLI mock, got %s", ns)
	}
	if conf != 0.98 {
		t.Errorf("expected conf 0.98, got %.2f", conf)
	}
}
