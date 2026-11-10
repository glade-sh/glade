package automation

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/storage"
)

type flowXML struct {
	graphStop        string
	graph            *flowTargetGraph
	decisionFormulas map[string]string
	frameFormulas    map[string]string

	APIVersion            string                       `xml:"apiVersion"`
	Label                 string                       `xml:"label"`
	StartElementReference string                       `xml:"startElementReference"`
	InterviewLabel        string                       `xml:"interviewLabel"`
	ProcessType           string                       `xml:"processType"`
	Status                string                       `xml:"status"`
	TriggerOrder          int                          `xml:"triggerOrder"`
	RunInMode             string                       `xml:"runInMode"`
	Start                 flowStartXML                 `xml:"start"`
	Formulas              []flowFormulaXML             `xml:"formulas"`
	Constants             []flowConstantXML            `xml:"constants"`
	TextTemplates         []flowTextTemplateXML        `xml:"textTemplates"`
	Decisions             []flowDecisionXML            `xml:"decisions"`
	Assignments           []flowAssignmentXML          `xml:"assignments"`
	RecordUpdates         []flowRecordUpdateXML        `xml:"recordUpdates"`
	Screens               []flowNamedNodeXML           `xml:"screens"`
	Loops                 []flowLoopXML                `xml:"loops"`
	Subflows              []flowSubflowXML             `xml:"subflows"`
	ActionCalls           []flowActionCallXML          `xml:"actionCalls"`
	RecordLookups         []flowRecordLookupXML        `xml:"recordLookups"`
	RecordCreates         []flowRecordCreateXML        `xml:"recordCreates"`
	RecordDeletes         []flowRecordDeleteXML        `xml:"recordDeletes"`
	CustomErrors          []flowCustomErrorXML         `xml:"customErrors"`
	ApexPluginCalls       []flowApexPluginCallXML      `xml:"apexPluginCalls"`
	Transforms            []flowTransformXML           `xml:"transforms"`
	CollectionProcessors  []flowCollectionProcessorXML `xml:"collectionProcessors"`
	OrchestratedStages    []flowNamedNodeXML           `xml:"orchestratedStages"`
	ScheduledPaths        []flowScheduledPathXML       `xml:"scheduledPaths"`
	Stages                []flowNamedNodeXML           `xml:"stages"`
	Environments          string                       `xml:"environments"`
	ExitRules             []flowNamedNodeXML           `xml:"exitRules"`
	Experiments           []flowNamedNodeXML           `xml:"experiments"`
	Segment               []flowNamedNodeXML           `xml:"segment"`
	Schedule              []flowNamedNodeXML           `xml:"schedule"`
	Waits                 []flowWaitXML                `xml:"waits"`
	Variables             []flowVariableXML            `xml:"variables"`
}

type flowStartXML struct {
	DoesRequireRecordChangedToMeetCriteria bool                   `xml:"doesRequireRecordChangedToMeetCriteria"`
	RecordTriggerType                      string                 `xml:"recordTriggerType"`
	Object                                 string                 `xml:"object"`
	TriggerType                            string                 `xml:"triggerType"`
	Filters                                []flowFilterXML        `xml:"filters"`
	FilterLogic                            string                 `xml:"filterLogic"`
	FilterFormula                          string                 `xml:"filterFormula"`
	Connector                              flowConnectorXML       `xml:"connector"`
	FaultConnector                         flowConnectorXML       `xml:"faultConnector"`
	ScheduledPaths                         []flowScheduledPathXML `xml:"scheduledPaths"`
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
	Name                             string                `xml:"name"`
	Label                            string                `xml:"label"`
	Object                           string                `xml:"object"`
	SortField                        string                `xml:"sortField"`
	SortOrder                        string                `xml:"sortOrder"`
	Filters                          []flowFilterXML       `xml:"filters"`
	FilterLogic                      string                `xml:"filterLogic"`
	Connector                        flowConnectorXML      `xml:"connector"`
	FaultConnector                   flowConnectorXML      `xml:"faultConnector"`
	GetFirstRecordOnly               bool                  `xml:"getFirstRecordOnly"`
	StoreOutputAutomatically         bool                  `xml:"storeOutputAutomatically"`
	AssignNullValuesIfNoRecordsFound bool                  `xml:"assignNullValuesIfNoRecordsFound"`
	OutputReference                  string                `xml:"outputReference"`
	QueriedFields                    []string              `xml:"queriedFields"`
	OutputAssignments                []flowLookupOutputXML `xml:"outputAssignments"`
}

type flowLookupOutputXML struct {
	AssignToReference string `xml:"assignToReference"`
	Field             string `xml:"field"`
}

type flowRecordCreateXML struct {
	Name                      string                   `xml:"name"`
	Label                     string                   `xml:"label"`
	Object                    string                   `xml:"object"`
	InputReference            string                   `xml:"inputReference"`
	AssignRecordIDToReference string                   `xml:"assignRecordIdToReference"`
	Fields                    []flowFieldAssignmentXML `xml:"inputAssignments"`
	Connector                 flowConnectorXML         `xml:"connector"`
	FaultConnector            flowConnectorXML         `xml:"faultConnector"`
	StoreOutputAutomatically  bool                     `xml:"storeOutputAutomatically"`
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
	OutputAssignments []flowSubflowOutputXML   `xml:"outputAssignments"`
	Connector         flowConnectorXML         `xml:"connector"`
	FaultConnector    flowConnectorXML         `xml:"faultConnector"`
}

type flowFieldAssignmentXML struct {
	Name  string       `xml:"name"`
	Field string       `xml:"field"`
	Value flowValueXML `xml:"value"`
}

type flowNamedNodeXML struct {
	Name string `xml:"name"`
}

type flowWaitXML struct {
	Name           string `xml:"name"`
	ElementSubtype string `xml:"elementSubtype"`
}

type flowLoopXML struct {
	Name                       string           `xml:"name"`
	CollectionReference        string           `xml:"collectionReference"`
	IterationOrder             string           `xml:"iterationOrder"`
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
	// OutputParameters assign returned fields to variables.
	OutputParameters []flowActionOutputXML `xml:"outputParameters"`
	// StoreOutputAutomatically exposes a single-output action result under
	// the action's own name as an elementReference.
	StoreOutputAutomatically bool `xml:"storeOutputAutomatically"`
}

type flowActionOutputXML struct {
	Name              string `xml:"name"`
	AssignToReference string `xml:"assignToReference"`
}

type flowActionInputXML struct {
	Name  string       `xml:"name"`
	Value flowValueXML `xml:"value"`
}

type flowVariableXML struct {
	Name         string       `xml:"name"`
	Description  string       `xml:"description"`
	DataType     string       `xml:"dataType"`
	ObjectType   string       `xml:"objectType"`
	ApexClass    string       `xml:"apexClass"`
	IsCollection bool         `xml:"isCollection"`
	IsInput      bool         `xml:"isInput"`
	IsOutput     bool         `xml:"isOutput"`
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
	Message        string `xml:"message"`
	ErrorMessage   string `xml:"errorMessage"`
	FieldSelection string `xml:"fieldSelection"`
	IsFieldError   bool   `xml:"isFieldError"`
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
	DataType         string                   `xml:"dataType"`
	IsCollection     string                   `xml:"isCollection"`
	ObjectType       string                   `xml:"objectType"`
	TransformValues  []flowTransformValuesXML `xml:"transformValues"`
}

type flowTransformValuesXML struct {
	Actions []flowTransformValueActionXML `xml:"transformValueActions"`
}

type flowTransformValueActionXML struct {
	OutputFieldAPIName string       `xml:"outputFieldApiName"`
	TransformType      string       `xml:"transformType"`
	Value              flowValueXML `xml:"value"`
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
	EmptyStringValue bool   `xml:"-"`
	BooleanValue     string `xml:"booleanValue"`
	NumberValue      string `xml:"numberValue"`
	DateValue        string `xml:"dateValue"`
	DateTimeValue    string `xml:"dateTimeValue"`
	ElementReference string `xml:"elementReference"`
	Formula          string `xml:"formula"`
	Expression       string `xml:"expression"`
}

func loadFlow(path string, definitions ...map[string]storage.ObjectDefinition) (Flow, []diagnostic.Diagnostic, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return Flow{}, nil, err
	}
	data, processBuilderSkip := normalizeProcessBuilderFlow(source, definitions...)
	flow, diagnostics, err := compileFlowSource(path, data, processBuilderSkip, definitions...)
	if err != nil || processBuilderSkip != "" || bytes.Equal(data, source) {
		return flow, diagnostics, err
	}
	// A process runs whole or not at all: a part the graph cannot model leaves
	// it unrun, as an uncaptured shape does.
	part := processBuilderUnmodeledPart(flow, diagnostics)
	if part == "" {
		part = processBuilderInlineFormulaOperands(flow, data)
	}
	if part != "" {
		return compileFlowSource(path, source, part, definitions...)
	}
	return flow, diagnostics, nil
}

// processBuilderInlineFormulaOperands moves a criterion whose right operand is
// a process formula into its branch formula, so the operand is evaluated with
// the record (cgl dayafter and dayon rows). It names any such criterion it
// cannot move.
func processBuilderInlineFormulaOperands(flow Flow, data []byte) string {
	var raw flowXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return "source"
	}
	formulas := flowFormulaMapWithConstants(raw.Formulas, raw.Constants)
	operators := map[string]string{"equals": "=", "notequal": "<>", "greaterthan": ">", "greaterthanorequalto": ">=", "lessthan": "<", "lessthanorequalto": "<="}
	unmoved := ""
	var steps func([]storage.FlowStep)
	branch := func(item *storage.FlowBranch) {
		kept := item.Criteria[:0:0]
		for _, criterion := range item.Criteria {
			reference, isReference := strings.CutPrefix(strings.TrimSpace(criterion.Value), "{!")
			reference = strings.TrimSuffix(reference, "}")
			expression, isFormula := formulas[strings.ToLower(reference)]
			if !isReference || !isFormula {
				kept = append(kept, criterion)
				continue
			}
			operator, known := operators[strings.ToLower(strings.TrimSpace(criterion.Operation))]
			if !known || strings.Contains(criterion.Field, "$") {
				unmoved = fmt.Sprintf("criterion on %q", criterion.Field)
				continue
			}
			moved := criterion.Field + " " + operator + " (" + expression + ")"
			if current := strings.TrimSpace(item.Formula); current != "" {
				moved = "AND(" + current + ", " + moved + ")"
			}
			item.Formula = moved
		}
		item.Criteria = kept
		steps(item.Steps)
	}
	steps = func(list []storage.FlowStep) {
		for i := range list {
			for j := range list[i].Branches {
				branch(&list[i].Branches[j])
			}
		}
	}
	for i := range flow.Rules {
		steps(flow.Rules[i].Steps)
		for j := range flow.Rules[i].Branches {
			branch(&flow.Rules[i].Branches[j])
		}
		for _, list := range flow.Rules[i].StepTargets {
			steps(list)
		}
	}
	return unmoved
}

func processBuilderUnmodeledPart(flow Flow, diagnostics []diagnostic.Diagnostic) string {
	for _, item := range diagnostics {
		if item.Code == "GLADEAUTO002" {
			_, message, found := strings.Cut(item.Message, ": ")
			if !found {
				message = item.Message
			}
			return message
		}
	}
	for _, rule := range flow.Rules {
		if part := processBuilderStepsUnmodeled(rule.Steps); part != "" {
			return part
		}
		for _, branch := range rule.Branches {
			if part := processBuilderStepsUnmodeled(branch.Steps); part != "" {
				return part
			}
		}
		for _, steps := range rule.StepTargets {
			if part := processBuilderStepsUnmodeled(steps); part != "" {
				return part
			}
		}
	}
	return ""
}

func processBuilderStepsUnmodeled(steps []storage.FlowStep) string {
	for _, step := range steps {
		switch step.Kind {
		case "reference", "recordUpdate", "action", "assignment":
		case "decision":
			for _, branch := range step.Branches {
				if part := processBuilderStepsUnmodeled(branch.Steps); part != "" {
					return part
				}
			}
		default:
			return fmt.Sprintf("step %q", step.Kind)
		}
	}
	return ""
}

