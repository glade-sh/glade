package resource

import (
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"path/filepath"
	"testing"
)

// Native: csnpe_capture/org.tsv:9 and :27; the label and principal are queryable.
func TestLegacyCredentialProjectionUsesCapturedMetadata(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Changed</label><endpoint>https://changed.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	captured := []byte(`<NamedCredential><label>Probe endpoint</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType></NamedCredential>`)
	read := func(path string) ([]byte, error) {
		if filepath.Clean(path) == filepath.Clean(p.NamedCredentialFiles[0]) {
			return captured, nil
		}
		return nil, fmt.Errorf("unexpected metadata path")
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p, read); err != nil {
		t.Fatal(err)
	}
	for _, row := range org.Objects["NamedCredential"].Records {
		if row.Fields["Endpoint"].String != "https://example.invalid" || row.Fields["MasterLabel"].String != "Probe endpoint" {
			t.Fatalf("snapshot row: %#v", row.Fields)
		}
		return
	}
	t.Fatal("missing credential")
}

func TestLegacyCredentialProjectionUpgradesPriorRow(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Probe endpoint</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "NamedCredential")
	obj := org.Objects["NamedCredential"]
	id := storage.ID(obj.Definition.KeyPrefix + "000000000001")
	obj.Records = map[storage.ID]storage.Record{id: {ID: id, Object: "NamedCredential", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("ProbeEndpoint"), "Endpoint": storage.StringValue("https://example.invalid"), "NamespacePrefix": storage.NullValue()}}}
	org.Objects["NamedCredential"] = obj
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	row := org.Objects["NamedCredential"].Records[id]
	if row.Fields["MasterLabel"].String != "Probe endpoint" || row.Fields["PrincipalType"].String != "Anonymous" {
		t.Fatalf("prior row not upgraded: %#v", row.Fields)
	}
}

func TestLegacyCredentialProjectionRefreshesChangedLabel(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Probe endpoint</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	writeFile(t, p.NamedCredentialFiles[0], `<NamedCredential><label>Updated endpoint</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType></NamedCredential>`)
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for _, row := range org.Objects["NamedCredential"].Records {
		if row.Fields["MasterLabel"].String != "Updated endpoint" {
			t.Fatalf("stale label: %#v", row.Fields)
		}
		return
	}
	t.Fatal("missing credential")
}

func TestLegacyCredentialProjectionDoesNotRetainAnonymousFieldsAfterPrincipalChange(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Probe endpoint</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	writeFile(t, p.NamedCredentialFiles[0], `<NamedCredential><label>Probe endpoint</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>NamedUser</principalType></NamedCredential>`)
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for _, row := range org.Objects["NamedCredential"].Records {
		if _, ok := row.Fields["PrincipalType"]; ok {
			t.Fatalf("stale anonymous principal: %#v", row.Fields)
		}
		if _, ok := row.Fields["MasterLabel"]; ok {
			t.Fatalf("uncaptured NamedUser label retained: %#v", row.Fields)
		}
		return
	}
	t.Fatal("missing credential")
}
