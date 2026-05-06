package automation

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/storage"
)

type flowXML struct {
	ProcessType   string                `xml:"processType"`
	Status        string                `xml:"status"`
	Start         flowStartXML          `xml:"start"`
	Formulas      []flowFormulaXML      `xml:"formulas"`
	Decisions     []flowDecisionXML     `xml:"decisions"`
	Assignments   []flowAssignmentXML   `xml:"assignments"`
	RecordUpdates []flowRecordUpdateXML `xml:"recordUpdates"`
	Screens       []flowNamedNodeXML    `xml:"screens"`
	Loops         []flowNamedNodeXML    `xml:"loops"`
	Subflows      []flowNamedNodeXML    `xml:"subflows"`
	ActionCalls   []flowActionCallXML   `xml:"actionCalls"`
	RecordLookups []flowNamedNodeXML    `xml:"recordLookups"`
	RecordCreates []flowNamedNodeXML    `xml:"recordCreates"`
	RecordDeletes []flowNamedNodeXML    `xml:"recordDeletes"`
	Waits         []flowNamedNodeXML    `xml:"waits"`
}

type flowStartXML struct {
	Object         string           `xml:"object"`
	TriggerType    string           `xml:"triggerType"`
	Filters        []flowFilterXML  `xml:"filters"`
	FilterLogic    string           `xml:"filterLogic"`
	Connector      flowConnectorXML `xml:"connector"`
	FaultConnector flowConnectorXML `xml:"faultConnector"`
}

type flowFilterXML struct {
	Field    string       `xml:"field"`
	Operator string       `xml:"operator"`
	Value    flowValueXML `xml:"value"`
}

type flowFormulaXML struct {
	Name       string `xml:"name"`
	DataType   string `xml:"dataType"`
	Expression string `xml:"expression"`
}

type flowDecisionXML struct {
	Name             string                `xml:"name"`
	Rules            []flowDecisionRuleXML `xml:"rules"`
	DefaultConnector flowConnectorXML      `xml:"defaultConnector"`
}

type flowDecisionRuleXML struct {
	Name           string             `xml:"name"`
	Conditions     []flowConditionXML `xml:"conditions"`
	ConditionLogic string             `xml:"conditionLogic"`
	Connector      flowConnectorXML   `xml:"connector"`
}

type flowConditionXML struct {
	LeftValueReference string       `xml:"leftValueReference"`
	Operator           string       `xml:"operator"`
	RightValue         flowValueXML `xml:"rightValue"`
}

type flowAssignmentXML struct {
	Name           string                  `xml:"name"`
	Label          string                  `xml:"label"`
	Items          []flowAssignmentItemXML `xml:"assignmentItems"`
	Connector      flowConnectorXML        `xml:"connector"`
	FaultConnector flowConnectorXML        `xml:"faultConnector"`
}

type flowAssignmentItemXML struct {
	AssignToReference string       `xml:"assignToReference"`
	Operator          string       `xml:"operator"`
	Value             flowValueXML `xml:"value"`
}

type flowRecordUpdateXML struct {
	Name           string                   `xml:"name"`
	Label          string                   `xml:"label"`
	InputReference string                   `xml:"inputReference"`
	Filters        []flowFilterXML          `xml:"filters"`
	FilterLogic    string                   `xml:"filterLogic"`
	Fields         []flowFieldAssignmentXML `xml:"inputAssignments"`
	Connector      flowConnectorXML         `xml:"connector"`
	FaultConnector flowConnectorXML         `xml:"faultConnector"`
}

type flowFieldAssignmentXML struct {
	Field string       `xml:"field"`
	Value flowValueXML `xml:"value"`
}

type flowNamedNodeXML struct {
	Name string `xml:"name"`
}

type flowActionCallXML struct {
	Name       string `xml:"name"`
	Label      string `xml:"label"`
	ActionType string `xml:"actionType"`
	ActionName string `xml:"actionName"`
}

type flowConnectorXML struct {
	TargetReference string `xml:"targetReference"`
}

