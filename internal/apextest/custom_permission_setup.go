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
