package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
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
