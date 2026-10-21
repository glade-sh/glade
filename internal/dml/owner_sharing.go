package dml

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Native owner-rule captures cover Private custom objects, their Owner row,
// and one Read rule whose source public group contains the owner directly.
// Other sharing models, nested/role membership and membership-change-triggered
// recalculation are separate boundaries; this does not grant recipient access.
func privateOwnerShareObject(org *storage.OrgState, definition storage.ObjectDefinition) (string, bool) {
	if org == nil || definition.SharingModel != "Private" || !strings.HasSuffix(strings.ToLower(definition.APIName), "__c") || storage.IsCustomSettingDefinition(definition) || storage.IsCustomMetadataDefinition(definition) {
		return "", false
	}
	name, ok := storage.ResolveObjectName(*org, definition.APIName[:len(definition.APIName)-3]+"__Share")
	if !ok {
		return "", false
	}
	share := org.Objects[name].Definition
	return name, share.Metadata["kind"] == "generatedShare" && strings.EqualFold(share.Metadata["parentObject"], definition.APIName)
}

func (e *Engine) reconcilePrivateOwnerShares(objectName string, parentID storage.ID) error {
	object := e.Org.Objects[objectName]
	shareName, ok := privateOwnerShareObject(e.Org, object.Definition)
	if !ok {
		return nil
	}
	_, parent, ok := storage.LookupRecordByID(object.Records, parentID)
	if !ok || parent.System.IsDeleted || !strings.HasPrefix(string(parent.System.OwnerID), "005") {
		return nil
	}
	type grant struct {
		cause, access string
		recipient     storage.ID
	}
	wanted := map[string]grant{"Owner": {cause: "Owner", access: "All", recipient: parent.System.OwnerID}}
	var managedTarget storage.ID
	// Configuration is populated only for the captured single-rule shape. Do
	// not project unsupported programmatic configurations as additional grants.
	if len(object.Definition.OwnerSharingRules) == 1 {
		rule := object.Definition.OwnerSharingRules[0]
		if rule.AccessLevel == "Read" {
			source := e.directSharingGroup(rule.SourceGroup)
			target := e.directSharingGroup(rule.TargetGroup)
			if source != "" && target != "" {
				managedTarget = target
				if e.ownerIsDirectGroupMember(source, parent.System.OwnerID) {
					wanted["Rule"] = grant{cause: "Rule", access: "Read", recipient: target}
				}
			}
		}
	}
	share := e.Org.Objects[shareName]
	ids := make([]storage.ID, 0)
	for id, row := range share.Records {
		p, _ := row.GetField("ParentId")
		if !row.System.IsDeleted && storage.IDsEqual(idFromStorageValue(p), parentID) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	type change struct {
		id             storage.ID
		old            storage.Record
		next           storage.Record
		remove, insert bool
	}
	changes := make([]change, 0)
	for _, id := range ids {
		row := share.Records[id]
		cause, _ := row.GetField("RowCause")
		recipient, _ := row.GetField("UserOrGroupId")
		key := ""
		if cause.String == "Owner" {
			key = "Owner"
		} else if cause.String == "Rule" && managedTarget != "" && storage.IDsEqual(managedTarget, idFromStorageValue(recipient)) {
			key = "Rule"
		}
		if key == "" {
			continue
		} // Manual and unrelated managed shares remain untouched.
		desired, keep := wanted[key]
		if !keep {
			changes = append(changes, change{id: id, old: row, remove: true})
			continue
		}
		delete(wanted, key)
		access, _ := row.GetField("AccessLevel")
		if storage.IDsEqual(idFromStorageValue(recipient), desired.recipient) && access.String == desired.access {
			continue
		}
		next := row.Clone()
		next.Fields["UserOrGroupId"] = storage.IDValue(desired.recipient)
		next.Fields["AccessLevel"] = storage.StringValue(desired.access)
		changes = append(changes, change{id: id, old: row, next: next})
	}
	keys := make([]string, 0, len(wanted))
	for key := range wanted {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		desired := wanted[key]
		e.recordJournalSequence(shareName)
		var id storage.ID
		for {
			next, err := e.IDs.Next(shareName)
			if err != nil {
				return err
			}
			id = next
			if _, _, exists := storage.LookupRecordByID(share.Records, id); !exists {
				break
			}
		}
		row := storage.Record{Object: shareName, ID: id, Fields: map[string]storage.Value{"ParentId": storage.IDValue(parentID), "UserOrGroupId": storage.IDValue(desired.recipient), "RowCause": storage.StringValue(desired.cause), "AccessLevel": storage.StringValue(desired.access)}}
		changes = append(changes, change{id: id, next: row, insert: true})
	}
	if len(changes) == 0 {
		return nil
	}
	storage.EnsureMutableObjectRecords(e.Org, shareName)
	share = e.Org.Objects[shareName]
	if share.Records == nil {
		share.Records = make(map[storage.ID]storage.Record)
	}
	for _, change := range changes {
		if e.IsolationJournal != nil {
			if change.insert {
				e.IsolationJournal.RecordInsert(shareName, change.id)
			} else {
				e.IsolationJournal.RecordUpdate(shareName, change.id, change.old)
			}
		}
		if change.remove {
			delete(share.Records, change.id)
		} else {
			share.Records[change.id] = change.next
		}
	}
	share.Indexes = nil
	e.Org.Objects[shareName] = share
	return nil
}

func (e *Engine) directSharingGroup(developerName string) storage.ID {
	for _, group := range e.Org.Objects["Group"].Records {
		if !group.System.IsDeleted && group.Fields["Type"].String == "Regular" && group.Fields["DeveloperName"].String == developerName {
			return group.ID
		}
	}
	return ""
}

func (e *Engine) ownerIsDirectGroupMember(group, owner storage.ID) bool {
	for _, member := range e.Org.Objects["GroupMember"].Records {
		if member.System.IsDeleted {
			continue
		}
		groupID, _ := member.GetField("GroupId")
		userID, _ := member.GetField("UserOrGroupId")
		if storage.IDsEqual(idFromStorageValue(groupID), group) && storage.IDsEqual(idFromStorageValue(userID), owner) {
			return true
		}
	}
	return false
}
