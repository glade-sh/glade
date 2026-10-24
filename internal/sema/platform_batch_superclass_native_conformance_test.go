package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlatformBatchSuperclassNativeConformance(t *testing.T) {
	const dir = "testdata/platform_batch_superclass"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	byID := map[string]struct{ name, source string }{}
	for _, c := range cases {
		if c.ID == "" || c.Name == "" || c.Source == "" || byID[c.ID].name != "" {
			t.Fatal("invalid source case")
		}
		byID[c.ID] = struct{ name, source string }{c.Name, c.Source}
	}
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(cases) != 7 || len(rows) != 1+2*len(cases) || strings.Join(rows[0], ",") != "api,route,case,outcome,diagnostics" {
		t.Fatalf("incomplete native matrix: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1] + "/" + row[2]
		c := byID[row[2]]
		if seen[key] || c.name == "" || (row[0] != "62.0" && row[0] != "67.0") || row[1] != "deploy" || (row[3] != "accepted" && row[3] != "rejected") {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[key] = true
		var messages []string
		if err := json.Unmarshal([]byte(row[4]), &messages); err != nil || (row[3] == "rejected") != (len(messages) > 0) {
			t.Fatalf("invalid native diagnostics: %v", row)
		}
		t.Run(key, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{c.name + ".cls": c.source}, row[0])
			if result.HasErrors() != (row[3] == "rejected") {
				t.Fatalf("native %s: %s; Glade diagnostics=%v", row[3], row[4], result.Diagnostics)
			}
			for _, message := range messages {
				found := false
				for _, d := range result.Diagnostics {
					if d.Message == message || d.NativeMessage == message {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing native diagnostic %q; Glade diagnostics=%v", message, result.Diagnostics)
				}
			}
		})
	}
}
