package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExecConfiguredParentDateNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	dir := "../apextest/testdata/conformance/configured_parent_date"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatal("incomplete native date matrix")
	}
	source, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if row[1] != "anonymous" {
			continue
		}
		if seen[row[0]] || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("unexpected native date API %s", row[0])
		}
		seen[row[0]] = true
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err == nil {
					writeTestFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", row[0]))
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", root, "--json", string(source)}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("native date roundtrip succeeded; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			var result struct{ Debug []string }
			decodeCLIEnvelopeData(t, stdout.Bytes(), "exec", &result)
			if len(result.Debug) != 1 || !strings.HasPrefix(result.Debug[0], "OBS|") {
				t.Fatalf("missing date observation: %v", result.Debug)
			}
			var got, want map[string]any
			if json.Unmarshal([]byte(strings.TrimPrefix(result.Debug[0], "OBS|")), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatal("invalid date observation")
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
	if len(seen) != 2 {
		t.Fatal("missing native date API")
	}
}
