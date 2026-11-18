package vm_test

import (
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"testing"
)

// Local host consistency for the captured English text order. Scalar HAVING
// is not admitted by the public parser, so the hosts exercise grouped HAVING.
func TestHavingTextLocaleVMRoutes(t *testing.T) {
	const source = `Integer w = [SELECT Id FROM Account WHERE Name > '9'].size();
 List<AggregateResult> g = [SELECT Name, COUNT(Id) c FROM Account GROUP BY Name HAVING Name > '9'];
 System.assertEquals(w, g.size());
 System.assertEquals(w, Database.query('SELECT Name, COUNT(Id) c FROM Account GROUP BY Name HAVING Name > \'9\'').size());
 return 'ok';`
	for _, api := range []string{"62.0", "67.0"} {
		for _, locale := range []string{"en_US", "en_GB", "de_DE"} {
			for _, route := range []string{"anonymous", "istest", "lwc", "vf"} {
				t.Run(api+"/"+locale+"/"+route, func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "Id": {APIName: "Id", Type: storage.FieldID}}}, Records: map[storage.ID]storage.Record{"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue("001000000000001"), "Name": storage.StringValue("_")}}}}
					machine := vm.New(nil)
					machine.SetOrg(&org)
					machine.SetCurrentUser(storage.Record{Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(locale)}})
					if route == "istest" {
						machine.EnableTestContext()
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					method := vm.Method{Name: "HavingHost.check", ClassName: "HavingHost", APIVersion: api, ReturnType: "String", IsStatic: route != "vf", Modifiers: []string{"public", "AuraEnabled"}, Program: program}
					if err := machine.RegisterClass(vm.Class{Name: "HavingHost", Methods: map[string]vm.Method{"check": method}}); err != nil {
						t.Fatal(err)
					}
					switch route {
					case "lwc":
						result, err := machine.InvokeLWCMethod("HavingHost", "check", nil)
						if err != nil || !result.Success {
							t.Fatalf("%v: %v", err, result.Error)
						}
					case "vf":
						result, err := machine.InvokeVisualforceAction("HavingHost", "check", "/apex/HavingHost", nil)
						if err != nil || !result.Success {
							t.Fatalf("%v: %v", err, result.Error)
						}
					default:
						caller, err := vm.CompileAnonymousWithOptions("HavingHost.check();", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(caller); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}
