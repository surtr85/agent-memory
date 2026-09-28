package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	// Both unreachable
	client := NewHTTPClient("http://127.0.0.1:1/invalid", "http://127.0.0.1:1/invalid", "bge-m3", 64)
	emb, err := client.GetEmbedding(context.Background(), "offline test")
	if err != nil {
		t.Fatalf("offline fallback should not return error, got: %v", err)
	}

	if len(emb) != 64 {
		t.Fatalf("expected fallback vector length 64, got %d", len(emb))
	}
}
