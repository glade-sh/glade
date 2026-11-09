package vm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native shadowed-root results: q3share/org.tsv:16-19.
func TestQ3ShadowedRootsProjectHosts(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"lwc", "vf"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				version := strings.TrimSuffix(api, ".0")
				helper := "Q3Shadow" + version
				files := map[string]string{
					"sfdx-project.json": fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api),
					"force-app/main/default/classes/Q3ShadowHost.cls":          "public class Q3ShadowHost { @AuraEnabled public static String lwc(){ " + helper + ".run(); return 'pass'; } public String vf(){ " + helper + ".run(); return 'pass'; } }",
					"force-app/main/default/classes/Q3ShadowHost.cls-meta.xml": "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>",
				}
				for _, ext := range []string{".cls", ".cls-meta.xml"} {
					b, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/q3share/shadow/classes", helper+ext))
					if err != nil {
						t.Fatal(err)
					}
					files["force-app/main/default/classes/"+helper+ext] = string(b)
				}
				for path, body := range files {
					dst := filepath.Join(root, path)
					if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(dst, []byte(body), 0644); err != nil {
						t.Fatal(err)
					}
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				s, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, s)
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				machine := vm.New(nil)
				org := apextest.OrgFromIndex(index)
				org.APIVersion = api
				machine.SetOrg(&org)
				if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
					t.Fatal(err)
				}
				var result vm.UIInvocationResult
				if route == "lwc" {
					result, err = machine.InvokeLWCMethod("Q3ShadowHost", "lwc", nil)
				} else {
					result, err = machine.InvokeVisualforceAction("Q3ShadowHost", "vf", "/apex/Q3ShadowHost", nil)
				}
				if err != nil || !result.Success || result.ReturnValue != "pass" {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			})
		}
	}
}
