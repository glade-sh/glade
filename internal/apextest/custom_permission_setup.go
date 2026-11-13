package apextest

import (
	"encoding/xml"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func applyProjectCustomPermissionRecords(org *storage.OrgState, p project.Project) {
	applyCustomPermissionRecordsInNamespace(org, p, "")
}

// customPermissionID finds a CustomPermission by DeveloperName within one
// namespace. Two packages may define the same DeveloperName; their rows stay
// separate, and the project's own rows carry no NamespacePrefix.
func customPermissionID(state storage.ObjectState, name, namespace string) (storage.ID, bool) {
	for _, record := range state.Records {
		if strings.EqualFold(record.Fields["DeveloperName"].String, name) && strings.EqualFold(record.Fields["NamespacePrefix"].String, namespace) {
			return record.ID, true
		}
	}
	return "", false
}

// applyCustomPermissionRecordsInNamespace seeds the project's custom permissions;
// a non-empty namespace stamps NamespacePrefix on each new row.
func applyCustomPermissionRecordsInNamespace(org *storage.OrgState, p project.Project, namespace string) {
	if org == nil || len(p.CustomPermissionFiles) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "CustomPermission")
	state := org.Objects["CustomPermission"]
	if state.Records == nil {
		state.SetRecords(make(map[storage.ID]storage.Record))
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
		if _, exists := customPermissionID(state, name, namespace); exists {
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
		fields := map[string]storage.Value{"DeveloperName": storage.StringValue(name), "MasterLabel": storage.StringValue(label), "Description": storage.StringValue(metadata.Description), "IsLicensed": storage.BooleanValue(metadata.IsLicensed)}
		if namespace != "" {
			fields["NamespacePrefix"] = storage.StringValue(namespace)
		}
		state.PutRecord(id, storage.Record{ID: id, Object: "CustomPermission", Fields: fields})
	}
	org.IDSequences = generator.Sequences
	org.Objects["CustomPermission"] = state
}

// Enabled custom permissions are queryable access settings, not only names on
// PermissionSet. See Salesforce's admin_troubleshoot_queries Access Settings
// section. Resolve existing metadata only; never synthesize a missing permission.
func applyProjectCustomPermissionGrants(org *storage.OrgState, parentID storage.ID, permissions []storage.Value, generator *storage.IDGenerator) {
	applyCustomPermissionGrantsInNamespace(org, parentID, permissions, "", generator)
}

// applyCustomPermissionGrantsInNamespace resolves each permission among the
// rows of one namespace, so a set never grants another package's same-named
// permission.
func applyCustomPermissionGrantsInNamespace(org *storage.OrgState, parentID storage.ID, permissions []storage.Value, namespace string, generator *storage.IDGenerator) {
	if org == nil || generator == nil || len(permissions) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "SetupEntityAccess")
	state := org.Objects["SetupEntityAccess"]
	for _, permission := range permissions {
		permissionID, ok := customPermissionID(org.Objects["CustomPermission"], permission.String, namespace)
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
		state.PutRecord(id, storage.Record{ID: id, Object: "SetupEntityAccess", Fields: map[string]storage.Value{
			"ParentId":        storage.IDValue(parentID),
			"SetupEntityId":   storage.IDValue(permissionID),
			"SetupEntityType": storage.StringValue("CustomPermission"),
		}})
	}
	org.Objects["SetupEntityAccess"] = state
}
