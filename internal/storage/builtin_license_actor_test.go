package storage

import (
	"reflect"
	"testing"
)

// The owned API62/64/67 anonymous and named observations establish these linked
// built-in identities. PermissionSet and ObjectPermissions rows were not
// observed, so this fixture regression does not prescribe such records.
func TestEnsureDeterministicPlatformDataBuiltinLicenseActor(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	profile, found := findRecordByStringField(org.Objects["Profile"].Records, "Name", "Chatter Free User")
	if !found {
		t.Fatal("missing captured built-in profile")
	}
	count := 0
	for _, record := range org.Objects["Profile"].Records {
		if record.Fields["Name"].String == "Chatter Free User" {
			count++
		}
	}
	if count != 1 || profile.Fields["UserType"].String != "CsnOnly" {
		t.Fatalf("captured profile count/type = %d/%q", count, profile.Fields["UserType"].String)
	}
	licenseID := profile.Fields["UserLicenseId"].ID
	license, found := org.Objects["UserLicense"].Records[licenseID]
	if !found || license.Fields["Name"].String != "Chatter Free" || license.Fields["LicenseDefinitionKey"].String != "CSN_User" {
		t.Fatalf("captured linked license = %#v, found %v", license, found)
	}
	for _, objectName := range []string{"Profile", "UserLicense"} {
		for id := range org.Objects[objectName].Records {
			if err := ValidateID(id); err != nil {
				t.Fatalf("invalid fixture identity for %s: %v", objectName, err)
			}
		}
	}
	// Fixture IDs and sequences are local implementation details. Repeated
	// seeding must preserve the whole org, including existing identities.
	before := FixtureFromOrg(org)
	EnsureDeterministicPlatformData(&org)
	if after := FixtureFromOrg(org); !reflect.DeepEqual(before, after) {
		t.Fatal("repeated platform seeding changed the fixture")
	}
}
