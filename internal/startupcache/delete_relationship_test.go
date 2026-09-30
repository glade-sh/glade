package startupcache

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestCachedSetNullRelationshipPreservesDeleteEffect(t *testing.T) {
	for _, subdir := range []string{SubdirTest, ".glade/delete-json"} {
		t.Run(subdir, func(t *testing.T) {
			root := t.TempDir()
			writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[],"sourceApiVersion":"53.0"}`)
			proof, err := ValidateInputWithSourceDigests(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			parentID, childID := storage.ID("001000000000001"), storage.ID("003000000000001")
			org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{}}, Records: map[storage.ID]storage.Record{parentID: {ID: parentID, Object: "Account"}}}
			org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}, Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"Account"}, SetNullOnDelete: true}}}, Records: map[storage.ID]storage.Record{childID: {ID: childID, Object: "Contact", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parentID)}}}}
			entry, err := NewEntryWithValidatedInput(proof, org, CompiledRuntime{})
			if err != nil {
				t.Fatal(err)
			}
			entry.RuntimeABI, entry.RuntimeKey = "delete-relationship-test", "delete-key"
			if err := Write(&entry, subdir); err != nil {
				t.Fatal(err)
			}
			got, err := ReadFreshRuntimeWithValidatedInput(root, subdir, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
			if err != nil || got == nil {
				t.Fatalf("fresh cache=%#v err=%v", got, err)
			}
			engine := dml.NewEngine(&got.Org)
			result := engine.Delete([]storage.Record{{ID: parentID, Object: "Account"}})
			if len(result) != 1 || !result[0].Success {
				t.Fatalf("delete=%#v", result)
			}
			child := got.Org.Objects["Contact"].Records[childID]
			if !child.HasExplicitNull("Parent__c") {
				t.Fatalf("cached relationship lost delete effect: %#v", child)
			}
			entry.Version = 5
			if err := Write(&entry, subdir); err != nil {
				t.Fatal(err)
			}
			got, err = ReadFreshRuntimeWithValidatedInput(root, subdir, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
			if err != nil || got != nil {
				t.Fatalf("pre-relationship cache accepted=%#v err=%v", got, err)
			}
		})
	}
}
