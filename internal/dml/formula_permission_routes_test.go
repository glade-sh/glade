package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These synthetic fixture fields must agree with FeatureManagement's local
// permission lookup; they are not native Salesforce metadata shapes.
func TestFormulaPermissionSyntheticGrantRoutes(t *testing.T) {
	for _, tc := range []struct{ object, field string }{
		{"User", "Permissions"}, {"User", "PermissionSets"},
		{"PermissionSet", "Permissions"}, {"PermissionSet", "PermissionSets"}, {"PermissionSet", "CustomPermissions"},
	} {
		t.Run(tc.object+"/"+tc.field, func(t *testing.T) {
			org := permissionRegexOrg(true, "Other")
			id := permTestUser
			if tc.object == "PermissionSet" {
				id = permTestSet
			}
			state := org.Objects[tc.object]
			record := state.Records[id]
			record.Fields[tc.field] = storage.StringValue("Bypass")
			state.Records[id] = record
			org.Objects[tc.object] = state
			p := formulaParser{org: &org, userID: permTestUser}
			if !p.customPermissionGranted("Bypass") {
				t.Fatal("fixture grant not visible to formula")
			}
			if p.customPermissionGranted("Missing") {
				t.Fatal("unrelated permission granted")
			}
		})
	}
}
