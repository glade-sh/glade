package storage

import "testing"

func TestAdminProfileModifyAllDataSeed(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	adminCount := 0
	nonAdminCount := 0
	for _, record := range org.Objects["Profile"].Records {
		name, _ := record.GetField("Name")
		permission, _ := record.GetField("PermissionsModifyAllData")
		if name.String == "System Administrator" {
			adminCount++
			userType, _ := record.GetField("UserType")
			if permission.Kind != ValueBoolean || !permission.Boolean || userType.String != "Standard" {
				t.Fatalf("admin profile=%+v", record)
			}
		} else {
			nonAdminCount++
			if permission.Kind == ValueBoolean && permission.Boolean {
				t.Fatalf("nonadmin granted modify-all: %+v", record)
			}
		}
	}
	if adminCount != 1 || nonAdminCount == 0 {
		t.Fatalf("profile populations admin=%d other=%d", adminCount, nonAdminCount)
	}
	// Existing captured/configured profiles must not be elevated by seed refresh.
	for id, record := range org.Objects["Profile"].Records {
		name, _ := record.GetField("Name")
		if name.String != "System Administrator" {
			continue
		}
		record.Fields["PermissionsModifyAllData"] = BooleanValue(false)
		org.Objects["Profile"].Records[id] = record
		EnsureDeterministicPlatformData(&org)
		updated := org.Objects["Profile"].Records[id]
		permission, _ := updated.GetField("PermissionsModifyAllData")
		if permission.Boolean {
			t.Fatal("seed refresh overwrote explicit existing permission")
		}
	}
}
