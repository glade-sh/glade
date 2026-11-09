package visualforce_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

// A custom field declared on Activity is a field of Task and Event. A page
// that repeats over Task or Event rows and binds that field registers, in a
// nested non-default folder with its -meta.xml and with the field declared in
// another package directory. An undeclared field on the same row still
// rejects the page.
func TestPageNamesAcceptActivityFieldsOnTaskAndEvent(t *testing.T) {
	controller := `public with sharing class CedarActivityExt {
    public List<Task> goals { get; set; }
    public List<Event> meetings { get; set; }
    public CedarActivityExt(ApexPages.StandardController sc) { goals = new List<Task>(); meetings = new List<Event>(); }
}`
	page := func(field string) string {
		return `<apex:page standardController="Cedar_Report__c" extensions="CedarActivityExt">` +
			`<apex:repeat value="{!goals}" var="goal"><apex:outputText value="{0}"><apex:param value="{!goal.` + field + `}"/></apex:outputText></apex:repeat>` +
			`<apex:repeat value="{!meetings}" var="m">{!m.` + field + `}</apex:repeat></apex:page>`
	}
	pageMeta := `<?xml version="1.0" encoding="UTF-8"?><ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>CedarActivityReport</label></ApexPage>`
	object := `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Cedar Report</label><pluralLabel>Cedar Reports</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`
	field := `<?xml version="1.0" encoding="UTF-8"?><CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Cedar_Done_At__c</fullName><label>Done At</label><type>DateTime</type></CustomField>`
	classMeta := `<ApexClass><apiVersion>59.0</apiVersion></ApexClass>`
	for _, api := range []string{"59.0", "67.0"} {
		for _, tc := range []struct {
			name  string
			field string
			want  []string
		}{
			{"activity_field", "Cedar_Done_At__c", []string{"CedarActivityReport"}},
			{"undeclared_field", "Cedar_Missing__c", nil},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writePageNameFixture(t, root, "sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true},{"path":"shared-app"}],"sourceApiVersion":%q}`, api))
				files := map[string]string{
					"force-app/cedarReport/pages/nested/CedarActivityReport.page":                     page(tc.field),
					"force-app/cedarReport/pages/nested/CedarActivityReport.page-meta.xml":            pageMeta,
					"force-app/cedarReport/classes/CedarActivityExt.cls":                              controller,
					"force-app/cedarReport/classes/CedarActivityExt.cls-meta.xml":                     classMeta,
					"force-app/main/default/objects/Cedar_Report__c/Cedar_Report__c.object-meta.xml":  object,
					"shared-app/main/default/objects/Activity/fields/Cedar_Done_At__c.field-meta.xml": field,
				}
				for name, source := range files {
					writePageNameFixture(t, root, filepath.FromSlash(name), source)
				}
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
