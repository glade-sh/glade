package automation

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

type Index struct {
	Workflows   []Workflow              `json:"workflows,omitempty"`
	Flows       []Flow                  `json:"flows,omitempty"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

type Workflow struct {
	ObjectName string                 `json:"objectName"`
	Rules      []storage.WorkflowRule `json:"rules,omitempty"`
	File       string                 `json:"file,omitempty"`
}

type Flow struct {
	ObjectName string             `json:"objectName"`
	Rules      []storage.FlowRule `json:"rules,omitempty"`
	File       string             `json:"file,omitempty"`
}

type workflowXML struct {
	Rules        []workflowRuleXML        `xml:"rules"`
	FieldUpdates []workflowFieldUpdateXML `xml:"fieldUpdates"`
	Alerts       []workflowAlertXML       `xml:"alerts"`
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

type workflowAlertXML struct {
	FullName   string                 `xml:"fullName"`
	Name       string                 `xml:"name"`
	Template   string                 `xml:"template"`
	Recipients []workflowRecipientXML `xml:"recipients"`
}

type workflowRecipientXML struct {
	Type      string `xml:"type"`
	Field     string `xml:"field"`
	Recipient string `xml:"recipient"`
}

func LoadProject(p project.Project) (Index, error) {
	idx := Index{}
	workflowUpdates, err := loadProjectWorkflowFieldUpdates(p.WorkflowFiles)
	if err != nil {
		return Index{}, err
	}
	for _, path := range p.WorkflowFiles {
		workflow, diagnostics, err := loadWorkflowWithUpdates(path, workflowUpdates[objectNameFromWorkflowPath(path)])
		if err != nil {
			return Index{}, err
		}
		idx.Workflows = append(idx.Workflows, workflow)
		idx.Diagnostics = append(idx.Diagnostics, diagnostics...)
	}
	for _, path := range p.FlowFiles {
		flow, diagnostics, err := loadFlow(path)
		if err != nil {
			return Index{}, err
		}
		idx.Flows = append(idx.Flows, flow)
		idx.Diagnostics = append(idx.Diagnostics, diagnostics...)
	}
	sort.Slice(idx.Workflows, func(i, j int) bool { return idx.Workflows[i].ObjectName < idx.Workflows[j].ObjectName })
	sort.Slice(idx.Flows, func(i, j int) bool { return idx.Flows[i].ObjectName < idx.Flows[j].ObjectName })
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
	for _, flow := range idx.Flows {
		objectName, ok := storage.ResolveObjectName(*org, flow.ObjectName)
		if !ok {
			continue
		}
		object := org.Objects[objectName]
		object.Definition.FlowRules = append([]storage.FlowRule(nil), flow.Rules...)
		org.Objects[objectName] = object
	}
}

func loadWorkflow(path string) (Workflow, []diagnostic.Diagnostic, error) {
	return loadWorkflowWithUpdates(path, nil)
}

func loadWorkflowWithUpdates(path string, externalUpdates map[string]storage.WorkflowFieldUpdate) (Workflow, []diagnostic.Diagnostic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, nil, err
	}
	var raw workflowXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return Workflow{}, nil, err
	}
	updates := make(map[string]storage.WorkflowFieldUpdate, len(externalUpdates)+len(raw.FieldUpdates))
	for name, update := range externalUpdates {
		updates[name] = update
	}
	mergeWorkflowFieldUpdates(updates, raw.FieldUpdates)
	alerts := make(map[string]storage.WorkflowEmailAlert, len(raw.Alerts))
	for _, rawAlert := range raw.Alerts {
		alert := storage.WorkflowEmailAlert{
			Name:     firstNonBlank(rawAlert.FullName, rawAlert.Name),
			Template: strings.TrimSpace(rawAlert.Template),
		}
		for _, rawRecipient := range rawAlert.Recipients {
			recipient := storage.WorkflowEmailRecipient{
				Type:      strings.TrimSpace(rawRecipient.Type),
				Field:     trimObjectPrefix(strings.TrimSpace(rawRecipient.Field)),
				Recipient: strings.TrimSpace(rawRecipient.Recipient),
			}
			if recipient.Type != "" || recipient.Field != "" || recipient.Recipient != "" {
				alert.Recipients = append(alert.Recipients, recipient)
			}
		}
		if alert.Name == "" {
			continue
		}
		alerts[strings.ToLower(alert.Name)] = alert
	}
	workflow := Workflow{ObjectName: objectNameFromWorkflowPath(path), File: path}
	diagnostics := make([]diagnostic.Diagnostic, 0)
	for _, rawRule := range raw.Rules {
		rule := storage.WorkflowRule{
			Name:    firstNonBlank(rawRule.FullName, filepath.Base(path)),
			Active:  rawRule.Active,
			Formula: strings.TrimSpace(rawRule.Formula),
		}
		if rawRule.Active && !workflowBooleanFilterSupported(rawRule.BooleanFilter, len(rawRule.CriteriaItems)) {
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
			actionType := strings.TrimSpace(action.Type)
			if strings.EqualFold(actionType, "FieldUpdate") {
				update, ok := updates[strings.ToLower(strings.TrimSpace(action.Name))]
				if !ok {
					diagnostics = append(diagnostics, unsupported(path, rule.Name, fmt.Sprintf("workflow field update %q was not found", action.Name)))
					continue
				}
				rule.FieldUpdates = append(rule.FieldUpdates, update)
				continue
			}
			if strings.EqualFold(actionType, "Alert") || strings.EqualFold(actionType, "EmailAlert") {
				alert, ok := alerts[strings.ToLower(strings.TrimSpace(action.Name))]
				if !ok {
					diagnostics = append(diagnostics, unsupported(path, rule.Name, fmt.Sprintf("workflow email alert %q was not found", action.Name)))
					continue
				}
				rule.EmailAlerts = append(rule.EmailAlerts, alert)
				continue
			}
			{
				diagnostics = append(diagnostics, unsupported(path, rule.Name, fmt.Sprintf("workflow action type %q is not supported", action.Type)))
				continue
			}
		}
		if len(rule.FieldUpdates) > 0 || len(rule.EmailAlerts) > 0 {
			workflow.Rules = append(workflow.Rules, rule)
		}
	}
	sort.Slice(workflow.Rules, func(i, j int) bool { return workflow.Rules[i].Name < workflow.Rules[j].Name })
	return workflow, diagnostics, nil
}

func loadProjectWorkflowFieldUpdates(paths []string) (map[string]map[string]storage.WorkflowFieldUpdate, error) {
	out := make(map[string]map[string]storage.WorkflowFieldUpdate)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var raw workflowXML
		if err := xml.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
		objectName := objectNameFromWorkflowPath(path)
		if out[objectName] == nil {
			out[objectName] = make(map[string]storage.WorkflowFieldUpdate)
		}
		mergeWorkflowFieldUpdates(out[objectName], raw.FieldUpdates)
	}
	return out, nil
}

func mergeWorkflowFieldUpdates(updates map[string]storage.WorkflowFieldUpdate, rawUpdates []workflowFieldUpdateXML) {
	for _, rawUpdate := range rawUpdates {
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
}

func unsupported(path, rule, message string) diagnostic.Diagnostic {
	if rule != "" {
		message = rule + ": " + message
	}
	return diagnostic.Diagnostic{Severity: diagnostic.Warning, Code: "GLADEAUTO001", Message: message, File: path}
}

func objectNameFromWorkflowPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".workflow-meta.xml")
	base = strings.TrimSuffix(base, ".workflow")
	return base
}

func workflowBooleanFilterSupported(filter string, criteriaItems int) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	filter = strings.ReplaceAll(filter, "(", " ")
	filter = strings.ReplaceAll(filter, ")", " ")
	tokens := strings.Fields(filter)
	if len(tokens) == 0 {
		return true
	}
	expectCriterion := true
	for _, token := range tokens {
		if expectCriterion {
			index, ok := parseWorkflowBooleanCriterion(token)
			if !ok || index < 1 || index > criteriaItems {
				return false
			}
			expectCriterion = false
			continue
		}
		if !strings.EqualFold(token, "AND") {
			return false
		}
		expectCriterion = true
	}
	return !expectCriterion
}

func parseWorkflowBooleanCriterion(token string) (int, bool) {
	if token == "" {
		return 0, false
	}
	out := 0
	for _, r := range token {
		if r < '0' || r > '9' {
			return 0, false
		}
		out = out*10 + int(r-'0')
	}
	return out, true
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
