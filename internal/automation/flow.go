package automation

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/storage"
)

type flowXML struct {
	ProcessType          string                       `xml:"processType"`
	Status               string                       `xml:"status"`
	TriggerOrder         int                          `xml:"triggerOrder"`
	RunInMode            string                       `xml:"runInMode"`
	Start                flowStartXML                 `xml:"start"`
	Formulas             []flowFormulaXML             `xml:"formulas"`
	Constants            []flowConstantXML            `xml:"constants"`
	TextTemplates        []flowTextTemplateXML        `xml:"textTemplates"`
	Decisions            []flowDecisionXML            `xml:"decisions"`
	Assignments          []flowAssignmentXML          `xml:"assignments"`
	RecordUpdates        []flowRecordUpdateXML        `xml:"recordUpdates"`
	Screens              []flowNamedNodeXML           `xml:"screens"`
	Loops                []flowLoopXML                `xml:"loops"`
	Subflows             []flowSubflowXML             `xml:"subflows"`
	ActionCalls          []flowActionCallXML          `xml:"actionCalls"`
	RecordLookups        []flowRecordLookupXML        `xml:"recordLookups"`
	RecordCreates        []flowRecordCreateXML        `xml:"recordCreates"`
	RecordDeletes        []flowRecordDeleteXML        `xml:"recordDeletes"`
	CustomErrors         []flowCustomErrorXML         `xml:"customErrors"`
	ApexPluginCalls      []flowApexPluginCallXML      `xml:"apexPluginCalls"`
	Transforms           []flowTransformXML           `xml:"transforms"`
	CollectionProcessors []flowCollectionProcessorXML `xml:"collectionProcessors"`
	OrchestratedStages   []flowNamedNodeXML           `xml:"orchestratedStages"`
	ScheduledPaths       []flowNamedNodeXML           `xml:"scheduledPaths"`
	Stages               []flowNamedNodeXML           `xml:"stages"`
	Environments         string                       `xml:"environments"`
	ExitRules            []flowNamedNodeXML           `xml:"exitRules"`
	Experiments          []flowNamedNodeXML           `xml:"experiments"`
	Segment              []flowNamedNodeXML           `xml:"segment"`
	Schedule             []flowNamedNodeXML           `xml:"schedule"`
	Waits                []flowNamedNodeXML           `xml:"waits"`
	Variables            []flowVariableXML            `xml:"variables"`
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
	Object         string                   `xml:"object"`
	InputReference string                   `xml:"inputReference"`
	Filters        []flowFilterXML          `xml:"filters"`
	FilterLogic    string                   `xml:"filterLogic"`
	Fields         []flowFieldAssignmentXML `xml:"inputAssignments"`
	Connector      flowConnectorXML         `xml:"connector"`
	FaultConnector flowConnectorXML         `xml:"faultConnector"`
}

type flowRecordLookupXML struct {
	Name                     string           `xml:"name"`
	Label                    string           `xml:"label"`
	Object                   string           `xml:"object"`
	Filters                  []flowFilterXML  `xml:"filters"`
	FilterLogic              string           `xml:"filterLogic"`
	Connector                flowConnectorXML `xml:"connector"`
	FaultConnector           flowConnectorXML `xml:"faultConnector"`
	GetFirstRecordOnly       bool             `xml:"getFirstRecordOnly"`
	StoreOutputAutomatically bool             `xml:"storeOutputAutomatically"`
}

type flowRecordCreateXML struct {
	Name                     string                   `xml:"name"`
	Label                    string                   `xml:"label"`
	Object                   string                   `xml:"object"`
	InputReference           string                   `xml:"inputReference"`
	Fields                   []flowFieldAssignmentXML `xml:"inputAssignments"`
	Connector                flowConnectorXML         `xml:"connector"`
	FaultConnector           flowConnectorXML         `xml:"faultConnector"`
	StoreOutputAutomatically bool                     `xml:"storeOutputAutomatically"`
}

type flowRecordDeleteXML struct {
	Name           string           `xml:"name"`
	Label          string           `xml:"label"`
	Object         string           `xml:"object"`
	InputReference string           `xml:"inputReference"`
	Filters        []flowFilterXML  `xml:"filters"`
	FilterLogic    string           `xml:"filterLogic"`
	Connector      flowConnectorXML `xml:"connector"`
	FaultConnector flowConnectorXML `xml:"faultConnector"`
}

type flowSubflowXML struct {
	Name              string                   `xml:"name"`
	Label             string                   `xml:"label"`
	FlowName          string                   `xml:"flowName"`
	InputAssignments  []flowFieldAssignmentXML `xml:"inputAssignments"`
	OutputAssignments []flowAssignmentItemXML  `xml:"outputAssignments"`
	Connector         flowConnectorXML         `xml:"connector"`
	FaultConnector    flowConnectorXML         `xml:"faultConnector"`
}

type flowFieldAssignmentXML struct {
	Field string       `xml:"field"`
	Value flowValueXML `xml:"value"`
}

type flowNamedNodeXML struct {
	Name string `xml:"name"`
}

type flowLoopXML struct {
	Name                       string           `xml:"name"`
	CollectionReference        string           `xml:"collectionReference"`
	NextValueConnector         flowConnectorXML `xml:"nextValueConnector"`
	NoMoreValuesConnector      flowConnectorXML `xml:"noMoreValuesConnector"`
	FaultConnector             flowConnectorXML `xml:"faultConnector"`
	AssignNextValueToReference string           `xml:"assignNextValueToReference"`
}

type flowActionCallXML struct {
	Name            string               `xml:"name"`
	Label           string               `xml:"label"`
	ActionType      string               `xml:"actionType"`
	ActionName      string               `xml:"actionName"`
	Connector       flowConnectorXML     `xml:"connector"`
	FaultConnector  flowConnectorXML     `xml:"faultConnector"`
	InputParameters []flowActionInputXML `xml:"inputParameters"`
}

type flowActionInputXML struct {
	Name  string       `xml:"name"`
	Value flowValueXML `xml:"value"`
}

type flowVariableXML struct {
	Name         string       `xml:"name"`
	DataType     string       `xml:"dataType"`
	ObjectType   string       `xml:"objectType"`
	IsCollection bool         `xml:"isCollection"`
	Value        flowValueXML `xml:"value"`
}

type flowConstantXML struct {
	Name     string       `xml:"name"`
	DataType string       `xml:"dataType"`
	Value    flowValueXML `xml:"value"`
}

type flowTextTemplateXML struct {
	Name            string `xml:"name"`
	Text            string `xml:"text"`
	IsViewedAsPlain bool   `xml:"isViewedAsPlain"`
}

type flowCustomErrorXML struct {
	Name        string                      `xml:"name"`
	Description string                      `xml:"description"`
	Messages    []flowCustomErrorMessageXML `xml:"customErrorMessages"`
}

type flowCustomErrorMessageXML struct {
	Message string `xml:"message"`
}

type flowApexPluginCallXML struct {
	Name             string               `xml:"name"`
	Label            string               `xml:"label"`
	ApexClass        string               `xml:"apexClass"`
	InputParameters  []flowActionInputXML `xml:"inputParameters"`
	OutputParameters []flowActionInputXML `xml:"outputParameters"`
	Connector        flowConnectorXML     `xml:"connector"`
	FaultConnector   flowConnectorXML     `xml:"faultConnector"`
}

type flowTransformXML struct {
	Name             string                   `xml:"name"`
	Label            string                   `xml:"label"`
	TransformType    string                   `xml:"transformType"`
	SourceCollection string                   `xml:"sourceCollection"`
	TargetCollection string                   `xml:"targetCollection"`
	FieldMappings    []flowFieldAssignmentXML `xml:"fieldMappings"`
	SumField         string                   `xml:"sumField"`
	Connector        flowConnectorXML         `xml:"connector"`
	FaultConnector   flowConnectorXML         `xml:"faultConnector"`
}

