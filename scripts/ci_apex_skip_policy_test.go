package scripts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

// Pins tie the static skip audit to the exact source whose leaf identities
// were observed and recorded. Changes to a skip-bearing file require re-audit.
func case345ValidateSourcePin(pins map[string]string, name string, body []byte) error {
	want, ok := pins[name]
	if !ok || want != fmt.Sprintf("%x", sha256.Sum256(body)) {
		return fmt.Errorf("native gap source %s is missing or changed", name)
	}
	return nil
}

func case345SourceNeedsPin(body []byte) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "source.go", body, 0)
	if err != nil {
		return false, err
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && (id.Name == "case345Skip" || id.Name == "Case345Skip") {
			found = true
		}
		return true
	})
	return found, nil
}

func case345ValidatePinnedInventory(pins map[string]string, seen map[string]bool) error {
	for name := range pins {
		if !seen[name] {
			return fmt.Errorf("pinned native gap source missing: %s", name)
		}
	}
	return nil
}

// Check skip sites against the same policy that consumes Apex shard events.
// Normal shard runs do not select benchmarks or enable testing.Short.
func TestCIApexSkipSitesMatchResultPolicy(t *testing.T) {
	const pkg = "github.com/glade-sh/glade/internal/apextest"
	files, err := filepath.Glob("../internal/apextest/*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("Apex test discovery: %v (%d files)", err, len(files))
	}
	pinData, err := os.ReadFile("ci-apex-native-gap-sources.json")
	if err != nil {
		t.Fatal(err)
	}
	var pins map[string]string
	if err := json.Unmarshal(pinData, &pins); err != nil {
		t.Fatal(err)
	}
	var gapNames []string
	gapData, err := os.ReadFile("ci-apex-native-gaps.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(gapData, &gapNames); err != nil {
		t.Fatal(err)
	}

	seenPins := map[string]bool{}
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		_, pinned := pins[name]
		needsPin, err := case345SourceNeedsPin(body)
		if err != nil {
			t.Fatal(err)
		}
		if pinned {
			seenPins[name] = true
		}
		if pinned || needsPin || strings.Contains(string(body), "captured native observation has a known local gap") {
			if err := case345ValidateSourcePin(pins, name, body); err != nil {
				t.Fatal(err)
			}
		}
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
					testIdentity := fn.Name.Name
					expectRejected := false
					if reason == "captured native observation has a known local gap" {
						if fn.Name.Name != "case345Skip" {
							t.Fatal("native gap skip must use exact-identity guard")
						}
						if len(gapNames) == 0 {
							testIdentity = "TestSynthetic/62.0/unregistered"
							expectRejected = true
						} else {
							testIdentity = gapNames[0]
							if testIdentity == "" {
								t.Fatal("native gap has an empty leaf")
							}
						}
					}
					var body strings.Builder
					encoder := json.NewEncoder(&body)
					for _, event := range []map[string]string{
						{"Action": "output", "Package": pkg, "Test": testIdentity, "Output": "    " + filepath.Base(path) + ":" + strconv.Itoa(position.Line) + ": " + reason + "\n"},
						{"Action": "skip", "Package": pkg, "Test": testIdentity},
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
					result, err := cmd.CombinedOutput()
					if expectRejected {
						if err == nil {
							t.Fatalf("%s: empty gap ledger accepted an unregistered skip", position)
						}
					} else if err != nil {
						t.Fatalf("%s: skip is rejected by Apex result validation: %v\n%s", position, err, result)
					}
				})
				return true
			})
		}
	}
	if err := case345ValidatePinnedInventory(pins, seenPins); err != nil {
		t.Fatal(err)
	}
}
