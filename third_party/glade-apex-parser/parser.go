//go:build cgo

package apexast

import (
	"os"
	"strings"
	"sync"

	"github.com/glade-sh/apex-parser/internal/tsapex"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Parser struct {
	mu     sync.Mutex
	parser *tree_sitter.Parser
	err    error
}

const voidIdentifierSentinel = "v0id"
const triggerContextSentinel = "Tr1gger"

func NewParser() *Parser {
	parser := tree_sitter.NewParser()
	err := parser.SetLanguage(tsapex.GetLanguage())
	return &Parser{parser: parser, err: err}
}

func (p *Parser) ParseFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return p.ParseSource(path, string(data)), nil
}

func (p *Parser) ParseSource(path, source string) File {
	out := File{Path: path, Kind: FileKindUnknown}
	parseSource := normalizeApexSource(source)
	if p.parser == nil && p.err == nil {
		parser := tree_sitter.NewParser()
		p.err = parser.SetLanguage(tsapex.GetLanguage())
		p.parser = parser
	}
	if p.err != nil {
		out.Diagnostics = append(out.Diagnostics, Diagnostic{
			Severity: Error,
			Code:     "APEXPARSE001",
			Message:  p.err.Error(),
			File:     path,
		})
		return out
	}

	p.mu.Lock()
	tree := p.parser.Parse([]byte(parseSource), nil)
	p.mu.Unlock()
	if tree == nil {
		out.Diagnostics = append(out.Diagnostics, Diagnostic{
			Severity: Error,
			Code:     "APEXPARSE001",
			Message:  "parse failed",
			File:     path,
		})
		return out
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return out
	}
	if root.HasError() {
		if diag, ok := treeSitterDiagnostic(path, source, root, NewLineMap(source)); ok {
			out.Diagnostics = append(out.Diagnostics, diag)
		}
	}
	lineMap := NewLineMap(source)
	for _, child := range namedChildren(root) {
		decl, ok := treeSitterDeclaration(&child, source, lineMap)
		if !ok {
			continue
		}
		out.Declarations = append(out.Declarations, decl)
		if out.Kind == FileKindUnknown {
			out.Kind = FileKind(decl.Kind)
		}
	}
	return out
}

func normalizeApexSource(source string) string {
	return normalizeExplicitConstructorInvocations(normalizeTriggerContextReferences(normalizeVoidIdentifiers(source)))
}

func treeSitterDeclaration(node *tree_sitter.Node, source string, lineMap LineMap) (Declaration, bool) {
	switch node.Kind() {
	case "class_declaration":
		return treeSitterClass(node, source, lineMap), true
	case "interface_declaration":
		return treeSitterInterface(node, source, lineMap), true
	case "enum_declaration":
		return treeSitterEnum(node, source, lineMap), true
	case "trigger_declaration":
		return treeSitterTrigger(node, source, lineMap), true
	default:
		return Declaration{}, false
	}
}

func treeSitterClass(node *tree_sitter.Node, source string, lineMap LineMap) Declaration {
	decl := Declaration{
		Kind:      DeclarationClass,
		Name:      nodeText(childByField(node, "name"), source),
		Modifiers: treeSitterModifiers(node, source),
		Range:     treeSitterRange(node, lineMap),
	}
	if body := childByField(node, "body"); body != nil {
		decl.Members = treeSitterMembers(body, source, lineMap)
	}
	return decl
}

func treeSitterInterface(node *tree_sitter.Node, source string, lineMap LineMap) Declaration {
	decl := Declaration{
		Kind:      DeclarationInterface,
		Name:      nodeText(childByField(node, "name"), source),
		Modifiers: treeSitterModifiers(node, source),
		Range:     treeSitterRange(node, lineMap),
	}
	if body := childByField(node, "body"); body != nil {
		for _, child := range namedChildren(body) {
			if child.Kind() == "method_declaration" {
				decl.Members = append(decl.Members, treeSitterMethod(&child, source, lineMap))
			}
		}
	}
	return decl
}

