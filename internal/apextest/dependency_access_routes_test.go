package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Deployment parity is captured in visualforce/testdata/dependency_access/org.tsv
// and visualforce/testdata/dependency_access_namespaced/org.tsv.
// These local adapter checks ensure accepted consumer pages reach every runtime
// registration path; they do not claim native browser observations.
func TestDependencyAccessPageRegistrationRoutes(t *testing.T) {
	runDependencyAccessPageRegistrationRoutes(t, "dependency_access", "BaseAccessController", "BaseAccessPage")
}

func TestNamespacedDependencyAccessPageRegistrationRoutes(t *testing.T) {
	runDependencyAccessPageRegistrationRoutes(t, "dependency_access_namespaced", "gladens001__VfDepAccessProbeClass", "gladens001__VfDepAccessProbePage")
}

func runDependencyAccessPageRegistrationRoutes(t *testing.T, fixture, class, page string) {
	t.Helper()
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			InvalidateRuntimeCaches()
			t.Cleanup(InvalidateRuntimeCaches)
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join("../visualforce/testdata", fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				return os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "62.0", api)), 0644)
			}); err != nil {
				t.Fatal(err)
			}
			consumer := filepath.Join(root, "consumer")
			writeFile(t, filepath.Join(consumer, "src/permissionsets/Access.permissionset-meta.xml"), fmt.Sprintf(`<PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><classAccesses><apexClass>%s</apexClass><enabled>true</enabled></classAccesses><pageAccesses><apexPage>%s</apexPage><enabled>true</enabled></pageAccesses><label>Dependency access probe</label></PermissionSet>`, class, page))
			// These adapter checks prove registration; URL casing is not a captured behavior.
			code := fmt.Sprintf(`System.assertEquals('/apex/consumeraccesspage',Page.ConsumerAccessPage.getUrl()); System.assertEquals('/apex/%s',Page.%s.getUrl().toLowerCase());`, strings.ToLower(page), page)
			writeFile(t, filepath.Join(consumer, "src/classes/DependencyAccessTest.cls"), "@IsTest private class DependencyAccessTest { @IsTest static void pages() { "+code+" } }")
			writeFile(t, filepath.Join(consumer, "src/classes/DependencyAccessTest.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, consumer)
			program, err := vm.CompileAnonymous(code)
			if err != nil {
				t.Fatal(err)
			}
			for _, route := range []struct {
				name     string
				register func(*vm.VM, typesys.Index) error
			}{
				{"project", RegisterProjectRuntime},
				{"request", RegisterProjectRuntimeForRequest},
				{"uncached_request", RegisterUncachedProjectRuntimeForRequest},
			} {
				t.Run(route.name, func(t *testing.T) {
					machine := vm.New(nil)
					if err := route.register(machine, index); err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
			for i := 0; i < 2; i++ {
				t.Run(fmt.Sprintf("cached_request_%d", i), func(t *testing.T) {
					machine, err := NewProjectRuntimeForRequest(index)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
				t.Run(fmt.Sprintf("project_test_%d", i), func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"DependencyAccessTest"}, Parallelism: 1})
					if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
						t.Fatalf("summary=%+v suites=%+v", got, run.Suites)
					}
				})
			}
		})
	}
}
