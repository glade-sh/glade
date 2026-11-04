package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native phone-only update succeeds: testdata/conformance/case349_mixed/org.tsv:7,14.
// Trigger hydration must not turn stored Name or RecordTypeId into user input.
func TestCase349HydratedPersonUpdates(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, personType := range []bool{true} {
			for _, field := range []string{"PersonHomePhone", "PersonMobilePhone", "PersonEmail"} {
				for _, route := range []string{"update", "upsert", "explicit-id-upsert", "explicit-same-type"} {
					t.Run(fmt.Sprintf("%s/person-type-%t/%s/%s", api, personType, field, route), func(t *testing.T) {
						org := testDataOrg()
						org.APIVersion = api
						storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
						storage.EnsureStandardObject(&org, "Contact")
						account := org.Objects["Account"]
						rt := storage.ID("012000000000001")
						id := storage.ID("001000000000101")
						account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: rt, DeveloperName: "Synthetic", Active: true, Available: true, PersonAccount: personType}}
						account.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{
							"Name": storage.StringValue("Synthetic Person"), "LastName": storage.StringValue("Person"),
							"RecordTypeId": storage.IDValue(rt), "IsPersonAccount": storage.BooleanValue(true),
						}}
						org.Objects["Account"] = account
						org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}}, Records: map[storage.ID]storage.Record{rt: {Object: "RecordType", ID: rt}}}
						value := "4155550123"
						if field == "PersonEmail" {
							value = "synthetic@example.invalid"
						}
						operation := "Database.update(a, false)"
						resultType := "Database.SaveResult"
						if route == "upsert" {
							operation = "Database.upsert(a, false)"
							resultType = "Database.UpsertResult"
						}
						if route == "explicit-id-upsert" {
							operation = "Database.upsert(a, Account.Fields.Id, false)"
							resultType = "Database.UpsertResult"
						}
						typeAssignment := ""
						if route == "explicit-same-type" {
							typeAssignment = fmt.Sprintf("a.RecordTypeId='%s';", rt)
						}
						source := fmt.Sprintf("Account a = new Account(Id='%s'); %s a.%s='%s'; %s r = %s; if(!r.isSuccess()) System.assert(false,String.valueOf(r.getErrors()[0].getStatusCode())+':'+r.getErrors()[0].getMessage());", id, typeAssignment, field, value, resultType, operation)
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := New(nil)
						machine.SetOrg(&org)
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
						stored := org.Objects["Account"].Records[id]
						got, ok := stored.GetField(field)
						if !ok || got.String != value {
							t.Fatalf("stored %s = %#v", field, got)
						}
						typ, _ := stored.GetField("RecordTypeId")
						gotRT := typ.ID
						if typ.Kind == storage.ValueString {
							gotRT = storage.ID(typ.String)
						}
						if !storage.IDsEqual(gotRT, rt) {
							t.Fatalf("record type changed: %#v", typ)
						}
					})
				}
			}
		}
	}
}

// Trigger changes and caller-provided fields must survive the hydration cleanup.
func TestCase349HydratedPersonFieldProvenance(t *testing.T) {
	for _, field := range []string{"Name", "RecordTypeId"} {
		for _, scenario := range []string{"hydrated", "caller", "trigger", "business"} {
			t.Run(field+"/"+scenario, func(t *testing.T) {
				old := storage.Record{Object: "Account", Fields: map[string]storage.Value{"IsPersonAccount": storage.BooleanValue(scenario != "business"), field: storage.StringValue("old")}}
				candidate := old.Clone()
				input := storage.Record{Object: "Account", Fields: map[string]storage.Value{}}
				if scenario == "caller" {
					input.Fields[field] = storage.StringValue("old")
				}
				if scenario == "trigger" {
					candidate.Fields[field] = storage.StringValue("new")
					candidate.MarkDMLTriggerAssignment(field)
				}
				records := []storage.Record{candidate}
				removeUnchangedHydratedPersonFields(records, []storage.Record{input}, []storage.Record{old})
				_, present := records[0].GetField(field)
				if present != (scenario != "hydrated" && !(field == "Name" && scenario == "trigger")) {
					t.Fatalf("presence=%t for %s", present, scenario)
				}
			})
		}
	}
}
