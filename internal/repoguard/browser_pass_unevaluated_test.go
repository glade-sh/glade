package repoguard

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserPassWarmupUnevaluatedOperandRoutes(t *testing.T) {
	for _, imported := range []struct{ name, declaration, qualifier string }{
		{"standard_import", `"unsafe"`, "unsafe."},
		{"aliased_import", `layout "unsafe"`, "layout."},
		{"dot_import", `. "unsafe"`, ""},
		{"raw_import", "`unsafe`", "unsafe."},
		{"escaped_import", `"\x75nsafe"`, "unsafe."},
	} {
		t.Run(imported.name, func(t *testing.T) {
			for _, route := range []struct {
				name, body string
				want       int
			}{
				{"reviewer_sizeof_assignment", `_ = layout.Sizeof(warm()); browser()`, 1},
				{"reviewer_alignof_assignment", `_ = layout.Alignof(warm()); browser()`, 1},
				{"offsetof_composite_field", `_ = layout.Offsetof(struct{ value bool }{warm()}.value); browser()`, 1},
				{"parenthesized_builtin", `_ = (layout.Sizeof)((warm())); browser()`, 1},
				{"declaration_initializer", `var size = layout.Sizeof(warm()); _ = size; browser()`, 1},
				{"constant_initializer", `const size = layout.Alignof(warm()); _ = size; browser()`, 1},
				{"expression_statement_argument", `consume(layout.Sizeof(warm())); browser()`, 1},
				{"if_initializer", `if size := layout.Sizeof(warm()); size > 0 { browser() }`, 1},
				{"if_condition", `if layout.Sizeof(warm()) > 0 { browser() }`, 1},
				{"for_initializer", `for size := layout.Sizeof(warm()); size > 0; size-- { browser(); break }`, 1},
				{"range_body", `for range versions { _ = layout.Sizeof(warm()) }; browser()`, 1},
				{"switch_initializer", `switch size := layout.Sizeof(warm()); size { default: browser() }`, 1},
				{"switch_tag", `switch layout.Sizeof(warm()) { default: browser() }`, 1},
				{"switch_case", `switch uintptr(0) { case layout.Sizeof(warm()): browser() }`, 1},
				{"type_switch_assignment", `switch any(layout.Sizeof(warm())).(type) { default: browser() }`, 1},
				{"select_clause", `select { default: _ = layout.Sizeof(warm()); browser() }`, 1},
				{"deferred_argument", `defer consume(layout.Sizeof(warm())); browser()`, 1},
				{"concurrent_argument", `go consume(layout.Sizeof(warm())); browser()`, 1},
				{"nested_constant_len", `_ = len([1]uintptr{layout.Sizeof(warm())}); browser()`, 1},
				{"nested_constant_cap", `_ = cap(&[1]uintptr{layout.Alignof(warm())}); browser()`, 1},
				{"array_type_length", `var values [layout.Sizeof(warm())]bool; _ = values; browser()`, 1},
				{"short_circuit_left", `_ = layout.Sizeof(warm()) > 0 && enabled; browser()`, 1},
				{"short_circuit_right", `_ = enabled || layout.Sizeof(warm()) > 0; browser()`, 1},
				{"operand_browser_call_conservatively_checked", `_ = layout.Sizeof(browser()); browser()`, 2},
				{"explicit_warmup_before_sizeof", `warm(); _ = layout.Sizeof(warm()); browser()`, 0},
				{"evaluated_sibling_argument", `_, _ = layout.Sizeof(warm()), warm(); browser()`, 0},
				{"evaluated_len_argument", `_ = len([1]bool{warm()}); browser()`, 0},
				{"evaluated_cap_argument", `_ = cap(&[1]bool{warm()}); browser()`, 0},
			} {
				body := strings.ReplaceAll(route.body, "layout.", imported.qualifier)
				// Keep the import used in the evaluated len/cap control routes.
				body = "_ = " + imported.qualifier + "Sizeof(0); " + body
				checkBrowserPassUnevaluatedRoute(t, route.name, "import "+imported.declaration, body, route.want)
			}
		})
	}
}

func TestBrowserPassWarmupVariableSizeOperandRoutes(t *testing.T) {
	for _, builtin := range []string{"Sizeof", "Alignof", "Offsetof"} {
		operand := `struct{ Value T; Passes bool }{Passes: browser()}`
		if builtin == "Offsetof" {
			operand += ".Passes"
		}
		checkBrowserPassUnevaluatedFunctionRoute(t, builtin, `import "unsafe"`, "func browserRoute[T any]",
			"_ = unsafe."+builtin+"("+operand+")", 1)
		checkBrowserPassUnevaluatedFunctionRoute(t, builtin+"/preceding_warmup", `import "unsafe"`, "func browserRoute[T any]",
			"warm(); _ = unsafe."+builtin+"("+operand+")", 0)
	}
}

func TestBrowserPassWarmupUncalledFunctionRoutes(t *testing.T) {
	for _, route := range []struct{ name, body string }{
		{"discarded_literal", `_ = func() { warm() }; browser()`},
		{"assigned_literal", `compile := func() { warm() }; _ = compile; browser()`},
		{"declared_literal", `var compile = func() { warm() }; _ = compile; browser()`},
		{"composite_literal", `_ = []func(){func() { warm() }}; browser()`},
		{"discarded_function_value", `_ = warmer; browser()`},
		{"assigned_function_value", `compile := warmer; _ = compile; browser()`},
		{"declared_function_value", `var compile = warmer; _ = compile; browser()`},
		{"converted_function_value", `_ = (func(int, string, []string) bool)(warmer); browser()`},
	} {
		checkBrowserPassUnevaluatedRoute(t, route.name, "", route.body, 1)
	}
}

