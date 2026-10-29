package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// These former carries are active assertions. The current native answers
// distinguish compilation from an uncatchable runtime exception at every API.
func TestSourceSyntaxBoundaryNativeConformance(t *testing.T) {
	var data struct {
		APIVersions []string `json:"apiVersions"`
		Helper      string
		Rows        []struct {
			ID, Code, Diagnostic, Exception  string
			Compiled, Success, NamedCompiled bool
		}
	}
	raw, err := os.ReadFile("testdata/conformance/source_syntax_boundary.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 4 || strings.Join(data.APIVersions, ",") != "62.0,65.0,66.0,67.0,68.0" || data.Helper == "" {
		t.Fatal("invalid source-syntax boundary capture")
	}
	checkCompilation := func(t *testing.T, accepted bool, want string, diagnostics []diagnostic.Diagnostic, compileErr error) {
		t.Helper()
		var messages []string
		for _, d := range diagnostics {
			if d.Severity == diagnostic.Error {
				message := d.NativeMessage
				if message == "" {
					message = d.Message
				}
				messages = append(messages, message)
			}
		}
		if accepted {
			if len(messages) != 0 || compileErr != nil {
				t.Fatalf("native compiles; Glade diagnostics=%v compile=%v", messages, compileErr)
			}
			return
		}
		if len(messages) != 1 || messages[0] != want {
			t.Fatalf("native rejection <%s>; Glade diagnostics=%v compile=%v", want, messages, compileErr)
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "A01-boundary", api)
			build := func(root string, sources map[string]string) typesys.Index {
				var paths []string
				for name, source := range sources {
					path := filepath.Join(root, name+".cls")
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					paths = append(paths, path)
				}
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, gladeschema.Schema{})
			}
			base := build(t.TempDir(), map[string]string{"P": data.Helper})
			checkCompilation(t, true, "", append(base.Diagnostics, sema.Analyze(base).Diagnostics...), nil)
			org := orgFromIndex(base)
			runner := newConformanceRunner(t, base, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					source := "P pq = new P();\n" + row.Code
					t.Run("anonymous", func(t *testing.T) {
						counts.TrackKind(t, "anonymous", "exact", 1)
						analysis := sema.AnalyzeAnonymous(base, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						checkCompilation(t, row.Compiled, row.Diagnostic, analysis.Diagnostics, compileErr)
						if !row.Compiled {
							return
						}
						_, err := runner.execute(program)
						if row.Success {
							if err != nil {
								t.Fatal(err)
							}
							return
						}
						observed := ""
						if err != nil {
							observed = err.Error()
							var runtimeErr *vm.RuntimeError
							if errors.As(err, &runtimeErr) {
								observed = runtimeErr.ExceptionMessage()
								if runtimeErr.Type != "" && !strings.HasPrefix(observed, runtimeErr.Type+": ") {
									observed = runtimeErr.Type + ": " + observed
								}
							}
						}
						if observed != row.Exception {
							t.Fatalf("native uncatchable exception <%s>; Glade <%s>", row.Exception, observed)
						}
					})
					t.Run("namedCompilation", func(t *testing.T) {
						counts.TrackKind(t, "named-compile", "exact", 1)
						index := build(t.TempDir(), map[string]string{"P": data.Helper,
							"SourceSyntaxBoundary": "public class SourceSyntaxBoundary { public static void observe(){\n" + source + "\n} }"})
						analysis := sema.Analyze(index)
						checkCompilation(t, row.NamedCompiled, row.Diagnostic, append(index.Diagnostics, analysis.Diagnostics...), nil)
						// The check-only capture does not observe named runtime behavior.
					})
				})
			}
		})
	}
}
