package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// equals_only_set.json rows 001-007 capture distinct identities, mutation,
// custom-hash collisions, same-reference/null deduplication and add membership.
// Salesforce's collections guide requires hashCode before equals on collisions.
func TestExecSetElementAliasesPreserveIdentityHash(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, element := range []string{"Parcel", "IdentityBoundaryProbe.Parcel", "identityboundaryprobe.parcel"} {
			t.Run(api+"/"+element, func(t *testing.T) {
				compile := func(source string) ir.Program {
					t.Helper()
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					return program
				}
				body := compile(fmt.Sprintf(`
Parcel first = new Parcel(); Parcel second = new Parcel();
Assert.isTrue(first.equals(second));
Set<%s> parcels = new Set<%s>{first, second};
Assert.areEqual(2, parcels.size());
for (Parcel item : parcels) { item.seen = true; }
Assert.isTrue(first.seen); Assert.isTrue(second.seen);
Assert.areEqual(2, new Set<%s>{first, first, null, null}.size());
Set<%s> added = new Set<%s>();
Assert.isTrue(added.add(first)); Assert.isTrue(added.add(second));
Assert.isTrue(added.contains(first));
Assert.isFalse(added.contains(new Parcel()));
Assert.isFalse(added.remove(new Parcel()));
Assert.isTrue(added.remove(first));
Set<%s> collected = new Set<%s>();
Assert.isTrue(collected.addAll(new List<Parcel>{first, second}));
Assert.areEqual(2, collected.size());
Assert.areEqual(2, new Set<%s>(collected).size());
Assert.isFalse(collected.contains(new Parcel()));
Assert.isFalse(collected.remove(new Parcel()));
Assert.areEqual(2, collected.size());
Set<HashedParcel> hashedBulk = new Set<HashedParcel>();
hashedBulk.addAll(new List<HashedParcel>{new HashedParcel(), new HashedParcel()});
Assert.areEqual(1, hashedBulk.size());
Assert.areEqual(1, new Set<HashedParcel>(hashedBulk).size());
Assert.areEqual(1, new Set<HashedParcel>{new HashedParcel(), new HashedParcel()}.size());
`, element, element, element, element, element, element, element, element))
				machine := New(nil)
				for _, name := range []string{"Parcel", "HashedParcel"} {
					className := "IdentityBoundaryProbe." + name
					class := Class{
						Name:   className,
						Fields: map[string]Field{"seen": {Name: "seen", Type: "Boolean", InitialValue: Bool(false)}},
						Methods: map[string]Method{
							"equals": {Name: className + ".equals", ClassName: className, ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: compile("return other instanceof " + className + ";")},
						},
					}
					if name == "HashedParcel" {
						class.Methods["hashCode"] = Method{Name: className + ".hashCode", ClassName: className, ReturnType: "Integer", Program: compile("return 7;")}
					}
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				if err := machine.RegisterClass(Class{Name: "IdentityBoundaryProbe", Methods: map[string]Method{
					"exercise": {Name: "IdentityBoundaryProbe.exercise", ClassName: "IdentityBoundaryProbe", IsStatic: true, APIVersion: api, Program: body},
				}}); err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(compile("IdentityBoundaryProbe.exercise();")); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
