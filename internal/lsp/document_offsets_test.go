package lsp

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// referenceOffsetForPosition is the original rune-by-rune scan from byte zero.
func referenceOffsetForPosition(text string, position Position) (int, error) {
	if position.Line < 0 || position.Character < 0 {
		return 0, fmt.Errorf("invalid text position")
	}
	line := 0
	character := 0
	for offset := 0; offset < len(text); {
		if line == position.Line && character == position.Character {
			return offset, nil
		}
		r, size := utf8.DecodeRuneInString(text[offset:])
		if r == '\r' || r == '\n' {
			if line == position.Line {
				return 0, fmt.Errorf("position past end of line")
			}
			line++
			character = 0
			offset += size
			if r == '\r' && offset < len(text) && text[offset] == '\n' {
				offset++
			}
			continue
		}
		width := 1
		if r > 0xFFFF {
			width = 2
		}
		if line == position.Line && character < position.Character && character+width > position.Character {
			return 0, fmt.Errorf("position splits UTF-16 surrogate pair")
		}
		character += width
		offset += size
	}
	if line == position.Line && character == position.Character {
		return len(text), nil
	}
	return 0, fmt.Errorf("position past end of document")
}

func offsetTestTexts() []string {
	return []string{
		"",
		"a",
		"abc\ndef\n",
		"abc\r\ndef\r\n\r\nx",
		"a\rb\rc\r",
		"a\r\n\nb\n\r",
		"héllo\n\U0001F600 smile \U0001F600\nend",
		"tab\there\r\n\U0001F600\r\n",
		"bad\xffbyte\n\xc3 half\nx",
		"\n\n\n",
		"trailing newline\n",
	}
}

func TestOffsetForPositionMatchesReferenceScan(t *testing.T) {
	for _, text := range offsetTestTexts() {
		lines := strings.Count(text, "\n") + strings.Count(text, "\r") + 2
		for line := -1; line <= lines+1; line++ {
			for character := -1; character <= utf8.RuneCountInString(text)*2+3; character++ {
				position := Position{Line: line, Character: character}
				want, wantErr := referenceOffsetForPosition(text, position)
				got, gotErr := offsetForPosition(text, position)
				if got != want || (gotErr == nil) != (wantErr == nil) || (wantErr != nil && gotErr.Error() != wantErr.Error()) {
					t.Fatalf("text %q position %+v: got (%d, %v), want (%d, %v)", text, position, got, gotErr, want, wantErr)
				}
			}
		}
	}
}

func TestApplyTextChangeMatchesReferenceForEveryRange(t *testing.T) {
	for _, text := range offsetTestTexts() {
		var positions []Position
		for line := 0; line < strings.Count(text, "\n")+strings.Count(text, "\r")+2; line++ {
			for character := 0; character <= 14; character++ {
				positions = append(positions, Position{Line: line, Character: character})
			}
		}
		for _, start := range positions {
			for _, end := range positions {
				change := TextDocumentContentChangeEvent{Range: &Range{Start: start, End: end}, Text: "<>"}
				got, gotErr := applyTextChange(text, change)
				want, wantErr := referenceApplyTextChange(text, change)
				if got != want || (gotErr == nil) != (wantErr == nil) {
					t.Fatalf("text %q range %+v: got (%q, %v), want (%q, %v)", text, change.Range, got, gotErr, want, wantErr)
				}
			}
		}
	}
}

func referenceApplyTextChange(text string, change TextDocumentContentChangeEvent) (string, error) {
	start, err := referenceOffsetForPosition(text, change.Range.Start)
	if err != nil {
		return "", err
	}
	end, err := referenceOffsetForPosition(text, change.Range.End)
	if err != nil {
		return "", err
	}
	if end < start {
		return "", fmt.Errorf("invalid text edit range")
	}
	return text[:start] + change.Text + text[end:], nil
}

func BenchmarkApplyTextChangeNearEnd(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 60000; i++ {
		fmt.Fprintf(&sb, "    Integer value%d = %d; // café\r\n", i, i)
	}
	text := sb.String()
	change := TextDocumentContentChangeEvent{Range: &Range{Start: Position{Line: 59990, Character: 4}, End: Position{Line: 59990, Character: 11}}, Text: "Long"}
	for _, mode := range []string{"reference-offsets", "current"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var err error
				if mode == "current" {
					_, err = applyTextChange(text, change)
				} else {
					_, err = referenceApplyTextChange(text, change)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
