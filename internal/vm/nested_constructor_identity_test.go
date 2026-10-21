package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func TestNestedConstructorFreshBranchIdentity52(t *testing.T) {
	runNestedConstructorIdentity52(t, nestedBranchIdentity52Machine(t), `
BranchItem root=new BranchItem('root',new List<String>{'left','right'});
System.assertEquals('root',root.name);
System.assertEquals(2,root.children.size());
BranchItem left=root.children[0];
BranchItem right=root.children[1];
System.assertEquals('left',left.name);
System.assertEquals('right',right.name);
System.assertEquals(0,left.children.size());
System.assertEquals(0,right.children.size());
System.assertNotEquals(root,left);
System.assertNotEquals(left,right);
left.name='changed';
System.assertEquals('root',root.name);
System.assertEquals('right',right.name);
`)
}

func TestNestedConstructorFreshSuperIdentity52(t *testing.T) {
	machine := New(nil)
	baseOne := nestedConstructorIdentity52Program(t, "this(depth,true);")
	baseTwo := nestedConstructorIdentity52Program(t, "this.depth=depth; if(build && depth>0) next=new OwnedLeaf(depth-1);")
	leaf := nestedConstructorIdentity52Program(t, "super(depth);")
	if err := machine.RegisterClass(Class{
		Name: "OwnedBase", APIVersion: "52.0", Modifiers: []string{"virtual"},
		Fields: map[string]Field{
			"depth": {Name: "depth", Type: "Integer", Access: "public"},
			"next":  {Name: "next", Type: "OwnedLeaf", Access: "public"},
		},
		Constructors: []Method{
			{Name: "OwnedBase.<init>", ClassName: "OwnedBase", APIVersion: "52.0", Access: "public", IsConstructor: true, Params: []Param{{Name: "depth", Type: "Integer"}}, Program: baseOne},
			{Name: "OwnedBase.<init>", ClassName: "OwnedBase", APIVersion: "52.0", Access: "public", IsConstructor: true, Params: []Param{{Name: "depth", Type: "Integer"}, {Name: "build", Type: "Boolean"}}, Program: baseTwo},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "OwnedLeaf", APIVersion: "52.0", SuperClass: "OwnedBase",
		Constructors: []Method{{Name: "OwnedLeaf.<init>", ClassName: "OwnedLeaf", APIVersion: "52.0", Access: "public", IsConstructor: true, Params: []Param{{Name: "depth", Type: "Integer"}}, Program: leaf}},
	}); err != nil {
		t.Fatal(err)
	}
	runNestedConstructorIdentity52(t, machine, `
OwnedLeaf root=new OwnedLeaf(3);
System.assertEquals(3,root.depth);
System.assertEquals(2,root.next.depth);
System.assertEquals(1,root.next.next.depth);
System.assertEquals(0,root.next.next.next.depth);
System.assertEquals(null,root.next.next.next.next);
System.assertNotEquals(root,root.next);
System.assertNotEquals(root.next,root.next.next);
root.next.depth=20;
System.assertEquals(3,root.depth);
System.assertEquals(1,root.next.next.depth);
`)
}

func TestNestedConstructorEmptyAndSequential52Controls(t *testing.T) {
	runNestedConstructorIdentity52(t, nestedBranchIdentity52Machine(t), `
BranchItem first=new BranchItem('first');
BranchItem second=new BranchItem('second');
System.assertEquals(0,first.children.size());
System.assertEquals(0,second.children.size());
System.assertEquals('first',first.name);
System.assertEquals('second',second.name);
System.assertNotEquals(first,second);
first.name='changed';
System.assertEquals('second',second.name);
`)
}

// Owned source52 named and anonymous cycle observations retain the legacy
// LimitException also recorded by A04 C003 at API62/67/68.
func TestNestedConstructorSameReceiverCycleControls(t *testing.T) {
	for _, cycle := range []struct{ name, first, second string }{
		{"direct", "this(value);", ""},
		{"mutual", "this(value,true);", "this(value);"},
	} {
		t.Run(cycle.name, func(t *testing.T) {
			machine := New(nil)
			constructors := []Method{{Name: "OwnedCycle.<init>", ClassName: "OwnedCycle", APIVersion: "52.0", IsConstructor: true, Params: []Param{{Name: "value", Type: "String"}}, Program: nestedConstructorIdentity52Program(t, cycle.first)}}
			if cycle.second != "" {
				constructors = append(constructors, Method{Name: "OwnedCycle.<init>", ClassName: "OwnedCycle", APIVersion: "52.0", IsConstructor: true, Params: []Param{{Name: "value", Type: "String"}, {Name: "forward", Type: "Boolean"}}, Program: nestedConstructorIdentity52Program(t, cycle.second)})
			}
			if err := machine.RegisterClass(Class{Name: "OwnedCycle", APIVersion: "52.0", Constructors: constructors}); err != nil {
				t.Fatal(err)
			}
			_, err := machine.Execute(nestedConstructorIdentity52Program(t, "OwnedCycle value=new OwnedCycle('owned');"))
			if err == nil || err.Error() != "System.LimitException: Maximum stack depth reached: 1001" {
				t.Fatalf("same-receiver cycle changed: %v", err)
			}
			if len(machine.activeConstructors) != 0 {
				t.Fatal("constructor error retained active bookkeeping")
			}
		})
	}
}

func nestedBranchIdentity52Machine(t *testing.T) *VM {
	t.Helper()
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "BranchItem", APIVersion: "52.0",
		Fields: map[string]Field{
			"name":     {Name: "name", Type: "String", Access: "public"},
			"children": {Name: "children", Type: "List<BranchItem>", Access: "public"},
		},
		Constructors: []Method{
			{Name: "BranchItem.<init>", ClassName: "BranchItem", APIVersion: "52.0", IsConstructor: true, Params: []Param{{Name: "name", Type: "String"}}, Program: nestedConstructorIdentity52Program(t, "this(name,new List<String>());")},
			{Name: "BranchItem.<init>", ClassName: "BranchItem", APIVersion: "52.0", IsConstructor: true, Params: []Param{{Name: "name", Type: "String"}, {Name: "labels", Type: "List<String>"}}, Program: nestedConstructorIdentity52Program(t, "this.name=name; children=new List<BranchItem>(); for(String label:labels) children.add(new BranchItem(label));")},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return machine
}

func nestedConstructorIdentity52Program(t *testing.T, source string) ir.Program {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "52.0"})
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func runNestedConstructorIdentity52(t *testing.T, machine *VM, source string) {
	t.Helper()
	if _, err := machine.Execute(nestedConstructorIdentity52Program(t, source)); err != nil {
		t.Fatal(err)
	}
	if len(machine.activeConstructors) != 0 {
		t.Fatal("completed nested constructors retained active bookkeeping")
	}
}

func TestConstructorCallKeyUsesReceiverIdentity(t *testing.T) {
	method := Method{Name: "OwnedBase.<init>", ClassName: "OwnedBase", Params: []Param{{Name: "depth", Type: "Integer"}}}
	receiver := Object("OwnedLeaf")
	copy := receiver
	baseView := receiver
	baseView.Type = "OwnedBase"
	baseView.Static = "OwnedBase"
	key := constructorCallKey(method, receiver)
	if constructorCallKey(method, copy) != key || constructorCallKey(method, baseView) != key {
		t.Fatal("receiver copy or superclass view changed constructor identity")
	}
	if constructorCallKey(method, Object("OwnedLeaf")) == key {
		t.Fatal("fresh receiver shared active constructor identity")
	}
	otherSignature := method
	otherSignature.Params = nil
	if constructorCallKey(otherSignature, receiver) == key {
		t.Fatal("distinct constructor signatures shared active identity")
	}
	legacy := "OwnedBase\x00OwnedBase.<init>\x00Integer"
	for _, receiver := range []Value{{Kind: ValueObject, Type: "OwnedLeaf"}, {Kind: ValueObject, Type: "OwnedBase"}, Null} {
		if constructorCallKey(method, receiver) != legacy {
			t.Fatal("zero-reference receiver changed legacy constructor signature key")
		}
	}
}
