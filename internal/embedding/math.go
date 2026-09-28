package embedding

import (
	"encoding/binary"
	"fmt"
	"math"
)

// CosineSimilarity computes the cosine similarity between two float32 vectors.
// Returns 0 if dimensions mismatch or vector norms are zero.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		valA := float64(a[i])
		valB := float64(b[i])
		dot += valA * valB
		normA += valA * valA
		normB += valB * valB
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// SerializeEmbedding serializes a float32 slice to binary little-endian format.
func SerializeEmbedding(emb []float32) []byte {
	buf := make([]byte, len(emb)*4)
	for i, v := range emb {
		binary.LittleEndian.PutUint32(buf[i*4:(i+1)*4], math.Float32bits(v))
	}
	return buf
}

// DeserializeEmbedding deserializes a binary little-endian slice to float32 slice.
func DeserializeEmbedding(data []byte) ([]float32, error) {
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("invalid embedding byte slice length %d (must be multiple of 4)", len(data))
	}

	n := len(data) / 4
	res := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
		res[i] = math.Float32frombits(bits)
	}
	return res, nil
}