type flowCollectionProcessorXML struct {
	Name                string                   `xml:"name"`
	Label               string                   `xml:"label"`
	ProcessorType       string                   `xml:"processorType"`
	CollectionReference string                   `xml:"collectionReference"`
	TargetCollection    string                   `xml:"targetCollection"`
	SortField           string                   `xml:"sortField"`
	SortOrder           string                   `xml:"sortOrder"`
	Filters             []flowFilterXML          `xml:"filters"`
	FieldMappings       []flowFieldAssignmentXML `xml:"fieldMappings"`
	Connector           flowConnectorXML         `xml:"connector"`
	FaultConnector      flowConnectorXML         `xml:"faultConnector"`
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
	if flowProcessTypeNonDML(raw.ProcessType) {
		return flow, diagnostics, nil
	}
	formulas := flowFormulaMap(raw.Formulas)
	variables := flowVariableMap(raw.Variables)
	constants := flowConstantMap(raw.Constants)
	diagnostics = flowUnsupportedNodeDiagnostics(path, name, raw, formulas, variables)
	if !flowProcessTypeSupported(raw.ProcessType) {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("processType %q is not modeled as DML-triggered automation", raw.ProcessType)))
	}
	triggerType := strings.TrimSpace(raw.Start.TriggerType)
	if triggerType != "" && !flowTriggerTypeDML(triggerType) {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("triggerType %q is not supported for local test execution", triggerType)))
	}
	if len(raw.OrchestratedStages) > 0 {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("orchestration (%d stages) is not supported for local test execution", len(raw.OrchestratedStages))))
	}
	if len(raw.ScheduledPaths) > 0 {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("scheduled paths are not supported for local test execution")))
	}
	if len(raw.Stages) > 0 {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("stages are not supported for local test execution")))
	}
	if raw.Environments != "" && !strings.EqualFold(strings.TrimSpace(raw.Environments), "Default") {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("environments %q is not supported for local test execution", raw.Environments)))
	}
	rule := storage.FlowRule{
		Name:         name,
		File:         path,
		Active:       true,
		ProcessType:  strings.TrimSpace(raw.ProcessType),
		TriggerType:  strings.TrimSpace(raw.Start.TriggerType),
		TriggerOrder: raw.TriggerOrder,
		RunInMode:    strings.TrimSpace(raw.RunInMode),
	}
	routedDecisions := false
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
		isRoutedDecision := flowDecisionHasRoutedBranch(decision)
		if isRoutedDecision {
			routedDecisions = true
		}
		if target := strings.TrimSpace(decision.DefaultConnector.TargetReference); target != "" {
			if !flowNodeReferenceModeled(target, raw) {
				diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("decision %q default connector to %q is not modeled", decision.Name, target)))
			}
		}
		if !isRoutedDecision {
			groups, ok := flowConditionLogicGroups(decision.Rules[0].ConditionLogic, len(decision.Rules[0].Conditions))
			if !ok {
				diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("decision %q condition logic %q is not supported", decision.Name, decision.Rules[0].ConditionLogic)))
				continue
			}
			if flowConditionGroupsNeedFormula(groups, decision.Rules[0].Conditions, formulas) {
				formula := flowConditionGroupsToFormula(groups, decision.Rules[0].Conditions, formulas, variables)
				if formula == "" {
					diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("decision %q condition logic %q is not supported", decision.Name, decision.Rules[0].ConditionLogic)))
					continue
				}
				rule.Formula = formula
				rule.Criteria = nil
				continue
			}
			for _, condition := range decision.Rules[0].Conditions {
				field := flowRecordFieldReference(condition.LeftValueReference)
				if field == "" {
					if f := flowVariableRecordField(condition.LeftValueReference, variables); f != "" {
						field = f
					}
				}
				if field == "" {
					if !flowReferenceModeled(condition.LeftValueReference, variables, constants, raw.RecordLookups, formulas) {
						diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("decision %q condition %q is not a $Record field or modeled flow resource", decision.Name, condition.LeftValueReference)))
						continue
					}
				}
				if field != "" {
					rule.Criteria = append(rule.Criteria, storage.WorkflowCriteriaItem{
						Field:     field,
						Operation: flowOperator(strings.TrimSpace(condition.Operator)),
						Value:     flowLiteralValue(condition.RightValue),
					})
				}
			}
		}
		if len(decision.Rules) > 1 {
			if !flowDecisionHasRoutedBranch(decision) {
				diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("decision %q has additional branches beyond the first supported rule", decision.Name)))
			}
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
				continue
			}
			rule.FieldUpdates = append(rule.FieldUpdates, storage.WorkflowFieldUpdate{
				Name:         firstNonBlank(assignment.Name, assignment.Label, field),
				Field:        field,
				LiteralValue: flowLiteralValue(item.Value),
				Formula:      flowExpressionValue(item.Value, formulas),
				SourceField:  flowSourceFieldValue(item.Value, variables),
			})
		}
	}
	for _, update := range raw.RecordUpdates {
		if strings.TrimSpace(update.Object) != "" {
			continue
		}
		if !flowUpdatesTriggeringRecord(update.InputReference) {
			if _, ok := modeledFlowRecordUpdate(update, formulas, variables, raw.RecordLookups); !ok && !flowRequiresOrderedGraph(raw) {
				diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("record update %q does not target the triggering record", firstNonBlank(update.Name, update.Label))))
			}
			continue
		}
		if logic := strings.TrimSpace(update.FilterLogic); logic != "" && !strings.EqualFold(logic, "and") {
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
				SourceField:  flowSourceFieldValue(assignment.Value, variables),
			})
		}
	}
	for _, lookup := range raw.RecordLookups {
		recordLookup, ok := modeledFlowRecordLookup(lookup, variables, raw.RecordLookups)
		if !ok {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("record lookup node %q is not modeled", firstNonBlank(lookup.Name, lookup.Label))))
			continue
		}
		rule.RecordLookups = append(rule.RecordLookups, recordLookup)
	}
	for _, create := range raw.RecordCreates {
		recordCreate, ok := modeledFlowRecordCreate(create, formulas, variables, raw.RecordLookups)
		if !ok {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("record create node %q is not modeled", firstNonBlank(create.Name, create.Label))))
			continue
		}
		rule.RecordCreates = append(rule.RecordCreates, recordCreate)
	}
	for _, action := range raw.ActionCalls {
		flowAction, ok := modeledFlowActionCall(action, formulas, variables, raw.RecordLookups)
		if !ok {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("action node %q is not a modeled Apex action", firstNonBlank(action.Name, action.Label, action.ActionName))))
			continue
		}
		rule.Actions = append(rule.Actions, flowAction)
	}
	for _, del := range raw.RecordDeletes {
		recordDelete, ok := modeledFlowRecordDelete(del, variables, raw.RecordLookups)
		if !ok {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("record delete node %q is not modeled", firstNonBlank(del.Name, del.Label))))
			continue
		}
		rule.RecordDeletes = append(rule.RecordDeletes, recordDelete)
	}
	for _, customError := range raw.CustomErrors {
		ce := modeledFlowCustomError(customError)
		if ce.Name != "" {
			rule.CustomErrors = append(rule.CustomErrors, ce)
		}
	}
	for _, plugin := range raw.ApexPluginCalls {
		pc := storage.FlowApexPluginCall{Name: firstNonBlank(plugin.Name, plugin.Label), Class: strings.TrimSpace(plugin.ApexClass)}
		for _, input := range plugin.InputParameters {
			pc.Inputs = append(pc.Inputs, storage.WorkflowFieldUpdate{
				Name:         strings.TrimSpace(input.Name),
				LiteralValue: flowLiteralOrVariableOrConstantValue(input.Value, variables, constants),
			})
		}
		if pc.Name != "" && pc.Class != "" {
			rule.ApexPlugins = append(rule.ApexPlugins, pc)
		}
	}
	if !routedDecisions || flowRequiresOrderedGraph(raw) {
		if target := strings.TrimSpace(raw.Start.Connector.TargetReference); target != "" {
			ordered, stepDiagnostics := flowPopulateBranchFromTarget(storage.FlowBranch{Name: name}, target, raw, formulas, variables)
			for _, diagnostic := range stepDiagnostics {
				diagnostics = append(diagnostics, flowUnsupported(path, name, diagnostic.Message))
			}
			if len(ordered.Steps) > 0 {
				rule.Steps = ordered.Steps
				rule.FieldUpdates = nil
				rule.Actions = nil
			}
		}
	}
	if routedDecisions && len(rule.Steps) == 0 {
		branches, branchDiagnostics := modeledFlowDecisionBranches(path, name, raw, formulas, variables)
		diagnostics = append(diagnostics, branchDiagnostics...)
		if len(branches) > 0 {
			rule.Branches = branches
			rule.FieldUpdates = nil
			rule.Actions = nil
		}
	}
	if len(rule.Branches) > 0 || len(rule.Steps) > 0 || len(rule.FieldUpdates) > 0 || len(rule.Actions) > 0 || len(rule.RecordLookups) > 0 || len(rule.RecordCreates) > 0 || len(rule.RecordDeletes) > 0 || len(rule.CustomErrors) > 0 || len(rule.ApexPlugins) > 0 {
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
	switch operator {
	case "", "assign", "equalto", "assigncount", "add":
		return true
	case "additem", "removefirst", "removeall", "removeafterfirst",
		"removebeforefirst", "removeposition", "removeuncommon",
		"subtract", "addatstart":
		return true
	default:
		return false
	}
}

