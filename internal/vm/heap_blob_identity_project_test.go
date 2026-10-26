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

func TestHeapUserBlobProjectAnonymousAndIsTestAPIRoutes(t *testing.T) {
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
				write(classes+"HeapIdentityProbe.cls", `public class HeapIdentityProbe {
                 public static Object held;
                 public static void observe() {
                  System.assert(Limits.getHeapSize() >= 100000, 'user Blob field payload');
                 }
                }`)
				write(classes+"HeapIdentityProbeTest.cls", `@IsTest private class HeapIdentityProbeTest { @IsTest static void observe() { HeapIdentityProbe.observe(); } }`)
				for _, name := range []string{"HeapIdentityProbe", "HeapIdentityProbeTest"} {
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
				// Salesforce reserves Blob in source. Register the VM class identity
				// directly while retaining the project runner and entry paths.
				if err := machine.RegisterClass(vm.Class{Name: "Blob", Access: "public", Fields: map[string]vm.Field{
					"value": {Name: "value", Type: "String", InitialValue: vm.String(strings.Repeat("x", 100000))},
				}}); err != nil {
					t.Fatal(err)
				}
				source := "HeapIdentityProbe.held = new Blob(); HeapIdentityProbe.observe();"
				if route == "isTest" {
					machine.EnableTestContext()
					if err := apextest.RegisterTestMethodForRequest(machine, index, "HeapIdentityProbeTest", "observe"); err != nil {
						t.Fatal(err)
					}
					source = "HeapIdentityProbe.held = new Blob(); HeapIdentityProbeTest.observe();"
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if route == "anonymous" {
					_, err = machine.Execute(program)
				} else {
					_, err = machine.ExecuteTestInClass(program, "HeapIdentityProbeTest")
				}
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
