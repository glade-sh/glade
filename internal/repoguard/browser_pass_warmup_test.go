package repoguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBrowserPassWarmupRoutes(t *testing.T) {
	const call = `runBrowserPasses(t, versions, parallel, observe)`
	const ensure = `if _, err := gladehome.EnsureRoot(); err != nil { t.Fatal(err) };`
	const compile = `first := l11CompileCases(t, versions[0], cases);`
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"missing_initialization", call, 1},
		{"explicit_root", ensure + call, 0},
		{"serial_compile", compile + call, 0},
		{"serial_compile_loop", `for _, api := range versions { compiled[api] = l07CompileCases(t, api, cases) };` + call, 0},
		{"serial_compile_parallel_branch", `if parallel > 1 { for _, api := range versions { compiled[api] = l19CompileCases(t, api, cases) };` + call + `}`, 0},
		{"other_serial_compile", `first := l21CompileCases(t, cases, versions[0]);` + call, 0},
		{"initialization_after_call", call + `;` + ensure, 1},
		{"initialization_other_branch", `if parallel == 1 {` + ensure + `} else {` + call + `}`, 1},
		{"initialization_optional_branch", `if warm {` + ensure + `};` + call, 1},
		{"initialization_inside_callback", `observe := func() {` + ensure + `};` + call, 1},
		{"compile_inside_callback", `observe := func() {` + compile + `};` + call, 1},
		{"deferred_initialization", `defer gladehome.EnsureRoot();` + call, 1},
		{"concurrent_initialization", `go gladehome.EnsureRoot();` + call, 1},
		{"empty_compile_loop", `for _, api := range []string{} { compiled[api] = l07CompileCases(t, api, cases) };` + call, 1},
		{"reassigned_compile_loop", `versions = nil; for _, api := range versions { compiled[api] = l07CompileCases(t, api, cases) };` + call, 1},
		{"optional_version_assignment", `versions = nil; if warm { versions = []string{"67.0"} }; for _, api := range versions { compiled[api] = l07CompileCases(t, api, cases) };` + call, 1},
		{"shadowed_version_assignment", `versions = nil; if warm { versions := []string{"67.0"}; _ = versions }; for _, api := range versions { compiled[api] = l07CompileCases(t, api, cases) };` + call, 1},
		{"compile_loop_early_exit", `for _, api := range versions { if !warm { break }; compiled[api] = l07CompileCases(t, api, cases) };` + call, 1},
		{"compile_optional_loop", `for warm { first := l11CompileCases(t, versions[0], cases) };` + call, 1},
		{"unrelated_compile", `first := somethingCompileCases(t, versions[0], cases);` + call, 1},
		{"root_lookup_is_not_install", `root, err := gladehome.Root();` + call, 1},
		{"comment_is_not_initialization", "// gladehome.EnsureRoot()\n" + call, 1},
		{"other_function_is_not_initialization", `}; func other() {` + ensure + `}; func unprepared() {` + call, 1},
		{"generic_call", `runBrowserPasses[result](t, versions, parallel, observe)`, 1},
		{"pure_scheduler_callback", `runBrowserPasses(t, versions, parallel, func(t *testing.T, pass int) int { return pass * pass })`, 0},
		{"compiling_scheduler_callback", `runBrowserPasses(t, versions, parallel, func(t *testing.T, pass int) result { return compile(t, pass) })`, 1},
		{"each_call_is_checked", call + `;` + ensure + call, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			positions := token.NewFileSet()
			source := "package fixture\nfunc route() { versions := []string{\"59.0\", \"67.0\"}; " + tc.body + " }"
			file, err := parser.ParseFile(positions, "route_test.go", source, 0)
			if err != nil {
				t.Fatal(err)
			}
			if problems := browserPassWarmupProblems(positions, file); len(problems) != tc.want {
				t.Fatalf("warmup violations = %d, want %d: %v", len(problems), tc.want, problems)
			}
		})
	}
}

func TestLWCCompileBrowserPassWarmup(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "internal", "lwc", "compile")
	for _, problem := range browserPassWarmupDirectoryProblems(t, dir) {
		t.Error(problem)
	}
}

func TestBrowserPassWarmupPlantedMissingWarmupProjectRoute(t *testing.T) {
	const name = "lwc_parallel_passes_test.go"
	source, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "lwc", "compile", name))
	if err != nil {
		t.Fatal(err)
	}
	const warmup = "\tif _, err := gladehome.EnsureRoot(); err != nil {\n\t\tt.Fatal(err)\n\t}\n"
	if strings.Count(string(source), warmup) != 1 {
		t.Fatal("parallel pass fixture must have exactly one root warmup to remove")
	}
	planted := strings.Replace(string(source), warmup, "", 1)
	planted = strings.Replace(planted, "\n\t\"github.com/glade-sh/glade/internal/gladehome\"", "", 1)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(planted), 0600); err != nil {
		t.Fatal(err)
	}
	problems := browserPassWarmupDirectoryProblems(t, dir)
	if len(problems) != 1 || !strings.Contains(problems[0], name+":") {
		t.Fatalf("planted missing warmup violations = %v, want one in %s", problems, name)
	}
}

func TestBrowserPassWarmupShortCircuitRoutes(t *testing.T) {
	for _, warmer := range []string{"gladehome.EnsureRoot", "l07CompileCases", "l11CompileCases", "l19CompileCases", "l21CompileCases"} {
		for _, route := range []struct {
			name, expression string
			want             int
		}{
			{"skipped_and_rhs", "false && %s()", 1},
			{"skipped_or_rhs", "true || %s()", 1},
			{"conditional_and_rhs", "enabled && %s()", 1},
			{"conditional_or_rhs", "enabled || %s()", 1},
			{"and_lhs", "%s() && enabled", 0},
			{"or_lhs", "%s() || enabled", 0},
			{"nested_and_rhs", "enabled && (%s() || enabled)", 1},
			{"nested_or_rhs", "enabled || (%s() && enabled)", 1},
			{"eager_comparison", "enabled == %s()", 0},
		} {
			t.Run(warmer+"/"+route.name, func(t *testing.T) {
				positions := token.NewFileSet()
				source := "package fixture\nfunc route() { _ = " + fmt.Sprintf(route.expression, warmer) + "; runBrowserPasses(t, versions, parallel, observe) }"
				file, err := parser.ParseFile(positions, "route_test.go", source, 0)
				if err != nil {
					t.Fatal(err)
				}
				if problems := browserPassWarmupProblems(positions, file); len(problems) != route.want {
					t.Fatalf("warmup violations = %d, want %d: %v", len(problems), route.want, problems)
				}
			})
		}
	}
}

