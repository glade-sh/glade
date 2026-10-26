package scripts

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Check skip sites against the same policy that consumes Apex shard events.
// Normal shard runs do not select benchmarks or enable testing.Short.
func TestCIApexSkipSitesMatchResultPolicy(t *testing.T) {
	const pkg = "github.com/glade-sh/glade/internal/apextest"
	files, err := filepath.Glob("../internal/apextest/*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("Apex test discovery: %v (%d files)", err, len(files))
	}
	for _, path := range files {
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || strings.HasPrefix(fn.Name.Name, "Benchmark") {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if branch, ok := node.(*ast.IfStmt); ok {
					if call, ok := branch.Cond.(*ast.CallExpr); ok {
						if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
							if receiver, ok := selector.X.(*ast.Ident); ok && receiver.Name == "testing" && selector.Sel.Name == "Short" {
								return false
							}
						}
					}
				}
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (selector.Sel.Name != "Skip" && selector.Sel.Name != "Skipf" && selector.Sel.Name != "SkipNow") {
					return true
				}
				position := positions.Position(call.Pos())
				t.Run(fn.Name.Name+"/"+filepath.Base(path)+":"+strconv.Itoa(position.Line), func(t *testing.T) {
					if len(call.Args) != 1 {
						t.Fatal("skip requires an exact validator-approved reason")
					}
					literal, ok := call.Args[0].(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						t.Fatal("skip reason must be a validator-approved string literal")
					}
					reason, err := strconv.Unquote(literal.Value)
					if err != nil {
						t.Fatal(err)
					}
					dir := t.TempDir()
					events := filepath.Join(dir, "events.json")
					metadata := filepath.Join(dir, "metadata.tsv")
					output := filepath.Join(dir, "summary.json")
					var body strings.Builder
					encoder := json.NewEncoder(&body)
					for _, event := range []map[string]string{
						{"Action": "output", "Package": pkg, "Test": fn.Name.Name, "Output": "    " + filepath.Base(path) + ":" + strconv.Itoa(position.Line) + ": " + reason + "\n"},
						{"Action": "skip", "Package": pkg, "Test": fn.Name.Name},
						{"Action": "pass", "Package": pkg},
					} {
						if err := encoder.Encode(event); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.WriteFile(events, []byte(body.String()), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(metadata, []byte(pkg+"\thas-tests\n"), 0600); err != nil {
						t.Fatal(err)
					}
					cmd := exec.Command("bash", "./ci-go-test.sh", "package-summary", "apextest", "Apex skip policy", events, output, metadata, pkg)
					for _, entry := range os.Environ() {
						if !strings.HasPrefix(entry, "GLADE_DATAWEAVE_APEX_TEST_HOME=") {
							cmd.Env = append(cmd.Env, entry)
						}
					}
					if result, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("%s: skip is rejected by Apex result validation: %v\n%s", position, err, result)
					}
				})
				return true
			})
		}
	}
}
