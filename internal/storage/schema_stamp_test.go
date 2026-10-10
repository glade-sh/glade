package storage

import (
	"reflect"
	"testing"
)

func TestEnsureStandardObjectKeepsRuntimeSchemaStampOnlyForNoop(t *testing.T) {
	ensured := func() OrgState {
		org := NewOrgState()
		EnsureStandardObject(&org, "PermissionSetAssignment")
		return org
	}

	org := ensured()
	org.RuntimeSchemaStamp = "trusted"
	before := org.Objects["PermissionSetAssignment"].Definition.Clone()
	EnsureStandardObject(&org, "PermissionSetAssignment")
	if org.RuntimeSchemaStamp != "trusted" {
		t.Fatal("no-op EnsureStandardObject dropped the runtime schema stamp")
	}
	if after := org.Objects["PermissionSetAssignment"].Definition; !reflect.DeepEqual(after, before) {
		t.Fatal("EnsureStandardObject changed a definition it reported as a no-op")
	}

	cases := []struct {
		name   string
		mutate func(*OrgState) string
	}{
		{"new object", func(org *OrgState) string { return "Lead" }},
		{"definition repair", func(org *OrgState) string {
			psa := org.Objects["PermissionSetAssignment"]
			psa.Definition = psa.Definition.Clone()
			psa.Definition.PluralLabel = ""
			org.Objects["PermissionSetAssignment"] = psa
			return "PermissionSetAssignment"
		}},
		{"field overlay refresh", func(org *OrgState) string {
			psa := org.Objects["PermissionSetAssignment"]
			psa.Definition = psa.Definition.Clone()
			psa.Definition.Metadata = map[string]string{standardFieldsOverlayMarker: "PersonAccounts"}
			org.Objects["PermissionSetAssignment"] = psa
			return "PermissionSetAssignment"
		}},
		{"record types", func(org *OrgState) string {
			psa := org.Objects["PermissionSetAssignment"]
			psa.Definition = psa.Definition.Clone()
			psa.Definition.RecordTypes = []RecordTypeInfo{{DeveloperName: "Partner", Name: "Partner", Active: true, Available: true}}
			org.Objects["PermissionSetAssignment"] = psa
			return "PermissionSetAssignment"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			org := ensured()
			name := tc.mutate(&org)
			org.RuntimeSchemaStamp = "trusted"
			EnsureStandardObject(&org, name)
			if org.RuntimeSchemaStamp != "" {
				t.Fatalf("EnsureStandardObject(%s) kept the runtime schema stamp", name)
			}
		})
	}
}
