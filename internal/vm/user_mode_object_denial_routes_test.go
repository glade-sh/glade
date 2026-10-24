package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Native test_entry_sharing omitted_none/without_none/with_none rows at
// API62/67 pin the same object denial for default and explicit user queries.
func TestUserModeObjectDenialQueryAndVisualforceRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		routes := map[string]string{
			"inline":             "[SELECT Id FROM Widget__c WITH USER_MODE]",
			"dynamic_query":      "Database.query('SELECT Id FROM Widget__c WITH USER_MODE')",
			"dynamic_count":      "Database.countQuery('SELECT COUNT() FROM Widget__c WITH USER_MODE')",
			"query_locator":      "Database.getQueryLocator('SELECT Id FROM Widget__c WITH USER_MODE')",
			"query_with_binds":   "Database.queryWithBinds('SELECT Id FROM Widget__c', new Map<String,Object>(), AccessLevel.USER_MODE)",
			"count_with_binds":   "Database.countQueryWithBinds('SELECT COUNT() FROM Widget__c', new Map<String,Object>(), AccessLevel.USER_MODE)",
			"locator_with_binds": "Database.getQueryLocatorWithBinds('SELECT Id FROM Widget__c', new Map<String,Object>(), AccessLevel.USER_MODE)",
		}
		if api == "67.0" {
			routes["default_inline"] = "[SELECT Id FROM Widget__c]"
			routes["query_cursor"] = "Database.getCursor('SELECT Id FROM Widget__c WITH USER_MODE')"
		}
		for route, expression := range routes {
			for _, context := range []string{"anonymous", "test_runas"} {
				t.Run(api+"/"+context+"/"+route, func(t *testing.T) {
					machine := objectDenialRouteVM(t, api)
					if context == "test_runas" {
						machine.EnableTestContext()
						machine.testContext.RunAsDepth = 1
					}
					program, err := CompileAnonymousWithOptions(fmt.Sprintf("Object rows = %s;", expression), CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					assertObjectDenialRoute(t, err)
				})
			}
		}
		for _, route := range []string{"authorize_fields", "read_fields", "read_record", "read_records", "controller_projection"} {
			t.Run(api+"/visualforce/"+route, func(t *testing.T) {
				machine := objectDenialRouteVM(t, api)
				var err error
				switch route {
				case "authorize_fields":
					_, err = machine.AuthorizeVisualforceRecordFields("Widget__c", []string{"Name"})
				case "read_fields":
					_, _, err = machine.ReadVisualforceRecordFields("Widget__c", "a00000000000001", []string{"Name"})
				case "read_record":
					_, _, err = machine.ReadVisualforceRecord("Widget__c", "a00000000000001")
				case "read_records":
					_, err = machine.ReadVisualforceRecords("Widget__c")
				case "controller_projection":
					record := Object("Widget__c")
					record.Fields["Id"] = String("a00000000000001")
					_, err = machine.standardControllerFieldProjection(record, List(String("Name")))
				}
				assertObjectDenialRoute(t, err)
			})
		}
	}
}

func objectDenialRouteVM(t *testing.T, api string) *VM {
	t.Helper()
	org := privateExecutionPolicyOrg()
	storage.EnsureDeterministicPlatformData(&org)
	var profileID storage.ID
	for id, profile := range org.Objects["Profile"].Records {
		if name, _ := profile.GetField("Name"); name.String == "Minimum Access - Salesforce" {
			profileID = id
		}
	}
	if profileID == "" {
		t.Fatal("minimum access profile missing")
	}
	actor := storage.Record{ID: "005000000000099", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(profileID)}}
	users := org.Objects["User"]
	users.Records[actor.ID] = actor
	org.Objects["User"] = users
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(actor)
	machine.currentMethod = Method{APIVersion: api}
	return machine
}

func assertObjectDenialRoute(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("object denial route returned data")
	}
	var message string
	switch issue := err.(type) {
	case *apexThrowError:
		if issue.value.Type != "QueryException" {
			t.Fatalf("denial changed exception type: %s", issue.value.Type)
		}
		message = exceptionMessage(issue.value)
	case *RuntimeError:
		if issue.Type != "QueryException" && issue.Type != "System.QueryException" {
			t.Fatalf("denial changed exception type: %s", issue.Type)
		}
		message = issue.ExceptionMessage()
	default:
		t.Fatalf("unexpected error route: %T: %v", err, err)
	}
	if want := soql.MissingObjectMessage("Widget__c"); message != want {
		t.Fatalf("native=%s; Glade=%s", want, message)
	}
}