func TestBrowserPassWarmupShortCircuitProjectRoutes(t *testing.T) {
	for _, route := range []struct {
		name, statement string
		want            int
	}{
		{"assignment_skipped_and_rhs", "_ = false && l11CompileCases(t, versions[0], cases)", 1},
		{"assignment_skipped_or_rhs", "_ = true || l11CompileCases(t, versions[0], cases)", 1},
		{"declaration_skipped_and_rhs", "var warm = false && l11CompileCases(t, versions[0], cases); _ = warm", 1},
		{"declaration_skipped_or_rhs", "var warm = true || l11CompileCases(t, versions[0], cases); _ = warm", 1},
		{"if_init_skipped_and_rhs", "if warm := false && l11CompileCases(t, versions[0], cases); warm {}", 1},
		{"if_init_skipped_or_rhs", "if warm := true || l11CompileCases(t, versions[0], cases); warm {}", 1},
		{"assignment_and_lhs", "_ = l11CompileCases(t, versions[0], cases) && false", 0},
		{"assignment_or_lhs", "_ = l11CompileCases(t, versions[0], cases) || true", 0},
	} {
		t.Run(route.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "internal", "lwc", "compile")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			// Model a conformance project with its scheduler and serial compiler
			// declared in a separate file from the browser pass call site.
			sources := map[string]string{
				"helpers_test.go": `package compile_test
import "testing"
func l11CompileCases(*testing.T, string, []string) bool { return true }
func runBrowserPasses(*testing.T, []string, int, func(*testing.T, int) bool) {}
`,
				"route_test.go": `package compile_test
import "testing"
func TestCompileBrowserRoute(t *testing.T) {
	versions := []string{"59.0", "67.0"}
	var cases []string
	` + route.statement + `
	runBrowserPasses(t, versions, 2, func(t *testing.T, pass int) bool {
		return l11CompileCases(t, versions[pass], cases)
	})
}
`,
			}
			for name, source := range sources {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			problems := browserPassWarmupDirectoryProblems(t, dir)
			if len(problems) != route.want {
				t.Fatalf("project warmup violations = %d, want %d: %v", len(problems), route.want, problems)
			}
			if route.want == 1 && !strings.Contains(problems[0], "route_test.go:7: runBrowserPasses requires preceding EnsureRoot or serial compile") {
				t.Fatalf("project diagnostic does not identify the unprepared browser pass: %v", problems)
			}
		})
	}
}

func TestBrowserPassWarmupShortCircuitBrowserCallRoutes(t *testing.T) {
	for _, operator := range []string{"&&", "||"} {
		for _, ready := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/preceding_warmup_%t", operator, ready), func(t *testing.T) {
				warmup := ""
				want := 1
				if ready {
					warmup = "first := l11CompileCases(t, versions[0], cases);"
					want = 0
				}
				positions := token.NewFileSet()
				source := "package fixture\nfunc route() { " + warmup + " _ = enabled " + operator + " (len(runBrowserPasses(t, versions, parallel, observe)) > 0) }"
				file, err := parser.ParseFile(positions, "route_test.go", source, 0)
				if err != nil {
					t.Fatal(err)
				}
				if problems := browserPassWarmupProblems(positions, file); len(problems) != want {
					t.Fatalf("RHS browser call warmup violations = %d, want %d: %v", len(problems), want, problems)
				}
			})
		}
	}
}

// Labels must preserve the execution guarantees of the statement they wrap.
func browserPassLabeledStatementRoutes() []struct {
	name, statement string
	want            int
} {
	return []struct {
		name, statement string
		want            int
	}{
		{"for_false", "for false { %s() }", 1},
		{"for_parenthesized_false", "for (false) { %s() }", 1},
		{"for_negated_true", "for !true { %s() }", 1},
		{"for_named_false_constant", "for never { %s() }", 1},
		{"for_constant_comparison", "for 1 == 2 { %s() }", 1},
		{"for_optional_body", "for enabled { %s() }", 1},
		{"for_zero_iterations", "for i := 0; i < 0; i++ { %s() }", 1},
		{"for_optional_post", "for ; enabled; %s() {}", 1},
		{"for_guaranteed_init", "for %s(); false; {}", 0},
		{"range_empty", "for range []string{} { %s() }", 1},
		{"range_nonempty", "for range versions { %s() }", 0},
		{"if_optional_body", "if enabled { %s() }", 1},
		{"if_guaranteed_init", "if %s(); enabled {}", 0},
		{"switch_optional_case", "switch enabled { case true: %s() }", 1},
		{"switch_guaranteed_init", "switch %s(); enabled {}", 0},
		{"type_switch_optional_case", "switch any(enabled).(type) { case bool: %s() }", 1},
		{"select_optional_case", "select { default: %s() }", 1},
		{"deferred_call", "defer %s()", 1},
		{"concurrent_call", "go %s()", 1},
		{"guaranteed_block", "{ %s() }", 0},
		{"guaranteed_expression", "%s()", 0},
		{"nested_label_for_false", "{ if false { goto inner }; inner: for false { %s() } }", 1},
	}
}

func TestBrowserPassWarmupLabeledStatementRoutes(t *testing.T) {
	for _, warmer := range []string{"gladehome.EnsureRoot", "l07CompileCases", "l11CompileCases", "l19CompileCases", "l21CompileCases"} {
		for _, route := range browserPassLabeledStatementRoutes() {
			for _, labeled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/labeled_%t", warmer, route.name, labeled), func(t *testing.T) {
					statement := fmt.Sprintf(route.statement, warmer)
					if labeled {
						statement = "if false { goto warmup }; warmup: " + statement
					}
					positions := token.NewFileSet()
					source := "package fixture\nfunc route() { const never = false; versions := []string{\"59.0\", \"67.0\"}; " + statement + "; runBrowserPasses(t, versions, parallel, observe) }"
					file, err := parser.ParseFile(positions, "route_test.go", source, 0)
					if err != nil {
						t.Fatal(err)
					}
					if problems := browserPassWarmupProblems(positions, file); len(problems) != route.want {
						t.Fatalf("warmup violations = %d, want %d: %v", len(problems), route.want, problems)
					}
				})
			}
		}
	}
}

