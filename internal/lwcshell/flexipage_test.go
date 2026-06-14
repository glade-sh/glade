package lwcshell

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFlexiPageParsesRegionsAndProperties(t *testing.T) {
	path := writeTempFile(t, "Account_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <masterLabel>Account Record Page</masterLabel>
  <sobjectType>Account</sobjectType>
  <template>
    <name>flexipage:recordHomeTemplateDesktop</name>
  </template>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <type>Region</type>
    <itemInstances>
      <componentInstance>
        <componentName>c:contextProbe</componentName>
        <identifier>contextProbeMain</identifier>
        <componentInstanceProperties>
          <name>title</name>
          <value>From FlexiPage</value>
        </componentInstanceProperties>
      </componentInstance>
    </itemInstances>
  </flexiPageRegions>
</FlexiPage>`)

	page, err := LoadFlexiPage(path)
	if err != nil {
		t.Fatal(err)
	}
	if page.Name != "Account_Record_Page" || page.Label != "Account Record Page" || page.Type != "RecordPage" || page.ObjectAPIName != "Account" {
		t.Fatalf("page header = %#v", page)
	}
	if page.Template != "flexipage:recordHomeTemplateDesktop" {
		t.Fatalf("template = %q", page.Template)
	}
	if len(page.Regions) != 1 || page.Regions[0].Name != "main" {
		t.Fatalf("regions = %#v", page.Regions)
	}
	components := page.Regions[0].Components
	if len(components) != 1 {
		t.Fatalf("components = %#v", components)
	}
	if components[0].ComponentName != "c:contextProbe" || components[0].Identifier != "contextProbeMain" {
		t.Fatalf("component = %#v", components[0])
	}
	if got := components[0].Properties["title"]; got != "From FlexiPage" {
		t.Fatalf("property title = %q", got)
	}
}

func TestLoadFlexiPageParsesLegacyComponentInstances(t *testing.T) {
	path := writeTempFile(t, "Sales_Dashboard.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <masterLabel>Sales Dashboard</masterLabel>
  <type>AppPage</type>
  <flexiPageRegions>
    <name>content</name>
    <componentInstances>
      <componentName>c:wireProbe</componentName>
      <properties>
        <name>mode</name>
        <value>summary</value>
      </properties>
    </componentInstances>
  </flexiPageRegions>
</FlexiPage>`)

	page, err := LoadFlexiPage(path)
	if err != nil {
		t.Fatal(err)
	}
	if page.Type != "AppPage" || len(page.Regions) != 1 || len(page.Regions[0].Components) != 1 {
		t.Fatalf("page = %#v", page)
	}
	component := page.Regions[0].Components[0]
	if component.ComponentName != "c:wireProbe" || component.Properties["mode"] != "summary" {
		t.Fatalf("legacy component = %#v", component)
	}
}

func writeTempFile(t *testing.T, name string, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
