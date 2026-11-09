package vm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestB3FlagPairNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			path := filepath.Join("testdata/conformance/b3_flag_pairs", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strings.TrimSpace(string(raw)), "\n")[1:]
			if len(rows) != 48 {
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
					n, e := strconv.Atoi(parts[2])
					if e != nil || n < 0 || n > 3 {
						t.Fatal(key)
					}
					row := `new Lead(LastName='Synthetic',Company='Synthetic')`
					if parts[0] == "Case" {
						row = `new Case(Subject='Synthetic',Status='New',Origin='Phone',ContactId=c.Id)`
					}
					source := `Contact c=new Contact(LastName='Synthetic',Email='synthetic@example.invalid');insert c;` + fmt.Sprintf("SObject row=%s;Database.DMLOptions o=new Database.DMLOptions();", row)
					if parts[1] == "userother" || parts[1] == "userauto" {
						source += fmt.Sprintf("o.EmailHeader.triggerUserEmail=%t;", n >= 2)
					}
					if parts[1] == "userother" {
						source += fmt.Sprintf("o.EmailHeader.triggerOtherEmail=%t;", n%2 == 1)
					}
					if parts[1] == "otherauto" {
						source += fmt.Sprintf("o.EmailHeader.triggerOtherEmail=%t;", n >= 2)
					}
					if parts[1] != "userother" {
						source += fmt.Sprintf("o.EmailHeader.triggerAutoResponseEmail=%t;", n%2 == 1)
					}
					if parts[3] == "record" {
						source += `row.setOptions(o);insert row;System.assertNotEquals(null,row.Id);`
					} else {
						source += `Database.SaveResult r=Database.insert(row,o);System.assertEquals(true,r.isSuccess());`
					}
					program, e := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if e != nil {
						t.Fatal(e)
					}
					org := storage.NewOrgState()
					org.APIVersion = api
					for _, name := range []string{"Contact", "Lead", "Case"} {
						storage.EnsureStandardObject(&org, name)
					}
					machine := vm.New(nil)
					machine.SetOrg(&org)
					if route == "istest" {
						machine.EnableTestContext()
						e = machine.RegisterMethod(vm.Method{Name: "B3FlagTest.observe", ClassName: "B3FlagTest", IsStatic: true, Access: "public", Modifiers: []string{"isTest"}, APIVersion: api, Program: program})
						if e != nil {
							t.Fatal(e)
						}
						invoke, err := vm.CompileAnonymousWithOptions("B3FlagTest.observe();", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, e = machine.ExecuteTestInClass(invoke, "B3FlagTest")
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
