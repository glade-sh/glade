package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dbmanager"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

// Explicit same-name writes reject in the native control:
// case349_mixed/org.tsv:6,13. Exercise the DML callers used by other hosts.
func TestCase349PersonNameExplicitHostRoutes(t *testing.T) {
	for _, route := range []string{"soap", "vf", "lwc", "dbmanager"} {
		t.Run(route, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureStandardObject(&org, "Contact")
			account := org.Objects["Account"]
			account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
			name := account.Definition.Fields["Name"]
			name.Updateable = storage.BoolFlag(true)
			account.Definition.Fields["Name"] = name
			phone := account.Definition.Fields["PersonHomePhone"]
			phone.Updateable = storage.BoolFlag(true)
			account.Definition.Fields["PersonHomePhone"] = phone
			org.Objects["Account"] = account
			engine := dml.NewEngine(&org)
			created := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"LastName": storage.StringValue("Synthetic"), "ProbeKey__c": storage.StringValue("person")}}})
			if len(created) != 1 || !created[0].Success {
				t.Fatal(created)
			}
			id := string(created[0].ID)
			switch route {
			case "soap":
				body := partnerSOAPEnvelope(`<urn:upsert><urn:externalIDFieldName>ProbeKey__c</urn:externalIDFieldName><urn:sObjects xsi:type="sf:Account"><sf:ProbeKey__c>person</sf:ProbeKey__c><sf:Name>Synthetic</sf:Name><sf:PersonHomePhone>4155550123</sf:PersonHomePhone></urn:sObjects></urn:upsert>`)
				rec := partnerSOAPPost(New(&org), body)
				if !strings.Contains(rec.Body.String(), "INVALID_FIELD_FOR_INSERT_UPDATE") {
					t.Fatalf("status=%d %s", rec.Code, rec.Body.String())
				}
			case "vf":
				desc := visualforce.RemoteObjectsDescriptor{Models: []visualforce.RemoteObjectModelDescriptor{{Name: "Account", JSName: "Account", Fields: []visualforce.RemoteObjectFieldDescriptor{{Name: "Name", JSName: "Name"}, {Name: "PersonHomePhone", JSName: "PersonHomePhone"}}}}}
				result := visualforce.DispatchRemoteObjectCRUD(&org, desc, visualforce.RemoteObjectCRUDRequest{Operation: "update", ObjectName: "Account", IDs: []string{id}, Fields: map[string]any{"Name": "Synthetic", "PersonHomePhone": "4155550123"}})
				if result.Success || len(result.Errors) == 0 || result.Errors[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
					t.Fatalf("%+v", result)
				}
			case "lwc":
				_, err := updateRecordWireData(&org, lwcbrowser.WireUpdateRecordRequest{Fields: map[string]any{"Id": id, "Name": "Synthetic", "PersonHomePhone": "4155550123"}})
				if err == nil || !strings.Contains(fmt.Sprint(err), "INVALID_FIELD_FOR_INSERT_UPDATE") {
					t.Fatalf("%+v", err)
				}
			case "dbmanager":
				manager := dbmanager.New(&org)
				result := manager.UpdateRecord("Account", id, dbmanager.MutationPayload{Fields: map[string]dbmanager.FieldInput{"Name": {State: "value", Value: "Synthetic"}, "PersonHomePhone": {State: "value", Value: "4155550123"}}})
				if result.Success || result.StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
					t.Fatalf("%+v", result)
				}
			}
		})
	}
}
