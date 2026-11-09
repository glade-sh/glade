package storage

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// TestIndustriesSchemaNativeFacts compares the effective standard definitions
// with the separately captured API and Apex-route describe observations.
func TestIndustriesSchemaNativeFacts(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_industries_schema/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		columns := strings.Split(scanner.Text(), "\t")
		if len(columns) != 5 || columns[0] != "combined" {
			continue
		}
		api, route, name := columns[1], columns[2], columns[3]
		result := strings.Split(columns[4], "|")
		if len(result) != 3 {
			t.Fatalf("invalid captured row %q", scanner.Text())
		}
		parts := strings.Split(name, ".")
		if len(parts) != 2 {
			t.Fatalf("invalid field %q", name)
		}
		count++
		t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
			d := ObjectDefinition{APIName: parts[0]}
			EnsureStandardObjectFieldsForFeatures(&d, []string{"PublicSectorAccess", "OrderManagement", "B2BCommerce", "Communities", "ContactsToMultipleAccounts", "CommerceEnabled", "MiddleNameEnabled"})
			field, ok := d.Fields[parts[1]]
			if !ok {
				t.Fatalf("field missing: %s", name)
			}
			if field.DisplayType != result[0] {
				t.Errorf("display type = %s, native %s", field.DisplayType, result[0])
			}
			if field.Nillable == nil || *field.Nillable != (result[1] == "true") {
				t.Errorf("nillable = %v, native %s", field.Nillable, result[1])
			}
			if field.DefaultedOnCreate == nil || *field.DefaultedOnCreate != (result[2] == "true") {
				t.Errorf("defaulted = %v, native %s", field.DefaultedOnCreate, result[2])
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 36 {
		t.Fatalf("captured rows = %d, want 36", count)
	}
}

func TestIndustriesSchemaIsolatedProfiles(t *testing.T) {
	tests := []struct {
		object, field string
		features      []string
		baseline      bool
	}{
		{"AccountContactRelation", "Roles", []string{"ContactsToMultipleAccounts"}, false},
		{"PricebookEntry", "ProductSellingModelId", []string{"B2BCommerce", "Communities", "CommerceEnabled"}, false},
		{"User", "MiddleName", []string{"MiddleNameEnabled"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.object+"."+tc.field, func(t *testing.T) {
			def := ObjectDefinition{APIName: tc.object}
			EnsureStandardObjectFieldsForFeatures(&def, nil)
			_, got := ResolveFieldName(def, "", tc.field)
			if got != tc.baseline {
				t.Errorf("baseline membership=%v, want %v", got, tc.baseline)
			}
			EnsureStandardObjectFieldsForFeatures(&def, tc.features)
			if _, ok := ResolveFieldName(def, "", tc.field); !ok {
				t.Errorf("enabled field missing")
			}
			EnsureStandardObjectFieldsForFeatures(&def, nil)
			if _, ok := ResolveFieldName(def, "", tc.field); ok != tc.baseline {
				t.Errorf("disabled membership=%v, want %v", ok, tc.baseline)
			}
			EnsureStandardObjectFieldsForFeatures(&def, tc.features)
			if _, ok := ResolveFieldName(def, "", tc.field); !ok {
				t.Errorf("re-enabled field missing")
			}
			configured := ObjectDefinition{APIName: tc.object, Fields: map[string]Field{tc.field: {APIName: tc.field, Type: FieldString, DisplayType: "STRING"}}}
			EnsureStandardObjectFieldsForFeatures(&configured, nil)
			if name, ok := ResolveFieldName(configured, "", tc.field); !ok || configured.Fields[name].DisplayType != "STRING" {
				t.Errorf("configured field not preserved: %#v", configured.Fields[tc.field])
			}
		})
	}
}

func TestIndustriesSchemaProvisionedOrgRoutes(t *testing.T) {
	features := []string{"ContactsToMultipleAccounts", "B2BCommerce", "Communities", "CommerceEnabled", "MiddleNameEnabled"}
	org := NewOrgState()
	ApplyOrgShape(&org, features)
	for _, row := range []struct {
		object, field string
		typ           FieldType
	}{
		{"AccountContactRelation", "Roles", FieldMultiPicklist},
		{"PricebookEntry", "ProductSellingModelId", FieldReference},
		{"User", "MiddleName", FieldString},
	} {
		t.Run(row.object+"."+row.field, func(t *testing.T) {
			for _, attempt := range []string{"cold", "warm"} {
				t.Run(attempt, func(t *testing.T) {
					EnsureStandardObject(&org, row.object)
					def, ok := org.Objects[row.object]
					if !ok {
						t.Fatalf("object missing")
					}
					name, ok := ResolveFieldName(def.Definition, "", row.field)
					if !ok {
						t.Fatalf("field missing")
					}
					if got := def.Definition.Fields[name].Type; got != row.typ {
						t.Fatalf("type=%s, want %s", got, row.typ)
					}
				})
			}
		})
	}
}