func flowConditionLogicGroups(logic string, conditionCount int) ([][]int, bool) {
	logic = strings.TrimSpace(logic)
	if conditionCount == 0 {
		return [][]int{{}}, true
	}
	if logic == "" || strings.EqualFold(logic, "and") {
		group := make([]int, conditionCount)
		for i := range group {
			group[i] = i
		}
		return [][]int{group}, true
	}
	if strings.EqualFold(logic, "or") {
		groups := make([][]int, conditionCount)
		for i := range groups {
			groups[i] = []int{i}
		}
		return groups, true
	}
	parser := flowConditionLogicParser{tokens: flowConditionLogicTokens(logic), conditionCount: conditionCount}
	groups, ok := parser.parseExpression()
	if !ok || parser.pos != len(parser.tokens) || len(groups) == 0 {
		return nil, false
	}
	return groups, true
}

func flowConditionGroupsNeedFormula(groups [][]int, conditions []flowConditionXML, formulas map[string]string) bool {
	if len(groups) != 1 {
		return true
	}
	for _, group := range groups {
		for _, idx := range group {
			if idx >= len(conditions) {
				continue
			}
			if _, ok := formulas[strings.ToLower(strings.TrimSpace(conditions[idx].LeftValueReference))]; ok {
				return true
			}
		}
	}
	return false
}

func flowConditionGroupsToFormula(groups [][]int, conditions []flowConditionXML, formulas map[string]string, variables map[string]flowVariableXML) string {
	var orParts []string
	for _, group := range groups {
		var andParts []string
		for _, idx := range group {
			if idx >= len(conditions) {
				continue
			}
			conditionFormula := flowConditionToFormula(conditions[idx], formulas, variables)
			if conditionFormula == "" {
				return ""
			}
			andParts = append(andParts, conditionFormula)
		}
		if len(andParts) > 0 {
			if len(andParts) == 1 {
				orParts = append(orParts, andParts[0])
			} else {
				orParts = append(orParts, "AND("+strings.Join(andParts, ",")+")")
			}
		}
	}
	if len(orParts) == 1 {
		return orParts[0]
	}
	return "OR(" + strings.Join(orParts, ",") + ")"
}

func flowConditionToFormula(condition flowConditionXML, formulas map[string]string, variables map[string]flowVariableXML) string {
	left := flowConditionFormulaOperand(condition.LeftValueReference, formulas, variables)
	if left == "" {
		return ""
	}
	value := flowLiteralValue(condition.RightValue)
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(condition.Operator), " ", "")) {
	case "contains":
		return fmt.Sprintf("CONTAINS(%s,%s)", left, formulaQuote(value))
	case "isnull", "isblank":
		if value == "" {
			value = "true"
		}
		return fmt.Sprintf("ISBLANK(%s) = %s", left, formulaQuote(value))
	default:
		return fmt.Sprintf("%s %s %s", left, conditionOperatorToFormula(condition.Operator), formulaQuote(value))
	}
}

func flowConditionFormulaOperand(reference string, formulas map[string]string, variables map[string]flowVariableXML) string {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return ""
	}
	if expression, ok := formulas[strings.ToLower(reference)]; ok {
		return "(" + expression + ")"
	}
	if field := flowRecordFieldReference(reference); field != "" {
		return field
	}
	if field := flowVariableRecordField(reference, variables); field != "" {
		return field
	}
	if field := flowRecordSourceFieldReference(reference); field != "" {
		return field
	}
	return ""
}

func conditionOperatorToFormula(operator string) string {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(operator), " ", "")) {
	case "", "equalto", "equals":
		return "="
	case "notequalto", "notequals":
		return "!="
	case "greaterthan", "gt":
		return ">"
	case "greaterthanorequalto", "gte", "ge":
		return ">="
	case "lessthan", "lt":
		return "<"
	case "lessthanorequalto", "lte", "le":
		return "<="
	case "isnull", "isblank":
		return "="
	case "contains":
		return "CONTAINS"
	default:
		return "="
	}
}

func formulaQuote(value string) string {
	if value == "" || value == "NULL" || value == "null" || value == "true" || value == "false" {
		return value
	}
	if _, err := fmt.Sscanf(value, "%d", new(int)); err == nil {
		return value
	}
	return `"` + value + `"`
}

type flowConditionLogicParser struct {
	tokens         []string
	pos            int
	conditionCount int
}

