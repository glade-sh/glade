package vm

import "testing"

// API67 Map membership uses the custom key's retained insertion hash. The
// mutation case changes the field used by both hashCode and equals.
func TestExecMapContainsKeyRetainsCustomInsertionHashAPI67(t *testing.T) {
	options := CompileOptions{APIVersion: "67.0"}
	constructor, err := CompileAnonymousWithOptions("this.x = initial;", options)
	if err != nil {
		t.Fatal(err)
	}
	equals, err := CompileAnonymousWithOptions(`
MutableKey candidate = other instanceof MutableKey ? (MutableKey)other : null;
return candidate != null && x == candidate.x;
`, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		hash   string
		source string
	}{
		{
			name: "changedHash",
			hash: "return x;",
			source: `
MutableKey k = new MutableKey(1);
Map<MutableKey, String> values = new Map<MutableKey, String>{k => 'v'};
k.x = 2;
System.assertEquals(false, values.containsKey(k));
`,
		},
		{
			name: "unchangedHash",
			hash: "return x;",
			source: `
MutableKey k = new MutableKey(1);
Map<MutableKey, String> values = new Map<MutableKey, String>{k => 'v'};
System.assertEquals(true, values.containsKey(k));
System.assertEquals(true, values.containsKey(new MutableKey(1)));
`,
		},
		{
			name: "hashCollisionControls",
			hash: "return 7;",
			source: `
MutableKey first = new MutableKey(1);
MutableKey second = new MutableKey(2);
Map<MutableKey, String> values = new Map<MutableKey, String>{first => 'first', second => 'second'};
System.assertEquals(2, values.size());
System.assertEquals(true, values.containsKey(new MutableKey(1)));
System.assertEquals(true, values.containsKey(new MutableKey(2)));
System.assertEquals(false, values.containsKey(new MutableKey(3)));
System.assert(values.get(new MutableKey(1)).equals('first'));
System.assert(values.get(new MutableKey(2)).equals('second'));
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := CompileAnonymousWithOptions(tc.hash, options)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			if err := machine.RegisterClass(Class{
				Name: "MutableKey",
				Fields: map[string]Field{
					"x": {Name: "x", Type: "Integer"},
				},
				Constructors: []Method{{
					Name: "MutableKey.<init>", ClassName: "MutableKey",
					Params: []Param{{Name: "initial", Type: "Integer"}}, Program: constructor,
				}},
				Methods: map[string]Method{
					"hashCode": {Name: "MutableKey.hashCode", ClassName: "MutableKey", ReturnType: "Integer", Program: hash},
					"equals": {
						Name: "MutableKey.equals", ClassName: "MutableKey", ReturnType: "Boolean",
						Params: []Param{{Name: "other", Type: "Object"}}, Program: equals,
					},
				},
			}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymousWithOptions(tc.source, options)
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
