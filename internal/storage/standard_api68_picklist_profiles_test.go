package storage

import "testing"

// Each applicable profile has an independent named API68 picklist capture.
func TestStandardAPI68NamedPicklistProfileCaptures(t *testing.T) {
	for _, profile := range []string{"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10", "PU"} {
		t.Run(profile, func(t *testing.T) {
			field, ok := StandardAPI68DescribeFields(profile, "ShiftAllocationRequest", true)["RequestType"]
			if !ok || len(field.PicklistValues) != 1 {
				t.Fatalf("named field=%t picklist=%+v", ok, field.PicklistValues)
			}
			value := field.PicklistValues[0]
			if value.Value != "Pickup" || value.Label != "Pick Up" || !value.Active || value.Default {
				t.Fatalf("native picklist entry mismatch: %+v", value)
			}
		})
	}
}
