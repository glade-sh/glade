package storage

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// The native parent-edge rows pin both the storage projection and the repair
// of a persisted overlay with stale facets, including read-only hidden fields
// and the non-nillable staged-sharing parent field.
func TestStandardParentPlatformChildFieldFacetsFreshAndPersistedRoutes(t *testing.T) {
	file, err := os.Open("../../testdata/conformance/standard_platform_child_fields/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	table, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, record := range table[1:] {
		if record[0] != "67.0" || record[1] != "anonymous" {
			continue
		}
		row := strings.Split(record[2], "|")
		if len(row) != 12 {
			t.Fatalf("invalid native row: %v", record)
		}
		rows++
		for _, route := range []string{"fresh-definition", "persisted-stale-facets"} {
			t.Run(row[0]+"/"+row[1]+"/"+route, func(t *testing.T) {
				org := NewOrgState()
				EnsureStandardObject(&org, row[2])
				definition := org.Objects[row[2]].Definition.Clone()
				name, ok := ResolveFieldName(definition, "", row[3])
				if !ok {
					t.Fatalf("native edge field %s.%s is absent", row[2], row[3])
				}
				if route == "persisted-stale-facets" {
					field := definition.Fields[name]
					field.Label = "Owned edge label"
					field.Type, field.DisplayType = FieldString, "STRING"
					field.ReferenceTo = nil
					field.RelationshipName, field.ChildRelationshipName = "Stale", "StaleChildren"
					field.Nillable = BoolFlag(row[6] != "true")
					field.Createable = BoolFlag(row[7] != "true")
					field.Updateable = BoolFlag(row[8] != "true")
					field.DefaultedOnCreate = BoolFlag(true)
					field.Accessible = BoolFlag(row[11] != "true")
					definition.Fields[name] = field
					if !StandardObjectFieldsNeedWrite(definition) {
						t.Fatal("persisted stale field facets are not reported for repair")
					}
					EnsureStandardObjectFields(&definition)
					if definition.Fields[name].Label != "Owned edge label" {
						t.Fatal("repair discarded the existing label")
					}
				}
				field := definition.Fields[name]
				refs := append([]string(nil), field.ReferenceTo...)
				sort.Strings(refs)
				got := []string{field.DisplayType, strings.Join(refs, ","),
					fmt.Sprint(FieldFlagValue(field.Nillable, false)),
					fmt.Sprint(FieldFlagValue(field.Createable, false)),
					fmt.Sprint(FieldFlagValue(field.Updateable, false)),
					fmt.Sprint(FieldFlagValue(field.DefaultedOnCreate, false)),
					field.RelationshipName, fmt.Sprint(FieldFlagValue(field.Accessible, false))}
				if field.Type != FieldReference || strings.Join(got, "|") != strings.Join(row[4:], "|") {
					t.Errorf("field facets = %v; native = %v", got, row[4:])
				}
				if field.ChildRelationshipName != row[1] {
					t.Errorf("child relationship = %q; native = %q", field.ChildRelationshipName, row[1])
				}
				if StandardObjectFieldsNeedWrite(definition) {
					t.Fatal("field projection is not idempotent")
				}
			})
		}
	}
	if rows != 15 {
		t.Fatalf("native guard has %d rows; want fifteen standard-parent edges", rows)
	}
}
