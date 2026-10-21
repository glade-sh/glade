package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// This directed capture observes final local bindings through executeAnonymous.
// Transient helper declarations are retained, without claiming named runtime
// parity. The original try/catch blocks distinguish uncaught final failures.
func TestFinalLocalScopeNativeConformance(t *testing.T) {
	var table struct {
		APIVersions []string `json:"apiVersions"`
		Rows        []struct {
			ID, Anonymous string
			Answers       map[string]struct {
				Compiled, Success bool
				CompileProblem    *string
				ExceptionMessage  *string
			}
		}
	}
	raw, err := os.ReadFile("testdata/conformance/final_local_scope.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	if strings.Join(table.APIVersions, ",") != "62.0,68.0" || len(table.Rows) != 15 {
		t.Fatal("invalid final-local native scope table")
	}
	seen := map[string]bool{}
	for _, row := range table.Rows {
		if row.ID == "" || seen[row.ID] || row.Anonymous == "" || len(row.Answers) != 2 {
			t.Fatal("invalid final-local native row")
		}
		seen[row.ID] = true
	}
	for _, api := range table.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "A01-final-local", api)
			for _, row := range table.Rows {
				t.Run(row.ID, func(t *testing.T) {
					counts.TrackKind(t, "anonymous", "exact", 1)
					answer, ok := row.Answers[api]
					if !ok || !answer.Compiled || answer.CompileProblem != nil || (!answer.Success && answer.ExceptionMessage == nil) {
						t.Fatal("invalid native compiled answer")
					}
					// Match the existing source-syntax fixture path: keep the
					// anonymous body offsets, loading transient declarations into
					// the linked request index rather than dropping their methods.
					parsed := apexast.NewParser().ParseSource("FinalLocalAnonymous.cls", row.Anonymous)
					body := []byte(row.Anonymous)
					root := t.TempDir()
					var paths []string
					for _, decl := range parsed.Declarations {
						if decl.Kind != apexast.DeclarationClass {
							continue
						}
						start, end := decl.Range.Start.Offset, decl.Range.End.Offset
						if start < 0 || end <= start || end > len(body) {
							t.Fatal("invalid transient declaration range")
						}
						path := filepath.Join(root, decl.Name+".cls")
						writeFile(t, path, row.Anonymous[start:end])
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						paths = append(paths, path)
						for i := start; i < end; i++ {
							if body[i] != '\n' && body[i] != '\r' {
								body[i] = ' '
							}
						}
					}
					if len(paths) > 0 && (apexast.Result{Diagnostics: parsed.Diagnostics}).HasErrors() {
						t.Fatalf("native compiles; transient declaration parse: %v", parsed.Diagnostics)
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
					declarations := sema.AnalyzeAnonymousDeclarations(index)
					analysis := sema.AnalyzeAnonymous(index, string(body), api)
					program, compileErr := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
					if index.HasErrors() || declarations.HasErrors() || analysis.HasErrors() || compileErr != nil {
						t.Fatalf("native compiles; Glade parse=%v declarations=%v semantics=%v compile=%v", index.Diagnostics, declarations.Diagnostics, analysis.Diagnostics, compileErr)
					}
					org := orgFromIndex(index)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					_, runErr := runner.execute(program)
					if answer.Success {
						if runErr != nil {
							t.Fatalf("native completes all value assertions; Glade: %v", runErr)
						}
						return
					}
					observed := ""
					if runErr != nil {
						observed = runErr.Error()
						var runtimeErr *vm.RuntimeError
						if errors.As(runErr, &runtimeErr) {
							observed = runtimeErr.ExceptionMessage()
							if runtimeErr.Type != "" && !strings.HasPrefix(observed, runtimeErr.Type+": ") {
								observed = runtimeErr.Type + ": " + observed
							}
						}
					}
					if observed != *answer.ExceptionMessage {
						t.Fatalf("native uncaught <%s>; Glade <%s>", *answer.ExceptionMessage, observed)
					}
				})
			}
		})
	}
}
