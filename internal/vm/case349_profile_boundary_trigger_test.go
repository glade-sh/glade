package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Connected before-trigger observations: case349_profile_boundary/org.tsv:26-31,62-67.
func TestCase349ProfileBoundaryExternalTrigger(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, profile := range []string{"business", "person"} {
			for _, tc := range []struct{ name, assignment, status string }{
				{"omitted", "", "INVALID_FIELD_FOR_INSERT_UPDATE"},
				{"person", "a.RecordTypeId='012000000000002';", "INVALID_FIELD_FOR_INSERT_UPDATE"},
				{"business", "a.RecordTypeId='012000000000001';", "INVALID_FIELD_FOR_INSERT_UPDATE"},
				{"invalid", "a.RecordTypeId='012000000000099';", "INVALID_CROSS_REFERENCE_KEY"},
				{"last", "a.LastName='Person';", "INVALID_FIELD_FOR_INSERT_UPDATE"},
				{"title", "a.PersonTitle='Synthetic';", "INVALID_FIELD_FOR_INSERT_UPDATE"},
			} {
				t.Run(api+"/"+profile+"/"+tc.name, func(t *testing.T) {
					org := testDataOrg()
					org.APIVersion = api
					storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
					storage.EnsureDeterministicPlatformData(&org)
					account := org.Objects["Account"]
					account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
					account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true, Default: true}, {ID: "012000000000002", DeveloperName: "Individual", Active: true, Available: true, Default: true, PersonAccount: true}}
					org.Objects["Account"] = account
					// The native twin's key is visible to the administrator; an
					// ungranted custom key is not (standard_ext_upsert rows).
					storage.EnsureStandardObject(&org, "FieldPermissions")
					org.Objects["FieldPermissions"].Records["0FP000000000349"] = storage.Record{ID: "0FP000000000349", Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(storage.DefaultProfileID), "SObjectType": storage.StringValue("Account"), "Field": storage.StringValue("Account.ProbeKey__c"), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
					if profile == "person" {
						org.Metadata.AccountRecordTypeDefaults = []storage.AccountRecordTypeDefault{{ProfileID: storage.DefaultProfileID, DeveloperName: "Individual", PersonAccount: true}}
					}
					machine := New(nil)
					machine.SetOrg(&org)
					if err := machine.RegisterClass(Class{Name: "BoundaryCounter", StaticFields: map[string]Field{"hits": {Name: "hits", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)}}}); err != nil {
						t.Fatal(err)
					}
					trigger, err := CompileAnonymousWithOptions("BoundaryCounter.hits++;", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err = machine.RegisterTrigger(Trigger{Name: "BoundaryObserver", Object: "Account", Timing: "before", Operation: "insert", APIVersion: api, Program: trigger}); err != nil {
						t.Fatal(err)
					}
					source := fmt.Sprintf("Account a=new Account(ProbeKey__c='new',Name='Synthetic',FirstName='Synthetic'); %s Database.UpsertResult r=Database.upsert(a,Account.Fields.ProbeKey__c,false); System.assertEquals(false,r.isSuccess()); System.assertEquals('%s',String.valueOf(r.getErrors()[0].getStatusCode())); System.assertEquals(true,r.isCreated()); System.assertEquals(0,BoundaryCounter.hits);", tc.assignment, tc.status)
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
