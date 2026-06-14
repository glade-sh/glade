package lwcshell

import "testing"

func TestLoadCustomTabParsesLWCComponentTarget(t *testing.T) {
	path := writeTempFile(t, "Lwc_Probe.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>LWC Probe</label>
  <lwcComponent>c:contextProbe</lwcComponent>
  <motif>Custom1: Heart</motif>
</CustomTab>`)

	tab, err := LoadCustomTab(path)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Name != "Lwc_Probe" || tab.Label != "LWC Probe" || tab.Type != TabTypeLWC {
		t.Fatalf("tab = %#v", tab)
	}
	if tab.Target != "c:contextProbe" {
		t.Fatalf("target = %q", tab.Target)
	}
}

func TestLoadCustomTabParsesVisualforceTabTarget(t *testing.T) {
	path := writeTempFile(t, "Legacy_VF.tab-meta.xml", `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Legacy VF</label>
  <page>LegacyPage</page>
</CustomTab>`)

	tab, err := LoadCustomTab(path)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Type != TabTypeVisualforce || tab.Target != "LegacyPage" {
		t.Fatalf("tab = %#v", tab)
	}
	diag := tab.UnsupportedDiagnostic()
	if diag.Code != "" {
		t.Fatalf("diagnostic = %#v", diag)
	}
}
