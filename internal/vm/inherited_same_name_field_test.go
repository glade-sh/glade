package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Independent synthetic controls for distinct declarations with exactly the
// same field spelling. Owned anonymous/@IsTest observations at APIs 62/67/68
// are exported in apextest/testdata/conformance/member_alias_native.json.
func TestInheritedSameNameInstanceFieldSlots(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, fieldType, baseValue, checks string
		}{
			{
				name: "string", fieldType: "String",
				baseValue: "'outer'",
				checks: `
System.assertEquals('outer', subject.readBase());
System.assertEquals('outer', subject.readBaseThis());
System.assertEquals('outer', subject.readThroughSuper());
System.assertEquals(null, subject.readChild());
System.assertEquals(null, subject.readChildThis());
subject.writeChild('inner');
System.assertEquals('inner', subject.readChildThis());
System.assertEquals('outer', subject.readBaseThis());
subject.writeBase('replacement');
System.assertEquals('replacement', subject.readThroughSuper());
System.assertEquals('inner', subject.readChild());
subject.writeChildThis(null);
System.assertEquals(null, subject.readChild());
System.assertEquals('replacement', subject.readBase());
subject.writeBaseThis('last');
System.assertEquals('last', subject.readBaseThis());
System.assertEquals(null, subject.readChildThis());`,
			},
			{
				name: "iterator", fieldType: "Iterator<Object>",
				baseValue: "new List<Object>{'outer'}.iterator()",
				checks: `
System.assert(subject.probeThroughSuper());
System.assertEquals(null, subject.readChild());
System.assertEquals(null, subject.readChildThis());
System.assertEquals('outer', subject.readBase().next());
System.assert(!subject.readBaseThis().hasNext());
subject.writeChild(new List<Object>{'inner'}.iterator());
System.assertEquals('inner', subject.readChildThis().next());
System.assert(!subject.readThroughSuper().hasNext());
subject.writeBase(new List<Object>{'replacement'}.iterator());
System.assert(subject.probeThroughSuper());
System.assert(!subject.readChild().hasNext());
subject.writeChildThis(null);
System.assertEquals('replacement', subject.readBaseThis().next());
System.assertEquals(null, subject.readChild());
subject.writeBaseThis(new List<Object>{'last'}.iterator());
System.assertEquals('last', subject.readThroughSuper().next());
System.assertEquals(null, subject.readChildThis());`,
			},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				compile := func(source string) ir.Program {
					t.Helper()
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					return program
				}
				machine := New(nil)
				for _, class := range []Class{
					{
						Name: "CursorBase", Modifiers: []string{"virtual"},
						Fields: map[string]Field{"cursor": {Name: "cursor", Type: tc.fieldType, Access: "protected"}},
						Constructors: []Method{{
							Name: "CursorBase.<init>", ClassName: "CursorBase", Access: "public", IsConstructor: true,
							Params: []Param{{Name: "cursor", Type: tc.fieldType}}, Program: compile("this.cursor = cursor;"),
						}},
					},
					{
						Name: "CursorChild", SuperClass: "CursorBase",
						Fields: map[string]Field{"cursor": {Name: "cursor", Type: tc.fieldType, Access: "private"}},
						Constructors: []Method{{
							Name: "CursorChild.<init>", ClassName: "CursorChild", Access: "public", IsConstructor: true,
							Params: []Param{{Name: "cursor", Type: tc.fieldType}}, Program: compile("super(cursor); this.cursor = null;"),
						}},
					},
				} {
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				for _, method := range []struct {
					owner, name, body string
					write             bool
				}{
					{"CursorBase", "readBase", "return cursor;", false},
					{"CursorBase", "readBaseThis", "return this.cursor;", false},
					{"CursorChild", "readChild", "return cursor;", false},
					{"CursorChild", "readChildThis", "return this.cursor;", false},
					{"CursorChild", "readThroughSuper", "return super.readBaseThis();", false},
					{"CursorBase", "writeBase", "cursor = value;", true},
					{"CursorBase", "writeBaseThis", "this.cursor = value;", true},
					{"CursorChild", "writeChild", "cursor = value;", true},
					{"CursorChild", "writeChildThis", "this.cursor = value;", true},
				} {
					definition := Method{Name: method.owner + "." + method.name, ClassName: method.owner, Access: "public", ReturnType: tc.fieldType, Program: compile(method.body)}
					if method.write {
						definition.ReturnType = "void"
						definition.Params = []Param{{Name: "value", Type: tc.fieldType}}
					}
					if err := machine.RegisterMethod(definition); err != nil {
						t.Fatal(err)
					}
				}
				if tc.name == "iterator" {
					for _, method := range []Method{
						{Name: "CursorBase.probe", ClassName: "CursorBase", Access: "public", ReturnType: "Boolean", Program: compile("return cursor.hasNext();")},
						{Name: "CursorChild.probeThroughSuper", ClassName: "CursorChild", Access: "public", ReturnType: "Boolean", Program: compile("return super.probe();")},
					} {
						if err := machine.RegisterMethod(method); err != nil {
							t.Fatal(err)
						}
					}
				}
				program := compile("CursorChild subject = new CursorChild(" + tc.baseValue + ");" + tc.checks)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Source-free repro for inherited code that reads a same-named protected
// declaration after a derived constructor assigns a differently typed field.
func TestInheritedMethodReadsRedefinedProtectedField(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			compile := func(source string) ir.Program {
				t.Helper()
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				return program
			}
			machine := New(nil)
			for _, class := range []Class{
				{
					Name: "ProtectedBase", Modifiers: []string{"virtual"},
					Fields: map[string]Field{"state": {Name: "state", Type: "List<String>", Access: "protected"}},
					Constructors: []Method{{
						Name: "ProtectedBase.<init>", ClassName: "ProtectedBase", Access: "public", IsConstructor: true,
						Program: compile(""),
					}},
				},
				{
					Name: "ProtectedChild", SuperClass: "ProtectedBase",
					Fields: map[string]Field{"state": {Name: "state", Type: "List<Object>", Access: "protected"}},
					Constructors: []Method{{
						Name: "ProtectedChild.<init>", ClassName: "ProtectedChild", Access: "public", IsConstructor: true,
						Program: compile("super(); this.state = new List<Object>{'child'};"),
					}},
				},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			if err := machine.RegisterMethod(Method{
				Name: "ProtectedBase.readBaseSize", ClassName: "ProtectedBase", Access: "public",
				ReturnType: "Integer", Program: compile("return state == null ? -1 : state.size();"),
			}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{
				Name: "ProtectedBase.writeState", ClassName: "ProtectedBase", Access: "public",
				ReturnType: "void", Program: compile("state = new List<String>{'base', 'updated'};"),
			}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{
				Name: "ProtectedChild.readDerivedSize", ClassName: "ProtectedChild", Access: "public",
				ReturnType: "Integer", Program: compile("return state.size();"),
			}); err != nil {
				t.Fatal(err)
			}
			program := compile("ProtectedChild child = new ProtectedChild(); System.assertEquals(-1, child.readBaseSize()); System.assertEquals(1, child.readDerivedSize()); child.writeState(); System.assertEquals(2, child.readBaseSize()); System.assertEquals(1, child.readDerivedSize());")
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
