package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE null Type argument: testdata/conformance/stdlib_type_depth/org.tsv:2-5
// NATIVE Type variable argument: testdata/conformance/stdlib_type_depth_neighbors/org.tsv:2-5
func TestTypeNullArgumentUIHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"lwc", "visualforce"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				code := `Type receiver = String.class; Boolean caught = false;
try { receiver.isAssignableFrom(null); } catch (NullPointerException e) {
  caught = true; System.assertEquals('Attempt to de-reference a null object', e.getMessage());
}
System.assert(caught);`
				if route == "visualforce" {
					code += "\nreturn new PageReference('/apex/TypeHost');"
				}
				program, err := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				org.APIVersion = api
				machine := vm.New(nil)
				machine.SetOrg(&org)
				method := vm.Method{Name: "TypeHost.run", ClassName: "TypeHost", ReturnType: "void", IsStatic: true, Modifiers: []string{"public", "AuraEnabled"}, Program: program, APIVersion: api}
				if route == "visualforce" {
					method.ReturnType = "PageReference"
					method.IsStatic = false
					method.Modifiers = []string{"public"}
				}
				if err := machine.RegisterClass(vm.Class{Name: "TypeHost", Methods: map[string]vm.Method{"run": method}}); err != nil {
					t.Fatal(err)
				}
				var got vm.UIInvocationResult
				if route == "lwc" {
					got, err = machine.InvokeLWCMethod("TypeHost", "run", nil)
				} else {
					got, err = machine.InvokeVisualforceAction("TypeHost", "run", "/apex/TypeHost", nil)
				}
				if err != nil || !got.Success {
					t.Fatalf("result=%+v err=%v", got, err)
				}
			})
		}
	}
}
