package vm

import (
	"errors"
	"testing"
)

// type_system.json R125/R126/R129 own the related instanceof boundaries.
// Exact Assert method twins LA01-LA09 remain pending native captures.
func TestSObjectListAssertionRetainsConcreteRuntimeType(t *testing.T) {
	machine := New(nil)
	body, err := CompileAnonymous("return (List<SObject>)stored;")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "RecordShelf.read", ClassName: "RecordShelf", IsStatic: true, ReturnType: "List<SObject>", Params: []Param{{Name: "stored", Type: "Object"}}, Program: body}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Account> concrete=new List<Account>{new Account(Name='owned')};
Map<String,Object> shelf=new Map<String,Object>{'rows'=>concrete};
List<SObject> view=RecordShelf.read(shelf.get('rows'));
System.assert(view instanceof List<Account>);
Assert.isInstanceOfType(view,List<Account>.class);
System.assertEquals(1,((List<Account>)view).size());
view=RecordShelf.read(new List<Account>());
Assert.isInstanceOfType(view,List<Account>.class);
Assert.isNotInstanceOfType(view,List<Contact>.class);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		view := machine.Globals["view"]
		t.Fatalf("%v; view type=%q runtime=%q static=%q target=%q", err, view.Type, view.Runtime, view.Static, typeValueIdentityName(machine.reflectionTypeToken("List<Account>")))
	}
}

func TestSObjectListAssertionKeepsOtherBoundaries(t *testing.T) {
	program, err := CompileAnonymous(`
Assert.isNotInstanceOfType(new List<SObject>(),List<Account>.class);
List<SObject> contacts=new List<Contact>{new Contact()};
Assert.isNotInstanceOfType(contacts,List<Account>.class);
Assert.isInstanceOfType(new Account(),Account.class);
Assert.isInstanceOfType(1,Long.class);
Assert.isNotInstanceOfType(null,List<Account>.class);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
	inverse, err := CompileAnonymous("List<SObject> view=new List<Account>(); Assert.isNotInstanceOfType(view,List<Account>.class);")
	if err != nil {
		t.Fatal(err)
	}
	_, execErr := New(nil).Execute(inverse)
	var failure *RuntimeError
	if !errors.As(execErr, &failure) || failure.Type != "System.AssertException" ||
		failure.ExceptionMessage() != "Assertion Failed: The provided object is an instance of class List<Account>" {
		t.Fatalf("concrete Account list inverse assertion: %v", execErr)
	}
}
