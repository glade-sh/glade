package lsp

import (
	"fmt"
	"strings"
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
	end := start
	if positionNotBefore(change.Range.End, change.Range.Start) {
		// The end is found by scanning on from the start, not from byte zero.
		end, err = offsetFromPosition(text, start, change.Range.Start, change.Range.End)
	} else {
		end, err = offsetForPosition(text, change.Range.End)
	}
	if err != nil {
		return "", err
	}
	if end < start {
		return "", fmt.Errorf("invalid text edit range")
	}
	return text[:start] + change.Text + text[end:], nil
}

func positionNotBefore(a, b Position) bool {
	return a.Line > b.Line || (a.Line == b.Line && a.Character >= b.Character)
}

func offsetForPosition(text string, position Position) (int, error) {
	return offsetFromPosition(text, 0, Position{}, position)
}

// offsetFromPosition returns the byte offset of position, scanning from the
// known point (offset, from). Lines end at \n, \r\n or a lone \r, and
// characters count UTF-16 code units. from must not be after position.
func offsetFromPosition(text string, offset int, from, position Position) (int, error) {
	if position.Line < 0 || position.Character < 0 || from.Line < 0 || from.Character < 0 {
		return 0, fmt.Errorf("invalid text position")
	}
	line, character := from.Line, from.Character
	for line < position.Line {
		i := strings.IndexAny(text[offset:], "\r\n")
		if i < 0 {
			return 0, fmt.Errorf("position past end of document")
		}
		offset += i
		if text[offset] == '\r' && offset+1 < len(text) && text[offset+1] == '\n' {
			offset += 2
		} else {
			offset++
		}
		line++
		character = 0
	}
	for offset < len(text) {
		if character == position.Character {
			return offset, nil
		}
		c := text[offset]
		if c == '\r' || c == '\n' {
			return 0, fmt.Errorf("position past end of line")
		}
		if c < utf8.RuneSelf {
			character++
			offset++
			continue
		}
		r, size := utf8.DecodeRuneInString(text[offset:])
		width := 1
		if r > 0xFFFF {
			width = 2
		}
		if character < position.Character && character+width > position.Character {
			return 0, fmt.Errorf("position splits UTF-16 surrogate pair")
		}
		character += width
		offset += size
	}
	if character == position.Character {
		return len(text), nil
	}
	return 0, fmt.Errorf("position past end of document")
}
