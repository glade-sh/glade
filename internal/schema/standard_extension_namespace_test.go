package schema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestLoadProjectStandardExtensionFieldUsesProjectNamespace(t *testing.T) {
	// Local metadata-provenance regression derived from NamespaceRoundTripProbe.
	// Native formula/JSON round-trip rows C01-C08 remain capture-pending.
	for _, namespace := range []string{"", "lyx"} {
		for _, inline := range []bool{false, true} {
			label := "loose"
			if inline {
				label = "inline"
			}
			t.Run(namespace+"/"+label, func(t *testing.T) {
				root := t.TempDir()
				base := filepath.Join(root, "force-app/main/default/objects/Account")
				p := project.Project{Namespace: namespace}
				fields := []string{
					`<fullName>OrbitEcho__c</fullName><type>Text</type><formula>"computed"</formula>`,
					`<fullName>vend__ForeignGlyph__c</fullName><type>Text</type>`,
					`<fullName>Description</fullName><type>Text</type>`,
				}
				if inline {
					path := filepath.Join(base, "Account.object-meta.xml")
					body := "<CustomObject>"
					for _, field := range fields {
						body += "<fields>" + field + "</fields>"
					}
					writeFile(t, path, body+"</CustomObject>")
					p.ObjectFiles = []string{path}
				} else {
					for i, name := range []string{"OrbitEcho__c", "vend__ForeignGlyph__c", "Description"} {
						path := filepath.Join(base, "fields", name+".field-meta.xml")
						writeFile(t, path, "<CustomField>"+fields[i]+"</CustomField>")
						p.FieldFiles = append(p.FieldFiles, path)
					}
				}
				loaded, err := LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				if len(loaded.Objects) != 1 || loaded.Objects[0].Name != "Account" {
					t.Fatalf("standard object identity changed: %#v", loaded.Objects)
				}
				actual := fieldsByName(loaded.Objects[0].Fields)
				name := "OrbitEcho__c"
				if namespace != "" {
					name = namespace + "__" + name
				}
				if actual[name].Formula != `"computed"` {
					t.Fatalf("owned formula field missing or changed: %#v", loaded.Objects[0].Fields)
				}
				if len(actual) != 3 || actual["vend__ForeignGlyph__c"].Name != "vend__ForeignGlyph__c" || actual["Description"].Name != "Description" {
					t.Fatalf("explicit foreign or standard field changed: %#v", actual)
				}
			})
		}
	}
}
