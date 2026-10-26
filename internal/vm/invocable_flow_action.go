package vm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Local action shapes share the Flow interpreter for scalar record and primitive
// variables, assignments, decisions, lookups, subflows, updates and supported creates.
// Collections, fault connectors and other steps retain an explicit boundary.
func invocableFlowActionSupported(rule storage.FlowRule) bool {
	if !strings.EqualFold(rule.ProcessType, "AutoLaunchedFlow") || len(rule.Variables) == 0 || len(rule.Steps) == 0 {
		return false
	}
	for _, variable := range rule.Variables {
		if variable.IsCollection {
			return false
		}
		if strings.EqualFold(variable.DataType, "Boolean") || strings.EqualFold(variable.DataType, "Number") {
			continue
		}
		if strings.EqualFold(variable.DataType, "String") {
			continue
		}
		if strings.EqualFold(variable.DataType, "Date") && !variable.IsInput && !variable.IsOutput {
			continue
		}
		if !strings.EqualFold(variable.DataType, "SObject") || strings.TrimSpace(variable.ObjectType) == "" {
			return false
		}
	}

	return invocableFlowActionStepsSupportedWithTargets(rule.Steps, rule.StepTargets)
}

func invocableFlowActionStepsSupported(steps []storage.FlowStep) bool {
	return invocableFlowActionStepsSupportedWithTargets(steps, nil)
}

