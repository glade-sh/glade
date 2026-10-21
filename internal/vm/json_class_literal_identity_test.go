package vm

import (
	"fmt"
	"testing"
)

// The Apex Developer Guide's "Using Schema Namespace" section requires
// Schema qualification to distinguish an sObject from a same-named custom
// class. A class literal must keep that class identity when passed to JSON.
func TestJSONDeserializeClassLiteralKeepsLexicalDTOIdentity(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, isTest := range []bool{false, true} {
			for _, tc := range []struct{ name, source string }{
				{"direct", `JsonTokenEnvelope.Account decoded=(JsonTokenEnvelope.Account)JSON.deserialize('{"label":"root","children":[{"label":"child"}]}',Account.class); System.assertEquals('root',decoded.label); System.assertEquals(1,decoded.children.size()); System.assertEquals('child',decoded.children[0].label);`},
				{"helper", `JsonTokenEnvelope.Account decoded=(JsonTokenEnvelope.Account)JsonTokenBridge.decode('{"label":"root","children":[{"label":"child"}]}',Account.class); System.assertEquals('root',decoded.label); System.assertEquals('child',decoded.children[0].label);`},
				{"strict", `JsonTokenEnvelope.Account decoded=(JsonTokenEnvelope.Account)JSON.deserializeStrict('{"label":"root","children":[{"label":"child"}]}',Account.class); System.assertEquals('root',decoded.label); System.assertEquals('child',decoded.children[0].label);`},
				{"ordinary", `JsonTokenEnvelope.Child decoded=(JsonTokenEnvelope.Child)JsonTokenBridge.decode('{"label":"child"}',Child.class); System.assertEquals('child',decoded.label);`},
				{"schema", `Schema.Account decoded=(Schema.Account)JsonTokenBridge.decode('{"Name":"record"}',Schema.Account.class); System.assertEquals('record',decoded.Name);`},
				{"display", `System.assertEquals('Account',Account.class.getName()); System.assertEquals('Account',Schema.Account.class.getName()); System.assertEquals('JsonTokenEnvelope.Child',Child.class.getName());`},
			} {
				t.Run(fmt.Sprintf("%s/isTest=%t/%s", api, isTest, tc.name), func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := jsonClassLiteralTestMachine(t, api)
					if isTest {
						machine.EnableTestContext()
					}
					if _, err := machine.ExecuteInClass(program, "JsonTokenEnvelope"); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func jsonClassLiteralTestMachine(t *testing.T, api string) *VM {
	t.Helper()
	decode, err := CompileAnonymousWithOptions("return JSON.deserialize(body,target);", CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	for _, class := range []Class{
		{Name: "JsonTokenEnvelope", APIVersion: api},
		{Name: "JsonTokenEnvelope.Account", APIVersion: api, Fields: map[string]Field{
			"label":    {Name: "label", Type: "String"},
			"children": {Name: "children", Type: "List<JsonTokenEnvelope.Child>"},
		}},
		{Name: "JsonTokenEnvelope.Child", APIVersion: api, Fields: map[string]Field{
			"label": {Name: "label", Type: "String"},
		}},
		{Name: "JsonTokenBridge", APIVersion: api, Methods: map[string]Method{
			"decode": {Name: "JsonTokenBridge.decode", ClassName: "JsonTokenBridge", IsStatic: true, ReturnType: "Object", Params: []Param{{Name: "body", Type: "String"}, {Name: "target", Type: "Type"}}, Program: decode},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}
