package vm

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/dml"
	"github.com/open-aer/oaer/internal/ir"
	"github.com/open-aer/oaer/internal/soql"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/trace"
)

const maxLoopIterations = 1000000
const (
	triggerTimingBefore = "before"
	triggerTimingAfter  = "after"
)

type VM struct {
	Globals          map[string]Value
	Methods          map[string]Method
	MethodOverloads  map[string][]Method
	Classes          map[string]Class
	Org              *storage.OrgState
	Triggers         map[string][]Trigger
	Stdout           io.Writer
	callStack        []callFrame
	currentClass     string
	currentMethod    Method
	testContext      *TestContext
	limits           Limits
	limitCaps        LimitCaps
	limitMode        LimitMode
	limitViolations  []LimitViolation
	fakeNow          time.Time
	activeExceptions []Value
}

type Result struct {
	Debug           []string         `json:"debug,omitempty"`
	Vars            map[string]Value `json:"vars,omitempty"`
	TraceFormat     string           `json:"traceFormat,omitempty"`
	Trace           []trace.Event    `json:"trace,omitempty"`
	Limits          Limits           `json:"limits,omitempty"`
	LimitMode       LimitMode        `json:"limitMode,omitempty"`
	LimitViolations []LimitViolation `json:"limitViolations,omitempty"`
}

type StackFrame struct {
	Symbol string
	File   string
	Line   int
	Column int
}

type RuntimeError struct {
	Type    string
	Message string
	Stack   []StackFrame
}

func (e *RuntimeError) Error() string {
	if e.Type == "" {
		return e.Message
	}
	return e.Type + ": " + e.Message
}

type callFrame struct {
	Symbol string
	File   string
	Line   int
	Column int
}

type TestContext struct {
	Started     bool
	Stopped     bool
	CurrentUser Value
	AsyncJobs   []Value
	Draining    bool
	HTTPMock    Value
}

type Trigger struct {
	Name      string
	Object    string
	Timing    string
	Operation string
	Program   ir.Program
	File      string
	Line      int
	Column    int
}

func New(stdout io.Writer) *VM {
	return &VM{
		Globals:         make(map[string]Value),
		Methods:         make(map[string]Method),
		MethodOverloads: make(map[string][]Method),
		Classes:         make(map[string]Class),
		Triggers:        make(map[string][]Trigger),
		Stdout:          stdout,
		limitCaps:       defaultLimitCaps(),
		limitMode:       LimitModePermissive,
		fakeNow:         time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
	}
}

func (vm *VM) SetOrg(org *storage.OrgState) {
	vm.Org = org
}

func (vm *VM) RegisterTrigger(trigger Trigger) error {
	if trigger.Object == "" {
		return fmt.Errorf("trigger object is required")
	}
	if trigger.Timing == "" || trigger.Operation == "" {
		return fmt.Errorf("trigger timing and operation are required")
	}
	if vm.Triggers == nil {
		vm.Triggers = make(map[string][]Trigger)
	}
	vm.Triggers[trigger.Object] = append(vm.Triggers[trigger.Object], trigger)
	return nil
}

func (vm *VM) EnableTestContext() {
	vm.testContext = &TestContext{CurrentUser: String("system")}
}

func (vm *VM) ResetStatics() error {
	for className, class := range vm.Classes {
		for _, fieldName := range orderedFieldNames(class.StaticFields, class.StaticFieldOrder) {
			field := class.StaticFields[fieldName]
			field.Value = defaultValue(field.Type, field.InitialValue)
			class.StaticFields[fieldName] = field
		}
		vm.Classes[className] = class
	}
	for _, class := range vm.Classes {
		if err := vm.runStaticInitializers(class); err != nil {
			return err
		}
	}
	return nil
}

func Execute(program ir.Program, stdout io.Writer) (Result, error) {
	return New(stdout).Execute(program)
}

func (vm *VM) Execute(program ir.Program) (result Result, err error) {
	result = Result{Vars: vm.Globals, TraceFormat: trace.FormatChromeTraceEvent}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("internal VM panic: %v", recovered)
		}
		result.Limits = vm.limits
		result.LimitMode = vm.limitMode
		result.LimitViolations = append([]LimitViolation(nil), vm.limitViolations...)
	}()
	out, err := vm.executeProgram(program, &result)
	if err != nil {
		var thrown *apexThrowError
		if errors.As(err, &thrown) {
			return result, runtimeError(thrown.value, thrown.stack)
		}
		return result, err
	}
	if out.signal == signalThrow {
		return result, vm.runtimeError(out.thrown)
	}
	if out.signal == signalBreak || out.signal == signalContinue {
		return result, fmt.Errorf("%s outside loop", out.signal)
	}
	return result, nil
}

type controlSignal string

const (
	signalNone     controlSignal = ""
	signalReturn   controlSignal = "return"
	signalBreak    controlSignal = "break"
	signalContinue controlSignal = "continue"
	signalThrow    controlSignal = "throw"
)

type execOutcome struct {
	value  Value
	signal controlSignal
	thrown Value
}

type apexThrowError struct {
	value Value
	stack []callFrame
}

func (e *apexThrowError) Error() string {
	return e.value.String()
}

func (vm *VM) executeProgram(program ir.Program, result *Result) (execOutcome, error) {
	for seq, inst := range program.Instructions {
		if err := vm.incrementLimit("cpuTime", 1); err != nil {
			return execOutcome{}, err
		}
		result.Trace = append(result.Trace, statementTraceEvent(seq, inst))
		switch inst.Op {
		case ir.OpDeclare:
			value := Null
			if inst.Expr.Kind != "" {
				evaluated, err := vm.evalForType(inst.Expr, inst.Type, result)
				if err != nil {
					return execOutcome{}, err
				}
				value = evaluated
			}
			if err := ensureAssignable(inst.Type, value); err != nil {
				return execOutcome{}, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
			}
			vm.Globals[inst.Name] = value
			if err := vm.incrementLimit("heapSize", approxValueSize(value)); err != nil {
				return execOutcome{}, err
			}
		case ir.OpAssign:
			value, err := vm.eval(inst.Expr, result)
			if err != nil {
				return execOutcome{}, err
			}
			if err := vm.assign(inst.Name, value); err != nil {
				return execOutcome{}, err
			}
		case ir.OpExpr:
			if _, err := vm.eval(inst.Expr, result); err != nil {
				return execOutcome{}, err
			}
		case ir.OpReturn:
			value := Null
			if inst.Expr.Kind != "" {
				evaluated, err := vm.eval(inst.Expr, result)
				if err != nil {
					return execOutcome{}, err
				}
				value = evaluated
			}
			return execOutcome{value: value, signal: signalReturn}, nil
		case ir.OpIf:
			condition, err := vm.eval(inst.Expr, result)
			if err != nil {
				return execOutcome{}, err
			}
			if condition.Kind != ValueBool {
				return execOutcome{}, fmt.Errorf("if condition requires Boolean, got %s", condition.Kind)
			}
			branch := inst.Else
			if condition.Bool {
				branch = inst.Then
			}
			if len(branch) > 0 {
				out, err := vm.executeProgram(ir.Program{Instructions: branch}, result)
				if err != nil || out.signal != signalNone {
					return out, err
				}
			}
		case ir.OpWhile:
			out, err := vm.executeWhile(inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpDoWhile:
			out, err := vm.executeDoWhile(inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpFor:
			out, err := vm.executeFor(inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpForEach:
			out, err := vm.executeForEach(inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpBreak:
			return execOutcome{signal: signalBreak}, nil
		case ir.OpContinue:
			return execOutcome{signal: signalContinue}, nil
		case ir.OpThrow:
			thrown := Null
			if inst.Expr.Kind != "" {
				value, err := vm.eval(inst.Expr, result)
				if err != nil {
					return execOutcome{}, err
				}
				thrown = value
			} else {
				if len(vm.activeExceptions) == 0 {
					return execOutcome{}, fmt.Errorf("rethrow outside catch block")
				}
				thrown = vm.activeExceptions[len(vm.activeExceptions)-1]
			}
			return execOutcome{signal: signalThrow, thrown: thrown}, nil
		case ir.OpTry:
			out, err := vm.executeTry(inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpSwitch:
			out, err := vm.executeSwitch(inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpRunAs:
			out, err := vm.executeRunAs(inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpDML:
			if err := vm.executeDML(inst.Name, inst.Expr, result); err != nil {
				return execOutcome{}, err
			}
		default:
			return execOutcome{}, fmt.Errorf("unsupported instruction %q", inst.Op)
		}
	}
	return execOutcome{}, nil
}

func statementTraceEvent(seq int, inst ir.Instruction) trace.Event {
	args := map[string]any{
		"op":           string(inst.Op),
		"sourceOffset": inst.Pos,
	}
	if inst.Name != "" {
		args["name"] = inst.Name
	}
	if inst.Type != "" {
		args["type"] = inst.Type
	}
	return trace.Instant("apex.statement."+string(inst.Op), "apex.statement", int64(seq), args)
}

func (vm *VM) executeWhile(inst ir.Instruction, result *Result) (execOutcome, error) {
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("while loop exceeded %d iterations", maxLoopIterations)
		}
		condition, err := vm.eval(inst.Expr, result)
		if err != nil {
			return execOutcome{}, err
		}
		if condition.Kind != ValueBool {
			return execOutcome{}, fmt.Errorf("while condition requires Boolean, got %s", condition.Kind)
		}
		if !condition.Bool {
			return execOutcome{}, nil
		}
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone:
		case signalContinue:
			continue
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
	}
}

func (vm *VM) executeDoWhile(inst ir.Instruction, result *Result) (execOutcome, error) {
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("do/while loop exceeded %d iterations", maxLoopIterations)
		}
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone, signalContinue:
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
		condition, err := vm.eval(inst.Expr, result)
		if err != nil {
			return execOutcome{}, err
		}
		if condition.Kind != ValueBool {
			return execOutcome{}, fmt.Errorf("do/while condition requires Boolean, got %s", condition.Kind)
		}
		if !condition.Bool {
			return execOutcome{}, nil
		}
	}
}

func (vm *VM) executeFor(inst ir.Instruction, result *Result) (execOutcome, error) {
	if inst.Init != nil {
		out, err := vm.executeProgram(ir.Program{Instructions: []ir.Instruction{*inst.Init}}, result)
		if err != nil || out.signal != signalNone {
			return out, err
		}
	}
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("for loop exceeded %d iterations", maxLoopIterations)
		}
		condition, err := vm.eval(inst.Expr, result)
		if err != nil {
			return execOutcome{}, err
		}
		if condition.Kind != ValueBool {
			return execOutcome{}, fmt.Errorf("for condition requires Boolean, got %s", condition.Kind)
		}
		if !condition.Bool {
			return execOutcome{}, nil
		}
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone, signalContinue:
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
		if inst.Update != nil {
			out, err := vm.executeProgram(ir.Program{Instructions: []ir.Instruction{*inst.Update}}, result)
			if err != nil || out.signal == signalReturn || out.signal == signalThrow {
				return out, err
			}
		}
	}
}

func (vm *VM) executeForEach(inst ir.Instruction, result *Result) (execOutcome, error) {
	iterable, err := vm.eval(inst.Expr, result)
	if err != nil {
		return execOutcome{}, err
	}
	values := iterable.List
	if iterable.Kind == ValueSet {
		values = iterable.Set
	}
	if iterable.Kind != ValueList && iterable.Kind != ValueSet {
		return execOutcome{}, fmt.Errorf("enhanced for requires List or Set, got %s", iterable.Kind)
	}
	_, existed := vm.Globals[inst.Name]
	previous := vm.Globals[inst.Name]
	defer func() {
		if existed {
			vm.Globals[inst.Name] = previous
		} else {
			delete(vm.Globals, inst.Name)
		}
	}()
	for iteration, value := range values {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("enhanced for loop exceeded %d iterations", maxLoopIterations)
		}
		if err := ensureAssignable(inst.Type, value); err != nil {
			return execOutcome{}, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
		}
		vm.Globals[inst.Name] = value
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone:
		case signalContinue:
			continue
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
	}
	return execOutcome{}, nil
}