func compileFlowSource(path string, data []byte, processBuilderSkip string, definitions ...map[string]storage.ObjectDefinition) (Flow, []diagnostic.Diagnostic, error) {
	var raw flowXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return Flow{}, nil, err
	}
	name := flowFileName(path)
	objectName := strings.TrimSpace(raw.Start.Object)
	flow := Flow{ObjectName: objectName, File: path}
	if declaration, ok := screenFlowDeclaration(raw, path); ok {
		flow.Declaration = &declaration
	}
	diagnostics := make([]diagnostic.Diagnostic, 0)
	// DML-triggered flows must be active and identify an object. Autolaunched
	// interview flows have no start object and can be invoked by Apex even when
	// the source metadata carries a Draft or Obsolete status.
	if (!flowActive(raw.Status) && objectName != "") || flowProcessTypeNonDML(raw.ProcessType) {
		return flow, diagnostics, nil
	}
	var sourceDefinitions []storage.ObjectDefinition
	if len(definitions) > 0 {
		for name, definition := range definitions[0] {
			if strings.EqualFold(name, objectName) {
				sourceDefinitions = append(sourceDefinitions, definition)
				break
			}
		}
	}
	flowResolveStandardRelationshipUpdates(&raw, sourceDefinitions...)
	formulas := flowFormulaMapWithConstants(raw.Formulas, raw.Constants)
	variables := flowVariableMap(raw.Variables)
	flowProcessPriorValueSources(raw, variables)
	frameFormulas := flowFrameFormulaMap(raw, variables)
	raw.frameFormulas = frameFormulas
	raw.decisionFormulas = flowDecisionFormulaMap(raw, variables)
	orderedGraph := flowNormalizeTextDecisionNegation(&raw, variables)
	orderedGraph = orderedGraph || flowRequiresOrderedGraph(raw) || raw.decisionFormulas != nil || frameFormulas != nil
	if triggerType := strings.TrimSpace(raw.Start.TriggerType); strings.EqualFold(triggerType, "PlatformEvent") || triggerType == "" {
		// Preserve the event or started interview's current-user resource in
		// record inputs (flow_tolabel_residuals: an autolaunched Create Records
		// writes $User.Id to a User lookup, API 62.0 and 67.0). This binding
		// map is private; globals are not declared Flow variables.
		variables["$user"] = flowVariableXML{Name: "$User", DataType: "SObject", ObjectType: "User"}
	}
	if invocableFlowActionInvalidNullAssignment(raw, variables) {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADEAUTO004",
			File:     path,
			Message:  `field integrity exception: unknown (The assignment using the Equals operator uses a reference to "$GlobalConstant.Null", which doesn't exist.)`,
		})
		return flow, diagnostics, nil
	}
	if problem := flowUndeclaredCustomErrorReference(raw); problem != "" {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: path, Message: problem})
		return flow, diagnostics, nil
	}
	if problem := flowInvalidValueReference(raw, variables); problem != "" {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: path, Message: problem})
		return flow, diagnostics, nil
	}
	if len(definitions) > 0 {
		if problem := flowBeforeSaveFormulaFieldReference(raw, definitions[0]); problem != "" {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: path, Message: problem})
			return flow, diagnostics, nil
		}
	}
	constants := flowConstantMap(raw.Constants)
	diagnostics = flowUnsupportedNodeDiagnostics(path, name, raw, formulas, variables)
	if processBuilderSkip != "" {
		diagnostics = append(diagnostics, flowUnsupported(path, name, "Process Builder process is not run: "+processBuilderSkip+" has no native row"))
	}
	if !flowProcessTypeSupported(raw.ProcessType) {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("processType %q is not modeled as DML-triggered automation", raw.ProcessType)))
	}
	triggerType := strings.TrimSpace(raw.Start.TriggerType)
	if len(raw.Subflows) > 0 && (strings.EqualFold(triggerType, "RecordBeforeSave") || strings.EqualFold(triggerType, "PlatformEvent")) {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADEAUTO004",
			File:     path,
			Message:  fmt.Sprintf("Subflow elements are not allowed when triggerType is %q", triggerType),
		})
		return flow, diagnostics, nil
	}
	if triggerType != "" && !flowTriggerTypeDML(triggerType) && !strings.EqualFold(triggerType, "PlatformEvent") {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("triggerType %q is not supported for local test execution", triggerType)))
	}
	if len(raw.OrchestratedStages) > 0 {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("orchestration (%d stages) is not supported for local test execution", len(raw.OrchestratedStages))))
	}
	if len(raw.Stages) > 0 {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("stages are not supported for local test execution")))
	}
	if raw.Environments != "" && !strings.EqualFold(strings.TrimSpace(raw.Environments), "Default") {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("environments %q is not supported for local test execution", raw.Environments)))
	}
	rule := storage.FlowRule{
		APIVersion:                             strings.TrimSpace(raw.APIVersion),
		SourceStatus:                           strings.TrimSpace(raw.Status),
		Label:                                  strings.TrimSpace(raw.Label),
		Name:                                   name,
		File:                                   path,
		Active:                                 true,
		ProcessType:                            strings.TrimSpace(raw.ProcessType),
		TriggerType:                            strings.TrimSpace(raw.Start.TriggerType),
		RecordTriggerType:                      strings.TrimSpace(raw.Start.RecordTriggerType),
		DoesRequireRecordChangedToMeetCriteria: raw.Start.DoesRequireRecordChangedToMeetCriteria,
		TriggerOrder:                           raw.TriggerOrder,
		RunInMode:                              strings.TrimSpace(raw.RunInMode),
		TextTemplates:                          flowTextTemplateMap(raw.TextTemplates),
	}
	rule.Variables = flowDeclarationVariables(raw.Variables)
	if strings.EqualFold(raw.ProcessType, "AutoLaunchedFlow") && strings.TrimSpace(raw.Start.TriggerType) == "" && len(raw.Variables) == 1 && len(raw.Constants) == 1 && len(raw.ActionCalls) == 2 && len(raw.Assignments) == 1 && len(raw.RecordUpdates) == 1 && len(raw.RecordLookups) == 0 && len(raw.Decisions) == 0 && strings.EqualFold(raw.ActionCalls[0].ActionName, raw.ActionCalls[1].ActionName) {
		constant := raw.Constants[0]
		if strings.EqualFold(constant.DataType, "String") && strings.TrimSpace(constant.Name) != "" && constant.Value.StringValue != "" {
			rule.ProcessStringConstants = map[string]string{strings.ToLower(strings.TrimSpace(constant.Name)): constant.Value.StringValue}
		}
	}

	routedDecisions := false
	for _, filter := range raw.Start.Filters {
		rule.Criteria = append(rule.Criteria, storage.WorkflowCriteriaItem{
			Field:     trimObjectPrefix(strings.TrimSpace(filter.Field)),
			Operation: flowOperator(strings.TrimSpace(filter.Operator)),
			Value:     flowLiteralValue(filter.Value),
		})
	}
	if formula := normalizeFlowFormula(raw.Start.FilterFormula); formula != "" {
		rule.Formula = formula
		rule.Criteria = nil
	} else if formula := flowCoreEntryFormula(raw.Start, formulas, variables); formula != "" {
		rule.Formula = formula
		rule.CoreEntryFilterLogic = true
		rule.Criteria = nil
	} else if groups, ok := flowStartCriteriaGroups(raw.Start, formulas); ok {
		if flowHasFilteredTriggerRecordUpdate(raw) {
			// The flat triggering-record update projection does not retain its
			// filters. Keep the earlier entry behavior until that shape is modeled.
			diagnostics = append(diagnostics, flowUnsupported(path, name, "start filter logic with a filtered triggering-record update is not supported; entry conditions are combined with AND"))
		} else {
			// Numbered and "or" logic keep every filter's operator and combine the
			// per-filter results as alternatives at entry (API 62.0, 67.0).
			rule.CriteriaGroups = groups
		}
	} else if formula := flowCriteriaFormula(raw.Start.FilterLogic, formulas); formula != "" {
		rule.Formula = formula
		rule.Criteria = nil
	} else if logic := strings.TrimSpace(raw.Start.FilterLogic); logic != "" && !strings.EqualFold(logic, "and") {
		diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("start filter logic %q is not supported; entry conditions are combined with AND", logic)))
	}
	if len(raw.Start.ScheduledPaths) > 0 || len(raw.ScheduledPaths) > 0 {
		// Each entry owns its connector graph. The legacy flat projection below
		// would merge deferred effects and decision conditions into the save path.
		diagnostics = append(diagnostics, populateFlowPaths(&rule, raw, formulas, variables)...)
		flow.Rules = append(flow.Rules, rule)
		return flow, diagnostics, nil
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
					field = flowSetupFieldReference(condition.LeftValueReference)
				}
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
	if routedDecisions && !orderedGraph && flowRoutedDecisionNeedsGraph(raw) {
		orderedGraph = true
	}
	for _, assignment := range raw.Assignments {
		frameOrOrdinary := flowAssignmentElementFormulas(raw, formulas, assignment)
		for _, item := range assignment.Items {
			if !flowAssignmentOperatorSupported(item.Operator) {
				diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("assignment %q operator %q is not supported", firstNonBlank(assignment.Name, assignment.Label), item.Operator)))
				continue
			}
			field := flowRecordFieldReference(item.AssignToReference)
			if invocableFlowActionRecordAssignment(item.AssignToReference, variables) {
				continue
			}
			if field == "" {
				continue
			}
			captured := strings.EqualFold(strings.TrimSpace(item.Operator), "Assign") && flowValueHasSelectedFormula(item.Value, flowAssignmentElementSelection(raw, assignment))
			itemFormulas := formulas
			if captured {
				itemFormulas = frameOrOrdinary
			}
			formula := flowNullTextLiteralFormula(item.Value, flowExpressionValue(item.Value, itemFormulas))
			rule.FieldUpdates = append(rule.FieldUpdates, storage.WorkflowFieldUpdate{
				Name:             firstNonBlank(assignment.Name, assignment.Label, field),
				Field:            field,
				LiteralValue:     flowLiteralValue(item.Value),
				Formula:          formula,
				FlowFrameFormula: formula != "" && captured,
				SourceField:      flowLookupSourceFieldValueUnlessFormula(item.Value, formula, variables, raw.RecordLookups),
			})
		}
	}
	for _, update := range raw.RecordUpdates {
		if strings.TrimSpace(update.Object) != "" {
			continue
		}
		if !flowUpdatesTriggeringRecord(update.InputReference) {
			if _, ok := modeledFlowRecordUpdate(update, formulas, variables, flowConstantMap(raw.Constants), raw.RecordLookups, strings.EqualFold(raw.ProcessType, "Workflow")); !ok && !orderedGraph {
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
			formula := flowExpressionValue(assignment.Value, formulas)
			rule.FieldUpdates = append(rule.FieldUpdates, storage.WorkflowFieldUpdate{
				Name:         firstNonBlank(update.Name, update.Label, field),
				Field:        field,
				LiteralValue: flowLiteralValue(assignment.Value),
				Formula:      formula,
				SourceField:  flowLookupSourceFieldValueUnlessFormula(assignment.Value, formula, variables, raw.RecordLookups),
			})
		}
	}
	for _, lookup := range raw.RecordLookups {
		recordLookup, ok := modeledFlowRecordLookup(lookup, variables, flowConstantMap(raw.Constants), raw.RecordLookups)
		if !ok {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("record lookup node %q is not modeled", firstNonBlank(lookup.Name, lookup.Label))))
			continue
		}
		rule.RecordLookups = append(rule.RecordLookups, recordLookup)
	}
	for _, create := range raw.RecordCreates {
		recordCreate, ok := modeledFlowCapturedRecordCreate(raw, create, formulas, variables)
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
		recordDelete, ok := modeledFlowRecordDelete(del, variables, flowConstantMap(raw.Constants), raw.RecordLookups)
		if !ok {
			diagnostics = append(diagnostics, flowUnsupported(path, name, fmt.Sprintf("record delete node %q is not modeled", firstNonBlank(del.Name, del.Label))))
			continue
		}
		rule.RecordDeletes = append(rule.RecordDeletes, recordDelete)
	}
	for _, customError := range raw.CustomErrors {
		ce := modeledFlowCustomError(customError, raw.Constants, flowCustomErrorFrameFormulas(raw), formulas)
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
	raw.graph = newFlowTargetGraph(raw)
	if raw.graph == nil {
		raw.graph = newLegacyPathFlowTargetGraph(raw)
	}
	if !routedDecisions || orderedGraph {
		target := strings.TrimSpace(raw.Start.Connector.TargetReference)
		if target == "" {
			target = flowCoreScalarAssignmentStart(raw, variables)
		}
		if target == "" {
			target = invocableFlowActionRecordAssignmentStart(raw, variables)
		}
		if target != "" {
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
	if raw.graph != nil && len(raw.graph.targets) > 0 {
		rule.StepTargets = raw.graph.targets
	}
	if len(rule.Branches) > 0 || len(rule.Steps) > 0 || len(rule.FieldUpdates) > 0 || len(rule.Actions) > 0 || len(rule.RecordLookups) > 0 || len(rule.RecordCreates) > 0 || len(rule.RecordDeletes) > 0 || len(rule.CustomErrors) > 0 || len(rule.ApexPlugins) > 0 {
		flow.Rules = append(flow.Rules, rule)
	}
	sort.Slice(flow.Rules, func(i, j int) bool { return flow.Rules[i].Name < flow.Rules[j].Name })
	return flow, diagnostics, nil
}

func flowFileName(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), ".flow-meta.xml")
	return strings.TrimSuffix(name, ".flow")
}

// Preserve a hierarchy-setting field reference as a single Flow operand.
func flowSetupFieldReference(reference string) string {
	reference = strings.TrimSpace(reference)
	parts := strings.Split(reference, ".")
	if len(parts) != 3 || parts[0] != "$Setup" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		return ""
	}
	return reference
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
			if flowSetupFieldReference(conditions[idx].LeftValueReference) != "" && firstNonBlank(conditions[idx].RightValue.ElementReference, conditions[idx].RightValue.Formula, conditions[idx].RightValue.Expression) != "" {
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
	right := formulaQuote(value)
	if flowSetupFieldReference(condition.LeftValueReference) != "" && firstNonBlank(condition.RightValue.Formula, condition.RightValue.Expression) != "" {
		return ""
	}
	if flowSetupFieldReference(condition.LeftValueReference) != "" && strings.TrimSpace(condition.RightValue.ElementReference) != "" {
		op := strings.ToLower(strings.TrimSpace(condition.Operator))
		if op != "equalto" && op != "equals" {
			return ""
		}
		right = flowRecordFieldReference(condition.RightValue.ElementReference)
		if right == "" {
			return ""
		}
	}
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(condition.Operator), " ", "")) {
	case "contains":
		return fmt.Sprintf("CONTAINS(%s,%s)", left, right)
	case "isnull", "isblank":
		if value == "" {
			value = "true"
		}
		return fmt.Sprintf("ISBLANK(%s) = %s", left, formulaQuote(value))
	default:
		return fmt.Sprintf("%s %s %s", left, conditionOperatorToFormula(condition.Operator), right)
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
	if field := flowSetupFieldReference(reference); field != "" {
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
		case (ch >= '0' && ch <= '9') || ((ch == '-' || ch == '+') && i+1 < len(logic) && logic[i+1] >= '0' && logic[i+1] <= '9'):
			start := i
			if ch == '-' || ch == '+' {
				i++
			}
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
	// Signed tokens help identify malformed numbered logic at the formula
	// fallback, but condition indexes themselves must remain unsigned.
	if token[0] == '-' || token[0] == '+' {
		return nil, false
	}
	index, err := strconv.Atoi(token)
	if err != nil || index < 1 || index > p.conditionCount {
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
	return flowFormulaMapWithConstants(formulas, nil)
}

// flowFormulaMapWithConstants also inlines Number, Boolean and String
// constants. A constant cannot change, so its literal is exact.
func flowFormulaMapWithConstants(formulas []flowFormulaXML, constants []flowConstantXML) map[string]string {
	return flowFormulaMapWithNormalization(formulas, constants, normalizeFlowFormula)
}

func flowFormulaMapWithNormalization(formulas []flowFormulaXML, constants []flowConstantXML, normalize func(string) string) map[string]string {
	declared := make(map[string]string, len(formulas))
	for _, formula := range formulas {
		name, expression := strings.TrimSpace(formula.Name), strings.TrimSpace(formula.Expression)
		if name != "" && expression != "" {
			declared[strings.ToLower(name)] = expression
		}
	}
	expandable := make(map[string]string, len(declared)+len(constants))
	for _, constant := range constants {
		name := strings.ToLower(strings.TrimSpace(constant.Name))
		if _, formula := declared[name]; name == "" || formula {
			continue
		}
		if literal, ok := flowConstantFormulaLiteral(constant.Value); ok {
			expandable[name] = literal
		}
	}
	for name, expression := range declared {
		expandable[name] = expression
	}
	out := make(map[string]string, len(declared))
	for name, expression := range declared {
		out[name] = normalize(expandFlowFormulaReferences(expression, expandable, map[string]bool{name: true}))
	}
	return out
}

// Preserve record qualification when a formula can also address a scalar in
// the interview. The framed step graph keeps these bindings through execution.
func flowFrameFormulaMap(raw flowXML, variables map[string]flowVariableXML) map[string]string {
	if strings.TrimSpace(raw.Start.Object) == "" || !strings.EqualFold(raw.Start.RecordTriggerType, "Create") ||
		(!strings.EqualFold(raw.Start.TriggerType, "RecordBeforeSave") && !strings.EqualFold(raw.Start.TriggerType, "RecordAfterSave")) {
		return nil
	}
	scalars := make(map[string]bool)
	for name, variable := range variables {
		if !variable.IsCollection && strings.EqualFold(variable.DataType, "String") {
			scalars[strings.ToLower(name)] = true
		}
	}
	if len(scalars) == 0 {
		return nil
	}
	formulas := flowFormulaMapWithNormalization(raw.Formulas, raw.Constants, func(expression string) string {
		normalized, found := normalizeFlowDecisionFormula(expression, scalars)
		if found {
			return normalized
		}
		return normalizeFlowFormula(expression)
	})
	selected := make(map[string]string)
	selectElement := func(names []string) {
		if !flowFormulaNamesUseBareScalar(names, formulas, scalars) {
			return
		}
		for _, name := range names {
			name = strings.ToLower(strings.TrimSpace(name))
			if expression, exists := formulas[name]; exists {
				if _, found := normalizeFlowDecisionFormula(expression, scalars); found {
					selected[name] = expression
				}
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordBeforeSave") {
		for _, assignment := range raw.Assignments {
			selectElement(flowCapturedAssignmentNames(assignment, variables))
		}
	}
	if strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordAfterSave") {
		for _, create := range raw.RecordCreates {
			var names []string
			for _, field := range create.Fields {
				names = append(names, flowFormulaValueNames(field.Value)...)
			}
			selectElement(names)
		}
	}
	if strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordBeforeSave") {
		for _, customError := range raw.CustomErrors {
			for _, message := range customError.Messages {
				text := strings.ToLower(firstNonBlank(message.ErrorMessage, message.Message))
				var names []string
				for name := range formulas {
					if strings.Contains(text, "{!"+name+"}") {
						names = append(names, name)
					}
				}
				selectElement(names)
			}
		}
	}
	if len(selected) != 0 {
		return selected
	}
	return nil
}

func flowCustomErrorFrameFormulas(raw flowXML) map[string]string {
	if !strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordBeforeSave") {
		return nil
	}
	return raw.frameFormulas
}

func flowFormulaValueNames(value flowValueXML) []string {
	return []string{value.ElementReference, value.Formula, value.Expression}
}

func flowFormulaNamesUseBareScalar(names []string, formulas map[string]string, scalars map[string]bool) bool {
	for _, name := range names {
		if _, found := normalizeFlowDecisionFormulaMode(formulas[strings.ToLower(strings.TrimSpace(name))], scalars, true); found {
			return true
		}
	}
	return false
}

func flowElementFormulaSelection(raw flowXML, names []string) map[string]string {
	if len(raw.frameFormulas) == 0 {
		return nil
	}
	scalars := make(map[string]bool)
	for name, variable := range flowVariableMap(raw.Variables) {
		if !variable.IsCollection && strings.EqualFold(variable.DataType, "String") {
			scalars[strings.ToLower(name)] = true
		}
	}
	if !flowFormulaNamesUseBareScalar(names, raw.frameFormulas, scalars) {
		return nil
	}
	selected := make(map[string]string)
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if expression, ok := raw.frameFormulas[name]; ok {
			selected[name] = expression
		}
	}
	return selected
}

func flowElementFormulaMap(raw flowXML, ordinary map[string]string, names []string) map[string]string {
	selected := flowElementFormulaSelection(raw, names)
	if len(selected) == 0 {
		return ordinary
	}
	return flowOverlayFormulaMap(ordinary, selected)
}

func flowValueHasSelectedFormula(value flowValueXML, selected map[string]string) bool {
	for _, name := range flowFormulaValueNames(value) {
		if _, ok := selected[strings.ToLower(strings.TrimSpace(name))]; ok {
			return true
		}
	}
	return false
}

// Select only the record-field assignment route observed in the native rows.
func flowCapturedAssignmentNames(assignment flowAssignmentXML, variables map[string]flowVariableXML) []string {
	var names []string
	for _, item := range assignment.Items {
		if !strings.EqualFold(strings.TrimSpace(item.Operator), "Assign") {
			continue
		}
		if _, ok := modeledFlowAssignmentUpdate(assignment, item, nil, variables, nil); ok {
			names = append(names, flowFormulaValueNames(item.Value)...)
		}
	}
	return names
}

func flowAssignmentElementSelection(raw flowXML, assignment flowAssignmentXML) map[string]string {
	if !strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordBeforeSave") {
		return nil
	}
	return flowElementFormulaSelection(raw, flowCapturedAssignmentNames(assignment, flowVariableMap(raw.Variables)))
}

func flowCreateElementSelection(raw flowXML, create flowRecordCreateXML) map[string]string {
	if !strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordAfterSave") {
		return nil
	}
	var names []string
	for _, field := range create.Fields {
		names = append(names, flowFormulaValueNames(field.Value)...)
	}
	return flowElementFormulaSelection(raw, names)
}

func flowAssignmentElementFormulas(raw flowXML, ordinary map[string]string, assignment flowAssignmentXML) map[string]string {
	selected := flowAssignmentElementSelection(raw, assignment)
	if len(selected) == 0 {
		return ordinary
	}
	return flowOverlayFormulaMap(ordinary, selected)
}

