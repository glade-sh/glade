package automation

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func loadAssignmentRules(p project.Project, read func(string) ([]byte, error)) ([]storage.AssignmentRule, []diagnostic.Diagnostic) {
	var rules []storage.AssignmentRule
	var diagnostics []diagnostic.Diagnostic
	for _, path := range assignmentRuleFiles(p) {
		object := ruleMetadataName(path, "assignmentRules")
		var document struct {
			XMLName xml.Name `xml:"AssignmentRules"`
			Rules   []struct {
				Name    string `xml:"fullName"`
				Active  bool   `xml:"active"`
				Entries []struct {
					AssignedTo     string                    `xml:"assignedTo"`
					AssignedToType string                    `xml:"assignedToType"`
					Formula        string                    `xml:"formula"`
					BooleanFilter  string                    `xml:"booleanFilter"`
					Criteria       []workflowCriteriaItemXML `xml:"criteriaItems"`
				} `xml:"ruleEntry"`
			} `xml:"assignmentRule"`
		}
		body, err := read(path)
		if err == nil {
			err = xml.Unmarshal(body, &document)
		}
		if err != nil {
			diagnostics = append(diagnostics, loadFailure(path, "assignment rule", object, err))
			continue
		}
		for _, raw := range document.Rules {
			rule := storage.AssignmentRule{Object: object, Name: strings.TrimSpace(raw.Name), Active: raw.Active}
			if rule.Name == "" {
				diagnostics = append(diagnostics, loadFailure(path, "assignment rule", object, fmt.Errorf("missing fullName")))
				continue
			}
			for _, entry := range raw.Entries {
				next := storage.AssignmentRuleEntry{AssignedTo: strings.TrimSpace(entry.AssignedTo), AssignedToType: strings.TrimSpace(entry.AssignedToType), Formula: strings.TrimSpace(entry.Formula), BooleanFilter: strings.TrimSpace(entry.BooleanFilter)}
				for _, criterion := range entry.Criteria {
					next.Criteria = append(next.Criteria, storage.WorkflowCriteriaItem{Field: trimObjectPrefix(strings.TrimSpace(criterion.Field)), Operation: strings.TrimSpace(criterion.Operation), Value: criterion.Value})
				}
				rule.Entries = append(rule.Entries, next)
			}
			rules = append(rules, rule)
		}
	}
	return rules, diagnostics
}

func applyAssignmentRules(org *storage.OrgState, rules []storage.AssignmentRule) {
	grouped := map[string][]storage.AssignmentRule{}
	for _, rule := range rules {
		name, ok := storage.ResolveObjectName(*org, rule.Object)
		if !ok {
			continue
		}
		for id, row := range org.Objects["AssignmentRule"].Records {
			if !row.System.IsDeleted && row.Fields["Name"].String == rule.Name && strings.EqualFold(row.Fields["SobjectType"].String, name) && (rule.ID == "" || id < rule.ID) {
				rule.ID = id
			}
		}
		grouped[name] = append(grouped[name], rule)
	}
	for name, rules := range grouped {
		state := org.Objects[name]
		state.Definition.AssignmentRules = storage.CloneAssignmentRules(rules)
		org.Objects[name] = state
	}
}

// Match the declared project views used for setup rows and source snapshots.
func assignmentRuleFiles(p project.Project) []string {
	paths := map[string]bool{}
	seen := map[*project.Project]bool{}
	var visit func(*project.Project)
	visit = func(current *project.Project) {
		if current == nil || seen[current] {
			return
		}
		seen[current] = true
		for _, path := range current.AssignmentRuleFiles {
			paths[path] = true
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
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}
