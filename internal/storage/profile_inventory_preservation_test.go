package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Project-supplied profile edits and an unspecified org shape must survive a
// native baseline refresh.
func TestProfileInventoryPreservesExplicitProfileData(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	var target ID
	for id, profile := range org.Objects["Profile"].Records {
		if profile.Fields["Name"].String == "Standard Platform User" {
			target = id
			break
		}
	}
	if target == "" {
		t.Fatal("standard profile missing")
	}
	profile := org.Objects["Profile"].Records[target]
	original := profile.Fields["UserLicenseId"]
	profile.Fields["Description"] = StringValue("project override")
	org.Objects["Profile"].Records[target] = profile
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
	EnsureDeterministicPlatformData(&org)
	got := org.Objects["Profile"].Records[target]
	if got.Fields["Description"].String != "project override" || got.Fields["UserLicenseId"].ID != original.ID {
		t.Fatalf("explicit profile data changed: %+v", got.Fields)
	}
}
