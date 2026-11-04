package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// External-ID parent references select the numerically equal key (native
// decimal equality: case349_review/org.tsv; parent lookup rule documented by
// https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-upsert.html).
func TestCase349DecimalParentRelationshipDMLRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"stored", "pending"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				account := org.Objects["Account"]
				account.Definition.Fields["Decimal_Key__c"] = storage.Field{APIName: "Decimal_Key__c", Type: storage.FieldDecimal, ExternalID: true}
				org.Objects["Account"] = account
				storage.EnsureStandardObject(&org, "Contact")
				machine := New(nil)
				machine.SetOrg(&org)
				prefix := `Account wrong=new Account(Name='Wrong',Decimal_Key__c=1.25); Account right=new Account(Name='Right',Decimal_Key__c=2.500); Contact child=new Contact(LastName='Synthetic'); child.putSObject('Account',new Account(Decimal_Key__c=2.5)); `
				operation := `insert new List<SObject>{wrong,right,child};`
				if route == "stored" {
					operation = `insert new List<Account>{wrong,right}; upsert child;`
				}
				source := prefix + operation + ` Contact actual=[SELECT AccountId FROM Contact WHERE Id=:child.Id]; System.assertEquals(right.Id,actual.AccountId);`
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
