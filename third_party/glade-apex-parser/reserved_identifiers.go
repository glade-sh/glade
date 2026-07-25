//go:build cgo

package apexast

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// salesforceReservedIdentifiers is the complete reserved-word table from the
// current Apex Developer Guide. Apex identifiers are case-insensitive.
var salesforceReservedIdentifiers = wordSet(`
	abstract activate and any array as asc autonomous begin bigdecimal blob
	boolean break bulk by byte case cast catch char class collect commit const
	continue currency date datetime decimal default delete desc do double else
	end enum exception exit export extends false final finally float for from
	global goto group having hint if implements import in inner insert instanceof
	int integer interface into join like limit list long loop map merge new not
	null nulls number object of on or outer override package parallel pragma
	private protected public retrieve return rollback select set short sobject
	sort static string super switch synchronized system testmethod then this
	throw time transaction trigger true try undelete update upsert using virtual
	void webservice when where while
`)

// Salesforce permits most reserved words as method names. These words are
// grammar keywords in every identifier context, including method declarations.
var salesforceAlwaysKeywords = wordSet(`
	trigger insert update upsert delete undelete merge new for select
`)

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

func wordSet(words string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, word := range strings.Fields(words) {
		out[strings.ToLower(word)] = struct{}{}
	}
	return out
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
				reserved := salesforceReservedIdentifiers
				if node.Kind() == "method_declaration" {
					reserved = salesforceAlwaysKeywords
				}
				if _, ok := reserved[strings.ToLower(word)]; ok {
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
