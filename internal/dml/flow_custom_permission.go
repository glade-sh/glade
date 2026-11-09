package dml

import (
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// flowCustomPermission resolves the captured direct-assignment grant states.
// Missing related metadata and uncaptured assignment states stay unsupported.
func (e *Engine) flowCustomPermission(name string) (bool, bool) {
	if e == nil || e.Org == nil || strings.TrimSpace(name) == "" {
		return false, false
	}
	if e.UserHasCustomPermission != nil && e.UserHasCustomPermission(name) {
		return true, true
	}
	userID := e.systemUserID()
	unknown := false
	for _, assignment := range e.Org.Objects["PermissionSetAssignment"].Records {
		if assignment.System.IsDeleted || !storage.IDsEqual(idFromStorageValue(assignment.Fields["AssigneeId"]), userID) {
			continue
		}
		if group := idFromStorageValue(assignment.Fields["PermissionSetGroupId"]); group != "" {
			unknown = true
			continue
		}
		id := idFromStorageValue(assignment.Fields["PermissionSetId"])
		if id == "" {
			unknown = true
			continue
		}
		stored, set, found := storage.LookupRecordByID(e.Org.Objects["PermissionSet"].Records, id)
		if !found {
			unknown = true
			continue
		}
		if set.System.IsDeleted {
			continue
		}
		grants := false
		for _, field := range []string{"Permissions", "PermissionSets", "CustomPermissions"} {
			if value, exists := set.GetField(field); exists && value.Kind != storage.ValueNull {
				if !flowPermissionValueWellFormed(value) {
					unknown = true
				} else if flowPermissionValueMatches(value, name) {
					grants = true
				}
			}
		}
		for _, access := range e.Org.Objects["SetupEntityAccess"].Records {
			if access.System.IsDeleted {
				continue
			}
			parent := idFromStorageValue(access.Fields["ParentId"])
			if parent == "" {
				unknown = true
				continue
			}
			if !storage.IDsEqual(parent, stored) {
				continue
			}
			entity := idFromStorageValue(access.Fields["SetupEntityId"])
			if entity == "" {
				unknown = true
				continue
			}
			_, permission, found := storage.LookupRecordByID(e.Org.Objects["CustomPermission"].Records, entity)
			if !found {
				prefix := storage.StandardKeyPrefix("CustomPermission")
				if prefix == "" || strings.HasPrefix(string(entity), prefix) {
					unknown = true
				}
				continue
			}
			if permission.System.IsDeleted {
				continue
			}
			permissionName, exists := permission.GetField("DeveloperName")
			if !exists || permissionName.Kind != storage.ValueString || strings.TrimSpace(permissionName.String) == "" {
				unknown = true
				continue
			}
			if flowPermissionValueMatches(permissionName, name) {
				grants = true
			}
		}
		if !grants {
			continue
		}
		active, known := PermissionAssignmentState(assignment, e.Now)
		if !known {
			unknown = true
			continue
		}
		if active {
			return true, true
		}
	}
	return false, !unknown
}

// PermissionAssignmentState evaluates the captured direct-assignment states.
// Uncaptured or malformed states remain unknown.
func PermissionAssignmentState(assignment storage.Record, now func() time.Time) (bool, bool) {
	inactive := false
	if revoked, exists := assignment.GetField("IsRevoked"); exists && revoked.Kind != storage.ValueNull {
		if revoked.Kind != storage.ValueBoolean {
			return false, false
		}
		inactive = revoked.Boolean
	}
	if expiration, exists := assignment.GetField("ExpirationDate"); exists && expiration.Kind != storage.ValueNull {
		if expiration.Kind != storage.ValueDateTime || now == nil {
			return false, false
		}
		expires, err := time.Parse(time.RFC3339Nano, expiration.String)
		if err != nil {
			return false, false
		}
		current := now().UTC()
		if expires.Equal(current) {
			return false, false
		}
		if expires.Before(current) {
			inactive = true
		}
	}
	if active, exists := assignment.GetField("IsActive"); exists && active.Kind != storage.ValueNull {
		if active.Kind != storage.ValueBoolean || (!active.Boolean && !inactive) {
			return false, false
		}
	}
	return !inactive, true
}

func flowPermissionValueMatches(value storage.Value, name string) bool {
	if value.Kind == storage.ValueString {
		return strings.EqualFold(strings.TrimSpace(value.String), strings.TrimSpace(name))
	}
	if value.Kind == storage.ValueList {
		for _, item := range value.List {
			if flowPermissionValueMatches(item, name) {
				return true
			}
		}
	}
	return false
}

func flowPermissionValueWellFormed(value storage.Value) bool {
	if value.Kind == storage.ValueString {
		return strings.TrimSpace(value.String) != ""
	}
	if value.Kind == storage.ValueList {
		for _, item := range value.List {
			if !flowPermissionValueWellFormed(item) {
				return false
			}
		}
		return true
	}
	return false
}
