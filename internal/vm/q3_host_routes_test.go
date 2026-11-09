package vm

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The operations and expected results are captured in q3semijoin/org.tsv:2-5
// and q3share/org.tsv:2-5. These tests exercise local host entry points.
func TestQ3CapturedHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, path string }{{"child", "../apextest/testdata/conformance/q3semijoin/seeded.apex"}, {"share", "../apextest/testdata/conformance/q3share/standard.apex"}} {
			for _, route := range []string{"lwc", "vf"} {
				t.Run(api+"/"+tc.name+"/"+route, func(t *testing.T) {
					body, err := os.ReadFile(tc.path)
					if err != nil {
						t.Fatal(err)
					}
					code := string(body)
					if route == "vf" {
						code += "\nreturn new PageReference('/apex/Q3Host');"
					}
					program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := storage.NewOrgState()
					org.APIVersion = api
					for _, name := range []string{"Account", "Contact", "Opportunity", "OpportunityContactRole", "AccountShare", "OpportunityShare"} {
						storage.EnsureStandardObject(&org, name)
					}
					machine := New(nil)
					machine.SetOrg(&org)
					method := Method{Name: "Q3Host.run", ClassName: "Q3Host", ReturnType: "void", IsStatic: true, Modifiers: []string{"public", "AuraEnabled"}, Program: program}
					if route == "vf" {
						method.ReturnType = "PageReference"
						method.IsStatic = false
						method.Modifiers = []string{"public"}
					}
					if err := machine.RegisterClass(Class{Name: "Q3Host", Methods: map[string]Method{"run": method}}); err != nil {
						t.Fatal(err)
					}
					var got UIInvocationResult
					if route == "lwc" {
						got, err = machine.InvokeLWCMethod("Q3Host", "run", nil)
					} else {
						got, err = machine.InvokeVisualforceAction("Q3Host", "run", "/apex/Q3Host", nil)
					}
					if err != nil || !got.Success {
						t.Fatalf("result=%+v err=%v", got, err)
					}
				})
			}
		}
	}
}
