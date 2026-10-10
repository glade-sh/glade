package vm

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecDateQueryFamilyUsesDateFunctionsAndExecutionTimezone(t *testing.T) {
	for _, apiVersion := range []string{"62.0", "67.0"} {
		t.Run("API"+apiVersion, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
List<DateQueryFixture__c> matches = [SELECT Name FROM DateQueryFixture__c
    WHERE CALENDAR_YEAR(ObservedDate__c) = 2024
      AND CALENDAR_MONTH(ObservedDate__c) = 2
      AND ObservedAt__c = TODAY];
System.assertEquals(1, matches.size());
System.assertEquals('Local today', matches[0].Name);
`, CompileOptions{APIVersion: apiVersion})
			if err != nil {
				t.Fatal(err)
			}

			org := storage.NewOrgState()
			org.Objects["DateQueryFixture__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{
					APIName:   "DateQueryFixture__c",
					KeyPrefix: "a0D",
					Fields: map[string]storage.Field{
						"Name":            {APIName: "Name", Type: storage.FieldString},
						"ObservedDate__c": {APIName: "ObservedDate__c", Type: storage.FieldDate},
						"ObservedAt__c":   {APIName: "ObservedAt__c", Type: storage.FieldDateTime},
					},
				},
				Records: map[storage.ID]storage.Record{
					"a0D000000000001": {
						ID:     "a0D000000000001",
						Object: "DateQueryFixture__c",
						Fields: map[string]storage.Value{
							"Name":            storage.StringValue("Local today"),
							"ObservedDate__c": storage.DateValue("2024-02-29"),
							"ObservedAt__c":   storage.DateTimeValue("2026-05-01T20:00:00Z"),
						},
					},
					"a0D000000000002": {
						ID:     "a0D000000000002",
						Object: "DateQueryFixture__c",
						Fields: map[string]storage.Value{
							"Name":            storage.StringValue("Next local day"),
							"ObservedDate__c": storage.DateValue("2024-02-29"),
							"ObservedAt__c":   storage.DateTimeValue("2026-05-02T08:00:00Z"),
						},
					},
				},
			}

			machine := New(nil)
			machine.SetOrg(&org)
			machine.fakeNow = time.Date(2026, 5, 2, 1, 0, 0, 0, time.UTC)
			machine.SetCurrentUser(storage.Record{
				ID:     "005000000000001",
				Object: "User",
				Fields: map[string]storage.Value{
					"TimeZoneSidKey": storage.StringValue("America/Los_Angeles"),
				},
			})
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
