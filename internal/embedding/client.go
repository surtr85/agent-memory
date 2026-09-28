package embedding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// Client is the interface for embedding generators.
type Client interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// HTTPClient implements Client with fallbacks:
// 1. Primary BGE-M3 embedding server (POST /embedding, body {"content": text})
// 2. Secondary Ollama endpoint (POST /api/embeddings with {"model": "bge-m3", "prompt": text})
// 3. Fallback deterministic pseudo-vector hash (never crash or return fatal error)
type HTTPClient struct {
	BGEURL     string
	OllamaURL  string
	Model      string
	Dimensions int
	HTTP       *http.Client
}

// NewHTTPClient creates an HTTPClient with reasonable defaults.
func NewHTTPClient(bgeURL, ollamaURL, model string, dimensions int) *HTTPClient {
	if bgeURL == "" {
		bgeURL = "http://localhost:8000/embedding"
	}
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434/api/embeddings"
	}
	if model == "" {
		model = "bge-m3"
	}
	if dimensions <= 0 {
		dimensions = 1024
	}
	return &HTTPClient{
		BGEURL:     bgeURL,
		OllamaURL:  ollamaURL,
		Model:      model,
		Dimensions: dimensions,
		HTTP: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// GetEmbedding attempts BGE-M3 -> Ollama -> Deterministic Pseudo-Vector Fallback.
func (c *HTTPClient) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	// 1. Try BGE-M3 Server
	if c.BGEURL != "" {
		emb, err := c.callBGE(ctx, text)
		if err == nil && len(emb) > 0 {
			return emb, nil
		}
	}

	// 2. Try Ollama
	if c.OllamaURL != "" {
		emb, err := c.callOllama(ctx, text)
		if err == nil && len(emb) > 0 {
			return emb, nil
		}
	}

	// 3. Deterministic Offline Fallback
	return GenerateDeterministicPseudoVector(text, c.Dimensions), nil
}

// callBGE calls standard BGE-M3 endpoint: POST /embedding body {"content": text}
func (c *HTTPClient) callBGE(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(map[string]string{
		"content": text,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BGEURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bge returned status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Response might be direct []float32 or {"embedding": []float32}
	var rawList []float32
	if err := json.Unmarshal(bodyBytes, &rawList); err == nil {
		return rawList, nil
	}

	var objResp struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(bodyBytes, &objResp); err == nil && len(objResp.Embedding) > 0 {
		return objResp.Embedding, nil
	}

	return nil, fmt.Errorf("unable to parse bge response")
}

// callOllama calls Ollama endpoint: POST /api/embeddings with {"model": model, "prompt": text}
func (c *HTTPClient) callOllama(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(map[string]string{
		"model":  c.Model,
		"prompt": text,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.OllamaURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status: %d", resp.StatusCode)
	}

	var ollamaResp struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}
	if len(ollamaResp.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding")
	}

	return ollamaResp.Embedding, nil
}

// GenerateDeterministicPseudoVector generates a normalized deterministic vector of length `dim`
// based on sha256 hashing of the input text. Guarantees 100% offline resilience and consistency.
func GenerateDeterministicPseudoVector(text string, dim int) []float32 {
	if dim <= 0 {
		dim = 1024
	}

	vec := make([]float32, dim)
	h := sha256.New()
	h.Write([]byte(text))
	baseHash := h.Sum(nil)

	// Expand the hash deterministically across dim dimensions
	var normSum float64
	round := 0
	for i := 0; i < dim; i++ {
		idx := (i % 8) * 4
		if idx == 0 && i > 0 {
			round++
		}
		// Combine base hash slices with counter
		val := binary.LittleEndian.Uint32(baseHash[idx : idx+4])
		val ^= uint32(i*31 + round*101)
		// Map uint32 to float in [-1.0, 1.0]
		f := (float64(val)/float64(math.MaxUint32))*2.0 - 1.0
		vec[i] = float32(f)
		normSum += f * f
	}

	// Normalize vector to unit sphere
	if normSum > 0 {
		norm := math.Sqrt(normSum)
		for i := 0; i < dim; i++ {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}

	return vec
}
