package vm

import "testing"

func TestLegacyRecordListScalarInstanceSource37(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Object populated=new List<SObject>{new Account(Name='Owned')};
Object empty=new List<SObject>();
Object placeholder=new List<SObject>{null};
System.assert(populated instanceof SObject);
System.assert(empty instanceof SObject);
System.assert(placeholder instanceof SObject);
System.assert(populated instanceof List<SObject>);
System.assertEquals(1,((List<SObject>)populated).size());
System.assertEquals(null,((List<SObject>)placeholder)[0]);
Object scalar=new Account();Object absent;
System.assert(scalar instanceof SObject);
System.assertEquals(false,absent!=null && absent instanceof SObject);
`, CompileOptions{APIVersion: "37.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyAccountListScalarInstanceSource37(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Object populated=new List<Account>{new Account(Name='Owned')};
Object empty=new List<Account>();
Object placeholder=new List<Account>{null};
System.assert(populated instanceof SObject);
System.assert(empty instanceof SObject);
System.assert(placeholder instanceof SObject);
System.assertEquals(1,((List<Account>)populated).size());
System.assertEquals(null,((List<Account>)placeholder)[0]);
Object scalar=new Account();Object absent;
System.assert(scalar instanceof SObject);
System.assertEquals(false,absent!=null && absent instanceof SObject);
`, CompileOptions{APIVersion: "37.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

// These retain local baseline boundaries, not unobserved native claims.
func TestLegacyRecordListScalarInstanceKeepsOtherShapes(t *testing.T) {
	for _, api := range []string{"34.0", "36.0", "38.0", "39.0", "48.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			machine.currentMethod.APIVersion = api
			v := List()
			v.Type = "List<SObject>"
			v.Runtime = v.Type
			v.Static = "Object"
			if got := machine.evalInstanceOf(v, "SObject"); got.Bool {
				t.Fatalf("uncaptured source %s changed", api)
			}
			v.Type = "List<Account>"
			v.Runtime = v.Type
			if machine.evalInstanceOf(v, "SObject").Bool {
				t.Fatalf("uncaptured Account source %s changed", api)
			}
		})
	}
	machine := New(nil)
	machine.currentMethod.APIVersion = "37.0"
	for _, element := range []string{"Contact", "Integer", "Object"} {
		v := List()
		v.Type = "List<" + element + ">"
		v.Runtime = v.Type
		v.Static = "Object"
		if got := machine.evalInstanceOf(v, "SObject"); got.Bool {
			t.Fatalf("uncaptured element %s changed", element)
		}
	}
	v := List()
	v.Type = "List<SObject>"
	v.Runtime = v.Type
	v.Static = "List<SObject>"
	if machine.evalInstanceOf(v, "SObject").Bool {
		t.Fatal("unboxed list changed")
	}
	v.Static = "Object"
	for _, target := range []string{"Account", "Contact", "Schema.SObject"} {
		if machine.evalInstanceOf(v, target).Bool {
			t.Fatalf("uncaptured target %s changed", target)
		}
	}
}

// Preserve the pre-extension boundary when the runtime label denotes a user
// class rather than the standard record. This is a local trust control.
func TestLegacyRecordListScalarInstanceKeepsAccountClassShadow(t *testing.T) {
	machine := New(nil)
	machine.currentMethod.APIVersion = "37.0"
	if err := machine.RegisterClass(Class{Name: "Account"}); err != nil {
		t.Fatal(err)
	}
	value := List()
	value.Type = "List<Account>"
	value.Runtime = value.Type
	value.Static = "Object"
	if machine.evalInstanceOf(value, "SObject").Bool {
		t.Fatal("user class Account became a record list")
	}
}
