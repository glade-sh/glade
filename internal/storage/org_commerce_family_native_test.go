package storage

import (
	"encoding/csv"
	"os"
	"testing"
)

// Every row is backed by both API versions and both anonymous/named routes.
func TestOrgCommerceFamilyAvailabilityCaptured(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/org_commerce_family/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 100 {
		t.Fatalf("insufficient native rows: %d", len(rows)-1)
	}
	org := NewOrgState()
	for _, row := range rows[1:] {
		EnsureStandardObject(&org, row[0])
	}
	ApplyProjectOrgShape(&org, nil)
	for _, row := range rows[1:] {
		if org.AllowsStandardObject(row[0]) {
			t.Errorf("plain shape exposes %s", row[0])
		}
	}
	ApplyProjectOrgShape(&org, []string{"OrderManagement"})
	for _, row := range rows[1:] {
		if !org.AllowsStandardObject(row[0]) {
			t.Errorf("enabled shape rejects %s", row[0])
		}
	}
}
