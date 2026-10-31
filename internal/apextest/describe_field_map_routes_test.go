package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Local ownership controls through actual test and request compilers. Native
// reads remain pinned by ../vm/testdata/describe_sharing/org.tsv:2,4,5,7;
// getMap clear and its empty-copy boundary match schema_field_map_copy/org.tsv C54–C60.
func TestFieldMapReadProjectAndRequestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "zzns"} {
			t.Run(api+"/"+namespace, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q,"namespace":%q}`, api, namespace))
				classes := filepath.Join(root, "force-app/main/default/classes")
				writeFile(t, filepath.Join(classes, "FieldMapOwner.cls"), `global class FieldMapOwner {
public static Map<String,Schema.SObjectField> stored;
public Map<String,Schema.SObjectField> member;
global static void observe() {
    Map<String,Schema.SObjectField> first = Schema.SObjectType.Account.fields.getMap();
    Map<String,Schema.SObjectField> second = Schema.SObjectType.Account.fields.getMap();
    Map<String,Schema.SObjectField> alias = first;
    Map<String,Schema.SObjectField> copied = first.clone();
    FieldMapOwner.stored = first;
    FieldMapOwner instance = new FieldMapOwner(); instance.member = first;
    Integer beforeSize = first.size();
    System.assertEquals('Account Name', first.get('Name').getDescribe().getLabel());
    Set<String> keys = first.keySet(); keys.remove('name');
    Set<String> otherKeys = first.keySet(); otherKeys.removeAll(new Set<String>{'name'});
    Set<String> retainedKeys = first.keySet(); retainedKeys.retainAll(new Set<String>{'id'});
    Set<String> emptyKeys = first.keySet(); emptyKeys.clear();
    System.assertEquals(beforeSize, first.size());
    System.assertEquals(beforeSize, second.size());
    System.assertEquals(beforeSize, alias.size());
    System.assertEquals(beforeSize, copied.size());
    System.assertEquals(beforeSize, FieldMapOwner.stored.size());
    System.assertEquals(beforeSize, instance.member.size());
    System.assertEquals('Name', first.get('Name').getDescribe().getName());
    Schema.SObjectField scalar = first.get('Name');
    System.assertEquals('Name', scalar.getDescribe().getName());
    System.assertEquals(true, scalar.getDescribe().isNameField());
    calls = 0;
    System.assertEquals('Name', token().getDescribe().getName());
    System.assertEquals(1, calls);
}
public static Integer calls = 0;
public static Schema.SObjectField token() { calls++; return Account.Name; }
global static void clearMap() {
    Map<String,Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
    fields.clear();
    System.assertEquals(0, fields.size());
    System.assertEquals(0, fields.clone().size());
}
}`)
				writeFile(t, filepath.Join(classes, "FieldMapOwnerTest.cls"), `@IsTest private class FieldMapOwnerTest {
@IsTest static void first() { FieldMapOwner.observe(); }
@IsTest static void next() { FieldMapOwner.observe(); }
@IsTest static void clearMap() { FieldMapOwner.clearMap(); }
}`)
				for _, name := range []string{"FieldMapOwner", "FieldMapOwnerTest"} {
					writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				}
				index := loadTestIndex(t, root)
				for range 2 {
					run := Run(index, Options{Parallelism: 1, NoDiskCache: true})
					if got := run.Summary(); got.Total != 3 || got.Passed != 3 || got.Failed != 0 {
						for _, suite := range run.Suites {
							for _, test := range suite.Cases {
								if test.Problem != nil {
									t.Logf("%s.%s: %+v", test.ClassName, test.MethodName, *test.Problem)
								}
							}
						}
						t.Fatalf("project outcomes=%+v suites=%+v", got, run.Suites)
					}
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
				program, err := vm.CompileAnonymousWithOptions("FieldMapOwner.observe();", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				for range 2 {
					if _, err := runner.newMachine().Execute(program); err != nil {
						t.Fatalf("request route: %v", err)
					}
				}
				clear, err := vm.CompileAnonymousWithOptions("FieldMapOwner.clearMap();", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := runner.newMachine().Execute(clear); err != nil {
					t.Fatalf("request clear: %v", err)
				}
			})
		}
	}
}
