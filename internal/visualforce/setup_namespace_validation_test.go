package visualforce_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestSetupNamespaceValidationPreservesOwnedHierarchySetting(t *testing.T) {
	// VFSET01-03 positive namespaced deployment rows are pending. This local
	// candidate covers only the owned hierarchy-setting object lookup. Native
	// V02 e_setup_absent/nonsetting/wrong_root at API 59/67 reject those shapes.
	cases := []struct {
		name, namespace, expression string
		wantError                   bool
	}{
		{"plain_local", "", "$Setup.OrbitConfig__c.Enabled__c", false},
		{"owned_local", "lyx", "$Setup.OrbitConfig__c.Enabled__c", false},
		{"owned_qualified", "lyx", "$Setup.lyx__OrbitConfig__c.lyx__Enabled__c", false},
		{"foreign_qualified", "lyx", "$Setup.vend__ForeignConfig__c.vend__Enabled__c", false},
		{"missing_foreign", "lyx", "$Setup.vend__OrbitConfig__c.Enabled__c", true},
		{"missing_local", "lyx", "$Setup.MissingConfig__c.Enabled__c", true},
		{"ordinary_object", "lyx", "$Setup.OrbitRecord__c.Enabled__c", true},
		{"wrong_root", "lyx", "$Setup.Enabled__c", true},
		{"local_list_setting", "lyx", "$Setup.OrbitList__c.Enabled__c", true},
		{"outside_candidate_shape", "lyx", "$Setup.OrbitConfig__c.Enabled__c.Nested", true},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writePageNameFixture(t, root, "sfdx-project.json", fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"app","default":false},{"path":"settings","default":true}]}`, tc.namespace, api))
				writePageNameFixture(t, root, "app/main/default/classes/OwnedPageController.cls", `public class OwnedPageController { public String getMessage() { return 'ready'; } }`)
				writePageNameFixture(t, root, "app/main/default/pages/orbitpanel.page", `<apex:page/>`)
				writePageNameFixture(t, root, "app/main/default/pages/settingspanel.page", fmt.Sprintf(`<apex:page controller="OwnedPageController"><apex:outputText value="{!%s}"/></apex:page>`, tc.expression))
				for name, settingType := range map[string]string{"OrbitConfig__c": "Hierarchy", "vend__ForeignConfig__c": "Hierarchy", "OrbitList__c": "List"} {
					writePageNameFixture(t, root, "settings/main/default/objects/"+name+"/"+name+".object-meta.xml", fmt.Sprintf(`<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><customSettingsType>%s</customSettingsType><label>Owned Setting</label><visibility>Public</visibility><fields><fullName>Enabled__c</fullName><defaultValue>false</defaultValue><label>Enabled</label><type>Checkbox</type></fields></CustomObject>`, settingType))
				}
				writePageNameFixture(t, root, "settings/main/default/objects/OrbitRecord__c/OrbitRecord__c.object-meta.xml", `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Orbit Record</label><pluralLabel>Orbit Records</pluralLabel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				_, err = visualforce.LoadProject(p)
				if (err != nil) != tc.wantError {
					t.Fatalf("LoadProject error = %v, want rejection = %t", err, tc.wantError)
				}
				var wantNames []string
				if !tc.wantError {
					wantNames = []string{"orbitpanel", "settingspanel"}
				}
				if got := visualforce.PageNames(p); !slices.Equal(got, wantNames) {
					t.Fatalf("PageNames = %v, want %v", got, wantNames)
				}
			})
		}
	}
}
