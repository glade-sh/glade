package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// standard_recordtype_field_policy/org.tsv:16-25 captures the field's
// reference and constructor contract at API 62/67. This project explicitly
// declares that field, without record-type metadata: its effective definition
// must give the constructor and setter the same answer.
func TestStandardFieldPolicyConfiguredRecordTypeSemaRouteTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "is-test"} {
			for _, operation := range []struct{ name, body string }{
				{"constructor", "Account row=new Account(RecordTypeId=null); System.assertEquals(null,row.RecordTypeId);"},
				{"static-setter", "Account row=new Account(); row.RecordTypeId=null; System.assertEquals(null,row.RecordTypeId);"},
			} {
				t.Run(api+"/"+route+"/"+operation.name, func(t *testing.T) {
					root := t.TempDir()
					write := func(relative, content string) {
						t.Helper()
						path := filepath.Join(root, relative)
						if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(content), 0600); err != nil {
							t.Fatal(err)
						}
					}
					write("sfdx-project.json", `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
					write("force-app/main/default/objects/Account/fields/RecordTypeId.field-meta.xml", `<CustomField><fullName>RecordTypeId</fullName><type>Lookup</type><referenceTo>RecordType</referenceTo><relationshipName>RecordType</relationshipName></CustomField>`)
					if route != "anonymous" {
						source := "public class ConfiguredRecordTypeProvenance { public static void sample(){" + operation.body + "} }"
						if route == "is-test" {
							source = "@IsTest private class ConfiguredRecordTypeProvenance { @IsTest static void sample(){" + operation.body + "} }"
						}
						write("force-app/main/default/classes/ConfiguredRecordTypeProvenance.cls", source)
						write("force-app/main/default/classes/ConfiguredRecordTypeProvenance.cls-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					}
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					s, err := schema.LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					for _, object := range s.Objects {
						if object.Name == "Account" && len(object.RecordTypes) != 0 {
							t.Fatal("configured-field fixture unexpectedly has record types")
						}
					}
					index := typesys.Build(p, s)
					result := Analyze(index)
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, operation.body, api)
					}
					if index.HasErrors() || result.HasErrors() {
						t.Fatalf("effective configured field rejected: %v %v", index.Diagnostics, result.Diagnostics)
					}
				})
			}
		}
	}
}
