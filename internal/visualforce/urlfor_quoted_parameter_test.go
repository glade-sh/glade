package visualforce_test

import (
	"fmt"
	"html"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestURLFORSingleQuotedParameterValidation(t *testing.T) {
	// URLFOR documents quoted input names. Exact single/double-quote native
	// acceptance is pending VFPAR01-03; this candidate adds single quotes only.
	// https://help.salesforce.com/s/articleView?id=platform.customize_functions_urlfor.htm&language=en_US&type=5
	cases := []struct {
		name, expression string
		wantError        bool
	}{
		{"single_quoted", "URLFOR($Page.orbitpanel,null,['id'='owned'])", false},
		{"bare_control", "URLFOR($Page.orbitpanel,null,[id='owned'])", false},
		{"missing_assignment", "URLFOR($Page.orbitpanel,null,['id' 'owned'])", true},
		{"missing_value", "URLFOR($Page.orbitpanel,null,['id'=])", true},
		{"dynamic_key", "URLFOR($Page.orbitpanel,null,['id'+'suffix'='owned'])", true},
		{"empty_key", "URLFOR($Page.orbitpanel,null,[''='owned'])", true},
		{"missing_bracket", "URLFOR($Page.orbitpanel,null,['id'='owned')", true},
		{"double_quote_outside_candidate", `URLFOR($Page.orbitpanel,null,["id"='owned'])`, true},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writePageNameFixture(t, root, "sfdx-project.json", fmt.Sprintf(`{"namespace":"lyx","sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				base := "force-app/main/default/"
				writePageNameFixture(t, root, base+"classes/OwnedPageController.cls", `public class OwnedPageController { public String getMessage() { return 'ready'; } }`)
				writePageNameFixture(t, root, base+"pages/orbitpanel.page", `<apex:page/>`)
				writePageNameFixture(t, root, base+"pages/linkpanel.page", fmt.Sprintf(`<apex:page controller="OwnedPageController"><apex:outputLink value="{!%s}">Open</apex:outputLink></apex:page>`, html.EscapeString(tc.expression)))
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
