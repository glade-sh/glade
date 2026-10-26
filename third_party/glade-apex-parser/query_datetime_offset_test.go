//go:build cgo

package apexast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompactQueryOffsetsPreserveDeclarationAndASTRanges(t *testing.T) {
	for _, literal := range []string{"2020-01-01T08:00:00+0500", "2020-01-01T08:00:00.000+0500"} {
		for _, query := range []string{
			"List<Contact> rows = [SELECT Id FROM Contact WHERE CreatedDate >= " + literal + "];",
			"List<List<SObject>> rows = [FIND 'OwnedClockLiteral' RETURNING Contact(Id WHERE CreatedDate >= " + literal + ")];",
		} {
			t.Run(query, func(t *testing.T) {
				body := "{\n String text = '" + literal + "';\n // " + literal + "\n " + query + "\n }"
				method := "public static void literalRoute() " + body
				source := "public class ClockLiteralRange {\n " + method + "\n}"
				normalized := normalizeInlineQueryOffsets(source)
				for _, control := range []string{"'" + literal + "'", "// " + literal} {
					if !strings.Contains(normalized, control) {
						t.Fatalf("normalization changed string/comment control %q", control)
					}
				}
				path := filepath.Join(t.TempDir(), "ClockLiteralRange.cls")
				if err := os.WriteFile(path, []byte(source), 0644); err != nil {
					t.Fatal(err)
				}
				p := NewParser()
				defer p.Close()
				for _, route := range []string{"ParseSource", "ParseFile"} {
					t.Run(route, func(t *testing.T) {
						file := p.ParseSource(path, source)
						if route == "ParseFile" {
							var err error
							file, err = p.ParseFile(path)
							if err != nil {
								t.Fatal(err)
							}
						}
						if len(file.Diagnostics) != 0 || len(file.Declarations) != 1 || len(file.Declarations[0].Members) != 1 {
							t.Fatalf("declarations: %+v", file)
						}
						member := file.Declarations[0].Members[0]
						if member.BodyRange == nil || source[member.BodyRange.Start.Offset:member.BodyRange.End.Offset] != body {
							t.Fatalf("body range lost original offset spelling: %+v", member.BodyRange)
						}
					})
				}
				for _, route := range []string{"ParseSourceAST", "ParseFileAST"} {
					t.Run(route, func(t *testing.T) {
						file := p.ParseSourceAST(path, source)
						if route == "ParseFileAST" {
							var err error
							file, err = p.ParseFileAST(path)
							if err != nil {
								t.Fatal(err)
							}
						}
						if len(file.Diagnostics) != 0 {
							t.Fatalf("AST diagnostics: %+v", file.Diagnostics)
						}
						found := false
						foundLiteral := false
						var visit func([]ASTNode)
						visit = func(nodes []ASTNode) {
							for _, node := range nodes {
								if node.Kind == "date_time" {
									foundLiteral = true
									if got := source[node.Range.Start.Offset:node.Range.End.Offset]; got != literal {
										t.Fatalf("AST DateTime range = %q, want %q", got, literal)
									}
								}
								if node.Kind == "method_declaration" {
									found = true
									if node.Range.Start.Offset != strings.Index(source, method) || source[node.Range.Start.Offset:node.Range.End.Offset] != method {
										t.Fatalf("AST method range changed: %+v", node.Range)
									}
								}
								visit(node.Children)
							}
						}
						visit(file.Nodes)
						if !found {
							t.Fatal("method declaration missing from AST")
						}
						if !foundLiteral {
							t.Fatal("DateTime literal missing from AST")
						}
					})
				}
			})
		}
	}
}
