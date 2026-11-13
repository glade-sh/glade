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

type ownerSharingSelectorXML struct {
	Group string                       `xml:"group"`
	Other []struct{ XMLName xml.Name } `xml:",any"`
}

type ownerSharingRuleXML struct {
	FullName    string                       `xml:"fullName"`
	AccessLevel string                       `xml:"accessLevel"`
	Description string                       `xml:"description"`
	Label       string                       `xml:"label"`
	SharedFrom  ownerSharingSelectorXML      `xml:"sharedFrom"`
	SharedTo    ownerSharingSelectorXML      `xml:"sharedTo"`
	Other       []struct{ XMLName xml.Name } `xml:",any"`
}

type criteriaSharingItemXML struct {
	Field     string                       `xml:"field"`
	Operation string                       `xml:"operation"`
	Value     string                       `xml:"value"`
	Other     []struct{ XMLName xml.Name } `xml:",any"`
}

type criteriaSharingRuleXML struct {
	FullName    string                       `xml:"fullName"`
	AccessLevel string                       `xml:"accessLevel"`
	Description string                       `xml:"description"`
	Label       string                       `xml:"label"`
	SharedTo    ownerSharingSelectorXML      `xml:"sharedTo"`
	Items       []criteriaSharingItemXML     `xml:"criteriaItems"`
	Other       []struct{ XMLName xml.Name } `xml:",any"`
}

type ownerSharingRulesXML struct {
	XMLName  xml.Name                     `xml:"SharingRules"`
	Rules    []ownerSharingRuleXML        `xml:"sharingOwnerRules"`
	Criteria []criteriaSharingRuleXML     `xml:"sharingCriteriaRules"`
	Other    []struct{ XMLName xml.Name } `xml:",any"`
}

// ApplyProjectOwnerSharingRules attaches declared configuration, without granting
// access or creating membership. Native API 62/67/68 observations cover one Read
// owner rule between public groups on a Private custom object with direct users.
func ApplyProjectOwnerSharingRules(org *storage.OrgState, p project.Project) error {
	if org == nil {
		return nil
	}
	paths := projectOwnerSharingRuleFiles(p)
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	declaredGroups := make(map[string]bool)
	for _, path := range projectGroupFiles(p) {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("load owner sharing group %s: %w", path, err)
		}
		var group publicGroupXML
		if err := xml.Unmarshal(data, &group); err != nil {
			return fmt.Errorf("load owner sharing group %s: %w", path, err)
		}
		name := strings.TrimSpace(group.FullName)
		if name == "" {
			name = trimKnownSuffix(trimKnownSuffix(filepath.Base(path), ".group-meta.xml"), ".group")
		}
		if group.DoesIncludeBosses != nil && group.Members == nil {
			declaredGroups[name] = true
		}
	}
	type stagedRules struct {
		owner    []storage.OwnerSharingRule
		criteria []storage.CriteriaSharingRule
	}
	staged := make(map[string]*stagedRules)
	requireGroup := func(path, groupName string) error {
		if !declaredGroups[groupName] {
			return fmt.Errorf("load owner sharing metadata %s: group %s is not declared", path, groupName)
		}
		for _, group := range org.Objects["Group"].Records {
			if group.Fields["DeveloperName"].String == groupName && group.Fields["Type"].String == "Regular" {
				return nil
			}
		}
		return fmt.Errorf("load owner sharing metadata %s: group %s is not loaded", path, groupName)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("load owner sharing metadata %s: %w", path, err)
		}
		var source ownerSharingRulesXML
		if err := xml.Unmarshal(data, &source); err != nil {
			return fmt.Errorf("load owner sharing metadata %s: %w", path, err)
		}
		if len(source.Rules) == 0 && len(source.Criteria) == 0 || len(source.Other) != 0 {
			return fmt.Errorf("load owner sharing metadata %s: only owner and criteria rules are supported", path)
		}
		name := trimKnownSuffix(trimKnownSuffix(filepath.Base(path), ".sharingRules-meta.xml"), ".sharingRules")
		object, ok := org.Objects[name]
		if !ok || object.Definition.SharingModel != "Private" {
			return fmt.Errorf("load owner sharing metadata %s: a Private object is required", path)
		}
		if len(source.Rules) != 0 && !strings.HasSuffix(name, "__c") {
			return fmt.Errorf("load owner sharing metadata %s: a Private custom object is required", path)
		}
		if len(source.Criteria) != 0 && name != "Case" {
			return fmt.Errorf("load owner sharing metadata %s: criteria rules are supported on Case only", path)
		}
		if _, duplicate := staged[name]; duplicate {
			return fmt.Errorf("load owner sharing metadata %s: duplicate object rule source is not supported", path)
		}
		rules := &stagedRules{}
		seen := make(map[string]bool)
		for _, r := range source.Rules {
			key := strings.ToLower(strings.TrimSpace(r.FullName))
			if seen[key] {
				return fmt.Errorf("load owner sharing metadata %s: duplicate owner rule name", path)
			}
			seen[key] = true
			if strings.TrimSpace(r.FullName) == "" || r.AccessLevel != "Read" || len(r.Other) != 0 || len(r.SharedFrom.Other) != 0 || len(r.SharedTo.Other) != 0 || r.SharedFrom.Group == "" || r.SharedTo.Group == "" {
				return fmt.Errorf("load owner sharing metadata %s: only Read rules between public groups are supported", path)
			}
			for _, groupName := range []string{r.SharedFrom.Group, r.SharedTo.Group} {
				if err := requireGroup(path, groupName); err != nil {
					return err
				}
			}
			rules.owner = append(rules.owner, storage.OwnerSharingRule{Name: r.FullName, SourceGroup: r.SharedFrom.Group, TargetGroup: r.SharedTo.Group, AccessLevel: r.AccessLevel})
		}
		criteriaSeen := make(map[string]bool)
		for _, r := range source.Criteria {
			rule, err := criteriaRuleFromXML(object.Definition, r)
			if err != nil {
				return fmt.Errorf("load owner sharing metadata %s: %w", path, err)
			}
			key := strings.ToLower(rule.Name)
			if criteriaSeen[key] {
				return fmt.Errorf("load owner sharing metadata %s: duplicate criteria rule name", path)
			}
			criteriaSeen[key] = true
			if err := requireGroup(path, rule.TargetGroup); err != nil {
				return err
			}
			rules.criteria = append(rules.criteria, rule)
		}
		staged[name] = rules
	}
	for name, rules := range staged {
		object := org.Objects[name]
		object.Definition = object.Definition.Clone()
		object.Definition.OwnerSharingRules = rules.owner
		object.Definition.CriteriaSharingRules = rules.criteria
		org.Objects[name] = object
		if len(rules.criteria) != 0 {
			// A rule's grant is a share row for a group member. Both tables must
			// exist in a lean database org, not only after a lazy first reference.
			storage.EnsureStandardObject(org, name+"Share")
			storage.EnsureStandardObject(org, "GroupMember")
		}
	}
	org.ClearRuntimeSchemaStamp()
	return nil
}

