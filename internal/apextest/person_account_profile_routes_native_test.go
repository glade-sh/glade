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

func TestPersonAccountProfileRoutesNativeConformance(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, fixture := range []profileRoutesNativeFixture{
		{"person_accounts_enabled", "../../testdata/conformance/person_account_profile_routes", "ProfileRoutesProbeTest", "routes-anonymous.apex", "PROFILEROUTES|", 112, 448, 16},
		{"person_accounts_disabled", "../../testdata/conformance/person_account_profile_disabled", "BusinessDefaultsProbeTest", "anonymous.apex", "BUSINESSDEFAULTS|", 4, 16, 4},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			runProfileRoutesNativeConformance(t, fixture)
		})
	}
}

type profileRoutesNativeFixture struct {
	name, path, testClass, anonymousFile, debugPrefix string
	anonymousRows, namedRows, methods                 int
}

func runProfileRoutesNativeConformance(t *testing.T, capture profileRoutesNativeFixture) {
	t.Helper()
	fixture := capture.path
	native, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(native)))
	reader.Comma, reader.FieldsPerRecord = '\t', 5
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || strings.Join(rows[0], "\t") != "api\troute\tcase\tvalue\tdetail" {
		t.Fatal("unexpected native oracle header")
	}
	want := map[string]map[string]string{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1]
		if want[key] == nil {
			want[key] = map[string]string{}
		}
		if _, exists := want[key][row[2]]; exists {
			t.Fatalf("duplicate native row %v", row)
		}
		want[key][row[2]] = row[3] + "|" + row[4]
	}
	if len(want) != 4 {
		t.Fatalf("native API/route groups=%d, want 4", len(want))
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
			compare := func(t *testing.T, route string, observed []string) {
				t.Helper()
				expected := want[api+"/"+route]
				count := capture.anonymousRows
				if route == "istest" {
					count = capture.namedRows
				}
				if len(expected) != count || len(observed) != count {
					t.Fatalf("%s rows: native=%d local=%d, want %d", route, len(expected), len(observed), count)
				}
				seen := map[string]bool{}
				for _, row := range observed {
					// Error messages are retained in diagnostics.tsv. Status and field
					// identity, or selected record type and account kind, are the oracle.
					parts := strings.SplitN(row, "|", 4)
					if len(parts) < 3 {
						t.Errorf("malformed observation %q", row)
						continue
					}
					name, value := parts[0], parts[1]+"|"+parts[2]
					wanted, exists := expected[name]
					if !exists || seen[name] || wanted != value {
						t.Errorf("%s: native %q; local %q (known=%v, duplicate=%v)", name, wanted, value, exists, seen[name])
					}
					seen[name] = true
				}
				for name := range expected {
					if !seen[name] {
						t.Errorf("missing observation %s", name)
					}
				}
			}
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(readFixtureFile(t, filepath.Join(fixture, capture.anonymousFile)), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatalf("%v; debug: %v", err, result.Debug)
				}
				var observed []string
				for _, line := range result.Debug {
					if strings.HasPrefix(line, capture.debugPrefix) {
						observed = append(observed, strings.TrimPrefix(line, capture.debugPrefix))
					}
				}
				compare(t, "anonymous", observed)
			})
			for _, route := range []string{"project", "warm", "disk_restore"} {
				t.Run(route, func(t *testing.T) {
					if route == "disk_restore" {
						InvalidateRuntimeCaches()
					}
					result := Run(index, Options{SelectedClasses: []string{capture.testClass}, Parallelism: 1, PerfCounters: route == "disk_restore"})
					if route == "disk_restore" && SnapshotPerfCounters().Phases.DiskCacheHits != 1 {
						t.Fatalf("disk restore was not used: %+v", SnapshotPerfCounters().Phases)
					}
					var observed []string
					methods := 0
					for _, suite := range result.Suites {
						for _, c := range suite.Cases {
							methods++
							if c.Problem == nil {
								t.Fatalf("missing observation from %s", c.MethodName)
							}
							_, payload, ok := strings.Cut(c.Problem.Message, "Actual: ")
							if !ok {
								t.Fatalf("unexpected failure: %+v", c.Problem)
							}
							observed = append(observed, strings.Split(payload, ";")...)
						}
					}
					if methods != capture.methods {
						t.Fatalf("observation methods=%d, want %d", methods, capture.methods)
					}
					compare(t, "istest", observed)
					if _, ok := tryLoadDiskRuntime(index); !ok {
						t.Fatal("runtime disk cache was not populated")
					}
				})
			}
		})
	}
}
