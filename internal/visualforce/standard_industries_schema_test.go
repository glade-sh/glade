package visualforce

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Labels come from the Salesforce 62.0/67.0 captured field facets.
func TestVisualforceIndustriesSchemaFields(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = api
			org.Features = []string{"ContactsToMultipleAccounts", "B2BCommerce", "Communities", "CommerceEnabled", "MiddleNameEnabled"}
			for _, name := range []string{"AccountContactRelation", "PricebookEntry", "User"} {
				storage.EnsureStandardObject(&org, name)
			}
			machine := vm.New(nil)
			machine.SetOrg(&org)
			got, err := RenderExpressionTemplate(`{!$ObjectType.AccountContactRelation.Fields.Roles.Label}|{!$ObjectType.PricebookEntry.Fields.ProductSellingModelId.Label}|{!$ObjectType.User.Fields.MiddleName.Label}`, &ExpressionContext{VM: machine})
			if err != nil {
				t.Fatal(err)
			}
			if got != "Roles|Product Selling Model ID|Middle Name" {
				t.Fatalf("schema labels=%q", got)
			}
		})
	}
}