func flowCreateElementFormulas(raw flowXML, ordinary map[string]string, create flowRecordCreateXML) map[string]string {
	selected := flowCreateElementSelection(raw, create)
	if len(selected) == 0 {
		return ordinary
	}
	return flowOverlayFormulaMap(ordinary, selected)
}

// Retain ordinary normalization for every formula outside the selected set.
func flowOverlayFormulaMap(ordinary, selected map[string]string) map[string]string {
	merged := make(map[string]string, len(ordinary))
	for name, expression := range ordinary {
		merged[name] = expression
	}
	for name, expression := range selected {
		merged[name] = expression
	}
	return merged
}

// Decision resources that depend on scalars need an interview frame and distinct
// explicit record bindings. Keep ordinary normalization for other resources.
func flowDecisionFormulaMap(raw flowXML, variables map[string]flowVariableXML) map[string]string {
	scalars := make(map[string]bool)
	for name, variable := range variables {
		if !variable.IsCollection && !strings.EqualFold(variable.DataType, "SObject") {
			scalars[strings.ToLower(name)] = true
		}
	}
	if len(scalars) == 0 {
		return nil
	}
	candidate := false
	for _, formula := range raw.Formulas {
		if _, found := normalizeFlowDecisionFormula(formula.Expression, scalars); found {
			candidate = true
			break
		}
	}
	if !candidate {
		return nil
	}
	formulas := flowFormulaMapWithNormalization(raw.Formulas, raw.Constants, func(expression string) string {
		normalized, found := normalizeFlowDecisionFormula(expression, scalars)
		if !found {
			return normalizeFlowFormula(expression)
		}
		return normalized
	})
	for _, decision := range raw.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				name := strings.ToLower(strings.TrimSpace(condition.LeftValueReference))
				if _, needsFrame := normalizeFlowDecisionFormula(formulas[name], scalars); needsFrame {
					return formulas
				}
			}
		}
	}
	return nil
}

func normalizeFlowDecisionFormula(expression string, scalars map[string]bool) (string, bool) {
	return normalizeFlowDecisionFormulaMode(expression, scalars, false)
}

func normalizeFlowDecisionFormulaMode(expression string, scalars map[string]bool, bareOnly bool) (string, bool) {
	needsFrame := false
	normalizeReference := func(reference string) string {
		reference = strings.TrimSpace(reference)
		for _, prefix := range []string{"$Record.", "Record."} {
			if len(reference) <= len(prefix) || !strings.EqualFold(reference[:len(prefix)], prefix) {
				continue
			}
			field := reference[len(prefix):]
			name, _, _ := strings.Cut(field, ".")
			if scalars[strings.ToLower(name)] {
				if !bareOnly {
					needsFrame = true
				}
				return "$Record." + field
			}
			return field
		}
		if scalars[strings.ToLower(reference)] {
			needsFrame = true
		}
		return reference
	}
	var out strings.Builder
	var quote byte
	for i := 0; i < len(expression); {
		character := expression[i]
		if quote != 0 {
			out.WriteByte(character)
			i++
			if character == '\\' && i < len(expression) {
				out.WriteByte(expression[i])
				i++
			} else if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			out.WriteByte(character)
			i++
			continue
		}
		if strings.HasPrefix(expression[i:], "{!") {
			if end := strings.IndexByte(expression[i+2:], '}'); end >= 0 {
				end += i + 2
				out.WriteString(normalizeReference(expression[i+2 : end]))
				i = end + 1
				continue
			}
		}
		if character == '$' || character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' {
			end := i + 1
			for end < len(expression) {
				next := expression[end]
				if next != '$' && next != '_' && next != '.' && !(next >= 'a' && next <= 'z') && !(next >= 'A' && next <= 'Z') && !(next >= '0' && next <= '9') {
					break
				}
				end++
			}
			out.WriteString(normalizeReference(expression[i:end]))
			i = end
			continue
		}
		out.WriteByte(character)
		i++
	}
	return strings.TrimSpace(out.String()), needsFrame
}

func flowConstantFormulaLiteral(value flowValueXML) (string, bool) {
	switch {
	case strings.TrimSpace(value.NumberValue) != "":
		number, err := strconv.ParseFloat(strings.TrimSpace(value.NumberValue), 64)
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return "", false
		}
		return strconv.FormatFloat(number, 'f', -1, 64), true
	case strings.EqualFold(strings.TrimSpace(value.BooleanValue), "true"):
		return "TRUE", true
	case strings.EqualFold(strings.TrimSpace(value.BooleanValue), "false"):
		return "FALSE", true
	case value.StringValue != "" && !strings.ContainsAny(value.StringValue, "\\\"{}"):
		return `"` + value.StringValue + `"`, true
	}
	return "", false
}

// flowResourceFormulaFunction equals dml.FlowResourceFormulaCall: the internal
// marker keeps a user-written GLADEFLOWRESOURCE() an unknown function
// (TestFlowResourceFormulaFunctionName keeps them together).
const flowResourceFormulaFunction = "\x1fGLADEFLOWRESOURCE"

var flowBRCall = regexp.MustCompile(`(?i)\bBR\s*\(\s*\)`)

// Expand only references to declared formula resources, before record merge
// syntax is normalized. String literals and cyclic references remain intact.
func expandFlowFormulaReferences(expression string, declared map[string]string, active map[string]bool) string {
	remaining := 64 << 10
	return expandFlowFormulaReferencesBounded(expression, declared, active, &remaining, 0)
}

