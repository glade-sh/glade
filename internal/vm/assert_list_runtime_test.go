package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Assert checks an object's instance type. These controls exercise a concrete
// record list's retained runtime type through the generic SObject declared view.
func TestExecAssertRecordListRuntimeType(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
List<SObject> direct = new List<Lead>();
Assert.isInstanceOfType(direct, List<Lead>.class);
Assert.isNotInstanceOfType(direct, List<Case>.class);
Assert.isInstanceOfType(direct, List<SObject>.class);
Type chosen = List<Lead>.class;
List<SObject> reflected = (List<SObject>)chosen.newInstance();
reflected.add(new Lead(LastName='Synthetic', Company='Synthetic'));
Assert.isInstanceOfType(reflected, List<Lead>.class);
Assert.isNotInstanceOfType(reflected, List<Case>.class);
List<SObject> generic = new List<SObject>();
Assert.isInstanceOfType(generic, List<SObject>.class);
Assert.isInstanceOfType(generic, List<Lead>.class);
Assert.isInstanceOfType(generic, List<Case>.class);
System.assert(generic instanceof List<Lead>);
System.assert(generic instanceof List<Case>);
generic.addAll(new List<Lead>{new Lead(LastName='Synthetic', Company='Synthetic')});
Assert.isInstanceOfType(generic, List<Lead>.class);
Assert.isInstanceOfType(generic, List<Case>.class);
System.assert(generic instanceof List<Lead>);
System.assert(generic instanceof List<Case>);
List<SObject> absent;
Assert.isNotInstanceOfType(absent, List<Lead>.class);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Lead")
			storage.EnsureStandardObject(&org, "Case")
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
