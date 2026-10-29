package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned native assertions and rejection twins: testdata/conformance/address_location/org.tsv.
func TestAddressLocationNativeConformance(t *testing.T) {
	for _, fixture := range []string{"testdata/conformance/address_location", "testdata/conformance/address_location_review"} {
		t.Run(filepath.Base(fixture), func(t *testing.T) { testAddressLocationNativeConformance(t, fixture) })
	}
}

func testAddressLocationNativeConformance(t *testing.T, fixture string) {
	restoreDisk := EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	t.Cleanup(InvalidateRuntimeCaches)
	raw, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID, Body, Status string
		NamedOnly        bool
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
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
	native := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) != 5 {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[1] + "/" + row[2] + "/" + row[0]
		if _, ok := native[key]; ok {
			t.Fatalf("duplicate native row %s", key)
		}
		native[key] = row[3]
	}
	if len(native) != len(cases)*4 {
		t.Fatalf("native cells=%d, want %d", len(native), len(cases)*4)
	}
	passing := 0
	for _, row := range cases {
		if row.Status == "PASS" {
			passing++
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "project", "force-app"), func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := strings.ReplaceAll(string(raw), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>")
				writeFile(t, filepath.Join(root, rel), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range cases {
				for _, route := range []string{"anonymous", "istest"} {
					expected := row.Status
					if row.NamedOnly && route == "anonymous" {
						expected = "RUNTIME_ERROR"
					}
					if native[api+"/"+route+"/"+row.ID] != expected {
						t.Fatalf("%s/%s/%s lacks matching native evidence", api, route, row.ID)
					}
				}
				t.Run(row.ID, func(t *testing.T) {
					result := sema.AnalyzeAnonymous(index, row.Body, api)
					if row.Status == "REJECT" {
						if !result.HasErrors() {
							t.Fatal("native rejected reverse assignment")
						}
						rejected := filepath.Join(root, "force-app/main/default/classes/RejectedAddress.cls")
						writeFile(t, rejected, "@IsTest private class RejectedAddress { @IsTest static void rejected() {"+row.Body+"} }")
						writeFile(t, rejected+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						if !sema.Analyze(loadTestIndex(t, root)).HasErrors() {
							t.Fatal("native rejected named reverse assignment")
						}
						if err := os.Remove(rejected); err != nil {
							t.Fatal(err)
						}
						if err := os.Remove(rejected + "-meta.xml"); err != nil {
							t.Fatal(err)
						}
						return
					}
					if result.HasErrors() {
						t.Fatal(result.Diagnostics)
					}
					// ItemArrange creation is native-test-only. Its anonymous rejection
					// is retained in the oracle; the project runner covers its value.
					if row.NamedOnly {
						return
					}
					program, err := vm.CompileAnonymousWithOptions(row.Body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					for pass := 0; pass < 2; pass++ {
						if _, err := runner.execute(program); err != nil {
							t.Fatalf("execution %d: %v", pass, err)
						}
					}
				})
			}
			for pass := 0; pass < 3; pass++ {
				if pass == 2 {
					InvalidateRuntimeCaches()
				}
				ResetPerfCounters()
				run := Run(index, Options{Parallelism: 1, PerfCounters: true})
				if got := run.Summary(); got.Total != passing || got.Passed != passing {
					for _, suite := range run.Suites {
						for _, result := range suite.Cases {
							if result.Problem != nil {
								t.Logf("%s.%s: %+v", result.ClassName, result.MethodName, result.Problem)
							}
						}
					}
					t.Fatalf("project runner pass %d: %#v: %s", pass, got, firstRunProblem(run))
				}
				if pass == 1 && SnapshotPerfCounters().Phases.MemoryCacheHits != 1 {
					t.Fatal("warm runner missed memory cache")
				}
				if pass == 2 && SnapshotPerfCounters().Phases.DiskCacheHits != 1 {
					t.Fatal("restored runner missed disk cache")
				}
			}
		})
	}
}