func treeSitterEnum(node *tree_sitter.Node, source string, lineMap LineMap) Declaration {
	return Declaration{
		Kind:      DeclarationEnum,
		Name:      nodeText(childByField(node, "name"), source),
		Modifiers: treeSitterModifiers(node, source),
		Range:     treeSitterRange(node, lineMap),
	}
}

func treeSitterTrigger(node *tree_sitter.Node, source string, lineMap LineMap) Declaration {
	decl := Declaration{
		Kind:       DeclarationTrigger,
		Name:       nodeText(childByField(node, "name"), source),
		ObjectName: nodeText(childByField(node, "object"), source),
		Range:      treeSitterRange(node, lineMap),
	}
	for _, child := range childrenByField(node, "events") {
		if child.Kind() != "trigger_event" {
			continue
		}
		event := strings.ToLower(strings.ReplaceAll(nodeText(&child, source), " ", ""))
		if event != "" {
			decl.Events = append(decl.Events, event)
		}
	}
	return decl
}

func treeSitterMembers(body *tree_sitter.Node, source string, lineMap LineMap) []Declaration {
	var members []Declaration
	for _, child := range namedChildren(body) {
		switch child.Kind() {
		case "method_declaration":
			members = append(members, treeSitterMethod(&child, source, lineMap))
		case "constructor_declaration":
			members = append(members, treeSitterConstructor(&child, source, lineMap))
		case "field_declaration":
			members = append(members, treeSitterFields(&child, source, lineMap)...)
		case "class_declaration", "interface_declaration", "enum_declaration":
			if decl, ok := treeSitterDeclaration(&child, source, lineMap); ok {
				members = append(members, decl)
			}
		case "block":
			members = append(members, Declaration{
				Kind:  DeclarationInitializer,
				Name:  "initializer",
				Range: treeSitterRange(&child, lineMap),
			})
		case "static_initializer":
			members = append(members, Declaration{
				Kind:      DeclarationInitializer,
				Name:      "initializer",
				Modifiers: []string{"static"},
				Range:     treeSitterRange(&child, lineMap),
			})
		}
	}
	return members
}

func treeSitterMethod(node *tree_sitter.Node, source string, lineMap LineMap) Declaration {
	return Declaration{
		Kind:       DeclarationMethod,
		Name:       nodeText(childByField(node, "name"), source),
		Type:       treeSitterTypeText(childByField(node, "type"), source),
		Modifiers:  treeSitterModifiers(node, source),
		Parameters: treeSitterParameters(childByField(node, "parameters"), source, lineMap),
		Range:      treeSitterRange(node, lineMap),
	}
}

func treeSitterConstructor(node *tree_sitter.Node, source string, lineMap LineMap) Declaration {
	return Declaration{
		Kind:       DeclarationConstructor,
		Name:       nodeText(childByField(node, "name"), source),
		Modifiers:  treeSitterModifiers(node, source),
		Parameters: treeSitterParameters(childByField(node, "parameters"), source, lineMap),
		Range:      treeSitterRange(node, lineMap),
	}
}

func treeSitterFields(node *tree_sitter.Node, source string, lineMap LineMap) []Declaration {
	typeNode := childByField(node, "type")
	mods := treeSitterModifiers(node, source)
	accessors := firstChildOfKind(node, "accessor_list")
	var decls []Declaration
	for _, variable := range childrenByField(node, "declarator") {
		nameNode := childByField(&variable, "name")
		decl := Declaration{
			Kind:      DeclarationField,
			Name:      nodeText(nameNode, source),
			Type:      treeSitterTypeText(typeNode, source),
			Modifiers: mods,
			Range:     treeSitterRange(&variable, lineMap),
		}
		if accessors != nil {
			decl.Kind = DeclarationProperty
			decl.Range = treeSitterRange(node, lineMap)
			decl.Accessors = treeSitterAccessors(accessors, source, lineMap)
		}
		decls = append(decls, decl)
	}
	return decls
}

