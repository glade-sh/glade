package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

// These are local dispatch regressions derived from the synthetic ConversionProbe.
// C01-C12/C22 and T08 capture conversion/message/relay behavior at APIs
// 62/67/68 on plain anonymous/test and owner namespace test. The twins below
// are local dispatch controls, not additional native observations.
func TestExecVerifierStringConversionsUseDeclaredOverride(t *testing.T) {
	for _, namespace := range []string{"", "prism"} {
		t.Run("namespace="+namespace, func(t *testing.T) {
			prefix := ""
			if namespace != "" {
				prefix = namespace + "."
			}
			payload := prefix + "framework_QualifiedMethod"
			verifier := prefix + "framework_MethodVerifier"
			exception := verifier + ".DisplayException"
			rendered := "prism.Orbit__sfdc_ApexStub.pulse(String)"
			machine := New(nil)
			org := storage.NewOrgState()
			org.Namespace = namespace
			machine.SetOrg(&org)
			compile := func(source string) ir.Program {
				t.Helper()
				program, err := CompileAnonymous(source)
				if err != nil {
					t.Fatal(err)
				}
				return program
			}
			register := func(class Class) {
				t.Helper()
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			register(Class{
				Name: "framework_QualifiedMethod", Namespace: namespace,
				Methods: map[string]Method{
					"toString": {Name: payload + ".toString", ClassName: payload, ReturnType: "String", Program: compile("return '" + rendered + "';")},
				},
			})
			register(Class{Name: "framework_MethodVerifier.DisplayException", Namespace: namespace, SuperClass: "Exception"})
			register(Class{
				Name: "RelayGlyph", Namespace: namespace,
				Methods: map[string]Method{
					"encodeGlyph": {ReturnType: "String", IsStatic: true, Params: []Param{{Name: "item", Type: "Object"}}, Program: compile("return String.valueOf(item);")},
				},
			})
			methods := map[string]Method{}
			for name, source := range map[string]string{
				"direct":     "return item.toString();",
				"implicit":   "return '' + item;",
				"viaValue":   "return String.valueOf(item);",
				"viaPattern": "return String.format('{0}', new List<Object>{item});",
				"viaRelay":   "return " + prefix + "RelayGlyph.encodeGlyph(item);",
				"raise":      "throw new " + exception + "('Missing call: ' + item + '.');",
			} {
				methods[name] = Method{ReturnType: "String", IsStatic: true, Params: []Param{{Name: "item", Type: "Object"}}, Program: compile(source)}
			}
			register(Class{Name: "framework_MethodVerifier", Namespace: namespace, Methods: methods})
			register(Class{Name: "ConversionProbe", Namespace: namespace})
			for _, expression := range []string{
				"token.toString()",
				"'' + token",
				verifier + ".direct(token)",
				verifier + ".implicit(token)",
				verifier + ".viaValue(token)",
				verifier + ".viaPattern(token)",
				verifier + ".viaRelay(token)",
			} {
				t.Run(expression, func(t *testing.T) {
					program := compile("Object token = new " + payload + "(); System.assertEquals(token.toString(), " + expression + ");")
					if _, err := machine.ExecuteInClass(program, prefix+"ConversionProbe"); err != nil {
						t.Fatal(err)
					}
				})
			}
			t.Run("exceptionMessage", func(t *testing.T) {
				program := compile("Object token = new " + payload + "(); try { " + verifier + ".raise(token); System.assert(false, 'Expected DisplayException'); } catch (" + exception + " fault) { System.assertEquals('Missing call: ' + token.toString() + '.', fault.getMessage()); }")
				if _, err := machine.ExecuteInClass(program, prefix+"ConversionProbe"); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

func TestDisplayStringVerifierOverridePreservesNamespace(t *testing.T) {
	// Compare conversion with explicit execution of the same synthetic method.
	// Field visibility is a local dispatch twin, not a native display oracle.
	for _, modifiers := range [][]string{{"private"}, {"public", "final"}} {
		t.Run(modifiers[0], func(t *testing.T) {
			program, err := CompileAnonymous("return typeName + '.' + methodName + String.valueOf(methodArgTypes);")
			if err != nil {
				t.Fatal(err)
			}
			method := Method{Name: "prism.framework_QualifiedMethod.toString", ClassName: "prism.framework_QualifiedMethod", ReturnType: "String", Program: program}
			machine := New(nil)
			org := storage.NewOrgState()
			org.Namespace = "prism"
			machine.SetOrg(&org)
			if err := machine.RegisterClass(Class{
				Name: "framework_QualifiedMethod", Namespace: "prism",
				Fields: map[string]Field{
					"typeName":       {Name: "typeName", Type: "String", Modifiers: modifiers},
					"methodName":     {Name: "methodName", Type: "String", Modifiers: modifiers},
					"methodArgTypes": {Name: "methodArgTypes", Type: "List<Type>", Modifiers: modifiers},
				},
				Methods: map[string]Method{"toString": method},
			}); err != nil {
				t.Fatal(err)
			}
			machine.currentMethod = Method{Name: "prism.framework_MethodVerifier.raise", ClassName: "prism.framework_MethodVerifier"}
			value := Object("prism.framework_QualifiedMethod")
			value.Fields["typeName"] = String("prism.Orbit__sfdc_ApexStub")
			value.Fields["methodName"] = String("pulse")
			value.Fields["methodArgTypes"] = List(platformScalar("Type", "String"))
			direct, err := machine.callMethodWithReceiver(method, value, nil, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			got, err := machine.displayString(value, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			if direct.Kind != ValueString || got != direct.Text {
				t.Fatalf("display = %q, direct override = %#v", got, direct)
			}
		})
	}
}
