package server

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestCompositeSObjectUpsertCreationContracts(t *testing.T) {
	// Native REST observations at APIs 62.0 and 67.0 report created=true
	// and omit the id when an attempted insert fails.
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
			if row["success"] != want.success || row["created"] != want.upsertCreated || row["referenceId"] != want.name || (result.ID != "" && row["id"] != result.ID) {
				t.Fatalf("REST upsert result = %#v, want %#v", row, want)
			}
			if !want.success && want.upsertCreated && result.ID == "" {
				if _, ok := row["id"]; ok {
					t.Fatalf("failed insert has id: %#v", row)
				}
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
