package server

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestCompositeSObjectUpsertCreationContracts(t *testing.T) {
	// DML transactions row R133 captures Apex isCreated=true for a failed
	// insert. The existing REST contract is preserved:
	// TestCompositeSObjectTypedUpsertPartialFailureCommitsSuccessfulRows
	// requires created=false for the same failed row.
	org := testOrg()
	engine := dml.NewEngine(&org)
	seed := engine.UpsertWithExternalID([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Existing"),
			"External_Id__c": storage.StringValue("existing"),
		},
	}}, "External_Id__c")
	if len(seed) != 1 || !seed[0].Success {
		t.Fatalf("seed upsert = %#v", seed)
	}
	results := engine.UpsertWithExternalID([]storage.Record{
		{Object: "Account", Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Created"),
			"External_Id__c": storage.StringValue("new"),
		}},
		{Object: "Account", Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Updated"),
			"External_Id__c": storage.StringValue("existing"),
		}},
		{Object: "Account", Fields: map[string]storage.Value{
			"External_Id__c": storage.StringValue("missing-name"),
		}},
		{Object: "Account", Fields: map[string]storage.Value{
			"External_Id__c": storage.StringValue("existing"),
		}, ExplicitNulls: map[string]bool{"Name": true}},
	}, "External_Id__c")
	wants := []struct {
		name             string
		success, created bool
		upsertCreated    bool
	}{
		{"insert", true, true, true},
		{"update", true, false, false},
		{"failed insert", false, false, true},
		{"failed update", false, false, false},
	}
	if len(results) != len(wants) {
		t.Fatalf("upsert results = %#v", results)
	}
	rows := compositeUpsertResults(results, []string{"insert", "update", "failed insert", "failed update"})
	for i, want := range wants {
		t.Run(want.name, func(t *testing.T) {
			result := results[i]
			if result.Success != want.success || result.Created != want.created || result.UpsertCreated != want.upsertCreated {
				t.Fatalf("DML creation result = %#v, want %#v", result, want)
			}
			row := rows[i]
			if row["success"] != want.success || row["created"] != want.created || row["referenceId"] != want.name || row["id"] != result.ID {
				t.Fatalf("REST upsert result = %#v, want %#v", row, want)
			}
			if !want.success {
				if result.StatusCode != "REQUIRED_FIELD_MISSING" || len(result.Fields) != 1 || result.Fields[0] != "Name" {
					t.Fatalf("DML failure = %#v", result)
				}
				errors := row["errors"].([]map[string]any)
				if len(errors) != 1 || errors[0]["statusCode"] != "REQUIRED_FIELD_MISSING" {
					t.Fatalf("REST upsert errors = %#v", errors)
				}
			}
		})
	}
	if len(org.Objects["Account"].Records) != 2 || org.Objects["Account"].Records[seed[0].ID].Fields["Name"].String != "Updated" {
		t.Fatalf("stored partial upsert records = %#v", org.Objects["Account"].Records)
	}
}
