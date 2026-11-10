package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The native inventory records built-in profile/license pairs and omits the
// synthetic guest profile in a plain Developer org. Reapplying the shape to a
// warm seed must produce the same result.
func TestProfileInventoryPlainShapeNative(t *testing.T) {
	shape := []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape}
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	ApplyProjectOrgShape(&org, shape)
	EnsureDeterministicPlatformData(&org)
	for _, pass := range []string{"cold", "warm"} {
		t.Run(pass, func(t *testing.T) {
			if pass == "warm" {
				ApplyProjectOrgShape(&org, shape)
				EnsureDeterministicPlatformData(&org)
			}
			want := map[string]string{"Standard Platform User": "Salesforce Platform", "Customer Community Login User": "Customer Community Login"}
			seen := map[string]bool{}
			for _, profile := range org.Objects["Profile"].Records {
				name := profile.Fields["Name"].String
				if name == "Customer Community Guest User" {
					t.Error("uncaptured synthetic guest profile present in native plain inventory")
				}
				if expected, ok := want[name]; ok {
					seen[name] = true
					licenseID := profile.Fields["UserLicenseId"].ID
					license := org.Objects["UserLicense"].Records[licenseID]
					if got := license.Fields["Name"].String; got != expected {
						t.Errorf("%s license = %q, native %q; fields=%+v; untouched=%t", name, got, expected, profile.Fields, untouchedBootstrapProfile(profile.ID, profile))
					}
				}
			}
			for name := range want {
				if !seen[name] {
					t.Errorf("native profile %s missing", name)
				}
			}
		})
	}
}
