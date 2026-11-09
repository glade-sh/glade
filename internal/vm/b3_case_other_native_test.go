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

func TestB3CaseOtherEmailContactNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			path := filepath.Join("testdata/conformance/b3_case_other", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			notification, err := os.ReadFile(filepath.Join(filepath.Dir(path), "notification.tsv"))
			if err != nil || !strings.Contains(string(notification), "case_other_valid_activity\ttrue,0,0,none") {
				t.Fatalf("missing native activity observation: %v", err)
			}
			for i, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				key, want, ok := strings.Cut(line, "\t")
				if !ok {
					t.Fatal(line)
				}
				t.Run(api+"/"+route+"/"+key, func(t *testing.T) {
					prefix := "Id contactId=null;"
					if i > 0 {
						email := "null"
						if i == 2 {
							email = "'synthetic@example.invalid'"
						}
						prefix += fmt.Sprintf("Contact c=new Contact(LastName='Synthetic',Email=%s);insert c;contactId=c.Id;", email)
					}
					source := prefix + `Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerOtherEmail=true;Database.SaveResult r=Database.insert(new Case(Subject='Synthetic',Status='New',Origin='Phone',ContactId=contactId),o);`
					if strings.HasPrefix(want, "false,") {
						parts := strings.SplitN(want, ",", 3)
						if len(parts) != 3 {
							t.Fatalf("missing native error message: %s", want)
						}
						source += `System.assertEquals(false,r.isSuccess());System.assertEquals(StatusCode.FIELD_INTEGRITY_EXCEPTION,r.getErrors()[0].getStatusCode());System.assertEquals('` + parts[2] + `',r.getErrors()[0].getMessage());`
					} else {
						source += `System.assertEquals(true,r.isSuccess());Id caseId=r.Id;System.assertEquals(0,[SELECT COUNT() FROM Task WHERE WhatId=:caseId]);System.assertEquals(0,[SELECT COUNT() FROM EmailMessage WHERE RelatedToId=:caseId]);`
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := storage.NewOrgState()
					org.APIVersion = api
					for _, object := range []string{"Case", "Contact", "Task", "EmailMessage"} {
						storage.EnsureStandardObject(&org, object)
					}
					machine := vm.New(nil)
					machine.SetOrg(&org)
					if route == "istest" {
						machine.EnableTestContext()
						err = machine.RegisterMethod(vm.Method{Name: "B3CaseOtherTest.observe", ClassName: "B3CaseOtherTest", IsStatic: true, Access: "public", Modifiers: []string{"isTest"}, APIVersion: api, Program: program})
						if err != nil {
							t.Fatal(err)
						}
						invoke, e := vm.CompileAnonymousWithOptions("B3CaseOtherTest.observe();", vm.CompileOptions{APIVersion: api})
						if e != nil {
							t.Fatal(e)
						}
						_, err = machine.ExecuteTestInClass(invoke, "B3CaseOtherTest")
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
