package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func TestOwnedStubLabelPreservesTextAcrossContexts(t *testing.T) {
	// Local I01 regression: an ordinary override owns this literal text.
	// Genuine proxy labels have separate native display rows; this is not a
	// proxy. Native C/T caller/payload twins cover plain anonymous/test and
	// owner-namespace test. These exact same-prefix local twins, namespace
	// anonymous, and outside callers are still pending independent captures.
	const label = "lyx.Orbit__sfdc_ApexStub.pulse(String)"
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, namespace := range []string{"", "lyx"} {
			for _, payload := range []string{"framework_QualifiedMethod", "OwnedStubLabelProbe"} {
				for _, helper := range []string{"framework_MethodVerifier", "OwnedVerificationHelper"} {
					for _, route := range []string{"anonymous", "isTest"} {
						t.Run(api+"/namespace="+namespace+"/"+payload+"/"+helper+"/"+route, func(t *testing.T) {
							prefix := ""
							if namespace != "" {
								prefix = namespace + "."
							}
							machine := New(nil)
							org := storage.NewOrgState()
							org.Namespace, org.APIVersion = namespace, api
							machine.SetOrg(&org)
							compile := func(source string) ir.Program {
								t.Helper()
								program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								return program
							}
							register := func(class Class) {
								t.Helper()
								class.Namespace, class.APIVersion = namespace, api
								if err := machine.RegisterClass(class); err != nil {
									t.Fatal(err)
								}
							}
							register(Class{Name: payload, Access: "global", Methods: map[string]Method{
								"toString": {ReturnType: "String", Access: "global", Program: compile("return '" + label + "';")},
							}})
							register(Class{Name: helper, Access: "global", Methods: map[string]Method{
								"render": {ReturnType: "String", Access: "global", IsStatic: true, Params: []Param{{Name: "item", Type: "Object"}},
									Program: compile("String direct = item.toString(); System.assert(direct.equals('' + item)); System.assert(direct.equals(String.format('{0}', new List<Object>{item}))); return String.valueOf(item);")},
							}})
							register(Class{Name: "LabelContext", IsTest: true})
							source := prefix + payload + " typed = new " + prefix + payload + "(); Object boxed = typed; String expected = '" + label + "';"
							for _, receiver := range []string{"typed", "boxed"} {
								for _, expression := range []string{
									receiver + ".toString()",
									"'' + " + receiver,
									"String.valueOf(" + receiver + ")",
									prefix + helper + ".render(" + receiver + ")",
								} {
									source += "System.assert(expected.equals(" + expression + "));"
								}
							}
							program := compile(source)
							var err error
							if route == "isTest" {
								machine.EnableTestContext()
								_, err = machine.ExecuteTestInClass(program, prefix+"LabelContext")
							} else {
								_, err = machine.Execute(program)
							}
							if err != nil {
								t.Fatal(err)
							}
						})
					}
				}
			}
		}
	}
}