func (vm *VM) executeTry(inst ir.Instruction, result *Result) (execOutcome, error) {
	out, err := vm.executeProgram(ir.Program{Instructions: inst.Then}, result)
	if err != nil {
		var thrown *apexThrowError
		if !errors.As(err, &thrown) {
			return execOutcome{}, err
		}
		out = execOutcome{signal: signalThrow, thrown: thrown.value}
	}
	if out.signal == signalThrow && len(inst.Catch) > 0 && vm.exceptionMatchesAny(catchTypes(inst), out.thrown) {
		previous, existed := vm.Globals[inst.Name]
		vm.Globals[inst.Name] = out.thrown
		vm.activeExceptions = append(vm.activeExceptions, out.thrown)
		out, err = vm.executeProgram(ir.Program{Instructions: inst.Catch}, result)
		vm.activeExceptions = vm.activeExceptions[:len(vm.activeExceptions)-1]
		if existed {
			vm.Globals[inst.Name] = previous
		} else {
			delete(vm.Globals, inst.Name)
		}
		if err != nil {
			return execOutcome{}, err
		}
	}
	if len(inst.Finally) > 0 {
		finallyOut, err := vm.executeProgram(ir.Program{Instructions: inst.Finally}, result)
		if err != nil {
			return execOutcome{}, err
		}
		if finallyOut.signal != signalNone {
			return finallyOut, nil
		}
	}
	return out, nil
}

func (vm *VM) executeSwitch(inst ir.Instruction, result *Result) (execOutcome, error) {
	value, err := vm.eval(inst.Expr, result)
	if err != nil {
		return execOutcome{}, err
	}
	var elseCase *ir.SwitchCase
	for i := range inst.Cases {
		c := &inst.Cases[i]
		if c.Else {
			elseCase = c
			continue
		}
		for _, expr := range c.Exprs {
			caseValue, err := vm.eval(expr, result)
			if err != nil {
				return execOutcome{}, err
			}
			if value.Equal(caseValue) {
				return vm.executeProgram(ir.Program{Instructions: c.Body}, result)
			}
		}
	}
	if elseCase != nil {
		return vm.executeProgram(ir.Program{Instructions: elseCase.Body}, result)
	}
	return execOutcome{}, nil
}

func (vm *VM) executeRunAs(inst ir.Instruction, result *Result) (execOutcome, error) {
	user, err := vm.eval(inst.Expr, result)
	if err != nil {
		return execOutcome{}, err
	}
	if vm.testContext == nil {
		return execOutcome{}, fmt.Errorf("System.runAs is only available in test context")
	}
	previous := vm.testContext.CurrentUser
	vm.testContext.CurrentUser = user
	defer func() {
		vm.testContext.CurrentUser = previous
	}()
	return vm.executeProgram(ir.Program{Instructions: inst.Then}, result)
}

func (vm *VM) eval(expr ir.Expr, result *Result) (Value, error) {
	switch expr.Kind {
	case ir.ExprLiteral:
		return parseLiteral(expr.Value)
	case ir.ExprVariable:
		return vm.lookup(expr.Name)
	case ir.ExprUnary:
		if expr.Left == nil {
			return Null, fmt.Errorf("unary expression %q missing operand", expr.Operator)
		}
		value, err := vm.eval(*expr.Left, result)
		if err != nil {
			return Null, err
		}
		return evalUnary(expr.Operator, value)
	case ir.ExprBinary:
		if expr.Left == nil || expr.Right == nil {
			return Null, fmt.Errorf("binary expression %q missing operand", expr.Operator)
		}
		left, err := vm.eval(*expr.Left, result)
		if err != nil {
			return Null, err
		}
		right, err := vm.eval(*expr.Right, result)
		if err != nil {
			return Null, err
		}
		return evalBinary(expr.Operator, left, right)
	case ir.ExprCall:
		args := make([]Value, 0, len(expr.Args))
		for _, arg := range expr.Args {
			value, err := vm.eval(arg, result)
			if err != nil {
				return Null, err
			}
			args = append(args, value)
		}
		namedArgs := make(map[string]Value, len(expr.NamedArgs))
		for _, arg := range expr.NamedArgs {
			value, err := vm.eval(arg.Expr, result)
			if err != nil {
				return Null, err
			}
			namedArgs[arg.Name] = value
		}
		return vm.call(expr.Callee, args, namedArgs, result)
	case ir.ExprSOQL:
		return vm.executeSOQL(expr.Value, result)
	default:
		return Null, fmt.Errorf("unsupported expression %q", expr.Kind)
	}
}

func (vm *VM) evalForType(expr ir.Expr, typeName string, result *Result) (Value, error) {
	if expr.Kind == ir.ExprSOQL {
		return vm.executeSOQLForType(expr.Value, typeName, result)
	}
	return vm.eval(expr, result)
}

