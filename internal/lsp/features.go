package lsp

import (
	"os"
	"sort"
	"strings"
	"unicode"
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

// definitionForName resolves one case-insensitive name to its declaration.
// The generation's name index is built once per generation in the order the
// symbol scan used to visit, so the first entry stored for a name is the
// match the scan would have returned first.
func (h *requestHandler) definitionForName(name string) (Location, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return Location{}, false
	}
	h.generation.nameOnce.Do(h.generation.buildNameIndex)
	if locations := h.generation.names[caseFoldKey(name)]; len(locations) > 0 {
		return locations[0], true
	}
	return Location{}, false
}

// buildNameIndex maps every name a definition request can carry to the
// location the symbol scan would return for it, in the scan's order: types
// and their members first, then triggers, then objects and their fields.
// Several symbols may share one folded name; the first stored entry wins,
// as the first match of the scan did.
func (g *generation) buildNameIndex() {
	names := make(map[string][]Location)
	add := func(name string, location Location) {
		if name == "" {
			// Queries are never empty, so an empty candidate cannot match.
			return
		}
		key := caseFoldKey(name)
		names[key] = append(names[key], location)
	}
	for _, typ := range g.index.Types {
		add(typ.Name, Location{URI: uriFromPath(typ.File), Range: toLSPRange(typ.Range)})
		for _, member := range typ.Members {
			location := Location{URI: uriFromPath(typ.File), Range: toLSPRange(member.Range)}
			add(member.Name, location)
			add(typ.Name+"."+member.Name, location)
		}
	}
	for _, trigger := range g.index.Triggers {
		add(trigger.Name, Location{URI: uriFromPath(trigger.File), Range: toLSPRange(trigger.Range)})
	}
	for _, object := range g.index.Objects {
		add(object.Name, Location{URI: schemaURI(object.Name), Range: Range{}})
		for _, field := range object.Fields {
			location := Location{URI: schemaURI(object.Name, "fields", field.Name), Range: Range{}}
			add(field.Name, location)
			add(object.Name+"."+field.Name, location)
		}
	}
	g.names = names
}

// caseFoldKey folds name to a canonical key: two strings share a key exactly
// when strings.EqualFold reports them equal, which is how names were compared
// before. A name that is already folded (ASCII with no uppercase) returns
// unchanged, so queries folded after ToLower do not allocate.
func caseFoldKey(name string) string {
	for i := 0; i < len(name); i++ {
		if c := name[i]; c >= utf8.RuneSelf || (c >= 'A' && c <= 'Z') {
			return foldKeySlow(name)
		}
	}
	return name
}

func foldKeySlow(name string) string {
	var folded strings.Builder
	folded.Grow(len(name))
	for _, r := range name {
		folded.WriteRune(foldRune(r))
	}
	return folded.String()
}

// foldRune returns r's canonical folded rune, one rune per Unicode simple
// fold cycle: the cycle's lowercase ASCII letter when it has one (the k and
// s cycles extend past ASCII, to the Kelvin sign and long s), else the
// cycle's greatest rune. Cycle members therefore share one key and no other
// rune does, exactly matching strings.EqualFold; in particular a lowercase
// ASCII name is its own key.
func foldRune(r rune) rune {
	if r < utf8.RuneSelf {
		if 'A' <= r && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}
	high, ascii := r, r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if f > high {
			high = f
		}
		if f < utf8.RuneSelf && f < ascii {
			ascii = f
		}
	}
	if ascii < utf8.RuneSelf {
		if 'A' <= ascii && ascii <= 'Z' {
			ascii += 'a' - 'A'
		}
		return ascii
	}
	return high
}

func (h *requestHandler) referenceLocations(name string) []Location {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	var locations []Location
	for _, file := range h.referenceCandidates(name) {
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

func (h *requestHandler) sourceFiles() []string {
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
	h.docMu.RLock()
	for _, doc := range h.documents {
		add(doc.Path)
	}
	h.docMu.RUnlock()
	sort.Strings(files)
	return files
}

func (h *Handler) documentText(path string) (string, bool) {
	h.docMu.RLock()
	if uri, ok := h.documentPaths[canonicalDocumentPath(path)]; ok {
		text := h.documents[uri].Text
		h.docMu.RUnlock()
		return text, true
	}
	h.docMu.RUnlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func (h *requestHandler) wordAt(uri DocumentURI, position Position) string {
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

func (h *requestHandler) wordRangeAt(uri DocumentURI, position Position) (Range, bool) {
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

func (h *requestHandler) symbolNameAt(uri DocumentURI, position Position) string {
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
	cursor := positionCursor{text: text}
	for offset := 0; offset < len(text); {
		next := strings.Index(lowerText[offset:], lowerWord)
		if next < 0 {
			break
		}
		start := offset + next
		end := start + len(lowerWord)
		if isIdentifierBoundary(text, start-1) && isIdentifierBoundary(text, end) {
			ranges = append(ranges, Range{Start: cursor.positionAt(start), End: cursor.positionAt(end)})
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

// positionCursor converts ascending byte offsets of one text to positions in a
// single pass. positionAt gives the same answer as positionForOffset; an offset
// before the previous one restarts from the beginning.
type positionCursor struct {
	text      string
	offset    int
	line      int
	character int
	// lastStart is where the last consumed character began, or -1.
	lastStart int
	started   bool
}

func (c *positionCursor) positionAt(target int) Position {
	if c.started && target <= c.lastStart {
		*c = positionCursor{text: c.text}
	}
	c.started = true
	text := c.text
	offset, line, character := c.offset, c.line, c.character
	lastStart := c.lastStart
	if offset == 0 {
		lastStart = -1
	}
	for offset < len(text) && offset < target {
		lastStart = offset
		b := text[offset]
		if b < utf8.RuneSelf && b != '\r' && b != '\n' {
			character++
			offset++
			continue
		}
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
	c.offset, c.line, c.character, c.lastStart = offset, line, character, lastStart
	return Position{Line: line, Character: character}
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
