package resource

import (
	"encoding/xml"
	"fmt"
	"maps"
	"path/filepath"
	"sort"
	"strconv"
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
	owners := setupRecordMetadataOwners(p)
	paths := make([]string, 0, len(owners))
	for path := range owners {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// setupRecordMetadataOwners maps each declared file to its package view. The
// root project owns "" and a dependency or shim owns its kind and namespace.
func setupRecordMetadataOwners(p project.Project) map[string]string {
	files := make(map[string]string)
	seen := make(map[*project.Project]bool)
	var visit func(*project.Project, string)
	visit = func(current *project.Project, owner string) {
		if current == nil || seen[current] {
			return
		}
		seen[current] = true
		lists := [][]string{current.BusinessHoursSettingsFiles, current.AutoResponseRuleFiles, current.ApprovalProcessFiles, current.AssignmentRuleFiles, current.MatchingRuleFiles, current.PlatformEventChannelMemberFiles, current.DuplicateRuleFiles}
		if len(current.AutoResponseRuleFiles) > 0 {
			lists = append(lists, current.EmailTemplateFiles)
		}
		for _, list := range lists {
			for _, path := range list {
				if _, exists := files[path]; !exists {
					files[path] = owner
				}
			}
		}
		for _, dep := range current.ManagedPackageDependencies {
			if dep.Status == "loaded" {
				visit(dep.Project, "dependency:"+dep.Namespace)
			}
		}
		for _, shim := range current.PackageShims {
			if shim.Status == "loaded" {
				visit(shim.Project, "shim:"+shim.Namespace)
			}
		}
	}
	visit(&p, "")
	return files
}

func applySetupRecordMetadata(org *storage.OrgState, p project.Project, readMetadata func(string) ([]byte, error)) error {
	if org == nil {
		return nil
	}
	owners := setupRecordMetadataOwners(p)
	paths := SetupRecordMetadataFiles(p)
	if len(paths) == 0 {
		return nil
	}
	// Native MD01/ST01: deployed rules sit beside the org's default rules.
	for _, path := range paths {
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".duplicaterule-meta.xml") || strings.HasSuffix(lower, ".duplicaterule") {
			storage.EnsureDefaultDuplicateRules(org)
			break
		}
	}
	staged := *org
	staged.Metadata.ChangeEventEntities = append([]string(nil), org.Metadata.ChangeEventEntities...)
	staged.Objects = maps.Clone(org.Objects)
	if staged.Objects == nil {
		staged.Objects = make(map[string]storage.ObjectState)
	}
	staged.IDSequences = maps.Clone(org.IDSequences)
	if staged.IDSequences == nil {
		staged.IDSequences = make(map[string]uint64)
	}
	for _, name := range []string{"BusinessHours", "DuplicateRule", "AssignmentRule", "ProcessDefinition"} {
		if state, exists := org.Objects[name]; exists {
			staged.Objects[name] = state.Clone()
		}
		storage.EnsureStandardObject(&staged, name)
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences = staged.IDSequences
	// Assignment metadata uses the local runtime identity range.
	assignmentIDs := storage.NewRuntimeIDGeneratorForOrg(&staged)
	assignmentIDs.Sequences = staged.IDSequences
	for _, path := range paths {
		body, err := readMetadata(path)
		if err != nil {
			return fmt.Errorf("load setup metadata %s: %w", path, err)
		}
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".autoresponserules-meta.xml") || strings.HasSuffix(lower, ".autoresponserules") || strings.HasSuffix(lower, ".email-meta.xml") || strings.HasSuffix(lower, ".email") {
			continue // Captured for the auto-response loader, not setup rows.
		}
		if strings.HasSuffix(lower, ".platformeventchannelmember-meta.xml") || strings.HasSuffix(lower, ".platformeventchannelmember") {
			if err := applyChangeEventChannelMember(&staged, body); err != nil {
				return fmt.Errorf("load PlatformEventChannelMember metadata %s: %w", path, err)
			}
			continue
		}
		if strings.HasSuffix(lower, ".approvalprocess-meta.xml") || strings.HasSuffix(lower, ".approvalprocess") {
			var definition struct {
				XMLName     xml.Name `xml:"ApprovalProcess"`
				Active      bool     `xml:"active"`
				AllowRecall bool     `xml:"allowRecall"`
				Label       string   `xml:"label"`
				Allowed     []struct {
					Type string `xml:"type"`
				} `xml:"allowedSubmitters"`
				RecordEditability string `xml:"recordEditability"`
				Steps             []struct {
					AllowDelegate  *bool `xml:"allowDelegate"`
					RejectBehavior struct {
						Type string `xml:"type"`
					} `xml:"rejectBehavior"`
					Assigned struct {
						Approvers []struct {
							Name string `xml:"name"`
							Type string `xml:"type"`
						} `xml:"approver"`
						Multiple string `xml:"whenMultipleApprovers"`
					} `xml:"assignedApprover"`
					Criteria         *struct{}  `xml:"entryCriteria"`
					ApprovalActions  []struct{} `xml:"approvalActions"`
					RejectionActions []struct{} `xml:"rejectionActions"`
				} `xml:"approvalStep"`
				InitialActions     []struct{} `xml:"initialSubmissionActions"`
				ApprovalActions    []struct{} `xml:"finalApprovalActions"`
				RejectionActions   []struct{} `xml:"finalRejectionActions"`
				RecallActions      []struct{} `xml:"recallActions"`
				FinalApprovalLock  *bool      `xml:"finalApprovalRecordLock"`
				FinalRejectionLock *bool      `xml:"finalRejectionRecordLock"`
				Criteria           struct {
					Formula string     `xml:"formula"`
					Items   []struct{} `xml:"criteriaItems"`
				} `xml:"entryCriteria"`
			}
			if err := xml.Unmarshal(body, &definition); err != nil {
				return fmt.Errorf("load ApprovalProcess metadata %s: %w", path, err)
			}
			fullName := trimKnownSuffix(trimKnownSuffix(filepath.Base(path), ".approvalProcess-meta.xml"), ".approvalProcess")
			object, name, ok := strings.Cut(fullName, ".")
			if !ok || object == "" || name == "" {
				return fmt.Errorf("load ApprovalProcess metadata %s: expected object and developer name", path)
			}
			state := "Inactive"
			if definition.Active {
				state = "Active"
			}
			criteria := strings.TrimSpace(definition.Criteria.Formula)
			if len(definition.Criteria.Items) != 0 || criteria == "" {
				criteria = "unsupported"
			}
			routing := "unsupported"
			if len(definition.Allowed) == 1 && strings.EqualFold(definition.Allowed[0].Type, "owner") && len(definition.Steps) == 1 && strings.EqualFold(definition.RecordEditability, "AdminOnly") && definition.Steps[0].AllowDelegate != nil && !*definition.Steps[0].AllowDelegate && strings.EqualFold(definition.Steps[0].RejectBehavior.Type, "RejectRequest") && len(definition.Steps[0].Assigned.Approvers) == 1 && strings.EqualFold(definition.Steps[0].Assigned.Approvers[0].Name, "Owner") && strings.EqualFold(definition.Steps[0].Assigned.Approvers[0].Type, "relatedUserField") && strings.EqualFold(definition.Steps[0].Assigned.Multiple, "FirstResponse") && definition.Steps[0].Criteria == nil && len(definition.Steps[0].ApprovalActions) == 0 && len(definition.Steps[0].RejectionActions) == 0 && len(definition.InitialActions) == 0 && len(definition.ApprovalActions) == 0 && len(definition.RejectionActions) == 0 && len(definition.RecallActions) == 0 && definition.FinalApprovalLock != nil && !*definition.FinalApprovalLock && definition.FinalRejectionLock != nil && !*definition.FinalRejectionLock {
				routing = "owner"
			}
			// Only the root project's definitions are captured shapes. A package
			// view keeps its own row and never routes locally.
			owner := owners[path]
			if owner != "" {
				routing = "unsupported"
			}
			fields := map[string]storage.Value{"DeveloperName": storage.StringValue(name), "Name": storage.StringValue(definition.Label), "State": storage.StringValue(state), "TableEnumOrId": storage.StringValue(object), "Type": storage.StringValue("Approval"), "GladeApprovalCriteria": storage.StringValue(criteria), "GladeApprovalRouting": storage.StringValue(routing), "GladeApprovalPackage": storage.StringValue(owner), "GladeApprovalRecall": storage.StringValue(strconv.FormatBool(definition.AllowRecall))}
			if err := upsertSetupMetadataRecord(&staged, &generator, "ProcessDefinition", fields, []string{"DeveloperName", "TableEnumOrId", "GladeApprovalPackage"}); err != nil {
				return err
			}
			continue
		}
		if strings.HasSuffix(lower, ".assignmentrules-meta.xml") || strings.HasSuffix(lower, ".assignmentrules") {
			var document struct {
				XMLName xml.Name `xml:"AssignmentRules"`
				Rules   []struct {
					Name   string `xml:"fullName"`
					Active bool   `xml:"active"`
				} `xml:"assignmentRule"`
			}
			if err := xml.Unmarshal(body, &document); err != nil {
				return fmt.Errorf("load AssignmentRule metadata %s: %w", path, err)
			}
			object := trimKnownSuffix(trimKnownSuffix(filepath.Base(path), ".assignmentRules-meta.xml"), ".assignmentRules")
			for _, rule := range document.Rules {
				if strings.TrimSpace(rule.Name) == "" {
					return fmt.Errorf("load AssignmentRule metadata %s: missing fullName", path)
				}
				fields := map[string]storage.Value{"Name": storage.StringValue(rule.Name), "SobjectType": storage.StringValue(object), "Active": storage.BooleanValue(rule.Active)}
				if err := upsertSetupMetadataRecord(&staged, &assignmentIDs, "AssignmentRule", fields, []string{"Name", "SobjectType"}); err != nil {
					return err
				}
			}
			continue
		}
		if strings.HasSuffix(lower, ".matchingrule-meta.xml") || strings.HasSuffix(lower, ".matchingrule") {
			continue // Matching declarations are installed by the automation loader.
		}
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
	for _, name := range []string{"BusinessHours", "DuplicateRule", "AssignmentRule", "ProcessDefinition"} {
		org.Objects[name] = staged.Objects[name]
		storage.RebuildObjectIndexes(org, name)
	}
	org.Metadata.ChangeEventEntities = staged.Metadata.ChangeEventEntities
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
