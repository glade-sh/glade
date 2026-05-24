package lsp

import (
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

const (
	semanticTokenClass = iota
	semanticTokenInterface
	semanticTokenEnum
	semanticTokenMethod
	semanticTokenProperty
	semanticTokenVariable
	semanticTokenEvent
)

type semanticToken struct {
	Line  int
	Start int
	Len   int
	Type  int
}

func semanticTokensLegend() SemanticTokensLegend {
	return SemanticTokensLegend{
		TokenTypes: []string{"class", "interface", "enum", "method", "property", "variable", "event"},
	}
}

func semanticTokenFromRange(name string, r diagnostic.Range, tokenType int) semanticToken {
	pos := toLSPPosition(r.Start)
	return semanticToken{Line: pos.Line, Start: pos.Character, Len: utf16Len(name), Type: tokenType}
}

func semanticTokenType(kind apexast.DeclarationKind) int {
	switch kind {
	case apexast.DeclarationClass:
		return semanticTokenClass
	case apexast.DeclarationInterface:
		return semanticTokenInterface
	case apexast.DeclarationEnum:
		return semanticTokenEnum
	case apexast.DeclarationMethod, apexast.DeclarationConstructor:
		return semanticTokenMethod
	case apexast.DeclarationField:
		return semanticTokenVariable
	case apexast.DeclarationProperty:
		return semanticTokenProperty
	default:
		return semanticTokenVariable
	}
}

func encodeSemanticTokens(tokens []semanticToken) []int {
	data := make([]int, 0, len(tokens)*5)
	prevLine, prevStart := 0, 0
	for i, token := range tokens {
		deltaLine := token.Line - prevLine
		deltaStart := token.Start
		if i > 0 && deltaLine == 0 {
			deltaStart = token.Start - prevStart
		}
		data = append(data, deltaLine, deltaStart, token.Len, token.Type, 0)
		prevLine, prevStart = token.Line, token.Start
	}
	return data
}

func (h *Handler) definitionForName(name string) (Location, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return Location{}, false
	}
	for _, typ := range h.index.Types {
		if strings.EqualFold(typ.Name, name) {
			return Location{URI: uriFromPath(typ.File), Range: toLSPRange(typ.Range)}, true
		}
		for _, member := range typ.Members {
			if strings.EqualFold(member.Name, name) || strings.EqualFold(typ.Name+"."+member.Name, name) {
				return Location{URI: uriFromPath(typ.File), Range: toLSPRange(member.Range)}, true
			}
		}
	}
	for _, trigger := range h.index.Triggers {
		if strings.EqualFold(trigger.Name, name) {
			return Location{URI: uriFromPath(trigger.File), Range: toLSPRange(trigger.Range)}, true
		}
	}
	for _, object := range h.index.Objects {
		if strings.EqualFold(object.Name, name) {
			return Location{URI: schemaURI(object.Name), Range: Range{}}, true
		}
		for _, field := range object.Fields {
			if strings.EqualFold(field.Name, name) || strings.EqualFold(object.Name+"."+field.Name, name) {
				return Location{URI: schemaURI(object.Name, "fields", field.Name), Range: Range{}}, true
			}
		}
	}
	return Location{}, false
}

func (h *Handler) referenceLocations(name string) []Location {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	var locations []Location
	for _, file := range h.sourceFiles() {
		text, ok := h.documentText(file)
		if !ok {
			continue
		}
		for _, r := range wordRanges(text, name) {
			locations = append(locations, Location{URI: uriFromPath(file), Range: r})
		}
	}
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].URI == locations[j].URI {
			if locations[i].Range.Start.Line == locations[j].Range.Start.Line {
				return locations[i].Range.Start.Character < locations[j].Range.Start.Character
			}
			return locations[i].Range.Start.Line < locations[j].Range.Start.Line
		}
		return locations[i].URI < locations[j].URI
	})
	return locations
}

func (h *Handler) sourceFiles() []string {
	seen := make(map[string]bool)
	var files []string
	add := func(file string) {
		if file == "" || seen[file] {
			return
		}
		seen[file] = true
		files = append(files, file)
	}
	for _, typ := range h.index.Types {
		add(typ.File)
	}
	for _, trigger := range h.index.Triggers {
		add(trigger.File)
	}
	for _, doc := range h.documents {
		add(doc.Path)
	}
	sort.Strings(files)
	return files
}

