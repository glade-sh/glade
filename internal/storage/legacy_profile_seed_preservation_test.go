package storage

import "testing"

// These cases preserve local persisted-seed behavior; they make no claim about
// Salesforce's treatment of user-created profiles or existing guest users.
func TestLegacyProfileSeedPreservesReferencedAndCustomizedRows(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	const guestID ID = "00e000000000007"
	const chatterID ID = "00e000000000003"
	const platformID ID = "00e000000000004"
	const customLicenseID ID = "100000000000091"
	org.Objects["Profile"].Records[guestID] = Record{ID: guestID, Object: "Profile", Fields: map[string]Value{"Name": StringValue("Customer Community Guest User"), "UserType": StringValue("Guest"), "UserLicenseId": IDValue(customLicenseID)}}
	org.Objects["User"].Records[ID("005000000000091")] = Record{ID: ID("005000000000091"), Object: "User", Fields: map[string]Value{"ProfileId": IDValue(guestID)}}
	chatter := org.Objects["Profile"].Records[chatterID]
	chatter.Fields["UserLicenseId"] = IDValue(customLicenseID)
	org.Objects["Profile"].Records[chatterID] = chatter
	platform := org.Objects["Profile"].Records[platformID]
	platform.Fields["UserLicenseId"] = IDValue(customLicenseID)
	org.Objects["Profile"].Records[platformID] = platform
	EnsureDeterministicPlatformData(&org)
	guest, ok := org.Objects["Profile"].Records[guestID]
	if !ok || guest.Fields["Name"].String != "Customer Community Guest User" || !IDsEqual(guest.Fields["UserLicenseId"].ID, customLicenseID) {
		t.Fatalf("referenced guest changed: %+v", guest)
	}
	for _, id := range []ID{chatterID, platformID} {
		row := org.Objects["Profile"].Records[id]
		if !IDsEqual(row.Fields["UserLicenseId"].ID, customLicenseID) {
			t.Errorf("customized license on %s changed: %+v", id, row)
		}
	}
}

// In the base implementation uncaptured guests did not acquire either field.
func TestUncapturedGuestManagePermissionFieldsKeepBaseline(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	ensureSitesGuestProfiles(&org)
	seen := 0
	for _, profile := range org.Objects["Profile"].Records {
		name := profile.Fields["Name"].String
		if name != "Standard Guest" && name != "Guest License User" {
			continue
		}
		seen++
		for _, field := range []string{"PermissionsManageUsers", "PermissionsManageInternalUsers"} {
			if value, ok := profile.Fields[field]; ok {
				t.Errorf("%s unexpectedly contains %s=%v", name, field, value)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("found %d guest profiles", seen)
	}
}
