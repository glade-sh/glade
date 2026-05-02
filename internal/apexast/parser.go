package apexast

import (
	"fmt"
	"os"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/octoberswimmer/apexfmt/parser"
	"github.com/open-aer/oaer/internal/diagnostic"
)

type Parser struct{}

const voidIdentifierSentinel = "v0id"

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) ParseFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return p.ParseSource(path, string(data)), nil
}

func (p *Parser) ParseSource(path, source string) File {
	listener := &syntaxErrorListener{path: path, source: source}
	parseSource := normalizeVoidIdentifiers(source)
	input := antlr.NewInputStream(parseSource)
	lexer := parser.NewApexLexer(input)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)

	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	apexParser := parser.NewApexParser(stream)
	apexParser.RemoveErrorListeners()
	apexParser.AddErrorListener(listener)

	tree := apexParser.CompilationUnit()
	out := File{
		Path:        path,
		Kind:        FileKindUnknown,
		Diagnostics: listener.diagnostics,
	}
	if len(listener.diagnostics) > 0 {
		return out
	}

	if trigger := tree.TriggerUnit(); trigger != nil {
		out.Kind = FileKindTrigger
		out.Declarations = append(out.Declarations, buildTrigger(trigger))
		return out
	}

	typeDecl := tree.TypeDeclaration()
	if typeDecl == nil {
		return out
	}
	out.Declarations = append(out.Declarations, buildTypeDeclaration(typeDecl, modifiers(typeDecl.AllModifier()))...)
	if len(out.Declarations) > 0 {
		out.Kind = FileKind(out.Declarations[0].Kind)
	}
	return out
}

type syntaxErrorListener struct {
	*antlr.DefaultErrorListener
	path        string
	source      string
	diagnostics []diagnostic.Diagnostic
}

func (l *syntaxErrorListener) SyntaxError(_ antlr.Recognizer, _ interface{}, line, column int, msg string, _ antlr.RecognitionException) {
	start := diagnostic.Position{Line: line, Column: column + 1}
	l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERPARSE001",
		Message:  msg,
		File:     l.path,
		Range: &diagnostic.Range{
			Start: start,
			End:   start,
		},
		Excerpt: excerpt(l.source, line),
	})
}

func buildTypeDeclaration(ctx parser.ITypeDeclarationContext, mods []string) []Declaration {
	switch {
	case ctx.ClassDeclaration() != nil:
		return []Declaration{buildClass(ctx.ClassDeclaration(), mods)}
	case ctx.InterfaceDeclaration() != nil:
		return []Declaration{buildInterface(ctx.InterfaceDeclaration(), mods)}
	case ctx.EnumDeclaration() != nil:
		return []Declaration{buildEnum(ctx.EnumDeclaration(), mods)}
	default:
		return nil
	}
}

func buildClass(ctx parser.IClassDeclarationContext, mods []string) Declaration {
	decl := Declaration{
		Kind:      DeclarationClass,
		Name:      textOf(ctx.Id()),
		Modifiers: mods,
		Range:     rangeOf(ctx),
	}
	if body := ctx.ClassBody(); body != nil {
		for _, child := range body.AllClassBodyDeclaration() {
			decl.Members = append(decl.Members, buildClassBodyDeclaration(child)...)
		}
	}
	return decl
}

func buildInterface(ctx parser.IInterfaceDeclarationContext, mods []string) Declaration {
	decl := Declaration{
		Kind:      DeclarationInterface,
		Name:      textOf(ctx.Id()),
		Modifiers: mods,
		Range:     rangeOf(ctx),
	}
	if body := ctx.InterfaceBody(); body != nil {
		for _, method := range body.AllInterfaceMethodDeclaration() {
			decl.Members = append(decl.Members, buildInterfaceMethod(method))
		}
	}
	return decl
}

func buildEnum(ctx parser.IEnumDeclarationContext, mods []string) Declaration {
	return Declaration{
		Kind:      DeclarationEnum,
		Name:      textOf(ctx.Id()),
		Modifiers: mods,
		Range:     rangeOf(ctx),
	}
}

func buildTrigger(ctx parser.ITriggerUnitContext) Declaration {
	ids := ctx.AllId()
	name := ""
	objectName := ""
	if len(ids) > 0 {
		name = textOf(ids[0])
	}
	if len(ids) > 1 {
		objectName = textOf(ids[1])
	}

	events := make([]string, 0, len(ctx.AllTriggerCase()))
	for _, triggerCase := range ctx.AllTriggerCase() {
		events = append(events, strings.ToLower(triggerCase.GetText()))
	}

	return Declaration{
		Kind:       DeclarationTrigger,
		Name:       name,
		ObjectName: objectName,
		Events:     events,
		Range:      rangeOf(ctx),
	}
}

func buildClassBodyDeclaration(ctx parser.IClassBodyDeclarationContext) []Declaration {
	member := ctx.MemberDeclaration()
	if member == nil {
		return nil
	}

	mods := modifiers(ctx.AllModifier())
	switch {
	case member.MethodDeclaration() != nil:
		return []Declaration{buildMethod(member.MethodDeclaration(), mods)}
	case member.FieldDeclaration() != nil:
		return buildFields(member.FieldDeclaration(), mods)
	case member.ConstructorDeclaration() != nil:
		return []Declaration{buildConstructor(member.ConstructorDeclaration(), mods)}
	case member.PropertyDeclaration() != nil:
		return []Declaration{buildProperty(member.PropertyDeclaration(), mods)}
	case member.ClassDeclaration() != nil:
		return []Declaration{buildClass(member.ClassDeclaration(), mods)}
	case member.InterfaceDeclaration() != nil:
		return []Declaration{buildInterface(member.InterfaceDeclaration(), mods)}
	case member.EnumDeclaration() != nil:
		return []Declaration{buildEnum(member.EnumDeclaration(), mods)}
	default:
		return nil
	}
}