func TestBrowserPassWarmupEvaluatedLayoutMethodRoutes(t *testing.T) {
	// A similarly named ordinary method evaluates its arguments. Imports from
	// the other fixture files must not turn it into an unsafe layout query.
	for _, name := range []string{"Sizeof", "Alignof", "Offsetof"} {
		body := `layout := struct{ ` + name + ` func(bool) uintptr }{func(bool) uintptr { return 1 }};
_ = layout.` + name + `(warm()); browser()`
		checkBrowserPassUnevaluatedRoute(t, name, "", body, 0)
	}
}

func TestBrowserPassWarmupShadowedLayoutBuiltinRoutes(t *testing.T) {
	for _, imported := range []struct{ name, declaration, qualifier string }{
		{"standard_import", `"unsafe"`, "unsafe"},
		{"aliased_import", `layout "unsafe"`, "layout"},
		{"dot_import", `. "unsafe"`, ""},
	} {
		for _, name := range []string{"Sizeof", "Alignof", "Offsetof"} {
			prefix := imported.qualifier + "."
			shadow := imported.qualifier + ` := struct{ ` + name + ` func(bool) uintptr }{func(bool) uintptr { return 1 }}; `
			if imported.qualifier == "" {
				prefix = ""
				shadow = name + ` := func(bool) uintptr { return 1 }; `
			}
			body := "_ = " + prefix + "Sizeof(0); " + shadow
			t.Run(imported.name+"/"+name, func(t *testing.T) {
				checkBrowserPassUnevaluatedRoute(t, "evaluated_warmup", "import "+imported.declaration,
					body+"_ = "+prefix+name+"(warm()); browser()", 0)
				checkBrowserPassUnevaluatedRoute(t, "evaluated_browser_call", "import "+imported.declaration,
					body+"_ = "+prefix+name+"(browser())", 1)
				checkBrowserPassUnevaluatedRoute(t, "parenthesized_evaluated_browser_call", "import "+imported.declaration,
					body+"_ = ("+prefix+name+")(browser())", 1)
			})
		}
	}
}

func checkBrowserPassUnevaluatedRoute(t *testing.T, name, imports, body string, want int) {
	t.Helper()
	checkBrowserPassUnevaluatedFunctionRoute(t, name, imports, "func browserRoute", body, want)
}

func checkBrowserPassUnevaluatedFunctionRoute(t *testing.T, name, imports, declaration, body string, want int) {
	t.Helper()
	const helpers = `package compile_test
var gladehome = struct { EnsureRoot func(int, string, []string) bool }{l11CompileCases}
func l07CompileCases(int, string, []string) bool { return true }
func l11CompileCases(int, string, []string) bool { return true }
func l19CompileCases(int, string, []string) bool { return true }
func l21CompileCases(int, string, []string) bool { return true }
func consume(uintptr) {}
func runBrowserPasses(int, []string, int, func(int, int) bool) bool { return true }
`
	for _, warmer := range []string{"gladehome.EnsureRoot", "l07CompileCases", "l11CompileCases", "l19CompileCases", "l21CompileCases"} {
		for _, project := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/project_%t", name, warmer, project), func(t *testing.T) {
				route := strings.ReplaceAll(body, "warm()", warmer+"(t, versions[0], cases)")
				route = strings.ReplaceAll(route, "warmer", warmer)
				route = strings.ReplaceAll(route, "browser()", "runBrowserPasses(t, versions, 2, func(t int, pass int) bool { return l11CompileCases(t, versions[pass], cases) })")
				source := "package compile_test\n" + imports + "\n" + declaration + `(t int, enabled bool) {
	versions := []string{"62.0", "67.0"}
	var cases []string
	` + route + "\n}"
				positions := token.NewFileSet()
				file, err := parser.ParseFile(positions, "route_test.go", source, 0)
				if err != nil {
					t.Fatal(err)
				}
				var problems []string
				if project {
					helperFile, err := parser.ParseFile(positions, "helpers_test.go", helpers, 0)
					if err != nil {
						t.Fatal(err)
					}
					config := types.Config{Importer: importer.Default()}
					if _, err := config.Check("fixture", positions, []*ast.File{file, helperFile}, nil); err != nil {
						t.Fatalf("synthetic project does not compile: %v", err)
					}
					dir := filepath.Join(t.TempDir(), "internal", "lwc", "compile")
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
					for filename, content := range map[string]string{"route_test.go": source, "helpers_test.go": helpers} {
						if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0600); err != nil {
							t.Fatal(err)
						}
					}
					problems = browserPassWarmupDirectoryProblems(t, dir)
				} else {
					problems = browserPassWarmupProblems(positions, file)
				}
				if len(problems) != want {
					t.Fatalf("unevaluated operand warmup violations = %d, want %d: %v", len(problems), want, problems)
				}
				if want == 1 && !strings.Contains(problems[0], "route_test.go:") {
					t.Fatalf("diagnostic does not identify the unprepared browser route: %v", problems)
				}
			})
		}
	}
}