func expandFlowFormulaReferencesBounded(expression string, declared map[string]string, active map[string]bool, remaining *int, depth int) string {
	if depth >= 32 || *remaining <= 0 {
		return expression
	}
	var out strings.Builder
	var quote byte
	for i := 0; i < len(expression); {
		if *remaining <= 0 || out.Len() > 64<<10 {
			return expression
		}
		*remaining -= 1
		character := expression[i]
		if quote != 0 {
			out.WriteByte(character)
			i++
			if character == '\\' && i < len(expression) {
				out.WriteByte(expression[i])
				i++
			} else if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			out.WriteByte(character)
			i++
			continue
		}
		if strings.HasPrefix(expression[i:], "{!") {
			if end := strings.IndexByte(expression[i+2:], '}'); end >= 0 {
				end += i + 2
				name := strings.ToLower(strings.TrimSpace(expression[i+2 : end]))
				if name == "$flow.currentdate" {
					out.WriteString("TODAY()")
					i = end + 1
					continue
				}
				if referenced, exists := declared[name]; exists && !active[name] {
					active[name] = true
					expanded := expandFlowFormulaReferencesBounded(referenced, declared, active, remaining, depth+1)
					delete(active, name)
					if len(expanded)+2 > *remaining || out.Len()+len(expanded)+2 > 64<<10 {
						return expression
					}
					*remaining -= len(expanded) + 2
					if flowBRCall.MatchString(expanded) {
						// The referenced resource's finished value feeds this
						// formula; BR() is a line feed by then (flow_br_surfaces).
						out.WriteString(flowResourceFormulaFunction + "(" + expanded + ")")
					} else {
						out.WriteString("(" + expanded + ")")
					}
					i = end + 1
					continue
				}
				out.WriteString(expression[i : end+1])
				i = end + 1
				continue
			}
		}
		out.WriteByte(character)
		i++
	}
	return out.String()
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

// The compact field-update projection cannot carry Update Records filters.
// Do not broaden entry for that shape until its filter semantics are modeled.
func flowHasFilteredTriggerRecordUpdate(raw flowXML) bool {
	for _, update := range raw.RecordUpdates {
		if strings.TrimSpace(update.Object) == "" && flowUpdatesTriggeringRecord(update.InputReference) && len(update.Filters) > 0 {
			return true
		}
	}
	return false
}

// flowStartCriteriaGroups parses start filterLogic that is "or" or numbered
// logic over the start filters. A logic string naming a flow formula is not
// numbered logic.
func flowStartCriteriaGroups(start flowStartXML, formulas map[string]string) ([][]int, bool) {
	logic := strings.TrimSpace(start.FilterLogic)
	if logic == "" || strings.EqualFold(logic, "and") || len(start.Filters) == 0 {
		return nil, false
	}
	if _, named := formulas[strings.ToLower(logic)]; named {
		return nil, false
	}
	return flowConditionLogicGroups(logic, len(start.Filters))
}

func flowCriteriaFormula(filterLogic string, formulas map[string]string) string {
	filterLogic = strings.TrimSpace(filterLogic)
	if filterLogic == "" {
		return ""
	}
	if expression, ok := formulas[strings.ToLower(filterLogic)]; ok {
		return expression
	}
	// Malformed numbered logic must retain the unsupported-logic warning,
	// rather than becoming a formula merely because it contains parentheses.
	tokens := flowConditionLogicTokens(filterLogic)
	numberedLogic := len(tokens) > 0
	for _, token := range tokens {
		switch token {
		case "(", ")", "AND", "OR", "NOT", "XOR":
			continue
		}
		if (token[0] >= '0' && token[0] <= '9') || token[0] == '-' || token[0] == '+' {
			continue
		}
		// A field/resource or function name retains the legacy formula path.
		numberedLogic = false
		break
	}
	if numberedLogic {
		return ""
	}
	normalized := normalizeFlowFormula(filterLogic)
	if strings.ContainsAny(normalized, "=!<>&|()") {
		return normalized
	}
	return ""
}

// flowDecisionRightValue keeps an element reference as {!reference} so the
// Invocable.Action interview can read it from its frame.
func flowDecisionRightValue(value flowValueXML) string {
	if literal := flowLiteralValue(value); literal != "" {
		return literal
	}
	if reference := strings.TrimSpace(value.ElementReference); reference != "" {
		return "{!" + reference + "}"
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
	if reference == "$Flow.CurrentDateTime" {
		return "NOW()"
	}
	return ""
}

func flowSourceFieldValue(value flowValueXML, variables map[string]flowVariableXML) string {
	reference := strings.TrimSpace(value.ElementReference)
	if reference == "" {
		return ""
	}
	if name, _, nested := strings.Cut(reference, "."); nested {
		if variable, ok := variables[strings.ToLower(name)]; ok && !variable.IsCollection && strings.EqualFold(variable.DataType, "SObject") {
			return reference
		}
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

func flowSourceFieldValueUnlessFormula(value flowValueXML, formula string, variables map[string]flowVariableXML) string {
	if strings.TrimSpace(formula) != "" {
		return ""
	}
	return flowSourceFieldValue(value, variables)
}

func flowLookupSourceFieldValueUnlessFormula(value flowValueXML, formula string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) string {
	if strings.TrimSpace(formula) != "" {
		return ""
	}
	return flowLookupSourceFieldValue(value, variables, lookups)
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

// Lookup criteria can compare a field with a scalar Flow variable. Preserve
// that variable reference so the runtime can resolve the value from the
// active interview frame instead of treating it as an empty literal.
func flowLookupCriteriaSourceFieldValue(value flowValueXML, variables map[string]flowVariableXML, constants map[string]flowValueXML, lookups []flowRecordLookupXML) string {
	reference := strings.TrimSpace(value.ElementReference)
	if reference != "" {
		if _, ok := constants[strings.ToLower(reference)]; ok {
			return ""
		}
	}
	if source := flowLookupSourceFieldValue(value, variables, lookups); source != "" {
		return source
	}
	if reference == "" {
		return ""
	}
	if _, ok := variables[strings.ToLower(reference)]; ok {
		return reference
	}
	return ""
}

func flowLookupFieldReference(reference string, lookups []flowRecordLookupXML) string {
	reference = strings.TrimSpace(reference)
	if relationship := flowLookupRelationshipReference(reference, lookups); relationship != "" {
		return relationship
	}
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

// flowLookupRelationshipReference keeps a relationship path read through a
// Get Records output (Output.Lookup.Field) as one source reference. The
// runtime resolves the remainder against the stored parent record.
func flowLookupRelationshipReference(reference string, lookups []flowRecordLookupXML) string {
	name, path, dotted := strings.Cut(reference, ".")
	if !dotted || !strings.Contains(path, ".") || strings.HasSuffix(path, ".") || strings.Contains(path, "..") {
		return ""
	}
	for _, lookup := range lookups {
		lookupName := firstNonBlank(lookup.Name, lookup.Label)
		if lookupName == "" || !lookup.StoreOutputAutomatically || !lookup.GetFirstRecordOnly {
			continue
		}
		if strings.EqualFold(name, lookupName) {
			return lookupName + "." + path
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

func modeledFlowRecordLookup(lookup flowRecordLookupXML, variables map[string]flowVariableXML, constants map[string]flowValueXML, lookups []flowRecordLookupXML) (storage.FlowRecordLookup, bool) {
	name := firstNonBlank(lookup.Name, lookup.Label)
	objectName := strings.TrimSpace(lookup.Object)
	if name == "" || objectName == "" {
		return storage.FlowRecordLookup{}, false
	}
	filterLogic, ok := flowLookupFilterLogic(lookup.FilterLogic, len(lookup.Filters))
	if !ok {
		return storage.FlowRecordLookup{}, false
	}
	out := storage.FlowRecordLookup{
		Name:                             name,
		ObjectName:                       objectName,
		SortField:                        strings.TrimSpace(lookup.SortField),
		SortOrder:                        strings.TrimSpace(lookup.SortOrder),
		CriteriaLogic:                    filterLogic,
		GetFirstRecordOnly:               lookup.GetFirstRecordOnly,
		StoreOutputAutomatically:         lookup.StoreOutputAutomatically,
		AssignNullValuesIfNoRecordsFound: lookup.AssignNullValuesIfNoRecordsFound,
		OutputReference:                  strings.TrimSpace(lookup.OutputReference),
		QueriedFields:                    append([]string(nil), lookup.QueriedFields...),
	}
	if variable, ok := variables[strings.ToLower(out.OutputReference)]; ok && !variable.IsCollection && strings.EqualFold(variable.DataType, "SObject") {
		out.GetFirstRecordOnly = true
	}
	for _, output := range lookup.OutputAssignments {
		out.OutputAssignments = append(out.OutputAssignments, storage.FlowAssignment{Target: strings.TrimSpace(output.AssignToReference), SourceField: strings.TrimSpace(output.Field)})
	}
	if len(out.OutputAssignments) > 0 {
		out.GetFirstRecordOnly = true
	}
	for _, filter := range lookup.Filters {
		field := trimObjectPrefix(strings.TrimSpace(filter.Field))
		if field == "" {
			return storage.FlowRecordLookup{}, false
		}
		out.Criteria = append(out.Criteria, storage.WorkflowCriteriaItem{
			Field:       field,
			Operation:   flowOperator(strings.TrimSpace(filter.Operator)),
			Value:       flowLiteralOrVariableOrConstantValue(filter.Value, variables, constants),
			SourceField: flowLookupCriteriaSourceFieldValue(filter.Value, variables, constants, lookups),
		})
	}
	return out, true
}

// Carry the selected formula resource on each create input through the stored graph.
func modeledFlowCapturedRecordCreate(raw flowXML, create flowRecordCreateXML, formulas map[string]string, variables map[string]flowVariableXML) (storage.FlowRecordCreate, bool) {
	result, ok := modeledFlowRecordCreate(create, flowCreateElementFormulas(raw, formulas, create), variables, raw.RecordLookups)
	if !ok {
		return result, false
	}
	selected := flowCreateElementSelection(raw, create)
	for i, field := range create.Fields {
		result.InputAssignments[i].FlowFrameFormula = result.InputAssignments[i].Formula != "" && flowValueHasSelectedFormula(field.Value, selected)
	}
	return result, true
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
	outputReference := strings.TrimSpace(create.AssignRecordIDToReference)
	if variable, ok := variables[strings.ToLower(outputReference)]; ok && !variable.IsCollection && strings.EqualFold(variable.DataType, "String") {
		out.AssignRecordIDToReference = outputReference
	}
	if name, field, dotted := strings.Cut(outputReference, "."); dotted && strings.EqualFold(field, "Id") {
		if variable, ok := variables[strings.ToLower(name)]; ok && !variable.IsCollection && strings.EqualFold(variable.DataType, "SObject") && strings.EqualFold(variable.ObjectType, objectName) {
			out.AssignRecordIDToReference = outputReference
		}
	}
	for _, assignment := range create.Fields {
		field := trimObjectPrefix(strings.TrimSpace(assignment.Field))
		if field == "" {
			return storage.FlowRecordCreate{}, false
		}
		formula := flowExpressionValue(assignment.Value, formulas)
		sourceField := flowLookupSourceFieldValueUnlessFormula(assignment.Value, formula, variables, lookups)
		if name, field, dotted := strings.Cut(strings.TrimSpace(assignment.Value.ElementReference), "."); formula == "" && sourceField == "" && dotted && strings.EqualFold(field, "Id") {
			if variable, declared := variables[strings.ToLower(name)]; declared && !variable.IsCollection && strings.EqualFold(variable.DataType, "SObject") {
				sourceField = strings.TrimSpace(assignment.Value.ElementReference)
			}
		}
		out.InputAssignments = append(out.InputAssignments, storage.WorkflowFieldUpdate{
			Name:         field,
			Field:        field,
			LiteralValue: flowLiteralOrVariableValue(assignment.Value, variables),
			Formula:      formula,
			SourceField:  sourceField,
		})
	}
	return out, true
}

func modeledFlowRecordDelete(del flowRecordDeleteXML, variables map[string]flowVariableXML, constants map[string]flowValueXML, lookups []flowRecordLookupXML) (storage.FlowRecordDelete, bool) {
	name := firstNonBlank(del.Name, del.Label)
	objectName := strings.TrimSpace(del.Object)
	inputReference := strings.TrimSpace(del.InputReference)
	if objectName == "" && inputReference != "" {
		if variable, ok := variables[strings.ToLower(inputReference)]; ok && strings.EqualFold(variable.DataType, "SObject") {
			objectName = strings.TrimSpace(variable.ObjectType)
		}
		if objectName == "" {
			for _, lookup := range lookups {
				if lookup.StoreOutputAutomatically && strings.EqualFold(inputReference, firstNonBlank(lookup.Name, lookup.Label)) {
					objectName = strings.TrimSpace(lookup.Object)
					break
				}
			}
		}
	}
	if name == "" || objectName == "" {
		return storage.FlowRecordDelete{}, false
	}
	if logic := strings.TrimSpace(del.FilterLogic); logic != "" && !strings.EqualFold(logic, "and") {
		return storage.FlowRecordDelete{}, false
	}
	out := storage.FlowRecordDelete{
		Name:           name,
		ObjectName:     objectName,
		InputReference: inputReference,
	}
	for _, filter := range del.Filters {
		field := trimObjectPrefix(strings.TrimSpace(filter.Field))
		if field == "" {
			return storage.FlowRecordDelete{}, false
		}
		out.Criteria = append(out.Criteria, storage.WorkflowCriteriaItem{
			Field:       field,
			Operation:   flowOperator(strings.TrimSpace(filter.Operator)),
			Value:       flowLiteralOrVariableOrConstantValue(filter.Value, variables, constants),
			SourceField: flowLookupCriteriaSourceFieldValue(filter.Value, variables, constants, lookups),
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
		field := trimObjectPrefix(firstNonBlank(input.Name, input.Field))
		if field == "" {
			continue
		}
		formula := flowExpressionValue(input.Value, formulas)
		out.InputAssignments = append(out.InputAssignments, storage.WorkflowFieldUpdate{
			Name:         field,
			Field:        field,
			LiteralValue: flowLiteralOrVariableValue(input.Value, variables),
			Formula:      formula,
			SourceField:  flowLookupSourceFieldValueUnlessFormula(input.Value, formula, variables, lookups),
		})
	}
	for _, output := range subflow.OutputAssignments {
		if strings.TrimSpace(output.Name) == "" || strings.TrimSpace(output.AssignToReference) == "" {
			continue
		}
		out.OutputAssignments = append(out.OutputAssignments, flowCoreSubflowOutput(output))
	}
	return out, true
}

func modeledFlowTransform(transform flowTransformXML, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowTransform, bool) {
	name := firstNonBlank(transform.Name, transform.Label)
	transformType := strings.TrimSpace(transform.TransformType)
	if name != "" && transformType == "" && len(transform.TransformValues) > 0 {
		return modeledFlowRecordTransform(name, transform)
	}
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

func modeledFlowRecordTransform(name string, transform flowTransformXML) (storage.FlowTransform, bool) {
	objectType := strings.TrimSpace(transform.ObjectType)
	if objectType == "" || !strings.EqualFold(strings.TrimSpace(transform.DataType), "SObject") ||
		strings.EqualFold(strings.TrimSpace(transform.IsCollection), "true") || len(transform.TransformValues) != 1 {
		return storage.FlowTransform{}, false
	}
	out := storage.FlowTransform{Name: name, TransformType: "RecordMap", ObjectType: objectType}
	seen := make(map[string]bool)
	for _, action := range transform.TransformValues[0].Actions {
		field := strings.TrimSpace(action.OutputFieldAPIName)
		reference := strings.TrimSpace(action.Value.ElementReference)
		literalString := action.Value.StringValue != "" || action.Value.EmptyStringValue
		key := strings.ToLower(field)
		if field == "" || seen[key] || (reference == "" && !literalString) || (reference != "" && literalString) || !strings.EqualFold(strings.TrimSpace(action.TransformType), "Map") {
			return storage.FlowTransform{}, false
		}
		seen[key] = true
		out.FieldMappings = append(out.FieldMappings, storage.WorkflowFieldUpdate{Field: field, SourceField: reference, LiteralValue: action.Value.StringValue, FlowLiteralString: literalString})
	}
	return out, len(out.FieldMappings) > 0
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
					if field == "" && (strings.EqualFold(strings.TrimSpace(condition.LeftValueReference), "$Profile.Name") || strings.EqualFold(strings.TrimSpace(condition.LeftValueReference), "$User.Id") || flowPermissionReference(condition.LeftValueReference)) {
						field = strings.TrimSpace(condition.LeftValueReference)
					}
					if field == "" {
						field = flowSetupFieldReference(condition.LeftValueReference)
					}
					if field == "" {
						diagnostics = append(diagnostics, flowUnsupported(path, flowName, fmt.Sprintf("decision %q branch %q condition %q is not a $Record field or modeled flow resource", decision.Name, branch.Name, condition.LeftValueReference)))
						modeled = false
						continue
					}
					item := storage.WorkflowCriteriaItem{
						Field:     field,
						Operation: flowOperator(strings.TrimSpace(condition.Operator)),
						Value:     flowLiteralValue(condition.RightValue),
					}
					if strings.EqualFold(field, "$User.Id") {
						item.Value = flowDecisionRightValue(condition.RightValue)
					}
					flowVariableEmptyOperand(&item, condition, variables)
					branch.Criteria = append(branch.Criteria, item)
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

func cloneFlowVisited(visited map[string]bool) map[string]bool {
	clone := make(map[string]bool, len(visited)+1)
	for key, value := range visited {
		if value {
			clone[key] = true
		}
	}
	return clone
}

func buildFlowFaultBranches(steps []storage.FlowStep, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML, visited map[string]bool) {
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
		faultVisited := cloneFlowVisited(visited)
		for name := range mainNames {
			faultVisited[name] = true
		}
		faultBranch, _ = flowPopulateBranchFromTargetWithStop(faultBranch, step.FaultTarget, "", raw, formulas, variables, faultVisited)
		if len(faultBranch.Steps) > 0 {
			step.FaultBranch = faultBranch.Steps
		}
	}
}

func flowPopulateBranchFromTarget(branch storage.FlowBranch, target string, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML) (storage.FlowBranch, []diagnostic.Diagnostic) {
	return flowPopulateBranchFromTargetWithStop(branch, target, "", raw, formulas, variables, make(map[string]bool))
}

func flowPopulateBranchFromTargetWithStop(branch storage.FlowBranch, target, stop string, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML, visited map[string]bool) (storage.FlowBranch, []diagnostic.Diagnostic) {
	if raw.graph != nil {
		if stop != "" {
			raw.graphStop = strings.ToLower(strings.TrimSpace(stop))
		}
		return raw.graph.populate(branch, target, raw, formulas, variables)
	}
	return flowPopulateBranchBody(branch, target, stop, raw, formulas, variables, visited)
}

func flowPopulateBranchBody(branch storage.FlowBranch, target, stop string, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML, visited map[string]bool) (storage.FlowBranch, []diagnostic.Diagnostic) {
	var diagnostics []diagnostic.Diagnostic
	for target = strings.TrimSpace(target); target != ""; {
		// A compact segment owns one source node. Keep its successor as an edge
		// so cloning and cache encoding never unfold shared execution paths.
		if raw.graph != nil && len(visited) > 0 {
			next, nextDiagnostics := raw.graph.populate(storage.FlowBranch{}, target, raw, formulas, variables)
			branch.Steps = append(branch.Steps, next.Steps...)
			return branch, append(diagnostics, nextDiagnostics...)
		}
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
			frameOrOrdinary := flowAssignmentElementFormulas(raw, formulas, assignment)
			for _, item := range assignment.Items {
				item = flowAssignmentItemConstantValue(item, variables, flowConstantMap(raw.Constants))
				captured := strings.EqualFold(strings.TrimSpace(item.Operator), "Assign") && flowValueHasSelectedFormula(item.Value, flowAssignmentElementSelection(raw, assignment))
				itemFormulas := formulas
				if captured {
					itemFormulas = frameOrOrdinary
				}
				if update, ok := modeledFlowAssignmentUpdate(assignment, item, itemFormulas, variables, raw.RecordLookups); ok {
					update.FlowFrameFormula = update.Formula != "" && captured
					if update.LiteralValue == "" && update.Formula == "" {
						update.ActionOutput = flowAutomaticApexActionOutput(raw, item.Value)
					}
					branch.FieldUpdates = append(branch.FieldUpdates, update)
					branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "fieldUpdate", FaultTarget: fault, FieldUpdates: []storage.WorkflowFieldUpdate{update}})
					continue
				}
				flowAssignment, ok := modeledFlowAssignment(assignment, item, formulas, variables, raw.RecordLookups)
				if !ok {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("assignment %q is not modeled in routed decision branch", firstNonBlank(assignment.Name, assignment.Label))})
					continue
				}
				if flowAssignment.Formula == "" {
					flowAssignment.SourceField = flowParentTextAssignmentSource(raw, item, variables, flowAssignment.SourceField)
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
			if updates, ok := modeledFlowRecordUpdates(flowRecordUpdateConstantValues(update, variables, flowConstantMap(raw.Constants)), formulas, variables, raw.RecordLookups); ok {
				branch.FieldUpdates = append(branch.FieldUpdates, updates...)
				if len(updates) > 0 {
					branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "fieldUpdate", FaultTarget: fault, FieldUpdates: updates})
				}
			} else if recordUpdate, ok := modeledFlowRecordUpdate(update, formulas, variables, flowConstantMap(raw.Constants), raw.RecordLookups, strings.EqualFold(raw.ProcessType, "Workflow")); ok {
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
			if recordLookup, ok := modeledFlowRecordLookup(lookup, variables, flowConstantMap(raw.Constants), raw.RecordLookups); ok {
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
			if recordCreate, ok := modeledFlowCapturedRecordCreate(raw, create, formulas, variables); ok {
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
			if recordDelete, ok := modeledFlowRecordDelete(del, variables, flowConstantMap(raw.Constants), raw.RecordLookups); ok {
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
			loopVisited := cloneFlowVisited(visited)
			loopVisited[strings.ToLower(loopName)] = true
			body, bodyDiagnostics = flowPopulateBranchFromTargetWithStop(body, bodyTarget, loopName, raw, formulas, variables, loopVisited)
			diagnostics = append(diagnostics, bodyDiagnostics...)
			currentItem := strings.TrimSpace(loop.AssignNextValueToReference)
			if currentItem == "" {
				currentItem = loopName
			}
			branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "loop", FaultTarget: strings.TrimSpace(loop.FaultConnector.TargetReference), Loop: storage.FlowLoop{Name: loopName, CollectionReference: collection, CurrentItemReference: currentItem, IterationOrder: strings.TrimSpace(loop.IterationOrder), Steps: body.Steps}})
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
			decision, lookupSteps := flowCoreLookupIDDecision(raw, decision)
			branch.Steps = append(branch.Steps, lookupSteps...)
			stepBranches, decisionDiagnostics := modeledFlowStepDecisionBranches(decision, raw, formulas, variables, visited)
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
			branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "customError", CustomError: modeledFlowCustomError(customError, raw.Constants, flowCustomErrorFrameFormulas(raw), formulas)})
			break
		}
		if modeled {
			target = ""
			continue
		}
		for _, wait := range raw.Waits {
			// Only a Wait for an amount of time (WaitDuration) has a native
			// row; other wait kinds stay unmodeled.
			if !strings.EqualFold(target, wait.Name) || !strings.EqualFold(strings.TrimSpace(wait.ElementSubtype), "WaitDuration") {
				continue
			}
			// The interview pauses here; nothing after the wait runs in this
			// transaction (process_builder_wait waits rows). Target carries the
			// interview label template of the paused FlowInterview.
			modeled = true
			branch.Steps = append(branch.Steps, storage.FlowStep{Kind: "wait", Target: strings.TrimSpace(raw.InterviewLabel), VisitKey: strings.TrimSpace(wait.Name)})
			break
		}
		if modeled {
			break
		}
		if !modeled {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("routed decision target %q is not modeled", target)})
		}
		break
	}
	buildFlowFaultBranches(branch.Steps, raw, formulas, variables, visited)
	return branch, diagnostics
}

// flowUndeclaredCustomErrorReference handles a bare reference only for the
// captured simple shape, where no other resources or executable nodes exist.
func flowUndeclaredCustomErrorReference(raw flowXML) string {
	if len(raw.Constants)+len(raw.Variables)+len(raw.Formulas)+len(raw.TextTemplates)+len(raw.Decisions)+len(raw.Assignments)+len(raw.RecordUpdates)+len(raw.Screens)+len(raw.Loops)+len(raw.Subflows)+len(raw.ActionCalls)+len(raw.RecordLookups)+len(raw.RecordCreates)+len(raw.RecordDeletes)+len(raw.ApexPluginCalls)+len(raw.Transforms)+len(raw.CollectionProcessors)+len(raw.OrchestratedStages)+len(raw.Stages)+len(raw.Waits) != 0 {
		return ""
	}
	for _, node := range raw.CustomErrors {
		for _, message := range node.Messages {
			text := firstNonBlank(message.ErrorMessage, message.Message)
			for len(text) > 0 {
				start := strings.Index(text, "{!")
				if start < 0 {
					break
				}
				text = text[start+2:]
				end := strings.IndexByte(text, '}')
				if end < 0 {
					break
				}
				ref := strings.TrimSpace(text[:end])
				text = text[end+1:]
				if ref == "" || !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(ref) {
					continue
				}
				known := false
				for _, candidate := range raw.CustomErrors {
					if strings.EqualFold(candidate.Name, ref) {
						known = true
						break
					}
				}
				if !known {
					return fmt.Sprintf("field integrity exception: unknown (\"%s\" refers to an element: \"%s\", which doesn't exist.)", node.Name, ref)
				}
			}
		}
	}
	return ""
}

// Group only ordinary decimal spellings; alternate metadata encodings have no native row.
var flowMergePlainDecimal = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func modeledFlowCustomError(customError flowCustomErrorXML, constants []flowConstantXML, formulaMaps ...map[string]string) storage.FlowCustomError {
	var formulas, resources map[string]string
	if len(formulaMaps) > 0 {
		formulas = formulaMaps[0]
	}
	if len(formulaMaps) > 1 {
		resources = formulaMaps[1]
	}
	ce := storage.FlowCustomError{Name: strings.TrimSpace(customError.Name), Description: strings.TrimSpace(customError.Description)}
	for _, msg := range customError.Messages {
		text := strings.TrimSpace(firstNonBlank(msg.ErrorMessage, msg.Message))
		numbers := make(map[string]string)
		for _, constant := range constants {
			value := strings.TrimSpace(constant.Value.NumberValue)
			if number, err := strconv.ParseFloat(value, 64); flowMergePlainDecimal.MatchString(value) && err == nil && !math.IsInf(number, 0) && !math.IsNaN(number) && math.Abs(number) >= 1000 {
				numbers[strings.ToLower(strings.TrimSpace(constant.Name))] = value
			}
		}
		message := storage.FlowCustomErrorMessage{Message: inlineFlowConstantMergeFields(text, constants), NumberConstants: numbers, Formulas: formulas}
		// Native rows (flow_assign_routes, API 62.0 and 67.0): a formula
		// resource merge field finishes its value, and a field-level error
		// names its selected field.
		lower := strings.ToLower(message.Message)
		for name, expression := range resources {
			if strings.Contains(lower, "{!"+name+"}") {
				if message.ResourceFormulas == nil {
					message.ResourceFormulas = make(map[string]string)
				}
				message.ResourceFormulas[name] = expression
			}
		}
		if msg.IsFieldError {
			message.Field = trimObjectPrefix(strings.TrimSpace(msg.FieldSelection))
		}
		ce.Messages = append(ce.Messages, message)
	}
	return ce
}

// inlineFlowConstantMergeFields replaces {!constant} in message text with the
// constant's value. Other merge fields stay for the interview to resolve.
func inlineFlowConstantMergeFields(text string, constants []flowConstantXML) string {
	if !strings.Contains(text, "{!") || len(constants) == 0 {
		return text
	}
	byName := make(map[string]string, len(constants))
	for _, constant := range constants {
		name := strings.ToLower(strings.TrimSpace(constant.Name))
		switch {
		case name == "":
		case strings.TrimSpace(constant.Value.NumberValue) != "":
			if number, err := strconv.ParseFloat(strings.TrimSpace(constant.Value.NumberValue), 64); err == nil && !math.IsNaN(number) && !math.IsInf(number, 0) && math.Abs(number) < 1000 {
				formatted := strconv.FormatFloat(number, 'f', -1, 64)
				if number == 0 {
					formatted = "0"
				}
				byName[name] = formatted
			}
		case strings.EqualFold(strings.TrimSpace(constant.Value.BooleanValue), "true"), strings.EqualFold(strings.TrimSpace(constant.Value.BooleanValue), "false"):
			byName[name] = strings.ToLower(strings.TrimSpace(constant.Value.BooleanValue))
		case constant.Value.StringValue != "":
			byName[name] = constant.Value.StringValue
		}
	}
	var out strings.Builder
	for offset := 0; offset < len(text); {
		start := strings.Index(text[offset:], "{!")
		if start < 0 {
			out.WriteString(text[offset:])
			break
		}
		start += offset
		out.WriteString(text[offset:start])
		end := strings.IndexByte(text[start+2:], '}')
		if end < 0 {
			out.WriteString(text[start:])
			break
		}
		end += start + 2
		if value, ok := byName[strings.ToLower(strings.TrimSpace(text[start+2:end]))]; ok {
			out.WriteString(value)
		} else {
			out.WriteString(text[start : end+1])
		}
		offset = end + 1
	}
	return out.String()
}

func modeledFlowStepDecisionBranches(decision flowDecisionXML, raw flowXML, formulas map[string]string, variables map[string]flowVariableXML, visited map[string]bool) ([]storage.FlowBranch, []diagnostic.Diagnostic) {
	var branches []storage.FlowBranch
	var diagnostics []diagnostic.Diagnostic
	decisionFormulas := formulas
	if raw.decisionFormulas != nil {
		decisionFormulas = raw.decisionFormulas
	}
	for _, rule := range decision.Rules {
		target := strings.TrimSpace(rule.Connector.TargetReference)
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
			if flowConditionGroupsNeedFormula([][]int{group}, rule.Conditions, decisionFormulas) {
				formula := flowConditionGroupsToFormula([][]int{group}, rule.Conditions, decisionFormulas, variables)
				if formula == "" {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("decision %q branch %q condition logic %q is not supported", decision.Name, branch.Name, rule.ConditionLogic)})
					continue
				}
				// Formula resources are executable values, not object fields. Keep
				// the branch formula intact so the DML frame evaluator can resolve
				// its $Record references instead of looking up the resource name as
				// a schema field.
				branch.Formula = formula
			} else {
				for _, conditionIndex := range group {
					condition := rule.Conditions[conditionIndex]
					left := firstNonBlank(flowVariableRecordField(condition.LeftValueReference, variables), flowRecordSourceFieldReference(condition.LeftValueReference), strings.TrimSpace(condition.LeftValueReference))
					if left == "" {
						diagnostics = append(diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("decision %q branch %q condition %q is not modeled", decision.Name, branch.Name, condition.LeftValueReference)})
						continue
					}
					item := storage.WorkflowCriteriaItem{
						Field:     left,
						Operation: flowOperator(strings.TrimSpace(condition.Operator)),
						Value:     flowDecisionRightValue(condition.RightValue),
					}
					flowVariableEmptyOperand(&item, condition, variables)
					branch.Criteria = append(branch.Criteria, item)
				}
			}
			// A rule without a connector ends the flow when it matches. Keep it,
			// empty, so the default path does not run for a matching record.
			if target == "" {
				branches = append(branches, branch)
				continue
			}
			var branchDiagnostics []diagnostic.Diagnostic
			branch, branchDiagnostics = flowPopulateBranchFromTargetWithStop(branch, target, "", raw, formulas, variables, cloneFlowVisited(visited))
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
		branch, branchDiagnostics = flowPopulateBranchFromTargetWithStop(branch, defaultTarget, "", raw, formulas, variables, cloneFlowVisited(visited))
		diagnostics = append(diagnostics, branchDiagnostics...)
		if flowBranchHasEffects(branch) {
			branches = append(branches, branch)
		}
	}
	return branches, diagnostics
}

func modeledFlowAssignmentUpdate(assignment flowAssignmentXML, item flowAssignmentItemXML, formulas map[string]string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.WorkflowFieldUpdate, bool) {
	if !flowAssignmentOperatorSupported(item.Operator) {
		return storage.WorkflowFieldUpdate{}, false
	}
	operator := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(item.Operator), " ", ""))
	if operator == "assigncount" || operator == "add" {
		return storage.WorkflowFieldUpdate{}, false
	}
	if _, variable := variables[strings.ToLower(strings.TrimSpace(item.AssignToReference))]; variable {
		return storage.WorkflowFieldUpdate{}, false
	}
	if invocableFlowActionRecordAssignment(item.AssignToReference, variables) {
		return storage.WorkflowFieldUpdate{}, false
	}
	field := flowRecordFieldReference(item.AssignToReference)
	if field == "" {
		return storage.WorkflowFieldUpdate{}, false
	}
	formula := flowNullTextLiteralFormula(item.Value, flowExpressionValue(item.Value, formulas))
	return storage.WorkflowFieldUpdate{
		Name:         firstNonBlank(assignment.Name, assignment.Label, field),
		Field:        field,
		LiteralValue: flowLiteralValue(item.Value),
		Formula:      formula,
		SourceField:  flowLookupSourceFieldValueUnlessFormula(item.Value, formula, variables, lookups),
	}, true
}

