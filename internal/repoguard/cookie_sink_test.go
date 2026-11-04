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

// The Visualforce cookie flags come from Apex. The public playground session
// cookie is separately pinned below because CodeQL's cookie queries are excluded.
const cookieSinkFile = "internal/server/visualforce.go"
const publicCookieSinkFile = "internal/playground/public_sessions.go"

func TestResponseCookieSinksAreReviewed(t *testing.T) {
	root := repoRoot(t)
	var found []string
	publicCookieLiterals := 0
	publicSetCookieCalls := 0
	publicCookieSafe := false
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
			if literal, ok := node.(*ast.CompositeLit); ok && rel == publicCookieSinkFile {
				if typ, ok := literal.Type.(*ast.SelectorExpr); ok && typ.Sel.Name == "Cookie" {
					if pkg, ok := typ.X.(*ast.Ident); ok && pkg.Name == "http" {
						publicCookieLiterals++
						flags := map[string]bool{}
						for _, element := range literal.Elts {
							kv, ok := element.(*ast.KeyValueExpr)
							if !ok {
								continue
							}
							key, ok := kv.Key.(*ast.Ident)
							if !ok {
								continue
							}
							switch value := kv.Value.(type) {
							case *ast.Ident:
								flags[key.Name] = value.Name == "true"
							case *ast.SelectorExpr:
								pkg, ok := value.X.(*ast.Ident)
								flags[key.Name] = ok && pkg.Name == "http" && value.Sel.Name == "SameSiteLaxMode"
							}
						}
						publicCookieSafe = flags["Secure"] && flags["HttpOnly"] && flags["SameSite"]
					}
				}
			}
			switch n := node.(type) {
			case *ast.SelectorExpr:
				if pkg, ok := n.X.(*ast.Ident); ok && pkg.Name == "http" && (n.Sel.Name == "SetCookie" || n.Sel.Name == "Cookie") {
					found = append(found, rel+": http."+n.Sel.Name)
					if rel == publicCookieSinkFile && n.Sel.Name == "SetCookie" {
						publicSetCookieCalls++
					}
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
	visualforceFound := false
	for _, use := range found {
		if strings.HasPrefix(use, cookieSinkFile+":") {
			visualforceFound = true
		} else if !strings.HasPrefix(use, publicCookieSinkFile+":") {
			t.Errorf("unreviewed response cookie sink %s", use)
		}
	}
	if !visualforceFound {
		t.Errorf("expected Visualforce cookie sink in %s", cookieSinkFile)
	}
	if publicCookieLiterals != 1 || publicSetCookieCalls != 1 || !publicCookieSafe {
		t.Errorf("public session cookie must pin Secure, HttpOnly and SameSite=Lax (literals=%d, sinks=%d)", publicCookieLiterals, publicSetCookieCalls)
	}
}
