package storage

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

// The capture records explicit Boolean values for seeded Developer profiles at
// API 62 and 67; it does not assert the values for user-created profiles.
func TestSeededManageUserPermissionsMatchNativeRows(t *testing.T) {
	file, err := os.Open(filepath.Join("../apextest/testdata/native-capture/user_manage_users/permission_values/org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	r := csv.NewReader(file)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 33 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	checked := 0
	for _, row := range rows[1:] {
		profile, ok := findRecordByStringField(org.Objects["Profile"].Records, "Name", row[2])
		if !ok {
			t.Fatalf("missing seeded profile %s", row[2])
		}
		var record Record
		switch row[0] {
		case "profile":
			record = profile
		case "permissions":
			found := false
			for _, candidate := range org.Objects["PermissionSet"].Records {
				if IDsEqual(candidate.Fields["ProfileId"].ID, profile.ID) {
					record = candidate
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing owned permission set for %s", row[2])
			}
		default:
			t.Fatalf("unknown source %q", row[0])
		}
		for i, name := range []string{"PermissionsManageUsers", "PermissionsManageInternalUsers"} {
			value := record.Fields[name]
			if value.Kind != ValueBoolean || value.Boolean != (row[4+i] == "true") {
				t.Errorf("%s %s API %s: %s = %v, native %s", row[0], row[2], row[1], name, value, row[4+i])
			}
		}
		checked++
	}
	if checked != 32 {
		t.Fatalf("compared %d rows, want 32", checked)
	}
}
