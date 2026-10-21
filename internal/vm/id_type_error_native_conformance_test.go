package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIDTypeErrorNativeConformance(t *testing.T) {
	const fixtures = "testdata/id_type_error"
	f, err := os.Open(filepath.Join(fixtures, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 3 || strings.Join(rows[0], ",") != "api,observation" {
		t.Fatalf("incomplete native Id matrix: rows=%d err=%v", len(rows), err)
	}
	source, err := os.ReadFile(filepath.Join(fixtures, "observe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		if len(row) != 2 || (row[0] != "62.0" && row[0] != "67.0") || seen[row[0]] {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[row[0]] = true
		t.Run(row[0], func(t *testing.T) {
			var want []any
			if err := json.Unmarshal([]byte(row[1]), &want); err != nil || len(want) != 7 {
				t.Fatalf("incomplete native observations: %v %v", want, err)
			}
			program, err := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			machine := schemaClassificationVM(t)
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatalf("native capture completed with caught errors; Glade escaped: %v", err)
			}
			if len(result.Debug) != 1 || !strings.HasPrefix(result.Debug[0], "OBS|") {
				t.Fatalf("missing observation: %v", result.Debug)
			}
			var got []any
			if err := json.Unmarshal([]byte(strings.TrimPrefix(result.Debug[0], "OBS|")), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", row[1], result.Debug[0])
			}
		})
	}
}