func (h *Handler) documentText(path string) (string, bool) {
	for _, doc := range h.documents {
		if sameDocument(doc.Path, path) {
			return doc.Text, true
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func (h *Handler) wordAt(uri DocumentURI, position Position) string {
	text, ok := h.documentText(pathFromURI(uri))
	if !ok {
		return h.symbolNameAt(uri, position)
	}
	offset, err := offsetForPosition(text, position)
	if err != nil {
		return ""
	}
	start, end := identifierBounds(text, offset)
	if start == end {
		return ""
	}
	return text[start:end]
}

func (h *Handler) wordRangeAt(uri DocumentURI, position Position) (Range, bool) {
	text, ok := h.documentText(pathFromURI(uri))
	if !ok {
		name := h.symbolNameAt(uri, position)
		if name == "" {
			return Range{}, false
		}
		if loc, ok := h.definitionForName(name); ok {
			return loc.Range, true
		}
		return Range{}, false
	}
	offset, err := offsetForPosition(text, position)
	if err != nil {
		return Range{}, false
	}
	start, end := identifierBounds(text, offset)
	if start == end {
		return Range{}, false
	}
	return rangeForOffsets(text, start, end), true
}

func (h *Handler) symbolNameAt(uri DocumentURI, position Position) string {
	path := pathFromURI(uri)
	for _, typ := range h.index.Types {
		if !sameDocument(path, typ.File) && !sameDocument(string(uri), typ.File) {
			continue
		}
		for _, member := range typ.Members {
			if containsPosition(member.Range, position) {
				return member.Name
			}
		}
		if containsPosition(typ.Range, position) {
			return typ.Name
		}
	}
	for _, trigger := range h.index.Triggers {
		if (sameDocument(path, trigger.File) || sameDocument(string(uri), trigger.File)) && containsPosition(trigger.Range, position) {
			return trigger.Name
		}
	}
	return ""
}

func wordRanges(text, word string) []Range {
	var ranges []Range
	lowerText := strings.ToLower(text)
	lowerWord := strings.ToLower(word)
	for offset := 0; offset < len(text); {
		next := strings.Index(lowerText[offset:], lowerWord)
		if next < 0 {
			break
		}
		start := offset + next
		end := start + len(lowerWord)
		if isIdentifierBoundary(text, start-1) && isIdentifierBoundary(text, end) {
			ranges = append(ranges, rangeForOffsets(text, start, end))
		}
		offset = end
	}
	return ranges
}

func identifierBounds(text string, offset int) (int, int) {
	if offset > len(text) {
		offset = len(text)
	}
	if offset > 0 && (offset == len(text) || !isIdentifierByte(text[offset])) && isIdentifierByte(text[offset-1]) {
		offset--
	}
	if offset < 0 || offset >= len(text) || !isIdentifierByte(text[offset]) {
		return offset, offset
	}
	start := offset
	for start > 0 && isIdentifierByte(text[start-1]) {
		start--
	}
	end := offset
	for end < len(text) && isIdentifierByte(text[end]) {
		end++
	}
	return start, end
}

func isIdentifierBoundary(text string, offset int) bool {
	return offset < 0 || offset >= len(text) || !isIdentifierByte(text[offset])
}

func isIdentifierByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}

func rangeForOffsets(text string, start, end int) Range {
	return Range{Start: positionForOffset(text, start), End: positionForOffset(text, end)}
}

func positionForOffset(text string, target int) Position {
	line, character := 0, 0
	for offset := 0; offset < len(text) && offset < target; {
		r, size := utf8.DecodeRuneInString(text[offset:])
		if r == '\r' || r == '\n' {
			line++
			character = 0
			offset += size
			if r == '\r' && offset < len(text) && text[offset] == '\n' {
				offset++
			}
			continue
		}
		character += utf16Len(string(r))
		offset += size
	}
	return Position{Line: line, Character: character}
}

func utf16Len(text string) int {
	n := 0
	for _, r := range text {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func typeLocations(index typesys.Index) []Location {
	var locations []Location
	for _, typ := range index.Types {
		locations = append(locations, Location{URI: uriFromPath(typ.File), Range: toLSPRange(typ.Range)})
	}
	return locations
}
