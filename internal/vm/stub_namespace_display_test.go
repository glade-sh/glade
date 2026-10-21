package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func TestStubNamespaceDisplayMatchesNativeRows(t *testing.T) {
	// Native namespace_RAW_VALUE/RAW_STRING, PROVIDER_PROVIDERSPLIT,
	// CONCAT_FULL/FORMAT_FULL and AFTER_FULL/AFTER_SPLIT at APIs 62/67;
	// AFTER_FULL/AFTER_SPLIT at API 68. Plain API 62/67 rows are the twins.
	// Generated names are local; declared Type values retain their namespace.
	for _, tc := range []struct{ api, namespace string }{
		{"62.0", ""}, {"62.0", "prism"},
		{"67.0", ""}, {"67.0", "prism"}, {"68.0", "prism"},
	} {
		t.Run(tc.api+"/namespace="+tc.namespace, func(t *testing.T) {
			prefix := ""
			if tc.namespace != "" {
				prefix = tc.namespace + "."
			}
			machine := New(nil)
			org := storage.NewOrgState()
			org.Namespace, org.APIVersion = tc.namespace, tc.api
			machine.SetOrg(&org)
			machine.EnableTestContext()
			compile := func(source string) ir.Program {
				t.Helper()
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: tc.api})
				if err != nil {
					t.Fatal(err)
				}
				return program
			}
			register := func(class Class) {
				t.Helper()
				class.Namespace = tc.namespace
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			register(Class{Name: "StubDisplayProbe", IsTest: true})
			register(Class{Name: "Orbit", Methods: map[string]Method{
				"pulse": {ReturnType: "void", Params: []Param{{Name: "tone", Type: "String"}}, Program: compile("return;")},
			}})
			register(Class{Name: "EchoProvider", Interfaces: []string{"StubProvider"}, Fields: map[string]Field{
				"declaredType":  {Name: "declaredType", Type: "Type"},
				"generatedName": {Name: "generatedName", Type: "String"},
			}, Methods: map[string]Method{
				"handleMethodCall": {
					ReturnType: "Object",
					Params: []Param{
						{Name: "proxyGlyph", Type: "Object"}, {Name: "invokedGlyph", Type: "String"},
						{Name: "outputGlyph", Type: "Type"}, {Name: "inputGlyphs", Type: "List<Type>"},
						{Name: "labelGlyphs", Type: "List<String>"}, {Name: "valueGlyphs", Type: "List<Object>"},
					},
					Program: compile("generatedName = String.valueOf(proxyGlyph).substringBefore(':'); return null;"),
				},
			}})
			before := "Orbit__sfdc_ApexStub:[stubProvider=EchoProvider:[declaredType=" + prefix + "Orbit, generatedName=null]]"
			after := "Orbit__sfdc_ApexStub:[stubProvider=EchoProvider:[declaredType=" + prefix + "Orbit, generatedName=Orbit__sfdc_ApexStub]]"
			source := "EchoProvider provider = new EchoProvider(); provider.declaredType = Orbit.class; Orbit proxy = (Orbit)Test.createStub(Orbit.class, provider);"
			if tc.api != "68.0" {
				source += "System.assert('" + before + "'.equals(String.valueOf(proxy)), 'raw=' + String.valueOf(proxy));"
				source += "System.assert('" + before + "'.equals('' + proxy), 'concat=' + proxy);"
				source += "System.assert('" + before + "'.equals(String.format('{0}', new List<Object>{proxy})), 'format=' + String.format('{0}', new List<Object>{proxy}));"
			}
			source += "proxy.pulse('x'); System.assertEquals('" + prefix + "Orbit', provider.declaredType.getName());"
			source += "System.assertEquals('Orbit__sfdc_ApexStub', provider.generatedName);"
			source += "System.assert('" + after + "'.equals(String.valueOf(proxy)), 'after=' + String.valueOf(proxy));"
			if _, err := machine.ExecuteTestInClass(compile(source), prefix+"StubDisplayProbe"); err != nil {
				t.Fatal(err)
			}
			proxy := machine.Globals["proxy"]
			if token, ok := stubProxyTypeName(proxy); !ok || token != prefix+"Orbit" || proxy.Type != prefix+"Orbit" {
				t.Fatalf("stub type identity changed: %q / %q", token, proxy.Type)
			}
		})
	}
}

func TestStubVerifierLiteralAndDynamicMessagesAgree(t *testing.T) {
	// Local regression twins: fixed literal versus a stub-derived expectation.
	// Native METHOD_RECORD/VERIFY_FAILURE/IN_ORDER_FAILURE support the stub
	// spelling; ordinary verifier override dispatch remains capture-pending.
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "prism"
	machine.SetOrg(&org)
	machine.EnableTestContext()
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
		class.Namespace = "prism"
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	register(Class{Name: "StubDisplayProbe", IsTest: true})
	register(Class{Name: "Orbit"})
	register(Class{Name: "EchoProvider", Interfaces: []string{"StubProvider"}})
	register(Class{Name: "framework_QualifiedMethod", Fields: map[string]Field{
		"rendered": {Name: "rendered", Type: "String", Access: "private"},
	}, Constructors: []Method{{
		Name: "prism.framework_QualifiedMethod", ClassName: "prism.framework_QualifiedMethod", IsConstructor: true,
		Params:  []Param{{Name: "item", Type: "Object"}},
		Program: compile("rendered = String.valueOf(item).substringBefore(':') + '.pulse(String)';"),
	}}, Methods: map[string]Method{
		"toString": {ReturnType: "String", Program: compile("return rendered;")},
	}})
	register(Class{Name: "framework_MethodVerifier.DisplayException", SuperClass: "Exception"})
	register(Class{Name: "framework_MethodVerifier", Methods: map[string]Method{
		"raise": {ReturnType: "void", IsStatic: true, Params: []Param{{Name: "item", Type: "Object"}},
			Program: compile("throw new framework_MethodVerifier.DisplayException('Missing call: ' + item + '.');")},
	}})
	register(Class{Name: "framework_ApexMocks", Methods: map[string]Method{
		"extractTypeName": {ReturnType: "String", IsStatic: true, Params: []Param{{Name: "item", Type: "Object"}},
			Program: compile("return String.valueOf(item).substringBefore(':');")},
	}})
	for _, expected := range []string{
		"'Missing call: Orbit__sfdc_ApexStub.pulse(String).'",
		"'Missing call: ' + String.valueOf(proxy).substringBefore(':') + '.pulse(String).'",
	} {
		t.Run(expected, func(t *testing.T) {
			program := compile("Object proxy = Test.createStub(Orbit.class, new EchoProvider()); Object token = new framework_QualifiedMethod(proxy); String expected = " + expected + "; try { framework_MethodVerifier.raise(token); System.assert(false, 'Expected DisplayException'); } catch (framework_MethodVerifier.DisplayException fault) { System.assert(expected.equals(fault.getMessage()), fault.getMessage()); } System.assertEquals(String.valueOf(proxy).substringBefore(':'), framework_ApexMocks.extractTypeName(proxy));")
			if _, err := machine.ExecuteTestInClass(program, "prism.StubDisplayProbe"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
