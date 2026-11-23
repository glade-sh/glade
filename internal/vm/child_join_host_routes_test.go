package vm_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/vm"
)

// Host calls replay the captured anonymous query; the invocation is a local contract.
func TestChildJoinHostRoutes(t *testing.T) {
	const fixture = "../soql/testdata/child_join_conjunction"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	wants := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) == 9 && row[0] == "combined_anon" && !strings.HasPrefix(row[8], "tainted:") {
			_, want, ok := strings.Cut(row[5], "OBS:")
			if !ok {
				t.Fatalf("bad row: %v", row)
			}
			wants[row[1]] = want
		}
	}
	source, err := os.ReadFile(filepath.Join(fixture, "combined.apex"))
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(source), "System.debug('OBS:'+result);", "return result;", 1)
	if body == string(source) {
		t.Fatal("capture observation not found")
	}
	for _, api := range []string{"62.0", "67.0"} {
		want, ok := wants[api]
		if !ok {
			t.Fatalf("missing native API %s", api)
		}
		for _, route := range []string{"lwc", "vf"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				index := capturedRequestIndex(t, fixture, api)
				org := apextest.OrgFromIndex(index)
				org.APIVersion = api
				machine := vm.New(nil)
				machine.SetOrg(&org)
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				method := vm.Method{Name: "ChildJoinHost.read", ClassName: "ChildJoinHost", APIVersion: api, ReturnType: "String", IsStatic: route == "lwc", Modifiers: []string{"public", "AuraEnabled"}, Program: program}
				if err := machine.RegisterClass(vm.Class{Name: "ChildJoinHost", Methods: map[string]vm.Method{"read": method}}); err != nil {
					t.Fatal(err)
				}
				if route == "lwc" {
					result, err := machine.InvokeLWCMethod("ChildJoinHost", "read", nil)
					if err != nil || !result.Success || result.ReturnValue != want {
						t.Fatalf("lwc: %v %v %v", err, result.Error, result.ReturnValue)
					}
					return
				}
				result, err := machine.InvokeVisualforceAction("ChildJoinHost", "read", "/apex/ChildJoinHost", nil)
				if err != nil || !result.Success || result.ReturnValue != want {
					t.Fatalf("vf: %v %v %v", err, result.Error, result.ReturnValue)
				}
			})
		}
	}
}
