package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestPersonRecordTypeIdentityNativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/person_record_type_identity"
	rows, err := readTypeNamePinRowsForApextest(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	restore := EnableDiskCacheForTesting()
	t.Cleanup(restore)
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var expected []string
			for _, row := range rows[1:] {
				if len(row) != 5 {
					t.Fatalf("invalid native row: %v", row)
				}
				if row[0] == api && row[1] == "anonymous" {
					expected = append(expected, strings.Join(row[2:], "|"))
				}
			}
			if len(expected) != 12 {
				t.Fatalf("native rows = %d; want 12", len(expected))
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), readFixtureFile(t, filepath.Join(fixture, "config/project-scratch-def.json")))
			for _, source := range []string{"force-app", "profile-source", "permission-source"} {
				err := filepath.WalkDir(filepath.Join(fixture, source), func(path string, entry os.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					rel, err := filepath.Rel(filepath.Join(fixture, source), path)
					if err != nil {
						return err
					}
					writeFile(t, filepath.Join(root, "force-app", rel), readFixtureFile(t, path))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			class := readFixtureFile(t, filepath.Join(fixture, "IdentityProbeTest.cls"))
			for _, row := range expected {
				if !strings.Contains(class, row) {
					t.Fatalf("unasserted native row: %s", row)
				}
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/IdentityProbeTest.cls"), class)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/IdentityProbeTest.cls-meta.xml"), strings.ReplaceAll(readFixtureFile(t, filepath.Join(fixture, "IdentityProbeTest.cls-meta.xml")), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>"))
			index := loadTestIndex(t, root)
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				source := readFixtureFile(t, filepath.Join(fixture, "anonymous.apex")) + "\nSystem.assertEquals('" + strings.Join(expected, ";") + "',String.join(rows,';'));"
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := runner.newMachine().Execute(program); err != nil {
					t.Fatal(err)
				}
			})
			for _, route := range []string{"project", "disk_cold", "memory_warm", "disk_restored"} {
				t.Run(route, func(t *testing.T) {
					if route != "memory_warm" {
						InvalidateRuntimeCaches()
					}
					ResetPerfCounters()
					run := Run(index, Options{SelectedClasses: []string{"IdentityProbeTest"}, NoDiskCache: route == "project", Parallelism: 1, PerfCounters: true})
					if got := run.Summary(); got.Total != 1 || got.Passed != 1 || len(run.ExcludedClasses) != 0 {
						for _, suite := range run.Suites {
							for _, c := range suite.Cases {
								if c.Problem != nil {
									t.Logf("%s: %s", c.MethodName, c.Problem.Message)
								}
							}
						}
						t.Fatalf("record type identity: %+v excluded=%v", got, run.ExcludedClasses)
					}
					phases := SnapshotPerfCounters().Phases
					if route == "disk_restored" && phases.DiskCacheHits != 1 {
						t.Fatalf("disk cache not used: %+v", phases)
					}
					if route == "memory_warm" && phases.MemoryCacheHits != 1 {
						t.Fatalf("memory cache not used: %+v", phases)
					}
				})
			}
		})
	}
}
