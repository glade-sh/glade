package vm

import "testing"

// Local entry-route coverage for the fatal category captured in
// json_generator_closed/org.tsv; this does not assert hosted Salesforce output.
func TestJSONGeneratorClosedUIEntryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"aura", "lwc", "vf"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions("JSONGenerator g=JSON.createGenerator(false); g.close(); try { g.writeNull(); } catch (Exception e) { System.assert(false,'caught'); }", CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				method := Method{Name: "ClosedUI.observe", ClassName: "ClosedUI", ReturnType: "void", APIVersion: api, Program: program}
				if route == "vf" {
					if err := machine.RegisterClass(Class{Name: "ClosedUI", Methods: map[string]Method{"observe": method}}); err != nil {
						t.Fatal(err)
					}
				} else {
					method.IsStatic = true
					method.Modifiers = []string{"AuraEnabled"}
					if err := machine.RegisterMethod(method); err != nil {
						t.Fatal(err)
					}
				}
				for i := 0; i < 2; i++ {
					var result UIInvocationResult
					switch route {
					case "aura":
						result, err = machine.InvokeAuraAction("ClosedUI", "observe", nil)
					case "lwc":
						result, err = machine.InvokeLWCMethod("ClosedUI", "observe", nil)
					case "vf":
						result, err = machine.InvokeVisualforceAction("ClosedUI", "observe", "/apex/ClosedUI", nil)
					}
					if err != nil || result.Success || result.Error == nil || result.Error.Type != "System.UnexpectedException" {
						t.Fatalf("invocation %d: result=%+v err=%v", i, result, err)
					}
				}
			})
		}
	}
}
