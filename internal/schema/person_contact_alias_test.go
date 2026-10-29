package schema

import (
	"github.com/glade-sh/glade/internal/project"
	"path/filepath"
	"testing"
)

func TestLoadProjectPersonContactAliases(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			root := t.TempDir()
			field := filepath.Join(root, "force-app/main/default/objects/Contact/fields/Drift_Tag__c.field-meta.xml")
			writeFile(t, field, `<CustomField><fullName>Drift_Tag__c</fullName><label>Drift Tag</label><type>Text</type><length>24</length></CustomField>`)
			if enabled {
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"features":["PersonAccounts"]}`)
			}
			p := project.Project{Root: root, FieldFiles: []string{field}}
			for range 2 {
				s, err := LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, o := range s.Objects {
					if o.Name == "Contact" && (len(o.Fields) != 1 || o.Fields[0].Name != "Drift_Tag__c") {
						t.Fatalf("contact fields changed: %v", o.Fields)
					}
					if o.Name == "Account" {
						for _, f := range o.Fields {
							if f.Name == "Drift_Tag__pc" {
								found = true
								if f.Type != "Text" || f.Length != 24 {
									t.Fatalf("alias metadata: %+v", f)
								}
							}
						}
					}
				}
				if found != enabled {
					t.Fatalf("alias=%v; feature=%v", found, enabled)
				}
			}
		})
	}
}
