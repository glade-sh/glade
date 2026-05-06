package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/storage"
)

func TestLoadProjectResourcesLabelsAndEndpoints(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/labels/CustomLabels.labels"), `<CustomLabels>
  <labels><fullName>Greeting</fullName><value>Hello</value><language>en_US</language></labels>
</CustomLabels>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/translations/fr.translation-meta.xml"), `<Translations>
  <customLabels><name>Greeting</name><label>Bonjour</label></customLabels>
</Translations>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Site.resource"), "site-body")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Site.resource-meta.xml"), `<StaticResource><contentType>text/plain</contentType><cacheControl>Public</cacheControl></StaticResource>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/contentassets/Logo.asset"), "asset-body")
	writeFile(t, filepath.Join(root, "force-app/main/default/contentassets/Logo.asset-meta.xml"), `<ContentAsset><contentType>image/png</contentType></ContentAsset>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/email/welcome.email"), "Hello {!Recipient.FirstName}")
	writeFile(t, filepath.Join(root, "force-app/main/default/email/welcome.email-meta.xml"), `<EmailTemplate>
  <fullName>unfiled$public/welcome</fullName>
  <name>Welcome</name>
  <subject>Welcome subject</subject>
  <htmlValue>&lt;p&gt;Hello {!Contact.FirstName}&lt;/p&gt;</htmlValue>
  <templateType>custom</templateType>
  <available>true</available>
</EmailTemplate>`)
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
	template := org.Objects["EmailTemplate"].Records["00X000000000001"]
	if got := template.Fields["DeveloperName"].String; got != "welcome" {
		t.Fatalf("EmailTemplate DeveloperName = %q", got)
	}
	if got := template.Fields["Subject"].String; got != "Welcome subject" {
		t.Fatalf("EmailTemplate Subject = %q", got)
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
