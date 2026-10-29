package vm

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native lifecycle and source-shadow twins at API 62/67 are retained in
// ../apextest/testdata/conformance/address_location_review/org.tsv. Values flow
// through the real SOQL, dynamic get, duplicate-match and builder producers.
func TestAddressLocationReviewNativeRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/address_location_review/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Body, Status string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	// Custom compound reads follow the train's custom Address rule (P02), which
	// resolves components through their CompoundFieldName links. The
	// addressfields schema always links them, so these rows run on the linked
	// shape only; the unlinked (inferred) shape reads null for them.
	linkedOnly := map[string]bool{
		"CUSTOM_QUERIED_DIRECT":        true,
		"CUSTOM_QUERIED_GET":           true,
		"DML_CLONE_DIRECT":             true,
		"DML_COMPONENT_AFTER_COMPOUND": true,
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, linked := range []bool{false, true} {
			shape := "inferred"
			if linked {
				shape = "linked"
			}
			for _, row := range cases {
				if row.Status != "PASS" || (!linked && linkedOnly[row.ID]) {
					continue
				}
				t.Run(api+"/"+shape+"/"+row.ID, func(t *testing.T) {
					org := testDataOrg()
					storage.EnsureStandardObject(&org, "Account")
					if row.ID == "DUPLICATE_SHADOW" {
						// The native scratch fixture has the standard Account rules.
						storage.EnsureDeterministicPlatformData(&org)
					}
					org.APIVersion = api
					account := org.Objects["Account"]
					account.Definition.Fields["Site__c"] = storage.Field{APIName: "Site__c", Type: storage.FieldAddress}
					for _, component := range []string{"Street", "Latitude", "Longitude"} {
						name := "Site__" + component + "__s"
						field := storage.Field{APIName: name, Type: storage.FieldDecimal}
						if component == "Street" {
							field.Type = storage.FieldString
						}
						if linked {
							field.CompoundFieldName = "Site__c"
						}
						account.Definition.Fields[name] = field
					}
					org.Objects["Account"] = account
					machine := New(nil)
					machine.EnableTestContext()
					machine.SetOrg(&org)
					for _, name := range []string{"Address", "Location"} {
						if err := machine.RegisterClass(Class{Name: name}); err != nil {
							t.Fatal(err)
						}
					}
					program, err := CompileAnonymousWithOptions(row.Body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// The platform-root guard must leave ordinary source inheritance untouched.
// SOURCE_BOUNDARY supplies the unrelated-source native control; these classes
// additionally exercise existing hierarchy traversal outside the platform pair.
func TestAddressLocationSourceHierarchyPreserved(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{
		{Name: "Location"},
		{Name: "Address", SuperClass: "Location"},
		{Name: "ParcelAddress", SuperClass: "Address"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	for _, pair := range [][2]string{{"Address", "Location"}, {"ParcelAddress", "Address"}, {"ParcelAddress", "Location"}} {
		if !machine.typeAssignableTo(pair[0], pair[1]) || !machine.typeMatches(pair[0], pair[1], make(map[string]bool)) {
			t.Fatalf("source hierarchy lost: %s -> %s", pair[0], pair[1])
		}
	}
}
