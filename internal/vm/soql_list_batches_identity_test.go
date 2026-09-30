package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSOQLListBatchIdentityKeepsDistinctAndNestedTypes(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "OwnedBatch"
	for _, name := range []string{"OwnedBatch__Row__c", "Other__Row__c"} {
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name}}
	}
	machine.SetOrg(&org)
	for _, test := range []struct {
		name, loop, iterable string
		want                 bool
	}{
		{"local alias", "List<Row__c>", "List<OwnedBatch__Row__c>", true},
		{"other namespace", "List<Row__c>", "List<Other__Row__c>", false},
		{"unknown local", "List<Missing__c>", "List<OwnedBatch__Row__c>", false},
		{"nested list", "List<Row__c>", "List<List<Row__c>>", false},
		{"scalar loop", "Row__c", "List<OwnedBatch__Row__c>", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := machine.shouldChunkEnhancedForList(test.loop, typedList(test.iterable)); got != test.want {
				t.Fatalf("chunk predicate = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSOQLListBatchKeepsOrdinaryEmptyLoops(t *testing.T) {
	program, err := CompileAnonymous(`
Integer calls=0;
for(Account row : new List<Account>()) calls++;
for(List<Account> rows : new List<List<Account>>()) calls++;
for(Account row : [SELECT Id FROM Account WHERE Name='owned-no-rows']) calls++;
System.assertEquals(0,calls,'ordinary empty loops stay empty');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