func treeSitterAccessors(node *tree_sitter.Node, source string, lineMap LineMap) []Accessor {
	var accessors []Accessor
	for _, child := range namedChildren(node) {
		if child.Kind() != "accessor_declaration" {
			continue
		}
		accessor := Accessor{
			Modifiers: treeSitterModifiers(&child, source),
			Range:     treeSitterRange(&child, lineMap),
		}
		for i := uint(0); i < child.ChildCount(); i++ {
			token := child.Child(i)
			if token == nil {
				continue
			}
			switch token.Kind() {
			case "get", "set":
				accessor.Kind = token.Kind()
			case "block":
				accessor.HasBody = true
			}
		}
		if accessor.Kind != "" {
			accessors = append(accessors, accessor)
		}
	}
	return accessors
}

func treeSitterParameters(node *tree_sitter.Node, source string, lineMap LineMap) []Parameter {
	if node == nil {
		return nil
	}
	var params []Parameter
	for _, child := range namedChildren(node) {
		if child.Kind() != "formal_parameter" {
			continue
		}
		params = append(params, Parameter{
			Name:      nodeText(childByField(&child, "name"), source),
			Type:      treeSitterTypeText(childByField(&child, "type"), source),
			Modifiers: treeSitterModifiers(&child, source),
			Range:     treeSitterRange(&child, lineMap),
		})
	}
	return params
}

func treeSitterModifiers(node *tree_sitter.Node, source string) []string {
	modNode := firstChildOfKind(node, "modifiers")
	if modNode == nil {
		return nil
	}
	var mods []string
	for _, child := range namedChildren(modNode) {
		if child.Kind() == "line_comment" || child.Kind() == "block_comment" {
			continue
		}
		text := normalizeModifierText(child.Kind(), nodeText(&child, source))
		if text != "" {
			mods = append(mods, text)
		}
	}
	return mods
}

func normalizeModifierText(kind, text string) string {
	if kind == "annotation" {
		return normalizeAnnotationText(text)
	}
	return strings.Join(strings.Fields(text), " ")
}

func normalizeAnnotationText(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); {
		switch {
		case text[i] == '\'':
			next := copyApexString(&out, text, i)
			i = next
		case hasPrefixAt(text, i, "//"):
			i = skipUntilNewline(text, i)
		case hasPrefixAt(text, i, "/*"):
			i = skipBlockComment(text, i)
		case isWhitespaceByte(text[i]):
			i++
		default:
			out.WriteByte(text[i])
			i++
		}
	}
	return out.String()
}

func treeSitterTypeText(node *tree_sitter.Node, source string) string {
	text := nodeText(node, source)
	return strings.Join(strings.Fields(text), "")
}

func treeSitterRange(node *tree_sitter.Node, lineMap LineMap) Range {
	if node == nil {
		return Range{}
	}
	return Range{
		Start: lineMap.Position(int(node.StartByte())),
		End:   lineMap.Position(int(node.EndByte())),
	}
}

func treeSitterDiagnostic(path, source string, root *tree_sitter.Node, lineMap LineMap) (Diagnostic, bool) {
	errorNode := firstErrorNode(root)
	if errorNode == nil {
		return Diagnostic{}, false
	}
	r := treeSitterRange(errorNode, lineMap)
	return Diagnostic{
		Severity: Error,
		Code:     "APEXPARSE001",
		Message:  "syntax error",
		File:     path,
		Range:    &r,
		Excerpt:  excerpt(source, r.Start.Line),
	}, true
}

func firstErrorNode(node *tree_sitter.Node) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	if node.IsError() || node.IsMissing() {
		return node
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if found := firstErrorNode(child); found != nil {
			return found
		}
	}
	return nil
}

func nodeText(node *tree_sitter.Node, source string) string {
	if node == nil {
		return ""
	}
	start, end := int(node.StartByte()), int(node.EndByte())
	if start < 0 || end < start || end > len(source) {
		return ""
	}
	return source[start:end]
}

