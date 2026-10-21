package vm

import (
	"fmt"
	"testing"
)

// Owned equals_only_set.json observations establish literal/add identity
// history at API 62/67/68 on both routes. Namespace alias
// spelling is a local regression candidate pending the corresponding captures.
func TestExecEqualsOnlySetUsesResolvedElementIdentity(t *testing.T) {
	for _, namespace := range []string{"", "triagelab"} {
		for _, elementType := range []string{"ResidualLens.Parcel", "Parcel"} {
			t.Run(namespace+"/"+elementType, func(t *testing.T) {
				machine := New(nil)
				equals, err := CompileAnonymous("return other instanceof ResidualLens.Parcel;")
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{
					Name: "ResidualLens.Parcel", Namespace: namespace,
					Fields: map[string]Field{"touched": {Name: "touched", Type: "Integer"}},
					Methods: map[string]Method{"equals": {
						Name: "ResidualLens.Parcel.equals", ClassName: "ResidualLens.Parcel",
						ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: equals,
					}},
				}); err != nil {
					t.Fatal(err)
				}
				machine.currentClass = "ResidualLens.Parcel"
				if namespace != "" {
					machine.currentClass = namespace + "." + machine.currentClass
				}
				program, err := CompileAnonymous(fmt.Sprintf(`
ResidualLens.Parcel left = new ResidualLens.Parcel();
ResidualLens.Parcel right = new ResidualLens.Parcel();
Set<%[1]s> literal = new Set<%[1]s>{left, right, left, null, null};
System.assertEquals(3, literal.size());
for (ResidualLens.Parcel parcel : literal) { if (parcel != null) parcel.touched = 1; }
System.assertEquals(1, left.touched);
System.assertEquals(1, right.touched);
Set<%[1]s> added = new Set<%[1]s>();
System.assert(added.add(left));
System.assert(added.add(right));
System.assert(!added.add(left));
System.assert(added.contains(left));
System.assert(!added.contains(new ResidualLens.Parcel()));
System.assert(!added.remove(new ResidualLens.Parcel()));
System.assert(added.remove(left));
System.assertEquals(1, added.size());
System.assert(new List<ResidualLens.Parcel>{left}.contains(right));
System.assert(left.equals(right));
`, elementType))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestTypedSetIdentityDoesNotMergeSameShortNameAcrossNamespaces(t *testing.T) {
	machine := New(nil)
	equals, err := CompileAnonymous("return true;")
	if err != nil {
		t.Fatal(err)
	}
	for _, namespace := range []string{"first", "second"} {
		if err := machine.RegisterClass(Class{
			Name: "ResidualLens.Parcel", Namespace: namespace,
			Methods: map[string]Method{"equals": {
				Name: "ResidualLens.Parcel.equals", ClassName: "ResidualLens.Parcel",
				ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: equals,
			}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	value := Object("second.ResidualLens.Parcel")
	hash, err := machine.captureTypedSetInsertionHash("Set<first.ResidualLens.Parcel>", value)
	if err != nil {
		t.Fatal(err)
	}
	if hash.tracked {
		t.Fatal("another namespace's declaration selected concrete identity history")
	}
}
