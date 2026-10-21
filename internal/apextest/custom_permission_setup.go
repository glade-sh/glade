package apextest

import (
	"encoding/xml"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func applyProjectCustomPermissionRecords(org *storage.OrgState, p project.Project) {
	if org == nil || len(p.CustomPermissionFiles) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "CustomPermission")
	state := org.Objects["CustomPermission"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	generator := storage.NewStandardIDGenerator()
	generator.Prefixes["CustomPermission"] = state.Definition.KeyPrefix
	if org.IDSequences != nil {
		generator.Sequences = org.IDSequences
	}
	for _, file := range p.CustomPermissionFiles {
		name := metadataNameFromPath(file, ".custompermission-meta.xml", ".custompermission")
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var metadata struct {
			XMLName     xml.Name `xml:"CustomPermission"`
			Label       string   `xml:"label"`
			Description string   `xml:"description"`
			IsLicensed  bool     `xml:"isLicensed"`
		}
		if xml.Unmarshal(data, &metadata) != nil || name == "" {
			continue
		}
		if _, exists := recordFieldID(state, "DeveloperName", name); exists {
			continue
		}
		id, err := generator.Next("CustomPermission")
		if err != nil {
			continue
		}
		label := strings.TrimSpace(metadata.Label)
		if label == "" {
			label = strings.ReplaceAll(name, "_", " ")
		}
		state.Records[id] = storage.Record{ID: id, Object: "CustomPermission", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue(name), "MasterLabel": storage.StringValue(label), "Description": storage.StringValue(metadata.Description), "IsLicensed": storage.BooleanValue(metadata.IsLicensed)}}
	}
	org.IDSequences = generator.Sequences
	org.Objects["CustomPermission"] = state
}

// Enabled custom permissions are queryable access settings, not only names on
// PermissionSet. See Salesforce's admin_troubleshoot_queries Access Settings
// section. Resolve existing metadata only; never synthesize a missing permission.
func applyProjectCustomPermissionGrants(org *storage.OrgState, parentID storage.ID, permissions []storage.Value, generator *storage.IDGenerator) {
	if org == nil || generator == nil || len(permissions) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "SetupEntityAccess")
	state := org.Objects["SetupEntityAccess"]
	for _, permission := range permissions {
		permissionID, ok := recordFieldID(org.Objects["CustomPermission"], "DeveloperName", permission.String)
		if !ok {
			continue
		}
		exists := false
		for _, record := range state.Records {
			parent, _ := record.GetField("ParentId")
			entity, _ := record.GetField("SetupEntityId")
			if storageIDValueEqualsText(parent, string(parentID)) && storageIDValueEqualsText(entity, string(permissionID)) {
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
			"ParentId":        storage.IDValue(parentID),
			"SetupEntityId":   storage.IDValue(permissionID),
			"SetupEntityType": storage.StringValue("CustomPermission"),
		}}
	}
	org.Objects["SetupEntityAccess"] = state
}
