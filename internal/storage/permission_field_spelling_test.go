package storage

import (
	"strings"
	"testing"
)

func TestPermissionFixtureFieldNamesMatchDescribe(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	for _, name := range []string{"ObjectPermissions", "FieldPermissions"} {
		t.Run(name, func(t *testing.T) {
			describe, found, err := lookupStandardDescribeCatalogV2(name)
			if err != nil || !found {
				t.Fatalf("describe found=%v error=%v", found, err)
			}
			canonical := ""
			for _, field := range describe.Fields {
				if strings.EqualFold(field.Name, "SObjectType") {
					canonical = field.Name
				}
			}
			if canonical == "" {
				t.Fatal("describe field missing")
			}
			object := org.Objects[name]
			field, found := object.Definition.Fields[canonical]
			if !found || field.APIName != canonical {
				t.Errorf("fixture field = %q, found=%v; native describe = %q", field.APIName, found, canonical)
			}
			if len(object.Records) == 0 {
				t.Fatal("permission records missing")
			}
			for id, record := range object.Records {
				if value, found := record.Fields[canonical]; !found || value.Kind != ValueString || value.String == "" {
					t.Errorf("record %s has no canonical %s value", id, canonical)
				}
				for key := range record.Fields {
					if strings.EqualFold(key, canonical) && key != canonical {
						t.Errorf("record %s uses %s instead of %s", id, key, canonical)
					}
				}
			}
		})
	}
}
