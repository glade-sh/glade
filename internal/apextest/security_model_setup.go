package apextest

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

// Profile metadata is exposed through its owning PermissionSet, including the
// Parent.Profile relationship on the permission junctions. Names and labels are
// opaque setup identities; Profile.Name supplies the human-readable name.
func ensureProjectProfilePermissionSet(org *storage.OrgState, profileID storage.ID, generator *storage.IDGenerator) storage.ID {
	for _, row := range org.Objects["PermissionSet"].Records {
		if row.Fields["IsOwnedByProfile"].Boolean && storage.IDsEqual(storage.ID(storageRecordKeyText(row.Fields["ProfileId"])), profileID) {
			return row.ID
		}
	}
	return insertProjectSecurityRow(org, "PermissionSet", map[string]storage.Value{
		"Name": storage.StringValue("X" + string(profileID)), "Label": storage.StringValue(string(profileID)),
		"Type": storage.StringValue("Profile"), "IsOwnedByProfile": storage.BooleanValue(true), "ProfileId": storage.IDValue(profileID),
	}, generator)
}

func insertProjectSecurityRow(org *storage.OrgState, objectName string, fields map[string]storage.Value, generator *storage.IDGenerator) storage.ID {
	ids := insertProjectSecurityRows(org, objectName, []map[string]storage.Value{fields}, generator)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func projectSecurityIDKey(text string) string {
	if len(text) >= 15 {
		return text[:15]
	}
	return text
}

func insertProjectSecurityRows(org *storage.OrgState, objectName string, rows []map[string]storage.Value, generator *storage.IDGenerator) []storage.ID {
	if len(rows) == 0 {
		return nil
	}
	storage.EnsureStandardObject(org, objectName)
	state := org.Objects[objectName]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	generator.Prefixes[objectName] = state.Definition.KeyPrefix
	occupied := make(map[string]bool, len(state.Records)+len(rows))
	for id := range state.Records {
		occupied[projectSecurityIDKey(string(id))] = true
	}
	ids := make([]storage.ID, 0, len(rows))
	for _, fields := range rows {
		for {
			id, err := generator.Next(objectName)
			if err != nil {
				org.Objects[objectName] = state
				return ids
			}
			key := projectSecurityIDKey(string(id))
			if occupied[key] {
				continue
			}
			occupied[key] = true
			state.Records[id] = storage.Record{ID: id, Object: objectName, Fields: fields}
			ids = append(ids, id)
			break
		}
	}
	org.Objects[objectName] = state
	return ids
}

func applyProjectSetupEntityGrant(org *storage.OrgState, parent storage.ID, target storage.Value, kind string, generator *storage.IDGenerator) {
	for _, row := range org.Objects["SetupEntityAccess"].Records {
		sameTarget := strings.EqualFold(storageRecordKeyText(row.Fields["SetupEntityId"]), storageRecordKeyText(target))
		if target.Kind == storage.ValueID {
			sameTarget = storage.IDsEqual(storage.ID(storageRecordKeyText(row.Fields["SetupEntityId"])), target.ID)
		}
		if storage.IDsEqual(storage.ID(storageRecordKeyText(row.Fields["ParentId"])), parent) && sameTarget && row.Fields["SetupEntityType"].String == kind {
			return
		}
	}
	insertProjectSecurityRow(org, "SetupEntityAccess", map[string]storage.Value{"ParentId": storage.IDValue(parent), "SetupEntityId": target, "SetupEntityType": storage.StringValue(kind)}, generator)
}

func applyProjectSecurityEntityGrants(org *storage.OrgState, p project.Project, parent storage.ID, metadata permissionSetMetadata, generator *storage.IDGenerator) {
	if p.Namespace != "" {
		return
	}
	applyProjectApexClassGrants(org, parent, metadata.ClassAccesses, generator)
	for _, entry := range metadata.PageAccesses {
		if !entry.Enabled || strings.Contains(entry.ApexPage, ".") {
			continue
		}
		if id, ok := recordFieldID(org.Objects["ApexPage"], "Name", strings.TrimSpace(entry.ApexPage)); ok {
			applyProjectSetupEntityGrant(org, parent, storage.IDValue(id), "ApexPage", generator)
		}
	}
	for _, entry := range metadata.MetadataTypeAccesses {
		if !entry.Enabled {
			continue
		}
		name, ok := storage.ResolveObjectName(*org, strings.TrimSpace(entry.Name))
		if ok && storage.IsCustomMetadataDefinition(org.Objects[name].Definition) {
			// EntityDefinition uses the canonical API name as its local durable identity.
			applyProjectSetupEntityGrant(org, parent, storage.StringValue(name), "CustomEntityDefinition", generator)
		}
	}
	for _, entry := range metadata.TabSettings {
		if entry.Visibility != "Visible" {
			continue
		}
		for _, tab := range org.Metadata.Tabs {
			if !strings.EqualFold(tab.Name, strings.TrimSpace(entry.Tab)) {
				continue
			}
			exists := false
			for _, row := range org.Objects["PermissionSetTabSetting"].Records {
				if storageIDValueEqualsText(row.Fields["ParentId"], string(parent)) && strings.EqualFold(row.Fields["Name"].String, tab.Name) {
					exists = true
					break
				}
			}
			if !exists {
				insertProjectSecurityRow(org, "PermissionSetTabSetting", map[string]storage.Value{"ParentId": storage.IDValue(parent), "Name": storage.StringValue(tab.Name), "Visibility": storage.StringValue("DefaultOn")}, generator)
			}
		}
	}
}

// Source deployment gives the default administrator object CRUD and Apex/page
// access. Optional field access remains explicit profile metadata, as on native.
func applyProjectAdminSecurityDefaults(org *storage.OrgState, objects []schema.Object) {
	profileID := storage.ID(storage.DefaultProfileID)
	if _, _, ok := storage.LookupRecordByID(org.Objects["Profile"].Records, profileID); !ok {
		return
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences = org.IDSequences
	parent := ensureProjectProfilePermissionSet(org, profileID, &generator)
	existing := make(map[string]bool)
	for _, row := range org.Objects["SetupEntityAccess"].Records {
		if storage.IDsEqual(storage.ID(storageRecordKeyText(row.Fields["ParentId"])), parent) {
			existing[row.Fields["SetupEntityType"].String+"|"+projectSecurityIDKey(storageRecordKeyText(row.Fields["SetupEntityId"]))] = true
		}
	}
	var setupRows []map[string]storage.Value
	for _, kind := range []string{"ApexClass", "ApexPage"} {
		for _, id := range sortedSecurityRecordIDs(org.Objects[kind]) {
			row := org.Objects[kind].Records[id]
			key := kind + "|" + projectSecurityIDKey(string(row.ID))
			if row.System.IsDeleted || existing[key] {
				continue
			}
			existing[key] = true
			setupRows = append(setupRows, map[string]storage.Value{"ParentId": storage.IDValue(parent), "SetupEntityId": storage.IDValue(row.ID), "SetupEntityType": storage.StringValue(kind)})
		}
	}
	insertProjectSecurityRows(org, "SetupEntityAccess", setupRows, &generator)
	var objectRows []map[string]storage.Value

	keys := objectPermissionRecordKeys(org.Objects["ObjectPermissions"])
	for _, object := range objects {
		// A declared name field distinguishes source custom objects from standard
		// object extensions and metadata types. Never infer permission fields.
		if object.NameField.Type == "" || object.CustomSettingsType != "" || object.Partial {
			continue
		}
		name, ok := storage.ResolveObjectName(*org, object.Name)
		if !ok {
			continue
		}
		key := objectPermissionRecordKey(string(parent), name)
		if keys[key] {
			continue
		}
		fields := map[string]storage.Value{"ParentId": storage.IDValue(parent), "SobjectType": storage.StringValue(name)}
		for _, field := range []string{"PermissionsRead", "PermissionsCreate", "PermissionsEdit", "PermissionsDelete", "PermissionsViewAllRecords", "PermissionsModifyAllRecords"} {
			fields[field] = storage.BooleanValue(true)
		}
		objectRows = append(objectRows, fields)
		keys[key] = true
	}
	insertProjectSecurityRows(org, "ObjectPermissions", objectRows, &generator)
	org.IDSequences = generator.Sequences
}

func sortedSecurityRecordIDs(state storage.ObjectState) []storage.ID {
	ids := make([]storage.ID, 0, len(state.Records))
	for id := range state.Records {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// A deployed group's aggregate PermissionSet exposes the union of its source
// components as queryable setup junctions, independently of user assignments.
func applyProjectPermissionSetGroupGrants(org *storage.OrgState) {
	generator := storage.NewStandardIDGenerator()
	generator.Sequences = org.IDSequences
	for _, id := range sortedSecurityRecordIDs(org.Objects["PermissionSet"]) {
		groupSet := org.Objects["PermissionSet"].Records[id]
		if groupSet.Fields["Type"].String != "Group" {
			continue
		}
		members := make(map[string]bool)
		for _, component := range org.Objects["PermissionSetGroupComponent"].Records {
			if storage.IDsEqual(storage.ID(storageRecordKeyText(component.Fields["PermissionSetGroupId"])), storage.ID(storageRecordKeyText(groupSet.Fields["PermissionSetGroupId"]))) {
				members[projectSecurityIDKey(storageRecordKeyText(component.Fields["PermissionSetId"]))] = true
			}
		}
		for _, shape := range []struct {
			object            string
			keys, permissions []string
		}{
			{"SetupEntityAccess", []string{"SetupEntityType", "SetupEntityId"}, nil},
			{"ObjectPermissions", []string{"SobjectType"}, []string{"PermissionsRead", "PermissionsCreate", "PermissionsEdit", "PermissionsDelete", "PermissionsViewAllRecords", "PermissionsModifyAllRecords"}},
			{"FieldPermissions", []string{"SobjectType", "Field"}, []string{"PermissionsRead", "PermissionsEdit"}},
			{"PermissionSetTabSetting", []string{"Name"}, nil},
		} {
			state := org.Objects[shape.object]
			keyFor := func(row storage.Record) string {
				var values []string
				for _, key := range shape.keys {
					value := storageRecordKeyText(row.Fields[key])
					if key == "SetupEntityId" && row.Fields[key].Kind == storage.ValueID {
						value = projectSecurityIDKey(value)
					} else {
						value = strings.ToLower(value)
					}
					values = append(values, value)
				}
				return strings.Join(values, "\x00")
			}
			existing := make(map[string]storage.ID)
			projected := make(map[string]storage.Record)
			for _, rowID := range sortedSecurityRecordIDs(state) {
				row := state.Records[rowID]
				if row.System.IsDeleted {
					continue
				}
				if storage.IDsEqual(storage.ID(storageRecordKeyText(row.Fields["ParentId"])), id) {
					existing[keyFor(row)] = rowID
				}
				if !members[projectSecurityIDKey(storageRecordKeyText(row.Fields["ParentId"]))] {
					continue
				}
				key := keyFor(row)
				combined, found := projected[key]
				if !found {
					combined = row.Clone()
					delete(combined.Fields, "Id")
					combined.Fields["ParentId"] = storage.IDValue(id)
				}
				for _, field := range shape.permissions {
					if row.Fields[field].Boolean {
						combined.Fields[field] = storage.BooleanValue(true)
					}
				}
				projected[key] = combined
			}
			keys := make([]string, 0, len(projected))
			for key := range projected {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			var pending []map[string]storage.Value
			for _, key := range keys {
				row := projected[key]
				if target, ok := existing[key]; ok {
					combined := state.Records[target].Clone()
					for _, field := range shape.permissions {
						if row.Fields[field].Boolean {
							combined.Fields[field] = storage.BooleanValue(true)
						}
					}
					state.Records[target] = combined
				} else {
					pending = append(pending, row.Fields)
				}
			}
			if len(keys) > 0 {
				org.Objects[shape.object] = state
				insertProjectSecurityRows(org, shape.object, pending, &generator)
			}
		}
	}
	org.IDSequences = generator.Sequences
}