// flowAutomaticApexActionOutput names the Apex action whose automatically
// stored output an assignment value references directly.
func flowAutomaticApexActionOutput(raw flowXML, value flowValueXML) string {
	reference := strings.TrimSpace(value.ElementReference)
	if reference == "" {
		return ""
	}
	for _, action := range raw.ActionCalls {
		name := firstNonBlank(action.Name, action.Label, action.ActionName)
		if action.StoreOutputAutomatically && strings.EqualFold(strings.TrimSpace(action.ActionType), "apex") && strings.EqualFold(reference, name) {
			return name
		}
	}
	return ""
}

func modeledFlowAssignment(assignment flowAssignmentXML, item flowAssignmentItemXML, formulas map[string]string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) (storage.FlowAssignment, bool) {
	if !flowAssignmentOperatorSupported(item.Operator) {
		return storage.FlowAssignment{}, false
	}
	target := strings.TrimSpace(item.AssignToReference)
	if target == "" {
		return storage.FlowAssignment{}, false
	}
	formula := flowExpressionValue(item.Value, formulas)
	if variable, declared := variables[strings.ToLower(target)]; formula == "" && declared && !variable.IsCollection && strings.EqualFold(variable.DataType, "String") && item.Value.EmptyStringValue {
		formula = `""`
	}
	if formula == "" {
		formula = invocableFlowActionBooleanAssignment(item, variables)
	}
	if formula == "" {
		formula = flowCoreAssignmentLiteralFormula(item, variables, lookups)
	}
	sourceField := ""
	if formula == "" {
		sourceField = firstNonBlank(flowLookupSourceFieldValue(item.Value, variables, lookups), strings.TrimSpace(item.Value.ElementReference))
		sourceField = flowCoreAssignmentSourceField(item, variables, lookups, sourceField)
	}
	return storage.FlowAssignment{
		Name:         firstNonBlank(assignment.Name, assignment.Label, target),
		Target:       target,
		Operator:     strings.TrimSpace(item.Operator),
		LiteralValue: flowLiteralOrVariableValue(item.Value, variables),
		Formula:      formula,
		SourceField:  sourceField,
	}, true
}

func modeledFlowRecordUpdates(update flowRecordUpdateXML, formulas map[string]string, variables map[string]flowVariableXML, lookups []flowRecordLookupXML) ([]storage.WorkflowFieldUpdate, bool) {
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
		formula := flowExpressionValue(assignment.Value, formulas)
		updates = append(updates, storage.WorkflowFieldUpdate{
			Name:         firstNonBlank(update.Name, update.Label, field),
			Field:        field,
			LiteralValue: flowLiteralValue(assignment.Value),
			Formula:      formula,
			SourceField:  flowRecordUpdateAssignmentSource(assignment.Value, formula, variables, lookups),
		})
	}
	return updates, true
}

func modeledFlowRecordUpdate(update flowRecordUpdateXML, formulas map[string]string, variables map[string]flowVariableXML, constants map[string]flowValueXML, lookups []flowRecordLookupXML, process bool) (storage.FlowRecordUpdate, bool) {
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
	logic := strings.TrimSpace(update.FilterLogic)
	if logic != "" && !strings.EqualFold(logic, "and") && (!process || !capturedProcessUpdateAlternatives(update)) {
		return storage.FlowRecordUpdate{}, false
	}
	out := storage.FlowRecordUpdate{Name: name, ObjectName: objectName, InputReference: inputReference}
	if logic != "" && !strings.EqualFold(logic, "and") {
		out.CriteriaLogic = logic
	}
	for _, filter := range update.Filters {
		field := trimObjectPrefix(strings.TrimSpace(filter.Field))
		if field == "" {
			return storage.FlowRecordUpdate{}, false
		}
		out.Criteria = append(out.Criteria, storage.WorkflowCriteriaItem{
			Field:       field,
			Operation:   flowOperator(strings.TrimSpace(filter.Operator)),
			Value:       flowLiteralOrVariableOrConstantValue(filter.Value, variables, constants),
			SourceField: flowLookupCriteriaSourceFieldValue(filter.Value, variables, constants, lookups),
		})
	}
	for _, assignment := range update.Fields {
		field := trimObjectPrefix(strings.TrimSpace(assignment.Field))
		if field == "" {
			return storage.FlowRecordUpdate{}, false
		}
		formula := flowExpressionValue(assignment.Value, formulas)
		out.InputAssignments = append(out.InputAssignments, storage.WorkflowFieldUpdate{
			Name:         field,
			Field:        field,
			LiteralValue: flowLiteralOrVariableValue(assignment.Value, variables),
			Formula:      formula,
			SourceField:  flowRecordUpdateAssignmentSource(assignment.Value, formula, variables, lookups),
		})
	}
	return out, true
}

// capturedProcessUpdateAlternatives bounds routed record-update filters to
// the five-predicate form observed in process_builder_review routedcontrol.
func capturedProcessUpdateAlternatives(update flowRecordUpdateXML) bool {
	logic := strings.ToUpper(strings.Join(strings.Fields(update.FilterLogic), ""))
	if logic != "1AND(2OR3OR4OR5)" || update.InputReference != "" || len(update.Filters) != 5 || len(update.Fields) != 1 || update.FaultConnector.TargetReference != "" {
		return false
	}
	id := update.Filters[0]
	if !strings.EqualFold(id.Field, "Id") || !strings.EqualFold(id.Operator, "EqualTo") || !strings.EqualFold(id.Value.ElementReference, "$Record.Id") {
		return false
	}
	field := strings.TrimSpace(update.Filters[1].Field)
	if field == "" || strings.EqualFold(field, "Id") {
		return false
	}
	for _, filter := range update.Filters[1:] {
		if !strings.EqualFold(filter.Field, field) || !strings.EqualFold(filter.Operator, "EqualTo") || filter.Value.StringValue == "" || filter.Value.ElementReference != "" {
			return false
		}
	}
	assignment := update.Fields[0]
	return assignment.Field != "" && assignment.Value.StringValue != "" && assignment.Value.ElementReference == ""
}

func flowBranchHasEffects(branch storage.FlowBranch) bool {
	return len(branch.Steps) > 0 || len(branch.FieldUpdates) > 0 || len(branch.Actions) > 0 || len(branch.RecordLookups) > 0 || len(branch.RecordCreates) > 0
}

func flowRequiresOrderedGraph(raw flowXML) bool {
	for _, lookup := range raw.RecordLookups {
		if lookup.StoreOutputAutomatically && !lookup.GetFirstRecordOnly && !strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordBeforeDelete") {
			return true
		}
	}
	if raw.Start.Object == "" && strings.EqualFold(raw.ProcessType, "AutoLaunchedFlow") && strings.TrimSpace(raw.Start.Connector.TargetReference) != "" {
		return true
	}
	if flowPriorDecisionNeedsGraph(raw) {
		return true
	}
	variables := flowVariableMap(raw.Variables)
	saveTrigger := strings.EqualFold(raw.Start.TriggerType, "RecordBeforeSave") || strings.EqualFold(raw.Start.TriggerType, "RecordAfterSave") ||
		strings.EqualFold(raw.Start.TriggerType, "RecordBeforeDelete") && strings.EqualFold(raw.Start.RecordTriggerType, "Delete")
	for _, decision := range raw.Decisions {
		for _, branch := range decision.Rules {
			for _, condition := range branch.Conditions {
				variable, declared := variables[strings.ToLower(strings.TrimSpace(condition.LeftValueReference))]
				if declared && !variable.IsCollection && !strings.EqualFold(variable.DataType, "SObject") {
					operator := strings.ToLower(strings.TrimSpace(condition.Operator))
					if strings.EqualFold(variable.DataType, "String") && (operator == "startswith" || operator == "endswith") {
						return true
					}
					if saveTrigger && (operator == "equalto" || operator == "notequalto" || operator == "isnull") && flowVariableRecordField(condition.LeftValueReference, variables) == "" {
						// Unbound scalar save decisions need their frame from entry.
						// Record aliases and other trigger/operator routes retain their
						// existing lowering.
						return true
					}
				}
			}
		}
		if flowCoreLookupIDReference(raw, decision) != "" {
			return true
		}
	}
	if flowRefsDecisionNeedsLookup(raw) {
		return true
	}
	for _, lookup := range raw.RecordLookups {
		if strings.TrimSpace(lookup.OutputReference) != "" || len(lookup.OutputAssignments) > 0 {
			return true
		}
		for _, formula := range raw.Formulas {
			name := firstNonBlank(lookup.Name, lookup.Label)
			if lookup.StoreOutputAutomatically && name != "" && strings.Contains(strings.ToLower(formula.Expression), "{!"+strings.ToLower(name)+".") {
				return true
			}
		}
	}
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
		for _, field := range update.Fields {
			if flowLookupFieldReference(field.Value.ElementReference, raw.RecordLookups) != "" {
				return true
			}
		}
	}
	for _, create := range raw.RecordCreates {
		if strings.TrimSpace(create.InputReference) != "" {
			return true
		}
	}
	for _, assignment := range raw.Assignments {
		for _, item := range assignment.Items {
			if flowRecordFieldReference(item.AssignToReference) == "" || flowLookupFieldReference(item.Value.ElementReference, raw.RecordLookups) != "" {
				return true
			}
		}
	}
	return false
}

