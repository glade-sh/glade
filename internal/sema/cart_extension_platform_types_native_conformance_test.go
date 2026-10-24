package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCartExtensionPlatformTypesNativeConformance(t *testing.T) {
	dir := "testdata/cart_extension_platform_types"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]struct{ name, source string }, len(cases))
	for _, c := range cases {
		if c.ID == "" || c.Name == "" || c.Source == "" {
			t.Fatal("incomplete synthetic source case")
		}
		if _, exists := sources[c.ID]; exists {
			t.Fatal("duplicate source case: " + c.ID)
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
	accepted, rejected := 0, 0
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
			t.Fatal("missing synthetic source: " + row[2])
		}
		t.Run(key, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{c.name + ".cls": c.source}, row[0])
			switch row[3] {
			case "accepted":
				accepted++
				if result.HasErrors() {
					t.Fatalf("native accepted; Glade diagnostics: %+v", result.Diagnostics)
				}
			case "rejected":
				rejected++
				if row[4] == "" {
					t.Fatal("native rejection has no diagnostic")
				}
				if !result.HasErrors() {
					t.Fatalf("native rejected with %q; Glade accepted", row[4])
				}
			default:
				t.Fatalf("unexpected native compile outcome %q", row[3])
			}
		})
	}
	if accepted != 10 || rejected != 6 || len(seen) != 16 {
		t.Fatalf("incomplete native compile matrix: accepted=%d rejected=%d keys=%d", accepted, rejected, len(seen))
	}
}
