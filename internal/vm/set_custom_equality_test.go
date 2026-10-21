package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Set equality compares members the way Set.contains finds them. Two sets of
// distinct key instances whose class declares equals and hashCode are equal
// through ==, Set.equals, Object views and nested List equality. A matcher
// that stores an expected Set argument as Object relies on this.
func TestExecSetEqualityUsesDeclaredEqualsAndHashCode(t *testing.T) {
	compile := func(source string) ir.Program {
		t.Helper()
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		return program
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Ticket",
		Fields: map[string]Field{
			"code": {Name: "code", Type: "String"},
			"tag":  {Name: "tag", Type: "String"},
		},
		Constructors: []Method{{
			Name: "Ticket.<init>", ClassName: "Ticket",
			Params: []Param{{Name: "initial", Type: "String"}}, Program: compile("this.code = initial;"),
		}},
		Methods: map[string]Method{
			"equals": {Name: "Ticket.equals", ClassName: "Ticket", ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: compile(`
if (other instanceof Ticket) {
    Ticket that = (Ticket) other;
    return code == that.code && tag == that.tag;
}
return false;
`)},
			"hashCode": {Name: "Ticket.hashCode", ClassName: "Ticket", ReturnType: "Integer", Program: compile(`
Integer hash = code.hashCode();
if (tag != null) { hash += tag.hashCode(); }
return hash;
`)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	program := compile(`
Set<Ticket> literal = new Set<Ticket>{ new Ticket('a1') };
Set<Ticket> built = new Set<Ticket>(new List<Ticket>{ new Ticket('a1') });
System.assert(literal == built);
System.assert(!(literal != built));
System.assert(literal.equals(built));
Object expected = literal;
Object actual = built;
System.assert(expected == actual);
System.assert(new List<Object>{ literal } == new List<Object>{ built });
System.assert(literal != new Set<Ticket>{ new Ticket('b2') });
System.assert(!literal.equals(new Set<Ticket>{ new Ticket('b2') }));
System.assert(literal != new Set<Ticket>{ new Ticket('a1'), new Ticket('b2') });
System.assert(new Set<String>{ 'x' } != new Set<String>{ 'X' });
`)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
