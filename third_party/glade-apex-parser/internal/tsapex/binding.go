//go:build cgo

package tsapex

// #include "tree_sitter/parser.h"
//
// const TSLanguage *tree_sitter_apex(void);
import "C"

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func GetLanguage() *tree_sitter.Language {
	return tree_sitter.NewLanguage(unsafe.Pointer(C.tree_sitter_apex()))
}
