package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Native outcomes: testdata/conformance/named_argument_describe/org.tsv:2,4.
func TestNamedArgumentDescribeProjectAndRequest(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "NamedDescribe.cls"), `global class NamedDescribe {
                global static void observe(String key) {
                    Map<String,Schema.SObjectField> beforeFields = Schema.SObjectType.ObjectPermissions.fields.getMap();
                    Integer countBefore = beforeFields.size();
                    System.assertEquals(false, beforeFields.containsKey('key'));
                    ObjectPermissions row = new ObjectPermissions(SobjectType = (key == 'a' ? 'Account' : 'Contact'), PermissionsRead = true);
                    System.assertEquals(key == 'a' ? 'Account' : 'Contact', row.SobjectType);
                    Map<String,Schema.SObjectField> afterFields = Schema.SObjectType.ObjectPermissions.fields.getMap();
                    System.assertEquals(false, afterFields.containsKey('key'));
                    System.assertEquals(false, afterFields.containsKey('getMap'));
                    System.assertEquals(countBefore, afterFields.size());
                }
            }`)
			writeFile(t, filepath.Join(classes, "NamedDescribeTest.cls"), `@IsTest private class NamedDescribeTest {
                @IsTest static void first() { NamedDescribe.observe('a'); }
                @IsTest static void second() { NamedDescribe.observe('b'); }
            }`)
			for _, name := range []string{"NamedDescribe", "NamedDescribeTest"} {
				writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			index := loadTestIndex(t, root)
			for range 2 {
				run := Run(index, Options{Parallelism: 1, NoDiskCache: true})
				if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 {
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem != nil {
								t.Logf("%s.%s: %+v", c.ClassName, c.MethodName, *c.Problem)
							}
						}
					}
					t.Fatalf("project outcomes=%+v", got)
				}
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			program, err := vm.CompileAnonymousWithOptions("NamedDescribe.observe('a'); NamedDescribe.observe('b');", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err := runner.newMachine().Execute(program); err != nil {
					t.Fatalf("request route: %v", err)
				}
			}
		})
	}
}
