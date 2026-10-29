package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRuntimeSettingBoundariesNativeConformance(t *testing.T) {
	runSettingsJSONBoundaryNativeConformance(t, "runtime_setting_boundaries", "RuntimeSettingCases", "RuntimeSettingBoundaryProbe")
}

func TestJSONAccessRuntimeBoundariesNativeConformance(t *testing.T) {
	runSettingsJSONBoundaryNativeConformance(t, "json_access_runtime_boundaries", "RuntimeJSONCases", "RuntimeJSONBoundaryProbe")
}

func TestRuntimeMissingInputBoundariesNativeConformance(t *testing.T) {
	runSettingsJSONBoundaryNativeConformance(t, "runtime_missing_input_boundaries", "RuntimeMissingInputCases", "RuntimeMissingInputProbe")
}

func TestSchemaTypeAliasesNativeConformance(t *testing.T) {
	runSettingsJSONBoundaryNativeConformance(t, "schema_type_alias_boundaries", "RuntimeSchemaTypeAliases", "RuntimeSchemaAliasProbe")
}

func TestSchemaTypeFactoryNativeConformance(t *testing.T) {
	runSettingsJSONBoundaryNativeConformance(t, "schema_type_factory_boundaries", "RuntimeSchemaFactoryCases", "RuntimeSchemaFactoryProbe")
}

func runSettingsJSONBoundaryNativeConformance(t *testing.T, family, observer, probe string) {
	t.Helper()
	fixture := filepath.Join("testdata", "conformance", family)
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatal("expected API 62/67 anonymous and named observations")
	}
	header := strings.Join(rows[0], ",")
	mixedSource := header == "source_api,caller_api,route,observation"
	if !mixedSource && header != "source_api,route,observation" {
		t.Fatal("expected API 62/67 anonymous and named observations")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) < 3 {
			t.Fatalf("invalid native row: %v", row)
		}
		callerAPI := row[0]
		if mixedSource {
			if len(row) != 4 || (row[1] != "62.0" && row[1] != "67.0") || row[0] == row[1] {
				t.Fatalf("invalid mixed-source native row: %v", row)
			}
			callerAPI = row[1]
			row = []string{row[0], row[2], row[3]}
		}
		if len(row) != 3 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") || seen[row[0]+"/"+row[1]] {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[row[0]+"/"+row[1]] = true
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
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
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				source := strings.ReplaceAll(string(body), "62.0", row[0])
				if name == "sfdx-project.json" || filepath.Base(name) == probe+".cls-meta.xml" {
					source = strings.ReplaceAll(source, row[0], callerAPI)
				}
				writeFile(t, filepath.Join(root, name), source)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			body := "System.assert(false, 'OBS|' + JSON.serialize(" + observer + ".observe()));"
			states := []string{"first", "repeat"}
			var execute func(*testing.T, string) string
			if row[1] == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: callerAPI})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = callerAPI
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				execute = func(t *testing.T, _ string) string {
					t.Helper()
					_, err := runner.execute(program)
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("expected assertion observation: %v", err)
					}
					return fault.ExceptionMessage()
				}
			} else {
				states = []string{"cold", "disk_restore"}
				t.Cleanup(EnableDiskCacheForTesting())
				t.Cleanup(InvalidateRuntimeCaches)
				t.Cleanup(ResetPerfCounters)
				execute = func(t *testing.T, state string) string {
					t.Helper()
					InvalidateRuntimeCaches()
					run := Run(index, Options{SelectedClasses: []string{probe}, SelectedMethod: "observe", Parallelism: 1, PerfCounters: true})
					phases := SnapshotPerfCounters().Phases
					if state == "cold" {
						if phases.CacheMisses != 1 || phases.DiskCacheHits != 0 || phases.MemoryCacheHits != 0 {
							t.Errorf("cold project route did not build a fresh runtime: %+v", phases)
						}
					} else if phases.DiskCacheHits != 1 || phases.MemoryCacheHits != 0 || phases.CacheMisses != 0 {
						t.Errorf("disk_restore project route did not restore the runtime from disk: %+v", phases)
					}
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("expected one native observation: %+v", run.Summary())
					}
					result := run.Suites[0].Cases[0]
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("missing assertion transport: %+v", result.Problem)
					}
					return result.Problem.Message
				}
			}
			for _, state := range states {
				t.Run(state, func(t *testing.T) {
					_, observed, ok := strings.Cut(execute(t, state), "OBS|")
					var got, want any
					if !ok || json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
						t.Fatalf("invalid observation: %q", observed)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("native=%s; Glade=%s", row[2], observed)
					}
				})
			}
		})
	}
}
