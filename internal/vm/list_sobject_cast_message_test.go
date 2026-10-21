package vm

import (
	"fmt"
	"testing"
)

// Typed lists retain their declared generic argument even when empty; an
// Object list reports ANY without inferring a type from its elements.
func TestListSObjectCastMessageElementName(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			for _, tc := range []struct {
				name, subject, element string
			}{
				{"inline-empty-string", "Account ignored = (Account)(Object)new List<String>();", "String"},
				{"held-empty-string", "Object held = new List<String>{}; Account ignored = (Account)held;", "String"},
				{"held-string", "Object held = new List<String>{'a'}; Account ignored = (Account)held;", "String"},
				{"held-integer", "Object held = new List<Integer>{1}; Account ignored = (Account)held;", "Integer"},
				{"inline-object-integer", "Account ignored = (Account)(Object)new List<Object>{1};", "ANY"},
				{"held-object-integer", "Object held = new List<Object>{1}; Account ignored = (Account)held;", "ANY"},
				{"held-empty-account", "Object held = new List<Account>{}; Account ignored = (Account)held;", "Account"},
				{"held-sobject", "Object held = new List<SObject>{new Account(Name='a')}; Account ignored = (Account)held;", "SObject"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					source := fmt.Sprintf(`
String message;
try {
    %s
    System.assert(false, 'Expected TypeException');
} catch (TypeException e) {
    System.assertEquals('System.TypeException', e.getTypeName());
    message = e.getMessage();
}
System.assertEquals('Invalid conversion from runtime type List<%s> to Account', message);
String element = message.substringBetween('List<', '>');
System.assertEquals('%s', element);
`, tc.subject, tc.element, tc.element)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := New(nil).Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
