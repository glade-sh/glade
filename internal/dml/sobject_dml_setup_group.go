package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) applyPermissionSetAssignmentDefaults(objectName string, record *storage.Record) {
	if objectName != "PermissionSetAssignment" || e.Org == nil || record == nil {
		return
	}
	groupID := idFromStorageValue(record.Fields["PermissionSetGroupId"])
	if groupID == "" || idFromStorageValue(record.Fields["PermissionSetId"]) != "" {
		return
	}
	for _, permissionSet := range e.Org.Objects["PermissionSet"].Records {
		if !permissionSet.System.IsDeleted && strings.EqualFold(permissionSet.Fields["Type"].String, "Group") &&
			storage.IDsEqual(idFromStorageValue(permissionSet.Fields["PermissionSetGroupId"]), groupID) {
			record.Fields["PermissionSetId"] = storage.IDValue(permissionSet.ID)
			return
		}
	}
}

func (e *Engine) afterInsertPermissionSetGroup(group storage.Record) error {
	// Captured creation covers unmanaged groups; keep other setup contexts unchanged.
	if !e.localPermissionSetGroup(group) {
		return nil
	}
	name, _ := group.GetField("DeveloperName")
	label, _ := group.GetField("MasterLabel")
	storage.EnsureStandardObject(e.Org, "PermissionSet")
	for _, record := range e.Org.Objects["PermissionSet"].Records {
		if !record.System.IsDeleted && storage.IDsEqual(idFromStorageValue(record.Fields["PermissionSetGroupId"]), group.ID) {
			return nil
		}
	}
	_, err := e.insertPlatformRecord(storage.Record{
		Object: "PermissionSet",
		Fields: map[string]storage.Value{
			"Name":                 name,
			"Label":                label,
			"Type":                 storage.StringValue("Group"),
			"PermissionSetGroupId": storage.IDValue(group.ID),
			"IsCustom":             storage.BooleanValue(true),
			"IsOwnedByProfile":     storage.BooleanValue(false),
		},
	})
	if err == nil {
		e.setPermissionSetGroupStatus(group.ID, "Updated")
	}
	return err
}

func (e *Engine) afterInsertPermissionSetGroupComponent(component storage.Record) {
	id := idFromStorageValue(component.Fields["PermissionSetGroupId"])
	_, group, found := storage.LookupRecordByID(e.Org.Objects["PermissionSetGroup"].Records, id)
	if !found || group.Fields["Status"].String != "Updated" && group.Fields["Status"].String != "Outdated" {
		return
	}
	e.setPermissionSetGroupStatus(id, "Outdated")
}

// RecalculatePermissionSetGroups advances the stored status of ordinary local
// groups. Effective permissions are resolved from their live components.
// The callback charges the native implicit update before any record changes.
func (e *Engine) RecalculatePermissionSetGroups(ids []storage.ID, beforeUpdate func(int) error) error {
	pending := make([]storage.ID, 0, len(ids))
	seen := make(map[storage.ID]bool, len(ids))
	for _, id := range ids {
		storedID, group, found := storage.LookupRecordByID(e.Org.Objects["PermissionSetGroup"].Records, id)
		status := group.Fields["Status"].String
		if !found || seen[storedID] || !e.localPermissionSetGroup(group) || status != "Updated" && status != "Outdated" {
			// Preserve the existing no-op for unobserved argument/state shapes.
			return nil
		}
		seen[storedID] = true
		if status == "Outdated" {
			pending = append(pending, storedID)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	if beforeUpdate != nil {
		if err := beforeUpdate(len(pending)); err != nil {
			return err
		}
	}
	for _, id := range pending {
		e.setPermissionSetGroupStatus(id, "Updated")
	}
	return nil
}

func (e *Engine) localPermissionSetGroup(group storage.Record) bool {
	if e.Org == nil || strings.TrimSpace(e.Org.Namespace) != "" || group.System.IsDeleted {
		return false
	}
	prefix, _ := group.GetField("NamespacePrefix")
	name, named := group.GetField("DeveloperName")
	label, labeled := group.GetField("MasterLabel")
	return strings.TrimSpace(prefix.String) == "" && named && labeled && name.Kind == storage.ValueString && label.Kind == storage.ValueString && strings.TrimSpace(name.String) != "" && strings.TrimSpace(label.String) != ""
}

func (e *Engine) setPermissionSetGroupStatus(id storage.ID, status string) {
	object := e.Org.Objects["PermissionSetGroup"]
	storedID, record, found := storage.LookupRecordByID(object.Records, id)
	if !found || !e.localPermissionSetGroup(record) || record.Fields["Status"].String == status {
		return
	}
	if mutable, _ := storage.EnsureMutableObjectRecords(e.Org, "PermissionSetGroup"); mutable != nil {
		object = *mutable
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate("PermissionSetGroup", storedID, record)
	}
	record = record.Clone()
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["Status"] = storage.StringValue(status)
	object.Records[storedID] = record
	e.Org.Objects["PermissionSetGroup"] = object
}
