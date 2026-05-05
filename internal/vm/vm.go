package vm

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	VarTypes         map[string]string
	Methods          map[string]Method
	MethodOverloads  map[string][]Method
	MethodFolded     map[string][]Method
	Classes          map[string]Class
	Org              *storage.OrgState
	Triggers         map[string][]Trigger
	Stdout           io.Writer
	callStack        []callFrame
	currentClass     string
	currentMethod    Method
	testContext      *TestContext
	localAsyncJobs   []AsyncJob
	localAsyncSeq    int
	localAsyncDrain  bool
	localAsyncChain  bool
	executionUser    Value
	limits           Limits
	limitCaps        LimitCaps
	limitMode        LimitMode
	limitViolations  []LimitViolation
	fakeNow          time.Time
	currentAsyncKind string
	activeExceptions []activeException
	currentStatement callFrame
	hasStatement     bool
	triggerDepth     int
	savepoints       map[string]storage.OrgState
	savepointOrder   map[string]int
	nextSavepoint    int
	pageMessages     []Value
	currentPage      Value
	restRequest      Value
	restResponse     Value
	debugHooks       DebugHooks
	hasDebugHooks    bool
	ctx              context.Context
	activeSetters    map[string]int
	triggerGlobals   map[string]Value
	cryptoRandomSeq  uint64
}

const maxTriggerDepth = 16

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
	if e.Type == "UnsupportedFeature" {
		return e.Message
	}
	return e.Type + ": " + e.Message
}

func unsupportedCallError(callee string) error {
	return &RuntimeError{Type: "UnsupportedFeature", Message: fmt.Sprintf("unsupported call %q", callee)}
}

type callFrame struct {
	Symbol string
	File   string
	Line   int
	Column int
}

type TestContext struct {
	Started          bool
	Stopped          bool
	CurrentUser      Value
	AsyncJobs        []AsyncJob
	Draining         bool
	HTTPMock         Value
	ParentLimits     Limits
	ParentViolations []LimitViolation
	RunAsDepth       int
	SetupDML         bool
	NonSetupDML      bool
	JobSeq           int
	ChainEnqueued    bool
}

type AsyncJob struct {
	ID        string
	Kind      string
	Object    Value
	Method    Method
	Args      []Value
	BatchSize int
	Name      string
	Cron      string
}

type Trigger struct {
	Name      string
	Namespace string
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
		VarTypes:        make(map[string]string),
		Methods:         make(map[string]Method),
		MethodOverloads: make(map[string][]Method),
		MethodFolded:    make(map[string][]Method),
		Classes:         make(map[string]Class),
		Triggers:        make(map[string][]Trigger),
		Stdout:          stdout,
		limitCaps:       defaultLimitCaps(),
		limitMode:       LimitModePermissive,
		fakeNow:         time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		savepoints:      make(map[string]storage.OrgState),
		savepointOrder:  make(map[string]int),
		ctx:             context.Background(),
		activeSetters:   make(map[string]int),
	}
}

func (vm *VM) SetOrg(org *storage.OrgState) {
	vm.Org = org
}

func (vm *VM) SetCurrentUser(record storage.Record) {
	if record.ID == "" {
		record.ID = storageIDFromValue(record.Fields["Id"])
	}
	if record.Object == "" {
		record.Object = "User"
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if record.ID == "" && len(record.Fields) == 0 {
		vm.executionUser = Null
		return
	}
	vm.executionUser = vmValueFromRecord(record)
}

func (vm *VM) SetDebugHooks(hooks DebugHooks) {
	vm.debugHooks = hooks
	vm.hasDebugHooks = true
}

func (vm *VM) SetContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	vm.ctx = ctx
}

func (vm *VM) newDMLEngine() dml.Engine {
	engine := dml.NewEngine(vm.Org)
	engine.Now = func() time.Time { return vm.fakeNow }
	if userID := vm.currentUserInfoField("Id", ""); userID != "" {
		engine.UserID = storage.ID(userID)
	}
	return engine
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
	vm.ensureAsyncObjects()
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
		appendTrace(&result, "apex.limits", "apex.limits", limitTraceArgs(vm.limits))
	}()
	out, err := vm.executeProgram(program, &result)
	if err != nil {
		var thrown *apexThrowError
		if errors.As(err, &thrown) {
			if len(thrown.stack) == 0 {
				thrown.stack = vm.rawStackFrames()
			}
			return result, runtimeError(thrown.value, thrown.stack)
		}
		return result, err
	}
	if out.signal == signalThrow {
		return result, runtimeError(out.thrown, out.thrownStack)
	}
	if out.signal == signalBreak || out.signal == signalContinue {
		return result, fmt.Errorf("%s outside loop", out.signal)
	}
	return result, nil
}

func (vm *VM) DrainAsync(result *Result) error {
	if vm.testContext != nil {
		return vm.drainTestAsync(result)
	}
	return vm.drainLocalAsync(result)
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
	value       Value
	signal      controlSignal
	thrown      Value
	thrownStack []callFrame
}

type apexThrowError struct {
	value Value
	stack []callFrame
}

type activeException struct {
	value Value
	stack []callFrame
}

func (e *apexThrowError) Error() string {
	return e.value.String()
}

func (vm *VM) executeProgram(program ir.Program, result *Result) (execOutcome, error) {
	for seq, inst := range program.Instructions {
		if err := vm.ctx.Err(); err != nil {
			return execOutcome{}, err
		}
		if err := vm.incrementLimit("cpuTime", 1); err != nil {
			return execOutcome{}, err
		}
		result.Trace = append(result.Trace, statementTraceEvent(seq, inst, program.Source))
		vm.setCurrentStatement(inst, program.Source)
		if err := vm.maybePauseForDebug(inst); err != nil {
			return execOutcome{}, err
		}
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
			coerced, err := vm.coerceAssignable(inst.Type, value)
			if err != nil {
				return execOutcome{}, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
			}
			value = coerced
			vm.Globals[inst.Name] = value
			vm.VarTypes[inst.Name] = inst.Type
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
			if err := vm.updateHeapLimit(); err != nil {
				return execOutcome{}, err
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
				out, err := vm.executeProgram(childProgram(program, branch), result)
				if err != nil || out.signal != signalNone {
					return out, err
				}
			}
		case ir.OpWhile:
			out, err := vm.executeWhile(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpDoWhile:
			out, err := vm.executeDoWhile(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpFor:
			out, err := vm.executeFor(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpForEach:
			out, err := vm.executeForEach(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpBreak:
			return execOutcome{signal: signalBreak}, nil
		case ir.OpContinue:
			return execOutcome{signal: signalContinue}, nil
		case ir.OpThrow:
			thrown := Null
			stack := vm.rawStackFrames()
			if inst.Expr.Kind != "" {
				value, err := vm.eval(inst.Expr, result)
				if err != nil {
					return execOutcome{}, err
				}
				thrown = annotateException(value, stack)
			} else {
				if len(vm.activeExceptions) == 0 {
					return execOutcome{}, fmt.Errorf("rethrow outside catch block")
				}
				active := vm.activeExceptions[len(vm.activeExceptions)-1]
				thrown = active.value
				stack = active.stack
			}
			return execOutcome{signal: signalThrow, thrown: thrown, thrownStack: stack}, nil
		case ir.OpTry:
			out, err := vm.executeTry(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpSwitch:
			out, err := vm.executeSwitch(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpRunAs:
			out, err := vm.executeRunAs(program.Source, inst, result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
		case ir.OpDML:
			if err := vm.executeDML(inst.Name, inst.Expr, inst.Field, result); err != nil {
				return execOutcome{}, err
			}
		default:
			return execOutcome{}, fmt.Errorf("unsupported instruction %q", inst.Op)
		}
		if err := vm.updateHeapLimit(); err != nil {
			return execOutcome{}, err
		}
	}
	return execOutcome{}, nil
}

func (vm *VM) updateHeapLimit() error {
	total := 0
	for name, value := range vm.Globals {
		total += len(name) + approxValueSize(value)
	}
	vm.limits.HeapSize = total
	return vm.checkLimit("heapSize", vm.limits.HeapSize, vm.limitCaps.HeapSize)
}

func statementTraceEvent(seq int, inst ir.Instruction, source string) trace.Event {
	args := map[string]any{
		"op":           string(inst.Op),
		"sourceOffset": inst.Pos,
	}
	if source != "" {
		line, column := sourceLineColumn(source, inst.Pos)
		args["line"] = line
		args["column"] = column
	}
	if inst.Name != "" {
		args["name"] = inst.Name
	}
	if inst.Type != "" {
		args["type"] = inst.Type
	}
	return trace.Instant("apex.statement."+string(inst.Op), "apex.statement", int64(seq), args)
}

func appendTrace(result *Result, name, category string, args map[string]any) {
	if result == nil {
		return
	}
	result.Trace = append(result.Trace, trace.Instant(name, category, int64(len(result.Trace)), args))
}

func limitTraceArgs(limits Limits) map[string]any {
	return map[string]any{
		"queries":          limits.Queries,
		"queryRows":        limits.QueryRows,
		"dmlStatements":    limits.DMLStatements,
		"dmlRows":          limits.DMLRows,
		"heapSize":         limits.HeapSize,
		"cpuTimeMs":        limits.CPUTimeMS,
		"callouts":         limits.Callouts,
		"asyncJobs":        limits.AsyncJobs,
		"futureCalls":      limits.FutureCalls,
		"queueableJobs":    limits.QueueableJobs,
		"batchJobs":        limits.BatchJobs,
		"scheduledJobs":    limits.ScheduledJobs,
		"emailInvocations": limits.EmailInvokes,
	}
}

func childProgram(parent ir.Program, instructions []ir.Instruction) ir.Program {
	return ir.Program{Instructions: instructions, Source: parent.Source}
}

func sourceLineColumn(source string, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	line, column := 1, 1
	for i := 0; i < offset; i++ {
		if source[i] == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}

func (vm *VM) setCurrentStatement(inst ir.Instruction, source string) {
	if source == "" {
		vm.hasStatement = false
		return
	}
	line, column := sourceLineColumn(source, inst.Pos)
	symbol := string(inst.Op)
	file := ""
	if vm.currentMethod.Name != "" {
		symbol = vm.currentMethod.Name
		file = vm.currentMethod.File
	}
	vm.currentStatement = callFrame{Symbol: symbol, File: file, Line: line, Column: column}
	vm.hasStatement = true
}

func (vm *VM) executeWhile(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
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
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
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

func (vm *VM) executeDoWhile(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("do/while loop exceeded %d iterations", maxLoopIterations)
		}
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
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

func (vm *VM) executeFor(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	if inst.Init != nil {
		out, err := vm.executeProgram(ir.Program{Instructions: []ir.Instruction{*inst.Init}, Source: source}, result)
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
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
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
			out, err := vm.executeProgram(ir.Program{Instructions: []ir.Instruction{*inst.Update}, Source: source}, result)
			if err != nil || out.signal == signalReturn || out.signal == signalThrow {
				return out, err
			}
		}
	}
}

func (vm *VM) executeForEach(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
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
	previousType, hadType := vm.VarTypes[inst.Name]
	defer func() {
		if existed {
			vm.Globals[inst.Name] = previous
		} else {
			delete(vm.Globals, inst.Name)
		}
		if hadType {
			vm.VarTypes[inst.Name] = previousType
		} else {
			delete(vm.VarTypes, inst.Name)
		}
	}()
	for iteration, value := range values {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("enhanced for loop exceeded %d iterations", maxLoopIterations)
		}
		coerced, err := vm.coerceAssignable(inst.Type, value)
		if err != nil {
			return execOutcome{}, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
		}
		vm.Globals[inst.Name] = coerced
		vm.VarTypes[inst.Name] = inst.Type
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
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

func (vm *VM) executeTry(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
	if err != nil {
		var thrown *apexThrowError
		if !errors.As(err, &thrown) {
			return execOutcome{}, err
		}
		if len(thrown.stack) == 0 {
			thrown.stack = vm.rawStackFrames()
		}
		thrown.value = annotateException(thrown.value, thrown.stack)
		out = execOutcome{signal: signalThrow, thrown: thrown.value, thrownStack: thrown.stack}
	}
	if out.signal == signalThrow {
		for _, catchClause := range vmCatchClauses(inst) {
			if !vm.exceptionMatchesAny(catchClause.Types, out.thrown) {
				continue
			}
			previous, existed := vm.Globals[catchClause.Name]
			vm.Globals[catchClause.Name] = out.thrown
			vm.activeExceptions = append(vm.activeExceptions, activeException{value: out.thrown, stack: out.thrownStack})
			out, err = vm.executeProgram(ir.Program{Instructions: catchClause.Body, Source: source}, result)
			vm.activeExceptions = vm.activeExceptions[:len(vm.activeExceptions)-1]
			if existed {
				vm.Globals[catchClause.Name] = previous
			} else {
				delete(vm.Globals, catchClause.Name)
			}
			if err != nil {
				return execOutcome{}, err
			}
			break
		}
	}
	if len(inst.Finally) > 0 {
		finallyOut, err := vm.executeProgram(ir.Program{Instructions: inst.Finally, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		if finallyOut.signal != signalNone {
			return finallyOut, nil
		}
	}
	return out, nil
}

func (vm *VM) executeSwitch(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
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
				out, err := vm.executeProgram(ir.Program{Instructions: c.Body, Source: source}, result)
				if err != nil {
					return execOutcome{}, err
				}
				if out.signal == signalBreak {
					return execOutcome{}, nil
				}
				return out, nil
			}
		}
	}
	if elseCase != nil {
		out, err := vm.executeProgram(ir.Program{Instructions: elseCase.Body, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		if out.signal == signalBreak {
			return execOutcome{}, nil
		}
		return out, nil
	}
	return execOutcome{}, nil
}

func (vm *VM) executeRunAs(source string, inst ir.Instruction, result *Result) (execOutcome, error) {
	user, err := vm.eval(inst.Expr, result)
	if err != nil {
		return execOutcome{}, err
	}
	if vm.testContext == nil {
		return execOutcome{}, fmt.Errorf("System.runAs is only available in test context")
	}
	previous := vm.testContext.CurrentUser
	vm.testContext.CurrentUser = user
	vm.testContext.RunAsDepth++
	defer func() {
		vm.testContext.RunAsDepth--
		vm.testContext.CurrentUser = previous
	}()
	return vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
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
		if expr.Operator == "instanceof" {
			return vm.evalInstanceOf(left, expr.Right.Name), nil
		}
		if expr.Operator == "&&" && left.Kind == ValueBool && !left.Bool {
			return Bool(false), nil
		}
		if expr.Operator == "||" && left.Kind == ValueBool && left.Bool {
			return Bool(true), nil
		}
		right, err := vm.eval(*expr.Right, result)
		if err != nil {
			return Null, err
		}
		return evalBinary(expr.Operator, left, right)
	case ir.ExprCall:
		if strings.HasPrefix(expr.Callee, "__cast:") {
			if len(expr.Args) != 1 {
				return Null, fmt.Errorf("cast expression requires 1 operand")
			}
			value, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			typeName := strings.TrimPrefix(expr.Callee, "__cast:")
			return vm.coerceAssignable(typeName, value)
		}
		if expr.Callee == "__ternary" {
			if len(expr.Args) != 3 {
				return Null, fmt.Errorf("ternary expression requires 3 operands")
			}
			condition, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			if condition.Kind != ValueBool {
				return Null, fmt.Errorf("ternary condition requires Boolean, got %s", condition.Kind)
			}
			if condition.Bool {
				return vm.eval(expr.Args[1], result)
			}
			return vm.eval(expr.Args[2], result)
		}
		if expr.Callee == "__coalesce" {
			if len(expr.Args) != 2 {
				return Null, fmt.Errorf("null coalescing expression requires 2 operands")
			}
			left, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			if left.Kind != ValueNull {
				return left, nil
			}
			return vm.eval(expr.Args[1], result)
		}
		if expr.Callee == "__mapEntry" {
			if len(expr.Args) != 2 {
				return Null, fmt.Errorf("map entry requires key and value")
			}
			key, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			value, err := vm.eval(expr.Args[1], result)
			if err != nil {
				return Null, err
			}
			entry := Object("__mapEntry")
			entry.Fields["__key"] = key
			entry.Fields["__value"] = value
			return entry, nil
		}
		if strings.HasPrefix(expr.Callee, "__field:") {
			if expr.Left == nil {
				return Null, fmt.Errorf("field access requires receiver")
			}
			receiver, err := vm.eval(*expr.Left, result)
			if err != nil {
				return Null, err
			}
			return vm.lookupPath(receiver, []string{strings.TrimPrefix(expr.Callee, "__field:")})
		}
		var receiver Value
		hasReceiver := expr.Left != nil
		if hasReceiver {
			var err error
			receiver, err = vm.eval(*expr.Left, result)
			if err != nil {
				return Null, err
			}
		}
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
		if hasReceiver {
			value, handled, err := vm.callValueMember("", receiver, expr.Callee, args, result)
			if handled || err != nil {
				return value, err
			}
			return Null, unsupportedCallError(expr.Callee)
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
		dispatchClass := vm.currentClass
		receiver := Null
		if this, ok := vm.Globals["this"]; ok {
			receiver = this
			if this.Kind == ValueObject && this.Type != "" {
				dispatchClass = this.Type
			}
		}
		if method, ok, ambiguous := vm.resolveInstanceMethodForArgs(dispatchClass, callee, args); ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethodWithReceiver(method, receiver, args, result)
		} else if ambiguous {
			return Null, fmt.Errorf("ambiguous overload for call %q", callee)
		}
		if method, ok, ambiguous := vm.resolveStaticMethodForArgs(vm.currentClass, callee, args); ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethod(method, args, result)
		} else if ambiguous {
			return Null, fmt.Errorf("ambiguous overload for call %q", callee)
		}
	}
	if method, ok, ambiguous := vm.matchRegisteredMethod(callee, args); ok {
		if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
			return Null, err
		}
		if vm.shouldEnqueueFuture(method) {
			return vm.enqueueFuture(method, args, result)
		}
		return vm.callMethod(method, args, result)
	} else if ambiguous {
		return Null, fmt.Errorf("ambiguous overload for call %q", callee)
	}
	if value, handled, err := vm.callSchemaSObjectTypePath(callee, args, result); handled || err != nil {
		return value, err
	}
	if value, handled, err := vm.callDottedReceiverMember(callee, args, result); handled || err != nil {
		return value, err
	}
	if dot := strings.LastIndex(callee, "."); dot > 0 && dot < len(callee)-1 {
		if value, handled, err := vm.callCustomDataStaticMember(callee[:dot], callee[dot+1:], args); handled || err != nil {
			return value, err
		}
	}
	if className, methodName, ok := vm.splitClassMember(callee); ok {
		if value, handled, err := vm.callCustomDataStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
		if value, handled, err := vm.callEnumStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
		if method, ok, ambiguous := vm.resolveStaticMethodForArgs(className, methodName, args); ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethod(method, args, result)
		} else if ambiguous {
			return Null, fmt.Errorf("ambiguous overload for call %q", callee)
		}
	}
	if strings.HasPrefix(callee, "Search.") {
		return Null, unsupportedCallError(callee + " local search/SOSL surface")
	}
	if strings.HasPrefix(callee, "System.JSON.") {
		callee = "JSON." + strings.TrimPrefix(callee, "System.JSON.")
	}
	if strings.HasPrefix(callee, "System.Json.") {
		callee = "JSON." + strings.TrimPrefix(callee, "System.Json.")
	}
	if strings.HasPrefix(callee, "Json.") {
		callee = "JSON." + strings.TrimPrefix(callee, "Json.")
	}
	if strings.HasPrefix(callee, "DateTime.") {
		callee = "Datetime." + strings.TrimPrefix(callee, "DateTime.")
	}
	if strings.HasPrefix(callee, "Datetime.") && len(callee) > len("Datetime.") {
		member := strings.TrimPrefix(callee, "Datetime.")
		switch strings.ToLower(member) {
		case "now":
			callee = "Datetime.now"
		case "newinstance":
			callee = "Datetime.newInstance"
		case "newinstancegmt":
			callee = "Datetime.newInstanceGmt"
		}
	}
	if reason, ok := unsupportedIntegrationSurface(callee); ok {
		return Null, unsupportedCallError(callee + " " + reason)
	}
	if strings.HasPrefix(callee, "Limits.") && unsupportedLimitGetter(strings.TrimPrefix(callee, "Limits.")) {
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Null, unsupportedCallError(callee)
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
			message, err := vm.assertMessage("assertion failed", args[1:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "System.assertEquals":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("System.assertEquals expects 2 or 3 arguments")
		}
		if !args[0].Equal(args[1]) {
			expected, err := vm.displayString(args[0], result)
			if err != nil {
				return Null, err
			}
			actual, err := vm.displayString(args[1], result)
			if err != nil {
				return Null, err
			}
			message, err := vm.assertMessage(fmt.Sprintf("expected <%s>, actual <%s>", expected, actual), args[2:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "System.assertNotEquals":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("System.assertNotEquals expects 2 or 3 arguments")
		}
		if args[0].Equal(args[1]) {
			value, err := vm.displayString(args[0], result)
			if err != nil {
				return Null, err
			}
			message, err := vm.assertMessage(fmt.Sprintf("values should not be equal: <%s>", value), args[2:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "System.debug":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("System.debug expects message or logging level and message")
		}
		messageArg := args[0]
		if len(args) == 2 {
			if !isLoggingLevelValue(args[0]) {
				return Null, fmt.Errorf("System.debug expects LoggingLevel as first argument")
			}
			messageArg = args[1]
		}
		line, err := vm.displayString(messageArg, result)
		if err != nil {
			return Null, err
		}
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
	case "Database.queryWithBinds":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("Database.queryWithBinds expects query String, bind Map, and optional AccessLevel")
		}
		if args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, fmt.Errorf("Database.queryWithBinds expects query String and bind Map")
		}
		if len(args) == 3 && (args[2].Kind != ValueObject || args[2].Type != "AccessLevel") {
			return Null, fmt.Errorf("Database.queryWithBinds expects AccessLevel")
		}
		return vm.executeSOQLWithBindMap(args[0].Text, args[1], result)
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
	case "Database.getQueryLocator":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Database.getQueryLocator expects query String")
		}
		value, err := vm.executeSOQL(args[0].Text, result)
		if err != nil {
			return Null, err
		}
		locator := Object("Database.QueryLocator")
		locator.Fields["Records"] = value
		locator.Fields["Query"] = String(args[0].Text)
		return locator, nil
	case "Database.setSavepoint":
		if len(args) != 0 {
			return Null, fmt.Errorf("Database.setSavepoint expects 0 arguments")
		}
		if vm.Org == nil {
			return Null, fmt.Errorf("Database.setSavepoint requires org storage")
		}
		if err := vm.incrementLimit("dmlStatements", 1); err != nil {
			return Null, err
		}
		vm.nextSavepoint++
		id := fmt.Sprintf("sp-%d", vm.nextSavepoint)
		vm.savepoints[id] = vm.Org.Clone()
		vm.savepointOrder[id] = vm.nextSavepoint
		savepoint := Object("System.Savepoint")
		savepoint.Fields["Id"] = String(id)
		return savepoint, nil
	case "Database.rollback":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "System.Savepoint" {
			return Null, fmt.Errorf("Database.rollback expects Savepoint")
		}
		if vm.Org == nil {
			return Null, fmt.Errorf("Database.rollback requires org storage")
		}
		idValue, ok := args[0].Fields["Id"]
		if !ok || idValue.Kind != ValueString {
			return Null, fmt.Errorf("Database.rollback received invalid Savepoint")
		}
		snapshot, ok := vm.savepoints[idValue.Text]
		if !ok {
			return Null, fmt.Errorf("Database.rollback received invalid Savepoint")
		}
		targetOrder := vm.savepointOrder[idValue.Text]
		if err := vm.incrementLimit("dmlStatements", 1); err != nil {
			return Null, err
		}
		restored := snapshot.Clone()
		*vm.Org = restored
		for id, order := range vm.savepointOrder {
			if order > targetOrder {
				delete(vm.savepoints, id)
				delete(vm.savepointOrder, id)
			}
		}
		return Null, nil
	case "Database.upsert", "Database.undelete":
		return vm.executeDatabaseDML(strings.TrimPrefix(callee, "Database."), args, result)
	case "Database.insert", "Database.update", "Database.delete":
		return vm.executeDatabaseDML(strings.TrimPrefix(callee, "Database."), args, result)
	case "Database.emptyRecycleBin":
		return vm.executeDatabaseRecordAction("emptyRecycleBin", args, result)
	case "Database.lock", "Database.unlock":
		return vm.executeDatabaseRecordAction(strings.TrimPrefix(callee, "Database."), args, result)
	case "Database.convertLead":
		return Null, unsupportedCallError("Database.convertLead local lead conversion surface")
	case "Approval.process", "Approval.lock", "Approval.unlock", "Approval.isLocked":
		return Null, unsupportedCallError(callee + " local approval process and lock surface")
	case "Database.merge":
		return vm.executeDatabaseMerge(args, result)
	case "Limits.getQueries", "Limits.getLimitQueries", "Limits.getQueryRows", "Limits.getLimitQueryRows",
		"Limits.getDmlStatements", "Limits.getLimitDmlStatements", "Limits.getDmlRows", "Limits.getLimitDmlRows",
		"Limits.getDMLStatements", "Limits.getLimitDMLStatements", "Limits.getDMLRows", "Limits.getLimitDMLRows",
		"Limits.getHeapSize", "Limits.getLimitHeapSize", "Limits.getCpuTime", "Limits.getLimitCpuTime",
		"Limits.getCallouts", "Limits.getLimitCallouts", "Limits.getQueueableJobs", "Limits.getLimitQueueableJobs",
		"Limits.getFutureCalls", "Limits.getLimitFutureCalls", "Limits.getAsyncJobs", "Limits.getLimitAsyncJobs",
		"Limits.getBatchJobs", "Limits.getLimitBatchJobs", "Limits.getScheduledJobs", "Limits.getLimitScheduledJobs",
		"Limits.getEmailInvocations", "Limits.getLimitEmailInvocations":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		if value, ok := vm.limitValue(strings.TrimPrefix(callee, "Limits.")); ok {
			return value, nil
		}
		return Null, unsupportedCallError(callee)
	case "String.valueOf":
		if len(args) != 1 {
			return Null, fmt.Errorf("String.valueOf expects 1 argument")
		}
		text, err := vm.displayString(args[0], result)
		if err != nil {
			return Null, err
		}
		return String(text), nil
	case "String.isBlank", "String.isNotBlank", "String.isEmpty", "String.isNotEmpty", "String.join", "String.format", "String.getCommonPrefix", "String.getLevenshteinDistance", "String.stripAll", "String.fromCharArray", "String.escapeSingleQuotes":
		return stringStatic(callee, args)
	case "Integer.valueOf", "Long.valueOf", "Decimal.valueOf", "Double.valueOf":
		return numericStatic(callee, args)
	case "RoundingMode.valueOf":
		return roundingModeStatic(args)
	case "Id.valueOf":
		return idStatic(callee, args)
	case "Pattern.compile":
		return patternCompile(args)
	case "Pattern.matches":
		return patternMatches(args)
	case "Pattern.quote":
		return patternQuote(args)
	case "Math.abs", "Math.floor", "Math.ceil", "Math.round", "Math.roundToLong", "Math.signum", "Math.sqrt",
		"Math.acos", "Math.asin", "Math.atan", "Math.cos", "Math.sin", "Math.tan", "Math.exp", "Math.log", "Math.log10":
		return mathUnary(callee, args)
	case "Math.max", "Math.min", "Math.mod", "Math.pow", "Math.atan2":
		return mathBinary(callee, args)
	case "Date.today", "Date.Today", "System.today":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return platformScalar("Date", vm.fakeNow.Format("2006-01-02")), nil
	case "Date.newInstance":
		if len(args) != 3 || args[0].Kind != ValueInt || args[1].Kind != ValueInt || args[2].Kind != ValueInt {
			return Null, fmt.Errorf("Date.newInstance expects year, month, day integers")
		}
		if err := validateDateParts(int(args[0].Int), int(args[1].Int), int(args[2].Int)); err != nil {
			return Null, err
		}
		return platformScalar("Date", fmt.Sprintf("%04d-%02d-%02d", args[0].Int, args[1].Int, args[2].Int)), nil
	case "Date.valueOf":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Date.valueOf expects String")
		}
		date, err := parseDateText(args[0].Text)
		if err != nil {
			return Null, err
		}
		return platformScalar("Date", date.Format("2006-01-02")), nil
	case "Datetime.now", "System.now":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return platformScalar("Datetime", vm.fakeNow.Format(time.RFC3339)), nil
	case "System.currentTimeMillis":
		if len(args) != 0 {
			return Null, fmt.Errorf("System.currentTimeMillis expects 0 arguments")
		}
		return Int(vm.fakeNow.UnixMilli()), nil
	case "System.isBatch", "System.isFuture", "System.isQueueable", "System.isScheduled":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Bool(vm.isAsyncKind(callee)), nil
	case "System.abortJob":
		return vm.abortJob(args)
	case "Database.scheduleBatch", "System.scheduleBatch":
		return Null, unsupportedCallError(callee + " local async scheduling surface")
	case "System.attachFinalizer":
		return Null, unsupportedCallError("System.attachFinalizer local queueable finalizers")
	case "AsyncInfo.getCurrentQueueableStackDepth", "AsyncInfo.getMaximumQueueableStackDepth",
		"AsyncInfo.getMinimumQueueableDelayInMinutes", "AsyncInfo.hasMaxStackDepth":
		return Null, unsupportedCallError(callee + " local async info surface")
	case "Datetime.newInstance", "Datetime.newInstanceGmt":
		if len(args) == 2 {
			if args[0].Kind != ValueObject || args[0].Type != "Date" || args[1].Kind != ValueObject || args[1].Type != "Time" {
				return Null, fmt.Errorf("%s expects Date and Time", callee)
			}
			date, err := parsePlatformDate(args[0])
			if err != nil {
				return Null, err
			}
			clock, err := parsePlatformTime(args[1])
			if err != nil {
				return Null, err
			}
			hour := int(clock / time.Hour)
			clock %= time.Hour
			minute := int(clock / time.Minute)
			clock %= time.Minute
			second := int(clock / time.Second)
			clock %= time.Second
			millisecond := int(clock / time.Millisecond)
			zoneID := "UTC"
			if callee == "Datetime.newInstance" {
				zoneID = vm.currentUserTimeZoneID()
			}
			value, err := datetimeFromLocalParts(date.Year(), int(date.Month()), date.Day(), hour, minute, second, millisecond, zoneID)
			if err != nil {
				return Null, err
			}
			return platformScalar("Datetime", formatPlatformDatetime(value)), nil
		}
		if len(args) != 3 && len(args) != 6 {
			return Null, fmt.Errorf("%s expects year, month, day[, hour, minute, second] integers", callee)
		}
		for i := 0; i < len(args); i++ {
			if args[i].Kind != ValueInt {
				return Null, fmt.Errorf("%s expects integer parts", callee)
			}
		}
		year, month, day := int(args[0].Int), int(args[1].Int), int(args[2].Int)
		hour, minute, second := 0, 0, 0
		if len(args) == 6 {
			hour, minute, second = int(args[3].Int), int(args[4].Int), int(args[5].Int)
		}
		if err := validateDateParts(year, month, day); err != nil {
			return Null, err
		}
		if err := validateTimeParts(hour, minute, second); err != nil {
			return Null, err
		}
		zoneID := "UTC"
		if callee == "Datetime.newInstance" {
			zoneID = vm.currentUserTimeZoneID()
		}
		value, err := datetimeFromLocalParts(year, month, day, hour, minute, second, 0, zoneID)
		if err != nil {
			return Null, err
		}
		return platformScalar("Datetime", formatPlatformDatetime(value)), nil
	case "Datetime.valueOf", "Datetime.valueOfGmt":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects String", callee)
		}
		value, err := parseDatetimeText(args[0].Text)
		if err != nil {
			return Null, err
		}
		return platformScalar("Datetime", formatPlatformDatetime(value)), nil
	case "LoggingLevel.values":
		return loggingLevelValues(args)
	case "ApexPages.Severity.values":
		return apexPagesSeverityValues(args)
	case "RoundingMode.values":
		return roundingModeValues(args)
	case "System.isRunningTest", "System.Test.isRunningTest", "Test.isRunningTest":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Bool(vm.testContext != nil), nil
	case "Test.Database.hasRecords":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.Database.hasRecords expects 0 arguments")
		}
		if vm.testContext != nil {
			return Null, unsupportedCallError("Test.Database.hasRecords local fflib test database surface")
		}
		return Bool(false), nil
	case "Type.forName":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Type.forName expects type name or namespace and type name")
		}
		if len(args) == 1 {
			if args[0].Kind == ValueNull {
				return Null, nil
			}
			if args[0].Kind != ValueString {
				return Null, fmt.Errorf("Type.forName expects String")
			}
			return vm.typeForName("", args[0].Text), nil
		}
		if args[0].Kind != ValueString && args[0].Kind != ValueNull {
			return Null, fmt.Errorf("Type.forName expects namespace String or null")
		}
		if args[1].Kind == ValueNull {
			return Null, nil
		}
		if args[1].Kind != ValueString {
			return Null, fmt.Errorf("Type.forName expects type name String")
		}
		namespace := ""
		if args[0].Kind == ValueString {
			namespace = args[0].Text
		}
		return vm.typeForName(namespace, args[1].Text), nil
	case "Test.getStandardPricebookId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.getStandardPricebookId expects 0 arguments")
		}
		if vm.testContext == nil {
			return Null, fmt.Errorf("Test.getStandardPricebookId is only available in test context")
		}
		return String("01s000000000001"), nil
	case "Time.newInstance":
		if len(args) < 3 || len(args) > 4 {
			return Null, fmt.Errorf("Time.newInstance expects hour, minute, second[, millisecond]")
		}
		for i := 0; i < len(args); i++ {
			if args[i].Kind != ValueInt {
				return Null, fmt.Errorf("Time.newInstance expects integer parts")
			}
		}
		if err := validateTimeParts(int(args[0].Int), int(args[1].Int), int(args[2].Int)); err != nil {
			return Null, err
		}
		millisecond := 0
		if len(args) == 4 {
			if args[3].Int < 0 || args[3].Int > 999 {
				return Null, fmt.Errorf("invalid Time millisecond: %d", args[3].Int)
			}
			millisecond = int(args[3].Int)
		}
		return platformScalar("Time", formatPlatformTime(int(args[0].Int), int(args[1].Int), int(args[2].Int), millisecond)), nil
	case "Time.valueOf":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Time.valueOf expects String")
		}
		parsed, err := parseTimeText(args[0].Text)
		if err != nil {
			return Null, err
		}
		return platformScalar("Time", parsed), nil
	case "TimeZone.getTimeZone":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("TimeZone.getTimeZone expects String")
		}
		return fixedTimeZone(args[0].Text)
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
			return Null, fmt.Errorf("EncodingUtil.base64Decode invalid base64 string: %w", err)
		}
		return platformScalar("Blob", string(decoded)), nil
	case "EncodingUtil.convertFromHex":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("EncodingUtil.convertFromHex expects String")
		}
		decoded, err := hex.DecodeString(args[0].Text)
		if err != nil {
			return Null, fmt.Errorf("EncodingUtil.convertFromHex invalid hexadecimal string: %w", err)
		}
		return platformScalar("Blob", string(decoded)), nil
	case "EncodingUtil.convertToHex":
		blob, err := blobStringArg("EncodingUtil.convertToHex", args)
		if err != nil {
			return Null, err
		}
		return String(hex.EncodeToString([]byte(blob))), nil
	case "EncodingUtil.urlEncode":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, fmt.Errorf("EncodingUtil.urlEncode expects String and charset")
		}
		encoded, err := urlEncodeWithCharset("EncodingUtil.urlEncode", args[0].Text, args[1].Text)
		if err != nil {
			return Null, err
		}
		return String(encoded), nil
	case "EncodingUtil.urlDecode":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, fmt.Errorf("EncodingUtil.urlDecode expects String and charset")
		}
		decoded, err := urlDecodeWithCharset("EncodingUtil.urlDecode", args[0].Text, args[1].Text)
		if err != nil {
			return Null, err
		}
		return String(decoded), nil
	case "Crypto.generateDigest":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.generateDigest expects algorithm and Blob")
		}
		blob, err := blobStringArg("Crypto.generateDigest", args[1:])
		if err != nil {
			return Null, err
		}
		digest, err := generateDigest(args[0].Text, []byte(blob))
		if err != nil {
			return Null, err
		}
		return platformScalar("Blob", string(digest)), nil
	case "Crypto.generateMac":
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.generateMac expects algorithm, input Blob, and privateKey Blob")
		}
		input, err := blobStringArg("Crypto.generateMac input", args[1:2])
		if err != nil {
			return Null, err
		}
		key, err := blobStringArg("Crypto.generateMac privateKey", args[2:])
		if err != nil {
			return Null, err
		}
		mac, err := generateMac(args[0].Text, []byte(input), []byte(key))
		if err != nil {
			return Null, err
		}
		return platformScalar("Blob", string(mac)), nil
	case "Crypto.areEqualConstantTime":
		if len(args) != 2 {
			return Null, fmt.Errorf("Crypto.areEqualConstantTime expects left Blob and right Blob")
		}
		left, err := blobStringArg("Crypto.areEqualConstantTime left", args[:1])
		if err != nil {
			return Null, err
		}
		right, err := blobStringArg("Crypto.areEqualConstantTime right", args[1:])
		if err != nil {
			return Null, err
		}
		return Bool(subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1), nil
	case "Crypto.encrypt", "Crypto.decrypt", "Crypto.encryptWithManagedIV", "Crypto.decryptWithManagedIV",
		"Crypto.sign", "Crypto.signWithCertificate", "Crypto.verify", "Crypto.verifyWithCertificate":
		return Null, unsupportedCallError(callee + " local deterministic key, certificate, and encryption surfaces")
	case "Crypto.generateAESKey", "Crypto.getRandomInteger":
		return Null, unsupportedCallError(callee + " local deterministic random/key generation surface")
	case "Crypto.getRandomLong":
		if len(args) != 0 {
			return Null, fmt.Errorf("Crypto.getRandomLong expects 0 arguments")
		}
		return Int(vm.nextDeterministicCryptoLong()), nil
	case "JSON.createGenerator":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, fmt.Errorf("JSON.createGenerator expects Boolean")
		}
		return newJSONGenerator(args[0].Bool), nil
	case "JSON.createParser":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("JSON.createParser expects String")
		}
		return newJSONParser(args[0].Text)
	case "JSON.serialize":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("JSON.serialize expects 1 or 2 arguments")
		}
		suppressNulls, err := jsonSuppressNulls("JSON.serialize", args[1:])
		if err != nil {
			return Null, err
		}
		data, err := json.Marshal(jsonFromValue(args[0], suppressNulls))
		if err != nil {
			return Null, err
		}
		return String(string(data)), nil
	case "JSON.serializePretty":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("JSON.serializePretty expects 1 or 2 arguments")
		}
		suppressNulls, err := jsonSuppressNulls("JSON.serializePretty", args[1:])
		if err != nil {
			return Null, err
		}
		data, err := json.MarshalIndent(jsonFromValue(args[0], suppressNulls), "", "  ")
		if err != nil {
			return Null, err
		}
		return String(string(data)), nil
	case "JSON.deserializeUntyped":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("JSON.deserializeUntyped expects String")
		}
		decoded, err := decodeJSONValue(args[0].Text)
		if err != nil {
			return Null, jsonDeserializeException("JSON.deserializeUntyped invalid JSON input: %v", err)
		}
		return valueFromJSON(decoded), nil
	case "JSON.deserialize", "JSON.deserializeStrict":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects String and Type", callee)
		}
		strict := callee == "JSON.deserializeStrict"
		decoded, err := decodeJSONValueForDeserialize(args[0].Text, strict)
		if err != nil {
			return Null, jsonDeserializeException("%s", err.Error())
		}
		if args[1].Kind == ValueObject && args[1].Type == "Type" {
			return vm.typedValueFromJSON(typeValueName(args[1]), decoded, strict)
		}
		return valueFromJSON(decoded), nil
	case "Schema.getGlobalDescribe":
		if len(args) != 0 {
			return Null, fmt.Errorf("Schema.getGlobalDescribe expects 0 arguments")
		}
		appendTrace(result, "apex.describe.global", "apex.describe", map[string]any{"operation": "getGlobalDescribe"})
		return vm.schemaGlobalDescribe(), nil
	case "Schema.describeSObjects":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("Schema.describeSObjects expects List")
		}
		if vm.Org == nil {
			return Null, fmt.Errorf("Schema.describeSObjects requires org state")
		}
		describes := make([]Value, 0, len(args[0].List))
		for _, item := range args[0].List {
			objectName, err := vm.schemaDescribeObjectName(item)
			if err != nil {
				return Null, err
			}
			resolved, ok := storage.ResolveObjectName(*vm.Org, objectName)
			if !ok {
				return Null, fmt.Errorf("Schema.describeSObjects unknown object %s", objectName)
			}
			describes = append(describes, vm.describeSObjectValue(resolved, vm.Org.Objects[resolved].Definition))
		}
		appendTrace(result, "apex.describe.sobjects", "apex.describe", map[string]any{
			"operation": "describeSObjects",
			"count":     len(describes),
		})
		return List(describes...), nil
	case "FeatureManagement.checkPermission":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("FeatureManagement.checkPermission expects String")
		}
		if vm.testContext != nil && userHasPermission(vm.testContext.CurrentUser, args[0].Text) {
			return Bool(true), nil
		}
		if userHasPermission(vm.executionUser, args[0].Text) {
			return Bool(true), nil
		}
		return Bool(false), nil
	case "EventBus.publish":
		return vm.eventBusPublish(args, result)
	case "EventBus.publishAfterCommit":
		return Null, unsupportedCallError(callee + " local platform event after-commit delivery surface")
	case "ConnectApi.Organization.getSettings":
		return vm.connectAPIOrganizationSettings(args)
	case "Messaging.sendEmail":
		if len(args) == 0 {
			return Null, fmt.Errorf("Messaging.sendEmail expects messages")
		}
		if len(args) > 2 {
			return Null, unsupportedCallError("Messaging.sendEmail send options overloads")
		}
		if args[0].Kind != ValueList {
			return Null, fmt.Errorf("Messaging.sendEmail expects List")
		}
		if len(args) == 2 && args[1].Kind != ValueBool {
			return Null, unsupportedCallError("Messaging.sendEmail send options overloads")
		}
		for _, message := range args[0].List {
			if !isLocalEmailMessage(message) {
				return Null, fmt.Errorf("Messaging.sendEmail expects SingleEmailMessage or MassEmailMessage list items")
			}
		}
		if err := vm.incrementLimit("emailInvocations", 1); err != nil {
			return Null, err
		}
		appendTrace(result, "apex.email.send", "apex.email", map[string]any{"messages": len(args[0].List)})
		results := make([]Value, 0, len(args[0].List))
		for range args[0].List {
			results = append(results, newSendEmailResult())
		}
		return List(results...), nil
	case "ApexPages.hasMessages":
		if len(args) != 0 {
			return Null, fmt.Errorf("ApexPages.hasMessages expects 0 arguments")
		}
		return Bool(len(vm.pageMessages) > 0), nil
	case "ApexPages.addMessage":
		if len(args) != 1 {
			return Null, fmt.Errorf("ApexPages.addMessage expects 1 argument")
		}
		vm.pageMessages = append(vm.pageMessages, args[0])
		return Null, nil
	case "ApexPages.getMessages":
		if len(args) != 0 {
			return Null, fmt.Errorf("ApexPages.getMessages expects 0 arguments")
		}
		return List(vm.pageMessages...), nil
	case "Test.clearApexPageMessages":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.clearApexPageMessages expects 0 arguments")
		}
		if err := vm.requireTestContext("Test.clearApexPageMessages"); err != nil {
			return Null, err
		}
		vm.pageMessages = nil
		return Null, nil
	case "ApexPages.currentPage":
		if len(args) != 0 {
			return Null, fmt.Errorf("ApexPages.currentPage expects 0 arguments")
		}
		if vm.currentPage.Kind == "" {
			vm.currentPage = newPageReference("/apex/current")
		}
		return vm.currentPage, nil
	case "Test.setCurrentPage":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "PageReference" {
			return Null, fmt.Errorf("Test.setCurrentPage expects PageReference")
		}
		if err := vm.requireTestContext("Test.setCurrentPage"); err != nil {
			return Null, err
		}
		vm.currentPage = args[0]
		return Null, nil
	case "Messaging.reserveSingleEmailCapacity", "Messaging.reserveMassEmailCapacity",
		"Messaging.renderEmailTemplate", "Messaging.renderStoredEmailTemplate",
		"Messaging.sendEmailMessage", "Messaging.sendPushNotification":
		return Null, unsupportedCallError(callee + " local messaging transport/template surface")
	case "URL.getSalesforceBaseUrl", "URL.getOrgDomainUrl":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return platformScalar("URL", "https://local.oaer.example"), nil
	case "URL.getCurrentRequestUrl":
		if len(args) != 0 {
			return Null, fmt.Errorf("URL.getCurrentRequestUrl expects 0 arguments")
		}
		return Null, unsupportedCallError(callee + " local current request URL surface")
	case "Test.setMock":
		return vm.testSetMock(args)
	case "Test.createStub":
		if vm.testContext == nil {
			return Null, unsupportedCallError(callee + " local stub API")
		}
		return vm.testCreateStub(args)
	case "Test.createSoqlStub":
		return Null, unsupportedCallError(callee + " local stub API")
	case "Continuation.addHttpRequest", "Continuation.getResponse":
		return Null, unsupportedCallError(callee + " local continuation callout surface")
	case "Test.setFixedSearchResults":
		return Null, unsupportedCallError(callee + " local SOSL fixed search results")
	case "Test.startTest":
		return vm.testStart()
	case "Test.stopTest":
		return vm.testStop(result)
	case "System.enqueueJob":
		return vm.enqueueJob(args, result)
	case "Database.executeBatch":
		return vm.executeBatch(args, result)
	case "System.schedule":
		return vm.scheduleJob(args, result)
	case "UserInfo.getUserId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserId expects 0 arguments")
		}
		return String(vm.currentUserInfoField("Id", "system")), nil
	case "UserInfo.getProfileId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getProfileId expects 0 arguments")
		}
		return String(vm.currentUserInfoField("ProfileId", "")), nil
	case "UserInfo.getUserName":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserName expects 0 arguments")
		}
		return String(vm.currentUserInfoField("Username", vm.currentUserInfoField("Id", "system"))), nil
	case "UserInfo.getName":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getName expects 0 arguments")
		}
		return String(vm.currentUserInfoField("Name", "System User")), nil
	case "UserInfo.getFirstName":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getFirstName expects 0 arguments")
		}
		return String(vm.currentUserInfoField("FirstName", "System")), nil
	case "UserInfo.getLastName":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getLastName expects 0 arguments")
		}
		return String(vm.currentUserInfoField("LastName", "User")), nil
	case "UserInfo.getUserEmail":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserEmail expects 0 arguments")
		}
		return String(vm.currentUserInfoField("Email", "system@example.invalid")), nil
	case "UserInfo.getOrganizationId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getOrganizationId expects 0 arguments")
		}
		return String("00D000000000001"), nil
	case "UserInfo.getSessionId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getSessionId expects 0 arguments")
		}
		return String(""), nil
	case "UserInfo.getLocale":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getLocale expects 0 arguments")
		}
		return String(vm.currentUserInfoField("LocaleSidKey", "en_US")), nil
	case "UserInfo.getLanguage":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getLanguage expects 0 arguments")
		}
		return String(vm.currentUserInfoField("LanguageLocaleKey", "en_US")), nil
	case "UserInfo.getTimeZone":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getTimeZone expects 0 arguments")
		}
		return fixedTimeZone(vm.currentUserTimeZoneID())
	default:
		if strings.HasPrefix(callee, "Crypto.") {
			return Null, unsupportedCallError(callee + " local key, certificate, encryption, and random surfaces")
		}
		return Null, unsupportedCallError(callee)
	}
}

func (vm *VM) nextDeterministicCryptoLong() int64 {
	vm.cryptoRandomSeq += 0x9e3779b97f4a7c15
	z := vm.cryptoRandomSeq
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	return int64(z)
}

func unsupportedIntegrationSurface(callee string) (string, bool) {
	for _, prefix := range []string{"Approval.", "Auth.", "QuickAction.", "Canvas.", "Continuation."} {
		if strings.HasPrefix(callee, prefix) {
			switch prefix {
			case "Approval.":
				return "local approval process and lock surface", true
			case "Auth.":
				return "local authentication token/cloud API surface", true
			case "QuickAction.":
				return "local quick action UI surface", true
			case "Canvas.":
				return "local canvas app integration surface", true
			case "Continuation.":
				return "local continuation callout surface", true
			}
		}
	}
	return "", false
}

func (vm *VM) eventBusPublish(args []Value, result *Result) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("EventBus.publish expects event record or list")
	}
	records := []Value{args[0]}
	if args[0].Kind == ValueList {
		records = args[0].List
	}
	results := make([]Value, 0, len(records))
	for _, record := range records {
		if record.Kind != ValueObject {
			return Null, fmt.Errorf("EventBus.publish expects SObject event record(s)")
		}
		row := Object("Database.SaveResult")
		row.Fields["success"] = Bool(true)
		row.Fields["id"] = Null
		row.Fields["error"] = String("")
		row.Fields["errors"] = List()
		results = append(results, row)
	}
	appendTrace(result, "apex.eventbus.publish", "apex.eventbus", map[string]any{
		"records":  len(records),
		"delivery": "local-noop",
	})
	if args[0].Kind == ValueList {
		return List(results...), nil
	}
	if len(results) == 0 {
		return Null, nil
	}
	return results[0], nil
}

func (vm *VM) connectAPIOrganizationSettings(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("ConnectApi.Organization.getSettings expects 0 arguments")
	}
	orgID := "00D000000000001"
	if vm.Org != nil && vm.Org.OrgID != "" {
		orgID = vm.Org.OrgID
	}
	settings := Object("ConnectApi.OrganizationSettings")
	settings.Fields["orgId"] = String(orgID)
	return settings, nil
}

func (vm *VM) callCustomDataStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	objectName, definition, kind, ok := vm.customDataObject(typeName)
	if !ok {
		if (method == "getOrgDefaults" || method == "getValues") && strings.HasSuffix(typeName, "__c") {
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
			}
			return Object(typeName), true, nil
		}
		return Null, false, nil
	}
	switch method {
	case "getAll":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getAll expects 0 arguments", typeName)
		}
		if err := unsupportedHierarchyCustomSettingStatic(definition, typeName, method); err != nil {
			return Null, true, err
		}
		out := Map()
		out.Type = "Map<String," + objectName + ">"
		object := vm.Org.Objects[objectName]
		records := make([]storage.Record, 0, len(object.Records))
		for _, record := range object.Records {
			if record.System.IsDeleted {
				continue
			}
			records = append(records, record)
		}
		sort.Slice(records, func(i, j int) bool {
			return customDataRecordLess(definition, kind, records[i], records[j], vm.Org.Namespace)
		})
		for _, record := range records {
			key := customDataRecordKey(definition, kind, record, vm.Org.Namespace)
			if key == "" {
				continue
			}
			out.Map[mapKey(String(key))] = vm.readOnlyCustomDataValue(record, kind)
		}
		return out, true, nil
	case "getInstance":
		if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
			if len(args) > 1 {
				return Null, true, fmt.Errorf("%s.getInstance expects optional setup owner Id", typeName)
			}
		} else if err := unsupportedHierarchyCustomSettingStatic(definition, typeName, method); err != nil {
			return Null, true, err
		}
		record, found, err := vm.customDataGetInstance(objectName, definition, kind, args)
		if err != nil || !found {
			if err == nil && strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
				return vm.readOnlyCustomDataDefaultValue(objectName, kind), true, nil
			}
			return Null, true, err
		}
		return vm.readOnlyCustomDataValue(record, kind), true, nil
	case "getOrgDefaults", "getValues":
		if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
			switch method {
			case "getOrgDefaults":
				if len(args) != 0 {
					return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
				}
				return vm.hierarchyCustomSettingOrgDefaults(objectName, kind), true, nil
			case "getValues":
				if len(args) > 1 {
					return Null, true, fmt.Errorf("%s.getValues expects optional setup owner Id", typeName)
				}
				if len(args) == 1 && args[0].Kind != ValueString && args[0].Kind != ValueNull {
					return Null, true, fmt.Errorf("%s.getValues expects optional setup owner Id", typeName)
				}
				if len(args) == 1 && args[0].Kind == ValueString {
					if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, args[0].Text); found {
						return vm.readOnlyCustomDataValue(record, kind), true, nil
					}
					return vm.readOnlyCustomDataDefaultValue(objectName, kind), true, nil
				}
				return vm.hierarchyCustomSettingOrgDefaults(objectName, kind), true, nil
			}
		}
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
		}
		record, found := vm.customDataOrgDefaultRecord(objectName)
		if !found {
			return vm.readOnlyCustomDataDefaultValue(objectName, kind), true, nil
		}
		return vm.readOnlyCustomDataValue(record, kind), true, nil
	default:
		return Null, false, nil
	}
}

func (vm *VM) customDataOrgDefaultRecord(objectName string) (storage.Record, bool) {
	if vm.Org == nil {
		return storage.Record{}, false
	}
	object := vm.Org.Objects[objectName]
	records := make([]storage.Record, 0, len(object.Records))
	for _, record := range object.Records {
		if !record.System.IsDeleted {
			records = append(records, record)
		}
	}
	if len(records) == 0 {
		return storage.Record{}, false
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID < records[j].ID
	})
	return records[0], true
}

func unsupportedHierarchyCustomSettingStatic(definition storage.ObjectDefinition, typeName, method string) error {
	if !storage.IsCustomSettingDefinition(definition) {
		return nil
	}
	if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
		return unsupportedCallError(typeName + "." + method + " hierarchy custom setting merge behavior")
	}
	return nil
}

func (vm *VM) hierarchyCustomSettingOrgDefaults(objectName, kind string) Value {
	if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, vm.orgID()); found {
		return vm.readOnlyCustomDataValue(record, kind)
	}
	return vm.readOnlyCustomDataDefaultValue(objectName, kind)
}

func (vm *VM) hierarchyCustomSettingRecordForOwner(objectName, ownerID string) (storage.Record, bool) {
	if vm.Org == nil || ownerID == "" {
		return storage.Record{}, false
	}
	object := vm.Org.Objects[objectName]
	for _, record := range sortedCustomDataRecords(object.Records, object.Definition, "custom setting", vm.Org.Namespace) {
		if record.System.IsDeleted {
			continue
		}
		value, ok := record.Fields["SetupOwnerId"]
		if ok && value.Kind == storage.ValueString && value.String == ownerID {
			return record, true
		}
	}
	return storage.Record{}, false
}

func (vm *VM) orgID() string {
	if vm.Org != nil && vm.Org.OrgID != "" {
		return vm.Org.OrgID
	}
	return "00D000000000001"
}

func (vm *VM) customDataObject(typeName string) (string, storage.ObjectDefinition, string, bool) {
	if vm.Org == nil {
		return "", storage.ObjectDefinition{}, "", false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, typeName)
	if !ok {
		return "", storage.ObjectDefinition{}, "", false
	}
	definition := vm.Org.Objects[objectName].Definition
	switch {
	case storage.IsCustomMetadataDefinition(definition):
		return objectName, definition, "custom metadata", true
	case storage.IsCustomSettingDefinition(definition):
		return objectName, definition, "custom setting", true
	default:
		return "", storage.ObjectDefinition{}, "", false
	}
}

func (vm *VM) customDataGetInstance(objectName string, definition storage.ObjectDefinition, kind string, args []Value) (storage.Record, bool, error) {
	object := vm.Org.Objects[objectName]
	if len(args) == 0 {
		if kind != "custom setting" {
			return storage.Record{}, false, fmt.Errorf("%s.getInstance expects record name", objectName)
		}
		if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
			if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, vm.orgID()); found {
				return record, true, nil
			}
			return storage.Record{}, false, nil
		}
		for _, record := range sortedCustomDataRecords(object.Records, definition, kind, vm.Org.Namespace) {
			if record.System.IsDeleted {
				continue
			}
			return record, true, nil
		}
		return storage.Record{}, false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueString {
		return storage.Record{}, false, fmt.Errorf("%s.getInstance expects optional String name", objectName)
	}
	wanted := args[0].Text
	if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
		if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, wanted); found {
			return record, true, nil
		}
		if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, vm.orgID()); found {
			return record, true, nil
		}
		return storage.Record{}, false, nil
	}
	for _, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		if customDataRecordMatches(definition, kind, record, wanted, vm.Org.Namespace) {
			return record, true, nil
		}
	}
	return storage.Record{}, false, nil
}

func sortedCustomDataRecords(records map[storage.ID]storage.Record, definition storage.ObjectDefinition, kind, namespace string) []storage.Record {
	out := make([]storage.Record, 0, len(records))
	for _, record := range records {
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		return customDataRecordLess(definition, kind, out[i], out[j], namespace)
	})
	return out
}

func customDataRecordLess(definition storage.ObjectDefinition, kind string, left, right storage.Record, namespace string) bool {
	leftKey := customDataRecordKey(definition, kind, left, namespace)
	rightKey := customDataRecordKey(definition, kind, right, namespace)
	if leftKey != rightKey {
		return leftKey < rightKey
	}
	return string(left.ID) < string(right.ID)
}

func customDataRecordMatches(definition storage.ObjectDefinition, kind string, record storage.Record, wanted, namespace string) bool {
	if string(record.ID) == wanted {
		return true
	}
	for _, candidate := range customDataRecordNames(definition, kind, record, namespace) {
		if strings.EqualFold(candidate, wanted) {
			return true
		}
	}
	return false
}

func customDataRecordKey(definition storage.ObjectDefinition, kind string, record storage.Record, namespace string) string {
	names := customDataRecordNames(definition, kind, record, namespace)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func customDataRecordNames(definition storage.ObjectDefinition, kind string, record storage.Record, namespace string) []string {
	fieldOrder := []string{"Name"}
	if kind == "custom metadata" {
		fieldOrder = []string{"DeveloperName", "QualifiedApiName", "Name"}
	}
	var out []string
	for _, field := range fieldOrder {
		if value, ok := record.Fields[field]; ok && value.Kind == storage.ValueString && value.String != "" {
			out = append(out, value.String)
		}
	}
	if kind == "custom metadata" {
		developerName := firstStringField(record, "DeveloperName", "Name")
		prefix := firstStringField(record, "NamespacePrefix")
		if developerName != "" && prefix != "" {
			out = append(out, prefix+"__"+developerName)
		}
		if developerName != "" && prefix == "" && namespace != "" && strings.HasPrefix(definition.APIName, namespace+"__") {
			out = append(out, namespace+"__"+developerName)
		}
	}
	return out
}

func firstStringField(record storage.Record, names ...string) string {
	for _, name := range names {
		value, ok := record.Fields[name]
		if ok && value.Kind == storage.ValueString {
			return value.String
		}
	}
	return ""
}

func (vm *VM) readOnlyCustomDataValue(record storage.Record, kind string) Value {
	value := vmValueFromRecord(record)
	value.Fields[sobjectReadOnlyField] = String(kind + " records returned by getAll/getInstance are read-only")
	return value
}

func (vm *VM) readOnlyCustomDataDefaultValue(objectName, kind string) Value {
	value := Object(objectName)
	if vm.Org != nil {
		if object, ok := vm.Org.Objects[objectName]; ok {
			for name, field := range object.Definition.Fields {
				if defaultValue, ok := storage.DefaultValueForField(field); ok {
					putVMFieldPath(value, name, vmValueFromStorage(defaultValue))
				}
			}
		}
	}
	value.Fields[sobjectReadOnlyField] = String(kind + " records returned by getAll/getInstance are read-only")
	return value
}

func userInfoField(user Value, field, fallback string) string {
	if user.Kind == ValueObject {
		if value, ok := user.Fields[field]; ok && value.Kind == ValueString {
			return value.Text
		}
		return fallback
	}
	if user.Kind == ValueString {
		if field != "Id" && field != "Username" {
			return fallback
		}
		return user.Text
	}
	return fallback
}

func (vm *VM) currentUserInfoField(field, fallback string) string {
	if vm.testContext != nil {
		return userInfoField(vm.testContext.CurrentUser, field, fallback)
	}
	if vm.executionUser.Kind != "" && vm.executionUser.Kind != ValueNull {
		return userInfoField(vm.executionUser, field, fallback)
	}
	return fallback
}

func (vm *VM) currentUserTimeZoneID() string {
	return vm.currentUserInfoField("TimeZoneSidKey", "UTC")
}

func userHasPermission(user Value, permission string) bool {
	if user.Kind != ValueObject {
		return false
	}
	for _, field := range []string{"Permissions", "PermissionSets"} {
		value, ok := user.Fields[field]
		if !ok {
			continue
		}
		if value.Kind == ValueString && strings.EqualFold(value.Text, permission) {
			return true
		}
		if value.Kind == ValueList {
			for _, item := range value.List {
				if item.Kind == ValueString && strings.EqualFold(item.Text, permission) {
					return true
				}
			}
		}
	}
	return false
}

func (vm *VM) shouldEnqueueFuture(method Method) bool {
	if vm.testContext == nil || vm.testContext.Draining {
		return false
	}
	return methodHasModifier(method.Modifiers, "future")
}

func (vm *VM) enqueueFuture(method Method, args []Value, result *Result) (Value, error) {
	if vm.testContext == nil {
		return Null, nil
	}
	if !method.IsStatic {
		return Null, fmt.Errorf("@future method %s must be static", method.Name)
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("futureCalls", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{
		ID:     vm.nextAsyncJobID(),
		Kind:   "Future",
		Method: method,
		Args:   append([]Value(nil), args...),
	}
	vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs, job)
	vm.recordAsyncJob(job, "Queued", "")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":   job.Kind,
		"jobId":  job.ID,
		"method": method.Name,
	})
	return Null, nil
}

func (vm *VM) assertError(message string) error {
	return &RuntimeError{
		Type:    "System.AssertException",
		Message: message,
		Stack:   vm.stackFrames(),
	}
}

func (vm *VM) requireTestContext(callee string) error {
	if vm.testContext == nil {
		return fmt.Errorf("%s is only available in test context", callee)
	}
	return nil
}

func (vm *VM) testSetMock(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.setMock expects mock type and mock instance")
	}
	if err := vm.requireTestContext("Test.setMock"); err != nil {
		return Null, err
	}
	mockType, ok := testMockTypeName(args[0])
	if !ok {
		return Null, fmt.Errorf("Test.setMock expects mock type")
	}
	if mockType != "HttpCalloutMock" {
		return Null, unsupportedCallError("Test.setMock " + mockType + " mock surface")
	}
	vm.testContext.HTTPMock = args[1]
	return Null, nil
}

func (vm *VM) testCreateStub(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.createStub expects Type and StubProvider")
	}
	if err := vm.requireTestContext("Test.createStub"); err != nil {
		return Null, err
	}
	stubbedType, ok := testMockTypeName(args[0])
	if !ok || stubbedType == "" {
		return Null, fmt.Errorf("Test.createStub expects Type")
	}
	if args[1].Kind != ValueObject || !vm.typeMatches(args[1].Type, "StubProvider", make(map[string]bool)) {
		return Null, fmt.Errorf("Test.createStub expects StubProvider")
	}
	if resolved, ok := vm.resolveClassName(stubbedType); ok {
		stubbedType = resolved
	} else {
		return Null, unsupportedCallError("Test.createStub local proxy for unknown type " + stubbedType)
	}
	proxy := Object(stubbedType)
	proxy.Fields["__oaerStubProvider"] = args[1]
	proxy.Fields["__oaerStubbedType"] = String(stubbedType)
	return proxy, nil
}

func testMockTypeName(value Value) (string, bool) {
	switch value.Kind {
	case ValueString:
		if value.Text == "" {
			return "", false
		}
		return value.Text, true
	case ValueObject:
		if value.Type == "Type" && value.Text != "" {
			return value.Text, true
		}
	}
	return "", false
}

func (vm *VM) testStart() (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.startTest is only available in test context")
	}
	if vm.testContext.Started {
		return Null, fmt.Errorf("Test.startTest cannot be called more than once")
	}
	vm.testContext.Started = true
	vm.testContext.Stopped = false
	vm.testContext.AsyncJobs = nil
	vm.testContext.ParentLimits = vm.limits
	vm.testContext.ParentViolations = append([]LimitViolation(nil), vm.limitViolations...)
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
	err := vm.drainTestAsync(result)
	vm.limits = vm.testContext.ParentLimits
	vm.limitViolations = append([]LimitViolation(nil), vm.testContext.ParentViolations...)
	return Null, err
}

func (vm *VM) enqueueJob(args []Value, result *Result) (Value, error) {
	if len(args) == 2 {
		return Null, unsupportedCallError("System.enqueueJob AsyncOptions overload")
	}
	if len(args) != 1 {
		return Null, fmt.Errorf("System.enqueueJob expects 1 argument")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("System.enqueueJob expects Queueable object")
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("queueableJobs", 1); err != nil {
		return Null, err
	}
	draining, chainEnqueued := vm.asyncDrainState()
	if draining && chainEnqueued {
		return Null, fmt.Errorf("Queueable chaining limit exceeded")
	}
	vm.markAsyncChainEnqueued()
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "Queueable", Object: args[0]}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":  job.Kind,
		"jobId": job.ID,
		"class": args[0].Type,
	})
	return String(job.ID), nil
}

func (vm *VM) executeBatch(args []Value, result *Result) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("Database.executeBatch expects batch instance[, scopeSize]")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("Database.executeBatch expects Batchable object")
	}
	batchSize := 200
	if len(args) == 2 {
		if args[1].Kind != ValueInt {
			return Null, fmt.Errorf("Database.executeBatch scope size expects Integer")
		}
		batchSize = int(args[1].Int)
		if batchSize <= 0 {
			return Null, fmt.Errorf("Database.executeBatch scope size must be positive")
		}
		if batchSize > 2000 {
			return Null, fmt.Errorf("Database.executeBatch scope size must be at most 2000")
		}
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("batchJobs", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "BatchApex", Object: args[0], BatchSize: batchSize}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":      job.Kind,
		"jobId":     job.ID,
		"class":     args[0].Type,
		"batchSize": batchSize,
	})
	return String(job.ID), nil
}

func (vm *VM) scheduleJob(args []Value, result *Result) (Value, error) {
	if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueObject {
		return Null, fmt.Errorf("System.schedule expects name, cron, and Schedulable object")
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("scheduledJobs", 1); err != nil {
		return Null, err
	}
	job := AsyncJob{ID: vm.nextAsyncJobID(), Kind: "ScheduledApex", Object: args[2], Name: args[0].Text, Cron: args[1].Text}
	vm.enqueueAsyncJob(job)
	vm.recordAsyncJob(job, "Queued", "")
	vm.recordCronTrigger(job, "Waiting")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":  job.Kind,
		"jobId": job.ID,
		"class": args[2].Type,
		"name":  job.Name,
	})
	return String(cronTriggerID(job.ID)), nil
}

func (vm *VM) abortJob(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("System.abortJob expects job Id")
	}
	if args[0].Kind != ValueString {
		return Null, fmt.Errorf("System.abortJob expects String job Id")
	}
	if vm.testContext == nil {
		return Null, unsupportedCallError("System.abortJob local async scheduling surface")
	}
	jobID := args[0].Text
	for i, job := range vm.testContext.AsyncJobs {
		if job.ID != jobID && cronTriggerID(job.ID) != jobID {
			continue
		}
		vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs[:i], vm.testContext.AsyncJobs[i+1:]...)
		vm.recordAsyncJob(job, "Aborted", "")
		if job.Kind == "ScheduledApex" {
			vm.recordCronTrigger(job, "Deleted")
		}
		return Null, nil
	}
	if vm.asyncJobRecordStatus(jobID) != "" {
		return Null, unsupportedCallError("System.abortJob completed local async records")
	}
	return Null, unsupportedCallError("System.abortJob unknown local async records")
}

func (vm *VM) asyncJobRecordStatus(jobID string) string {
	if vm.Org == nil {
		return ""
	}
	vm.ensureAsyncObjects()
	if strings.HasPrefix(jobID, "08e") {
		jobID = strings.Replace(jobID, "08e", "707", 1)
	}
	object := vm.Org.Objects["AsyncApexJob"]
	record, ok := object.Records[storage.ID(jobID)]
	if !ok {
		return ""
	}
	if status, ok := record.Fields["Status"]; ok && status.Kind == storage.ValueString {
		return status.String
	}
	return ""
}

func (vm *VM) drainTestAsync(result *Result) error {
	if vm.testContext == nil {
		return nil
	}
	return vm.drainAsyncJobs(result, &vm.testContext.AsyncJobs, &vm.testContext.Draining, &vm.testContext.ChainEnqueued)
}

func (vm *VM) drainLocalAsync(result *Result) error {
	return vm.drainAsyncJobs(result, &vm.localAsyncJobs, &vm.localAsyncDrain, &vm.localAsyncChain)
}

func (vm *VM) drainAsyncJobs(result *Result, jobs *[]AsyncJob, draining *bool, chainEnqueued *bool) error {
	if *draining {
		return nil
	}
	*draining = true
	defer func() {
		*draining = false
	}()
	for len(*jobs) > 0 {
		job := (*jobs)[0]
		*jobs = (*jobs)[1:]
		if err := vm.ResetStatics(); err != nil {
			return err
		}
		*chainEnqueued = false
		vm.recordAsyncJob(job, "Processing", "")
		appendTrace(result, "apex.async.run", "apex.async", map[string]any{
			"kind":  job.Kind,
			"jobId": job.ID,
		})
		if err := vm.runAsyncJob(job, result); err != nil {
			vm.recordAsyncJob(job, "Failed", err.Error())
			return err
		}
		vm.recordAsyncJob(job, "Completed", "")
	}
	return nil
}

func (vm *VM) asyncDrainState() (bool, bool) {
	if vm.testContext != nil {
		return vm.testContext.Draining, vm.testContext.ChainEnqueued
	}
	return vm.localAsyncDrain, vm.localAsyncChain
}

func (vm *VM) markAsyncChainEnqueued() {
	if vm.testContext != nil {
		if vm.testContext.Draining {
			vm.testContext.ChainEnqueued = true
		}
		return
	}
	if vm.localAsyncDrain {
		vm.localAsyncChain = true
	}
}

func (vm *VM) enqueueAsyncJob(job AsyncJob) {
	if vm.testContext != nil {
		vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs, job)
		return
	}
	vm.localAsyncJobs = append(vm.localAsyncJobs, job)
}

func (vm *VM) runAsyncJob(job AsyncJob, result *Result) error {
	switch job.Kind {
	case "Future":
		_, err := vm.withAsyncKind("Future", func() (Value, error) {
			return vm.callMethod(job.Method, job.Args, result)
		})
		return err
	case "Queueable":
		target, ok := vm.resolveInstanceMethod(job.Object.Type, "execute")
		if !ok {
			return fmt.Errorf("async job %s has no execute method", job.Object.Type)
		}
		args := []Value{asyncContext("QueueableContext", job.ID)}
		if len(target.Params) == 0 {
			args = nil
		}
		_, err := vm.withAsyncKind("Queueable", func() (Value, error) {
			return vm.callMethodWithReceiver(target, job.Object, args, result)
		})
		return err
	case "BatchApex":
		_, err := vm.withAsyncKind("BatchApex", func() (Value, error) {
			return Null, vm.runBatchJob(job, result)
		})
		return err
	case "ScheduledApex":
		target, ok := vm.resolveInstanceMethod(job.Object.Type, "execute")
		if !ok {
			return fmt.Errorf("scheduled job %s has no execute method", job.Object.Type)
		}
		args := []Value{schedulableContext(job.ID)}
		if len(target.Params) == 0 {
			args = nil
		}
		_, err := vm.withAsyncKind("ScheduledApex", func() (Value, error) {
			return vm.callMethodWithReceiver(target, job.Object, args, result)
		})
		vm.recordCronTrigger(job, "Complete")
		return err
	default:
		return fmt.Errorf("unsupported async job kind %s", job.Kind)
	}
}

func (vm *VM) withAsyncKind(kind string, run func() (Value, error)) (Value, error) {
	previous := vm.currentAsyncKind
	vm.currentAsyncKind = kind
	defer func() {
		vm.currentAsyncKind = previous
	}()
	return run()
}

func (vm *VM) isAsyncKind(callee string) bool {
	switch callee {
	case "System.isBatch":
		return vm.currentAsyncKind == "BatchApex"
	case "System.isFuture":
		return vm.currentAsyncKind == "Future"
	case "System.isQueueable":
		return vm.currentAsyncKind == "Queueable"
	case "System.isScheduled":
		return vm.currentAsyncKind == "ScheduledApex"
	default:
		return false
	}
}

func (vm *VM) runBatchJob(job AsyncJob, result *Result) error {
	var scope []Value
	if start, ok := vm.resolveInstanceMethod(job.Object.Type, "start"); ok {
		value, err := vm.callMethodWithReceiver(start, job.Object, batchArgs(start, "Database.BatchableContext", job.ID), result)
		if err != nil {
			return err
		}
		if value.Kind == ValueList {
			scope = append(scope, value.List...)
		}
		if value.Kind == ValueObject && value.Type == "Database.QueryLocator" {
			if records, ok := value.Fields["Records"]; ok && records.Kind == ValueList {
				scope = append(scope, records.List...)
			}
		}
	}
	if execute, ok := vm.resolveInstanceMethod(job.Object.Type, "execute"); ok {
		chunks := batchChunks(scope, job.BatchSize)
		vm.recordAsyncJobTotals(job, len(chunks), 0, 0)
		for _, chunk := range chunks {
			if _, err := vm.callMethodWithReceiver(execute, job.Object, batchExecuteArgs(execute, chunk, job.ID), result); err != nil {
				return err
			}
		}
	}
	if finish, ok := vm.resolveInstanceMethod(job.Object.Type, "finish"); ok {
		if _, err := vm.callMethodWithReceiver(finish, job.Object, batchArgs(finish, "Database.BatchableContext", job.ID), result); err != nil {
			return err
		}
	}
	return nil
}

func batchChunks(values []Value, size int) [][]Value {
	if size <= 0 {
		size = 200
	}
	if len(values) == 0 {
		return [][]Value{{}}
	}
	var chunks [][]Value
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

func batchArgs(method Method, contextType, jobID string) []Value {
	if len(method.Params) == 0 {
		return nil
	}
	return []Value{asyncContext(contextType, jobID)}
}

func batchExecuteArgs(method Method, scope []Value, jobID string) []Value {
	switch len(method.Params) {
	case 0:
		return nil
	case 1:
		return []Value{List(scope...)}
	default:
		return []Value{asyncContext("Database.BatchableContext", jobID), List(scope...)}
	}
}

func asyncContext(typeName, jobID string) Value {
	ctx := Object(typeName)
	if jobID != "" {
		ctx.Fields["JobId"] = String(jobID)
	}
	return ctx
}

func schedulableContext(jobID string) Value {
	ctx := Object("SchedulableContext")
	if jobID != "" {
		ctx.Fields["TriggerId"] = String(cronTriggerID(jobID))
	}
	return ctx
}

func cronTriggerID(jobID string) string {
	return strings.Replace(jobID, "707", "08e", 1)
}

func (vm *VM) nextAsyncJobID() string {
	if vm.testContext != nil {
		vm.testContext.JobSeq++
		return fmt.Sprintf("707%012d", vm.testContext.JobSeq)
	}
	vm.localAsyncSeq++
	return fmt.Sprintf("707%012d", vm.localAsyncSeq)
}

func (vm *VM) ensureAsyncObjects() {
	if vm.Org == nil {
		return
	}
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "AsyncApexJob",
		Label:     "Async Apex Job",
		KeyPrefix: "707",
		Fields: map[string]storage.Field{
			"Id":                {APIName: "Id", Type: storage.FieldID},
			"Status":            {APIName: "Status", Type: storage.FieldString},
			"JobType":           {APIName: "JobType", Type: storage.FieldString},
			"ApexClassName":     {APIName: "ApexClassName", Type: storage.FieldString},
			"MethodName":        {APIName: "MethodName", Type: storage.FieldString},
			"TotalJobItems":     {APIName: "TotalJobItems", Type: storage.FieldInteger},
			"JobItemsProcessed": {APIName: "JobItemsProcessed", Type: storage.FieldInteger},
			"NumberOfErrors":    {APIName: "NumberOfErrors", Type: storage.FieldInteger},
			"ExtendedStatus":    {APIName: "ExtendedStatus", Type: storage.FieldString},
		},
	})
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "CronTrigger",
		Label:     "Cron Trigger",
		KeyPrefix: "08e",
		Fields: map[string]storage.Field{
			"Id":             {APIName: "Id", Type: storage.FieldID},
			"State":          {APIName: "State", Type: storage.FieldString},
			"CronExpression": {APIName: "CronExpression", Type: storage.FieldString},
			"CronJobDetail":  {APIName: "CronJobDetail", Type: storage.FieldString},
		},
	})
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "User",
		Label:     "User",
		KeyPrefix: "005",
		Fields: map[string]storage.Field{
			"Id":        {APIName: "Id", Type: storage.FieldID},
			"Username":  {APIName: "Username", Type: storage.FieldString},
			"ProfileId": {APIName: "ProfileId", Type: storage.FieldString},
		},
	})
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "Profile",
		Label:     "Profile",
		KeyPrefix: "00e",
		Fields: map[string]storage.Field{
			"Id":   {APIName: "Id", Type: storage.FieldID},
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	})
}

func ensureObject(org *storage.OrgState, definition storage.ObjectDefinition) {
	if org.Objects == nil {
		org.Objects = make(map[string]storage.ObjectState)
	}
	if existing, ok := org.Objects[definition.APIName]; ok {
		if existing.Records == nil {
			existing.Records = make(map[storage.ID]storage.Record)
		}
		if existing.Definition.Fields == nil {
			existing.Definition.Fields = definition.Fields
		}
		org.Objects[definition.APIName] = existing
		return
	}
	org.Objects[definition.APIName] = storage.ObjectState{Definition: definition, Records: make(map[storage.ID]storage.Record)}
}

func (vm *VM) recordAsyncJob(job AsyncJob, status, detail string) {
	if vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["AsyncApexJob"]
	record := object.Records[storage.ID(job.ID)]
	if record.ID == "" {
		record = storage.Record{ID: storage.ID(job.ID), Object: "AsyncApexJob", Fields: make(map[string]storage.Value)}
	}
	record.Fields["Status"] = storage.StringValue(status)
	record.Fields["JobType"] = storage.StringValue(job.Kind)
	record.Fields["ApexClassName"] = storage.StringValue(asyncClassName(job))
	record.Fields["MethodName"] = storage.StringValue(asyncMethodName(job))
	if existing, ok := record.Fields["TotalJobItems"]; ok && existing.Kind == storage.ValueInteger && existing.Integer > 0 && job.Kind == "BatchApex" {
		record.Fields["TotalJobItems"] = existing
	} else {
		record.Fields["TotalJobItems"] = storage.IntegerValue(int64(asyncTotalItems(job)))
	}
	if status == "Completed" {
		record.Fields["JobItemsProcessed"] = record.Fields["TotalJobItems"]
		record.Fields["NumberOfErrors"] = storage.IntegerValue(0)
	} else if status == "Failed" {
		record.Fields["NumberOfErrors"] = storage.IntegerValue(1)
		record.Fields["ExtendedStatus"] = storage.StringValue(detail)
	}
	object.Records[record.ID] = record
	vm.Org.Objects["AsyncApexJob"] = object
}

func (vm *VM) recordAsyncJobTotals(job AsyncJob, total, processed, errors int) {
	if vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["AsyncApexJob"]
	record := object.Records[storage.ID(job.ID)]
	if record.ID == "" {
		record = storage.Record{ID: storage.ID(job.ID), Object: "AsyncApexJob", Fields: make(map[string]storage.Value)}
	}
	record.Fields["TotalJobItems"] = storage.IntegerValue(int64(total))
	record.Fields["JobItemsProcessed"] = storage.IntegerValue(int64(processed))
	record.Fields["NumberOfErrors"] = storage.IntegerValue(int64(errors))
	object.Records[record.ID] = record
	vm.Org.Objects["AsyncApexJob"] = object
}

func (vm *VM) recordCronTrigger(job AsyncJob, state string) {
	if vm.Org == nil {
		return
	}
	vm.ensureAsyncObjects()
	object := vm.Org.Objects["CronTrigger"]
	id := storage.ID(cronTriggerID(job.ID))
	record := object.Records[id]
	if record.ID == "" {
		record = storage.Record{ID: id, Object: "CronTrigger", Fields: make(map[string]storage.Value)}
	}
	record.Fields["State"] = storage.StringValue(state)
	record.Fields["CronExpression"] = storage.StringValue(job.Cron)
	record.Fields["CronJobDetail"] = storage.StringValue(job.Name)
	object.Records[record.ID] = record
	vm.Org.Objects["CronTrigger"] = object
}

func asyncClassName(job AsyncJob) string {
	if job.Method.ClassName != "" {
		return job.Method.ClassName
	}
	return job.Object.Type
}

func asyncMethodName(job AsyncJob) string {
	if job.Method.Name == "" {
		return "execute"
	}
	name := job.Method.Name
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

func asyncTotalItems(job AsyncJob) int {
	if job.Kind != "BatchApex" || job.BatchSize <= 0 {
		return 1
	}
	return 1
}

func (vm *VM) executeSOQL(raw string, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRows(raw, execResult)
	if err != nil {
		return Null, err
	}
	return List(values...), nil
}

func (vm *VM) executeSOQLWithBindMap(raw string, binds Value, execResult *Result) (Value, error) {
	values, err := vm.executeSOQLRowsWithExpander(raw, execResult, func(query string) (string, error) {
		return vm.expandSOQLBindsFromMap(query, binds)
	})
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
		if strings.HasPrefix(typeName, "List<") {
			value.Type = typeName
		}
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
	return vm.executeSOQLRowsWithExpander(raw, execResult, vm.expandSOQLBinds)
}

func (vm *VM) executeSOQLRowsWithExpander(raw string, execResult *Result, expand func(string) (string, error)) ([]Value, error) {
	if soql.IsSOSLFind(raw) {
		return nil, unsupportedCallError("SOSL/FIND local search surface")
	}
	if vm.Org == nil {
		return nil, fmt.Errorf("SOQL requires org state")
	}
	if err := vm.incrementLimit("queries", 1); err != nil {
		return nil, err
	}
	queryText, err := expand(raw)
	if err != nil {
		return nil, newExceptionError("QueryException", fmt.Sprintf("%s in query %q", err.Error(), raw))
	}
	if soql.IsSOSLFind(queryText) {
		return nil, unsupportedCallError("SOSL/FIND local search surface")
	}
	result, err := soql.ParseAndExecuteAt(*vm.Org, queryText, vm.fakeNow)
	if err != nil {
		var unsupported *soql.UnsupportedFeatureError
		if errors.As(err, &unsupported) {
			return nil, &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
		}
		return nil, newExceptionError("QueryException", err.Error())
	}
	limitRows := soqlLimitRows(result)
	if err := vm.incrementLimit("queryRows", limitRows); err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("cpuTime", limitRows); err != nil {
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

func soqlLimitRows(result soql.Result) int {
	rows := len(result.Records)
	for _, record := range result.Records {
		for _, children := range record.Children {
			rows += len(children)
		}
	}
	return rows
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
	return vm.expandSOQLBindsWith(raw, vm.lookup)
}

func (vm *VM) expandSOQLBindsFromMap(raw string, binds Value) (string, error) {
	if binds.Kind != ValueMap {
		return "", fmt.Errorf("queryWithBinds bind values must be a Map")
	}
	return vm.expandSOQLBindsWith(raw, func(name string) (Value, error) {
		if strings.Contains(name, ".") {
			return Null, fmt.Errorf("queryWithBinds does not support dotted bind path %q", name)
		}
		value, ok := binds.Map[mapKey(String(name))]
		if !ok {
			return Null, fmt.Errorf("missing bind value %q", name)
		}
		return value, nil
	})
}

func (vm *VM) expandSOQLBindsWith(raw string, lookup func(string) (Value, error)) (string, error) {
	var out strings.Builder
	for i := 0; i < len(raw); {
		if raw[i] == '\'' {
			out.WriteByte(raw[i])
			i++
			for i < len(raw) {
				out.WriteByte(raw[i])
				if raw[i] == '\'' {
					if i+1 < len(raw) && raw[i+1] == '\'' {
						i++
						out.WriteByte(raw[i])
						i++
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}
		if raw[i] != ':' {
			out.WriteByte(raw[i])
			i++
			continue
		}
		valueStart := i + 1
		for valueStart < len(raw) && (raw[valueStart] == ' ' || raw[valueStart] == '\t' || raw[valueStart] == '\n' || raw[valueStart] == '\r') {
			valueStart++
		}
		if isSOQLDateLiteralBind(raw, i) {
			trimmed := strings.TrimRight(out.String(), " \t\n\r")
			if len(trimmed) != out.Len() {
				out.Reset()
				out.WriteString(trimmed)
			}
			out.WriteByte(':')
			i++
			if valueStart < len(raw) && valueStart != i && raw[valueStart] >= '0' && raw[valueStart] <= '9' {
				for valueStart < len(raw) && raw[valueStart] >= '0' && raw[valueStart] <= '9' {
					out.WriteByte(raw[valueStart])
					valueStart++
				}
				i = valueStart
			}
			continue
		}
		if valueStart >= len(raw) || !isIdentStart(raw[valueStart]) {
			out.WriteByte(raw[i])
			i++
			continue
		}
		nameStart := valueStart
		j := nameStart
		var name strings.Builder
		for j < len(raw) {
			if isIdentPart(raw[j]) {
				name.WriteByte(raw[j])
				j++
				continue
			}
			dot := j
			for dot < len(raw) && (raw[dot] == ' ' || raw[dot] == '\t' || raw[dot] == '\n' || raw[dot] == '\r') {
				dot++
			}
			if dot < len(raw) && raw[dot] == '.' {
				next := dot + 1
				for next < len(raw) && (raw[next] == ' ' || raw[next] == '\t' || raw[next] == '\n' || raw[next] == '\r') {
					next++
				}
				if next < len(raw) && isIdentStart(raw[next]) {
					name.WriteByte('.')
					j = next
					name.WriteByte(raw[j])
					j++
					for j < len(raw) && isIdentPart(raw[j]) {
						name.WriteByte(raw[j])
						j++
					}
					continue
				}
			}
			if raw[j] == '.' && j+1 < len(raw) && isIdentStart(raw[j+1]) {
				name.WriteByte('.')
				name.WriteByte(raw[j+1])
				j += 2
				for j < len(raw) && isIdentPart(raw[j]) {
					name.WriteByte(raw[j])
					j++
				}
				continue
			}
			break
		}
		value, err := lookup(name.String())
		if err != nil {
			return "", err
		}
		out.WriteString(soqlLiteral(value))
		if callEnd, ok := consumeEmptyCallSuffix(raw, j); ok {
			i = callEnd
		} else {
			i = j
		}
	}
	return out.String(), nil
}

func consumeEmptyCallSuffix(raw string, index int) (int, bool) {
	j := index
	for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
		j++
	}
	if j >= len(raw) || raw[j] != '(' {
		return index, false
	}
	j++
	for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
		j++
	}
	if j >= len(raw) || raw[j] != ')' {
		return index, false
	}
	return j + 1, true
}

func isSOQLDateLiteralBind(raw string, colon int) bool {
	start := colon - 1
	for start >= 0 && (raw[start] == ' ' || raw[start] == '\t' || raw[start] == '\n' || raw[start] == '\r') {
		start--
	}
	end := start + 1
	for start >= 0 && (raw[start] == '_' || raw[start] >= 'A' && raw[start] <= 'Z' || raw[start] >= 'a' && raw[start] <= 'z') {
		start--
	}
	prefix := strings.ToUpper(raw[start+1 : end])
	switch prefix {
	case "LAST_N_DAYS", "NEXT_N_DAYS", "N_DAYS_AGO", "LAST_N_WEEKS", "NEXT_N_WEEKS", "LAST_N_MONTHS", "NEXT_N_MONTHS", "LAST_N_YEARS", "NEXT_N_YEARS":
		return true
	default:
		return false
	}
}

func (vm *VM) executeDML(op string, expr ir.Expr, externalIDField string, result *Result) error {
	if op == "merge" {
		if expr.Kind != ir.ExprCall || len(expr.Args) < 2 {
			return fmt.Errorf("merge statement requires master and duplicate record(s)")
		}
		args := make([]Value, 0, len(expr.Args))
		for _, arg := range expr.Args {
			value, err := vm.eval(arg, result)
			if err != nil {
				return err
			}
			args = append(args, value)
		}
		value, err := vm.executeDatabaseMerge(args, result)
		if err != nil {
			return err
		}
		results := []Value{value}
		if value.Kind == ValueList {
			results = value.List
		}
		for _, mergeResult := range results {
			if mergeResult.Kind != ValueObject {
				continue
			}
			success, ok := mergeResult.Fields["success"]
			if ok && success.Kind == ValueBool && success.Bool {
				continue
			}
			if errValue, ok := mergeResult.Fields["error"]; ok && errValue.Kind == ValueString && errValue.Text != "" {
				return errors.New(errValue.Text)
			}
			return errors.New("merge failed")
		}
		return nil
	}
	value, err := vm.eval(expr, result)
	if err != nil {
		return err
	}
	results, err := vm.applyDML(op, value, true, externalIDField, result)
	if err != nil {
		return err
	}
	for _, dmlResult := range results {
		if !dmlResult.Success {
			return errors.New(dmlResult.Error)
		}
	}
	if expr.Kind == ir.ExprVariable {
		vm.populateDMLResultFields(&value, results)
		if err := vm.assign(expr.Name, value); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) executeDatabaseDML(op string, args []Value, result *Result) (Value, error) {
	if len(args) == 0 || len(args) > 3 {
		return Null, fmt.Errorf("Database.%s expects records, optional external id field, and optional allOrNone", op)
	}
	if err := unsupportedDatabaseDMLOverload(op, args); err != nil {
		return Null, err
	}
	allOrNone := true
	externalIDField := ""
	if len(args) >= 2 {
		if args[1].Kind == ValueBool {
			allOrNone = args[1].Bool
		} else if op == "upsert" {
			field, err := vm.externalIDFieldName(args[1])
			if err != nil {
				return Null, err
			}
			externalIDField = field
		} else {
			return Null, fmt.Errorf("Database.%s allOrNone expects Boolean", op)
		}
	}
	if len(args) == 3 {
		if op != "upsert" {
			return Null, fmt.Errorf("Database.%s expects at most records and allOrNone", op)
		}
		if args[2].Kind != ValueBool {
			return Null, fmt.Errorf("Database.%s allOrNone expects Boolean", op)
		}
		allOrNone = args[2].Bool
	}
	if op == "delete" {
		records, ok := vm.deleteIDsToSObjects(args[0])
		if ok {
			args[0] = records
		}
	}
	results, err := vm.applyDML(op, args[0], allOrNone, externalIDField, result)
	if err != nil {
		return Null, err
	}
	if allOrNone && hasDMLFailures(results) {
		return Null, databaseDMLException(op, results)
	}
	values := make([]Value, 0, len(results))
	for _, dmlResult := range results {
		resultType := databaseDMLResultType(op)
		row := Object(resultType)
		row.Fields["success"] = Bool(dmlResult.Success)
		row.Fields["id"] = databaseResultIDValue(dmlResult.ID)
		row.Fields["error"] = String(dmlResult.Error)
		if op == "upsert" {
			row.Fields["created"] = Bool(dmlResult.Created)
		}
		row.Fields["errors"] = databaseErrorsList(dmlResult)
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

func (vm *VM) deleteIDsToSObjects(value Value) (Value, bool) {
	switch value.Kind {
	case ValueString:
		record, ok := vm.deleteIDToSObject(value.Text)
		return record, ok
	case ValueObject:
		if strings.EqualFold(value.Type, "Id") {
			id, err := platformScalarText(value, "Id")
			if err != nil {
				return value, false
			}
			record, ok := vm.deleteIDToSObject(id)
			return record, ok
		}
	case ValueList:
		if len(value.List) == 0 {
			return value, false
		}
		out := List()
		out.Type = "List<sObject>"
		for _, item := range value.List {
			record, ok := vm.deleteIDsToSObjects(item)
			if !ok || record.Kind != ValueObject {
				return value, false
			}
			out.List = append(out.List, record)
		}
		return out, true
	}
	return value, false
}

func (vm *VM) deleteIDToSObject(id string) (Value, bool) {
	if len(id) < 3 {
		return Null, false
	}
	objectName, ok := vm.sObjectNameForIDPrefix(id[:3])
	if !ok {
		return Null, false
	}
	record := Object(objectName)
	record.Fields["Id"] = platformScalar("Id", id)
	return record, true
}

func (vm *VM) executeDatabaseRecordAction(op string, args []Value, result *Result) (Value, error) {
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
	results, err := vm.applyDatabaseRecordAction(op, args[0], allOrNone, result)
	if err != nil {
		return Null, err
	}
	if allOrNone && hasDMLFailures(results) {
		return Null, databaseDMLException(op, results)
	}
	values := make([]Value, 0, len(results))
	for _, dmlResult := range results {
		row := Object(databaseRecordActionResultType(op))
		row.Fields["success"] = Bool(dmlResult.Success)
		row.Fields["id"] = databaseResultIDValue(dmlResult.ID)
		row.Fields["error"] = String(dmlResult.Error)
		row.Fields["errors"] = databaseErrorsList(dmlResult)
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

func (vm *VM) applyDatabaseRecordAction(op string, value Value, allOrNone bool, result *Result) ([]dml.Result, error) {
	if vm.Org == nil {
		return nil, fmt.Errorf("DML requires org state")
	}
	records, _, err := vm.recordsFromValue(value)
	if err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("dmlStatements", 1); err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("dmlRows", len(records)); err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("cpuTime", len(records)); err != nil {
		return nil, err
	}
	if err := vm.checkMixedDML(records); err != nil {
		return nil, err
	}
	if result != nil {
		result.Trace = append(result.Trace, trace.Instant("apex.dml."+op, "apex.dml", int64(len(result.Trace)), map[string]any{
			"operation": op,
			"rows":      len(records),
		}))
	}
	backup := vm.Org.Clone()
	engine := vm.newDMLEngine()
	var results []dml.Result
	switch op {
	case "emptyRecycleBin":
		results = engine.EmptyRecycleBin(records)
	case "lock":
		results = engine.Lock(records)
	case "unlock":
		results = engine.Unlock(records)
	default:
		return nil, fmt.Errorf("unsupported Database.%s operation", op)
	}
	if allOrNone && hasDMLFailures(results) {
		*vm.Org = backup
	}
	return results, nil
}

func unsupportedDatabaseDMLOverload(op string, args []Value) error {
	for _, arg := range args[1:] {
		if arg.Kind != ValueObject {
			continue
		}
		switch arg.Type {
		case "AccessLevel":
			return unsupportedCallError("Database." + op + " AccessLevel overload")
		case "Database.DMLOptions", "DMLOptions":
			return unsupportedCallError("Database." + op + " DMLOptions overload")
		}
	}
	return nil
}

func databaseDMLResultType(op string) string {
	switch op {
	case "delete":
		return "Database.DeleteResult"
	case "undelete":
		return "Database.UndeleteResult"
	case "upsert":
		return "Database.UpsertResult"
	default:
		return "Database.SaveResult"
	}
}

func databaseRecordActionResultType(op string) string {
	switch op {
	case "emptyRecycleBin":
		return "Database.EmptyRecycleBinResult"
	case "lock":
		return "Database.LockResult"
	case "unlock":
		return "Database.UnlockResult"
	default:
		return "Database.SaveResult"
	}
}

func databaseResultIDValue(id storage.ID) Value {
	if id == "" {
		return Null
	}
	return String(string(id))
}

func databaseDMLException(op string, results []dml.Result) error {
	message := "DML operation failed"
	if op != "" {
		message = "Database." + op + " failed"
	}
	for _, result := range results {
		if !result.Success && result.Error != "" {
			message += ": " + result.Error
			break
		}
	}
	value := Object("DmlException")
	value.Fields["message"] = String(message)
	value.Fields["__dmlErrors"] = dmlExceptionErrorDetails(results)
	return &apexThrowError{value: value}
}

func dmlExceptionErrorDetails(results []dml.Result) Value {
	details := List()
	for index, result := range results {
		if result.Success || result.Error == "" {
			continue
		}
		for _, err := range dmlResultErrors(result) {
			detail := databaseErrorValue(err)
			detail.Fields["id"] = databaseResultIDValue(result.ID)
			detail.Fields["index"] = Int(int64(index))
			details.List = append(details.List, detail)
		}
	}
	return details
}

func (vm *VM) executeDatabaseMerge(args []Value, result *Result) (Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return Null, fmt.Errorf("Database.merge expects master, duplicate record(s), and optional allOrNone")
	}
	allOrNone := true
	if len(args) == 3 {
		if args[2].Kind != ValueBool {
			return Null, fmt.Errorf("Database.merge allOrNone expects Boolean")
		}
		allOrNone = args[2].Bool
	}
	if vm.Org == nil {
		return Null, fmt.Errorf("DML requires org state")
	}
	master, _, err := vm.recordsFromValue(args[0])
	if err != nil {
		return Null, err
	}
	if len(master) != 1 {
		return Null, fmt.Errorf("Database.merge master expects one sObject")
	}
	duplicates, _, err := vm.recordsFromValue(args[1])
	if err != nil {
		return Null, err
	}
	recordsForChecks := append([]storage.Record{master[0]}, duplicates...)
	if err := vm.incrementLimit("dmlStatements", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("dmlRows", len(recordsForChecks)); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("cpuTime", len(recordsForChecks)); err != nil {
		return Null, err
	}
	if err := vm.checkMixedDML(recordsForChecks); err != nil {
		return Null, err
	}
	if result != nil {
		result.Trace = append(result.Trace, trace.Instant("apex.dml.merge", "apex.dml", int64(len(result.Trace)), map[string]any{
			"operation": "merge",
			"rows":      len(recordsForChecks),
		}))
	}
	backup := vm.Org.Clone()
	masterBefore, err := vm.oldRecords("update", master)
	if err != nil {
		return Null, err
	}
	duplicateBefore, err := vm.oldRecords("delete", duplicates)
	if err != nil {
		return Null, err
	}
	if beforeUpdateFailures, err := vm.runTriggers(triggerTimingBefore, "update", master, masterBefore, result); err != nil {
		*vm.Org = backup
		return Null, err
	} else if hasDMLFailures(beforeUpdateFailures) {
		*vm.Org = backup
		results := make([]dml.Result, len(duplicates))
		failure := beforeUpdateFailures[0]
		for i := range results {
			results[i] = failure
		}
		if allOrNone {
			return Null, databaseDMLException("merge", results)
		}
		return vm.mergeResultValue(args[1].Kind == ValueList, duplicates, results), nil
	}
	beforeDeleteFailures, err := vm.runTriggers(triggerTimingBefore, "delete", duplicates, duplicateBefore, result)
	if err != nil {
		*vm.Org = backup
		return Null, err
	}
	mergeDuplicates := duplicates
	mergeDuplicateBefore := duplicateBefore
	if hasDMLFailures(beforeDeleteFailures) {
		if allOrNone {
			*vm.Org = backup
			return Null, databaseDMLException("merge", beforeDeleteFailures)
		}
		mergeDuplicates, mergeDuplicateBefore, _ = filterDMLInputs(duplicates, duplicateBefore, nil, beforeDeleteFailures)
		if len(mergeDuplicates) == 0 {
			return vm.mergeResultValue(args[1].Kind == ValueList, duplicates, beforeDeleteFailures), nil
		}
	}
	engine := vm.newDMLEngine()
	results := engine.Merge(master[0], mergeDuplicates)
	if hasDMLFailures(beforeDeleteFailures) {
		results = mergeDMLResults(beforeDeleteFailures, results)
	}
	engineRolledBack := false
	if allOrNone {
		for _, dmlResult := range results {
			if !dmlResult.Success {
				*vm.Org = backup
				engineRolledBack = true
				break
			}
		}
	}
	if allOrNone && hasDMLFailures(results) {
		return Null, databaseDMLException("merge", results)
	}
	successfulDuplicates := make([]storage.Record, 0, len(duplicates))
	successfulDuplicateBefore := make([]storage.Record, 0, len(mergeDuplicateBefore))
	if !engineRolledBack {
		successIndex := 0
		for i, dmlResult := range results {
			if !dmlResult.Success {
				continue
			}
			if i < len(beforeDeleteFailures) && !beforeDeleteFailures[i].Success && beforeDeleteFailures[i].Error != "" {
				continue
			}
			if successIndex < len(mergeDuplicates) {
				successfulDuplicates = append(successfulDuplicates, mergeDuplicates[successIndex])
			}
			if successIndex < len(mergeDuplicateBefore) {
				successfulDuplicateBefore = append(successfulDuplicateBefore, mergeDuplicateBefore[successIndex])
			}
			successIndex++
		}
	}
	if len(successfulDuplicates) > 0 {
		afterMaster, err := vm.afterRecords("update", master, []dml.Result{{ID: master[0].ID, Success: true}})
		if err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return Null, err
		}
		if _, err := vm.runTriggers(triggerTimingAfter, "update", afterMaster, masterBefore, result); err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return Null, err
		}
		if _, err := vm.runTriggers(triggerTimingAfter, "delete", successfulDuplicates, successfulDuplicateBefore, result); err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return Null, err
		}
	}
	return vm.mergeResultValue(args[1].Kind == ValueList, duplicates, results), nil
}

func (vm *VM) mergeResultValue(listInput bool, duplicates []storage.Record, results []dml.Result) Value {
	values := make([]Value, 0, len(results))
	for _, dmlResult := range results {
		row := Object("Database.MergeResult")
		row.Fields["success"] = Bool(dmlResult.Success)
		row.Fields["id"] = databaseResultIDValue(dmlResult.ID)
		row.Fields["error"] = String(dmlResult.Error)
		mergedIDs := List()
		for _, id := range dmlResult.MergedRecordIDs {
			mergedIDs.List = append(mergedIDs.List, String(string(id)))
		}
		row.Fields["mergedRecordIds"] = mergedIDs
		updatedRelatedIDs := List()
		for _, id := range dmlResult.UpdatedRelatedIDs {
			updatedRelatedIDs.List = append(updatedRelatedIDs.List, String(string(id)))
		}
		row.Fields["updatedRelatedIds"] = updatedRelatedIDs
		row.Fields["errors"] = databaseErrorsList(dmlResult)
		values = append(values, row)
	}
	if listInput {
		return List(values...)
	}
	if len(values) == 0 {
		return Null
	}
	return values[0]
}

func (vm *VM) externalIDFieldName(value Value) (string, error) {
	switch value.Kind {
	case ValueString:
		return value.Text, nil
	case ValueObject:
		if value.Type == "Schema.SObjectField" {
			field, ok := value.Fields["field"]
			if !ok || field.Kind != ValueString {
				return "", fmt.Errorf("Database.upsert external id field token is missing field name")
			}
			return field.Text, nil
		}
	}
	return "", fmt.Errorf("Database.upsert external id field expects Schema.SObjectField")
}

func (vm *VM) populateDMLResultFields(value *Value, results []dml.Result) {
	if value.Kind == ValueList {
		for i := range value.List {
			if i >= len(results) || !results[i].Success {
				continue
			}
			vm.populateDMLResultFields(&value.List[i], results[i:i+1])
		}
		return
	}
	if len(results) > 0 && results[0].Success && value.Kind == ValueObject {
		id := results[0].ID
		value.Fields["Id"] = String(string(id))
		if vm.Org == nil || id == "" {
			return
		}
		objectName, ok := storage.ResolveObjectName(*vm.Org, value.Type)
		if !ok {
			return
		}
		record, ok := vm.Org.Objects[objectName].Records[id]
		if !ok {
			return
		}
		putSystemFields(*value, record.System)
	}
}

func hasDMLFailures(results []dml.Result) bool {
	for _, result := range results {
		if !result.Success && result.Error != "" {
			return true
		}
	}
	return false
}

func filterDMLInputs(records, before []storage.Record, targets []*Value, failures []dml.Result) ([]storage.Record, []storage.Record, []*Value) {
	filteredRecords := make([]storage.Record, 0, len(records))
	filteredBefore := make([]storage.Record, 0, len(before))
	filteredTargets := make([]*Value, 0, len(targets))
	for i, record := range records {
		if i < len(failures) && !failures[i].Success && failures[i].Error != "" {
			continue
		}
		filteredRecords = append(filteredRecords, record)
		if i < len(before) {
			filteredBefore = append(filteredBefore, before[i])
		}
		if i < len(targets) {
			filteredTargets = append(filteredTargets, targets[i])
		}
	}
	return filteredRecords, filteredBefore, filteredTargets
}

func mergeDMLResults(failures, successes []dml.Result) []dml.Result {
	out := make([]dml.Result, len(failures))
	successIndex := 0
	for i, failure := range failures {
		if !failure.Success && failure.Error != "" {
			out[i] = failure
			continue
		}
		if successIndex < len(successes) {
			out[i] = successes[successIndex]
			successIndex++
		}
	}
	return out
}

func mergeDMLFailuresInPlace(target, source []dml.Result) {
	for i, failure := range source {
		if i >= len(target) || failure.Success || failure.Error == "" {
			continue
		}
		if target[i].Error == "" {
			target[i] = failure
			continue
		}
		combinedErrors := append(dmlResultErrors(target[i]), dmlResultErrors(failure)...)
		target[i].Error += "; " + failure.Error
		target[i].StatusCode = failure.StatusCode
		target[i].Fields = append(target[i].Fields, failure.Fields...)
		target[i].Errors = combinedErrors
	}
}

func (vm *VM) applyDML(op string, value Value, allOrNone bool, externalIDField string, result *Result) ([]dml.Result, error) {
	if vm.Org == nil {
		return nil, fmt.Errorf("DML requires org state")
	}
	records, targets, err := vm.recordsFromValue(value)
	if err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("dmlStatements", 1); err != nil {
		return nil, err
	}
	dmlRows := len(records)
	if op == "delete" {
		dmlRows += vm.cascadeDeleteRowCount(records)
	}
	if err := vm.incrementLimit("dmlRows", dmlRows); err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("cpuTime", dmlRows); err != nil {
		return nil, err
	}
	if err := vm.checkMixedDML(records); err != nil {
		return nil, err
	}
	if op == "upsert" {
		return vm.applyUpsertDML(records, targets, allOrNone, externalIDField, result)
	}
	if result != nil {
		result.Trace = append(result.Trace, trace.Instant("apex.dml."+op, "apex.dml", int64(len(result.Trace)), map[string]any{
			"operation": op,
			"rows":      len(records),
		}))
	}
	if op == "insert" {
		vm.applySObjectFieldDefaults(records)
	}
	before, err := vm.oldRecords(op, records)
	if err != nil {
		return nil, err
	}
	backup := vm.Org.Clone()
	var beforeFailures []dml.Result
	if op != "undelete" {
		beforeFailures, err = vm.runTriggers(triggerTimingBefore, op, records, before, result)
		if err != nil {
			*vm.Org = backup
			return nil, err
		}
	}
	if hasDMLFailures(beforeFailures) {
		if allOrNone {
			*vm.Org = backup
			return beforeFailures, nil
		}
		records, before, targets = filterDMLInputs(records, before, targets, beforeFailures)
		if len(records) == 0 {
			return beforeFailures, nil
		}
	}
	engine := vm.newDMLEngine()
	var results []dml.Result
	switch op {
	case "insert":
		results = engine.Insert(records)
	case "update":
		results = engine.Update(records)
	case "delete":
		results = engine.Delete(records)
	case "upsert":
		if externalIDField != "" {
			results = engine.UpsertWithExternalID(records, externalIDField)
		} else {
			results = engine.Upsert(records)
		}
	case "undelete":
		results = engine.Undelete(records)
	default:
		return nil, fmt.Errorf("unsupported DML operation %s", op)
	}
	engineResults := results
	if hasDMLFailures(beforeFailures) {
		results = mergeDMLResults(beforeFailures, results)
	}
	if allOrNone {
		for _, dmlResult := range results {
			if !dmlResult.Success {
				*vm.Org = backup
				return results, nil
			}
		}
	}
	for i, dmlResult := range engineResults {
		if dmlResult.Success && i < len(targets) && targets[i] != nil {
			vm.populateDMLResultFields(targets[i], engineResults[i:i+1])
		}
	}
	afterInputRecords, afterInputBefore, afterInputResults := successfulDMLInputs(records, before, engineResults)
	afterRecords, err := vm.afterRecords(op, afterInputRecords, afterInputResults)
	if err != nil {
		if allOrNone {
			*vm.Org = backup
		}
		return results, err
	}
	if _, err := vm.runTriggers(triggerTimingAfter, op, afterRecords, afterInputBefore, result); err != nil {
		if allOrNone {
			*vm.Org = backup
		}
		return nil, err
	}
	return results, nil
}

func (vm *VM) applySObjectFieldDefaults(records []storage.Record) {
	if vm.Org == nil {
		return
	}
	for i := range records {
		objectName, ok := storage.ResolveObjectName(*vm.Org, records[i].Object)
		if !ok {
			continue
		}
		definition := vm.Org.Objects[objectName].Definition
		if records[i].Fields == nil {
			records[i].Fields = make(map[string]storage.Value)
		}
		for name, field := range definition.Fields {
			if _, ok := records[i].Fields[name]; ok {
				continue
			}
			if records[i].ExplicitNulls != nil && records[i].ExplicitNulls[name] {
				continue
			}
			if value, ok := storage.DefaultValueForField(field); ok {
				records[i].Fields[name] = value
			}
		}
	}
}

func successfulDMLInputs(records, before []storage.Record, results []dml.Result) ([]storage.Record, []storage.Record, []dml.Result) {
	filteredRecords := make([]storage.Record, 0, len(records))
	filteredBefore := make([]storage.Record, 0, len(before))
	filteredResults := make([]dml.Result, 0, len(results))
	for i, record := range records {
		if i >= len(results) || !results[i].Success {
			continue
		}
		filteredRecords = append(filteredRecords, record)
		if i < len(before) {
			filteredBefore = append(filteredBefore, before[i])
		}
		filteredResults = append(filteredResults, results[i])
	}
	return filteredRecords, filteredBefore, filteredResults
}

func (vm *VM) applyUpsertDML(records []storage.Record, targets []*Value, allOrNone bool, externalIDField string, result *Result) ([]dml.Result, error) {
	if result != nil {
		result.Trace = append(result.Trace, trace.Instant("apex.dml.upsert", "apex.dml", int64(len(result.Trace)), map[string]any{
			"operation": "upsert",
			"rows":      len(records),
		}))
	}
	backup := vm.Org.Clone()
	kinds := make([]string, len(records))
	before := make([]storage.Record, len(records))
	for i, record := range records {
		kind, old, err := vm.classifyUpsert(record, externalIDField)
		if err != nil {
			return nil, err
		}
		kinds[i] = kind
		before[i] = old
		if kind == "update" && records[i].ID == "" && old.ID != "" {
			records[i].ID = old.ID
		}
	}
	beforeFailures := make([]dml.Result, len(records))
	for _, kind := range []string{"insert", "update"} {
		groupRecords, groupBefore, indices := groupedDMLInputs(records, before, kinds, kind)
		failures, err := vm.runTriggers(triggerTimingBefore, kind, groupRecords, groupBefore, result)
		if err != nil {
			*vm.Org = backup
			return nil, err
		}
		for groupIndex, failure := range failures {
			if groupIndex < len(indices) && !failure.Success && failure.Error != "" {
				beforeFailures[indices[groupIndex]] = failure
			}
		}
		for groupIndex, index := range indices {
			if groupIndex < len(groupRecords) {
				records[index] = groupRecords[groupIndex]
			}
		}
	}
	if hasDMLFailures(beforeFailures) {
		if allOrNone {
			*vm.Org = backup
			return beforeFailures, nil
		}
		records, before, targets, kinds = filterUpsertInputs(records, before, targets, kinds, beforeFailures)
		if len(records) == 0 {
			return beforeFailures, nil
		}
	}
	engine := vm.newDMLEngine()
	var engineResults []dml.Result
	if externalIDField != "" {
		engineResults = engine.UpsertWithExternalID(records, externalIDField)
	} else {
		engineResults = engine.Upsert(records)
	}
	results := engineResults
	if hasDMLFailures(beforeFailures) {
		results = mergeDMLResults(beforeFailures, engineResults)
	}
	if allOrNone {
		for _, dmlResult := range results {
			if !dmlResult.Success {
				*vm.Org = backup
				return results, nil
			}
		}
	}
	for i, dmlResult := range engineResults {
		if dmlResult.Success && i < len(targets) && targets[i] != nil {
			vm.populateDMLResultFields(targets[i], engineResults[i:i+1])
		}
	}
	for _, kind := range []string{"insert", "update"} {
		groupRecords, groupBefore, groupResults, _ := successfulGroupedDMLInputs(records, before, engineResults, kinds, kind)
		afterRecords, err := vm.afterRecords(kind, groupRecords, groupResults)
		if err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return results, err
		}
		if _, err := vm.runTriggers(triggerTimingAfter, kind, afterRecords, groupBefore, result); err != nil {
			if allOrNone {
				*vm.Org = backup
			}
			return nil, err
		}
	}
	return results, nil
}

func groupedDMLInputs(records, before []storage.Record, kinds []string, want string) ([]storage.Record, []storage.Record, []int) {
	groupRecords := make([]storage.Record, 0, len(records))
	groupBefore := make([]storage.Record, 0, len(before))
	indices := make([]int, 0, len(records))
	for i, kind := range kinds {
		if kind != want {
			continue
		}
		groupRecords = append(groupRecords, records[i])
		if i < len(before) {
			groupBefore = append(groupBefore, before[i])
		}
		indices = append(indices, i)
	}
	return groupRecords, groupBefore, indices
}

func successfulGroupedDMLInputs(records, before []storage.Record, results []dml.Result, kinds []string, want string) ([]storage.Record, []storage.Record, []dml.Result, []int) {
	groupRecords := make([]storage.Record, 0, len(records))
	groupBefore := make([]storage.Record, 0, len(before))
	groupResults := make([]dml.Result, 0, len(results))
	indices := make([]int, 0, len(records))
	for i, kind := range kinds {
		if kind != want || i >= len(results) || !results[i].Success {
			continue
		}
		groupRecords = append(groupRecords, records[i])
		if i < len(before) {
			groupBefore = append(groupBefore, before[i])
		}
		groupResults = append(groupResults, results[i])
		indices = append(indices, i)
	}
	return groupRecords, groupBefore, groupResults, indices
}

func filterUpsertInputs(records, before []storage.Record, targets []*Value, kinds []string, failures []dml.Result) ([]storage.Record, []storage.Record, []*Value, []string) {
	filteredRecords := make([]storage.Record, 0, len(records))
	filteredBefore := make([]storage.Record, 0, len(before))
	filteredTargets := make([]*Value, 0, len(targets))
	filteredKinds := make([]string, 0, len(kinds))
	for i, record := range records {
		if i < len(failures) && !failures[i].Success && failures[i].Error != "" {
			continue
		}
		filteredRecords = append(filteredRecords, record)
		if i < len(before) {
			filteredBefore = append(filteredBefore, before[i])
		}
		if i < len(targets) {
			filteredTargets = append(filteredTargets, targets[i])
		}
		if i < len(kinds) {
			filteredKinds = append(filteredKinds, kinds[i])
		}
	}
	return filteredRecords, filteredBefore, filteredTargets, filteredKinds
}

func (vm *VM) checkMixedDML(records []storage.Record) error {
	if vm.testContext == nil || vm.testContext.RunAsDepth > 0 {
		return nil
	}
	hasSetup := false
	hasNonSetup := false
	for _, record := range records {
		if isSetupObject(record.Object) {
			hasSetup = true
		} else {
			hasNonSetup = true
		}
	}
	if hasSetup {
		vm.testContext.SetupDML = true
	}
	if hasNonSetup {
		vm.testContext.NonSetupDML = true
	}
	if vm.testContext.SetupDML && vm.testContext.NonSetupDML {
		return &RuntimeError{
			Type:    "System.DmlException",
			Message: "Mixed DML operation detected; wrap supported setup/non-setup test work in System.runAs",
			Stack:   vm.stackFrames(),
		}
	}
	return nil
}

func isSetupObject(objectName string) bool {
	switch strings.ToLower(objectName) {
	case "user", "profile", "permissionset", "permissionsetassignment", "group", "groupmember", "queuesobject":
		return true
	default:
		return false
	}
}

func (vm *VM) recordsFromValue(value Value) ([]storage.Record, []*Value, error) {
	if value.Kind == ValueList {
		records := make([]storage.Record, 0, len(value.List))
		targets := make([]*Value, 0, len(value.List))
		for i := range value.List {
			record, err := vm.recordFromValue(&value.List[i])
			if err != nil {
				return nil, nil, err
			}
			records = append(records, record)
			targets = append(targets, &value.List[i])
		}
		return records, targets, nil
	}
	record, err := vm.recordFromValue(&value)
	if err != nil {
		return nil, nil, err
	}
	return []storage.Record{record}, []*Value{&value}, nil
}

func (vm *VM) recordFromValue(value *Value) (storage.Record, error) {
	if value.Kind != ValueObject {
		return storage.Record{}, fmt.Errorf("DML requires sObject value, got %s", value.Kind)
	}
	if reason, ok := sobjectReadOnlyReason(*value); ok {
		return storage.Record{}, fmt.Errorf("DML cannot modify read-only %s", reason)
	}
	objectType := value.Type
	var definition storage.ObjectDefinition
	if vm.Org != nil {
		if canonical, ok := storage.ResolveObjectName(*vm.Org, objectType); ok {
			objectType = canonical
			definition = vm.Org.Objects[canonical].Definition
		}
	}
	record := storage.Record{
		Object:        objectType,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
	for field, fieldValue := range value.Fields {
		if field == sobjectErrorsField || field == sobjectReadOnlyField {
			continue
		}
		if field == "Id" {
			if fieldValue.Kind == ValueString {
				record.ID = storage.ID(fieldValue.Text)
			} else if fieldValue.Kind == ValueObject && strings.EqualFold(fieldValue.Type, "Id") {
				if raw, err := platformScalarText(fieldValue, "Id"); err == nil {
					record.ID = storage.ID(raw)
				}
			}
			continue
		}
		if isSObjectSystemField(field) {
			continue
		}
		canonicalField := field
		if definition.APIName != "" {
			if resolved, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field); ok {
				canonicalField = resolved
			}
		}
		converted, err := storageValueFromVM(fieldValue)
		if definition.APIName != "" {
			if fieldDef, ok := definition.Fields[canonicalField]; ok {
				converted, err = storageValueFromVMForField(fieldValue, fieldDef.Type)
			}
		}
		if err != nil {
			return storage.Record{}, fmt.Errorf("%s.%s: %w", value.Type, field, err)
		}
		if converted.Kind == storage.ValueNull {
			record.ExplicitNulls[canonicalField] = true
		} else {
			record.Fields[canonicalField] = converted
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
	for relationship, records := range record.Children {
		children := make([]Value, 0, len(records))
		for _, child := range records {
			children = append(children, vmValueFromRecord(child))
		}
		value.Fields[relationship] = List(children...)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			value.Fields[field] = Null
		}
	}
	putSystemFields(value, record.System)
	return value
}

func isSObjectSystemField(field string) bool {
	switch field {
	case "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SystemModstamp", "OwnerId", "IsDeleted":
		return true
	default:
		return false
	}
}

func putSystemFields(value Value, fields storage.SystemFields) {
	if fields.CreatedDate != "" {
		value.Fields["CreatedDate"] = platformScalar("Datetime", fields.CreatedDate)
	}
	if fields.CreatedByID != "" {
		value.Fields["CreatedById"] = String(string(fields.CreatedByID))
	}
	if fields.LastModifiedDate != "" {
		value.Fields["LastModifiedDate"] = platformScalar("Datetime", fields.LastModifiedDate)
	}
	if fields.LastModifiedByID != "" {
		value.Fields["LastModifiedById"] = String(string(fields.LastModifiedByID))
	}
	if fields.SystemModstamp != "" {
		value.Fields["SystemModstamp"] = platformScalar("Datetime", fields.SystemModstamp)
	}
	if fields.OwnerID != "" {
		value.Fields["OwnerId"] = String(string(fields.OwnerID))
	}
	value.Fields["IsDeleted"] = Bool(fields.IsDeleted)
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
	case ValueDecimal:
		return storage.DecimalValue(value.String()), nil
	case ValueBool:
		return storage.BooleanValue(value.Bool), nil
	case ValueObject:
		if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
			switch value.Type {
			case "Id", "String":
				return storage.StringValue(raw.Text), nil
			case "Date":
				return storage.DateValue(raw.Text), nil
			case "Datetime", "DateTime":
				return storage.DateTimeValue(raw.Text), nil
			case "Time":
				return storage.StringValue(raw.Text), nil
			case "Blob":
				return storage.BlobValue(raw.Text), nil
			}
		}
		return storage.Value{}, fmt.Errorf("unsupported storage value %s", value.Kind)
	default:
		return storage.Value{}, fmt.Errorf("unsupported storage value %s", value.Kind)
	}
}

func storageValueFromVMForField(value Value, fieldType storage.FieldType) (storage.Value, error) {
	if value.Kind == ValueNull || fieldType == storage.FieldAny {
		return storageValueFromVM(value)
	}
	switch fieldType {
	case storage.FieldID, storage.FieldString, storage.FieldPicklist, storage.FieldReference:
		if value.Kind == ValueString {
			return storageValueFromVM(value)
		}
		if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
			if raw, err := platformScalarText(value, "Id"); err == nil {
				return storage.IDValue(storage.ID(raw)), nil
			}
		}
	case storage.FieldBlob:
		if value.Kind == ValueObject && value.Type == "Blob" {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return storage.BlobValue(raw.Text), nil
			}
		}
		if value.Kind == ValueString {
			return storage.BlobValue(value.Text), nil
		}
	case storage.FieldDate:
		if value.Kind == ValueString {
			return storage.DateValue(value.Text), nil
		}
		if value.Kind == ValueObject && value.Type == "Date" {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return storage.DateValue(raw.Text), nil
			}
		}
	case storage.FieldDateTime:
		if value.Kind == ValueString {
			return storage.DateTimeValue(value.Text), nil
		}
		if value.Kind == ValueObject && value.Type == "Datetime" {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return storage.DateTimeValue(raw.Text), nil
			}
		}
	case storage.FieldBoolean:
		if value.Kind == ValueBool {
			return storageValueFromVM(value)
		}
	case storage.FieldInteger:
		if value.Kind == ValueInt {
			return storageValueFromVM(value)
		}
	case storage.FieldDecimal:
		if value.Kind == ValueInt {
			return storage.DecimalValue(strconv.FormatInt(value.Int, 10)), nil
		}
		if value.Kind == ValueDecimal {
			return storageValueFromVM(value)
		}
	}
	return storage.Value{}, fmt.Errorf("cannot assign %s to %s field", value.Kind, fieldType)
}

func vmValueFromStorage(value storage.Value) Value {
	switch value.Kind {
	case storage.ValueNull:
		return Null
	case storage.ValueString:
		return String(value.String)
	case storage.ValueDate:
		return platformScalar("Date", value.String)
	case storage.ValueDateTime:
		return platformScalar("Datetime", value.String)
	case storage.ValueBlob:
		return platformScalar("Blob", value.String)
	case storage.ValueInteger:
		return Int(value.Integer)
	case storage.ValueBoolean:
		return Bool(value.Boolean)
	case storage.ValueDecimal:
		parsed, err := strconv.ParseFloat(value.Decimal, 64)
		if err != nil {
			return String(value.Decimal)
		}
		return Decimal(parsed)
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
	case ValueList:
		items := make([]string, 0, len(value.List))
		for _, item := range value.List {
			items = append(items, soqlLiteral(item))
		}
		return "(" + strings.Join(items, ", ") + ")"
	case ValueSet:
		items := make([]string, 0, len(value.Set))
		for _, item := range value.Set {
			items = append(items, soqlLiteral(item))
		}
		return "(" + strings.Join(items, ", ") + ")"
	case ValueObject:
		if value.Type == "Date" || value.Type == "Datetime" || value.Type == "Time" || value.Type == "Id" || value.Type == "String" {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return "'" + strings.ReplaceAll(raw.Text, "'", "''") + "'"
			}
		}
		if idValue, ok := value.Fields["Id"]; ok {
			if idValue.Kind == ValueString {
				return "'" + strings.ReplaceAll(idValue.Text, "'", "''") + "'"
			}
			if idValue.Kind == ValueObject && strings.EqualFold(idValue.Type, "Id") {
				if raw, err := platformScalarText(idValue, "Id"); err == nil && raw != "" {
					return "'" + strings.ReplaceAll(raw, "'", "''") + "'"
				}
			}
		}
		return "'" + strings.ReplaceAll(value.String(), "'", "''") + "'"
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
		objectName := record.Object
		if canonical, ok := storage.ResolveObjectName(*vm.Org, record.Object); ok {
			objectName = canonical
		}
		if record.ID == "" {
			out = append(out, storage.Record{Object: objectName})
			continue
		}
		object, ok := vm.Org.Objects[objectName]
		if !ok {
			return nil, fmt.Errorf("dml: unknown object %s", record.Object)
		}
		old, ok := object.Records[record.ID]
		if !ok {
			out = append(out, storage.Record{ID: record.ID, Object: objectName})
			continue
		}
		out = append(out, old.Clone())
	}
	return out, nil
}

func (vm *VM) classifyUpsert(record storage.Record, externalIDField string) (string, storage.Record, error) {
	objectName := record.Object
	if canonical, ok := storage.ResolveObjectName(*vm.Org, record.Object); ok {
		objectName = canonical
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return "", storage.Record{}, fmt.Errorf("dml: unknown object %s", record.Object)
	}
	if record.ID != "" {
		old, ok := object.Records[record.ID]
		if ok && !old.System.IsDeleted {
			return "update", old.Clone(), nil
		}
		return "update", storage.Record{ID: record.ID, Object: objectName}, nil
	}
	fieldName, value, ok := upsertMatchField(object.Definition, vm.Org.Namespace, record, externalIDField)
	if !ok {
		return "insert", storage.Record{Object: objectName}, nil
	}
	for _, stored := range object.Records {
		if stored.System.IsDeleted {
			continue
		}
		if storedValue, exists := stored.Fields[fieldName]; exists && storageValuesEqualForVM(object.Definition.Fields[fieldName], storedValue, value) {
			return "update", stored.Clone(), nil
		}
	}
	return "insert", storage.Record{Object: objectName}, nil
}

func upsertMatchField(definition storage.ObjectDefinition, namespace string, record storage.Record, externalIDField string) (string, storage.Value, bool) {
	if externalIDField != "" {
		fieldName := externalIDField
		if canonical, ok := storage.ResolveFieldName(definition, namespace, fieldName); ok {
			fieldName = canonical
		}
		value, ok := record.Fields[fieldName]
		return fieldName, value, ok && value.Kind != storage.ValueNull
	}
	for name, field := range definition.Fields {
		if !field.ExternalID {
			continue
		}
		value, ok := record.Fields[name]
		if ok && value.Kind != storage.ValueNull {
			return name, value, true
		}
	}
	return "", storage.Value{}, false
}

func storageValuesEqualForVM(field storage.Field, left, right storage.Value) bool {
	if left.Kind == storage.ValueString && right.Kind == storage.ValueString && !field.CaseSensitive {
		return strings.EqualFold(left.String, right.String)
	}
	if left.Kind != right.Kind {
		if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
			return string(left.ID) == right.String
		}
		if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
			return left.String == string(right.ID)
		}
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return left.String == right.String
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueID:
		return left.ID == right.ID
	default:
		return false
	}
}

func (vm *VM) cascadeDeleteRowCount(records []storage.Record) int {
	if vm.Org == nil {
		return 0
	}
	total := 0
	seen := make(map[string]bool)
	for _, record := range records {
		objectName := record.Object
		if canonical, ok := storage.ResolveObjectName(*vm.Org, record.Object); ok {
			objectName = canonical
		}
		total += vm.cascadeDeleteRowCountFrom(objectName, record.ID, seen)
	}
	return total
}

func (vm *VM) cascadeDeleteRowCountFrom(objectName string, id storage.ID, seen map[string]bool) int {
	if id == "" {
		return 0
	}
	key := objectName + ":" + string(id)
	if seen[key] {
		return 0
	}
	seen[key] = true
	total := 0
	for childObjectName, childObject := range vm.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if !relation.CascadeDelete || !stringSliceContains(relation.ParentObjects, objectName) {
				continue
			}
			for childID, child := range childObject.Records {
				if child.System.IsDeleted {
					continue
				}
				value, ok := child.Fields[relation.Field]
				if !ok || storageIDFromValue(value) != id {
					continue
				}
				total++
				total += vm.cascadeDeleteRowCountFrom(childObjectName, childID, seen)
			}
		}
	}
	return total
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func storageIDFromValue(value storage.Value) storage.ID {
	switch value.Kind {
	case storage.ValueID:
		return value.ID
	case storage.ValueString:
		return storage.ID(value.String)
	default:
		return ""
	}
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
		objectName := record.Object
		if canonical, ok := storage.ResolveObjectName(*vm.Org, record.Object); ok {
			objectName = canonical
		}
		object := vm.Org.Objects[objectName]
		stored, ok := object.Records[id]
		if !ok {
			continue
		}
		out = append(out, stored.Clone())
	}
	return out, nil
}

func (vm *VM) runTriggers(timing, op string, records, oldRecords []storage.Record, result *Result) ([]dml.Result, error) {
	if len(records) == 0 {
		return nil, nil
	}
	object := records[0].Object
	triggers := make([]Trigger, 0, len(vm.Triggers[object]))
	for _, trigger := range vm.Triggers[object] {
		if trigger.Timing == timing && trigger.Operation == op {
			triggers = append(triggers, trigger)
		}
	}
	if len(triggers) == 0 {
		return nil, nil
	}
	if vm.triggerDepth >= maxTriggerDepth {
		return nil, newExceptionError("DmlException", fmt.Sprintf("maximum trigger depth exceeded (%d)", maxTriggerDepth))
	}
	vm.triggerDepth++
	defer func() {
		vm.triggerDepth--
	}()
	failures := make([]dml.Result, len(records))
	for _, trigger := range triggers {
		triggerFailures, err := vm.runTrigger(trigger, records, oldRecords, result)
		if err != nil {
			return nil, err
		}
		mergeDMLFailuresInPlace(failures, triggerFailures)
	}
	if hasDMLFailures(failures) {
		return failures, nil
	}
	return nil, nil
}

func (vm *VM) runTrigger(trigger Trigger, records, oldRecords []storage.Record, result *Result) ([]dml.Result, error) {
	appendTrace(result, "apex.trigger."+trigger.Name, "apex.trigger", map[string]any{
		"trigger":   trigger.Name,
		"object":    trigger.Object,
		"timing":    trigger.Timing,
		"operation": trigger.Operation,
		"rows":      len(records),
		"file":      trigger.File,
		"line":      trigger.Line,
		"column":    trigger.Column,
	})
	caller := vm.Globals
	callerClass := vm.currentClass
	callerTriggerGlobals := vm.triggerGlobals
	frame := make(map[string]Value)
	ctx := triggerContext(trigger, records, oldRecords)
	for key, value := range ctx {
		frame[key] = value
	}
	vm.Globals = frame
	vm.triggerGlobals = ctx
	vm.currentClass = trigger.Name
	vm.callStack = append(vm.callStack, callFrame{Symbol: trigger.Name, File: trigger.File, Line: trigger.Line, Column: trigger.Column})
	defer func() {
		vm.callStack = vm.callStack[:len(vm.callStack)-1]
		vm.Globals = caller
		vm.triggerGlobals = callerTriggerGlobals
		vm.currentClass = callerClass
	}()
	out, err := vm.executeProgram(trigger.Program, result)
	if err != nil {
		return nil, err
	}
	if out.signal == signalThrow {
		return nil, &apexThrowError{value: out.thrown, stack: append([]callFrame(nil), vm.callStack...)}
	}
	if trigger.Timing == triggerTimingBefore {
		updated := ctx["Trigger.new"]
		if trigger.Operation == "delete" {
			updated = ctx["Trigger.old"]
		}
		if updated.Kind == ValueList {
			failures := dmlResultsFromSObjectErrors(records, updated.List)
			if trigger.Operation != "delete" {
				for i, item := range updated.List {
					if i >= len(records) {
						break
					}
					record, err := vm.recordFromValue(&item)
					if err != nil {
						return nil, err
					}
					if records[i].ID != "" && record.ID == "" {
						record.ID = records[i].ID
					}
					records[i] = record
				}
			}
			if hasDMLFailures(failures) {
				return failures, nil
			}
		}
	}
	return nil, nil
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
	newListValue := Null
	newMapValue := Null
	if trigger.Operation == "insert" || trigger.Operation == "update" || trigger.Operation == "undelete" {
		newListValue = List(newValues...)
		if trigger.Operation != "insert" || trigger.Timing == triggerTimingAfter {
			newMapValue = newMap
		}
	}
	oldListValue := Null
	oldMapValue := Null
	if trigger.Operation == "update" || trigger.Operation == "delete" {
		oldListValue = List(oldValues...)
		oldMapValue = oldMap
	}
	ctx := map[string]Value{
		"Trigger.new":           newListValue,
		"Trigger.old":           oldListValue,
		"Trigger.newMap":        newMapValue,
		"Trigger.oldMap":        oldMapValue,
		"Trigger.isExecuting":   Bool(true),
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

func (vm *VM) sObjectNameForIDPrefix(prefix string) (string, bool) {
	if vm.Org != nil {
		names := make([]string, 0, len(vm.Org.Objects))
		for name := range vm.Org.Objects {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if strings.EqualFold(vm.Org.Objects[name].Definition.KeyPrefix, prefix) {
				return name, true
			}
		}
	}
	name, ok := standardSObjectPrefixes[prefix]
	return name, ok
}

var standardSObjectPrefixes = map[string]string{
	"001": "Account",
	"003": "Contact",
	"005": "User",
	"006": "Opportunity",
	"00Q": "Lead",
	"00T": "Task",
	"00U": "Event",
	"00D": "Organization",
	"500": "Case",
	"701": "Campaign",
}

func platformScalar(typeName, value string) Value {
	out := Object(typeName)
	out.Fields["value"] = String(value)
	return out
}

func platformScalarText(value Value, typeName string) (string, error) {
	if value.Kind != ValueObject || value.Type != typeName {
		return "", fmt.Errorf("expected %s value", typeName)
	}
	raw, ok := value.Fields["value"]
	if !ok || raw.Kind != ValueString {
		return "", fmt.Errorf("%s value is missing scalar text", typeName)
	}
	return raw.Text, nil
}

func defaultURLPort(scheme string) int64 {
	switch strings.ToLower(scheme) {
	case "http":
		return 80
	case "https":
		return 443
	case "ftp":
		return 21
	default:
		return -1
	}
}

func parsePlatformDate(value Value) (time.Time, error) {
	text, err := platformScalarText(value, "Date")
	if err != nil {
		return time.Time{}, err
	}
	date, err := parseDateText(text)
	if err != nil {
		return time.Time{}, err
	}
	return date, nil
}

func parsePlatformDatetime(value Value) (time.Time, error) {
	text, err := platformScalarText(value, "Datetime")
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := parseDatetimeText(text)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func datetimeFromLocalParts(year, month, day, hour, minute, second, millisecond int, zoneID string) (time.Time, error) {
	canonical, offset, ok := parseFixedTimeZoneID(zoneID)
	if ok {
		return time.Date(year, time.Month(month), day, hour, minute, second, millisecond*int(time.Millisecond), time.FixedZone(canonical, int(offset/time.Second))).UTC(), nil
	}
	zone, ok := supportedNamedTimeZone(zoneID)
	if !ok {
		return time.Time{}, unsupportedCallError("Datetime.newInstance timezone " + zoneID)
	}
	return zone.instantFromLocal(year, time.Month(month), day, hour, minute, second, millisecond), nil
}

func addMonthsClamped(value time.Time, months int) time.Time {
	year, month, day := value.Date()
	monthIndex := year*12 + int(month) - 1 + months
	targetYear := monthIndex / 12
	targetMonthIndex := monthIndex % 12
	if targetMonthIndex < 0 {
		targetMonthIndex += 12
		targetYear--
	}
	targetMonth := time.Month(targetMonthIndex + 1)
	if maxDay := daysInMonth(targetYear, targetMonth); day > maxDay {
		day = maxDay
	}
	return time.Date(targetYear, targetMonth, day, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location())
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func parseDatetimeText(text string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.000", "2006-01-02 15:04:05", "2006-01-02T15:04:05.000", "2006-01-02T15:04:05"} {
		if value, err := time.Parse(layout, text); err == nil {
			if err := validateDateParts(value.Year(), int(value.Month()), value.Day()); err != nil {
				return time.Time{}, err
			}
			return value, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported Datetime value %q", text)
}

func parseDateText(text string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if value, err := time.Parse(layout, text); err == nil {
			if err := validateDateParts(value.Year(), int(value.Month()), value.Day()); err != nil {
				return time.Time{}, err
			}
			return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
}

func formatPlatformDatetime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatApexDatetimePattern(value time.Time, pattern, zoneID, zoneLabel string, offset time.Duration) (string, error) {
	var b strings.Builder
	for i := 0; i < len(pattern); {
		ch := pattern[i]
		if ch == '\'' {
			next, literal, err := readApexDatePatternLiteral(pattern, i)
			if err != nil {
				return "", err
			}
			b.WriteString(literal)
			i = next
			continue
		}
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') {
			b.WriteByte(ch)
			i++
			continue
		}
		j := i + 1
		for j < len(pattern) && pattern[j] == ch {
			j++
		}
		token := pattern[i:j]
		text, err := formatApexDatetimeToken(value, token, zoneID, zoneLabel, offset)
		if err != nil {
			return "", err
		}
		b.WriteString(text)
		i = j
	}
	return b.String(), nil
}

func readApexDatePatternLiteral(pattern string, start int) (int, string, error) {
	var b strings.Builder
	for i := start + 1; i < len(pattern); i++ {
		if pattern[i] != '\'' {
			b.WriteByte(pattern[i])
			continue
		}
		if i+1 < len(pattern) && pattern[i+1] == '\'' {
			b.WriteByte('\'')
			i++
			continue
		}
		return i + 1, b.String(), nil
	}
	return 0, "", fmt.Errorf("Datetime.format unsupported unterminated quoted literal")
}

func formatApexDatetimeToken(value time.Time, token, zoneID, zoneLabel string, offset time.Duration) (string, error) {
	count := len(token)
	switch token[0] {
	case 'y':
		year := value.Year()
		if count == 2 {
			return fmt.Sprintf("%02d", year%100), nil
		}
		return fmt.Sprintf("%0*d", maxInt(count, 4), year), nil
	case 'M':
		month := value.Month()
		switch {
		case count >= 4:
			return month.String(), nil
		case count == 3:
			return month.String()[:3], nil
		case count == 2:
			return fmt.Sprintf("%02d", int(month)), nil
		default:
			return strconv.Itoa(int(month)), nil
		}
	case 'd':
		return formatPaddedDateNumber(value.Day(), count), nil
	case 'H':
		return formatPaddedDateNumber(value.Hour(), count), nil
	case 'h':
		hour := value.Hour() % 12
		if hour == 0 {
			hour = 12
		}
		return formatPaddedDateNumber(hour, count), nil
	case 'm':
		return formatPaddedDateNumber(value.Minute(), count), nil
	case 's':
		return formatPaddedDateNumber(value.Second(), count), nil
	case 'S':
		if count > 3 {
			return "", fmt.Errorf("Datetime.format unsupported pattern token %q", token)
		}
		millisecond := value.Nanosecond() / int(time.Millisecond)
		if count <= 1 {
			return strconv.Itoa(millisecond), nil
		}
		return fmt.Sprintf("%0*d", minInt(count, 3), millisecond), nil
	case 'a':
		if value.Hour() < 12 {
			return "AM", nil
		}
		return "PM", nil
	case 'E':
		name := value.Weekday().String()
		if count >= 4 {
			return name, nil
		}
		return name[:3], nil
	case 'G', 'L', 'c', 'e':
		return "", unsupportedCallError(fmt.Sprintf("Datetime.format locale-dependent pattern token %q", token))
	case 'Z':
		return formatRFC822Offset(offset), nil
	case 'z':
		if zoneID == "UTC" {
			return "UTC", nil
		}
		return zoneLabel, nil
	default:
		return "", fmt.Errorf("Datetime.format unsupported pattern token %q", token)
	}
}

func formatPaddedDateNumber(value, count int) string {
	if count >= 2 {
		return fmt.Sprintf("%02d", value)
	}
	return strconv.Itoa(value)
}

func formatRFC822Offset(offset time.Duration) string {
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	totalMinutes := int(offset / time.Minute)
	return fmt.Sprintf("%s%02d%02d", sign, totalMinutes/60, totalMinutes%60)
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func validateDateParts(year, month, day int) error {
	if year < 1 || year > 9999 {
		return fmt.Errorf("invalid Date parts: year=%d month=%d day=%d", year, month, day)
	}
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if value.Year() != year || int(value.Month()) != month || value.Day() != day {
		return fmt.Errorf("invalid Date parts: year=%d month=%d day=%d", year, month, day)
	}
	return nil
}

func validateTimeParts(hour, minute, second int) error {
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return fmt.Errorf("invalid Time parts: hour=%d minute=%d second=%d", hour, minute, second)
	}
	return nil
}

func parseTimeText(text string) (string, error) {
	for _, layout := range []string{"15:04:05.000", "15:04:05"} {
		if value, err := time.Parse(layout, text); err == nil {
			return formatPlatformTime(value.Hour(), value.Minute(), value.Second(), value.Nanosecond()/int(time.Millisecond)), nil
		}
	}
	return "", fmt.Errorf("unsupported Time value %q", text)
}

func parsePlatformTime(value Value) (time.Duration, error) {
	text, err := platformScalarText(value, "Time")
	if err != nil {
		return 0, err
	}
	parsed, err := parseTimeText(text)
	if err != nil {
		return 0, err
	}
	t, err := time.Parse("15:04:05.000", ensureTimeMillis(parsed))
	if err != nil {
		return 0, err
	}
	return time.Duration(t.Hour())*time.Hour +
		time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second +
		time.Duration(t.Nanosecond()), nil
}

func ensureTimeMillis(text string) string {
	if strings.Contains(text, ".") {
		return text
	}
	return text + ".000"
}

func formatPlatformTime(hour, minute, second, millisecond int) string {
	base := fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
	if millisecond == 0 {
		return base
	}
	return fmt.Sprintf("%s.%03d", base, millisecond)
}

func platformTimeFromDuration(value time.Duration) Value {
	day := 24 * time.Hour
	value %= day
	if value < 0 {
		value += day
	}
	hour := int(value / time.Hour)
	value %= time.Hour
	minute := int(value / time.Minute)
	value %= time.Minute
	second := int(value / time.Second)
	value %= time.Second
	millisecond := int(value / time.Millisecond)
	return platformScalar("Time", formatPlatformTime(hour, minute, second, millisecond))
}

func fixedTimeZone(id string) (Value, error) {
	canonical, offset, ok := parseFixedTimeZoneID(id)
	locationName := ""
	if !ok {
		location, locationOK := supportedNamedTimeZone(id)
		if !locationOK {
			return Null, unsupportedCallError("TimeZone.getTimeZone " + id)
		}
		canonical = id
		offset = location.standardOffset
		locationName = location.id
	} else if canonical == "UTC" {
		locationName = "UTC"
	}
	out := Object("TimeZone")
	out.Fields["id"] = String(canonical)
	out.Fields["offsetMillis"] = Int(int64(offset / time.Millisecond))
	out.Fields["location"] = String(locationName)
	return out, nil
}

type modeledTimeZone struct {
	id             string
	standardOffset time.Duration
	daylightOffset time.Duration
	standardLabel  string
	daylightLabel  string
	daylightRule   string
}

var supportedNamedTimeZones = map[string]modeledTimeZone{
	"America/Los_Angeles": {id: "America/Los_Angeles", standardOffset: -8 * time.Hour, daylightOffset: -7 * time.Hour, standardLabel: "PST", daylightLabel: "PDT", daylightRule: "us"},
	"America/New_York":    {id: "America/New_York", standardOffset: -5 * time.Hour, daylightOffset: -4 * time.Hour, standardLabel: "EST", daylightLabel: "EDT", daylightRule: "us"},
	"America/Chicago":     {id: "America/Chicago", standardOffset: -6 * time.Hour, daylightOffset: -5 * time.Hour, standardLabel: "CST", daylightLabel: "CDT", daylightRule: "us"},
	"America/Denver":      {id: "America/Denver", standardOffset: -7 * time.Hour, daylightOffset: -6 * time.Hour, standardLabel: "MST", daylightLabel: "MDT", daylightRule: "us"},
	"Europe/London":       {id: "Europe/London", standardOffset: 0, daylightOffset: time.Hour, standardLabel: "GMT", daylightLabel: "BST", daylightRule: "europe"},
	"Europe/Berlin":       {id: "Europe/Berlin", standardOffset: time.Hour, daylightOffset: 2 * time.Hour, standardLabel: "CET", daylightLabel: "CEST", daylightRule: "europe"},
	"Asia/Tokyo":          {id: "Asia/Tokyo", standardOffset: 9 * time.Hour, standardLabel: "JST"},
	"Australia/Sydney":    {id: "Australia/Sydney", standardOffset: 10 * time.Hour, daylightOffset: 11 * time.Hour, standardLabel: "AEST", daylightLabel: "AEDT", daylightRule: "sydney"},
}

func supportedNamedTimeZone(id string) (modeledTimeZone, bool) {
	location, ok := supportedNamedTimeZones[id]
	return location, ok
}

func resolveTimeZoneForInstant(id string, instant time.Time) (string, time.Duration, time.Time, string, bool) {
	canonical, offset, ok := parseFixedTimeZoneID(id)
	if ok {
		local := instant.UTC().In(time.FixedZone(canonical, int(offset/time.Second)))
		return canonical, offset, local, canonical, true
	}
	location, ok := supportedNamedTimeZone(id)
	if !ok {
		return "", 0, time.Time{}, "", false
	}
	offset, label := location.offsetAt(instant)
	local := instant.UTC().In(time.FixedZone(label, int(offset/time.Second)))
	return id, offset, local, label, true
}

func timeZoneOffsetMillis(receiver Value, instant time.Time) (Value, error) {
	locationValue := receiver.Fields["location"]
	if locationValue.Kind == ValueString && locationValue.Text != "" && locationValue.Text != "UTC" {
		location, ok := supportedNamedTimeZone(locationValue.Text)
		if !ok {
			return Null, unsupportedCallError("TimeZone.getOffset " + locationValue.Text)
		}
		offset, _ := location.offsetAt(instant)
		return Int(int64(offset / time.Millisecond)), nil
	}
	offsetValue := receiver.Fields["offsetMillis"]
	if offsetValue.Kind != ValueInt {
		return Null, fmt.Errorf("TimeZone offset is missing")
	}
	return offsetValue, nil
}

func timeZoneDisplayName(receiver Value, daylight bool) Value {
	locationValue := receiver.Fields["location"]
	if locationValue.Kind == ValueString && locationValue.Text != "" && locationValue.Text != "UTC" {
		if location, ok := supportedNamedTimeZone(locationValue.Text); ok {
			if daylight && location.daylightLabel != "" {
				return String(location.daylightLabel)
			}
			return String(location.standardLabel)
		}
	}
	return receiver.Fields["id"]
}

func (zone modeledTimeZone) offsetAt(instant time.Time) (time.Duration, string) {
	if zone.daylightRule == "" || !zone.isDaylight(instant.UTC()) {
		return zone.standardOffset, zone.standardLabel
	}
	return zone.daylightOffset, zone.daylightLabel
}

func (zone modeledTimeZone) instantFromLocal(year int, month time.Month, day, hour, minute, second, millisecond int) time.Time {
	local := time.Date(year, month, day, hour, minute, second, millisecond*int(time.Millisecond), time.UTC)
	offsets := []time.Duration{zone.standardOffset}
	if zone.daylightRule != "" && zone.daylightOffset != zone.standardOffset {
		offsets = append(offsets, zone.daylightOffset)
	}
	var matches []time.Time
	for _, offset := range offsets {
		candidate := local.Add(-offset)
		actualOffset, _ := zone.offsetAt(candidate)
		if candidate.Add(actualOffset).Equal(local) {
			matches = append(matches, candidate.UTC())
		}
	}
	if len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool { return matches[i].Before(matches[j]) })
		return matches[0]
	}
	return local.Add(-zone.standardOffset).UTC()
}

func (zone modeledTimeZone) isDaylight(instant time.Time) bool {
	year := instant.Year()
	switch zone.daylightRule {
	case "us":
		start := localRuleTransitionUTC(year, time.March, nthWeekdayOfMonth(year, time.March, time.Sunday, 2), 2, zone.standardOffset)
		end := localRuleTransitionUTC(year, time.November, nthWeekdayOfMonth(year, time.November, time.Sunday, 1), 2, zone.daylightOffset)
		return !instant.Before(start) && instant.Before(end)
	case "europe":
		start := time.Date(year, time.March, lastWeekdayOfMonth(year, time.March, time.Sunday), 1, 0, 0, 0, time.UTC)
		end := time.Date(year, time.October, lastWeekdayOfMonth(year, time.October, time.Sunday), 1, 0, 0, 0, time.UTC)
		return !instant.Before(start) && instant.Before(end)
	case "sydney":
		start := localRuleTransitionUTC(year, time.October, nthWeekdayOfMonth(year, time.October, time.Sunday, 1), 2, zone.standardOffset)
		end := localRuleTransitionUTC(year, time.April, nthWeekdayOfMonth(year, time.April, time.Sunday, 1), 3, zone.daylightOffset)
		return !instant.Before(start) || instant.Before(end)
	default:
		return false
	}
}

func localRuleTransitionUTC(year int, month time.Month, day, hour int, offsetBefore time.Duration) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.UTC).Add(-offsetBefore)
}

func nthWeekdayOfMonth(year int, month time.Month, weekday time.Weekday, n int) int {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	delta := (int(weekday) - int(first.Weekday()) + 7) % 7
	return 1 + delta + 7*(n-1)
}

func lastWeekdayOfMonth(year int, month time.Month, weekday time.Weekday) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
	delta := (int(last.Weekday()) - int(weekday) + 7) % 7
	return last.Day() - delta
}

func parseFixedTimeZoneID(id string) (string, time.Duration, bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed != id {
		return "", 0, false
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "UTC", "GMT", "ETC/UTC", "Z":
		return "UTC", 0, true
	}
	if !strings.HasPrefix(upper, "GMT+") && !strings.HasPrefix(upper, "GMT-") && !strings.HasPrefix(upper, "UTC+") && !strings.HasPrefix(upper, "UTC-") {
		return "", 0, false
	}
	prefix := upper[:3]
	signText := upper[3:4]
	rest := upper[4:]
	if prefix == "UTC" {
		rest = upper[4:]
	}
	parts := strings.Split(rest, ":")
	if len(parts) > 2 || parts[0] == "" {
		return "", 0, false
	}
	if len(parts[0]) > 2 || !allASCIIDigits(parts[0]) {
		return "", 0, false
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", 0, false
	}
	minutes := 0
	if len(parts) == 2 {
		if len(parts[1]) != 2 || !allASCIIDigits(parts[1]) {
			return "", 0, false
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, false
		}
	}
	if hours > 14 || minutes > 59 || (hours == 14 && minutes != 0) {
		return "", 0, false
	}
	offset := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
	if signText == "-" {
		offset = -offset
	}
	return fmt.Sprintf("GMT%s%02d:%02d", signText, hours, minutes), offset, true
}

func allASCIIDigits(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return text != ""
}

const (
	defaultHttpTimeoutMillis int64 = 10000
	maxHttpTimeoutMillis     int64 = 120000
)

func validateHttpRequest(request Value) error {
	endpoint, ok := request.Fields["endpoint"]
	if !ok || endpoint.Kind != ValueString {
		return fmt.Errorf("HttpRequest endpoint is required before Http.send")
	}
	if strings.TrimSpace(endpoint.Text) == "" {
		return fmt.Errorf("HttpRequest endpoint is required before Http.send")
	}
	if err := validateHttpEndpoint(endpoint.Text); err != nil {
		return err
	}
	method, ok := request.Fields["method"]
	if !ok || method.Kind != ValueString {
		return fmt.Errorf("HttpRequest method is required before Http.send")
	}
	if strings.TrimSpace(method.Text) == "" {
		return fmt.Errorf("HttpRequest method is required before Http.send")
	}
	if _, err := normalizeHttpMethod(method.Text); err != nil {
		return err
	}
	if timeout, ok := request.Fields["timeout"]; ok {
		if timeout.Kind != ValueInt {
			return fmt.Errorf("HttpRequest timeout must be Integer")
		}
		return validateHttpTimeout(timeout.Int)
	}
	return nil
}

func validateHttpEndpoint(endpoint string) error {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return fmt.Errorf("HttpRequest endpoint is required")
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "callout:") {
		if strings.TrimSpace(trimmed[len("callout:"):]) == "" {
			return fmt.Errorf("HttpRequest endpoint named credential is required")
		}
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("HttpRequest endpoint must be an absolute http, https, or callout URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("HttpRequest endpoint must use http, https, or callout scheme")
	}
	return nil
}

func normalizeHttpMethod(method string) (string, error) {
	trimmed := strings.TrimSpace(method)
	if trimmed == "" {
		return "", fmt.Errorf("HttpRequest method is required")
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "DELETE", "GET", "HEAD", "PATCH", "POST", "PUT", "TRACE":
		return upper, nil
	default:
		return "", fmt.Errorf("HttpRequest method %q is not supported", method)
	}
}

func validateHttpTimeout(timeout int64) error {
	if timeout < 1 || timeout > maxHttpTimeoutMillis {
		return fmt.Errorf("HttpRequest timeout must be between 1 and %d milliseconds", maxHttpTimeoutMillis)
	}
	return nil
}

func httpSetHeader(receiver Value, name string, value Value) {
	headers, ok := receiver.Fields["headers"]
	if !ok || headers.Kind != ValueMap {
		headers = Map()
	}
	headers.Map[mapKey(String(strings.ToLower(name)))] = value
	receiver.Fields["headers"] = headers
}

func httpGetHeader(receiver Value, name string) Value {
	headers, ok := receiver.Fields["headers"]
	if !ok || headers.Kind != ValueMap {
		return Null
	}
	if value, ok := headers.Map[mapKey(String(strings.ToLower(name)))]; ok {
		return value
	}
	return Null
}

func httpHeaderKeys(receiver Value) Value {
	headers, ok := receiver.Fields["headers"]
	if !ok || headers.Kind != ValueMap {
		return List()
	}
	keys := make([]string, 0, len(headers.Map))
	for rawKey := range headers.Map {
		decoded := valueFromMapKey(rawKey)
		if decoded.Kind == ValueString {
			keys = append(keys, decoded.Text)
		}
	}
	sort.Strings(keys)
	out := make([]Value, 0, len(keys))
	for _, key := range keys {
		out = append(out, String(key))
	}
	return List(out...)
}

func (vm *VM) assertMessage(base string, extra []Value, result *Result) (string, error) {
	if len(extra) == 0 {
		return base, nil
	}
	message, err := vm.displayString(extra[0], result)
	if err != nil {
		return "", err
	}
	return base + ": " + message, nil
}

func blobStringArg(name string, args []Value) (string, error) {
	if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
		return "", fmt.Errorf("%s expects Blob", name)
	}
	return args[0].Fields["value"].String(), nil
}

func urlEncodeWithCharset(name, text, charset string) (string, error) {
	switch normalizeURLCharset(charset) {
	case "utf-8":
		return url.QueryEscape(text), nil
	case "us-ascii":
		return urlEncodeASCII(name, text)
	case "iso-8859-1":
		return urlEncodeLatin1(name, text)
	default:
		return "", unsupportedCallError(fmt.Sprintf("%s charset %q", name, charset))
	}
}

func urlDecodeWithCharset(name, text, charset string) (string, error) {
	switch normalizeURLCharset(charset) {
	case "utf-8":
		return url.QueryUnescape(text)
	case "us-ascii":
		return urlDecodeASCII(name, text)
	case "iso-8859-1":
		return urlDecodeLatin1(text)
	default:
		return "", unsupportedCallError(fmt.Sprintf("%s charset %q", name, charset))
	}
}

func normalizeURLCharset(charset string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(charset), "_", "-"))
	switch normalized {
	case "utf-8", "utf8":
		return "utf-8"
	case "us-ascii", "usascii", "ascii":
		return "us-ascii"
	case "iso-8859-1", "iso8859-1", "iso-88591", "iso88591", "latin1", "latin-1":
		return "iso-8859-1"
	default:
		return normalized
	}
}

func urlEncodeASCII(name, text string) (string, error) {
	var out strings.Builder
	for _, r := range text {
		if r > 0x7f {
			return "", fmt.Errorf("%s charset \"US-ASCII\" cannot encode U+%04X", name, r)
		}
		writeURLEncodedByte(&out, byte(r))
	}
	return out.String(), nil
}

func urlEncodeLatin1(name, text string) (string, error) {
	var out strings.Builder
	for _, r := range text {
		if r > 0xff {
			return "", fmt.Errorf("%s charset \"ISO-8859-1\" cannot encode U+%04X", name, r)
		}
		writeURLEncodedByte(&out, byte(r))
	}
	return out.String(), nil
}

func writeURLEncodedByte(out *strings.Builder, b byte) {
	switch {
	case b == ' ':
		out.WriteByte('+')
	case (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == '*':
		out.WriteByte(b)
	default:
		const hexDigits = "0123456789ABCDEF"
		out.WriteByte('%')
		out.WriteByte(hexDigits[b>>4])
		out.WriteByte(hexDigits[b&0x0f])
	}
}

func urlDecodeASCII(name, text string) (string, error) {
	decoded, err := urlDecodeBytes(text)
	if err != nil {
		return "", err
	}
	for _, b := range decoded {
		if b > 0x7f {
			return "", fmt.Errorf("%s charset \"US-ASCII\" cannot decode byte 0x%02X", name, b)
		}
	}
	return string(decoded), nil
}

func urlDecodeLatin1(text string) (string, error) {
	decoded, err := urlDecodeBytes(text)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, b := range decoded {
		out.WriteRune(rune(b))
	}
	return out.String(), nil
}

func urlDecodeBytes(text string) ([]byte, error) {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		ch := text[i]
		switch ch {
		case '+':
			out = append(out, ' ')
		case '%':
			if i+2 >= len(text) {
				return nil, fmt.Errorf("invalid URL escape %q", text[i:])
			}
			hi, ok := fromHex(text[i+1])
			if !ok {
				return nil, fmt.Errorf("invalid URL escape %q", text[i:i+3])
			}
			lo, ok := fromHex(text[i+2])
			if !ok {
				return nil, fmt.Errorf("invalid URL escape %q", text[i:i+3])
			}
			out = append(out, hi<<4|lo)
			i += 2
		default:
			out = append(out, ch)
		}
	}
	return out, nil
}

func fromHex(ch byte) (byte, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0', true
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10, true
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10, true
	default:
		return 0, false
	}
}

func normalizeCryptoAlgorithm(algorithm string) string {
	normalized := strings.ToUpper(strings.TrimSpace(algorithm))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	return normalized
}

func generateDigest(algorithm string, data []byte) ([]byte, error) {
	normalized := normalizeCryptoAlgorithm(algorithm)
	switch normalized {
	case "MD5":
		sum := md5.Sum(data)
		return sum[:], nil
	case "SHA1":
		sum := sha1.Sum(data)
		return sum[:], nil
	case "SHA256":
		sum := sha256.Sum256(data)
		return sum[:], nil
	case "SHA512":
		sum := sha512.Sum512(data)
		return sum[:], nil
	case "SHA3256":
		sum := sha3.Sum256(data)
		return sum[:], nil
	case "SHA3384":
		sum := sha3.Sum384(data)
		return sum[:], nil
	case "SHA3512":
		sum := sha3.Sum512(data)
		return sum[:], nil
	default:
		return nil, fmt.Errorf("unsupported digest algorithm %q", algorithm)
	}
}

func generateMac(algorithm string, input, privateKey []byte) ([]byte, error) {
	normalized := normalizeCryptoAlgorithm(algorithm)
	var mac hash.Hash
	switch normalized {
	case "HMACMD5":
		mac = hmac.New(md5.New, privateKey)
	case "HMACSHA1":
		mac = hmac.New(sha1.New, privateKey)
	case "HMACSHA256":
		mac = hmac.New(sha256.New, privateKey)
	case "HMACSHA512":
		mac = hmac.New(sha512.New, privateKey)
	default:
		return nil, fmt.Errorf("unsupported MAC algorithm %q", algorithm)
	}
	if _, err := mac.Write(input); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

func mathUnary(callee string, args []Value) (Value, error) {
	if len(args) != 1 || (args[0].Kind != ValueInt && args[0].Kind != ValueDecimal) {
		return Null, fmt.Errorf("%s expects numeric argument", callee)
	}
	n := numericFloat(args[0])
	if math.IsInf(n, 0) || math.IsNaN(n) {
		return Null, fmt.Errorf("%s argument must be finite", callee)
	}
	switch callee {
	case "Math.abs":
		if args[0].Kind == ValueInt {
			if args[0].Int == math.MinInt64 {
				return Null, fmt.Errorf("Math.abs integer overflow")
			}
			if args[0].Int < 0 {
				return Int(-args[0].Int), nil
			}
			return args[0], nil
		}
		return Decimal(math.Abs(n)), nil
	case "Math.floor", "Math.ceil", "Math.round":
		switch callee {
		case "Math.floor":
			return Decimal(math.Floor(n)), nil
		case "Math.ceil":
			return Decimal(math.Ceil(n)), nil
		default:
			return Decimal(math.Round(n)), nil
		}
	case "Math.roundToLong":
		rounded, err := int64FromFloat("Math.roundToLong", math.Round(n))
		if err != nil {
			return Null, err
		}
		return Int(rounded), nil
	case "Math.signum":
		switch {
		case n > 0:
			return Int(1), nil
		case n < 0:
			return Int(-1), nil
		default:
			return Int(0), nil
		}
	case "Math.sqrt":
		if n < 0 {
			return Null, fmt.Errorf("Math.sqrt argument out of domain")
		}
		return finiteDecimalResult(callee, math.Sqrt(n))
	case "Math.acos":
		if n < -1 || n > 1 {
			return Null, fmt.Errorf("Math.acos argument out of domain")
		}
		return finiteDecimalResult(callee, math.Acos(n))
	case "Math.asin":
		if n < -1 || n > 1 {
			return Null, fmt.Errorf("Math.asin argument out of domain")
		}
		return finiteDecimalResult(callee, math.Asin(n))
	case "Math.atan":
		return finiteDecimalResult(callee, math.Atan(n))
	case "Math.cos":
		return finiteDecimalResult(callee, math.Cos(n))
	case "Math.sin":
		return finiteDecimalResult(callee, math.Sin(n))
	case "Math.tan":
		return finiteDecimalResult(callee, math.Tan(n))
	case "Math.exp":
		return finiteDecimalResult(callee, math.Exp(n))
	case "Math.log":
		if n <= 0 {
			return Null, fmt.Errorf("Math.log argument out of domain")
		}
		return finiteDecimalResult(callee, math.Log(n))
	case "Math.log10":
		if n <= 0 {
			return Null, fmt.Errorf("Math.log10 argument out of domain")
		}
		return finiteDecimalResult(callee, math.Log10(n))
	default:
		return Null, unsupportedCallError(callee)
	}
}

func mathBinary(callee string, args []Value) (Value, error) {
	if len(args) != 2 || !isMathNumeric(args[0]) || !isMathNumeric(args[1]) {
		return Null, fmt.Errorf("%s expects two numeric arguments", callee)
	}
	left := numericFloat(args[0])
	right := numericFloat(args[1])
	if math.IsInf(left, 0) || math.IsNaN(left) || math.IsInf(right, 0) || math.IsNaN(right) {
		return Null, fmt.Errorf("%s arguments must be finite", callee)
	}
	switch callee {
	case "Math.max":
		if args[0].Kind == ValueInt && args[1].Kind == ValueInt {
			return Int(int64(math.Max(left, right))), nil
		}
		return Decimal(math.Max(left, right)), nil
	case "Math.min":
		if args[0].Kind == ValueInt && args[1].Kind == ValueInt {
			return Int(int64(math.Min(left, right))), nil
		}
		return Decimal(math.Min(left, right)), nil
	case "Math.mod":
		if right == 0 {
			return Null, fmt.Errorf("Math.mod divisor cannot be zero")
		}
		if args[0].Kind == ValueInt && args[1].Kind == ValueInt {
			return Int(args[0].Int % args[1].Int), nil
		}
		return Decimal(math.Mod(left, right)), nil
	case "Math.pow":
		return finiteDecimalResult(callee, math.Pow(left, right))
	case "Math.atan2":
		return finiteDecimalResult(callee, math.Atan2(left, right))
	default:
		return Null, unsupportedCallError(callee)
	}
}

func finiteDecimalResult(callee string, value float64) (Value, error) {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return Null, fmt.Errorf("%s result must be finite", callee)
	}
	return Decimal(value), nil
}

func isMathNumeric(value Value) bool {
	return value.Kind == ValueInt || value.Kind == ValueDecimal
}

func numericFloat(value Value) float64 {
	if value.Kind == ValueInt {
		return float64(value.Int)
	}
	return value.Decimal
}

func jsonSuppressNulls(callee string, args []Value) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueBool {
		return false, fmt.Errorf("%s expects suppressApexObjectNulls Boolean", callee)
	}
	return args[0].Bool, nil
}

func jsonFromValue(value Value, suppressObjectNulls bool) any {
	switch value.Kind {
	case ValueNull:
		return nil
	case ValueInt:
		return value.Int
	case ValueDecimal:
		return value.Decimal
	case ValueBool:
		return value.Bool
	case ValueString:
		return value.Text
	case ValueList:
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			out = append(out, jsonFromValue(item, suppressObjectNulls))
		}
		return out
	case ValueSet:
		out := make([]any, 0, len(value.Set))
		for _, item := range value.Set {
			out = append(out, jsonFromValue(item, suppressObjectNulls))
		}
		return out
	case ValueMap:
		out := make(map[string]any, len(value.Map))
		for key, item := range value.Map {
			out[valueFromMapKey(key).String()] = jsonFromValue(item, suppressObjectNulls)
		}
		return out
	case ValueObject:
		if scalar, ok := jsonPlatformScalarFromValue(value); ok {
			return scalar
		}
		out := make(map[string]any, len(value.Fields)+1)
		if value.Type != "" {
			attributes := map[string]any{"type": value.Type}
			if id, ok := value.Fields["Id"]; ok && id.Kind == ValueString && id.Text != "" && !strings.Contains(value.Type, ".") {
				attributes["url"] = "/services/data/v60.0/sobjects/" + value.Type + "/" + id.Text
			}
			out["attributes"] = attributes
		}
		for field, item := range value.Fields {
			if suppressObjectNulls && item.Kind == ValueNull {
				continue
			}
			out[field] = jsonFromValue(item, suppressObjectNulls)
		}
		return out
	default:
		return nil
	}
}

func decodeJSONValue(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("JSON input contains multiple values")
		}
		return nil, err
	}
	return decoded, nil
}

func decodeJSONValueForDeserialize(text string, strict bool) (any, error) {
	if strict {
		if err := validateJSONNoDuplicateObjectFields(text); err != nil {
			return nil, err
		}
	}
	return decodeJSONValue(text)
}

func validateJSONNoDuplicateObjectFields(text string) error {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	return validateJSONValueNoDuplicateObjectFields(decoder)
}

func validateJSONValueNoDuplicateObjectFields(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON.deserializeStrict expected object field name")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("JSON.deserializeStrict found duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := validateJSONValueNoDuplicateObjectFields(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("JSON.deserializeStrict expected object end")
		}
	case '[':
		for decoder.More() {
			if err := validateJSONValueNoDuplicateObjectFields(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("JSON.deserializeStrict expected array end")
		}
	default:
		return fmt.Errorf("JSON.deserializeStrict found unexpected delimiter %q", delim)
	}
	return nil
}

func valueFromJSON(raw any) Value {
	switch v := raw.(type) {
	case nil:
		return Null
	case bool:
		return Bool(v)
	case float64:
		if math.Trunc(v) == v {
			if converted, err := int64FromFloat("JSON number", v); err == nil {
				return Int(converted)
			}
		}
		return Decimal(v)
	case json.Number:
		text := v.String()
		if !strings.ContainsAny(text, ".eE") {
			if converted, err := strconv.ParseInt(text, 10, 64); err == nil {
				return Int(converted)
			}
		}
		if converted, err := strconv.ParseFloat(text, 64); err == nil {
			return Decimal(converted)
		}
		return String(text)
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

func (vm *VM) typedValueFromJSON(typeName string, raw any, strict bool) (Value, error) {
	if value, ok, err := typedScalarFromJSON(typeName, raw); ok || err != nil {
		return value, err
	}
	if strings.HasPrefix(typeName, "List<") {
		items, ok := raw.([]any)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		elementType, _ := collectionElementType(typeName)
		out := List()
		out.Type = typeName
		for _, item := range items {
			value, err := vm.typedValueFromJSON(elementType, item, strict)
			if err != nil {
				return Null, err
			}
			out.List = append(out.List, value)
		}
		return out, nil
	}
	if strings.HasPrefix(typeName, "Set<") {
		items, ok := raw.([]any)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		elementType, _ := collectionElementType(typeName)
		out := Set()
		out.Type = typeName
		for _, item := range items {
			value, err := vm.typedValueFromJSON(elementType, item, strict)
			if err != nil {
				return Null, err
			}
			if !containsValue(out.Set, value) {
				out.Set = append(out.Set, value)
			}
		}
		return out, nil
	}
	if strings.HasPrefix(typeName, "Map<") {
		fields, ok := raw.(map[string]any)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		if keyType != "String" && keyType != "Object" {
			return Null, jsonDeserializeException("JSON.deserialize supports Map keys only for String/Object targets, got %s", keyType)
		}
		out := Map()
		out.Type = typeName
		for key, item := range fields {
			value, err := vm.typedValueFromJSON(valueType, item, strict)
			if err != nil {
				return Null, err
			}
			out.Map[mapKey(String(key))] = value
		}
		return out, nil
	}
	if typeName == "Object" {
		return valueFromJSON(raw), nil
	}
	if strings.EqualFold(typeName, "sObject") {
		return vm.sObjectValueFromJSON(raw, strict)
	}
	if !vm.isJSONTypedObjectTarget(typeName) {
		return Null, unsupportedCallError("JSON.deserialize local class/SObject mapping for " + typeName)
	}
	obj := Object(typeName)
	vm.initializeFields(&obj, typeName)
	fields, ok := raw.(map[string]any)
	if !ok {
		return Null, jsonTypeMappingError(typeName, raw)
	}
	if strict {
		if !vm.allowOpenSObjectJSONFields(typeName) {
			allowed := vm.jsonAllowedFields(typeName)
			for key := range fields {
				if key == "attributes" {
					continue
				}
				if _, ok := allowed[key]; !ok {
					return Null, newExceptionError("JSONException", fmt.Sprintf("JSON.deserializeStrict found unknown field %q for %s", key, typeName))
				}
			}
		}
	}
	for key, item := range fields {
		if key == "attributes" {
			continue
		}
		if field, _, ok := vm.lookupField(typeName, key); ok && field.Type != "" {
			value, err := vm.typedValueFromJSON(field.Type, item, strict)
			if err != nil {
				return Null, err
			}
			obj.Fields[key] = value
			continue
		}
		if fieldType, ok := vm.jsonSObjectFieldType(typeName, key); ok {
			value, err := vm.typedValueFromJSON(fieldType, item, strict)
			if err != nil {
				return Null, err
			}
			obj.Fields[vm.resolveSObjectFieldName(typeName, key)] = value
			continue
		}
		obj.Fields[key] = valueFromJSON(item)
	}
	return obj, nil
}

func (vm *VM) sObjectValueFromJSON(raw any, strict bool) (Value, error) {
	fields, ok := raw.(map[string]any)
	if !ok {
		return Null, jsonTypeMappingError("sObject", raw)
	}
	typeName := "sObject"
	if attrs, ok := fields["attributes"].(map[string]any); ok {
		if rawType, ok := attrs["type"].(string); ok && strings.TrimSpace(rawType) != "" {
			typeName = strings.TrimSpace(rawType)
		}
	}
	obj := Object(typeName)
	vm.initializeFields(&obj, typeName)
	for key, item := range fields {
		if key == "attributes" {
			continue
		}
		if fieldType, ok := vm.jsonSObjectFieldType(typeName, key); ok {
			value, err := vm.typedValueFromJSON(fieldType, item, strict)
			if err != nil {
				return Null, err
			}
			obj.Fields[vm.resolveSObjectFieldName(typeName, key)] = value
			continue
		}
		obj.Fields[key] = valueFromJSON(item)
	}
	return obj, nil
}

func (vm *VM) jsonSObjectFieldType(typeName, fieldName string) (string, bool) {
	if vm.Org == nil {
		return "", false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, typeName)
	if !ok {
		return "", false
	}
	fieldName = vm.resolveSObjectFieldName(typeName, fieldName)
	field, ok := vm.Org.Objects[objectName].Definition.Fields[fieldName]
	if !ok {
		return "", false
	}
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		return "Id", true
	case storage.FieldString, storage.FieldPicklist:
		return "String", true
	case storage.FieldBoolean:
		return "Boolean", true
	case storage.FieldInteger:
		return "Integer", true
	case storage.FieldDecimal:
		return "Decimal", true
	case storage.FieldDate:
		return "Date", true
	case storage.FieldDateTime:
		return "Datetime", true
	default:
		return "", false
	}
}

func (vm *VM) isJSONTypedObjectTarget(typeName string) bool {
	if _, ok := vm.Classes[typeName]; ok {
		return true
	}
	if vm.isSObjectLikeType(typeName) {
		return true
	}
	if vm.Org != nil {
		if _, ok := vm.Org.Objects[typeName]; ok {
			return true
		}
	}
	return false
}

func typedScalarFromJSON(typeName string, raw any) (Value, bool, error) {
	canonical := canonicalJSONScalarType(typeName)
	if raw == nil {
		return Null, true, nil
	}
	switch canonical {
	case "String":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return String(text), true, nil
	case "Boolean":
		value, ok := raw.(bool)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return Bool(value), true, nil
	case "Integer", "Long":
		value, ok := jsonIntegralNumber(raw)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return Int(value), true, nil
	case "Decimal", "Double":
		value, ok := jsonDecimalNumber(raw)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return Decimal(value), true, nil
	case "Date":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return Null, true, jsonDeserializeException("JSON.deserialize cannot parse Date %q", text)
		}
		return platformScalar("Date", text), true, nil
	case "Datetime":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		value, err := parseDatetimeText(text)
		if err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return platformScalar("Datetime", value.UTC().Format(time.RFC3339)), true, nil
	case "Time":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		value, err := parseTimeText(text)
		if err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return platformScalar("Time", value), true, nil
	case "Id":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		if err := validateApexID(text); err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return platformScalar("Id", text), true, nil
	case "Blob":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return Null, true, jsonDeserializeException("JSON.deserialize cannot decode Blob base64: %v", err)
		}
		return platformScalar("Blob", string(decoded)), true, nil
	}
	return Null, false, nil
}

func canonicalJSONScalarType(typeName string) string {
	switch {
	case strings.EqualFold(typeName, "String"):
		return "String"
	case strings.EqualFold(typeName, "Boolean"):
		return "Boolean"
	case strings.EqualFold(typeName, "Integer"):
		return "Integer"
	case strings.EqualFold(typeName, "Long"):
		return "Long"
	case strings.EqualFold(typeName, "Decimal"):
		return "Decimal"
	case strings.EqualFold(typeName, "Double"):
		return "Double"
	case strings.EqualFold(typeName, "Date"):
		return "Date"
	case strings.EqualFold(typeName, "Datetime") || strings.EqualFold(typeName, "DateTime"):
		return "Datetime"
	case strings.EqualFold(typeName, "Time"):
		return "Time"
	case strings.EqualFold(typeName, "Id"):
		return "Id"
	case strings.EqualFold(typeName, "Blob"):
		return "Blob"
	default:
		return typeName
	}
}

func jsonIntegralNumber(raw any) (int64, bool) {
	switch value := raw.(type) {
	case json.Number:
		text := value.String()
		if strings.ContainsAny(text, ".eE") {
			decimal, err := strconv.ParseFloat(text, 64)
			if err != nil || math.Trunc(decimal) != decimal {
				return 0, false
			}
			converted, err := int64FromFloat("JSON number", decimal)
			return converted, err == nil
		}
		converted, err := strconv.ParseInt(text, 10, 64)
		return converted, err == nil
	case float64:
		if math.Trunc(value) != value {
			return 0, false
		}
		converted, err := int64FromFloat("JSON number", value)
		return converted, err == nil
	default:
		return 0, false
	}
}

func jsonDecimalNumber(raw any) (float64, bool) {
	switch value := raw.(type) {
	case json.Number:
		converted, err := strconv.ParseFloat(value.String(), 64)
		return converted, err == nil
	case float64:
		return value, true
	default:
		return 0, false
	}
}

func jsonTypeMappingError(typeName string, raw any) error {
	return jsonDeserializeException("JSON.deserialize cannot map JSON %s to %s", jsonRawKind(raw), typeName)
}

func jsonDeserializeException(format string, args ...any) error {
	return newExceptionError("JSONException", fmt.Sprintf(format, args...))
}

func jsonRawKind(raw any) string {
	switch raw.(type) {
	case nil:
		return "null"
	case bool:
		return "Boolean"
	case json.Number, float64:
		return "number"
	case string:
		return "String"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", raw)
	}
}

func (vm *VM) jsonAllowedFields(typeName string) map[string]struct{} {
	allowed := map[string]struct{}{
		"Id": struct{}{},
	}
	if vm.Org != nil {
		if object, ok := vm.Org.Objects[typeName]; ok {
			for name := range object.Definition.Fields {
				allowed[name] = struct{}{}
			}
		}
	}
	for className := typeName; className != ""; {
		class, ok := vm.Classes[className]
		if !ok {
			break
		}
		for name := range class.Fields {
			allowed[name] = struct{}{}
		}
		className = class.SuperClass
	}
	return allowed
}

func (vm *VM) allowOpenSObjectJSONFields(typeName string) bool {
	if !vm.isSObjectLikeType(typeName) {
		return false
	}
	if _, ok := vm.Classes[typeName]; ok {
		return false
	}
	if vm.Org == nil {
		return true
	}
	_, ok := storage.ResolveObjectName(*vm.Org, typeName)
	return !ok
}

func (vm *VM) schemaGlobalDescribe() Value {
	out := Map()
	if vm.Org == nil {
		return out
	}
	names := make([]string, 0, len(vm.Org.Objects))
	for name := range vm.Org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out.Map[mapKey(String(name))] = sObjectTypeToken(name)
	}
	return out
}

func (vm *VM) schemaDescribeObjectName(value Value) (string, error) {
	if value.Kind == ValueString {
		return value.Text, nil
	}
	if value.Kind == ValueObject && value.Type == "Schema.SObjectType" {
		objectValue, ok := value.Fields["object"]
		if !ok || objectValue.Kind != ValueString {
			return "", fmt.Errorf("Schema.SObjectType token missing object")
		}
		return objectValue.Text, nil
	}
	return "", fmt.Errorf("Schema.describeSObjects expects object names or SObjectType tokens")
}

func (vm *VM) describeSObjectValue(name string, definition storage.ObjectDefinition) Value {
	desc := Object("Schema.DescribeSObjectResult")
	desc.Fields["name"] = String(name)
	desc.Fields["label"] = String(definition.Label)
	desc.Fields["labelPlural"] = String(definition.PluralLabel)
	desc.Fields["keyPrefix"] = String(definition.KeyPrefix)
	fieldsMap := Map()
	fieldNames := make([]string, 0, len(definition.Fields))
	for fieldName := range definition.Fields {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)
	for _, fieldName := range fieldNames {
		field := definition.Fields[fieldName]
		fieldsMap.Map[mapKey(String(field.APIName))] = sObjectFieldToken(name, field.APIName)
	}
	fields := Object("Schema.SObjectFieldMap")
	fields.Fields["map"] = fieldsMap
	desc.Fields["fields"] = fields
	desc.Fields["fieldSets"] = Object("Schema.FieldSetMapUnsupported")
	childRelationships := make([]Value, 0)
	if vm.Org != nil {
		childObjects := make([]string, 0, len(vm.Org.Objects))
		for childName := range vm.Org.Objects {
			childObjects = append(childObjects, childName)
		}
		sort.Strings(childObjects)
		for _, childName := range childObjects {
			childDefinition := vm.Org.Objects[childName].Definition
			relationships := append([]storage.Relationship(nil), childDefinition.Relations...)
			sort.Slice(relationships, func(i, j int) bool {
				if relationships[i].ChildRelationship == relationships[j].ChildRelationship {
					return relationships[i].Field < relationships[j].Field
				}
				return relationships[i].ChildRelationship < relationships[j].ChildRelationship
			})
			for _, relationship := range relationships {
				if relationshipTargetsObject(relationship, name) {
					childRelationships = append(childRelationships, childRelationshipValue(childName, relationship))
				}
			}
		}
	}
	desc.Fields["childRelationships"] = List(childRelationships...)
	recordTypes := make([]Value, 0, len(definition.RecordTypes))
	byName := Map()
	byDeveloperName := Map()
	for _, recordType := range definition.RecordTypes {
		value := recordTypeInfoValue(recordType)
		recordTypes = append(recordTypes, value)
		if recordType.Name != "" {
			byName.Map[mapKey(String(recordType.Name))] = value
		}
		if recordType.DeveloperName != "" {
			byDeveloperName.Map[mapKey(String(recordType.DeveloperName))] = value
		}
	}
	desc.Fields["recordTypeInfos"] = List(recordTypes...)
	desc.Fields["recordTypeInfosByName"] = byName
	desc.Fields["recordTypeInfosByDeveloperName"] = byDeveloperName
	return desc
}

func relationshipTargetsObject(relationship storage.Relationship, objectName string) bool {
	for _, parent := range relationship.ParentObjects {
		if strings.EqualFold(parent, objectName) {
			return true
		}
	}
	return false
}

func childRelationshipValue(childObject string, relationship storage.Relationship) Value {
	value := Object("Schema.ChildRelationship")
	value.Fields["relationshipName"] = String(relationship.ChildRelationship)
	value.Fields["field"] = sObjectFieldToken(childObject, relationship.Field)
	value.Fields["childSObject"] = sObjectTypeToken(childObject)
	value.Fields["cascadeDelete"] = Bool(relationship.CascadeDelete)
	value.Fields["restrictedDelete"] = Bool(relationship.RestrictedDelete)
	return value
}

func recordTypeInfoValue(recordType storage.RecordTypeInfo) Value {
	value := Object("Schema.RecordTypeInfo")
	value.Fields["recordTypeId"] = String(recordType.ID.String())
	value.Fields["developerName"] = String(recordType.DeveloperName)
	value.Fields["name"] = String(recordType.Name)
	value.Fields["active"] = Bool(recordType.Active)
	value.Fields["available"] = Bool(recordType.Available)
	value.Fields["default"] = Bool(recordType.Default)
	return value
}

func (vm *VM) describeFieldValue(objectName, fieldName string) (Value, error) {
	if vm.Org == nil {
		return Null, fmt.Errorf("Schema field describe requires org state")
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, objectName)
	if !ok {
		return Null, fmt.Errorf("Schema field describe unknown object %s", objectName)
	}
	definition := vm.Org.Objects[objectName].Definition
	fieldName, ok = storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return Null, fmt.Errorf("Schema field describe unknown field %s.%s", objectName, fieldName)
	}
	field := definition.Fields[fieldName]
	desc := Object("Schema.DescribeFieldResult")
	desc.Fields["name"] = String(field.APIName)
	label := field.Label
	if label == "" {
		label = field.APIName
	}
	desc.Fields["label"] = String(label)
	desc.Fields["type"] = String(string(field.Type))
	desc.Fields["nillable"] = Bool(!field.Required)
	desc.Fields["externalId"] = Bool(field.ExternalID)
	desc.Fields["unique"] = Bool(field.Unique)
	if field.RelationshipName == "" {
		desc.Fields["relationshipName"] = Null
	} else {
		desc.Fields["relationshipName"] = String(field.RelationshipName)
	}
	references := make([]Value, 0, len(field.ReferenceTo))
	for _, target := range field.ReferenceTo {
		references = append(references, sObjectTypeToken(target))
	}
	desc.Fields["referenceTo"] = List(references...)
	picklistValues := make([]Value, 0, len(field.PicklistValues))
	for _, value := range field.PicklistValues {
		entry := Object("Schema.PicklistEntry")
		entry.Fields["value"] = String(value.Value)
		label := value.Label
		if label == "" {
			label = value.Value
		}
		entry.Fields["label"] = String(label)
		entry.Fields["default"] = Bool(value.Default)
		entry.Fields["active"] = Bool(value.Active)
		picklistValues = append(picklistValues, entry)
	}
	desc.Fields["picklistValues"] = List(picklistValues...)
	return desc, nil
}

func approxValueSize(value Value) int {
	switch value.Kind {
	case ValueNull:
		return 4
	case ValueInt, ValueDecimal, ValueBool:
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
		if value.Kind == ValueNull && value.Type == "" {
			value.Type = vm.VarTypes[name]
		}
		return value, nil
	}
	if value, ok := vm.triggerGlobals[name]; ok {
		return value, nil
	}
	if actual, ok := vm.lookupGlobalName(name); ok {
		value := vm.Globals[actual]
		if value.Kind == ValueNull && value.Type == "" {
			value.Type = vm.VarTypes[actual]
		}
		return value, nil
	}
	if value, ok, err := vm.lookupRestContextField(name); ok || err != nil {
		return value, err
	}
	switch name {
	case "AccessLevel.USER_MODE", "AccessLevel.SYSTEM_MODE":
		return Value{Kind: ValueObject, Type: "AccessLevel", Text: strings.TrimPrefix(name, "AccessLevel.")}, nil
	}
	if strings.HasPrefix(name, "RoundingMode.") {
		mode := strings.TrimPrefix(name, "RoundingMode.")
		if isDecimalRoundingModeName(mode) {
			return Value{Kind: ValueObject, Type: "RoundingMode", Text: mode}, nil
		}
	}
	if strings.HasPrefix(name, "LoggingLevel.") {
		level := strings.TrimPrefix(name, "LoggingLevel.")
		if isLoggingLevelName(level) {
			return Value{Kind: ValueObject, Type: "LoggingLevel", Text: level}, nil
		}
	}
	if strings.HasPrefix(name, "Label.") {
		label := strings.TrimPrefix(name, "Label.")
		if label != "" && !strings.Contains(label, ".") {
			return String(label), nil
		}
	}
	if strings.HasPrefix(name, "JSONToken.") {
		tokenName := strings.TrimPrefix(name, "JSONToken.")
		for _, jsonTokenName := range jsonTokenNames {
			if tokenName == jsonTokenName {
				return jsonTokenValue(tokenName), nil
			}
		}
	}
	if value, ok := apexPagesSeverityStaticValue(name); ok {
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
		if actual, ok := vm.lookupGlobalName(parts[0]); ok {
			return vm.lookupPath(vm.Globals[actual], parts[1:])
		}
		if vm.currentClass != "" {
			if field, owner, ok := vm.lookupStaticField(vm.currentClass, parts[0]); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+parts[0]); err != nil {
					return Null, err
				}
				root := field.Value
				if field.Getter != nil {
					if field.Getter.Name == vm.currentMethod.Name {
						root = field.Value
					} else {
						var err error
						root, err = vm.callMethod(*field.Getter, nil, resultForLookup())
						if err != nil {
							return Null, err
						}
					}
				}
				return vm.lookupPath(root, parts[1:])
			}
		}
		if token, ok := vm.lookupSObjectTypeToken(parts); ok {
			return token, nil
		}
		if token, ok := vm.lookupSObjectFieldToken(parts); ok {
			return token, nil
		}
		if len(parts) == 2 {
			if value, ok := builtinStaticField(parts[0], parts[1]); ok {
				return value, nil
			}
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			if value, ok := builtinStaticField(className, memberName); ok {
				return value, nil
			}
			if field, owner, ok := vm.lookupStaticField(className, memberName); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+memberName); err != nil {
					return Null, err
				}
				if field.Getter != nil {
					if field.Getter.Name == vm.currentMethod.Name {
						return field.Value, nil
					}
					return vm.callMethod(*field.Getter, nil, resultForLookup())
				}
				return field.Value, nil
			}
			if class, ok := vm.Classes[className]; ok {
				for _, enumValue := range class.EnumValues {
					if enumValue == memberName {
						return Value{Kind: ValueObject, Type: class.Name, Text: memberName}, nil
					}
				}
			}
			if dot := strings.IndexByte(memberName, '.'); dot > 0 {
				nestedEnumName := className + "." + memberName[:dot]
				nestedMemberName := memberName[dot+1:]
				nestedCandidates := []string{nestedEnumName, memberName[:dot]}
				for _, candidate := range nestedCandidates {
					if class, ok := vm.Classes[candidate]; ok {
						for _, enumValue := range class.EnumValues {
							if enumValue == nestedMemberName {
								return Value{Kind: ValueObject, Type: class.Name, Text: nestedMemberName}, nil
							}
						}
					}
				}
				if _, ok := vm.Classes[className]; ok && apexIdentifierStartsUpper(memberName[:dot]) && apexIdentifierStartsUpper(nestedMemberName) {
					return Value{Kind: ValueObject, Type: nestedEnumName, Text: nestedMemberName}, nil
				}
			}
		}
	}
	if len(parts) == 3 && apexIdentifierStartsUpper(parts[0]) && apexIdentifierStartsUpper(parts[1]) && apexIdentifierStartsUpper(parts[2]) {
		return Value{Kind: ValueObject, Type: parts[0] + "." + parts[1], Text: parts[2]}, nil
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if value, ok := this.Fields[name]; ok {
			if field, owner, ok := vm.lookupField(this.Type, name); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name); err != nil {
					return Null, err
				}
				if field.Getter != nil {
					if field.Getter.Name == vm.currentMethod.Name {
						return value, nil
					}
					return vm.callMethodWithReceiver(*field.Getter, this, nil, resultForLookup())
				}
			}
			return value, nil
		}
	}
	if vm.currentClass != "" {
		if field, owner, ok := vm.lookupStaticField(vm.currentClass, name); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name); err != nil {
				return Null, err
			}
			if field.Getter != nil {
				if field.Getter.Name == vm.currentMethod.Name {
					return field.Value, nil
				}
				return vm.callMethod(*field.Getter, nil, resultForLookup())
			}
			return field.Value, nil
		}
	}
	return Null, fmt.Errorf("unknown variable %q", name)
}

func (vm *VM) lookupGlobalName(name string) (string, bool) {
	if _, ok := vm.Globals[name]; ok {
		return name, true
	}
	normalized := strings.ToLower(name)
	for candidate := range vm.Globals {
		if strings.ToLower(candidate) == normalized {
			return candidate, true
		}
	}
	return "", false
}

var roundingModeNames = []string{"UP", "DOWN", "CEILING", "FLOOR", "HALF_UP", "HALF_DOWN", "HALF_EVEN", "UNNECESSARY"}

func isDecimalRoundingModeName(name string) bool {
	for _, candidate := range roundingModeNames {
		if name == candidate {
			return true
		}
	}
	return false
}

func (vm *VM) lookupSObjectTypeToken(parts []string) (Value, bool) {
	if vm.Org == nil {
		return Null, false
	}
	var objectName string
	switch {
	case len(parts) == 2 && strings.EqualFold(parts[1], "SObjectType"):
		objectName = parts[0]
	case len(parts) == 3 && strings.EqualFold(parts[0], "Schema") && strings.EqualFold(parts[1], "SObjectType"):
		objectName = parts[2]
	default:
		return Null, false
	}
	canonical, ok := storage.ResolveObjectName(*vm.Org, objectName)
	if !ok {
		if strings.HasSuffix(objectName, "__c") || strings.HasSuffix(objectName, "__e") || strings.HasSuffix(objectName, "__mdt") {
			canonical = objectName
		} else {
			return Null, false
		}
	}
	return sObjectTypeToken(canonical), true
}

func (vm *VM) lookupSObjectFieldToken(parts []string) (Value, bool) {
	if vm.Org == nil || len(parts) < 2 {
		return Null, false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, parts[0])
	if !ok {
		return Null, false
	}
	fieldName := ""
	switch {
	case len(parts) == 2:
		fieldName = parts[1]
	case len(parts) == 3 && strings.EqualFold(parts[1], "Fields"):
		fieldName = parts[2]
	default:
		return Null, false
	}
	definition := vm.Org.Objects[objectName].Definition
	canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return Null, false
	}
	return sObjectFieldToken(objectName, canonical), true
}

func (vm *VM) callSchemaSObjectTypePath(callee string, args []Value, result *Result) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) < 4 || !strings.EqualFold(parts[len(parts)-2], "fields") || !strings.EqualFold(parts[len(parts)-1], "getMap") {
		return Null, false, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
	}
	tokenParts := parts[:len(parts)-2]
	token, ok := vm.lookupSObjectTypeToken(tokenParts)
	if !ok {
		return Null, false, nil
	}
	describe, updated, mutated, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
	if err != nil || !handled {
		return describe, true, err
	}
	_ = updated
	_ = mutated
	fields, ok := describe.Fields["fields"]
	if !ok {
		return Null, true, fmt.Errorf("%s describe fields are not available", callee)
	}
	value, _, _, _, err := vm.callPlatformObjectMember(fields, "getMap", nil, result)
	return value, true, err
}

func (vm *VM) callDottedReceiverMember(callee string, args []Value, result *Result) (Value, bool, error) {
	dot := strings.LastIndex(callee, ".")
	if dot <= 0 || dot >= len(callee)-1 {
		return Null, false, nil
	}
	receiverName := callee[:dot]
	if !strings.Contains(receiverName, ".") {
		return Null, false, nil
	}
	method := callee[dot+1:]
	receiver, err := vm.lookup(receiverName)
	if err != nil {
		return Null, false, nil
	}
	return vm.callValueMember(receiverName, receiver, method, args, result)
}

func sObjectTypeToken(objectName string) Value {
	token := Object("Schema.SObjectType")
	token.Fields["object"] = String(objectName)
	return token
}

func sObjectFieldToken(objectName, fieldName string) Value {
	token := Object("Schema.SObjectField")
	token.Fields["object"] = String(objectName)
	token.Fields["field"] = String(fieldName)
	return token
}

func (vm *VM) lookupPath(root Value, parts []string) (Value, error) {
	current := root
	for _, part := range parts {
		if current.Kind == ValueNull {
			return Null, newExceptionError("NullPointerException", "Attempt to de-reference a null object")
		}
		if current.Kind != ValueObject {
			if current.Kind == ValueMap {
				switch part {
				case "values":
					out := List()
					for _, key := range sortedMapKeys(current.Map) {
						out.List = append(out.List, current.Map[key])
					}
					if _, valueType, ok := mapTypeArgs(current.Type); ok {
						out.Type = "List<" + valueType + ">"
					}
					current = out
					continue
				case "keySet":
					out := Set()
					for _, rawKey := range sortedMapKeys(current.Map) {
						out.Set = append(out.Set, valueFromMapKey(rawKey))
					}
					if keyType, _, ok := mapTypeArgs(current.Type); ok {
						out.Type = "Set<" + keyType + ">"
					}
					current = out
					continue
				}
			}
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
		canonicalPart := vm.resolveSObjectFieldName(current.Type, part)
		value, ok := current.Fields[canonicalPart]
		if !ok && canonicalPart != part {
			value, ok = current.Fields[part]
		}
		if !ok {
			if vm.hasSObjectField(current.Type, canonicalPart) {
				current = Null
				continue
			}
			if strings.HasSuffix(current.Type, "__c") || strings.HasSuffix(current.Type, "__r") {
				current = Null
				continue
			}
			return Null, fmt.Errorf("unknown field %q on %s", part, current.Type)
		}
		current = value
	}
	return current, nil
}

func (vm *VM) assign(name string, value Value) error {
	if actual, ok := vm.lookupGlobalName(name); ok {
		if typeName := vm.VarTypes[actual]; typeName != "" {
			coerced, err := vm.coerceAssignable(typeName, value)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			value = coerced
		}
		vm.Globals[actual] = value
		return nil
	}
	if ok, err := vm.assignRestContextField(name, value); ok || err != nil {
		return err
	}
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		if rootName, ok := vm.lookupGlobalName(parts[0]); ok {
			return vm.assignPath(vm.Globals[rootName], parts[1:], value)
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			if field, owner, ok := vm.lookupStaticField(className, memberName); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+memberName); err != nil {
					return err
				}
				coerced, err := vm.coerceAssignable(field.Type, value)
				if err != nil {
					return fmt.Errorf("%s.%s: %w", owner, memberName, err)
				}
				value = coerced
				if field.Setter != nil {
					key := owner + "." + memberName
					if vm.activeSetters[key] > 0 {
						field.Value = value
						class := vm.Classes[owner]
						class.StaticFields[memberName] = field
						vm.Classes[owner] = class
						vm.storeClassAliases(class)
						return nil
					}
					vm.activeSetters[key]++
					defer func() {
						vm.activeSetters[key]--
						if vm.activeSetters[key] == 0 {
							delete(vm.activeSetters, key)
						}
					}()
					_, err := vm.callMethod(*field.Setter, []Value{value}, resultForLookup())
					return err
				}
				field.Value = value
				class := vm.Classes[owner]
				class.StaticFields[memberName] = field
				vm.Classes[owner] = class
				vm.storeClassAliases(class)
				return nil
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
				coerced, err := vm.coerceAssignable(def.Type, value)
				if err != nil {
					return fmt.Errorf("%s.%s: %w", this.Type, name, err)
				}
				value = coerced
				if def.Setter != nil {
					key := class.Name + "." + name
					if vm.activeSetters[key] > 0 {
						this.Fields[name] = value
						return nil
					}
					vm.activeSetters[key]++
					defer func() {
						vm.activeSetters[key]--
						if vm.activeSetters[key] == 0 {
							delete(vm.activeSetters, key)
						}
					}()
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
		if field, owner, ok := vm.lookupStaticField(vm.currentClass, name); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name); err != nil {
				return err
			}
			coerced, err := vm.coerceAssignable(field.Type, value)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", owner, name, err)
			}
			value = coerced
			if field.Setter != nil {
				key := owner + "." + name
				if vm.activeSetters[key] > 0 {
					field.Value = value
					class := vm.Classes[owner]
					class.StaticFields[name] = field
					vm.Classes[owner] = class
					vm.storeClassAliases(class)
					return nil
				}
				vm.activeSetters[key]++
				defer func() {
					vm.activeSetters[key]--
					if vm.activeSetters[key] == 0 {
						delete(vm.activeSetters, key)
					}
				}()
				_, err := vm.callMethod(*field.Setter, []Value{value}, resultForLookup())
				return err
			}
			field.Value = value
			class := vm.Classes[owner]
			class.StaticFields[name] = field
			vm.Classes[owner] = class
			vm.storeClassAliases(class)
			return nil
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
		next, ok := current.Fields[vm.resolveSObjectFieldName(current.Type, part)]
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
	if reason, ok := sobjectReadOnlyReason(current); ok {
		return fmt.Errorf("cannot modify read-only %s", reason)
	}
	class := vm.Classes[current.Type]
	if def, ok := class.Fields[fieldName]; ok {
		if err := vm.checkMemberAccess(class.Name, def.Access, class.Name+"."+fieldName); err != nil {
			return err
		}
		coerced, err := vm.coerceAssignable(def.Type, value)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", current.Type, fieldName, err)
		}
		value = coerced
		if def.Setter != nil {
			_, err := vm.callMethodWithReceiver(*def.Setter, current, []Value{value}, resultForLookup())
			return err
		}
	}
	current.Fields[vm.resolveSObjectFieldName(current.Type, fieldName)] = value
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

func (vm *VM) lookupStaticField(typeName, fieldName string) (Field, string, bool) {
	for search := typeName; search != ""; {
		for current := search; current != ""; {
			class, ok := vm.Classes[current]
			if !ok {
				break
			}
			if field, ok := class.StaticFields[fieldName]; ok {
				return field, class.Name, true
			}
			current = class.SuperClass
		}
		dot := strings.LastIndex(search, ".")
		if dot < 0 {
			break
		}
		search = search[:dot]
	}
	return Field{}, "", false
}

func builtinStaticField(typeName, fieldName string) (Value, bool) {
	switch typeName {
	case "Math":
		switch fieldName {
		case "E":
			return Decimal(math.E), true
		case "PI":
			return Decimal(math.Pi), true
		}
	case "Integer":
		switch fieldName {
		case "MAX_VALUE":
			return Int(math.MaxInt32), true
		case "MIN_VALUE":
			return Int(math.MinInt32), true
		}
	case "Long":
		switch fieldName {
		case "MAX_VALUE":
			return Int(math.MaxInt64), true
		case "MIN_VALUE":
			return Int(math.MinInt64), true
		}
	case "Pattern":
		switch fieldName {
		case "UNIX_LINES":
			return Int(patternFlagUnixLines), true
		case "CASE_INSENSITIVE":
			return Int(patternFlagCaseInsensitive), true
		case "COMMENTS":
			return Int(patternFlagComments), true
		case "MULTILINE":
			return Int(patternFlagMultiline), true
		case "LITERAL":
			return Int(patternFlagLiteral), true
		case "DOTALL":
			return Int(patternFlagDotall), true
		case "UNICODE_CASE":
			return Int(patternFlagUnicodeCase), true
		case "CANON_EQ":
			return Int(patternFlagCanonEq), true
		case "UNICODE_CHARACTER_CLASS":
			return Int(patternFlagUnicodeCharacterClass), true
		}
	}
	return Null, false
}

func (vm *VM) checkMemberAccess(ownerClass, access, member string, modifierSets ...[]string) error {
	if err := vm.checkClassAccess(ownerClass, member); err != nil {
		return err
	}
	if err := vm.checkNamespaceAccess(ownerClass, access, member); err != nil {
		return err
	}
	switch strings.ToLower(access) {
	case "", "public", "global", "webservice":
		return nil
	case "private":
		if vm.currentClassIsTest() && hasAnyMethodModifier(modifierSets, "testvisible") {
			return nil
		}
		if vm.currentClass == ownerClass || strings.HasPrefix(vm.currentClass, ownerClass+".") {
			return nil
		}
	case "protected":
		if vm.currentClassIsTest() && hasAnyMethodModifier(modifierSets, "testvisible") {
			return nil
		}
		if vm.currentClass == ownerClass || vm.isSubclass(vm.currentClass, ownerClass) || vm.isSubclass(ownerClass, vm.currentClass) {
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

func (vm *VM) checkClassAccess(ownerClass, member string) error {
	class, ok := vm.Classes[ownerClass]
	if !ok {
		if resolved, found := vm.resolveClassName(ownerClass); found {
			class, ok = vm.Classes[resolved]
		}
	}
	if !ok || class.Namespace == "" {
		return nil
	}
	callerNS := vm.classNamespace(vm.currentClass)
	if callerNS == class.Namespace {
		return nil
	}
	switch strings.ToLower(class.Access) {
	case "global", "webservice":
		return nil
	}
	if vm.currentClass == "" {
		return fmt.Errorf("%s is not global and not visible outside namespace %s", member, class.Namespace)
	}
	return fmt.Errorf("%s is not global and not visible from namespace %s", member, callerNS)
}

func (vm *VM) currentClassIsTest() bool {
	class, ok := vm.Classes[vm.currentClass]
	return ok && class.IsTest
}

func hasAnyMethodModifier(modifierSets [][]string, expected string) bool {
	for _, modifiers := range modifierSets {
		for _, modifier := range modifiers {
			if strings.EqualFold(strings.TrimPrefix(modifier, "@"), expected) {
				return true
			}
		}
	}
	return false
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
		for _, triggers := range vm.Triggers {
			for _, trigger := range triggers {
				if trigger.Name == className {
					return trigger.Namespace
				}
			}
		}
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

func apexIdentifierStartsUpper(name string) bool {
	if name == "" {
		return false
	}
	first := name[0]
	return first >= 'A' && first <= 'Z'
}

func (vm *VM) typeForName(namespace, name string) Value {
	if strings.TrimSpace(name) == "" {
		return Null
	}
	if strings.HasPrefix(name, "System.") {
		systemName := strings.TrimPrefix(name, "System.")
		if isBuiltinTypeName(systemName) {
			return platformScalar("Type", "System."+systemName)
		}
	}
	if strings.TrimSpace(namespace) == "System" {
		systemName := strings.TrimPrefix(name, "System.")
		if isBuiltinTypeName(systemName) {
			return platformScalar("Type", "System."+systemName)
		}
		return Null
	}
	if namespace != "" {
		return platformScalar("Type", namespace+"."+name)
	}
	if resolved, ok := vm.resolveClassName(name); ok {
		return platformScalar("Type", resolved)
	}
	if vm.Org != nil {
		if canonical, ok := storage.ResolveObjectName(*vm.Org, name); ok {
			return platformScalar("Type", canonical)
		}
	}
	if isBuiltinTypeName(name) || isGenericTypeName(name) || isCommonSObjectTypeName(name) {
		return platformScalar("Type", name)
	}
	return Null
}

func isBuiltinTypeName(name string) bool {
	if isBuiltinExceptionType(exceptionTypeName(name)) {
		return true
	}
	switch name {
	case "Object", "String", "Boolean", "Integer", "Long", "Decimal", "Double", "Date", "Datetime", "Time", "TimeZone", "Blob", "Id", "Type", "URL", "PageReference", "LoggingLevel", "ApexPages.Severity", "RestContext", "RestRequest", "RestResponse", "Callable", "StubProvider":
		return true
	default:
		return false
	}
}

func isGenericTypeName(name string) bool {
	open := strings.IndexByte(name, '<')
	if open <= 0 || !strings.HasSuffix(name, ">") {
		return false
	}
	base := name[:open]
	args, ok := genericTypeArgs(name)
	if !ok {
		return false
	}
	switch base {
	case "List", "Set":
		return len(args) == 1 && isTypeNameToken(args[0])
	case "Map":
		return len(args) == 2 && isTypeNameToken(args[0]) && isTypeNameToken(args[1])
	default:
		return false
	}
}

func isTypeNameToken(name string) bool {
	return isBuiltinTypeName(name) || isGenericTypeName(name) || isCommonSObjectTypeName(name)
}

func isCommonSObjectTypeName(name string) bool {
	for _, objectName := range standardSObjectPrefixes {
		if name == objectName {
			return true
		}
	}
	return false
}

func (vm *VM) resolveClassName(typeName string) (string, bool) {
	if class, ok := vm.Classes[typeName]; ok {
		return class.Name, true
	}
	if !strings.Contains(typeName, ".") && vm.currentClass != "" {
		for owner := vm.currentClass; owner != ""; {
			candidate := owner + "." + typeName
			if class, ok := vm.Classes[candidate]; ok {
				return class.Name, true
			}
			dot := strings.LastIndex(owner, ".")
			if dot < 0 {
				break
			}
			owner = owner[:dot]
		}
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

func newRestRequest() Value {
	request := Object("RestRequest")
	request.Fields["requestURI"] = String("")
	request.Fields["resourcePath"] = String("")
	request.Fields["httpMethod"] = String("")
	request.Fields["remoteAddress"] = String("")
	request.Fields["headers"] = typedMap("Map<String,String>")
	request.Fields["params"] = typedMap("Map<String,String>")
	request.Fields["requestBody"] = Null
	return request
}

func newPageReference(rawURL string) Value {
	page := Object("PageReference")
	page.Fields["url"] = String(rawURL)
	page.Fields["parameters"] = typedMap("Map<String,String>")
	page.Fields["headers"] = typedMap("Map<String,String>")
	return page
}

func newHttpRequest() Value {
	request := Object("HttpRequest")
	request.Fields["endpoint"] = String("")
	request.Fields["method"] = String("")
	request.Fields["headers"] = typedMap("Map<String,String>")
	request.Fields["body"] = String("")
	request.Fields["compressed"] = Bool(false)
	request.Fields["timeout"] = Int(defaultHttpTimeoutMillis)
	return request
}

func newHttpResponse() Value {
	response := Object("HttpResponse")
	response.Fields["statusCode"] = Int(200)
	response.Fields["status"] = String("OK")
	response.Fields["headers"] = typedMap("Map<String,String>")
	response.Fields["body"] = String("")
	return response
}

func newSendEmailResult() Value {
	result := Object("Messaging.SendEmailResult")
	result.Fields["success"] = Bool(true)
	result.Fields["errors"] = List()
	return result
}

func newSingleEmailMessage() Value {
	message := Object("Messaging.SingleEmailMessage")
	for _, field := range []string{
		"toAddresses", "ccAddresses", "bccAddresses", "fileAttachments",
		"entityAttachments", "documentAttachments", "targetObjectIds",
	} {
		message.Fields[field] = List()
	}
	for _, field := range []string{
		"subject", "plainTextBody", "htmlBody", "replyTo", "senderDisplayName",
		"charset", "inReplyTo", "references", "orgWideEmailAddressId",
		"targetObjectId", "templateId", "whatId", "optOutPolicy", "emailPriority",
	} {
		message.Fields[field] = Null
	}
	for _, field := range []string{
		"saveAsActivity", "treatBodiesAsTemplate", "treatTargetObjectAsRecipient",
		"useSignature", "bccSender",
	} {
		message.Fields[field] = Bool(false)
	}
	return message
}

func newMassEmailMessage() Value {
	message := Object("Messaging.MassEmailMessage")
	for _, field := range []string{"targetObjectIds", "whatIds"} {
		message.Fields[field] = List()
	}
	for _, field := range []string{"templateId", "description", "optOutPolicy"} {
		message.Fields[field] = Null
	}
	message.Fields["saveAsActivity"] = Bool(false)
	return message
}

func isLocalEmailMessage(value Value) bool {
	return value.Kind == ValueObject && (value.Type == "Messaging.SingleEmailMessage" || value.Type == "Messaging.MassEmailMessage")
}

func newRestResponse() Value {
	response := Object("RestResponse")
	response.Fields["statusCode"] = Int(200)
	response.Fields["headers"] = typedMap("Map<String,String>")
	response.Fields["responseBody"] = Null
	return response
}

func typedMap(typeName string) Value {
	value := Map()
	value.Type = typeName
	return value
}

func (vm *VM) lookupRestContextField(name string) (Value, bool, error) {
	switch name {
	case "RestContext.request":
		if vm.restRequest.Kind == "" {
			return Null, true, nil
		}
		return vm.restRequest, true, nil
	case "RestContext.response":
		if vm.restResponse.Kind == "" || vm.restResponse.Kind == ValueNull {
			vm.restResponse = newRestResponse()
		}
		return vm.restResponse, true, nil
	default:
		for _, root := range []string{"RestContext.request", "RestContext.response"} {
			if strings.HasPrefix(name, root+".") {
				value, _, err := vm.lookupRestContextField(root)
				if err != nil {
					return Null, true, err
				}
				out, err := vm.lookupPath(value, strings.Split(strings.TrimPrefix(name, root+"."), "."))
				if err != nil {
					return Null, true, err
				}
				return out, true, nil
			}
		}
		return Null, false, nil
	}
}

func (vm *VM) assignRestContextField(name string, value Value) (bool, error) {
	switch name {
	case "RestContext.request":
		if value.Kind != ValueNull && (value.Kind != ValueObject || value.Type != "RestRequest") {
			return true, fmt.Errorf("RestContext.request expects RestRequest")
		}
		vm.restRequest = value
		return true, nil
	case "RestContext.response":
		if value.Kind != ValueNull && (value.Kind != ValueObject || value.Type != "RestResponse") {
			return true, fmt.Errorf("RestContext.response expects RestResponse")
		}
		vm.restResponse = value
		return true, nil
	default:
		for _, root := range []string{"RestContext.request", "RestContext.response"} {
			if strings.HasPrefix(name, root+".") {
				current, _, err := vm.lookupRestContextField(root)
				if err != nil {
					return true, err
				}
				if current.Kind == ValueNull {
					return true, newExceptionError("NullPointerException", "Attempt to de-reference a null object")
				}
				if err := vm.assignPath(current, strings.Split(strings.TrimPrefix(name, root+"."), "."), value); err != nil {
					return true, err
				}
				if root == "RestContext.request" {
					vm.restRequest = current
				} else {
					vm.restResponse = current
				}
				return true, nil
			}
		}
		return false, nil
	}
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
		if len(args) == 1 && (args[0].Kind == ValueList || args[0].Kind == ValueSet) {
			value := List(append([]Value(nil), collectionMembers(args[0])...)...)
			return vm.coerceAssignable(typeName, value)
		}
		return vm.coerceAssignable(typeName, List(args...))
	case strings.HasPrefix(typeName, "Set<"):
		if len(namedArgs) > 0 {
			return Null, fmt.Errorf("Set constructor does not accept named fields")
		}
		if len(args) == 1 && (args[0].Kind == ValueList || args[0].Kind == ValueSet) {
			value := Set(collectionMembers(args[0])...)
			return vm.coerceAssignable(typeName, value)
		}
		return vm.coerceAssignable(typeName, Set(args...))
	case strings.HasPrefix(typeName, "Map<"):
		if len(namedArgs) != 0 {
			return Null, fmt.Errorf("Map constructor does not accept named fields")
		}
		if len(args) > 0 && allMapEntryValues(args) {
			value := Map()
			value.Type = typeName
			for _, entry := range args {
				keyValue := entry.Fields["__key"]
				item := entry.Fields["__value"]
				key, coerced, err := vm.coerceMapEntry(typeName, keyValue, item)
				if err != nil {
					return Null, fmt.Errorf("Map constructor: %w", err)
				}
				value.Map[mapKey(key)] = coerced
			}
			return value, nil
		}
		if len(args) == 1 && args[0].Kind == ValueMap {
			value := Map()
			value.Type = typeName
			for rawKey, item := range args[0].Map {
				keyValue := valueFromMapKey(rawKey)
				key, coerced, err := vm.coerceMapEntry(typeName, keyValue, item)
				if err != nil {
					return Null, fmt.Errorf("Map constructor: %w", err)
				}
				value.Map[mapKey(key)] = coerced
			}
			return value, nil
		}
		if len(args) == 1 && args[0].Kind == ValueList {
			value, err := vm.mapFromSObjectList(typeName, args[0])
			if err != nil {
				return Null, err
			}
			return value, nil
		}
		if len(args) != 0 {
			return Null, fmt.Errorf("Map constructor does not accept positional values")
		}
		value := Map()
		value.Type = typeName
		return value, nil
	}
	if class, ok := vm.Classes[typeName]; ok {
		if class.IsInterface {
			return Null, fmt.Errorf("cannot instantiate interface %s", typeName)
		}
		if err := vm.checkClassAccess(class.Name, typeName); err != nil {
			return Null, err
		}
		if class.IsAbstract {
			return Null, fmt.Errorf("cannot instantiate abstract class %s", typeName)
		}
		if len(class.EnumValues) > 0 {
			return Null, fmt.Errorf("cannot instantiate enum %s", typeName)
		}
		object := Object(typeName)
		for field, value := range namedArgs {
			object.Fields[field] = value
		}
		vm.initializeFields(&object, typeName)
		if err := vm.runInstanceInitializers(class, object, result); err != nil {
			return Null, err
		}
		ctor, ok, ambiguous := vm.matchConstructor(class, args)
		if ok {
			if _, err := vm.callMethodWithReceiver(ctor, object, args, result); err != nil {
				return Null, err
			}
		} else if ambiguous {
			return Null, fmt.Errorf("ambiguous %s constructor with %d argument(s)", typeName, len(args))
		} else if len(args) != 0 {
			if isExceptionType(typeName) && len(args) == 1 {
				if args[0].Kind == ValueString {
					object.Fields["message"] = args[0]
				} else if args[0].Kind != ValueNull {
					object.Fields["message"] = String(args[0].String())
				}
				return object, nil
			}
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return object, nil
	}
	switch typeName {
	case "HttpRequest":
		if len(args) != 0 {
			return Null, fmt.Errorf("HttpRequest constructor expects 0 arguments")
		}
		request := newHttpRequest()
		for field, value := range namedArgs {
			request.Fields[field] = value
		}
		return request, nil
	case "HttpResponse":
		if len(args) != 0 {
			return Null, fmt.Errorf("HttpResponse constructor expects 0 arguments")
		}
		response := newHttpResponse()
		for field, value := range namedArgs {
			response.Fields[field] = value
		}
		return response, nil
	case "StaticResourceCalloutMock", "MultiStaticResourceCalloutMock":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		mock := Object(typeName)
		mock.Fields["headers"] = typedMap("Map<String,String>")
		mock.Fields["statusCode"] = Int(200)
		if typeName == "MultiStaticResourceCalloutMock" {
			mock.Fields["staticResources"] = typedMap("Map<String,String>")
		}
		for field, value := range namedArgs {
			mock.Fields[field] = value
		}
		return mock, nil
	case "RestRequest":
		if len(args) != 0 {
			return Null, fmt.Errorf("RestRequest constructor expects 0 arguments")
		}
		request := newRestRequest()
		for field, value := range namedArgs {
			request.Fields[field] = value
		}
		return request, nil
	case "RestResponse":
		if len(args) != 0 {
			return Null, fmt.Errorf("RestResponse constructor expects 0 arguments")
		}
		response := newRestResponse()
		for field, value := range namedArgs {
			response.Fields[field] = value
		}
		return response, nil
	case "Continuation":
		return Null, unsupportedCallError("Continuation constructor local continuation callout surface")
	case "PageReference":
		if len(args) > 1 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("PageReference constructor expects optional URL String")
		}
		rawURL := ""
		if len(args) == 1 {
			if args[0].Kind != ValueString {
				return Null, fmt.Errorf("PageReference constructor expects URL String")
			}
			rawURL = args[0].Text
		}
		return newPageReference(rawURL), nil
	case "ApexPages.Message", "ApexPages.message":
		if len(args) < 2 || len(args) > 3 {
			return Null, fmt.Errorf("ApexPages.Message constructor expects severity, summary[, detail]")
		}
		message := Object("ApexPages.Message")
		message.Fields["severity"] = args[0]
		message.Fields["summary"] = args[1]
		if len(args) == 3 {
			message.Fields["detail"] = args[2]
		}
		return message, nil
	case "Messaging.SendEmailResult":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Messaging.SendEmailResult constructor expects 0 arguments")
		}
		return newSendEmailResult(), nil
	case "Messaging.SingleEmailMessage":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return newSingleEmailMessage(), nil
	case "Messaging.MassEmailMessage":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return newMassEmailMessage(), nil
	case "Messaging.SendEmailOptions":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return Object(typeName), nil
	case "URL":
		if len(namedArgs) != 0 {
			return Null, fmt.Errorf("URL constructor does not accept named fields")
		}
		var raw string
		switch len(args) {
		case 1:
			if args[0].Kind != ValueString {
				return Null, fmt.Errorf("URL constructor expects String")
			}
			raw = args[0].Text
		case 2:
			if args[0].Kind != ValueObject || args[0].Type != "URL" || args[1].Kind != ValueString {
				return Null, fmt.Errorf("URL constructor expects URL context and String spec")
			}
			baseRaw, err := platformScalarText(args[0], "URL")
			if err != nil {
				return Null, err
			}
			base, err := url.Parse(baseRaw)
			if err != nil {
				return Null, err
			}
			ref, err := url.Parse(args[1].Text)
			if err != nil {
				return Null, err
			}
			raw = base.ResolveReference(ref).String()
		case 3, 4:
			if args[0].Kind != ValueString || args[1].Kind != ValueString || args[len(args)-1].Kind != ValueString {
				return Null, fmt.Errorf("URL constructor expects protocol, host, [port,] file")
			}
			protocol, host, file := args[0].Text, args[1].Text, args[len(args)-1].Text
			if len(args) == 4 {
				if args[2].Kind != ValueInt {
					return Null, fmt.Errorf("URL constructor port expects Integer")
				}
				host = fmt.Sprintf("%s:%d", host, args[2].Int)
			}
			raw = protocol + "://" + host + file
		default:
			return Null, fmt.Errorf("URL constructor expects spec, context and spec, or protocol, host, [port,] file")
		}
		if err := validateURLConstructorValue(raw); err != nil {
			return Null, err
		}
		return platformScalar("URL", raw), nil
	}
	objectType := typeName
	var definition storage.ObjectDefinition
	if vm.Org != nil {
		if canonical, ok := storage.ResolveObjectName(*vm.Org, typeName); ok {
			objectType = canonical
			definition = vm.Org.Objects[canonical].Definition
		}
	}
	object := Object(objectType)
	for field, value := range namedArgs {
		if definition.APIName != "" {
			if canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field); ok {
				field = canonical
			}
		}
		object.Fields[field] = value
	}
	vm.initializeFields(&object, objectType)
	if len(args) != 0 {
		if isExceptionType(typeName) && len(args) == 1 {
			if args[0].Kind == ValueString {
				object.Fields["message"] = args[0]
			} else if args[0].Kind != ValueNull {
				object.Fields["message"] = String(args[0].String())
			}
			return object, nil
		}
		return Null, fmt.Errorf("%s constructor does not accept arguments", typeName)
	}
	return object, nil
}

func allMapEntryValues(values []Value) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if value.Kind != ValueObject || value.Type != "__mapEntry" {
			return false
		}
	}
	return true
}

func validateURLConstructorValue(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("URL constructor invalid URL: %w", err)
	}
	if parsed.Scheme == "" {
		return fmt.Errorf("URL constructor invalid URL: missing protocol")
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL constructor invalid URL: missing host")
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.ParseInt(port, 10, 64)
		if err != nil || value < 0 || value > 65535 {
			return fmt.Errorf("URL constructor invalid URL: invalid port")
		}
	}
	return nil
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
	typeName = exceptionTypeName(typeName)
	return isBuiltinExceptionType(typeName) || strings.HasSuffix(typeName, "Exception")
}

func typeNewInstanceAllowsDottedBuiltin(typeName string) bool {
	return isExceptionType(typeName) ||
		strings.HasPrefix(typeName, "Schema.") ||
		typeName == "ApexPages.Message"
}

func typeNewInstanceUnsupportedBuiltin(typeName string) (string, bool) {
	canonical := strings.TrimPrefix(typeName, "System.")
	switch canonical {
	case "Object", "String", "Boolean", "Integer", "Long", "Decimal", "Double",
		"Date", "Datetime", "Time", "TimeZone", "Blob", "Id", "Type", "URL",
		"LoggingLevel", "RestContext":
		return canonical, true
	default:
		return "", false
	}
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

func (vm *VM) matchConstructor(class Class, args []Value) (Method, bool, bool) {
	return vm.matchMethodByArgs(class.Constructors, args)
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
	target, found, ambiguous := vm.matchConstructor(targetClass, args)
	if ambiguous {
		return Null, fmt.Errorf("ambiguous %s constructor with %d argument(s)", targetClass.Name, len(args))
	}
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
	return vm.resolveInstanceMethodSeen(typeName, method, make(map[string]bool))
}

func (vm *VM) resolveInstanceMethodSeen(typeName, method string, seen map[string]bool) (Method, bool) {
	var interfaces []string
	for typeName != "" {
		if seen[typeName] {
			return Method{}, false
		}
		seen[typeName] = true
		if target, ok := vm.Methods[typeName+"."+method]; ok {
			return target, true
		}
		class, ok := vm.Classes[typeName]
		if !ok {
			break
		}
		interfaces = append(interfaces, class.Interfaces...)
		typeName = class.SuperClass
	}
	for _, iface := range interfaces {
		if target, ok := vm.resolveInterfaceMethodSeen(iface, method, seen); ok {
			return target, true
		}
	}
	return Method{}, false
}

func (vm *VM) resolveInstanceMethodForArgs(typeName, method string, args []Value) (Method, bool, bool) {
	return vm.resolveInstanceMethodForArgsSeen(typeName, method, args, make(map[string]bool))
}

func (vm *VM) resolveInstanceMethodForArgsSeen(typeName, method string, args []Value, seen map[string]bool) (Method, bool, bool) {
	var interfaces []string
	for typeName != "" {
		if seen[typeName] {
			return Method{}, false, false
		}
		seen[typeName] = true
		if target, ok, ambiguous := vm.matchRegisteredMethod(typeName+"."+method, args); ok || ambiguous {
			return target, ok, ambiguous
		}
		class, ok := vm.Classes[typeName]
		if !ok {
			break
		}
		interfaces = append(interfaces, class.Interfaces...)
		typeName = class.SuperClass
	}
	for _, iface := range interfaces {
		if target, ok, ambiguous := vm.resolveInterfaceMethodForArgsSeen(iface, method, args, seen); ok || ambiguous {
			return target, ok, ambiguous
		}
	}
	return Method{}, false, false
}

func (vm *VM) resolveInterfaceMethodSeen(typeName, method string, seen map[string]bool) (Method, bool) {
	if typeName == "" || seen[typeName] {
		return Method{}, false
	}
	seen[typeName] = true
	if target, ok := vm.Methods[typeName+"."+method]; ok {
		return target, true
	}
	class, ok := vm.Classes[typeName]
	if !ok {
		return Method{}, false
	}
	for _, iface := range class.Interfaces {
		if target, ok := vm.resolveInterfaceMethodSeen(iface, method, seen); ok {
			return target, true
		}
	}
	return Method{}, false
}

func (vm *VM) resolveInterfaceMethodForArgsSeen(typeName, method string, args []Value, seen map[string]bool) (Method, bool, bool) {
	if typeName == "" || seen[typeName] {
		return Method{}, false, false
	}
	seen[typeName] = true
	if target, ok, ambiguous := vm.matchRegisteredMethod(typeName+"."+method, args); ok || ambiguous {
		return target, ok, ambiguous
	}
	class, ok := vm.Classes[typeName]
	if !ok {
		return Method{}, false, false
	}
	for _, iface := range class.Interfaces {
		if target, ok, ambiguous := vm.resolveInterfaceMethodForArgsSeen(iface, method, args, seen); ok || ambiguous {
			return target, ok, ambiguous
		}
	}
	return Method{}, false, false
}

func (vm *VM) resolveStaticMethodForArgs(typeName, method string, args []Value) (Method, bool, bool) {
	for typeName != "" {
		target, ok, ambiguous := vm.matchRegisteredMethod(typeName+"."+method, args)
		if ambiguous {
			return Method{}, false, true
		}
		if ok {
			if target.IsStatic {
				return target, true, false
			}
			return Method{}, false, false
		}
		class, ok := vm.Classes[typeName]
		if !ok {
			return Method{}, false, false
		}
		typeName = class.SuperClass
	}
	return Method{}, false, false
}

func (vm *VM) matchRegisteredMethod(name string, args []Value) (Method, bool, bool) {
	if candidates := vm.MethodOverloads[name]; len(candidates) > 0 {
		return vm.matchMethodByArgs(candidates, args)
	}
	if candidates := vm.MethodFolded[strings.ToLower(name)]; len(candidates) > 0 {
		return vm.matchMethodByArgs(candidates, args)
	}
	method, ok := vm.Methods[name]
	if !ok {
		return Method{}, false, false
	}
	if len(method.Params) != len(args) {
		return Method{}, false, false
	}
	for i, param := range method.Params {
		if err := vm.ensureAssignable(param.Type, args[i]); err != nil {
			return Method{}, false, false
		}
	}
	return method, true, false
}

func (vm *VM) matchMethodByArgs(candidates []Method, args []Value) (Method, bool, bool) {
	applicable := make([]Method, 0, len(candidates))
	for _, candidate := range candidates {
		if vm.methodApplicable(candidate, args) {
			applicable = append(applicable, candidate)
		}
	}
	return vm.bestMethodBySpecificity(applicable)
}

func (vm *VM) methodApplicable(candidate Method, args []Value) bool {
	if len(candidate.Params) != len(args) {
		return false
	}
	for i, param := range candidate.Params {
		paramType := vm.resolveTypeNameInClass(candidate.ClassName, param.Type)
		if vm.conversionScore(paramType, args[i]) < 0 {
			return false
		}
	}
	return true
}

func (vm *VM) bestMethodBySpecificity(applicable []Method) (Method, bool, bool) {
	if len(applicable) == 0 {
		return Method{}, false, false
	}
	bestIndex := -1
	for i, candidate := range applicable {
		moreSpecificThanAll := true
		for j, other := range applicable {
			if i == j {
				continue
			}
			switch vm.compareMethodSpecificity(candidate, other) {
			case -1, 2:
				moreSpecificThanAll = false
			}
			if !moreSpecificThanAll {
				break
			}
		}
		if moreSpecificThanAll {
			if bestIndex >= 0 && vm.compareMethodSpecificity(candidate, applicable[bestIndex]) == 0 {
				continue
			}
			if bestIndex >= 0 {
				return Method{}, false, true
			}
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return Method{}, false, true
	}
	return applicable[bestIndex], true, false
}

func (vm *VM) compareMethodSpecificity(left, right Method) int {
	leftBetter := false
	rightBetter := false
	for i := range left.Params {
		leftType := vm.resolveTypeNameInClass(left.ClassName, left.Params[i].Type)
		rightType := vm.resolveTypeNameInClass(right.ClassName, right.Params[i].Type)
		switch vm.compareTypeSpecificity(leftType, rightType) {
		case 1:
			leftBetter = true
		case -1:
			rightBetter = true
		case 2:
			return 2
		}
		if leftBetter && rightBetter {
			return 2
		}
	}
	switch {
	case leftBetter:
		return 1
	case rightBetter:
		return -1
	default:
		return 0
	}
}

func (vm *VM) resolveTypeNameInClass(className, typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	if base := collectionBase(typeName); base != "" {
		element, ok := collectionElementType(typeName)
		if !ok {
			return typeName
		}
		return base + "<" + vm.resolveTypeNameInClass(className, element) + ">"
	}
	if keyType, valueType, ok := mapTypeArgs(typeName); ok {
		return "Map<" + vm.resolveTypeNameInClass(className, keyType) + "," + vm.resolveTypeNameInClass(className, valueType) + ">"
	}
	if strings.Contains(typeName, ".") || className == "" {
		return typeName
	}
	for owner := className; owner != ""; {
		candidate := owner + "." + typeName
		if class, ok := vm.Classes[candidate]; ok {
			return class.Name
		}
		dot := strings.LastIndex(owner, ".")
		if dot < 0 {
			break
		}
		owner = owner[:dot]
	}
	return typeName
}

func (vm *VM) compareTypeSpecificity(left, right string) int {
	if strings.EqualFold(left, right) {
		return 0
	}
	leftToRight := vm.typeAssignableTo(left, right)
	rightToLeft := vm.typeAssignableTo(right, left)
	switch {
	case leftToRight && !rightToLeft:
		return 1
	case rightToLeft && !leftToRight:
		return -1
	case !leftToRight && !rightToLeft:
		return 2
	default:
		return 0
	}
}

func (vm *VM) typeAssignableTo(from, to string) bool {
	if strings.EqualFold(from, to) || strings.EqualFold(to, "Object") {
		return true
	}
	if (strings.EqualFold(from, "String") && strings.EqualFold(to, "Id")) ||
		(strings.EqualFold(from, "Id") && strings.EqualFold(to, "String")) {
		return true
	}
	if collectionBase(from) != "" && strings.EqualFold(collectionBase(from), collectionBase(to)) {
		fromElement, fromOK := collectionElementType(from)
		toElement, toOK := collectionElementType(to)
		if fromOK && toOK {
			return vm.typeAssignableTo(fromElement, toElement)
		}
	}
	if numericConversionScore(to, from) >= 0 {
		return true
	}
	if _, ok := vm.typeDistance(from, to, make(map[string]bool)); ok {
		return true
	}
	return false
}

func collectionBase(typeName string) string {
	switch {
	case strings.HasPrefix(typeName, "List<"):
		return "List"
	case strings.HasPrefix(typeName, "Set<"):
		return "Set"
	default:
		return ""
	}
}

func (vm *VM) conversionScore(paramType string, value Value) int {
	if value.Kind == ValueNull {
		if value.Type != "" {
			if strings.EqualFold(paramType, value.Type) {
				return 1000
			}
			if vm.typeAssignableTo(value.Type, paramType) {
				return 900
			}
			return -1
		}
		return 1
	}
	valueType := valueTypeName(value)
	if strings.EqualFold(paramType, valueType) {
		return 1000
	}
	if collectionBase(valueType) != "" && collectionBase(paramType) != "" {
		if vm.typeAssignableTo(valueType, paramType) {
			return 900
		}
		if vm.sObjectCollectionDowncastAssignable(valueType, paramType) {
			return 850
		}
		if vm.collectionElementsAssignable(paramType, value) {
			return 850
		}
		return -1
	}
	if score := numericConversionScore(paramType, valueType); score >= 0 {
		return score
	}
	if strings.EqualFold(paramType, "Object") {
		return 10
	}
	if value.Kind == ValueObject {
		if distance, ok := vm.typeDistance(value.Type, paramType, make(map[string]bool)); ok {
			return 800 - distance
		}
	}
	if err := vm.ensureAssignable(paramType, value); err != nil {
		return -1
	}
	return 1
}

func (vm *VM) sObjectCollectionDowncastAssignable(fromType, toType string) bool {
	fromElement, fromOK := collectionElementType(fromType)
	toElement, toOK := collectionElementType(toType)
	if !fromOK || !toOK {
		return false
	}
	if !strings.EqualFold(fromElement, "sObject") {
		return false
	}
	return vm.isSObjectLikeType(toElement)
}

func (vm *VM) collectionElementsAssignable(paramType string, value Value) bool {
	elementType, ok := collectionElementType(paramType)
	if !ok {
		return false
	}
	switch value.Kind {
	case ValueList:
		if len(value.List) == 0 {
			return false
		}
		for _, item := range value.List {
			if err := vm.ensureAssignable(elementType, item); err != nil {
				return false
			}
		}
		return true
	case ValueSet:
		if len(value.Set) == 0 {
			return false
		}
		for _, item := range value.Set {
			if err := vm.ensureAssignable(elementType, item); err != nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func numericConversionScore(paramType, valueType string) int {
	switch valueType {
	case "Integer":
		switch paramType {
		case "Long":
			return 900
		case "Decimal":
			return 800
		case "Double":
			return 700
		}
	case "Long":
		switch paramType {
		case "Decimal":
			return 800
		case "Double":
			return 700
		}
	case "Decimal":
		if paramType == "Double" {
			return 800
		}
	}
	return -1
}

func (vm *VM) typeDistance(typeName, target string, seen map[string]bool) (int, bool) {
	typeName = systemInterfaceAlias(typeName)
	target = systemInterfaceAlias(target)
	if resolved, ok := vm.resolveClassName(typeName); ok {
		typeName = resolved
	}
	if resolved, ok := vm.resolveClassName(target); ok {
		target = resolved
	}
	if typeName == "" || seen[typeName] {
		return 0, false
	}
	if typeName == target {
		return 0, true
	}
	seen[typeName] = true
	class, ok := vm.Classes[typeName]
	if !ok {
		return 0, false
	}
	best := 0
	found := false
	if distance, ok := vm.typeDistance(class.SuperClass, target, seen); ok {
		best = distance + 1
		found = true
	}
	for _, iface := range class.Interfaces {
		if distance, ok := vm.typeDistance(iface, target, seen); ok {
			distance++
			if !found || distance < best {
				best = distance
				found = true
			}
		}
	}
	return best, found
}

func valueTypeName(value Value) string {
	switch value.Kind {
	case ValueInt:
		return "Integer"
	case ValueDecimal:
		return "Decimal"
	case ValueBool:
		return "Boolean"
	case ValueString:
		return "String"
	case ValueList:
		if value.Type != "" {
			return value.Type
		}
		return "List"
	case ValueSet:
		if value.Type != "" {
			return value.Type
		}
		return "Set"
	case ValueMap:
		if value.Type != "" {
			return value.Type
		}
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

func annotateException(value Value, stack []callFrame) Value {
	if value.Kind != ValueObject || !isExceptionType(value.Type) || len(stack) == 0 {
		return value
	}
	if _, ok := value.Fields["__stackTrace"]; ok {
		return value
	}
	value.Fields["__lineNumber"] = Int(int64(stack[len(stack)-1].Line))
	value.Fields["__stackTrace"] = String(stackTraceString(stack))
	return value
}

func stackTraceString(stack []callFrame) string {
	frames := stackFrames(stack)
	lines := make([]string, 0, len(frames))
	for _, frame := range frames {
		location := frame.File
		if frame.Line > 0 {
			if location != "" {
				location += ":"
			}
			location += strconv.Itoa(frame.Line)
			if frame.Column > 0 {
				location += ":" + strconv.Itoa(frame.Column)
			}
		}
		if location == "" {
			lines = append(lines, frame.Symbol)
		} else {
			lines = append(lines, frame.Symbol+" ("+location+")")
		}
	}
	return strings.Join(lines, "\n")
}

func catchTypes(inst ir.Instruction) []string {
	if len(inst.CatchTypes) > 0 {
		return inst.CatchTypes
	}
	return []string{inst.Type}
}

func vmCatchClauses(inst ir.Instruction) []ir.CatchClause {
	if len(inst.Catches) > 0 {
		return inst.Catches
	}
	if len(inst.Catch) == 0 {
		return nil
	}
	return []ir.CatchClause{{Types: catchTypes(inst), Name: inst.Name, Body: inst.Catch, Pos: inst.Pos}}
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
	if catchType == "" || exceptionTypeName(catchType) == "Exception" || strings.EqualFold(catchType, "Object") {
		return true
	}
	if thrown.Kind == ValueObject {
		return vm.typeMatches(thrown.Type, catchType, make(map[string]bool))
	}
	return false
}

func (vm *VM) typeMatches(typeName, target string, seen map[string]bool) bool {
	typeName = systemInterfaceAlias(typeName)
	target = systemInterfaceAlias(target)
	if resolved, ok := vm.resolveClassName(typeName); ok {
		typeName = resolved
	}
	if resolved, ok := vm.resolveClassName(target); ok {
		target = resolved
	}
	if vm.Org != nil {
		if resolved, ok := storage.ResolveObjectName(*vm.Org, typeName); ok {
			typeName = resolved
		}
		if resolved, ok := storage.ResolveObjectName(*vm.Org, target); ok {
			target = resolved
		}
	}
	if typeName == "" || seen[typeName] {
		return false
	}
	if strings.EqualFold(typeName, target) {
		return true
	}
	if strings.EqualFold(target, "sObject") && vm.isSObjectLikeType(typeName) {
		return true
	}
	if builtinExceptionTypeMatches(typeName, target) {
		return true
	}
	seen[typeName] = true
	class, ok := vm.Classes[typeName]
	if !ok {
		return false
	}
	if vm.typeMatches(class.SuperClass, target, seen) {
		return true
	}
	for _, iface := range class.Interfaces {
		if vm.typeMatches(iface, target, seen) {
			return true
		}
	}
	return false
}

func systemInterfaceAlias(typeName string) string {
	switch strings.TrimSpace(typeName) {
	case "System.Callable":
		return "Callable"
	case "System.StubProvider":
		return "StubProvider"
	default:
		return typeName
	}
}

func (vm *VM) evalInstanceOf(value Value, target string) Value {
	target = strings.TrimSpace(target)
	if target == "" || value.Kind == ValueNull {
		return Bool(false)
	}
	if value.Kind == ValueObject {
		return Bool(vm.typeMatches(value.Type, target, make(map[string]bool)))
	}
	if strings.EqualFold(target, "Object") {
		return Bool(true)
	}
	return Bool(strings.EqualFold(valueTypeName(value), target))
}

func builtinExceptionTypeMatches(typeName, target string) bool {
	typeName = exceptionTypeName(typeName)
	target = exceptionTypeName(target)
	if typeName == "" || target == "" {
		return false
	}
	for current := typeName; current != ""; current = builtinExceptionParent(current) {
		if current == target {
			return true
		}
	}
	return false
}

func builtinExceptionParent(typeName string) string {
	typeName = exceptionTypeName(typeName)
	if parent, ok := builtinExceptionParents[typeName]; ok {
		return parent
	}
	if strings.HasSuffix(typeName, "Exception") {
		return "Exception"
	}
	return ""
}

func isBuiltinExceptionType(typeName string) bool {
	typeName = exceptionTypeName(typeName)
	_, ok := builtinExceptionParents[typeName]
	return ok
}

var builtinExceptionParents = map[string]string{
	"Exception": "Object",

	"AssertException":                 "Exception",
	"AsyncException":                  "Exception",
	"CalloutException":                "Exception",
	"DmlException":                    "Exception",
	"EmailException":                  "Exception",
	"ExternalObjectException":         "Exception",
	"IllegalArgumentException":        "Exception",
	"IllegalStateException":           "Exception",
	"InvalidParameterValueException":  "Exception",
	"JSONException":                   "Exception",
	"LimitException":                  "Exception",
	"ListException":                   "Exception",
	"MathException":                   "Exception",
	"NoAccessException":               "Exception",
	"NoDataFoundException":            "Exception",
	"NoSuchElementException":          "Exception",
	"NullPointerException":            "Exception",
	"PatternSyntaxException":          "IllegalArgumentException",
	"QueryException":                  "Exception",
	"RequiredFeatureMissingException": "Exception",
	"SearchException":                 "Exception",
	"SecurityException":               "Exception",
	"SerializationException":          "Exception",
	"SObjectException":                "Exception",
	"StringException":                 "Exception",
	"TypeException":                   "Exception",
	"VisualforceException":            "Exception",
	"XmlException":                    "Exception",
}

func exceptionTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimPrefix(typeName, "System.")
	return typeName
}

func exceptionToString(value Value) string {
	typeName := exceptionTypeName(value.Type)
	if typeName == "" {
		typeName = "Exception"
	}
	message := ""
	if raw, ok := value.Fields["message"]; ok && raw.Kind == ValueString {
		message = raw.Text
	}
	prefix := "System." + typeName
	if message == "" {
		return prefix
	}
	return prefix + ": " + message
}

func typeValueName(value Value) string {
	if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
		return raw.Text
	}
	return value.Text
}

var loggingLevelNames = []string{"NONE", "ERROR", "WARN", "INFO", "DEBUG", "FINE", "FINER", "FINEST"}
var apexPagesSeverityNames = []string{"CONFIRM", "INFO", "WARNING", "ERROR", "FATAL"}

func isLoggingLevelName(level string) bool {
	for _, name := range loggingLevelNames {
		if level == name {
			return true
		}
	}
	return false
}

func isLoggingLevelValue(value Value) bool {
	if value.Kind != ValueObject || value.Type != "LoggingLevel" {
		return false
	}
	return isLoggingLevelName(value.Text)
}

func (vm *VM) coerceAssignable(typeName string, value Value) (Value, error) {
	if value.Kind == ValueString {
		switch typeName {
		case "Date":
			parsed, err := parseDateText(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Date", parsed.Format("2006-01-02")), nil
		case "Datetime":
			parsed, err := parseDatetimeText(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Datetime", formatPlatformDatetime(parsed)), nil
		case "Time":
			parsed, err := parseTimeText(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Time", parsed), nil
		}
	}
	if value.Kind == ValueObject {
		if strings.EqualFold(typeName, "String") && (strings.EqualFold(value.Type, "Id") || strings.EqualFold(value.Type, "String")) {
			text, err := platformScalarText(value, value.Type)
			if err != nil {
				return Null, err
			}
			return String(text), nil
		}
		if strings.EqualFold(typeName, "Id") && strings.EqualFold(value.Type, "String") {
			text, err := platformScalarText(value, value.Type)
			if err != nil {
				return Null, err
			}
			return platformScalar("Id", text), nil
		}
		if strings.EqualFold(typeName, "Object") || vm.typeMatches(value.Type, typeName, make(map[string]bool)) {
			return value, nil
		}
		return Null, fmt.Errorf("cannot assign %s to %s", value.Type, typeName)
	}
	if strings.HasPrefix(typeName, "List<") && value.Kind == ValueList {
		value.Type = typeName
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		for i, item := range value.List {
			coerced, err := vm.coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			value.List[i] = coerced
		}
		return value, nil
	}
	if strings.HasPrefix(typeName, "Set<") && value.Kind == ValueSet {
		value.Type = typeName
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		out := make([]Value, 0, len(value.Set))
		for _, item := range value.Set {
			coerced, err := vm.coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			if !containsValue(out, coerced) {
				out = append(out, coerced)
			}
		}
		value.Set = out
		return value, nil
	}
	if strings.HasPrefix(typeName, "Map<") && value.Kind == ValueMap {
		value.Type = typeName
		return value, nil
	}
	return coerceAssignable(typeName, value)
}

func (vm *VM) ensureAssignable(typeName string, value Value) error {
	_, err := vm.coerceAssignable(typeName, value)
	return err
}

func (vm *VM) coerceCollectionElement(collectionType string, value Value) (Value, error) {
	elementType, ok := collectionElementType(collectionType)
	if !ok {
		return value, nil
	}
	return vm.coerceAssignable(elementType, value)
}

func (vm *VM) coerceMapEntry(mapType string, key, value Value) (Value, Value, error) {
	keyType, valueType, ok := mapTypeArgs(mapType)
	if !ok {
		return key, value, nil
	}
	coercedKey, err := vm.coerceAssignable(keyType, key)
	if err != nil {
		return Null, Null, fmt.Errorf("key: %w", err)
	}
	coercedValue, err := vm.coerceAssignable(valueType, value)
	if err != nil {
		return Null, Null, fmt.Errorf("value: %w", err)
	}
	return coercedKey, coercedValue, nil
}

func (vm *VM) mapFromSObjectList(mapType string, list Value) (Value, error) {
	keyType, valueType, ok := mapTypeArgs(mapType)
	if !ok || !strings.EqualFold(keyType, "Id") {
		return Null, unsupportedCallError("Map constructor from SObject list")
	}
	out := Map()
	out.Type = mapType
	for i, item := range list.List {
		if item.Kind == ValueNull {
			return Null, fmt.Errorf("Map constructor from SObject list requires non-null SObject at index %d", i)
		}
		if item.Kind != ValueObject {
			return Null, fmt.Errorf("Map constructor from SObject list requires SObject values at index %d", i)
		}
		coerced, err := vm.coerceAssignable(valueType, item)
		if err != nil {
			return Null, fmt.Errorf("Map constructor from SObject list: value at index %d: %w", i, err)
		}
		id, ok := coerced.Fields["Id"]
		if !ok || id.Kind == ValueNull {
			return Null, fmt.Errorf("Map constructor from SObject list requires non-null Id at index %d", i)
		}
		key, err := vm.coerceAssignable(keyType, id)
		if err != nil {
			return Null, fmt.Errorf("Map constructor from SObject list: Id at index %d: %w", i, err)
		}
		encodedKey := mapKey(key)
		if _, exists := out.Map[encodedKey]; exists {
			return Null, fmt.Errorf("Map constructor from SObject list found duplicate Id at index %d", i)
		}
		out.Map[encodedKey] = coerced
	}
	return out, nil
}

func (vm *VM) putAllSObjectList(receiver Value, list Value) (Value, error) {
	value, err := vm.mapFromSObjectList(receiver.Type, list)
	if err != nil {
		return receiver, err
	}
	for key, item := range value.Map {
		receiver.Map[key] = item
	}
	return receiver, nil
}

func collectionMembers(value Value) []Value {
	switch value.Kind {
	case ValueList:
		return value.List
	case ValueSet:
		return value.Set
	default:
		return nil
	}
}

func collectionIterator(value Value) Value {
	snapshot := List(append([]Value(nil), collectionMembers(value)...)...)
	iterator := Object(collectionIteratorType(value.Type))
	iterator.Fields["__values"] = snapshot
	iterator.Fields["__index"] = Int(0)
	return iterator
}

func collectionIteratorType(collectionType string) string {
	if elementType, ok := collectionElementType(collectionType); ok {
		return "Iterator<" + elementType + ">"
	}
	return "Iterator"
}

func isIteratorValue(value Value) bool {
	return value.Kind == ValueObject && (value.Type == "Iterator" || strings.HasPrefix(value.Type, "Iterator<") || value.Type == "Database.QueryLocatorIterator")
}

func callIteratorMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	values, ok := receiver.Fields["__values"]
	if !ok || values.Kind != ValueList {
		return Null, receiver, false, true, fmt.Errorf("Iterator missing snapshot")
	}
	indexValue, ok := receiver.Fields["__index"]
	if !ok || indexValue.Kind != ValueInt {
		return Null, receiver, false, true, fmt.Errorf("Iterator missing index")
	}
	index := int(indexValue.Int)
	switch method {
	case "hasNext":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Iterator.hasNext expects 0 arguments")
		}
		return Bool(index < len(values.List)), receiver, false, true, nil
	case "next":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Iterator.next expects 0 arguments")
		}
		if index >= len(values.List) {
			return Null, receiver, false, true, newExceptionError("NoSuchElementException", "Iterator has no more elements")
		}
		receiver.Fields["__index"] = Int(int64(index + 1))
		return values.List[index], receiver, true, true, nil
	case "remove":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Iterator.remove expects 0 arguments")
		}
		return Null, receiver, false, true, unsupportedCallError("Iterator.remove")
	default:
		return Null, receiver, false, false, nil
	}
}

func sortComparableValues(values []Value) error {
	for _, value := range values {
		switch value.Kind {
		case ValueInt, ValueDecimal, ValueString, ValueBool:
		default:
			return unsupportedCallError("List.sort for non-primitive comparable values")
		}
	}
	sort.SliceStable(values, func(i, j int) bool {
		left, right := values[i], values[j]
		if collectionNumericKind(left.Kind) && collectionNumericKind(right.Kind) {
			return collectionNumericValue(left) < collectionNumericValue(right)
		}
		if left.Kind != right.Kind {
			return collectionSortKindRank(left.Kind) < collectionSortKindRank(right.Kind)
		}
		switch left.Kind {
		case ValueInt:
			return left.Int < right.Int
		case ValueDecimal:
			return left.Decimal < right.Decimal
		case ValueString:
			return left.Text < right.Text
		case ValueBool:
			return !left.Bool && right.Bool
		default:
			return false
		}
	})
	return nil
}

func collectionNumericKind(kind ValueKind) bool {
	return kind == ValueInt || kind == ValueDecimal
}

func collectionNumericValue(value Value) float64 {
	if value.Kind == ValueInt {
		return float64(value.Int)
	}
	return value.Decimal
}

func collectionSortKindRank(kind ValueKind) int {
	switch kind {
	case ValueBool:
		return 0
	case ValueInt, ValueDecimal:
		return 1
	case ValueString:
		return 2
	default:
		return 3
	}
}

func valueFromMapKey(key string) Value {
	if strings.HasPrefix(key, string(ValueObject)+":") {
		rest := strings.TrimPrefix(key, string(ValueObject)+":")
		typeName, text, ok := strings.Cut(rest, ":")
		if ok && platformScalarObject(typeName) {
			return platformScalar(typeName, text)
		}
	}
	kind, text, ok := strings.Cut(key, ":")
	if !ok {
		return String(key)
	}
	switch ValueKind(kind) {
	case ValueNull:
		return Null
	case ValueInt:
		var parsed int64
		if _, err := fmt.Sscan(text, &parsed); err == nil {
			return Int(parsed)
		}
	case ValueDecimal:
		var parsed float64
		if _, err := fmt.Sscan(text, &parsed); err == nil {
			return Decimal(parsed)
		}
	case ValueBool:
		return Bool(strings.EqualFold(text, "true"))
	case ValueString:
		return String(text)
	}
	return String(text)
}

func (vm *VM) runtimeError(thrown Value) error {
	return runtimeError(thrown, vm.callStack)
}

func runtimeError(thrown Value, stack []callFrame) error {
	message := "unhandled exception"
	errorType := "Exception"
	thrown = annotateException(thrown, stack)
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
		if (typeName == "Decimal" || typeName == "Double") && explicit.Kind == ValueInt {
			return Decimal(float64(explicit.Int))
		}
		if strings.HasPrefix(typeName, "List<") || strings.HasPrefix(typeName, "Set<") || strings.HasPrefix(typeName, "Map<") {
			if coerced, err := coerceCollectionValue(typeName, explicit); err == nil {
				return coerced
			}
		}
		return explicit
	}
	switch typeName {
	case "Integer", "Long":
		return Int(0)
	case "Decimal", "Double":
		return Decimal(0)
	case "Boolean":
		return Bool(false)
	case "String":
		return Null
	default:
		return Null
	}
}

func (vm *VM) stackFrames() []StackFrame {
	return stackFrames(vm.rawStackFrames())
}

func (vm *VM) rawStackFrames() []callFrame {
	frames := append([]callFrame(nil), vm.callStack...)
	if vm.hasStatement && vm.currentStatement.Line > 0 {
		frames = append(frames, vm.currentStatement)
	}
	return frames
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
	if method.Unsupported != "" {
		return Null, fmt.Errorf("%s is not supported by the local VM: %s", method.Name, method.Unsupported)
	}
	if methodHasModifier(method.Modifiers, "abstract") {
		return Null, fmt.Errorf("cannot execute abstract method %s", method.Name)
	}
	frame := make(map[string]Value, len(method.Params))
	frameTypes := make(map[string]string, len(method.Params))
	for i, param := range method.Params {
		paramType := vm.resolveTypeNameInClass(method.ClassName, param.Type)
		coerced, err := vm.coerceAssignable(paramType, args[i])
		if err != nil {
			return Null, fmt.Errorf("%s parameter %s: %w", method.Name, param.Name, err)
		}
		frame[param.Name] = coerced
		frameTypes[param.Name] = paramType
	}
	if receiver.Kind != ValueNull {
		frame["this"] = receiver
	}
	caller := vm.Globals
	callerTypes := vm.VarTypes
	callerClass := vm.currentClass
	callerMethod := vm.currentMethod
	vm.Globals = frame
	vm.VarTypes = frameTypes
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
		vm.VarTypes = callerTypes
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
		stack := out.thrownStack
		if len(stack) == 0 {
			stack = vm.rawStackFrames()
		}
		return Null, &apexThrowError{value: out.thrown, stack: stack}
	}
	if out.signal == signalBreak || out.signal == signalContinue {
		return Null, fmt.Errorf("%s outside loop", out.signal)
	}
	value := out.value
	if out.signal != signalReturn {
		if method.ReturnType != "" && method.ReturnType != "void" {
			return Null, fmt.Errorf("%s must return %s", method.Name, method.ReturnType)
		}
		value = Null
	}
	if method.ReturnType != "" && method.ReturnType != "void" {
		coerced, err := vm.coerceAssignable(method.ReturnType, value)
		if err != nil {
			return Null, fmt.Errorf("%s return: %w", method.Name, err)
		}
		value = coerced
	}
	return value, nil
}

func methodHasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(strings.TrimPrefix(modifier, "@"), expected) {
			return true
		}
	}
	return false
}

func (vm *VM) displayString(value Value, result *Result) (string, error) {
	if value.Kind != ValueObject {
		return value.String(), nil
	}
	if value.Type == "LoggingLevel" && isLoggingLevelName(value.Text) {
		return value.Text, nil
	}
	if value.Type == "RoundingMode" && isDecimalRoundingModeName(value.Text) {
		return value.Text, nil
	}
	if class, ok := vm.Classes[value.Type]; ok && len(class.EnumValues) > 0 && value.Text != "" {
		return value.Text, nil
	}
	if value.Text != "" && strings.Contains(value.Type, ".") {
		return value.Text, nil
	}
	if isExceptionType(value.Type) {
		return exceptionToString(value), nil
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(value.Type, "toString", nil)
	if ambiguous {
		return "", fmt.Errorf("ambiguous overload for call %q", "toString")
	}
	if !ok {
		return value.String(), nil
	}
	out, err := vm.callMethodWithReceiver(target, value, nil, result)
	if err != nil {
		return "", err
	}
	if out.Kind != ValueString {
		return "", fmt.Errorf("%s returned %s, want String", target.Name, out.Kind)
	}
	return out.Text, nil
}

func (vm *VM) callMember(callee string, args []Value, result *Result) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) < 2 {
		return Null, false, nil
	}
	receiverName, method := parts[0], parts[1]
	if receiverName == "super" {
		if len(parts) != 2 {
			return Null, false, nil
		}
		receiver, ok := vm.Globals["this"]
		if !ok || receiver.Kind != ValueObject {
			return Null, true, fmt.Errorf("super call requires instance receiver")
		}
		dispatchClass := vm.currentClass
		if dispatchClass == "" {
			dispatchClass = receiver.Type
		}
		class := vm.Classes[dispatchClass]
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(class.SuperClass, method, args)
		if ambiguous {
			return Null, true, fmt.Errorf("ambiguous overload for call %q", callee)
		}
		if !ok {
			return Null, true, unsupportedCallError(callee)
		}
		if err := vm.checkMemberAccess(target.ClassName, target.Access, target.Name, target.Modifiers); err != nil {
			return Null, true, err
		}
		value, err := vm.callMethodWithReceiver(target, receiver, args, result)
		return value, true, err
	}
	if len(parts) > 2 {
		receiverName = strings.Join(parts[:len(parts)-1], ".")
		method = parts[len(parts)-1]
		if method == "addError" {
			value, handled, err := vm.callSObjectFieldAddError(parts[:len(parts)-1], args)
			if handled || err != nil {
				return value, true, err
			}
		}
		receiver, err := vm.lookup(receiverName)
		if err != nil {
			if _, ok := vm.Globals[parts[0]]; ok {
				return Null, true, err
			}
			return Null, false, nil
		}
		return vm.callValueMember(receiverName, receiver, method, args, result)
	}
	receiver, ok := vm.Globals[receiverName]
	if !ok {
		if vm.currentClass != "" {
			if field, owner, ok := vm.lookupStaticField(vm.currentClass, receiverName); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+receiverName); err != nil {
					return Null, true, err
				}
				return vm.callValueMember(receiverName, field.Value, method, args, result)
			}
		}
		if receiverName == "" {
			return Null, false, nil
		}
		if thisValue, hasThis := vm.Globals["this"]; hasThis && thisValue.Kind == ValueObject {
			if _, hasField := thisValue.Fields[receiverName]; hasField {
				receiver, err := vm.lookup(receiverName)
				if err != nil {
					if !strings.Contains(err.Error(), "unknown variable") {
						return Null, true, err
					}
					return Null, false, nil
				}
				return vm.callValueMember(receiverName, receiver, method, args, result)
			}
		}
		if !unicode.IsLower([]rune(receiverName)[0]) {
			return Null, false, nil
		}
		var err error
		receiver, err = vm.lookup(receiverName)
		if err != nil {
			return Null, false, nil
		}
		return vm.callValueMember(receiverName, receiver, method, args, result)
	}
	return vm.callValueMember(receiverName, receiver, method, args, result)
}

func (vm *VM) callSObjectFieldAddError(path []string, args []Value) (Value, bool, error) {
	if len(path) != 2 {
		return Null, false, nil
	}
	root, ok := vm.Globals[path[0]]
	if !ok || root.Kind != ValueObject || !vm.isSObjectType(root.Type) {
		return Null, false, nil
	}
	field := vm.resolveSObjectFieldName(root.Type, path[1])
	if !vm.sObjectFieldExists(root.Type, field) {
		return Null, false, nil
	}
	message, err := sObjectAddErrorMessage(args, "SObject field addError")
	if err != nil {
		return Null, true, err
	}
	addSObjectError(&root, message, []string{field})
	vm.Globals[path[0]] = root
	return Null, true, nil
}

func (vm *VM) callValueMember(receiverName string, receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if receiver.Kind == ValueNull {
		return Null, true, newExceptionError("NullPointerException", "Attempt to de-reference a null object")
	}
	if receiverType := vm.VarTypes[receiverName]; strings.EqualFold(receiverType, "Id") {
		if value, handled, err := vm.callIdMember(receiver, method, args); handled || err != nil {
			return value, true, err
		}
	}
	if value, updated, mutated, ok, err := callStdlibMember(receiver, method, args); ok || err != nil {
		if mutated {
			if err := vm.storeReceiver(receiverName, updated); err != nil {
				return Null, true, err
			}
		}
		return value, true, err
	}
	if receiver.Kind != ValueObject {
		if value, handled, err := callObjectMember(receiver, method, args); handled || err != nil {
			return value, true, err
		}
	}
	if receiver.Kind == ValueObject {
		if isStubProxy(receiver) {
			value, handled, err := vm.callStubProxyMember(receiver, method, args, result)
			if handled || err != nil {
				return value, true, err
			}
		}
		if value, handled, err := vm.callEnumMember(receiver, method, args); handled || err != nil {
			return value, true, err
		}
		if vm.isSObjectLikeType(receiver.Type) {
			if value, handled, err := vm.callSObjectMember(receiver, method, args); handled || err != nil {
				if method == "put" || method == "addError" || method == "clear" {
					if err := vm.storeReceiver(receiverName, receiver); err != nil {
						return Null, true, err
					}
				}
				return value, true, err
			}
		}
		if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args, result); handled || err != nil {
			if mutated {
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(receiver.Type, method, args)
		if ambiguous {
			return Null, true, fmt.Errorf("ambiguous overload for call %q", memberCallName(receiverName, receiver.Type, method))
		}
		if !ok {
			if value, handled, err := callObjectMember(receiver, method, args); handled || err != nil {
				return value, true, err
			}
			return Null, true, unsupportedCallError(memberCallName(receiverName, receiver.Type, method))
		}
		if err := vm.checkMemberAccess(target.ClassName, target.Access, target.Name, target.Modifiers); err != nil {
			return Null, true, err
		}
		value, err := vm.callMethodWithReceiver(target, receiver, args, result)
		return value, true, err
	}

	switch receiver.Kind {
	case ValueList:
		switch method {
		case "add":
			if len(args) != 1 && len(args) != 2 {
				return Null, true, fmt.Errorf("List.add expects 1 or 2 arguments")
			}
			valueArg := args[0]
			insertAt := -1
			if len(args) == 2 {
				if args[0].Kind != ValueInt {
					return Null, true, fmt.Errorf("List.add index expects Integer")
				}
				insertAt = int(args[0].Int)
				if insertAt < 0 || insertAt > len(receiver.List) {
					return Null, true, fmt.Errorf("List index out of bounds: %d", insertAt)
				}
				valueArg = args[1]
			}
			item, err := vm.coerceCollectionElement(receiver.Type, valueArg)
			if err != nil {
				return Null, true, fmt.Errorf("List.add: %w", err)
			}
			if insertAt >= 0 {
				receiver.List = append(receiver.List, Null)
				copy(receiver.List[insertAt+1:], receiver.List[insertAt:])
				receiver.List[insertAt] = item
			} else {
				receiver.List = append(receiver.List, item)
			}
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			if insertAt >= 0 {
				return Null, true, nil
			}
			return Bool(true), true, nil
		case "addAll":
			if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
				return Null, true, fmt.Errorf("List.addAll expects List or Set")
			}
			values := collectionMembers(args[0])
			for _, value := range values {
				item, err := vm.coerceCollectionElement(receiver.Type, value)
				if err != nil {
					return Null, true, fmt.Errorf("List.addAll: %w", err)
				}
				receiver.List = append(receiver.List, item)
			}
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			return Null, true, nil
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
		case "indexOf":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("List.indexOf expects 1 argument")
			}
			for i, value := range receiver.List {
				if value.Equal(args[0]) {
					return Int(int64(i)), true, nil
				}
			}
			return Int(-1), true, nil
		case "clone":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("List.clone expects 0 arguments")
			}
			cloned := receiver
			cloned.List = append([]Value(nil), receiver.List...)
			return cloned, true, nil
		case "deepClone":
			if len(args) != 0 {
				return Null, true, unsupportedCallError("List.deepClone with preserve options")
			}
			return cloneValue(receiver), true, nil
		case "iterator":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("List.iterator expects 0 arguments")
			}
			return collectionIterator(receiver), true, nil
		case "sort":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("List.sort expects 0 arguments")
			}
			sorted := append([]Value(nil), receiver.List...)
			if err := sortComparableValues(sorted); err != nil {
				return Null, true, err
			}
			receiver.List = sorted
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			return Null, true, nil
		case "remove":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, true, fmt.Errorf("List.remove expects integer index")
			}
			i := int(args[0].Int)
			if i < 0 || i >= len(receiver.List) {
				return Null, true, fmt.Errorf("List index out of bounds: %d", i)
			}
			removed := receiver.List[i]
			receiver.List = append(receiver.List[:i], receiver.List[i+1:]...)
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			return removed, true, nil
		case "set":
			if len(args) != 2 || args[0].Kind != ValueInt {
				return Null, true, fmt.Errorf("List.set expects integer index and value")
			}
			i := int(args[0].Int)
			if i < 0 || i >= len(receiver.List) {
				return Null, true, fmt.Errorf("List index out of bounds: %d", i)
			}
			item, err := vm.coerceCollectionElement(receiver.Type, args[1])
			if err != nil {
				return Null, true, fmt.Errorf("List.set: %w", err)
			}
			receiver.List[i] = item
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			return Null, true, nil
		}
	case ValueSet:
		switch method {
		case "add":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Set.add expects 1 argument")
			}
			item, err := vm.coerceCollectionElement(receiver.Type, args[0])
			if err != nil {
				return Null, true, fmt.Errorf("Set.add: %w", err)
			}
			if !containsValue(receiver.Set, item) {
				receiver.Set = append(receiver.Set, item)
				if err := vm.storeReceiver(receiverName, receiver); err != nil {
					return Null, true, err
				}
				return Bool(true), true, nil
			}
			return Bool(false), true, nil
		case "addAll":
			if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
				return Null, true, fmt.Errorf("Set.addAll expects List or Set")
			}
			changed := false
			for _, value := range collectionMembers(args[0]) {
				item, err := vm.coerceCollectionElement(receiver.Type, value)
				if err != nil {
					return Null, true, fmt.Errorf("Set.addAll: %w", err)
				}
				if !containsValue(receiver.Set, item) {
					receiver.Set = append(receiver.Set, item)
					changed = true
				}
			}
			if changed {
				if err := vm.storeReceiver(receiverName, receiver); err != nil {
					return Null, true, err
				}
			}
			return Bool(changed), true, nil
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
		case "containsAll":
			if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
				return Null, true, fmt.Errorf("Set.containsAll expects List or Set")
			}
			for _, value := range collectionMembers(args[0]) {
				if !containsValue(receiver.Set, value) {
					return Bool(false), true, nil
				}
			}
			return Bool(true), true, nil
		case "remove":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Set.remove expects 1 argument")
			}
			for i, value := range receiver.Set {
				if value.Equal(args[0]) {
					receiver.Set = append(receiver.Set[:i], receiver.Set[i+1:]...)
					if err := vm.storeReceiver(receiverName, receiver); err != nil {
						return Null, true, err
					}
					return Bool(true), true, nil
				}
			}
			return Bool(false), true, nil
		case "removeAll":
			if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
				return Null, true, fmt.Errorf("Set.removeAll expects List or Set")
			}
			changed := false
			out := receiver.Set[:0]
			remove := collectionMembers(args[0])
			for _, value := range receiver.Set {
				if containsValue(remove, value) {
					changed = true
					continue
				}
				out = append(out, value)
			}
			receiver.Set = out
			if changed {
				if err := vm.storeReceiver(receiverName, receiver); err != nil {
					return Null, true, err
				}
			}
			return Bool(changed), true, nil
		case "retainAll":
			if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
				return Null, true, fmt.Errorf("Set.retainAll expects List or Set")
			}
			changed := false
			keep := collectionMembers(args[0])
			out := receiver.Set[:0]
			for _, value := range receiver.Set {
				if containsValue(keep, value) {
					out = append(out, value)
					continue
				}
				changed = true
			}
			receiver.Set = out
			if changed {
				if err := vm.storeReceiver(receiverName, receiver); err != nil {
					return Null, true, err
				}
			}
			return Bool(changed), true, nil
		case "clone":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Set.clone expects 0 arguments")
			}
			cloned := receiver
			cloned.Set = append([]Value(nil), receiver.Set...)
			return cloned, true, nil
		case "deepClone":
			if len(args) != 0 {
				return Null, true, unsupportedCallError("Set.deepClone with preserve options")
			}
			return cloneValue(receiver), true, nil
		case "iterator":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Set.iterator expects 0 arguments")
			}
			return collectionIterator(receiver), true, nil
		}
	case ValueMap:
		switch method {
		case "put":
			if len(args) != 2 {
				return Null, true, fmt.Errorf("Map.put expects 2 arguments")
			}
			key, item, err := vm.coerceMapEntry(receiver.Type, args[0], args[1])
			if err != nil {
				return Null, true, fmt.Errorf("Map.put: %w", err)
			}
			previous := Null
			if existing, ok := receiver.Map[mapKey(key)]; ok {
				previous = existing
			}
			receiver.Map[mapKey(key)] = item
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			return previous, true, nil
		case "putAll":
			if len(args) != 1 || (args[0].Kind != ValueMap && args[0].Kind != ValueList) {
				return Null, true, fmt.Errorf("Map.putAll expects Map or List")
			}
			if args[0].Kind == ValueList {
				updated, err := vm.putAllSObjectList(receiver, args[0])
				if err != nil {
					return Null, true, err
				}
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
				return Null, true, nil
			}
			for rawKey, value := range args[0].Map {
				keyValue := valueFromMapKey(rawKey)
				key, item, err := vm.coerceMapEntry(receiver.Type, keyValue, value)
				if err != nil {
					return Null, true, fmt.Errorf("Map.putAll: %w", err)
				}
				receiver.Map[mapKey(key)] = item
			}
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
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
		case "containsValue":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Map.containsValue expects 1 argument")
			}
			for _, value := range receiver.Map {
				if value.Equal(args[0]) {
					return Bool(true), true, nil
				}
			}
			return Bool(false), true, nil
		case "keySet":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Map.keySet expects 0 arguments")
			}
			out := Set()
			for _, rawKey := range sortedMapKeys(receiver.Map) {
				out.Set = append(out.Set, valueFromMapKey(rawKey))
			}
			if keyType, _, ok := mapTypeArgs(receiver.Type); ok {
				out.Type = "Set<" + keyType + ">"
			}
			return out, true, nil
		case "values":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Map.values expects 0 arguments")
			}
			out := List()
			for _, key := range sortedMapKeys(receiver.Map) {
				out.List = append(out.List, receiver.Map[key])
			}
			if _, valueType, ok := mapTypeArgs(receiver.Type); ok {
				out.Type = "List<" + valueType + ">"
			}
			return out, true, nil
		case "size":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Map.size expects 0 arguments")
			}
			return Int(int64(len(receiver.Map))), true, nil
		case "clone":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("Map.clone expects 0 arguments")
			}
			cloned := receiver
			cloned.Map = make(map[string]Value, len(receiver.Map))
			for key, value := range receiver.Map {
				cloned.Map[key] = value
			}
			return cloned, true, nil
		case "deepClone":
			if len(args) != 0 {
				return Null, true, unsupportedCallError("Map.deepClone with preserve options")
			}
			return cloneValue(receiver), true, nil
		}
	}
	return Null, true, unsupportedCallError(memberCallName(receiverName, receiver.Type, method))
}

func memberCallName(receiverName, receiverType, method string) string {
	if receiverName != "" {
		return receiverName + "." + method
	}
	if receiverType != "" {
		return receiverType + "." + method
	}
	return "." + method
}

func sObjectAddErrorMessage(args []Value, name string) (string, error) {
	if len(args) != 1 && len(args) != 2 {
		return "", fmt.Errorf("%s expects message and optional escapeHtml", name)
	}
	if len(args) == 2 && args[1].Kind != ValueBool {
		return "", fmt.Errorf("%s escapeHtml expects Boolean", name)
	}
	message := args[0].String()
	if args[0].Kind == ValueObject {
		if value, ok := args[0].Fields["message"]; ok {
			message = value.String()
		}
	}
	return message, nil
}

func (vm *VM) sObjectFieldExists(typeName, field string) bool {
	if vm.Org == nil {
		return true
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, typeName)
	if !ok {
		return true
	}
	if field == "Id" {
		return true
	}
	_, ok = storage.ResolveFieldName(vm.Org.Objects[objectName].Definition, vm.Org.Namespace, field)
	return ok
}

func (vm *VM) storeReceiver(receiverName string, value Value) error {
	if receiverName == "" {
		return nil
	}
	if strings.Contains(receiverName, ".") {
		return vm.assign(receiverName, value)
	}
	vm.Globals[receiverName] = value
	return nil
}

func (vm *VM) isSObjectType(typeName string) bool {
	if vm.Org == nil {
		return false
	}
	_, ok := storage.ResolveObjectName(*vm.Org, typeName)
	return ok
}

func (vm *VM) isSObjectLikeType(typeName string) bool {
	if strings.EqualFold(typeName, "sObject") {
		return true
	}
	if isCommonSObjectTypeName(typeName) || strings.HasSuffix(typeName, "__c") || strings.HasSuffix(typeName, "__e") || strings.HasSuffix(typeName, "__mdt") {
		return true
	}
	return vm.isSObjectType(typeName)
}

func isStubProxy(receiver Value) bool {
	if receiver.Kind != ValueObject {
		return false
	}
	provider, ok := receiver.Fields["__oaerStubProvider"]
	return ok && provider.Kind == ValueObject
}

func (vm *VM) callStubProxyMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(receiver.Type, method, args)
	if ambiguous {
		return Null, true, fmt.Errorf("ambiguous overload for stubbed call %q", receiver.Type+"."+method)
	}
	if !ok {
		return Null, true, unsupportedCallError("Test.createStub dynamic method " + receiver.Type + "." + method + " without local target method metadata")
	}
	provider := receiver.Fields["__oaerStubProvider"]
	paramTypes := make([]Value, 0, len(target.Params))
	paramNames := make([]Value, 0, len(target.Params))
	for _, param := range target.Params {
		paramTypes = append(paramTypes, platformScalar("Type", param.Type))
		paramNames = append(paramNames, String(param.Name))
	}
	returnType := target.ReturnType
	if returnType == "" {
		returnType = "Object"
	}
	metadataArgs := []Value{
		receiver,
		String(method),
		platformScalar("Type", returnType),
		{Kind: ValueList, Type: "List<Type>", List: paramTypes},
		{Kind: ValueList, Type: "List<String>", List: paramNames},
		{Kind: ValueList, Type: "List<Object>", List: append([]Value(nil), args...)},
	}
	handler, ok, ambiguous := vm.resolveInstanceMethodForArgs(provider.Type, "handleMethodCall", metadataArgs)
	if ambiguous {
		return Null, true, fmt.Errorf("ambiguous overload for call %q", provider.Type+".handleMethodCall")
	}
	if !ok {
		return Null, true, fmt.Errorf("StubProvider %s must implement handleMethodCall", provider.Type)
	}
	value, err := vm.callMethodWithReceiver(handler, provider, metadataArgs, result)
	if err != nil {
		return Null, true, err
	}
	if target.ReturnType == "" || strings.EqualFold(target.ReturnType, "void") {
		return Null, true, nil
	}
	coerced, err := vm.coerceAssignable(target.ReturnType, value)
	if err != nil {
		return Null, true, fmt.Errorf("stubbed %s.%s return: %w", receiver.Type, method, err)
	}
	return coerced, true, nil
}

func (vm *VM) callSObjectMember(receiver Value, method string, args []Value) (Value, bool, error) {
	switch method {
	case "addError":
		if reason, ok := sobjectReadOnlyReason(receiver); ok {
			return Null, true, fmt.Errorf("cannot modify read-only %s", reason)
		}
		message, err := sObjectAddErrorMessage(args, "SObject.addError")
		if err != nil {
			return Null, true, err
		}
		addSObjectError(&receiver, message, nil)
		return Null, true, nil
	case "hasErrors":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.hasErrors expects 0 arguments")
		}
		return Bool(len(sobjectErrors(receiver)) > 0), true, nil
	case "getErrors":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.getErrors expects 0 arguments")
		}
		return List(sobjectErrors(receiver)...), true, nil
	case "get":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("SObject.get expects field name String or Schema.SObjectField")
		}
		fieldArg, err := vm.sObjectFieldArg(receiver.Type, args[0])
		if err != nil {
			return Null, true, fmt.Errorf("SObject.get expects field name String or Schema.SObjectField")
		}
		field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
		value, ok := receiver.Fields[field]
		if !ok {
			return Null, true, nil
		}
		return value, true, nil
	case "put":
		if len(args) != 2 {
			return Null, true, fmt.Errorf("SObject.put expects field name String or Schema.SObjectField and value")
		}
		fieldArg, err := vm.sObjectFieldArg(receiver.Type, args[0])
		if err != nil {
			return Null, true, fmt.Errorf("SObject.put expects field name String or Schema.SObjectField and value")
		}
		if reason, ok := sobjectReadOnlyReason(receiver); ok {
			return Null, true, fmt.Errorf("cannot modify read-only %s", reason)
		}
		field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
		previous, ok := receiver.Fields[field]
		if !ok {
			previous = Null
		}
		receiver.Fields[field] = args[1]
		return previous, true, nil
	case "isSet":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("SObject.isSet expects field name String or Schema.SObjectField")
		}
		fieldArg, err := vm.sObjectFieldArg(receiver.Type, args[0])
		if err != nil {
			return Null, true, fmt.Errorf("SObject.isSet expects field name String or Schema.SObjectField")
		}
		field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
		_, ok := receiver.Fields[field]
		return Bool(ok), true, nil
	case "clear":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.clear expects 0 arguments")
		}
		if reason, ok := sobjectReadOnlyReason(receiver); ok {
			return Null, true, fmt.Errorf("cannot modify read-only %s", reason)
		}
		for field := range receiver.Fields {
			delete(receiver.Fields, field)
		}
		return Null, true, nil
	case "getPopulatedFieldsAsMap":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.getPopulatedFieldsAsMap expects 0 arguments")
		}
		out := Map()
		out.Type = "Map<String,Object>"
		for field, value := range receiver.Fields {
			if field == sobjectErrorsField || field == sobjectReadOnlyField {
				continue
			}
			out.Map[mapKey(String(field))] = value
		}
		return out, true, nil
	default:
		return Null, false, nil
	}
}

const (
	sobjectErrorsField   = "__oaer_errors"
	sobjectReadOnlyField = "__oaer_readonly"
)

func sobjectReadOnlyReason(value Value) (string, bool) {
	if value.Kind != ValueObject {
		return "", false
	}
	reason, ok := value.Fields[sobjectReadOnlyField]
	if !ok || reason.Kind != ValueString || reason.Text == "" {
		return "", false
	}
	return reason.Text, true
}

func addSObjectError(value *Value, message string, fields []string) {
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	errorValue := Object("Database.Error")
	errorValue.Fields["message"] = String(message)
	errorValue.Fields["statusCode"] = String("FIELD_CUSTOM_VALIDATION_EXCEPTION")
	fieldsList := List()
	for _, field := range fields {
		fieldsList.List = append(fieldsList.List, String(field))
	}
	errorValue.Fields["fields"] = fieldsList
	errorsList, ok := value.Fields[sobjectErrorsField]
	if !ok || errorsList.Kind != ValueList {
		errorsList = List()
	}
	errorsList.List = append(errorsList.List, errorValue)
	value.Fields[sobjectErrorsField] = errorsList
}

func sobjectErrors(value Value) []Value {
	errorsList, ok := value.Fields[sobjectErrorsField]
	if !ok || errorsList.Kind != ValueList {
		return nil
	}
	return append([]Value(nil), errorsList.List...)
}

func dmlResultsFromSObjectErrors(records []storage.Record, values []Value) []dml.Result {
	results := make([]dml.Result, len(records))
	for i, value := range values {
		if i >= len(results) {
			break
		}
		errors := sobjectErrors(value)
		if len(errors) == 0 {
			continue
		}
		dmlErrors := make([]dml.Error, 0, len(errors))
		messages := make([]string, 0, len(errors))
		aggregateFields := make([]string, 0, len(errors))
		for _, errValue := range errors {
			dmlError := dml.Error{
				Message:    "record blocked by addError",
				StatusCode: "FIELD_CUSTOM_VALIDATION_EXCEPTION",
			}
			if errValue.Kind == ValueObject {
				if value, ok := errValue.Fields["message"]; ok {
					dmlError.Message = value.String()
				}
				if value, ok := errValue.Fields["statusCode"]; ok {
					dmlError.StatusCode = value.String()
				}
				if value, ok := errValue.Fields["fields"]; ok && value.Kind == ValueList {
					for _, field := range value.List {
						dmlError.Fields = append(dmlError.Fields, field.String())
					}
				}
			}
			messages = append(messages, dmlError.Message)
			aggregateFields = append(aggregateFields, dmlError.Fields...)
			dmlErrors = append(dmlErrors, dmlError)
		}
		results[i] = dml.Result{
			ID:         records[i].ID,
			Success:    false,
			Error:      strings.Join(messages, "; "),
			StatusCode: dmlErrors[0].StatusCode,
			Fields:     aggregateFields,
			Errors:     dmlErrors,
		}
	}
	return results
}

func databaseErrorsList(result dml.Result) Value {
	errors := dmlResultErrors(result)
	values := make([]Value, 0, len(errors))
	for _, err := range errors {
		values = append(values, databaseErrorValue(err))
	}
	return List(values...)
}

func dmlResultErrors(result dml.Result) []dml.Error {
	if len(result.Errors) > 0 {
		out := make([]dml.Error, len(result.Errors))
		copy(out, result.Errors)
		return out
	}
	if result.Error == "" {
		return nil
	}
	code := result.StatusCode
	if code == "" {
		code = "FIELD_CUSTOM_VALIDATION_EXCEPTION"
	}
	return []dml.Error{{
		Message:    result.Error,
		StatusCode: code,
		Fields:     append([]string(nil), result.Fields...),
	}}
}

func databaseErrorValue(err dml.Error) Value {
	value := Object("Database.Error")
	value.Fields["message"] = String(err.Message)
	code := err.StatusCode
	if code == "" {
		code = "FIELD_CUSTOM_VALIDATION_EXCEPTION"
	}
	value.Fields["statusCode"] = String(code)
	fields := List()
	for _, field := range err.Fields {
		fields.List = append(fields.List, String(field))
	}
	value.Fields["fields"] = fields
	value.Fields["extendedErrorDetails"] = List()
	return value
}

func dmlExceptionDetail(receiver Value, method string, args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueInt {
		return Null, fmt.Errorf("%s.%s expects Integer index", receiver.Type, method)
	}
	details, ok := receiver.Fields["__dmlErrors"]
	if !ok || details.Kind != ValueList {
		return Null, fmt.Errorf("%s.%s index out of bounds: %d", receiver.Type, method, args[0].Int)
	}
	index := int(args[0].Int)
	if index < 0 || index >= len(details.List) {
		return Null, fmt.Errorf("%s.%s index out of bounds: %d", receiver.Type, method, args[0].Int)
	}
	detail := details.List[index]
	if detail.Kind != ValueObject {
		return Null, fmt.Errorf("%s.%s detail is not available: %d", receiver.Type, method, args[0].Int)
	}
	return detail, nil
}

func (vm *VM) resolveSObjectFieldName(typeName, field string) string {
	if vm.Org == nil {
		return field
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, typeName)
	if !ok {
		return storage.StripNamespaceToken(vm.Org.Namespace, field)
	}
	if canonical, ok := storage.ResolveFieldName(vm.Org.Objects[objectName].Definition, vm.Org.Namespace, field); ok {
		return canonical
	}
	return storage.StripNamespaceToken(vm.Org.Namespace, field)
}

func (vm *VM) hasSObjectField(typeName, field string) bool {
	if vm.Org == nil {
		return false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, typeName)
	if !ok {
		return false
	}
	_, ok = storage.ResolveFieldName(vm.Org.Objects[objectName].Definition, vm.Org.Namespace, field)
	return ok
}

func (vm *VM) sObjectFieldArg(receiverType string, value Value) (string, error) {
	if value.Kind == ValueString {
		return value.Text, nil
	}
	if value.Kind == ValueObject && value.Type == "Schema.SObjectField" {
		if objectValue, ok := value.Fields["object"]; ok && objectValue.Kind == ValueString && receiverType != "" && !strings.EqualFold(receiverType, "SObject") {
			if vm.Org != nil {
				if receiverObject, ok := storage.ResolveObjectName(*vm.Org, receiverType); ok {
					if tokenObject, ok := storage.ResolveObjectName(*vm.Org, objectValue.Text); ok && tokenObject != receiverObject {
						return "", fmt.Errorf("field token belongs to %s, not %s", objectValue.Text, receiverType)
					}
				}
			}
		}
		field, ok := value.Fields["field"]
		if !ok || field.Kind != ValueString {
			return "", fmt.Errorf("field token missing field name")
		}
		return field.Text, nil
	}
	return "", fmt.Errorf("expected field name")
}

func apexPagesSeverityStaticValue(name string) (Value, bool) {
	if !strings.HasPrefix(name, "ApexPages.Severity.") {
		return Null, false
	}
	severity := strings.TrimPrefix(name, "ApexPages.Severity.")
	for i, candidate := range apexPagesSeverityNames {
		if severity == candidate {
			return Value{Kind: ValueObject, Type: "ApexPages.Severity", Text: severity, Fields: map[string]Value{"ordinal": Int(int64(i))}}, true
		}
	}
	return Null, false
}

func apexPagesSeverityValues(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("ApexPages.Severity.values expects 0 arguments")
	}
	values := make([]Value, 0, len(apexPagesSeverityNames))
	for i, name := range apexPagesSeverityNames {
		value := Value{Kind: ValueObject, Type: "ApexPages.Severity", Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func loggingLevelValues(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("LoggingLevel.values expects 0 arguments")
	}
	values := make([]Value, 0, len(loggingLevelNames))
	for i, name := range loggingLevelNames {
		value := Value{Kind: ValueObject, Type: "LoggingLevel", Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func (vm *VM) callEnumStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	if typeName == "LoggingLevel" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := loggingLevelValues(args)
		return value, true, err
	}
	if typeName == "RoundingMode" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := roundingModeValues(args)
		return value, true, err
	}
	class, ok := vm.Classes[typeName]
	if !ok || len(class.EnumValues) == 0 || method != "values" {
		return Null, false, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.values expects 0 arguments", typeName)
	}
	values := make([]Value, 0, len(class.EnumValues))
	for i, name := range class.EnumValues {
		value := Value{Kind: ValueObject, Type: class.Name, Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), true, nil
}

func roundingModeValues(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("RoundingMode.values expects 0 arguments")
	}
	values := make([]Value, 0, len(roundingModeNames))
	for i, name := range roundingModeNames {
		value := Value{Kind: ValueObject, Type: "RoundingMode", Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func (vm *VM) callEnumMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if receiver.Type == "JSONToken" {
		if len(args) != 0 {
			return Null, true, fmt.Errorf("JSONToken.%s expects 0 arguments", method)
		}
		switch method {
		case "name", "toString":
			return String(receiver.Text), true, nil
		case "ordinal":
			for i, name := range jsonTokenNames {
				if name == receiver.Text {
					return Int(int64(i)), true, nil
				}
			}
			return Int(-1), true, nil
		default:
			return Null, false, nil
		}
	}
	if receiver.Type == "ApexPages.Severity" {
		return callNamedEnumMember("ApexPages.Severity", apexPagesSeverityNames, receiver, method, args)
	}
	if receiver.Type == "LoggingLevel" {
		return callNamedEnumMember("LoggingLevel", loggingLevelNames, receiver, method, args)
	}
	if receiver.Type == "RoundingMode" {
		return callNamedEnumMember("RoundingMode", roundingModeNames, receiver, method, args)
	}
	class, ok := vm.Classes[receiver.Type]
	if !ok || len(class.EnumValues) == 0 {
		return Null, false, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "name":
		return String(receiver.Text), true, nil
	case "ordinal":
		for i, name := range class.EnumValues {
			if name == receiver.Text {
				return Int(int64(i)), true, nil
			}
		}
		return Int(-1), true, nil
	default:
		return Null, false, nil
	}
}

func callNamedEnumMember(typeName string, names []string, receiver Value, method string, args []Value) (Value, bool, error) {
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		for i, name := range names {
			if name == receiver.Text {
				return Int(int64(i)), true, nil
			}
		}
		return Int(-1), true, nil
	default:
		return Null, false, nil
	}
}

func (vm *VM) callPlatformObjectMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	if isExceptionType(receiver.Type) {
		switch method {
		case "getMessage":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getMessage expects 0 arguments", receiver.Type)
			}
			if message, ok := receiver.Fields["message"]; ok {
				return message, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getNumDml":
			if exceptionTypeName(receiver.Type) != "DmlException" {
				break
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getNumDml expects 0 arguments", receiver.Type)
			}
			details, _ := receiver.Fields["__dmlErrors"]
			if details.Kind != ValueList {
				return Int(0), receiver, false, true, nil
			}
			return Int(int64(len(details.List))), receiver, false, true, nil
		case "getDmlMessage", "getDmlStatusCode", "getDmlFields", "getDmlId", "getDmlIndex":
			if exceptionTypeName(receiver.Type) != "DmlException" {
				break
			}
			detail, err := dmlExceptionDetail(receiver, method, args)
			if err != nil {
				return Null, receiver, false, true, err
			}
			switch method {
			case "getDmlMessage":
				if value, ok := detail.Fields["message"]; ok {
					return value, receiver, false, true, nil
				}
				return String(""), receiver, false, true, nil
			case "getDmlStatusCode":
				if value, ok := detail.Fields["statusCode"]; ok {
					return value, receiver, false, true, nil
				}
				return String("FIELD_CUSTOM_VALIDATION_EXCEPTION"), receiver, false, true, nil
			case "getDmlFields":
				if value, ok := detail.Fields["fields"]; ok {
					return value, receiver, false, true, nil
				}
				return List(), receiver, false, true, nil
			case "getDmlId":
				if value, ok := detail.Fields["id"]; ok {
					return value, receiver, false, true, nil
				}
				return Null, receiver, false, true, nil
			case "getDmlIndex":
				if value, ok := detail.Fields["index"]; ok {
					return value, receiver, false, true, nil
				}
				return Int(-1), receiver, false, true, nil
			}
		case "getCause":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getCause expects 0 arguments", receiver.Type)
			}
			if cause, ok := receiver.Fields["__cause"]; ok {
				return cause, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "initCause":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("%s.initCause expects 1 argument", receiver.Type)
			}
			if args[0].Kind != ValueNull && (args[0].Kind != ValueObject || !isExceptionType(args[0].Type)) {
				return Null, receiver, false, true, fmt.Errorf("%s.initCause expects Exception", receiver.Type)
			}
			if receiver.Equal(args[0]) {
				return Null, receiver, false, true, newExceptionError("IllegalArgumentException", "Self-causation not permitted")
			}
			if initialized, ok := receiver.Fields["__causeInitialized"]; ok && initialized.Kind == ValueBool && initialized.Bool {
				return Null, receiver, false, true, newExceptionError("IllegalStateException", "Can't overwrite cause")
			}
			receiver.Fields["__causeInitialized"] = Bool(true)
			receiver.Fields["__cause"] = args[0]
			return receiver, receiver, true, true, nil
		case "getDescription":
			if exceptionTypeName(receiver.Type) != "PatternSyntaxException" {
				break
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getDescription expects 0 arguments", receiver.Type)
			}
			if description, ok := receiver.Fields["description"]; ok {
				return description, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getIndex":
			if exceptionTypeName(receiver.Type) != "PatternSyntaxException" {
				break
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getIndex expects 0 arguments", receiver.Type)
			}
			if index, ok := receiver.Fields["index"]; ok {
				return index, receiver, false, true, nil
			}
			return Int(-1), receiver, false, true, nil
		case "getPattern":
			if exceptionTypeName(receiver.Type) != "PatternSyntaxException" {
				break
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getPattern expects 0 arguments", receiver.Type)
			}
			if pattern, ok := receiver.Fields["pattern"]; ok {
				return pattern, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getTypeName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getTypeName expects 0 arguments", receiver.Type)
			}
			return String(exceptionTypeName(receiver.Type)), receiver, false, true, nil
		case "getLineNumber":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getLineNumber expects 0 arguments", receiver.Type)
			}
			if line, ok := receiver.Fields["__lineNumber"]; ok {
				return line, receiver, false, true, nil
			}
			return Int(0), receiver, false, true, nil
		case "getStackTraceString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getStackTraceString expects 0 arguments", receiver.Type)
			}
			if stack, ok := receiver.Fields["__stackTrace"]; ok {
				return stack, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		case "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.toString expects 0 arguments", receiver.Type)
			}
			return String(exceptionToString(receiver)), receiver, false, true, nil
		}
	}
	if isIteratorValue(receiver) {
		return callIteratorMember(receiver, method, args)
	}
	switch receiver.Type {
	case "Database.QueryLocator":
		switch method {
		case "getQuery":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.QueryLocator.getQuery expects 0 arguments")
			}
			if query, ok := receiver.Fields["Query"]; ok && query.Kind == ValueString {
				return query, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		case "iterator":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.QueryLocator.iterator expects 0 arguments")
			}
			records, ok := receiver.Fields["Records"]
			if !ok || records.Kind != ValueList {
				return Null, receiver, false, true, fmt.Errorf("Database.QueryLocator missing records")
			}
			iterator := Object("Database.QueryLocatorIterator")
			iterator.Fields["__values"] = List(append([]Value(nil), records.List...)...)
			iterator.Fields["__index"] = Int(0)
			return iterator, receiver, false, true, nil
		}
	case "QueueableContext", "BatchableContext", "Database.BatchableContext":
		if method == "getJobId" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getJobId expects 0 arguments", receiver.Type)
			}
			if jobID, ok := receiver.Fields["JobId"]; ok {
				return jobID, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		}
	case "SchedulableContext":
		if method == "getTriggerId" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("SchedulableContext.getTriggerId expects 0 arguments")
			}
			if triggerID, ok := receiver.Fields["TriggerId"]; ok {
				return triggerID, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		}
	case "System.FinalizerContext", "FinalizerContext":
		switch method {
		case "getAsyncApexJobId", "getRequestId", "getResult", "getException":
			return Null, receiver, false, true, unsupportedCallError(receiver.Type + "." + method + " local queueable finalizers")
		}
	case "AsyncOptions":
		switch method {
		case "getDuplicateSignature", "setDuplicateSignature",
			"getMaximumQueueableStackDepth", "setMaximumQueueableStackDepth",
			"getMinimumQueueableDelayInMinutes", "setMinimumQueueableDelayInMinutes":
			return Null, receiver, false, true, unsupportedCallError("AsyncOptions." + method + " local async options surface")
		}
	case "JSONGenerator":
		return callJSONGeneratorMember(receiver, method, args)
	case "JSONParser":
		return callJSONParserMember(receiver, method, args)
	case "RestRequest":
		return callRestRequestMember(receiver, method, args)
	case "RestResponse":
		return callRestResponseMember(receiver, method, args)
	case "Schema.SObjectType":
		if method == "getDescribe" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.getDescribe expects 0 arguments")
			}
			objectValue, ok := receiver.Fields["object"]
			if !ok || objectValue.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType token missing object")
			}
			if vm.Org == nil {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.getDescribe requires org state")
			}
			objectName, ok := storage.ResolveObjectName(*vm.Org, objectValue.Text)
			if !ok {
				if strings.EqualFold(objectValue.Text, "SObject") {
					return vm.describeSObjectValue("SObject", storage.ObjectDefinition{APIName: "SObject", Label: "SObject", PluralLabel: "SObjects"}), receiver, false, true, nil
				}
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.getDescribe unknown object %s", objectValue.Text)
			}
			appendTrace(result, "apex.describe.sobject", "apex.describe", map[string]any{
				"operation": "SObjectType.getDescribe",
				"object":    objectName,
			})
			return vm.describeSObjectValue(objectName, vm.Org.Objects[objectName].Definition), receiver, false, true, nil
		}
	case "Schema.SObjectFieldMap":
		if method == "getMap" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectFieldMap.getMap expects 0 arguments")
			}
			appendTrace(result, "apex.describe.fields", "apex.describe", map[string]any{"operation": "fields.getMap"})
			return receiver.Fields["map"], receiver, false, true, nil
		}
	case "Schema.SObjectField":
		if method == "getDescribe" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectField.getDescribe expects 0 arguments")
			}
			objectValue, ok := receiver.Fields["object"]
			if !ok || objectValue.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectField token missing object")
			}
			fieldValue, ok := receiver.Fields["field"]
			if !ok || fieldValue.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectField token missing field")
			}
			describe, err := vm.describeFieldValue(objectValue.Text, fieldValue.Text)
			if err != nil {
				return Null, receiver, false, true, err
			}
			appendTrace(result, "apex.describe.field", "apex.describe", map[string]any{
				"operation": "SObjectField.getDescribe",
				"object":    objectValue.Text,
				"field":     fieldValue.Text,
			})
			return describe, receiver, false, true, nil
		}
	case "Schema.DescribeFieldResult":
		switch method {
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getType expects 0 arguments")
			}
			return receiver.Fields["type"], receiver, false, true, nil
		case "isNillable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isNillable expects 0 arguments")
			}
			return receiver.Fields["nillable"], receiver, false, true, nil
		case "isExternalId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isExternalId expects 0 arguments")
			}
			return receiver.Fields["externalId"], receiver, false, true, nil
		case "isUnique":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isUnique expects 0 arguments")
			}
			return receiver.Fields["unique"], receiver, false, true, nil
		case "getReferenceTo":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getReferenceTo expects 0 arguments")
			}
			return receiver.Fields["referenceTo"], receiver, false, true, nil
		case "getRelationshipName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getRelationshipName expects 0 arguments")
			}
			return receiver.Fields["relationshipName"], receiver, false, true, nil
		case "getPicklistValues":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getPicklistValues expects 0 arguments")
			}
			return receiver.Fields["picklistValues"], receiver, false, true, nil
		case "getController", "getControllerValues":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			return Null, receiver, false, true, unsupportedCallError("Schema.DescribeFieldResult." + method + " dependent picklist controller metadata")
		case "isAccessible", "isCreateable", "isUpdateable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			return Bool(true), receiver, false, true, nil
		}
	case "Schema.PicklistEntry":
		switch method {
		case "getValue":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.PicklistEntry.getValue expects 0 arguments")
			}
			return receiver.Fields["value"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.PicklistEntry.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "isDefaultValue":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.PicklistEntry.isDefaultValue expects 0 arguments")
			}
			return receiver.Fields["default"], receiver, false, true, nil
		case "isActive":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.PicklistEntry.isActive expects 0 arguments")
			}
			return receiver.Fields["active"], receiver, false, true, nil
		}
	case "Schema.FieldSetMapUnsupported":
		if method == "getMap" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMap.getMap expects 0 arguments")
			}
			return Null, receiver, false, true, unsupportedCallError("Schema.DescribeSObjectResult.fieldSets local field set metadata")
		}
	case "Pattern":
		return callPatternMember(receiver, method, args)
	case "Matcher":
		return callMatcherMember(receiver, method, args)
	case "Date":
		switch method {
		case "format", "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects 0 arguments", method)
			}
			text, err := platformScalarText(receiver, "Date")
			if err != nil {
				return Null, receiver, false, true, err
			}
			return String(text), receiver, false, true, nil
		case "addDays", "addMonths", "addYears":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects Integer", method)
			}
			date, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			switch method {
			case "addDays":
				date = date.AddDate(0, 0, int(args[0].Int))
			case "addMonths":
				date = addMonthsClamped(date, int(args[0].Int))
			case "addYears":
				date = addMonthsClamped(date, int(args[0].Int)*12)
			}
			return platformScalar("Date", date.Format("2006-01-02")), receiver, false, true, nil
		case "daysBetween":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Date" {
				return Null, receiver, false, true, fmt.Errorf("Date.daysBetween expects Date")
			}
			start, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			end, err := parsePlatformDate(args[0])
			if err != nil {
				return Null, receiver, false, true, err
			}
			return Int(int64(end.Sub(start).Hours() / 24)), receiver, false, true, nil
		case "year", "month", "day":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects 0 arguments", method)
			}
			date, err := parsePlatformDate(receiver)
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
		case "toStartOfMonth", "toEndOfMonth":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects 0 arguments", method)
			}
			date, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			year, month := date.Year(), date.Month()
			if method == "toStartOfMonth" {
				return platformScalar("Date", time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")), receiver, false, true, nil
			}
			return platformScalar("Date", time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")), receiver, false, true, nil
		}
	case "Datetime":
		switch method {
		case "format", "formatGmt", "toString":
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if len(args) == 0 {
				if method == "format" {
					_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
					if !ok {
						return Null, receiver, false, true, unsupportedCallError("Datetime.format timezone " + vm.currentUserTimeZoneID())
					}
					return String(local.Format(time.RFC3339Nano)), receiver, false, true, nil
				}
				return String(formatPlatformDatetime(t)), receiver, false, true, nil
			}
			if method == "toString" {
				return Null, receiver, false, true, fmt.Errorf("Datetime.toString expects 0 arguments")
			}
			if len(args) != 1 && !(method == "format" && len(args) == 2) {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects optional pattern String", method)
			}
			if args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects pattern String", method)
			}
			tzID := "UTC"
			zoneLabel := "UTC"
			offset := time.Duration(0)
			if method == "format" {
				formatTimeZoneID := vm.currentUserTimeZoneID()
				if len(args) == 2 {
					if args[1].Kind != ValueString {
						return Null, receiver, false, true, fmt.Errorf("Datetime.format expects timezone String")
					}
					formatTimeZoneID = args[1].Text
				}
				canonical, parsedOffset, local, label, ok := resolveTimeZoneForInstant(formatTimeZoneID, t)
				if !ok {
					return Null, receiver, false, true, unsupportedCallError("Datetime.format timezone " + formatTimeZoneID)
				}
				tzID = canonical
				offset = parsedOffset
				zoneLabel = label
				t = local
			} else {
				t = t.UTC()
			}
			formatted, err := formatApexDatetimePattern(t, args[0].Text, tzID, zoneLabel, offset)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return String(formatted), receiver, false, true, nil
		case "date", "dateGmt", "dateGMT":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects 0 arguments", method)
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if method == "date" {
				_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
				if !ok {
					return Null, receiver, false, true, unsupportedCallError("Datetime.date timezone " + vm.currentUserTimeZoneID())
				}
				t = local
			} else {
				t = t.UTC()
			}
			return platformScalar("Date", t.Format("2006-01-02")), receiver, false, true, nil
		case "time", "timeGmt", "timeGMT":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects 0 arguments", method)
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if method == "time" {
				_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
				if !ok {
					return Null, receiver, false, true, unsupportedCallError("Datetime.time timezone " + vm.currentUserTimeZoneID())
				}
				t = local
			} else {
				t = t.UTC()
			}
			return platformScalar("Time", formatPlatformTime(t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/int(time.Millisecond))), receiver, false, true, nil
		case "addDays", "addMonths", "addYears", "addHours", "addMinutes", "addSeconds", "addMilliseconds":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects Integer", method)
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			amount := int(args[0].Int)
			switch method {
			case "addDays":
				t = t.AddDate(0, 0, amount)
			case "addMonths":
				t = addMonthsClamped(t, amount)
			case "addYears":
				t = addMonthsClamped(t, amount*12)
			case "addHours":
				t = t.Add(time.Duration(amount) * time.Hour)
			case "addMinutes":
				t = t.Add(time.Duration(amount) * time.Minute)
			case "addSeconds":
				t = t.Add(time.Duration(amount) * time.Second)
			case "addMilliseconds":
				t = t.Add(time.Duration(amount) * time.Millisecond)
			}
			return platformScalar("Datetime", formatPlatformDatetime(t)), receiver, false, true, nil
		case "year", "month", "day", "hour", "minute", "second", "millisecond",
			"yearGmt", "monthGmt", "dayGmt", "hourGmt", "minuteGmt", "secondGmt":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects 0 arguments", method)
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if strings.HasSuffix(method, "Gmt") {
				t = t.UTC()
				method = strings.TrimSuffix(method, "Gmt")
			} else {
				_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
				if !ok {
					return Null, receiver, false, true, unsupportedCallError("Datetime." + method + " timezone " + vm.currentUserTimeZoneID())
				}
				t = local
			}
			switch method {
			case "year":
				return Int(int64(t.Year())), receiver, false, true, nil
			case "month":
				return Int(int64(t.Month())), receiver, false, true, nil
			case "day":
				return Int(int64(t.Day())), receiver, false, true, nil
			case "hour":
				return Int(int64(t.Hour())), receiver, false, true, nil
			case "minute":
				return Int(int64(t.Minute())), receiver, false, true, nil
			case "second":
				return Int(int64(t.Second())), receiver, false, true, nil
			default:
				return Int(int64(t.Nanosecond() / int(time.Millisecond))), receiver, false, true, nil
			}
		}
	case "Time", "Blob":
		switch method {
		case "format", "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
			}
			text := receiver.Fields["value"].String()
			if receiver.Type == "Blob" && method == "toString" && !utf8.ValidString(text) {
				return Null, receiver, false, true, fmt.Errorf("Blob.toString invalid UTF-8 data")
			}
			return String(text), receiver, false, true, nil
		case "size":
			if receiver.Type != "Blob" {
				return Null, receiver, false, false, nil
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Blob.size expects 0 arguments")
			}
			return Int(int64(len([]byte(receiver.Fields["value"].String())))), receiver, false, true, nil
		case "hour", "minute", "second", "millisecond":
			if receiver.Type != "Time" {
				return Null, receiver, false, false, nil
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Time.%s expects 0 arguments", method)
			}
			duration, err := parsePlatformTime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			parsed := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(duration)
			switch method {
			case "hour":
				return Int(int64(parsed.Hour())), receiver, false, true, nil
			case "minute":
				return Int(int64(parsed.Minute())), receiver, false, true, nil
			case "second":
				return Int(int64(parsed.Second())), receiver, false, true, nil
			default:
				return Int(int64(parsed.Nanosecond() / int(time.Millisecond))), receiver, false, true, nil
			}
		case "addHours", "addMinutes", "addSeconds", "addMilliseconds":
			if receiver.Type != "Time" {
				return Null, receiver, false, false, nil
			}
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("Time.%s expects Integer", method)
			}
			duration, err := parsePlatformTime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			amount := time.Duration(args[0].Int)
			switch method {
			case "addHours":
				duration += amount * time.Hour
			case "addMinutes":
				duration += amount * time.Minute
			case "addSeconds":
				duration += amount * time.Second
			case "addMilliseconds":
				duration += amount * time.Millisecond
			}
			return platformTimeFromDuration(duration), receiver, false, true, nil
		}
	case "TimeZone":
		switch method {
		case "getID":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("TimeZone.getID expects 0 arguments")
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "getDisplayName":
			if len(args) == 0 {
				return receiver.Fields["id"], receiver, false, true, nil
			}
			if len(args) == 1 && args[0].Kind == ValueBool {
				return timeZoneDisplayName(receiver, args[0].Bool), receiver, false, true, nil
			}
			return Null, receiver, false, true, unsupportedCallError("TimeZone.getDisplayName locale/style overloads")
		case "getOffset":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Datetime" {
				return Null, receiver, false, true, fmt.Errorf("TimeZone.getOffset expects Datetime")
			}
			instant, err := parsePlatformDatetime(args[0])
			if err != nil {
				return Null, receiver, false, true, err
			}
			offset, err := timeZoneOffsetMillis(receiver, instant)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return offset, receiver, false, true, nil
		}
	case "Id":
		switch method {
		case "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Id.toString expects 0 arguments")
			}
			return receiver.Fields["value"], receiver, false, true, nil
		case "to15":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Id.to15 expects 0 arguments")
			}
			text, err := platformScalarText(receiver, "Id")
			if err != nil {
				return Null, receiver, false, true, err
			}
			if err := validateApexID(text); err != nil {
				return Null, receiver, false, true, err
			}
			if len(text) == 15 {
				return String(text), receiver, false, true, nil
			}
			return String(text[:15]), receiver, false, true, nil
		}
	case "Database.SaveResult", "Database.DeleteResult", "Database.UndeleteResult", "Database.EmptyRecycleBinResult", "Database.LockResult", "Database.UnlockResult":
		switch method {
		case "isSuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.isSuccess expects 0 arguments", receiver.Type)
			}
			return receiver.Fields["success"], receiver, false, true, nil
		case "getId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getId expects 0 arguments", receiver.Type)
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "getErrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getErrors expects 0 arguments", receiver.Type)
			}
			return receiver.Fields["errors"], receiver, false, true, nil
		}
	case "Database.UpsertResult":
		switch method {
		case "isSuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.UpsertResult.isSuccess expects 0 arguments")
			}
			return receiver.Fields["success"], receiver, false, true, nil
		case "getId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.UpsertResult.getId expects 0 arguments")
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "isCreated":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.UpsertResult.isCreated expects 0 arguments")
			}
			if created, ok := receiver.Fields["created"]; ok {
				return created, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "getErrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.UpsertResult.getErrors expects 0 arguments")
			}
			return receiver.Fields["errors"], receiver, false, true, nil
		}
	case "Database.MergeResult":
		switch method {
		case "isSuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.isSuccess expects 0 arguments")
			}
			return receiver.Fields["success"], receiver, false, true, nil
		case "getId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.getId expects 0 arguments")
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "getErrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.getErrors expects 0 arguments")
			}
			return receiver.Fields["errors"], receiver, false, true, nil
		case "getMergedRecordIds":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.getMergedRecordIds expects 0 arguments")
			}
			return receiver.Fields["mergedRecordIds"], receiver, false, true, nil
		case "getUpdatedRelatedIds":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.getUpdatedRelatedIds expects 0 arguments")
			}
			return receiver.Fields["updatedRelatedIds"], receiver, false, true, nil
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
		case "getFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.Error.getFields expects 0 arguments")
			}
			if fields, ok := receiver.Fields["fields"]; ok {
				return fields, receiver, false, true, nil
			}
			return List(), receiver, false, true, nil
		case "getExtendedErrorDetails":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.Error.getExtendedErrorDetails expects 0 arguments")
			}
			if details, ok := receiver.Fields["extendedErrorDetails"]; ok {
				return details, receiver, false, true, nil
			}
			return List(), receiver, false, true, nil
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
		if method == "getName" || method == "toString" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Type.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields["value"]; ok && value.Kind == ValueString {
				return value, receiver, false, true, nil
			}
			return String(receiver.Text), receiver, false, true, nil
		}
		if method == "equals" {
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Type.equals expects 1 argument")
			}
			return Bool(receiver.Equal(args[0])), receiver, false, true, nil
		}
		if method == "hashCode" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Type.hashCode expects 0 arguments")
			}
			return Int(int64(valueHashCode(receiver))), receiver, false, true, nil
		}
		if method == "newInstance" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Type.newInstance expects 0 arguments")
			}
			typeName := typeValueName(receiver)
			if unsupported, ok := typeNewInstanceUnsupportedBuiltin(typeName); ok {
				return Null, receiver, false, true, unsupportedCallError("Type.newInstance uninstantiable built-in " + unsupported)
			}
			if strings.Contains(typeName, ".") {
				if _, ok := vm.resolveClassName(typeName); !ok && !typeNewInstanceAllowsDottedBuiltin(typeName) {
					return Null, receiver, false, true, unsupportedCallError("Type.newInstance namespace/package reflection for " + typeName)
				}
			}
			value, err := vm.constructValue(typeName, nil, nil, result)
			return value, receiver, false, true, err
		}
		if method == "isAssignableFrom" {
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Type" {
				return Null, receiver, false, true, fmt.Errorf("Type.isAssignableFrom expects Type")
			}
			target := typeValueName(receiver)
			source := typeValueName(args[0])
			return Bool(vm.typeMatches(source, target, make(map[string]bool))), receiver, false, true, nil
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
		case "getLabelPlural":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getLabelPlural expects 0 arguments")
			}
			return receiver.Fields["labelPlural"], receiver, false, true, nil
		case "getKeyPrefix":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getKeyPrefix expects 0 arguments")
			}
			return receiver.Fields["keyPrefix"], receiver, false, true, nil
		case "getFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getFields expects 0 arguments")
			}
			return receiver.Fields["fields"], receiver, false, true, nil
		case "getRecordTypeInfos":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfos expects 0 arguments")
			}
			return receiver.Fields["recordTypeInfos"], receiver, false, true, nil
		case "getRecordTypeInfosByName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfosByName expects 0 arguments")
			}
			return receiver.Fields["recordTypeInfosByName"], receiver, false, true, nil
		case "getRecordTypeInfosByDeveloperName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfosByDeveloperName expects 0 arguments")
			}
			return receiver.Fields["recordTypeInfosByDeveloperName"], receiver, false, true, nil
		case "getChildRelationships":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getChildRelationships expects 0 arguments")
			}
			return receiver.Fields["childRelationships"], receiver, false, true, nil
		case "isAccessible", "isCreateable", "isUpdateable", "isDeletable", "isQueryable", "isSearchable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.%s expects 0 arguments", method)
			}
			return Bool(true), receiver, false, true, nil
		case "isCustom":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.isCustom expects 0 arguments")
			}
			name, _ := receiver.Fields["name"]
			return Bool(name.Kind == ValueString && strings.HasSuffix(name.Text, "__c")), receiver, false, true, nil
		}
	case "Schema.RecordTypeInfo":
		switch method {
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getDeveloperName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.getDeveloperName expects 0 arguments")
			}
			return receiver.Fields["developerName"], receiver, false, true, nil
		case "getRecordTypeId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.getRecordTypeId expects 0 arguments")
			}
			return receiver.Fields["recordTypeId"], receiver, false, true, nil
		case "isAvailable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.isAvailable expects 0 arguments")
			}
			return receiver.Fields["available"], receiver, false, true, nil
		case "isDefaultRecordTypeMapping":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.isDefaultRecordTypeMapping expects 0 arguments")
			}
			return receiver.Fields["default"], receiver, false, true, nil
		case "isActive":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.isActive expects 0 arguments")
			}
			return receiver.Fields["active"], receiver, false, true, nil
		}
	case "Schema.ChildRelationship":
		switch method {
		case "getRelationshipName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.getRelationshipName expects 0 arguments")
			}
			return receiver.Fields["relationshipName"], receiver, false, true, nil
		case "getField":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.getField expects 0 arguments")
			}
			return receiver.Fields["field"], receiver, false, true, nil
		case "getChildSObject":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.getChildSObject expects 0 arguments")
			}
			return receiver.Fields["childSObject"], receiver, false, true, nil
		case "isCascadeDelete":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.isCascadeDelete expects 0 arguments")
			}
			return receiver.Fields["cascadeDelete"], receiver, false, true, nil
		case "isRestrictedDelete":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.isRestrictedDelete expects 0 arguments")
			}
			return receiver.Fields["restrictedDelete"], receiver, false, true, nil
		}
	case "HttpRequest":
		switch method {
		case "setEndpoint":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setEndpoint expects String")
			}
			if err := validateHttpEndpoint(args[0].Text); err != nil {
				return Null, receiver, false, true, err
			}
			receiver.Fields["endpoint"] = args[0]
			return Null, receiver, true, true, nil
		case "getEndpoint":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getEndpoint expects 0 arguments")
			}
			return receiver.Fields["endpoint"], receiver, false, true, nil
		case "setMethod":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setMethod expects String")
			}
			method, err := normalizeHttpMethod(args[0].Text)
			if err != nil {
				return Null, receiver, false, true, err
			}
			receiver.Fields["method"] = String(method)
			return Null, receiver, true, true, nil
		case "getMethod":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getMethod expects 0 arguments")
			}
			return receiver.Fields["method"], receiver, false, true, nil
		case "setBody":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setBody expects String")
			}
			receiver.Fields["body"] = args[0]
			return Null, receiver, true, true, nil
		case "setBodyAsBlob":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setBodyAsBlob expects Blob")
			}
			receiver.Fields["body"] = args[0].Fields["value"]
			return Null, receiver, true, true, nil
		case "setClientCertificateName":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setClientCertificateName expects String")
			}
			return Null, receiver, false, true, unsupportedCallError("HttpRequest.setClientCertificateName local client certificate callout surface")
		case "setClientCertificate":
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setClientCertificate expects certificate and password Strings")
			}
			return Null, receiver, false, true, unsupportedCallError("HttpRequest.setClientCertificate local client certificate callout surface")
		case "setHeader":
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setHeader expects name and value Strings")
			}
			httpSetHeader(receiver, args[0].Text, args[1])
			return Null, receiver, true, true, nil
		case "getHeaderKeys":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getHeaderKeys expects 0 arguments")
			}
			return httpHeaderKeys(receiver), receiver, false, true, nil
		case "getHeader":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getHeader expects name String")
			}
			return httpGetHeader(receiver, args[0].Text), receiver, false, true, nil
		case "setCompressed":
			if len(args) != 1 || args[0].Kind != ValueBool {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setCompressed expects Boolean")
			}
			receiver.Fields["compressed"] = args[0]
			return Null, receiver, true, true, nil
		case "getCompressed":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getCompressed expects 0 arguments")
			}
			if value, ok := receiver.Fields["compressed"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "setTimeout":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setTimeout expects Integer")
			}
			if err := validateHttpTimeout(args[0].Int); err != nil {
				return Null, receiver, false, true, err
			}
			receiver.Fields["timeout"] = args[0]
			return Null, receiver, true, true, nil
		case "getTimeout":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getTimeout expects 0 arguments")
			}
			if value, ok := receiver.Fields["timeout"]; ok {
				return value, receiver, false, true, nil
			}
			return Int(defaultHttpTimeoutMillis), receiver, false, true, nil
		case "getBody":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getBody expects 0 arguments")
			}
			return receiver.Fields["body"], receiver, false, true, nil
		case "getBodyAsBlob":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getBodyAsBlob expects 0 arguments")
			}
			body := ""
			if value, ok := receiver.Fields["body"]; ok && value.Kind == ValueString {
				body = value.Text
			}
			return platformScalar("Blob", body), receiver, false, true, nil
		}
	case "HttpResponse":
		switch method {
		case "setBody":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setBody expects String")
			}
			receiver.Fields["body"] = args[0]
			return Null, receiver, true, true, nil
		case "setBodyAsBlob":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setBodyAsBlob expects Blob")
			}
			receiver.Fields["body"] = args[0].Fields["value"]
			return Null, receiver, true, true, nil
		case "getBody":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getBody expects 0 arguments")
			}
			return receiver.Fields["body"], receiver, false, true, nil
		case "getBodyAsBlob":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getBodyAsBlob expects 0 arguments")
			}
			body := ""
			if value, ok := receiver.Fields["body"]; ok && value.Kind == ValueString {
				body = value.Text
			}
			return platformScalar("Blob", body), receiver, false, true, nil
		case "setStatusCode":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setStatusCode expects Integer")
			}
			receiver.Fields["statusCode"] = args[0]
			return Null, receiver, true, true, nil
		case "setStatus":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setStatus expects String")
			}
			receiver.Fields["status"] = args[0]
			return Null, receiver, true, true, nil
		case "getStatus":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getStatus expects 0 arguments")
			}
			if value, ok := receiver.Fields["status"]; ok {
				return value, receiver, false, true, nil
			}
			return String("OK"), receiver, false, true, nil
		case "setHeader":
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setHeader expects name and value Strings")
			}
			httpSetHeader(receiver, args[0].Text, args[1])
			return Null, receiver, true, true, nil
		case "getHeaderKeys":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getHeaderKeys expects 0 arguments")
			}
			return httpHeaderKeys(receiver), receiver, false, true, nil
		case "getHeader":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getHeader expects name String")
			}
			return httpGetHeader(receiver, args[0].Text), receiver, false, true, nil
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
			if err := validateHttpRequest(args[0]); err != nil {
				return Null, receiver, false, true, err
			}
			if err := vm.incrementLimit("callouts", 1); err != nil {
				return Null, receiver, false, true, err
			}
			appendTrace(result, "apex.callout.http", "apex.callout", map[string]any{"operation": "Http.send"})
			if vm.testContext != nil && vm.testContext.HTTPMock.Kind == ValueObject {
				if target, ok := vm.resolveInstanceMethod(vm.testContext.HTTPMock.Type, "respond"); ok {
					value, err := vm.callMethodWithReceiver(target, vm.testContext.HTTPMock, []Value{args[0]}, &Result{})
					if err != nil {
						return Null, receiver, false, true, err
					}
					if value.Kind == ValueObject && value.Type == "HttpResponse" {
						return value, receiver, false, true, nil
					}
					return Null, receiver, false, true, fmt.Errorf("HttpCalloutMock.respond must return HttpResponse")
				}
				value, err := vm.localHTTPMockResponse(vm.testContext.HTTPMock, args[0])
				if err != nil {
					return Null, receiver, false, true, err
				}
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, unsupportedCallError("Http.send real network transport")
		}
	case "Messaging.SendEmailResult":
		switch method {
		case "isSuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Messaging.SendEmailResult.isSuccess expects 0 arguments")
			}
			if value, ok := receiver.Fields["success"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(true), receiver, false, true, nil
		case "getErrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Messaging.SendEmailResult.getErrors expects 0 arguments")
			}
			if value, ok := receiver.Fields["errors"]; ok {
				return value, receiver, false, true, nil
			}
			return List(), receiver, false, true, nil
		}
	case "Messaging.SingleEmailMessage":
		return callSingleEmailMessageMember(receiver, method, args)
	case "Messaging.MassEmailMessage":
		return callMassEmailMessageMember(receiver, method, args)
	case "Messaging.SendEmailOptions":
		return Null, receiver, false, true, unsupportedCallError("Messaging.SendEmailOptions." + method + " local messaging send-options surface")
	case "ApexPages.Message":
		switch method {
		case "getSeverity":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.getSeverity expects 0 arguments")
			}
			return receiver.Fields["severity"], receiver, false, true, nil
		case "getSummary":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.getSummary expects 0 arguments")
			}
			return receiver.Fields["summary"], receiver, false, true, nil
		case "getDetail":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.getDetail expects 0 arguments")
			}
			if value, ok := receiver.Fields["detail"]; ok {
				return value, receiver, false, true, nil
			}
			return receiver.Fields["summary"], receiver, false, true, nil
		}
	case "Continuation":
		return Null, receiver, false, true, unsupportedCallError("Continuation local continuation callout surface")
	case "StaticResourceCalloutMock":
		return callStaticResourceCalloutMockMember(receiver, method, args)
	case "MultiStaticResourceCalloutMock":
		return callMultiStaticResourceCalloutMockMember(receiver, method, args)
	case "PageReference":
		switch method {
		case "getContent", "getContentAsPDF":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.%s expects 0 arguments", method)
			}
			return Null, receiver, false, true, unsupportedCallError("PageReference." + method + " local Visualforce page rendering surface")
		case "getUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getUrl expects 0 arguments")
			}
			return receiver.Fields["url"], receiver, false, true, nil
		case "setRedirect":
			if len(args) != 1 || args[0].Kind != ValueBool {
				return Null, receiver, false, true, fmt.Errorf("PageReference.setRedirect expects Boolean")
			}
			receiver.Fields["redirect"] = args[0]
			return Null, receiver, true, true, nil
		case "getRedirect":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getRedirect expects 0 arguments")
			}
			if value, ok := receiver.Fields["redirect"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "getParameters":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getParameters expects 0 arguments")
			}
			if value, ok := receiver.Fields["parameters"]; ok {
				return value, receiver, false, true, nil
			}
			params := typedMap("Map<String,String>")
			receiver.Fields["parameters"] = params
			return params, receiver, true, true, nil
		case "getHeaders":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getHeaders expects 0 arguments")
			}
			if value, ok := receiver.Fields["headers"]; ok {
				return value, receiver, false, true, nil
			}
			headers := typedMap("Map<String,String>")
			receiver.Fields["headers"] = headers
			return headers, receiver, true, true, nil
		}
	case "URL":
		if method == "toExternalForm" || method == "toString" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("URL.%s expects 0 arguments", method)
			}
			return receiver.Fields["value"], receiver, false, true, nil
		}
		if method == "getProtocol" || method == "getHost" || method == "getAuthority" ||
			method == "getPath" || method == "getQuery" || method == "getRef" ||
			method == "getFile" || method == "getPort" || method == "getDefaultPort" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("URL.%s expects 0 arguments", method)
			}
			raw, err := platformScalarText(receiver, "URL")
			if err != nil {
				return Null, receiver, false, true, err
			}
			parsed, err := url.Parse(raw)
			if err != nil {
				return Null, receiver, false, true, err
			}
			switch method {
			case "getProtocol":
				return String(parsed.Scheme), receiver, false, true, nil
			case "getHost":
				return String(parsed.Hostname()), receiver, false, true, nil
			case "getAuthority":
				authority := parsed.Host
				if parsed.User != nil {
					authority = parsed.User.String() + "@" + authority
				}
				return String(authority), receiver, false, true, nil
			case "getPath":
				return String(parsed.Path), receiver, false, true, nil
			case "getQuery":
				return String(parsed.RawQuery), receiver, false, true, nil
			case "getRef":
				return String(parsed.Fragment), receiver, false, true, nil
			case "getFile":
				file := parsed.Path
				if parsed.RawQuery != "" {
					file += "?" + parsed.RawQuery
				}
				return String(file), receiver, false, true, nil
			case "getPort":
				if parsed.Port() == "" {
					return Int(-1), receiver, false, true, nil
				}
				port, err := strconv.ParseInt(parsed.Port(), 10, 64)
				if err != nil {
					return Null, receiver, false, true, err
				}
				return Int(port), receiver, false, true, nil
			case "getDefaultPort":
				return Int(defaultURLPort(parsed.Scheme)), receiver, false, true, nil
			}
		}
	}
	return Null, receiver, false, false, nil
}

func callRestRequestMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "addHeader":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.addHeader expects name and value Strings")
		}
		restMapPut(&receiver, "headers", args[0].Text, args[1], true)
		return Null, receiver, true, true, nil
	case "getHeader":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.getHeader expects name String")
		}
		return restMapGet(receiver, "headers", args[0].Text), receiver, false, true, nil
	case "getHeaderKeys":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.getHeaderKeys expects 0 arguments")
		}
		return restMapKeys(receiver, "headers"), receiver, false, true, nil
	case "addParameter", "addParam":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.%s expects name and value Strings", method)
		}
		restMapPut(&receiver, "params", args[0].Text, args[1], false)
		return Null, receiver, true, true, nil
	case "getParameter", "getParam":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.%s expects name String", method)
		}
		return restMapGet(receiver, "params", args[0].Text), receiver, false, true, nil
	case "getParameterKeys", "getParamKeys":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.%s expects 0 arguments", method)
		}
		return restMapKeys(receiver, "params"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callStaticResourceCalloutMockMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "setStaticResource":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("StaticResourceCalloutMock.setStaticResource expects String")
		}
		receiver.Fields["staticResource"] = args[0]
		return Null, receiver, true, true, nil
	case "setStatusCode":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("StaticResourceCalloutMock.setStatusCode expects Integer")
		}
		receiver.Fields["statusCode"] = args[0]
		return Null, receiver, true, true, nil
	case "setHeader":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("StaticResourceCalloutMock.setHeader expects name and value Strings")
		}
		httpSetHeader(receiver, args[0].Text, args[1])
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callMultiStaticResourceCalloutMockMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "setStaticResource":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("MultiStaticResourceCalloutMock.setStaticResource expects endpoint and static resource Strings")
		}
		resources, ok := receiver.Fields["staticResources"]
		if !ok || resources.Kind != ValueMap {
			resources = typedMap("Map<String,String>")
		}
		resources.Map[mapKey(args[0])] = args[1]
		receiver.Fields["staticResources"] = resources
		return Null, receiver, true, true, nil
	case "setStatusCode":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("MultiStaticResourceCalloutMock.setStatusCode expects Integer")
		}
		receiver.Fields["statusCode"] = args[0]
		return Null, receiver, true, true, nil
	case "setHeader":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("MultiStaticResourceCalloutMock.setHeader expects name and value Strings")
		}
		httpSetHeader(receiver, args[0].Text, args[1])
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) localHTTPMockResponse(mock Value, request Value) (Value, error) {
	switch mock.Type {
	case "StaticResourceCalloutMock":
		resource, ok := mock.Fields["staticResource"]
		if !ok || resource.Kind != ValueString || strings.TrimSpace(resource.Text) == "" {
			return Null, fmt.Errorf("StaticResourceCalloutMock static resource is required before Http.send")
		}
		return vm.staticResourceMockResponse(mock, resource.Text), nil
	case "MultiStaticResourceCalloutMock":
		endpoint, ok := request.Fields["endpoint"]
		if !ok || endpoint.Kind != ValueString {
			return Null, fmt.Errorf("MultiStaticResourceCalloutMock request endpoint is missing")
		}
		resources, ok := mock.Fields["staticResources"]
		if !ok || resources.Kind != ValueMap {
			return Null, fmt.Errorf("MultiStaticResourceCalloutMock has no static resource for endpoint %s", endpoint.Text)
		}
		resource, ok := resources.Map[mapKey(endpoint)]
		if !ok || resource.Kind != ValueString || strings.TrimSpace(resource.Text) == "" {
			return Null, fmt.Errorf("MultiStaticResourceCalloutMock has no static resource for endpoint %s", endpoint.Text)
		}
		return vm.staticResourceMockResponse(mock, resource.Text), nil
	default:
		response := newHttpResponse()
		if body, ok := mock.Fields["body"]; ok {
			response.Fields["body"] = body
		}
		if status, ok := mock.Fields["statusCode"]; ok {
			response.Fields["statusCode"] = status
		}
		if headers, ok := mock.Fields["headers"]; ok {
			response.Fields["headers"] = headers
		}
		return response, nil
	}
}

func (vm *VM) staticResourceMockResponse(mock Value, resourceName string) Value {
	response := newHttpResponse()
	response.Fields["body"] = String(vm.staticResourceBody(resourceName))
	if status, ok := mock.Fields["statusCode"]; ok {
		response.Fields["statusCode"] = status
	}
	if headers, ok := mock.Fields["headers"]; ok {
		response.Fields["headers"] = headers
	}
	return response
}

func (vm *VM) staticResourceBody(resourceName string) string {
	if vm.Org == nil {
		return resourceName
	}
	object, ok := vm.Org.Objects["StaticResource"]
	if !ok {
		return resourceName
	}
	for _, record := range object.Records {
		if !staticResourceNameMatches(record, resourceName) {
			continue
		}
		for _, field := range []string{"Body", "Content"} {
			if value, ok := record.Fields[field]; ok {
				if body, ok := staticResourceBodyValue(value); ok {
					return body
				}
			}
		}
	}
	return resourceName
}

func staticResourceNameMatches(record storage.Record, resourceName string) bool {
	if strings.EqualFold(string(record.ID), resourceName) {
		return true
	}
	for _, field := range []string{"Name", "DeveloperName"} {
		value, ok := record.Fields[field]
		if ok && value.Kind == storage.ValueString && strings.EqualFold(value.String, resourceName) {
			return true
		}
	}
	return false
}

func staticResourceBodyValue(value storage.Value) (string, bool) {
	switch value.Kind {
	case storage.ValueString, storage.ValueBlob:
		return value.String, true
	default:
		return "", false
	}
}

func callRestResponseMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "addHeader":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestResponse.addHeader expects name and value Strings")
		}
		restMapPut(&receiver, "headers", args[0].Text, args[1], true)
		return Null, receiver, true, true, nil
	case "getHeader":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestResponse.getHeader expects name String")
		}
		return restMapGet(receiver, "headers", args[0].Text), receiver, false, true, nil
	case "getHeaderKeys":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("RestResponse.getHeaderKeys expects 0 arguments")
		}
		return restMapKeys(receiver, "headers"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callSingleEmailMessageMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "setToAddresses", "setCcAddresses", "setBccAddresses", "setFileAttachments", "setEntityAttachments", "setDocumentAttachments", "setTargetObjectIds":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects List", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setSubject", "setPlainTextBody", "setHtmlBody", "setReplyTo", "setSenderDisplayName",
		"setCharset", "setInReplyTo", "setReferences", "setOrgWideEmailAddressId",
		"setTargetObjectId", "setTemplateId", "setWhatId", "setOptOutPolicy", "setEmailPriority":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects String", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setSaveAsActivity", "setTreatBodiesAsTemplate", "setTreatTargetObjectAsRecipient", "setUseSignature", "setBccSender":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects Boolean", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "getToAddresses", "getCcAddresses", "getBccAddresses", "getFileAttachments", "getEntityAttachments", "getDocumentAttachments", "getTargetObjectIds",
		"getSubject", "getPlainTextBody", "getHtmlBody", "getReplyTo", "getSenderDisplayName",
		"getCharset", "getInReplyTo", "getReferences", "getOrgWideEmailAddressId",
		"getTargetObjectId", "getTemplateId", "getWhatId", "getOptOutPolicy", "getEmailPriority",
		"getSaveAsActivity", "getTreatBodiesAsTemplate", "getTreatTargetObjectAsRecipient", "getUseSignature", "getBccSender":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callMassEmailMessageMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "setTargetObjectIds", "setWhatIds":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects List", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setTemplateId", "setDescription", "setOptOutPolicy":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects String", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setSaveAsActivity":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects Boolean", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "getTargetObjectIds", "getWhatIds", "getTemplateId", "getDescription", "getOptOutPolicy", "getSaveAsActivity":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func emailMessageFieldName(method string) string {
	if strings.HasPrefix(method, "get") && len(method) > len("get") {
		field := strings.TrimPrefix(method, "get")
		return strings.ToLower(field[:1]) + field[1:]
	}
	if !strings.HasPrefix(method, "set") || len(method) <= len("set") {
		return method
	}
	field := strings.TrimPrefix(method, "set")
	return strings.ToLower(field[:1]) + field[1:]
}

func restMapPut(receiver *Value, field, key string, value Value, caseInsensitive bool) {
	current := receiver.Fields[field]
	if current.Kind != ValueMap {
		current = typedMap("Map<String,String>")
	}
	if caseInsensitive {
		for rawKey := range current.Map {
			decoded := valueFromMapKey(rawKey)
			if decoded.Kind == ValueString && strings.EqualFold(decoded.Text, key) {
				delete(current.Map, rawKey)
				break
			}
		}
	}
	current.Map[mapKey(String(key))] = value
	receiver.Fields[field] = current
}

func restMapGet(receiver Value, field, key string) Value {
	current := receiver.Fields[field]
	if current.Kind != ValueMap {
		return Null
	}
	if value, ok := current.Map[mapKey(String(key))]; ok {
		return value
	}
	for rawKey, value := range current.Map {
		decoded := valueFromMapKey(rawKey)
		if decoded.Kind == ValueString && strings.EqualFold(decoded.Text, key) {
			return value
		}
	}
	return Null
}

func restMapKeys(receiver Value, field string) Value {
	current := receiver.Fields[field]
	if current.Kind != ValueMap {
		return List()
	}
	keys := make([]string, 0, len(current.Map))
	for rawKey := range current.Map {
		decoded := valueFromMapKey(rawKey)
		if decoded.Kind == ValueString {
			keys = append(keys, decoded.Text)
		}
	}
	sort.Strings(keys)
	out := make([]Value, 0, len(keys))
	for _, key := range keys {
		out = append(out, String(key))
	}
	return List(out...)
}
