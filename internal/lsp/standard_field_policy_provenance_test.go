package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// standard_recordtype_field_policy/org.tsv:16-25 and ORG-RUN.txt provide
// the native API62/67 reference and constructor contract. Explicit metadata
// makes that field available to both LSP constructor and static-setter routes.
func TestStandardFieldPolicyConfiguredRecordTypeLSPRouteTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, operation := range []struct{ name, body string }{
			{"constructor", "Account row=new Account(RecordTypeId=null); System.assertEquals(null,row.RecordTypeId);"},
			{"static-setter", "Account row=new Account(); row.RecordTypeId=null; System.assertEquals(null,row.RecordTypeId);"},
		} {
			t.Run(api+"/named/"+operation.name, func(t *testing.T) {
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
				classPath := "force-app/main/default/classes/ConfiguredRecordTypeProvenance.cls"
				write(classPath, "public class ConfiguredRecordTypeProvenance { public static void sample(){"+operation.body+"} }")
				write(classPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
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
				if index.HasErrors() {
					t.Fatalf("index: %v", index.Diagnostics)
				}
				handler := NewHandler(index)
				defer handler.parser.Close()
				for _, read := range []string{"first-diagnostics", "repeated-diagnostics"} {
					t.Run(read, func(t *testing.T) {
						diagnostics := handler.diagnosticsForDocument(uriFromPath(filepath.Join(root, classPath)))
						for _, item := range diagnostics {
							if item.Severity == diagnostic.Error {
								t.Fatalf("effective configured field rejected: %v", diagnostics)
							}
						}
					})
				}
			})
		}
	}
}
