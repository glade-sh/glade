package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// withSourceScanMemo runs fn with the per-source memos on and empty.
func withSourceScanMemo(t *testing.T, fn func()) {
	t.Helper()
	was := sourceScanMemoEnabled.Load()
	standardFieldMatchMemo = sourceScanMemo[*standardFieldSourceMatches]{}
	recordTypeScanMemo = sourceScanMemo[[]projectRecordTypeReference]{}
	fieldSetConstantMemo = sourceScanMemo[[][]string]{}
	sourceScanMemoEnabled.Store(true)
	defer func() {
		sourceScanMemoEnabled.Store(was)
		standardFieldMatchMemo = sourceScanMemo[*standardFieldSourceMatches]{}
		recordTypeScanMemo = sourceScanMemo[[]projectRecordTypeReference]{}
	}()
	fn()
}

func sourceScanMemoFixtures(t *testing.T) []string {
	t.Helper()
	var sources []string
	_, source := fieldScanFixture()
	sources = append(sources, source)
	root := filepath.Join("..", "..", "testdata", "local-tests")
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".cls") || len(sources) >= 120 {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			sources = append(sources, string(data))
		}
		return nil
	})
	if len(sources) < 20 {
		t.Fatalf("only %d fixture sources", len(sources))
	}
	return sources
}

// TestSourceScanMemoMatchesDirectScan requires memoized scans to equal direct
// scans, against orgs that differ, after the source changes, and on repeat.
func TestSourceScanMemoMatchesDirectScan(t *testing.T) {
	sources := sourceScanMemoFixtures(t)
	orgs := []*storage.OrgState{}
	small, _ := fieldScanFixture()
	orgs = append(orgs, small)
	wide := storage.NewOrgState()
	for _, name := range []string{"Account", "Contact", "Opportunity", "Case", "Lead", "Asset"} {
		wide.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{}}}
	}
	orgs = append(orgs, &wide)
	type want struct {
		fields standardFieldScanResult
		types  []projectRecordTypeReference
	}
	direct := make([][]want, len(orgs))
	for i, org := range orgs {
		for _, source := range sources {
			direct[i] = append(direct[i], want{scanProjectReferencedStandardFieldsFromSource(org, source, nil), scanRecordTypesFromApexSource(source)})
		}
	}
	withSourceScanMemo(t, func() {
		for round := 0; round < 2; round++ {
			for i, org := range orgs {
				for j, source := range sources {
					got := scanProjectReferencedStandardFieldsFromSource(org, source, nil)
					if !reflect.DeepEqual(got, direct[i][j].fields) {
						t.Fatalf("round %d org %d source %d: memoized field scan differs", round, i, j)
					}
					if sets := fieldSetConstantMemo.get(source, fieldSetConstantMatches); !reflect.DeepEqual(sets, apexFieldSetConstantPattern.FindAllStringSubmatch(source, -1)) {
						t.Fatalf("round %d source %d: memoized field set scan differs", round, j)
					}
					if types := scanRecordTypesFromApexSourceMemo(source); !reflect.DeepEqual(types, direct[i][j].types) {
						t.Fatalf("round %d source %d: memoized record type scan differs", round, j)
					}
				}
			}
		}
		if got := standardFieldMatchMemo.len(); got > len(sources) {
			t.Fatalf("memo holds %d entries for %d sources", got, len(sources))
		}
		// An edited source is a different key and scans afresh.
		org, source := fieldScanFixture()
		edited := strings.Replace(source, "Extra0__c", "Edited0__c", 1)
		got := scanProjectReferencedStandardFieldsFromSource(org, edited, nil)
		if _, ok := got.Inferred["Account"]["Edited0__c"]; !ok {
			t.Fatal("edited source reused the old scan")
		}
		if _, ok := got.Inferred["Account"]["Extra0__c"]; ok {
			t.Fatal("edited source kept the old field")
		}
	})
}

