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

type roleXML struct {
	XMLName                 xml.Name `xml:"Role"`
	FullName                string   `xml:"fullName"`
	Name                    string   `xml:"name"`
	ParentRole              string   `xml:"parentRole"`
	CaseAccessLevel         string   `xml:"caseAccessLevel"`
	ContactAccessLevel      string   `xml:"contactAccessLevel"`
	OpportunityAccessLevel  string   `xml:"opportunityAccessLevel"`
	Description             *string  `xml:"description"`
	MayForecastManagerShare *bool    `xml:"mayForecastManagerShare"`
}

// ApplyProjectRoles materializes declared role identities and hierarchy links.
// Call after platform seeding: existing setup identities must reserve their IDs
// before authored roles are allocated. No source roles means no setup changes.
// RS03 observes the label, developer name, parent link, omitted/Edit access
// fields, description and explicit forecast flag (API 62/67/68).
func ApplyProjectRoles(org *storage.OrgState, p project.Project) error {
	if org == nil {
		return nil
	}
	paths := projectRoleFiles(p)
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	roles := make([]roleXML, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("load Role metadata %s: %w", path, err)
		}
		var role roleXML
		if err := xml.Unmarshal(data, &role); err != nil {
			return fmt.Errorf("load Role metadata %s: %w", path, err)
		}
		role.FullName = strings.TrimSpace(role.FullName)
		if role.FullName == "" {
			role.FullName = trimKnownSuffix(trimKnownSuffix(filepath.Base(path), ".role-meta.xml"), ".role")
		}
		// Lower access levels deploy only when organization-wide defaults allow
		// them. Scratch-org rows (API 62, Case and Opportunity Private) project
		// Read and None as declared; a project that deployed has valid defaults.
		for _, level := range []string{role.CaseAccessLevel, role.ContactAccessLevel, role.OpportunityAccessLevel} {
			switch level {
			case "", "Edit", "Read", "None":
			default:
				return fmt.Errorf("load Role metadata %s: access level %q is not supported", role.FullName, level)
			}
		}
		roles = append(roles, role)
	}
	// Stage only this setup object so an unresolved parent never partially loads roles.
	staged := *org
	staged.Objects = maps.Clone(org.Objects)
	storage.EnsureStandardObject(&staged, "UserRole")
	state := staged.Objects["UserRole"].Clone()
	byName := make(map[string]storage.ID, len(state.Records))
	for id, record := range state.Records {
		name := strings.ToLower(record.Fields["DeveloperName"].String)
		if previous := byName[name]; previous == "" || id < previous {
			byName[name] = id
		}
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences["UserRole"] = org.IDSequences["UserRole"]
	for _, role := range roles {
		name := strings.ToLower(role.FullName)
		id := byName[name]
		record, exists := state.Records[id]
		if !exists {
			for {
				var err error
				id, err = generator.Next("UserRole")
				if err != nil {
					return err
				}
				if _, _, occupied := storage.LookupRecordByID(state.Records, id); !occupied {
					break
				}
			}
			byName[name] = id
			record = storage.Record{ID: id, Object: "UserRole", Fields: make(map[string]storage.Value)}
		} else {
			record = record.Clone()
		}
		record.Fields["Id"] = storage.IDValue(id)
		record.Fields["DeveloperName"] = storage.StringValue(role.FullName)
		record.Fields["Name"] = storage.StringValue(role.Name)
		for field, value := range map[string]string{
			"CaseAccessForAccountOwner":        role.CaseAccessLevel,
			"ContactAccessForAccountOwner":     role.ContactAccessLevel,
			"OpportunityAccessForAccountOwner": role.OpportunityAccessLevel,
		} {
			if value != "" {
				record.Fields[field] = storage.StringValue(value)
			} else if _, ok := record.Fields[field]; !ok {
				record.Fields[field] = storage.StringValue("Edit")
			}
		}
		if role.Description != nil {
			record.Fields["RollupDescription"] = storage.StringValue(*role.Description)
		}
		if role.MayForecastManagerShare != nil {
			record.Fields["MayForecastManagerShare"] = storage.BooleanValue(*role.MayForecastManagerShare)
		}
		state.Records[id] = record
	}
	// Allocate every identity before resolving parent names, independent of file order.
	for _, role := range roles {
		id := byName[strings.ToLower(role.FullName)]
		record := state.Records[id]
		parent := strings.TrimSpace(role.ParentRole)
		record.Fields["ParentRoleId"] = storage.NullValue()
		if parent != "" {
			parentID, ok := byName[strings.ToLower(parent)]
			if !ok {
				return fmt.Errorf("load Role metadata %s: parent role %q is not available", role.FullName, parent)
			}
			record.Fields["ParentRoleId"] = storage.IDValue(parentID)
		}
		state.Records[id] = record
	}
	if org.Objects == nil {
		org.Objects = make(map[string]storage.ObjectState)
	}
	org.Objects["UserRole"] = state
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	org.IDSequences["UserRole"] = generator.Sequences["UserRole"]
	org.ClearRuntimeSchemaStamp()
	return nil
}

func projectRoleFiles(p project.Project) []string {
	files := append([]string(nil), p.RoleFiles...)
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project != nil {
			files = append(files, projectRoleFiles(*dep.Project)...)
		}
	}
	return files
}
