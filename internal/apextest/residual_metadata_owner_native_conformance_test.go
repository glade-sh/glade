package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestResidualMetadataOwnerNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/residual_metadata_owner"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 || strings.Join(rows[0], "\t") != "source_api\troute\tobservation" {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}
	capture, err := os.ReadFile(filepath.Join(fixture, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 3 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") || seen[row[0]+row[1]] {
			t.Fatal("invalid native route")
		}
		seen[row[0]+row[1]] = true
		api, route := row[0], row[1]
		t.Run(api+"/"+route, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GLADE_HOME", filepath.Join(root, ".glade"))
			for _, tree := range []string{"provider", "project"} {
				source := filepath.Join(fixture, tree)
				err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					name, err := filepath.Rel(source, path)
					if err != nil {
						return err
					}
					if tree == "project" && name != "glade.yml" && !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(name), sha256.Sum256(body))) {
						t.Fatalf("captured source changed: %s", name)
					}
					rendered := strings.ReplaceAll(string(body), "62.0", api)
					if tree == "project" && name != "glade.yml" && !strings.Contains(string(capture), fmt.Sprintf("source_api_sha256:%s:%s=%x\n", api, filepath.ToSlash(name), sha256.Sum256([]byte(rendered)))) {
						t.Fatalf("missing native deployment input: %s/%s", api, name)
					}
					writeFile(t, filepath.Join(root, tree, name), rendered)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			index := loadTestIndex(t, filepath.Join(root, "project"))
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native deployed; Glade=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			if org.Namespace != "gladens002" || len(org.Objects["gladens001__Ns113bFlag__mdt"].Records) != 1 {
				t.Fatal("provider type or consumer record failed to load")
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, temperature := range []string{"cold", "warm"} {
				t.Run(temperature, func(t *testing.T) {
					var message string
					if route == "isTest" {
						run := Run(index, Options{SelectedClasses: []string{"ResidualMetadataOwnerTest"}, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
							t.Fatalf("missing observation: %+v", run.Summary())
						}
						problem := run.Suites[0].Cases[0].Problem
						if problem.Type != "System.AssertException" {
							t.Fatalf("invalid observation transport: %+v", problem)
						}
						message = problem.Message
					} else {
						source, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
						if err != nil {
							t.Fatal(err)
						}
						if !strings.Contains(string(capture), fmt.Sprintf("anonymous_api_sha256:%s=%x\n", api, sha256.Sum256(source))) {
							t.Fatal("missing anonymous deployment input")
						}
						if analysis := sema.AnalyzeAnonymous(index, string(source), api); analysis.HasErrors() {
							t.Fatal(analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						// Native anonymous execution occurred in the consumer dev org.
						_, err = base.execute(program, func(machine *vm.VM) { machine.SetCurrentNamespace(org.Namespace) })
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
							t.Fatalf("missing observation: %v", err)
						}
						message = runtimeErr.ExceptionMessage()
					}
					_, payload, found := strings.Cut(message, "OWNER_BOUNDARY|")
					var got, want map[string]any
					if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
						t.Fatal("invalid native observation")
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%v; Glade=%v", want, got)
					}
				})
			}
		})
	}
}
