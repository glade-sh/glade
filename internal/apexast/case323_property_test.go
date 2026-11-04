package apexast

import (
	"github.com/glade-sh/glade/internal/diagnostic"
	"reflect"
	"strings"
	"testing"
)

func case323OriginalPropertySyntax(file File, source string) File {
	if !strings.Contains(strings.ToLower(source), "transient") {
		return file
	}
	tokens := annotationSyntaxTokens(source)
	lines := NewLineMap(source)
	add := func(message string, token annotationSyntaxToken) {
		file.Diagnostics = append(file.Diagnostics, diagnostic.Diagnostic{
			Severity:      diagnostic.Error,
			Code:          "APEXPARSE001",
			Message:       message,
			NativeMessage: message,
			File:          file.Path,
			Range:         &diagnostic.Range{Start: lines.Position(token.offset), End: lines.Position(token.offset + len(token.text))},
		})
	}
	var visit func([]Declaration)
	visit = func(declarations []Declaration) {
		for _, decl := range declarations {
			visit(decl.Members)
			if decl.Kind != DeclarationProperty || len(decl.Accessors) != 2 || decl.Accessors[0].HasBody || decl.Accessors[1].HasBody ||
				!strings.EqualFold(decl.Accessors[0].Kind, "get") || !strings.EqualFold(decl.Accessors[1].Kind, "set") {
				continue
			}
			for i := 0; i+7 < len(tokens); i++ {
				if tokens[i].offset < decl.Range.Start.Offset || tokens[i+6].offset >= decl.Range.End.Offset {
					continue
				}
				shape := []string{"{", "transient", "get", ";", "set", ";", "}", "public"}
				matches := true
				for j, token := range shape {
					if !strings.EqualFold(tokens[i+j].text, token) {
						matches = false
						break
					}
				}
				if !matches {
					continue
				}
				add("Invalid type: transient", tokens[i+1])
				add("Missing '<EOF>' at 'public'", tokens[i+7])
				add("Unexpected token ';'.", tokens[i+5])
				add("Unexpected token 'transient'.", tokens[i+1])
				break
			}
		}
	}
	visit(file.Declarations)
	return file
}

func TestCase323PropertyDiagnostics(t *testing.T) {
	for _, source := range []string{
		"class A { String x { transient get; set; } public Integer y; }",
		"class A { String x { get; set; } public Integer y; /* transient */ }",
		"class A { class B { String x { TRANSIENT get; set; } public Integer y; } String z { get; set; } }",
		"class A { String x { transient get; set; } private Integer y; }",
	} {
		parsed := ParseSource("A.cls", source)
		parsed.Diagnostics = nil
		want := case323OriginalPropertySyntax(parsed, source)
		got := nativePropertySyntax(parsed, source)
		if !reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
			t.Fatalf("%s: got %+v want %+v", source, got.Diagnostics, want.Diagnostics)
		}
	}
}
