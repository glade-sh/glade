package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native field/constructor contract: standard_recordtype_field_policy/org.tsv
// physical lines 16-25 and ORG-RUN.txt (API62/67). This synthetic project has
// an explicit field declaration and no record-type metadata, reproducing the
// effective-schema constructor/setter mismatch rather than feature inference.
func TestStandardFieldPolicyConfiguredRecordTypeNativeProjectRouteTwins(t *testing.T) {
	metadata := map[string]string{
		"Account/fields/RecordTypeId.field-meta.xml": `<CustomField><fullName>RecordTypeId</fullName><type>Lookup</type><referenceTo>RecordType</referenceTo><relationshipName>RecordType</relationshipName></CustomField>`,
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "test-runner"} {
			for _, operation := range []struct{ name, body string }{
				{"constructor", "Account row=new Account(RecordTypeId=null); System.assertEquals(null,row.RecordTypeId);"},
				{"static-setter", "Account row=new Account(); row.RecordTypeId=null; System.assertEquals(null,row.RecordTypeId);"},
			} {
				t.Run(api+"/"+route+"/"+operation.name, func(t *testing.T) {
					runStandardFieldPolicyProvenanceProjectRoute(t, api, route, operation.body, metadata)
				})
			}
		}
	}
}

// P11/org.tsv:2577,2586,2602 and 12066,12075,12091 (and P11/ORG-RUN.txt)
// capture the configured membership. P0's API62/67 named maps omit the same
// fields. Replay both shapes through anonymous, named and @IsTest runtime.
func TestStandardFieldPolicyConfiguredCaseNativeProjectDescribeRoutes(t *testing.T) {
	configuredFields := map[string]string{
		"Case/fields/HasCommentsUnreadByOwner.field-meta.xml": `<CustomField><fullName>HasCommentsUnreadByOwner</fullName><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`,
		"Case/fields/IsVisibleInSelfService.field-meta.xml":   `<CustomField><fullName>IsVisibleInSelfService</fullName><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`,
		"Case/fields/SourceId.field-meta.xml":                 `<CustomField><fullName>SourceId</fullName><type>Lookup</type><referenceTo>EmailMessage</referenceTo><relationshipName>Source</relationshipName></CustomField>`,
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"configured", "bare-catalog"} {
			for _, route := range []string{"anonymous", "named", "test-runner"} {
				t.Run(api+"/"+shape+"/"+route, func(t *testing.T) {
					want := "true"
					metadata := configuredFields
					if shape == "bare-catalog" {
						want = "false"
						metadata = nil
					}
					body := "Map<String,Schema.SObjectField> fields=Case.SObjectType.getDescribe().fields.getMap();"
					for _, field := range []string{"HasCommentsUnreadByOwner", "IsVisibleInSelfService", "SourceId"} {
						body += "System.assertEquals(" + want + ",fields.containsKey('" + field + "'),'" + field + "');"
					}
					runStandardFieldPolicyProvenanceProjectRoute(t, api, route, body, metadata)
				})
			}
		}
	}
}

func runStandardFieldPolicyProvenanceProjectRoute(t *testing.T, api, route, body string, metadata map[string]string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	for relative, content := range metadata {
		writeFile(t, filepath.Join(root, "force-app/main/default/objects", relative), content)
	}
	const class = "ConfiguredFieldProvenance"
	if route != "anonymous" {
		source := "public class " + class + " { public static void sample(){" + body + "} }"
		if route == "test-runner" {
			source = "@IsTest private class " + class + " { @IsTest static void sample(){" + body + "} }"
		}
		path := filepath.Join(root, "force-app/main/default/classes", class+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	index := loadTestIndex(t, root)
	for _, object := range index.Objects {
		if object.Name == "Account" && len(object.RecordTypes) != 0 {
			t.Fatal("configured-field fixture unexpectedly has record types")
		}
	}
	analysis := sema.Analyze(index)
	if route == "anonymous" {
		analysis = sema.AnalyzeAnonymous(index, body, api)
	}
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("configured project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	if route == "test-runner" {
		for _, runRoute := range []string{"initial-runtime", "reused-runtime"} {
			t.Run(runRoute, func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("configured project runner: %+v", run.Summary())
				}
			})
		}
		return
	}
	if route == "named" {
		body = class + ".sample();"
	}
	program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	org := orgFromIndex(index)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	if _, err := runner.execute(program); err != nil {
		t.Fatal(err)
	}
}