func normalizeVoidIdentifiers(source string) string {
	// Apex allows methods named "void". The upstream grammar treats a bare
	// void(...) call as the void type token, so rewrite only that identifier
	// form with a same-length sentinel to preserve byte offsets.
	var out strings.Builder
	out.Grow(len(source))

	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'':
			next := copyApexString(&out, source, i)
			i = next
		case hasPrefixAt(source, i, "//"):
			next := copyUntilNewline(&out, source, i)
			i = next
		case hasPrefixAt(source, i, "/*"):
			next := copyBlockComment(&out, source, i)
			i = next
		case hasWordAt(source, i, "void") && nextSignificantByte(source, i+len("void")) == '(':
			out.WriteString(voidIdentifierSentinel)
			i += len("void")
		default:
			out.WriteByte(source[i])
			i++
		}
	}
	return out.String()
}

func normalizeTriggerContextReferences(source string) string {
	// The upstream grammar reserves trigger for trigger declarations. Apex also
	// exposes Trigger.* context variables inside bodies, which are not part of
	// this package's declaration API.
	out := []byte(source)
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'':
			i = skipApexString(source, i)
		case hasPrefixAt(source, i, "//"):
			i = skipUntilNewline(source, i)
		case hasPrefixAt(source, i, "/*"):
			i = skipBlockComment(source, i)
		case hasWordAtFold(source, i, "trigger") && nextSignificantByte(source, i+len("trigger")) == '.':
			copy(out[i:i+len(triggerContextSentinel)], triggerContextSentinel)
			i += len(triggerContextSentinel)
		default:
			i++
		}
	}
	return string(out)
}

func normalizeExplicitConstructorInvocations(source string) string {
	// The upstream grammar treats this(...) and super(...) as Java-style
	// constructor chaining, which rejects Apex code that calls them elsewhere.
	// Body expression details are not part of this package's API, so rewrite
	// the call keyword and any leading cast prefix with same-length text.
	out := []byte(source)
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'':
			i = skipApexString(source, i)
		case hasPrefixAt(source, i, "//"):
			i = skipUntilNewline(source, i)
		case hasPrefixAt(source, i, "/*"):
			i = skipBlockComment(source, i)
		case hasWordAt(source, i, "this"):
			i = normalizeExplicitConstructorInvocation(out, source, i, "thiz")
		case hasWordAt(source, i, "super"):
			i = normalizeExplicitConstructorInvocation(out, source, i, "soper")
		default:
			i++
		}
	}
	return string(out)
}

func normalizeExplicitConstructorInvocation(out []byte, source string, keywordStart int, replacement string) int {
	openParen := nextSignificantIndex(source, keywordStart+len(replacement))
	if openParen >= len(source) || source[openParen] != '(' {
		return keywordStart + len(replacement)
	}
	copy(out[keywordStart:keywordStart+len(replacement)], replacement)
	argStart := nextSignificantIndex(source, openParen+1)
	if argStart >= len(source) || source[argStart] != '(' {
		return openParen + 1
	}
	castEnd, ok := castTypeEnd(source, argStart)
	if !ok {
		return argStart + 1
	}
	for i := argStart; i <= castEnd; i++ {
		out[i] = ' '
	}
	return castEnd + 1
}

func castTypeEnd(source string, openParen int) (int, bool) {
	depth := 0
	for i := openParen + 1; i < len(source); i++ {
		switch source[i] {
		case '<':
			depth++
		case '>':
			if depth == 0 {
				return 0, false
			}
			depth--
		case ')':
			if depth != 0 {
				return 0, false
			}
			if !looksLikeCastType(source[openParen+1 : i]) {
				return 0, false
			}
			next := nextSignificantIndex(source, i+1)
			return i, next < len(source) && isCastOperandStart(source[next])
		}
	}
	return 0, false
}

func looksLikeCastType(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	seenIdentifier := false
	for i := 0; i < len(text); i++ {
		b := text[i]
		switch {
		case isIdentifierByte(b):
			seenIdentifier = true
		case b == '.' || b == '<' || b == '>' || b == ',' || b == '[' || b == ']' || b == '?' || b == ' ' || b == '\t' || b == '\n' || b == '\r':
		default:
			return false
		}
	}
	return seenIdentifier
}

