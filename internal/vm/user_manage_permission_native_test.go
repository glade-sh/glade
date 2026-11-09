package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// TestUserManagePermissionNative replays the observed cross-user update outcomes.
func TestUserManagePermissionNative(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/default_user_review/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, row := range rows[1:] {
		if row[1] != "istest" {
			continue
		}
		row := row
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			machine := New(nil)
			machine.SetOrg(&org)
			fields := map[string]storage.Value{}
			if strings.HasSuffix(row[2], "/privileged") {
				if !strings.HasPrefix(row[2], "manage-internal-users/") {
					fields["PermissionsModifyAllData"] = storage.BooleanValue(true)
				}
				if strings.Contains(row[2], "manage-internal-users/") {
					fields["PermissionsManageInternalUsers"] = storage.BooleanValue(true)
				}
				if strings.HasPrefix(row[2], "modify-all-and-manage-users/") {
					fields["PermissionsManageUsers"] = storage.BooleanValue(true)
				}
			}
			machine.SetCurrentUser(storage.Record{ID: "005000000000123", Object: "User", Fields: fields})
			target := storage.Record{ID: "005000000000124", Object: "User"}
			got := machine.currentUserCanWriteRecord("User", target, "005000000000123", "update")
			want := strings.Contains(row[3], "CAP|ok|Changed")
			if got != want {
				t.Fatalf("native=%s; allowed=%v", row[3], got)
			}
			if !machine.currentUserCanWriteRecord("User", storage.Record{ID: "005000000000123", Object: "User"}, "005000000000123", "update") {
				t.Fatal("self update denied")
			}
		})
		compared++
	}
	if compared != 16 {
		t.Fatalf("compared %d rows, want 16", compared)
	}
}
