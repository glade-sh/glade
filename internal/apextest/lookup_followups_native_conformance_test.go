package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestLookupFollowupsNativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/lookup_filter_followups"
	rows, err := readTypeNamePinRowsForApextest(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	class := readFixtureFile(t, filepath.Join(fixture, "LookupFollowupsProbeTest.cls"))
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
			if len(expected) != 55 {
				t.Fatalf("native rows: got %d, want 55", len(expected))
			}
			for _, row := range expected {
				if !strings.Contains(class, row) {
					t.Fatalf("unasserted native row: %s", row)
				}
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), readFixtureFile(t, filepath.Join(fixture, "config/project-scratch-def.json")))
			if err := filepath.WalkDir(filepath.Join(fixture, "force-app"), func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), readFixtureFile(t, path))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"LookupFollowupsProbeTest.cls", "LookupFollowupsProbeTest.cls-meta.xml"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name), strings.ReplaceAll(readFixtureFile(t, filepath.Join(fixture, name)), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>"))
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Followup.profile-meta.xml"), readFixtureFile(t, filepath.Join(fixture, "profile-source/main/default/profiles/Followup.profile-meta.xml")))
			// The anonymous compiler accepts statements, not local method
			// declarations. Lift only the captured result-formatting helper;
			// all DML statements and native expectations remain unchanged.
			const helper = "String outcome(Database.SaveResult r) { return r.isSuccess() ? 'OK' : String.valueOf(r.getErrors()[0].getStatusCode()); }"
			anonymous := readFixtureFile(t, filepath.Join(fixture, "anonymous.apex"))
			if !strings.Contains(anonymous, helper) {
				t.Fatal("captured outcome helper changed")
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/LookupFollowupsOutcome.cls"), "public class LookupFollowupsOutcome { public static "+helper+" }")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/LookupFollowupsOutcome.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			anonymous = strings.Replace(anonymous, helper, "", 1)
			anonymous = strings.ReplaceAll(anonymous, "outcome(", "LookupFollowupsOutcome.outcome(")
			index := loadTestIndex(t, root)
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				source := anonymous + "\nSystem.assertEquals('" + strings.Join(expected, ";") + "', String.join(rows, ';'));"
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
					run := Run(index, Options{SelectedClasses: []string{"LookupFollowupsProbeTest"}, NoDiskCache: route == "project", Parallelism: 1, PerfCounters: true})
					if got := run.Summary(); got.Total != 1 || got.Passed != 1 || len(run.ExcludedClasses) != 0 {
						for _, suite := range run.Suites {
							for _, c := range suite.Cases {
								if c.Problem != nil {
									t.Logf("%s: %s", c.MethodName, c.Problem.Message)
								}
							}
						}
						t.Fatalf("lookup followups: %+v excluded=%v", got, run.ExcludedClasses)
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
