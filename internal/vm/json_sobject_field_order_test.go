package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// SF174 accepted exact duplicate and case-alias orders for both SObject targets.
func TestJSONSObjectFieldTextualOrderAtAPI44(t *testing.T) {
	for _, target := range []string{"SObject", "Account"} {
		for _, tc := range []struct{ name, fields, expected string }{
			{"duplicate_null_value", `"GladeMemberType44__c":null,"GladeMemberType44__c":"MOCK"`, "'MOCK'"},
			{"duplicate_value_null", `"GladeMemberType44__c":"MOCK","GladeMemberType44__c":null`, "null"},
			{"case_null_value", `"GladeMemberType44__c":null,"glademembertype44__c":"MOCK"`, "'MOCK'"},
			{"case_value_null", `"glademembertype44__c":"MOCK","GladeMemberType44__c":null`, "null"},
		} {
			t.Run(target+"/"+tc.name, func(t *testing.T) {
				source := fmt.Sprintf(`Account row = (Account)JSON.deserialize('{"attributes":{"type":"Account"},%s}',%s.class);
System.assertEquals(%s,row.GladeMemberType44__c);
System.assertEquals(%s,row.get(Account.GladeMemberType44__c));`, tc.fields, target, tc.expected, tc.expected)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "44.0"})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				account := org.Objects["Account"]
				account.Definition.Fields["GladeMemberType44__c"] = storage.Field{APIName: "GladeMemberType44__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `"BASE"`}
				org.Objects["Account"] = account
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
