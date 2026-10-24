package automation

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

type duplicateRuleXML struct {
	XMLName            xml.Name                    `xml:"DuplicateRule"`
	Label              string                      `xml:"masterLabel"`
	Active             bool                        `xml:"isActive"`
	ActionOnInsert     string                      `xml:"actionOnInsert"`
	ActionOnUpdate     string                      `xml:"actionOnUpdate"`
	OperationsOnInsert []string                    `xml:"operationsOnInsert"`
	OperationsOnUpdate []string                    `xml:"operationsOnUpdate"`
	AlertText          string                      `xml:"alertText"`
	SecurityOption     string                      `xml:"securityOption"`
	SortOrder          int                         `xml:"sortOrder"`
	Filter             storage.DuplicateRuleFilter `xml:"duplicateRuleFilter"`
	Matches            []struct {
		Object string `xml:"matchRuleSObjectType"`
		Name   string `xml:"matchingRule"`
	} `xml:"duplicateRuleMatchRules"`
}

func ruleMetadataName(path, kind string) string {
	name := filepath.Base(path)
	for _, suffix := range []string{"." + kind + "-meta.xml", "." + kind} {
		if strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}

func matchingRuleKey(object, name string) string {
	return strings.ToLower(strings.TrimSpace(object) + "." + strings.TrimSpace(name))
}

func loadDuplicateRules(p project.Project, read func(string) ([]byte, error)) ([]storage.DuplicateRule, []diagnostic.Diagnostic) {
	var diagnostics []diagnostic.Diagnostic
	matches := make(map[string]storage.MatchingRule)
	for _, path := range p.MatchingRuleFiles {
		var document struct {
			XMLName xml.Name               `xml:"MatchingRules"`
			Rules   []storage.MatchingRule `xml:"matchingRules"`
		}
		object := ruleMetadataName(path, "matchingRule")
		body, err := read(path)
		if err == nil {
			err = xml.Unmarshal(body, &document)
		}
		if err != nil {
			diagnostics = append(diagnostics, loadFailure(path, "matching rule", object, err))
			continue
		}
		for _, rule := range document.Rules {
			rule.Object = object
			rule.Name = strings.TrimSpace(rule.Name)
			if rule.Name == "" {
				diagnostics = append(diagnostics, loadFailure(path, "matching rule", object, fmt.Errorf("missing fullName")))
				continue
			}
			matches[matchingRuleKey(object, rule.Name)] = rule
		}
	}
	var rules []storage.DuplicateRule
	for _, path := range p.DuplicateRuleFiles {
		fullName := ruleMetadataName(path, "duplicateRule")
		object, name, ok := strings.Cut(fullName, ".")
		if !ok || object == "" || name == "" {
			diagnostics = append(diagnostics, loadFailure(path, "duplicate rule", fullName, fmt.Errorf("expected object and developer name")))
			continue
		}
		var raw duplicateRuleXML
		body, err := read(path)
		if err == nil {
			err = xml.Unmarshal(body, &raw)
		}
		if err != nil {
			diagnostics = append(diagnostics, loadFailure(path, "duplicate rule", fullName, err))
			continue
		}
		rule := storage.DuplicateRule{
			Object: object, Name: name, Label: raw.Label, Active: raw.Active,
			ActionOnInsert: raw.ActionOnInsert, ActionOnUpdate: raw.ActionOnUpdate,
			OperationsOnInsert: raw.OperationsOnInsert, OperationsOnUpdate: raw.OperationsOnUpdate,
			AlertText: raw.AlertText, SecurityOption: raw.SecurityOption, SortOrder: raw.SortOrder, Filter: raw.Filter,
		}
		if strings.EqualFold(object, "PersonAccount") {
			rule.Object, rule.Subtype = "Account", "PersonAccount"
		}
		for _, ref := range raw.Matches {
			match, found := matches[matchingRuleKey(ref.Object, ref.Name)]
			if !found {
				match = storage.MatchingRule{Object: ref.Object, Name: ref.Name}
			}
			rule.MatchRules = append(rule.MatchRules, match)
		}
		rules = append(rules, rule)
	}
	sort.SliceStable(rules, func(i, j int) bool {
		a, b := rules[i], rules[j]
		if a.Object != b.Object {
			return a.Object < b.Object
		}
		if a.SortOrder != b.SortOrder {
			return a.SortOrder < b.SortOrder
		}
		return a.Name < b.Name
	})
	return rules, diagnostics
}

func applyDuplicateRules(org *storage.OrgState, rules []storage.DuplicateRule) {
	grouped := make(map[string][]storage.DuplicateRule)
	for _, rule := range rules {
		name, ok := storage.ResolveObjectName(*org, rule.Object)
		if !ok {
			continue
		}
		// Resolve the same record identity used by setup metadata SOQL rows.
		for id, row := range org.Objects["DuplicateRule"].Records {
			if row.System.IsDeleted || row.Fields["DeveloperName"].String != rule.Name || !strings.EqualFold(row.Fields["SobjectType"].String, name) || !strings.EqualFold(row.Fields["SobjectSubtype"].String, rule.Subtype) {
				continue
			}
			if rule.ID == "" || id < rule.ID {
				rule.ID = id
			}
		}
		grouped[name] = append(grouped[name], rule)
	}
	for name, rules := range grouped {
		state := org.Objects[name]
		state.Definition.DuplicateRules = storage.CloneDuplicateRules(rules)
		org.Objects[name] = state
	}
}