func flowConditionLogicTokens(logic string) []string {
	var tokens []string
	for i := 0; i < len(logic); {
		ch := logic[i]
		switch {
		case ch == '(' || ch == ')':
			tokens = append(tokens, string(ch))
			i++
		case ch >= '0' && ch <= '9':
			start := i
			for i < len(logic) && logic[i] >= '0' && logic[i] <= '9' {
				i++
			}
			tokens = append(tokens, logic[start:i])
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			i++
		case (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z'):
			start := i
			for i < len(logic) && ((logic[i] >= 'A' && logic[i] <= 'Z') || (logic[i] >= 'a' && logic[i] <= 'z')) {
				i++
			}
			tokens = append(tokens, strings.ToUpper(logic[start:i]))
		default:
			return nil
		}
	}
	return tokens
}

func (p *flowConditionLogicParser) parseExpression() ([][]int, bool) {
	groups, ok := p.parseTerm()
	if !ok {
		return nil, false
	}
	for p.match("OR") {
		right, ok := p.parseTerm()
		if !ok {
			return nil, false
		}
		groups = append(groups, right...)
		if len(groups) > 32 {
			return nil, false
		}
	}
	return groups, true
}

func (p *flowConditionLogicParser) parseTerm() ([][]int, bool) {
	groups, ok := p.parseFactor()
	if !ok {
		return nil, false
	}
	for p.match("AND") {
		right, ok := p.parseFactor()
		if !ok {
			return nil, false
		}
		groups = flowConditionLogicAnd(groups, right)
		if len(groups) > 32 {
			return nil, false
		}
	}
	return groups, true
}

func (p *flowConditionLogicParser) parseFactor() ([][]int, bool) {
	if p.match("(") {
		groups, ok := p.parseExpression()
		if !ok || !p.match(")") {
			return nil, false
		}
		return groups, true
	}
	if p.pos >= len(p.tokens) {
		return nil, false
	}
	token := p.tokens[p.pos]
	index := 0
	for _, r := range token {
		if r < '0' || r > '9' {
			return nil, false
		}
		index = index*10 + int(r-'0')
	}
	if index < 1 || index > p.conditionCount {
		return nil, false
	}
	p.pos++
	return [][]int{{index - 1}}, true
}

func (p *flowConditionLogicParser) match(token string) bool {
	if p.pos >= len(p.tokens) || p.tokens[p.pos] != token {
		return false
	}
	p.pos++
	return true
}

func flowConditionLogicAnd(left, right [][]int) [][]int {
	out := make([][]int, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			group := make([]int, 0, len(l)+len(r))
			group = append(group, l...)
			group = append(group, r...)
			out = append(out, group)
		}
	}
	return out
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

func flowVariableMap(variables []flowVariableXML) map[string]flowVariableXML {
	out := make(map[string]flowVariableXML, len(variables))
	for _, variable := range variables {
		name := strings.TrimSpace(variable.Name)
		if name != "" {
			out[strings.ToLower(name)] = variable
		}
	}
	return out
}

func flowConstantMap(constants []flowConstantXML) map[string]flowValueXML {
	out := make(map[string]flowValueXML, len(constants))
	for _, constant := range constants {
		name := strings.TrimSpace(constant.Name)
		if name != "" {
			out[strings.ToLower(name)] = constant.Value
		}
	}
	return out
}

func flowTextTemplateMap(templates []flowTextTemplateXML) map[string]string {
	out := make(map[string]string, len(templates))
	for _, template := range templates {
		name := strings.TrimSpace(template.Name)
		if name != "" {
			out[strings.ToLower(name)] = template.Text
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

func flowSourceFieldValue(value flowValueXML, variables map[string]flowVariableXML) string {
	reference := strings.TrimSpace(value.ElementReference)
	if reference == "" {
		return ""
	}
	if field := flowRecordSourceFieldReference(reference); field != "" {
		return field
	}
	if field := flowRecordFieldReference(reference); field != "" {
		return field
	}
	if variable, ok := variables[strings.ToLower(reference)]; ok {
		if field := flowRecordSourceFieldReference(variable.Value.ElementReference); field != "" {
			return field
		}
		return flowRecordFieldReference(variable.Value.ElementReference)
	}
	return ""
}

func flowRecordSourceFieldReference(reference string) string {
	reference = strings.TrimSpace(reference)
	for _, prefix := range []string{"$Record.", "Record."} {
		if strings.HasPrefix(reference, prefix) && len(reference) > len(prefix) {
			return strings.TrimSpace(reference[len(prefix):])
		}
	}
	return ""
}

func flowLookupSourceFieldValue(value flowValueXML, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) string {
	reference := strings.TrimSpace(value.ElementReference)
	if reference == "" {
		return ""
	}
	if field := flowSourceFieldValue(value, variables); field != "" {
		return field
	}
	if source := flowLookupFieldReference(reference, lookups); source != "" {
		return source
	}
	if variable, ok := variables[strings.ToLower(reference)]; ok {
		return flowLookupFieldReference(variable.Value.ElementReference, lookups)
	}
	return ""
}

func flowLookupFieldReference(reference string, lookups []flowRecordLookupXML) string {
	reference = strings.TrimSpace(reference)
	dot := strings.LastIndex(reference, ".")
	if dot <= 0 || dot == len(reference)-1 {
		return ""
	}
	lookupName := reference[:dot]
	field := trimObjectPrefix(reference[dot+1:])
	if field == "" {
		return ""
	}
	for _, lookup := range lookups {
		name := firstNonBlank(lookup.Name, lookup.Label)
		if name == "" || !lookup.StoreOutputAutomatically || !lookup.GetFirstRecordOnly {
			continue
		}
		if strings.EqualFold(lookupName, name) {
			return name + "." + field
		}
	}
	return ""
}

func flowLiteralOrVariableValue(value flowValueXML, variables map[string]flowVariableXML) string {
	if literal := flowLiteralValue(value); literal != "" {
		return literal
	}
	reference := strings.TrimSpace(value.ElementReference)
	if variable, ok := variables[strings.ToLower(reference)]; ok {
		return flowLiteralValue(variable.Value)
	}
	return ""
}

func flowLiteralOrConstantValue(value flowValueXML, constants map[string]flowValueXML) string {
	if literal := flowLiteralValue(value); literal != "" {
		return literal
	}
	reference := strings.TrimSpace(value.ElementReference)
	if constant, ok := constants[strings.ToLower(reference)]; ok {
		return flowLiteralValue(constant)
	}
	return ""
}

func flowLiteralOrVariableOrConstantValue(value flowValueXML, variables map[string]flowVariableXML, constants map[string]flowValueXML) string {
	if literal := flowLiteralOrVariableValue(value, variables); literal != "" {
		return literal
	}
	return flowLiteralOrConstantValue(value, constants)
}

func modeledFlowRecordLookup(lookup flowRecordLookupXML, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowRecordLookup, bool) {
	name := firstNonBlank(lookup.Name, lookup.Label)
	objectName := strings.TrimSpace(lookup.Object)
	if name == "" || objectName == "" {
		return storage.FlowRecordLookup{}, false
	}
	if logic := strings.TrimSpace(lookup.FilterLogic); logic != "" && !strings.EqualFold(logic, "and") {
		return storage.FlowRecordLookup{}, false
	}
	out := storage.FlowRecordLookup{
		Name:                     name,
		ObjectName:               objectName,
		GetFirstRecordOnly:       lookup.GetFirstRecordOnly,
		StoreOutputAutomatically: lookup.StoreOutputAutomatically,
	}
	for _, filter := range lookup.Filters {
		field := trimObjectPrefix(strings.TrimSpace(filter.Field))
		if field == "" {
			return storage.FlowRecordLookup{}, false
		}
		out.Criteria = append(out.Criteria, storage.WorkflowCriteriaItem{
			Field:       field,
			Operation:   flowOperator(strings.TrimSpace(filter.Operator)),
			Value:       flowLiteralValue(filter.Value),
			SourceField: flowLookupSourceFieldValue(filter.Value, variables, lookups),
		})
	}
	return out, true
}

func modeledFlowRecordCreate(create flowRecordCreateXML, formulas map[string]string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowRecordCreate, bool) {
	name := firstNonBlank(create.Name, create.Label)
	objectName := strings.TrimSpace(create.Object)
	inputReference := strings.TrimSpace(create.InputReference)
	if objectName == "" && inputReference != "" {
		if variable, ok := variables[strings.ToLower(inputReference)]; ok {
			objectName = strings.TrimSpace(variable.ObjectType)
		}
	}
	if name == "" || objectName == "" {
		return storage.FlowRecordCreate{}, false
	}
	out := storage.FlowRecordCreate{
		Name:                     name,
		ObjectName:               objectName,
		InputReference:           inputReference,
		StoreOutputAutomatically: create.StoreOutputAutomatically,
	}
	for _, assignment := range create.Fields {
		field := trimObjectPrefix(strings.TrimSpace(assignment.Field))
		if field == "" {
			return storage.FlowRecordCreate{}, false
		}
		out.InputAssignments = append(out.InputAssignments, storage.WorkflowFieldUpdate{
			Name:         field,
			Field:        field,
			LiteralValue: flowLiteralOrVariableValue(assignment.Value, variables),
			Formula:      flowExpressionValue(assignment.Value, formulas),
			SourceField:  flowLookupSourceFieldValue(assignment.Value, variables, lookups),
		})
	}
	return out, true
}

func modeledFlowRecordDelete(del flowRecordDeleteXML, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowRecordDelete, bool) {
	name := firstNonBlank(del.Name, del.Label)
	objectName := strings.TrimSpace(del.Object)
	if name == "" || objectName == "" {
		return storage.FlowRecordDelete{}, false
	}
	if logic := strings.TrimSpace(del.FilterLogic); logic != "" && !strings.EqualFold(logic, "and") {
		return storage.FlowRecordDelete{}, false
	}
	out := storage.FlowRecordDelete{
		Name:           name,
		ObjectName:     objectName,
		InputReference: strings.TrimSpace(del.InputReference),
	}
	for _, filter := range del.Filters {
		field := trimObjectPrefix(strings.TrimSpace(filter.Field))
		if field == "" {
			return storage.FlowRecordDelete{}, false
		}
		out.Criteria = append(out.Criteria, storage.WorkflowCriteriaItem{
			Field:       field,
			Operation:   flowOperator(strings.TrimSpace(filter.Operator)),
			Value:       flowLiteralValue(filter.Value),
			SourceField: flowLookupSourceFieldValue(filter.Value, variables, lookups),
		})
	}
	return out, true
}

func modeledFlowSubflow(subflow flowSubflowXML, formulas map[string]string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowSubflow, bool) {
	name := firstNonBlank(subflow.Name, subflow.Label)
	flowName := strings.TrimSpace(subflow.FlowName)
	if name == "" || flowName == "" {
		return storage.FlowSubflow{}, false
	}
	out := storage.FlowSubflow{
		Name:     name,
		FlowName: flowName,
	}
	for _, input := range subflow.InputAssignments {
		field := trimObjectPrefix(strings.TrimSpace(input.Field))
		if field == "" {
			continue
		}
		out.InputAssignments = append(out.InputAssignments, storage.WorkflowFieldUpdate{
			Name:         field,
			Field:        field,
			LiteralValue: flowLiteralOrVariableValue(input.Value, variables),
			Formula:      flowExpressionValue(input.Value, formulas),
			SourceField:  flowLookupSourceFieldValue(input.Value, variables, lookups),
		})
	}
	for _, output := range subflow.OutputAssignments {
		flowAssignment, ok := modeledFlowAssignment(flowAssignmentXML{}, output, formulas, variables, lookups)
		if !ok {
			continue
		}
		out.OutputAssignments = append(out.OutputAssignments, flowAssignment)
	}
	return out, true
}

func modeledFlowTransform(transform flowTransformXML, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowTransform, bool) {
	name := firstNonBlank(transform.Name, transform.Label)
	transformType := strings.TrimSpace(transform.TransformType)
	if name == "" || transformType == "" {
		return storage.FlowTransform{}, false
	}
	out := storage.FlowTransform{
		Name:             name,
		TransformType:    transformType,
		SourceCollection: strings.TrimSpace(transform.SourceCollection),
		TargetCollection: strings.TrimSpace(transform.TargetCollection),
		SumField:         strings.TrimSpace(transform.SumField),
	}
	for _, mapping := range transform.FieldMappings {
		field := trimObjectPrefix(strings.TrimSpace(mapping.Field))
		if field == "" {
			continue
		}
		out.FieldMappings = append(out.FieldMappings, storage.WorkflowFieldUpdate{
			Name:         field,
			Field:        field,
			LiteralValue: flowLiteralValue(mapping.Value),
			SourceField:  flowLookupSourceFieldValue(mapping.Value, variables, lookups),
		})
	}
	return out, true
}

func modeledFlowCollectionProcessor(proc flowCollectionProcessorXML, variables map[string]flowVariableXML) (storage.FlowCollectionProcessor, bool) {
	name := firstNonBlank(proc.Name, proc.Label)
	processorType := strings.TrimSpace(proc.ProcessorType)
	if name == "" || processorType == "" {
		return storage.FlowCollectionProcessor{}, false
	}
	out := storage.FlowCollectionProcessor{
		Name:                name,
		ProcessorType:       processorType,
		CollectionReference: strings.TrimSpace(proc.CollectionReference),
		TargetCollection:    strings.TrimSpace(proc.TargetCollection),
		SortField:           strings.TrimSpace(proc.SortField),
		SortOrder:           strings.TrimSpace(proc.SortOrder),
	}
	for _, filter := range proc.Filters {
		out.Criteria = append(out.Criteria, storage.WorkflowCriteriaItem{
			Field:     trimObjectPrefix(strings.TrimSpace(filter.Field)),
			Operation: flowOperator(strings.TrimSpace(filter.Operator)),
			Value:     flowLiteralValue(filter.Value),
		})
	}
	for _, mapping := range proc.FieldMappings {
		field := trimObjectPrefix(strings.TrimSpace(mapping.Field))
		if field == "" {
			continue
		}
		out.FieldMappings = append(out.FieldMappings, storage.WorkflowFieldUpdate{
			Name:         field,
			Field:        field,
			LiteralValue: flowLiteralValue(mapping.Value),
			SourceField:  flowLookupSourceFieldValue(mapping.Value, variables, nil),
		})
	}
	return out, true
}

func flowDecisionHasRoutedBranch(decision flowDecisionXML) bool {
	if strings.TrimSpace(decision.DefaultConnector.TargetReference) != "" {
		return true
	}
	for _, rule := range decision.Rules {
		if strings.TrimSpace(rule.Connector.TargetReference) != "" {
			return true
		}
	}
	return false
}

func modeledFlowDecisionBranches(path, flowName string, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML) ([]storage.FlowBranch, []diagnostic.Diagnostic) {
	var branches []storage.FlowBranch
	var diagnostics []diagnostic.Diagnostic
	for _, decision := range raw.Decisions {
		if !flowDecisionHasRoutedBranch(decision) {
			continue
		}
		for _, decisionRule := range decision.Rules {
			branchName := firstNonBlank(decisionRule.Name, decision.Name)
			groups, ok := flowConditionLogicGroups(decisionRule.ConditionLogic, len(decisionRule.Conditions))
			if !ok {
				diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("decision %q branch %q condition logic %q is not supported", decision.Name, branchName, decisionRule.ConditionLogic)))
				continue
			}
			target := strings.TrimSpace(decisionRule.Connector.TargetReference)
			if target == "" {
				continue
			}
			for groupIndex, group := range groups {
				branch := storage.FlowBranch{Name: branchName}
				if len(groups) > 1 {
					branch.Name = fmt.Sprintf("%s#%d", branchName, groupIndex+1)
				}
				modeled := true
				for _, conditionIndex := range group {
					condition := decisionRule.Conditions[conditionIndex]
					if _, ok := formulas[strings.ToLower(strings.TrimSpace(condition.LeftValueReference))]; ok {
						continue
					}
					field := flowVariableRecordField(condition.LeftValueReference, variables)
					if field == "" {
						field = flowRecordSourceFieldReference(condition.LeftValueReference)
					}
					if field == "" {
						diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("decision %q branch %q condition %q is not a $Record field or modeled flow resource", decision.Name, branch.Name, condition.LeftValueReference)))
						modeled = false
						continue
					}
					branch.Criteria = append(branch.Criteria, storage.WorkflowCriteriaItem{
						Field:     field,
						Operation: flowOperator(strings.TrimSpace(condition.Operator)),
						Value:     flowLiteralValue(condition.RightValue),
					})
				}
				if modeled && flowConditionGroupsNeedFormula([][]int{group}, decisionRule.Conditions, formulas) {
					formula := flowConditionGroupsToFormula([][]int{group}, decisionRule.Conditions, formulas, variables)
					if formula == "" {
						diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("decision %q branch %q condition logic %q is not supported", decision.Name, branch.Name, decisionRule.ConditionLogic)))
						continue
					}
					branch.Formula = formula
					branch.Criteria = nil
				}
				if !modeled {
					continue
				}
				var branchDiagnostics []diagnostic.Diagnostic
				branch, branchDiagnostics = flowPopulateBranchFromTarget(branch, target, raw, formulas, variables)
				for _, diagnostic := range branchDiagnostics {
					diagnostics = append(diagnostics, flowUnsupported(path, flowName, diagnostic.Message))
				}
				if flowBranchHasEffects(branch) {
					branches = append(branches, branch)
				}
			}
		}
		defaultTarget := strings.TrimSpace(decision.DefaultConnector.TargetReference)
		if defaultTarget == "" {
			continue
		}
		branch := storage.FlowBranch{Name: firstNonBlank(decision.Name+"_Default", decision.Name), Default: true}
		var branchDiagnostics []diagnostic.Diagnostic
		branch, branchDiagnostics = flowPopulateBranchFromTarget(branch, defaultTarget, raw, formulas, variables)
		for _, diagnostic := range branchDiagnostics {
			diagnostics = append(diagnostics, flowUnsupported(path, flowName, diagnostic.Message))
		}
		if flowBranchHasEffects(branch) {
			branches = append(branches, branch)
		}
	}
	return branches, diagnostics
}

