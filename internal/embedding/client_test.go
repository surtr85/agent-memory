package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuiltinVectorizer_Basic(t *testing.T) {
	v := NewBuiltinVectorizer()
	ctx := context.Background()

	emb1, err := v.GetEmbedding(ctx, "کتابخانه مدرن برای هوش مصنوعی")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(emb1) != DefaultBuiltinDimensions {
		t.Fatalf("expected dimensions %d, got %d", DefaultBuiltinDimensions, len(emb1))
	}

	// Test unit normalization: dot product with itself must be ~1.0
	dotSelf := CosineSimilarity(emb1, emb1)
	if dotSelf < 0.999 || dotSelf > 1.001 {
		t.Fatalf("expected unit normalization dot(v,v) ~ 1.0, got %f", dotSelf)
	}
}

func TestBuiltinVectorizer_SubwordSemanticMatching(t *testing.T) {
	v := NewBuiltinVectorizer()
	ctx := context.Background()

	// 1. English prefixes / stems / related words
	embRun, _ := v.GetEmbedding(ctx, "running fast")
	embRunner, _ := v.GetEmbedding(ctx, "fast runner")
	embCar, _ := v.GetEmbedding(ctx, "astronomy galaxies")

	simRunning := CosineSimilarity(embRun, embRunner)
	simUnrelated := CosineSimilarity(embRun, embCar)

	if simRunning <= simUnrelated {
		t.Fatalf("expected related terms (%f) to have higher similarity than unrelated (%f)", simRunning, simUnrelated)
	}
	if simRunning < 0.3 {
		t.Fatalf("expected substantial subword similarity between running and runner, got %f", simRunning)
	}

	// 2. Persian subwords / stems
	// کتابخانه (library) and کتابدار (librarian) share root stem کتاب (book)
	embKetabkhane, _ := v.GetEmbedding(ctx, "کتابخانه")
	embKetabdar, _ := v.GetEmbedding(ctx, "کتابدار")
	embAseman, _ := v.GetEmbedding(ctx, "کهکشان آسمان")

	simPersianStem := CosineSimilarity(embKetabkhane, embKetabdar)
	simPersianDiff := CosineSimilarity(embKetabkhane, embAseman)

	if simPersianStem <= simPersianDiff {
		t.Fatalf("persian stems should have higher similarity: stem=%f, diff=%f", simPersianStem, simPersianDiff)
	}
	if simPersianStem < 0.25 {
		t.Fatalf("expected subword overlap for persian words sharing stem, got %f", simPersianStem)
	}
}

func TestDeterministicPseudoVector(t *testing.T) {
	v1 := GenerateDeterministicPseudoVector("test sentence", 128)
	v2 := GenerateDeterministicPseudoVector("test sentence", 128)
	v3 := GenerateDeterministicPseudoVector("another sentence", 128)

	if len(v1) != 128 {
		t.Fatalf("expected length 128, got %d", len(v1))
	}

	simSame := CosineSimilarity(v1, v2)
	if simSame < 0.9999 {
		t.Fatalf("expected identical pseudo-vectors to have sim ~1.0, got %f", simSame)
	}

	simDiff := CosineSimilarity(v1, v3)
	if simDiff > 0.95 {
		t.Fatalf("different sentences should not be nearly identical, got %f", simDiff)
	}
}

func TestHTTPClient_BGE(t *testing.T) {
	expectedEmbedding := []float32{0.1, 0.2, 0.3}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embedding" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedEmbedding)
	}))
	defer ts.Close()

	client := NewHTTPClient(ts.URL+"/embedding", "", "bge-m3", 3)
	emb, err := client.GetEmbedding(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(emb) != 3 || emb[0] != 0.1 || emb[1] != 0.2 || emb[2] != 0.3 {
		t.Fatalf("unexpected embedding returned: %v", emb)
	}
}

func TestHTTPClient_OllamaFallback(t *testing.T) {
	expectedEmbedding := []float32{0.4, 0.5, 0.6}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/embeddings" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"embedding": expectedEmbedding,
			})
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	// bgeURL points to invalid port, ollamaURL points to mock server
	client := NewHTTPClient("http://127.0.0.1:1/invalid", ts.URL+"/api/embeddings", "bge-m3", 3)
	emb, err := client.GetEmbedding(context.Background(), "test ollama")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(emb) != 3 || emb[0] != 0.4 || emb[1] != 0.5 || emb[2] != 0.6 {
		t.Fatalf("unexpected embedding returned: %v", emb)
	}
}

func TestHTTPClient_OfflineFallback(t *testing.T) {
	// Both unreachable -> BuiltinVectorizer fallback
	client := NewHTTPClient("http://127.0.0.1:1/invalid", "http://127.0.0.1:1/invalid", "bge-m3", 64)
	emb, err := client.GetEmbedding(context.Background(), "offline test")
	if err != nil {
		t.Fatalf("offline fallback should not return error, got: %v", err)
	}

	if len(emb) != 64 {
		t.Fatalf("expected fallback vector length 64, got %d", len(emb))
	}
}
