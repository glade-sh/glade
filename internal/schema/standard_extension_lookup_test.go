package schema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestLoadProjectStandardExtensionLookupUsesProjectNamespace(t *testing.T) {
	// Local provenance regression from the invented Account.TileBridge lookup.
	// Native describe/map-key rows G01-G03 remain capture-pending.
	for _, namespace := range []string{"", "lyx"} {
		for _, inline := range []bool{false, true} {
			label := "loose"
			if inline {
				label = "inline"
			}
			t.Run(namespace+"/"+label, func(t *testing.T) {
				root := t.TempDir()
				base := filepath.Join(root, "force-app/main/default/objects")
				p := project.Project{Namespace: namespace}
				targetPath := filepath.Join(base, "LyricTile__c", "LyricTile__c.object-meta.xml")
				writeFile(t, targetPath, "<CustomObject><label>Lyric Tile</label></CustomObject>")
				p.ObjectFiles = append(p.ObjectFiles, targetPath)
				fields := []struct{ name, target string }{
					{"TileBridge__c", "LyricTile__c"},
					{"StandardBridge__c", "Account"},
					{"ForeignBridge__c", "vend__LyricTile__c"},
				}
				body := "<CustomObject>"
				for _, field := range fields {
					xml := "<fullName>" + field.name + "</fullName><type>Lookup</type><referenceTo>" + field.target + "</referenceTo>"
					if inline {
						body += "<fields>" + xml + "</fields>"
					} else {
						path := filepath.Join(base, "Account", "fields", field.name+".field-meta.xml")
						writeFile(t, path, "<CustomField>"+xml+"</CustomField>")
						p.FieldFiles = append(p.FieldFiles, path)
					}
				}
				if inline {
					path := filepath.Join(base, "Account", "Account.object-meta.xml")
					writeFile(t, path, body+"</CustomObject>")
					p.ObjectFiles = append(p.ObjectFiles, path)
				}
				loaded, err := LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				objects := objectsByName(loaded.Objects)
				actual := fieldsByName(objects["Account"].Fields)
				prefix := ""
				if namespace != "" {
					prefix = namespace + "__"
				}
				for _, field := range fields {
					want := field.target
					if field.name == "TileBridge__c" {
						want = prefix + want
					}
					got := actual[prefix+field.name].ReferenceTo
					if len(got) != 1 || got[0] != want {
						t.Fatalf("%s reference = %v, want %s", field.name, got, want)
					}
				}
				if namespace != "" {
					if _, exists := objects["LyricTile__c"]; exists {
						t.Fatal("reference discovery invented a local target beside the owned object")
					}
				}
				if target := objects[prefix+"LyricTile__c"]; target.Name == "" || target.Partial {
					t.Fatalf("real target missing or replaced: %#v", target)
				}
			})
		}
	}
}
