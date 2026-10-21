package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type listViewNativeCapture struct {
	API, NativeStatus, ReceiptSHA256 string
	TestsCompleted                   int
	Files                            map[string]string
}

func loadListViewNativeCaptures(t *testing.T) []listViewNativeCapture {
	t.Helper()
	raw, err := os.ReadFile("testdata/conformance/listview_metadata_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Captures []listViewNativeCapture }
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Captures) != 2 {
		t.Fatal("missing native list-view observations")
	}
	return data.Captures
}

func TestListViewMetadataNativeConformance(t *testing.T) {
	for _, capture := range loadListViewNativeCaptures(t) {
		t.Run(capture.API, func(t *testing.T) {
			if capture.NativeStatus != "Succeeded" || capture.TestsCompleted != 1 {
				t.Fatal("incomplete native observation")
			}
			root := t.TempDir()
			for name, source := range capture.Files {
				writeFile(t, filepath.Join(root, filepath.FromSlash(name)), source)
			}
			index := loadTestIndex(t, root)
			report := Run(index, Options{SelectedClasses: []string{"OwnedViewQueryTest"}, RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
			if summary := report.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
				for _, suite := range report.Suites {
					for _, row := range suite.Cases {
						if row.Problem != nil {
							t.Logf("%s: %s", row.Problem.Type, row.Problem.Message)
						}
					}
				}
				t.Fatalf("native list-view replay: %+v", summary)
			}
		})
	}
}

func TestListViewMetadataRefreshesRuntimeCache(t *testing.T) {
	capture := loadListViewNativeCaptures(t)[0]
	root := t.TempDir()
	for name, source := range capture.Files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), source)
	}
	index := loadTestIndex(t, root)
	firstKey, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	var changed string
	for name, source := range capture.Files {
		if strings.HasSuffix(name, "OwnedViewAll.listView-meta.xml") {
			changed = filepath.Join(root, filepath.FromSlash(name))
			writeFile(t, changed, strings.Replace(source, "Owned All", "Owned Renamed", 1))
		}
	}
	if changed == "" {
		t.Fatal("list-view fixture missing")
	}
	nextKey, next, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	if nextKey == firstKey {
		t.Fatal("metadata-only edit reused the earlier runtime")
	}
	org := next.restored.CloneOrg()
	found := false
	for _, row := range org.Objects["ListView"].Records {
		name, _ := row.GetField("Name")
		if name.String == "Owned Renamed" {
			found = true
		}
	}
	if !found {
		t.Fatal("runtime did not load the captured renamed list view")
	}
}

func TestMalformedListViewMetadataStopsRuntime(t *testing.T) {
	capture := loadListViewNativeCaptures(t)[0]
	root := t.TempDir()
	for name, source := range capture.Files {
		if strings.HasSuffix(name, "OwnedViewAll.listView-meta.xml") {
			source = "<ListView>"
		}
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), source)
	}
	index := loadTestIndex(t, root)
	_, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err == nil {
		t.Fatal("malformed list-view metadata was silently discarded")
	}
}