func isCastOperandStart(b byte) bool {
	return isIdentifierByte(b) || b == '(' || b == '\'' || b == '"' || b == '@'
}

func copyApexString(out *strings.Builder, source string, start int) int {
	i := start
	out.WriteByte(source[i])
	i++
	for i < len(source) {
		out.WriteByte(source[i])
		if source[i] == '\'' {
			if i+1 < len(source) && source[i+1] == '\'' {
				i++
				out.WriteByte(source[i])
				i++
				continue
			}
			i++
			break
		}
		i++
	}
	return i
}

func skipApexString(source string, start int) int {
	i := start + 1
	for i < len(source) {
		if source[i] == '\'' {
			if i+1 < len(source) && source[i+1] == '\'' {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return i
}

func copyUntilNewline(out *strings.Builder, source string, start int) int {
	i := start
	for i < len(source) {
		out.WriteByte(source[i])
		if source[i] == '\n' {
			i++
			break
		}
		i++
	}
	return i
}

func skipUntilNewline(source string, start int) int {
	i := start
	for i < len(source) {
		if source[i] == '\n' {
			return i + 1
		}
		i++
	}
	return i
}

func copyBlockComment(out *strings.Builder, source string, start int) int {
	i := start
	for i < len(source) {
		out.WriteByte(source[i])
		if i > start && source[i-1] == '*' && source[i] == '/' {
			i++
			break
		}
		i++
	}
	return i
}

func skipBlockComment(source string, start int) int {
	i := start
	for i < len(source) {
		if i > start && source[i-1] == '*' && source[i] == '/' {
			return i + 1
		}
		i++
	}
	return i
}

func hasPrefixAt(source string, index int, prefix string) bool {
	return index+len(prefix) <= len(source) && source[index:index+len(prefix)] == prefix
}

func hasWordAt(source string, index int, word string) bool {
	if !hasPrefixAt(source, index, word) {
		return false
	}
	beforeOK := index == 0 || !isIdentifierByte(source[index-1])
	after := index + len(word)
	afterOK := after >= len(source) || !isIdentifierByte(source[after])
	return beforeOK && afterOK
}

func hasWordAtFold(source string, index int, word string) bool {
	if index+len(word) > len(source) || !strings.EqualFold(source[index:index+len(word)], word) {
		return false
	}
	beforeOK := index == 0 || !isIdentifierByte(source[index-1])
	after := index + len(word)
	afterOK := after >= len(source) || !isIdentifierByte(source[after])
	return beforeOK && afterOK
}

func nextSignificantByte(source string, index int) byte {
	index = nextSignificantIndex(source, index)
	if index < len(source) {
		return source[index]
	}
	return 0
}

func nextSignificantIndex(source string, index int) int {
	for index < len(source) {
		switch {
		case isWhitespaceByte(source[index]):
			index++
		case hasPrefixAt(source, index, "//"):
			index = skipUntilNewline(source, index)
		case hasPrefixAt(source, index, "/*"):
			index = skipBlockComment(source, index)
		default:
			return index
		}
	}
	return len(source)
}

func isIdentifierByte(b byte) bool {
	return b == '_' || b == '$' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func childByField(node *tree_sitter.Node, field string) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	return node.ChildByFieldName(field)
}

func childrenByField(node *tree_sitter.Node, field string) []tree_sitter.Node {
	if node == nil {
		return nil
	}
	cursor := node.Walk()
	defer cursor.Close()
	return node.ChildrenByFieldName(field, cursor)
}

func firstChildOfKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	for _, child := range namedChildren(node) {
		if child.Kind() == kind {
			return &child
		}
	}
	return nil
}

func namedChildren(node *tree_sitter.Node) []tree_sitter.Node {
	if node == nil {
		return nil
	}
	cursor := node.Walk()
	defer cursor.Close()
	return node.NamedChildren(cursor)
}