func buildMethod(ctx parser.IMethodDeclarationContext, mods []string) Declaration {
	return Declaration{
		Kind:      DeclarationMethod,
		Name:      textOf(ctx.MethodId()),
		Type:      returnType(ctx.TypeRef(), ctx.VOID() != nil),
		Modifiers: mods,
		Range:     rangeOf(ctx),
	}
}

func buildInterfaceMethod(ctx parser.IInterfaceMethodDeclarationContext) Declaration {
	return Declaration{
		Kind:      DeclarationMethod,
		Name:      textOf(ctx.MethodId()),
		Type:      returnType(ctx.TypeRef(), ctx.VOID() != nil),
		Modifiers: modifiers(ctx.AllModifier()),
		Range:     rangeOf(ctx),
	}
}

func buildFields(ctx parser.IFieldDeclarationContext, mods []string) []Declaration {
	var decls []Declaration
	fieldType := textOf(ctx.TypeRef())
	for _, variable := range ctx.VariableDeclarators().AllVariableDeclarator() {
		decls = append(decls, Declaration{
			Kind:      DeclarationField,
			Name:      textOf(variable.Id()),
			Type:      fieldType,
			Modifiers: mods,
			Range:     rangeOf(variable),
		})
	}
	return decls
}

func buildConstructor(ctx parser.IConstructorDeclarationContext, mods []string) Declaration {
	return Declaration{
		Kind:      DeclarationConstructor,
		Name:      textOf(ctx.QualifiedName()),
		Modifiers: mods,
		Range:     rangeOf(ctx),
	}
}

func buildProperty(ctx parser.IPropertyDeclarationContext, mods []string) Declaration {
	return Declaration{
		Kind:      DeclarationProperty,
		Name:      textOf(ctx.Id()),
		Type:      textOf(ctx.TypeRef()),
		Modifiers: mods,
		Range:     rangeOf(ctx),
	}
}

func returnType(ctx parser.ITypeRefContext, isVoid bool) string {
	if isVoid {
		return "void"
	}
	return textOf(ctx)
}

func modifiers(mods []parser.IModifierContext) []string {
	out := make([]string, 0, len(mods))
	for _, mod := range mods {
		text := normalizeModifier(mod)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func normalizeModifier(mod parser.IModifierContext) string {
	switch {
	case mod.WITH() != nil && mod.SHARING() != nil:
		return "with sharing"
	case mod.WITHOUT() != nil && mod.SHARING() != nil:
		return "without sharing"
	case mod.INHERITED() != nil && mod.SHARING() != nil:
		return "inherited sharing"
	default:
		return textOf(mod)
	}
}

func textOf(node interface{ GetText() string }) string {
	if node == nil {
		return ""
	}
	return denormalizeVoidIdentifier(node.GetText())
}

func rangeOf(ctx antlr.ParserRuleContext) diagnostic.Range {
	start := ctx.GetStart()
	stop := ctx.GetStop()
	out := diagnostic.Range{}
	if start != nil {
		out.Start = diagnostic.Position{
			Line:   start.GetLine(),
			Column: start.GetColumn() + 1,
			Offset: start.GetStart(),
		}
	}
	if stop != nil {
		out.End = diagnostic.Position{
			Line:   stop.GetLine(),
			Column: stop.GetColumn() + len(stop.GetText()) + 1,
			Offset: stop.GetStop() + 1,
		}
	}
	return out
}

func excerpt(source string, line int) string {
	if line <= 0 {
		return ""
	}
	lines := strings.Split(source, "\n")
	if line > len(lines) {
		return ""
	}
	return lines[line-1]
}

func normalizeVoidIdentifiers(source string) string {
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

func denormalizeVoidIdentifier(text string) string {
	if text == voidIdentifierSentinel {
		return "void"
	}
	return text
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

func copyUntilNewline(out *strings.Builder, source string, start int) int {
	i := start
	for i < len(source) {
		out.WriteByte(source[i])
		i++
		if source[i-1] == '\n' {
			break
		}
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

func nextSignificantByte(source string, start int) byte {
	for i := start; i < len(source); {
		switch {
		case isWhitespace(source[i]):
			i++
		case hasPrefixAt(source, i, "//"):
			i = skipLineComment(source, i)
		case hasPrefixAt(source, i, "/*"):
			i = skipBlockComment(source, i)
		default:
			return source[i]
		}
	}
	return 0
}

func skipLineComment(source string, start int) int {
	i := start
	for i < len(source) && source[i] != '\n' {
		i++
	}
	return i
}

func skipBlockComment(source string, start int) int {
	i := start + 2
	for i < len(source) {
		if i > start+2 && source[i-1] == '*' && source[i] == '/' {
			return i + 1
		}
		i++
	}
	return len(source)
}

func hasPrefixAt(source string, start int, prefix string) bool {
	return start+len(prefix) <= len(source) && source[start:start+len(prefix)] == prefix
}

func hasWordAt(source string, start int, word string) bool {
	if !hasPrefixAt(source, start, word) {
		return false
	}
	beforeOK := start == 0 || !isIdentifierByte(source[start-1])
	after := start + len(word)
	afterOK := after == len(source) || !isIdentifierByte(source[after])
	return beforeOK && afterOK
}

func isIdentifierByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func ParseError(file File) error {
	if len(file.Diagnostics) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %d parse diagnostic(s)", file.Path, len(file.Diagnostics))
}
