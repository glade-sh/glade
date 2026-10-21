package resource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTabPreservesPageAndWebProvenance(t *testing.T) {
	for _, tc := range []struct {
		name, xml, page, url, object string
	}{
		{"PanelView", "<page> PanelView </page>", "PanelView", "", ""},
		{"PanelLink", "<url> https://example.invalid/panel </url>", "", "https://example.invalid/panel", ""},
		{"Panel__c", "<customObject>true</customObject>", "", "", "Panel__c"},
		{"PanelComponent", "<lwcComponent>panel</lwcComponent>", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name+".tab-meta.xml")
			if err := os.WriteFile(path, []byte("<CustomTab>"+tc.xml+"</CustomTab>"), 0600); err != nil {
				t.Fatal(err)
			}
			tab, err := loadTab(path)
			if err != nil {
				t.Fatal(err)
			}
			if tab.Page != tc.page || tab.URL != tc.url || tab.SObjectName != tc.object {
				t.Fatalf("tab = %#v, want page=%q url=%q object=%q", tab, tc.page, tc.url, tc.object)
			}
		})
	}
}
