package apexast

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// Native compiler diagnostics record recovery for an automatic
// transient getter followed by an automatic setter and a public declaration.
// The grammar retains this property, so reject the measured shape here while
// preserving its class identity for dependent Visualforce page diagnostics.
func nativePropertySyntax(file File, source string) File {
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
