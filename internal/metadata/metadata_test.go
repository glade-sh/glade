package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestLoadProjectIndexesLegacyReadOnlyMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/labels/CustomLabels.labels"), `<CustomLabels>
  <labels>
    <fullName>Greeting</fullName>
    <value>Hello trail</value>
    <language>en_US</language>
    <protected>false</protected>
    <shortDescription>Greeting label</shortDescription>
    <categories>General</categories>
  </labels>
</CustomLabels>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Site.resource"), "body { color: black; }")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/site.resource-meta.xml"), `<StaticResource>
  <contentType>text/css</contentType>
  <cacheControl>Public</cacheControl>
  <description>Site styles</description>
</StaticResource>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/namedCredentials/Billing.namedCredential"), `<NamedCredential>
  <fullName>Billing</fullName>
  <label>Billing API</label>
  <endpoint>https://billing.example.test</endpoint>
  <protocol>Password</protocol>
  <principalType>NamedUser</principalType>
</NamedCredential>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/remoteSiteSettings/Maps.remoteSite"), `<RemoteSiteSetting>
  <fullName>Maps</fullName>
  <url>https://maps.example.test</url>
  <isActive>true</isActive>
  <description>Maps API</description>
</RemoteSiteSetting>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/remoteSiteSettings/Billing.remoteSite"), `<RemoteSiteSetting>
  <fullName>Billing</fullName>
  <url>https://legacy-billing.example.test</url>
  <isActive>true</isActive>
</RemoteSiteSetting>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/customMetadata/Feature.Default.md"), `<CustomMetadata>
  <label>Default Feature</label>
  <protected>true</protected>
  <values>
    <field>Enabled__c</field>
    <value>true</value>
  </values>
</CustomMetadata>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fieldSets/Summary.fieldSet-meta.xml"), `<FieldSet>
  <fullName>Summary</fullName>
  <label>Summary Fields</label>
  <displayedFields>
    <field>Name</field>
    <isRequired>true</isRequired>
  </displayedFields>
</FieldSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page"), `<apex:page/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/Picker.component"), `<apex:component/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Widget/Widget.cmp"), `<aura:component/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/widget/widget.js"), `export default class Widget {}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/workflows/Account.workflow-meta.xml"), `<Workflow/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/flows/Onboard.flow-meta.xml"), `<Flow/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/layouts/Account-Account Layout.layout-meta.xml"), `<Layout/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/compactLayouts/Card.compactLayout-meta.xml"), `<CompactLayout/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/tabs/Account.tab-meta.xml"), `<CustomTab/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/webLinks/Open.webLink-meta.xml"), `<WebLink/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/quickActions/Account.New.quickAction-meta.xml"), `<QuickAction/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/globalValueSets/Region.globalValueSet-meta.xml"), `<GlobalValueSet/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/standardValueSets/CaseStatus.standardValueSet-meta.xml"), `<StandardValueSet/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/flexipages/Home.flexipage-meta.xml"), `<FlexiPage/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/applications/Console.app-meta.xml"), `<CustomApplication/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Admin.profile-meta.xml"), `<Profile>
  <fullName>Admin</fullName>
  <objectPermissions>
    <object>Account</object>
    <allowRead>true</allowRead>
    <allowCreate>true</allowCreate>
  </objectPermissions>
  <fieldPermissions>
    <field>Account.Name</field>
    <readable>true</readable>
  </fieldPermissions>
</Profile>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/App.permissionset-meta.xml"), `<PermissionSet>
  <fullName>App</fullName>
  <label>App User</label>
</PermissionSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionSetAssignments/App.permissionsetassignment-meta.xml"), `<PermissionSetAssignment/>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}

	label, ok := idx.CustomLabel("Greeting")
	if !ok || label.Value != "Hello trail" || label.Language != "en_US" {
		t.Fatalf("label lookup = %#v, %v", label, ok)
	}
	resource, ok := idx.StaticResource("SITE")
	if !ok || resource.ContentType != "text/css" || resource.CacheControl != "Public" || resource.ContentPath == "" {
		t.Fatalf("resource lookup = %#v, %v", resource, ok)
	}
	namedEndpoint, ok := idx.Endpoint("Billing")
	if !ok || namedEndpoint.Kind != "NamedCredential" || namedEndpoint.URL != "https://billing.example.test" {
		t.Fatalf("named endpoint lookup = %#v, %v", namedEndpoint, ok)
	}
	remoteEndpoint, ok := idx.Endpoint("Maps")
	if !ok || remoteEndpoint.Kind != "RemoteSiteSetting" || remoteEndpoint.URL != "https://maps.example.test" {
		t.Fatalf("remote endpoint lookup = %#v, %v", remoteEndpoint, ok)
	}
	endpoints := idx.EndpointRefs("billing")
	if len(endpoints) != 2 || endpoints[0].Kind != "NamedCredential" || endpoints[1].Kind != "RemoteSiteSetting" {
		t.Fatalf("colliding endpoint refs = %#v", endpoints)
	}
	named, ok := idx.NamedCredential("BILLING")
	if !ok || named.Endpoint != "https://billing.example.test" {
		t.Fatalf("named credential lookup = %#v, %v", named, ok)
	}
	remote, ok := idx.RemoteSite("billing")
	if !ok || remote.URL != "https://legacy-billing.example.test" {
		t.Fatalf("remote site lookup = %#v, %v", remote, ok)
	}
	record, ok := idx.CustomMetadataRecord("Feature.Default")
	if !ok || record.ObjectName != "Feature__mdt" || record.DeveloperName != "Default" || !record.Protected {
		t.Fatalf("custom metadata lookup = %#v, %v", record, ok)
	}
	if len(record.Values) != 1 || record.Values[0].Field != "Enabled__c" || record.Values[0].Value != "true" {
		t.Fatalf("custom metadata values = %#v", record.Values)
	}
	fieldSet, ok := idx.FieldSet("Account", "Summary")
	if !ok || fieldSet.Label != "Summary Fields" || len(fieldSet.Fields) != 1 || fieldSet.Fields[0].Field != "Name" || !fieldSet.Fields[0].Required {
		t.Fatalf("field set lookup = %#v, %v", fieldSet, ok)
	}
	if len(idx.VisualforcePages) != 1 || idx.VisualforcePages[0].Name != "Edit" || len(idx.VisualforceComponents) != 1 || idx.VisualforceComponents[0].Name != "Picker" {
		t.Fatalf("visualforce assets = %#v %#v", idx.VisualforcePages, idx.VisualforceComponents)
	}
	if len(idx.AuraComponents) != 1 || idx.AuraComponents[0].Name != "Widget" || len(idx.LWCComponents) != 1 || idx.LWCComponents[0].Name != "widget" {
		t.Fatalf("ui assets = %#v %#v", idx.AuraComponents, idx.LWCComponents)
	}
	if len(idx.Workflows) != 1 || idx.Workflows[0].Name != "Account" || len(idx.Flows) != 1 || idx.Flows[0].Name != "Onboard" {
		t.Fatalf("automation assets = %#v %#v", idx.Workflows, idx.Flows)
	}
	if len(idx.Layouts) != 1 || idx.Layouts[0].Name != "Account-Account Layout" || len(idx.CompactLayouts) != 1 || idx.CompactLayouts[0].Name != "Card" {
		t.Fatalf("layout assets = %#v %#v", idx.Layouts, idx.CompactLayouts)
	}
	if len(idx.Tabs) != 1 || idx.Tabs[0].Name != "Account" || len(idx.WebLinks) != 1 || idx.WebLinks[0].Name != "Open" || len(idx.QuickActions) != 1 || idx.QuickActions[0].Name != "Account.New" {
		t.Fatalf("navigation assets = %#v %#v %#v", idx.Tabs, idx.WebLinks, idx.QuickActions)
	}
	if len(idx.GlobalValueSets) != 1 || idx.GlobalValueSets[0].Name != "Region" || len(idx.StandardValueSets) != 1 || idx.StandardValueSets[0].Name != "CaseStatus" {
		t.Fatalf("value set assets = %#v %#v", idx.GlobalValueSets, idx.StandardValueSets)
	}
	if len(idx.FlexiPages) != 1 || idx.FlexiPages[0].Name != "Home" || len(idx.Applications) != 1 || idx.Applications[0].Name != "Console" {
		t.Fatalf("presentation assets = %#v %#v", idx.FlexiPages, idx.Applications)
	}
	if len(idx.Profiles) != 1 || idx.Profiles[0].Name != "Admin" || len(idx.Profiles[0].ObjectPermissions) != 1 || !idx.Profiles[0].ObjectPermissions[0].Read || len(idx.Profiles[0].FieldPermissions) != 1 {
		t.Fatalf("profile stub = %#v", idx.Profiles)
	}
	if len(idx.PermissionSets) != 1 || idx.PermissionSets[0].Name != "App" || idx.PermissionSets[0].Label != "App User" || len(idx.PermissionAssignments) != 1 || idx.PermissionAssignments[0].Name != "App" {
		t.Fatalf("permission stubs = %#v %#v", idx.PermissionSets, idx.PermissionAssignments)
	}
}

func TestLoadProjectKeepsStaticResourceMetadataWithoutBody(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Logo.staticresource-meta.xml"), `<StaticResource>
  <contentType>image/png</contentType>
  <cacheControl>Private</cacheControl>
</StaticResource>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	resource, ok := idx.StaticResource("Logo")
	if !ok || resource.MetadataPath == "" || resource.ContentPath != "" || resource.ContentType != "image/png" {
		t.Fatalf("resource metadata lookup = %#v, %v", resource, ok)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
