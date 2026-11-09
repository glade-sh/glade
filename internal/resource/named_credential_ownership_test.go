package resource

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestLegacyCredentialProjectionPreservesOtherProjectOwner(t *testing.T) {
	first := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>First</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	second := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Second</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, first); err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, second); err != nil {
		t.Fatal(err)
	}
	for _, row := range org.Objects["NamedCredential"].Records {
		if row.Fields["MasterLabel"].String != "First" {
			t.Fatalf("credential from another project was overwritten: %#v", row.Fields)
		}
		return
	}
	t.Fatal("missing credential")
}

func TestLegacyCredentialProjectionRemovesDeletedSource(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Probe</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p.NamedCredentialFiles[0]); err != nil {
		t.Fatal(err)
	}
	p.NamedCredentialFiles = nil
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if rows := org.Objects["NamedCredential"].Records; len(rows) != 0 {
		t.Fatalf("credential survived deleted metadata: %#v", rows)
	}
}

func TestLegacyCredentialProjectionConvertsToSecuredEndpoint(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Probe</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	writeFile(t, p.NamedCredentialFiles[0], `<NamedCredential><label>Secured probe</label><namedCredentialType>SecuredEndpoint</namedCredentialType></NamedCredential>`)
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for _, row := range org.Objects["NamedCredential"].Records {
		if row.Fields["MasterLabel"].String != "Secured probe" || row.Fields["Endpoint"].Kind != "null" {
			t.Fatalf("credential retained legacy fields after conversion: %#v", row.Fields)
		}
		return
	}
	t.Fatal("missing secured credential")
}

func TestSecuredCredentialProjectionRemovesDeletedSource(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Secured probe</label><namedCredentialType>SecuredEndpoint</namedCredentialType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["NamedCredential"].Records) != 1 {
		t.Fatal("missing secured credential")
	}
	if err := os.Remove(p.NamedCredentialFiles[0]); err != nil {
		t.Fatal(err)
	}
	p.NamedCredentialFiles = nil
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if rows := org.Objects["NamedCredential"].Records; len(rows) != 0 {
		t.Fatalf("secured credential survived deleted metadata: %#v", rows)
	}
}

func TestNamedCredentialProjectionRemovesSourceAfterProjectMove(t *testing.T) {
	for _, contents := range []string{`<label>Probe</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`, `<label>Secured probe</label><namedCredentialType>SecuredEndpoint</namedCredentialType>`} {
		t.Run(contents[:12], func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "before")
			path := filepath.Join(root, "Probe.namedCredential-meta.xml")
			writeFile(t, path, `<NamedCredential>`+contents+`</NamedCredential>`)
			p := project.Project{Root: root, NamedCredentialFiles: []string{path}}
			org := storage.NewOrgState()
			if err := ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			if len(org.Objects["NamedCredential"].Records) != 1 {
				t.Fatal("missing credential")
			}
			moved := filepath.Join(filepath.Dir(root), "after")
			if err := os.Rename(root, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(moved, filepath.Base(path))); err != nil {
				t.Fatal(err)
			}
			p.Root = moved
			p.NamedCredentialFiles = nil
			if err := ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			if len(org.Objects["NamedCredential"].Records) != 0 {
				t.Fatal("credential survived moved project deletion")
			}
		})
	}
}

func TestNamedCredentialProjectionMigratesPathBasedOwnership(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Before</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(filepath.Clean(p.Root)))
	legacy := fmt.Sprintf("glade:legacy-named-credential:%x", digest)
	object := org.Objects["NamedCredential"]
	for id, row := range object.Records {
		row.MetadataSourceUnit = legacy
		object.Records[id] = row
	}
	org.Objects["NamedCredential"] = object
	writeFile(t, p.NamedCredentialFiles[0], `<NamedCredential><label>After</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType></NamedCredential>`)
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for _, row := range org.Objects["NamedCredential"].Records {
		if row.MetadataSourceUnit == legacy || row.Fields["MasterLabel"].String != "After" {
			t.Fatalf("legacy credential not migrated: %#v", row)
		}
		return
	}
	t.Fatal("credential missing")
}
