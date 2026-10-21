package resource

import (
	"path/filepath"
	"testing"
)

// CustomTab.customObject identifies an object-backed tab, not whether the tab
// is custom. Page and component tabs are CustomTab metadata too.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/api_meta.pdf
func TestLoadCustomTabKindsRemainCustom(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		object        bool
	}{
		{"Harbor__c", "<customObject>true</customObject>", true},
		{"HarborWeb", "<customObject>false</customObject><url>https://example.com</url>", false},
		{"HarborPage", "<page>HarborView</page>", false},
		{"HarborFlexi", "<flexiPage>HarborDashboard</flexiPage>", false},
		{"HarborAura", "<auraComponent>HarborPanel</auraComponent>", false},
		{"HarborLwc", "<lwcComponent>harborPanel</lwcComponent>", false},
		{"HarborControl", "<scontrol>HarborControl</scontrol>", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name+".tab-meta.xml")
			writeFile(t, path, `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata"><label>Harbor</label><motif>Custom9: Lightning</motif>`+tc.content+`</CustomTab>`)
			tab, err := loadTab(path)
			if err != nil {
				t.Fatal(err)
			}
			if !tab.Custom {
				t.Fatal("CustomTab metadata was classified as a standard tab")
			}
			wantObject := ""
			if tc.object {
				wantObject = tc.name
			}
			if tab.SObjectName != wantObject {
				t.Fatalf("associated object = %q; want %q", tab.SObjectName, wantObject)
			}
		})
	}
}
