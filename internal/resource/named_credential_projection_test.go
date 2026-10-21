package resource

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestApplyProjectProjectsLegacyAnonymousNamedCredential(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	object := org.Objects["NamedCredential"]
	if len(object.Records) != 1 {
		t.Fatalf("NamedCredential records = %#v, want one source metadata row", object.Records)
	}
	var projected storage.Record
	for _, record := range object.Records {
		projected = record
	}
	if object.Definition.KeyPrefix == "" || !strings.HasPrefix(string(projected.ID), object.Definition.KeyPrefix) {
		t.Fatalf("NamedCredential Id %q does not use object key prefix %q", projected.ID, object.Definition.KeyPrefix)
	}
	wantFields := map[string]storage.Value{
		"Id":              storage.IDValue(projected.ID),
		"DeveloperName":   storage.StringValue("ProbeEndpoint"),
		"Endpoint":        storage.StringValue("https://example.invalid"),
		"NamespacePrefix": storage.NullValue(),
	}
	if projected.Object != "NamedCredential" || !reflect.DeepEqual(projected.Fields, wantFields) {
		t.Fatalf("NamedCredential = %#v, want fields %#v", projected, wantFields)
	}
	if got, ok := ResolveEndpoint(org.Metadata, "callout:ProbeEndpoint/path"); !ok || got != "https://example.invalid/path" {
		t.Fatalf("endpoint resolution = %q, %v", got, ok)
	}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if got := org.Objects["NamedCredential"].Records; len(got) != 1 || !reflect.DeepEqual(got[projected.ID], projected) {
		t.Fatalf("reloaded NamedCredential rows = %#v, want unchanged identity and fields", got)
	}
}

// Scratch-org row (API 62): a legacy NoAuthentication credential with
// principalType NamedUser answers SELECT DeveloperName, Endpoint
// with its source values.
func TestApplyProjectProjectsLegacyNamedUserNamedCredential(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>NamedUser</principalType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, record := range org.Objects["NamedCredential"].Records {
		found = record.Fields["DeveloperName"].String == "ProbeEndpoint" && record.Fields["Endpoint"].String == "https://example.invalid"
	}
	if !found {
		t.Fatalf("NamedUser NoAuthentication credential was not projected: %#v", org.Objects["NamedCredential"].Records)
	}
}

func TestApplyProjectNamedCredentialProjectionLeavesOtherShapesUnchanged(t *testing.T) {
	for _, test := range []struct {
		name             string
		projectNamespace string
		orgNamespace     string
		credentialName   string
		xml              string
	}{
		{name: "authenticated", xml: `<protocol>Password</protocol><principalType>NamedUser</principalType>`},
		{name: "unspecifiedPrincipal", xml: `<protocol>NoAuthentication</protocol>`},
		{name: "privateEndpoint", xml: `<namedCredentialType>PrivateEndpoint</namedCredentialType><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`},
		{name: "qualifiedName", credentialName: "pkg__ProbeEndpoint", xml: `<protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`},
		{name: "projectNamespace", projectNamespace: "pkg", xml: `<protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`},
		{name: "orgNamespace", orgNamespace: "pkg", xml: `<protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := test.credentialName
			if name == "" {
				name = "ProbeEndpoint"
			}
			p := namedCredentialProjectionProject(t, test.projectNamespace, name, `<endpoint>https://example.invalid</endpoint>`+test.xml)
			org := storage.NewOrgState()
			org.Namespace = test.orgNamespace
			if err := ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			if got := org.Objects["NamedCredential"].Records; len(got) != 0 {
				t.Fatalf("unsupported projection shape created NamedCredential rows: %#v", got)
			}
			if got, ok := ResolveEndpoint(org.Metadata, "callout:"+name+"/path"); !ok || got != "https://example.invalid/path" {
				t.Fatalf("existing endpoint resolution = %q, %v", got, ok)
			}
		})
	}
	t.Run("remoteSite", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "Remote.remoteSite-meta.xml")
		writeFile(t, path, `<RemoteSiteSetting><url>https://example.invalid</url></RemoteSiteSetting>`)
		org := storage.NewOrgState()
		if err := ApplyProject(&org, project.Project{Root: root, RemoteSiteFiles: []string{path}}); err != nil {
			t.Fatal(err)
		}
		if got := org.Objects["NamedCredential"].Records; len(got) != 0 {
			t.Fatalf("remote site created NamedCredential rows: %#v", got)
		}
	})
}

func TestApplyProjectNamedCredentialProjectionPreservesExistingRows(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "NamedCredential")
	object := org.Objects["NamedCredential"]
	for index, name := range []string{"ProbeEndpoint", "UnrelatedEndpoint"} {
		id := storage.ID(object.Definition.KeyPrefix + leftPad(index+1, 12))
		object.Records[id] = storage.Record{ID: id, Object: "NamedCredential", Fields: map[string]storage.Value{
			"Id":              storage.IDValue(id),
			"DeveloperName":   storage.StringValue(name),
			"Endpoint":        storage.StringValue("https://existing.invalid"),
			"NamespacePrefix": storage.NullValue(),
		}}
	}
	org.Objects["NamedCredential"] = object
	before := object.Clone().Records
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if got := org.Objects["NamedCredential"].Records; !reflect.DeepEqual(got, before) {
		t.Fatalf("existing NamedCredential rows changed: got %#v, want %#v", got, before)
	}
}

func namedCredentialProjectionProject(t *testing.T, namespace, name, contents string) project.Project {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/namedCredentials", name+".namedCredential-meta.xml")
	writeFile(t, path, `<NamedCredential xmlns="http://soap.sforce.com/2006/04/metadata">`+contents+`</NamedCredential>`)
	return project.Project{Root: root, Namespace: namespace, NamedCredentialFiles: []string{path}}
}
