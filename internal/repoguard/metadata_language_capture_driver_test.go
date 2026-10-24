package repoguard

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Keep the captured anonymous and named routes connected to the executable
// conformance harness even when another Apex test file does not compile.
func TestMetadataLanguageDescribeAnonymousAndNamedCaptureDriverRegistered(t *testing.T) {
	positions := token.NewFileSet()
	packages, err := parser.ParseDir(positions, filepath.Join(repoRoot(t), "internal", "apextest"), func(info os.FileInfo) bool {
		return strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	registered := 0
	for _, file := range packages["apextest"].Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Recv != nil || fn.Body == nil {
				continue
			}
			for _, statement := range fn.Body.List {
				expr, ok := statement.(*ast.ExprStmt)
				if !ok {
					continue
				}
				var source bytes.Buffer
				if err := format.Node(&source, positions, expr.X); err != nil {
					t.Fatal(err)
				}
				if source.String() == `runNativeProjectRoutes(t, "metadata_language_describe", "LocaleRuleProbeTest", []string{"62.0", "67.0"}, 6)` {
					registered++
				}
			}
		}
	}
	if registered != 1 {
		t.Fatalf("metadata Language captures have %d conformance drivers, want one covering six methods on anonymous and named routes at API 62/67", registered)
	}
}
