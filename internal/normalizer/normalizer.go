package normalizer

import (
	"strings"
	"unicode"
)

// Mapping of Persian/Arabic digits to ASCII '0'-'9'
var digitMap = map[rune]rune{
	// Persian digits
	'۰': '0', '۱': '1', '۲': '2', '۳': '3', '۴': '4',
	'۵': '5', '۶': '6', '۷': '7', '۸': '8', '۹': '9',
	// Arabic digits
	'٠': '0', '١': '1', '٢': '2', '٣': '3', '٤': '4',
	'٥': '5', '٦': '6', '٧': '7', '٨': '8', '٩': '9',
}

// Arabic and Persian diacritics / tashkeel / tanwin to strip
func isDiacritic(r rune) bool {
	switch r {
	case '\u064B', // Fathatan ً
		'\u064C', // Dammatan ٌ
		'\u064D', // Kasratan ٍ
		'\u064E', // Fatha َ
		'\u064F', // Damma ُ
		'\u0650', // Kasra ِ
		'\u0651', // Shadda ّ
		'\u0652', // Sukun ْ
		'\u0653', // Maddah above ٓ
		'\u0654', // Hamza above ٔ
		'\u0655', // Hamza below ٕ
		'\u0670': // Superscript alef ٰ
		return true
	default:
		return false
	}
}

// Normalize applies multilingual Persian/Arabic text normalization:
// - Converts Arabic Yeh (ي, ى) -> Persian ی
// - Converts Arabic Kaf (ك) -> Persian ک
// - Standardizes or retains ZWNJ (\u200c) appropriately
// - Strips diacritics/tashkeel/tanwin (َ, ِ, ُ, ً, ٍ, ٌ, ّ, ْ)
// - Normalizes Persian/Arabic digits (۰-۹, ٠-٩) to ASCII 0-9
func Normalize(text string) string {
	if text == "" {
		return ""
	}

	var sb strings.Builder
	sb.Grow(len(text))

	for _, r := range text {
		// Strip diacritics
		if isDiacritic(r) {
			continue
		}

		// Normalize digits
		if d, ok := digitMap[r]; ok {
			sb.WriteRune(d)
			continue
		}

		// Arabic Yeh variants to Persian Yeh
		// \u064A is Arabic letter Yeh (ي)
		// \u0649 is Arabic letter Alef Maksura (ى)
		// \u06CC is Farsi Yeh (ی)
		if r == '\u064A' || r == '\u0649' {
			sb.WriteRune('\u06CC')
			continue
		}

		// Arabic Kaf to Persian Kaf
		// \u0643 is Arabic Kaf (ك)
		// \u06A9 is Persian Keheh (ک)
		if r == '\u0643' {
			sb.WriteRune('\u06A9')
			continue
		}

		// Standardize multiple ZWNJs or keep single ZWNJ
		if r == '\u200C' {
			sb.WriteRune('\u200C')
			continue
		}

		sb.WriteRune(r)
	}

	return strings.TrimSpace(sb.String())
}

// Tokenize normalizes the text and returns lowercased non-empty tokens,
// splitting on whitespace and punctuation (treating ZWNJ as a separator or token boundary).
func Tokenize(text string) []string {
	normalized := Normalize(text)
	if normalized == "" {
		return []string{}
	}

	isSeparator := func(r rune) bool {
		// ZWNJ can be treated as separator for tokenization
		if r == '\u200C' {
			return true
		}
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	}

	fields := strings.FieldsFunc(normalized, isSeparator)
	var tokens []string
	for _, f := range fields {
		token := strings.ToLower(strings.TrimSpace(f))
		if token != "" {
			tokens = append(tokens, token)
		}
	}

	if tokens == nil {
		return []string{}
	}
	return tokens
}
