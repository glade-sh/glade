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

func TestDerivedCollectionTraversalNativeDirectAnonymousRoutes(t *testing.T) {
	dir := filepath.Join("..", "apextest", "testdata", "conformance", "derived_collection_traversal")
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bodies map[string]string
	if err := json.Unmarshal(raw, &bodies); err != nil {
		t.Fatal(err)
	}
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
	if len(rows) != 1+2*len(bodies) {
		t.Fatal("incomplete direct-route oracle")
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/anonymous/"+row[1], func(t *testing.T) {
			body, ok := bodies[row[1]]
			if !ok {
				t.Fatal("missing captured body")
			}
			before, value, ok := strings.Cut(body, "return ")
			if !ok {
				t.Fatal("missing observation")
			}
			source := before + "System.debug('OBSERVED|'+JSON.serialize(" + strings.TrimSuffix(value, ";") + "));"
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			result, err := New(nil).Execute(program)
			var got map[string]any
			if err != nil {
				message := err.Error()
				kind := ""
				if _, tail, ok := strings.Cut(message, "System.FinalException: "); ok {
					kind = "System.FinalException"
					message = strings.SplitN(tail, "\n", 2)[0]
				}
				got = map[string]any{"exception": kind, "message": message}
			} else {
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "OBSERVED|"); ok {
						if err := json.Unmarshal([]byte(payload), &got); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%v", row[2], got)
			}
		})
	}
}
