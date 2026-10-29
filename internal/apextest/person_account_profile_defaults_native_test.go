package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestPersonAccountProfileDefaultsNativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/person_account_profile_defaults"
	native, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(native)), "\n")[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 5 {
			t.Fatalf("invalid native row %q", line)
		}
		key := row[0] + "/" + row[1]
		if want[key] == nil {
			want[key] = map[string]string{}
		}
		if _, exists := want[key][row[2]]; exists {
			t.Fatalf("duplicate native row %q", line)
		}
		want[key][row[2]] = row[3] + "|" + row[4]
	}
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
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
				if !(strings.HasPrefix(rel, "force-app/") || strings.HasPrefix(rel, "config/")) {
					return nil
				}
				source := readFixtureFile(t, path)
				if strings.HasSuffix(path, ".cls-meta.xml") {
					source = strings.ReplaceAll(source, "67.0", api)
				}
				writeFile(t, filepath.Join(root, rel), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			index := loadTestIndex(t, root)
			compare := func(t *testing.T, route string, rows []string) {
				t.Helper()
				expected := want[api+"/"+route]
				count := 6
				if route == "istest" {
					count = 24
				}
				if len(expected) != count || len(rows) != count {
					t.Fatalf("%s rows: native=%d local=%d (%v)", route, len(expected), len(rows), rows)
				}
				seen := map[string]bool{}
				for _, row := range rows {
					name, value, ok := strings.Cut(row, "|")
					if !ok || seen[name] || expected[name] != value {
						t.Errorf("%s: native %q; local %q", name, expected[name], value)
					}
					seen[name] = true
				}
			}
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(readFixtureFile(t, filepath.Join(fixture, "anonymous.apex")), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatalf("%v; debug: %v", err, result.Debug)
				}
				var rows []string
				for _, line := range result.Debug {
					if strings.HasPrefix(line, "PERSONRTDEFAULT|") {
						rows = append(rows, strings.TrimPrefix(line, "PERSONRTDEFAULT|"))
					}
				}
				compare(t, "anonymous", rows)
			})
			for _, route := range []string{"project", "warm", "disk_restore"} {
				t.Run(route, func(t *testing.T) {
					if route == "disk_restore" {
						InvalidateRuntimeCaches()
					}
					result := Run(index, Options{SelectedClasses: []string{"PersonDefaultsProbeTest"}, Parallelism: 1, PerfCounters: route == "disk_restore"})
					if route == "disk_restore" && SnapshotPerfCounters().Phases.DiskCacheHits != 1 {
						t.Fatalf("disk restore was not used: %+v", SnapshotPerfCounters().Phases)
					}
					var rows []string
					for _, suite := range result.Suites {
						for _, c := range suite.Cases {
							if c.Problem == nil {
								t.Fatalf("missing observation from %s", c.MethodName)
							}
							_, payload, ok := strings.Cut(c.Problem.Message, "Actual: ")
							if !ok {
								t.Fatalf("unexpected failure: %+v", c.Problem)
							}
							rows = append(rows, strings.Split(payload, ";")...)
						}
					}
					compare(t, "istest", rows)
					if _, ok := tryLoadDiskRuntime(index); !ok {
						t.Fatal("runtime disk cache was not populated")
					}
				})
			}
		})
	}
}