func (vm *VM) call(callee string, args []Value, namedArgs map[string]Value, result *Result) (Value, error) {
	if strings.HasPrefix(callee, "new:") {
		return vm.constructValue(strings.TrimPrefix(callee, "new:"), args, namedArgs, result)
	}
	if (callee == "this" || callee == "super") && vm.currentMethod.IsConstructor {
		return vm.callChainedConstructor(callee, args, result)
	}
	if value, ok, err := vm.callMember(callee, args, result); ok || err != nil {
		return value, err
	}
	if vm.currentClass != "" && !strings.Contains(callee, ".") {
		if method, ok := vm.resolveInstanceMethodForArgs(vm.currentClass, callee, args); ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name); err != nil {
				return Null, err
			}
			receiver := Null
			if this, ok := vm.Globals["this"]; ok {
				receiver = this
			}
			return vm.callMethodWithReceiver(method, receiver, args, result)
		}
	}
	if method, ok := vm.matchRegisteredMethod(callee, args); ok {
		if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name); err != nil {
			return Null, err
		}
		return vm.callMethod(method, args, result)
	}
	if className, methodName, ok := vm.splitClassMember(callee); ok {
		if method, ok := vm.matchRegisteredMethod(className+"."+methodName, args); ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name); err != nil {
				return Null, err
			}
			return vm.callMethod(method, args, result)
		}
	}
	switch callee {
	case "System.assert":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("System.assert expects 1 or 2 arguments")
		}
		if args[0].Kind != ValueBool {
			return Null, fmt.Errorf("System.assert expects Boolean, got %s", args[0].Kind)
		}
		if !args[0].Bool {
			return Null, vm.assertError(assertMessage("assertion failed", args[1:]))
		}
		return Null, nil
	case "System.assertEquals":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("System.assertEquals expects 2 or 3 arguments")
		}
		if !args[0].Equal(args[1]) {
			return Null, vm.assertError(assertMessage(fmt.Sprintf("expected <%s>, actual <%s>", args[0].String(), args[1].String()), args[2:]))
		}
		return Null, nil
	case "System.assertNotEquals":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("System.assertNotEquals expects 2 or 3 arguments")
		}
		if args[0].Equal(args[1]) {
			return Null, vm.assertError(assertMessage(fmt.Sprintf("values should not be equal: <%s>", args[0].String()), args[2:]))
		}
		return Null, nil
	case "System.debug":
		if len(args) != 1 {
			return Null, fmt.Errorf("System.debug expects 1 argument")
		}
		line := args[0].String()
		result.Debug = append(result.Debug, line)
		if vm.Stdout != nil {
			fmt.Fprintln(vm.Stdout, line)
		}
		return Null, nil
	case "Database.query":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Database.query expects query String")
		}
		return vm.executeSOQL(args[0].Text, result)
	case "Database.countQuery":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Database.countQuery expects query String")
		}
		value, err := vm.executeSOQL(args[0].Text, result)
		if err != nil {
			return Null, err
		}
		if count, ok := aggregateCount(value); ok {
			return count, nil
		}
		return Int(int64(len(value.List))), nil
	case "Database.upsert", "Database.undelete":
		return vm.executeDatabaseDML(strings.TrimPrefix(callee, "Database."), args, result)
	case "Database.insert", "Database.update", "Database.delete":
		return vm.executeDatabaseDML(strings.TrimPrefix(callee, "Database."), args, result)
	case "Limits.getQueries", "Limits.getLimitQueries", "Limits.getQueryRows", "Limits.getLimitQueryRows",
		"Limits.getDmlStatements", "Limits.getLimitDmlStatements", "Limits.getDmlRows", "Limits.getLimitDmlRows",
		"Limits.getHeapSize", "Limits.getLimitHeapSize", "Limits.getCpuTime", "Limits.getLimitCpuTime",
		"Limits.getCallouts", "Limits.getLimitCallouts", "Limits.getQueueableJobs", "Limits.getLimitQueueableJobs",
		"Limits.getFutureCalls", "Limits.getLimitFutureCalls":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		if value, ok := vm.limitValue(strings.TrimPrefix(callee, "Limits.")); ok {
			return value, nil
		}
		return Null, fmt.Errorf("unsupported call %q", callee)
	case "Math.abs", "Math.floor", "Math.ceil", "Math.round":
		return mathUnary(callee, args)
	case "Math.max", "Math.min":
		return mathBinary(callee, args)
	case "Date.today":
		if len(args) != 0 {
			return Null, fmt.Errorf("Date.today expects 0 arguments")
		}
		return platformScalar("Date", vm.fakeNow.Format("2006-01-02")), nil
	case "Date.newInstance":
		if len(args) != 3 || args[0].Kind != ValueInt || args[1].Kind != ValueInt || args[2].Kind != ValueInt {
			return Null, fmt.Errorf("Date.newInstance expects year, month, day integers")
		}
		return platformScalar("Date", fmt.Sprintf("%04d-%02d-%02d", args[0].Int, args[1].Int, args[2].Int)), nil
	case "Datetime.now", "System.now":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return platformScalar("Datetime", vm.fakeNow.Format(time.RFC3339)), nil
	case "System.isRunningTest":
		if len(args) != 0 {
			return Null, fmt.Errorf("System.isRunningTest expects 0 arguments")
		}
		return Bool(vm.testContext != nil), nil
	case "Time.newInstance":
		if len(args) < 3 || len(args) > 4 {
			return Null, fmt.Errorf("Time.newInstance expects hour, minute, second[, millisecond]")
		}
		return platformScalar("Time", fmt.Sprintf("%02d:%02d:%02d", args[0].Int, args[1].Int, args[2].Int)), nil
	case "Blob.valueOf":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Blob.valueOf expects String")
		}
		return platformScalar("Blob", args[0].Text), nil
	case "EncodingUtil.base64Encode":
		blob, err := blobStringArg("EncodingUtil.base64Encode", args)
		if err != nil {
			return Null, err
		}
		return String(base64.StdEncoding.EncodeToString([]byte(blob))), nil
	case "EncodingUtil.base64Decode":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("EncodingUtil.base64Decode expects String")
		}
		decoded, err := base64.StdEncoding.DecodeString(args[0].Text)
		if err != nil {
			return Null, err
		}
		return platformScalar("Blob", string(decoded)), nil
	case "EncodingUtil.convertToHex":
		blob, err := blobStringArg("EncodingUtil.convertToHex", args)
		if err != nil {
			return Null, err
		}
		return String(hex.EncodeToString([]byte(blob))), nil
	case "Crypto.generateDigest":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.generateDigest expects algorithm and Blob")
		}
		blob, err := blobStringArg("Crypto.generateDigest", args[1:])
		if err != nil {
			return Null, err
		}
		if !strings.EqualFold(args[0].Text, "SHA-256") && !strings.EqualFold(args[0].Text, "SHA256") {
			return Null, fmt.Errorf("unsupported digest algorithm %q", args[0].Text)
		}
		sum := sha256.Sum256([]byte(blob))
		return platformScalar("Blob", string(sum[:])), nil
	case "JSON.serialize":
		if len(args) != 1 {
			return Null, fmt.Errorf("JSON.serialize expects 1 argument")
		}
		data, err := json.Marshal(jsonFromValue(args[0]))
		if err != nil {
			return Null, err
		}
		return String(string(data)), nil
	case "JSON.deserializeUntyped":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("JSON.deserializeUntyped expects String")
		}
		var decoded any
		if err := json.Unmarshal([]byte(args[0].Text), &decoded); err != nil {
			return Null, err
		}
		return valueFromJSON(decoded), nil
	case "JSON.deserialize":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("JSON.deserialize expects String and Type")
		}
		var decoded any
		if err := json.Unmarshal([]byte(args[0].Text), &decoded); err != nil {
			return Null, err
		}
		if args[1].Kind == ValueObject && args[1].Type == "Type" {
			return typedValueFromJSON(args[1].Text, decoded), nil
		}
		return valueFromJSON(decoded), nil
	case "Schema.getGlobalDescribe":
		if len(args) != 0 {
			return Null, fmt.Errorf("Schema.getGlobalDescribe expects 0 arguments")
		}
		return vm.schemaGlobalDescribe(), nil
	case "FeatureManagement.checkPermission":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("FeatureManagement.checkPermission expects String")
		}
		return Bool(false), nil
	case "Messaging.sendEmail":
		if len(args) != 1 {
			return Null, fmt.Errorf("Messaging.sendEmail expects messages")
		}
		return List(Object("Messaging.SendEmailResult")), nil
	case "ApexPages.hasMessages":
		if len(args) != 0 {
			return Null, fmt.Errorf("ApexPages.hasMessages expects 0 arguments")
		}
		return Bool(false), nil
	case "ApexPages.addMessage":
		if len(args) != 1 {
			return Null, fmt.Errorf("ApexPages.addMessage expects 1 argument")
		}
		return Null, nil
	case "Test.setMock":
		if len(args) != 2 {
			return Null, fmt.Errorf("Test.setMock expects mock type and mock instance")
		}
		if vm.testContext != nil {
			vm.testContext.HTTPMock = args[1]
		}
		return Null, nil
	case "Test.startTest":
		return vm.testStart()
	case "Test.stopTest":
		return vm.testStop(result)
	case "System.enqueueJob":
		return vm.enqueueJob(args)
	case "UserInfo.getUserId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserId expects 0 arguments")
		}
		if vm.testContext != nil {
			return String(vm.testContext.CurrentUser.String()), nil
		}
		return String("system"), nil
	default:
		return Null, fmt.Errorf("unsupported call %q", callee)
	}
}

func (vm *VM) assertError(message string) error {
	return &RuntimeError{
		Type:    "System.AssertException",
		Message: message,
		Stack:   vm.stackFrames(),
	}
}

func (vm *VM) testStart() (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.startTest is only available in test context")
	}
	if vm.testContext.Started && !vm.testContext.Stopped {
		return Null, fmt.Errorf("Test.startTest cannot be called again before Test.stopTest")
	}
	vm.testContext.Started = true
	vm.testContext.Stopped = false
	vm.testContext.AsyncJobs = nil
	vm.ResetLimits()
	return Null, nil
}

func (vm *VM) testStop(result *Result) (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.stopTest is only available in test context")
	}
	if !vm.testContext.Started {
		return Null, fmt.Errorf("Test.stopTest called before Test.startTest")
	}
	if vm.testContext.Stopped {
		return Null, fmt.Errorf("Test.stopTest cannot be called more than once")
	}
	vm.testContext.Stopped = true
	return Null, vm.drainAsync(result)
}

func (vm *VM) enqueueJob(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("System.enqueueJob expects 1 argument")
	}
	if vm.testContext == nil {
		return String("async-job-1"), nil
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("System.enqueueJob expects Queueable object")
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs, args[0])
	return String(fmt.Sprintf("async-job-%d", len(vm.testContext.AsyncJobs))), nil
}

func (vm *VM) drainAsync(result *Result) error {
	if vm.testContext == nil || vm.testContext.Draining {
		return nil
	}
	vm.testContext.Draining = true
	defer func() {
		vm.testContext.Draining = false
	}()
	for len(vm.testContext.AsyncJobs) > 0 {
		job := vm.testContext.AsyncJobs[0]
		vm.testContext.AsyncJobs = vm.testContext.AsyncJobs[1:]
		target, ok := vm.resolveInstanceMethod(job.Type, "execute")
		if !ok {
			return fmt.Errorf("async job %s has no execute method", job.Type)
		}
		args := []Value{Object("QueueableContext")}
		if len(target.Params) == 0 {
			args = nil
		}
		if _, err := vm.callMethodWithReceiver(target, job, args, result); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) executeSOQL(raw string, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRows(raw, execResult)
	if err != nil {
		return Null, err
	}
	return List(values...), nil
}

func (vm *VM) executeSOQLForType(raw, typeName string, result *Result) (Value, error) {
	value, err := vm.executeSOQL(raw, result)
	if err != nil {
		return Null, err
	}
	if strings.HasPrefix(typeName, "List<") || typeName == "Object" {
		return value, nil
	}
	if typeName == "Integer" || typeName == "Long" {
		if count, ok := aggregateCount(value); ok {
			return count, nil
		}
	}
	if len(value.List) == 0 {
		return Null, fmt.Errorf("SOQL assignment to %s returned no rows", typeName)
	}
	if len(value.List) > 1 {
		return Null, fmt.Errorf("SOQL assignment to %s returned more than one row", typeName)
	}
	return value.List[0], nil
}

func (vm *VM) executeSOQLRows(raw string, execResult *Result) ([]Value, error) {
	if vm.Org == nil {
		return nil, fmt.Errorf("SOQL requires org state")
	}
	if err := vm.incrementLimit("queries", 1); err != nil {
		return nil, err
	}
	queryText, err := vm.expandSOQLBinds(raw)
	if err != nil {
		return nil, err
	}
	result, err := soql.ParseAndExecute(*vm.Org, queryText)
	if err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("queryRows", result.Rows); err != nil {
		return nil, err
	}
	values := make([]Value, 0, len(result.Records))
	for _, record := range result.Records {
		values = append(values, vmValueFromRecord(record))
	}
	if execResult != nil {
		execResult.Trace = append(execResult.Trace, trace.Instant("apex.soql", "apex.soql", int64(len(execResult.Trace)), map[string]any{
			"query": queryText,
			"rows":  result.Rows,
		}))
	}
	return values, nil
}

func aggregateCount(value Value) (Value, bool) {
	if value.Kind != ValueList || len(value.List) != 1 {
		return Null, false
	}
	row := value.List[0]
	if row.Kind != ValueObject || row.Type != "AggregateResult" {
		return Null, false
	}
	count, ok := row.Fields["expr0"]
	return count, ok && count.Kind == ValueInt
}

func (vm *VM) expandSOQLBinds(raw string) (string, error) {
	tokens := strings.Fields(raw)
	out := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == ":" && i+1 < len(tokens) {
			nameParts := []string{tokens[i+1]}
			i++
			for i+2 < len(tokens) && tokens[i+1] == "." {
				nameParts = append(nameParts, tokens[i+2])
				i += 2
			}
			token = ":" + strings.Join(nameParts, ".")
		}
		if !strings.HasPrefix(token, ":") {
			out = append(out, token)
			continue
		}
		name := strings.TrimPrefix(token, ":")
		value, err := vm.lookup(name)
		if err != nil {
			return "", err
		}
		out = append(out, soqlLiteral(value))
	}
	return strings.Join(out, " "), nil
}

