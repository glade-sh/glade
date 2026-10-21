package vm

import (
	"errors"
	"testing"
)

// T199/T200 in test_runner.json capture the exact generic-list assertion
// outcomes. The reflected Account list is an identity-preservation regression;
// it does not assert plain List<SObject> narrowing to other record types.
func TestExecAssertCollectionRuntimeIdentity(t *testing.T) {
	cases := []struct{ name, source, message string }{
		{name: "positive identity", source: `
Assert.isInstanceOfType(new List<Integer>(), List<Integer>.class, 'type check');
Type concrete=Type.forName('List<Account>');
List<SObject> records=(List<SObject>)concrete.newInstance();
Assert.isInstanceOfType(records,List<Account>.class);
Assert.isNotInstanceOfType(records,List<Opportunity>.class);
records.add(new Account(Name='First'));
Assert.isInstanceOfType(records,List<Account>.class);
Assert.isNotInstanceOfType(records,List<Opportunity>.class);
Assert.isInstanceOfType(records,List<SObject>.class);
`},
		{name: "captured negative assertion", source: `Assert.isNotInstanceOfType(new List<Integer>(),List<Integer>.class,'type check');`, message: "Assertion Failed: The provided object is an instance of class List<Integer>: type check"},
		{name: "runtime negative assertion", source: `Type concrete=Type.forName('List<Account>'); List<SObject> records=(List<SObject>)concrete.newInstance();records.add(new Account(Name='First'));Assert.isNotInstanceOfType(records,List<Account>.class,'runtime list');`, message: "Assertion Failed: The provided object is an instance of class List<Account>: runtime list"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			_, err = machine.Execute(program)
			if tc.message == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" || runtimeErr.Message != tc.message {
				t.Fatalf("expected AssertException %q, got %v", tc.message, err)
			}
		})
	}
}
