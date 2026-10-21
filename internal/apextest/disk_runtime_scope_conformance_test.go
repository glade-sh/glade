package apextest

import (
	"context"
	"encoding/csv"
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

	root := filepath.Join(t.TempDir(), "project")
	copyDiskRuntimeScopeProject(t, filepath.Join(fixture, "project"), root)
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
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("summary = %#v problem=%q", summary, firstRunProblem(run))
			}
			if phases := SnapshotPerfCounters().Phases; phases.DiskCacheHits != 1 || phases.CacheMisses != 0 {
				t.Fatalf("restored runtime phases = %#v, want one disk hit", phases)
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
