package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardIndustriesSchemaNativeRoutes(t *testing.T) {
	f, err := os.Open("testdata/conformance/standard_industries_schema/org.tsv")
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
	count := 0
	for _, row := range rows[1:] {
		if row[0] != "combined" {
			continue
		}
		api, route, field := row[1], row[2], row[3]
		parts := strings.Split(field, ".")
		values := strings.Split(row[4], "|")
		if len(parts) != 2 || len(values) != 3 {
			t.Fatalf("invalid capture: %q", row)
		}
		count++
		t.Run(api+"/"+route+"/"+field, func(t *testing.T) {
			body := fmt.Sprintf(`Schema.DescribeFieldResult d = Schema.getGlobalDescribe().get('%s').getDescribe().fields.getMap().get('%s').getDescribe();
System.assertEquals('%s', String.valueOf(d.getType()).toUpperCase());
System.assertEquals(%s, d.isNillable());
System.assertEquals(%s, d.isDefaultedOnCreate());`, parts[0], parts[1], values[0], values[1], values[2])
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Enterprise","features":["PublicSectorAccess","OrderManagement","B2BCommerce","Communities","ContactsToMultipleAccounts"],"settings":{"orderManagementSettings":{"enableOrderManagement":true},"communitiesSettings":{"enableNetworksEnabled":true},"commerceSettings":{"commerceEnabled":true},"nameSettings":{"enableMiddleName":true}}}`)
			source := "@IsTest private class IndustriesSchemaTest { @IsTest static void checkFields() {\n" + body + "\n} }"
			if route == "named" {
				source = "public class IndustriesSchemaTest { public static void checkFields() {\n" + body + "\n} }"
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/IndustriesSchemaTest.cls"), source)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/IndustriesSchemaTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			if a := sema.Analyze(index); index.HasErrors() || a.HasErrors() {
				t.Fatalf("diagnostics: %v %v", index.Diagnostics, a.Diagnostics)
			}
			if route == "test-runner" {
				run := Run(index, Options{SelectedClasses: []string{"IndustriesSchemaTest"}, SelectedMethod: "checkFields", NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("named: %+v", run)
				}
				return
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if route == "named" {
				body = "IndustriesSchemaTest.checkFields();"
			}
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = runner.newMachine().Execute(program); err != nil {
				t.Fatalf("anonymous: %v", err)
			}
		})
	}
	if count != 36 {
		t.Fatalf("captured rows = %d, want 36", count)
	}
}

func TestStandardIndustriesSchemaNativeBehaviors(t *testing.T) {
	cases := []struct{ name, body string }{
		{"VisitDatetime", `Visit v = new Visit(PlannedVisitStartTime = Datetime.newInstance(2025,1,2,3,4,5)); System.assertNotEquals(null, v.PlannedVisitStartTime);`},
		{"RolesIncludes", `List<SObject> found = Database.query('SELECT Id FROM AccountContactRelation WHERE Roles INCLUDES (\'x\')'); System.assertEquals(0, found.size());`},
		{"AssessmentTaskMissing", `Database.SaveResult r = Database.insert(new AssessmentTask(), false); System.assertEquals(false, r.isSuccess()); System.assertEquals(StatusCode.REQUIRED_FIELD_MISSING, r.getErrors()[0].getStatusCode()); System.assert(r.getErrors()[0].getFields().contains('TaskType'));`},
		{"BusinessLicenseApplicationMissing", `Database.SaveResult r = Database.insert(new BusinessLicenseApplication(), false); System.assertEquals(false, r.isSuccess()); System.assertEquals(StatusCode.REQUIRED_FIELD_MISSING, r.getErrors()[0].getStatusCode()); System.assert(r.getErrors()[0].getFields().contains('Category'));`},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "test-runner"} {
			for _, c := range cases {
				t.Run(api+"/"+route+"/"+c.name, func(t *testing.T) {
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Enterprise","features":["PublicSectorAccess","OrderManagement","B2BCommerce","Communities","ContactsToMultipleAccounts"],"settings":{"orderManagementSettings":{"enableOrderManagement":true},"communitiesSettings":{"enableNetworksEnabled":true},"commerceSettings":{"commerceEnabled":true},"nameSettings":{"enableMiddleName":true}}}`)
					source := "@IsTest private class IndustriesBehaviorTest { @IsTest static void observe() {\n" + c.body + "\n} }"
					if route == "named" {
						source = "public class IndustriesBehaviorTest { public static void observe() {\n" + c.body + "\n} }"
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/classes/IndustriesBehaviorTest.cls"), source)
					writeFile(t, filepath.Join(root, "force-app/main/default/classes/IndustriesBehaviorTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
					index := loadTestIndex(t, root)
					if a := sema.Analyze(index); index.HasErrors() || a.HasErrors() {
						t.Fatalf("diagnostics: %v %v", index.Diagnostics, a.Diagnostics)
					}
					if route == "test-runner" {
						run := Run(index, Options{SelectedClasses: []string{"IndustriesBehaviorTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
						if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
							t.Fatalf("named: %+v", run)
						}
						return
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					body := c.body
					if route == "named" {
						body = "IndustriesBehaviorTest.observe();"
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err = runner.newMachine().Execute(program); err != nil {
						t.Fatalf("anonymous: %v", err)
					}
				})
			}
		}
	}
}

// Isolated profiles independently exercise the feature and setting controls.
func TestStandardIndustriesSchemaIsolatedRoutes(t *testing.T) {
	defs := map[string]string{
		"isolated-contacts":   `{"edition":"Enterprise","features":["ContactsToMultipleAccounts"]}`,
		"isolated-name":       `{"edition":"Enterprise","settings":{"nameSettings":{"enableMiddleName":true}}}`,
		"isolated-commerce":   `{"edition":"Enterprise","features":["B2BCommerce","Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true},"commerceSettings":{"commerceEnabled":true}}}`,
		"isolated-industries": `{"edition":"Enterprise","features":["PublicSectorAccess"]}`,
		"isolated-license":    `{"edition":"Enterprise","features":["B2BCommerce","Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`,
	}
	f, err := os.Open("testdata/conformance/standard_industries_schema/org.tsv")
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
	for _, api := range []string{"62.0", "67.0"} {
		for profile, definition := range defs {
			t.Run(api+"/"+profile, func(t *testing.T) {
				body := "Map<String,Schema.SObjectType> allTypes = Schema.getGlobalDescribe();\n"
				count := 0
				for _, row := range rows[1:] {
					if row[0] != profile || row[1] != api || row[2] != "anonymous" {
						continue
					}
					parts := strings.Split(row[3], ".")
					if len(parts) != 2 {
						t.Fatalf("bad row %q", row)
					}
					present := row[4] != "field_absent" && row[4] != "object_absent"
					body += fmt.Sprintf("System.assertEquals(%t, allTypes.containsKey('%s') && allTypes.get('%s').getDescribe().fields.getMap().containsKey('%s'));\n", present, parts[0], parts[0], parts[1])
					count++
				}
				if count != 3 {
					t.Fatalf("captured rows=%d", count)
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), definition)
				index := loadTestIndex(t, root)
				if a := sema.Analyze(index); index.HasErrors() || a.HasErrors() {
					t.Fatalf("diagnostics %v %v", index.Diagnostics, a.Diagnostics)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = runner.newMachine().Execute(program); err != nil {
					t.Fatalf("native profile %s: %v", profile, err)
				}
			})
		}
	}
}
