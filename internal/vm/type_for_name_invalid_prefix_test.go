package vm

import "testing"

func TestTypeForNameRejectsTrailingSeparatorForLocalNestedClass(t *testing.T) {
	for _, name := range []string{"ReflectionOwner.PrivateItem", "ReflectionOwner.PublicItem"} {
		t.Run(name, func(t *testing.T) {
			machine := privateNestedLookupMachine(t, "return Type.forName('ReflectionOwner.', name);")
			program, err := CompileAnonymous("System.assertEquals(null, ExternalTypeLookup.resolve('" + name + "'));")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
