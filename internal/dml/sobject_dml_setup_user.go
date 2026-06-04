package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func applySetupInsertDefaults(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil {
		return
	}
	defaultRequiredBoolean := func(fieldName string) bool {
		switch {
		case (strings.EqualFold(objectName, "PermissionSet") || strings.EqualFold(objectName, "Profile")) && strings.HasPrefix(fieldName, "Permissions"):
			return true
		case strings.EqualFold(objectName, "User"):
			return true
		default:
			return false
		}
	}
	if !strings.EqualFold(objectName, "PermissionSet") && !strings.EqualFold(objectName, "Profile") && !strings.EqualFold(objectName, "User") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	for name, field := range definition.Fields {
		if field.Type != storage.FieldBoolean || !field.Required || !defaultRequiredBoolean(name) {
			continue
		}
		if _, ok := record.GetField(name); ok {
			continue
		}
		if record.HasExplicitNull(name) {
			continue
		}
		record.Fields[name] = storage.BooleanValue(false)
	}
	if strings.EqualFold(objectName, "User") {
		if _, ok := definition.Fields["CommunityNickname"]; !ok {
			return
		}
		defaultUserCommunityNickname(record)
	}
}

func defaultUserCommunityNickname(record *storage.Record) {
	if record == nil || record.Fields == nil {
		return
	}
	if _, ok := record.Fields["CommunityNickname"]; ok {
		return
	}
	if record.ExplicitNulls != nil && record.ExplicitNulls["CommunityNickname"] {
		return
	}
	for _, field := range []string{"Alias", "Username", "LastName"} {
		value, ok := record.Fields[field]
		if !ok || value.Kind != storage.ValueString || strings.TrimSpace(value.String) == "" {
			continue
		}
		record.Fields["CommunityNickname"] = storage.StringValue(strings.TrimSpace(value.String))
		return
	}
}

func ownerIDReferenceTargets(definition storage.ObjectDefinition) []string {
	if field, ok := fieldByName(definition, "OwnerId"); ok && len(field.ReferenceTo) != 0 {
		return field.ReferenceTo
	}
	return []string{"User", "Group"}
}

func (e *Engine) applyUserContactAccountDefault(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if e == nil || e.Org == nil || record == nil || !strings.EqualFold(objectName, "User") {
		return
	}
	if _, ok := definition.Fields["AccountId"]; !ok {
		return
	}
	if _, ok := record.GetField("AccountId"); ok || record.HasExplicitNull("AccountId") {
		return
	}
	contactID := idFromStorageValue(record.Fields["ContactId"])
	if contactID == "" {
		return
	}
	contacts, ok := e.Org.Objects["Contact"]
	if !ok {
		return
	}
	contact, ok := contacts.Records[contactID]
	if !ok || contact.System.IsDeleted {
		return
	}
	accountID := idFromStorageValue(contact.Fields["AccountId"])
	if accountID == "" {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["AccountId"] = storage.IDValue(accountID)
}

func (e *Engine) afterInsertUser(record storage.Record) {
	if e == nil || e.Org == nil || record.ID == "" {
		return
	}
	e.ensureUserProfilePermissionSetAssignment(record)
	storage.EnsureStandardObject(e.Org, "UserLogin")
	state := e.Org.Objects["UserLogin"]
	for _, existing := range state.Records {
		if idFromStorageValue(existing.Fields["UserId"]) == record.ID {
			return
		}
	}
	e.recordJournalSequence("UserLogin")
	id, err := e.IDs.Next("UserLogin")
	if err != nil {
		return
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, "UserLogin"); cloned {
		state = e.Org.Objects["UserLogin"]
	}
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordInsert("UserLogin", id)
	}
	state.Records[id] = storage.Record{
		ID:     id,
		Object: "UserLogin",
		Fields: map[string]storage.Value{
			"UserId":   storage.IDValue(record.ID),
			"IsFrozen": storage.BooleanValue(false),
		},
	}
	e.Org.Objects["UserLogin"] = state
}

func (e *Engine) ensureUserProfilePermissionSetAssignment(record storage.Record) {
	profileID := idFromStorageValue(record.Fields["ProfileId"])
	if e == nil || e.Org == nil || record.ID == "" || profileID == "" {
		return
	}
	permissionSetID := profileOwnedPermissionSetID(*e.Org, profileID)
	if permissionSetID == "" {
		return
	}
	storage.EnsureStandardObject(e.Org, "PermissionSetAssignment")
	state := e.Org.Objects["PermissionSetAssignment"]
	for _, existing := range state.Records {
		if idFromStorageValue(existing.Fields["AssigneeId"]) == record.ID &&
			idFromStorageValue(existing.Fields["PermissionSetId"]) == permissionSetID {
			return
		}
	}
	e.recordJournalSequence("PermissionSetAssignment")
	id, err := e.IDs.Next("PermissionSetAssignment")
	if err != nil {
		return
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, "PermissionSetAssignment"); cloned {
		state = e.Org.Objects["PermissionSetAssignment"]
	}
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordInsert("PermissionSetAssignment", id)
	}
	state.Records[id] = storage.Record{
		ID:     id,
		Object: "PermissionSetAssignment",
		Fields: map[string]storage.Value{
			"AssigneeId":      storage.IDValue(record.ID),
			"PermissionSetId": storage.IDValue(permissionSetID),
		},
	}
	e.Org.Objects["PermissionSetAssignment"] = state
}

func profileOwnedPermissionSetID(org storage.OrgState, profileID storage.ID) storage.ID {
	state, ok := org.Objects["PermissionSet"]
	if !ok {
		return ""
	}
	for id, record := range state.Records {
		ownedByProfile, ok := record.Fields["IsOwnedByProfile"]
		if !ok || ownedByProfile.Kind != storage.ValueBoolean || !ownedByProfile.Boolean {
			continue
		}
		if idFromStorageValue(record.Fields["ProfileId"]) == profileID {
			if record.ID != "" {
				return record.ID
			}
			return id
		}
	}
	return ""
}
