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

func TestStandardAnonymousRouteNativeConformance(t *testing.T) {
	dir := "../apextest/testdata/conformance/standard_anonymous_route"
	file, err := os.Open(filepath.Join(dir, "anonymous.tsv"))
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
	if len(rows) != 25 || strings.Join(rows[0], ",") != "source_api,method,observation" {
		t.Fatal("incomplete native route matrix")
	}
	source, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeTestFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", row[0]))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			marker := "OBSERVED|" + row[1] + "|"
			line := ""
			for _, candidate := range strings.Split(string(source), "\n") {
				if strings.Contains(candidate, marker) {
					line = candidate
				}
			}
			if line == "" {
				t.Fatal("missing captured anonymous source")
			}
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", root, "--json", line}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("native query and insert succeeded; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			var envelope struct{ Data struct{ Debug []string } }
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			payload := ""
			for _, message := range envelope.Data.Debug {
				if _, value, ok := strings.Cut(message, marker); ok {
					payload = value
				}
			}
			var got, want map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatalf("missing observation: %s", stdout.String())
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", row[2], payload)
			}
		})
	}
}
