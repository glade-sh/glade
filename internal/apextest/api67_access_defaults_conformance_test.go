package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Native rows for the API 66, 67 and 68 default access modes: sharing keyword
// defaults and inheritance, unqualified DML and SOQL (static, dynamic and
// WithBinds) as a Standard User without FLS, and the same statements inside a
// trigger body. testdata/conformance/api67_access_defaults/ORG-RUN.txt names
// the capture, including SOSL with an inaccessible field and
// getQueryLocatorWithBinds in USER_MODE.
func TestAPI67AccessDefaultsConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "api67_access_defaults")
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
	if len(rows) < 2 || strings.Join(rows[0], ",") != "source_api,test,row,observation" {
		t.Fatal("incomplete native matrix")
	}
	want := map[string]map[string][]string{}
	for _, row := range rows[1:] {
		if want[row[0]] == nil {
			want[row[0]] = map[string][]string{}
		}
		want[row[0]][row[1]] = append(want[row[0]][row[1]], row[3])
	}
	recordID := regexp.MustCompile(`\ba[A-Za-z0-9]{14}(?:[A-Za-z0-9]{3})?\b`)
	normalize := func(line string) string {
		line = strings.Join(strings.Fields(line), " ")
		return recordID.ReplaceAllStringFunc(line, func(id string) string {
			if strings.ContainsAny(id, "0123456789") {
				return "ID"
			}
			return id
		})
	}
	for _, api := range []string{"66.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
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
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			class := "GlT" + strings.TrimSuffix(api, ".0")
			result := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
			if len(result.Suites) != 1 {
				t.Fatalf("suites=%d: %+v", len(result.Suites), result.Summary())
			}
			seen := 0
			for _, c := range result.Suites[0].Cases {
				expected := want[api][c.MethodName]
				if len(expected) == 0 {
					continue
				}
				seen++
				t.Run(c.MethodName, func(t *testing.T) {
					if c.Problem == nil {
						t.Fatal("test method unexpectedly passed")
					}
					message := strings.TrimPrefix(c.Problem.Message, "Assertion Failed: RESULT ")
					var actual []string
					for _, part := range strings.Split(message, " ;; ") {
						if part = normalize(part); part != "" {
							actual = append(actual, part)
						}
					}
					if len(actual) != len(expected) {
						t.Fatalf("rows=%d, native %d\nGlade: %q\nnative: %q", len(actual), len(expected), actual, expected)
					}
					for i := range expected {
						if actual[i] != expected[i] {
							t.Errorf("row %d\n native: %s\n Glade:  %s", i, expected[i], actual[i])
						}
					}
				})
			}
			if seen != len(want[api]) {
				t.Fatalf("ran %d of %d native tests", seen, len(want[api]))
			}
		})
	}
}
