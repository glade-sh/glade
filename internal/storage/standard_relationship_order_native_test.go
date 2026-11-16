package storage

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"
)

// The native sweep describes every reference field of every standard object
// and reports getRelationshipOrder. Only the fields listed in the org.tsv have
// a non-null order. Every other reference field on the same objects is null.
// NATIVE getRelationshipOrder relationship_order_standard/org.tsv anonymous 62.0 67.0
func TestStandardRelationshipOrderNativeConformance(t *testing.T) {
	const path = "../apextest/testdata/conformance/relationship_order_standard/org.tsv"
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 100 || rows[0][0] != "api" || rows[0][3] != "relationshipOrder" {
		t.Fatalf("incomplete native table: rows=%d err=%v", len(rows), err)
	}
	type key struct{ object, field string }
	want := map[key]int{}
	for _, row := range rows[1:] {
		order, err := strconv.Atoi(row[3])
		if err != nil {
			t.Fatalf("invalid row %v", row)
		}
		k := key{row[1], row[2]}
		if prior, seen := want[k]; seen && prior != order {
			t.Fatalf("native orders differ across APIs for %v", k)
		}
		want[k] = order
	}
	objects := map[string]bool{}
	for k := range want {
		objects[k.object] = true
	}
	// The standard schema does not define these native reference fields at all.
	// That gap is separate from the order; each one needs its own native shape.
	schemaGaps := map[key]bool{
		{"ConsumptionRate", "ConsumptionScheduleId"}:                true,
		{"SchedulingObjectiveParameter", "SchedulingObjectiveId"}:   true,
		{"SchedulingWorkspaceTerritory", "SchedulingWorkspaceId"}:   true,
		{"ServiceAppointmentCapacityUsage", "ServiceAppointmentId"}: true,
	}
	known := 0
	for object := range objects {
		definition, ok := StandardObjectDefinition(object)
		if !ok {
			// A few rows belong to objects the standard catalog does not
			// know yet; they have no definition to carry the order.
			continue
		}
		known++
		for name, field := range definition.Fields {
			if field.Type != FieldReference {
				continue
			}
			if _, listed := want[key{object, name}]; !listed && field.RelationshipOrder != nil {
				t.Errorf("%s.%s relationshipOrder = %v, native null", object, name, describeOrder(field.RelationshipOrder))
			}
		}
		for k, expected := range want {
			if k.object != object {
				continue
			}
			field, ok := definition.Fields[k.field]
			if !ok && schemaGaps[k] {
				continue
			}
			if !ok {
				t.Errorf("%s.%s is a native reference field missing from the standard definition (order %d)", k.object, k.field, expected)
				continue
			}
			if field.RelationshipOrder == nil || *field.RelationshipOrder != expected {
				t.Errorf("%s.%s relationshipOrder = %v, native %d", k.object, k.field, describeOrder(field.RelationshipOrder), expected)
			}
		}
	}
	if known < len(objects)*9/10 {
		t.Fatalf("standard catalog knows %d of %d native objects", known, len(objects))
	}
}

func describeOrder(order *int) string {
	if order == nil {
		return "null"
	}
	return strconv.Itoa(*order)
}
