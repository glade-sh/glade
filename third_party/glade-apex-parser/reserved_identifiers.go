//go:build cgo

package apexast

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

var identifierDeclarationKinds = map[string]struct{}{
	"class_declaration":       {},
	"constructor_declaration": {},
	"enhanced_for_statement":  {},
	"enum_constant":           {},
	"enum_declaration":        {},
	"formal_parameter":        {},
	"interface_declaration":   {},
	"method_declaration":      {},
	"property_declaration":    {},
	"trigger_declaration":     {},
	"variable_declarator":     {},
}

func reservedIdentifierDiagnostics(path, source string, root *tree_sitter.Node, lineMap LineMap) []Diagnostic {
	return declarationIdentifierDiagnostics(path, source, root, lineMap)
}

func declarationIdentifierDiagnostics(path, source string, root *tree_sitter.Node, lineMap LineMap) []Diagnostic {
	var diagnostics []Diagnostic
	seenReserved := make(map[[2]uint]struct{})
	seenShape := make(map[[2]uint]struct{})

	var visit func(*tree_sitter.Node)
	visit = func(node *tree_sitter.Node) {
		if node == nil {
			return
		}
		if _, ok := identifierDeclarationKinds[node.Kind()]; ok {
			name := childByField(node, "name")
			if name != nil {
				word := nodeText(name, source)
				key := [2]uint{name.StartByte(), name.EndByte()}
				if IsReservedSourceIdentifier(word, node.Kind() == "method_declaration") {
					if _, duplicate := seenReserved[key]; !duplicate {
						seenReserved[key] = struct{}{}
						r := treeSitterRange(name, lineMap)
						diagnostics = append(diagnostics, Diagnostic{
							Severity: Error,
							Code:     "APEXPARSE002",
							Message:  "Identifier name is reserved: " + word,
							File:     path,
							Range:    &r,
							Excerpt:  excerpt(source, r.Start.Line),
						})
					}
				} else if err := ValidateSourceIdentifier(word); err != nil {
					if _, duplicate := seenShape[key]; !duplicate {
						seenShape[key] = struct{}{}
						r := treeSitterRange(name, lineMap)
						diagnostics = append(diagnostics, Diagnostic{
							Severity: Error,
							Code:     "APEXPARSE003",
							Message:  sourceIdentifierErrorMessage(word),
							File:     path,
							Range:    &r,
							Excerpt:  excerpt(source, r.Start.Line),
						})
					}
				}
			}
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			visit(node.NamedChild(i))
		}
	}

	visit(root)
	return diagnostics
}
