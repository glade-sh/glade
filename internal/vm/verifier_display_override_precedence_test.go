package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

// C01/C04 observe that verifier conversion retains an ordinary override's
// returned text. These field and inherited-method twins are local dispatch
// controls; the same-org field-built signature still needs native capture.
func TestVerifierDisplayStubSignatureCallsOverrideOnce(t *testing.T) {
	for _, namespace := range []string{"", "prism"} {
		for _, inherited := range []bool{false, true} {
			t.Run(namespace+"/inherited="+map[bool]string{false: "false", true: "true"}[inherited], func(t *testing.T) {
				prefix := ""
				if namespace != "" {
					prefix = namespace + "."
				}
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
				owner := "framework_QualifiedMethod"
				if inherited {
					owner = "OverrideGlyph"
				}
				method := Method{Name: prefix + owner + ".toString", ClassName: prefix + owner, ReturnType: "String", Program: compile("calls++; return 'prism.Orbit__sfdc_ApexStub.pulse(String)';")}
				if err := machine.RegisterClass(Class{Name: owner, Namespace: namespace, StaticFields: map[string]Field{"calls": {Name: "calls", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)}}, Methods: map[string]Method{"toString": method}}); err != nil {
					t.Fatal(err)
				}
				if inherited {
					if err := machine.RegisterClass(Class{Name: "framework_QualifiedMethod", Namespace: namespace, SuperClass: prefix + owner}); err != nil {
						t.Fatal(err)
					}
				}
				machine.currentMethod = Method{Name: prefix + "framework_MethodVerifier.raise", ClassName: prefix + "framework_MethodVerifier"}
				value := Object(prefix + "framework_QualifiedMethod")
				value.Fields["typeName"] = String("prism.Orbit__sfdc_ApexStub")
				value.Fields["methodName"] = String("pulse")
				value.Fields["methodArgTypes"] = List(platformScalar("Type", "String"))
				got, err := machine.displayString(value, &Result{})
				if err != nil || got != "prism.Orbit__sfdc_ApexStub.pulse(String)" {
					t.Fatalf("display=%q; error=%v", got, err)
				}
				if _, err := machine.Execute(compile("System.assertEquals(1, " + prefix + owner + ".calls);")); err != nil {
					t.Fatal(err)
				}
				direct, err := machine.callMethodWithReceiver(method, value, nil, &Result{})
				if err != nil || direct.Kind != ValueString || direct.Text != got {
					t.Fatalf("direct=%#v; error=%v", direct, err)
				}
				if _, err := machine.Execute(compile("System.assertEquals(2, " + prefix + owner + ".calls);")); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Dependency-owned methods deliberately bypass stringValueOfOverride and
// exercise its existing value-returning fallback. This is a local control.
func TestStringValueOfFallbackOverrideRunsOnce(t *testing.T) {
	for _, literal := range []string{"null", "'prism.Orbit__sfdc_ApexStub.pulse(String)'"} {
		t.Run(literal, func(t *testing.T) {
			body, err := CompileAnonymous("calls++; return " + literal + ";")
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			if err := machine.RegisterClass(Class{
				Name: "FallbackGlyph", Dependency: true,
				StaticFields: map[string]Field{"calls": {Name: "calls", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)}},
				Methods:      map[string]Method{"toString": {Name: "FallbackGlyph.toString", ClassName: "FallbackGlyph", ReturnType: "String", Program: body}},
			}); err != nil {
				t.Fatal(err)
			}
			if _, handled, err := machine.stringValueOfOverride(Object("FallbackGlyph"), &Result{}); handled || err != nil {
				t.Fatalf("dependency method should use fallback: handled=%v, error=%v", handled, err)
			}
			program, err := CompileAnonymous("Object value = new FallbackGlyph(); System.assertEquals(" + literal + ", String.valueOf(value)); System.assertEquals(1, FallbackGlyph.calls);")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