func flowPermissionReference(reference string) bool {
	reference = strings.TrimSpace(reference)
	return len(reference) > len("$Permission.") && strings.EqualFold(reference[:len("$Permission.")], "$Permission.")
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

var flowBareRecordFormulaPrefix = regexp.MustCompile(`(^|[^[:alnum:]_$])Record\.`)

func normalizeFlowFormula(expression string) string {
	expression = strings.TrimSpace(expression)
	expression = strings.ReplaceAll(expression, "{!$Record.", "")
	expression = strings.ReplaceAll(expression, "{!Record.", "")
	expression = strings.ReplaceAll(expression, "{!", "")
	expression = strings.ReplaceAll(expression, "$Record.", "")
	expression = flowBareRecordFormulaPrefix.ReplaceAllString(expression, "${1}")
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
	case "flow":
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
		formula := flowExpressionValue(input.Value, formulas)
		sourceField := flowLookupSourceFieldValueUnlessFormula(input.Value, formula, variables, lookups)
		if sourceField == "" && strings.TrimSpace(formula) == "" {
			reference := strings.TrimSpace(input.Value.ElementReference)
			if _, ok := variables[strings.ToLower(reference)]; ok {
				sourceField = reference
			} else if strings.EqualFold(reference, "$Record") || strings.EqualFold(reference, "$Record__Prior") {
				sourceField = reference
			}
		}
		out.Inputs = append(out.Inputs, storage.WorkflowFieldUpdate{
			Name:         name,
			Field:        name,
			LiteralValue: flowLiteralOrVariableValue(input.Value, variables),
			Formula:      formula,
			SourceField:  sourceField,
		})
	}
	for _, output := range action.OutputParameters {
		name, target := strings.TrimSpace(output.Name), strings.TrimSpace(output.AssignToReference)
		if name != "" && target != "" {
			out.Outputs = append(out.Outputs, storage.FlowActionOutput{Name: name, AssignTo: target})
		}
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

// A constant is a named literal. Native assignment rows (API 62.0 and 67.0,
// anonymous and test routes) give an assignment from a constant the value of
// the same literal, whether the target is a variable or a record field.
func flowAssignmentItemConstantValue(item flowAssignmentItemXML, variables map[string]flowVariableXML, constants map[string]flowValueXML) flowAssignmentItemXML {
	reference := strings.ToLower(strings.TrimSpace(item.Value.ElementReference))
	if reference == "" || flowLiteralValue(item.Value) != "" {
		return item
	}
	if _, variable := variables[reference]; variable {
		return item
	}
	if constant, ok := constants[reference]; ok {
		item.Value = constant
	}
	return item
}

// A scalar variable compared without a right value is a null test, and an
// explicit empty String is a distinct operand that no null variable equals
// (native rows at API 62.0 and 67.0). Record fields keep their existing path.
func flowVariableEmptyOperand(item *storage.WorkflowCriteriaItem, condition flowConditionXML, variables map[string]flowVariableXML) {
	if item.Value != "" || strings.TrimSpace(condition.RightValue.ElementReference) != "" || strings.TrimSpace(condition.RightValue.Formula) != "" || strings.TrimSpace(condition.RightValue.Expression) != "" {
		return
	}
	variable, declared := variables[strings.ToLower(strings.TrimSpace(condition.LeftValueReference))]
	if !declared || strings.Contains(condition.LeftValueReference, ".") || strings.EqualFold(variable.DataType, "SObject") || variable.IsCollection {
		return
	}
	switch item.Operation {
	case "equals", "notEqual":
	default:
		return
	}
	if condition.RightValue.EmptyStringValue {
		item.Value = "{!" + flowEmptyStringOperand + "}"
		return
	}
	if item.Operation == "equals" {
		item.Operation, item.Value = "isNull", "true"
	} else {
		item.Operation, item.Value = "isNull", "false"
	}
}

const flowEmptyStringOperand = "$EmptyString"

func flowRecordUpdateConstantValues(update flowRecordUpdateXML, variables map[string]flowVariableXML, constants map[string]flowValueXML) flowRecordUpdateXML {
	update.Fields = append([]flowFieldAssignmentXML(nil), update.Fields...)
	for i := range update.Fields {
		item := flowAssignmentItemConstantValue(flowAssignmentItemXML{Value: update.Fields[i].Value}, variables, constants)
		if item.Value.ElementReference == "" && update.Fields[i].Value.ElementReference != "" && flowLiteralValue(item.Value) == "" {
			item.Value = flowValueXML{Formula: "NULL"}
		}
		update.Fields[i].Value = item.Value
	}
	return update
}

// flowLookupFilterLogic retains and/or and accepts the native numbered
// conjunction shape (1 AND 2 AND 3). Mixed or partial logic stays unmodeled.
func flowLookupFilterLogic(logic string, filters int) (string, bool) {
	logic = strings.TrimSpace(logic)
	if logic == "" || strings.EqualFold(logic, "and") || strings.EqualFold(logic, "or") {
		return strings.ToLower(logic), true
	}
	fields := strings.Fields(logic)
	if len(fields) < 3 || len(fields)%2 == 0 || filters != (len(fields)+1)/2 {
		return "", false
	}
	connective := strings.ToLower(fields[1])
	if connective != "and" {
		return "", false
	}
	for i, field := range fields {
		if i%2 == 1 {
			if !strings.EqualFold(field, connective) {
				return "", false
			}
			continue
		}
		if field != fmt.Sprint(i/2+1) {
			return "", false
		}
	}
	return connective, true
}

// processBuilderXML reads the top-level processMetadataValues of a Process
// Builder process (processType Workflow).
type processBuilderXML struct {
	ProcessType string `xml:"processType"`
	Status      string `xml:"status"`
	Elements    []struct {
		XMLName xml.Name
	} `xml:",any"`
	Decisions []struct {
		Rules []struct {
			Name       string `xml:"name"`
			Conditions []struct {
				LeftValueReference string `xml:"leftValueReference"`
				Operator           string `xml:"operator"`
				RightValue         struct {
					ElementReference string `xml:"elementReference"`
					BooleanValue     string `xml:"booleanValue"`
				} `xml:"rightValue"`
			} `xml:"conditions"`
		} `xml:"rules"`
	} `xml:"decisions"`
	ActionCalls []struct {
		ActionType string `xml:"actionType"`
		Inputs     []struct {
			Value struct {
				ElementReference string `xml:"elementReference"`
			} `xml:"value"`
		} `xml:"inputParameters"`
	} `xml:"actionCalls"`
	Assignments []struct {
		Name string `xml:"name"`
	} `xml:"assignments"`
	Formulas []struct {
		Name       string `xml:"name"`
		DataType   string `xml:"dataType"`
		Expression string `xml:"expression"`
	} `xml:"formulas"`
	RecordUpdates []struct {
		Object      string `xml:"object"`
		FilterLogic string `xml:"filterLogic"`
		Filters     []struct {
			Field    string `xml:"field"`
			Operator string `xml:"operator"`
			Value    struct {
				ElementReference string `xml:"elementReference"`
			} `xml:"value"`
		} `xml:"filters"`
		InputAssignments []struct {
			Field string `xml:"field"`
			Value struct {
				ElementReference string `xml:"elementReference"`
			} `xml:"value"`
		} `xml:"inputAssignments"`
	} `xml:"recordUpdates"`
	Variables []struct {
		Name string `xml:"name"`
	} `xml:"variables"`
	StartElementReference string `xml:"startElementReference"`
	Start                 struct {
		Object string `xml:"object"`
	} `xml:"start"`
	Metadata []struct {
		Name  string `xml:"name"`
		Value struct {
			StringValue      string `xml:"stringValue"`
			ElementReference string `xml:"elementReference"`
		} `xml:"value"`
	} `xml:"processMetadataValues"`
}

// normalizeProcessBuilderFlow rewrites a record-change Process Builder process
// into the record-triggered after-save flow it runs as: the ObjectType and
// TriggerType metadata become the start element, and the ObjectVariable and
// OldObjectVariable references become $Record and $Record__Prior. Native
// (process_builder_wait: a processType Workflow process at Flow API 49.0
// deploys at API 62.0 and 67.0, runs on insert and update, and its ISCHANGED
// decision calls an Apex action with the record). Other sources are returned
// unchanged.
//
// Only captured shapes run (process_builder_wait, process_builder_shapes): an
// active onAllChanges or onCreateOnly process of decisions, record updates,
// Apex, flow and email alert actions, with no resources besides the two object
// variables and the wait-start stamp, and criteria the rows cover. Any other
// active process (scheduled actions, Chatter actions, record creates, a recursion
// counter, an uncaptured operator or global) is left unrun, as before, and skip
// names the shape for an unsupported diagnostic, so it never half-runs.
func normalizeProcessBuilderFlow(data []byte, definitions ...map[string]storage.ObjectDefinition) (out []byte, skip string) {
	if !bytes.Contains(data, []byte("<processType>Workflow</processType>")) {
		return data, ""
	}
	var pb processBuilderXML
	if err := xml.Unmarshal(data, &pb); err != nil || !strings.EqualFold(strings.TrimSpace(pb.ProcessType), "Workflow") || strings.TrimSpace(pb.Start.Object) != "" || strings.TrimSpace(pb.StartElementReference) == "" {
		return data, ""
	}
	if !flowActive(pb.Status) {
		return data, ""
	}
	// A source-tool namespace token has no org row: a deploy tool replaces it.
	if bytes.Contains(data, []byte("%%%NAMESPACE")) {
		return data, "namespace token"
	}
	var objectType, triggerType, current, old string
	for _, item := range pb.Metadata {
		switch item.Name {
		case "ObjectType":
			objectType = strings.TrimSpace(item.Value.StringValue)
		case "TriggerType":
			triggerType = strings.TrimSpace(item.Value.StringValue)
		case "ObjectVariable":
			current = strings.TrimSpace(item.Value.ElementReference)
		case "OldObjectVariable":
			old = strings.TrimSpace(item.Value.ElementReference)
		}
	}
	if objectType == "" || current == "" {
		return data, ""
	}
	recordTriggerType := ""
	switch triggerType {
	case "onAllChanges":
		recordTriggerType = "CreateAndUpdate"
	case "onCreateOnly":
		recordTriggerType = "Create"
	default:
		return data, fmt.Sprintf("TriggerType %q", triggerType)
	}
	if skip := processBuilderUncoveredShape(pb, data, current, old, definitions...); skip != "" {
		return data, skip
	}
	text := string(data)
	replace := func(variable, record string) {
		if variable == "" {
			return
		}
		text = strings.ReplaceAll(text, "{!"+variable+".", "{!"+record+".")
		text = strings.ReplaceAll(text, "<elementReference>"+variable+"</elementReference>", "<elementReference>"+record+"</elementReference>")
		text = strings.ReplaceAll(text, "<elementReference>"+variable+".", "<elementReference>"+record+".")
		text = strings.ReplaceAll(text, "<leftValueReference>"+variable+".", "<leftValueReference>"+record+".")
		text = strings.ReplaceAll(text, "<leftValueReference>"+variable+"</leftValueReference>", "<leftValueReference>"+record+"</leftValueReference>")
		text = strings.ReplaceAll(text, "<assignToReference>"+variable+".", "<assignToReference>"+record+".")
		text = strings.ReplaceAll(text, "<inputReference>"+variable+"</inputReference>", "<inputReference>"+record+"</inputReference>")
		// The declaration would shadow the record resource.
		text = strings.ReplaceAll(text, "<name>"+variable+"</name>", "<name>"+variable+"_processBuilderVariable</name>")
	}
	replace(current, "$Record")
	replace(old, "$Record__Prior")
	start := "<start><connector><targetReference>" + html.EscapeString(strings.TrimSpace(pb.StartElementReference)) + "</targetReference></connector>" +
		"<object>" + html.EscapeString(objectType) + "</object><recordTriggerType>" + recordTriggerType + "</recordTriggerType><triggerType>RecordAfterSave</triggerType></start>"
	closing := strings.LastIndex(text, "</Flow>")
	if closing < 0 {
		return data, ""
	}
	return []byte(text[:closing] + start + text[closing:]), ""
}

// processBuilderUncoveredShape names the first part of a process outside the
// captured shapes, or returns "".
func processBuilderUncoveredShape(pb processBuilderXML, data []byte, current, old string, definitions ...map[string]storage.ObjectDefinition) string {
	var startVariable string
	for _, element := range pb.Elements {
		switch element.XMLName.Local {
		case "apiVersion", "label", "description", "interviewLabel", "processType", "status", "startElementReference",
			"processMetadataValues", "formulas", "variables", "decisions", "actionCalls", "recordUpdates", "areMetricsLoggedToDataCloud", "isTemplate":
		default:
			return fmt.Sprintf("element %q", element.XMLName.Local)
		}
	}
	if len(pb.Assignments) > 0 {
		var ok bool
		startVariable, ok = processBuilderCapturedWaitStart(data, current, old)
		if !ok {
			return fmt.Sprintf("assignment %q with uncaptured start shape", strings.TrimSpace(pb.Assignments[0].Name))
		}
	}
	for _, action := range pb.ActionCalls {
		switch actionType := strings.TrimSpace(action.ActionType); strings.ToLower(actionType) {
		case "apex", "flow", "emailalert":
		default:
			return fmt.Sprintf("actionType %q", actionType)
		}
	}
	for _, variable := range pb.Variables {
		name := strings.TrimSpace(variable.Name)
		if name == current || name == old || (startVariable != "" && strings.EqualFold(name, startVariable)) {
			continue
		}
		return fmt.Sprintf("variable %q", name)
	}
	return processBuilderUncoveredOperand(pb, current, definitions...)
}

// processBuilderUncoveredOperand names the first criterion, filter or value the
// process_builder_shapes rows do not cover. Native evaluates these, so running
// them with a guess or failing the save at run time would both be wrong: an
// operator without a row, a global such as $User or $Setup, and a field right
// operand other than the current record's under equals or not equals.
func processBuilderUncoveredOperand(pb processBuilderXML, current string, definitions ...map[string]storage.ObjectDefinition) string {
	formulas := make(map[string]bool, len(pb.Formulas))
	for _, formula := range pb.Formulas {
		name := strings.TrimSpace(formula.Name)
		formulas[strings.ToLower(name)] = true
		globalAlias := processBuilderCapturedUserAliasFormula(formula.Expression) && strings.EqualFold(formula.DataType, "Boolean") && processBuilderCapturedAliasUsage(pb, name)
		valueResource := processBuilderCapturedValueResource(pb, name, formula.DataType, formula.Expression, current, definitions...)
		pickChanged := processBuilderCapturedPickChanged(pb, name, formula.DataType, formula.Expression, current, definitions...)
		formatFormula := processBuilderCapturedFormatFormula(pb, name, formula.DataType, formula.Expression, current, definitions...)
		priorBlank := processBuilderCapturedPriorBlankFormula(pb, name, formula.DataType, formula.Expression, current, definitions...)
		if strings.Contains(formula.Expression, "$") && !globalAlias && !valueResource {
			return fmt.Sprintf("formula %q reading a global", name)
		}
		if function := processBuilderUnknownFunction(formula.Expression); function != "" {
			return fmt.Sprintf("formula %q calling %s", name, function)
		}
		if !globalAlias && !valueResource && !pickChanged && !formatFormula && !priorBlank && (!processBuilderCapturedFormula(formula.Expression, current) || !processBuilderCapturedFormulaUse(pb, name, formula.DataType, formula.Expression, current, definitions...)) {
			return fmt.Sprintf("formula %q with uncaptured expression or usage", name)
		}
	}
	for _, decision := range pb.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				operator := strings.TrimSpace(condition.Operator)
				switch strings.ToLower(operator) {
				case "equalto", "notequalto", "greaterthan", "greaterthanorequalto", "isnull":
				default:
					return fmt.Sprintf("operator %q", operator)
				}
				left := strings.TrimSpace(condition.LeftValueReference)
				if strings.Contains(left, "$") {
					return fmt.Sprintf("reference %q", left)
				}
				right := strings.TrimSpace(condition.RightValue.ElementReference)
				if right == "" || formulas[strings.ToLower(right)] {
					continue
				}
				_, field := strings.CutPrefix(right, current+".")
				equality := strings.EqualFold(operator, "EqualTo") || strings.EqualFold(operator, "NotEqualTo")
				if !field || !equality {
					return fmt.Sprintf("right operand %q with operator %q", right, operator)
				}
			}
		}
	}
	for _, update := range pb.RecordUpdates {
		if logic := strings.TrimSpace(update.FilterLogic); logic != "" && !strings.EqualFold(logic, "and") {
			if len(definitions) == 0 || len(update.Filters) != 5 || len(update.InputAssignments) != 1 {
				return "routed record update without captured field types"
			}
			var def storage.ObjectDefinition
			for name, candidate := range definitions[0] {
				if strings.EqualFold(name, update.Object) {
					def = candidate
					break
				}
			}
			filter, okFilter := storage.ResolveFieldName(def, "", update.Filters[1].Field)
			assignment, okAssignment := storage.ResolveFieldName(def, "", update.InputAssignments[0].Field)
			if !okFilter || !okAssignment {
				return "routed record update with uncaptured field types"
			}
			filterType, assignmentType := def.Fields[filter].Type, def.Fields[assignment].Type
			text := filterType == storage.FieldString && assignmentType == storage.FieldString
			picklist := filterType == storage.FieldPicklist && assignmentType == storage.FieldPicklist && strings.EqualFold(filter, assignment)
			if !text && !picklist {
				return "routed record update with uncaptured field types"
			}
		}
		for _, filter := range update.Filters {
			if operator := strings.TrimSpace(filter.Operator); !strings.EqualFold(operator, "EqualTo") {
				return fmt.Sprintf("record update filter operator %q", operator)
			}
			if reference := strings.TrimSpace(filter.Value.ElementReference); strings.Contains(reference, "$") {
				return fmt.Sprintf("reference %q", reference)
			}
		}
		for _, assignment := range update.InputAssignments {
			if reference := strings.TrimSpace(assignment.Value.ElementReference); strings.Contains(reference, "$") {
				return fmt.Sprintf("reference %q", reference)
			}
		}
	}
	return ""
}

var (
	processBuilderFormulaStrings = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`)
	processBuilderFormulaCalls   = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

// processBuilderKnownFunctions lists what the formula evaluator reads. Another
// function would write its source text or stop the interview, so such a process
// stays unrun.
var processBuilderKnownFunctions = map[string]bool{
	"ABS": true, "ADDMONTHS": true, "AND": true, "BEGINS": true, "BLANKVALUE": true, "BR": true, "CASE": true, "CASESAFEID": true,
	"CEILING": true, "CONTAINS": true, "DATE": true, "DATETIMEVALUE": true, "DATEVALUE": true, "DAY": true, "DAYOFYEAR": true, "EXP": true,
	"FIND": true, "FLOOR": true, "IF": true, "IMAGE": true, "INCLUDES": true, "ISBLANK": true, "ISCHANGED": true, "ISNEW": true,
	"ISNULL": true, "ISNUMBER": true, "ISPICKVAL": true, "LEFT": true, "LEN": true, "LN": true, "LOG": true, "LOWER": true, "LPAD": true,
	"MAX": true, "MID": true, "MIN": true, "MOD": true, "MONTH": true, "NOT": true, "NOW": true, "NULLVALUE": true, "OR": true, "POWER": true,
	"PRIORVALUE": true, "REGEX": true, "RIGHT": true, "ROUND": true, "RPAD": true, "SQRT": true, "SUBSTITUTE": true, "TEXT": true,
	"TIMEVALUE": true, "TODAY": true, "TRIM": true, "UPPER": true, "VALUE": true, "WEEKDAY": true, "YEAR": true,
}

func processBuilderUnknownFunction(expression string) string {
	bare := processBuilderFormulaStrings.ReplaceAllString(html.UnescapeString(expression), `""`)
	for _, call := range processBuilderFormulaCalls.FindAllStringSubmatch(bare, -1) {
		if !processBuilderKnownFunctions[strings.ToUpper(call[1])] {
			return call[1] + "()"
		}
	}
	return ""
}

func processBuilderFormulaExtraUse(pb processBuilderXML, name string) bool {
	for _, action := range pb.ActionCalls {
		for _, input := range action.Inputs {
			if strings.EqualFold(input.Value.ElementReference, name) {
				return true
			}
		}
	}
	for _, update := range pb.RecordUpdates {
		for _, filter := range update.Filters {
			if strings.EqualFold(filter.Value.ElementReference, name) {
				return true
			}
		}
	}
	return false
}

// processBuilderCapturedAliasUsage admits the alias resource only as the
// true-valued decision criterion observed by globalcontrol.
func processBuilderCapturedAliasUsage(pb processBuilderXML, name string) bool {
	if processBuilderFormulaExtraUse(pb, name) {
		return false
	}
	found := 0
	for _, decision := range pb.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				if strings.EqualFold(condition.RightValue.ElementReference, name) {
					return false
				}
				if strings.EqualFold(condition.LeftValueReference, name) {
					if !strings.EqualFold(condition.Operator, "EqualTo") || !strings.EqualFold(condition.RightValue.BooleanValue, "true") {
						return false
					}
					found++
				}
			}
		}
	}
	for _, update := range pb.RecordUpdates {
		for _, assignment := range update.InputAssignments {
			if strings.EqualFold(assignment.Value.ElementReference, name) {
				return false
			}
		}
	}
	return found == 1
}

// processBuilderCapturedPickChanged recognizes the captured change/picklist
// decision and requires the participating fields to have the observed types.
func processBuilderCapturedPickChanged(pb processBuilderXML, name, dataType, expression, current string, definitions ...map[string]storage.ObjectDefinition) bool {
	if !strings.EqualFold(dataType, "Boolean") || len(definitions) == 0 || !processBuilderCapturedAliasUsage(pb, name) {
		return false
	}
	field := `\{!\s*` + regexp.QuoteMeta(current) + `\.([A-Za-z_][A-Za-z0-9_]*)\s*\}`
	pattern := `(?i)^\s*ISCHANGED\s*\(\s*` + field + `\s*\)\s*&&\s*ISPICKVAL\s*\(\s*` + field + `\s*,\s*"[^"\\]*"\s*\)\s*&&\s*\(\s*` + field + `\s*=\s*FALSE\s*\)\s*$`
	match := regexp.MustCompile(pattern).FindStringSubmatch(html.UnescapeString(expression))
	if len(match) != 4 || !strings.EqualFold(match[1], match[2]) {
		return false
	}
	var objectName string
	for _, item := range pb.Metadata {
		if item.Name == "ObjectType" {
			objectName = strings.TrimSpace(item.Value.StringValue)
			break
		}
	}
	var def storage.ObjectDefinition
	for object, candidate := range definitions[0] {
		if strings.EqualFold(object, objectName) {
			def = candidate
			break
		}
	}
	pick, okPick := storage.ResolveFieldName(def, "", match[1])
	flag, okFlag := storage.ResolveFieldName(def, "", match[3])
	return okPick && okFlag && def.Fields[pick].Type == storage.FieldPicklist && def.Fields[flag].Type == storage.FieldBoolean
}

