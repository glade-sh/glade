package vm

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Output observations belong to the invoking method frame, not the transaction.
type invocableFlowOutputObservation struct {
	ref    uint64
	name   string
	absent bool
}

func (vm *VM) rememberInvocableFlowRecordOutput(rule storage.FlowRule, outputs Value) {
	if len(vm.callStack) == 0 || vm.currentTrigger || !vm.triggerGlobals["Trigger.isBefore"].Bool || !(vm.triggerGlobals["Trigger.isInsert"].Bool || vm.triggerGlobals["Trigger.isUpdate"].Bool) || len(rule.Variables) != 2 {
		return
	}
	name := ""
	for _, variable := range rule.Variables {
		if variable.IsCollection {
			return
		}
		if strings.EqualFold(variable.DataType, "SObject") && strings.EqualFold(variable.ObjectType, "Account") && variable.IsInput && !variable.IsOutput {
			name = variable.Name
		} else if !strings.EqualFold(variable.DataType, "Boolean") || variable.IsInput || !variable.IsOutput {
			return
		}
	}
	if name != "" {
		vm.callStack[len(vm.callStack)-1].invocableFlowOutput = &invocableFlowOutputObservation{ref: outputs.Ref, name: name}
	}
}

func (vm *VM) observeInvocableFlowRecordOutput(receiver, key Value, found bool) {
	if found || key.Kind != ValueString || len(vm.callStack) == 0 {
		return
	}
	observation := vm.callStack[len(vm.callStack)-1].invocableFlowOutput
	if observation != nil && observation.ref != 0 && observation.ref == receiver.Ref && observation.name == key.Text {
		observation.absent = true
	}
}

type invocableFlowCallerTriggerFailure struct {
	cause  error
	detail string
}

func (e *invocableFlowCallerTriggerFailure) Error() string { return e.detail }
func (e *invocableFlowCallerTriggerFailure) Unwrap() error { return e.cause }

func (vm *VM) wrapInvocableFlowCallerTriggerFailure(trigger Trigger, records []storage.Record, err error) error {
	if trigger.Timing != triggerTimingBefore || (trigger.Operation != "insert" && trigger.Operation != "update") || !strings.EqualFold(trigger.Object, "Account") || len(records) != 1 {
		return err
	}
	var thrown *apexThrowError
	if !errors.As(err, &thrown) || thrown.value.Kind != ValueObject || isBuiltinExceptionType(exceptionTypeName(thrown.value.Type)) {
		return err
	}
	caller := ""
	for _, frame := range thrown.stack {
		if observation := frame.invocableFlowOutput; observation != nil && observation.absent {
			caller = apexStackFrameSymbol(frame.Symbol)
		}
	}
	trace, ok := thrown.value.Fields["__stackTrace"]
	if caller == "" || !ok || trace.Kind != ValueString {
		return err
	}
	frames, ok := invocableFlowCallerTriggerFrames(trace.Text, caller, trigger.Name)
	if !ok {
		return err
	}
	event := "Before" + strings.ToUpper(trigger.Operation[:1]) + trigger.Operation[1:]
	detail := trigger.Name + ": execution of " + event + "\n\ncaused by: " + vm.exceptionQualifiedTypeName(thrown.value.Type) + ": " + exceptionMessage(thrown.value) + "\n\n" + frames
	return &invocableFlowCallerTriggerFailure{cause: err, detail: detail}
}

// The exception retains its original source trace across an explicit rethrow.
// Collapse the method-entry duplicate and stop at this trigger activation.
func invocableFlowCallerTriggerFrames(trace, caller, trigger string) (string, bool) {
	var lines []string
	previous := ""
	for _, frame := range strings.Split(trace, "\n") {
		symbol, position, ok := strings.Cut(frame, ": line ")
		if !ok || len(lines) == 0 && symbol != caller {
			return "", false
		}
		if symbol == previous {
			continue
		}
		previous = symbol
		lineText, _, _ := strings.Cut(position, ",")
		line, err := strconv.Atoi(lineText)
		if err != nil || line <= 0 {
			return "", false
		}
		if symbol == trigger || symbol == "Trigger."+trigger {
			lines = append(lines, "Trigger."+trigger+": line "+strconv.Itoa(line)+", column 1")
			return strings.Join(lines, "\n"), true
		}
		if !strings.HasPrefix(symbol, "Class.") {
			return "", false
		}
		lines = append(lines, symbol+": line "+strconv.Itoa(line)+", column 1")
	}
	return "", false
}

func invocableFlowCallerTriggerDMLResult(op string, allOrNone bool, records []storage.Record, err error) (dml.Result, bool) {
	var failure *invocableFlowCallerTriggerFailure
	if len(records) != 1 || !(op == "insert" && allOrNone || op == "update" && !allOrNone) || !errors.As(err, &failure) {
		return dml.Result{}, false
	}
	return dmlFailure(records[0].ID, failure.detail, "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil), true
}