func TestBrowserPassWarmupLabeledStatementProjectRoutes(t *testing.T) {
	for _, route := range browserPassLabeledStatementRoutes() {
		for _, labeled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/labeled_%t", route.name, labeled), func(t *testing.T) {
				statement := strings.ReplaceAll(fmt.Sprintf(route.statement, "warm"), "warm()", "l11CompileCases(t, versions[0], cases)")
				if labeled {
					statement = "if false { goto warmup }; warmup: " + statement
				}
				dir := filepath.Join(t.TempDir(), "internal", "lwc", "compile")
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				// Keep the synthetic project type-correct without external imports.
				// The callback compiles serially only after the scheduler starts it.
				sources := map[string]string{
					"helpers_test.go": `package compile_test
func l11CompileCases(int, string, []string) bool { return true }
func runBrowserPasses(int, []string, int, func(int, int) bool) {}
`,
					"route_test.go": `package compile_test
func browserRoute(t int, enabled bool) {
	const never = false
	versions := []string{"59.0", "67.0"}
	var cases []string
	` + statement + `
	runBrowserPasses(t, versions, 2, func(t int, pass int) bool {
		return l11CompileCases(t, versions[pass], cases)
	})
}
`,
				}
				positions := token.NewFileSet()
				var files []*ast.File
				for name, source := range sources {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
						t.Fatal(err)
					}
					file, err := parser.ParseFile(positions, name, source, 0)
					if err != nil {
						t.Fatal(err)
					}
					files = append(files, file)
				}
				if _, err := new(types.Config).Check("fixture", positions, files, nil); err != nil {
					t.Fatalf("synthetic project does not compile: %v", err)
				}
				problems := browserPassWarmupDirectoryProblems(t, dir)
				if len(problems) != route.want {
					t.Fatalf("project warmup violations = %d, want %d: %v", len(problems), route.want, problems)
				}
				if route.want == 1 && !strings.Contains(problems[0], "route_test.go:7: runBrowserPasses requires preceding EnsureRoot or serial compile") {
					t.Fatalf("project diagnostic does not identify the unprepared browser pass: %v", problems)
				}
			})
		}
	}
}

func TestBrowserPassWarmupLabeledForFalseL10ProjectRoute(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "lwc", "compile", "l10_conformance_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	const warmup = "\t\tif _, err := gladehome.EnsureRoot(); err != nil {\n\t\t\tt.Fatal(err)\n\t\t}"
	if strings.Count(string(source), warmup) != 1 {
		t.Fatal("expected exactly one explicit warmup in the browser conformance route")
	}
	for _, route := range []struct {
		name, replacement string
		want              int
	}{
		{"guaranteed_warmup", warmup, 0},
		{"labeled_for_false_warmup", "if false { goto warmup }; warmup: for false {\n" + warmup + "\n}", 1},
		{"unlabeled_for_false_warmup", "for false {\n" + warmup + "\n}", 1},
	} {
		t.Run(route.name, func(t *testing.T) {
			dir := t.TempDir()
			planted := strings.Replace(string(source), warmup, route.replacement, 1)
			if err := os.WriteFile(filepath.Join(dir, "l10_conformance_test.go"), []byte(planted), 0600); err != nil {
				t.Fatal(err)
			}
			problems := browserPassWarmupDirectoryProblems(t, dir)
			if len(problems) != route.want {
				t.Fatalf("planted browser project warmup violations = %d, want %d: %v", len(problems), route.want, problems)
			}
			if route.want == 1 && !strings.Contains(problems[0], "l10_conformance_test.go:") {
				t.Fatalf("project diagnostic does not identify the planted file: %v", problems)
			}
		})
	}
}

func TestBrowserPassWarmupLabeledBrowserCallRoutes(t *testing.T) {
	for _, route := range []struct{ name, statement string }{
		{"expression", "%s"},
		{"block", "{ %s }"},
		{"for_body", "for enabled { %s }"},
		{"range_body", "for range versions { %s }"},
		{"if_body", "if enabled { %s }"},
		{"switch_case", "switch enabled { case true: %s }"},
		{"type_switch_case", "switch any(enabled).(type) { case bool: %s }"},
		{"select_case", "select { default: %s }"},
		{"deferred_call", "defer %s"},
		{"concurrent_call", "go %s"},
	} {
		for _, ready := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/preceding_warmup_%t", route.name, ready), func(t *testing.T) {
				warmup, want := "", 1
				if ready {
					warmup, want = "l11CompileCases();", 0
				}
				statement := fmt.Sprintf(route.statement, "runBrowserPasses(t, versions, parallel, observe)")
				positions := token.NewFileSet()
				source := "package fixture\nfunc route() { " + warmup + " if false { goto browser }; browser: " + statement + " }"
				file, err := parser.ParseFile(positions, "route_test.go", source, 0)
				if err != nil {
					t.Fatal(err)
				}
				if problems := browserPassWarmupProblems(positions, file); len(problems) != want {
					t.Fatalf("labeled browser call warmup violations = %d, want %d: %v", len(problems), want, problems)
				}
			})
		}
	}
}