// processBuilderCapturedValueResource admits only the direct user and clock
// formula resources assigned to matching field types in the userclock rows.
func processBuilderCapturedValueResource(pb processBuilderXML, name, dataType, expression, current string, definitions ...map[string]storage.ObjectDefinition) bool {
	if processBuilderFormulaExtraUse(pb, name) {
		return false
	}
	var expected storage.FieldType
	wrappedUser := strings.EqualFold(strings.TrimSpace(expression), "{!$User.Id}")
	userID := wrappedUser || strings.EqualFold(strings.TrimSpace(expression), "$User.Id")
	switch {
	case userID && strings.EqualFold(dataType, "String"):
		expected = storage.FieldString
	case strings.EqualFold(strings.TrimSpace(expression), "TODAY()") && strings.EqualFold(dataType, "Date"):
		expected = storage.FieldDate
	case strings.EqualFold(strings.TrimSpace(expression), "NOW()") && strings.EqualFold(dataType, "DateTime"):
		expected = storage.FieldDateTime
	default:
		return false
	}
	if len(definitions) == 0 {
		return false
	}
	for _, decision := range pb.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				if strings.EqualFold(condition.LeftValueReference, name) || strings.EqualFold(condition.RightValue.ElementReference, name) {
					return false
				}
			}
		}
	}
	for _, other := range pb.Formulas {
		if !strings.EqualFold(other.Name, name) && strings.Contains(strings.ToLower(other.Expression), strings.ToLower("{!"+name+"}")) {
			return false
		}
	}
	found := 0
	for _, update := range pb.RecordUpdates {
		for _, assignment := range update.InputAssignments {
			if !strings.EqualFold(assignment.Value.ElementReference, name) {
				continue
			}
			if len(update.Filters) != 1 || !strings.EqualFold(update.Filters[0].Field, "Id") || !strings.EqualFold(update.Filters[0].Operator, "EqualTo") || !strings.EqualFold(update.Filters[0].Value.ElementReference, current+".Id") {
				return false
			}
			var def storage.ObjectDefinition
			for object, candidate := range definitions[0] {
				if strings.EqualFold(object, update.Object) {
					def = candidate
					break
				}
			}
			if len(update.InputAssignments) != 1 {
				// Only the paired User lookup and NOW DateTime assignments are captured.
				if len(update.InputAssignments) != 2 {
					return false
				}
				user, clock := false, false
				for _, item := range update.InputAssignments {
					fieldName, ok := storage.ResolveFieldName(def, "", item.Field)
					if !ok {
						return false
					}
					fd := def.Fields[fieldName]
					for _, f := range pb.Formulas {
						if !strings.EqualFold(item.Value.ElementReference, f.Name) {
							continue
						}
						if (strings.EqualFold(strings.TrimSpace(f.Expression), "$User.Id") || strings.EqualFold(strings.TrimSpace(f.Expression), "{!$User.Id}")) && strings.EqualFold(f.DataType, "String") && fd.Type == storage.FieldReference && len(fd.ReferenceTo) == 1 && strings.EqualFold(fd.ReferenceTo[0], "User") {
							user = true
						}
						if strings.EqualFold(strings.TrimSpace(f.Expression), "NOW()") && strings.EqualFold(f.DataType, "DateTime") && fd.Type == storage.FieldDateTime {
							clock = true
						}
					}
				}
				if !user || !clock {
					return false
				}
			}
			field, ok := storage.ResolveFieldName(def, "", assignment.Field)
			if !ok {
				return false
			}
			fieldDefinition := def.Fields[field]
			matches := !wrappedUser && fieldDefinition.Type == expected
			if userID && fieldDefinition.Type == storage.FieldReference && len(fieldDefinition.ReferenceTo) == 1 && strings.EqualFold(fieldDefinition.ReferenceTo[0], "User") {
				matches = true
			}
			if !matches {
				return false
			}
			found++
		}
	}
	return found == 1
}

// processBuilderCapturedPriorBlankFormula bounds the IF/ISBLANK/PRIORVALUE
// Boolean decision captured in process_builder_review_prior.
func processBuilderCapturedPriorBlankFormula(pb processBuilderXML, name, dataType, expression, current string, definitions ...map[string]storage.ObjectDefinition) bool {
	if !strings.EqualFold(dataType, "Boolean") || len(definitions) == 0 || !processBuilderCapturedAliasUsage(pb, name) {
		return false
	}
	field := `\{!\s*` + regexp.QuoteMeta(current) + `\.([A-Za-z_][A-Za-z0-9_]*)\s*\}`
	pattern := `(?i)^\s*IF\s*\(\s*ISBLANK\s*\(\s*` + field + `\s*\)\s*,\s*TRUE\s*,\s*PRIORVALUE\s*\(\s*` + field + `\s*\)\s*<>\s*` + field + `\s*\)\s*$`
	match := regexp.MustCompile(pattern).FindStringSubmatch(html.UnescapeString(expression))
	if len(match) != 4 || !strings.EqualFold(match[2], match[3]) || strings.EqualFold(match[1], match[2]) {
		return false
	}
	var objectName string
	for _, meta := range pb.Metadata {
		if strings.EqualFold(meta.Name, "ObjectType") {
			objectName = meta.Value.StringValue
			break
		}
	}
	var def storage.ObjectDefinition
	for object, candidate := range definitions[0] {
		if strings.EqualFold(object, objectName) {
			def = candidate
			break
		}
	}
	if storage.IsKnownStandardObject(objectName) {
		def = def.Clone()
		if def.APIName == "" {
			def.APIName = objectName
		}
		if strings.EqualFold(def.Metadata[processBuilderPersonAccountsFeature], "true") {
			storage.EnsureStandardObjectFieldsForFeatures(&def, []string{"PersonAccounts"})
		} else {
			storage.EnsureStandardObjectFields(&def)
		}
	}
	first, okFirst := storage.ResolveFieldName(def, "", match[1])
	second, okSecond := storage.ResolveFieldName(def, "", match[2])
	return okFirst && okSecond && def.Fields[first].Type == storage.FieldString && strings.EqualFold(def.Fields[first].DisplayType, "STRING") && def.Fields[second].Type == storage.FieldString && strings.EqualFold(def.Fields[second].DisplayType, "STRING")
}

// processBuilderCapturedFormatFormula bounds the Picklist/DateTime formatting
// assignment observed in process_builder_review_format.
func processBuilderCapturedFormatFormula(pb processBuilderXML, name, dataType, expression, current string, definitions ...map[string]storage.ObjectDefinition) bool {
	if !strings.EqualFold(dataType, "String") || len(definitions) == 0 || processBuilderFormulaExtraUse(pb, name) {
		return false
	}
	field := `\{!\s*` + regexp.QuoteMeta(current) + `\.([A-Za-z_][A-Za-z0-9_]*)\s*\}`
	pattern := `(?i)^\s*TEXT\s*\(\s*` + field + `\s*\)\s*\+\s*"[^"\\]*"\s*\+\s*TEXT\s*\(\s*DATEVALUE\s*\(\s*` + field + `\s*\)\s*\)\s*$`
	match := regexp.MustCompile(pattern).FindStringSubmatch(html.UnescapeString(expression))
	if len(match) != 3 {
		return false
	}
	for _, decision := range pb.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				if strings.EqualFold(condition.LeftValueReference, name) || strings.EqualFold(condition.RightValue.ElementReference, name) {
					return false
				}
			}
		}
	}
	found := 0
	for _, update := range pb.RecordUpdates {
		for _, assignment := range update.InputAssignments {
			if !strings.EqualFold(assignment.Value.ElementReference, name) {
				continue
			}
			if len(update.InputAssignments) != 1 || len(update.Filters) != 1 || !strings.EqualFold(update.Filters[0].Field, "Id") || !strings.EqualFold(update.Filters[0].Operator, "EqualTo") || !strings.EqualFold(update.Filters[0].Value.ElementReference, current+".Id") {
				return false
			}
			var def storage.ObjectDefinition
			for object, candidate := range definitions[0] {
				if strings.EqualFold(object, update.Object) {
					def = candidate
					break
				}
			}
			first, okFirst := storage.ResolveFieldName(def, "", match[1])
			second, okSecond := storage.ResolveFieldName(def, "", match[2])
			dest, okDest := storage.ResolveFieldName(def, "", assignment.Field)
			if !okFirst || !okSecond || !okDest || def.Fields[first].Type != storage.FieldPicklist || def.Fields[second].Type != storage.FieldDateTime || def.Fields[dest].Type != storage.FieldString || !strings.EqualFold(def.Fields[dest].DisplayType, "STRING") {
				return false
			}
			found++
		}
	}
	return found == 1
}

// Captured formula assignments have matching source and destination types. A
// Boolean formula was observed as a true-valued decision, not as an assignment.
func processBuilderCapturedFormulaUse(pb processBuilderXML, name, dataType, expression, current string, definitions ...map[string]storage.ObjectDefinition) bool {
	expression = strings.TrimSpace(html.UnescapeString(expression))
	upper := strings.ToUpper(expression)
	boolFormula := upper == "TRUE" || upper == "FALSE" || strings.HasPrefix(upper, "ISNEW(") || strings.HasPrefix(upper, "PRIORVALUE(") || strings.HasPrefix(upper, "AND(")
	for _, action := range pb.ActionCalls {
		for _, input := range action.Inputs {
			if strings.EqualFold(input.Value.ElementReference, name) {
				return false
			}
		}
	}
	for _, decision := range pb.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				if strings.EqualFold(condition.RightValue.ElementReference, name) {
					if !strings.HasPrefix(upper, "DATE(") || !strings.EqualFold(dataType, "Date") || !strings.EqualFold(condition.Operator, "GreaterThan") || len(definitions) == 0 {
						return false
					}
					fieldName, ok := strings.CutPrefix(condition.LeftValueReference, current+".")
					if !ok {
						return false
					}
					var def storage.ObjectDefinition
					for _, meta := range pb.Metadata {
						if strings.EqualFold(meta.Name, "ObjectType") {
							for object, candidate := range definitions[0] {
								if strings.EqualFold(object, meta.Value.StringValue) {
									def = candidate
									break
								}
							}
						}
					}
					resolved, ok := storage.ResolveFieldName(def, "", fieldName)
					if !ok || def.Fields[resolved].Type != storage.FieldDate {
						return false
					}
				}
				if strings.EqualFold(condition.LeftValueReference, name) {
					if !boolFormula || !strings.EqualFold(dataType, "Boolean") {
						return false
					}
					if strings.EqualFold(condition.Operator, "EqualTo") && condition.RightValue.BooleanValue == "" && condition.RightValue.ElementReference == "" {
						return false
					}
					if condition.RightValue.BooleanValue != "" {
						switch strings.ToLower(condition.Operator) {
						case "equalto":
							if !strings.EqualFold(condition.RightValue.BooleanValue, "true") {
								return false
							}
						case "notequalto", "greaterthan", "greaterthanorequalto", "isnull":
							return false
						}
					}
					if strings.EqualFold(condition.Operator, "EqualTo") && strings.HasPrefix(condition.RightValue.ElementReference, current+".") {
						return false
					}
					pattern := `(?i)\{!\s*` + regexp.QuoteMeta(current) + `\.([A-Za-z_][A-Za-z0-9_]*)\s*\}`
					refs := regexp.MustCompile(pattern).FindAllStringSubmatch(expression, -1)
					if len(refs) > 0 {
						if len(definitions) == 0 {
							return false
						}
						var def storage.ObjectDefinition
						for _, meta := range pb.Metadata {
							if strings.EqualFold(meta.Name, "ObjectType") {
								for object, candidate := range definitions[0] {
									if strings.EqualFold(object, meta.Value.StringValue) {
										def = candidate
										break
									}
								}
							}
						}
						want := storage.FieldString
						if strings.HasPrefix(upper, "AND(") {
							want = storage.FieldBoolean
						}
						for _, ref := range refs {
							field, ok := storage.ResolveFieldName(def, "", ref[1])
							if !ok || def.Fields[field].Type != want {
								return false
							}
						}
					}
				}
			}
		}
	}
	for _, update := range pb.RecordUpdates {
		for _, filter := range update.Filters {
			if strings.EqualFold(filter.Value.ElementReference, name) {
				return false
			}
		}
		for _, assignment := range update.InputAssignments {
			if !strings.EqualFold(assignment.Value.ElementReference, name) {
				continue
			}
			if boolFormula || len(definitions) == 0 {
				return false
			}
			var want storage.FieldType
			switch {
			case strings.EqualFold(dataType, "Number"):
				want = storage.FieldDecimal
			case strings.EqualFold(dataType, "String"):
				want = storage.FieldString
			default:
				return false
			}
			var def storage.ObjectDefinition
			for object, candidate := range definitions[0] {
				if strings.EqualFold(object, update.Object) {
					def = candidate
					break
				}
			}
			field, ok := storage.ResolveFieldName(def, "", assignment.Field)
			if !ok || def.Fields[field].Type != want {
				return false
			}
			pattern := `(?i)\{!\s*` + regexp.QuoteMeta(current) + `\.([A-Za-z_][A-Za-z0-9_]*)\s*\}`
			refs := regexp.MustCompile(pattern).FindAllStringSubmatch(expression, -1)
			if len(refs) != 1 {
				return false
			}
			source, ok := storage.ResolveFieldName(def, "", refs[0][1])
			if !ok {
				return false
			}
			sourceType := want
			if strings.HasPrefix(upper, "IF(") {
				sourceType = storage.FieldString
			}
			if def.Fields[source].Type != sourceType {
				return false
			}
			if strings.HasPrefix(upper, "BLANKVALUE(") && !strings.EqualFold(source, field) {
				return false
			}
		}
	}
	return true
}

// The global decision form is backed by the globalcontrol native rows.
func processBuilderCapturedUserAliasFormula(expression string) bool {
	matched, _ := regexp.MatchString(`(?i)^\s*NOT\s*\(\s*ISBLANK\s*\(\s*\$User\.Alias\s*\)\s*\)\s*$`, html.UnescapeString(expression))
	return matched
}

// processBuilderCapturedFormula admits the expression structures observed in
// process_builder_shapes and process_builder_wait. Evaluator support alone is
// not native evidence for a Process Builder formula composition.
func processBuilderCapturedFormula(expression, current string) bool {
	field := `\{!\s*` + regexp.QuoteMeta(current) + `\.[A-Za-z_][A-Za-z0-9_]*\s*\}`
	number := `-?[0-9]+(?:\.[0-9]+)?`
	quoted := `"[^"\\]*"`
	space := `\s*`
	patterns := []string{
		`true`,
		`false`,
		`DATE\(` + space + number + space + `,` + space + number + space + `,` + space + number + space + `\)`,
		`ISNEW\(` + space + `\)`,
		`PRIORVALUE\(` + space + field + space + `\)` + space + `=` + space + quoted,
		`BLANKVALUE\(` + space + field + space + `,` + space + number + space + `\)` + space + `\+` + space + number,
		`BLANKVALUE\(` + space + field + space + `,` + space + quoted + space + `\)` + space + `\+` + space + quoted,
		`IF\(` + space + field + space + `=` + space + quoted + space + `,` + space + number + space + `,` + space + number + space + `\)`,
		`AND\(` + space + field + space + `=` + space + `FALSE` + space + `,` + space + `ISCHANGED\(` + space + field + space + `\)` + space + `\)`,
	}
	for index, pattern := range patterns {
		if matched, _ := regexp.MatchString(`(?i)^\s*`+pattern+`\s*$`, html.UnescapeString(expression)); matched {
			if index == len(patterns)-1 {
				fields := regexp.MustCompile(`(?i)`+field).FindAllString(expression, -1)
				return len(fields) == 2 && strings.EqualFold(strings.ReplaceAll(fields[0], " ", ""), strings.ReplaceAll(fields[1], " ", ""))
			}
			return true
		}
	}
	return false
}