func flowStepDisplayName(step storage.FlowStep) string {
	switch step.Kind {
	case "fieldUpdate":
		if len(step.FieldUpdates) > 0 {
			return step.FieldUpdates[0].Name
		}
	case "recordLookup":
		return step.RecordLookup.Name
	case "recordCreate":
		return step.RecordCreate.Name
	case "recordUpdate":
		return step.RecordUpdate.Name
	case "recordDelete":
		return step.RecordDelete.Name
	case "subflow":
		return step.Subflow.Name
	case "action":
		return step.Action.Name
	case "assignment":
		return step.Assignment.Name
	case "loop":
		return step.Loop.Name
	case "customError":
		return step.CustomError.Name
	case "transform":
		return step.Transform.Name
	case "collectionProcessor":
		return step.CollectionProcessor.Name
	}
	return ""
}

func buildFlowFaultBranches(steps []storage.FlowStep, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML) {
	hasFault := false
	for _, step := range steps {
		if step.FaultTarget != "" {
			hasFault = true
			break
		}
	}
	if !hasFault {
		return
	}
	mainNames := make(map[string]bool, len(steps))
	for _, step := range steps {
		if name := strings.ToLower(strings.TrimSpace(flowStepDisplayName(step))); name != "" {
			mainNames[name] = true
		}
	}
	for i := range steps {
		step := &steps[i]
		if step.FaultTarget == "" {
			continue
		}
		faultName := strings.ToLower(strings.TrimSpace(step.FaultTarget))
		if mainNames[faultName] {
			continue
		}
		faultBranch := storage.FlowBranch{}
		faultBranch, _ = flowPopulateBranchFromTargetWithStop(faultBranch, step.FaultTarget, "", raw, formulas, variables, make(map[string]bool))
		if len(faultBranch.Steps) > 0 {
			step.FaultBranch = faultBranch.Steps
		}
	}
}

func flowPopulateBranchFromTarget(branch storage.FlowBranch, target string, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML) (storage.FlowBranch, []diagnostic.Diagnostic) {
	return flowPopulateBranchFromTargetWithStop(branch, target, "", raw, formulas, variables, make(map[string]bool))
}