func (vm *VM) executeDML(op string, expr ir.Expr, result *Result) error {
	value, err := vm.eval(expr, result)
	if err != nil {
		return err
	}
	results, err := vm.applyDML(op, value, true, result)
	if err != nil {
		return err
	}
	for _, dmlResult := range results {
		if !dmlResult.Success {
			return errors.New(dmlResult.Error)
		}
	}
	if expr.Kind == ir.ExprVariable {
		applyDMLIDs(&value, results)
		if err := vm.assign(expr.Name, value); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) executeDatabaseDML(op string, args []Value, result *Result) (Value, error) {
	if len(args) == 0 || len(args) > 2 {
		return Null, fmt.Errorf("Database.%s expects records and optional allOrNone", op)
	}
	allOrNone := true
	if len(args) == 2 {
		if args[1].Kind != ValueBool {
			return Null, fmt.Errorf("Database.%s allOrNone expects Boolean", op)
		}
		allOrNone = args[1].Bool
	}
	results, err := vm.applyDML(op, args[0], allOrNone, result)
	if err != nil {
		return Null, err
	}
	values := make([]Value, 0, len(results))
	for _, dmlResult := range results {
		row := Object("Database.SaveResult")
		row.Fields["success"] = Bool(dmlResult.Success)
		row.Fields["id"] = String(string(dmlResult.ID))
		row.Fields["error"] = String(dmlResult.Error)
		errorsList := List()
		if dmlResult.Error != "" {
			errValue := Object("Database.Error")
			errValue.Fields["message"] = String(dmlResult.Error)
			errValue.Fields["statusCode"] = String("FIELD_CUSTOM_VALIDATION_EXCEPTION")
			errorsList = List(errValue)
		}
		row.Fields["errors"] = errorsList
		values = append(values, row)
	}
	if args[0].Kind == ValueList {
		return List(values...), nil
	}
	if len(values) == 0 {
		return Null, nil
	}
	return values[0], nil
}

func applyDMLIDs(value *Value, results []dml.Result) {
	if value.Kind == ValueList {
		for i := range value.List {
			if i >= len(results) || !results[i].Success {
				continue
			}
			value.List[i].Fields["Id"] = String(string(results[i].ID))
		}
		return
	}
	if len(results) > 0 && results[0].Success && value.Kind == ValueObject {
		value.Fields["Id"] = String(string(results[0].ID))
	}
}

func (vm *VM) applyDML(op string, value Value, allOrNone bool, result *Result) ([]dml.Result, error) {
	if vm.Org == nil {
		return nil, fmt.Errorf("DML requires org state")
	}
	records, targets, err := recordsFromValue(value)
	if err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("dmlStatements", 1); err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("dmlRows", len(records)); err != nil {
		return nil, err
	}
	if result != nil {
		result.Trace = append(result.Trace, trace.Instant("apex.dml."+op, "apex.dml", int64(len(result.Trace)), map[string]any{
			"operation": op,
			"rows":      len(records),
		}))
	}
	before, err := vm.oldRecords(op, records)
	if err != nil {
		return nil, err
	}
	backup := vm.Org.Clone()
	if err := vm.runTriggers(triggerTimingBefore, op, records, before, result); err != nil {
		*vm.Org = backup
		return nil, err
	}
	engine := dml.NewEngine(vm.Org)
	var results []dml.Result
	switch op {
	case "insert":
		results = engine.Insert(records)
	case "update":
		results = engine.Update(records)
	case "delete":
		results = engine.Delete(records)
	case "upsert":
		results = engine.Upsert(records)
	case "undelete":
		results = engine.Undelete(records)
	default:
		return nil, fmt.Errorf("unsupported DML operation %s", op)
	}
	if allOrNone {
		for _, dmlResult := range results {
			if !dmlResult.Success {
				*vm.Org = backup
				return results, nil
			}
		}
	}
	for i, dmlResult := range results {
		if dmlResult.Success && i < len(targets) && targets[i] != nil {
			targets[i].Fields["Id"] = String(string(dmlResult.ID))
		}
	}
	afterRecords, err := vm.afterRecords(op, records, results)
	if err != nil {
		if allOrNone {
			*vm.Org = backup
		}
		return results, err
	}
	if err := vm.runTriggers(triggerTimingAfter, op, afterRecords, before, result); err != nil {
		if allOrNone {
			*vm.Org = backup
		}
		return nil, err
	}
	return results, nil
}

func recordsFromValue(value Value) ([]storage.Record, []*Value, error) {
	if value.Kind == ValueList {
		records := make([]storage.Record, 0, len(value.List))
		targets := make([]*Value, 0, len(value.List))
		for i := range value.List {
			record, err := recordFromValue(&value.List[i])
			if err != nil {
				return nil, nil, err
			}
			records = append(records, record)
			targets = append(targets, &value.List[i])
		}
		return records, targets, nil
	}
	record, err := recordFromValue(&value)
	if err != nil {
		return nil, nil, err
	}
	return []storage.Record{record}, []*Value{&value}, nil
}

func recordFromValue(value *Value) (storage.Record, error) {
	if value.Kind != ValueObject {
		return storage.Record{}, fmt.Errorf("DML requires sObject value, got %s", value.Kind)
	}
	record := storage.Record{
		Object:        value.Type,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
	for field, fieldValue := range value.Fields {
		if field == "Id" {
			if fieldValue.Kind == ValueString {
				record.ID = storage.ID(fieldValue.Text)
			}
			continue
		}
		converted, err := storageValueFromVM(fieldValue)
		if err != nil {
			return storage.Record{}, fmt.Errorf("%s.%s: %w", value.Type, field, err)
		}
		if converted.Kind == storage.ValueNull {
			record.ExplicitNulls[field] = true
		} else {
			record.Fields[field] = converted
		}
	}
	return record, nil
}

func vmValueFromRecord(record storage.Record) Value {
	value := Object(record.Object)
	if record.ID != "" {
		value.Fields["Id"] = String(string(record.ID))
	}
	for field, fieldValue := range record.Fields {
		putVMFieldPath(value, field, vmValueFromStorage(fieldValue))
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			value.Fields[field] = Null
		}
	}
	return value
}

func putVMFieldPath(root Value, field string, fieldValue Value) {
	if !strings.Contains(field, ".") {
		root.Fields[field] = fieldValue
		return
	}
	parts := strings.Split(field, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current.Fields[part]
		if !ok || next.Kind != ValueObject {
			next = Object(part)
			current.Fields[part] = next
		}
		current = next
	}
	current.Fields[parts[len(parts)-1]] = fieldValue
}

func storageValueFromVM(value Value) (storage.Value, error) {
	switch value.Kind {
	case ValueNull:
		return storage.NullValue(), nil
	case ValueString:
		return storage.StringValue(value.Text), nil
	case ValueInt:
		return storage.IntegerValue(value.Int), nil
	case ValueBool:
		return storage.BooleanValue(value.Bool), nil
	default:
		return storage.Value{}, fmt.Errorf("unsupported storage value %s", value.Kind)
	}
}

func vmValueFromStorage(value storage.Value) Value {
	switch value.Kind {
	case storage.ValueNull:
		return Null
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return String(value.String)
	case storage.ValueInteger:
		return Int(value.Integer)
	case storage.ValueBoolean:
		return Bool(value.Boolean)
	case storage.ValueDecimal:
		return String(value.Decimal)
	case storage.ValueID:
		return String(string(value.ID))
	case storage.ValueList:
		values := make([]Value, 0, len(value.List))
		for _, item := range value.List {
			values = append(values, vmValueFromStorage(item))
		}
		return List(values...)
	default:
		return Null
	}
}

func soqlLiteral(value Value) string {
	switch value.Kind {
	case ValueNull:
		return "null"
	case ValueString:
		return "'" + strings.ReplaceAll(value.Text, "'", "''") + "'"
	case ValueInt:
		return fmt.Sprintf("%d", value.Int)
	case ValueBool:
		if value.Bool {
			return "true"
		}
		return "false"
	default:
		return "'" + strings.ReplaceAll(value.String(), "'", "''") + "'"
	}
}

func (vm *VM) oldRecords(op string, records []storage.Record) ([]storage.Record, error) {
	if op != "update" && op != "delete" && op != "upsert" {
		return nil, nil
	}
	out := make([]storage.Record, 0, len(records))
	for _, record := range records {
		if record.ID == "" {
			out = append(out, storage.Record{Object: record.Object})
			continue
		}
		object, ok := vm.Org.Objects[record.Object]
		if !ok {
			return nil, fmt.Errorf("dml: unknown object %s", record.Object)
		}
		old, ok := object.Records[record.ID]
		if !ok {
			out = append(out, storage.Record{ID: record.ID, Object: record.Object})
			continue
		}
		out = append(out, old.Clone())
	}
	return out, nil
}

func (vm *VM) afterRecords(op string, records []storage.Record, results []dml.Result) ([]storage.Record, error) {
	if op == "delete" {
		return records, nil
	}
	out := make([]storage.Record, 0, len(records))
	for i, record := range records {
		if i >= len(results) || !results[i].Success {
			continue
		}
		id := results[i].ID
		if id == "" {
			id = record.ID
		}
		object := vm.Org.Objects[record.Object]
		stored, ok := object.Records[id]
		if !ok {
			continue
		}
		out = append(out, stored.Clone())
	}
	return out, nil
}

func (vm *VM) runTriggers(timing, op string, records, oldRecords []storage.Record, result *Result) error {
	if len(records) == 0 {
		return nil
	}
	object := records[0].Object
	for _, trigger := range vm.Triggers[object] {
		if trigger.Timing != timing || trigger.Operation != op {
			continue
		}
		if err := vm.runTrigger(trigger, records, oldRecords, result); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) runTrigger(trigger Trigger, records, oldRecords []storage.Record, result *Result) error {
	caller := vm.Globals
	callerClass := vm.currentClass
	frame := make(map[string]Value)
	ctx := triggerContext(trigger, records, oldRecords)
	for key, value := range ctx {
		frame[key] = value
	}
	vm.Globals = frame
	vm.currentClass = trigger.Name
	vm.callStack = append(vm.callStack, callFrame{Symbol: trigger.Name, File: trigger.File, Line: trigger.Line, Column: trigger.Column})
	defer func() {
		vm.callStack = vm.callStack[:len(vm.callStack)-1]
		vm.Globals = caller
		vm.currentClass = callerClass
	}()
	out, err := vm.executeProgram(trigger.Program, result)
	if err != nil {
		return err
	}
	if out.signal == signalThrow {
		return &apexThrowError{value: out.thrown, stack: append([]callFrame(nil), vm.callStack...)}
	}
	if trigger.Timing == triggerTimingBefore {
		if updated := ctx["Trigger.new"]; updated.Kind == ValueList {
			for i, item := range updated.List {
				if i >= len(records) {
					break
				}
				record, err := recordFromValue(&item)
				if err != nil {
					return err
				}
				if records[i].ID != "" && record.ID == "" {
					record.ID = records[i].ID
				}
				records[i] = record
			}
		}
	}
	return nil
}

func triggerContext(trigger Trigger, records, oldRecords []storage.Record) map[string]Value {
	newValues := make([]Value, 0, len(records))
	newMap := Map()
	for _, record := range records {
		value := vmValueFromRecord(record)
		newValues = append(newValues, value)
		if record.ID != "" {
			newMap.Map[mapKey(String(string(record.ID)))] = value
		}
	}
	oldValues := make([]Value, 0, len(oldRecords))
	oldMap := Map()
	for _, record := range oldRecords {
		value := vmValueFromRecord(record)
		oldValues = append(oldValues, value)
		if record.ID != "" {
			oldMap.Map[mapKey(String(string(record.ID)))] = value
		}
	}
	ctx := map[string]Value{
		"Trigger.new":           List(newValues...),
		"Trigger.old":           List(oldValues...),
		"Trigger.newMap":        newMap,
		"Trigger.oldMap":        oldMap,
		"Trigger.isBefore":      Bool(trigger.Timing == triggerTimingBefore),
		"Trigger.isAfter":       Bool(trigger.Timing == triggerTimingAfter),
		"Trigger.isInsert":      Bool(trigger.Operation == "insert"),
		"Trigger.isUpdate":      Bool(trigger.Operation == "update"),
		"Trigger.isDelete":      Bool(trigger.Operation == "delete"),
		"Trigger.isUndelete":    Bool(trigger.Operation == "undelete"),
		"Trigger.operationType": String(strings.ToUpper(trigger.Timing + "_" + trigger.Operation)),
		"Trigger.size":          Int(int64(len(records))),
	}
	return ctx
}

func platformScalar(typeName, value string) Value {
	out := Object(typeName)
	out.Fields["value"] = String(value)
	return out
}

func assertMessage(base string, extra []Value) string {
	if len(extra) == 0 {
		return base
	}
	return base + ": " + extra[0].String()
}

func blobStringArg(name string, args []Value) (string, error) {
	if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
		return "", fmt.Errorf("%s expects Blob", name)
	}
	return args[0].Fields["value"].String(), nil
}

func mathUnary(callee string, args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueInt {
		return Null, fmt.Errorf("%s expects Integer", callee)
	}
	switch callee {
	case "Math.abs":
		if args[0].Int < 0 {
			return Int(-args[0].Int), nil
		}
		return args[0], nil
	case "Math.floor", "Math.ceil", "Math.round":
		return args[0], nil
	default:
		return Null, fmt.Errorf("unsupported call %q", callee)
	}
}

func mathBinary(callee string, args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueInt || args[1].Kind != ValueInt {
		return Null, fmt.Errorf("%s expects two Integers", callee)
	}
	switch callee {
	case "Math.max":
		return Int(int64(math.Max(float64(args[0].Int), float64(args[1].Int)))), nil
	case "Math.min":
		return Int(int64(math.Min(float64(args[0].Int), float64(args[1].Int)))), nil
	default:
		return Null, fmt.Errorf("unsupported call %q", callee)
	}
}

func jsonFromValue(value Value) any {
	switch value.Kind {
	case ValueNull:
		return nil
	case ValueInt:
		return value.Int
	case ValueBool:
		return value.Bool
	case ValueString:
		return value.Text
	case ValueList:
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			out = append(out, jsonFromValue(item))
		}
		return out
	case ValueSet:
		out := make([]any, 0, len(value.Set))
		for _, item := range value.Set {
			out = append(out, jsonFromValue(item))
		}
		return out
	case ValueMap:
		out := make(map[string]any, len(value.Map))
		for key, item := range value.Map {
			out[key] = jsonFromValue(item)
		}
		return out
	case ValueObject:
		out := make(map[string]any, len(value.Fields)+1)
		if value.Type != "" {
			out["attributes"] = map[string]any{"type": value.Type}
		}
		for field, item := range value.Fields {
			out[field] = jsonFromValue(item)
		}
		return out
	default:
		return nil
	}
}

