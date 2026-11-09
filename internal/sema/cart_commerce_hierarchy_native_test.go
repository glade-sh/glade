package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// TestCartCommerceHierarchyNativeConformance compiles the owned synthetic
// declarations in testdata/cart_commerce_hierarchy at API 62.0 and 67.0 and
// compares the outcome of each with the native deployment row.
func TestCartCommerceHierarchyNativeConformance(t *testing.T) {
	runNativeDeployMatrix(t, "testdata/cart_commerce_hierarchy")
}

// runNativeDeployMatrix compiles every case in dir/cases.json for each API in
// dir/org.tsv in one project and compares each class with its native row.
func runNativeDeployMatrix(t *testing.T, dir string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]struct{ name, source string }, len(cases))
	for _, c := range cases {
		if c.ID == "" || c.Name == "" || c.Source == "" {
			t.Fatal("incomplete synthetic source case")
		}
		if _, dup := byID[c.ID]; dup {
			t.Fatal("duplicate source case: " + c.ID)
		}
		byID[c.ID] = struct{ name, source string }{c.Name, c.Source}
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "api,route,case,outcome,diagnostic" {
		t.Fatal("invalid native table")
	}
	perAPI := map[string][][]string{}
	for _, row := range rows[1:] {
		if row[1] != "deploy" || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("unexpected native row %v", row)
		}
		perAPI[row[0]] = append(perAPI[row[0]], row)
	}
	for api, apiRows := range perAPI {
		if len(apiRows) != len(cases) {
			t.Fatalf("API %s has %d rows for %d cases", api, len(apiRows), len(cases))
		}
		files := make(map[string]string, len(apiRows))
		for _, row := range apiRows {
			c, ok := byID[row[2]]
			if !ok {
				t.Fatal("missing synthetic source: " + row[2])
			}
			files[c.name+".cls"] = c.source
		}
		result := analyzeDeclarationProjectWithAPIVersion(t, files, api)
		failed := map[string][]string{}
		for _, d := range result.Diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			failed[strings.TrimSuffix(filepath.Base(d.File), ".cls")] = append(failed[strings.TrimSuffix(filepath.Base(d.File), ".cls")], d.Message)
		}
		for _, row := range apiRows {
			c := byID[row[2]]
			msgs := failed[c.name]
			switch row[3] {
			case "accepted":
				if len(msgs) != 0 {
					t.Errorf("API %s %s: native accepted; Glade: %v", api, row[2], msgs)
				}
			case "rejected":
				if row[4] == "" || row[4] == "-" {
					t.Fatal("native rejection has no diagnostic")
				}
				if len(msgs) == 0 {
					t.Errorf("API %s %s: native rejected with %q; Glade accepted", api, row[2], row[4])
				}
			default:
				t.Fatalf("unexpected outcome %q", row[3])
			}
		}
	}
}

// TestCommerceServiceOverridesNativeConformance covers subclasses of the
// platform inventory and pricing services in testdata/commerce_service_overrides.
func TestCommerceServiceOverridesNativeConformance(t *testing.T) {
	runNativeDeployMatrix(t, "testdata/commerce_service_overrides")
}
