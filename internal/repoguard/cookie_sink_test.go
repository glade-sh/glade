package repoguard

import (
	"go/ast"
	"strings"
	"testing"
)

// The Visualforce cookie flags come from Apex. The public playground session
// cookie is separately pinned below because CodeQL's cookie queries are excluded.
const cookieSinkFile = "internal/server/visualforce.go"
const publicCookieSinkFile = "internal/playground/public_sessions.go"

// cookieSinkScope reports whether the cookie-sink review scans rel: every
// non-test Go file of the working tree except the directories its walk
// skips. The shared walk already skips .git and node_modules.
func cookieSinkScope(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		switch part {
		case "third_party", "dist", "bin":
			return false
		}
	}
	return true
}

// cookieSinkFileScan holds the public session cookie file's pinned flags.
type cookieSinkFileScan struct {
	literals       int
	setCookieCalls int
	safe           bool
}

// cookieSinkScanFile inspects one parsed non-test Go file for response cookie
// sinks. The returned uses are the file's http.SetCookie and http.Cookie
// references; the sink statistics exist only for the public session cookie
// file, whose literal must pin Secure, HttpOnly and SameSite=Lax.
func cookieSinkScanFile(rel string, file *ast.File) ([]string, *cookieSinkFileScan) {
	var sink *cookieSinkFileScan
	if rel == publicCookieSinkFile {
		sink = &cookieSinkFileScan{}
	}
	var uses []string
	ast.Inspect(file, func(node ast.Node) bool {
		if literal, ok := node.(*ast.CompositeLit); ok && rel == publicCookieSinkFile {
			if typ, ok := literal.Type.(*ast.SelectorExpr); ok && typ.Sel.Name == "Cookie" {
				if pkg, ok := typ.X.(*ast.Ident); ok && pkg.Name == "http" {
					sink.literals++
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
					sink.safe = flags["Secure"] && flags["HttpOnly"] && flags["SameSite"]
				}
			}
		}
		switch n := node.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := n.X.(*ast.Ident); ok && pkg.Name == "http" && (n.Sel.Name == "SetCookie" || n.Sel.Name == "Cookie") {
				uses = append(uses, rel+": http."+n.Sel.Name)
				if rel == publicCookieSinkFile && n.Sel.Name == "SetCookie" {
					sink.setCookieCalls++
				}
			}
		}
		return true
	})
	return uses, sink
}

// The shared scan walks the working tree once and parses each in-scope Go
// file once for both structural guards; this guard reports the cookie-sink
// review's findings from that parse.
func TestResponseCookieSinksAreReviewed(t *testing.T) {
	var found []string
	visualforceFound := false
	publicCookieLiterals := 0
	publicSetCookieCalls := 0
	publicCookieSafe := false
	for _, file := range structuralScanFiles(t) {
		found = append(found, file.cookieUses...)
		if file.cookieSink != nil {
			publicCookieLiterals += file.cookieSink.literals
			publicSetCookieCalls += file.cookieSink.setCookieCalls
			publicCookieSafe = file.cookieSink.safe
		}
	}
	if len(found) == 0 {
		t.Fatalf("expected the cookie sink in %s; update this guard if it moved", cookieSinkFile)
	}
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