func valueFromJSON(raw any) Value {
	switch v := raw.(type) {
	case nil:
		return Null
	case bool:
		return Bool(v)
	case float64:
		return Int(int64(v))
	case string:
		return String(v)
	case []any:
		out := make([]Value, 0, len(v))
		for _, item := range v {
			out = append(out, valueFromJSON(item))
		}
		return List(out...)
	case map[string]any:
		out := Map()
		for key, item := range v {
			out.Map[mapKey(String(key))] = valueFromJSON(item)
		}
		return out
	default:
		return Null
	}
}

func typedValueFromJSON(typeName string, raw any) Value {
	obj := Object(typeName)
	fields, ok := raw.(map[string]any)
	if !ok {
		return valueFromJSON(raw)
	}
	for key, item := range fields {
		if key == "attributes" {
			continue
		}
		obj.Fields[key] = valueFromJSON(item)
	}
	return obj
}

func (vm *VM) schemaGlobalDescribe() Value {
	out := Map()
	if vm.Org == nil {
		return out
	}
	for name, object := range vm.Org.Objects {
		desc := Object("Schema.DescribeSObjectResult")
		desc.Fields["name"] = String(name)
		desc.Fields["label"] = String(object.Definition.Label)
		desc.Fields["keyPrefix"] = String(object.Definition.KeyPrefix)
		out.Map[mapKey(String(name))] = desc
	}
	return out
}

func approxValueSize(value Value) int {
	switch value.Kind {
	case ValueNull:
		return 4
	case ValueInt, ValueBool:
		return 8
	case ValueString:
		return len(value.Text)
	case ValueList:
		total := 24
		for _, item := range value.List {
			total += approxValueSize(item)
		}
		return total
	case ValueSet:
		total := 24
		for _, item := range value.Set {
			total += approxValueSize(item)
		}
		return total
	case ValueMap:
		total := 24
		for key, item := range value.Map {
			total += len(key) + approxValueSize(item)
		}
		return total
	case ValueObject:
		total := 32 + len(value.Type)
		for key, item := range value.Fields {
			total += len(key) + approxValueSize(item)
		}
		return total
	default:
		return 0
	}
}

func (vm *VM) lookup(name string) (Value, error) {
	if value, ok := vm.Globals[name]; ok {
		return value, nil
	}
	if strings.HasSuffix(name, ".class") {
		className := strings.TrimSuffix(name, ".class")
		if resolved, ok := vm.resolveClassName(className); ok {
			return Value{Kind: ValueObject, Type: "Type", Text: resolved}, nil
		}
		return Value{Kind: ValueObject, Type: "Type", Text: className}, nil
	}
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		if root, ok := vm.Globals[parts[0]]; ok {
			return vm.lookupPath(root, parts[1:])
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			if class, ok := vm.Classes[className]; ok {
				if field, ok := class.StaticFields[memberName]; ok {
					if err := vm.checkMemberAccess(class.Name, field.Access, className+"."+memberName); err != nil {
						return Null, err
					}
					if field.Getter != nil {
						return vm.callMethod(*field.Getter, nil, resultForLookup())
					}
					return field.Value, nil
				}
				for _, enumValue := range class.EnumValues {
					if enumValue == memberName {
						return Value{Kind: ValueObject, Type: class.Name, Text: memberName}, nil
					}
				}
			}
		}
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if value, ok := this.Fields[name]; ok {
			if field, owner, ok := vm.lookupField(this.Type, name); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name); err != nil {
					return Null, err
				}
				if field.Getter != nil {
					return vm.callMethodWithReceiver(*field.Getter, this, nil, resultForLookup())
				}
			}
			return value, nil
		}
	}
	if vm.currentClass != "" {
		if class, ok := vm.Classes[vm.currentClass]; ok {
			if field, ok := class.StaticFields[name]; ok {
				if err := vm.checkMemberAccess(class.Name, field.Access, class.Name+"."+name); err != nil {
					return Null, err
				}
				if field.Getter != nil {
					return vm.callMethod(*field.Getter, nil, resultForLookup())
				}
				return field.Value, nil
			}
		}
	}
	return Null, fmt.Errorf("unknown variable %q", name)
}

func (vm *VM) lookupPath(root Value, parts []string) (Value, error) {
	current := root
	for _, part := range parts {
		if current.Kind == ValueNull {
			return Null, newExceptionError("NullPointerException", "Attempt to de-reference a null object")
		}
		if current.Kind != ValueObject {
			return Null, fmt.Errorf("cannot access %s on %s", part, current.Kind)
		}
		if field, owner, ok := vm.lookupField(current.Type, part); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+part); err != nil {
				return Null, err
			}
			if field.Getter != nil {
				value, err := vm.callMethodWithReceiver(*field.Getter, current, nil, resultForLookup())
				if err != nil {
					return Null, err
				}
				current = value
				continue
			}
		}
		value, ok := current.Fields[part]
		if !ok {
			return Null, fmt.Errorf("unknown field %q on %s", part, current.Type)
		}
		current = value
	}
	return current, nil
}

