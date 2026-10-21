package resource

import (
	"encoding/xml"
	"fmt"
	"maps"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

type businessHoursSettingsXML struct {
	XMLName xml.Name                 `xml:"BusinessHoursSettings"`
	Hours   []businessHoursRecordXML `xml:"businessHours"`
}

type businessHoursRecordXML struct {
	Name     string `xml:"name"`
	Active   *bool  `xml:"active"`
	Default  *bool  `xml:"default"`
	TimeZone string `xml:"timeZoneId"`
	Fields   []struct {
		XMLName xml.Name
		Value   string `xml:",chardata"`
	} `xml:",any"`
}

type duplicateRuleRecordXML struct {
	XMLName xml.Name `xml:"DuplicateRule"`
	Label   string   `xml:"masterLabel"`
	Active  *bool    `xml:"isActive"`
}

// SetupRecordMetadataFiles includes only declared package views. The same paths
// feed runtime snapshots, so later disk changes cannot replace captured bodies.
func SetupRecordMetadataFiles(p project.Project) []string {
	files := make(map[string]bool)
	seen := make(map[*project.Project]bool)
	var visit func(*project.Project)
	visit = func(current *project.Project) {
		if current == nil || seen[current] {
			return
		}
		seen[current] = true
		for _, path := range current.BusinessHoursSettingsFiles {
			files[path] = true
		}
		for _, path := range current.DuplicateRuleFiles {
			files[path] = true
		}
		for _, dep := range current.ManagedPackageDependencies {
			if dep.Status == "loaded" {
				visit(dep.Project)
			}
		}
		for _, shim := range current.PackageShims {
			if shim.Status == "loaded" {
				visit(shim.Project)
			}
		}
	}
	visit(&p)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func applySetupRecordMetadata(org *storage.OrgState, p project.Project, readMetadata func(string) ([]byte, error)) error {
	if org == nil {
		return nil
	}
	paths := SetupRecordMetadataFiles(p)
	if len(paths) == 0 {
		return nil
	}
	staged := *org
	staged.Objects = maps.Clone(org.Objects)
	if staged.Objects == nil {
		staged.Objects = make(map[string]storage.ObjectState)
	}
	staged.IDSequences = maps.Clone(org.IDSequences)
	if staged.IDSequences == nil {
		staged.IDSequences = make(map[string]uint64)
	}
	for _, name := range []string{"BusinessHours", "DuplicateRule"} {
		if state, exists := org.Objects[name]; exists {
			staged.Objects[name] = state.Clone()
		}
		storage.EnsureStandardObject(&staged, name)
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences = staged.IDSequences
	for _, path := range paths {
		body, err := readMetadata(path)
		if err != nil {
			return fmt.Errorf("load setup metadata %s: %w", path, err)
		}
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".duplicaterule-meta.xml") || strings.HasSuffix(lower, ".duplicaterule") {
			var rule duplicateRuleRecordXML
			if err := xml.Unmarshal(body, &rule); err != nil {
				return fmt.Errorf("load DuplicateRule metadata %s: %w", path, err)
			}
			fullName := trimKnownSuffix(trimKnownSuffix(filepath.Base(path), ".duplicateRule-meta.xml"), ".duplicateRule")
			object, name, ok := strings.Cut(fullName, ".")
			if !ok || object == "" || name == "" {
				return fmt.Errorf("load DuplicateRule metadata %s: expected object and developer name", path)
			}
			subtype := ""
			if strings.EqualFold(object, "PersonAccount") {
				object, subtype = "Account", "PersonAccount"
			}
			fields := map[string]storage.Value{"DeveloperName": storage.StringValue(name), "MasterLabel": storage.StringValue(rule.Label), "SobjectType": storage.StringValue(object)}
			if subtype != "" {
				fields["SobjectSubtype"] = storage.StringValue(subtype)
			}
			if rule.Active != nil {
				fields["IsActive"] = storage.BooleanValue(*rule.Active)
			}
			if err := upsertSetupMetadataRecord(&staged, &generator, "DuplicateRule", fields, []string{"DeveloperName", "SobjectType", "SobjectSubtype"}); err != nil {
				return err
			}
			continue
		}
		var settings businessHoursSettingsXML
		if err := xml.Unmarshal(body, &settings); err != nil {
			return fmt.Errorf("load BusinessHours metadata %s: %w", path, err)
		}
		for _, hours := range settings.Hours {
			if strings.TrimSpace(hours.Name) == "" {
				return fmt.Errorf("load BusinessHours metadata %s: missing name", path)
			}
			fields := map[string]storage.Value{"Name": storage.StringValue(hours.Name), "TimeZoneSidKey": storage.StringValue(hours.TimeZone)}
			if hours.Active != nil {
				fields["IsActive"] = storage.BooleanValue(*hours.Active)
			}
			if hours.Default != nil {
				fields["IsDefault"] = storage.BooleanValue(*hours.Default)
				if *hours.Default {
					state := staged.Objects["BusinessHours"]
					for id, record := range state.Records {
						if record.Fields["IsDefault"].Boolean {
							record = record.Clone()
							record.Fields["IsDefault"] = storage.BooleanValue(false)
							state.Records[id] = record
						}
					}
					staged.Objects["BusinessHours"] = state
				}
			}
			for _, field := range hours.Fields {
				for _, day := range []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"} {
					for _, bound := range []string{"StartTime", "EndTime"} {
						name := day + bound
						if strings.EqualFold(field.XMLName.Local, name) && strings.TrimSpace(field.Value) != "" {
							fields[name] = storage.StringValue(strings.TrimSpace(field.Value))
						}
					}
				}
			}
			if err := upsertSetupMetadataRecord(&staged, &generator, "BusinessHours", fields, []string{"Name"}); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"BusinessHours", "DuplicateRule"} {
		org.Objects[name] = staged.Objects[name]
		storage.RebuildObjectIndexes(org, name)
	}
	org.IDSequences = generator.Sequences
	org.ClearRuntimeSchemaStamp()
	return nil
}

func upsertSetupMetadataRecord(org *storage.OrgState, generator *storage.IDGenerator, object string, fields map[string]storage.Value, keys []string) error {
	state := org.Objects[object]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	var selected storage.ID
	for id, record := range state.Records {
		if record.System.IsDeleted {
			continue
		}
		match := true
		for _, key := range keys {
			if !strings.EqualFold(record.Fields[key].String, fields[key].String) {
				match = false
				break
			}
		}
		if match && (selected == "" || id < selected) {
			selected = id
		}
	}
	if selected == "" {
		for {
			id, err := generator.Next(object)
			if err != nil {
				return err
			}
			if _, _, exists := storage.LookupRecordByID(state.Records, id); !exists {
				selected = id
				break
			}
		}
	}
	record := state.Records[selected].Clone()
	record.ID, record.Object = selected, object
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["Id"] = storage.IDValue(selected)
	for name, value := range fields {
		record.Fields[name] = value
	}
	state.Records[selected] = record
	org.Objects[object] = state
	return nil
}
