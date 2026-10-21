package resource

import (
	"encoding/xml"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

type publicGroupXML struct {
	XMLName           xml.Name  `xml:"Group"`
	FullName          string    `xml:"fullName"`
	Name              string    `xml:"name"`
	DoesIncludeBosses *bool     `xml:"doesIncludeBosses"`
	Members           *struct{} `xml:"groupMembers"`
}

// ApplyProjectGroups projects declared public groups after platform seeding.
// GS02 observes explicit boss flags, Regular type, null RelatedId and default
// email flag at API 62/67/68. Membership and sharing grants are not synthesized.
func ApplyProjectGroups(org *storage.OrgState, p project.Project) error {
	if org == nil {
		return nil
	}
	paths := projectGroupFiles(p)
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	groups := make([]publicGroupXML, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("load Group metadata %s: %w", path, err)
		}
		var group publicGroupXML
		if err := xml.Unmarshal(data, &group); err != nil {
			return fmt.Errorf("load Group metadata %s: %w", path, err)
		}
		if group.DoesIncludeBosses == nil {
			// GS01 observes different values on initial deployment and redeployment
			// at the same API. Do not infer a source-API default from those cells.
			return fmt.Errorf("load Group metadata %s: omitted doesIncludeBosses is not supported", path)
		}
		if group.Members != nil {
			return fmt.Errorf("load Group metadata %s: declared group members are not supported", path)
		}
		group.FullName = strings.TrimSpace(group.FullName)
		if group.FullName == "" {
			group.FullName = trimKnownSuffix(trimKnownSuffix(filepath.Base(path), ".group-meta.xml"), ".group")
		}
		groups = append(groups, group)
	}
	staged := *org
	staged.Objects = maps.Clone(org.Objects)
	storage.EnsureStandardObject(&staged, "Group")
	state := staged.Objects["Group"].Clone()
	byName := make(map[string]storage.ID, len(state.Records))
	for id, r := range state.Records {
		name := strings.ToLower(r.Fields["DeveloperName"].String)
		if previous := byName[name]; previous == "" || id < previous {
			byName[name] = id
		}
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences["Group"] = org.IDSequences["Group"]
	for _, group := range groups {
		name := strings.ToLower(group.FullName)
		id := byName[name]
		record, exists := state.Records[id]
		if exists {
			if record.Fields["Type"].String != "Regular" {
				return fmt.Errorf("load Group metadata %s: existing group is not Regular", group.FullName)
			}
			record = record.Clone()
		} else {
			for {
				var err error
				id, err = generator.Next("Group")
				if err != nil {
					return err
				}
				if _, _, occupied := storage.LookupRecordByID(state.Records, id); !occupied {
					break
				}
			}
			byName[name] = id
			record = storage.Record{ID: id, Object: "Group", Fields: make(map[string]storage.Value)}
			record.Fields["RelatedId"] = storage.NullValue()
			record.Fields["DoesSendEmailToMembers"] = storage.BooleanValue(false)
		}
		record.Fields["Id"] = storage.IDValue(id)
		record.Fields["DeveloperName"] = storage.StringValue(group.FullName)
		record.Fields["Name"] = storage.StringValue(group.Name)
		record.Fields["Type"] = storage.StringValue("Regular")
		record.Fields["DoesIncludeBosses"] = storage.BooleanValue(*group.DoesIncludeBosses)
		state.Records[id] = record
	}
	if org.Objects == nil {
		org.Objects = make(map[string]storage.ObjectState)
	}
	org.Objects["Group"] = state
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	org.IDSequences["Group"] = generator.Sequences["Group"]
	org.ClearRuntimeSchemaStamp()
	return nil
}

func projectGroupFiles(p project.Project) []string {
	files := append([]string(nil), p.GroupFiles...)
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project != nil {
			files = append(files, projectGroupFiles(*dep.Project)...)
		}
	}
	return files
}
