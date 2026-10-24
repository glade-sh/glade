package storage

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
)

func childEdgeFlag(field *Field, flag string) **bool {
	switch flag {
	case "nillable":
		return &field.Nillable
	case "createable":
		return &field.Createable
	case "updateable":
		return &field.Updateable
	case "defaultedOnCreate":
		return &field.DefaultedOnCreate
	case "accessible":
		return &field.Accessible
	default:
		panic(flag)
	}
}

// Every captured facet must be explicit after repair, including native false.
func TestPlatformChildUnsetFlagStorageRepairRoutes(t *testing.T) {
	f, err := os.Open("../../testdata/conformance/standard_platform_child_fields/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, record := range records[1:] {
		if record[1] != "anonymous" {
			continue
		}
		row := strings.Split(record[2], "|")
		rows++
		for flag, column := range map[string]int{"nillable": 6, "createable": 7, "updateable": 8, "defaultedOnCreate": 9, "accessible": 11} {
			t.Run(record[0]+"/"+row[0]+"/"+row[1]+"/"+flag, func(t *testing.T) {
				original := enrichedStandardOverlayDefinition(row[2], nil)
				field := original.Fields[row[3]]
				if p := *childEdgeFlag(&field, flag); p == nil || *p != (row[column] == "true") {
					t.Fatal("fresh catalog differs from native facet")
				}
				definition := original.Clone()
				field.Label = "Owned edge label"
				*childEdgeFlag(&field, flag) = nil
				definition.Fields[row[3]] = field
				for route, check := range map[string]func(ObjectDefinition, string) bool{
					"current-write":      standardObjectFieldsNeedWrite,
					"current-refresh":    standardFieldsOverlayNeedsRefresh,
					"unmemoized-write":   legacyStandardObjectFieldsNeedWrite,
					"unmemoized-refresh": legacyStandardFieldsOverlayNeedsRefresh,
				} {
					t.Run(route, func(t *testing.T) {
						if check(original, "") {
							t.Fatal("explicit native flag requests repair")
						}
						if !check(definition, "") {
							t.Error("unset native flag does not request repair")
						}
					})
				}
				for route, repair := range map[string]func(*ObjectDefinition){
					"ensure-fields":   EnsureStandardObjectFields,
					"refresh-overlay": func(d *ObjectDefinition) { RefreshStandardObjectFieldOverlay(d) },
				} {
					t.Run(route, func(t *testing.T) {
						repaired := definition.Clone()
						repair(&repaired)
						field := repaired.Fields[row[3]]
						if p := *childEdgeFlag(&field, flag); p == nil || *p != (row[column] == "true") {
							t.Errorf("unset %s not restored to native %s", flag, row[column])
						}
						if field.Label != "Owned edge label" {
							t.Error("repair lost label")
						}
						if StandardObjectFieldsNeedWrite(repaired) {
							t.Error("repair is not idempotent")
						}
					})
				}
			})
		}
	}
	if rows != 23 {
		t.Fatalf("got %d native rows, want 23", rows)
	}
}
