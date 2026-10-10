package vm

import "testing"

// Retained Map SHA256 ba4ea14ec65fac7b98a1a36ece78d99e9aff2db973a09fbaed9ec5b7fba4e158.
// Retained custom-key guide SHA256 3c03a450dbe66c146ef92b94b807b37a07647bd1e4be9f76e324b308a58049ab.
// API67 Map keySet source apex_methods_system_map.md:434-445 and custom-key
// guide langCon_apex_collections_maps_keys_userdefined.md:13-19 support these
// two cases. The Set is obtained before mutation; no general backed-view
// mutation, post-mutation producer, native or API-interval claim is made.
func TestExecMapKeySetRetainsCustomInsertionHashAPI67(t *testing.T) {
	options := CompileOptions{APIVersion: "67.0"}
	constructor, err := CompileAnonymousWithOptions("this.x = initial;", options)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := CompileAnonymousWithOptions("return x;", options)
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
		name, source string
	}{
		{"changedHashAfterKeySet", `
MutableKey key = new MutableKey(1);
Map<MutableKey, String> values = new Map<MutableKey, String>{key => 'v'};
Set<MutableKey> keys = values.keySet();
key.x = 2;
System.assertEquals(false, keys.contains(key));
`},
		{"unchangedHashControl", `
MutableKey key = new MutableKey(1);
Map<MutableKey, String> values = new Map<MutableKey, String>{key => 'v'};
Set<MutableKey> keys = values.keySet();
System.assertEquals(true, keys.contains(key));
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			if err := machine.RegisterClass(Class{
				Name:   "MutableKey",
				Fields: map[string]Field{"x": {Name: "x", Type: "Integer"}},
				Constructors: []Method{{
					Name: "MutableKey.<init>", ClassName: "MutableKey",
					Params: []Param{{Name: "initial", Type: "Integer"}}, Program: constructor,
				}},
				Methods: map[string]Method{
					"hashCode": {Name: "MutableKey.hashCode", ClassName: "MutableKey", ReturnType: "Integer", Program: hash},
					"equals": {Name: "MutableKey.equals", ClassName: "MutableKey", ReturnType: "Boolean",
						Params: []Param{{Name: "other", Type: "Object"}}, Program: equals},
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