// criteriaRuleFromXML accepts the captured shape: a Read rule for one public
// group whose items all compare a stored text field by equals or notEqual to
// one literal. Everything else is refused at load, so no unmodeled rule runs.
func criteriaRuleFromXML(definition storage.ObjectDefinition, r criteriaSharingRuleXML) (storage.CriteriaSharingRule, error) {
	name := strings.TrimSpace(r.FullName)
	if name == "" || r.AccessLevel != "Read" || len(r.Other) != 0 || len(r.SharedTo.Other) != 0 || r.SharedTo.Group == "" || len(r.Items) == 0 {
		return storage.CriteriaSharingRule{}, fmt.Errorf("only Read criteria rules for one public group are supported")
	}
	rule := storage.CriteriaSharingRule{Name: name, TargetGroup: r.SharedTo.Group, AccessLevel: r.AccessLevel}
	for _, item := range r.Items {
		operation := strings.TrimSpace(item.Operation)
		if len(item.Other) != 0 || operation != "equals" && operation != "notEqual" || strings.Contains(item.Value, ",") {
			return storage.CriteriaSharingRule{}, fmt.Errorf("criteria rule %s: only equals and notEqual against one literal are supported", name)
		}
		field, ok := criteriaFieldDefinition(definition, strings.TrimSpace(item.Field))
		if !ok || field.Type != storage.FieldString && field.Type != storage.FieldPicklist {
			return storage.CriteriaSharingRule{}, fmt.Errorf("criteria rule %s: field %s is not a supported text field", name, item.Field)
		}
		rule.Criteria = append(rule.Criteria, storage.CriteriaSharingCondition{Field: field.APIName, Operation: operation, Value: item.Value})
	}
	return rule, nil
}

func criteriaFieldDefinition(definition storage.ObjectDefinition, name string) (storage.Field, bool) {
	for key, field := range definition.Fields {
		if strings.EqualFold(key, name) {
			if field.APIName == "" {
				field.APIName = key
			}
			return field, true
		}
	}
	return storage.Field{}, false
}

func projectOwnerSharingRuleFiles(p project.Project) []string {
	files := append([]string(nil), p.OwnerSharingRuleFiles...)
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project != nil {
			files = append(files, projectOwnerSharingRuleFiles(*dep.Project)...)
		}
	}
	return files
}

// RefreshProjectOwnerSharingRules replaces project-owned rule configuration on
// a retained org. Stage group metadata and all rule validation before making
// the replacement, so removal revokes stale grants and errors are atomic.
func RefreshProjectOwnerSharingRules(org *storage.OrgState, p project.Project) error {
	if org == nil {
		return nil
	}
	staged := *org
	staged.Objects = maps.Clone(org.Objects)
	staged.IDSequences = maps.Clone(org.IDSequences)
	for name, object := range staged.Objects {
		if len(object.Definition.OwnerSharingRules) == 0 && len(object.Definition.CriteriaSharingRules) == 0 {
			continue
		}
		object.Definition = object.Definition.Clone()
		object.Definition.OwnerSharingRules = nil
		object.Definition.CriteriaSharingRules = nil
		staged.Objects[name] = object
	}
	if len(projectOwnerSharingRuleFiles(p)) != 0 {
		// Only hydrate supported direct groups. Unsupported unrelated groups do
		// not affect these rules; validation below still rejects a rule that
		// references a group outside the supported direct-membership shape.
		var supportedGroups []string
		for _, path := range projectGroupFiles(p) {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("load owner sharing group %s: %w", path, err)
			}
			var group publicGroupXML
			if err := xml.Unmarshal(data, &group); err != nil {
				return fmt.Errorf("load owner sharing group %s: %w", path, err)
			}
			if group.DoesIncludeBosses != nil && group.Members == nil {
				supportedGroups = append(supportedGroups, path)
			}
		}
		if err := ApplyProjectGroups(&staged, project.Project{GroupFiles: supportedGroups}); err != nil {
			return err
		}
		if err := ApplyProjectOwnerSharingRules(&staged, p); err != nil {
			return err
		}
	}
	staged.ClearRuntimeSchemaStamp()
	*org = staged
	return nil
}
