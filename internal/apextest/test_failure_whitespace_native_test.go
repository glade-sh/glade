package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFailureWhitespaceNative(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "test_failure_whitespace")
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSuffix(string(body), "\n"), "\n") {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 {
			t.Fatalf("invalid native row: %q", line)
		}
		rows = append(rows, row)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyPermsetMutingProject(t, filepath.Join(dir, "project"), api)
			index := loadTestIndex(t, root)
			className := "ReportShapeV" + strings.TrimSuffix(api, ".0")
			run := Run(index, Options{SelectedClasses: []string{className}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 {
				t.Fatalf("expected one suite: %+v", run.Summary())
			}
			found := map[string]string{}
			for _, result := range run.Suites[0].Cases {
				if result.Problem == nil {
					t.Errorf("%s unexpectedly passed", result.MethodName)
					continue
				}
				name := result.Problem.Type
				if !strings.Contains(name, ".") {
					name = "System." + name
				}
				found[strings.ToLower(result.MethodName)] = name + ": " + result.Problem.Message
			}
			n := 0
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				n++
				t.Run(row[2], func(t *testing.T) {
					var want string
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					if got := found[strings.ToLower(row[2])]; got != want {
						t.Fatalf("native=%q; Glade=%q", want, got)
					}
				})
			}
			if n != 6 {
				t.Fatalf("native rows: %d", n)
			}
		})
	}
}
