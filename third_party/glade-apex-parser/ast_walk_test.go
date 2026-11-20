//go:build cgo

package apexast

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

func legacyASTNode(node *sitter.Node, source string, lines LineMap) ASTNode {
	out := ASTNode{Kind: node.Kind(), Range: treeSitterRange(node, lines)}
	if out.Kind == "date_time" {
		out.Range = originalCompactQueryOffsetRange(out.Range, source, lines)
	}
	for _, child := range namedChildren(node) {
		converted := legacyASTNode(&child, source, lines)
		out.Children = append(out.Children, converted)
		if converted.Range.End.Offset > out.Range.End.Offset {
			out.Range.End = converted.Range.End
		}
	}
	return out
}

func TestIndexedASTIdentity(t *testing.T) {
	sources := []string{"", "class A {}", "// π\nclass A { String s = 'ø'; }", "class A { void f(){ String select; } }", "class A { class B { Integer x; }}", "public class A { !!", "class D { Object d = [SELECT Id FROM Account WHERE CreatedDate > 2024-06-08T06:00:10+0130]; }", "class N { Integer n; }\x00\n", benchmarkASTInput(256)}
	p := NewParser()
	defer p.Close()
	for i, source := range sources {
		tree := p.parser.Parse([]byte(normalizeApexSource(source)), nil)
		if tree == nil {
			t.Fatalf("case %d: nil tree", i)
		}
		root := tree.RootNode()
		var want []ASTNode
		for _, child := range namedChildren(root) {
			want = append(want, legacyASTNode(&child, source, NewLineMap(source)))
		}
		got := p.ParseSourceAST("Input.cls", source)
		if !reflect.DeepEqual(got.Nodes, want) {
			t.Fatalf("case %d: tree differs", i)
		}
		var diagnostics []Diagnostic
		lines := NewLineMap(source)
		if root.HasError() {
			if d, ok := treeSitterDiagnostic("Input.cls", source, root, lines); ok {
				diagnostics = append(diagnostics, d)
			}
		}
		diagnostics = append(diagnostics, reservedIdentifierDiagnostics("Input.cls", source, root, lines)...)
		a, _ := json.Marshal(got.Diagnostics)
		b, _ := json.Marshal(diagnostics)
		if string(a) != string(b) {
			t.Fatalf("case %d: diagnostics differ: %s != %s", i, a, b)
		}
		tree.Close()
	}
}

func benchmarkASTInput(methods int) string {
	var s strings.Builder
	s.WriteString("public class Measure {\n")
	for i := 0; i < methods; i++ {
		fmt.Fprintf(&s, "public static Integer m%d(Integer x) { Integer y = x + 1; if (y > 4) { y++; } return y; }\n", i)
	}
	s.WriteString("}\n")
	return s.String()
}

func BenchmarkFullAST256(b *testing.B) {
	source := benchmarkASTInput(256)
	p := NewParser()
	defer p.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := p.ParseSourceAST("Input.cls", source)
		if len(f.Diagnostics) != 0 || len(f.Nodes) != 1 || f.Nodes[0].Kind != "class_declaration" {
			b.Fatal("unexpected result")
		}
	}
}
