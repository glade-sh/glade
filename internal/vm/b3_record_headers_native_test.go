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

func TestB3RecordHeadersNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			path := filepath.Join("testdata/conformance/b3_record_headers", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strings.TrimSpace(string(raw)), "\n")[1:]
			if len(rows) != 36 {
				t.Fatal("incomplete native capture")
			}
			for _, line := range rows {
				key, want, ok := strings.Cut(line, "\t")
				if !ok || want != "true" {
					t.Fatal(line)
				}
				t.Run(api+"/"+route+"/"+key, func(t *testing.T) {
					parts := strings.Split(key, "_")
					if len(parts) != 4 {
						t.Fatal(key)
					}
					row := map[string]string{"Account": "new Account(Name='Synthetic')", "Lead": "new Lead(LastName='Synthetic',Company='Synthetic')", "Case": "new Case(Subject='Synthetic',Status='New',Origin='Phone',ContactId=c.Id)"}[parts[0]]
					flag := map[string]string{"user": "triggerUserEmail", "other": "triggerOtherEmail", "auto": "triggerAutoResponseEmail"}[parts[1]]
					source := `Contact c=new Contact(LastName='Synthetic',Email='synthetic@example.invalid');insert c;` + fmt.Sprintf("SObject row=%s;Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.%s=%s;row.setOptions(o);", row, flag, parts[2])
					if parts[3] == "keyword" {
						source += `insert row;System.assertNotEquals(null,row.Id);`
					} else {
						source += `Database.SaveResult r=Database.insert(row,false);System.assertEquals(true,r.isSuccess());`
					}
					program, e := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if e != nil {
						t.Fatal(e)
					}
					org := storage.NewOrgState()
					org.APIVersion = api
					for _, name := range []string{"Account", "Contact", "Lead", "Case"} {
						storage.EnsureStandardObject(&org, name)
					}
					machine := vm.New(nil)
					machine.SetOrg(&org)
					if route == "istest" {
						machine.EnableTestContext()
						e = machine.RegisterMethod(vm.Method{Name: "B3RecordTest.observe", ClassName: "B3RecordTest", IsStatic: true, Access: "public", Modifiers: []string{"isTest"}, APIVersion: api, Program: program})
						if e != nil {
							t.Fatal(e)
						}
						invoke, err := vm.CompileAnonymousWithOptions("B3RecordTest.observe();", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, e = machine.ExecuteTestInClass(invoke, "B3RecordTest")
					} else {
						_, e = machine.Execute(program)
					}
					if e != nil {
						t.Fatalf("native %s: %v", want, e)
					}
				})
			}
		}
	}
}