type flowValueXML struct {
	StringValue      string `xml:"stringValue"`
	BooleanValue     string `xml:"booleanValue"`
	NumberValue      string `xml:"numberValue"`
	DateValue        string `xml:"dateValue"`
	DateTimeValue    string `xml:"dateTimeValue"`
	ElementReference string `xml:"elementReference"`
	Formula          string `xml:"formula"`
	Expression       string `xml:"expression"`
}

func loadFlow(path string) (Flow, []diagnostic.Diagnostic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Flow{}, nil, err
	}
	var raw flowXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return Flow{}, nil, err
	}
	name := strings.TrimSuffix(filepath.Base(path), ".flow-meta.xml")
	name = strings.TrimSuffix(name, ".flow")
	objectName := strings.TrimSpace(raw.Start.Object)
	flow := Flow{ObjectName: objectName, File: path}
	diagnostics := make([]diagnostic.Diagnostic, 0)
	if !flowActive(raw.Status) || objectName == "" {
		return flow, diagnostics, nil
	}
	diagnostics = flowUnsupportedNodeDiagnostics(path, name, raw)
	if !flowProcessTypeSupported(raw.ProcessType) {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("processType %q is not modeled as DML-triggered automation", raw.ProcessType)))
	}
	rule := storage.FlowRule{
		Name:        name,
		File:        path,
		Active:      true,
		ProcessType: strings.TrimSpace(raw.ProcessType),
		TriggerType: strings.TrimSpace(raw.Start.TriggerType),
	}
	formulas := flowFormulaMap(raw.Formulas)
	for _, filter := range raw.Start.Filters {
		rule.Criteria = append(rule.Criteria, storage.WorkflowCriteriaItem{
			Field:     trimObjectPrefix(strings.TrimSpace(filter.Field)),
			Operation: flowOperator(strings.TrimSpace(filter.Operator)),
			Value:     flowLiteralValue(filter.Value),
		})
	}
	if formula := flowCriteriaFormula(raw.Start.FilterLogic, formulas); formula != "" {
		rule.Formula = formula
		rule.Criteria = nil
	}
	for _, decision := range raw.Decisions {
		if len(decision.Rules) == 0 {
			continue
		}
		if target := strings.TrimSpace(decision.DefaultConnector.TargetReference); target != "" {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("decision %q default connector to %q is not modeled", decision.Name, target)))
		}
		if logic := strings.TrimSpace(decision.Rules[0].ConditionLogic); logic != "" && !strings.EqualFold(logic, "and") {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("decision %q condition logic %q is not supported", decision.Name, logic)))
			continue
		}
		for _, condition := range decision.Rules[0].Conditions {
			field := flowRecordFieldReference(condition.LeftValueReference)
			if field == "" {
				diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("decision %q condition %q is not a $Record field", decision.Name, condition.LeftValueReference)))
				continue
			}
			rule.Criteria = append(rule.Criteria, storage.WorkflowCriteriaItem{
				Field:     field,
				Operation: flowOperator(strings.TrimSpace(condition.Operator)),
				Value:     flowLiteralValue(condition.RightValue),
			})
		}
		if len(decision.Rules) > 1 {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("decision %q has additional branches beyond the first supported rule", decision.Name)))
		}
	}
	for _, assignment := range raw.Assignments {
		for _, item := range assignment.Items {
			if !flowAssignmentOperatorSupported(item.Operator) {
				diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("assignment %q operator %q is not supported", firstNonBlank(assignment.Name, assignment.Label), item.Operator)))
				continue
			}
			field := flowRecordFieldReference(item.AssignToReference)
			if field == "" {
				diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("assignment %q target %q is not a $Record field", firstNonBlank(assignment.Name, assignment.Label), item.AssignToReference)))
				continue
			}
			rule.FieldUpdates = append(rule.FieldUpdates, storage.WorkflowFieldUpdate{
				Name:         firstNonBlank(assignment.Name, assignment.Label, field),
				Field:        field,
				LiteralValue: flowLiteralValue(item.Value),
				Formula:      flowExpressionValue(item.Value, formulas),
				SourceField:  flowSourceFieldValue(item.Value),
			})
		}
	}
	for _, update := range raw.RecordUpdates {
		if !flowUpdatesTriggeringRecord(update.InputReference) {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("record update %q does not target the triggering record", firstNonBlank(update.Name, update.Label))))
			continue
		}
		if logic := strings.TrimSpace(update.FilterLogic); logic != "" && !strings.EqualFold(logic, "and") {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("record update %q filter logic %q is not supported", firstNonBlank(update.Name, update.Label), logic)))
			continue
		}
		for _, assignment := range update.Fields {
			field := trimObjectPrefix(strings.TrimSpace(assignment.Field))
			if field == "" {
				continue
			}
			rule.FieldUpdates = append(rule.FieldUpdates, storage.WorkflowFieldUpdate{
				Name:         firstNonBlank(update.Name, update.Label, field),
				Field:        field,
				LiteralValue: flowLiteralValue(assignment.Value),
				Formula:      flowExpressionValue(assignment.Value, formulas),
				SourceField:  flowSourceFieldValue(assignment.Value),
			})
		}
	}
	for _, action := range raw.ActionCalls {
		flowAction, ok := modeledFlowActionCall(action)
		if !ok {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("action node %q is not a modeled Apex action", firstNonBlank(action.Name, action.Label, action.ActionName))))
			continue
		}
		rule.Actions = append(rule.Actions, flowAction)
	}
	if len(rule.FieldUpdates) > 0 || len(rule.Actions) > 0 {
		flow.Rules = append(flow.Rules, rule)
	}
	sort.Slice(flow.Rules, func(i, j int) bool { return flow.Rules[i].Name < flow.Rules[j].Name })
	return flow, diagnostics, nil
}

