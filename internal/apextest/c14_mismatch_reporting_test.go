package apextest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A locally checked native mismatch must not be logged as a conformance pass.
func TestNativeMismatchReporting(t *testing.T) {
	files, err := filepath.Glob("*_conformance_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no conformance tests found")
	}
	minimumSkips := map[string]int{
		"async_caps_c14_conformance_test.go": 0, "async_lifecycle_c14_conformance_test.go": 0,
		"async_y9_c14_conformance_test.go": 0, "batch_limit_c09_conformance_test.go": 0,
		"date_locale_a12_c14_conformance_test.go": 0, "scheduled_limit_c09_conformance_test.go": 0,
		"soql_clock_c14_conformance_test.go": 0, "soql_controls_c14_conformance_test.go": 0,
		"soql_identity_c14_conformance_test.go": 0, "soql_locale_c14_conformance_test.go": 0,
		"soql_gapprobe_c14_conformance_test.go": 0,
		// c11sharing: the Q2/Q3/Q4/Q5/Q14 security twins now assert every captured row exactly.
		// The locale/crypto and unicode families now assert every captured row.
		// stdlib_core: a2Surrogate now asserts the captured row (lone surrogates carry UTF-16 units).
		"stdlib_core_c14_conformance_test.go":          0,
		"stdlib_locale_crypto_c14_conformance_test.go": 0, "stdlib_unicode_small_c14_conformance_test.go": 0,
		// stdlib_surrogate: lone_json_roundtrip now asserts the captured row (json_lone_surrogate_c4).
		"stdlib_surrogate_c14_conformance_test.go": 0,
		// string_equality_locale_change: p3/p3b/p4 (a runAs Standard User updating its own User record) stay a carry.
		"string_equality_locale_change_c14_conformance_test.go": 1,
	}
	for _, name := range files {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		skips := 0
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "case345Skip" {
					skips++
				}
			}
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "Skip" || sel.Sel.Name == "Skipf" {
				skips++
				return true
			}
			if sel.Sel.Name != "Log" && sel.Sel.Name != "Logf" {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			message, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(message, "CARRY batch Dot stdlib cases Dot Dot SOQL cases and Dot async cases") || strings.Contains(message, "known gap batch Dot stdlib cases Dot Dot SOQL cases and Dot async cases") {
				t.Errorf("%s: native mismatch logged as pass: %s", fset.Position(call.Pos()), message)
			}
			return true
		})
		if skips < minimumSkips[name] {
			t.Errorf("%s: found %d skip sites, want at least %d", name, skips, minimumSkips[name])
		}
		delete(minimumSkips, name)
	}
	for name := range minimumSkips {
		t.Errorf("missing conformance source %s", name)
	}
}
