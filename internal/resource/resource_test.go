package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestLoadProjectResourcesLabelsAndEndpoints(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/labels/CustomLabels.labels"), `<CustomLabels>
  <labels><fullName>Greeting</fullName><value>Hello</value><language>en_US</language></labels>
  <labels><fullName>pkg__ManagedGreeting</fullName><value>Hello managed</value><language>en_US</language></labels>
</CustomLabels>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/ext__Widget__c/ext__Widget__c.object-meta.xml"), `<CustomObject><label>Widget</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/translations/fr.translation-meta.xml"), `<Translations>
  <customLabels><name>Greeting</name><label>Bonjour</label></customLabels>
</Translations>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Site.resource"), "site-body")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Site.resource-meta.xml"), `<StaticResource><contentType>text/plain</contentType><cacheControl>Public</cacheControl></StaticResource>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/contentassets/Logo.asset"), "asset-body")
	writeFile(t, filepath.Join(root, "force-app/main/default/contentassets/Logo.asset-meta.xml"), `<ContentAsset><contentType>image/png</contentType></ContentAsset>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/tabs/ext__Widget__c.tab-meta.xml"), `<CustomTab>
  <customObject>true</customObject>
  <description>Widget tab</description>
  <label>Widgets</label>
  <motif>Custom1: Heart</motif>
</CustomTab>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/quickActions/Account.NewTask.quickAction-meta.xml"), `<QuickAction>
  <label>New Task</label>
  <type>Create</type>
  <targetObject>Account</targetObject>
</QuickAction>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fieldSets/Summary.fieldSet-meta.xml"), `<FieldSet>
  <fullName>Summary</fullName>
  <label>Account Summary</label>
  <displayedFields><field>Name</field><isRequired>true</isRequired></displayedFields>
  <availableFields><field>Rating</field></availableFields>
</FieldSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject>
  <fieldSets>
    <fullName>InlineSummary</fullName>
    <label>Inline Summary</label>
    <displayedFields><field>Phone</field><isRequired>true</isRequired></displayedFields>
  </fieldSets>
</CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/email/welcome.email"), "Hello {!Recipient.FirstName}")
	writeFile(t, filepath.Join(root, "force-app/main/default/email/welcome.email-meta.xml"), `<EmailTemplate>
  <fullName>unfiled$public/welcome</fullName>
  <name>Welcome</name>
  <subject>Welcome subject</subject>
  <htmlValue>&lt;p&gt;Hello {!Contact.FirstName}&lt;/p&gt;</htmlValue>
  <templateType>custom</templateType>
  <available>true</available>
</EmailTemplate>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/GLExport.documentFolder-meta.xml"), `<DocumentFolder>
  <accessType>Public</accessType>
  <name>GL Export</name>
  <publicFolderAccess>ReadWrite</publicFolderAccess>
</DocumentFolder>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/namedCredentials/Billing.namedCredential"), `<NamedCredential><endpoint>https://billing.example.test</endpoint><protocol>NoAuthentication</protocol></NamedCredential>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/remoteSiteSettings/Maps.remoteSite"), `<RemoteSiteSetting><url>https://maps.example.test</url></RemoteSiteSetting>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := LookupLabel(registry, "pkg", "Greeting"); !ok || got != "Bonjour" {
		t.Fatalf("translated label = %q, %v", got, ok)
	}
	if got, ok := LookupLabel(registry, "pkg", "ManagedGreeting"); !ok || got != "Hello managed" {
		t.Fatalf("managed label fallback = %q, %v", got, ok)
	}
	if got, status := ResolveLabel(registry, "pkg", "ext", "External_Message"); got != "External_Message" || status != LabelLookupManagedNamespaceFallback {
		t.Fatalf("external managed label fallback = %q, %s", got, status)
	}
	if got, status := ResolveLabel(registry, "pkg", "Site", "invalid_email"); got != "invalid_email" || status != LabelLookupPlatformFallback {
		t.Fatalf("platform label fallback = %q, %s", got, status)
	}
	if got, status := ResolveLabel(registry, "pkg", "pkg", "Missing"); got != "" || status != LabelLookupMissing {
		t.Fatalf("own namespace missing label = %q, %s", got, status)
	}
	if got, ok := URLForStaticResource(registry, "Site", "css/app.css"); !ok || got != "/resource/Site/css/app.css" {
		t.Fatalf("resource url = %q, %v", got, ok)
	}
	if got, ok := URLForStaticResource(registry, "pkg__Site", "css/app.css"); !ok || got != "/resource/Site/css/app.css" {
		t.Fatalf("namespaced resource url = %q, %v", got, ok)
	}
	if got, ok := URLForStaticResource(registry, "Logo", ""); !ok || got != "/sfc/servlet.shepherd/version/download/Logo" {
		t.Fatalf("asset url = %q, %v", got, ok)
	}
	if got, ok := ResolveEndpoint(registry, "callout:Billing/v1/accounts"); !ok || got != "https://billing.example.test/v1/accounts" {
		t.Fatalf("endpoint = %q, %v", got, ok)
	}
	if got, ok := ResolveEndpoint(registry, "callout:pkg__Billing/v1/accounts"); !ok || got != "https://billing.example.test/v1/accounts" {
		t.Fatalf("namespaced endpoint = %q, %v", got, ok)
	}
	if len(registry.Tabs) != 1 || registry.Tabs[0].Name != "ext__Widget__c" || registry.Tabs[0].Label != "Widgets" || registry.Tabs[0].SObjectName != "ext__Widget__c" {
		t.Fatalf("tabs = %#v", registry.Tabs)
	}
	if len(registry.QuickActions) != 1 || registry.QuickActions[0].Name != "Account.NewTask" || registry.QuickActions[0].TargetObject != "Account" || registry.QuickActions[0].Label != "New Task" {
		t.Fatalf("quick actions = %#v", registry.QuickActions)
	}
	if len(registry.FieldSets) != 2 || registry.FieldSets[0].ObjectName != "Account" || registry.FieldSets[0].Name != "InlineSummary" || len(registry.FieldSets[0].Fields) != 1 || !registry.FieldSets[0].Fields[0].Required || registry.FieldSets[1].Name != "Summary" || len(registry.FieldSets[1].Fields) != 2 {
		t.Fatalf("field sets = %#v", registry.FieldSets)
	}
	if len(registry.EmailTemplates) != 1 || registry.EmailTemplates[0].DeveloperName != "welcome" || registry.EmailTemplates[0].Body != "Hello {!Recipient.FirstName}" {
		t.Fatalf("email templates = %#v", registry.EmailTemplates)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if got := org.Objects["StaticResource"].Records["081000000000001"].Fields["Body"].String; got != "site-body" {
		t.Fatalf("StaticResource body = %q", got)
	}
	template := org.Objects["EmailTemplate"].Records["00X000000100001"]
	if got := template.Fields["DeveloperName"].String; got != "welcome" {
		t.Fatalf("EmailTemplate DeveloperName = %q", got)
	}
	if got := template.Fields["Subject"].String; got != "Welcome subject" {
		t.Fatalf("EmailTemplate Subject = %q", got)
	}
	folder := org.Objects["Folder"].Records["00l000000000001"]
	if got := folder.Fields["Name"].String; got != "GL Export" {
		t.Fatalf("Folder Name = %q", got)
	}
	if got := folder.Fields["DeveloperName"].String; got != "GLExport" {
		t.Fatalf("Folder DeveloperName = %q", got)
	}
	if got := folder.Fields["Type"].String; got != "Document" {
		t.Fatalf("Folder Type = %q", got)
	}
}

func TestApplyProjectLoadsDependencyFolders(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/documents/GLExport.documentFolder-meta.xml"), `<DocumentFolder>
  <name>GL Export</name>
</DocumentFolder>`)

	depProject, err := project.Load(depRoot)
	if err != nil {
		t.Fatal(err)
	}
	p := project.Project{
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace: "pkg",
			Status:    "loaded",
			Project:   &depProject,
		}},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	folder := org.Objects["Folder"].Records["00l000000000001"]
	if got := folder.Fields["Name"].String; got != "GL Export" {
		t.Fatalf("Folder Name = %q", got)
	}
}

func TestLoadProjectDiscoversUnpackagedStaticResourceFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/resetcss.resource"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/resetcss.resource-meta.xml"), `<StaticResource><contentType>text/css</contentType><cacheControl>Public</cacheControl></StaticResource>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := URLForStaticResource(registry, "resetcss", ""); !ok {
		t.Fatalf("resetcss static resource was not loaded; resources=%#v", registry.StaticResources)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	object := org.Objects["StaticResource"]
	for _, record := range object.Records {
		if record.Fields["Name"].String == "resetcss" {
			return
		}
	}
	t.Fatalf("resetcss StaticResource record was not created; records=%#v", object.Records)
}

func TestApplyProjectIncludesLoadedDependencyFieldSets(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/Account/fieldSets/BillingAddress.fieldSet-meta.xml"), `<FieldSet>
  <fullName>BillingAddress</fullName>
  <label>Billing Address</label>
  <displayedFields><field>BillingStreet</field></displayedFields>
</FieldSet>`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)

	p, err := project.Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if len(org.Metadata.FieldSets) != 1 || org.Metadata.FieldSets[0].ObjectName != "Account" || org.Metadata.FieldSets[0].Name != "BillingAddress" || org.Metadata.FieldSets[0].Namespace != "pkg" {
		t.Fatalf("field sets = %#v", org.Metadata.FieldSets)
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
