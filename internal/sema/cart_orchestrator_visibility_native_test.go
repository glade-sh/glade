package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCartOrchestratorVisibilityNativeConformance(t *testing.T) {
	dir := "testdata/cart_orchestrator_visibility"
	data, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]struct{ ID, Name, Source string })
	for _, c := range cases {
		sources[c.ID] = c
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 15 {
		t.Fatalf("native rows=%d, want 14", len(rows)-1)
	}
	checked := 0
	for _, row := range rows[1:] {
		if row[1] != "deploy" {
			continue
		}
		checked++
		c, ok := sources[row[2]]
		if !ok {
			t.Fatal("missing case", row[2])
		}
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{c.Name + ".cls": c.Source}, row[0])
			if row[3] == "accepted" && result.HasErrors() {
				t.Fatalf("native accepted: %+v", result.Diagnostics)
			}
			if row[3] == "rejected" && !result.HasErrors() {
				t.Fatalf("native rejected: %s", row[4])
			}
		})
	}
	if checked != 10 || !strings.HasPrefix(rows[0][0], "api") {
		t.Fatalf("invalid capture matrix: %d", checked)
	}
}
