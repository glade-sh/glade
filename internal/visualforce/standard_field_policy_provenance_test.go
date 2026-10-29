package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Synthetic VF host routes share the captured field surface: SDP/P11/org.tsv
// lines 2577,2586,2602,12066,12075,12091 and P11/ORG-RUN.txt; Account uses
// standard_recordtype_field_policy/org.tsv:2-4,9-11,16-25 and ORG-RUN.txt.
// These test configured schema propagation, not a separate native VF capture.
func TestStandardFieldPolicyConfiguredVisualforceRemoteObjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, field := range []struct{ object, name, xmlType, extra string }{
			{"Case", "HasCommentsUnreadByOwner", "Checkbox", "<defaultValue>false</defaultValue>"},
			{"Case", "IsVisibleInSelfService", "Checkbox", "<defaultValue>false</defaultValue>"},
			{"Case", "SourceId", "Lookup", "<referenceTo>EmailMessage</referenceTo><relationshipName>Source</relationshipName>"},
			{"Account", "RecordTypeId", "Lookup", "<referenceTo>RecordType</referenceTo><relationshipName>RecordType</relationshipName>"},
		} {
			for _, shape := range []string{"configured", "bare-catalog"} {
				for _, route := range []string{"model-fields", "child-field"} {
					t.Run(api+"/"+shape+"/"+field.object+"."+field.name+"/"+route, func(t *testing.T) {
						root := t.TempDir()
						writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
						if shape == "configured" {
							xml := "<CustomField><fullName>" + field.name + "</fullName><type>" + field.xmlType + "</type>" + field.extra + "</CustomField>"
							writeFile(t, filepath.Join(root, "force-app/main/default/objects", field.object, "fields", field.name+".field-meta.xml"), xml)
						}
						model := `<apex:remoteObjectModel name="` + field.object + `" fields="Id,` + field.name + `"/>`
						if route == "child-field" {
							model = `<apex:remoteObjectModel name="` + field.object + `" fields="Id"><apex:remoteObjectField name="` + field.name + `"/></apex:remoteObjectModel>`
						}
						page := filepath.Join(root, "force-app/main/default/pages/ConfiguredPolicy.page")
						writeFile(t, page, `<apex:page><apex:remoteObjects>`+model+`</apex:remoteObjects></apex:page>`)
						writeFile(t, page+"-meta.xml", "<ApexPage><apiVersion>"+api+"</apiVersion><label>Configured policy</label></ApexPage>")
						p, err := project.Load(root)
						if err != nil {
							t.Fatal(err)
						}
						loaded, err := LoadProject(p)
						if shape == "configured" {
							if err != nil || len(loaded.Pages) != 1 {
								t.Fatalf("strict VF load rejected configured field: pages=%d err=%v", len(loaded.Pages), err)
							}
						} else if err == nil || !strings.Contains(err.Error(), "Invalid field '"+field.object+"."+field.name+"'") {
							t.Fatalf("bare catalog needs field rejection, got %v", err)
						}
					})
				}
			}
		}
	}
}
