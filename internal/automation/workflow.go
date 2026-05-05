package automation

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/storage"
)

type Index struct {
	Workflows   []Workflow              `json:"workflows,omitempty"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

type Workflow struct {
	ObjectName string                 `json:"objectName"`
	Rules      []storage.WorkflowRule `json:"rules,omitempty"`
	File       string                 `json:"file,omitempty"`
}

type workflowXML struct {
	Rules        []workflowRuleXML        `xml:"rules"`
	FieldUpdates []workflowFieldUpdateXML `xml:"fieldUpdates"`
}

type workflowRuleXML struct {
	FullName      string                    `xml:"fullName"`
	Active        bool                      `xml:"active"`
	Formula       string                    `xml:"formula"`
	BooleanFilter string                    `xml:"booleanFilter"`
	CriteriaItems []workflowCriteriaItemXML `xml:"criteriaItems"`
	Actions       []workflowActionXML       `xml:"actions"`
}

type workflowCriteriaItemXML struct {
	Field     string `xml:"field"`
	Operation string `xml:"operation"`
	Value     string `xml:"value"`
}

type workflowActionXML struct {
	Name string `xml:"name"`
	Type string `xml:"type"`
}

type workflowFieldUpdateXML struct {
	FullName     string `xml:"fullName"`
	Name         string `xml:"name"`
	Field        string `xml:"field"`
	LiteralValue string `xml:"literalValue"`
	Formula      string `xml:"formula"`
	SourceField  string `xml:"sourceField"`
}

func LoadProject(p project.Project) (Index, error) {
	idx := Index{}
	for _, path := range p.WorkflowFiles {
		workflow, diagnostics, err := loadWorkflow(path)
		if err != nil {
			return Index{}, err
		}
		idx.Workflows = append(idx.Workflows, workflow)
		idx.Diagnostics = append(idx.Diagnostics, diagnostics...)
	}
	sort.Slice(idx.Workflows, func(i, j int) bool { return idx.Workflows[i].ObjectName < idx.Workflows[j].ObjectName })
	return idx, nil
}

func ApplyToOrg(org *storage.OrgState, idx Index) {
	if org == nil {
		return
	}
	for _, workflow := range idx.Workflows {
		objectName, ok := storage.ResolveObjectName(*org, workflow.ObjectName)
		if !ok {
			continue
		}
		object := org.Objects[objectName]
		object.Definition.WorkflowRules = append([]storage.WorkflowRule(nil), workflow.Rules...)
		org.Objects[objectName] = object
	}
}

func loadWorkflow(path string) (Workflow, []diagnostic.Diagnostic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, nil, err
	}
	var raw workflowXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return Workflow{}, nil, err
	}
	updates := make(map[string]storage.WorkflowFieldUpdate, len(raw.FieldUpdates))
	for _, rawUpdate := range raw.FieldUpdates {
		update := storage.WorkflowFieldUpdate{
			Name:         firstNonBlank(rawUpdate.FullName, rawUpdate.Name),
			Field:        trimObjectPrefix(strings.TrimSpace(rawUpdate.Field)),
			LiteralValue: strings.TrimSpace(rawUpdate.LiteralValue),
			Formula:      strings.TrimSpace(rawUpdate.Formula),
			SourceField:  trimObjectPrefix(strings.TrimSpace(rawUpdate.SourceField)),
		}
		if update.Name == "" || update.Field == "" {
			continue
		}
		updates[strings.ToLower(update.Name)] = update
	}
	workflow := Workflow{ObjectName: objectNameFromWorkflowPath(path), File: path}
	diagnostics := make([]diagnostic.Diagnostic, 0)
	for _, rawRule := range raw.Rules {
		rule := storage.WorkflowRule{
			Name:    firstNonBlank(rawRule.FullName, filepath.Base(path)),
			Active:  rawRule.Active,
			Formula: strings.TrimSpace(rawRule.Formula),
		}
		if strings.TrimSpace(rawRule.BooleanFilter) != "" {
			diagnostics = append(diagnostics, unsupported(path, rule.Name, "workflow booleanFilter is not supported; criteriaItems use implicit AND"))
			continue
		}
		for _, rawCriteria := range rawRule.CriteriaItems {
			rule.Criteria = append(rule.Criteria, storage.WorkflowCriteriaItem{
				Field:     trimObjectPrefix(strings.TrimSpace(rawCriteria.Field)),
				Operation: strings.TrimSpace(rawCriteria.Operation),
				Value:     strings.TrimSpace(rawCriteria.Value),
			})
		}
		for _, action := range rawRule.Actions {
			if !strings.EqualFold(strings.TrimSpace(action.Type), "FieldUpdate") {
				diagnostics = append(diagnostics, unsupported(path, rule.Name, fmt.Sprintf("workflow action type %q is not supported", action.Type)))
				continue
			}
			update, ok := updates[strings.ToLower(strings.TrimSpace(action.Name))]
			if !ok {
				diagnostics = append(diagnostics, unsupported(path, rule.Name, fmt.Sprintf("workflow field update %q was not found", action.Name)))
				continue
			}
			rule.FieldUpdates = append(rule.FieldUpdates, update)
		}
		if len(rule.FieldUpdates) > 0 {
			workflow.Rules = append(workflow.Rules, rule)
		}
	}
	sort.Slice(workflow.Rules, func(i, j int) bool { return workflow.Rules[i].Name < workflow.Rules[j].Name })
	return workflow, diagnostics, nil
}

func unsupported(path, rule, message string) diagnostic.Diagnostic {
	if rule != "" {
		message = rule + ": " + message
	}
	return diagnostic.Diagnostic{Severity: diagnostic.Warning, Code: "OAERAUTO001", Message: message, File: path}
}

func objectNameFromWorkflowPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".workflow-meta.xml")
}

func trimObjectPrefix(field string) string {
	if dot := strings.LastIndex(field, "."); dot >= 0 && dot < len(field)-1 {
		return field[dot+1:]
	}
	return field
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
