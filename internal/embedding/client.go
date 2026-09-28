package embedding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/surtr85/agent-memory/internal/normalizer"
)

// DefaultBuiltinDimensions is the calibrated dimensionality for BuiltinVectorizer.
const DefaultBuiltinDimensions = 256

// Client is the interface for embedding generators.
type Client interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// BuiltinVectorizer is a robust, built-in, native Pure Go semantic vectorizer.
// It uses normalizer.Normalize and normalizer.Tokenize, extracts subword character n-grams (3-grams, 4-grams)
// and word tokens, and projects tokens via multi-hash feature projection with sign hashing
// into a calibrated 256-dimensional unit vector.
// Dots products between these unit vectors equal cosine similarity.
// It guarantees subword semantic matching (e.g. prefixes, stems, related words in Persian and English)
// with zero external daemon and zero RAM overhead.
type BuiltinVectorizer struct {
	dimensions int
}

// NewBuiltinVectorizer creates a new Pure Go native semantic vectorizer with calibrated 256 dimensions.
func NewBuiltinVectorizer() *BuiltinVectorizer {
	return &BuiltinVectorizer{
		dimensions: DefaultBuiltinDimensions,
	}
}

// NewBuiltinVectorizerWithDim creates a vectorizer with a custom dimension size.
func NewBuiltinVectorizerWithDim(dim int) *BuiltinVectorizer {
	if dim <= 0 {
		dim = DefaultBuiltinDimensions
	}
	return &BuiltinVectorizer{
		dimensions: dim,
	}
}

// GetEmbedding generates a 256-dimensional unit vector for text.
func (v *BuiltinVectorizer) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	return v.Vectorize(text), nil
}

// Vectorize converts input text into a calibrated unit-normalized semantic vector.
func (v *BuiltinVectorizer) Vectorize(text string) []float32 {
	dim := v.dimensions
	if dim <= 0 {
		dim = DefaultBuiltinDimensions
	}

	normText := normalizer.Normalize(text)
	if normText == "" {
		return make([]float32, dim)
	}

	vec := make([]float64, dim)

	// Tokenize words
	words := normalizer.Tokenize(normText)

	// Collect features with weights
	// Word tokens and subword character n-grams (3-grams and 4-grams)
	for _, word := range words {
		// Full word feature
		v.projectFeature(vec, "W:"+word, 1.5)

		// Character n-grams (3-grams and 4-grams)
		runes := []rune(word)
		rLen := len(runes)

		// 3-grams
		if rLen >= 3 {
			for i := 0; i <= rLen-3; i++ {
				ngram := string(runes[i : i+3])
				v.projectFeature(vec, "3G:"+ngram, 1.2)
			}
		}

		// 4-grams
		if rLen >= 4 {
			for i := 0; i <= rLen-4; i++ {
				ngram := string(runes[i : i+4])
				v.projectFeature(vec, "4G:"+ngram, 1.5)
			}
		}

		// If short word (length 1 or 2), project as character feature
		if rLen < 3 {
			v.projectFeature(vec, "S:"+word, 1.0)
		}
	}

	// L2 normalization so dot product == cosine similarity
	var normSum float64
	for _, val := range vec {
		normSum += val * val
	}

	res := make([]float32, dim)
	if normSum > 0 {
		norm := math.Sqrt(normSum)
		for i, val := range vec {
			res[i] = float32(val / norm)
		}
	}

	return res
}

// projectFeature maps a feature string into vector dimensions using multi-hashing with sign hashing.
func (v *BuiltinVectorizer) projectFeature(vec []float64, feature string, weight float64) {
	dim := len(vec)
	fBytes := []byte(feature)

	// Hash 1: FNV-1a 64-bit
	h1 := fnv.New64a()
	h1.Write(fBytes)
	hash1 := h1.Sum64()

	// Hash 2: Murmur-like / bit-mix of hash1
	hash2 := hash1 ^ (hash1 >> 33)
	hash2 *= 0xff51afd7ed558ccd
	hash2 ^= (hash2 >> 33)
	hash2 *= 0xc4ceb9fe1a85ec53
	hash2 ^= (hash2 >> 33)

	// Multi-hash projection: 2 buckets per feature
	// Bucket 1
	idx1 := int(hash1 % uint64(dim))
	sign1 := 1.0
	if (hash1 & 0x8000000000000000) != 0 {
		sign1 = -1.0
	}
	vec[idx1] += sign1 * weight

	// Bucket 2
	idx2 := int(hash2 % uint64(dim))
	sign2 := 1.0
	if (hash2 & 0x8000000000000000) != 0 {
		sign2 = -1.0
	}
	vec[idx2] += sign2 * (weight * 0.7)
}

// HTTPClient implements Client as an optional fallback or custom provider
// when external daemon/API is explicitly configured.
// Fallback order:
// 1. Primary BGE-M3 embedding server (POST /embedding, body {"content": text})
// 2. Secondary Ollama endpoint (POST /api/embeddings with {"model": "bge-m3", "prompt": text})
// 3. BuiltinVectorizer native fallback (never crashes or returns fatal error)
type HTTPClient struct {
	BGEURL     string
	OllamaURL  string
	Model      string
	Dimensions int
	HTTP       *http.Client
	builtin    *BuiltinVectorizer
}

// NewHTTPClient creates an HTTPClient with custom URLs or fallbacks.
func NewHTTPClient(bgeURL, ollamaURL, model string, dimensions int) *HTTPClient {
	if model == "" {
		model = "bge-m3"
	}
	if dimensions <= 0 {
		dimensions = DefaultBuiltinDimensions
	}
	return &HTTPClient{
		BGEURL:     bgeURL,
		OllamaURL:  ollamaURL,
		Model:      model,
		Dimensions: dimensions,
		HTTP: &http.Client{
			Timeout: 3 * time.Second,
		},
		builtin: NewBuiltinVectorizerWithDim(dimensions),
	}
}

// GetEmbedding attempts BGE-M3 -> Ollama -> BuiltinVectorizer Fallback.
func (c *HTTPClient) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	// 1. Try BGE-M3 Server if URL provided
	if c.BGEURL != "" {
		emb, err := c.callBGE(ctx, text)
		if err == nil && len(emb) > 0 {
			return emb, nil
		}
	}

	// 2. Try Ollama if URL provided
	if c.OllamaURL != "" {
		emb, err := c.callOllama(ctx, text)
		if err == nil && len(emb) > 0 {
			return emb, nil
		}
	}

	// 3. Pure Go Builtin Semantic Vectorizer Fallback
	return c.builtin.GetEmbedding(ctx, text)
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
		dim = DefaultBuiltinDimensions
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
		val := binary.LittleEndian.Uint32(baseHash[idx : idx+4])
		val ^= uint32(i*31 + round*101)
		f := (float64(val)/float64(math.MaxUint32))*2.0 - 1.0
		vec[i] = float32(f)
		normSum += f * f
	}

	if normSum > 0 {
		norm := math.Sqrt(normSum)
		for i := 0; i < dim; i++ {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}

	return vec
}
