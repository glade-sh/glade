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

func legacyGuestOrg(license ID) (OrgState, Record) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	const guestID ID = "00e000000000007"
	guest := Record{ID: guestID, Object: "Profile", Fields: map[string]Value{"Name": StringValue("Customer Community Guest User"), "UserType": StringValue("Guest"), "UserLicenseId": IDValue(license)}}
	org.Objects["Profile"].Records[guestID] = guest
	return org, guest
}

func TestLegacyGuestProfileKeepsCustomizedOrReferenced(t *testing.T) {
	const guestID ID = "00e000000000007"
	const seededLicense ID = "100000000000002"
	cases := map[string]func(*OrgState){
		"license": func(org *OrgState) {
			g := org.Objects["Profile"].Records[guestID]
			g.Fields["UserLicenseId"] = IDValue("100000000000091")
			org.Objects["Profile"].Records[guestID] = g
		},
		"permission": func(org *OrgState) {
			g := org.Objects["Profile"].Records[guestID]
			g.Fields["PermissionsEditTask"] = BooleanValue(true)
			org.Objects["Profile"].Records[guestID] = g
		},
		"other object": func(org *OrgState) {
			org.Objects["Account"].Records["001000000000091"] = Record{ID: "001000000000091", Object: "Account", Fields: map[string]Value{"Ref": IDValue(guestID)}}
		},
	}
	for name, customize := range cases {
		t.Run(name, func(t *testing.T) {
			org, _ := legacyGuestOrg(seededLicense)
			if _, ok := org.Objects["Account"]; !ok {
				EnsureStandardObject(&org, "Account")
			}
			customize(&org)
			EnsureDeterministicPlatformData(&org)
			if _, ok := org.Objects["Profile"].Records[guestID]; !ok {
				t.Fatal("customized or referenced guest was deleted")
			}
		})
	}
	t.Run("untouched is removed", func(t *testing.T) {
		org, _ := legacyGuestOrg(seededLicense)
		EnsureDeterministicPlatformData(&org)
		if _, ok := org.Objects["Profile"].Records[guestID]; ok {
			t.Fatal("untouched legacy guest kept")
		}
	})
}

func TestInventoryLicenseIDCollisionSelectsFreeID(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	var key = "CSN_External_User"
	var want ID
	for id, l := range org.Objects["UserLicense"].Records {
		if l.Fields["LicenseDefinitionKey"].String == key {
			want = id
		}
	}
	// Re-key the row at its inventory ID, then ask for the key again.
	l := org.Objects["UserLicense"].Records[want]
	l.Fields["LicenseDefinitionKey"] = StringValue("other-key")
	org.Objects["UserLicense"].Records[want] = l
	got := ensureInventoryUserLicense(&org, "Chatter External", key)
	if got == want {
		t.Fatalf("returned occupied id %s", got)
	}
	if org.Objects["UserLicense"].Records[got].Fields["LicenseDefinitionKey"].String != key {
		t.Fatalf("license not installed at %s", got)
	}
	if org.Objects["UserLicense"].Records[want].Fields["LicenseDefinitionKey"].String != "other-key" {
		t.Fatal("occupant changed")
	}
}
