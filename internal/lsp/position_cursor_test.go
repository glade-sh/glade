package lsp

import (
	"fmt"
	"strings"
	"testing"
)

func TestPositionCursorMatchesPositionForOffset(t *testing.T) {
	for _, text := range offsetTestTexts() {
		cursor := positionCursor{text: text}
		for target := 0; target <= len(text)+2; target++ {
			if got, want := cursor.positionAt(target), positionForOffset(text, target); got != want {
				t.Fatalf("text %q ascending offset %d: got %+v, want %+v", text, target, got, want)
			}
		}
		// Repeats, steps back and jumps must also agree.
		for _, target := range []int{len(text), 0, len(text) / 2, len(text) / 2, 1, len(text), 2, 2, 3} {
			if got, want := cursor.positionAt(target), positionForOffset(text, target); got != want {
				t.Fatalf("text %q offset %d after jumps: got %+v, want %+v", text, target, got, want)
			}
		}
	}
}

// referenceWordRanges converts every match from byte zero, as wordRanges once did.
func referenceWordRanges(text, word string) []Range {
	var ranges []Range
	lowerText, lowerWord := strings.ToLower(text), strings.ToLower(word)
	for offset := 0; offset < len(text); {
		next := strings.Index(lowerText[offset:], lowerWord)
		if next < 0 {
			break
		}
		start, end := offset+next, offset+next+len(lowerWord)
		if isIdentifierBoundary(text, start-1) && isIdentifierBoundary(text, end) {
			ranges = append(ranges, rangeForOffsets(text, start, end))
		}
		offset = end
	}
	return ranges
}

func TestWordRangesMatchesPerMatchScan(t *testing.T) {
	text := "Foo foo\r\nxFOOx FOO\n\U0001F600 foo\rfoo\u00e9 foo"
	want := referenceWordRanges(text, "Foo")
	got := wordRanges(text, "Foo")
	if len(want) == 0 || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("ranges = %v, want %v", got, want)
	}
}

func BenchmarkWordRangesCommonWord(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 4000; i++ {
		fmt.Fprintf(&sb, "    Integer value = value + %d; // café value\r\n", i)
	}
	text := sb.String()
	for _, mode := range []string{"reference-per-match-scan", "current"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				find := wordRanges
				if mode != "current" {
					find = referenceWordRanges
				}
				if len(find(text, "value")) == 0 {
					b.Fatal("no matches")
				}
			}
		})
	}
}
