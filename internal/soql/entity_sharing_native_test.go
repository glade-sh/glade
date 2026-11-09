package soql

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestEntitySharingModelNativeRowsAndWarmState(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/entity_sharing_models/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("native rows=%d, want 4", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			var want map[string]string
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			for name, sharing := range want {
				storage.EnsureStandardObject(&org, name)
				state := org.Objects[name]
				state.Definition.APIName = name
				state.Definition.SharingModel = sharing
				org.Objects[name] = state
			}
			check := func(state storage.OrgState, name, expected string) {
				t.Helper()
				result, err := ParseAndExecute(state, "SELECT QualifiedApiName, InternalSharingModel FROM EntityDefinition WHERE QualifiedApiName = '"+name+"'")
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Records) != 1 {
					t.Fatalf("%s: records=%d", name, len(result.Records))
				}
				if got := result.Records[0].Fields["InternalSharingModel"].String; got != expected {
					t.Errorf("%s: Glade=%q native=%q", name, got, expected)
				}
			}
			for name, sharing := range want {
				check(org, name, sharing)
			}
			view := VirtualSchemaView(org)
			changed := view.Objects["OwnedPrivate__c"]
			changed.Definition.SharingModel = want["OwnedPublic__c"]
			view.Objects["OwnedPrivate__c"] = changed
			check(view, "OwnedPrivate__c", want["OwnedPublic__c"])
			cleared := view.Objects["OwnedPrivate__c"]
			cleared.Definition.SharingModel = ""
			view.Objects["OwnedPrivate__c"] = cleared
			check(view, "OwnedPrivate__c", "")
		})
	}
}

// Explicitly supplied EntityDefinition records retain their own values; only
// fields synthesized by the virtual view follow later definition changes.
func TestEntitySharingProjectionPreservesExplicitRecords(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	account := org.Objects["Account"]
	account.Definition.SharingModel = "ReadWrite"
	org.Objects["Account"] = account
	storage.EnsureStandardObject(&org, "EntityDefinition")
	entities := org.Objects["EntityDefinition"]
	if entities.Records == nil {
		entities.Records = map[storage.ID]storage.Record{}
	}
	entities.Records[storage.ID("Account")] = storage.Record{ID: storage.ID("Account"), Object: "EntityDefinition", Fields: map[string]storage.Value{
		"QualifiedApiName": storage.StringValue("Account"), "InternalSharingModel": storage.StringValue("Private"),
	}}
	org.Objects["EntityDefinition"] = entities
	view := VirtualSchemaView(org)
	if got := view.Objects["EntityDefinition"].Records[storage.ID("Account")].Fields["InternalSharingModel"].String; got != "Private" {
		t.Fatalf("explicit field changed to %q", got)
	}
}