func flowPopulateBranchFromTargetWithStop(branch storage.FlowBranch, target, stop string, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML, visited map[string]bool) (storage.FlowBranch, []diagnostic.Diagnostic) {
	var diagnostics []diagnostic.Diagnostic
	for target = strings.TrimSpace(target); target != ""; {
		if stop != "" && strings.EqualFold(target, stop) {
			break
		}
		key := strings.ToLower(target)
		if visited[key] {
			break
		}
		visited[key] = true
		next := ""
		modeled := false
		for _, assignment := range raw.Assignments {
			if !strings.EqualFold(target, firstNonBlank(assignment.Name, assignment.Label)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(assignment.FaultConnector.TargetReference)
			for _, item := range assignment.Items {
				if update, ok := modeledFlowAssignmentUpdate(assignment, item, formulas, variables); ok {
					branch.FieldUpdates = append(branch.FieldUpdates, update)
					branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "fieldUpdate", FaultTarget: fault, FieldUpdates: []storage.WorkflowFieldUpdate{update}})
					continue
				}
				flowAssignment, ok := modeledFlowAssignment(assignment, item, formulas, variables, raw.RecordLookups)
				if !ok {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("assignment %q is not modeled in routed decision branch", firstNonBlank(assignment.Name, assignment.Label))})
					continue
				}
				branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "assignment", FaultTarget: fault, Assignment: flowAssignment})
			}
			next = assignment.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, update := range raw.RecordUpdates {
			if !strings.EqualFold(target, firstNonBlank(update.Name, update.Label)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(update.FaultConnector.TargetReference)
			if updates, ok := modeledFlowRecordUpdates(update, formulas, variables); ok {
				branch.FieldUpdates = append(branch.FieldUpdates, updates...)
				if len(updates) > 0 {
					branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "fieldUpdate", FaultTarget: fault, FieldUpdates: updates})
				}
			} else if recordUpdate, ok := modeledFlowRecordUpdate(update, formulas, variables, raw.RecordLookups); ok {
				branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "recordUpdate", FaultTarget: fault, RecordUpdate: recordUpdate})
			} else {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("record update %q is not modeled in routed decision branch", firstNonBlank(update.Name, update.Label))})
			}
			next = update.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, lookup := range raw.RecordLookups {
			if !strings.EqualFold(target, firstNonBlank(lookup.Name, lookup.Label)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(lookup.FaultConnector.TargetReference)
			if recordLookup, ok := modeledFlowRecordLookup(lookup, variables, raw.RecordLookups); ok {
				branch.RecordLookups = append(branch.RecordLookups, recordLookup)
				branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "recordLookup", FaultTarget: fault, RecordLookup: recordLookup})
			} else {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("record lookup node %q is not modeled in routed decision branch", firstNonBlank(lookup.Name, lookup.Label))})
			}
			next = lookup.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, create := range raw.RecordCreates {
			if !strings.EqualFold(target, firstNonBlank(create.Name, create.Label)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(create.FaultConnector.TargetReference)
			if recordCreate, ok := modeledFlowRecordCreate(create, formulas, variables, raw.RecordLookups); ok {
				branch.RecordCreates = append(branch.RecordCreates, recordCreate)
				branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "recordCreate", FaultTarget: fault, RecordCreate: recordCreate})
			} else {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("record create node %q is not modeled in routed decision branch", firstNonBlank(create.Name, create.Label))})
			}
			next = create.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, del := range raw.RecordDeletes {
			if !strings.EqualFold(target, firstNonBlank(del.Name, del.Label)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(del.FaultConnector.TargetReference)
			if recordDelete, ok := modeledFlowRecordDelete(del, variables, raw.RecordLookups); ok {
				branch.RecordDeletes = append(branch.RecordDeletes, recordDelete)
				branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "recordDelete", FaultTarget: fault, RecordDelete: recordDelete})
			} else {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("record delete node %q is not modeled in routed decision branch", firstNonBlank(del.Name, del.Label))})
			}
			next = del.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, subflow := range raw.Subflows {
			if !strings.EqualFold(target, firstNonBlank(subflow.Name, subflow.Label)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(subflow.FaultConnector.TargetReference)
			if flowSubflow, ok := modeledFlowSubflow(subflow, formulas, variables, raw.RecordLookups); ok {
				branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "subflow", FaultTarget: fault, Subflow: flowSubflow})
			} else {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("subflow node %q is not modeled in routed decision branch", firstNonBlank(subflow.Name, subflow.Label))})
			}
			next = subflow.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, plugin := range raw.ApexPluginCalls {
			if !strings.EqualFold(target, firstNonBlank(plugin.Name, plugin.Label)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(plugin.FaultConnector.TargetReference)
			pc := storage.FlowApexPluginCall{Name: firstNonBlank(plugin.Name, plugin.Label), Class: strings.TrimSpace(plugin.ApexClass)}
			for _, input := range plugin.InputParameters {
				pc.Inputs = append(pc.Inputs, storage.WorkflowFieldUpdate{
					Name: strings.TrimSpace(input.Name), LiteralValue: flowLiteralOrVariableValue(input.Value, variables),
				})
			}
			if pc.Name == "" || pc.Class == "" {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("apex plugin call node %q is not modeled", firstNonBlank(plugin.Name, plugin.Label))})
				break
			}
			branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "action", FaultTarget: fault, Action: storage.FlowAction{Name: pc.Name, ActionType: "apexPlugin", ActionName: pc.Class, Inputs: pc.Inputs}})
			next = plugin.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, transform := range raw.Transforms {
			if !strings.EqualFold(target, firstNonBlank(transform.Name, transform.Label)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(transform.FaultConnector.TargetReference)
			if ft, ok := modeledFlowTransform(transform, variables, raw.RecordLookups); ok {
				branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "transform", FaultTarget: fault, Transform: ft})
			} else {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("transform node %q is not modeled", firstNonBlank(transform.Name, transform.Label))})
			}
			next = transform.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, proc := range raw.CollectionProcessors {
			if !strings.EqualFold(target, firstNonBlank(proc.Name, proc.Label)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(proc.FaultConnector.TargetReference)
			if cp, ok := modeledFlowCollectionProcessor(proc, variables); ok {
				branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "collectionProcessor", FaultTarget: fault, CollectionProcessor: cp})
			} else {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("collection processor node %q is not modeled", firstNonBlank(proc.Name, proc.Label))})
			}
			next = proc.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, action := range raw.ActionCalls {
			if !strings.EqualFold(target, firstNonBlank(action.Name, action.Label, action.ActionName)) {
				continue
			}
			modeled = true
			fault := strings.TrimSpace(action.FaultConnector.TargetReference)
			if flowAction, ok := modeledFlowActionCall(action, formulas, variables, raw.RecordLookups); ok {
				branch.Actions = append(branch.Actions, flowAction)
				branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "action", FaultTarget: fault, Action: flowAction})
			} else {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("action node %q is not modeled in routed decision branch", firstNonBlank(action.Name, action.Label, action.ActionName))})
			}
			next = action.Connector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, loop := range raw.Loops {
			if !strings.EqualFold(target, loop.Name) {
				continue
			}
			modeled = true
			loopName := strings.TrimSpace(loop.Name)
			collection := strings.TrimSpace(loop.CollectionReference)
			bodyTarget := strings.TrimSpace(loop.NextValueConnector.TargetReference)
			if loopName == "" || collection == "" || bodyTarget == "" {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("loop node %q is not modeled", loop.Name)})
				break
			}
			body := storage.FlowBranch{Name: loopName}
			var bodyDiagnostics []diagnostic.Diagnostic
			body, bodyDiagnostics = flowPopulateBranchFromTargetWithStop(body, bodyTarget, loopName, raw, formulas, variables, make(map[string]bool))
			diagnostics = append(diagnostics, bodyDiagnostics...)
			currentItem := strings.TrimSpace(loop.AssignNextValueToReference)
			if currentItem == "" {
				currentItem = loopName
			}
			branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "loop", FaultTarget: strings.TrimSpace(loop.FaultConnector.TargetReference), Loop: storage.FlowLoop{Name: loopName, CollectionReference: collection, CurrentItemReference: currentItem, Steps: body.Steps}})
			next = loop.NoMoreValuesConnector.TargetReference
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, decision := range raw.Decisions {
			if !strings.EqualFold(target, decision.Name) {
				continue
			}
			modeled = true
			stepBranches, decisionDiagnostics := modeledFlowStepDecisionBranches(decision, raw, formulas, variables)
			diagnostics = append(diagnostics, decisionDiagnostics...)
			if len(stepBranches) == 0 {
				next = flowDefaultDecisionTarget(decision)
				break
			}
			branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "decision", Branches: stepBranches})
			break
		}
		if modeled {
			target = next
			continue
		}
		for _, customError := range raw.CustomErrors {
			if !strings.EqualFold(target, customError.Name) {
				continue
			}
			modeled = true
			branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "customError", CustomError: modeledFlowCustomError(customError)})
			break
		}
		if modeled {
			target = ""
			continue
		}
		if !modeled {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("routed decision target %q is not modeled", target)})
		}
		break
	}
	buildFlowFaultBranches(branch.Steps, raw, formulas, variables)
	return branch, diagnostics
}

