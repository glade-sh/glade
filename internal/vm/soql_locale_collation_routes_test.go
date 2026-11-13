package vm_test

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// The native value set contains both ä and z. Their observed order distinguishes
// root, Swedish tailoring and the English fallback without a language heuristic.
func TestLocaleCollationVMUserAndHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest", "lwc", "vf"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{
					"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue("001000000000001"), "Name": storage.StringValue("ä")}},
					"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue("001000000000002"), "Name": storage.StringValue("z")}},
				}}
				machine := vm.New(nil)
				machine.SetOrg(&org)
				const source = `String text=''; for(Account a:[SELECT Name FROM Account ORDER BY Name]) text+=a.Name;
   String dynamicText=''; for(Account a:Database.query('SELECT Name FROM Account ORDER BY Name')) dynamicText+=a.Name;
   System.assertEquals(text,dynamicText);
   List<AggregateResult> mm=[SELECT MIN(Name) mn,MAX(Name) mx FROM Account];
   return UserInfo.getLocale()+'|'+text+'|'+String.valueOf(mm[0].get('mn'))+'|'+String.valueOf(mm[0].get('mx'));`
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				method := vm.Method{Name: "LocaleHost.read", ClassName: "LocaleHost", APIVersion: api, ReturnType: "String", IsStatic: route != "vf", Modifiers: []string{"public", "AuraEnabled"}, Program: program}
				if err := machine.RegisterClass(vm.Class{Name: "LocaleHost", Methods: map[string]vm.Method{"read": method}}); err != nil {
					t.Fatal(err)
				}
				caller, err := vm.CompileAnonymousWithOptions("System.debug(LocaleHost.read());", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				for _, tc := range []struct{ locale, language, want string }{{"de_DE", "en_US", "äz|ä|z"}, {"sv_SE", "en_US", "zä|z|ä"}, {"af_ZA", "de", "zä|z|ä"}, {"de_DE", "de", "äz|ä|z"}} {
					machine.SetCurrentUser(storage.Record{Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(tc.locale), "LanguageLocaleKey": storage.StringValue(tc.language)}})
					if route == "istest" {
						machine.EnableTestContext()
					}
					want := tc.locale + "|" + tc.want
					switch route {
					case "lwc":
						result, err := machine.InvokeLWCMethod("LocaleHost", "read", nil)
						if err != nil || !result.Success || result.ReturnValue != want {
							t.Fatalf("%s: %v, %v, %v", tc.locale, err, result.Error, result.ReturnValue)
						}
					case "vf":
						result, err := machine.InvokeVisualforceAction("LocaleHost", "read", "/apex/LocaleHost", nil)
						if err != nil || !result.Success || result.ReturnValue != want {
							t.Fatalf("%s: %v, %v, %v", tc.locale, err, result.Error, result.ReturnValue)
						}
					default:
						result, err := machine.Execute(caller)
						if err != nil || !strings.Contains(strings.Join(result.Debug, "\n"), want) {
							t.Fatalf("%s: %v, %v", tc.locale, err, result.Debug)
						}
					}
				}
			})
		}
	}
}
