package vm

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/open-aer/oaer/internal/ir"
)

type DebugPauseReason string

const (
	DebugPauseBreakpoint DebugPauseReason = "breakpoint"
	DebugPauseStep       DebugPauseReason = "step"
)

type DebugAction string

const (
	DebugActionContinue DebugAction = "continue"
	DebugActionStop     DebugAction = "stop"
)

type DebugBreakpoint struct {
	File   string
	Line   int
	Column int
}

type DebugLocation struct {
	Symbol string
	File   string
	Line   int
	Column int
}

type DebugPause struct {
	Reason   DebugPauseReason
	Location DebugLocation
	Stack    []StackFrame
	Vars     map[string]Value
	Statics  map[string]Value
}

type DebugHooks struct {
	Breakpoints []DebugBreakpoint
	Step        bool
	OnPause     func(DebugPause) DebugAction
}

type DebugStopError struct {
	Location DebugLocation
}

func (e *DebugStopError) Error() string {
	if e.Location.Line > 0 {
		return fmt.Sprintf("debug execution stopped at line %d", e.Location.Line)
	}
	return "debug execution stopped"
}

func (vm *VM) maybePauseForDebug(inst ir.Instruction) error {
	if !vm.hasDebugHooks || !vm.hasStatement {
		return nil
	}
	reason, ok := vm.debugPauseReason()
	if !ok {
		return nil
	}
	location := DebugLocation{
		Symbol: vm.currentStatement.Symbol,
		File:   vm.currentStatement.File,
		Line:   vm.currentStatement.Line,
		Column: vm.currentStatement.Column,
	}
	if location.Symbol == "" {
		location.Symbol = string(inst.Op)
	}
	pause := DebugPause{
		Reason:   reason,
		Location: location,
		Stack:    stackFrames(vm.rawStackFrames()),
		Vars:     cloneValues(vm.Globals),
		Statics:  vm.debugStaticValues(),
	}
	action := DebugActionContinue
	if vm.debugHooks.OnPause != nil {
		action = vm.debugHooks.OnPause(pause)
	}
	if action == DebugActionStop {
		return &DebugStopError{Location: location}
	}
	return nil
}

func (vm *VM) debugPauseReason() (DebugPauseReason, bool) {
	for _, breakpoint := range vm.debugHooks.Breakpoints {
		if breakpoint.Line <= 0 || breakpoint.Line != vm.currentStatement.Line {
			continue
		}
		if breakpoint.Column > 0 && vm.currentStatement.Column > 0 && breakpoint.Column != vm.currentStatement.Column {
			continue
		}
		if !debugFileMatches(breakpoint.File, vm.currentStatement.File) {
			continue
		}
		return DebugPauseBreakpoint, true
	}
	if vm.debugHooks.Step {
		return DebugPauseStep, true
	}
	return "", false
}

func debugFileMatches(breakpointFile, statementFile string) bool {
	breakpointFile = filepath.ToSlash(strings.TrimSpace(breakpointFile))
	statementFile = filepath.ToSlash(strings.TrimSpace(statementFile))
	if breakpointFile == "" || statementFile == "" {
		return true
	}
	if breakpointFile == statementFile {
		return true
	}
	return strings.HasSuffix(statementFile, "/"+breakpointFile) || strings.HasSuffix(breakpointFile, "/"+statementFile)
}

func (vm *VM) debugStaticValues() map[string]Value {
	out := make(map[string]Value)
	for className, class := range vm.Classes {
		fields := orderedFieldNames(class.StaticFields, class.StaticFieldOrder)
		if len(fields) == 0 {
			continue
		}
		value := Object(class.Name)
		if value.Type == "" {
			value.Type = className
		}
		for _, fieldName := range fields {
			field := class.StaticFields[fieldName]
			fieldValue := field.Value
			if fieldValue.Kind == "" {
				fieldValue = defaultValue(field.Type, field.Value)
			}
			value.Fields[fieldName] = cloneValue(fieldValue)
		}
		out[className] = value
	}
	return out
}

func cloneValues(in map[string]Value) map[string]Value {
	out := make(map[string]Value, len(in))
	for name, value := range in {
		out[name] = cloneValue(value)
	}
	return out
}

func cloneValue(value Value) Value {
	out := value
	switch value.Kind {
	case ValueObject, ValueList, ValueSet, ValueMap:
		out.Ref = newValueRef()
	}
	if value.Fields != nil {
		out.Fields = make(map[string]Value, len(value.Fields))
		for name, child := range value.Fields {
			out.Fields[name] = cloneValue(child)
		}
	}
	if value.Map != nil {
		out.Map = make(map[string]Value, len(value.Map))
		for name, child := range value.Map {
			out.Map[name] = cloneValue(child)
		}
	}
	if value.List != nil {
		out.List = make([]Value, len(value.List))
		for i, child := range value.List {
			out.List[i] = cloneValue(child)
		}
	}
	if value.Set != nil {
		out.Set = make([]Value, len(value.Set))
		for i, child := range value.Set {
			out.Set[i] = cloneValue(child)
		}
	}
	return out
}
