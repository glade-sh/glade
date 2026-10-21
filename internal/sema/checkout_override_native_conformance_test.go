package sema

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
)

func TestCheckoutValidateNativeOverrideConformance(t *testing.T) {
	source, err := os.ReadFile("testdata/checkout_override/OrchardCheckout.cls")
	if err != nil {
		t.Fatal(err)
	}
	testSource, err := os.ReadFile("testdata/checkout_override/OrchardCheckoutTest.cls")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/checkout_override/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 7 {
		t.Fatalf("native matrix: rows=%d err=%v", len(rows), err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[2] != "override" || row[3] != "true" || (row[0] != "62.0" && row[0] != "67.0" && row[0] != "68.0") || (row[1] != "anonymous" && row[1] != "istest") {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[0] + "/" + row[1]
		if seen[key] {
			t.Fatal("duplicate native row: " + key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			files := map[string]string{"OrchardCheckout.cls": string(source)}
			if row[1] == "istest" {
				files["OrchardCheckoutTest.cls"] = string(testSource)
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, files, row[0])
			if result.HasErrors() {
				t.Fatalf("native override accepted; Glade diagnostics: %+v", result.Diagnostics)
			}
		})
	}
}
