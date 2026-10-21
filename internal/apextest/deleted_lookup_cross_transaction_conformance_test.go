package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestDeletedLookupCrossTransactionNativeConformance(t *testing.T) {
	family := filepath.Join("testdata", "conformance", "deleted_lookup")
	data, err := os.Open(filepath.Join(family, "cross-transaction", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	reader := csv.NewReader(data)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatalf("incomplete native transaction rows: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(family, "project")
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
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			for _, route := range []string{"setup", "restore"} {
				body, err := os.ReadFile(filepath.Join(family, "cross-transaction", route+"-"+strings.TrimSuffix(api, ".0")+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatalf("%s: %v", route, err)
				}
				var expected string
				for _, row := range rows[1:] {
					if row[0] == api && row[1] == route {
						expected = row[2]
					}
				}
				var payload string
				for _, line := range result.Debug {
					if _, observed, found := strings.Cut(line, "OBSERVED|"); found {
						_, payload, _ = strings.Cut(observed, "|")
					}
				}
				var got, want map[string]any
				if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(expected), &want) != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("%s native=%s Glade=%s", route, expected, payload)
				}
			}
		})
	}
}