func flowRecordFieldReference(reference string) string {
	reference = strings.TrimSpace(reference)
	for _, prefix := range []string{"$Record.", "Record."} {
		if strings.HasPrefix(reference, prefix) && len(reference) > len(prefix) {
			return trimObjectPrefix(reference[len(prefix):])
		}
	}
	if !strings.Contains(reference, ".") {
		return trimObjectPrefix(reference)
	}
	return ""
}

func flowAssignmentOperatorSupported(operator string) bool {
	operator = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(operator), " ", ""))
	return operator == "" || operator == "assign" || operator == "equalto"
}

func flowFormulaMap(formulas []flowFormulaXML) map[string]string {
	out := make(map[string]string, len(formulas))
	for _, formula := range formulas {
		name := strings.TrimSpace(formula.Name)
		expression := normalizeFlowFormula(strings.TrimSpace(formula.Expression))
		if name != "" && expression != "" {
			out[strings.ToLower(name)] = expression
		}
	}
	return out
}

func flowCriteriaFormula(filterLogic string, formulas map[string]string) string {
	filterLogic = strings.TrimSpace(filterLogic)
	if filterLogic == "" {
		return ""
	}
	if expression, ok := formulas[strings.ToLower(filterLogic)]; ok {
		return expression
	}
	normalized := normalizeFlowFormula(filterLogic)
	if strings.ContainsAny(normalized, "=!<>&|()") {
		return normalized
	}
	return ""
}

func flowLiteralValue(value flowValueXML) string {
	switch {
	case strings.TrimSpace(value.StringValue) != "":
		return strings.TrimSpace(value.StringValue)
	case strings.TrimSpace(value.BooleanValue) != "":
		return strings.TrimSpace(value.BooleanValue)
	case strings.TrimSpace(value.NumberValue) != "":
		return strings.TrimSpace(value.NumberValue)
	case strings.TrimSpace(value.DateValue) != "":
		return strings.TrimSpace(value.DateValue)
	case strings.TrimSpace(value.DateTimeValue) != "":
		return strings.TrimSpace(value.DateTimeValue)
	default:
		return ""
	}
}

func flowExpressionValue(value flowValueXML, formulas map[string]string) string {
	for _, candidate := range []string{value.Formula, value.Expression} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if expression, ok := formulas[strings.ToLower(candidate)]; ok {
			return expression
		}
		return normalizeFlowFormula(candidate)
	}
	reference := strings.TrimSpace(value.ElementReference)
	if expression, ok := formulas[strings.ToLower(reference)]; ok {
		return expression
	}
	return ""
}

