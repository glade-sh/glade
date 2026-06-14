package visualforce

import "testing"

func TestGeneratedComponentCatalogMatchesLocalDocsCount(t *testing.T) {
	got := len(StandardComponentCatalog())
	if got != 161 {
		t.Fatalf("component catalog count = %d, want 161 from local docs scrape", got)
	}
}

func TestGeneratedComponentCatalogPreservesNamesAndAttributes(t *testing.T) {
	catalog := StandardComponentCatalog()
	page := findCatalogEntry(catalog, "apex:page")
	if page == nil {
		t.Fatalf("catalog missing apex:page")
	}
	if page.SourceFile != "visualforce/pages_compref_page.md" {
		t.Fatalf("apex:page source = %q", page.SourceFile)
	}
	if !hasCatalogAttribute(page.Attributes, "action", "ApexPages.Action", false, "10.0") {
		t.Fatalf("apex:page missing action attribute: %#v", page.Attributes)
	}
	if !hasCatalogAttribute(page.Attributes, "showHeader", "Boolean", false, "10.0") {
		t.Fatalf("apex:page missing showHeader attribute: %#v", page.Attributes)
	}
	if findCatalogEntry(catalog, "flow:interview") == nil {
		t.Fatalf("catalog missing flow:interview")
	}
}

func findCatalogEntry(catalog []ComponentCatalogEntry, name string) *ComponentCatalogEntry {
	for i := range catalog {
		if catalog[i].Name == name {
			return &catalog[i]
		}
	}
	return nil
}

func hasCatalogAttribute(attrs []ComponentAttribute, name string, attrType string, required bool, apiVersion string) bool {
	for _, attr := range attrs {
		if attr.Name == name && attr.Type == attrType && attr.Required == required && attr.APIVersion == apiVersion {
			return true
		}
	}
	return false
}
