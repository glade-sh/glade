package apextest

import (
	"github.com/glade-sh/glade/internal/vm"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrimitiveSetReceiverNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "primitive_set_receiver", "CollectionWalkProbeTest", func(t *testing.T, got, want any) {
		// Compare Id identity; rendering a short Id as eighteen characters is
		// independent of membership and iteration order.
		for _, observation := range []any{got, want} {
			ids := observation.(map[string]any)["idAfter"].([]any)
			for i, raw := range ids {
				id := raw.(string)
				if len(id) == 18 {
					ids[i] = id[:15]
				}
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("native=%v; Glade=%v", want, got)
		}
	}, "62.0", "67.0")
}

func TestPrimitiveSetIterationNativeRoutes(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/primitive_set_receiver/iteration-org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(rows) != 5 {
		t.Fatal("incomplete iteration matrix")
	}
	for _, row := range rows[1:] {
		cells := strings.Split(row, "\t")
		if len(cells) != 3 || !strings.Contains(cells[2], "System.FinalException") {
			t.Fatal("invalid native row")
		}
		api, route := cells[0], cells[1]
		t.Run(api+"/"+route, func(t *testing.T) {
			root := t.TempDir()
			src := "testdata/conformance/primitive_set_receiver/project"
			err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if route == "anonymous" {
				source, err := os.ReadFile("testdata/conformance/primitive_set_receiver/iteration.apex")
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				_, err = runner.execute(program)
				if err == nil {
					t.Fatal("native rejects modifying the iterated Set")
				}
			} else {
				report := Run(index, Options{SelectedClasses: []string{"CollectionWalkProbeTest"}, SelectedMethod: "iteration", NoDiskCache: true, Parallelism: 1})
				if len(report.Suites) != 1 || len(report.Suites[0].Cases) != 1 || report.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("native rejects modifying the iterated Set: %+v", report.Summary())
				}
			}
		})
	}
}