// Goto is outside the structured execution proof used by this guard. Keep the
// unit and type-checked project routes identical so no syntax-only fixture can
// hide an invalid jump or omit a scheduler entry point.
func browserPassGotoRoutes() []struct {
	name, body string
	want       int
} {
	return []struct {
		name, body string
		want       int
	}{
		{"unconditional_forward", "goto browser; warm(); browser: browser()", 1},
		{"conditional_forward", "if enabled { goto browser }; warm(); browser: browser()", 1},
		{"nested_block", "{ goto browser }; warm(); browser: browser()", 1},
		{"else_branch", "if enabled {} else { goto browser }; warm(); browser: browser()", 1},
		{"for_body", "for enabled { goto browser }; warm(); browser: browser()", 1},
		{"range_body", "for range versions { goto browser }; warm(); browser: browser()", 1},
		{"switch_case", "switch enabled { case true: goto browser }; warm(); browser: browser()", 1},
		{"type_switch_case", "switch any(enabled).(type) { case bool: goto browser }; warm(); browser: browser()", 1},
		{"select_case", "select { default: goto browser }; warm(); browser: browser()", 1},
		{"labeled_loop", "loop: for enabled { if enabled { goto browser }; break loop }; warm(); browser: browser()", 1},
		{"transitive_jump", "goto skip; warm(); skip: goto browser; browser: browser()", 1},
		{"backward_first_pass", "goto browser; warmup: warm(); browser: browser(); if enabled { goto warmup }", 1},
		{"skipped_serial_range", "goto browser; for range versions { warm() }; browser: browser()", 1},
		{"warmup_before_jump_conservatively_rejected", "warm(); if enabled { goto browser }; _ = enabled; browser: browser()", 1},
		{"warmup_at_jump_target_conservatively_rejected", "goto warmup; warmup: warm(); browser()", 1},
		{"immediate_function", "func() { goto browser; warm(); browser: browser() }()", 1},
		{"deferred_function", "defer func() { goto browser; warm(); browser: browser() }()", 1},
		{"concurrent_function", "go func() { goto browser; warm(); browser: browser() }()", 1},
		{"outer_warmup_does_not_prepare_jumping_function", "warm(); func() { goto browser; warm(); browser: browser() }()", 1},
		{"shadowed_false_condition", "false := enabled; if false { goto browser }; warm(); browser: browser()", 1},
		{"constant_false_jump_conservatively_rejected", "if false { goto browser }; warm(); browser: browser()", 1},
		{"nested_constant_false_jump_conservatively_rejected", "if false { if enabled { goto browser } }; warm(); browser: browser()", 1},
		{"conditional_jump_to_next_statement_after_warmup", "warm(); if enabled { goto browser }; browser: browser()", 0},
		{"conditional_jump_to_next_warmup_statement", "if enabled { goto warmup }; warmup: warm(); browser()", 0},
		{"unrelated_function_jump", "_ = func() { goto done; done: return }; warm(); browser()", 0},
		{"jumping_parent_with_prepared_function", "if enabled { goto after }; after: func() { warm(); browser() }()", 0},
		{"empty_range_direct_route", "versions = nil; for range versions { warm() }; browser()", 1},
		{"captured_empty_range_immediate_function", "versions = nil; func() { for range versions { warm() }; browser() }()", 1},
		{"captured_empty_range_deferred_function", "versions = nil; defer func() { for range versions { warm() }; browser() }()", 1},
		{"captured_empty_range_concurrent_function", "versions = nil; go func() { for range versions { warm() }; browser() }()", 1},
		{"captured_empty_range_stored_function", "versions = nil; _ = func() { for range versions { warm() }; browser() }", 1},
		{"local_nonempty_range_function", "func() { versions := []string{\"59.0\"}; for range versions { warm() }; browser() }()", 0},
		{"captured_unchanged_nonempty_range_function", "func() { for range versions { warm() }; browser() }()", 0},
	}
}

func browserPassGotoSource(warmer, body string) string {
	body = strings.ReplaceAll(body, "warm()", warmer+"()")
	body = strings.ReplaceAll(body, "browser()", "runBrowserPasses(t, versions, 2, func(t int, pass int) bool { return l11CompileCases() })")
	return "package compile_test\nfunc browserRoute(t int, enabled bool) { versions := []string{\"59.0\", \"67.0\"}; _ = versions; " + body + " }"
}

func TestBrowserPassWarmupGotoRoutes(t *testing.T) {
	for _, warmer := range []string{"gladehome.EnsureRoot", "l07CompileCases", "l11CompileCases", "l19CompileCases", "l21CompileCases"} {
		for _, route := range browserPassGotoRoutes() {
			t.Run(warmer+"/"+route.name, func(t *testing.T) {
				positions := token.NewFileSet()
				file, err := parser.ParseFile(positions, "route_test.go", browserPassGotoSource(warmer, route.body), 0)
				if err != nil {
					t.Fatal(err)
				}
				if problems := browserPassWarmupProblems(positions, file); len(problems) != route.want {
					t.Fatalf("warmup violations = %d, want %d: %v", len(problems), route.want, problems)
				}
			})
		}
	}
}

func TestBrowserPassWarmupGotoProjectRoutes(t *testing.T) {
	for _, warmer := range []string{"gladehome.EnsureRoot", "l07CompileCases", "l11CompileCases", "l19CompileCases", "l21CompileCases"} {
		for _, route := range browserPassGotoRoutes() {
			t.Run(warmer+"/"+route.name, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "internal", "lwc", "compile")
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				sources := map[string]string{
					"helpers_test.go": `package compile_test
var gladehome = struct { EnsureRoot func() bool }{func() bool { return true }}
func l07CompileCases() bool { return true }
func l11CompileCases() bool { return true }
func l19CompileCases() bool { return true }
func l21CompileCases() bool { return true }
func runBrowserPasses(int, []string, int, func(int, int) bool) {}
`,
					"route_test.go": browserPassGotoSource(warmer, route.body),
				}
				positions := token.NewFileSet()
				var files []*ast.File
				for name, source := range sources {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
						t.Fatal(err)
					}
					file, err := parser.ParseFile(positions, name, source, 0)
					if err != nil {
						t.Fatal(err)
					}
					files = append(files, file)
				}
				if _, err := new(types.Config).Check("fixture", positions, files, nil); err != nil {
					t.Fatalf("synthetic project does not compile: %v", err)
				}
				problems := browserPassWarmupDirectoryProblems(t, dir)
				if len(problems) != route.want {
					t.Fatalf("project warmup violations = %d, want %d: %v", len(problems), route.want, problems)
				}
				if route.want == 1 && !strings.Contains(problems[0], "route_test.go:2: runBrowserPasses requires preceding EnsureRoot or serial compile") {
					t.Fatalf("project diagnostic does not identify the unprepared browser pass: %v", problems)
				}
			})
		}
	}
}

func TestBrowserPassWarmupGotoL10ProjectRoutes(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "lwc", "compile", "l10_conformance_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	const warmup = "\t\tif _, err := gladehome.EnsureRoot(); err != nil {\n\t\t\tt.Fatal(err)\n\t\t}"
	if strings.Count(string(source), warmup) != 1 {
		t.Fatal("expected exactly one explicit warmup in the browser conformance route")
	}
	for _, route := range []struct{ name, jump string }{
		{"unconditional_forward", "goto browser"},
		{"conditional_forward", "if parallel > 1 { goto browser }"},
	} {
		t.Run(route.name, func(t *testing.T) {
			dir := t.TempDir()
			planted := strings.Replace(string(source), warmup, route.jump+"\n"+warmup+"\nbrowser: ;", 1)
			if err := os.WriteFile(filepath.Join(dir, "l10_conformance_test.go"), []byte(planted), 0600); err != nil {
				t.Fatal(err)
			}
			problems := browserPassWarmupDirectoryProblems(t, dir)
			if len(problems) != 1 {
				t.Fatalf("planted browser project warmup violations = %d, want 1: %v", len(problems), problems)
			}
			if !strings.Contains(problems[0], "l10_conformance_test.go:") {
				t.Fatalf("project diagnostic does not identify the planted file: %v", problems)
			}
		})
	}
}

