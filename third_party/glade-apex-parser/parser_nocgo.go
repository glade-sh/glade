//go:build !cgo

package apexast

import "os"

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) ParseFile(path string) (File, error) {
	if _, err := os.ReadFile(path); err != nil {
		return File{}, err
	}
	return p.ParseSource(path, ""), nil
}

func (p *Parser) ParseSource(path, source string) File {
	return File{
		Path: path,
		Kind: FileKindUnknown,
		Diagnostics: []Diagnostic{{
			Severity: Error,
			Code:     "APEXPARSECGO",
			Message:  "apex parser requires CGO because it uses the generated tree-sitter Apex parser",
			File:     path,
		}},
	}
}
