package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestGenericMapLiteralKeysDoNotUseCallerNamespace(t *testing.T) {
	for _, namespace := range []string{"", "PKG"} {
		t.Run(namespace, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Map<String,Object> fields = new Map<String,Object>{'GladeMemberType44__c'=>'BASE'};
System.assertEquals(false,fields.containsKey('PKG__GladeMemberType44__c'));
System.assertEquals(null,fields.get('PKG__GladeMemberType44__c'));
fields.put('PKG__GladeMemberType44__c','MOCK');
System.assertEquals(2,fields.size());
System.assertEquals('BASE',fields.get('GladeMemberType44__c'));
System.assertEquals('MOCK',fields.remove('PKG__GladeMemberType44__c'));
System.assertEquals('BASE',fields.get('GladeMemberType44__c'));
System.assertEquals(false,fields.containsKey('PKG__GladeMemberType44__c'));
`, CompileOptions{APIVersion: "44.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.Namespace = namespace
			machine := New(nil)
			machine.SetOrg(&org)
			if err := machine.RegisterClass(Class{Name: "Harness", Namespace: namespace}); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(program, "Harness"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