// flowProcessPriorValueSources admits a process record update's prior-record
// value, which process_builder_shapes pri rows pin at API 62.0 and 67.0. The
// binding map is private; other flows have no such row and keep their path.
func flowProcessPriorValueSources(raw flowXML, variables map[string]flowVariableXML) {
	if !strings.EqualFold(strings.TrimSpace(raw.ProcessType), "Workflow") {
		return
	}
	for _, update := range raw.RecordUpdates {
		for _, assignment := range update.Fields {
			reference := strings.TrimSpace(assignment.Value.ElementReference)
			if field, found := strings.CutPrefix(strings.ToLower(reference), "$record__prior."); found && field != "" {
				variables[strings.ToLower(reference)] = flowVariableXML{Name: reference}
			}
		}
	}
}

// processBuilderCapturedPair checks the two dependent, literal Text updates
// captured in process_builder_review_multiprocess and process_builder_review_reverse.
func processBuilderCapturedPair(object string, paths []string, definitions map[string]storage.ObjectDefinition) bool {
	if len(paths) != 2 {
		return false
	}
	type part struct{ criterionField, criterionValue, targetField, value string }
	var pair []part
	var def storage.ObjectDefinition
	for name, candidate := range definitions {
		if strings.EqualFold(name, object) {
			def = candidate
			break
		}
	}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		var raw flowXML
		var pb processBuilderXML
		if xml.Unmarshal(source, &raw) != nil || xml.Unmarshal(source, &pb) != nil {
			return false
		}
		current, trigger := "", ""
		for _, m := range pb.Metadata {
			if strings.EqualFold(m.Name, "ObjectVariable") {
				current = m.Value.ElementReference
			}
			if strings.EqualFold(m.Name, "TriggerType") {
				trigger = m.Value.StringValue
			}
		}
		if current == "" || !strings.EqualFold(trigger, "onAllChanges") || len(raw.Decisions) != 1 || len(raw.RecordUpdates) != 1 || len(raw.Formulas)+len(raw.Constants)+len(raw.TextTemplates)+len(raw.Assignments)+len(raw.Screens)+len(raw.Loops)+len(raw.Subflows)+len(raw.ActionCalls)+len(raw.RecordLookups)+len(raw.RecordCreates)+len(raw.RecordDeletes)+len(raw.CustomErrors)+len(raw.ApexPluginCalls)+len(raw.Transforms)+len(raw.CollectionProcessors)+len(raw.OrchestratedStages)+len(raw.ScheduledPaths)+len(raw.Stages)+len(raw.ExitRules)+len(raw.Experiments)+len(raw.Segment)+len(raw.Schedule)+len(raw.Waits) != 0 {
			return false
		}
		d, u := raw.Decisions[0], raw.RecordUpdates[0]
		if !strings.EqualFold(raw.StartElementReference, d.Name) || d.DefaultConnector.TargetReference != "" || len(d.Rules) != 1 {
			return false
		}
		rule := d.Rules[0]
		if len(rule.Conditions) != 1 || !strings.EqualFold(rule.ConditionLogic, "and") || !strings.EqualFold(rule.Connector.TargetReference, u.Name) {
			return false
		}
		c := rule.Conditions[0]
		prefix := current + "."
		if !strings.HasPrefix(strings.ToLower(c.LeftValueReference), strings.ToLower(prefix)) || !strings.EqualFold(c.Operator, "EqualTo") || c.RightValue.StringValue == "" || c.RightValue != (flowValueXML{StringValue: c.RightValue.StringValue}) {
			return false
		}
		if !strings.EqualFold(u.Object, object) || u.InputReference != "" || len(u.Filters) != 1 || len(u.Fields) != 1 || !strings.EqualFold(u.FilterLogic, "and") || u.Connector.TargetReference != "" || u.FaultConnector.TargetReference != "" {
			return false
		}
		f, a := u.Filters[0], u.Fields[0]
		if !strings.EqualFold(f.Field, "Id") || !strings.EqualFold(f.Operator, "EqualTo") || !strings.EqualFold(f.Value.ElementReference, current+".Id") || f.Value != (flowValueXML{ElementReference: f.Value.ElementReference}) || a.Value.StringValue == "" || a.Value != (flowValueXML{StringValue: a.Value.StringValue}) {
			return false
		}
		criterion := c.LeftValueReference[len(prefix):]
		for _, field := range []string{criterion, a.Field} {
			name, ok := storage.ResolveFieldName(def, "", field)
			if !ok || def.Fields[name].Type != storage.FieldString || !strings.EqualFold(def.Fields[name].DisplayType, "STRING") {
				return false
			}
		}
		pair = append(pair, part{criterion, c.RightValue.StringValue, a.Field, a.Value.StringValue})
	}
	dependent := func(a, b part) bool {
		return strings.EqualFold(a.targetField, b.criterionField) && a.value == b.criterionValue && !strings.EqualFold(a.criterionField, a.targetField) && !strings.EqualFold(a.criterionField, b.targetField) && !strings.EqualFold(a.targetField, b.targetField)
	}
	return dependent(pair[0], pair[1]) || dependent(pair[1], pair[0])
}

// leaveUncoveredProcessesUnrun applies the checks that read other flows.
// The captured dependent pair is admitted; other multiple-process shapes stay
// unrun. Flow-action targets must match the separately captured graphs.
func leaveUncoveredProcessesUnrun(idx *Index, paths []string, definitions map[string]storage.ObjectDefinition, features ...[]string) error {
	counts := make(map[string]int)
	processPaths := make(map[string][]string)
	flows := make(map[string]storage.FlowRule)
	for i, flow := range idx.Flows {
		if flowIsActiveProcess(flow) && strings.TrimSpace(flow.ObjectName) != "" {
			counts[strings.ToLower(flow.ObjectName)]++
			processPaths[strings.ToLower(flow.ObjectName)] = append(processPaths[strings.ToLower(flow.ObjectName)], paths[i])
		}
		for _, rule := range flow.Rules {
			flows[strings.ToLower(rule.Name)] = rule
		}
	}
	afterSave := make(map[string][]storage.FlowRule)
	for _, flow := range idx.Flows {
		if flowIsActiveProcess(flow) || strings.TrimSpace(flow.ObjectName) == "" {
			continue
		}
		for _, rule := range flow.Rules {
			if rule.Active && strings.EqualFold(strings.TrimSpace(rule.TriggerType), "RecordAfterSave") {
				afterSave[strings.ToLower(flow.ObjectName)] = append(afterSave[strings.ToLower(flow.ObjectName)], rule)
				break
			}
		}
	}
	for i, flow := range idx.Flows {
		if !flowIsActiveProcess(flow) || strings.TrimSpace(flow.ObjectName) == "" {
			continue
		}
		skip := ""
		if counts[strings.ToLower(flow.ObjectName)] > 1 && !processBuilderCapturedPair(flow.ObjectName, processPaths[strings.ToLower(flow.ObjectName)], definitions) {
			skip = "several active processes on one object"
		} else if after := afterSave[strings.ToLower(flow.ObjectName)]; len(after) > 1 {
			if len(after) != 2 {
				var def storage.ObjectDefinition
				for name, candidate := range definitions {
					if strings.EqualFold(name, flow.ObjectName) {
						def = candidate
						break
					}
				}
				if _, ok := storage.CapturedFourAfterSavePlan(after, def, definitions); !ok && !processBuilderCapturedMixedAfterSave(idx, flow.ObjectName, after, def, definitions) && !capturedCaseAfterSave(after, def) && !processBuilderCapturedBranchedAfterSave(idx, flow.ObjectName, after, def, definitions, features...) {
					skip = "several after-save flows on its object"
				}
			} else {
				_, first := after[0].SimpleProcessAfterSaveAction()
				_, second := after[1].SimpleProcessAfterSaveAction()
				if !first || !second {
					skip = "several after-save flows on its object"
				}
			}
		}
		if skip == "" {
			skip = processBuilderUncoveredFlowAction(flow, flows, definitions)
		}
		if skip == "" {
			continue
		}
		source, err := os.ReadFile(paths[i])
		if err != nil {
			return err
		}
		unrun, diagnostics, err := compileFlowSource(paths[i], source, skip, definitions)
		if err != nil {
			return err
		}
		idx.Flows[i] = unrun
		for _, item := range diagnostics {
			if strings.Contains(item.Message, "Process Builder process is not run") {
				idx.Diagnostics = append(idx.Diagnostics, item)
			}
		}
	}
	return nil
}

func processBuilderUncoveredFlowAction(flow Flow, flows map[string]storage.FlowRule, definitions ...map[string]storage.ObjectDefinition) string {
	skip := ""
	check := func(steps []storage.FlowStep) {
		for _, step := range steps {
			if step.Kind != "action" || !strings.EqualFold(strings.TrimSpace(step.Action.ActionType), "flow") || skip != "" {
				continue
			}
			target, found := flows[strings.ToLower(strings.TrimSpace(step.Action.ActionName))]
			if !found || !target.Active || strings.TrimSpace(target.TriggerType) != "" {
				skip = fmt.Sprintf("flow action %q without its flow's source", step.Action.ActionName)
			} else if _, linear := target.ProcessRecordUpdates(); !linear {
				skip = fmt.Sprintf("flow action %q on a flow with other steps", step.Action.ActionName)
			} else if !processBuilderCapturedSubflow(flow.ObjectName, step.Action, target, definitions...) {
				skip = fmt.Sprintf("flow action %q with uncaptured update shape", step.Action.ActionName)
			}
		}
	}
	var walk func([]storage.FlowStep)
	walk = func(steps []storage.FlowStep) {
		check(steps)
		for _, step := range steps {
			for _, branch := range step.Branches {
				walk(branch.Steps)
			}
		}
	}
	for _, rule := range flow.Rules {
		walk(rule.Steps)
		for _, branch := range rule.Branches {
			walk(branch.Steps)
		}
		for _, steps := range rule.StepTargets {
			walk(steps)
		}
	}
	return skip
}

// processBuilderCapturedSubflow admits the input and update structures observed
// in flowact and flow2. Other flows remain unrun until captured.
func processBuilderCapturedSubflow(objectName string, action storage.FlowAction, target storage.FlowRule, definitions ...map[string]storage.ObjectDefinition) bool {
	if steps, ok := target.CapturedProcessDependentLookupSteps(); ok {
		if len(definitions) == 0 || len(action.Inputs) != 2 || len(action.Outputs) != 0 || !strings.EqualFold(steps[0].RecordLookup.ObjectName, objectName) {
			return false
		}
		recordVar := steps[0].RecordLookup.Criteria[0].SourceField
		rootVar := steps[0].RecordLookup.Criteria[1].SourceField
		linkField := steps[0].RecordLookup.Criteria[1].Field
		boundRecord, boundRoot := false, false
		for _, input := range action.Inputs {
			if input.Formula != "" || input.LiteralValue != "" || input.ActionOutput != "" {
				return false
			}
			if strings.EqualFold(input.Field, recordVar) && strings.EqualFold(input.SourceField, "Id") {
				boundRecord = true
			} else if strings.EqualFold(input.Field, rootVar) {
				_, relationshipID := target.CapturedProcessRelationshipIDBinding(input)
				if !strings.EqualFold(input.SourceField, linkField) && !relationshipID {
					return false
				}
				boundRoot = true
			} else {
				return false
			}
		}
		if !boundRecord || !boundRoot {
			return false
		}
		var def, related storage.ObjectDefinition
		for name, candidate := range definitions[0] {
			if strings.EqualFold(name, objectName) {
				def = candidate
			}
			if strings.EqualFold(name, steps[1].RecordLookup.ObjectName) {
				related = candidate
			}
		}
		if storage.IsKnownStandardObject(objectName) {
			def = def.Clone()
			if def.APIName == "" {
				def.APIName = objectName
			}
			storage.EnsureStandardObjectFields(&def)
		}
		link, ok := storage.ResolveFieldName(def, "", linkField)
		if !ok {
			return false
		}
		linkDef := def.Fields[link]
		if linkDef.Type != storage.FieldReference || len(linkDef.ReferenceTo) != 1 || !strings.EqualFold(linkDef.ReferenceTo[0], steps[1].RecordLookup.ObjectName) {
			return false
		}
		_, sourceName, _ := strings.Cut(steps[2].Assignment.SourceField, ".")
		source, okSource := storage.ResolveFieldName(related, "", sourceName)
		dest, okDest := storage.ResolveFieldName(def, "", steps[3].RecordUpdate.InputAssignments[0].Field)
		if !okSource || !okDest {
			return false
		}
		src, dst := related.Fields[source], def.Fields[dest]
		if src.Type != storage.FieldString || dst.Type != storage.FieldString {
			return false
		}
		standardText := strings.EqualFold(sourceName, "Name") && !src.LongTextArea && strings.EqualFold(dst.DisplayType, "TEXTAREA") && !dst.LongTextArea
		longTextToHTML := src.LongTextArea && strings.EqualFold(dst.DisplayType, "RICHTEXTAREA")
		if target.Variables[0].IsOutput {
			return longTextToHTML
		}
		return standardText || longTextToHTML
	}
	if steps, ok := target.CapturedProcessRecordActionSteps(); ok {
		if len(definitions) == 0 || len(action.Inputs) != 1 || len(action.Outputs) != 0 || !strings.EqualFold(target.Variables[0].ObjectType, objectName) {
			return false
		}
		input := action.Inputs[0]
		if !strings.EqualFold(input.Field, target.Variables[0].Name) || !strings.EqualFold(input.SourceField, "$Record") || input.Formula != "" || input.LiteralValue != "" || input.ActionOutput != "" {
			return false
		}
		var def storage.ObjectDefinition
		for name, candidate := range definitions[0] {
			if strings.EqualFold(name, objectName) {
				def = candidate
				break
			}
		}
		if storage.IsKnownStandardObject(objectName) {
			def = def.Clone()
			if def.APIName == "" {
				def.APIName = objectName
			}
			storage.EnsureStandardObjectFields(&def)
		}
		_, field, _ := strings.Cut(steps[1].Assignment.Target, ".")
		resolved, found := storage.ResolveFieldName(def, "", field)
		if !found || def.Fields[resolved].Type != storage.FieldString {
			return false
		}
		display := def.Fields[resolved].DisplayType
		return strings.EqualFold(display, "TEXTAREA") || strings.EqualFold(display, "PHONE")
	}
	updates, ok := target.ProcessRecordUpdates()
	lookupSteps, lookupShape := target.CapturedProcessLookupSteps()
	if !ok || len(updates) < 1 || len(updates) > 2 || objectName == "" || (!lookupShape && len(target.Variables) != 1) || len(action.Inputs) != 1 || len(action.Outputs) != 0 || len(definitions) == 0 {
		return false
	}
	variable, input := target.Variables[0], action.Inputs[0]
	if lookupShape {
		for _, candidate := range target.Variables {
			if candidate.IsInput {
				variable = candidate
				break
			}
		}
		for _, step := range lookupSteps[:2] {
			if !strings.EqualFold(step.RecordLookup.ObjectName, objectName) {
				return false
			}
		}
	}
	if !variable.IsInput || variable.IsOutput || variable.IsCollection || !strings.EqualFold(variable.DataType, "String") ||
		!strings.EqualFold(input.Field, variable.Name) || !strings.EqualFold(input.SourceField, "Id") || input.Formula != "" || input.LiteralValue != "" || input.ActionOutput != "" {
		return false
	}
	var definition storage.ObjectDefinition
	for name, candidate := range definitions[0] {
		if strings.EqualFold(name, objectName) {
			definition = candidate
			break
		}
	}
	fields := make(map[string]bool)
	for _, update := range updates {
		if !strings.EqualFold(update.ObjectName, objectName) || update.InputReference != "" || len(update.Criteria) != 1 || len(update.InputAssignments) != 1 {
			return false
		}
		criterion, assignment := update.Criteria[0], update.InputAssignments[0]
		if !strings.EqualFold(criterion.Field, "Id") || !strings.EqualFold(criterion.Operation, "equals") || !strings.EqualFold(criterion.SourceField, variable.Name) || criterion.Value != "" ||
			assignment.Field == "" || assignment.Formula != "" || assignment.ActionOutput != "" || (!lookupShape && (assignment.LiteralValue == "" || assignment.SourceField != "")) || (lookupShape && (assignment.LiteralValue != "" || !strings.EqualFold(assignment.SourceField, lookupSteps[2].Assignment.Target))) {
			return false
		}
		resolved, found := storage.ResolveFieldName(definition, "", assignment.Field)
		if !found || definition.Fields[resolved].Type != storage.FieldString {
			return false
		}
		key := strings.ToLower(assignment.Field)
		if fields[key] {
			return false
		}
		fields[key] = true
	}
	for _, step := range target.Steps {
		if step.FaultTarget != "" || len(step.FaultBranch) != 0 {
			return false
		}
	}
	for _, steps := range target.StepTargets {
		for _, step := range steps {
			if step.FaultTarget != "" || len(step.FaultBranch) != 0 {
				return false
			}
		}
	}
	return true
}

func flowIsActiveProcess(flow Flow) bool {
	for _, rule := range flow.Rules {
		if rule.Active && strings.EqualFold(strings.TrimSpace(rule.ProcessType), "Workflow") {
			return true
		}
	}
	return false
}

func capturedCaseAfterSave(rules []storage.FlowRule, def storage.ObjectDefinition) bool {
	_, ok := storage.CapturedCaseAfterSavePlan(rules, def)
	return ok
}
