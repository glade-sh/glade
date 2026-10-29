package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native metadata/constructor contract: standard_recordtype_field_policy/
// org.tsv:16-25, API62/67. No explicit RecordTypeId field hides synthesis here.
func TestStandardFieldPolicyRecordTypeMetadataProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []string{"Account", "Contact", "Opportunity"} {
			for _, operation := range []string{"constructor", "setter"} {
				for _, route := range []string{"check", "anonymous", "named", "test-runner"} {
					t.Run(api+"/"+object+"/"+operation+"/"+route, func(t *testing.T) {
						root := t.TempDir()
						writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
						writeFile(t, filepath.Join(root, "force-app/main/default/objects", object, "recordTypes/Owned.recordType-meta.xml"), `<RecordType><fullName>Owned</fullName><label>Owned</label><active>true</active></RecordType>`)
						body := object + " row=new " + object + "(RecordTypeId=null);"
						if operation == "setter" {
							body = object + " row=new " + object + "();row.RecordTypeId=null;"
						}
						body += "System.assertEquals(null,row.RecordTypeId);System.assertEquals(true," + object + ".SObjectType.getDescribe().fields.getMap().containsKey('RecordTypeId'));"
						const class = "MetadataRecordTypeTest"
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
						analysis := sema.Analyze(index)
						if route == "anonymous" {
							analysis = sema.AnalyzeAnonymous(index, body, api)
						}
						if index.HasErrors() || analysis.HasErrors() {
							t.Fatalf("metadata compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
						}
						if route == "check" {
							return
						}
						if route == "test-runner" {
							for _, state := range []string{"cold", "warm"} {
								t.Run(state, func(t *testing.T) {
									run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
									if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
										t.Fatalf("metadata runner: %+v", run.Summary())
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
						for _, state := range []string{"cold", "warm"} {
							t.Run(state, func(t *testing.T) {
								if _, err := runner.execute(program); err != nil {
									t.Fatal(err)
								}
							})
						}
					})
				}
			}
		}
	}
}
