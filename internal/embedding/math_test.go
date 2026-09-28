package embedding

import (
	"math"
	"reflect"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0},
			b:        []float32{0.0, 1.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 0.0},
			b:        []float32{-1.0, 0.0},
			expected: -1.0,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0.0,
		},
		{
			name:     "dimension mismatch",
			a:        []float32{1.0},
			b:        []float32{1.0, 2.0},
			expected: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CosineSimilarity(tc.a, tc.b)
			if math.Abs(float64(got-tc.expected)) > 1e-5 {
				t.Errorf("CosineSimilarity(%v, %v) = %f; expected %f", tc.a, tc.b, got, tc.expected)
			}
		})
	}
}

func TestSerializeDeserializeEmbedding(t *testing.T) {
	orig := []float32{0.12345, -0.6789, 1.0, 0.0, -1.0, 3.141592}
	data := SerializeEmbedding(orig)

	if len(data) != len(orig)*4 {
		t.Fatalf("expected serialized length %d, got %d", len(orig)*4, len(data))
	}

	deser, err := DeserializeEmbedding(data)
	if err != nil {
		t.Fatalf("DeserializeEmbedding failed: %v", err)
	}

	if !reflect.DeepEqual(orig, deser) {
		t.Fatalf("deserialized vector %v != original vector %v", deser, orig)
	}

	// Corrupted data (not multiple of 4)
	_, err = DeserializeEmbedding([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for corrupted byte length, got nil")
	}
}

func TestBuiltinVectorizer_DotProductEqualsCosineSimilarity(t *testing.T) {
	v := NewBuiltinVectorizer()
	vec1 := v.Vectorize("نرم‌افزار مدیریت حافظه برای هوش مصنوعی")
	vec2 := v.Vectorize("سامانه شناختی حافظه دائمی هوش مصنوعی")

	// Calculate cosine similarity via CosineSimilarity
	cosSim := CosineSimilarity(vec1, vec2)

	// Calculate direct dot product
	var dot float32
	for i := 0; i < len(vec1); i++ {
		dot += vec1[i] * vec2[i]
	}

	diff := math.Abs(float64(cosSim - dot))
	if diff > 1e-5 {
		t.Fatalf("expected dot product (%f) to equal cosine similarity (%f), diff %e", dot, cosSim, diff)
	}
}
