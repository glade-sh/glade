package resource

import (
	"encoding/xml"
	"fmt"
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

type ownerSharingRulesXML struct {
	XMLName xml.Name                     `xml:"SharingRules"`
	Rules   []ownerSharingRuleXML        `xml:"sharingOwnerRules"`
	Other   []struct{ XMLName xml.Name } `xml:",any"`
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
	staged := make(map[string][]storage.OwnerSharingRule)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("load owner sharing metadata %s: %w", path, err)
		}
		var source ownerSharingRulesXML
		if err := xml.Unmarshal(data, &source); err != nil {
			return fmt.Errorf("load owner sharing metadata %s: %w", path, err)
		}
		if len(source.Rules) != 1 || len(source.Other) != 0 {
			return fmt.Errorf("load owner sharing metadata %s: only a single owner rule is supported", path)
		}
		name := trimKnownSuffix(trimKnownSuffix(filepath.Base(path), ".sharingRules-meta.xml"), ".sharingRules")
		object, ok := org.Objects[name]
		if !ok || !strings.HasSuffix(name, "__c") || object.Definition.SharingModel != "Private" {
			return fmt.Errorf("load owner sharing metadata %s: a Private custom object is required", path)
		}
		if _, duplicate := staged[name]; duplicate {
			return fmt.Errorf("load owner sharing metadata %s: duplicate object rule source is not supported", path)
		}
		r := source.Rules[0]
		if strings.TrimSpace(r.FullName) == "" || r.AccessLevel != "Read" || len(r.Other) != 0 || len(r.SharedFrom.Other) != 0 || len(r.SharedTo.Other) != 0 || r.SharedFrom.Group == "" || r.SharedTo.Group == "" {
			return fmt.Errorf("load owner sharing metadata %s: only Read rules between public groups are supported", path)
		}
		for _, groupName := range []string{r.SharedFrom.Group, r.SharedTo.Group} {
			if !declaredGroups[groupName] {
				return fmt.Errorf("load owner sharing metadata %s: group %s is not declared", path, groupName)
			}
			found := false
			for _, group := range org.Objects["Group"].Records {
				if group.Fields["DeveloperName"].String == groupName && group.Fields["Type"].String == "Regular" {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("load owner sharing metadata %s: group %s is not loaded", path, groupName)
			}
		}
		staged[name] = []storage.OwnerSharingRule{{Name: r.FullName, SourceGroup: r.SharedFrom.Group, TargetGroup: r.SharedTo.Group, AccessLevel: r.AccessLevel}}
	}
	for name, rules := range staged {
		object := org.Objects[name]
		object.Definition = object.Definition.Clone()
		object.Definition.OwnerSharingRules = rules
		org.Objects[name] = object
	}
	org.ClearRuntimeSchemaStamp()
	return nil
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
