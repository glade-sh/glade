package apextest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
)

type diskRuntimeScopeNativeRow struct {
	apiVersion string
	route      string
	className  string
	methodName string
	status     string
	lookup     string
}

func TestDiskRuntimeLookupScopeOrgConformance(t *testing.T) {
	const fixture = "testdata/conformance/disk_runtime_scope"
	capture, err := os.ReadFile(filepath.Join(fixture, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), "API 62.0 DiskScopeProbe62Test.lookupIsNullAfterParentDelete: PASS") ||
		!strings.Contains(string(capture), "API 67.0 DiskScopeProbe67Test.lookupIsNullAfterParentDelete: PASS") {
		t.Fatal("native capture does not include passing API 62.0 and 67.0 tests")
	}
	cases := readDiskRuntimeScopeTSV(t, filepath.Join(fixture, "cases.tsv"))
	if len(cases) != 4 || strings.Join(cases[0], "\t") != "api_version\troute\tclass_name\tmethod_name\tobservation" {
		t.Fatal("invalid disk runtime scope case table")
	}
	nativeRows := readDiskRuntimeScopeNativeRows(t, filepath.Join(fixture, "org.tsv"))
	if len(nativeRows) != 3 {
		t.Fatalf("native rows = %d, want anonymous plus API 62.0 and 67.0 tests", len(nativeRows))
	}

	permissions := readDiskRuntimeScopePermissionRows(t, filepath.Join(fixture, "permission_replay", "org.tsv"))
	for _, configuration := range []string{"unassigned", "granted"} {
		t.Run(configuration, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "project")
			copyDiskRuntimeScopeProject(t, filepath.Join(fixture, "project"), root)
			if configuration == "unassigned" {
				if err := os.Remove(filepath.Join(root, "glade.yml")); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
			for _, row := range nativeRows[1:] {
				expected, ok := permissions[row.apiVersion+"/"+configuration]
				if !ok || expected.Class != row.className || expected.Method != row.methodName {
					t.Fatal("missing native permission setup", row.apiVersion, configuration)
				}
				path := filepath.Join(root, "force-app/main/default/classes", row.className+".cls")
				source, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				// Assert the recorded actor and grants inside the unchanged native test
				// method, after the disk template is restored and runner setup applied.
				prelude := fmt.Sprintf(`
    System.assertEquals('%s', [SELECT Name FROM Profile WHERE Id=:UserInfo.getProfileId()].Name);
    System.assertEquals(%d, [SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId()]);
    System.assertEquals(%d, [SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.Name='DiskRuntimeLookupProbe']);
    System.assertEquals(%d, [SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSetGroupId!=null]);
    System.assertEquals(%t, LookupScopeChild__c.Parent__c.getDescribe().isAccessible());
    System.assertEquals(%t, LookupScopeChild__c.Parent__c.getDescribe().isCreateable());
    System.assertEquals(%t, LookupScopeChild__c.Parent__c.getDescribe().isUpdateable());
`, expected.Profile, expected.AssignmentCount, expected.ProbeAssignments, expected.GroupCount, expected.Readable, expected.Createable, expected.Updateable)
				const marker = "lookupIsNullAfterParentDelete() {"
				if strings.Count(string(source), marker) != 1 {
					t.Fatal("missing native test method")
				}
				writeFile(t, path, strings.Replace(string(source), marker, marker+prelude, 1))
			}
			index := loadTestIndex(t, root)
			restoreDiskCache := EnableDiskCacheForTesting()
			t.Cleanup(restoreDiskCache)
			t.Cleanup(InvalidateRuntimeCaches)
			InvalidateRuntimeCaches()

			key, built, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
			if err != nil {
				t.Fatal(err)
			}
			org := built.restored.CloneOrg()
			engine := dml.NewEngine(&org)
			parents := engine.Insert([]storage.Record{{
				Object: "LookupScopeParent__c",
				Fields: map[string]storage.Value{"Name": storage.StringValue("disk-runtime-scope-parent-20261019")},
			}})
			if len(parents) != 1 || !parents[0].Success {
				t.Fatalf("parent insert = %#v", parents)
			}
			children := engine.Insert([]storage.Record{{
				Object: "LookupScopeChild__c",
				Fields: map[string]storage.Value{
					"Name":      storage.StringValue("disk-runtime-scope-child-20261019"),
					"Parent__c": storage.IDValue(parents[0].ID),
				},
			}})
			if len(children) != 1 || !children[0].Success {
				t.Fatalf("child insert = %#v", children)
			}
			deleted := engine.Delete([]storage.Record{{ID: parents[0].ID, Object: "LookupScopeParent__c"}})
			if len(deleted) != 1 || !deleted[0].Success {
				t.Fatalf("parent delete = %#v", deleted)
			}
			child := org.Objects["LookupScopeChild__c"].Records[children[0].ID]
			if value, exists := child.GetField("Parent__c"); exists && value.Kind != storage.ValueNull {
				t.Fatalf("native SetNull observation = %#v, want NULL", value)
			}
			if nativeRows[0].apiVersion != "67.0" || nativeRows[0].route != "anonymous" || nativeRows[0].status != "pass" || nativeRows[0].lookup != "NULL" {
				t.Fatalf("native anonymous row = %#v", nativeRows[0])
			}

			input, err := validatedDiskRuntimeInput(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			cacheEntry, err := startupcache.NewEntryWithValidatedInput(input, org, startupcache.CompiledRuntime{
				Methods:   built.Methods,
				Classes:   built.Classes,
				Triggers:  built.Triggers,
				PageNames: built.PageNames,
			})
			if err != nil {
				t.Fatal(err)
			}
			cacheEntry.RuntimeABI = testRuntimeCacheABI
			cacheEntry.RuntimeKey = string(key)
			if err := startupcache.Write(&cacheEntry, startupcache.SubdirTest); err != nil {
				t.Fatalf("write SetNull runtime snapshot: %v", err)
			}

			for _, row := range nativeRows[1:] {
				if row.route != "apex_test" || row.status != "pass" || row.lookup != "NULL" {
					t.Fatalf("native Apex test row = %#v", row)
				}
				t.Run(row.apiVersion+"/"+row.className, func(t *testing.T) {
					cases := Discover(index, Options{SelectedClasses: []string{row.className}})
					if len(cases) != 1 || cases[0].MethodName != row.methodName {
						t.Fatalf("discovered cases = %#v", cases)
					}
					InvalidateRuntimeCaches()
					ResetPerfCounters()
					run := RunCasesContext(context.Background(), index, Options{
						SelectedClasses: []string{row.className},
						PerfCounters:    true,
					}, cases)
					expected := permissions[row.apiVersion+"/"+configuration]
					summary := run.Summary()
					if summary.Total != 1 {
						t.Fatalf("summary = %#v", summary)
					}
					if expected.Outcome == "Pass" {
						if summary.Passed != 1 {
							t.Fatalf("summary = %#v problem=%q", summary, firstRunProblem(run))
						}
					} else if summary.Failed != 1 || firstRunProblem(run) != strings.TrimPrefix(expected.Message, "System.QueryException: ") {
						t.Fatalf("summary = %#v problem=%q native=%q", summary, firstRunProblem(run), expected.Message)
					}
					if phases := SnapshotPerfCounters().Phases; phases.DiskCacheHits != 1 || phases.CacheMisses != 0 {
						t.Fatalf("restored runtime phases = %#v, want one disk hit", phases)
					}
				})
			}
		})
	}
}

