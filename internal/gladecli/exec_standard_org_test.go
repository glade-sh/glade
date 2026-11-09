package gladecli

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func execStandardOrgProject(t *testing.T, config string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	if config != "" {
		writeTestFile(t, filepath.Join(root, "glade.yml"), config)
	}
	return root
}

func execCount(root, object string) error {
	return runExec(context.Background(), []string{"--project", root, "--json", "Integer n = [SELECT COUNT() FROM " + object + "];"}, io.Discard)
}

// glade exec builds the same standard org spread as glade test, so platform
// objects that every edition carries are not "not supported" in anonymous Apex.
func TestRunExecProjectOrgHasStandardSpread(t *testing.T) {
	root := execStandardOrgProject(t, "")
	for _, object := range []string{
		"AsyncApexJob", "CronTrigger", "PricebookEntry", "OrgWideEmailAddress",
		"OpportunityLineItem", "ApexClass", "GroupMember", "EmailTemplate",
	} {
		if err := execCount(root, object); err != nil {
			t.Errorf("%s: %v", object, err)
		}
	}
}

// A features list adds to the edition. Quote stays off until Quotes is listed,
// matching org_features_additive/org.tsv, and the spread survives either way.
func TestRunExecFeatureListKeepsSpreadAndGates(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		quote        bool
	}{
		{"communities", "org:\n  features: [Communities]\n", false},
		{"communities and quotes", "org:\n  features: [Communities, Quotes]\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := execStandardOrgProject(t, tc.config)
			for _, object := range []string{"AsyncApexJob", "PricebookEntry", "OrgWideEmailAddress"} {
				if err := execCount(root, object); err != nil {
					t.Errorf("%s: %v", object, err)
				}
			}
			err := execCount(root, "Quote")
			if tc.quote && err != nil {
				t.Errorf("Quote with Quotes listed: %v", err)
			}
			// A declared shape without Quotes rejects the static query at compile
			// time (standard_object_shape_compile fam_Quote_q: Invalid type:
			// Schema.Quote), not as a run-time "not supported" error.
			if !tc.quote && (err == nil || !strings.Contains(err.Error(), `unknown SObject "Quote"`)) {
				t.Errorf("Quote without Quotes: err=%v, want a compile-time unknown SObject", err)
			}
		})
	}
}

// An undeclared shape retains the compatibility fallback, unlike a declared
// plain Developer org (org_object_availability/org.tsv, APIs 62 and 67).
func TestRunExecNoFeatureInputsAvailability(t *testing.T) {
	root := execStandardOrgProject(t, "")
	for _, object := range []string{"Quote", "QuoteLineItem", "AccountContactRelation", "WebStore"} {
		if err := execCount(root, object); err != nil {
			t.Errorf("%s: %v", object, err)
		}
	}
}

func TestRunExecStandardSeedProjectFieldsStayIsolated(t *testing.T) {
	root := execStandardOrgProject(t, "")
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/OwnedMarker__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>OwnedMarker__c</fullName><label>Owned Marker</label><type>Text</type><length>40</length></CustomField>`)
	source := `System.assert(Schema.SObjectType.Account.fields.getMap().containsKey('OwnedMarker__c'));`
	if err := runExec(context.Background(), []string{"--project", root, "--json", source}, io.Discard); err != nil {
		t.Fatal(err)
	}
	other := execStandardOrgProject(t, "")
	source = `System.assert(!Schema.SObjectType.Account.fields.getMap().containsKey('OwnedMarker__c'));`
	if err := runExec(context.Background(), []string{"--project", other, "--json", source}, io.Discard); err != nil {
		t.Fatal(err)
	}
}
