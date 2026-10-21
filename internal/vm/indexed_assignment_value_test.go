package vm

import "testing"

// Apex's right-associative assignment operator produces a value, unlike the
// explicit Void List.set method. Exact slot-expression probe twins are pending.
func TestIndexedAssignmentReturnsValueAndKeepsLazyFallback(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> slots = new List<Integer>{null};
Integer chosen = slots[0] = 7;
System.assertEquals(7, chosen);
System.assertEquals(7, slots[0]);
List<Integer> fallback = new List<Integer>{11};
Integer nextIndex = 0;
System.assertEquals(7, slots[0] ?? fallback[nextIndex++]);
System.assertEquals(0, nextIndex);
slots[0] = null;
System.assertEquals(11, slots[0] ?? fallback[nextIndex++]);
System.assertEquals(1, nextIndex);
Integer receiverIndex = 0;
Integer slotIndex = 0;
Integer source = 13;
List<List<Integer>> holders = new List<List<Integer>>{slots};
Integer once = holders[receiverIndex++][slotIndex++] = source++;
System.assertEquals(13, once);
System.assertEquals(13, slots[0]);
System.assertEquals(1, receiverIndex);
System.assertEquals(1, slotIndex);
System.assertEquals(14, source);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestIndexedAssignmentReturnsCachedRecordAlias(t *testing.T) {
	machine := New(nil)
	body, err := CompileAnonymous("row.Id = '001000000000001'; return cache[0] = row;")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "SlotFactory.save", ClassName: "SlotFactory", IsStatic: true, ReturnType: "Account", Params: []Param{{Name: "cache", Type: "List<Account>"}, {Name: "row", Type: "Account"}}, Program: body}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Account> cache = new List<Account>{null};
Account row = new Account(Name='owned');
Account saved = SlotFactory.save(cache, row);
System.assertNotEquals(null, saved);
System.assertEquals('001000000000001', saved.Id);
saved.Name='changed';
System.assertEquals('changed', row.Name);
System.assertEquals('changed', cache[0].Name);
Account chosen = cache[0] = row;
System.assertNotEquals(null, chosen);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestIndexedAssignmentKeepsExplicitSetVoid(t *testing.T) {
	machine := New(nil)
	slots := List(Int(0))
	slots.Type = "List<Integer>"
	machine.Globals["slots"] = slots
	value, handled, err := machine.callListValueMember("slots", slots, "set", []Value{Int(0), Int(7)}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || value.Kind != ValueNull || machine.Globals["slots"].List[0].Int != 7 {
		t.Fatalf("explicit set changed: %#v", value)
	}
}

func TestIndexedAssignmentReturnsStoredType(t *testing.T) {
	machine := New(nil)
	setup, err := CompileAnonymous("List<Account> slots=new List<Account>{new Account()};")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(setup); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("return slots[0]=null;")
	if err != nil {
		t.Fatal(err)
	}
	value, err := machine.eval(program.Instructions[0].Expr, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	stored := machine.Globals["slots"].List[0]
	if value.Kind != ValueNull || value.Type != "Account" || value.Type != stored.Type {
		t.Fatalf("assignment lost stored type: %#v / %#v", value, stored)
	}
}
