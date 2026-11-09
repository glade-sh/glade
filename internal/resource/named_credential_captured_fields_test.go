package resource

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native: csnpe_capture/org.tsv:9,18,27,36 (legacy anonymous metadata).
func TestLegacyNamedCredentialCapturedFields(t *testing.T) {
	p := namedCredentialProjectionProject(t, "", "ProbeEndpoint", `<label>Probe endpoint</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType>`)
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for _, row := range org.Objects["NamedCredential"].Records {
		if row.Fields["MasterLabel"].String != "Probe endpoint" || row.Fields["PrincipalType"].String != "Anonymous" {
			t.Fatalf("captured fields: %#v", row.Fields)
		}
		return
	}
	t.Fatal("missing named credential row")
}
