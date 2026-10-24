package repoguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Check every conformance route without importing a potentially broken test
// package, so an incomplete scheduler migration has a runnable regression.
func TestLWCCompileBrowserPassCallSignatures(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "internal", "lwc", "compile")
	positions := token.NewFileSet()
	packages, err := parser.ParseDir(positions, dir, func(info os.FileInfo) bool {
		return strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg := packages["compile_test"]
	if pkg == nil {
		t.Fatal("LWC compile conformance package is missing")
	}
	var signatures []*ast.FuncType
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "runBrowserPasses" {
				signatures = append(signatures, fn.Type)
			}
		}
	}
	if len(signatures) != 1 {
		t.Fatalf("browser pass helper declarations = %d, want one shared helper", len(signatures))
	}
	want := signatures[0].Params.NumFields()
	calls := 0
	for _, file := range pkg.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := call.Fun.(*ast.Ident)
			if !ok || fn.Name != "runBrowserPasses" {
				return true
			}
			calls++
			position := positions.Position(call.Pos())
			t.Run(filepath.Base(position.Filename), func(t *testing.T) {
				if len(call.Args) != want {
					t.Errorf("line %d: browser pass call has %d arguments; shared helper requires %d", position.Line, len(call.Args), want)
				}
			})
			return true
		})
	}
	if calls == 0 {
		t.Fatal("no browser pass callers checked")
	}
}