func (vm *VM) assign(name string, value Value) error {
	if _, ok := vm.Globals[name]; ok {
		vm.Globals[name] = value
		return nil
	}
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		if root, ok := vm.Globals[parts[0]]; ok {
			return vm.assignPath(root, parts[1:], value)
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			if class, ok := vm.Classes[className]; ok {
				if field, ok := class.StaticFields[memberName]; ok {
					if err := vm.checkMemberAccess(class.Name, field.Access, className+"."+memberName); err != nil {
						return err
					}
					if err := ensureAssignable(field.Type, value); err != nil {
						return fmt.Errorf("%s.%s: %w", className, memberName, err)
					}
					if field.Setter != nil {
						_, err := vm.callMethod(*field.Setter, []Value{value}, resultForLookup())
						return err
					}
					field.Value = value
					class.StaticFields[memberName] = field
					vm.Classes[className] = class
					vm.storeClassAliases(class)
					return nil
				}
			}
		}
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if field, ok := this.Fields[name]; ok {
			class := vm.Classes[this.Type]
			if def, ok := class.Fields[name]; ok {
				if err := vm.checkMemberAccess(class.Name, def.Access, class.Name+"."+name); err != nil {
					return err
				}
				if err := ensureAssignable(def.Type, value); err != nil {
					return fmt.Errorf("%s.%s: %w", this.Type, name, err)
				}
				if def.Setter != nil {
					_, err := vm.callMethodWithReceiver(*def.Setter, this, []Value{value}, resultForLookup())
					return err
				}
			}
			_ = field
			this.Fields[name] = value
			return nil
		}
	}
	if vm.currentClass != "" {
		if class, ok := vm.Classes[vm.currentClass]; ok {
			if field, ok := class.StaticFields[name]; ok {
				if err := vm.checkMemberAccess(class.Name, field.Access, class.Name+"."+name); err != nil {
					return err
				}
				if err := ensureAssignable(field.Type, value); err != nil {
					return fmt.Errorf("%s.%s: %w", vm.currentClass, name, err)
				}
				if field.Setter != nil {
					_, err := vm.callMethod(*field.Setter, []Value{value}, resultForLookup())
					return err
				}
				field.Value = value
				class.StaticFields[name] = field
				vm.Classes[vm.currentClass] = class
				return nil
			}
		}
	}
	return fmt.Errorf("unknown variable %q", name)
}

func (vm *VM) assignPath(root Value, parts []string, value Value) error {
	if len(parts) == 0 {
		return fmt.Errorf("empty assignment target")
	}
	current := root
	for _, part := range parts[:len(parts)-1] {
		if current.Kind == ValueNull {
			return newExceptionError("NullPointerException", "Attempt to de-reference a null object")
		}
		if current.Kind != ValueObject {
			return fmt.Errorf("cannot assign field %s on %s", part, current.Kind)
		}
		next, ok := current.Fields[part]
		if !ok || next.Kind != ValueObject {
			return fmt.Errorf("unknown field %q on %s", part, current.Type)
		}
		current = next
	}
	fieldName := parts[len(parts)-1]
	if current.Kind == ValueNull {
		return newExceptionError("NullPointerException", "Attempt to de-reference a null object")
	}
	if current.Kind != ValueObject {
		return fmt.Errorf("cannot assign field %s on %s", fieldName, current.Kind)
	}
	class := vm.Classes[current.Type]
	if def, ok := class.Fields[fieldName]; ok {
		if err := vm.checkMemberAccess(class.Name, def.Access, class.Name+"."+fieldName); err != nil {
			return err
		}
		if err := ensureAssignable(def.Type, value); err != nil {
			return fmt.Errorf("%s.%s: %w", current.Type, fieldName, err)
		}
		if def.Setter != nil {
			_, err := vm.callMethodWithReceiver(*def.Setter, current, []Value{value}, resultForLookup())
			return err
		}
	}
	current.Fields[fieldName] = value
	return nil
}

func (vm *VM) lookupField(typeName, fieldName string) (Field, string, bool) {
	for typeName != "" {
		class, ok := vm.Classes[typeName]
		if !ok {
			return Field{}, "", false
		}
		if field, ok := class.Fields[fieldName]; ok {
			return field, class.Name, true
		}
		typeName = class.SuperClass
	}
	return Field{}, "", false
}

func (vm *VM) checkMemberAccess(ownerClass, access, member string) error {
	if err := vm.checkNamespaceAccess(ownerClass, access, member); err != nil {
		return err
	}
	switch strings.ToLower(access) {
	case "", "public", "global", "webservice":
		return nil
	case "private":
		if vm.currentClass == ownerClass {
			return nil
		}
	case "protected":
		if vm.currentClass == ownerClass || vm.isSubclass(vm.currentClass, ownerClass) {
			return nil
		}
	default:
		return nil
	}
	if vm.currentClass == "" {
		return fmt.Errorf("%s is %s and not visible", member, access)
	}
	return fmt.Errorf("%s is %s and not visible from %s", member, access, vm.currentClass)
}

func (vm *VM) checkNamespaceAccess(ownerClass, access, member string) error {
	ownerNS := vm.classNamespace(ownerClass)
	if ownerNS == "" {
		return nil
	}
	callerNS := vm.classNamespace(vm.currentClass)
	if callerNS == ownerNS {
		return nil
	}
	switch strings.ToLower(access) {
	case "global", "webservice":
		return nil
	}
	if vm.currentClass == "" {
		return fmt.Errorf("%s is not global and not visible outside namespace %s", member, ownerNS)
	}
	return fmt.Errorf("%s is not global and not visible from namespace %s", member, callerNS)
}

func (vm *VM) classNamespace(className string) string {
	class, ok := vm.Classes[className]
	if !ok {
		if resolved, found := vm.resolveClassName(className); found {
			class, ok = vm.Classes[resolved]
		}
	}
	if !ok {
		return ""
	}
	return class.Namespace
}

func (vm *VM) isSubclass(child, parent string) bool {
	for child != "" {
		class, ok := vm.Classes[child]
		if !ok {
			return false
		}
		if class.SuperClass == parent {
			return true
		}
		child = class.SuperClass
	}
	return false
}

func (vm *VM) splitClassMember(name string) (string, string, bool) {
	parts := strings.Split(name, ".")
	for i := len(parts) - 1; i > 0; i-- {
		className := strings.Join(parts[:i], ".")
		if resolved, ok := vm.resolveClassName(className); ok {
			return resolved, strings.Join(parts[i:], "."), true
		}
	}
	return "", "", false
}

func (vm *VM) resolveClassName(typeName string) (string, bool) {
	if class, ok := vm.Classes[typeName]; ok {
		return class.Name, true
	}
	for _, class := range vm.Classes {
		if class.Namespace != "" && typeName == class.Namespace+"."+class.Name {
			return class.Name, true
		}
	}
	return "", false
}

func (vm *VM) storeClassAliases(class Class) {
	vm.Classes[class.Name] = class
	if class.Namespace != "" && !strings.Contains(class.Name, ".") {
		vm.Classes[class.Namespace+"."+class.Name] = class
	}
}

func resultForLookup() *Result {
	return &Result{TraceFormat: trace.FormatChromeTraceEvent}
}

func (vm *VM) constructValue(typeName string, args []Value, namedArgs map[string]Value, result *Result) (Value, error) {
	if resolved, ok := vm.resolveClassName(typeName); ok {
		typeName = resolved
	}
	switch {
	case strings.HasPrefix(typeName, "List<"):
		if len(namedArgs) > 0 {
			return Null, fmt.Errorf("List constructor does not accept named fields")
		}
		return List(args...), nil
	case strings.HasPrefix(typeName, "Set<"):
		if len(namedArgs) > 0 {
			return Null, fmt.Errorf("Set constructor does not accept named fields")
		}
		return Set(args...), nil
	case strings.HasPrefix(typeName, "Map<"):
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Map constructor does not accept positional values")
		}
		return Map(), nil
	}
	object := Object(typeName)
	for field, value := range namedArgs {
		object.Fields[field] = value
	}
	vm.initializeFields(&object, typeName)
	if class, ok := vm.Classes[typeName]; ok {
		if err := vm.runInstanceInitializers(class, object, result); err != nil {
			return Null, err
		}
		ctor, ok := vm.matchConstructor(class, args)
		if ok {
			if _, err := vm.callMethodWithReceiver(ctor, object, args, result); err != nil {
				return Null, err
			}
		} else if len(args) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return object, nil
	}
	if len(args) != 0 {
		if isExceptionType(typeName) && len(args) == 1 && args[0].Kind == ValueString {
			object.Fields["message"] = args[0]
			return object, nil
		}
		return Null, fmt.Errorf("%s constructor does not accept arguments", typeName)
	}
	return object, nil
}

func (vm *VM) runInstanceInitializers(class Class, object Value, result *Result) error {
	if class.SuperClass != "" {
		if superClass, ok := vm.Classes[class.SuperClass]; ok {
			if err := vm.runInstanceInitializers(superClass, object, result); err != nil {
				return err
			}
		}
	}
	for _, initializer := range class.InstanceInitializers {
		if initializer.Name == "" {
			initializer.Name = class.Name + ".<init_block>"
		}
		if initializer.ClassName == "" {
			initializer.ClassName = class.Name
		}
		if _, err := vm.callMethodWithReceiver(initializer, object, nil, result); err != nil {
			return err
		}
	}
	return nil
}

func isExceptionType(typeName string) bool {
	return typeName == "Exception" || strings.HasSuffix(typeName, "Exception")
}

func (vm *VM) initializeFields(object *Value, typeName string) {
	class, ok := vm.Classes[typeName]
	if !ok {
		return
	}
	if class.SuperClass != "" {
		vm.initializeFields(object, class.SuperClass)
	}
	for _, name := range orderedFieldNames(class.Fields, class.FieldOrder) {
		field := class.Fields[name]
		object.Fields[name] = defaultValue(field.Type, field.InitialValue)
	}
}

func (vm *VM) matchConstructor(class Class, args []Value) (Method, bool) {
	return matchMethodByArgs(class.Constructors, args)
}

