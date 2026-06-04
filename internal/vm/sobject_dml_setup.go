package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func isSetupObject(objectName string) bool {
	switch strings.ToLower(objectName) {
	case "user", "profile", "userrole", "permissionset", "permissionsetassignment", "permissionsetgroup", "permissionsetgroupcomponent", "fieldpermissions", "objectpermissions", "setupentityaccess":
		return true
	default:
		return false
	}
}

func isSetupDMLRecord(record storage.Record) bool {
	return mixedDMLRecordKind(record) == "setup"
}

func mixedDMLRecordKind(record storage.Record) string {
	if !isSetupObject(record.Object) {
		return "nonsetup"
	}
	if !strings.EqualFold(record.Object, "User") {
		return "setup"
	}
	roleID, ok := record.GetField("UserRoleId")
	if !ok || roleID.Kind == storage.ValueNull || storageIDFromValue(roleID) == "" {
		return "neutral"
	}
	return "setup"
}
