package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestPersonAccountBoundaryReviewNativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/person_account_followups"
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	f, err := os.Open(filepath.Join(fixture, "review2", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1]
		want[key] = append(want[key], row[2]+"|"+row[3])
	}
	diagnosticFile, err := os.Open(filepath.Join(fixture, "review2", "diagnostics.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer diagnosticFile.Close()
	diagnosticReader := csv.NewReader(diagnosticFile)
	diagnosticReader.Comma = '\t'
	diagnostics, err := diagnosticReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	wantDiagnostics := map[string][]string{}
	for _, row := range diagnostics[1:] {
		wantDiagnostics[row[0]] = append(wantDiagnostics[row[0]], row[2]+"|"+row[3])
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				if !strings.HasPrefix(rel, "force-app/") && !strings.HasPrefix(rel, "config/") {
					return nil
				}
				source := readFixtureFile(t, path)
				if strings.HasSuffix(rel, ".cls-meta.xml") {
					source = strings.ReplaceAll(strings.ReplaceAll(source, "62.0", api), "67.0", api)
				}
				writeFile(t, filepath.Join(root, rel), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			index := loadTestIndex(t, root)
			compare := func(route, payload string) {
				t.Helper()
				got := strings.Split(payload, "~")
				expected := want[api+"/"+route]
				if len(expected) != 42 || len(got) != len(expected) {
					t.Fatalf("%s: rows got %d want %d", route, len(got), len(expected))
				}
				for i := range got {
					if got[i] != expected[i] {
						t.Errorf("%s: got %q want %q", route, got[i], expected[i])
					}
				}
			}
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(readFixtureFile(t, filepath.Join(fixture, "review2", "anonymous.apex")), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if strings.HasPrefix(line, "BOUNDARY|") {
						compare("anon", strings.TrimPrefix(line, "BOUNDARY|"))
						return
					}
				}
				t.Fatal("missing capture")
			})
			t.Run("diagnostics", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(readFixtureFile(t, filepath.Join(fixture, "review2", "diagnostics.apex")), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, line := range result.Debug {
					if strings.HasPrefix(line, "BOUNDARY_MESSAGE|") {
						got = append(got, strings.TrimPrefix(line, "BOUNDARY_MESSAGE|"))
					}
				}
				expected := wantDiagnostics[api]
				if len(got) != 8 || len(expected) != len(got) {
					t.Fatalf("diagnostic rows got %d want %d", len(got), len(expected))
				}
				for i := range got {
					if got[i] != expected[i] {
						t.Errorf("diagnostic got %q want %q", got[i], expected[i])
					}
				}
			})
			for _, route := range []string{"project", "warm", "disk_restore"} {
				t.Run(route, func(t *testing.T) {
					if route == "disk_restore" {
						InvalidateRuntimeCaches()
					}
					result := Run(index, Options{SelectedClasses: []string{"PersonBoundaryReviewTest"}, Parallelism: 1, PerfCounters: route == "disk_restore"})
					if route == "disk_restore" && SnapshotPerfCounters().Phases.DiskCacheHits != 1 {
						t.Fatalf("disk restore was not used: %+v", SnapshotPerfCounters().Phases)
					}
					if _, ok := tryLoadDiskRuntime(index); !ok {
						t.Fatal("runtime disk cache was not populated")
					}
					for _, suite := range result.Suites {
						for _, c := range suite.Cases {
							if c.Problem == nil {
								t.Fatal("missing assertion capture")
							}
							_, payload, ok := strings.Cut(c.Problem.Message, "Actual: ")
							if !ok {
								t.Fatalf("unexpected failure: %s", c.Problem.Message)
							}
							compare("test", payload)
							return
						}
					}
					t.Fatal("missing test")
				})
			}
		})
	}
}