func (vm *VM) callChainedConstructor(callee string, args []Value, result *Result) (Value, error) {
	receiver, ok := vm.Globals["this"]
	if !ok || receiver.Kind != ValueObject {
		return Null, fmt.Errorf("%s constructor call requires instance receiver", callee)
	}
	class, ok := vm.Classes[receiver.Type]
	if !ok {
		return Null, fmt.Errorf("%s constructor call requires registered class %q", callee, receiver.Type)
	}
	targetClass := class
	if callee == "super" {
		if class.SuperClass == "" {
			if len(args) == 0 {
				return Null, nil
			}
			return Null, fmt.Errorf("%s has no superclass constructor", receiver.Type)
		}
		var found bool
		targetClass, found = vm.Classes[class.SuperClass]
		if !found {
			return Null, fmt.Errorf("unknown superclass %q", class.SuperClass)
		}
	}
	target, found := vm.matchConstructor(targetClass, args)
	if !found {
		if len(args) == 0 {
			return Null, nil
		}
		return Null, fmt.Errorf("%s constructor expects 0 arguments", targetClass.Name)
	}
	if callee == "this" && sameConstructorSignature(vm.currentMethod, target) {
		return Null, fmt.Errorf("recursive constructor invocation %s", target.Name)
	}
	_, err := vm.callMethodWithReceiver(target, receiver, args, result)
	return Null, err
}

func sameConstructorSignature(left, right Method) bool {
	if left.Name != right.Name || len(left.Params) != len(right.Params) {
		return false
	}
	for i := range left.Params {
		if left.Params[i].Type != right.Params[i].Type {
			return false
		}
	}
	return true
}

func (vm *VM) resolveInstanceMethod(typeName, method string) (Method, bool) {
	for typeName != "" {
		if target, ok := vm.Methods[typeName+"."+method]; ok {
			return target, true
		}
		class, ok := vm.Classes[typeName]
		if !ok {
			break
		}
		typeName = class.SuperClass
	}
	return Method{}, false
}

func (vm *VM) resolveInstanceMethodForArgs(typeName, method string, args []Value) (Method, bool) {
	for typeName != "" {
		if target, ok := vm.matchRegisteredMethod(typeName+"."+method, args); ok {
			return target, true
		}
		class, ok := vm.Classes[typeName]
		if !ok {
			break
		}
		typeName = class.SuperClass
	}
	return Method{}, false
}

func (vm *VM) matchRegisteredMethod(name string, args []Value) (Method, bool) {
	if candidates := vm.MethodOverloads[name]; len(candidates) > 0 {
		return matchMethodByArgs(candidates, args)
	}
	method, ok := vm.Methods[name]
	if !ok {
		return Method{}, false
	}
	return method, len(method.Params) == len(args)
}

func matchMethodByArgs(candidates []Method, args []Value) (Method, bool) {
	bestScore := -1
	var best Method
	matched := false
	for _, candidate := range candidates {
		if len(candidate.Params) != len(args) {
			continue
		}
		score := 0
		assignable := true
		for i, param := range candidate.Params {
			if err := ensureAssignable(param.Type, args[i]); err != nil {
				assignable = false
				break
			}
			if param.Type == valueTypeName(args[i]) {
				score += 2
			} else if param.Type != "Object" {
				score++
			}
		}
		if assignable && score > bestScore {
			bestScore = score
			best = candidate
			matched = true
		}
	}
	return best, matched
}

func valueTypeName(value Value) string {
	switch value.Kind {
	case ValueInt:
		return "Integer"
	case ValueBool:
		return "Boolean"
	case ValueString:
		return "String"
	case ValueList:
		return "List"
	case ValueSet:
		return "Set"
	case ValueMap:
		return "Map"
	case ValueObject:
		return value.Type
	case ValueNull:
		return "null"
	default:
		return string(value.Kind)
	}
}

func newExceptionError(typeName, message string) error {
	value := Object(typeName)
	value.Fields["message"] = String(message)
	return &apexThrowError{value: value}
}

func catchTypes(inst ir.Instruction) []string {
	if len(inst.CatchTypes) > 0 {
		return inst.CatchTypes
	}
	return []string{inst.Type}
}

func (vm *VM) exceptionMatchesAny(catchTypes []string, thrown Value) bool {
	for _, catchType := range catchTypes {
		if vm.exceptionMatches(catchType, thrown) {
			return true
		}
	}
	return false
}

func (vm *VM) exceptionMatches(catchType string, thrown Value) bool {
	if catchType == "" || catchType == "Exception" || catchType == "Object" {
		return true
	}
	if thrown.Kind == ValueObject {
		if thrown.Type == catchType {
			return true
		}
		for typeName := thrown.Type; typeName != ""; {
			class, ok := vm.Classes[typeName]
			if !ok {
				return false
			}
			if class.SuperClass == catchType {
				return true
			}
			typeName = class.SuperClass
		}
	}
	return false
}

func (vm *VM) runtimeError(thrown Value) error {
	return runtimeError(thrown, vm.callStack)
}

func runtimeError(thrown Value, stack []callFrame) error {
	message := "unhandled exception"
	errorType := "Exception"
	if thrown.Kind != ValueNull {
		message = thrown.String()
		if thrown.Kind == ValueObject && thrown.Type != "" {
			errorType = thrown.Type
		}
	}
	if len(stack) == 0 {
		return &RuntimeError{Type: errorType, Message: message}
	}
	return &RuntimeError{Type: errorType, Message: message, Stack: stackFrames(stack)}
}

func classNameFromMethod(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[:i]
	}
	return ""
}

func defaultValue(typeName string, explicit Value) Value {
	if explicit.Kind != "" {
		return explicit
	}
	switch typeName {
	case "Integer", "Long":
		return Int(0)
	case "Boolean":
		return Bool(false)
	case "String":
		return Null
	default:
		return Null
	}
}

func (vm *VM) stackFrames() []StackFrame {
	return stackFrames(vm.callStack)
}

func stackFrames(frames []callFrame) []StackFrame {
	out := make([]StackFrame, 0, len(frames))
	for i := len(frames) - 1; i >= 0; i-- {
		frame := frames[i]
		out = append(out, StackFrame{
			Symbol: frame.Symbol,
			File:   frame.File,
			Line:   frame.Line,
			Column: frame.Column,
		})
	}
	return out
}

func (vm *VM) callMethod(method Method, args []Value, result *Result) (Value, error) {
	return vm.callMethodWithReceiver(method, Null, args, result)
}

func (vm *VM) callMethodWithReceiver(method Method, receiver Value, args []Value, result *Result) (Value, error) {
	if len(args) != len(method.Params) {
		return Null, fmt.Errorf("%s expects %d arguments", method.Name, len(method.Params))
	}
	frame := make(map[string]Value, len(method.Params))
	for i, param := range method.Params {
		if err := ensureAssignable(param.Type, args[i]); err != nil {
			return Null, fmt.Errorf("%s parameter %s: %w", method.Name, param.Name, err)
		}
		frame[param.Name] = args[i]
	}
	if receiver.Kind != ValueNull {
		frame["this"] = receiver
	}
	caller := vm.Globals
	callerClass := vm.currentClass
	callerMethod := vm.currentMethod
	vm.Globals = frame
	vm.currentClass = method.ClassName
	vm.currentMethod = method
	if vm.currentClass == "" {
		vm.currentClass = classNameFromMethod(method.Name)
	}
	vm.callStack = append(vm.callStack, callFrame{
		Symbol: method.Name,
		File:   method.File,
		Line:   method.Line,
		Column: method.Column,
	})
	if result != nil {
		result.Trace = append(result.Trace, trace.Instant("apex.method."+method.Name, "apex.method", int64(len(result.Trace)), map[string]any{
			"method": method.Name,
			"class":  method.ClassName,
			"file":   method.File,
			"line":   method.Line,
			"column": method.Column,
		}))
	}
	defer func() {
		vm.callStack = vm.callStack[:len(vm.callStack)-1]
		vm.Globals = caller
		vm.currentClass = callerClass
		vm.currentMethod = callerMethod
	}()

	out, err := vm.executeProgram(method.Program, result)
	if err != nil {
		var runtimeErr *RuntimeError
		if errors.As(err, &runtimeErr) {
			if len(runtimeErr.Stack) == 0 {
				runtimeErr.Stack = vm.stackFrames()
			}
			return Null, runtimeErr
		}
		return Null, &RuntimeError{Type: "RuntimeError", Message: err.Error(), Stack: vm.stackFrames()}
	}
	if out.signal == signalThrow {
		return Null, &apexThrowError{value: out.thrown, stack: append([]callFrame(nil), vm.callStack...)}
	}
	if out.signal == signalBreak || out.signal == signalContinue {
		return Null, fmt.Errorf("%s outside loop", out.signal)
	}
	value := out.value
	if out.signal != signalReturn {
		value = Null
	}
	if method.ReturnType != "" && method.ReturnType != "void" {
		if err := ensureAssignable(method.ReturnType, value); err != nil {
			return Null, fmt.Errorf("%s return: %w", method.Name, err)
		}
	}
	return value, nil
}

