package storage

import "testing"

// PU-workload-facets/org.tsv and PU-requesttype-picklist/org.tsv capture these named facets.
func TestStandardAPI68UnionNamedOnlyNative(t *testing.T) {
	fields := StandardAPI68DescribeFields("PU", "WorkloadUnit", true)
	field, ok := fields["IntradayActualVolume"]
	if !ok || field.Type != FieldInteger || StandardAPI68DescribeAccessRule("PU", "WorkloadUnit", "IntradayActualVolume") != "inaccessible" {
		t.Fatalf("union named-only workload field=%+v present=%t", field, ok)
	}
	if _, ok := StandardAPI68DescribeFields("PU", "WorkloadUnit", false)["IntradayActualVolume"]; ok {
		t.Fatal("union anonymous route exposes named-only field")
	}
	shift := StandardAPI68DescribeFields("PU", "ShiftAllocationRequest", true)["RequestType"]
	if len(shift.PicklistValues) != 1 || shift.PicklistValues[0].Value != "Pickup" || shift.PicklistValues[0].Label != "Pick Up" {
		t.Fatalf("union picklist=%+v", shift.PicklistValues)
	}
}
