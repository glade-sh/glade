package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Same-value and changed Name in before-update trigger are ignored: case349_trigger_capture/org.tsv:3-6,9-12,24-27,30-33.
func TestCase349CapturedBeforeTriggerAssignments(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, op := range []string{"update", "upsert"} {
			for _, tc := range []struct {
				name, trigger string
				wantSuccess   bool
			}{
				{"same-name", "Trigger.new[0].Name=Trigger.new[0].Name;", true}, {"changed-name", "Trigger.new[0].Name='Changed Synthetic';", true}, {"same-type", "Trigger.new[0].RecordTypeId=Trigger.new[0].RecordTypeId;", true}, {"input-name", "Integer observed=1;", false},
			} {
				t.Run(api+"/"+op+"/"+tc.name, func(t *testing.T) {
					org := testDataOrg()
					org.APIVersion = api
					storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
					storage.EnsureStandardObject(&org, "Contact")
					machine := New(nil)
					machine.SetOrg(&org)
					trigger, err := CompileAnonymousWithOptions(tc.trigger, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err = machine.RegisterTrigger(Trigger{Name: "SyntheticObserver", Object: "Account", Timing: "before", Operation: "update", APIVersion: api, Program: trigger}); err != nil {
						t.Fatal(err)
					}
					assign := ""
					if tc.name == "input-name" {
						assign = "a.Name='Synthetic';"
					}
					resultType := "Database.SaveResult"
					if op == "upsert" {
						resultType = "Database.UpsertResult"
					}
					source := fmt.Sprintf("Account seed=new Account(LastName='Synthetic'); insert seed; Account a=new Account(Id=seed.Id,PersonHomePhone='4155550123'); %s %s r=Database.%s(a,false); System.assertEquals(%t,r.isSuccess()); Account actual=[SELECT Name FROM Account WHERE Id=:seed.Id]; System.assertEquals('Synthetic',actual.Name);", assign, resultType, op, tc.wantSuccess)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err = machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// Extra fields, nulls and multiple selectors reject: case349_trigger_capture/org.tsv:18-21,39-42.
func TestCase349CapturedExternalBusinessShapes(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, shape := range []string{"plain", "extra", "null", "multiple"} {
		t.Run(shape, func(t *testing.T) {
			fields := map[string]storage.Value{"ProbeKey__c": storage.StringValue("key-" + shape), "RecordTypeId": storage.IDValue("012000000000001"), "Name": storage.StringValue("Synthetic Business"), "FirstName": storage.StringValue("Synthetic")}
			if shape == "extra" {
				fields["BillingCity"] = storage.StringValue("Synthetic")
			}
			if shape == "multiple" {
				fields["LastName"] = storage.StringValue("Person")
				fields["PersonTitle"] = storage.StringValue("Synthetic")
			}
			record := storage.Record{Object: "Account", Fields: fields}
			if shape == "null" {
				record.ExplicitNulls = map[string]bool{"BillingCity": true}
			}
			got := machine.preflightBusinessAccountNameFailures([]storage.Record{record}, "upsert", "ProbeKey__c")
			if len(got) != 1 || got[0].Success || got[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" || !got[0].UpsertCreated {
				t.Fatalf("got %#v", got)
			}
		})
	}
}

// Error details for explicit types: case349_trigger_capture/details/org.tsv:2-4,6-8.
func TestCase349CapturedExternalRecordTypeErrors(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true}, {ID: "012000000000002", DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct{ name, id, status, field, message string }{
		{"business", "012000000000001", "INVALID_FIELD_FOR_INSERT_UPDATE", "FirstName", "Account: bad field names on insert/update call: FirstName"},
		{"person", "012000000000002", "INVALID_FIELD_FOR_INSERT_UPDATE", "Name", "Unable to create/update fields: Name. Please check the security settings of this field and verify that it is read/write for your profile or permission set."},
		{"invalid", "012000000000099", "INVALID_CROSS_REFERENCE_KEY", "RecordTypeId", "Record Type ID: this ID value isn't valid for the user: 012000000000099"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("key-" + tc.name), "RecordTypeId": storage.IDValue(storage.ID(tc.id)), "Name": storage.StringValue("Synthetic Business"), "FirstName": storage.StringValue("Synthetic")}}
			got := machine.preflightBusinessAccountNameFailures([]storage.Record{rec}, "upsert", "ProbeKey__c")
			if len(got) != 1 || got[0].Success || !got[0].UpsertCreated || got[0].StatusCode != tc.status || got[0].Error != tc.message || len(got[0].Fields) != 1 || got[0].Fields[0] != tc.field {
				t.Fatalf("unexpected error: %#v", got)
			}
		})
	}
}

// Uncaptured person selector combinations retain the pre-capture path.
func TestCase349UncapturedPersonExtraShapesRemainOnPriorPath(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000002", DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, shape := range []string{"city", "last", "title", "null"} {
		t.Run(shape, func(t *testing.T) {
			fields := map[string]storage.Value{"ProbeKey__c": storage.StringValue("synthetic"), "RecordTypeId": storage.IDValue("012000000000002"), "Name": storage.StringValue("Synthetic"), "FirstName": storage.StringValue("Synthetic")}
			record := storage.Record{Object: "Account", Fields: fields}
			switch shape {
			case "city":
				fields["BillingCity"] = storage.StringValue("Synthetic")
			case "last":
				fields["LastName"] = storage.StringValue("Synthetic")
			case "title":
				fields["PersonTitle"] = storage.StringValue("Synthetic")
			case "null":
				record.ExplicitNulls = map[string]bool{"BillingCity": true}
			}
			if status, observed := machine.accountExternalCapturedNameShape(record, "ProbeKey__c"); observed {
				t.Fatalf("uncaptured shape entered preflight: %s", status)
			}
		})
	}
}

// Uncaptured combinations retain the pre-capture selector path.
func TestCase349UncapturedBusinessExtraShapesRemainOnPriorPath(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, shape := range []string{"city-last", "city-title", "city-null-last"} {
		t.Run(shape, func(t *testing.T) {
			fields := map[string]storage.Value{"ProbeKey__c": storage.StringValue("synthetic"), "RecordTypeId": storage.IDValue("012000000000001"), "Name": storage.StringValue("Synthetic"), "FirstName": storage.StringValue("Synthetic")}
			record := storage.Record{Object: "Account", Fields: fields}
			if shape == "city-null-last" {
				record.ExplicitNulls = map[string]bool{"BillingCity": true}
			} else {
				fields["BillingCity"] = storage.StringValue("Synthetic")
			}
			if shape == "city-title" {
				fields["PersonTitle"] = storage.StringValue("Synthetic")
			} else {
				fields["LastName"] = storage.StringValue("Synthetic")
			}
			if status, observed := machine.accountExternalCapturedNameShape(record, "ProbeKey__c"); observed {
				t.Fatalf("uncaptured shape entered preflight: %s", status)
			}
		})
	}
}

// Invalid types with extra selectors retain their captured error and boundary:
// case349_review2/org.tsv:6-9,15-18 (API 62/67).
func TestCase349CapturedInvalidTypeWithExtraSelectors(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, shape := range []string{"plain", "city", "null", "multiple"} {
		t.Run(shape, func(t *testing.T) {
			fields := map[string]storage.Value{"ProbeKey__c": storage.StringValue("key-" + shape), "RecordTypeId": storage.IDValue("012000000000099"), "Name": storage.StringValue("Synthetic"), "FirstName": storage.StringValue("Synthetic")}
			rec := storage.Record{Object: "Account", Fields: fields}
			switch shape {
			case "city":
				fields["BillingCity"] = storage.StringValue("Synthetic")
			case "null":
				rec.ExplicitNulls = map[string]bool{"BillingCity": true}
			case "multiple":
				fields["LastName"] = storage.StringValue("Person")
				fields["PersonTitle"] = storage.StringValue("Synthetic")
			}
			got := machine.preflightBusinessAccountNameFailures([]storage.Record{rec}, "upsert", "ProbeKey__c")
			if len(got) != 1 || got[0].Success || !got[0].UpsertCreated || got[0].StatusCode != "INVALID_CROSS_REFERENCE_KEY" || len(got[0].Fields) != 1 || got[0].Fields[0] != "RecordTypeId" || got[0].Error != "Record Type ID: this ID value isn't valid for the user: 012000000000099" {
				t.Fatalf("unexpected invalid type result: %#v", got)
			}
		})
	}
}
