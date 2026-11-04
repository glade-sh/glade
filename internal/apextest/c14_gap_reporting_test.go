package apextest

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Unreached oracle comparisons must be reported as skips, not passing cases.
func TestNativeCaptureGapReporting(t *testing.T) {
	cases := []struct {
		file, condition string
		count           int
	}{
		{"c11_q2_capture_test.go", "c11ActorGateCarry(", 2},
		{"c11_q345_capture_test.go", "c11ActorGateCarry(", 2},
		{"c11_q14_corrected_test.go", "c11ActorGateCarry(", 2},
		{"c11_q3_repair_admin_test.go", "c11ActorGateCarry(", 1},
		{"async_lifecycle_c14_conformance_test.go", `row[2] == "oversizedLocator"`, 1},
		{"stdlib_locale_crypto_c14_conformance_test.go", `key == "DootGcmNull67A1031/observe"`, 1},
		{"c14_warm_routes_test.go", `class == "DootGcmNull67A1031"`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, tc.file, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			ast.Inspect(file, func(n ast.Node) bool {
				ifStmt, ok := n.(*ast.IfStmt)
				if !ok {
					return true
				}
				var condition bytes.Buffer
				if err := format.Node(&condition, fset, ifStmt.Cond); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(condition.String(), tc.condition) {
					return true
				}
				count++
				body := ifStmt.Body.List
				if len(body) == 0 {
					t.Errorf("%s: empty gap guard", fset.Position(ifStmt.Pos()))
					return true
				}
				expr, ok := body[len(body)-1].(*ast.ExprStmt)
				if !ok {
					t.Errorf("%s: unreached comparison must end in t.Skip or t.Skipf", fset.Position(ifStmt.Pos()))
					return true
				}
				call, ok := expr.X.(*ast.CallExpr)
				if !ok {
					t.Errorf("%s: expected skip call", fset.Position(expr.Pos()))
					return true
				}
				ident, internal := call.Fun.(*ast.Ident)
				selector, external := call.Fun.(*ast.SelectorExpr)
				if !(internal && ident.Name == "case345Skip") && !(external && selector.Sel.Name == "Case345Skip") {
					t.Errorf("%s: expected skip call", fset.Position(expr.Pos()))
				}
				return true
			})
			if count != tc.count {
				t.Errorf("found %d guards, want %d", count, tc.count)
			}
		})
	}
}
