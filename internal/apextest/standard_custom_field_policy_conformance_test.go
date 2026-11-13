package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Standard User running under System.runAs writes a custom field of a
// standard object (Account) that no permission grants. Native rows: explicit
// USER_MODE denies at API 62.0 and 67.0; the default mode denies only from
// source API 67.0; SYSTEM_MODE and a granted field always save. Denials are a
// statement DmlException or a failed SaveResult, never a SecurityException.
func TestStandardCustomFieldPolicyNativeConformance(t *testing.T) {
	const family = "testdata/conformance/standard_custom_field_policy"
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+2*36 || strings.Join(rows[0], ",") != "api,method,observation" {
		t.Fatalf("incomplete native rows: %d", len(rows))
	}
	root := t.TempDir()
	project := filepath.Join(family, "project")
	err = filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(project, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, rel), string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	for i, row := range rows[1:] {
		api, method, want := row[0], row[1], row[2]
		class := "PolicyProbe" + strings.TrimSuffix(api, ".0")
		line := i + 2
		t.Run(api+"/"+method, func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing runner observation %+v", run.Summary())
			}
			message := run.Suites[0].Cases[0].Problem.Message
			_, got, ok := strings.Cut(message, " => ")
			if !ok || got != want {
				t.Fatalf("NATIVE %s org.tsv:%d istest %s\nnative=%q\nglade =%q", method, line, api, want, message)
			}
		})
	}
}