// TestParallelSourceScansWithMemo covers the project-wide scans: an edit, a
// delete and a rename give the same merged results as a scan without memos.
func TestParallelSourceScansWithMemo(t *testing.T) {
	dir := t.TempDir()
	sources := sourceScanMemoFixtures(t)
	var files []string
	for i, source := range sources[:30] {
		path := filepath.Join(dir, "F"+string(rune('a'+i%26))+strings.Repeat("x", i/26)+".cls")
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, path)
	}
	org, _ := fieldScanFixture()
	scan := func(files []string) ([]projectRecordTypeReference, standardFieldScanResult) {
		cache := newSourceCache()
		types := parallelScanProjectReferencedRecordTypes(files, cache)
		sort.Slice(types, func(i, j int) bool { return fmt.Sprintf("%+v", types[i]) < fmt.Sprintf("%+v", types[j]) })
		var results []standardFieldScanResult
		for _, file := range files {
			source, err := cache.read(file)
			if err == nil {
				results = append(results, scanProjectReferencedStandardFieldsFromSource(org, source, nil))
			}
		}
		return types, mergeStandardFieldScanResults(results)
	}
	steps := []func(){
		func() {},
		func() { // edit
			if err := os.WriteFile(files[0], []byte(strings.Replace(sources[0], "Extra1__c", "Changed1__c", 1)), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		func() { // delete
			if err := os.Remove(files[1]); err != nil {
				t.Fatal(err)
			}
			files = append(files[:1], files[2:]...)
		},
		func() { // rename
			renamed := filepath.Join(dir, "Renamed.cls")
			if err := os.Rename(files[2], renamed); err != nil {
				t.Fatal(err)
			}
			files[2] = renamed
		},
	}
	withSourceScanMemo(t, func() {
		for i, step := range steps {
			step()
			sourceScanMemoEnabled.Store(false)
			wantTypes, wantFields := scan(files)
			sourceScanMemoEnabled.Store(true)
			for pass := 0; pass < 2; pass++ {
				gotTypes, gotFields := scan(files)
				if !reflect.DeepEqual(gotTypes, wantTypes) || !reflect.DeepEqual(gotFields, wantFields) {
					t.Fatalf("step %d pass %d: memoized project scan differs", i, pass)
				}
			}
		}
	})
}

func TestSourceScanMemoBoundsBytes(t *testing.T) {
	withSourceScanMemo(t, func() {
		var memo sourceScanMemo[int]
		big := strings.Repeat("x", sourceScanMemoByteLimit/2+1)
		memo.get(big+"a", func(string) int { return 1 })
		memo.get(big+"b", func(string) int { return 2 })
		if memo.len() != 1 || memo.bytes > sourceScanMemoByteLimit {
			t.Fatalf("memo holds %d entries and %d bytes past the byte bound", memo.len(), memo.bytes)
		}
	})
}

func TestBackgroundDiskPublicationDrains(t *testing.T) {
	var order []int
	var mu sync.Mutex
	for i := 0; i < 3; i++ {
		i := i
		publishInBackground("drain-root-"+strconv.Itoa(i), func() {
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
		})
	}
	WaitForRuntimeCacheWork()
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("WaitForRuntimeCacheWork returned before %d publications finished", 3-len(order))
	}
}

// TestBackgroundDiskPublicationKeepsNewest pins the order: a write handed off
// earlier never replaces one handed off later for the same root.
func TestBackgroundDiskPublicationKeepsNewest(t *testing.T) {
	var mu sync.Mutex
	var written []int
	backgroundPublications.Lock()
	for i := 1; i <= 4; i++ {
		i := i
		publishInBackground("order-root", func() {
			mu.Lock()
			written = append(written, i)
			mu.Unlock()
		})
	}
	backgroundPublications.Unlock()
	WaitForRuntimeCacheWork()
	mu.Lock()
	defer mu.Unlock()
	for i := 1; i < len(written); i++ {
		if written[i] < written[i-1] {
			t.Fatalf("writes ran as %v: an older entry replaced a newer one", written)
		}
	}
	if len(written) == 0 || written[len(written)-1] != 4 {
		t.Fatalf("writes ran as %v, want the newest last", written)
	}
}
