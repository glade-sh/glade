package repoguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// The Visualforce Apex Cookie emulation is the only place Glade writes a
// response cookie, and its Secure and HttpOnly flags come from the Apex
// program. CodeQL's cookie queries are excluded for that reason, so a new
// sink must be added here deliberately and reviewed.
const cookieSinkFile = "internal/server/visualforce.go"

func TestOnlyVisualforceWritesResponseCookies(t *testing.T) {
	root := repoRoot(t)
	var found []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "third_party", "dist", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.SelectorExpr:
				if pkg, ok := n.X.(*ast.Ident); ok && pkg.Name == "http" && (n.Sel.Name == "SetCookie" || n.Sel.Name == "Cookie") {
					found = append(found, rel+": http."+n.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatalf("expected the cookie sink in %s; update this guard if it moved", cookieSinkFile)
	}
	for _, use := range found {
		if !strings.HasPrefix(use, cookieSinkFile+":") {
			t.Errorf("new response cookie sink %s: review Secure and HttpOnly, then update this guard and the CodeQL exclusions in security.yml", use)
		}
	}
}
