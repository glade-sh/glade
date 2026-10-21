package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Apex Developer Guide, "Using Custom Types in Map Keys and Sets": Set
// uniqueness uses hashCode and equals. Literal insertion must use the same
// identity hash as add when a class overrides only equals(Object).
func TestExecSetEqualsOnlyLiteralMatchesAdd(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, namespace := range []string{"", "fixture"} {
			t.Run(fmt.Sprintf("%s/namespace=%s", api, namespace), func(t *testing.T) {
				compile := func(source string) ir.Program {
					t.Helper()
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					return program
				}
				machine := New(nil)
				outer := Class{Name: "SetIdentityProbe", Namespace: namespace, APIVersion: api}
				outer.Methods = map[string]Method{"check": {
					Name: "SetIdentityProbe.check", ClassName: outer.Name,
					ReturnType: "void", IsStatic: true, APIVersion: api,
					Program: compile(`
Probe first = new Probe();
Probe second = new Probe();
System.assert(first.equals(second));
Set<Probe> added = new Set<Probe>();
System.assert(added.add(first));
System.assert(added.add(second));
Set<Probe> literal = new Set<Probe>{first, second};
System.assertEquals(added.size(), literal.size());
System.assertEquals(2, literal.size());
for (Probe value : literal) { value.parts.add('visited'); }
System.assertEquals(1, first.parts.size());
System.assertEquals(1, second.parts.size());
System.assertEquals(1, new Set<Probe>{first, first}.size());
System.assert(!added.add(first));
`),
				}}
				if err := machine.RegisterClass(outer); err != nil {
					t.Fatal(err)
				}
				inner := outer.Name + ".Probe"
				if err := machine.RegisterClass(Class{
					Name: inner, Namespace: namespace, APIVersion: api,
					Fields:       map[string]Field{"parts": {Name: "parts", Type: "List<String>"}},
					Constructors: []Method{{Name: inner + ".<init>", ClassName: inner, APIVersion: api, Program: compile("parts = new List<String>();")}},
					Methods: map[string]Method{"equals": {
						Name: inner + ".equals", ClassName: inner, ReturnType: "Boolean", APIVersion: api,
						Params:  []Param{{Name: "other", Type: "Object"}},
						Program: compile("return other instanceof Probe && ((Probe)other).parts.size() == parts.size();"),
					}},
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteInClass(compile(runtimeClassName(outer)+".check();"), runtimeClassName(outer)); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
