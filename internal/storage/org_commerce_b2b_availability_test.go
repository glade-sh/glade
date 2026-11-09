package storage

import (
	"encoding/csv"
	"os"
	"testing"
)

// Native P10-shaped B2B capture: org_commerce_b2b/org.tsv.
func TestOrgCommerceB2BAvailabilityShapes(t *testing.T) {
	for _, name := range []string{"BuyerGroup", "CartItem", "CartTax"} {
		t.Run(name, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, name)
			ApplyProjectOrgShape(&org, []string{"B2BCommerce", "Communities", "NetworksEnabled", "CommerceEnabled"})
			if !org.AllowsStandardObject(name) {
				t.Fatalf("B2B shape rejects %s", name)
			}
			if _, ok := org.Objects[name]; !ok {
				t.Fatalf("B2B shape lacks %s", name)
			}
		})
	}
}

// The complete rows include both API versions and anonymous and test routes.
func TestOrgCommerceB2BCapturedFamily(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/org_commerce_b2b/org.tsv")
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
	org := NewOrgState()
	ApplyProjectOrgShape(&org, []string{"B2BCommerce", "Communities", "NetworksEnabled", "CommerceEnabled"})
	for _, row := range rows[1:] {
		if row[1] != row[2] || row[3] != row[4] || row[1] != row[3] {
			continue
		}
		if got, want := org.AllowsStandardObject(row[0]), row[1] == "111"; got != want {
			t.Errorf("%s: got %t, want %t", row[0], got, want)
		}
	}
}
