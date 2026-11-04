package soql

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// case332OriginalLikePattern is the unchanged pre-optimization matcher.
func case332OriginalLikePattern(text, pattern string) bool {
	// Tokenize escaped LIKE wildcards before matching. SOQL string decoding
	// keeps these markers so a literal backslash remains distinct from \_ / \%.
	type patternToken struct {
		kind  byte // '%', '_', or literal
		value byte
	}
	tokens := make([]patternToken, 0, len(pattern))
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) && (pattern[i+1] == '\\' || pattern[i+1] == '_' || pattern[i+1] == '%') {
			i++
			tokens = append(tokens, patternToken{value: pattern[i]})
			continue
		}
		switch pattern[i] {
		case '%', '_':
			tokens = append(tokens, patternToken{kind: pattern[i]})
		default:
			tokens = append(tokens, patternToken{value: pattern[i]})
		}
	}
	// Dynamic programming over text bytes and the tokenized LIKE pattern.
	m, n := len(text), len(tokens)
	// dp[i][j] = true if text[i:] matches pattern[j:]
	// Use two rows to keep O(n) space.
	prev := make([]bool, n+1)
	curr := make([]bool, n+1)
	prev[n] = true
	for j := n - 1; j >= 0; j-- {
		if tokens[j].kind == '%' {
			prev[j] = prev[j+1]
		} else {
			prev[j] = false
		}
	}
	for i := m - 1; i >= 0; i-- {
		curr[n] = false
		for j := n - 1; j >= 0; j-- {
			switch tokens[j].kind {
			case '%':
				curr[j] = curr[j+1] || prev[j]
			case '_':
				curr[j] = prev[j+1]
			default:
				curr[j] = (asciiLower(text[i]) == asciiLower(tokens[j].value)) && prev[j+1]
			}
		}
		prev, curr = curr, prev
	}
	return prev[0]
}

func TestCase332LikeAgainstOriginal(t *testing.T) {
	scratch := &predicateScratch{}
	for _, pattern := range []string{"", "%", "_", `a\_b`, `a\%b`, `a\\b`, "ab%_", "abc\\", "\xff%", strings.Repeat("x", 4097)} {
		for _, value := range []string{"", "a", "a_b", "a%b", `a\b`, "ABCxy", "\xffx", strings.Repeat("x", 8)} {
			got := likeMatchWithScratch(storage.StringValue(value), storage.StringValue(pattern), scratch)
			want := case332OriginalLikePattern(value, pattern)
			if got != want {
				t.Fatalf("value %q pattern %q: got %v want %v", value, pattern, got, want)
			}
		}
	}
}
