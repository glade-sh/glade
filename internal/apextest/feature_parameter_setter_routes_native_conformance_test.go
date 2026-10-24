package apextest

import (
	"context"
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

func TestFeatureParameterSetterIsTestNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	const fixture = "testdata/conformance/feature_parameters"
	file, err := os.Open(filepath.Join(fixture, "setter_routes/org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 || strings.Join(rows[0], ",") != "source_api,route,operation,observation" {
		t.Fatalf("incomplete native setter matrix: %v", rows)
	}
	wants := make(map[string]map[string]map[string]any)
	for _, row := range rows[1:] {
		if len(row) != 4 || row[1] != "isTest" || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native setter row: %v", row)
		}
		if wants[row[0]] == nil {
			wants[row[0]] = make(map[string]map[string]any)
		}
		if wants[row[0]][row[2]] != nil {
			t.Fatalf("duplicate native setter row: %v", row)
		}
		var want map[string]any
		if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
			t.Fatal(err)
		}
		wants[row[0]][row[2]] = want
	}
	for _, api := range []string{"62.0", "67.0"} {
		root := copyFeatureParameterProject(t, fixture, api)
		index := loadTestIndex(t, root)
		if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native setters compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
		}
		for _, route := range []struct {
			name        string
			parallelism int
		}{{"serial", 1}, {"parallel", 2}} {
			for _, mode := range []string{"built", "memory", "disk"} {
				t.Run(api+"/isTest/"+route.name+"/"+mode, func(t *testing.T) {
					if mode != "memory" {
						InvalidateRuntimeCaches()
					}
					ResetPerfCounters()
					opts := Options{
						SelectedClasses: []string{"FeatureParameterSetterRouteTest"},
						NoDiskCache:     mode != "disk", Parallelism: route.parallelism,
						ParallelMethods: route.parallelism > 1, RestoredRuntimeMultiWorker: true,
						PerfCounters:     true,
						MethodDurationMS: map[string]int64{"FeatureParameterSetterRouteTest.setValues": 100},
					}
					run := RunCasesContext(context.Background(), index, opts, Discover(index, opts))
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 4 {
						t.Fatalf("expected four native setter observations: %+v", run.Summary())
					}
					seen := map[string]bool{}
					for _, result := range run.Suites[0].Cases {
						t.Run(result.MethodName, func(t *testing.T) {
							want, ok := wants[api][result.MethodName]
							if !ok || seen[result.MethodName] {
								t.Fatalf("unexpected setter method %q", result.MethodName)
							}
							seen[result.MethodName] = true
							if result.Problem == nil {
								t.Fatalf("missing native exception/observation: %+v", result)
							}
							if expected, ok := want["exception"].(string); ok {
								if result.Problem.Type != expected {
									t.Fatalf("error=%+v, want %s", result.Problem, expected)
								}
								return
							}
							if !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing assertion transport: %+v", result.Problem)
							}
							_, observed, _ := strings.Cut(result.Problem.Message, "OBS|")
							var got map[string]any
							if err := json.Unmarshal([]byte(observed), &got); err != nil {
								t.Fatalf("invalid setter observation %q: %v", observed, err)
							}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("native=%v; Glade=%v", want, got)
							}
						})
					}
					phases := SnapshotPerfCounters().Phases
					if mode == "memory" && phases.MemoryCacheHits != 1 {
						t.Fatalf("expected memory cache hit: %+v", phases)
					}
					if mode == "disk" && phases.DiskCacheHits != 1 {
						t.Fatalf("expected disk cache hit: %+v", phases)
					}
					if mode == "built" {
						key, entry, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
						if err != nil {
							t.Fatal(err)
						}
						persistDiskRuntimeWithPerf(index, nil, key, entry.restored.CloneOrg(), entry, newRunPerfCounters(true))
					}
				})
			}
		}
	}
}

func TestFeatureParameterRuntimeTemplateAnonymousProjectIsolation(t *testing.T) {
	const fixture = "testdata/conformance/feature_parameters"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/anonymous", func(t *testing.T) {
			root := copyFeatureParameterProject(t, fixture, api)
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			before := append(org.Metadata.FeatureParameters[:0:0], org.Metadata.FeatureParameters...)
			first, sibling := runner.newMachine(), runner.newMachine()
			set, err := featureParameterOperationProgram(fixture, "set", api)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := first.Execute(set); err != nil {
				t.Fatal(err)
			}
			for name, metadata := range map[string]any{"source": org.Metadata.FeatureParameters, "template": runner.org.Org.Metadata.FeatureParameters, "sibling": sibling.Org.Metadata.FeatureParameters} {
				if !reflect.DeepEqual(metadata, before) {
					t.Errorf("setter changed %s metadata: %v, want %v", name, metadata, before)
				}
			}
			// Reinstalling the sibling org and creating a later row must both
			// rebuild defaults from metadata that the setter could not mutate.
			sibling.SetRuntimeTemplateOrg(sibling.Org)
			check, err := vm.CompileAnonymousWithOptions(`
System.assertEquals(true, System.FeatureManagement.checkPackageBooleanValue('Enabled'));
System.assertEquals(11, System.FeatureManagement.checkPackageIntegerValue('Quota'));
System.assertEquals(Date.newInstance(2020, 2, 2), System.FeatureManagement.checkPackageDateValue('StartDate'));
`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := sibling.Execute(check); err != nil {
				t.Errorf("existing sibling: %v", err)
			}
			if _, err := runner.execute(check); err != nil {
				t.Errorf("later conformance row: %v", err)
			}
		})
	}
}
