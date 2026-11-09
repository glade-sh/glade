package vm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestB3DMLHeadersNativeNoRuleRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			dir := filepath.Join("testdata", "conformance", "b3_dml_headers", "api"+strings.TrimSuffix(api, ".0"), route)
			raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				key, want, ok := strings.Cut(line, "\t")
				if !ok {
					t.Fatal(line)
				}
				t.Run(api+"/"+route+"/"+key, func(t *testing.T) {
					var source string
					switch key {
					case "account_assignment":
						source = `Database.DMLOptions o=new Database.DMLOptions();o.AssignmentRuleHeader.useDefaultRule=true;Database.SaveResult r=Database.insert(new Account(Name='Synthetic'),o);System.assertEquals(true,r.isSuccess());`
					case "partial":
						source = `Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerOtherEmail=true;Database.SaveResult[] r=Database.insert(new List<Account>{new Account(Name='Synthetic'),new Account()},o);System.assertEquals(true,r[0].isSuccess());System.assertEquals(false,r[1].isSuccess());System.assertEquals(StatusCode.REQUIRED_FIELD_MISSING,r[1].getErrors()[0].getStatusCode());`
					default:
						bits := strings.Split(key, "_")
						if len(bits) != 3 {
							t.Fatal(key)
						}
						rows := map[string]string{"Account": "new Account(Name='Synthetic')", "Lead": "new Lead(LastName='Synthetic',Company='Synthetic')", "Case": "new Case(Subject='Synthetic',Status='New',Origin='Phone')"}
						flags := map[string]string{"user": "triggerUserEmail", "other": "triggerOtherEmail", "auto": "triggerAutoResponseEmail"}
						source = fmt.Sprintf("Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.%s=%s;Database.SaveResult r=Database.insert(%s,o);System.assertEquals(%s,r.isSuccess());", flags[bits[1]], bits[2], rows[bits[0]], strings.Split(want, ",")[0])
						if strings.HasPrefix(want, "false,") {
							source += `System.assertEquals(StatusCode.FIELD_INTEGRITY_EXCEPTION,r.getErrors()[0].getStatusCode());`
						}
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := storage.NewOrgState()
					org.APIVersion = api
					for _, object := range []string{"Account", "Lead", "Case"} {
						storage.EnsureStandardObject(&org, object)
					}
					machine := vm.New(nil)
					machine.SetOrg(&org)
					if route == "istest" {
						machine.EnableTestContext()
						err = machine.RegisterMethod(vm.Method{Name: "B3HeaderTest.observe", ClassName: "B3HeaderTest", IsStatic: true, Access: "public", Modifiers: []string{"isTest"}, APIVersion: api, Program: program})
						if err != nil {
							t.Fatal(err)
						}
						invoke, e := vm.CompileAnonymousWithOptions("B3HeaderTest.observe();", vm.CompileOptions{APIVersion: api})
						if e != nil {
							t.Fatal(e)
						}
						_, err = machine.ExecuteTestInClass(invoke, "B3HeaderTest")
					} else {
						_, err = machine.Execute(program)
					}
					if err != nil {
						t.Fatalf("native %s: %v", want, err)
					}
				})
			}
		}
	}
}
