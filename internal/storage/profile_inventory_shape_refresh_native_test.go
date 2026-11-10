package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"strings"
	"testing"
)

// standard_profile_inventory/org.tsv:2-49 captures the profile/license pairs
// in plain and Communities Developer shapes at APIs 62 and 67.
func TestProfileInventoryShapeRefreshNative(t *testing.T) {
	for _, features := range [][]string{{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape}, {project.StandardFieldEditionDeveloper, "NetworksEnabled"}, {project.StandardFieldEditionDeveloper, "Communities", "NetworksEnabled"}} {
		t.Run(strings.Join(features, "/"), func(t *testing.T) {
			org := NewOrgState()
			EnsureDeterministicPlatformData(&org)
			ApplyProjectOrgShape(&org, features)
			want := map[string]string{"Standard Platform User": "Salesforce Platform", "Customer Community Login User": "Customer Community Login"}
			seen := map[string]bool{}
			for _, profile := range org.Objects["Profile"].Records {
				name := profile.Fields["Name"].String
				if expected, ok := want[name]; ok {
					seen[name] = true
					license := org.Objects["UserLicense"].Records[profile.Fields["UserLicenseId"].ID]
					if got := license.Fields["Name"].String; got != expected {
						t.Errorf("%s license=%q native=%q immediately after shape refresh", name, got, expected)
					}
				}
			}
			for name := range want {
				if !seen[name] {
					t.Errorf("missing profile %s", name)
				}
			}
		})
	}
}
