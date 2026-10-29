package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func relationshipScanOrg(unrelated int) storage.OrgState {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Objects["Parcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parcel__c",
		Fields: map[string]storage.Field{
			"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		},
		Relations: []storage.Relationship{{Field: "Account__c", ParentObjects: []string{"Account"}, ChildRelationship: "Parcels__r"}},
	}, Records: map[storage.ID]storage.Record{}}
	for i := 0; i < unrelated; i++ {
		name := fmt.Sprintf("Island%03d__c", i)
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: name,
			Fields:  map[string]storage.Field{"Name__c": {APIName: "Name__c", Type: storage.FieldString}},
		}, Records: map[storage.ID]storage.Record{}}
	}
	return org
}

// A template org's child relationship lookups scan only the objects that can
// target the parent, not every object in the org.
func TestChildRelationshipLookupScansOnlyCandidateChildren(t *testing.T) {
	const unrelated = 40
	template := storage.NewRuntimeTemplate(relationshipScanOrg(unrelated))
	PrimeRuntimeTemplateSchema(&template)
	machine := New(nil)
	machine.PrimeMetadataSchema(&template.Org)
	org := template.CloneRuntimeOrg()
	machine.SetRuntimeTemplateOrg(&org)
	if machine.Org.RuntimeSchemaStamp == "" {
		t.Fatal("template org has no runtime schema stamp")
	}

	before := childRelationshipScanVisits.Load()
	lookup := machine.childRelationshipLookup("Account", "Parcels__r")
	if lookup.ChildType != "Parcel__c" {
		t.Fatalf("child type = %q, want Parcel__c", lookup.ChildType)
	}
	lazy := machine.lazyChildRelationshipLookup("Account", "Parcels__r")
	if !lazy.OK || lazy.ChildType != "Parcel__c" {
		t.Fatalf("lazy lookup = %+v, want Parcel__c", lazy)
	}
	loaded := machine.loadedChildRelationshipLookup("Account", "Account", "Parcels__r")
	if len(loaded.ChildRelationshipNames) == 0 {
		t.Fatalf("loaded lookup = %+v, want Parcels__r", loaded)
	}
	visits := childRelationshipScanVisits.Load() - before
	// Three lookups over at most the objects referencing Account (Parcel__c and
	// the standard objects that EnsureStandardObject wires to Account).
	limit := uint64(3 * (len(machine.Org.Objects) - unrelated))
	if visits > limit {
		t.Fatalf("relationship scans visited %d objects, want at most %d (org has %d objects)", visits, limit, len(machine.Org.Objects))
	}

	// A mutable org narrows the scan too, and still sees schema added later:
	// installing an object changes the object count and in-place definition
	// changes clear the schema stamp.
	plain := New(nil)
	plainOrg := relationshipScanOrg(unrelated)
	plain.SetOrg(&plainOrg)
	before = childRelationshipScanVisits.Load()
	if got := plain.childRelationshipLookup("Account", "Parcels__r").ChildType; got != "Parcel__c" {
		t.Fatalf("mutable org child type = %q, want Parcel__c", got)
	}
	if visits := childRelationshipScanVisits.Load() - before; visits > uint64(len(plain.Org.Objects)-unrelated) {
		t.Fatalf("mutable org scan visited %d objects, want at most %d", visits, len(plain.Org.Objects)-unrelated)
	}
	plain.Org.Objects["Island000__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Island000__c",
		Fields:    map[string]storage.Field{"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}},
		Relations: []storage.Relationship{{Field: "Account__c", ParentObjects: []string{"Account"}, ChildRelationship: "Islands__r"}},
	}, Records: map[storage.ID]storage.Record{}}
	plain.Org.ClearRuntimeSchemaStamp()
	if got := plain.childRelationshipLookup("Account", "Islands__r").ChildType; got != "Island000__c" {
		t.Fatalf("child type after schema change = %q, want Island000__c", got)
	}
	plain.Org.Objects["Harbor__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Harbor__c",
		Fields:    map[string]storage.Field{"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}},
		Relations: []storage.Relationship{{Field: "Account__c", ParentObjects: []string{"Account"}, ChildRelationship: "Harbors__r"}},
	}, Records: map[storage.ID]storage.Record{}}
	if got := plain.childRelationshipLookup("Account", "Harbors__r").ChildType; got != "Harbor__c" {
		t.Fatalf("child type after installing an object = %q, want Harbor__c", got)
	}
}
