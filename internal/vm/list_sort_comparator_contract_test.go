package vm

import "testing"

// API67 List.sort(comparator) requires a class implementing Comparator. Source:
// catalog SHA256 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/2205/members/21 and /documents/2139/members/0; List guide SHA256
// 195f91a116ecd28fca7dd8a62dface15c6ed55e2a44bbff485cd74b88b2ca6a3,
// lines 774-793; Comparator guide SHA256
// c11be587181a152480245f76c78cce67d1c87a153ee302107242dbea36926871,
// lines 1-4 and 21-45. The admitted nominal applicability inference rejects a
// compare method without the interface, without requiring a rejection phase,
// exception class, message, or catchability. Inherited and System-qualified
// declarations are compatibility controls for the existing nominal resolver.
// Collator guide SHA256
// 83648a58044ab727191cb3f7df3fc09f7bf3bc2b2cce655135cb18b3dc1fc2a5,
// lines 3-6, explicitly admits Collator as List.sort's Comparator. Empty and
// singleton controls exercise applicability without asserting locale ordering.
func TestExecListSortRequiresDeclaredComparatorAPI67(t *testing.T) {
	compareProgram, err := CompileAnonymousWithOptions(`
if (left < right) {
	return -1;
}
if (left > right) {
	return 1;
}
return 0;
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name          string
		comparator    string
		source        string
		empty         bool
		wantRejection bool
	}{
		{name: "PlainNonemptyRejects", comparator: "PlainOrder", wantRejection: true},
		{name: "PlainEmptyRejects", comparator: "PlainOrder", empty: true, wantRejection: true},
		{name: "DeclaredNonempty", comparator: "DeclaredOrder"},
		{name: "DeclaredEmpty", comparator: "DeclaredOrder", empty: true},
		{name: "InheritedNonempty", comparator: "InheritedOrder"},
		{name: "InheritedEmpty", comparator: "InheritedOrder", empty: true},
		{name: "SystemQualifiedNonempty", comparator: "SystemOrder"},
		{name: "SystemQualifiedEmpty", comparator: "SystemOrder", empty: true},
		{
			name: "BuiltinCollatorEmpty",
			source: `
List<String> values = new List<String>();
values.sort(Collator.getInstance());
System.assertEquals(0, values.size());
`,
		},
		{
			name: "BuiltinCollatorSingleton",
			source: `
List<String> values = new List<String>{'only'};
values.sort(Collator.getInstance());
System.assertEquals(1, values.size());
System.assertEquals('only', values.get(0));
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			for _, definition := range []struct {
				name       string
				interfaces []string
			}{
				{name: "PlainOrder"},
				{name: "DeclaredOrder", interfaces: []string{"Comparator<Integer>"}},
				{name: "SystemOrder", interfaces: []string{"System.Comparator<Integer>"}},
			} {
				if err := machine.RegisterClass(Class{
					Name:       definition.name,
					Interfaces: definition.interfaces,
					Methods: map[string]Method{
						"compare": {
							Name:       definition.name + ".compare",
							ClassName:  definition.name,
							ReturnType: "Integer",
							Params:     []Param{{Name: "left", Type: "Integer"}, {Name: "right", Type: "Integer"}},
							Program:    compareProgram,
						},
					},
				}); err != nil {
					t.Fatal(err)
				}
			}
			if err := machine.RegisterClass(Class{Name: "InheritedOrder", SuperClass: "DeclaredOrder"}); err != nil {
				t.Fatal(err)
			}
			list := "new List<Integer>{2, 1}"
			checks := "System.assertEquals(1, values.get(0)); System.assertEquals(2, values.get(1));"
			if tc.empty {
				list = "new List<Integer>()"
				checks = "System.assertEquals(0, values.size());"
			}
			source := "List<Integer> values = " + list + "; values.sort(new " + tc.comparator + "());"
			if !tc.wantRejection {
				source += checks
			}
			if tc.source != "" {
				source = tc.source
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				if tc.wantRejection {
					return // Compile-time or execution rejection satisfies the contract.
				}
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			_, err = machine.Execute(program)
			if tc.wantRejection {
				if err == nil {
					t.Fatal("List.sort accepted compare without a declared Comparator interface")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
