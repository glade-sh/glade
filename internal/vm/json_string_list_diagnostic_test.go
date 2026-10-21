package vm

import (
	"fmt"
	"testing"
)

// Owned API 62/67/68 anonymous and named observations cover these single-invalid
// List<String> shapes. They do not establish traversal order for multiple errors.
func TestJSONDeserializeStringListNativeDiagnostics(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, payload, target, want string
		}{
			{"root", `"LeafOne"`, "List<String>", "Malformed JSON: Expected '[' at the beginning of List/Set"},
			{"nested", `{"entries":"LeafOne"}`, "StringListDiagnosticBox", `Expected List<String> but found "LeafOne" at [line:1, column:2]`},
			{"inList", `[{"entries":["ok"]},{"entries":"LeafTwo"}]`, "List<StringListDiagnosticBox>", `Expected List<String> but found "LeafTwo" at [line:1, column:22]`},
			{"escaped", `{"entries":"Leaf\"One"}`, "StringListDiagnosticBox", `Expected List<String> but found "Leaf"One" at [line:1, column:2]`},
			{"object", `{"entries":{}}`, "StringListDiagnosticBox", "Expected List<String> but found { at [line:1, column:2]"},
			{"number", `{"entries":7}`, "StringListDiagnosticBox", "Expected List<String> but found null at [line:1, column:2]"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
String observedType='';
String observedMessage='';
try {
 Object decoded=JSON.deserialize(payload,Type.forName(targetName));
} catch(JSONException error) {
 observedType=error.getTypeName();
 observedMessage=error.getMessage();
}
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := stringListDiagnosticMachine(t, api)
				machine.Globals["payload"] = String(tc.payload)
				machine.Globals["targetName"] = String(tc.target)
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if got := result.Vars["observedType"]; got.Kind != ValueString || got.Text != "System.JSONException" {
					t.Fatalf("exception type=%#v, want System.JSONException", got)
				}
				if got := result.Vars["observedMessage"]; got.Kind != ValueString || got.Text != tc.want {
					t.Fatalf("exception message=%q, native=%q", got.Text, tc.want)
				}
			})
		}
	}
}

func TestJSONDeserializeStringListNativeValueControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ name, payload, assertion string }{
			{"valid", `{"entries":["LeafOne"]}`, "System.assertEquals(new List<String>{'LeafOne'},decoded.entries);"},
			{"null", `{"entries":null}`, "System.assertEquals(null,decoded.entries);"},
			{"empty", `{"entries":[]}`, "System.assertEquals(new List<String>(),decoded.entries);"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				source := fmt.Sprintf(`StringListDiagnosticBox decoded=(StringListDiagnosticBox)JSON.deserialize(payload,StringListDiagnosticBox.class); %s`, tc.assertion)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := stringListDiagnosticMachine(t, api)
				machine.Globals["payload"] = String(tc.payload)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func stringListDiagnosticMachine(t *testing.T, api string) *VM {
	t.Helper()
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "StringListDiagnosticBox", APIVersion: api,
		Fields: map[string]Field{"entries": {Name: "entries", Type: "List<String>"}},
	}); err != nil {
		t.Fatal(err)
	}
	return machine
}
