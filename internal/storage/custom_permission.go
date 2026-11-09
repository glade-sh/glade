package storage

import "strings"

// CustomPermissionGranted resolves both synthetic fixture grants and metadata
// grants for a user. extraUserFields carries an unsaved execution-user snapshot.
func CustomPermissionGranted(org *OrgState, userID ID, extraUserFields map[string]Value, permission string) bool {
	return CustomPermissionGrantedWhere(org, userID, extraUserFields, permission, nil)
}

// CustomPermissionGrantedWhere is CustomPermissionGranted restricted to the
// PermissionSetAssignment records for which assignmentActive reports true; a
// nil predicate accepts every undeleted assignment.
func CustomPermissionGrantedWhere(org *OrgState, userID ID, extraUserFields map[string]Value, permission string, assignmentActive func(Record) bool) bool {
	matches := func(fields map[string]Value, names ...string) bool {
		record := Record{Fields: fields}
		for _, name := range names {
			if value, ok := record.GetField(name); ok && permissionValueMatches(value, permission) {
				return true
			}
		}
		return false
	}
	if matches(extraUserFields, "Permissions", "PermissionSets") {
		return true
	}
	if org == nil || userID == "" {
		return false
	}
	if _, user, ok := LookupRecordByID(org.Objects["User"].Records, userID); ok && !user.System.IsDeleted && matches(user.Fields, "Permissions", "PermissionSets") {
		return true
	}
	var sets []ID
	for _, assignment := range org.Objects["PermissionSetAssignment"].Records {
		if assignment.System.IsDeleted || assignmentActive != nil && !assignmentActive(assignment) {
			continue
		}
		assignee, ok := assignment.GetField("AssigneeId")
		if !ok || !IDsEqual(permissionID(assignee), userID) {
			continue
		}
		if value, ok := assignment.GetField("PermissionSetId"); ok {
			sets = append(sets, permissionID(value))
		}
		if value, ok := assignment.GetField("PermissionSetGroupId"); ok {
			groupID := permissionID(value)
			if groupID != "" {
				for _, component := range org.Objects["PermissionSetGroupComponent"].Records {
					if component.System.IsDeleted {
						continue
					}
					group, ok := component.GetField("PermissionSetGroupId")
					if !ok || !IDsEqual(permissionID(group), groupID) {
						continue
					}
					if set, ok := component.GetField("PermissionSetId"); ok {
						sets = append(sets, permissionID(set))
					}
				}
			}
		}
	}
	for _, setID := range sets {
		if setID == "" {
			continue
		}
		_, set, ok := LookupRecordByID(org.Objects["PermissionSet"].Records, setID)
		if !ok || set.System.IsDeleted {
			continue
		}
		if matches(set.Fields, "Permissions", "PermissionSets", "CustomPermissions") {
			return true
		}
		for _, access := range org.Objects["SetupEntityAccess"].Records {
			if access.System.IsDeleted {
				continue
			}
			parent, ok := access.GetField("ParentId")
			if !ok || !IDsEqual(permissionID(parent), setID) {
				continue
			}
			entity, ok := access.GetField("SetupEntityId")
			if !ok {
				continue
			}
			_, custom, ok := LookupRecordByID(org.Objects["CustomPermission"].Records, permissionID(entity))
			if !ok || custom.System.IsDeleted {
				continue
			}
			if value, ok := custom.GetField("DeveloperName"); ok && permissionValueMatches(value, permission) {
				return true
			}
		}
	}
	return false
}

func permissionID(value Value) ID {
	switch value.Kind {
	case ValueID:
		return value.ID
	case ValueString:
		return ID(value.String)
	}
	return ""
}

func permissionValueMatches(value Value, permission string) bool {
	if value.Kind == ValueString {
		return strings.EqualFold(strings.TrimSpace(value.String), strings.TrimSpace(permission))
	}
	if value.Kind == ValueList {
		for _, item := range value.List {
			if permissionValueMatches(item, permission) {
				return true
			}
		}
	}
	return false
}