func readDiskRuntimeScopeTSV(t *testing.T, path string) [][]string {
	t.Helper()
	file, err := os.Open(path)
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
	return rows
}

func readDiskRuntimeScopeNativeRows(t *testing.T, path string) []diskRuntimeScopeNativeRow {
	t.Helper()
	rows := readDiskRuntimeScopeTSV(t, path)
	wantHeader := []string{"api_version", "route", "class_name", "method_name", "status", "observed_parent_lookup"}
	if len(rows) == 0 || strings.Join(rows[0], "\t") != strings.Join(wantHeader, "\t") {
		t.Fatal("invalid native row header")
	}
	result := make([]diskRuntimeScopeNativeRow, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) != len(wantHeader) {
			t.Fatalf("invalid native row: %#v", row)
		}
		result = append(result, diskRuntimeScopeNativeRow{
			apiVersion: row[0],
			route:      row[1],
			className:  row[2],
			methodName: row[3],
			status:     row[4],
			lookup:     row[5],
		})
	}
	return result
}

func copyDiskRuntimeScopeProject(t *testing.T, source, destination string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, contents, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

type diskRuntimeScopePermissionObservation struct {
	Profile          string `json:"profile"`
	AssignmentCount  int    `json:"assignmentCount"`
	ProbeAssignments int    `json:"probeAssignments"`
	GroupCount       int    `json:"groupCount"`
	Readable         bool   `json:"readable"`
	Createable       bool   `json:"createable"`
	Updateable       bool   `json:"updateable"`
	Class            string `json:"class"`
	Method           string `json:"method"`
	Outcome          string `json:"outcome"`
	Message          string `json:"message"`
}

func readDiskRuntimeScopePermissionRows(t *testing.T, path string) map[string]diskRuntimeScopePermissionObservation {
	t.Helper()
	rows := readDiskRuntimeScopeTSV(t, path)
	if len(rows) != 5 || strings.Join(rows[0], "|") != "source_api|configuration|observation" {
		t.Fatal("incomplete native disk permission matrix")
	}
	result := make(map[string]diskRuntimeScopePermissionObservation)
	for _, row := range rows[1:] {
		if len(row) != 3 {
			t.Fatal("invalid native disk permission row")
		}
		key := row[0] + "/" + row[1]
		if _, exists := result[key]; exists {
			t.Fatal("duplicate native disk permission row", key)
		}
		var observation diskRuntimeScopePermissionObservation
		if err := json.Unmarshal([]byte(row[2]), &observation); err != nil {
			t.Fatal(err)
		}
		if observation.Outcome != "Pass" && observation.Outcome != "Fail" {
			t.Fatal("unknown native disk permission outcome", observation.Outcome)
		}
		result[key] = observation
	}
	return result
}