func flowSourceFieldValue(value flowValueXML) string {
	reference := strings.TrimSpace(value.ElementReference)
	if reference == "" {
		return ""
	}
	return flowRecordFieldReference(reference)
}

func normalizeFlowFormula(expression string) string {
	expression = strings.TrimSpace(expression)
	expression = strings.ReplaceAll(expression, "{!$Record.", "")
	expression = strings.ReplaceAll(expression, "{!Record.", "")
	expression = strings.ReplaceAll(expression, "{!", "")
	expression = strings.ReplaceAll(expression, "$Record.", "")
	expression = strings.ReplaceAll(expression, "Record.", "")
	expression = strings.ReplaceAll(expression, "}", "")
	return strings.TrimSpace(expression)
}

func flowActive(status string) bool {
	status = strings.TrimSpace(status)
	return status == "" || strings.EqualFold(status, "Active")
}

func flowProcessTypeSupported(processType string) bool {
	switch strings.ToLower(strings.TrimSpace(processType)) {
	case "", "autolaunchedflow", "workflow", "customevent", "invocableprocess":
		return true
	default:
		return false
	}
}

func flowUpdatesTriggeringRecord(input string) bool {
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "" || input == "$record" || input == "record" || input == "$record__prior"
}

func modeledFlowActionCall(action flowActionCallXML) (storage.FlowAction, bool) {
	actionType := strings.TrimSpace(action.ActionType)
	if actionType != "" && !strings.EqualFold(actionType, "apex") {
		return storage.FlowAction{}, false
	}
	actionName := strings.TrimSpace(action.ActionName)
	if actionName == "" {
		return storage.FlowAction{}, false
	}
	className := actionName
	methodName := ""
	if dot := strings.LastIndex(actionName, "."); dot > 0 && dot < len(actionName)-1 {
		className = actionName[:dot]
		methodName = actionName[dot+1:]
	}
	return storage.FlowAction{
		Name:       firstNonBlank(action.Name, action.Label, actionName),
		ActionType: actionType,
		ActionName: actionName,
		ClassName:  className,
		MethodName: methodName,
	}, true
}

func flowOperator(operator string) string {
	switch strings.ToLower(strings.ReplaceAll(operator, " ", "")) {
	case "", "equalto", "equals":
		return "equals"
	case "notequalto", "notequals":
		return "notEqual"
	case "isnull":
		return "isNull"
	default:
		return operator
	}
}

func flowUnsupportedNodeDiagnostics(path, flowName string, raw flowXML) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if target := strings.TrimSpace(raw.Start.FaultConnector.TargetReference); target != "" {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("start fault connector to %q is not supported", target)))
	}
	for _, screen := range raw.Screens {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("screen node %q is not supported", screen.Name)))
	}
	for _, loop := range raw.Loops {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("loop node %q is not supported", loop.Name)))
	}
	for _, subflow := range raw.Subflows {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("subflow node %q is not supported", subflow.Name)))
	}
	for _, lookup := range raw.RecordLookups {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("record lookup node %q is not supported", lookup.Name)))
	}
	for _, create := range raw.RecordCreates {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("record create node %q is not supported", create.Name)))
	}
	for _, delete := range raw.RecordDeletes {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("record delete node %q is not supported", delete.Name)))
	}
	for _, wait := range raw.Waits {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("wait node %q is not supported", wait.Name)))
	}
	for _, assignment := range raw.Assignments {
		if target := strings.TrimSpace(assignment.FaultConnector.TargetReference); target != "" {
			diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("assignment %q fault connector to %q is not supported", firstNonBlank(assignment.Name, assignment.Label), target)))
		}
	}
	for _, update := range raw.RecordUpdates {
		if target := strings.TrimSpace(update.FaultConnector.TargetReference); target != "" {
			diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("record update %q fault connector to %q is not supported", firstNonBlank(update.Name, update.Label), target)))
		}
	}
	return diagnostics
}

func flowUnsupported(path, flowName, message string) diagnostic.Diagnostic {
	if flowName != "" {
		message = flowName + ": " + message
	}
	return diagnostic.Diagnostic{Severity: diagnostic.Warning, Code: "OAERAUTO002", Message: message, File: path}
}
