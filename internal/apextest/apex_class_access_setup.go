package apextest

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// PermissionSet.classAccesses declares top-level Apex class access (Metadata
// API Guide). SetupEntityAccess exposes those enabled settings and the
// ApexClass.SetupEntityAccessItems relationship (Object Reference). Project
// import projects resolved plain declarations only; it never assigns users.
func applyProjectApexClassGrants(org *storage.OrgState, parentID storage.ID, entries []permissionSetClassAccess, generator *storage.IDGenerator) {
	if org == nil || generator == nil || len(entries) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "SetupEntityAccess")
	state := org.Objects["SetupEntityAccess"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.ApexClass)
		if !entry.Enabled || name == "" || strings.Contains(name, ".") {
			continue
		}
		var targetID storage.ID
		ambiguous := false
		for _, target := range org.Objects["ApexClass"].Records {
			namespace, _ := target.GetField("NamespacePrefix")
			if namespace.String != "" || !strings.EqualFold(target.Fields["Name"].String, name) {
				continue
			}
			if targetID != "" && !storage.IDsEqual(targetID, target.ID) {
				ambiguous = true
				break
			}
			targetID = target.ID
		}
		if targetID == "" || ambiguous {
			continue
		}
		exists := false
		for _, link := range state.Records {
			parent, _ := link.GetField("ParentId")
			entity, _ := link.GetField("SetupEntityId")
			if storage.IDsEqual(storage.ID(storageRecordKeyText(parent)), parentID) && storage.IDsEqual(storage.ID(storageRecordKeyText(entity)), targetID) {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		id, err := generator.Next("SetupEntityAccess")
		for err == nil {
			if _, _, occupied := storage.LookupRecordByID(state.Records, id); !occupied {
				break
			}
			id, err = generator.Next("SetupEntityAccess")
		}
		if err != nil {
			continue
		}
		state.Records[id] = storage.Record{ID: id, Object: "SetupEntityAccess", Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(parentID), "SetupEntityId": storage.IDValue(targetID), "SetupEntityType": storage.StringValue("ApexClass"),
		}}
	}
	org.Objects["SetupEntityAccess"] = state
}
