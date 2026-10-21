package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCartExtensionSurfaceNativeConformance(t *testing.T) {
	dir := "testdata/cart_extension_surface"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	sources := map[string]struct{ name, source string }{}
	for _, c := range cases {
		if c.ID == "" || c.Name == "" || c.Source == "" {
			t.Fatal("incomplete source case")
		}
		if _, exists := sources[c.ID]; exists {
			t.Fatal("duplicate source case")
		}
		sources[c.ID] = struct{ name, source string }{c.Name, c.Source}
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "api,route,case,outcome,diagnostic" {
		t.Fatal("invalid native table")
	}
	seen := map[string]bool{}
	boundaries, checked := 0, 0
	for _, row := range rows[1:] {
		if row[1] != "deploy" {
			continue
		}
		key := row[0] + "/" + row[2]
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native key %q", key)
		}
		seen[key] = true
		c, exists := sources[row[2]]
		if !exists {
			t.Fatal("missing source: " + row[2])
		}
		if row[3] == "boundary" {
			if !strings.Contains(row[4], "Type is not visible: CartExtension.InventoryCartCalculator") {
				t.Fatal("unexpected boundary: " + row[4])
			}
			boundaries++
			t.Logf("%s environment boundary: %s", key, row[4])
			continue
		}
		checked++
		t.Run(key, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{c.name + ".cls": c.source}, row[0])
			if row[3] == "accepted" {
				if result.HasErrors() {
					t.Fatalf("native accepted; Glade diagnostics: %+v", result.Diagnostics)
				}
				return
			}
			if row[3] != "rejected" || row[4] == "" {
				t.Fatal("unexpected native outcome: " + row[3])
			}
			for _, d := range result.Diagnostics {
				if d.NativeMessage == row[4] || d.Message == row[4] {
					return
				}
			}
			t.Fatalf("native rejected: %s; Glade diagnostics: %+v", row[4], result.Diagnostics)
		})
	}
	if checked != 48 || boundaries != 6 || len(seen) != 54 {
		t.Fatalf("incomplete native table: checked=%d boundaries=%d keys=%d", checked, boundaries, len(seen))
	}
}
