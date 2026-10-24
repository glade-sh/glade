package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestAsyncReturnIdentityAnonymousNamedAndTestRunnerNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/async_return_identity"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 61 || strings.Join(rows[0], ",") != "source_api,route,operation,observation" {
		t.Fatalf("incomplete native return identity matrix: %v", err)
	}
	seen := map[string]bool{}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, typ := range index.Types {
				if typ.EffectiveAPIVersion != api {
					t.Fatalf("native source API %s; %s uses %s", api, typ.Name, typ.EffectiveAPIVersion)
				}
			}
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				route, operation := row[1], row[2]
				key := api + "/" + route + "/" + operation
				if seen[key] || (route != "anonymous" && route != "named" && route != "isTest") {
					t.Fatalf("invalid native return identity cell: %s", key)
				}
				seen[key] = true
				t.Run(route+"/"+operation, func(t *testing.T) {
					var message string
					if route == "isTest" {
						run := Run(index, Options{SelectedClasses: []string{"AsyncReturnIdentityProbeTest"}, SelectedMethod: strings.ReplaceAll(operation, "-", "_"), NoDiskCache: true, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("missing test runner route: %+v", run.Summary())
						}
						problem := run.Suites[0].Cases[0].Problem
						if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
							t.Fatalf("missing native assertion transport: %+v", problem)
						}
						message = problem.Message
					} else {
						source := "System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(AsyncReturnIdentityProbe.observe('" + operation + "')));"
						if operation == "boxed-controls" {
							source = "System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(AsyncReturnIdentityProbe.controls()));"
						}
						if operation == "direct-controls" {
							source = "System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(AsyncReturnIdentityProbe.directControls()));"
						}
						if operation == "member-controls" {
							source = "System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(AsyncReturnIdentityProbe.memberControls()));"
						}
						if operation == "typed-return-controls" {
							source = "System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(AsyncReturnIdentityProbe.typedReturnControls()));"
						}
						if operation == "object-equality-controls" {
							source = "System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(AsyncReturnIdentityProbe.objectEqualityControls()));"
						}
						if route == "anonymous" {
							body, err := os.ReadFile(filepath.Join(fixture, operation+".apex"))
							if err != nil {
								t.Fatal(err)
							}
							source = string(body)
						}
						if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
							t.Fatalf("native accepted anonymous: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						result, err := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true}).execute(program)
						if err != nil {
							t.Fatal(err)
						}
						for _, line := range result.Debug {
							if strings.HasPrefix(line, "OBS|") {
								message = line
							}
						}
					}
					_, observation, found := strings.Cut(message, "OBS|")
					var got, want map[string]any
					if !found || json.Unmarshal([]byte(observation), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
						t.Fatalf("invalid observation: %q", message)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%v; Glade=%v", want, got)
					}
				})
			}
		})
	}
	if len(seen) != 60 {
		t.Fatalf("native matrix covered %d cells, want 60", len(seen))
	}
}
