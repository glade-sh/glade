package startupcache

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCachedReferenceIndexPreservesIDIdentity(t *testing.T) {
	for _, subdir := range []string{SubdirTest, ".glade/reference-json"} {
		t.Run(subdir, func(t *testing.T) {
			root := t.TempDir()
			writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[],"sourceApiVersion":"53.0"}`)
			proof, err := ValidateInputWithSourceDigests(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.Objects["Contact"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}, Indexes: []storage.IndexDefinition{{Name: "Contact.Account", Object: "Contact", Fields: []string{"AccountId"}}}},
				Records:    map[storage.ID]storage.Record{"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000001AAA")}}},
			}
			storage.RebuildIndexes(&org)
			entry, err := NewEntryWithValidatedInput(proof, org, CompiledRuntime{})
			if err != nil {
				t.Fatal(err)
			}
			entry.RuntimeABI, entry.RuntimeKey = "reference-index-test", "reference-key"
			if err := Write(&entry, subdir); err != nil {
				t.Fatal(err)
			}
			restored, err := ReadFreshRuntimeWithValidatedInput(root, subdir, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
			if err != nil || restored == nil {
				t.Fatalf("restore=%#v err=%v", restored, err)
			}
			for _, id := range []storage.ID{"001000000000001", "001000000000001AAA"} {
				rows, ok := storage.LookupIndex(restored.Org.Objects["Contact"], "AccountId", storage.IDValue(id))
				if !ok || len(rows) != 1 || rows[0] != "003000000000001" {
					t.Fatalf("restored lookup %s=%v indexed=%t", id, rows, ok)
				}
			}
			entry.Version = 6
			if err := Write(&entry, subdir); err != nil {
				t.Fatal(err)
			}
			restored, err = ReadFreshRuntimeWithValidatedInput(root, subdir, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
			if err != nil || restored != nil {
				t.Fatalf("old index cache accepted=%#v err=%v", restored, err)
			}
		})
	}
}