func invocableFlowActionStepsSupportedWithTargets(steps []storage.FlowStep, targets map[string][]storage.FlowStep) bool {
	state := make(map[string]uint8)
	activeVisitKeys := make(map[string]string)
	var walk func([]storage.FlowStep) bool
	walk = func(steps []storage.FlowStep) bool {
		for _, step := range steps {
			if len(step.FaultBranch) != 0 || step.FaultTarget != "" {
				return false
			}
			switch step.Kind {
			case "reference":
				key := strings.ToLower(strings.TrimSpace(step.Target))
				target, found := targets[key]
				if !found {
					return false
				}
				visitKey := strings.ToLower(strings.TrimSpace(step.VisitKey))
				if entryVisitKey, active := activeVisitKeys[key]; active {
					if visitKey != "" && entryVisitKey == visitKey {
						continue
					}
					return false
				}
				if state[key] == 2 {
					continue
				}
				state[key] = 1
				activeVisitKeys[key] = visitKey
				valid := walk(target)
				delete(activeVisitKeys, key)
				if !valid {
					return false
				}
				state[key] = 2
			case "assignment":
				// AddItem on record.field appends to a multi-select or text value.
				addItem := strings.EqualFold(step.Assignment.Operator, "AddItem") && strings.Contains(step.Assignment.Target, ".")
				addNumber := strings.EqualFold(step.Assignment.Operator, "Add") && step.Assignment.Target != "" && !strings.Contains(step.Assignment.Target, ".")
				if !strings.EqualFold(step.Assignment.Operator, "Assign") && !addItem && !addNumber {
					return false
				}
			case "decision":
				for _, branch := range step.Branches {
					if !walk(branch.Steps) {
						return false
					}
				}
			case "recordLookup":
				lookup := step.RecordLookup
				automatic := lookup.GetFirstRecordOnly && lookup.StoreOutputAutomatically
				explicit := len(lookup.OutputAssignments) > 0 && !lookup.StoreOutputAutomatically && strings.TrimSpace(lookup.OutputReference) == ""
				if !automatic && !explicit {
					return false
				}
			case "recordCreate":
				create := step.RecordCreate
				scalarObject := strings.EqualFold(create.ObjectName, "Account") || strings.EqualFold(create.ObjectName, "Task") || hasSuffixFold(create.ObjectName, "__c")
				if !scalarObject || create.InputReference != "" {
					return false
				}
			case "recordUpdate", "subflow":
			case "action":
				if !strings.EqualFold(step.Action.ActionType, "apex") || strings.TrimSpace(step.Action.ClassName) == "" {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	return walk(steps)
}

type invocableFlowActionFault struct{ code, message string }

func (e *invocableFlowActionFault) Error() string { return e.message }

// Replacing the sole scalar Account
// input with an undeclared Account-valued parameter returns a normal error DTO.
// Other undeclared, unavailable, missing or wrongly typed inputs keep their
// existing boundary; no Flow executes on this rejection.
type invocableFlowActionParameterError struct{ name string }

func (e *invocableFlowActionParameterError) Error() string {
	return fmt.Sprintf("The flow doesn't have a variable with the name %q, or the variable isn't available for input.", e.name)
}

func invocableFlowActionUnknownAccountInput(rule storage.FlowRule, invocation, name, value Value) bool {
	if name.Kind != ValueString || value.Kind != ValueObject || !strings.EqualFold(value.Type, "Account") || len(invocation.Map) != 1 {
		return false
	}
	inputs := 0
	for _, variable := range rule.Variables {
		if variable.Name == name.Text {
			return false
		}
		if !variable.IsInput {
			continue
		}
		inputs++
		if variable.IsCollection || !strings.EqualFold(variable.DataType, "SObject") || !strings.EqualFold(variable.ObjectType, "Account") {
			return false
		}
	}
	return inputs == 1
}

func (vm *VM) invokeInvocableFlowAction(action, invocation Value, executionResult *Result) (Value, bool, error) {
	if len(vm.callStack) > 0 {
		vm.callStack[len(vm.callStack)-1].invocableFlowOutput = nil
	}
	name := invocableActionDisplayName(action)
	rule, found := vm.autolaunchedFlowRule(name)
	if !found {
		return Null, false, &invocableMissingActionError{name: name}
	}
	if !invocableFlowActionSupported(rule) {
		return Null, false, unsupportedCallError("Invocable.Action.invoke hosted Flow action " + name)
	}
	// Validate before materializing input or executing the Flow. The captured
	// one-Account unknown-name rejection has its own normal Result error.
	for encoded, key := range invocation.MapKeys {
		declared := false
		for _, variable := range rule.Variables {
			if key.Kind != ValueString || key.Text != variable.Name || !variable.IsInput {
				continue
			}
			value := invocation.Map[encoded]
			if strings.EqualFold(variable.DataType, "Boolean") {
				declared = value.Kind == ValueBool
			} else if strings.EqualFold(variable.DataType, "Number") {
				declared = value.Kind == ValueInt || value.Kind == ValueDecimal
			} else if strings.EqualFold(variable.DataType, "String") {
				declared = value.Kind == ValueString
			} else {
				declared = value.Kind == ValueNull || (value.Kind == ValueObject && strings.EqualFold(variable.DataType, "SObject") &&
					strings.EqualFold(value.Type, variable.ObjectType))
			}
			break
		}
		if !declared {
			if invocableFlowActionUnknownAccountInput(rule, invocation, key, invocation.Map[encoded]) {
				return Null, false, &invocableFlowActionParameterError{name: key.Text}
			}
			return Null, false, unsupportedCallError("Invocable.Action.invoke hosted Flow action " + name)
		}
	}
	for _, variable := range rule.Variables {
		// An omitted SObject input is an empty record, as for the Flow interview.
		if variable.IsInput && !strings.EqualFold(variable.DataType, "String") && !strings.EqualFold(variable.DataType, "SObject") && !(strings.EqualFold(variable.DataType, "Number") && variable.DefaultValue != nil) {
			if _, supplied := invocation.Map[mapKey(String(variable.Name))]; !supplied && !invocableFlowBooleanAssignedBeforeUse(rule, variable) {
				return Null, false, unsupportedCallError("Invocable.Action.invoke hosted Flow action " + name)
			}
		}
	}
	interview := Object("Flow.Interview")
	interview.Fields["flowName"] = String(name)
	interview.Fields["variables"] = invocableFlowActionInputSnapshot(invocation, rule)
	input, _, err := vm.flowInterviewInput(interview)
	if err != nil {
		return Null, false, err
	}
	// An explicit null Account is a declared empty record, seeded by the shared
	// interpreter; it must not also appear as a scalar output.
	for _, variable := range rule.Variables {
		if strings.EqualFold(variable.DataType, "SObject") {
			delete(input.Scalars, variable.Name)
		}
	}
	engine := vm.newDMLEngine(executionResult)
	engine.InvocableFlow = true
	engine.InvocableFlowRecordCreator = func(record storage.Record) (dml.Result, error) {
		return vm.insertInvocableFlowRecord(record, executionResult)
	}
	engine.InvocableFlowRecordUpdater = func(record storage.Record) (dml.Result, error) {
		return vm.mutateInvocableFlowRecord("update", record, executionResult)
	}
	output, err := engine.RunAutolaunchedFlowWithOutput(rule, input)
	outputs := typedMap("Map<String,Object>")
	set := func(name string, value Value) {
		key := String(name)
		encoded := mapKey(key)
		outputs.Map[encoded] = value
		outputs.MapKeys[encoded] = key
		outputs.MapOrder = append(outputs.MapOrder, encoded)
	}
	// GUID is an opaque local interview identity; fixtures scrub its value.
	set("Flow__InterviewGuid", String(vm.nextDeterministicUUID()))
	if err != nil {
		set("Flow__InterviewStatus", String("Error"))
		if fault, ok := engine.InvocableFlowActionError(err); ok {
			return outputs, false, &invocableFlowActionFault{code: fault.StatusCode, message: fault.Message}
		}
		return Null, false, err
	}
	set("Flow__InterviewStatus", String("Finished"))
	for _, variable := range rule.Variables {
		if !variable.IsOutput {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(variable.Name))
		value := Null
		if record, ok := output.Records[key]; ok {
			value = vmValueFromRecord(record)
			// Preserve explicit nulls for caller DML write-back, without making
			// the caller's original input object an alias of the Flow output.
			for field := range record.Fields {
				markExplicitSObjectField(&value, field)
			}
			for field, isNull := range record.ExplicitNulls {
				if isNull {
					markExplicitSObjectField(&value, field)
				}
			}
			if record.ID != "" {
				markExplicitSObjectField(&value, "Id")
			}
		} else if scalar, ok := output.Scalars[key]; ok {
			value = vmValueFromStorage(scalar)
			if strings.EqualFold(variable.DataType, "Number") && scalar.Kind != storage.ValueNull {
				if number, err := strconv.ParseFloat(value.String(), 64); err == nil {
					value = decimalAsDouble(Decimal(number))
				}
			}
		}
		set(variable.Name, value)
	}
	vm.rememberInvocableFlowRecordOutput(rule, outputs)
	return outputs, true, nil
}

func (vm *VM) insertInvocableFlowRecord(record storage.Record, executionResult *Result) (dml.Result, error) {
	return vm.mutateInvocableFlowRecord("insert", record, executionResult)
}

func (vm *VM) mutateInvocableFlowRecord(operation string, record storage.Record, executionResult *Result) (dml.Result, error) {
	value := vmValueFromRecord(record)
	for field := range record.Fields {
		markExplicitSObjectField(&value, field)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			markExplicitSObjectField(&value, field)
		}
	}
	results, err := vm.applyDML(operation, value, true, "", dml.Options{}, executionResult)
	if err != nil {
		return dml.Result{}, err
	}
	if len(results) != 1 {
		return dml.Result{}, fmt.Errorf("Flow %s %s returned %d results", record.Object, operation, len(results))
	}
	return results[0], nil
}

// A scalar Account input is a Flow value, even when its source is a read-only
// Trigger record. Detach that value without removing the original write guard.
func invocableFlowActionInputSnapshot(invocation Value, rule storage.FlowRule) Value {
	input := invocation
	copied := false
	for key, value := range invocation.Map {
		if !invocableFlowActionDeclaresSObjectType(rule, value.Type) {
			continue
		}
		reason, readOnly := sobjectReadOnlyReason(value)
		trigger := isTriggerSObject(value) && readOnly && reason == "trigger record"
		_, queried := value.Fields[sobjectQueriedFieldsField]
		if !trigger && !queried {
			continue
		}
		if !copied {
			// Copy only the input map; unrelated values keep their identities.
			input.Map = make(map[string]Value, len(invocation.Map))
			for encoded, original := range invocation.Map {
				input.Map[encoded] = original
			}
			copied = true
		}
		copy := cloneValue(value)
		if trigger {
			deleteObjectField(copy.Fields, sobjectReadOnlyField)
		}
		if queried {
			for field := range copy.Fields {
				if strings.EqualFold(field, "RecordTypeId") {
					markExplicitSObjectField(&copy, field)
				}
			}
		}
		input.Map[key] = copy
	}
	return input
}

func invocableFlowActionDeclaresSObjectType(rule storage.FlowRule, objectType string) bool {
	for _, variable := range rule.Variables {
		if strings.EqualFold(variable.DataType, "SObject") && strings.EqualFold(variable.ObjectType, objectType) {
			return true
		}
	}
	return false
}
