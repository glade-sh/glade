package server

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Object-info is a local LWC bridge over the captured Apex org shape; this
// test does not infer Salesforce-hosted UI API error text or field ordering.
func TestB5B9FeatureObjectInfoBridge(t *testing.T) {
	for _, tc := range []struct {
		object   string
		features []string
	}{
		{"GiftTransaction", []string{"Fundraising", "FundraisingEnabled"}},
		{"GiftCommitment", []string{"Fundraising", "FundraisingEnabled"}},
		{"DocumentGenerationProcess", []string{"DocGen"}},
		{"DocumentTemplate", []string{"DocGen"}},
		{"RegulatoryCode", []string{"PublicSectorAccess"}},
	} {
		t.Run(tc.object, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.ApplyOrgShape(&org, nil)
			if _, err := getObjectInfoWireData(&org, tc.object); err == nil {
				t.Fatal("disabled feature exposed object info")
			}
			storage.ApplyOrgShape(&org, tc.features)
			if _, err := getObjectInfoWireData(&org, tc.object); err != nil {
				t.Fatalf("enabled feature rejected object info: %v", err)
			}
		})
	}
}

// The local object-info bridge uses the captured Commerce field policy; this
// does not claim an independent native UI API response shape.
func TestB5B9CommerceFieldObjectInfoBridge(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "license-only"
		features := []string{"B2BCommerce", "Communities"}
		if enabled {
			name = "enabled"
			features = append(features, "CommerceEnabled")
		}
		t.Run(name, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.ApplyOrgShape(&org, features)
			payload, err := getObjectInfoWireData(&org, "PricebookEntry")
			if err != nil {
				t.Fatal(err)
			}
			fields, ok := payload["fields"].(map[string]any)
			if !ok {
				t.Fatalf("object info fields=%#v", payload["fields"])
			}
			for _, field := range []string{"ProductSellingModelId", "ActivePriceAdjustmentQuantity"} {
				_, present := fields[field]
				if present != enabled {
					t.Errorf("%s present=%t want=%t", field, present, enabled)
				}
			}
		})
	}
}
