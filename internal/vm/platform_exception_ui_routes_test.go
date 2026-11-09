package vm

import "testing"

// Local routing controls preserve hosted entry behavior while anonymous and
// named restrictions are pinned by winter27_context/org.tsv.
func TestPlatformExceptionHostedEntryPreservesConstruction(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"aura", "lwc", "vf"} {
			for _, tc := range []struct{ name, expression string }{
				{"aura", "new System.AuraHandledException('probe')"},
				{"parameter", "new System.InvalidParameterValueException('value','probe')"},
				{"access", "new System.NoAccessException()"},
				{"data", "new System.NoDataFoundException()"},
				{"touch", "new System.TouchHandledException('probe')"},
			} {
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions("System.Exception value="+tc.expression+"; System.assertNotEquals(null,value);", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					method := Method{Name: "ExceptionUI.observe", ClassName: "ExceptionUI", ReturnType: "void", APIVersion: api, Program: program}
					if route == "vf" {
						if err := machine.RegisterClass(Class{Name: "ExceptionUI", Methods: map[string]Method{"observe": method}}); err != nil {
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
							result, err = machine.InvokeAuraAction("ExceptionUI", "observe", nil)
						case "lwc":
							result, err = machine.InvokeLWCMethod("ExceptionUI", "observe", nil)
						case "vf":
							result, err = machine.InvokeVisualforceAction("ExceptionUI", "observe", "/apex/ExceptionUI", nil)
						}
						if err != nil || !result.Success {
							t.Fatalf("invocation %d: result=%+v err=%v", i, result, err)
						}
					}
				})
			}
		}
	}
}