func (vm *VM) callMember(callee string, args []Value, result *Result) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) != 2 {
		return Null, false, nil
	}
	receiverName, method := parts[0], parts[1]
	if receiverName == "super" {
		receiver, ok := vm.Globals["this"]
		if !ok || receiver.Kind != ValueObject {
			return Null, true, fmt.Errorf("super call requires instance receiver")
		}
		class := vm.Classes[receiver.Type]
		target, ok := vm.resolveInstanceMethodForArgs(class.SuperClass, method, args)
		if !ok {
			return Null, true, fmt.Errorf("unsupported call %q", callee)
		}
		if err := vm.checkMemberAccess(target.ClassName, target.Access, target.Name); err != nil {
			return Null, true, err
		}
		value, err := vm.callMethodWithReceiver(target, receiver, args, result)
		return value, true, err
	}
	receiver, ok := vm.Globals[receiverName]
	if !ok {
		return Null, false, nil
	}
	if receiver.Kind == ValueNull {
		return Null, true, newExceptionError("NullPointerException", "Attempt to de-reference a null object")
	}
	if value, updated, mutated, ok, err := callStdlibMember(receiver, method, args); ok || err != nil {
		if mutated {
			vm.Globals[receiverName] = updated
		}
		return value, true, err
	}
	if receiver.Kind == ValueObject {
		if value, handled, err := vm.callSObjectMember(receiver, method, args); handled || err != nil {
			if method == "put" {
				vm.Globals[receiverName] = receiver
			}
			return value, true, err
		}
		if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args); handled || err != nil {
			if mutated {
				vm.Globals[receiverName] = updated
			}
			return value, true, err
		}
		target, ok := vm.resolveInstanceMethodForArgs(receiver.Type, method, args)
		if !ok {
			return Null, true, fmt.Errorf("unsupported call %q", callee)
		}
		if err := vm.checkMemberAccess(target.ClassName, target.Access, target.Name); err != nil {
			return Null, true, err
		}
		value, err := vm.callMethodWithReceiver(target, receiver, args, result)
		return value, true, err
	}

	switch receiver.Kind {
	case ValueList:
		switch method {
		case "add":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("List.add expects 1 argument")
			}
			receiver.List = append(receiver.List, args[0])
			vm.Globals[receiverName] = receiver
			return Bool(true), true, nil
		case "size":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("List.size expects 0 arguments")
			}
			return Int(int64(len(receiver.List))), true, nil
		case "get":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, true, fmt.Errorf("List.get expects integer index")
			}
			i := int(args[0].Int)
			if i < 0 || i >= len(receiver.List) {
				return Null, true, fmt.Errorf("List index out of bounds: %d", i)
			}
			return receiver.List[i], true, nil
		case "contains":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("List.contains expects 1 argument")
			}
			return Bool(containsValue(receiver.List, args[0])), true, nil
		}
	case ValueSet:
		switch method {
		case "add":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Set.add expects 1 argument")
			}
			if !containsValue(receiver.Set, args[0]) {
				receiver.Set = append(receiver.Set, args[0])
				vm.Globals[receiverName] = receiver
				return Bool(true), true, nil
			}
			return Bool(false), true, nil
		case "size":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Set.size expects 0 arguments")
			}
			return Int(int64(len(receiver.Set))), true, nil
		case "contains":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Set.contains expects 1 argument")
			}
			return Bool(containsValue(receiver.Set, args[0])), true, nil
		}
	case ValueMap:
		switch method {
		case "put":
			if len(args) != 2 {
				return Null, true, fmt.Errorf("Map.put expects 2 arguments")
			}
			receiver.Map[mapKey(args[0])] = args[1]
			vm.Globals[receiverName] = receiver
			return Null, true, nil
		case "get":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Map.get expects 1 argument")
			}
			value, ok := receiver.Map[mapKey(args[0])]
			if !ok {
				return Null, true, nil
			}
			return value, true, nil
		case "containsKey":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Map.containsKey expects 1 argument")
			}
			_, ok := receiver.Map[mapKey(args[0])]
			return Bool(ok), true, nil
		case "size":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Map.size expects 0 arguments")
			}
			return Int(int64(len(receiver.Map))), true, nil
		}
	}
	return Null, true, fmt.Errorf("unsupported call %q", callee)
}

func (vm *VM) callSObjectMember(receiver Value, method string, args []Value) (Value, bool, error) {
	switch method {
	case "get":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("SObject.get expects field name String")
		}
		value, ok := receiver.Fields[args[0].Text]
		if !ok {
			return Null, true, nil
		}
		return value, true, nil
	case "put":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("SObject.put expects field name String and value")
		}
		receiver.Fields[args[0].Text] = args[1]
		return Null, true, nil
	default:
		return Null, false, nil
	}
}

func (vm *VM) callPlatformObjectMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if isExceptionType(receiver.Type) && method == "getMessage" {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getMessage expects 0 arguments", receiver.Type)
		}
		if message, ok := receiver.Fields["message"]; ok {
			return message, receiver, false, true, nil
		}
		return String(receiver.String()), receiver, false, true, nil
	}
	switch receiver.Type {
	case "Date":
		switch method {
		case "format", "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects 0 arguments", method)
			}
			return String(receiver.Fields["value"].String()), receiver, false, true, nil
		case "addDays":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("Date.addDays expects Integer")
			}
			date, err := time.Parse("2006-01-02", receiver.Fields["value"].String())
			if err != nil {
				return Null, receiver, false, true, err
			}
			return platformScalar("Date", date.AddDate(0, 0, int(args[0].Int)).Format("2006-01-02")), receiver, false, true, nil
		case "daysBetween":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Date" {
				return Null, receiver, false, true, fmt.Errorf("Date.daysBetween expects Date")
			}
			start, err := time.Parse("2006-01-02", receiver.Fields["value"].String())
			if err != nil {
				return Null, receiver, false, true, err
			}
			end, err := time.Parse("2006-01-02", args[0].Fields["value"].String())
			if err != nil {
				return Null, receiver, false, true, err
			}
			return Int(int64(end.Sub(start).Hours() / 24)), receiver, false, true, nil
		case "year", "month", "day":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects 0 arguments", method)
			}
			date, err := time.Parse("2006-01-02", receiver.Fields["value"].String())
			if err != nil {
				return Null, receiver, false, true, err
			}
			switch method {
			case "year":
				return Int(int64(date.Year())), receiver, false, true, nil
			case "month":
				return Int(int64(date.Month())), receiver, false, true, nil
			default:
				return Int(int64(date.Day())), receiver, false, true, nil
			}
		}
	case "Datetime":
		switch method {
		case "format", "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects 0 arguments", method)
			}
			return String(receiver.Fields["value"].String()), receiver, false, true, nil
		case "date":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.date expects 0 arguments")
			}
			t, err := time.Parse(time.RFC3339, receiver.Fields["value"].String())
			if err != nil {
				return Null, receiver, false, true, err
			}
			return platformScalar("Date", t.Format("2006-01-02")), receiver, false, true, nil
		case "addDays":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("Datetime.addDays expects Integer")
			}
			t, err := time.Parse(time.RFC3339, receiver.Fields["value"].String())
			if err != nil {
				return Null, receiver, false, true, err
			}
			return platformScalar("Datetime", t.AddDate(0, 0, int(args[0].Int)).Format(time.RFC3339)), receiver, false, true, nil
		}
	case "Time", "Blob":
		switch method {
		case "format", "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
			}
			return String(receiver.Fields["value"].String()), receiver, false, true, nil
		}
	case "Database.SaveResult":
		switch method {
		case "isSuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.SaveResult.isSuccess expects 0 arguments")
			}
			return receiver.Fields["success"], receiver, false, true, nil
		case "getId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.SaveResult.getId expects 0 arguments")
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "getErrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.SaveResult.getErrors expects 0 arguments")
			}
			return receiver.Fields["errors"], receiver, false, true, nil
		}
	case "Database.Error":
		switch method {
		case "getMessage":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.Error.getMessage expects 0 arguments")
			}
			return receiver.Fields["message"], receiver, false, true, nil
		case "getStatusCode":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.Error.getStatusCode expects 0 arguments")
			}
			return receiver.Fields["statusCode"], receiver, false, true, nil
		}
	case "Exception":
		if method == "getMessage" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Exception.getMessage expects 0 arguments")
			}
			if message, ok := receiver.Fields["message"]; ok {
				return message, receiver, false, true, nil
			}
			return String(receiver.String()), receiver, false, true, nil
		}
	case "Type":
		if method == "getName" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Type.getName expects 0 arguments")
			}
			return String(receiver.Text), receiver, false, true, nil
		}
	case "Schema.DescribeSObjectResult":
		switch method {
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getKeyPrefix":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getKeyPrefix expects 0 arguments")
			}
			return receiver.Fields["keyPrefix"], receiver, false, true, nil
		}
	case "HttpRequest":
		switch method {
		case "setEndpoint":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setEndpoint expects String")
			}
			receiver.Fields["endpoint"] = args[0]
			return Null, receiver, true, true, nil
		case "setMethod":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setMethod expects String")
			}
			receiver.Fields["method"] = args[0]
			return Null, receiver, true, true, nil
		case "setBody":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setBody expects String")
			}
			receiver.Fields["body"] = args[0]
			return Null, receiver, true, true, nil
		case "getBody":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getBody expects 0 arguments")
			}
			return receiver.Fields["body"], receiver, false, true, nil
		}
	case "HttpResponse":
		switch method {
		case "setBody":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setBody expects String")
			}
			receiver.Fields["body"] = args[0]
			return Null, receiver, true, true, nil
		case "getBody":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getBody expects 0 arguments")
			}
			return receiver.Fields["body"], receiver, false, true, nil
		case "setStatusCode":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setStatusCode expects Integer")
			}
			receiver.Fields["statusCode"] = args[0]
			return Null, receiver, true, true, nil
		case "getStatusCode":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getStatusCode expects 0 arguments")
			}
			if value, ok := receiver.Fields["statusCode"]; ok {
				return value, receiver, false, true, nil
			}
			return Int(200), receiver, false, true, nil
		}
	case "Http":
		if method == "send" {
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "HttpRequest" {
				return Null, receiver, false, true, fmt.Errorf("Http.send expects HttpRequest")
			}
			if err := vm.incrementLimit("callouts", 1); err != nil {
				return Null, receiver, false, true, err
			}
			response := Object("HttpResponse")
			response.Fields["statusCode"] = Int(200)
			response.Fields["body"] = String("")
			if vm.testContext != nil && vm.testContext.HTTPMock.Kind == ValueObject {
				if target, ok := vm.resolveInstanceMethod(vm.testContext.HTTPMock.Type, "respond"); ok {
					value, err := vm.callMethodWithReceiver(target, vm.testContext.HTTPMock, []Value{args[0]}, &Result{})
					if err != nil {
						return Null, receiver, false, true, err
					}
					if value.Kind == ValueObject && value.Type == "HttpResponse" {
						return value, receiver, false, true, nil
					}
				} else {
					if body, ok := vm.testContext.HTTPMock.Fields["body"]; ok {
						response.Fields["body"] = body
					}
					if status, ok := vm.testContext.HTTPMock.Fields["statusCode"]; ok {
						response.Fields["statusCode"] = status
					}
				}
			}
			return response, receiver, false, true, nil
		}
	}
	return Null, receiver, false, false, nil
}
