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

func loadAutoResponseRules(p project.Project, read func(string) ([]byte, error)) ([]storage.AutoResponseRule, []diagnostic.Diagnostic) {
	var rules []storage.AutoResponseRule
	var diagnostics []diagnostic.Diagnostic
	seen := map[*project.Project]bool{}
	var visit func(*project.Project)
	visit = func(cur *project.Project) {
		if cur == nil || seen[cur] {
			return
		}
		seen[cur] = true
		paths := append([]string(nil), cur.AutoResponseRuleFiles...)
		sort.Strings(paths)
		for _, path := range paths {
			object := ruleMetadataName(path, "autoResponseRules")
			var doc struct {
				XMLName xml.Name `xml:"AutoResponseRules"`
				Rules   []struct {
					Name    string `xml:"fullName"`
					Active  bool   `xml:"active"`
					Entries []struct {
						Formula       string                    `xml:"formula"`
						BooleanFilter string                    `xml:"booleanFilter"`
						Criteria      []workflowCriteriaItemXML `xml:"criteriaItems"`
						Template      string                    `xml:"template"`
					} `xml:"ruleEntry"`
				} `xml:"autoResponseRule"`
			}
			body, err := read(path)
			if err == nil {
				err = xml.Unmarshal(body, &doc)
			}
			if err != nil {
				diagnostics = append(diagnostics, loadFailure(path, "auto-response rule", object, err))
				continue
			}
			for _, raw := range doc.Rules {
				rule := storage.AutoResponseRule{Object: object, Name: strings.TrimSpace(raw.Name), Active: raw.Active}
				if rule.Name == "" {
					diagnostics = append(diagnostics, loadFailure(path, "auto-response rule", object, fmt.Errorf("missing fullName")))
					continue
				}
				for _, entry := range raw.Entries {
					next := storage.AutoResponseRuleEntry{Formula: strings.TrimSpace(entry.Formula), BooleanFilter: strings.TrimSpace(entry.BooleanFilter), Template: strings.TrimSpace(entry.Template)}
					for _, c := range entry.Criteria {
						next.Criteria = append(next.Criteria, storage.WorkflowCriteriaItem{Field: trimObjectPrefix(strings.TrimSpace(c.Field)), Operation: strings.TrimSpace(c.Operation), Value: c.Value})
					}
					for _, templatePath := range cur.EmailTemplateFiles {
						if !strings.HasSuffix(strings.ToLower(templatePath), ".email-meta.xml") {
							continue
						}
						name := strings.TrimSuffix(filepath.Base(templatePath), ".email-meta.xml")
						key := filepath.Base(filepath.Dir(templatePath)) + "/" + name
						if !strings.EqualFold(next.Template, key) {
							continue
						}
						data, e := read(templatePath)
						var template struct {
							Subject string `xml:"subject"`
						}
						if e == nil {
							e = xml.Unmarshal(data, &template)
						}
						if e != nil {
							diagnostics = append(diagnostics, loadFailure(templatePath, "email template", name, e))
							continue
						}
						next.Subject = template.Subject
					}
					rule.Entries = append(rule.Entries, next)
				}
				rules = append(rules, rule)
			}
		}
		for _, dep := range cur.ManagedPackageDependencies {
			if dep.Status == "loaded" {
				visit(dep.Project)
			}
		}
		for _, shim := range cur.PackageShims {
			if shim.Status == "loaded" {
				visit(shim.Project)
			}
		}
	}
	visit(&p)
	return rules, diagnostics
}
func applyAutoResponseRules(org *storage.OrgState, rules []storage.AutoResponseRule) {
	grouped := map[string][]storage.AutoResponseRule{}
	for _, rule := range rules {
		if name, ok := storage.ResolveKnownStandardObjectName(rule.Object); ok {
			storage.EnsureStandardObject(org, name)
		}
		name, ok := storage.ResolveObjectName(*org, rule.Object)
		if ok {
			grouped[name] = append(grouped[name], rule)
		}
	}
	for name, items := range grouped {
		state := org.Objects[name]
		state.Definition.AutoResponseRules = storage.CloneAutoResponseRules(items)
		org.Objects[name] = state
	}
}
