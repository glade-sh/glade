package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReflectionEdgesNativeConformance(t *testing.T) {
	runNamedObservationRows(t, "reflection_edges", "ReflectionEdgeObservation", 6)
}

func TestNullableFlowCollectionNativeConformance(t *testing.T) {
	runNamedObservationRows(t, "nullable_flow_collection", "NullableFlowCollectionObservation", 4)
}

func runNamedObservationRows(t *testing.T, family, class string, count int) {
	t.Helper()
	fixture := filepath.Join("testdata", "conformance", family)
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
	if len(rows) != count+1 {
		t.Fatalf("expected %d native rows, got %d", count, len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") || row[1] != "istest" || row[2] == "" {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[0] + "/" + row[2]
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				relative, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				source := strings.ReplaceAll(string(raw), "67.0", row[0])
				if strings.HasSuffix(path, class+".cls") {
					transport := "System.assert(false, 'OBSERVED|" + row[2] + "|' + observed + '|END');"
					if strings.Count(source, transport) != 1 {
						t.Fatal("missing observation transport for " + row[2])
					}
					source = strings.Replace(source, transport, "System.assertEquals("+conformanceApexString(row[3])+", observed);", 1)
				}
				writeFile(t, filepath.Join(root, relative), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{class}, SelectedMethod: row[2], NoDiskCache: true})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("native mismatch: %#v: %s", summary, firstRunProblem(run))
			}
		})
	}
}
