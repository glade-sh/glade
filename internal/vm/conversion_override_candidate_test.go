package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// PENDING-ROWS: ordinary override dispatch is supported by owned test_runner
// rows J094/J095. The exact verifier-frame and namespace twins below are
// local candidate checks, not native conformance results.
func TestExecConversionOverrideVerifierCandidate(t *testing.T) {
	for _, namespace := range []string{"", "prism"} {
		t.Run("namespace="+namespace, func(t *testing.T) {
			machine := New(nil)
			org := storage.NewOrgState()
			org.Namespace = namespace
			machine.SetOrg(&org)
			qualify := func(name string) string {
				if namespace != "" {
					return namespace + "." + name
				}
				return name
			}
			override, err := CompileAnonymous("return 'prism.Orbit__sfdc_ApexStub.pulse(String)';")
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{
				Name: "framework_QualifiedMethod", Namespace: namespace,
				Methods: map[string]Method{"toString": {
					Name: qualify("framework_QualifiedMethod.toString"), ClassName: qualify("framework_QualifiedMethod"),
					ReturnType: "String", Access: "public", Program: override,
				}},
			}); err != nil {
				t.Fatal(err)
			}
			for _, caller := range []string{"framework_MethodVerifier", "NeutralRelay"} {
				if err := machine.RegisterClass(Class{Name: caller, Namespace: namespace}); err != nil {
					t.Fatal(err)
				}
				for _, method := range []struct{ name, body string }{
					{"direct", "return item.toString();"},
					{"implicit", "return '' + item;"},
					{"viaValue", "return String.valueOf(item);"},
					{"viaPattern", "return String.format('{0}',new List<Object>{item});"},
					{"viaRelay", "return NeutralRelay.viaValue(item);"},
					{"raise", "throw new DisplayException('Missing call: '+item+'.');"},
				} {
					program, err := CompileAnonymous(method.body)
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterMethod(Method{
						Name: qualify(caller + "." + method.name), ClassName: qualify(caller), IsStatic: true,
						ReturnType: "String", Access: "public", Params: []Param{{Name: "item", Type: "Object"}}, Program: program,
					}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := machine.RegisterClass(Class{Name: "DisplayException", Namespace: namespace, SuperClass: "Exception"}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymous(`
Object token=new framework_QualifiedMethod();
String wanted=token.toString();
System.assertEquals(wanted,''+token);
System.assertEquals(wanted,String.valueOf(token));
System.assertEquals(wanted,framework_MethodVerifier.direct(token));
System.assertEquals(wanted,framework_MethodVerifier.implicit(token));
System.assertEquals(wanted,framework_MethodVerifier.viaValue(token));
System.assertEquals(wanted,framework_MethodVerifier.viaPattern(token));
System.assertEquals(wanted,framework_MethodVerifier.viaRelay(token));
System.assertEquals(wanted,NeutralRelay.viaValue(token));
Boolean caught=false;
try { framework_MethodVerifier.raise(token); }
catch (DisplayException failure) {
 caught=true;
 System.assertEquals('Missing call: '+wanted+'.',failure.getMessage());
}

System.assert(caught);
System.assertEquals('literal',framework_MethodVerifier.viaValue('literal'));
`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(program, qualify("NeutralRelay")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Local dispatch controls: a declared override owns stub-like field text.
// C01/C04 capture literal override ownership. These exact field-built,
// same-org signatures remain PENDING-ROWS; they are not native assertions.
func TestExecGeneratedStubSignatureUsesDeclaredOverride(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "prism"
	machine.SetOrg(&org)
	override, err := CompileAnonymous("return typeName+'.'+methodName+String.valueOf(methodArgTypes);")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "framework_QualifiedMethod", Namespace: "prism",
		Fields: map[string]Field{
			"typeName":       {Name: "typeName", Type: "String", Access: "private"},
			"methodName":     {Name: "methodName", Type: "String", Access: "private"},
			"methodArgTypes": {Name: "methodArgTypes", Type: "List<Type>", Access: "private"},
		},
		Methods: map[string]Method{"toString": {
			Name: "prism.framework_QualifiedMethod.toString", ClassName: "prism.framework_QualifiedMethod",
			ReturnType: "String", Access: "public", Program: override,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, className := range []string{"framework_MethodVerifier", "NeutralRelay"} {
		if err := machine.RegisterClass(Class{Name: className, Namespace: "prism"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, method := range []struct{ name, body string }{
		{"implicit", "return ''+item;"},
		{"viaValue", "return String.valueOf(item);"},
		{"viaPattern", "return String.format('{0}',new List<Object>{item});"},
		{"viaRelay", "return NeutralRelay.viaValue(item);"},
	} {
		program, err := CompileAnonymous(method.body)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{
			Name: "prism.framework_MethodVerifier." + method.name, ClassName: "prism.framework_MethodVerifier",
			IsStatic: true, Access: "public", ReturnType: "String", Params: []Param{{Name: "item", Type: "Object"}}, Program: program,
		}); err != nil {
			t.Fatal(err)
		}
	}
	relay, err := CompileAnonymous("return String.valueOf(item);")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name: "prism.NeutralRelay.viaValue", ClassName: "prism.NeutralRelay", IsStatic: true,
		Access: "public", ReturnType: "String", Params: []Param{{Name: "item", Type: "Object"}}, Program: relay,
	}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ typeName, inside string }{
		{"prism.Pebble__sfdc_ApexStub", "prism.Pebble__sfdc_ApexStub.pulse(String)"},
		{"cedar.Pebble__sfdc_ApexStub", "cedar.Pebble__sfdc_ApexStub.pulse(String)"},
		{"Pebble__sfdc_ApexStub", "Pebble__sfdc_ApexStub.pulse(String)"},
	} {
		t.Run(test.typeName, func(t *testing.T) {
			token := Object("prism.framework_QualifiedMethod")
			token.Fields["typeName"] = String(test.typeName)
			token.Fields["methodName"] = String("pulse")
			token.Fields["methodArgTypes"] = List(platformScalar("Type", "String"))
			machine.Globals["token"] = token
			machine.Globals["inside"] = String(test.inside)
			machine.Globals["outside"] = String(test.typeName + ".pulse(String)")
			program, err := CompileAnonymous(`
System.assert(outside.equals(token.toString()));
System.assert(outside.equals(String.valueOf(token)));
System.assert(inside.equals(framework_MethodVerifier.implicit(token)));
System.assert(inside.equals(framework_MethodVerifier.viaValue(token)));
System.assert(inside.equals(framework_MethodVerifier.viaPattern(token)));
System.assert(inside.equals(framework_MethodVerifier.viaRelay(token)));
`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(program, "prism.NeutralRelay"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
