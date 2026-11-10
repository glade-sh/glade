package storage

import (
	"reflect"
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

func TestProfileInventoryPreservesEditedBootstrapFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		id    ID
		field string
		value Value
		// legacy starts from an org persisted with the former guest seed;
		// profile_usertype rows keep the guest out of a fresh seed.
		legacy bool
	}{
		{"renamed unreferenced", ID("00e000000000007"), "Name", StringValue("Project Guest"), true},
		{"changed license", ID("00e000000000004"), "UserLicenseId", IDValue(ID("100000000000002")), false},
		{"changed user type", ID("00e000000000004"), "UserType", StringValue("ProjectType"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			EnsureDeterministicPlatformData(&org)
			if tc.legacy {
				org, _ = legacyGuestOrg(ID("100000000000002"))
			}
			profile, ok := org.Objects["Profile"].Records[tc.id]
			if !ok {
				t.Fatal("bootstrap profile missing")
			}
			profile.Fields[tc.field] = tc.value
			want := profile.Clone().Fields
			org.Objects["Profile"].Records[tc.id] = profile
			ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
			EnsureDeterministicPlatformData(&org)
			got, ok := org.Objects["Profile"].Records[tc.id]
			if !ok {
				t.Fatal("edited profile was removed")
			}
			if !reflect.DeepEqual(got.Fields, want) {
				t.Fatalf("edited profile fields changed: got %+v want %+v", got.Fields, want)
			}
		})
	}
}
