package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

func TestCustomMetadataPrefixesKeepExplicitAndSeparateLocalPool(t *testing.T) {
	prefixes := AssignDeterministicPrefixes([]string{"A__mdt", "B__mdt", "pkg__C__mdt", "Widget__c", "Imported__mdt", "Account"}, map[string]string{"Imported__mdt": "z91"})
	for name, want := range map[string]string{"A__mdt": "m00", "B__mdt": "m01", "pkg__C__mdt": "m02", "Imported__mdt": "z91", "Account": "001", "Widget__c": "a02"} {
		if got := prefixes[name]; got != want {
			t.Errorf("%s=%s want%s", name, got, want)
		}
	}
}

func TestCustomMetadataPrefixRepairPreservesLaterProvidedPrefix(t *testing.T) {
	org := NewOrgState()
	org.Objects["A__mdt"] = ObjectState{Definition: ObjectDefinition{APIName: "A__mdt"}}
	org.Objects["B__mdt"] = ObjectState{Definition: ObjectDefinition{APIName: "B__mdt", KeyPrefix: "m00"}}
	org.Objects["Imported__mdt"] = ObjectState{Definition: ObjectDefinition{APIName: "Imported__mdt", KeyPrefix: "z91"}}
	EnsureUniqueKeyPrefixes(&org)
	for name, want := range map[string]string{"A__mdt": "m01", "B__mdt": "m00", "Imported__mdt": "z91"} {
		if got := org.Objects[name].Definition.KeyPrefix; got != want {
			t.Errorf("%s=%s want%s", name, got, want)
		}
	}
}

func TestCustomMetadataRecordPrefixAllocationReservesExistingTypes(t *testing.T) {
	org := NewOrgState()
	org.Objects["Z__mdt"] = ObjectState{Definition: ObjectDefinition{APIName: "Z__mdt", KeyPrefix: "m00"}}
	ensureCustomMetadataPrefixes(&org, []schema.CustomMetadataRecord{{ObjectName: "A__mdt"}})
	if got := org.Objects["A__mdt"].Definition.KeyPrefix; got != "m01" {
		t.Fatalf("new prefix=%s", got)
	}
	if got := org.Objects["Z__mdt"].Definition.KeyPrefix; got != "m00" {
		t.Fatalf("provided prefix=%s", got)
	}
}