func TestBrowserPassWarmupGotoPureSchedulerRoute(t *testing.T) {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, "route_test.go", `package fixture
func route() {
	goto browser
browser:
	runBrowserPasses(t, versions, parallel, func(t *testing.T, pass int) int { return pass * pass })
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if problems := browserPassWarmupProblems(positions, file); len(problems) != 0 {
		t.Fatalf("pure scheduler unexpectedly requires a toolchain warmup: %v", problems)
	}
}

// The warmup range and scheduled passes must be separate slices: emptying the
// former must not accidentally suppress the browser work in these regressions.
type browserPassRangeMutationRoute struct {
	name, body string
	want       int
}

func checkBrowserPassRangeMutationRoutes(t *testing.T, routes []browserPassRangeMutationRoute) {
	t.Helper()
	var sources []browserPassRangeMutationRoute
	for _, route := range routes {
		sources = append(sources, browserPassRangeMutationRoute{route.name, `package compile_test
func browserRoute(t int) {
	passes := []string{"59.0", "67.0"}
	versions := []string{"59.0", "67.0"}
	` + route.body + "\n}", route.want})
	}
	checkBrowserPassRangeMutationSources(t, sources)
}

func checkBrowserPassRangeMutationSources(t *testing.T, routes []browserPassRangeMutationRoute) {
	t.Helper()
	for _, warmer := range []string{"gladehome.EnsureRoot", "l07CompileCases", "l11CompileCases", "l19CompileCases", "l21CompileCases"} {
		for _, route := range routes {
			for _, project := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/project_%t", warmer, route.name, project), func(t *testing.T) {
					source := strings.ReplaceAll(route.body, "warm()", warmer+"()")
					source = strings.ReplaceAll(source, "browser()", "runBrowserPasses(t, passes, 2, func(t int, pass int) bool { return l11CompileCases() })")
					positions := token.NewFileSet()
					file, err := parser.ParseFile(positions, "route_test.go", source, 0)
					if err != nil {
						t.Fatal(err)
					}
					var problems []string
					if project {
						dir := filepath.Join(t.TempDir(), "internal", "lwc", "compile")
						if err := os.MkdirAll(dir, 0700); err != nil {
							t.Fatal(err)
						}
						const helpers = `package compile_test
var gladehome = struct { EnsureRoot func() bool }{func() bool { return true }}
func l07CompileCases() bool { return true }
func l11CompileCases() bool { return true }
func l19CompileCases() bool { return true }
func l21CompileCases() bool { return true }
func emptyVersions(versions *[]string) { *versions = nil }
func runBrowserPasses(int, []string, int, func(int, int) bool) {}
`
						helperFile, err := parser.ParseFile(positions, "helpers_test.go", helpers, 0)
						if err != nil {
							t.Fatal(err)
						}
						if _, err := new(types.Config).Check("fixture", positions, []*ast.File{file, helperFile}, nil); err != nil {
							t.Fatalf("synthetic project does not compile: %v", err)
						}
						for name, content := range map[string]string{"route_test.go": source, "helpers_test.go": helpers} {
							if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
								t.Fatal(err)
							}
						}
						problems = browserPassWarmupDirectoryProblems(t, dir)
					} else {
						problems = browserPassWarmupProblems(positions, file)
					}
					if len(problems) != route.want {
						t.Fatalf("range mutation warmup violations = %d, want %d: %v", len(problems), route.want, problems)
					}
					if route.want == 1 && !strings.Contains(problems[0], "route_test.go:") {
						t.Fatalf("diagnostic does not identify the unprepared browser route: %v", problems)
					}
				})
			}
		}
	}
}

func TestBrowserPassWarmupLateCapturedAssignmentRoutes(t *testing.T) {
	checkBrowserPassRangeMutationRoutes(t, []browserPassRangeMutationRoute{
		{"stored_function_then_assignment", `run := func() { for range versions { warm() }; browser() }; versions = nil; run()`, 1},
		{"deferred_function_then_assignment", `defer func() { for range versions { warm() }; browser() }(); versions = nil`, 1},
		{"concurrent_function_then_synchronized_assignment", `release := make(chan struct{}); done := make(chan struct{}); go func() { <-release; for range versions { warm() }; browser(); close(done) }(); versions = nil; close(release); <-done`, 1},
	})
}

func TestBrowserPassWarmupLateCapturedWriterRoutes(t *testing.T) {
	checkBrowserPassRangeMutationRoutes(t, []browserPassRangeMutationRoute{
		{"stored_function_then_captured_writer", `run := func() { for range versions { warm() }; browser() }; empty := func() { versions = nil }; empty(); run()`, 1},
		{"stored_function_then_immediate_writer", `run := func() { for range versions { warm() }; browser() }; func() { versions = nil }(); run()`, 1},
		{"deferred_function_then_deferred_writer", `defer func() { for range versions { warm() }; browser() }(); defer func() { versions = nil }()`, 1},
		{"nested_stored_function_then_captured_writer", `func() { run := func() { for range versions { warm() }; browser() }; empty := func() { versions = nil }; empty(); run() }()`, 1},
		{"stored_function_then_parenthesized_captured_writer", `run := func() { for range versions { warm() }; browser() }; empty := func() { (versions) = nil }; empty(); run()`, 1},
	})
}

func TestBrowserPassWarmupPointerWriteRoutes(t *testing.T) {
	checkBrowserPassRangeMutationRoutes(t, []browserPassRangeMutationRoute{
		{"direct_pointer_write", `*(&versions) = nil; for range versions { warm() }; browser()`, 1},
		{"aliased_pointer_write", `pointer := &versions; *pointer = nil; for range versions { warm() }; browser()`, 1},
		{"chained_pointer_alias_write", `pointer := &versions; alias := pointer; *alias = nil; for range versions { warm() }; browser()`, 1},
		{"parenthesized_address_write", `pointer := &((versions)); *pointer = nil; for range versions { warm() }; browser()`, 1},
		{"helper_pointer_write", `emptyVersions(&versions); for range versions { warm() }; browser()`, 1},
		{"captured_pointer_write", `pointer := &versions; empty := func() { *pointer = nil }; empty(); for range versions { warm() }; browser()`, 1},
		{"stored_function_then_pointer_write", `run := func() { for range versions { warm() }; browser() }; pointer := &versions; *pointer = nil; run()`, 1},
		{"deferred_function_then_pointer_write", `defer func() { for range versions { warm() }; browser() }(); emptyVersions(&versions)`, 1},
		{"concurrent_function_then_synchronized_pointer_write", `release := make(chan struct{}); done := make(chan struct{}); go func() { <-release; for range versions { warm() }; browser(); close(done) }(); emptyVersions(&versions); close(release); <-done`, 1},
	})
}

func TestBrowserPassWarmupCapturedRangeBindingWriteRoutes(t *testing.T) {
	checkBrowserPassRangeMutationRoutes(t, []browserPassRangeMutationRoute{
		{"direct_range_binding_write", `for _, versions = range [][]string{nil} {}; for range versions { warm() }; browser()`, 1},
		{"stored_function_then_range_binding_write", `run := func() { for range versions { warm() }; browser() }; for _, versions = range [][]string{nil} {}; run()`, 1},
		{"deferred_function_then_parenthesized_range_binding_write", `defer func() { for range versions { warm() }; browser() }(); for _, (versions) = range [][]string{nil} {}`, 1},
	})
}

func TestBrowserPassWarmupRangeMutationBoundaryRoutes(t *testing.T) {
	checkBrowserPassRangeMutationRoutes(t, []browserPassRangeMutationRoute{
		{"unchanged_direct_range", `for range versions { warm() }; browser()`, 0},
		{"unchanged_stored_range", `run := func() { for range versions { warm() }; browser() }; run()`, 0},
		{"unchanged_deferred_range", `defer func() { for range versions { warm() }; browser() }()`, 0},
		{"unchanged_concurrent_range", `done := make(chan struct{}); go func() { for range versions { warm() }; browser(); close(done) }(); <-done`, 0},
		{"copied_slice_header_write", `copy := versions; emptyVersions(&copy); for range versions { warm() }; browser()`, 0},
		{"slice_element_write", `versions[0] = "62.0"; for range versions { warm() }; browser()`, 0},
		{"slice_element_pointer_write", `element := &versions[0]; *element = "62.0"; for range versions { warm() }; browser()`, 0},
		{"shadowed_slice_header_write", `func() { versions := []string{"62.0"}; emptyVersions(&versions) }(); for range versions { warm() }; browser()`, 0},
		{"shadowed_range_binding", `for _, versions := range [][]string{nil} { _ = versions }; for range versions { warm() }; browser()`, 0},
		{"explicit_warmup_with_mutated_range", `emptyVersions(&versions); warm(); for range versions { warm() }; browser()`, 0},
	})
}

func TestBrowserPassWarmupPackageClosureRoutes(t *testing.T) {
	for _, declaration := range []struct{ name, source string }{
		{"stored_package_closure", "var route = func(t int) { %s }"},
		{"immediate_package_closure", "var initialized = func() bool { t := 0; %s; return true }()"},
		{"grouped_package_closure", "var (\nroute = func(t int) { %s }\n)"},
		{"composite_package_closure", "var routes = []func(int){func(t int) { %s }}"},
	} {
		t.Run(declaration.name, func(t *testing.T) {
			var sources []browserPassRangeMutationRoute
			for _, route := range []browserPassRangeMutationRoute{
				{"assignment_before_range", `versions = nil; for range versions { warm() }; browser()`, 1},
				{"pointer_write_before_range", `emptyVersions(&versions); for range versions { warm() }; browser()`, 1},
				{"range_binding_before_range", `for _, versions = range [][]string{nil} {}; for range versions { warm() }; browser()`, 1},
				{"stored_nested_closure_then_assignment", `run := func() { for range versions { warm() }; browser() }; versions = nil; run()`, 1},
				{"deferred_nested_closure_then_writer", `defer func() { for range versions { warm() }; browser() }(); defer func() { versions = nil }()`, 1},
				{"concurrent_nested_closure_then_assignment", `release := make(chan struct{}); done := make(chan struct{}); go func() { <-release; for range versions { warm() }; browser(); close(done) }(); versions = nil; close(release); <-done`, 1},
			} {
				body := `passes := []string{"59.0", "67.0"}; versions := []string{"59.0", "67.0"}; ` + route.body
				// The unrelated declaration must not supply the closure's range
				// mutation scope, even when it is the last named function seen.
				source := "package compile_test\nfunc unrelated() {}\n" + fmt.Sprintf(declaration.source, body)
				sources = append(sources, browserPassRangeMutationRoute{route.name, source, route.want})
			}
			checkBrowserPassRangeMutationSources(t, sources)
		})
	}
}

func TestBrowserPassWarmupMapMutationRoutes(t *testing.T) {
	checkBrowserPassRangeMutationRoutes(t, []browserPassRangeMutationRoute{
		{"cleared_map", `apis := map[string]bool{versions[0]: true}; clear(apis); for range apis { warm() }; browser()`, 1},
		{"deleted_map", `apis := map[string]bool{versions[0]: true}; delete(apis, versions[0]); for range apis { warm() }; browser()`, 1},
		{"cleared_map_alias", `apis := map[string]bool{versions[0]: true}; alias := apis; clear(alias); for range apis { warm() }; browser()`, 1},
		{"deleted_map_alias", `apis := map[string]bool{versions[0]: true}; alias := apis; delete(alias, versions[0]); for range apis { warm() }; browser()`, 1},
		{"cleared_map_helper", `apis := map[string]bool{versions[0]: true}; func(value map[string]bool) { clear(value) }(apis); for range apis { warm() }; browser()`, 1},
		{"stored_closure_then_map_clear", `apis := map[string]bool{versions[0]: true}; run := func() { for range apis { warm() }; browser() }; clear(apis); run()`, 1},
		{"deferred_closure_then_map_delete", `apis := map[string]bool{versions[0]: true}; defer func() { for range apis { warm() }; browser() }(); defer delete(apis, versions[0])`, 1},
		{"concurrent_closure_then_map_clear", `apis := map[string]bool{versions[0]: true}; release := make(chan struct{}); done := make(chan struct{}); go func() { <-release; for range apis { warm() }; browser(); close(done) }(); clear(apis); close(release); <-done`, 1},
		{"cleared_named_map", `type apiSet map[string]bool; apis := apiSet{versions[0]: true}; clear(apis); for range apis { warm() }; browser()`, 1},
		{"deleted_named_map_alias", `type apiSet = map[string]bool; apis := apiSet{versions[0]: true}; delete(apis, versions[0]); for range apis { warm() }; browser()`, 1},
	})
}

func TestBrowserPassWarmupDeclarationScopeBoundaryRoutes(t *testing.T) {
	const prepared = `passes := []string{"59.0", "67.0"}; versions := []string{"59.0"}; for range versions { warm() }; browser()`
	checkBrowserPassRangeMutationSources(t, []browserPassRangeMutationRoute{
		{"first_declaration_closure", "package compile_test\nvar route = func(t int) { " + prepared + " }", 0},
		{"first_declaration_immediate_closure", "package compile_test\nvar initialized = func() bool { t := 0; " + prepared + "; return true }()", 0},
		{"closure_after_unrelated_mutation", "package compile_test\nfunc unrelated() { versions := []string{\"59.0\"}; emptyVersions(&versions) }\nvar route = func(t int) { " + prepared + " }", 0},
		{"grouped_sibling_closure_mutation", "package compile_test\nfunc unrelated() {}\nvar (\nempty = func() { versions := []string{\"59.0\"}; emptyVersions(&versions) }; route = func(t int) { " + prepared + " }\n)", 0},
		{"named_function_after_closure_mutation", "package compile_test\nvar empty = func() { versions := []string{\"59.0\"}; emptyVersions(&versions) }\nfunc route(t int) { " + prepared + " }", 0},
		{"mutated_package_closure_with_explicit_warmup", `package compile_test
func unrelated() {}
var route = func(t int) { passes := []string{"59.0", "67.0"}; versions := []string{"59.0"}; versions = nil; warm(); for range versions { warm() }; browser() }`, 0},
	})
}

func TestBrowserPassWarmupLiteralBoundaryRoutes(t *testing.T) {
	checkBrowserPassRangeMutationRoutes(t, []browserPassRangeMutationRoute{
		{"direct_slice_literal", `for range []string{versions[0]} { warm() }; browser()`, 0},
		{"direct_array_literal", `for range [1]string{versions[0]} { warm() }; browser()`, 0},
		{"assigned_array_literal", `apis := [...]string{versions[0]}; for range apis { warm() }; browser()`, 0},
		{"cleared_slice_stays_nonempty", `clear(versions); for range versions { warm() }; browser()`, 0},
		{"cleared_map_with_explicit_warmup", `apis := map[string]bool{versions[0]: true}; clear(apis); warm(); for range apis { warm() }; browser()`, 0},
		{"deleted_map_with_explicit_warmup", `apis := map[string]bool{versions[0]: true}; delete(apis, versions[0]); warm(); for range apis { warm() }; browser()`, 0},
	})
}

func browserPassWarmupDirectoryProblems(t *testing.T, dir string) []string {
	t.Helper()
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
	var problems []string
	for _, file := range pkg.Files {
		problems = append(problems, browserPassWarmupProblems(positions, file)...)
	}
	return problems
}

func browserPassWarmupProblems(positions *token.FileSet, file *ast.File) []string {
	check := browserPassWarmupCheck{positions: positions, unevaluated: make(map[string]bool)}
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil || path != "unsafe" {
			continue
		}
		qualifier := "unsafe."
		if imported.Name != nil {
			qualifier = imported.Name.Name + "."
			if imported.Name.Name == "." {
				qualifier = ""
			}
		}
		for _, name := range []string{"Sizeof", "Alignof", "Offsetof"} {
			check.unevaluated[qualifier+name] = true
		}
	}
	for _, decl := range file.Decls {
		// Package initializers can contain closures too. Every declaration
		// supplies its own mutation scope, including nested captured writes.
		check.declaration = decl
		if fn, ok := decl.(*ast.FuncDecl); ok {
			check.functionBody(fn.Body)
		} else {
			check.expression(decl, false)
		}
	}
	return check.problems
}

type browserPassWarmupCheck struct {
	positions   *token.FileSet
	declaration ast.Decl
	unevaluated map[string]bool
	unsafeGoto  bool
	problems    []string
}

func (check *browserPassWarmupCheck) functionBody(body *ast.BlockStmt) {
	// Jump targets are local to this body. Range proofs still need the enclosing
	// declaration's assignments when a literal captures its version slice.
	unsafeGoto := check.unsafeGoto
	check.unsafeGoto = browserPassUnsafeGoto(body)
	check.block(body, false)
	check.unsafeGoto = unsafeGoto
}

// Source order proves warmup only for structured control flow. Fail closed for
// goto anywhere in this function, including jumps after a browser call: a
// backward edge can reach code that an earlier jump bypassed. Nested functions
// get their own proof. Do not try to infer reachability from condition names.
func browserPassUnsafeGoto(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	harmless := make(map[*ast.BranchStmt]bool)
	unsafe := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BlockStmt:
			for i := 0; i+1 < len(node.List); i++ {
				branch, ok := node.List[i].(*ast.IfStmt)
				if !ok || branch.Else != nil || len(branch.Body.List) != 1 {
					continue
				}
				jump, ok := branch.Body.List[0].(*ast.BranchStmt)
				label, nextIsLabel := node.List[i+1].(*ast.LabeledStmt)
				if ok && jump.Tok == token.GOTO && nextIsLabel && jump.Label.Name == label.Label.Name {
					// if condition { goto next }; next: statement skips no
					// statement on either path, regardless of the condition.
					harmless[jump] = true
				}
			}
		case *ast.BranchStmt:
			if node.Tok == token.GOTO && !harmless[node] {
				unsafe = true
			}
		}
		return true
	})
	return unsafe
}

// Follow statement order on the parent goroutine. Initialization in an
// optional branch, deferred call or callback does not warm its caller.
func (check *browserPassWarmupCheck) block(block *ast.BlockStmt, ready bool) bool {
	if block != nil {
		for _, stmt := range block.List {
			ready = check.statement(stmt, ready)
		}
	}
	return ready
}

func (check *browserPassWarmupCheck) statement(stmt ast.Stmt, ready bool) bool {
	if stmt == nil {
		return ready
	}
	switch stmt := stmt.(type) {
	case *ast.LabeledStmt:
		return check.statement(stmt.Stmt, ready)
	case *ast.BlockStmt:
		return check.block(stmt, ready)
	case *ast.IfStmt:
		ready = check.statement(stmt.Init, ready)
		check.expression(stmt.Cond, ready)
		thenReady := check.block(stmt.Body, ready)
		elseReady := check.statement(stmt.Else, ready)
		return thenReady && elseReady
	case *ast.RangeStmt:
		check.expression(stmt.X, ready)
		bodyReady := check.block(stmt.Body, ready)
		return ready || (check.nonemptyRange(stmt) && bodyReady)
	case *ast.ForStmt:
		ready = check.statement(stmt.Init, ready)
		check.expression(stmt.Cond, ready)
		check.block(stmt.Body, ready)
		check.statement(stmt.Post, ready)
		return ready
	case *ast.GoStmt, *ast.DeferStmt:
		check.expression(stmt, ready)
		return ready
	case *ast.SwitchStmt:
		ready = check.statement(stmt.Init, ready)
		check.expression(stmt.Tag, ready)
		check.block(stmt.Body, ready)
		return ready
	case *ast.TypeSwitchStmt:
		ready = check.statement(stmt.Init, ready)
		check.expression(stmt.Assign, ready)
		check.block(stmt.Body, ready)
		return ready
	case *ast.SelectStmt:
		check.block(stmt.Body, ready)
		return ready
	case *ast.CaseClause:
		for _, expr := range stmt.List {
			check.expression(expr, ready)
		}
		check.block(&ast.BlockStmt{List: stmt.Body}, ready)
		return ready
	case *ast.CommClause:
		check.statement(stmt.Comm, ready)
		check.block(&ast.BlockStmt{List: stmt.Body}, ready)
		return ready
	default:
		warms := check.expression(stmt, ready)
		return ready || warms
	}
}

func (check *browserPassWarmupCheck) expression(node ast.Node, ready bool) bool {
	if node == nil {
		return false
	}
	warms := false
	ast.Inspect(node, func(node ast.Node) bool {
		if binary, ok := node.(*ast.BinaryExpr); ok && (binary.Op == token.LAND || binary.Op == token.LOR) {
			// Only the left operand is guaranteed to run. Still check the
			// conditional right operand for unprepared browser pass calls.
			leftWarms := check.expression(binary.X, ready)
			check.expression(binary.Y, ready || leftWarms)
			warms = warms || leftWarms
			return false
		}
		if fn, ok := node.(*ast.FuncLit); ok {
			check.functionBody(fn.Body)
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := browserPassCallName(call.Fun)
		if check.unevaluated[name] {
			callee := ast.Unparen(call.Fun)
			if selector, ok := callee.(*ast.SelectorExpr); ok {
				callee = ast.Unparen(selector.X)
			}
			if ident, ok := callee.(*ast.Ident); ok && ident.Obj == nil {
				// Imports have no local AST object; a local shadow is an
				// ordinary call that still evaluates its arguments.
				// Layout queries may skip their operands. Discard warmup
				// evidence, but retain diagnostics conservatively: generic
				// variable-sized operands can still evaluate browser calls.
				for _, arg := range call.Args {
					check.expression(arg, ready)
				}
				return false
			}
		}
		switch name {
		case "gladehome.EnsureRoot", "l07CompileCases", "l11CompileCases", "l19CompileCases", "l21CompileCases":
			// These reviewed compile helpers call the LWC compiler synchronously.
			warms = true
		case "runBrowserPasses":
			if (!ready || check.unsafeGoto) && !browserPassPureCallback(call) {
				position := check.positions.Position(call.Pos())
				check.problems = append(check.problems, fmt.Sprintf("%s:%d: runBrowserPasses requires preceding EnsureRoot or serial compile", filepath.Base(position.Filename), position.Line))
			}
		}
		return true
	})
	return warms
}

func browserPassCallName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.SelectorExpr:
		return browserPassCallName(expr.X) + "." + expr.Sel.Name
	case *ast.IndexExpr:
		return browserPassCallName(expr.X)
	case *ast.IndexListExpr:
		return browserPassCallName(expr.X)
	case *ast.ParenExpr:
		return browserPassCallName(expr.X)
	}
	return ""
}

// The scheduler-order unit test returns arithmetic only and needs no toolchain.
// Any callback that invokes another function must obey the warmup contract.
func browserPassPureCallback(call *ast.CallExpr) bool {
	if len(call.Args) != 4 {
		return false
	}
	fn, ok := call.Args[3].(*ast.FuncLit)
	if !ok || len(fn.Body.List) != 1 {
		return false
	}
	if _, ok := fn.Body.List[0].(*ast.ReturnStmt); !ok {
		return false
	}
	pure := true
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.CallExpr, *ast.FuncLit:
			pure = false
		}
		return pure
	})
	return pure
}

// A serial compile loop only counts when it runs at least once. Recognize the
// nonempty version literals used by these conformance routes; reject unknown
// iteration counts and loops with an early exit rather than guessing.
func (check *browserPassWarmupCheck) nonemptyRange(loop *ast.RangeStmt) bool {
	expr := loop.X
	if name, ok := expr.(*ast.Ident); ok {
		if name.Obj == nil {
			return false
		}
		declaration, ok := name.Obj.Decl.(*ast.AssignStmt)
		if !ok || declaration.Pos() >= loop.Pos() || len(declaration.Lhs) != len(declaration.Rhs) {
			return false
		}
		expr = nil
		for index, lhs := range declaration.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok && ident.Obj == name.Obj {
				expr = declaration.Rhs[index]
			}
		}
		// A closure can run after writes that appear later in source order.
		// Require the same variable to stay unchanged throughout the enclosing
		// declaration, including nested functions. Taking its address also
		// invalidates this proof: aliases or callees can replace the slice.
		sameVariable := func(expr ast.Expr) bool {
			ident, ok := ast.Unparen(expr).(*ast.Ident)
			return ok && ident.Obj == name.Obj
		}
		unchanged := true
		ast.Inspect(check.declaration, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.AssignStmt:
				if node != declaration {
					for _, lhs := range node.Lhs {
						if sameVariable(lhs) {
							unchanged = false
						}
					}
				}
			case *ast.RangeStmt:
				if node.Tok == token.ASSIGN && (sameVariable(node.Key) || sameVariable(node.Value)) {
					unchanged = false
				}
			case *ast.UnaryExpr:
				if node.Op == token.AND && sameVariable(node.X) {
					unchanged = false
				}
			}
			return unchanged
		})
		if !unchanged {
			return false
		}
	}
	literal, ok := expr.(*ast.CompositeLit)
	if !ok || len(literal.Elts) == 0 {
		return false
	}
	// Only explicit slice and array literals provide a length proof here.
	// Map contents can disappear through clear, delete, aliases or callees
	// without assigning the range variable or taking its address. Named
	// composite types can also be maps; require a separate warmup for them.
	if _, ok := literal.Type.(*ast.ArrayType); !ok {
		return false
	}
	guaranteed := true
	ast.Inspect(loop.Body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		switch node.(type) {
		case *ast.BranchStmt, *ast.ReturnStmt:
			guaranteed = false
		}
		return guaranteed
	})
	return guaranteed
}
