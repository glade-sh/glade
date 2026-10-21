package visualforce_test

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestURLFORCustomTabValidation(t *testing.T) {
	// Salesforce's URLFOR documentation permits a custom tab action target
	// paired with the tab's object type. Exact namespace/boundary acceptance
	// remains pending VFACT01-05; rendering support is a separate contract.
	// https://help.salesforce.com/s/articleView?id=platform.customize_functions_urlfor.htm&language=en_US&type=5
	cases := []struct {
		name, namespace, expression string
		tab, wantError              bool
	}{
		{"plain", "", "URLFOR($Action.OrbitRecord__c.Tab,$ObjectType.OrbitRecord__c)", true, false},
		{"owned_local", "lyx", "URLFOR($Action.OrbitRecord__c.Tab,$ObjectType.OrbitRecord__c)", true, false},
		{"owned_qualified", "lyx", "URLFOR($Action.lyx__OrbitRecord__c.Tab,$ObjectType.lyx__OrbitRecord__c)", true, false},
		{"missing_tab", "lyx", "URLFOR($Action.OrbitRecord__c.Tab,$ObjectType.OrbitRecord__c)", false, true},
		{"missing_object", "lyx", "URLFOR($Action.Missing__c.Tab,$ObjectType.Missing__c)", true, true},
		{"missing_foreign", "lyx", "URLFOR($Action.vend__OrbitRecord__c.Tab,$ObjectType.vend__OrbitRecord__c)", true, true},
		{"setting", "lyx", "URLFOR($Action.OrbitConfig__c.Tab,$ObjectType.OrbitConfig__c)", true, true},
		{"different_object_type", "lyx", "URLFOR($Action.OrbitRecord__c.Tab,$ObjectType.OrbitConfig__c)", true, true},
		{"other_action", "lyx", "URLFOR($Action.OrbitRecord__c.New,$ObjectType.OrbitRecord__c)", true, true},
		{"extra_path", "lyx", "URLFOR($Action.OrbitRecord__c.Tab.Nested,$ObjectType.OrbitRecord__c)", true, true},
		{"outside_urlfor", "lyx", "$Action.OrbitRecord__c.Tab", true, true},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writePageNameFixture(t, root, "sfdx-project.json", fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, tc.namespace, api))
				base := "force-app/main/default/"
				writePageNameFixture(t, root, base+"classes/OwnedPageController.cls", `public class OwnedPageController { public String getMessage() { return 'ready'; } }`)
				writePageNameFixture(t, root, base+"pages/orbitpanel.page", fmt.Sprintf(`<apex:page controller="OwnedPageController"><apex:outputLink value="{!%s}">Open</apex:outputLink></apex:page>`, tc.expression))
				writePageNameFixture(t, root, base+"objects/OrbitRecord__c/OrbitRecord__c.object-meta.xml", `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Orbit Record</label><pluralLabel>Orbit Records</pluralLabel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>`)
				writePageNameFixture(t, root, base+"objects/OrbitConfig__c/OrbitConfig__c.object-meta.xml", `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><customSettingsType>Hierarchy</customSettingsType><label>Orbit Config</label><visibility>Public</visibility></CustomObject>`)
				if tc.tab {
					writePageNameFixture(t, root, base+"tabs/OrbitRecord__c.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata"><customObject>true</customObject></CustomTab>`)
				}
				// Unrelated invalid tab metadata must not become a prerequisite.
				writePageNameFixture(t, root, base+"tabs/Unrelated.tab-meta.xml", `<CustomTab>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				_, err = visualforce.LoadProject(p)
				if (err != nil) != tc.wantError {
					t.Fatalf("LoadProject error = %v, want rejection = %t", err, tc.wantError)
				}
			})
		}
	}
}
