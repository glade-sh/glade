package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native component boundaries: testdata/conformance/case349_component_query/org.tsv.
func TestCase349QueriedNameComponentNativeBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, op := range []string{"update", "upsert"} {
			for _, field := range []string{"FirstName", "LastName"} {
				for _, projection := range []struct {
					name, fields string
					reject       bool
				}{{"name", "Id,Name", true}, {"id", "Id", false}} {
					t.Run(api+"/"+op+"/"+field+"/"+projection.name, func(t *testing.T) {
						org := testDataOrg()
						org.APIVersion = api
						storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
						storage.EnsureStandardObject(&org, "Contact")
						machine := New(nil)
						machine.SetOrg(&org)
						resultType := "Database.SaveResult"
						if op == "upsert" {
							resultType = "Database.UpsertResult"
						}
						source := fmt.Sprintf("Account seed=new Account(FirstName='Initial',LastName='Synthetic'); insert seed; Account a=[SELECT %s FROM Account WHERE Id=:seed.Id]; a.%s='Changed'; %s r=Database.%s(a,false); System.assertEquals(%t,r.isSuccess());", projection.fields, field, resultType, op, !projection.reject)
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
