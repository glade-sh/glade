package vm

import (
	"fmt"
	"testing"
)

// Owned rows in equals_only_set.json establish the fully qualified
// equals-only identity, mutation, explicit-hash, null and List controls at
// API 62/67/68 in anonymous and test execution.
func TestEqualsOnlySetQualifiedClassIdentity(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			runSetClassAliasIdentity(t, api, "", "TokenHost.Token")
		})
	}
}

// PENDING-ROWS: the short/qualified/case-varied nested and namespace twins
// extend the captured identity contract through source aliases. These twins
// still need native observations at API 62/67/68 in both
// anonymous and @IsTest execution. No bulk/copy producer contract is asserted.
func TestEqualsOnlySetClassAliasesPendingRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, namespace := range []string{"", "lyx"} {
			types := []string{"Token", "tOkEn", "TokenHost.Token", "tOkEnHoSt.tOkEn"}
			if namespace != "" {
				types = append(types, "lyx.TokenHost.Token", "lYx.tOkEnHoSt.tOkEn")
			}
			for _, setElement := range types {
				t.Run(api+"/"+namespace+"/"+setElement, func(t *testing.T) {
					runSetClassAliasIdentity(t, api, namespace, setElement)
				})
			}
		}
	}
}

func runSetClassAliasIdentity(t *testing.T, api, namespace, setElement string) {
	t.Helper()
	equals, err := CompileAnonymousWithOptions("return true;", CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := CompileAnonymousWithOptions("return 7;", CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "TokenHost", Namespace: namespace}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Token", "HashedToken"} {
		className := "TokenHost." + name
		methods := map[string]Method{
			"equals": {Name: className + ".equals", ClassName: className, ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: equals},
		}
		if name == "HashedToken" {
			methods["hashCode"] = Method{Name: className + ".hashCode", ClassName: className, ReturnType: "Integer", Program: hash}
		}
		if err := machine.RegisterClass(Class{
			Name: className, Namespace: namespace,
			Fields:  map[string]Field{"visits": {Name: "visits", Type: "Integer"}},
			Methods: methods,
		}); err != nil {
			t.Fatal(err)
		}
	}
	qualified := "TokenHost.Token"
	hashed := "TokenHost.HashedToken"
	entry := "TokenHost"
	if namespace != "" {
		qualified = namespace + "." + qualified
		hashed = namespace + "." + hashed
		entry = namespace + "." + entry
	}
	source := fmt.Sprintf(`
%[1]s first = new %[1]s();
%[1]s second = new %[1]s();
first.visits = 0;
second.visits = 0;
System.assert(first.equals(second));
Set<%[2]s> values = new Set<%[2]s>{first, second};
System.assertEquals(2, values.size());
for (%[1]s token : values) { token.visits++; }
System.assertEquals(1, first.visits);
System.assertEquals(1, second.visits);
System.assert(values.contains(first));
System.assert(!values.contains(new %[1]s()));
Set<%[2]s> added = new Set<%[2]s>();
System.assert(added.add(first));
System.assert(added.add(second));
System.assertEquals(2, added.size());
System.assert(!added.remove(new %[1]s()));
System.assert(added.remove(first));
System.assertEquals(1, added.size());
System.assertEquals(1, new Set<%[2]s>{first, first}.size());
System.assertEquals(2, new Set<%[2]s>{null, null, first, first}.size());
System.assert(new List<%[1]s>{first}.contains(new %[1]s()));
System.assertEquals(1, new Set<%[3]s>{new %[3]s(), new %[3]s()}.size());
`, qualified, setElement, hashed)
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, entry); err != nil {
		t.Fatal(err)
	}
}
