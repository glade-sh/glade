package vm

import "testing"

// PENDING-ROWS: hydration of a JSON null into an automatic List<user class>
// property needs its own native capture. The captured row and the captured row establish
// the ordinary declared-field/return List<Object> overload view, not hydration.
func TestExecJSONNullListFieldOverloadCandidate(t *testing.T) {
	for _, test := range []struct{ name, payload, decode string }{
		{"explicit null", `{"items":null}`, "deserialize"},
		{"strict explicit null", `{"items":null}`, "deserializeStrict"},
		{"omitted property", `{}`, "deserialize"},
		{"empty list", `{"items":[]}`, "deserialize"},
		{"nonempty list", `{"items":[{}]}`, "deserialize"},
	} {
		for _, property := range []bool{false, true} {
			name := test.name + "/field"
			if property {
				name = test.name + "/property"
			}
			t.Run(name, func(t *testing.T) {
				machine := New(nil)
				for _, class := range []Class{
					{Name: "NullChoiceLab"},
					{Name: "NullChoiceLab.Pebble"},
					{Name: "NullChoiceLab.Parcel", Fields: map[string]Field{
						"items": {Name: "items", Type: "List<NullChoiceLab.Pebble>", Access: "public", Property: property},
					}},
				} {
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				for _, choice := range []struct{ param, label string }{
					{"List<String>", "strings"}, {"List<Object>", "objects"}, {"List<Id>", "ids"},
				} {
					program, err := CompileAnonymous("return '" + choice.label + "';")
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterMethod(Method{
						Name: "NullChoiceLab.inspect", ClassName: "NullChoiceLab", IsStatic: true, Access: "public", ReturnType: "String",
						Params: []Param{{Name: "input", Type: choice.param}}, Program: program,
					}); err != nil {
						t.Fatal(err)
					}
				}
				program, err := CompileAnonymous("NullChoiceLab.Parcel parcel=(NullChoiceLab.Parcel)JSON." + test.decode + "('" + test.payload + "',NullChoiceLab.Parcel.class); System.assertEquals('objects',NullChoiceLab.inspect(parcel.items));")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestJSONNullListFieldCandidateBoundaries(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Pebble"}); err != nil {
		t.Fatal(err)
	}
	for _, class := range []Class{
		{Name: "PlatformPebble", PlatformType: true},
		{Name: "EnumPebble", EnumValues: []string{"One"}},
		{Name: "InterfacePebble", IsInterface: true},
		{Name: "AbstractPebble", IsAbstract: true},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	root, err := machine.typedValueFromJSON("List<Pebble>", nil, false)
	if err != nil || root.Kind != ValueNull || root.Type != "" || root.Static != "" {
		t.Fatalf("root JSON-null conversion changed: %#v, %v", root, err)
	}
	for _, fieldType := range []string{"List<Integer>", "List<String>", "List<Account>", "Set<Pebble>", "Map<String,Pebble>", "Pebble",
		"Pebble[]", "List<PlatformPebble>", "List<EnumPebble>", "List<InterfacePebble>", "List<AbstractPebble>", "List<UnknownPebble>", "List<List<Pebble>>"} {
		value, err := machine.typedApexFieldValueFromJSON(fieldType, orderedJSONField{}, false)
		if err != nil || value.Kind != ValueNull || value.Type != "" || value.Static != "" {
			t.Fatalf("uncaptured field shape %s changed: %#v, %v", fieldType, value, err)
		}
	}
}
