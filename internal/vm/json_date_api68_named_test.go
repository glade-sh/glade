package vm

import "testing"

// The named-project observations include a named caller and test-context twin.
func TestJSONRootDateNullAPI68Named(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, testContext := range []bool{false, true} {
			route := "named"
			if testContext {
				route = "isTest"
			}
			t.Run(api+"/"+route, func(t *testing.T) {
				machine := New(nil)
				if testContext {
					machine.EnableTestContext()
				}
				if err := machine.RegisterClass(Class{Name: "OwnedDateProbe", APIVersion: api}); err != nil {
					t.Fatal(err)
				}
				body, err := CompileAnonymousWithOptions(`
String observed = 'returned';
try { JSON.deserialize('null', Date.class); }
catch (Exception problem) { observed = problem.getTypeName() + '|' + problem.getMessage(); }
System.assertEquals('System.JSONException|Invalid format: "null"', observed);
System.assertEquals('2026-11-03', String.valueOf((Date) JSON.deserialize('"2026-11-03"', Date.class)));
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: "OwnedDateProbe.check", ClassName: "OwnedDateProbe", APIVersion: api, ReturnType: "void", IsStatic: true, Program: body}); err != nil {
					t.Fatal(err)
				}
				caller, err := CompileAnonymousWithOptions(`OwnedDateProbe.check();`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(caller); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
