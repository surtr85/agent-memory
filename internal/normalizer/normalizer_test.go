package normalizer

import (
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Arabic Yeh and Kaf conversion",
			input:    "كتاب علي ومصطفى",
			expected: "کتاب علی ومصطفی",
		},
		{
			name:     "Diacritics removal",
			input:    "کِتَابٌ جَمِیلٌ وَمُفِیدّ",
			expected: "کتاب جمیل ومفید",
		},
		{
			name:     "Persian and Arabic digits",
			input:    "شماره ۱۲۳ و رقم ٤٥٦",
			expected: "شماره 123 و رقم 456",
		},
		{
			name:     "ZWNJ preservation",
			input:    "می\u200cخواهم بروم",
			expected: "می\u200cخواهم بروم",
		},
		{
			name:     "English mixed with Persian and diacritics",
			input:    "Hello عَالَم! Code: ۱۲۳",
			expected: "Hello عالم! Code: 123",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Normalize(tc.input)
			if got != tc.expected {
				t.Errorf("Normalize(%q) = %q; expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Persian sentence with Arabic letters and punctuation",
			input:    "كتابِ علي، شمارهٔ ۱۲۳ است!",
			expected: []string{"کتاب", "علی", "شماره", "123", "است"},
		},
		{
			name:     "English mixed sentence",
			input:    "AgentMemory Universal (v3.0) rocks!",
			expected: []string{"agentmemory", "universal", "v3", "0", "rocks"},
		},
		{
			name:     "ZWNJ token splitting",
			input:    "می\u200cدانم",
			expected: []string{"می", "دانم"},
		},
		{
			name:     "Empty input",
			input:    "   ",
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokenize(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("Tokenize(%q) = %v; expected %v", tc.input, got, tc.expected)
			}
		})
	}
}
