package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestCase349UnqueriedDecimalDefaultNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, input := range []string{"1.250", "1.251", "0.000"} {
			t.Run(api+"/"+input, func(t *testing.T) {
				source := `Account row=new Account(Name='Synthetic',Balance__c=` + input + `); insert row;
Account partial=[SELECT Id FROM Account WHERE Id=:row.Id]; Boolean rejected=false;
try { Decimal ignored=partial.Balance__c; } catch(SObjectException e) { rejected=true; }
System.assertEquals(true,rejected); Account selected=[SELECT Balance__c FROM Account WHERE Id=:row.Id];
System.assertEquals(` + input + `,selected.Balance__c);`
				p, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				org.APIVersion = api
				account := org.Objects["Account"]
				account.Definition.Fields["Balance__c"] = storage.Field{APIName: "Balance__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", DefaultValue: "1.25", Scale: 3}
				org.Objects["Account"] = account
				m := New(nil)
				m.SetOrg(&org)
				if _, err = m.Execute(p); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
