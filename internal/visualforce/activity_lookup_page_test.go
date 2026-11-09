package visualforce_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

// An <apex:outputField> on a repeated Event or Task row may traverse an
// Activity custom lookup (Field__r.Name). The lookup is declared in another
// package directory or in a loaded source dependency (glade.yml
// dependencyPaths), and the project also carries Event and Task object
// folders with a standard-field override, so Event and Task are indexed
// objects whose own files do not list the Activity field. An undeclared
// relationship on the same row still rejects the page.
func TestPageNamesAcceptActivityLookupOutputFieldOnEventAndTask(t *testing.T) {
	controller := `public with sharing class CedarMeetingExt {
    public List<Event> meetings { get; set; }
    public List<Task> goals { get; set; }
    public CedarMeetingExt(ApexPages.StandardController sc) { meetings = new List<Event>(); goals = new List<Task>(); }
}`
	page := func(rel string) string {
		return `<apex:page standardController="Cedar_Report__c" extensions="CedarMeetingExt">` +
			`<apex:repeat value="{!meetings}" var="rel"><apex:outputField value="{!rel.` + rel + `.name}"/><apex:outputField value="{!rel.Cedar_Contact__r.Name}"/></apex:repeat>` +
			`<apex:repeat value="{!goals}" var="g"><apex:outputField value="{!g.` + rel + `.Name}"/></apex:repeat></apex:page>`
	}
	pageMeta := `<?xml version="1.0" encoding="UTF-8"?><ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>CedarMeetingReport</label></ApexPage>`
	object := `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Cedar Report</label><pluralLabel>Cedar Reports</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`
	lookup := func(name, rel, target string) string {
		return `<?xml version="1.0" encoding="UTF-8"?><CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>` + name + `</fullName><deleteConstraint>SetNull</deleteConstraint><label>L</label><referenceTo>` + target + `</referenceTo><relationshipLabel>R</relationshipLabel><relationshipName>` + rel + `</relationshipName><required>false</required><type>Lookup</type></CustomField>`
	}
	picklist := func(name string) string {
		return `<?xml version="1.0" encoding="UTF-8"?><CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>` + name + `</fullName><type>Picklist</type></CustomField>`
	}
	activityMeta := `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"></CustomObject>`
	classMeta := `<ApexClass><apiVersion>59.0</apiVersion></ApexClass>`
	for _, api := range []string{"59.0", "67.0"} {
		for _, tc := range []struct {
			name       string
			rel        string
			dependency bool
			want       []string
		}{
			{"package_dir_lookup", "Cedar_Account__r", false, []string{"CedarMeetingReport"}},
			{"dependency_lookup", "Cedar_Account__r", true, []string{"CedarMeetingReport"}},
			{"undeclared_lookup", "Cedar_Missing__r", false, nil},
			{"dependency_undeclared_lookup", "Cedar_Missing__r", true, nil},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "consumer")
				shared := root
				sharedDir := "shared-app"
				if tc.dependency {
					shared = filepath.Join(filepath.Dir(root), "base")
					sharedDir = "base-app"
					writePageNameFixture(t, root, "sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"CedarBase"}]}],"sourceApiVersion":%q}`, api))
					writePageNameFixture(t, root, "glade.yml", "project:\n  dependencyPaths: [\"../base\"]\n")
					writePageNameFixture(t, shared, "sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"base-app","default":true,"package":"CedarBase"}],"sourceApiVersion":%q}`, api))
				} else {
					writePageNameFixture(t, root, "sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true},{"path":"shared-app"}],"sourceApiVersion":%q}`, api))
				}
				files := map[string]string{
					"force-app/cedarReport/pages/CedarMeetingReport.page":                            page(tc.rel),
					"force-app/cedarReport/pages/CedarMeetingReport.page-meta.xml":                   pageMeta,
					"force-app/cedarReport/classes/CedarMeetingExt.cls":                              controller,
					"force-app/cedarReport/classes/CedarMeetingExt.cls-meta.xml":                     classMeta,
					"force-app/main/default/objects/Cedar_Report__c/Cedar_Report__c.object-meta.xml": object,
					"force-app/main/default/objects/Activity/Activity.object-meta.xml":               activityMeta,
					"force-app/main/default/objects/Activity/fields/Cedar_Contact__c.field-meta.xml": lookup("Cedar_Contact__c", "Cedar_Contact_Activities", "Contact"),
					"force-app/main/default/objects/Event/fields/Subject.field-meta.xml":             picklist("Subject"),
					"force-app/main/default/objects/Task/fields/Type.field-meta.xml":                 picklist("Type"),
				}
				for name, source := range files {
					writePageNameFixture(t, root, filepath.FromSlash(name), source)
				}
				writePageNameFixture(t, shared, filepath.FromSlash(sharedDir+"/main/default/objects/Activity/Activity.object-meta.xml"), activityMeta)
				writePageNameFixture(t, shared, filepath.FromSlash(sharedDir+"/main/default/objects/Activity/fields/Cedar_Account__c.field-meta.xml"), lookup("Cedar_Account__c", "Cedar_Activities", "Account"))
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				names, rejected := visualforce.PageNamesWithDiagnostics(p, nil)
				if !slices.Equal(names, tc.want) {
					t.Fatalf("page names = %v, want %v; rejected %+v", names, tc.want, rejected)
				}
			})
		}
	}
}