func modeledFlowCustomError(customError flowCustomErrorXML) storage.FlowCustomError {
	ce := storage.FlowCustomError{Name: strings.TrimSpace(customError.Name), Description: strings.TrimSpace(customError.Description)}
	for _, msg := range customError.Messages {
		ce.Messages = append(ce.Messages, storage.FlowCustomErrorMessage{Message: strings.TrimSpace(msg.Message)})
	}
	return ce
}

func modeledFlowStepDecisionBranches(decision flowDecisionXML, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML) ([]storage.FlowBranch, []diagnostic.Diagnostic) {
	var branches []storage.FlowBranch
	var diagnostics []diagnostic.Diagnostic
	for _, rule := range decision.Rules {
		target := strings.TrimSpace(rule.Connector.TargetReference)
		if target == "" {
			continue
		}
		groups, ok := flowConditionLogicGroups(rule.ConditionLogic, len(rule.Conditions))
		if !ok {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("decision %q branch %q condition logic %q is not supported", decision.Name, firstNonBlank(rule.Name, decision.Name), rule.ConditionLogic)})
			continue
		}
		for groupIndex, group := range groups {
			branch := storage.FlowBranch{Name: firstNonBlank(rule.Name, decision.Name)}
			if len(groups) > 1 {
				branch.Name = fmt.Sprintf("%s#%d", branch.Name, groupIndex+1)
			}
			for _, conditionIndex := range group {
				condition := rule.Conditions[conditionIndex]
				left := firstNonBlank(flowVariableRecordField(condition.LeftValueReference, variables), flowRecordSourceFieldReference(condition.LeftValueReference), strings.TrimSpace(condition.LeftValueReference))
				if left == "" {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("decision %q branch %q condition %q is not modeled", decision.Name, branch.Name, condition.LeftValueReference)})
					continue
				}
				branch.Criteria = append(branch.Criteria, storage.WorkflowCriteriaItem{
					Field:     left,
					Operation: flowOperator(strings.TrimSpace(condition.Operator)),
					Value:     flowLiteralValue(condition.RightValue),
				})
			}
			var branchDiagnostics []diagnostic.Diagnostic
			branch, branchDiagnostics = flowPopulateBranchFromTargetWithStop(branch, target, "", raw, formulas, variables, make(map[string]bool))
			diagnostics = append(diagnostics, branchDiagnostics...)
			if flowBranchHasEffects(branch) {
				branches = append(branches, branch)
			}
		}
	}
	defaultTarget := strings.TrimSpace(decision.DefaultConnector.TargetReference)
	if defaultTarget != "" {
		branch := storage.FlowBranch{Name: firstNonBlank(decision.Name+"_Default", decision.Name), Default: true}
		var branchDiagnostics []diagnostic.Diagnostic
		branch, branchDiagnostics = flowPopulateBranchFromTargetWithStop(branch, defaultTarget, "", raw, formulas, variables, make(map[string]bool))
		diagnostics = append(diagnostics, branchDiagnostics...)
		if flowBranchHasEffects(branch) {
			branches = append(branches, branch)
		}
	}
	return branches, diagnostics
}

func modeledFlowAssignmentUpdate(assignment flowAssignmentXML, item flowAssignmentItemXML, formulas map[string]string, variables map[string]flowVariableXML) (storage.WorkflowFieldUpdate, bool) {
	if !flowAssignmentOperatorSupported(item.Operator) {
		return storage.WorkflowFieldUpdate{}, false
	}
	operator := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(item.Operator), " ", ""))
	if operator == "assigncount" || operator == "add" {
		return storage.WorkflowFieldUpdate{}, false
	}
	field := flowRecordFieldReference(item.AssignToReference)
	if field == "" {
		return storage.WorkflowFieldUpdate{}, false
	}
	return storage.WorkflowFieldUpdate{
		Name:         firstNonBlank(assignment.Name, assignment.Label, field),
		Field:        field,
		LiteralValue: flowLiteralValue(item.Value),
		Formula:      flowExpressionValue(item.Value, formulas),
		SourceField:  flowSourceFieldValue(item.Value, variables),
	}, true
}

func modeledFlowAssignment(assignment flowAssignmentXML, item flowAssignmentItemXML, formulas map[string]string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowAssignment, bool) {
	if !flowAssignmentOperatorSupported(item.Operator) {
		return storage.FlowAssignment{}, false
	}
	target := strings.TrimSpace(item.AssignToReference)
	if target == "" {
		return storage.FlowAssignment{}, false
	}
	return storage.FlowAssignment{
		Name:         firstNonBlank(assignment.Name, assignment.Label, target),
		Target:       target,
		Operator:     strings.TrimSpace(item.Operator),
		LiteralValue: flowLiteralOrVariableValue(item.Value, variables),
		SourceField:  firstNonBlank(flowLookupSourceFieldValue(item.Value, variables, lookups), strings.TrimSpace(item.Value.ElementReference)),
	}, true
}

func modeledFlowRecordUpdates(update flowRecordUpdateXML, formulas map[string]string, variables map[string]flowVariableXML) ([]storage.WorkflowFieldUpdate, bool) {
	if strings.TrimSpace(update.Object) != "" {
		return nil, false
	}
	if !flowUpdatesTriggeringRecord(update.InputReference) {
		return nil, false
	}
	if logic := strings.TrimSpace(update.FilterLogic); logic != "" && !strings.EqualFold(logic, "and") {
		return nil, false
	}
	var updates []storage.WorkflowFieldUpdate
	for _, assignment := range update.Fields {
		field := trimObjectPrefix(strings.TrimSpace(assignment.Field))
		if field == "" {
			return nil, false
		}
		updates = append(updates, storage.WorkflowFieldUpdate{
			Name:         firstNonBlank(update.Name, update.Label, field),
			Field:        field,
			LiteralValue: flowLiteralValue(assignment.Value),
			Formula:      flowExpressionValue(assignment.Value, formulas),
			SourceField:  flowSourceFieldValue(assignment.Value, variables),
		})
	}
	return updates, true
}

func modeledFlowRecordUpdate(update flowRecordUpdateXML, formulas map[string]string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowRecordUpdate, bool) {
	name := firstNonBlank(update.Name, update.Label)
	inputReference := strings.TrimSpace(update.InputReference)
	objectName := strings.TrimSpace(update.Object)
	if objectName == "" && inputReference != "" {
		if variable, ok := variables[strings.ToLower(inputReference)]; ok {
			objectName = strings.TrimSpace(variable.ObjectType)
		}
		if objectName == "" {
			if obj := flowRecordReferenceObject(inputReference); obj != "" {
				objectName = obj
			}
		}
		if objectName == "" {
			for _, lookup := range lookups {
				if lookup.StoreOutputAutomatically && lookup.GetFirstRecordOnly && strings.EqualFold(inputReference, firstNonBlank(lookup.Name, lookup.Label)) {
					objectName = strings.TrimSpace(lookup.Object)
					break
				}
			}
		}
	}
	if name == "" || objectName == "" {
		return storage.FlowRecordUpdate{}, false
	}
	if logic := strings.TrimSpace(update.FilterLogic); logic != "" && !strings.EqualFold(logic, "and") {
		return storage.FlowRecordUpdate{}, false
	}
	out := storage.FlowRecordUpdate{Name: name, ObjectName: objectName, InputReference: inputReference}
	for _, filter := range update.Filters {
		field := trimObjectPrefix(strings.TrimSpace(filter.Field))
		if field == "" {
			return storage.FlowRecordUpdate{}, false
		}
		out.Criteria = append(out.Criteria, storage.WorkflowCriteriaItem{
			Field:       field,
			Operation:   flowOperator(strings.TrimSpace(filter.Operator)),
			Value:       flowLiteralValue(filter.Value),
			SourceField: flowLookupSourceFieldValue(filter.Value, variables, lookups),
		})
	}
	for _, assignment := range update.Fields {
		field := trimObjectPrefix(strings.TrimSpace(assignment.Field))
		if field == "" {
			return storage.FlowRecordUpdate{}, false
		}
		out.InputAssignments = append(out.InputAssignments, storage.WorkflowFieldUpdate{
			Name:         field,
			Field:        field,
			LiteralValue: flowLiteralOrVariableValue(assignment.Value, variables),
			Formula:      flowExpressionValue(assignment.Value, formulas),
			SourceField:  flowLookupSourceFieldValue(assignment.Value, variables, lookups),
		})
	}
	return out, true
}

