package lsp

import (
	"fmt"
	"unicode/utf8"
)

func applyTextChange(text string, change TextDocumentContentChangeEvent) (string, error) {
	if change.Range == nil {
		return change.Text, nil
	}
	start, err := offsetForPosition(text, change.Range.Start)
	if err != nil {
		return "", err
	}
	end, err := offsetForPosition(text, change.Range.End)
	if err != nil {
		return "", err
	}
	if end < start {
		return "", fmt.Errorf("invalid text edit range")
	}
	return text[:start] + change.Text + text[end:], nil
}

func offsetForPosition(text string, position Position) (int, error) {
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
