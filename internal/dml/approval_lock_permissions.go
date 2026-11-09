package dml

import (
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// canEditLockedRecord resolves the effective admin and object grants for
// all callers, including the VM runAs context.
func (e *Engine) canEditLockedRecord(objectName string) bool {
	if e.Org == nil {
		return false
	}
	uid := e.systemUserID()
	_, user, ok := storage.LookupRecordByID(e.Org.Objects["User"].Records, uid)
	if !ok {
		return storage.IDsEqual(uid, storage.DefaultUserID)
	}
	if user.System.IsDeleted {
		return false
	}
	// User rows carry no Modify All Data field; only the profile and permission
	// sets below are captured grants.
	profileID := approvalPermissionID(user, "ProfileId")
	_, profile, ok := storage.LookupRecordByID(e.Org.Objects["Profile"].Records, profileID)
	if ok && profile.System.IsDeleted {
		return false
	}
	if ok && approvalPermissionTrue(profile, "PermissionsModifyAllData") {
		return true
	}
	parents := []storage.ID{profileID}
	for _, ps := range e.Org.Objects["PermissionSet"].Records {
		if !ps.System.IsDeleted && approvalPermissionTrue(ps, "IsOwnedByProfile") && storage.IDsEqual(approvalPermissionID(ps, "ProfileId"), profileID) {
			parents = append(parents, ps.ID)
		}
	}
	for _, assignment := range e.Org.Objects["PermissionSetAssignment"].Records {
		if !e.approvalAssignmentActive(assignment) || !storage.IDsEqual(approvalPermissionID(assignment, "AssigneeId"), uid) {
			continue
		}
		groupID := approvalPermissionID(assignment, "PermissionSetGroupId")
		if groupID == "" {
			parents = append(parents, approvalPermissionID(assignment, "PermissionSetId"))
			continue
		}
		// Group components have to be evaluated together: muting affects this
		// group only, while directly assigned permission sets remain effective.
		var members []storage.ID
		mutedData, mutedObject := false, false
		for _, component := range e.Org.Objects["PermissionSetGroupComponent"].Records {
			if component.System.IsDeleted || !storage.IDsEqual(approvalPermissionID(component, "PermissionSetGroupId"), groupID) {
				continue
			}
			id := approvalPermissionID(component, "PermissionSetId")
			_, mute, exists := storage.LookupRecordByID(e.Org.Objects["MutingPermissionSet"].Records, id)
			if exists {
				if !mute.System.IsDeleted {
					mutedData = mutedData || approvalPermissionTrue(mute, "PermissionsModifyAllData")
					mutedObject = mutedObject || e.approvalObjectModifyAll(id, objectName)
				}
				continue
			}
			members = append(members, id)
		}
		for _, member := range members {
			_, ps, exists := storage.LookupRecordByID(e.Org.Objects["PermissionSet"].Records, member)
			if !exists || ps.System.IsDeleted {
				continue
			}
			if !mutedData && approvalPermissionTrue(ps, "PermissionsModifyAllData") || !mutedObject && e.approvalObjectModifyAll(member, objectName) {
				return true
			}
		}
	}
	for _, parent := range parents {
		if parent == "" {
			continue
		}
		if !storage.IDsEqual(parent, profileID) {
			_, ps, exists := storage.LookupRecordByID(e.Org.Objects["PermissionSet"].Records, parent)
			if !exists || ps.System.IsDeleted {
				continue
			}
			if approvalPermissionTrue(ps, "PermissionsModifyAllData") {
				return true
			}
		}
		if e.approvalObjectModifyAll(parent, objectName) {
			return true
		}
	}
	return false
}

func (e *Engine) approvalAssignmentActive(assignment storage.Record) bool {
	if assignment.System.IsDeleted || approvalPermissionTrue(assignment, "IsRevoked") {
		return false
	}
	// IsActive is computed by Salesforce; local inserts currently retain the
	// schema default, so derive activity from revocation and expiration.
	if expiry, present := assignment.GetField("ExpirationDate"); present && expiry.Kind != storage.ValueNull && expiry.String != "" {
		when, err := time.Parse(time.RFC3339Nano, expiry.String)
		if err != nil {
			return false
		}
		now := time.Now()
		if e.Now != nil {
			now = e.Now()
		}
		if !when.After(now) {
			return false
		}
	}
	return true
}

func (e *Engine) approvalObjectModifyAll(parent storage.ID, objectName string) bool {
	for _, grant := range e.Org.Objects["ObjectPermissions"].Records {
		object, _ := grant.GetField("SObjectType")
		if !grant.System.IsDeleted && strings.EqualFold(object.String, objectName) && storage.IDsEqual(approvalPermissionID(grant, "ParentId"), parent) && approvalPermissionTrue(grant, "PermissionsModifyAllRecords") {
			return true
		}
	}
	return false
}

func approvalPermissionTrue(record storage.Record, name string) bool {
	value, ok := record.GetField(name)
	return ok && value.Kind == storage.ValueBoolean && value.Boolean
}
func approvalPermissionID(record storage.Record, name string) storage.ID {
	value, ok := record.GetField(name)
	if !ok {
		return ""
	}
	if value.Kind == storage.ValueID {
		return value.ID
	}
	return storage.ID(value.String)
}

func lockedRecordError() error {
	return dmlErrorf("ENTITY_IS_LOCKED", nil, "This record is locked. If you need to edit it, contact your admin.")
}
