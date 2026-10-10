package apexast

import (
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	external "github.com/glade-sh/apex-parser"
	"github.com/glade-sh/glade/internal/diagnostic"
)

type ASTFile = external.ASTFile
type ASTNode = external.ASTNode
type Range = external.Range

type Parser struct {
	parser    *external.Parser
	closeOnce sync.Once
}

var openParsers atomic.Int64

var parserPool = sync.Pool{New: func() any { return NewParser() }}

func NewParser() *Parser {
	p := &Parser{parser: external.NewParser()}
	openParsers.Add(1)
	runtime.SetFinalizer(p, (*Parser).Close)
	return p
}

// Close releases the parser's native memory. It is nil-safe and idempotent;
// the parser must not be used after Close.
func (p *Parser) Close() {
	if p == nil {
		return
	}
	p.closeOnce.Do(func() {
		runtime.SetFinalizer(p, nil)
		if p.parser != nil {
			p.parser.Close()
			openParsers.Add(-1)
		}
	})
}

// OpenParsersForTesting reports parsers created by NewParser that are not closed.
func OpenParsersForTesting() int64 {
	return openParsers.Load()
}

// ParseSource parses source using an exclusively borrowed parser.
func ParseSource(path, source string) File {
	p := parserPool.Get().(*Parser)
	defer parserPool.Put(p)
	return p.ParseSource(path, source)
}

// ParseSourceAST parses source using an exclusively borrowed parser.
func ParseSourceAST(path, source string) ASTFile {
	p := parserPool.Get().(*Parser)
	defer parserPool.Put(p)
	return p.ParseSourceAST(path, source)
}

func (p *Parser) ParseFile(path string) (File, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return p.ParseSource(path, string(source)), nil
}

func (p *Parser) ParseFileAST(path string) (ASTFile, error) {
	defer runtime.KeepAlive(p)
	file, err := p.parser.ParseFileAST(path)
	if err == nil && hasGenericSOSLSyntaxDiagnostic(file.Diagnostics) {
		if source, err := os.ReadFile(path); err == nil {
			file.Diagnostics = p.refineInlineSOSLDiagnostics(path, string(source), file.Diagnostics)
		}
	}
	return file, err
}

func (p *Parser) ParseSource(path, source string) File {
	defer runtime.KeepAlive(p)
	file := p.parser.ParseSource(path, source)
	file.Diagnostics = p.refineInlineSOSLDiagnostics(path, source, file.Diagnostics)
	return withSharingDeclarationDiagnostics(nativePropertySyntax(nativeAnnotationSyntax(convertFile(file), source), source), source)
}

func (p *Parser) ParseSourceAST(path, source string) ASTFile {
	defer runtime.KeepAlive(p)
	file := p.parser.ParseSourceAST(path, source)
	file.Diagnostics = p.refineInlineSOSLDiagnostics(path, source, file.Diagnostics)
	return file
}

func convertFile(file external.File) File {
	return File{
		Path:         file.Path,
		Kind:         FileKind(file.Kind),
		Declarations: convertDeclarations(file.Declarations),
		Diagnostics:  convertDiagnostics(file.Diagnostics),
	}
}

func convertDeclarations(decls []external.Declaration) []Declaration {
	if len(decls) == 0 {
		return nil
	}
	out := make([]Declaration, 0, len(decls))
	for _, decl := range decls {
		out = append(out, convertDeclaration(decl))
	}
	return out
}

func convertDeclaration(decl external.Declaration) Declaration {
	return Declaration{
		Kind:           DeclarationKind(decl.Kind),
		Name:           decl.Name,
		SuperClass:     decl.SuperClass,
		Interfaces:     append([]string(nil), decl.Interfaces...),
		Type:           decl.Type,
		Modifiers:      decl.Modifiers,
		Annotations:    convertAnnotations(decl.Annotations),
		Parameters:     convertParameters(decl.Parameters),
		Accessors:      convertAccessors(decl.Accessors),
		ObjectName:     decl.ObjectName,
		Events:         decl.Events,
		TypeParameters: append([]string(nil), decl.TypeParameters...),
		HasBody:        decl.HasBody,
		BodyRange:      convertRangePtr(decl.BodyRange),
		Range:          convertRange(decl.Range),
		Members:        convertDeclarations(decl.Members),
	}
}

func convertAnnotations(items []external.Annotation) []Annotation {
	out := make([]Annotation, 0, len(items))
	for _, item := range items {
		annotation := Annotation{Name: item.Name, Range: convertRange(item.Range)}
		for _, argument := range item.Arguments {
			annotation.Arguments = append(annotation.Arguments, AnnotationArgument{Name: argument.Name, Value: argument.Value, Range: convertRange(argument.Range)})
		}
		out = append(out, annotation)
	}
	return out
}

func convertParameters(params []external.Parameter) []Parameter {
	if len(params) == 0 {
		return nil
	}
	out := make([]Parameter, 0, len(params))
	for _, param := range params {
		out = append(out, Parameter{
			Name:        param.Name,
			Type:        param.Type,
			Modifiers:   param.Modifiers,
			Annotations: convertAnnotations(param.Annotations),
			Range:       convertRange(param.Range),
		})
	}
	return out
}

func convertAccessors(accessors []external.Accessor) []Accessor {
	if len(accessors) == 0 {
		return nil
	}
	out := make([]Accessor, 0, len(accessors))
	for _, accessor := range accessors {
		out = append(out, Accessor{
			Kind:        accessor.Kind,
			Modifiers:   accessor.Modifiers,
			Annotations: convertAnnotations(accessor.Annotations),
			Range:       convertRange(accessor.Range),
			HasBody:     accessor.HasBody,
			BodyRange:   convertRangePtr(accessor.BodyRange),
		})
	}
	return out
}

func convertDiagnostics(diags []external.Diagnostic) []diagnostic.Diagnostic {
	if len(diags) == 0 {
		return nil
	}
	out := make([]diagnostic.Diagnostic, 0, len(diags))
	for _, diag := range diags {
		var r *diagnostic.Range
		if diag.Range != nil {
			converted := convertRange(*diag.Range)
			r = &converted
		}
		out = append(out, diagnostic.Diagnostic{
			Severity: diagnostic.Severity(diag.Severity),
			Code:     diag.Code,
			Message:  diag.Message,
			File:     diag.File,
			Range:    r,
			Excerpt:  diag.Excerpt,
		})
	}
	return out
}

func convertRange(r external.Range) diagnostic.Range {
	return diagnostic.Range{
		Start: convertPosition(r.Start),
		End:   convertPosition(r.End),
	}
}

func convertRangePtr(r *external.Range) *diagnostic.Range {
	if r == nil {
		return nil
	}
	converted := convertRange(*r)
	return &converted
}

func convertPosition(pos external.Position) diagnostic.Position {
	return diagnostic.Position{
		Line:   pos.Line,
		Column: pos.Column,
		Offset: pos.Offset,
	}
}

func containsModifier(mods []string, expected string) bool {
	for _, mod := range mods {
		if strings.EqualFold(mod, expected) {
			return true
		}
	}
	return false
}