func flowBranchHasEffects(branch storage.FlowBranch) bool {
	return len(branch.Steps) > 0 || len(branch.FieldUpdates) > 0 || len(branch.Actions) > 0 || len(branch.RecordLookups) > 0 || len(branch.RecordCreates) > 0
}

func flowRequiresOrderedGraph(raw flowXML) bool {
	if len(raw.Loops) > 0 {
		return true
	}
	if len(raw.CustomErrors) > 0 {
		return true
	}
	if len(raw.RecordDeletes) > 0 {
		return true
	}
	if len(raw.Subflows) > 0 {
		return true
	}
	if len(raw.ApexPluginCalls) > 0 {
		return true
	}
	if len(raw.Transforms) > 0 {
		return true
	}
	if len(raw.CollectionProcessors) > 0 {
		return true
	}
	for _, update := range raw.RecordUpdates {
		if strings.TrimSpace(update.Object) != "" || !flowUpdatesTriggeringRecord(update.InputReference) {
			return true
		}
	}
	for _, create := range raw.RecordCreates {
		if strings.TrimSpace(create.InputReference) != "" {
			return true
		}
	}
	for _, assignment := range raw.Assignments {
		for _, item := range assignment.Items {
			if flowRecordFieldReference(item.AssignToReference) == "" {
				return true
			}
		}
	}
	return false
}

func flowReferenceModeled(reference string, variables map[string]flowVariableXML, constants map[string]flowValueXML, lookups []flowRecordLookupXML, formulas map[string]string) bool {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return false
	}
	key := strings.ToLower(reference)
	if _, ok := variables[key]; ok {
		return true
	}
	if _, ok := constants[key]; ok {
		return true
	}
	if _, ok := formulas[key]; ok {
		return true
	}
	for _, lookup := range lookups {
		if strings.EqualFold(reference, firstNonBlank(lookup.Name, lookup.Label)) {
			return true
		}
	}
	return false
}

func flowVariableRecordField(reference string, variables map[string]flowVariableXML) string {
	variable, ok := variables[strings.ToLower(strings.TrimSpace(reference))]
	if !ok {
		return ""
	}
	return flowRecordFieldReference(variable.Value.ElementReference)
}

func flowRecordReferenceObject(reference string) string {
	reference = strings.TrimSpace(reference)
	for _, prefix := range []string{"$Record.", "Record."} {
		if strings.HasPrefix(reference, prefix) {
			reference = strings.TrimPrefix(reference, prefix)
		}
	}
	if strings.HasSuffix(reference, "__r") {
		return strings.TrimSuffix(reference, "__r") + "__c"
	}
	return ""
}

func flowDefaultDecisionTarget(decision flowDecisionXML) string {
	if target := strings.TrimSpace(decision.DefaultConnector.TargetReference); target != "" {
		return target
	}
	for _, rule := range decision.Rules {
		if target := strings.TrimSpace(rule.Connector.TargetReference); target != "" {
			return target
		}
	}
	return ""
}

func flowNodeReferenceModeled(reference string, raw flowXML) bool {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return false
	}
	for _, assignment := range raw.Assignments {
		if strings.EqualFold(reference, firstNonBlank(assignment.Name, assignment.Label)) {
			return true
		}
	}
	for _, update := range raw.RecordUpdates {
		if strings.EqualFold(reference, firstNonBlank(update.Name, update.Label)) {
			return true
		}
	}
	for _, lookup := range raw.RecordLookups {
		if strings.EqualFold(reference, firstNonBlank(lookup.Name, lookup.Label)) {
			return true
		}
	}
	for _, create := range raw.RecordCreates {
		if strings.EqualFold(reference, firstNonBlank(create.Name, create.Label)) {
			return true
		}
	}
	for _, action := range raw.ActionCalls {
		if strings.EqualFold(reference, firstNonBlank(action.Name, action.Label, action.ActionName)) {
			return true
		}
	}
	for _, loop := range raw.Loops {
		if strings.EqualFold(reference, loop.Name) {
			return true
		}
	}
	for _, decision := range raw.Decisions {
		if strings.EqualFold(reference, decision.Name) {
			return true
		}
	}
	for _, customError := range raw.CustomErrors {
		if strings.EqualFold(reference, customError.Name) {
			return true
		}
	}
	for _, plugin := range raw.ApexPluginCalls {
		if strings.EqualFold(reference, firstNonBlank(plugin.Name, plugin.Label)) {
			return true
		}
	}
	for _, subflow := range raw.Subflows {
		if strings.EqualFold(reference, firstNonBlank(subflow.Name, subflow.Label)) {
			return true
		}
	}
	for _, del := range raw.RecordDeletes {
		if strings.EqualFold(reference, firstNonBlank(del.Name, del.Label)) {
			return true
		}
	}
	return false
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
	case "", "autolaunchedflow", "workflow", "customevent", "invocableprocess", "approvalworkflow":
		return true
	default:
		return false
	}
}

func flowProcessTypeNonDML(processType string) bool {
	switch strings.ToLower(strings.TrimSpace(processType)) {
	case "appprocess", "orchestrator":
		return true
	default:
		return false
	}
}

func flowTriggerTypeDML(triggerType string) bool {
	switch strings.ToLower(strings.TrimSpace(triggerType)) {
	case "", "recordaftersave", "recordbeforesave", "recordbeforedelete", "recordafterdelete":
		return true
	case "scheduled", "platformevent", "automationevent", "dataclouddatachange", "activation":
		return false
	default:
		return false
	}
}

func flowUpdatesTriggeringRecord(input string) bool {
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "" || input == "$record" || input == "record" || input == "$record__prior"
}

func modeledFlowActionCall(action flowActionCallXML, formulas map[string]string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowAction, bool) {
	actionType := strings.TrimSpace(action.ActionType)
	if actionType != "" && !strings.EqualFold(actionType, "apex") && !flowBuiltinActionType(actionType) {
		return storage.FlowAction{}, false
	}
	actionName := strings.TrimSpace(action.ActionName)
	if actionName == "" && typeOnlyBuiltinAction(actionType) {
		actionName = actionType
	}
	if actionName == "" {
		return storage.FlowAction{}, false
	}
	className := actionName
	methodName := ""
	if dot := strings.LastIndex(actionName, "."); dot > 0 && dot < len(actionName)-1 {
		className = actionName[:dot]
		methodName = actionName[dot+1:]
	}
	out := storage.FlowAction{
		Name:       firstNonBlank(action.Name, action.Label, actionName),
		ActionType: actionType,
		ActionName: actionName,
		ClassName:  className,
		MethodName: methodName,
	}
	for _, input := range action.InputParameters {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return storage.FlowAction{}, false
		}
		out.Inputs = append(out.Inputs, storage.WorkflowFieldUpdate{
			Name:         name,
			Field:        name,
			LiteralValue: flowLiteralOrVariableValue(input.Value, variables),
			Formula:      flowExpressionValue(input.Value, formulas),
			SourceField:  flowLookupSourceFieldValue(input.Value, variables, lookups),
		})
	}
	return out, true
}

func flowBuiltinActionType(actionType string) bool {
	switch strings.ToLower(strings.TrimSpace(actionType)) {
	case "chatterpost", "emailsimple", "customnotificationaction", "flow", "quickaction", "emailalert":
		return true
	default:
		return false
	}
}

func typeOnlyBuiltinAction(actionType string) bool {
	switch strings.ToLower(strings.TrimSpace(actionType)) {
	case "flow", "quickaction", "emailalert":
		return true
	default:
		return false
	}
}

func flowOperator(operator string) string {
	switch strings.ToLower(strings.ReplaceAll(operator, " ", "")) {
	case "", "equalto", "equals":
		return "equals"
	case "notequalto", "notequals":
		return "notEqual"
	case "isnull":
		return "isNull"
	case "isblank":
		return "isBlank"
	case "isempty":
		return "isEmpty"
	case "ischanged":
		return "isChanged"
	case "in":
		return "in"
	case "notin":
		return "notIn"
	case "wasset":
		return "wasSet"
	case "haserror":
		return "hasError"
	default:
		return operator
	}
}

func flowUnsupportedNodeDiagnostics(path, flowName string, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, screen := range raw.Screens {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("screen node %q is not supported", screen.Name)))
	}
	for _, wait := range raw.Waits {
		diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("wait node %q is not supported", wait.Name)))
	}
	return diagnostics
}

func flowUnsupported(path, flowName, message string) diagnostic.Diagnostic {
	if flowName != "" {
		message = flowName + ": " + message
	}
	return diagnostic.Diagnostic{Severity: diagnostic.Warning, Code: "GLADEAUTO002", Message: message, File: path}
}
