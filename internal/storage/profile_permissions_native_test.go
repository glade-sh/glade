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
	seen := 0
	for _, record := range org.Objects["Profile"].Records {
		name := record.Fields["Name"].String
		if name != "Standard Guest" && name != "Guest License User" && name != "Customer Community Guest User" {
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
	if seen == 0 {
		t.Fatal("no guest profile seeded")
	}
}
