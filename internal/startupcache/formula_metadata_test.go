package startupcache

import (
	"github.com/glade-sh/glade/internal/storage"
	"path/filepath"
	"testing"
)

func TestFormulaMetadataFreshCacheRoundTrip(t *testing.T) {
	for _, subdir := range []string{SubdirTest, ".glade/formula-json"} {
		t.Run(subdir, func(t *testing.T) {
			root := t.TempDir()
			writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[],"sourceApiVersion":"53.0"}`)
			proof, err := ValidateInputWithSourceDigests(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.Objects["Policy__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Policy__c", Fields: map[string]storage.Field{
				"Total__c": {APIName: "Total__c", Type: storage.FieldCalculated, DisplayType: "CURRENCY", Formula: "Amount__c", Scale: 0, ScaleSpecified: true, FormulaTreatBlanksAs: "BlankAsZero"},
			}}}
			entry, err := NewEntryWithValidatedInput(proof, org, CompiledRuntime{})
			if err != nil {
				t.Fatal(err)
			}
			entry.RuntimeABI = "formula-policy-test"
			entry.RuntimeKey = "formula-key"
			if err := Write(&entry, subdir); err != nil {
				t.Fatal(err)
			}
			got, err := ReadFreshRuntimeWithValidatedInput(root, subdir, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
			if err != nil || got == nil {
				t.Fatalf("fresh cache=%#v err=%v", got, err)
			}
			field := got.Org.Objects["Policy__c"].Definition.Fields["Total__c"]
			if !field.ScaleSpecified || field.Scale != 0 || field.FormulaTreatBlanksAs != "BlankAsZero" {
				t.Fatalf("cached field=%#v", field)
			}
			entry.Version = Version - 1
			if err := Write(&entry, subdir); err != nil {
				t.Fatal(err)
			}
			got, err = ReadFreshRuntimeWithValidatedInput(root, subdir, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
			if err != nil || got != nil {
				t.Fatalf("old cache accepted=%#v err=%v", got, err)
			}
		})
	}
}
