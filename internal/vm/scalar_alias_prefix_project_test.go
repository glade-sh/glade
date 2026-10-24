package vm_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The retained nested-list result follows the native collection-refresh rows;
// the visit bound is a local cost invariant, not a Salesforce governor claim.
func TestScalarAliasPrefixProjectAnonymousAndIsTestAPIRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				write := func(name, body string) {
					path := filepath.Join(root, name)
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(body), 0644); err != nil {
						t.Fatal(err)
					}
				}
				write("sfdx-project.json", `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				const classes = "force-app/main/default/classes/"
				write(classes+"ScalarPrefixProbe.cls", `public class ScalarPrefixProbe {
     public static void append(List<String> leaf) { leaf.add('after'); }
     public static void observe() {
      List<String> leaf = new List<String>{'before'};
      List<Object> envelope = new List<Object>{`+strings.Repeat("'noise',", 200)+`leaf};
      append(leaf);
      List<String> retained = (List<String>)envelope[200];
      System.assertEquals(new List<String>{'before','after'},retained);
     }
    }`)
				write(classes+"ScalarPrefixProbeTest.cls", `@IsTest private class ScalarPrefixProbeTest { @IsTest static void observe() { ScalarPrefixProbe.observe(); } }`)
				for _, name := range []string{"ScalarPrefixProbe", "ScalarPrefixProbeTest"} {
					write(classes+name+".cls-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
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
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				org := apextest.OrgFromIndex(index)
				org.APIVersion = api
				machine := vm.New(nil)
				machine.SetOrg(&org)
				if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
					t.Fatal(err)
				}
				source := "ScalarPrefixProbe.observe();"
				if route == "isTest" {
					machine.EnableTestContext()
					if err := apextest.RegisterTestMethodForRequest(machine, index, "ScalarPrefixProbeTest", "observe"); err != nil {
						t.Fatal(err)
					}
					source = "ScalarPrefixProbeTest.observe();"
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				recorder := vm.NewPerfRecorder()
				machine.SetPerfRecorder(recorder)
				if route == "anonymous" {
					_, err = machine.Execute(program)
				} else {
					_, err = machine.ExecuteTestInClass(program, "ScalarPrefixProbeTest")
				}
				if err != nil {
					t.Fatal(err)
				}
				visits := recorder.Snapshot().ScopeAlias.RecursiveVisits
				t.Logf("scope alias visits=%d", visits)
				if visits > 160 {
					t.Fatalf("scalar prefix traversed recursively: visits=%d; want <=160", visits)
				}
			})
		}
	}
}
