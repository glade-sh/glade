package storage

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

// The captured rows come from a fresh scratch org (see ORG-RUN.txt). A seeded
// profile and its profile-owned permission set carry the observed flags.
func TestSeededProfilePermissionFlagsMatchNativeRows(t *testing.T) {
	file, err := os.Open(filepath.Join("../apextest/testdata/conformance/profile_permissions/native/org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	file.Close()
	if err != nil || len(rows) < 17 {
		t.Fatalf("incomplete native matrix: %v rows=%d", err, len(rows))
	}
	flags := rows[0][2:]
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	profileIDs := map[string]ID{}
	for _, record := range org.Objects["Profile"].Records {
		profileIDs[record.Fields["Name"].String] = record.ID
	}
	for _, row := range rows[1:] {
		name := row[1]
		id, ok := profileIDs[name]
		if !ok {
			t.Fatalf("profile %s is not seeded", name)
		}
		t.Run(row[0]+"/"+name, func(t *testing.T) {
			for i, flag := range flags {
				want := row[2+i] == "1"
				field := "Permissions" + flag
				got := org.Objects["Profile"].Records[id].Fields[field]
				if got.Kind != ValueBoolean || got.Boolean != want {
					t.Errorf("Profile.%s = %v, native %v", field, got, want)
				}
				for _, ps := range org.Objects["PermissionSet"].Records {
					if ps.Fields["ProfileId"].String == string(id) {
						got := ps.Fields[field]
						if got.Kind != ValueBoolean || got.Boolean != want {
							t.Errorf("PermissionSet(profile %s).%s = %v, native %v", name, field, got, want)
						}
					}
				}
			}
		})
	}
}

// No native row covers the guest profiles, so their seed stays as it was
// before the capture: four explicit false flags, nothing else claimed.
func TestUncapturedGuestProfilesKeepLegacyPermissionSeed(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	ensureSitesGuestProfiles(&org)
	seen := 0
	for _, record := range org.Objects["Profile"].Records {
		name := record.Fields["Name"].String
		if name != "Standard Guest" && name != "Guest License User" {
			continue
		}
		seen++
		for _, flag := range []string{"CustomizeApplication", "AuthorApex", "ViewAllData"} {
			if _, present := record.Fields["Permissions"+flag]; present {
				t.Errorf("%s carries unbacked Permissions%s", name, flag)
			}
		}
		for _, flag := range uncapturedProfileLegacyFlags {
			if v := record.Fields["Permissions"+flag]; v.Kind != ValueBoolean || v.Boolean {
				t.Errorf("%s Permissions%s = %v, want legacy false", name, flag, v)
			}
		}
	}
	if seen != 2 {
		t.Fatal("no guest profile seeded")
	}
}

// profile_usertype P1 (API 62.0 and 67.0): the hand-seeded profiles link to
// the native license names and keys, and the fresh org has no Customer
// Community Guest User. An org persisted with the former seed is repaired.
func TestSeededProfileLicensesFollowProfileUserTypeRows(t *testing.T) {
	want := map[string][2]string{
		"Chatter External User":         {"Chatter External", "CSN_External_User"},
		"Customer Community Login User": {"Customer Community Login", "PID_Customer_Community_Login"},
		"Standard Platform User":        {"Salesforce Platform", "AUL"},
		"Standard User":                 {"Salesforce", "SFDC"},
	}
	check := func(t *testing.T, org OrgState) {
		t.Helper()
		for name, license := range want {
			profile, ok := findRecordByStringField(org.Objects["Profile"].Records, "Name", name)
			if !ok {
				t.Fatalf("missing profile %s", name)
			}
			got := org.Objects["UserLicense"].Records[profile.Fields["UserLicenseId"].ID]
			if got.Fields["Name"].String != license[0] || got.Fields["LicenseDefinitionKey"].String != license[1] {
				t.Errorf("%s license = %q/%q, want %q/%q", name, got.Fields["Name"].String, got.Fields["LicenseDefinitionKey"].String, license[0], license[1])
			}
		}
		if profile, ok := findRecordByStringField(org.Objects["Profile"].Records, "Name", "Customer Community Guest User"); ok {
			t.Errorf("unexpected Customer Community Guest User: %#v", profile)
		}
		if n := len(org.Objects["UserLicense"].Records); n != 31 {
			t.Errorf("UserLicense rows = %d, want 31", n)
		}
	}
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	check(t, org)

	t.Run("legacy", func(t *testing.T) {
		legacy := NewOrgState()
		EnsureDeterministicPlatformData(&legacy)
		licenses := legacy.Objects["UserLicense"]
		license := licenses.Records["100000000000002"]
		license.Fields["Name"] = StringValue("Chatter External")
		licenses.Records["100000000000002"] = license
		profiles := legacy.Objects["Profile"]
		chatter := profiles.Records["00e000000000003"]
		chatter.Fields["UserLicenseId"] = IDValue("100000000000002")
		platform := profiles.Records["00e000000000004"]
		platform.Fields["UserLicenseId"] = IDValue("100000000000001")
		profiles.Records["00e000000000007"] = Record{ID: "00e000000000007", Object: "Profile", Fields: map[string]Value{
			"Name": StringValue("Customer Community Guest User"), "UserType": StringValue("Guest"), "UserLicenseId": IDValue("100000000000002"),
		}}
		EnsureDeterministicPlatformData(&legacy)
		check(t, legacy)
	})
}
