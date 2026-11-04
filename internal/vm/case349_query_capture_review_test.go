package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native projection boundaries: case349_query_trigger/org.tsv:12-43,55-86.
func TestCase349QueriedPersonNameNativeBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, op := range []string{"update", "upsert"} {
			for _, projection := range []struct {
				name, fields string
				reject       bool
			}{{"name", "Id,Name", true}, {"both", "Id,Name,RecordTypeId", true}, {"type", "Id,RecordTypeId", false}, {"id", "Id", false}} {
				for _, field := range []string{"PersonHomePhone", "PersonMobilePhone", "PersonEmail", "none"} {
					t.Run(api+"/"+op+"/"+projection.name+"/"+field, func(t *testing.T) {
						org := testDataOrg()
						org.APIVersion = api
						storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
						storage.EnsureStandardObject(&org, "Contact")
						machine := New(nil)
						machine.SetOrg(&org)
						assign := ""
						if field != "none" {
							value := "4155550123"
							if field == "PersonEmail" {
								value = "synthetic@example.invalid"
							}
							assign = fmt.Sprintf("a.%s='%s';", field, value)
						}
						resultType := "Database.SaveResult"
						if op == "upsert" {
							resultType = "Database.UpsertResult"
						}
						source := fmt.Sprintf("Account seed=new Account(LastName='Synthetic'); insert seed; Account a=[SELECT %s FROM Account WHERE Id=:seed.Id]; %s %s r=Database.%s(a,false); System.assertEquals(%t,r.isSuccess());", projection.fields, assign, resultType, op, !projection.reject)
						if projection.reject {
							source += " System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(r.getErrors()[0].getStatusCode())); System.assertEquals('Name',r.getErrors()[0].getFields()[0]); System.assertEquals('Unable to create/update fields: Name. Please check the security settings of this field and verify that it is read/write for your profile or permission set.',r.getErrors()[0].getMessage());"
						}
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
}
