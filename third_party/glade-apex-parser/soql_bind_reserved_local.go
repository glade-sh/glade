//go:build cgo

package apexast

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"strings"
)

// Keep type/member reserved-name diagnostics unchanged. Native LX030/LX032
// report a token diagnostic for these String local declarations.
func soqlBindReservedLocalMessage(source string, name *tree_sitter.Node, word string, local bool) string {
	if local && (word == "select" || word == "class") {
		prefix := source[:name.StartByte()]
		start := strings.LastIndexAny(prefix, ";{}\n") + 1
		if strings.TrimSpace(prefix[start:]) == "String" {
			return "Unexpected token '" + word + "'."
		}
	}
	return "Identifier name is reserved: " + word
}
