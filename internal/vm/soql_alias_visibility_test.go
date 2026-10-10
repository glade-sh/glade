package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSOQLRootToLabelAliasSelectionMarkers(t *testing.T) {
	machine := New(nil)
	for _, tc := range []struct {
		name, query                   string
		sourceSelected, aliasSelected bool
	}{
		{"alias", "SELECT Id,toLabel(Name) DisplayName FROM Account", false, true},
		{"case", "SELECT Id,TOLABEL(Name) DisplayName FROM Account", false, true},
		{"unaliased", "SELECT Id,toLabel(Name) FROM Account", true, false},
		{"sourceBeforeAlias", "SELECT Id,Name,toLabel(Name) DisplayName FROM Account", true, true},
		{"sourceAfterAlias", "SELECT Id,toLabel(Name) DisplayName,Name FROM Account", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields := machine.queriedSObjectFields(tc.query)
			if fields["name"] != tc.sourceSelected || fields["displayname"] != tc.aliasSelected || !fields["id"] {
				t.Fatalf("queried fields = %#v; want source=%v alias=%v and Id", fields, tc.sourceSelected, tc.aliasSelected)
			}
		})
	}
}

func TestRecordFromValueQueryProjectionAndAuthoredUnknownFields(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		queried, explicit, userSet, wantStored bool
	}{
		{"projection", true, false, false, false},
		{"authored", false, false, false, true},
		{"explicitOnQueried", true, true, false, true},
		{"userSetOnQueried", true, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "Widget__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
			}, Records: map[storage.ID]storage.Record{}}
			machine := New(nil)
			machine.SetOrg(&org)
			row := Object("Widget__c")
			row.Fields["Name"] = String("original")
			row.Fields["DisplayName"] = String("label")
			if tc.queried {
				row.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Widget__c", map[string]bool{"name": true, "displayname": true})
			}
			if tc.explicit {
				markExplicitSObjectField(&row, "DisplayName")
			}
			if tc.userSet {
				markUserSetSObjectField(&row, "displayname")
			}
			record, err := machine.recordFromValue(&row)
			if err != nil {
				t.Fatal(err)
			}
			_, stored := record.Fields["DisplayName"]
			if stored != tc.wantStored {
				t.Fatalf("DML fields = %#v; retain authored field = %v", record.Fields, tc.wantStored)
			}
			if row.Fields["DisplayName"].Text != "label" {
				t.Fatal("DML conversion removed readable query alias")
			}
			engine := machine.newDMLEngine(&Result{})
			results := engine.Insert([]storage.Record{record})
			if results[0].Success == tc.wantStored {
				t.Fatalf("DML result = %#v; unknown authored field must be rejected", results[0])
			}
		})
	}
}
