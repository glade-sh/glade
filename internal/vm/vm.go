package vm

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
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
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/open-aer/oaer/internal/dml"
	"github.com/open-aer/oaer/internal/ir"
	"github.com/open-aer/oaer/internal/resource"
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
	Globals            map[string]Value
	VarTypes           map[string]string
	Methods            map[string]Method
	MethodOverloads    map[string][]Method
	MethodFolded       map[string][]Method
	Classes            map[string]Class
	classLookup        map[string]Class
	enumLookup         map[string]enumClassLookup
	Org                *storage.OrgState
	Triggers           map[string][]Trigger
	Stdout             io.Writer
	callStack          []callFrame
	currentClass       string
	currentMethod      Method
	testContext        *TestContext
	localAsyncJobs     []AsyncJob
	localAsyncSeq      int
	localAsyncDrain    bool
	localAsyncChain    bool
	executionUser      Value
	limits             Limits
	limitCaps          LimitCaps
	limitMode          LimitMode
	limitViolations    []LimitViolation
	fakeNow            time.Time
	currentAsyncKind   string
	activeExceptions   []activeException
	currentStatement   callFrame
	hasStatement       bool
	triggerDepth       int
	savepoints         map[string]storage.OrgState
	emailSavepoints    map[string][]CapturedEmail
	savepointOrder     map[string]int
	nextSavepoint      int
	pageMessages       []Value
	currentPage        Value
	pageReferences     map[string]string
	platformCache      map[string]map[string]cacheEntry
	capturedEmails     []CapturedEmail
	restRequest        Value
	restResponse       Value
	serverBaseURL      string
	metadataDeploys    map[string]Value
	debugHooks         DebugHooks
	hasDebugHooks      bool
	ctx                context.Context
	activeGetters      map[string]int
	activeSetters      map[string]int
	triggerGlobals     map[string]Value
	cryptoRandomSeq    uint64
	staticInitState    map[string]staticInitState
	lastAmbiguous      *overloadDiagnostic
	activeConstructors map[string]int
}

type enumClassLookup struct {
	Class Class
	OK    bool
}

const maxTriggerDepth = 16

type staticInitState uint8

const (
	staticInitUninitialized staticInitState = iota
	staticInitRunning
	staticInitDone
)

type Result struct {
	Debug           []string         `json:"debug,omitempty"`
	Vars            map[string]Value `json:"vars,omitempty"`
	TraceFormat     string           `json:"traceFormat,omitempty"`
	Trace           []trace.Event    `json:"trace,omitempty"`
	CapturedEmails  []CapturedEmail  `json:"capturedEmails,omitempty"`
	Limits          Limits           `json:"limits,omitempty"`
	LimitMode       LimitMode        `json:"limitMode,omitempty"`
	LimitViolations []LimitViolation `json:"limitViolations,omitempty"`
}

type CapturedEmail struct {
	Kind                string   `json:"kind"`
	ToAddresses         []string `json:"toAddresses,omitempty"`
	CcAddresses         []string `json:"ccAddresses,omitempty"`
	BccAddresses        []string `json:"bccAddresses,omitempty"`
	TargetObjectIDs     []string `json:"targetObjectIds,omitempty"`
	WhatIDs             []string `json:"whatIds,omitempty"`
	Subject             string   `json:"subject,omitempty"`
	PlainTextBody       string   `json:"plainTextBody,omitempty"`
	HTMLBody            string   `json:"htmlBody,omitempty"`
	TemplateID          string   `json:"templateId,omitempty"`
	TargetObjectID      string   `json:"targetObjectId,omitempty"`
	WhatID              string   `json:"whatId,omitempty"`
	SaveAsActivity      bool     `json:"saveAsActivity,omitempty"`
	FileAttachments     []string `json:"fileAttachments,omitempty"`
	EntityAttachments   []string `json:"entityAttachments,omitempty"`
	DocumentAttachments []string `json:"documentAttachments,omitempty"`
}

type sideEffectSnapshot struct {
	capturedEmails []CapturedEmail
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

const maxApexCallDepth = 256

type overloadDiagnostic struct {
	Args       []Value
	Candidates []Method
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

type cacheEntry struct {
	Value    Value
	ExpireAt time.Time
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
		classLookup:     make(map[string]Class),
		enumLookup:      make(map[string]enumClassLookup),
		Triggers:        make(map[string][]Trigger),
		Stdout:          stdout,
		limitCaps:       defaultLimitCaps(),
		limitMode:       LimitModePermissive,
		fakeNow:         time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		savepoints:      make(map[string]storage.OrgState),
		emailSavepoints: make(map[string][]CapturedEmail),
		savepointOrder:  make(map[string]int),
		platformCache:   make(map[string]map[string]cacheEntry),
		metadataDeploys: make(map[string]Value),
		ctx:             context.Background(),
		activeGetters:   make(map[string]int),
		activeSetters:   make(map[string]int),
		staticInitState: make(map[string]staticInitState),
	}
}

// CloneRuntime returns a fresh VM with the same registered Apex methods,
// classes, and triggers. Mutable runtime state such as globals, limits, org
// state, current user, and static field values remains request-local.
func (vm *VM) CloneRuntime(stdout io.Writer) *VM {
	clone := New(stdout)
	clone.Methods = copyMethodMap(vm.Methods)
	clone.MethodOverloads = copyMethodSliceMap(vm.MethodOverloads)
	clone.MethodFolded = copyMethodSliceMap(vm.MethodFolded)
	clone.Classes = copyClassMap(vm.Classes)
	clone.rebuildClassLookup()
	clone.Triggers = copyTriggerSliceMap(vm.Triggers)
	clone.staticInitState = copyStaticInitStateMap(vm.staticInitState)
	clone.pageReferences = copyStringMap(vm.pageReferences)
	clone.platformCache = copyCacheMap(vm.platformCache)
	return clone
}

func copyMethodMap(in map[string]Method) map[string]Method {
	out := make(map[string]Method, len(in))
	for name, method := range in {
		out[name] = method
	}
	return out
}

func copyMethodSliceMap(in map[string][]Method) map[string][]Method {
	out := make(map[string][]Method, len(in))
	for name, methods := range in {
		out[name] = append([]Method(nil), methods...)
	}
	return out
}

func copyClassMap(in map[string]Class) map[string]Class {
	out := make(map[string]Class, len(in))
	byCanonicalName := make(map[string]Class, len(in))
	for name, class := range in {
		canonical := class.Name
		if canonical == "" {
			canonical = name
		}
		copied, ok := byCanonicalName[canonical]
		if !ok {
			copied = copyClass(class)
			byCanonicalName[canonical] = copied
		}
		out[name] = copied
	}
	return out
}

func copyClass(class Class) Class {
	class.Interfaces = append([]string(nil), class.Interfaces...)
	class.FieldOrder = append([]string(nil), class.FieldOrder...)
	class.StaticFieldOrder = append([]string(nil), class.StaticFieldOrder...)
	class.Constructors = append([]Method(nil), class.Constructors...)
	class.StaticInitializers = append([]Method(nil), class.StaticInitializers...)
	class.InstanceInitializers = append([]Method(nil), class.InstanceInitializers...)
	class.EnumValues = append([]string(nil), class.EnumValues...)
	class.Fields = copyFieldMap(class.Fields)
	class.StaticFields = copyFieldMap(class.StaticFields)
	class.Methods = copyMethodMap(class.Methods)
	return class
}

func copyFieldMap(in map[string]Field) map[string]Field {
	out := make(map[string]Field, len(in))
	for name, field := range in {
		out[name] = field
	}
	return out
}

func copyTriggerSliceMap(in map[string][]Trigger) map[string][]Trigger {
	out := make(map[string][]Trigger, len(in))
	for name, triggers := range in {
		out[name] = append([]Trigger(nil), triggers...)
	}
	return out
}

func copyStaticInitStateMap(in map[string]staticInitState) map[string]staticInitState {
	out := make(map[string]staticInitState, len(in))
	for name, state := range in {
		out[name] = state
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func copyCacheMap(in map[string]map[string]cacheEntry) map[string]map[string]cacheEntry {
	if in == nil {
		return nil
	}
	out := make(map[string]map[string]cacheEntry, len(in))
	for partition, entries := range in {
		copied := make(map[string]cacheEntry, len(entries))
		for key, entry := range entries {
			copied[key] = entry
		}
		out[partition] = copied
	}
	return out
}

func (vm *VM) RegisterPageReference(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if strings.HasPrefix(strings.ToLower(name), "page.") {
		name = name[len("Page."):]
	}
	if vm.pageReferences == nil {
		vm.pageReferences = make(map[string]string)
	}
	vm.pageReferences[strings.ToLower(name)] = name
}

func (vm *VM) ResetApexPageState() {
	vm.pageMessages = nil
	vm.currentPage = Value{}
}

func (vm *VM) SetOrg(org *storage.OrgState) {
	vm.Org = org
}

func (vm *VM) SetServerBaseURL(rawURL string) {
	vm.serverBaseURL = strings.TrimRight(rawURL, "/")
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

func (vm *VM) newDMLEngine(result *Result) dml.Engine {
	engine := dml.NewEngine(vm.Org)
	engine.Now = func() time.Time { return vm.fakeNow }
	if userID := vm.currentUserInfoField("Id", ""); userID != "" {
		engine.UserID = storage.ID(userID)
	}
	engine.FlowActionInvoker = func(action storage.FlowAction, record storage.Record) error {
		return vm.invokeFlowAction(action, record, result)
	}
	engine.WorkflowEmailer = func(alert storage.WorkflowEmailAlert, record storage.Record) error {
		return vm.captureWorkflowEmail(alert, record, result)
	}
	engine.AutomationTracer = func(name string, args map[string]any) {
		appendTrace(result, name, "apex.flow", args)
	}
	return engine
}

func (vm *VM) newDeferredAutomationDMLEngine(result *Result) dml.Engine {
	engine := vm.newDMLEngine(result)
	engine.DeferAutomation = true
	return engine
}

func (vm *VM) invokeFlowAction(action storage.FlowAction, record storage.Record, result *Result) error {
	method, ok, err := vm.resolveFlowInvocableMethod(action)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("flow action %s: no static @InvocableMethod found on %s", action.Name, flowActionTargetName(action))
	}
	if len(method.Params) != 1 || collectionBase(method.Params[0].Type) != "List" {
		return fmt.Errorf("flow action %s: %s must accept exactly one List parameter", action.Name, method.Name)
	}
	arg := List(vm.vmValueFromRecord(record))
	arg.Type = method.Params[0].Type
	if result != nil {
		result.Trace = append(result.Trace, trace.Instant("apex.flow.action", "apex.flow", int64(len(result.Trace)), map[string]any{
			"action": action.Name,
			"class":  method.ClassName,
			"method": method.Name,
			"record": string(record.ID),
			"object": record.Object,
		}))
	}
	_, err = vm.callMethod(method, []Value{arg}, result)
	if err != nil {
		var thrown *apexThrowError
		if errors.As(err, &thrown) {
			if len(thrown.stack) == 0 {
				thrown.stack = vm.rawStackFrames()
			}
			return runtimeError(thrown.value, thrown.stack)
		}
	}
	return err
}

func (vm *VM) resolveFlowInvocableMethod(action storage.FlowAction) (Method, bool, error) {
	className := strings.TrimSpace(action.ClassName)
	if className == "" {
		className = strings.TrimSpace(action.ActionName)
	}
	if className == "" {
		return Method{}, false, fmt.Errorf("flow action %s: missing Apex class name", action.Name)
	}
	methodName := strings.TrimSpace(action.MethodName)
	if methodName != "" {
		candidates := vm.MethodOverloads[className+"."+methodName]
		if len(candidates) == 0 {
			candidates = vm.MethodFolded[strings.ToLower(className+"."+methodName)]
		}
		if len(candidates) == 0 {
			class, ok := vm.Classes[className]
			if !ok {
				return Method{}, false, nil
			}
			for name, candidate := range class.Methods {
				if strings.EqualFold(name, methodName) || strings.EqualFold(candidate.Name, className+"."+methodName) {
					candidates = append(candidates, candidate)
				}
			}
		}
		for _, method := range candidates {
			if !method.IsStatic {
				continue
			}
			if !methodHasModifier(method.Modifiers, "InvocableMethod") {
				return Method{}, false, fmt.Errorf("flow action %s: %s is not annotated @InvocableMethod", action.Name, method.Name)
			}
			return method, true, nil
		}
		if len(candidates) > 0 {
			return Method{}, false, nil
		}
	}
	class, ok := vm.Classes[className]
	if !ok {
		return Method{}, false, nil
	}
	var matches []Method
	for _, method := range class.Methods {
		if method.IsStatic && methodHasModifier(method.Modifiers, "InvocableMethod") {
			matches = append(matches, method)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Name < matches[j].Name })
	if len(matches) == 0 {
		return Method{}, false, nil
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, method := range matches {
			names = append(names, method.Name)
		}
		return Method{}, false, fmt.Errorf("flow action %s: multiple @InvocableMethod candidates on %s: %s", action.Name, className, strings.Join(names, ", "))
	}
	return matches[0], true, nil
}

func flowActionTargetName(action storage.FlowAction) string {
	if strings.TrimSpace(action.ClassName) != "" {
		return strings.TrimSpace(action.ClassName)
	}
	return strings.TrimSpace(action.ActionName)
}

func (vm *VM) applyDeferredAutomation(engine *dml.Engine, records []storage.Record, allOrNone bool, backup storage.OrgState, result *Result) error {
	if engine == nil {
		return nil
	}
	sideEffects := vm.snapshotSideEffects()
	for _, record := range records {
		if record.Object == "" || record.ID == "" {
			continue
		}
		if result != nil {
			appendTrace(result, "apex.automation.apply", "apex.automation", map[string]any{
				"object": record.Object,
				"id":     string(record.ID),
			})
		}
		if err := engine.ApplyAutomation(record.Object, record.ID); err != nil {
			if allOrNone {
				*vm.Org = backup
				vm.restoreSideEffects(sideEffects)
				appendTrace(result, "apex.automation.rollback", "apex.automation", map[string]any{
					"object": record.Object,
					"id":     string(record.ID),
					"reason": err.Error(),
				})
			}
			return err
		}
	}
	return nil
}

func (vm *VM) snapshotSideEffects() sideEffectSnapshot {
	return sideEffectSnapshot{
		capturedEmails: append([]CapturedEmail(nil), vm.capturedEmails...),
	}
}

func (vm *VM) restoreSideEffects(snapshot sideEffectSnapshot) {
	vm.capturedEmails = append([]CapturedEmail(nil), snapshot.capturedEmails...)
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
	vm.testContext = &TestContext{CurrentUser: vm.defaultTestCurrentUser()}
	vm.ensureAsyncObjects()
}

func (vm *VM) defaultTestCurrentUser() Value {
	if vm.executionUser.Kind != "" && vm.executionUser.Kind != ValueNull {
		return vm.executionUser
	}
	if vm.Org != nil {
		if users, ok := vm.Org.Objects["User"]; ok && len(users.Records) > 0 {
			ids := make([]string, 0, len(users.Records))
			for id := range users.Records {
				ids = append(ids, string(id))
			}
			sort.Strings(ids)
			return vmValueFromRecord(users.Records[storage.ID(ids[0])])
		}
	}
	return String("system")
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
	if vm.staticInitState == nil {
		vm.staticInitState = make(map[string]staticInitState)
	}
	for name, class := range vm.Classes {
		vm.staticInitState[name] = staticInitUninitialized
		if class.Name != "" {
			vm.staticInitState[class.Name] = staticInitUninitialized
		}
	}
	vm.ResetApexPageState()
	return nil
}

func Execute(program ir.Program, stdout io.Writer) (Result, error) {
	return New(stdout).Execute(program)
}

func (vm *VM) Execute(program ir.Program) (result Result, err error) {
	return vm.execute(program, "")
}

func (vm *VM) ExecuteInClass(program ir.Program, className string) (result Result, err error) {
	return vm.execute(program, className)
}

func (vm *VM) execute(program ir.Program, className string) (result Result, err error) {
	result = Result{Vars: vm.Globals, TraceFormat: trace.FormatChromeTraceEvent}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("internal VM panic: %v", recovered)
		}
		result.CapturedEmails = append([]CapturedEmail(nil), vm.capturedEmails...)
		result.Limits = vm.limits
		result.LimitMode = vm.limitMode
		result.LimitViolations = append([]LimitViolation(nil), vm.limitViolations...)
		appendTrace(&result, "apex.limits", "apex.limits", limitTraceArgs(vm.limits))
	}()
	callerClass := vm.currentClass
	if className != "" {
		vm.currentClass = className
		defer func() {
			vm.currentClass = callerClass
		}()
	}
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
				value = plainNull(evaluated)
			}
			coerced, err := vm.coerceAssignable(inst.Type, value)
			if err != nil {
				return execOutcome{}, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
			}
			value = coerced
			vm.Globals[inst.Name] = value
			vm.VarTypes[inst.Name] = inst.Type
		case ir.OpAssign:
			value, err := vm.evalForAssignment(inst.Name, inst.Expr, result)
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
				returnType := ""
				if vm.currentMethod.ReturnType != "" && vm.currentMethod.ReturnType != "void" {
					returnType = vm.currentMethod.ReturnType
				}
				evaluated, err := vm.evalForType(inst.Expr, returnType, result)
				if err != nil {
					return execOutcome{}, err
				}
				value = evaluated
			}
			if err := vm.updateHeapLimit(); err != nil {
				return execOutcome{}, err
			}
			return execOutcome{value: value, signal: signalReturn}, nil
		case ir.OpBlock:
			out, err := vm.executeProgram(childProgram(program, inst.Then), result)
			if err != nil || out.signal != signalNone {
				return out, err
			}
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
	total := vm.currentHeapSize()
	vm.limits.HeapSize = total
	return vm.checkLimit("heapSize", vm.limits.HeapSize, vm.limitCaps.HeapSize)
}

func (vm *VM) currentHeapSize() int {
	total := 0
	for name, value := range vm.Globals {
		total += len(name) + approxValueSize(value)
	}
	return total
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
	inits := inst.Inits
	if len(inits) == 0 && inst.Init != nil {
		inits = []ir.Instruction{*inst.Init}
	}
	if len(inits) > 0 {
		out, err := vm.executeProgram(ir.Program{Instructions: inits, Source: source}, result)
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
		updates := inst.Updates
		if len(updates) == 0 && inst.Update != nil {
			updates = []ir.Instruction{*inst.Update}
		}
		if len(updates) > 0 {
			out, err := vm.executeProgram(ir.Program{Instructions: updates, Source: source}, result)
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
				matches, handled := switchCaseEnumNameMatch(value, expr, err)
				if !handled {
					return execOutcome{}, err
				}
				if !matches {
					continue
				}
				out, err := vm.executeProgram(ir.Program{Instructions: c.Body, Source: source}, result)
				if err != nil {
					return execOutcome{}, err
				}
				if out.signal == signalBreak {
					return execOutcome{}, nil
				}
				return out, nil
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

func switchCaseEnumNameMatch(value Value, expr ir.Expr, err error) (bool, bool) {
	if err == nil || value.Kind != ValueObject || value.Text == "" || expr.Kind != ir.ExprVariable {
		return false, false
	}
	if !strings.Contains(err.Error(), "unknown variable") {
		return false, false
	}
	return expr.Name == value.Text, true
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
		return vm.evalBinary(expr.Operator, left, right, result)
	case ir.ExprCall:
		if strings.HasPrefix(expr.Callee, "__assign:") {
			if len(expr.Args) != 1 {
				return Null, fmt.Errorf("assignment expression requires 1 operand")
			}
			target := strings.TrimPrefix(expr.Callee, "__assign:")
			value, err := vm.evalForAssignment(target, expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			if err := vm.assign(target, value); err != nil {
				return Null, err
			}
			return value, nil
		}
		if strings.HasPrefix(expr.Callee, "__assignField:") {
			if expr.Left == nil || len(expr.Args) != 1 {
				return Null, fmt.Errorf("field assignment expression requires receiver and value")
			}
			receiver, err := vm.eval(*expr.Left, result)
			if err != nil {
				return Null, err
			}
			field := strings.TrimPrefix(expr.Callee, "__assignField:")
			value, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			if err := vm.assignPath(receiver, []string{field}, value); err != nil {
				return Null, err
			}
			return value, nil
		}
		if strings.HasPrefix(expr.Callee, "__prefix:") || strings.HasPrefix(expr.Callee, "__postfix:") {
			return vm.evalIncrementExpression(expr, result)
		}
		if strings.HasPrefix(expr.Callee, "__cast:") {
			if len(expr.Args) != 1 {
				return Null, fmt.Errorf("cast expression requires 1 operand")
			}
			value, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			typeName := strings.TrimPrefix(expr.Callee, "__cast:")
			return vm.coerceCast(typeName, value)
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
		if strings.HasPrefix(expr.Callee, "__field:") || strings.HasPrefix(expr.Callee, "__safe_field:") {
			if expr.Left == nil {
				return Null, fmt.Errorf("field access requires receiver")
			}
			receiver, err := vm.eval(*expr.Left, result)
			if err != nil {
				return Null, err
			}
			if isSafeNavigationNull(receiver) {
				return receiver, nil
			}
			if strings.HasPrefix(expr.Callee, "__safe_field:") {
				if receiver.Kind == ValueNull {
					return safeNavigationNull(), nil
				}
				return vm.lookupPath(receiver, []string{strings.TrimPrefix(expr.Callee, "__safe_field:")})
			}
			return vm.lookupPath(receiver, []string{strings.TrimPrefix(expr.Callee, "__field:")})
		}
		var receiver Value
		hasReceiver := expr.Left != nil
		receiverResolved := false
		callee := expr.Callee
		if hasReceiver {
			if receiverName := exprReceiverName(*expr.Left); receiverName != "" {
				member := strings.TrimPrefix(expr.Callee, "__safe_call:")
				if canonical, ok := canonicalBuiltinStaticCall(receiverName + "." + member); ok {
					hasReceiver = false
					callee = canonical
				} else if typeName, fieldName, ok := splitDottedTypeMember(receiverName); ok {
					if value, ok := builtinStaticField(typeName, fieldName); ok {
						receiver = value
						receiverResolved = true
					}
				}
			}
		}
		if hasReceiver && !receiverResolved {
			var err error
			receiver, err = vm.eval(*expr.Left, result)
			if err != nil {
				return Null, err
			}
		}
		if hasReceiver {
			if strings.HasPrefix(callee, "__safe_call:") {
				if receiver.Kind == ValueNull {
					return safeNavigationNull(), nil
				}
				callee = strings.TrimPrefix(callee, "__safe_call:")
			} else if isSafeNavigationNull(receiver) {
				return receiver, nil
			}
		}
		args := make([]Value, 0, len(expr.Args))
		for _, arg := range expr.Args {
			value, err := vm.eval(arg, result)
			if err != nil {
				return Null, err
			}
			args = append(args, plainNull(value))
		}
		namedArgs := make(map[string]Value, len(expr.NamedArgs))
		for _, arg := range expr.NamedArgs {
			value, err := vm.eval(arg.Expr, result)
			if err != nil {
				return Null, err
			}
			namedArgs[arg.Name] = plainNull(value)
		}
		if hasReceiver {
			receiverName := exprReceiverName(*expr.Left)
			value, handled, err := vm.callValueMember(receiverName, receiver, callee, args, result)
			if handled || err != nil {
				return value, err
			}
			return Null, unsupportedCallError(expr.Callee)
		}
		return vm.call(callee, args, namedArgs, result)
	case ir.ExprSOQL:
		return vm.executeSOQL(expr.Value, result)
	default:
		return Null, fmt.Errorf("unsupported expression %q", expr.Kind)
	}
}

func (vm *VM) evalForType(expr ir.Expr, typeName string, result *Result) (Value, error) {
	if expr.Kind == ir.ExprSOQL && typeName != "" {
		return vm.executeSOQLForType(expr.Value, typeName, result)
	}
	return vm.eval(expr, result)
}

func (vm *VM) evalForAssignment(name string, expr ir.Expr, result *Result) (Value, error) {
	if expr.Kind != ir.ExprSOQL {
		return vm.eval(expr, result)
	}
	if typeName := vm.assignmentTargetType(name); typeName != "" {
		return vm.executeSOQLForType(expr.Value, typeName, result)
	}
	return vm.eval(expr, result)
}

func (vm *VM) evalBinary(op string, left, right Value, result *Result) (Value, error) {
	switch op {
	case "+":
		if left.Kind == ValueString || right.Kind == ValueString {
			leftText, err := vm.displayString(left, result)
			if err != nil {
				return Null, err
			}
			rightText, err := vm.displayString(right, result)
			if err != nil {
				return Null, err
			}
			return String(leftText + rightText), nil
		}
		return evalBinary(op, left, right)
	case "==", "!=":
		equal, err := vm.apexEquals(left, right, result)
		if err != nil {
			return Null, err
		}
		if op == "!=" {
			equal = !equal
		}
		return Bool(equal), nil
	case "===", "!==":
		equal := valueIdentityEqual(left, right)
		if op == "!==" {
			equal = !equal
		}
		return Bool(equal), nil
	default:
		return evalBinary(op, left, right)
	}
}

func (vm *VM) apexEquals(left, right Value, result *Result) (bool, error) {
	if left.Kind == ValueNull || right.Kind == ValueNull {
		return left.Kind == ValueNull && right.Kind == ValueNull, nil
	}
	if left.Kind == ValueList && right.Kind == ValueList {
		if len(left.List) != len(right.List) {
			return false, nil
		}
		for i := range left.List {
			equal, err := vm.apexEquals(left.List[i], right.List[i], result)
			if err != nil || !equal {
				return equal, err
			}
		}
		return true, nil
	}
	if left.Kind != ValueObject || platformScalarObject(left.Type) || left.Type == "Type" {
		return left.Equal(right), nil
	}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(left.Type, "equals", []Value{right})
	if ambiguous {
		return false, vm.ambiguousOverloadError(left.Type+".equals", []Value{right})
	}
	if !ok || strings.EqualFold(method.ClassName, "Object") {
		return left.Equal(right), nil
	}
	value, err := vm.callMethodWithReceiver(method, left, []Value{right}, result)
	if err != nil {
		return false, err
	}
	if value.Kind != ValueBool {
		return false, fmt.Errorf("%s.equals returned %s, expected Boolean", left.Type, value.Kind)
	}
	return value.Bool, nil
}

func (vm *VM) evalIncrementExpression(expr ir.Expr, result *Result) (Value, error) {
	if expr.Left != nil && expr.Left.Kind == ir.ExprCall {
		return vm.evalIndexedIncrementExpression(expr, result)
	}
	if expr.Left == nil || expr.Left.Kind != ir.ExprVariable {
		return Null, fmt.Errorf("%s requires assignable variable target", expr.Callee)
	}
	target := expr.Left.Name
	current, err := vm.eval(*expr.Left, result)
	if err != nil {
		return Null, err
	}
	operator := "+"
	if strings.HasSuffix(expr.Callee, "--") {
		operator = "-"
	}
	next, err := evalBinary(operator, current, Int(1))
	if err != nil {
		return Null, err
	}
	if err := vm.assign(target, next); err != nil {
		return Null, err
	}
	if strings.HasPrefix(expr.Callee, "__postfix:") {
		return current, nil
	}
	return next, nil
}

func (vm *VM) evalIndexedIncrementExpression(expr ir.Expr, result *Result) (Value, error) {
	target := expr.Left
	if target == nil || target.Left == nil || target.Left.Kind != ir.ExprVariable || len(target.Args) != 1 {
		return Null, fmt.Errorf("%s requires assignable variable target", expr.Callee)
	}
	receiverName := target.Left.Name
	receiver, err := vm.lookup(receiverName)
	if err != nil {
		return Null, err
	}
	index, err := vm.eval(target.Args[0], result)
	if err != nil {
		return Null, err
	}
	if receiver.Kind != ValueList || index.Kind != ValueInt {
		return Null, fmt.Errorf("%s requires List integer index target", expr.Callee)
	}
	i := int(index.Int)
	if i < 0 || i >= len(receiver.List) {
		return Null, fmt.Errorf("List index out of bounds: %d", i)
	}
	current := receiver.List[i]
	operator := "+"
	if strings.HasSuffix(expr.Callee, "--") {
		operator = "-"
	}
	next, err := evalBinary(operator, current, Int(1))
	if err != nil {
		return Null, err
	}
	receiver.List[i] = next
	if err := vm.storeReceiver(receiverName, receiver); err != nil {
		return Null, err
	}
	if strings.HasPrefix(expr.Callee, "__postfix:") {
		return current, nil
	}
	return next, nil
}

func normalizeStaticCallCasing(callee string) string {
	if canonical, ok := canonicalBuiltinStaticCall(callee); ok {
		return canonical
	}
	dot := strings.IndexByte(callee, '.')
	if dot < 0 {
		return callee
	}
	typeName := callee[:dot]
	member := callee[dot+1:]
	switch strings.ToLower(typeName) {
	case "system":
		switch strings.ToLower(member) {
		case "assert":
			return "System.assert"
		case "assertequals":
			return "System.assertEquals"
		case "assertnotequals":
			return "System.assertNotEquals"
		case "debug":
			return "System.debug"
		}
	case "database":
		if strings.EqualFold(member, "setSavePoint") {
			return "Database.setSavepoint"
		}
	case "pattern":
		switch strings.ToLower(member) {
		case "compile":
			return "Pattern.compile"
		case "matches":
			return "Pattern.matches"
		case "quote":
			return "Pattern.quote"
		}
	case "integer":
		if strings.EqualFold(member, "valueOf") {
			return "Integer.valueOf"
		}
	case "long":
		if strings.EqualFold(member, "valueOf") {
			return "Long.valueOf"
		}
	case "decimal":
		if strings.EqualFold(member, "valueOf") {
			return "Decimal.valueOf"
		}
	case "double":
		if strings.EqualFold(member, "valueOf") {
			return "Double.valueOf"
		}
	case "boolean":
		if strings.EqualFold(member, "valueOf") {
			return "Boolean.valueOf"
		}
	case "userinfo":
		for _, known := range []string{
			"getUserId",
			"getProfileId",
			"getUserName",
			"getName",
			"getFirstName",
			"getLastName",
			"getUserEmail",
			"getOrganizationId",
			"getUserType",
			"getSessionId",
			"getLocale",
			"getLanguage",
			"getTimeZone",
			"isMultiCurrencyOrganization",
		} {
			if strings.EqualFold(member, known) {
				return "UserInfo." + known
			}
		}
	}
	return callee
}

func (vm *VM) shouldUseBuiltinStaticPrecedence(original, canonical string) bool {
	if _, ok := canonicalBuiltinStaticCall(canonical); !ok {
		return false
	}
	if _, systemPrefixed := stripLeadingSystemNamespace(original); systemPrefixed {
		return true
	}
	root, _, ok := strings.Cut(original, ".")
	if !ok {
		return true
	}
	if _, ok := vm.Globals[root]; ok {
		return false
	}
	if actual, found := vm.lookupGlobalName(root); found {
		if _, ok := vm.Globals[actual]; ok {
			return false
		}
	}
	if vm.currentClass != "" {
		if _, _, ok := vm.lookupStaticField(vm.currentClass, root); ok {
			return false
		}
	}
	return true
}

var canonicalBuiltinStaticCalls = func() map[string]string {
	names := []string{
		"System.assert", "System.assertEquals", "System.assertNotEquals", "System.debug", "System.today",
		"Assert.areEqual", "Assert.areNotEqual", "Assert.isTrue", "Assert.isFalse", "Assert.isNull", "Assert.isNotNull", "Assert.fail",
		"System.now", "System.currentTimeMillis", "System.isBatch", "System.isFuture", "System.isQueueable",
		"System.isScheduled", "System.abortJob", "System.attachFinalizer", "System.isRunningTest",
		"Test.isRunningTest", "System.currentPageReference", "System.setPassword", "System.enqueueJob", "System.schedule",
		"Limits.getQueries", "Limits.getLimitQueries", "Limits.getQueryRows", "Limits.getLimitQueryRows",
		"Limits.getDmlStatements", "Limits.getLimitDmlStatements", "Limits.getDMLStatements", "Limits.getLimitDMLStatements",
		"Limits.getDmlRows", "Limits.getLimitDmlRows", "Limits.getDMLRows", "Limits.getLimitDMLRows",
		"Limits.getHeapSize", "Limits.getLimitHeapSize", "Limits.getCpuTime", "Limits.getLimitCpuTime",
		"Limits.getCallouts", "Limits.getLimitCallouts", "Limits.getAsyncJobs", "Limits.getLimitAsyncJobs",
		"Limits.getAsyncCalls", "Limits.getLimitAsyncCalls", "Limits.getQueueableJobs", "Limits.getLimitQueueableJobs",
		"Limits.getFutureCalls", "Limits.getLimitFutureCalls", "Limits.getBatchJobs", "Limits.getLimitBatchJobs",
		"Limits.getScheduledJobs", "Limits.getLimitScheduledJobs",
		"Limits.getEmailInvocations", "Limits.getLimitEmailInvocations",
		"Database.query", "Database.queryWithBinds", "Database.countQuery", "Database.getQueryLocator",
		"Database.setSavepoint", "Database.rollback", "Database.insert", "Database.update", "Database.delete",
		"Database.upsert", "Database.undelete", "Database.emptyRecycleBin", "Database.lock", "Database.unlock",
		"Database.convertLead", "Database.merge",
		"Approval.process", "Approval.lock", "Approval.unlock", "Approval.isLocked",
		"String.valueOf", "String.isBlank", "String.isNotBlank", "String.isEmpty", "String.isNotEmpty",
		"String.join", "String.format", "String.getCommonPrefix", "String.getLevenshteinDistance",
		"String.stripAll", "String.fromCharArray", "String.escapeSingleQuotes",
		"Integer.valueOf", "Long.valueOf", "Decimal.valueOf", "Double.valueOf", "Boolean.valueOf",
		"RoundingMode.valueOf", "Id.valueOf",
		"Pattern.compile", "Pattern.matches", "Pattern.quote",
		"Math.abs", "Math.floor", "Math.ceil", "Math.round", "Math.roundToLong", "Math.signum",
		"Math.sqrt", "Math.acos", "Math.asin", "Math.atan", "Math.cos", "Math.sin", "Math.tan",
		"Math.exp", "Math.log", "Math.log10", "Math.max", "Math.min", "Math.mod", "Math.pow",
		"Math.atan2", "Math.random",
		"Date.today", "Date.newInstance", "Date.valueOf",
		"Datetime.now", "Datetime.newInstance", "Datetime.newInstanceGmt", "Datetime.valueOf", "Datetime.valueOfGmt",
		"Time.newInstance", "Time.valueOf",
		"URL.getSalesforceBaseUrl", "URL.getOrgDomainUrl", "URL.getCurrentRequestUrl",
		"JSON.createGenerator", "JSON.createParser", "JSON.serialize", "JSON.serializePretty",
		"JSON.deserializeUntyped", "JSON.deserialize", "JSON.deserializeStrict",
		"ConnectApi.Organization.getSettings", "ConnectApi.Communities.getCommunity",
		"ConnectApi.UserProfiles.setPhoto", "ConnectApi.UserProfiles.deletePhoto",
		"Auth.AuthToken.revokeAccess", "Auth.SessionManagement.getCurrentSession",
		"Auth.AuthConfiguration.getAuthProviderSsoUrl", "Auth.CommunitiesUtil.isGuestUser",
		"Messaging.sendEmail", "Messaging.renderStoredEmailTemplate",
		"Messaging.reserveSingleEmailCapacity", "Messaging.reserveMassEmailCapacity",
		"ApexPages.hasMessages", "ApexPages.addMessage", "ApexPages.getMessages", "ApexPages.currentPage",
		"Test.clearApexPageMessages", "Test.setCurrentPage", "Test.setCurrentPageReference",
		"Test.setMock", "Test.testInstall", "Test.createStub", "Test.createSoqlStub",
		"Test.setFixedSearchResults", "Test.startTest", "Test.stopTest", "Test.getStandardPricebookId",
		"Test.Database.hasRecords",
		"Site.getSiteId", "Site.getBaseUrl", "Site.getPathPrefix", "Site.getAdminEmail", "Site.getAdminId",
		"Site.getMasterLabel", "Site.isRegistrationEnabled", "Site.isLoginEnabled", "Site.isValidUsername",
		"Site.setExperienceId", "Site.getErrorMessage", "Site.getErrorDescription", "Site.forgotPassword",
		"Site.login", "Site.changePassword", "Site.validatePassword", "Site.createExternalUser", "Site.createPortalUser",
		"Network.getNetworkId", "Network.getLoginUrl", "Network.communitiesLanding",
		"LoggingLevel.values", "ApexPages.Severity.values", "RoundingMode.values",
	}
	calls := make(map[string]string, len(names))
	for _, name := range names {
		calls[strings.ToLower(name)] = name
	}
	for alias, canonical := range map[string]string{
		"Assert.areEqual":    "System.assertEquals",
		"Assert.areNotEqual": "System.assertNotEquals",
		"Assert.isTrue":      "System.assert",
	} {
		calls[strings.ToLower(alias)] = canonical
	}
	return calls
}()

func canonicalBuiltinStaticCall(callee string) (string, bool) {
	if canonical, ok := canonicalBuiltinStaticCalls[strings.ToLower(callee)]; ok {
		return canonical, true
	}
	if rest, ok := stripLeadingSystemNamespace(callee); ok {
		if canonical, ok := canonicalBuiltinStaticCalls[strings.ToLower(rest)]; ok {
			return canonical, true
		}
		if typeName, member, ok := splitDottedTypeMember(rest); ok {
			if canonicalType, typeOK := canonicalSystemNamespaceType(typeName); typeOK {
				callee := canonicalType + "." + member
				if canonical, ok := canonicalBuiltinStaticCalls[strings.ToLower(callee)]; ok {
					return canonical, true
				}
				return callee, true
			}
		}
	}
	return "", false
}

func stripLeadingSystemNamespace(callee string) (string, bool) {
	const prefix = "System."
	if len(callee) <= len(prefix) || !strings.EqualFold(callee[:len(prefix)], prefix) {
		return "", false
	}
	return callee[len(prefix):], true
}

func splitDottedTypeMember(callee string) (string, string, bool) {
	dot := strings.LastIndex(callee, ".")
	if dot <= 0 || dot >= len(callee)-1 {
		return "", "", false
	}
	return callee[:dot], callee[dot+1:], true
}

func canonicalSystemNamespaceType(typeName string) (string, bool) {
	for _, known := range systemNamespaceTypes {
		if strings.EqualFold(typeName, known) {
			return known, true
		}
	}
	return "", false
}

var systemNamespaceTypes = []string{
	"System",
	"Database",
	"String",
	"Integer",
	"Long",
	"Decimal",
	"Double",
	"Boolean",
	"Id",
	"Pattern",
	"Math",
	"Date",
	"Datetime",
	"Time",
	"URL",
	"JSON",
	"Limits",
	"Test",
	"UserInfo",
	"Messaging",
	"ApexPages",
	"RoundingMode",
	"LoggingLevel",
	"Blob",
	"Type",
}

func lexicalOuterClasses(className string) []string {
	outers := []string{}
	for {
		dot := strings.LastIndex(className, ".")
		if dot <= 0 {
			return outers
		}
		className = className[:dot]
		outers = append(outers, className)
	}
}

func (vm *VM) call(callee string, args []Value, namedArgs map[string]Value, result *Result) (Value, error) {
	if strings.HasPrefix(callee, "new:") {
		return vm.constructValue(strings.TrimPrefix(callee, "new:"), args, namedArgs, result)
	}
	if (callee == "this" || callee == "super") && vm.currentMethod.IsConstructor {
		return vm.callChainedConstructor(callee, args, result)
	}
	originalCallee := callee
	callee = normalizeStaticCallCasing(callee)
	if vm.shouldUseBuiltinStaticPrecedence(originalCallee, callee) {
		goto platformStaticCall
	}
	if value, handled, err := vm.callBuiltinStaticFieldMember(callee, args, result); handled || err != nil {
		return value, err
	}
	if className, methodName, ok := vm.splitClassMember(callee); ok {
		if value, handled, err := vm.callEnumStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
		if method, ok, ambiguous := vm.resolveStaticMethodForArgs(className, methodName, args); ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(method.ClassName); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethod(method, args, result)
		} else if ambiguous {
			return Null, vm.ambiguousOverloadError(callee, args)
		}
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
			accessMethod := vm.dispatchAccessMethod(vm.currentClass, method, callee, args)
			if err := vm.checkMemberAccess(accessMethod.ClassName, accessMethod.Access, accessMethod.Name, accessMethod.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(method.ClassName); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethodWithReceiver(method, receiver, args, result)
		} else if ambiguous {
			return Null, vm.ambiguousOverloadError(callee, args)
		}
		if method, ok, ambiguous := vm.resolveStaticMethodForArgs(vm.currentClass, callee, args); ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(method.ClassName); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethod(method, args, result)
		} else if ambiguous {
			return Null, vm.ambiguousOverloadError(callee, args)
		}
		for _, outerClass := range lexicalOuterClasses(vm.currentClass) {
			if method, ok, ambiguous := vm.resolveStaticMethodForArgs(outerClass, callee, args); ok {
				if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(method.ClassName); err != nil {
					return Null, err
				}
				if vm.shouldEnqueueFuture(method) {
					return vm.enqueueFuture(method, args, result)
				}
				return vm.callMethod(method, args, result)
			} else if ambiguous {
				return Null, vm.ambiguousOverloadError(callee, args)
			}
		}
	}
	if method, ok, ambiguous := vm.matchRegisteredMethod(callee, args); ok {
		if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
			return Null, err
		}
		if err := vm.ensureClassInitialized(method.ClassName); err != nil {
			return Null, err
		}
		if vm.shouldEnqueueFuture(method) {
			return vm.enqueueFuture(method, args, result)
		}
		return vm.callMethod(method, args, result)
	} else if ambiguous {
		return Null, vm.ambiguousOverloadError(callee, args)
	}
	if value, handled, err := vm.callSchemaSObjectTypePath(callee, args, result); handled || err != nil {
		return value, err
	}
	if value, handled, err := vm.callDottedReceiverMember(callee, args, result); handled || err != nil {
		return value, err
	}
	if dot := strings.LastIndex(callee, "."); dot > 0 && dot < len(callee)-1 {
		typeName, methodName := callee[:dot], callee[dot+1:]
		if _, classExists := vm.resolveClassName(typeName); !classExists {
			if value, handled, err := vm.callSObjectTypeStaticMember(typeName, methodName, args); handled || err != nil {
				return value, err
			}
		}
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
			if err := vm.ensureClassInitialized(method.ClassName); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethod(method, args, result)
		} else if ambiguous {
			return Null, vm.ambiguousOverloadError(callee, args)
		}
		if value, handled, err := vm.callSObjectTypeStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
	}
	if strings.HasPrefix(callee, "Search.") {
		return Null, unsupportedCallError(callee + " local search/SOSL surface")
	}
platformStaticCall:
	callee = normalizeStaticCallCasing(callee)
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
	case "Assert.isFalse":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Assert.isFalse expects 1 or 2 arguments")
		}
		if args[0].Kind != ValueBool {
			return Null, fmt.Errorf("Assert.isFalse expects Boolean, got %s", args[0].Kind)
		}
		if args[0].Bool {
			message, err := vm.assertMessage("assertion failed", args[1:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "Assert.isNull":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Assert.isNull expects 1 or 2 arguments")
		}
		if args[0].Kind != ValueNull {
			value, err := vm.displayString(args[0], result)
			if err != nil {
				return Null, err
			}
			message, err := vm.assertMessage(fmt.Sprintf("expected null, actual <%s>", value), args[1:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "Assert.isNotNull":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Assert.isNotNull expects 1 or 2 arguments")
		}
		if args[0].Kind == ValueNull {
			message, err := vm.assertMessage("value should not be null", args[1:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "Assert.fail":
		if len(args) > 1 {
			return Null, fmt.Errorf("Assert.fail expects 0 or 1 arguments")
		}
		message, err := vm.assertMessage("assertion failed", args, result)
		if err != nil {
			return Null, err
		}
		return Null, vm.assertError(message)
	case "System.assertEquals":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("System.assertEquals expects 2 or 3 arguments")
		}
		equal, err := vm.apexEquals(args[0], args[1], result)
		if err != nil {
			return Null, err
		}
		if !equal {
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
		equal, err := vm.apexEquals(args[0], args[1], result)
		if err != nil {
			return Null, err
		}
		if equal {
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
		vm.emailSavepoints[id] = append([]CapturedEmail(nil), vm.capturedEmails...)
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
		vm.capturedEmails = append([]CapturedEmail(nil), vm.emailSavepoints[idValue.Text]...)
		for id, order := range vm.savepointOrder {
			if order > targetOrder {
				delete(vm.savepoints, id)
				delete(vm.emailSavepoints, id)
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
		"Limits.getAsyncCalls", "Limits.getLimitAsyncCalls",
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
		if args[0].Kind == ValueNull {
			return Null, nil
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
	case "Boolean.valueOf":
		if len(args) != 1 {
			return Null, fmt.Errorf("Boolean.valueOf expects 1 argument")
		}
		if args[0].Kind == ValueNull {
			return Bool(false), nil
		}
		if args[0].Kind == ValueBool {
			return args[0], nil
		}
		if args[0].Kind != ValueString {
			return Null, fmt.Errorf("Boolean.valueOf expects String or Boolean")
		}
		return Bool(strings.EqualFold(strings.TrimSpace(args[0].Text), "true")), nil
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
	case "Math.random":
		if len(args) != 0 {
			return Null, fmt.Errorf("Math.random expects 0 arguments")
		}
		return Decimal(0.5), nil
	case "Date.today", "Date.Today", "System.today":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return platformScalar("Date", vm.fakeNow.Format("2006-01-02")), nil
	case "Date.newInstance":
		if len(args) != 3 || args[0].Kind != ValueInt || args[1].Kind != ValueInt || args[2].Kind != ValueInt {
			return Null, fmt.Errorf("Date.newInstance expects year, month, day integers")
		}
		year := int(args[0].Int)
		if year == 0 {
			year = 1
		}
		if err := validateDateParts(year, int(args[1].Int), int(args[2].Int)); err != nil {
			return Null, err
		}
		return platformScalar("Date", fmt.Sprintf("%04d-%02d-%02d", year, args[1].Int, args[2].Int)), nil
	case "Date.valueOf":
		if len(args) != 1 {
			return Null, newExceptionError("System.NullPointerException", "Date.valueOf expects String")
		}
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Date.valueOf expects String")
		}
		if args[0].Kind != ValueString {
			return Null, newExceptionError("System.TypeException", "Date.valueOf expects String")
		}
		date, err := parseDateText(args[0].Text)
		if err != nil {
			return Null, newExceptionError("System.TypeException", "Invalid date: "+args[0].Text)
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
		if len(args) == 1 {
			if args[0].Kind != ValueInt {
				return Null, fmt.Errorf("%s expects integer milliseconds", callee)
			}
			return platformScalar("Datetime", formatPlatformDatetime(time.UnixMilli(args[0].Int).UTC())), nil
		}
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
		if year == 0 {
			year = 1
		}
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
		if len(args) != 1 {
			return Null, newExceptionError("System.NullPointerException", fmt.Sprintf("%s expects String", callee))
		}
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", fmt.Sprintf("%s expects String", callee))
		}
		if args[0].Kind != ValueString {
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("%s expects String", callee))
		}
		value, err := parseDatetimeText(args[0].Text)
		if err != nil {
			return Null, newExceptionError("System.TypeException", "Invalid date/time: "+args[0].Text)
		}
		return platformScalar("Datetime", formatPlatformDatetime(value)), nil
	case "LoggingLevel.values":
		return loggingLevelValues(args)
	case "ApexPages.Severity.values":
		return apexPagesSeverityValues(args)
	case "Metadata.DeployStatus.values":
		return metadataDeployStatusValues(args)
	case "Metadata.MetadataType.values":
		return metadataMetadataTypeValues(args)
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
	case "Type.forName", "System.Type.forName":
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
	case "Crypto.encrypt":
		if len(args) != 4 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.encrypt expects algorithm, privateKey Blob, initializationVector Blob, and clearText Blob")
		}
		key, err := blobStringArg("Crypto.encrypt privateKey", args[1:2])
		if err != nil {
			return Null, err
		}
		iv, err := blobStringArg("Crypto.encrypt initializationVector", args[2:3])
		if err != nil {
			return Null, err
		}
		clearText, err := blobStringArg("Crypto.encrypt clearText", args[3:])
		if err != nil {
			return Null, err
		}
		cipherText, err := encryptAESCBC(args[0].Text, []byte(key), []byte(iv), []byte(clearText))
		if err != nil {
			return Null, err
		}
		return platformScalar("Blob", string(cipherText)), nil
	case "Crypto.decrypt", "Crypto.encryptWithManagedIV", "Crypto.decryptWithManagedIV",
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
	case "Schema.describeTabs":
		if len(args) != 0 {
			return Null, fmt.Errorf("Schema.describeTabs expects 0 arguments")
		}
		appendTrace(result, "apex.describe.tabs", "apex.describe", map[string]any{
			"operation": "describeTabs",
			"count":     len(vm.schemaDescribeTabValues()),
		})
		return vm.schemaDescribeTabs(), nil
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
	case "ConnectApi.Communities.getCommunity":
		return vm.connectAPICommunity(args)
	case "ConnectApi.UserProfiles.setPhoto":
		if len(args) != 4 {
			return Null, fmt.Errorf("ConnectApi.UserProfiles.setPhoto expects 4 arguments")
		}
		return Null, nil
	case "ConnectApi.UserProfiles.deletePhoto":
		if len(args) != 2 {
			return Null, fmt.Errorf("ConnectApi.UserProfiles.deletePhoto expects 2 arguments")
		}
		return Null, nil
	case "Metadata.Operations.enqueueDeployment":
		return vm.metadataEnqueueDeployment(args, result)
	case "Metadata.Operations.checkDeployStatus":
		return vm.metadataCheckDeployStatus(args, result)
	case "Metadata.Operations.retrieve":
		return vm.metadataRetrieve(args)
	case "UserManagement.initSelfRegistration":
		if len(args) != 2 {
			return Null, fmt.Errorf("UserManagement.initSelfRegistration expects 2 arguments")
		}
		return String("local-self-registration"), nil
	case "UserManagement.verifySelfRegistration":
		if len(args) != 4 {
			return Null, fmt.Errorf("UserManagement.verifySelfRegistration expects 4 arguments")
		}
		redirect := newPageReference("/")
		if args[3].Kind == ValueString && strings.TrimSpace(args[3].Text) != "" {
			redirect = newPageReference(args[3].Text)
		}
		return newAuthVerificationResult(redirect, Bool(true), Null), nil
	case "Auth.AuthToken.revokeAccess":
		if len(args) != 3 {
			return Null, fmt.Errorf("Auth.AuthToken.revokeAccess expects 3 arguments")
		}
		return Bool(true), nil
	case "Auth.SessionManagement.getCurrentSession":
		if len(args) != 0 {
			return Null, fmt.Errorf("Auth.SessionManagement.getCurrentSession expects 0 arguments")
		}
		session := typedMap("Map<String,String>")
		session.Map[mapKey(String("SessionId"))] = String(vm.currentUserInfoField("Id", "005-local-user") + "-session")
		return session, nil
	case "Auth.AuthConfiguration.getAuthProviderSsoUrl":
		if len(args) != 3 {
			return Null, fmt.Errorf("Auth.AuthConfiguration.getAuthProviderSsoUrl expects 3 arguments")
		}
		communityURL := scalarText(args[0])
		startURL := scalarText(args[1])
		providerName := scalarText(args[2])
		if communityURL == "" {
			communityURL = vm.salesforceBaseURL()
		}
		return String(strings.TrimRight(communityURL, "/") + "/services/auth/sso/" + providerName + "?startURL=" + startURL), nil
	case "Cache.Org.getPartition", "Cache.Session.getPartition":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects String partition name", callee)
		}
		partition := Object("Cache.OrgPartition")
		if strings.HasPrefix(callee, "Cache.Session.") {
			partition.Type = "Cache.SessionPartition"
		}
		partition.Fields["name"] = args[0]
		partition.Fields["scope"] = String(strings.TrimSuffix(callee, ".getPartition"))
		return partition, nil
	case "Messaging.sendEmail":
		return vm.sendEmail(args, result)
	case "Messaging.renderStoredEmailTemplate":
		return vm.renderStoredEmailTemplate(args)
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
	case "ApexPages.currentPage", "System.currentPageReference":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		if vm.currentPage.Kind == "" {
			vm.currentPage = newPageReference("/apex/current")
		}
		return vm.currentPage, nil
	case "Test.setCurrentPage", "Test.setCurrentPageReference":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "PageReference" {
			return Null, fmt.Errorf("%s expects PageReference", callee)
		}
		if err := vm.requireTestContext(callee); err != nil {
			return Null, err
		}
		vm.currentPage = args[0]
		return Null, nil
	case "Messaging.reserveSingleEmailCapacity", "Messaging.reserveMassEmailCapacity",
		"Messaging.renderEmailTemplate",
		"Messaging.sendEmailMessage", "Messaging.sendPushNotification":
		return Null, unsupportedCallError(callee + " local messaging transport/template surface")
	case "URL.getSalesforceBaseUrl", "URL.getOrgDomainUrl":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return platformScalar("URL", vm.salesforceBaseURL()), nil
	case "URL.getCurrentRequestUrl":
		if len(args) != 0 {
			return Null, fmt.Errorf("URL.getCurrentRequestUrl expects 0 arguments")
		}
		return Null, unsupportedCallError(callee + " local current request URL surface")
	case "Test.setMock":
		return vm.testSetMock(args)
	case "Test.testInstall":
		return vm.testInstall(args, result)
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
	case "System.setPassword":
		if len(args) != 2 {
			return Null, fmt.Errorf("System.setPassword expects userId and password")
		}
		appendTrace(result, "apex.user.password.set", "apex.user", map[string]any{"userId": scalarText(args[0])})
		return Null, nil
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
	case "UserInfo.getUserType":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserType expects 0 arguments")
		}
		return String(vm.currentUserInfoField("UserType", "Standard")), nil
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
	case "UserInfo.isMultiCurrencyOrganization":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.isMultiCurrencyOrganization expects 0 arguments")
		}
		return Bool(vm.orgBool("Organization", "IsMultiCurrencyEnabled", false)), nil
	case "Site.getSiteId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getSiteId expects 0 arguments")
		}
		return String(vm.firstOrgRecordID("Site", "local-site")), nil
	case "Site.getBaseUrl":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getBaseUrl expects 0 arguments")
		}
		return String(vm.siteBaseURL()), nil
	case "Site.getPathPrefix":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getPathPrefix expects 0 arguments")
		}
		return String(vm.firstOrgRecordString("Site", "UrlPathPrefix", "")), nil
	case "Site.getAdminEmail":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getAdminEmail expects 0 arguments")
		}
		return String(vm.siteAdminEmail()), nil
	case "Site.getAdminId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getAdminId expects 0 arguments")
		}
		return String(vm.firstOrgRecordIDField("Site", "AdminId", vm.currentUserInfoField("Id", "005-local-user"))), nil
	case "Site.getMasterLabel":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getMasterLabel expects 0 arguments")
		}
		return String(vm.firstOrgRecordString("Site", "MasterLabel", "Local Site")), nil
	case "Site.isRegistrationEnabled":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.isRegistrationEnabled expects 0 arguments")
		}
		return Bool(true), nil
	case "Site.isLoginEnabled":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.isLoginEnabled expects 0 arguments")
		}
		return Bool(true), nil
	case "Site.isValidUsername":
		if len(args) != 1 {
			return Null, fmt.Errorf("Site.isValidUsername expects 1 argument")
		}
		return Bool(args[0].Kind == ValueString && strings.Contains(args[0].Text, "@")), nil
	case "Site.setExperienceId":
		if len(args) != 1 {
			return Null, fmt.Errorf("Site.setExperienceId expects 1 argument")
		}
		return Null, nil
	case "Site.getErrorMessage":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getErrorMessage expects 0 arguments")
		}
		return String(""), nil
	case "Site.getErrorDescription":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getErrorDescription expects 0 arguments")
		}
		return String(""), nil
	case "Site.forgotPassword":
		if len(args) != 1 {
			return Null, fmt.Errorf("Site.forgotPassword expects 1 argument")
		}
		return Null, nil
	case "Site.login":
		if len(args) != 3 {
			return Null, fmt.Errorf("Site.login expects 3 arguments")
		}
		startURL := "/"
		if args[2].Kind == ValueString && strings.TrimSpace(args[2].Text) != "" {
			startURL = args[2].Text
		}
		return newPageReference(startURL), nil
	case "Site.changePassword":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("Site.changePassword expects 2 or 3 arguments")
		}
		if vm.testContext != nil {
			return Null, nil
		}
		return newPageReference("/" + strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")), nil
	case "Site.validatePassword":
		if len(args) != 3 {
			return Null, fmt.Errorf("Site.validatePassword expects 3 arguments")
		}
		return Null, nil
	case "Site.createExternalUser":
		if len(args) != 3 && len(args) != 4 {
			return Null, fmt.Errorf("Site.createExternalUser expects 3 or 4 arguments")
		}
		userID := String("005000000000E01")
		if len(args) > 0 && args[0].Kind == ValueObject {
			args[0].Fields["Id"] = userID
		}
		return userID, nil
	case "Site.createPortalUser":
		if len(args) != 3 {
			return Null, fmt.Errorf("Site.createPortalUser expects 3 arguments")
		}
		userID := String("005000000000E01")
		if len(args) > 0 && args[0].Kind == ValueObject {
			args[0].Fields["Id"] = userID
		}
		return userID, nil
	case "Network.getNetworkId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Network.getNetworkId expects 0 arguments")
		}
		return String(vm.firstOrgRecordID("Network", "0DB-local-network")), nil
	case "Network.getLoginUrl":
		if len(args) != 1 {
			return Null, fmt.Errorf("Network.getLoginUrl expects 1 argument")
		}
		prefix := strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")
		if prefix == "" {
			prefix = "local"
		}
		return String(strings.TrimRight(vm.salesforceBaseURL(), "/") + "/" + prefix + "/login"), nil
	case "Network.communitiesLanding":
		if len(args) != 0 {
			return Null, fmt.Errorf("Network.communitiesLanding expects 0 arguments")
		}
		return newPageReference("/" + strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")), nil
	case "Auth.CommunitiesUtil.isGuestUser":
		if len(args) != 0 {
			return Null, fmt.Errorf("Auth.CommunitiesUtil.isGuestUser expects 0 arguments")
		}
		return Bool(vm.currentUserInfoField("UserType", "") == "Guest"), nil
	default:
		if strings.HasPrefix(callee, "Crypto.") {
			return Null, unsupportedCallError(callee + " local key, certificate, encryption, and random surfaces")
		}
		return Null, unsupportedCallError(callee)
	}
}

func (vm *VM) salesforceBaseURL() string {
	if vm.serverBaseURL != "" {
		return vm.serverBaseURL
	}
	return "https://local.oaer.example"
}

func (vm *VM) siteBaseURL() string {
	base := strings.TrimRight(vm.salesforceBaseURL(), "/")
	prefix := strings.Trim(vm.firstOrgRecordString("Site", "UrlPathPrefix", ""), "/")
	if prefix == "" {
		return base
	}
	return base + "/" + prefix
}

func (vm *VM) siteAdminEmail() string {
	adminID := vm.firstOrgRecordIDField("Site", "AdminId", "")
	if adminID != "" && vm.Org != nil {
		if userObject, ok := vm.Org.Objects["User"]; ok {
			if user, ok := userObject.Records[storage.ID(adminID)]; ok {
				if email, ok := user.Fields["Email"]; ok && email.Kind == storage.ValueString && email.String != "" {
					return email.String
				}
			}
		}
	}
	return "system@example.invalid"
}

func (vm *VM) orgBool(objectName, field string, fallback bool) bool {
	if vm.Org == nil {
		return fallback
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return fallback
	}
	for _, record := range object.Records {
		value, ok := record.Fields[field]
		if ok && value.Kind == storage.ValueBoolean {
			return value.Boolean
		}
	}
	return fallback
}

func (vm *VM) firstOrgRecordID(objectName, fallback string) string {
	if vm.Org == nil {
		return fallback
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return fallback
	}
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return fallback
	}
	return ids[0]
}

func (vm *VM) firstOrgRecordIDField(objectName, field string, fallback string) string {
	value := vm.firstOrgRecordValue(objectName, field)
	if value.Kind == storage.ValueID {
		return string(value.ID)
	}
	if value.Kind == storage.ValueString {
		return value.String
	}
	return fallback
}

func (vm *VM) firstOrgRecordString(objectName, field, fallback string) string {
	value := vm.firstOrgRecordValue(objectName, field)
	if value.Kind == storage.ValueString {
		return value.String
	}
	if value.Kind == storage.ValueID {
		return string(value.ID)
	}
	return fallback
}

func (vm *VM) firstOrgRecordValue(objectName, field string) storage.Value {
	if vm.Org == nil {
		return storage.Value{}
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return storage.Value{}
	}
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		record := object.Records[storage.ID(id)]
		if value, ok := record.Fields[field]; ok {
			return value
		}
	}
	return storage.Value{}
}

func exprReceiverName(expr ir.Expr) string {
	if expr.Kind == ir.ExprVariable {
		return expr.Name
	}
	return ""
}

func nullMemberContext(receiverName, member string) string {
	if receiverName == "" {
		return "while invoking member " + member + " on null receiver"
	}
	return "while invoking " + receiverName + "." + member + " on null receiver " + receiverName
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
	if canonical, ok := canonicalBuiltinStaticCall(callee); ok {
		callee = canonical
	}
	switch {
	case strings.EqualFold(callee, "Auth.AuthConfiguration.getAuthProviderSsoUrl"),
		strings.EqualFold(callee, "Auth.AuthToken.revokeAccess"),
		strings.EqualFold(callee, "Auth.CommunitiesUtil.isGuestUser"),
		strings.EqualFold(callee, "Auth.SessionManagement.getCurrentSession"):
		return "", false
	}
	switch callee {
	case "Auth.AuthConfiguration.getAuthProviderSsoUrl", "Auth.AuthToken.revokeAccess", "Auth.CommunitiesUtil.isGuestUser", "Auth.SessionManagement.getCurrentSession":
		return "", false
	}
	for _, prefix := range []string{"Approval.", "Auth.", "QuickAction.", "Canvas.", "Continuation."} {
		if len(callee) >= len(prefix) && strings.EqualFold(callee[:len(prefix)], prefix) {
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
	triggerRecords := make([]storage.Record, 0, len(records))
	for _, record := range records {
		if record.Kind != ValueObject {
			return Null, fmt.Errorf("EventBus.publish expects SObject event record(s)")
		}
		stored, err := vm.recordFromValue(&record)
		if err != nil {
			return Null, err
		}
		triggerRecords = append(triggerRecords, stored)
		row := Object("Database.SaveResult")
		row.Fields["success"] = Bool(true)
		row.Fields["id"] = Null
		row.Fields["error"] = String("")
		row.Fields["errors"] = List()
		results = append(results, row)
	}
	if _, err := vm.runTriggers(triggerTimingAfter, "insert", triggerRecords, nil, result); err != nil {
		return Null, err
	}
	appendTrace(result, "apex.eventbus.publish", "apex.eventbus", map[string]any{
		"records":  len(records),
		"delivery": "local-after-insert-trigger",
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
	settings.Fields["name"] = String(vm.firstOrgRecordString("Organization", "Name", "Local Organization"))
	settings.Fields["userSettings"] = vm.connectAPIUserSettings()
	return settings, nil
}

func (vm *VM) connectAPIUserSettings() Value {
	settings := Object("ConnectApi.UserSettings")
	settings.Fields["approvalPosts"] = Bool(true)
	settings.Fields["canAccessPersonalStreams"] = Bool(true)
	settings.Fields["canFollow"] = Bool(true)
	settings.Fields["canModifyAllData"] = Bool(true)
	settings.Fields["canOwnGroups"] = Bool(true)
	settings.Fields["canViewAllData"] = Bool(true)
	settings.Fields["canViewAllGroups"] = Bool(true)
	settings.Fields["canViewAllUsers"] = Bool(true)
	settings.Fields["canViewCommunitySwitcher"] = Bool(true)
	settings.Fields["canViewFullUserProfile"] = Bool(true)
	settings.Fields["canViewPublicFiles"] = Bool(true)
	settings.Fields["currencySymbol"] = String("$")
	settings.Fields["externalUser"] = Bool(vm.currentUserInfoField("UserType", "") == "Guest")
	settings.Fields["fileSyncLimit"] = Int(0)
	settings.Fields["fileSyncStorageLimit"] = Int(0)
	settings.Fields["folderSyncLimit"] = Int(0)
	settings.Fields["hasAccessToInternalOrg"] = Bool(true)
	settings.Fields["hasChatter"] = Bool(true)
	settings.Fields["hasFileSync"] = Bool(false)
	settings.Fields["hasFieldServiceLocationTracking"] = Bool(false)
	settings.Fields["hasFieldServiceMobileAccess"] = Bool(false)
	settings.Fields["hasFileSyncManagedClientAutoUpdate"] = Bool(false)
	settings.Fields["hasRestDataApiAccess"] = Bool(true)
	settings.Fields["timeZone"] = connectAPITimeZone(vm.currentUserTimeZoneID())
	settings.Fields["userDefaultCurrencyIsoCode"] = String(vm.currentUserInfoField("DefaultCurrencyIsoCode", "USD"))
	settings.Fields["userId"] = String(vm.currentUserInfoField("Id", "005-local-user"))
	settings.Fields["userLocale"] = String(vm.currentUserInfoField("LocaleSidKey", "en_US"))
	return settings
}

func connectAPITimeZone(name string) Value {
	if strings.TrimSpace(name) == "" {
		name = "America/Los_Angeles"
	}
	tz := Object("ConnectApi.TimeZone")
	tz.Fields["name"] = String(name)
	tz.Fields["gmtOffset"] = Int(0)
	return tz
}

func (vm *VM) connectAPICommunity(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("ConnectApi.Communities.getCommunity expects 1 argument")
	}
	networkID := scalarText(args[0])
	if networkID == "" {
		networkID = vm.firstOrgRecordID("Network", "0DB-local-network")
	}
	prefix := strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")
	community := Object("ConnectApi.Community")
	community.Fields["id"] = String(networkID)
	community.Fields["name"] = String(vm.firstOrgRecordString("Network", "Name", "Local Community"))
	community.Fields["urlPathPrefix"] = String(prefix)
	community.Fields["siteUrl"] = String(strings.TrimRight(vm.salesforceBaseURL(), "/") + "/" + prefix)
	return community, nil
}

func scalarText(value Value) string {
	switch value.Kind {
	case ValueString:
		return value.Text
	case ValueObject:
		if value.Type == "Id" || value.Type == "URL" {
			return value.Text
		}
	}
	return ""
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
			for _, record := range sortedCustomDataRecords(object.Records, definition, kind, vm.Org.Namespace) {
				if !record.System.IsDeleted && customSettingRecordHasNoSetupOwner(record) {
					return record, true, nil
				}
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

func customSettingRecordHasNoSetupOwner(record storage.Record) bool {
	value, ok := record.Fields["SetupOwnerId"]
	if !ok || value.Kind == storage.ValueNull {
		return true
	}
	return value.Kind == storage.ValueString && strings.TrimSpace(value.String) == ""
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
	value := vm.vmValueFromRecord(record)
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

func (vm *VM) currentUserObjectPermission(objectName, method string) bool {
	if method == "isQueryable" || method == "isSearchable" {
		return true
	}
	if vm.Org == nil {
		return true
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	profileID := stringField(user, "ProfileId")
	if vm.currentProfileIsSystemAdministrator(profileID) {
		return true
	}
	for _, permissionSetID := range vm.assignedPermissionSetIDs(stringField(user, "Id")) {
		if allowed, ok := vm.explicitObjectPermission(permissionSetID, objectName, method); ok && allowed {
			return true
		}
	}
	if allowed, ok := vm.explicitObjectPermission(profileID, objectName, method); ok {
		return allowed
	}
	if vm.profileHasLicense(profileID, "Chatter External") && !isSetupObject(objectName) {
		return false
	}
	switch vm.currentProfileName(profileID) {
	case "Minimum Access - Salesforce":
		return false
	case "Read Only":
		return method == "isAccessible"
	}
	return true
}

func (vm *VM) currentUserFieldPermission(objectName, fieldName, method string) bool {
	if vm.Org == nil {
		return true
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	profileID := stringField(user, "ProfileId")
	if vm.currentProfileIsSystemAdministrator(profileID) {
		return true
	}
	for _, permissionSetID := range vm.assignedPermissionSetIDs(stringField(user, "Id")) {
		if allowed, ok := vm.explicitFieldPermission(permissionSetID, objectName, fieldName, method); ok && allowed {
			return true
		}
	}
	if allowed, ok := vm.explicitFieldPermission(profileID, objectName, fieldName, method); ok {
		return allowed
	}
	if method == "isAccessible" && vm.isBaselineReadableField(objectName, fieldName) {
		return true
	}
	switch vm.currentProfileName(profileID) {
	case "Minimum Access - Salesforce":
		return false
	case "Read Only":
		return method == "isAccessible"
	}
	if vm.profileHasLicense(profileID, "Chatter External") && !isSetupObject(objectName) {
		return false
	}
	return true
}

func stringField(value Value, field string) string {
	if value.Kind != ValueObject {
		return ""
	}
	raw, ok := value.Fields[field]
	if !ok {
		return ""
	}
	switch raw.Kind {
	case ValueString:
		return raw.Text
	case ValueObject:
		if raw.Text != "" {
			return raw.Text
		}
		if nested, ok := raw.Fields["value"]; ok && nested.Kind == ValueString {
			return nested.Text
		}
	}
	return ""
}

func (vm *VM) explicitObjectPermission(parentID, objectName, method string) (bool, bool) {
	if parentID == "" || vm.Org == nil {
		return false, false
	}
	state, ok := vm.Org.Objects["ObjectPermissions"]
	if !ok {
		return false, false
	}
	field := objectPermissionField(method)
	for _, record := range state.Records {
		if !storageIDValueEquals(record.Fields["ParentId"], parentID) {
			continue
		}
		if !storageStringValueEquals(record.Fields["SObjectType"], objectName) {
			continue
		}
		value, ok := record.Fields[field]
		if !ok || value.Kind != storage.ValueBoolean {
			return false, false
		}
		return value.Boolean, true
	}
	return false, false
}

func (vm *VM) explicitFieldPermission(parentID, objectName, fieldName, method string) (bool, bool) {
	if parentID == "" || vm.Org == nil {
		return false, false
	}
	state, ok := vm.Org.Objects["FieldPermissions"]
	if !ok {
		return false, false
	}
	field := "PermissionsRead"
	if method == "isCreateable" || method == "isUpdateable" {
		field = "PermissionsEdit"
	}
	for _, record := range state.Records {
		if !storageIDValueEquals(record.Fields["ParentId"], parentID) {
			continue
		}
		if !storageStringValueEquals(record.Fields["SObjectType"], objectName) {
			continue
		}
		if !fieldPermissionFieldMatches(record.Fields["Field"], objectName, fieldName) {
			continue
		}
		value, ok := record.Fields[field]
		if !ok || value.Kind != storage.ValueBoolean {
			return false, false
		}
		return value.Boolean, true
	}
	return false, false
}

func fieldPermissionFieldMatches(value storage.Value, objectName, fieldName string) bool {
	if value.Kind != storage.ValueString {
		return false
	}
	text := value.String
	if dot := strings.LastIndexByte(text, '.'); dot >= 0 {
		return strings.EqualFold(text[:dot], objectName) && strings.EqualFold(text[dot+1:], fieldName)
	}
	return strings.EqualFold(text, fieldName)
}

func objectPermissionField(method string) string {
	switch method {
	case "isCreateable":
		return "PermissionsCreate"
	case "isUpdateable":
		return "PermissionsEdit"
	case "isDeletable":
		return "PermissionsDelete"
	default:
		return "PermissionsRead"
	}
}

func (vm *VM) assignedPermissionSetIDs(userID string) []string {
	if userID == "" || vm.Org == nil {
		return nil
	}
	state, ok := vm.Org.Objects["PermissionSetAssignment"]
	if !ok {
		return nil
	}
	var out []string
	for _, record := range state.Records {
		if storageIDValueEquals(record.Fields["AssigneeId"], userID) {
			if id := storageValueIDText(record.Fields["PermissionSetId"]); id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

func (vm *VM) currentProfileIsSystemAdministrator(profileID string) bool {
	return vm.currentProfileName(profileID) == "System Administrator"
}

func (vm *VM) currentProfileName(profileID string) string {
	if profileID == "" || vm.Org == nil {
		return ""
	}
	profiles, ok := vm.Org.Objects["Profile"]
	if !ok {
		return ""
	}
	profile, ok := profiles.Records[storage.ID(profileID)]
	if !ok {
		return ""
	}
	if value, ok := profile.Fields["Name"]; ok && value.Kind == storage.ValueString {
		return value.String
	}
	return ""
}

func (vm *VM) isBaselineReadableField(objectName, fieldName string) bool {
	if strings.EqualFold(fieldName, "Id") {
		return true
	}
	if vm.Org == nil {
		return false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, objectName)
	if !ok {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	fieldName, ok = storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return false
	}
	field := definition.Fields[fieldName]
	return field.Required || isNameFieldDescribe(field)
}

func (vm *VM) profileHasLicense(profileID, licenseName string) bool {
	if profileID == "" || vm.Org == nil {
		return false
	}
	profiles, ok := vm.Org.Objects["Profile"]
	if !ok {
		return false
	}
	profile, ok := profiles.Records[storage.ID(profileID)]
	if !ok {
		return false
	}
	licenseID := storageValueIDText(profile.Fields["UserLicenseId"])
	if licenseID == "" {
		return false
	}
	licenses, ok := vm.Org.Objects["UserLicense"]
	if !ok {
		return false
	}
	license, ok := licenses.Records[storage.ID(licenseID)]
	if !ok {
		return false
	}
	return storageStringValueEquals(license.Fields["Name"], licenseName)
}

func storageIDValueEquals(value storage.Value, text string) bool {
	return storageValueIDText(value) == text
}

func storageValueIDText(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return value.String
	default:
		return ""
	}
}

func storageStringValueEquals(value storage.Value, text string) bool {
	return value.Kind == storage.ValueString && strings.EqualFold(value.String, text)
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

func (vm *VM) testInstall(args []Value, result *Result) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.testInstall expects InstallHandler and previousVersion")
	}
	if err := vm.requireTestContext("Test.testInstall"); err != nil {
		return Null, err
	}
	handler := args[0]
	if handler.Kind != ValueObject || handler.Type == "" {
		return Null, fmt.Errorf("Test.testInstall expects InstallHandler")
	}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(handler.Type, "onInstall", []Value{Object("InstallContext")})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(handler.Type+".onInstall", []Value{Object("InstallContext")})
	}
	if !ok {
		return Null, fmt.Errorf("Test.testInstall expects InstallHandler with onInstall")
	}
	context := Object("InstallContext")
	context.Fields["PreviousVersion"] = args[1]
	if _, err := vm.callMethodWithReceiver(method, handler, []Value{context}, result); err != nil {
		return Null, err
	}
	return Null, nil
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
	if collectionBase(typeName) == "List" || typeName == "Object" {
		if collectionBase(typeName) == "List" {
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
		return Null, newExceptionError("QueryException", "List has no rows for assignment to SObject")
	}
	if len(value.List) > 1 {
		return Null, newExceptionError("QueryException", "List has more than 1 row for assignment to SObject")
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
	query, err := soql.ParseAt(queryText, vm.fakeNow)
	if err != nil {
		var unsupported *soql.UnsupportedFeatureError
		if errors.As(err, &unsupported) {
			return nil, &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
		}
		return nil, newExceptionError("QueryException", fmt.Sprintf("%s in generated SOQL %q", err.Error(), queryText))
	}
	if err := vm.enforceSOQLSecurity(query); err != nil {
		return nil, err
	}
	result, err := soql.Execute(*vm.Org, query)
	if err != nil {
		var unsupported *soql.UnsupportedFeatureError
		if errors.As(err, &unsupported) {
			return nil, &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
		}
		return nil, newExceptionError("QueryException", fmt.Sprintf("%s in generated SOQL %q", err.Error(), queryText))
	}
	limitRows := soqlLimitRows(result)
	if err := vm.incrementLimit("queryRows", limitRows); err != nil {
		return nil, err
	}
	if err := vm.incrementLimit("cpuTime", limitRows); err != nil {
		return nil, err
	}
	values := make([]Value, 0, len(result.Records))
	queriedFields := vm.queriedSObjectFields(queryText)
	for _, record := range result.Records {
		value := vm.vmValueFromRecord(record)
		if len(queriedFields) > 0 && value.Kind == ValueObject {
			value.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Object, queriedFields)
		}
		values = append(values, value)
	}
	if execResult != nil {
		execResult.Trace = append(execResult.Trace, trace.Instant("apex.soql", "apex.soql", int64(len(execResult.Trace)), map[string]any{
			"query": queryText,
			"rows":  result.Rows,
		}))
	}
	return values, nil
}

func (vm *VM) queriedSObjectFields(queryText string) map[string]bool {
	query, err := soql.Parse(queryText)
	if err != nil || query.Count || len(query.Aggregates) > 0 || len(query.GroupBy) > 0 {
		return nil
	}
	objectName := query.Object
	if vm.Org != nil {
		if canonical, ok := storage.ResolveObjectName(*vm.Org, objectName); ok {
			objectName = canonical
		}
	}
	fields := make(map[string]bool)
	for _, field := range query.Fields {
		if strings.Contains(field, "(") {
			continue
		}
		if dot := strings.IndexByte(field, '.'); dot >= 0 {
			relationship := field[:dot]
			if lookupField, ok := vm.parentRelationshipField(objectName, relationship); ok {
				fields[strings.ToLower(lookupField)] = true
			}
			field = relationship
		}
		if vm.Org != nil {
			if object, ok := vm.Org.Objects[objectName]; ok {
				if canonical, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
					field = canonical
				}
			}
		}
		fields[strings.ToLower(field)] = true
	}
	if len(fields) == 0 {
		return nil
	}
	fields["id"] = true
	return fields
}

func (vm *VM) parentRelationshipField(objectName, relationshipName string) (string, bool) {
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" || strings.TrimSpace(relationshipName) == "" {
		return "", false
	}
	canonicalObject, ok := storage.ResolveObjectName(*vm.Org, objectName)
	if !ok {
		canonicalObject = objectName
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return "", false
	}
	for _, relation := range object.Definition.Relations {
		if vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) && strings.TrimSpace(relation.Field) != "" {
			return relation.Field, true
		}
	}
	return "", false
}

func (vm *VM) parentRelationshipNameForField(definition storage.ObjectDefinition, fieldName string) (string, bool) {
	if vm == nil || strings.TrimSpace(fieldName) == "" {
		return "", false
	}
	for _, relation := range definition.Relations {
		if strings.EqualFold(relation.Field, fieldName) && strings.TrimSpace(relation.ParentRelationship) != "" {
			return relation.ParentRelationship, true
		}
	}
	return "", false
}

func (vm *VM) enforceSOQLSecurity(query soql.Query) error {
	mode := strings.ToUpper(strings.TrimSpace(query.SecurityMode))
	if mode == "" || mode == "SYSTEM_MODE" {
		return nil
	}
	objectName := query.Object
	if vm.Org != nil {
		if canonical, ok := storage.ResolveObjectName(*vm.Org, objectName); ok {
			objectName = canonical
		}
	}
	if !vm.currentUserObjectPermission(objectName, "isAccessible") {
		return newExceptionError("QueryException", fmt.Sprintf("%s requires read access to %s", mode, objectName))
	}
	for _, field := range query.Fields {
		for _, fieldName := range vm.securityFieldNames(objectName, field) {
			if !vm.currentUserFieldPermission(objectName, fieldName, "isAccessible") {
				return newExceptionError("QueryException", fmt.Sprintf("%s requires read access to %s.%s", mode, objectName, fieldName))
			}
		}
	}
	for _, order := range query.Order {
		for _, fieldName := range vm.securityFieldNames(objectName, order.Field) {
			if !vm.currentUserFieldPermission(objectName, fieldName, "isAccessible") {
				return newExceptionError("QueryException", fmt.Sprintf("%s requires read access to %s.%s", mode, objectName, fieldName))
			}
		}
	}
	for _, child := range query.ChildQueries {
		if err := vm.enforceSOQLSecurity(child.Query); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) securityFieldNames(objectName, expression string) []string {
	expression = strings.TrimSpace(expression)
	if expression == "" || strings.Contains(expression, "(") {
		return nil
	}
	if dot := strings.IndexByte(expression, '.'); dot >= 0 {
		expression = expression[:dot]
	}
	if vm.Org != nil {
		if object, ok := vm.Org.Objects[objectName]; ok {
			if canonical, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, expression); ok {
				expression = canonical
			}
		}
	}
	if strings.EqualFold(expression, "Id") {
		return nil
	}
	return []string{expression}
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
	return vm.expandSOQLBindsWith(raw, vm.lookup, func(name string) (Value, error) {
		return vm.call(name, nil, nil, resultForLookup())
	})
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
	}, nil)
}

func (vm *VM) expandSOQLBindsWith(raw string, lookup func(string) (Value, error), call func(string) (Value, error)) (string, error) {
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
		callEnd, isCall := consumeEmptyCallSuffix(raw, j)
		nameString := name.String()
		if isSOQLLiteralBind(nameString) {
			out.WriteString(strings.ToLower(nameString))
			i = j
			continue
		}
		value, err := lookup(nameString)
		if err != nil && isCall && call != nil {
			value, err = call(nameString)
		}
		if err != nil {
			return "", err
		}
		if value.Kind == ValueList || value.Kind == ValueSet {
			rewriteTrailingSOQLEqualsToIn(&out)
		}
		out.WriteString(soqlLiteral(value))
		if isCall {
			i = callEnd
		} else {
			i = j
		}
	}
	return out.String(), nil
}

func isSOQLLiteralBind(name string) bool {
	return strings.EqualFold(name, "true") || strings.EqualFold(name, "false") || strings.EqualFold(name, "null")
}

func rewriteTrailingSOQLEqualsToIn(out *strings.Builder) {
	text := out.String()
	trimmed := strings.TrimRight(text, " \t\n\r")
	if !strings.HasSuffix(trimmed, "=") {
		return
	}
	out.Reset()
	out.WriteString(strings.TrimRight(trimmed[:len(trimmed)-1], " \t\n\r"))
	out.WriteString(" IN ")
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
			return databaseDMLException(op, results)
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
	engine := vm.newDMLEngine(result)
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
			if result.StatusCode != "" {
				message += ": " + result.StatusCode + ": " + result.Error
			} else {
				message += ": " + result.Error
			}
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
	engine := vm.newDMLEngine(result)
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
		vm.applySObjectFieldDefaults(records)
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
	engine := vm.newDeferredAutomationDMLEngine(result)
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
	if op == "insert" || op == "update" || op == "upsert" {
		if err := vm.applyDeferredAutomation(&engine, afterRecords, allOrNone, backup, result); err != nil {
			return results, err
		}
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
			if value, ok := storage.DefaultValueForRecordField(definition, records[i], field); ok {
				records[i].Fields[name] = value
			}
		}
		vm.applyTestSObjectNameDefault(definition, &records[i])
	}
}

func (vm *VM) applyTestSObjectNameDefault(definition storage.ObjectDefinition, record *storage.Record) {
	if vm.testContext == nil || record == nil || record.Fields == nil {
		return
	}
	if _, ok := record.Fields["Name"]; ok {
		return
	}
	if record.ExplicitNulls != nil && record.ExplicitNulls["Name"] {
		return
	}
	field, ok := definition.Fields["Name"]
	if !ok || !field.Required || field.Type != storage.FieldString || field.AutoNumber || field.DefaultValue != "" {
		return
	}
	if !strings.HasSuffix(definition.APIName, "__c") && !strings.HasSuffix(definition.APIName, "__e") {
		return
	}
	name := strings.TrimSpace(definition.Label)
	if name == "" {
		name = strings.TrimSuffix(definition.APIName, "__c")
		name = strings.TrimSuffix(name, "__e")
	}
	if name == "" {
		name = "Test Record"
	}
	record.Fields["Name"] = storage.StringValue(name)
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
	engine := vm.newDeferredAutomationDMLEngine(result)
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
		if err := vm.applyDeferredAutomation(&engine, afterRecords, allOrNone, backup, result); err != nil {
			return results, err
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
	nextSetup := vm.testContext.SetupDML || hasSetup
	nextNonSetup := vm.testContext.NonSetupDML || hasNonSetup
	if nextSetup && nextNonSetup {
		return newExceptionError("DmlException", "Mixed DML operation detected; wrap supported setup/non-setup test work in System.runAs")
	}
	vm.testContext.SetupDML = nextSetup
	vm.testContext.NonSetupDML = nextNonSetup
	return nil
}

func isSetupObject(objectName string) bool {
	switch strings.ToLower(objectName) {
	case "user", "profile", "userrole", "permissionset", "permissionsetassignment", "fieldpermissions", "objectpermissions", "setupentityaccess":
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
	record.ID = sObjectIDFromFields(value.Fields)
	for field, fieldValue := range value.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		if strings.EqualFold(field, "Id") {
			continue
		}
		if strings.EqualFold(field, "OwnerId") {
			converted, err := storageValueFromVM(fieldValue)
			if err != nil {
				return storage.Record{}, fmt.Errorf("%s.%s: %w", value.Type, field, err)
			}
			if ownerID := storageIDFromValue(converted); ownerID != "" {
				record.System.OwnerID = ownerID
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
		if definition.APIName != "" {
			if fieldDef, ok := definition.Fields[canonicalField]; ok && (fieldDef.Type == storage.FieldCalculated || fieldDef.Type == storage.FieldSummary) {
				continue
			}
		}
		if fieldValue.Kind == ValueList && vm.isChildRelationshipField(definition, field) {
			continue
		}
		if fieldValue.Kind == ValueObject && vm.isParentRelationshipField(definition, field) {
			continue
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

func (vm *VM) isParentRelationshipField(definition storage.ObjectDefinition, field string) bool {
	if vm == nil || vm.Org == nil || definition.APIName == "" || strings.TrimSpace(field) == "" {
		return false
	}
	for _, relation := range definition.Relations {
		if vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, field) {
			return true
		}
	}
	return false
}

func (vm *VM) isChildRelationshipField(definition storage.ObjectDefinition, field string) bool {
	if vm == nil || vm.Org == nil || definition.APIName == "" || strings.TrimSpace(field) == "" {
		return false
	}
	for _, childObject := range vm.Org.Objects {
		for _, relation := range childObject.Definition.Relations {
			if !relationshipTargetsObject(relation, definition.APIName) {
				continue
			}
			childRelationshipName := relation.ChildRelationship
			if childRelationshipName == "" {
				childRelationshipName = derivedVMChildRelationshipName(childObject.Definition)
			}
			if vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, field) {
				return true
			}
		}
	}
	return false
}

func derivedVMChildRelationshipName(definition storage.ObjectDefinition) string {
	if strings.TrimSpace(definition.PluralLabel) != "" {
		return normalizeDerivedChildRelationshipName(definition.PluralLabel)
	}
	if strings.TrimSpace(definition.Label) != "" {
		return normalizeDerivedChildRelationshipName(definition.Label)
	}
	if definition.APIName != "" {
		return normalizeDerivedChildRelationshipName(definition.APIName)
	}
	return ""
}

func normalizeDerivedChildRelationshipName(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, "ys") && len(name) > 2 {
		return strings.TrimSuffix(name, "ys") + "ies"
	}
	if strings.HasSuffix(name, "s") {
		return name
	}
	if strings.HasSuffix(name, "y") && len(name) > 1 {
		return strings.TrimSuffix(name, "y") + "ies"
	}
	return name + "s"
}

func sObjectIDFromFields(fields map[string]Value) storage.ID {
	for _, name := range []string{"Id", "id"} {
		if id, ok := sObjectIDFromValue(fields[name]); ok {
			return id
		}
	}
	names := make([]string, 0)
	for name := range fields {
		if strings.EqualFold(name, "Id") && name != "Id" && name != "id" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if id, ok := sObjectIDFromValue(fields[name]); ok {
			return id
		}
	}
	return ""
}

func sObjectIDFromValue(value Value) (storage.ID, bool) {
	if value.Kind == ValueString {
		if value.Text == "" {
			return "", false
		}
		return storage.ID(value.Text), true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		if raw, err := platformScalarText(value, "Id"); err == nil && raw != "" {
			return storage.ID(raw), true
		}
	}
	return "", false
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

func (vm *VM) hydrateParentLookupFields(value Value) {
	if vm == nil || vm.Org == nil || value.Kind != ValueObject {
		return
	}
	object, ok := vm.Org.Objects[value.Type]
	if !ok {
		return
	}
	for _, relation := range object.Definition.Relations {
		if strings.TrimSpace(relation.Field) == "" || strings.TrimSpace(relation.ParentRelationship) == "" {
			continue
		}
		if _, exists := value.Fields[relation.Field]; exists {
			continue
		}
		relationshipValue, ok := value.Fields[relation.ParentRelationship]
		if !ok || relationshipValue.Kind != ValueObject {
			continue
		}
		if _, idValue, ok := objectFieldValue(relationshipValue, "Id"); ok && idValue.Kind != ValueNull {
			value.Fields[relation.Field] = idValue
		}
	}
}

func (vm *VM) vmValueFromRecord(record storage.Record) Value {
	value := Object(record.Object)
	if record.ID != "" {
		value.Fields["Id"] = String(string(record.ID))
	}
	for field, fieldValue := range record.Fields {
		vm.putVMRecordFieldPath(value, record.Object, field, vmValueFromStorage(fieldValue))
	}
	for relationship, records := range record.Children {
		children := make([]Value, 0, len(records))
		for _, child := range records {
			children = append(children, vm.vmValueFromRecord(child))
		}
		value.Fields[relationship] = List(children...)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			value.Fields[field] = Null
		}
	}
	vm.hydrateParentLookupFields(value)
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

func (vm *VM) putVMRecordFieldPath(root Value, objectName, field string, fieldValue Value) {
	if !strings.Contains(field, ".") {
		root.Fields[field] = fieldValue
		return
	}
	parts := strings.Split(field, ".")
	current := root
	currentObject := objectName
	for _, part := range parts[:len(parts)-1] {
		next, ok := current.Fields[part]
		if !ok || next.Kind != ValueObject {
			nextType := part
			if parentType, ok := vm.parentRelationshipObjectType(currentObject, part); ok {
				nextType = parentType
			}
			next = Object(nextType)
			current.Fields[part] = next
		}
		current = next
		if parentType, ok := vm.parentRelationshipObjectType(currentObject, part); ok {
			currentObject = parentType
		} else if next.Type != "" {
			currentObject = next.Type
		}
	}
	current.Fields[parts[len(parts)-1]] = fieldValue
}

func (vm *VM) parentRelationshipObjectType(objectName, relationshipName string) (string, bool) {
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" || strings.TrimSpace(relationshipName) == "" {
		return "", false
	}
	canonicalObject, ok := storage.ResolveObjectName(*vm.Org, objectName)
	if !ok {
		canonicalObject = objectName
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return "", false
	}
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) {
			continue
		}
		for _, parent := range relation.ParentObjects {
			if canonicalParent, ok := storage.ResolveObjectName(*vm.Org, parent); ok {
				return canonicalParent, true
			}
			if strings.TrimSpace(parent) != "" {
				return parent, true
			}
		}
	}
	return "", false
}

func vmRelationshipNameMatches(namespace, canonical, candidate string) bool {
	if canonical == candidate || strings.EqualFold(canonical, candidate) {
		return true
	}
	if strings.HasSuffix(candidate, "__r") && strings.EqualFold(canonical+"__r", candidate) {
		return true
	}
	if strings.HasSuffix(canonical, "__r") && strings.EqualFold(strings.TrimSuffix(canonical, "__r"), candidate) {
		return true
	}
	if namespace == "" {
		return false
	}
	stripped := storage.StripNamespaceToken(namespace, candidate)
	if stripped == canonical || strings.EqualFold(stripped, canonical) {
		return true
	}
	if strings.HasSuffix(stripped, "__r") && strings.EqualFold(canonical+"__r", stripped) {
		return true
	}
	return strings.HasSuffix(canonical, "__r") && strings.EqualFold(strings.TrimSuffix(canonical, "__r"), stripped)
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
	case storage.FieldID, storage.FieldReference:
		if value.Kind == ValueString {
			return storage.IDValue(storage.ID(value.Text)), nil
		}
		if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
			if raw, err := platformScalarText(value, "Id"); err == nil {
				return storage.IDValue(storage.ID(raw)), nil
			}
		}
	case storage.FieldString, storage.FieldPicklist:
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
		if value.Kind == ValueObject && value.Type == "Date" {
			if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
				return storage.DateTimeValue(raw.Text + "T00:00:00Z"), nil
			}
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
		return platformScalar("Id", string(value.ID))
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
		"Trigger.isUnDelete":    Bool(trigger.Operation == "undelete"),
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

func idTextFromValue(value Value) (string, bool) {
	switch value.Kind {
	case ValueString:
		return value.Text, true
	case ValueObject:
		if strings.EqualFold(value.Type, "Id") {
			return platformScalarObjectText(value)
		}
	}
	return "", false
}

func idPrefix(idText string) string {
	if len(idText) < 3 {
		return idText
	}
	return idText[:3]
}

var standardSObjectPrefixes = map[string]string{
	"001": "Account",
	"003": "Contact",
	"005": "User",
	"006": "Opportunity",
	"00G": "Group",
	"00Q": "Lead",
	"00T": "Task",
	"00U": "Event",
	"00D": "Organization",
	"500": "Case",
	"701": "Campaign",
}

func init() {
	for objectName, prefix := range storage.StandardKeyPrefixes() {
		if prefix != "" {
			standardSObjectPrefixes[prefix] = objectName
		}
	}
}

func CommonSObjectTypeNames() []string {
	names := make([]string, 0, len(standardSObjectPrefixes))
	seen := make(map[string]bool, len(standardSObjectPrefixes))
	for _, name := range standardSObjectPrefixes {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	sort.Strings(names)
	return names
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
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999Z0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05Z0700",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
	} {
		if value, err := time.Parse(layout, text); err == nil {
			year := value.Year()
			if year == 0 {
				year = 1
				value = time.Date(year, value.Month(), value.Day(), value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location())
			}
			if err := validateDateParts(year, int(value.Month()), value.Day()); err != nil {
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
		return newExceptionError("System.TypeException", fmt.Sprintf("invalid Date parts: year=%d month=%d day=%d", year, month, day))
	}
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if value.Year() != year || int(value.Month()) != month || value.Day() != day {
		return newExceptionError("System.TypeException", fmt.Sprintf("invalid Date parts: year=%d month=%d day=%d", year, month, day))
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

func encryptAESCBC(algorithm string, privateKey, initializationVector, clearText []byte) ([]byte, error) {
	keySize, err := aesKeySizeForAlgorithm(algorithm)
	if err != nil {
		return nil, err
	}
	if len(privateKey) != keySize {
		return nil, fmt.Errorf("Crypto.encrypt %s privateKey expects %d bytes, got %d", normalizeCryptoAlgorithm(algorithm), keySize, len(privateKey))
	}
	if len(initializationVector) != aes.BlockSize {
		return nil, fmt.Errorf("Crypto.encrypt initializationVector expects %d bytes, got %d", aes.BlockSize, len(initializationVector))
	}
	block, err := aes.NewCipher(privateKey)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(clearText, aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, initializationVector).CryptBlocks(out, padded)
	return out, nil
}

func aesKeySizeForAlgorithm(algorithm string) (int, error) {
	switch normalizeCryptoAlgorithm(algorithm) {
	case "AES128":
		return 16, nil
	case "AES192":
		return 24, nil
	case "AES256":
		return 32, nil
	default:
		return 0, fmt.Errorf("unsupported encryption algorithm %q", algorithm)
	}
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
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
			return Decimal(roundHalfEven(n)), nil
		}
	case "Math.roundToLong":
		rounded, err := int64FromFloat("Math.roundToLong", roundHalfEven(n))
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

func builtinEnumStaticValue(typeName, memberName string) (Value, bool) {
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	switch {
	case strings.EqualFold(typeName, "AccessLevel"):
		for _, known := range []string{"USER_MODE", "SYSTEM_MODE"} {
			if strings.EqualFold(memberName, known) {
				return Value{Kind: ValueObject, Type: "AccessLevel", Text: known}, true
			}
		}
	case strings.EqualFold(typeName, "RoundingMode"):
		if mode, ok := canonicalDecimalRoundingModeName(memberName); ok {
			return Value{Kind: ValueObject, Type: "RoundingMode", Text: mode}, true
		}
	case strings.EqualFold(typeName, "LoggingLevel"):
		if level, ok := canonicalLoggingLevelName(memberName); ok {
			return Value{Kind: ValueObject, Type: "LoggingLevel", Text: level}, true
		}
	case strings.EqualFold(typeName, "JSONToken"):
		if token, ok := canonicalJSONTokenName(memberName); ok {
			return Value{Kind: ValueObject, Type: "JSONToken", Text: token}, true
		}
	case strings.EqualFold(typeName, "DisplayType") || strings.EqualFold(typeName, "Schema.DisplayType"):
		return schemaDisplayTypeStaticValue("Schema.DisplayType." + memberName)
	case strings.EqualFold(typeName, "SOAPType") || strings.EqualFold(typeName, "SoapType") || strings.EqualFold(typeName, "Schema.SOAPType") || strings.EqualFold(typeName, "Schema.SoapType"):
		return schemaSOAPTypeStaticValue("Schema.SOAPType." + memberName)
	}
	return Null, false
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
			if id, ok := value.Fields["Id"]; ok && !strings.Contains(value.Type, ".") {
				if idText, ok := idValueText(id); ok && idText != "" {
					attributes["url"] = "/services/data/v60.0/sobjects/" + value.Type + "/" + idText
				}
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
	if collectionBase(typeName) == "List" {
		items, ok := raw.([]any)
		if !ok {
			if records, recordsOK := jsonQueryResultRecords(raw); recordsOK {
				items = records
			} else {
				return Null, jsonTypeMappingError(typeName, raw)
			}
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
	if collectionBase(typeName) == "Set" {
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
	if isMapType(typeName) {
		fields, ok := raw.(map[string]any)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		if !strings.EqualFold(keyType, "String") && !strings.EqualFold(keyType, "Object") {
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
	if strings.EqualFold(typeName, "Object") {
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
				if !jsonAllowedFieldContains(allowed, key) {
					return Null, newExceptionError("JSONException", fmt.Sprintf("JSON.deserializeStrict found unknown field %q for %s", key, typeName))
				}
			}
		}
	}
	for key, item := range fields {
		if key == "attributes" {
			continue
		}
		if _, hasRecords := jsonQueryResultRecords(item); hasRecords {
			if relationshipType, ok := vm.jsonSObjectChildRelationshipType(typeName, key); ok {
				value, err := vm.typedValueFromJSON(relationshipType, item, strict)
				if err != nil {
					return Null, err
				}
				obj.Fields[key] = value
				continue
			}
		}
		if relationshipType, ok := vm.jsonSObjectParentRelationshipType(typeName, key); ok {
			value, err := vm.typedValueFromJSON(relationshipType, item, strict)
			if err != nil {
				return Null, err
			}
			obj.Fields[key] = value
			continue
		}
		if field, _, ok := vm.lookupField(typeName, key); ok && field.Type != "" {
			value, err := vm.typedValueFromJSON(field.Type, item, strict)
			if err != nil {
				return Null, err
			}
			if field.Setter != nil {
				if _, err := vm.callMethodWithReceiver(*field.Setter, obj, []Value{value}, resultForLookup()); err != nil {
					return Null, err
				}
				continue
			}
			fieldName := field.Name
			if fieldName == "" {
				fieldName = key
			}
			obj.Fields[fieldName] = value
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
		if _, hasRecords := jsonQueryResultRecords(item); hasRecords {
			if relationshipType, ok := vm.jsonSObjectChildRelationshipType(typeName, key); ok {
				value, err := vm.typedValueFromJSON(relationshipType, item, strict)
				if err != nil {
					return Null, err
				}
				obj.Fields[key] = value
				continue
			}
		}
		if relationshipType, ok := vm.jsonSObjectParentRelationshipType(typeName, key); ok {
			value, err := vm.typedValueFromJSON(relationshipType, item, strict)
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

func jsonQueryResultRecords(raw any) ([]any, bool) {
	fields, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	records, ok := fields["records"].([]any)
	return records, ok
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

func (vm *VM) jsonSObjectParentRelationshipType(typeName, relationshipName string) (string, bool) {
	if vm.Org == nil {
		return "", false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, typeName)
	if !ok {
		return "", false
	}
	object := vm.Org.Objects[objectName]
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) {
			continue
		}
		if len(relation.ParentObjects) == 0 {
			continue
		}
		return relation.ParentObjects[0], true
	}
	return "", false
}

func (vm *VM) jsonSObjectChildRelationshipType(typeName, relationshipName string) (string, bool) {
	if vm.Org == nil {
		return "", false
	}
	parentObject, ok := storage.ResolveObjectName(*vm.Org, typeName)
	if !ok {
		return "", false
	}
	for childName, childState := range vm.Org.Objects {
		childRelationshipName := ""
		for _, relation := range childState.Definition.Relations {
			if !relationshipTargetsObject(relation, parentObject) {
				continue
			}
			childRelationshipName = relation.ChildRelationship
			if childRelationshipName == "" {
				childRelationshipName = derivedVMChildRelationshipName(childState.Definition)
			}
			if vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, relationshipName) {
				return "List<" + childName + ">", true
			}
		}
	}
	return "", false
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
			if text, textOK := raw.(string); textOK {
				parsed, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(text)))
				if err == nil {
					return Bool(parsed), true, nil
				}
			}
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return Bool(value), true, nil
	case "Integer", "Long":
		value, ok := jsonIntegralNumber(raw)
		if !ok {
			if text, textOK := raw.(string); textOK {
				parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
				if err == nil {
					return Int(parsed), true, nil
				}
			}
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return Int(value), true, nil
	case "Decimal", "Double":
		value, ok := jsonDecimalNumber(raw)
		if !ok {
			if text, textOK := raw.(string); textOK {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
				if err == nil {
					return Decimal(parsed), true, nil
				}
			}
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

func jsonAllowedFieldContains(allowed map[string]struct{}, key string) bool {
	if _, ok := allowed[key]; ok {
		return true
	}
	for candidate := range allowed {
		if strings.EqualFold(candidate, key) {
			return true
		}
	}
	return false
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
	out.Type = "Schema.GlobalDescribeMap"
	if vm.Org == nil {
		return out
	}
	names := make([]string, 0, len(vm.Org.Objects))
	for name := range vm.Org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		token := sObjectTypeToken(name)
		out.Map[mapKey(String(name))] = token
		if lowered := strings.ToLower(name); lowered != name {
			out.Map[mapKey(String(lowered))] = token
		}
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

func (vm *VM) schemaDescribeTabs() Value {
	tabs := vm.schemaDescribeTabValues()
	if len(tabs) == 0 {
		return List()
	}
	tabSet := Object("Schema.DescribeTabSetResult")
	tabSet.Fields["name"] = String("AllTabs")
	tabSet.Fields["label"] = String("All Tabs")
	tabSet.Fields["tabs"] = List(tabs...)
	tabSet.Fields["selected"] = Bool(false)
	return List(tabSet)
}

func (vm *VM) schemaDescribeTabValues() []Value {
	if vm.Org == nil || len(vm.Org.Metadata.Tabs) == 0 {
		return nil
	}
	tabs := append([]storage.TabMetadata(nil), vm.Org.Metadata.Tabs...)
	sort.Slice(tabs, func(i, j int) bool { return tabs[i].Name < tabs[j].Name })
	values := make([]Value, 0, len(tabs))
	for _, tab := range tabs {
		values = append(values, describeTabValue(tab))
	}
	return values
}

func describeTabValue(tab storage.TabMetadata) Value {
	value := Object("Schema.DescribeTabResult")
	label := tab.Label
	if label == "" {
		label = tab.Name
	}
	value.Fields["name"] = String(tab.Name)
	value.Fields["label"] = String(label)
	if strings.TrimSpace(tab.SObjectName) == "" {
		value.Fields["sObjectName"] = Null
	} else {
		value.Fields["sObjectName"] = String(tab.SObjectName)
	}
	value.Fields["custom"] = Bool(tab.Custom)
	value.Fields["iconUrl"] = String(tab.Motif)
	value.Fields["url"] = String("/lightning/o/" + tab.Name + "/list")
	return value
}

func (vm *VM) describeSObjectValue(name string, definition storage.ObjectDefinition) Value {
	storage.EnsureStandardObjectFields(&definition)
	if strings.EqualFold(definition.APIName, "Account") && len(definition.RecordTypes) == 0 {
		storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
	}
	if strings.EqualFold(definition.APIName, "Account") && vm.Org != nil {
		if personAccountName, ok := storage.ResolveObjectName(*vm.Org, "PersonAccount"); ok {
			personAccount := vm.Org.Objects[personAccountName]
			definition.RecordTypes = appendMissingRecordTypes(definition.RecordTypes, personAccount.Definition.RecordTypes)
		}
	}
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
		apiName := field.APIName
		if strings.TrimSpace(apiName) == "" {
			apiName = fieldName
		}
		if strings.TrimSpace(apiName) == "" {
			continue
		}
		token := sObjectFieldToken(name, apiName)
		fieldsMap.Map[mapKey(String(apiName))] = token
		if lowered := strings.ToLower(apiName); lowered != apiName {
			fieldsMap.Map[mapKey(String(lowered))] = token
		}
	}
	for _, fieldName := range []string{"Id", "Name", "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SystemModstamp"} {
		if _, ok := fieldsMap.Map[mapKey(String(fieldName))]; ok {
			continue
		}
		token := sObjectFieldToken(name, fieldName)
		fieldsMap.Map[mapKey(String(fieldName))] = token
		if lowered := strings.ToLower(fieldName); lowered != fieldName {
			fieldsMap.Map[mapKey(String(lowered))] = token
		}
	}
	fields := Object("Schema.SObjectFieldMap")
	fields.Fields["map"] = fieldsMap
	desc.Fields["fields"] = fields
	desc.Fields["fieldSets"] = vm.fieldSetMapValue(name, definition)
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
					if relationship.ChildRelationship == "" {
						relationship.ChildRelationship = childDefinition.PluralLabel
					}
					childRelationships = append(childRelationships, childRelationshipValue(childName, relationship))
				}
			}
		}
	}
	desc.Fields["childRelationships"] = List(childRelationships...)
	recordTypes := make([]Value, 0, len(definition.RecordTypes))
	byName := Map()
	byName.Type = "Schema.RecordTypeInfoByNameMap"
	byDeveloperName := Map()
	byID := Map()
	for _, recordType := range definition.RecordTypes {
		value := recordTypeInfoValue(recordType)
		recordTypes = append(recordTypes, value)
		if name := recordTypeName(recordType); name != "" {
			byName.Map[mapKey(String(name))] = value
		}
		if recordType.DeveloperName != "" {
			byDeveloperName.Map[mapKey(String(recordType.DeveloperName))] = value
		}
		if recordType.ID != "" {
			byID.Map[mapKey(String(string(recordType.ID)))] = value
		}
	}
	desc.Fields["recordTypeInfos"] = List(recordTypes...)
	desc.Fields["recordTypeInfosByName"] = byName
	desc.Fields["recordTypeInfosByDeveloperName"] = byDeveloperName
	desc.Fields["recordTypeInfosById"] = byID
	return desc
}

func appendMissingRecordTypes(recordTypes []storage.RecordTypeInfo, extra []storage.RecordTypeInfo) []storage.RecordTypeInfo {
	for _, candidate := range extra {
		found := false
		for _, existing := range recordTypes {
			if candidate.ID != "" && existing.ID == candidate.ID {
				found = true
				break
			}
			if candidate.DeveloperName != "" && strings.EqualFold(existing.DeveloperName, candidate.DeveloperName) {
				found = true
				break
			}
			if recordTypeName(candidate) != "" && strings.EqualFold(recordTypeName(existing), recordTypeName(candidate)) {
				found = true
				break
			}
		}
		if !found {
			recordTypes = append(recordTypes, candidate)
		}
	}
	return recordTypes
}

func recordTypeName(recordType storage.RecordTypeInfo) string {
	if recordType.Name != "" {
		return recordType.Name
	}
	return recordType.DeveloperName
}

func defaultRecordTypeID(definition storage.ObjectDefinition) storage.ID {
	for _, recordType := range definition.RecordTypes {
		if recordType.Default && (recordType.Available || recordType.Active) && recordType.ID != "" {
			return recordType.ID
		}
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.Available || recordType.Active {
			if recordType.ID != "" {
				return recordType.ID
			}
		}
	}
	return ""
}

func isNameFieldDescribe(field storage.Field) bool {
	return strings.EqualFold(field.APIName, "Name")
}

func isCustomSchemaName(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, "__c") || strings.HasSuffix(name, "__pc") || strings.HasSuffix(name, "__pr")
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

func (vm *VM) fieldSetMapValue(objectName string, definition storage.ObjectDefinition) Value {
	fieldSets := Object("Schema.FieldSetMap")
	m := Map()
	if vm.Org == nil {
		fieldSets.Fields["map"] = m
		return fieldSets
	}
	for _, fieldSet := range vm.Org.Metadata.FieldSets {
		if !metadataObjectNameMatches(vm.Org.Namespace, objectName, fieldSet.ObjectName) {
			continue
		}
		value := vm.fieldSetValue(objectName, definition, fieldSet)
		m.Map[mapKey(String(fieldSet.Name))] = value
		if lower := strings.ToLower(fieldSet.Name); lower != fieldSet.Name {
			m.Map[mapKey(String(lower))] = value
		}
	}
	fieldSets.Fields["map"] = m
	return fieldSets
}

func (vm *VM) fieldSetValue(objectName string, definition storage.ObjectDefinition, fieldSet storage.FieldSetMetadata) Value {
	value := Object("Schema.FieldSet")
	value.Fields["name"] = String(fieldSet.Name)
	label := fieldSet.Label
	if label == "" {
		label = fieldSet.Name
	}
	value.Fields["label"] = String(label)
	members := make([]Value, 0, len(fieldSet.Fields))
	for _, member := range fieldSet.Fields {
		members = append(members, vm.fieldSetMemberValue(objectName, definition, member))
	}
	value.Fields["fields"] = List(members...)
	return value
}

func (vm *VM) fieldSetMemberValue(objectName string, definition storage.ObjectDefinition, member storage.FieldSetMemberMetadata) Value {
	value := Object("Schema.FieldSetMember")
	value.Fields["fieldPath"] = String(member.Field)
	value.Fields["required"] = Bool(member.Required)
	value.Fields["dbRequired"] = Bool(member.Required)
	label := member.Field
	soapType := String("STRING")
	if fieldName, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, member.Field); ok {
		field := definition.Fields[fieldName]
		if field.Label != "" {
			label = field.Label
		} else if field.APIName != "" {
			label = field.APIName
		}
		soapType = schemaSOAPTypeValue(soapTypeForStorageField(field))
		value.Fields["dbRequired"] = Bool(field.Required)
	}
	value.Fields["label"] = String(label)
	value.Fields["type"] = soapType
	return value
}

func metadataObjectNameMatches(namespace, canonical, candidate string) bool {
	if candidate == "" {
		return false
	}
	if strings.EqualFold(canonical, candidate) {
		return true
	}
	if namespace == "" {
		return false
	}
	stripped := storage.StripNamespaceToken(namespace, candidate)
	return strings.EqualFold(canonical, stripped)
}

func recordTypeInfoValue(recordType storage.RecordTypeInfo) Value {
	value := Object("Schema.RecordTypeInfo")
	value.Fields["recordTypeId"] = platformScalar("Id", recordType.ID.String())
	value.Fields["developerName"] = String(recordType.DeveloperName)
	value.Fields["name"] = String(recordTypeName(recordType))
	value.Fields["active"] = Bool(recordType.Active)
	value.Fields["available"] = Bool(recordType.Available || recordType.Active)
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
		if strings.TrimSpace(fieldName) == "" {
			return emptySObjectFieldDescribe(objectName), nil
		}
		return Null, fmt.Errorf("Schema field describe unknown field %s.%s", objectName, fieldName)
	}
	field := definition.Fields[fieldName]
	if field.APIName == "" && strings.EqualFold(fieldName, "Id") {
		field = storage.Field{APIName: "Id", Label: "Record ID", Type: storage.FieldID}
	}
	desc := Object("Schema.DescribeFieldResult")
	desc.Fields["name"] = String(field.APIName)
	desc.Fields["sObjectName"] = String(objectName)
	label := field.Label
	if label == "" {
		label = field.APIName
	}
	desc.Fields["label"] = String(label)
	displayType := field.DisplayType
	if displayType == "" {
		displayType = string(field.Type)
	}
	desc.Fields["type"] = schemaDisplayTypeValue(displayType)
	desc.Fields["soapType"] = schemaSOAPTypeValue(soapTypeForStorageField(field))
	desc.Fields["nillable"] = Bool(!field.Required)
	desc.Fields["externalId"] = Bool(field.ExternalID)
	desc.Fields["unique"] = Bool(field.Unique)
	desc.Fields["encrypted"] = Bool(field.Encrypted)
	desc.Fields["calculated"] = Bool(field.Type == storage.FieldCalculated)
	desc.Fields["nameField"] = Bool(isNameFieldDescribe(field))
	desc.Fields["custom"] = Bool(isCustomSchemaName(field.APIName))
	relationshipName := field.RelationshipName
	if parentRelationship, ok := vm.parentRelationshipNameForField(definition, field.APIName); ok {
		relationshipName = parentRelationship
	}
	if relationshipName == "" {
		desc.Fields["relationshipName"] = Null
	} else {
		desc.Fields["relationshipName"] = String(relationshipName)
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

func emptySObjectFieldDescribe(objectName string) Value {
	desc := Object("Schema.DescribeFieldResult")
	desc.Fields["name"] = String("")
	desc.Fields["sObjectName"] = String(objectName)
	desc.Fields["label"] = String("")
	desc.Fields["type"] = String("")
	desc.Fields["soapType"] = schemaSOAPTypeValue("xsd:string")
	desc.Fields["nillable"] = Bool(true)
	desc.Fields["externalId"] = Bool(false)
	desc.Fields["unique"] = Bool(false)
	desc.Fields["encrypted"] = Bool(false)
	desc.Fields["calculated"] = Bool(false)
	desc.Fields["nameField"] = Bool(false)
	desc.Fields["custom"] = Bool(false)
	desc.Fields["relationshipName"] = Null
	desc.Fields["referenceTo"] = List()
	desc.Fields["picklistValues"] = List()
	return desc
}

type approxVisitKey struct {
	kind ValueKind
	ptr  uintptr
}

func approxValueSize(value Value) int {
	return approxValueSizeSeen(value, make(map[approxVisitKey]bool))
}

func approxValueSizeSeen(value Value, seen map[approxVisitKey]bool) int {
	switch value.Kind {
	case ValueNull:
		return 4
	case ValueInt, ValueDecimal, ValueBool:
		return 8
	case ValueString:
		return len(value.Text)
	case ValueList:
		if len(value.List) > 0 {
			key := approxVisitKey{kind: value.Kind, ptr: reflect.ValueOf(value.List).Pointer()}
			if seen[key] {
				return 0
			}
			seen[key] = true
		}
		total := 24
		for _, item := range value.List {
			total += approxValueSizeSeen(item, seen)
		}
		return total
	case ValueSet:
		if len(value.Set) > 0 {
			key := approxVisitKey{kind: value.Kind, ptr: reflect.ValueOf(value.Set).Pointer()}
			if seen[key] {
				return 0
			}
			seen[key] = true
		}
		total := 24
		for _, item := range value.Set {
			total += approxValueSizeSeen(item, seen)
		}
		return total
	case ValueMap:
		if value.Map != nil {
			key := approxVisitKey{kind: value.Kind, ptr: reflect.ValueOf(value.Map).Pointer()}
			if seen[key] {
				return 0
			}
			seen[key] = true
		}
		total := 24
		for key, item := range value.Map {
			total += len(key) + approxValueSizeSeen(item, seen)
		}
		return total
	case ValueObject:
		if value.Fields != nil {
			key := approxVisitKey{kind: value.Kind, ptr: reflect.ValueOf(value.Fields).Pointer()}
			if seen[key] {
				return 0
			}
			seen[key] = true
		}
		total := 32 + len(value.Type)
		for key, item := range value.Fields {
			total += len(key) + approxValueSizeSeen(item, seen)
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
	if value, ok := vm.lookupTriggerGlobal(name); ok {
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
	if className, memberName, ok := vm.splitClassMember(name); ok {
		if value, ok := builtinEnumStaticValue(className, memberName); ok {
			return value, nil
		}
	}
	if value, ok := metadataDeployStatusStaticValue(name); ok {
		return value, nil
	}
	if value, ok := metadataMetadataTypeStaticValue(name); ok {
		return value, nil
	}
	if value, ok := schemaSOAPTypeStaticValue(name); ok {
		return value, nil
	}
	if value, ok := schemaDisplayTypeStaticValue(name); ok {
		return value, nil
	}
	if strings.HasPrefix(name, "Label.") {
		if value, ok := vm.lookupLabel(name); ok {
			return value, nil
		}
	}
	if strings.HasPrefix(name, "System.Label.") {
		if value, ok := vm.lookupLabel(strings.TrimPrefix(name, "System.")); ok {
			return value, nil
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
	if strings.HasSuffix(strings.ToLower(name), ".class") {
		className := name[:len(name)-len(".class")]
		if exceptionTypeName(className) == "XmlException" {
			return Value{Kind: ValueObject, Type: "Type", Text: "System.XmlException"}, nil
		}
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
		if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
			if root, field, owner, ok := vm.lookupThisFieldRoot(this, parts[0]); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+field.Name, field.Modifiers); err != nil {
					return Null, err
				}
				if field.Getter != nil {
					value, err := vm.callGetter(owner, field, this)
					if err != nil {
						return Null, err
					}
					root = value
				}
				return vm.lookupPath(root, parts[1:])
			}
		}
		if vm.currentClass != "" {
			if field, owner, ok := vm.lookupStaticField(vm.currentClass, parts[0]); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+parts[0], field.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return Null, err
				}
				field, _, _ = vm.lookupStaticField(owner, parts[0])
				root := field.Value
				if field.Getter != nil {
					var err error
					root, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return Null, err
					}
				}
				return vm.lookupPath(root, parts[1:])
			}
		}
		if token, ok := vm.lookupSObjectTypeToken(parts); ok {
			return token, nil
		}
		if len(parts) > 2 {
			if token, ok := vm.lookupSObjectTypeToken(parts[:2]); ok {
				return vm.lookupPath(token, parts[2:])
			}
		}
		if len(parts) > 3 {
			if token, ok := vm.lookupSObjectTypeToken(parts[:3]); ok {
				return vm.lookupPath(token, parts[3:])
			}
		}
		if token, ok := vm.lookupSObjectFieldToken(parts); ok {
			return token, nil
		}
		if len(parts) == 2 {
			if strings.EqualFold(parts[0], "Page") {
				pageName := parts[1]
				if vm.pageReferences != nil {
					registered, ok := vm.pageReferences[strings.ToLower(pageName)]
					if !ok {
						return Null, fmt.Errorf("unknown Visualforce page Page.%s", pageName)
					}
					pageName = registered
				}
				return newPageReference("/apex/" + pageName), nil
			}
			if value, ok := builtinStaticField(parts[0], parts[1]); ok {
				return value, nil
			}
		}
		if len(parts) > 2 {
			if field, owner, ok := vm.lookupStaticField(parts[0], parts[1]); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+parts[1], field.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return Null, err
				}
				field, _, _ = vm.lookupStaticField(owner, parts[1])
				root := field.Value
				if field.Getter != nil {
					var err error
					root, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return Null, err
					}
				}
				return vm.lookupPath(root, parts[2:])
			}
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			if value, ok := builtinStaticField(className, memberName); ok {
				return value, nil
			}
			if field, owner, ok := vm.lookupStaticField(className, memberName); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+memberName, field.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return Null, err
				}
				field, _, _ = vm.lookupStaticField(owner, memberName)
				if field.Getter != nil {
					return vm.callGetter(owner, field, Null)
				}
				return field.Value, nil
			}
			if class, ok := vm.Classes[className]; ok {
				if err := vm.ensureClassInitialized(className); err != nil {
					return Null, err
				}
				for _, enumValue := range class.EnumValues {
					if strings.EqualFold(enumValue, memberName) {
						return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
					}
				}
			}
			if !strings.Contains(className, ".") {
				suffix := "." + className
				for _, class := range vm.Classes {
					if !strings.HasSuffix(class.Name, suffix) || len(class.EnumValues) == 0 {
						continue
					}
					if err := vm.ensureClassInitialized(class.Name); err != nil {
						return Null, err
					}
					for _, enumValue := range class.EnumValues {
						if strings.EqualFold(enumValue, memberName) {
							return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
						}
					}
				}
			}
			if dot := strings.IndexByte(memberName, '.'); dot > 0 {
				nestedEnumName := className + "." + memberName[:dot]
				nestedMemberName := memberName[dot+1:]
				nestedCandidates := []string{nestedEnumName, memberName[:dot]}
				for _, candidate := range nestedCandidates {
					if class, ok := vm.Classes[candidate]; ok {
						if err := vm.ensureClassInitialized(candidate); err != nil {
							return Null, err
						}
						for _, enumValue := range class.EnumValues {
							if strings.EqualFold(enumValue, nestedMemberName) {
								return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
							}
						}
					}
				}
			}
		}
	}
	if len(parts) == 3 && apexIdentifierStartsUpper(parts[0]) && apexIdentifierStartsUpper(parts[1]) && apexIdentifierStartsUpper(parts[2]) {
		return Value{Kind: ValueObject, Type: parts[0] + "." + parts[1], Text: parts[2]}, nil
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if actualName, value, ok := objectFieldValue(this, name); ok {
			if field, owner, ok := vm.lookupReceiverField(this.Type, actualName); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+actualName, field.Modifiers); err != nil {
					return Null, err
				}
				if field.Getter != nil {
					if field.Getter.Name == vm.currentMethod.Name {
						return value, nil
					}
					return vm.callGetter(owner, field, this)
				}
			}
			return value, nil
		}
		if field, owner, ok := vm.lookupReceiverField(this.Type, name); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
				return Null, err
			}
			if field.Getter != nil {
				return vm.callGetter(owner, field, this)
			}
			return defaultValue(field.Type, field.InitialValue), nil
		}
	}
	if vm.currentClass != "" {
		if field, owner, ok := vm.lookupStaticField(vm.currentClass, name); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(owner); err != nil {
				return Null, err
			}
			field, _, _ = vm.lookupStaticField(owner, name)
			if field.Getter != nil {
				return vm.callGetter(owner, field, Null)
			}
			return field.Value, nil
		}
	}
	return Null, fmt.Errorf("unknown variable %q", name)
}

func (vm *VM) lookupThisFieldRoot(this Value, name string) (Value, Field, string, bool) {
	actualName, root, ok := objectFieldValue(this, name)
	if !ok {
		return Null, Field{}, "", false
	}
	field, owner, ok := vm.lookupField(this.Type, actualName)
	if !ok {
		field = Field{Name: actualName, Type: root.Type}
		owner = this.Type
	}
	if field.Name == "" {
		field.Name = actualName
	}
	return root, field, owner, true
}

func objectFieldValue(object Value, name string) (string, Value, bool) {
	if object.Kind != ValueObject || object.Fields == nil {
		return "", Null, false
	}
	if value, ok := object.Fields[name]; ok {
		return name, value, true
	}
	normalized := strings.ToLower(name)
	for candidate, value := range object.Fields {
		if strings.ToLower(candidate) == normalized {
			return candidate, value, true
		}
	}
	return "", Null, false
}

const safeNavigationNullRuntime = "__oaer_safe_navigation_null"

func safeNavigationNull() Value {
	value := Null
	value.Runtime = safeNavigationNullRuntime
	return value
}

func isSafeNavigationNull(value Value) bool {
	return value.Kind == ValueNull && value.Runtime == safeNavigationNullRuntime
}

func plainNull(value Value) Value {
	if isSafeNavigationNull(value) {
		return Null
	}
	return value
}

func (vm *VM) lookupThisSimpleField(name string) (Value, bool, error) {
	this, ok := vm.Globals["this"]
	if !ok || this.Kind != ValueObject {
		return Null, false, nil
	}
	if actualName, value, ok := objectFieldValue(this, name); ok {
		if field, owner, ok := vm.lookupReceiverField(this.Type, actualName); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+actualName, field.Modifiers); err != nil {
				return Null, true, err
			}
			if field.Getter != nil {
				if field.Getter.Name == vm.currentMethod.Name {
					return value, true, nil
				}
				value, err := vm.callGetter(owner, field, this)
				return value, true, err
			}
		}
		return value, true, nil
	}
	field, owner, ok := vm.lookupReceiverField(this.Type, name)
	if !ok {
		return Null, false, nil
	}
	if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
		return Null, true, err
	}
	if field.Getter != nil {
		value, err := vm.callGetter(owner, field, this)
		return value, true, err
	}
	return defaultValue(field.Type, field.InitialValue), true, nil
}

func (vm *VM) callGetter(owner string, field Field, receiver Value) (Value, error) {
	if field.Getter == nil {
		if _, value, ok := objectFieldValue(receiver, field.Name); ok {
			return value, nil
		}
		return field.Value, nil
	}
	fieldName := field.Name
	if fieldName == "" {
		fieldName = strings.TrimPrefix(field.Getter.Name, owner+".")
		fieldName = strings.TrimSuffix(fieldName, ".get")
	}
	key := getterCallKey(owner, fieldName, receiver)
	if vm.activeGetters[key] > 0 {
		if _, value, ok := objectFieldValue(receiver, fieldName); ok {
			return value, nil
		}
		return field.Value, nil
	}
	vm.activeGetters[key]++
	defer func() {
		vm.activeGetters[key]--
		if vm.activeGetters[key] == 0 {
			delete(vm.activeGetters, key)
		}
	}()
	return vm.callMethodWithReceiver(*field.Getter, receiver, nil, resultForLookup())
}

func getterCallKey(owner, fieldName string, receiver Value) string {
	key := owner + "." + fieldName
	if receiver.Kind == ValueObject && receiver.Ref != 0 {
		key += fmt.Sprintf("#%d", receiver.Ref)
	}
	return key
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

func (vm *VM) lookupTriggerGlobal(name string) (Value, bool) {
	if value, ok := vm.triggerGlobals[name]; ok {
		return value, true
	}
	normalized := strings.ToLower(name)
	for candidate, value := range vm.triggerGlobals {
		if strings.ToLower(candidate) == normalized {
			return value, true
		}
	}
	return Null, false
}

func (vm *VM) assignmentTargetType(name string) string {
	if actual, ok := vm.lookupGlobalName(name); ok {
		return vm.VarTypes[actual]
	}
	parts := strings.Split(name, ".")
	if len(parts) <= 1 {
		if vm.currentClass != "" {
			if field, _, ok := vm.lookupStaticField(vm.currentClass, name); ok {
				return field.Type
			}
		}
		if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
			if _, field, _, ok := vm.lookupThisFieldRoot(this, name); ok {
				return field.Type
			}
		}
		return ""
	}
	if rootName, ok := vm.lookupGlobalName(parts[0]); ok {
		return vm.fieldPathTargetType(vm.VarTypes[rootName], parts[1:])
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if _, field, _, ok := vm.lookupThisFieldRoot(this, parts[0]); ok {
			return vm.fieldPathTargetType(field.Type, parts[1:])
		}
	}
	if className, memberName, ok := vm.splitClassMember(name); ok {
		if field, _, ok := vm.lookupStaticField(className, memberName); ok {
			return field.Type
		}
	}
	return ""
}

func (vm *VM) fieldPathTargetType(typeName string, parts []string) string {
	for _, part := range parts {
		if typeName == "" {
			return ""
		}
		if field, _, ok := vm.lookupField(typeName, part); ok {
			typeName = field.Type
			continue
		}
		if vm.Org != nil {
			if objectName, ok := storage.ResolveObjectName(*vm.Org, typeName); ok {
				definition := vm.Org.Objects[objectName].Definition
				if fieldName, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, part); ok {
					field := definition.Fields[fieldName]
					typeName = storageFieldTypeName(field)
					continue
				}
			}
		}
		return ""
	}
	return typeName
}

func storageFieldTypeName(field storage.Field) string {
	switch field.Type {
	case storage.FieldID:
		return "Id"
	case storage.FieldReference:
		if len(field.ReferenceTo) == 1 {
			return field.ReferenceTo[0]
		}
		return "Id"
	case storage.FieldString, storage.FieldPicklist:
		return "String"
	case storage.FieldBoolean:
		return "Boolean"
	case storage.FieldInteger:
		return "Integer"
	case storage.FieldDecimal:
		return "Decimal"
	case storage.FieldDate:
		return "Date"
	case storage.FieldDateTime:
		return "Datetime"
	default:
		return ""
	}
}

var roundingModeNames = []string{"UP", "DOWN", "CEILING", "FLOOR", "HALF_UP", "HALF_DOWN", "HALF_EVEN", "UNNECESSARY"}

func isDecimalRoundingModeName(name string) bool {
	_, ok := canonicalDecimalRoundingModeName(name)
	return ok
}

func canonicalDecimalRoundingModeName(name string) (string, bool) {
	for _, candidate := range roundingModeNames {
		if strings.EqualFold(name, candidate) {
			return candidate, true
		}
	}
	return "", false
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
	case len(parts) == 2 && strings.EqualFold(parts[0], "SObjectType"):
		objectName = parts[1]
	default:
		return Null, false
	}
	canonical, ok := storage.ResolveObjectName(*vm.Org, objectName)
	if !ok {
		if isCommonSObjectTypeName(objectName) || strings.HasSuffix(objectName, "__c") || strings.HasSuffix(objectName, "__e") || strings.HasSuffix(objectName, "__mdt") {
			canonical = objectName
		} else {
			return Null, false
		}
	}
	return sObjectTypeToken(canonical), true
}

func (vm *VM) sObjectTypeTokenForName(objectName string) (Value, bool) {
	if strings.EqualFold(objectName, "SObject") {
		return sObjectTypeToken("SObject"), true
	}
	if strings.EqualFold(objectName, "AggregateResult") {
		return sObjectTypeToken("AggregateResult"), true
	}
	if vm.Org != nil {
		if canonical, ok := storage.ResolveObjectName(*vm.Org, objectName); ok {
			return sObjectTypeToken(canonical), true
		}
	}
	if isCommonSObjectTypeName(objectName) || strings.HasSuffix(objectName, "__c") || strings.HasSuffix(objectName, "__e") || strings.HasSuffix(objectName, "__mdt") {
		return sObjectTypeToken(objectName), true
	}
	return Null, false
}

func (vm *VM) callSObjectTypeStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	if method != "getSObjectType" {
		return Null, false, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.getSObjectType expects 0 arguments", typeName)
	}
	token, ok := vm.sObjectTypeTokenForName(typeName)
	if !ok {
		return Null, false, nil
	}
	return token, true, nil
}

func (vm *VM) lookupSObjectFieldToken(parts []string) (Value, bool) {
	if vm.Org == nil || len(parts) < 2 {
		return Null, false
	}
	objectName := parts[0]
	fieldName := ""
	switch {
	case len(parts) == 2:
		fieldName = parts[1]
	case len(parts) == 3 && strings.EqualFold(parts[1], "Fields"):
		fieldName = parts[2]
	case len(parts) == 4 && strings.EqualFold(parts[1], "SObjectType") && strings.EqualFold(parts[2], "Fields"):
		fieldName = parts[3]
	case len(parts) == 5 && strings.EqualFold(parts[0], "Schema") && strings.EqualFold(parts[2], "SObjectType") && strings.EqualFold(parts[3], "Fields"):
		objectName = parts[1]
		fieldName = parts[4]
	default:
		return Null, false
	}
	canonicalObject, ok := storage.ResolveObjectName(*vm.Org, objectName)
	if !ok {
		return Null, false
	}
	objectName = canonicalObject
	definition := vm.Org.Objects[objectName].Definition
	canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		standardDefinition := definition
		storage.EnsureStandardObjectFieldsForFeatures(&standardDefinition, []string{"PersonAccounts"})
		if standardField, standardOK := storage.ResolveFieldName(standardDefinition, vm.Org.Namespace, fieldName); standardOK {
			canonical = standardField
		} else if !isSObjectSystemField(fieldName) {
			return Null, false
		} else {
			canonical = fieldName
		}
	}
	return sObjectFieldToken(objectName, canonical), true
}

func (vm *VM) callSchemaSObjectTypePath(callee string, args []Value, result *Result) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) >= 4 && strings.EqualFold(parts[len(parts)-2], "fields") && strings.EqualFold(parts[len(parts)-1], "getMap") {
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
	if len(parts) >= 4 && strings.EqualFold(parts[len(parts)-2], "fieldSets") && strings.EqualFold(parts[len(parts)-1], "getMap") {
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
		}
		tokenParts := parts[:len(parts)-2]
		token, ok := vm.lookupSObjectTypeToken(tokenParts)
		if !ok {
			return Null, false, nil
		}
		describe, _, _, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
		if err != nil || !handled {
			return describe, true, err
		}
		fieldSets, ok := describe.Fields["fieldSets"]
		if !ok {
			return Null, true, fmt.Errorf("%s describe field sets are not available", callee)
		}
		value, _, _, _, err := vm.callPlatformObjectMember(fieldSets, "getMap", nil, result)
		return value, true, err
	}
	if len(parts) >= 5 && strings.EqualFold(parts[len(parts)-3], "fieldSets") && strings.EqualFold(parts[len(parts)-1], "getFields") {
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
		}
		tokenParts := parts[:len(parts)-3]
		token, ok := vm.lookupSObjectTypeToken(tokenParts)
		if !ok {
			return Null, false, nil
		}
		describe, _, _, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
		if err != nil || !handled {
			return describe, true, err
		}
		fieldSets, ok := describe.Fields["fieldSets"]
		if !ok {
			return Null, true, fmt.Errorf("%s describe field sets are not available", callee)
		}
		fieldSet, _, _, handled, err := vm.callPlatformObjectMember(fieldSets, "get", []Value{String(parts[len(parts)-2])}, result)
		if err != nil || !handled {
			return fieldSet, true, err
		}
		if fieldSet.Kind == ValueNull {
			return Null, true, newNullDereferenceError("while accessing " + callee)
		}
		value, _, _, _, err := vm.callPlatformObjectMember(fieldSet, "getFields", nil, result)
		return value, true, err
	}
	if len(parts) >= 4 && strings.EqualFold(parts[len(parts)-2], "fields") {
		tokenParts := parts[:len(parts)-2]
		token, ok := vm.lookupSObjectTypeToken(tokenParts)
		if !ok {
			return Null, false, nil
		}
		objectValue, ok := token.Fields["object"]
		if !ok || objectValue.Kind != ValueString {
			return Null, true, fmt.Errorf("%s token missing object", callee)
		}
		fieldToken, ok := vm.lookupSObjectFieldToken([]string{objectValue.Text, parts[len(parts)-1]})
		if !ok {
			return Null, false, nil
		}
		if len(args) == 0 {
			return fieldToken, true, nil
		}
		return Null, true, fmt.Errorf("%s does not accept arguments", callee)
	}
	if len(parts) >= 5 && strings.EqualFold(parts[len(parts)-3], "fields") {
		tokenParts := parts[:len(parts)-3]
		token, ok := vm.lookupSObjectTypeToken(tokenParts)
		if !ok {
			return Null, false, nil
		}
		objectValue, ok := token.Fields["object"]
		if !ok || objectValue.Kind != ValueString {
			return Null, true, fmt.Errorf("%s token missing object", callee)
		}
		fieldToken, ok := vm.lookupSObjectFieldToken([]string{objectValue.Text, parts[len(parts)-2]})
		if !ok {
			return Null, false, nil
		}
		value, _, _, handled, err := vm.callPlatformObjectMember(fieldToken, parts[len(parts)-1], args, result)
		if err != nil || !handled {
			return value, true, err
		}
		return value, true, nil
	}
	if len(parts) < 4 || !schemaSObjectTypeDescribeForwardMethod(parts[len(parts)-1]) {
		return Null, false, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
	}
	tokenParts := parts[:len(parts)-1]
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
	value, _, _, _, err := vm.callPlatformObjectMember(describe, parts[len(parts)-1], nil, result)
	return value, true, err
}

func schemaSObjectTypeDescribeForwardMethod(method string) bool {
	return strings.EqualFold(method, "getRecordTypeInfosByName")
}

func (vm *VM) callDottedReceiverMember(callee string, args []Value, result *Result) (Value, bool, error) {
	dot := strings.LastIndex(callee, ".")
	if dot <= 0 || dot >= len(callee)-1 {
		return Null, false, nil
	}
	receiverName := callee[:dot]
	method := callee[dot+1:]
	if typeName, fieldName, ok := splitDottedTypeMember(receiverName); ok {
		if receiver, ok := builtinStaticField(typeName, fieldName); ok {
			return vm.callValueMember(receiverName, receiver, method, args, result)
		}
	}
	receiver, err := vm.lookup(receiverName)
	if err != nil {
		if !strings.Contains(receiverName, ".") {
			if value, ok, fieldErr := vm.lookupThisSimpleField(receiverName); ok || fieldErr != nil {
				if fieldErr != nil {
					return Null, true, fieldErr
				}
				return vm.callValueMember(receiverName, value, method, args, result)
			}
		}
		return Null, false, nil
	}
	return vm.callValueMember(receiverName, receiver, method, args, result)
}

func (vm *VM) callBuiltinStaticFieldMember(callee string, args []Value, result *Result) (Value, bool, error) {
	dot := strings.LastIndex(callee, ".")
	if dot <= 0 || dot >= len(callee)-1 {
		return Null, false, nil
	}
	receiverName := callee[:dot]
	method := callee[dot+1:]
	typeName, fieldName, ok := splitDottedTypeMember(receiverName)
	if !ok {
		return Null, false, nil
	}
	receiver, ok := builtinStaticField(typeName, fieldName)
	if !ok {
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
	token.Fields["Name"] = String(fieldName)
	token.Fields["name"] = String(fieldName)
	return token
}

func (vm *VM) lookupPath(root Value, parts []string) (Value, error) {
	current := root
	for i, part := range parts {
		if current.Kind == ValueNull {
			return Null, newNullDereferenceError("while accessing " + strings.Join(parts[:i+1], "."))
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
		switch current.Type {
		case "Schema.SObjectType":
			objectValue, ok := current.Fields["object"]
			if !ok || objectValue.Kind != ValueString {
				return Null, fmt.Errorf("Schema.SObjectType token missing object")
			}
			if strings.EqualFold(part, "fields") {
				if vm.Org == nil {
					return Null, fmt.Errorf("Schema.SObjectType.fields requires org state")
				}
				objectName := objectValue.Text
				if canonical, ok := storage.ResolveObjectName(*vm.Org, objectName); ok {
					objectName = canonical
				}
				state, ok := vm.Org.Objects[objectName]
				if !ok {
					return Null, fmt.Errorf("Schema.SObjectType.fields unknown object %s", objectName)
				}
				describe := vm.describeSObjectValue(objectName, state.Definition)
				current = describe.Fields["fields"]
				continue
			}
			if strings.EqualFold(part, "fieldSets") {
				if vm.Org == nil {
					return Null, fmt.Errorf("Schema.SObjectType.fieldSets requires org state")
				}
				objectName := objectValue.Text
				if canonical, ok := storage.ResolveObjectName(*vm.Org, objectName); ok {
					objectName = canonical
				}
				state, ok := vm.Org.Objects[objectName]
				if !ok {
					return Null, fmt.Errorf("Schema.SObjectType.fieldSets unknown object %s", objectName)
				}
				describe := vm.describeSObjectValue(objectName, state.Definition)
				current = describe.Fields["fieldSets"]
				continue
			}
		case "Schema.SObjectFieldMap":
			mapValue, ok := current.Fields["map"]
			if !ok || mapValue.Kind != ValueMap {
				return Null, fmt.Errorf("Schema.SObjectFieldMap is missing map")
			}
			if value, ok := mapValue.Map[mapKey(String(part))]; ok {
				current = value
				continue
			}
			if value, ok := mapValue.Map[mapKey(String(strings.ToLower(part)))]; ok {
				current = value
				continue
			}
			current = Null
			continue
		case "Schema.FieldSetMap":
			mapValue, ok := current.Fields["map"]
			if !ok || mapValue.Kind != ValueMap {
				return Null, fmt.Errorf("Schema.FieldSetMap is missing map")
			}
			if value, ok := mapValue.Map[mapKey(String(part))]; ok {
				current = value
				continue
			}
			if value, ok := mapValue.Map[mapKey(String(strings.ToLower(part)))]; ok {
				current = value
				continue
			}
			current = Null
			continue
		}
		if field, owner, ok := vm.lookupField(current.Type, part); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+part, field.Modifiers); err != nil {
				return Null, err
			}
			if field.Getter != nil {
				value, err := vm.callGetter(owner, field, current)
				if err != nil {
					return Null, err
				}
				current = value
				continue
			}
			if _, value, ok := objectFieldValue(current, field.Name); ok {
				current = value
				continue
			}
			if _, value, ok := objectFieldValue(current, part); ok {
				current = value
				continue
			}
			current = Null
			continue
		}
		canonicalPart := vm.resolveSObjectFieldName(current.Type, part)
		_, value, ok := objectFieldValue(current, canonicalPart)
		if !ok && canonicalPart != part {
			_, value, ok = objectFieldValue(current, part)
		}
		if ok && value.Kind == ValueNull {
			if relationshipValue, hasRelationship := vm.parentRelationshipValue(current, canonicalPart); hasRelationship {
				value = relationshipValue
			}
		}
		if !ok {
			if err := vm.unqueriedSObjectFieldError(current, canonicalPart); err != nil {
				return Null, err
			}
			if value, ok := vm.missingSObjectFieldValue(current, canonicalPart); ok {
				current = value
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
	value = plainNull(value)
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
		if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
			if root, field, owner, ok := vm.lookupThisFieldRoot(this, parts[0]); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+field.Name, field.Modifiers); err != nil {
					return err
				}
				if field.Getter != nil {
					var err error
					root, err = vm.callGetter(owner, field, this)
					if err != nil {
						return err
					}
				}
				return vm.assignPath(root, parts[1:], value)
			}
		}
		if vm.currentClass != "" {
			if field, owner, ok := vm.lookupStaticField(vm.currentClass, parts[0]); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+parts[0], field.Modifiers); err != nil {
					return err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return err
				}
				field, _, _ = vm.lookupStaticField(owner, parts[0])
				root := field.Value
				if field.Getter != nil {
					var err error
					root, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return err
					}
				}
				return vm.assignPath(root, parts[1:], value)
			}
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			if field, owner, ok := vm.lookupStaticField(className, memberName); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+memberName, field.Modifiers); err != nil {
					return err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return err
				}
				field, _, _ = vm.lookupStaticField(owner, memberName)
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
		if actualName, field, ok := objectFieldValue(this, name); ok {
			if def, owner, ok := vm.lookupReceiverField(this.Type, actualName); ok {
				if err := vm.checkMemberAccess(owner, def.Access, owner+"."+name, def.Modifiers); err != nil {
					return err
				}
				coerced, err := vm.coerceAssignable(def.Type, value)
				if err != nil {
					return fmt.Errorf("%s.%s: %w", this.Type, name, err)
				}
				value = coerced
				if def.Setter != nil {
					key := owner + "." + name
					if vm.activeSetters[key] > 0 {
						this.Fields[actualName] = value
						vm.Globals["this"] = this
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
			this.Fields[actualName] = value
			vm.Globals["this"] = this
			return nil
		}
	}
	if vm.currentClass != "" {
		if field, owner, ok := vm.lookupStaticField(vm.currentClass, name); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
				return err
			}
			if err := vm.ensureClassInitialized(owner); err != nil {
				return err
			}
			field, _, _ = vm.lookupStaticField(owner, name)
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
	for i, part := range parts[:len(parts)-1] {
		if current.Kind == ValueNull {
			return newNullDereferenceError("while assigning " + strings.Join(parts[:i+1], "."))
		}
		if current.Kind != ValueObject {
			return fmt.Errorf("cannot assign field %s on %s", part, current.Kind)
		}
		if field, owner, ok := vm.lookupField(current.Type, part); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+part, field.Modifiers); err != nil {
				return err
			}
			if field.Getter != nil {
				next, err := vm.callGetter(owner, field, current)
				if err != nil {
					return err
				}
				if next.Kind != ValueObject {
					return fmt.Errorf("unknown field %q on %s", part, current.Type)
				}
				current = next
				continue
			}
		}
		next, ok := current.Fields[vm.resolveSObjectFieldName(current.Type, part)]
		if !ok {
			_, next, ok = objectFieldValue(current, part)
		}
		if !ok || next.Kind != ValueObject {
			return fmt.Errorf("unknown field %q on %s", part, current.Type)
		}
		current = next
	}
	fieldName := parts[len(parts)-1]
	if current.Kind == ValueNull {
		return newNullDereferenceError("while assigning " + strings.Join(parts, "."))
	}
	if current.Kind != ValueObject {
		return fmt.Errorf("cannot assign field %s on %s", fieldName, current.Kind)
	}
	if reason, ok := sobjectReadOnlyReason(current); ok {
		return fmt.Errorf("cannot modify read-only %s", reason)
	}
	if def, owner, ok := vm.lookupField(current.Type, fieldName); ok {
		actualName := def.Name
		if actualName == "" {
			actualName = fieldName
		}
		if err := vm.checkMemberAccess(owner, def.Access, owner+"."+actualName, def.Modifiers); err != nil {
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
		current.Fields[actualName] = value
		return nil
	}
	current.Fields[vm.resolveSObjectFieldName(current.Type, fieldName)] = value
	return nil
}

func (vm *VM) lookupField(typeName, fieldName string) (Field, string, bool) {
	for typeName != "" {
		class, ok := vm.lookupClass(typeName)
		if !ok {
			return Field{}, "", false
		}
		if field, ok := class.Fields[fieldName]; ok {
			if field.Name == "" {
				field.Name = fieldName
			}
			return field, class.Name, true
		}
		normalized := strings.ToLower(fieldName)
		for candidate, field := range class.Fields {
			if strings.ToLower(candidate) == normalized || (field.Name != "" && strings.ToLower(field.Name) == normalized) {
				if field.Name == "" {
					field.Name = candidate
				}
				return field, class.Name, true
			}
		}
		typeName = class.SuperClass
	}
	return Field{}, "", false
}

func (vm *VM) lookupReceiverField(typeName, fieldName string) (Field, string, bool) {
	if vm.currentClass != "" && (strings.EqualFold(typeName, vm.currentClass) || vm.isSubclass(typeName, vm.currentClass)) {
		if class, ok := vm.Classes[vm.currentClass]; ok {
			if field, ok := class.Fields[fieldName]; ok {
				return field, class.Name, true
			}
		}
	}
	return vm.lookupField(typeName, fieldName)
}

func (vm *VM) lookupStaticField(typeName, fieldName string) (Field, string, bool) {
	for search := typeName; search != ""; {
		for current := search; current != ""; {
			class, ok := vm.lookupClass(current)
			if !ok {
				break
			}
			if field, ok := class.StaticFields[fieldName]; ok {
				return field, class.Name, true
			}
			normalized := strings.ToLower(fieldName)
			for candidate, field := range class.StaticFields {
				if strings.ToLower(candidate) == normalized || (field.Name != "" && strings.ToLower(field.Name) == normalized) {
					if field.Name == "" {
						field.Name = candidate
					}
					return field, class.Name, true
				}
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
	if value, ok := builtinEnumStaticValue(typeName, fieldName); ok {
		return value, true
	}
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	switch {
	case strings.EqualFold(typeName, "Math"):
		switch {
		case strings.EqualFold(fieldName, "E"):
			return Decimal(math.E), true
		case strings.EqualFold(fieldName, "PI"):
			return Decimal(math.Pi), true
		}
	}
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
	case "Dom.XmlNodeType":
		return domXmlNodeTypeValue(fieldName)
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
		if vm.currentClass == ownerClass || strings.HasPrefix(vm.currentClass, ownerClass+".") || strings.HasPrefix(ownerClass, vm.currentClass+".") {
			return nil
		}
		methodOwner := vm.currentMethod.ClassName
		if methodOwner == "" {
			methodOwner = classNameFromMethod(vm.currentMethod.Name)
		}
		if methodOwner == ownerClass || strings.HasPrefix(methodOwner, ownerClass+".") || strings.HasPrefix(ownerClass, methodOwner+".") {
			return nil
		}
	case "protected":
		if vm.currentClassIsTest() && hasAnyMethodModifier(modifierSets, "testvisible") {
			return nil
		}
		if vm.currentClass == ownerClass || strings.HasPrefix(vm.currentClass, ownerClass+".") || strings.HasPrefix(ownerClass, vm.currentClass+".") || vm.isSubclass(vm.currentClass, ownerClass) || vm.isSubclass(ownerClass, vm.currentClass) {
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
	case "Object", "String", "Boolean", "Integer", "Long", "Decimal", "Double", "Date", "Datetime", "Time", "TimeZone", "Blob", "Id", "Type", "URL", "JSONGenerator", "JSONParser", "JSONToken", "ChildRelationship", "DescribeFieldResult", "DescribeSObjectResult", "DescribeTabResult", "DescribeTabSetResult", "PicklistEntry", "RecordTypeInfo", "XmlStreamReader", "XmlStreamWriter", "PageReference", "SelectOption", "LoggingLevel", "ApexPages.Severity", "ApexPages.StandardController", "ApexPages.StandardSetController", "RestContext", "RestRequest", "RestResponse", "Callable", "StubProvider", "InstallContext", "InstallHandler", "Auth.JWT", "ConnectApi.UserSettings", "ConnectApi.TimeZone", "Metadata.Metadata", "Metadata.MetadataType", "Metadata.DeployContainer", "Metadata.CustomMetadata", "Metadata.CustomMetadataValue", "Metadata.CustomObject", "Metadata.CustomField", "Metadata.DeployCallback", "Metadata.DeployCallBack", "Metadata.DeployResult", "Metadata.DeployStatus", "Metadata.DeployDetails", "Metadata.DeployMessage", "Metadata.DeployCallbackContext", "Metadata.AsyncResult":
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
	if strings.HasSuffix(name, "[]") {
		return isTypeNameToken(strings.TrimSpace(strings.TrimSuffix(name, "[]")))
	}
	return isBuiltinTypeName(name) || isGenericTypeName(name) || isCommonSObjectTypeName(name)
}

func isCommonSObjectTypeName(name string) bool {
	if storage.IsKnownStandardObject(name) {
		return true
	}
	for _, objectName := range standardSObjectPrefixes {
		if strings.EqualFold(name, objectName) {
			return true
		}
	}
	return false
}

func (vm *VM) resolveClassName(typeName string) (string, bool) {
	if class, ok := vm.lookupClass(typeName); ok {
		return class.Name, true
	}
	if !strings.Contains(typeName, ".") && vm.currentClass != "" {
		for owner := vm.currentClass; owner != ""; {
			candidate := owner + "." + typeName
			if class, ok := vm.lookupClass(candidate); ok {
				return class.Name, true
			}
			dot := strings.LastIndex(owner, ".")
			if dot < 0 {
				break
			}
			owner = owner[:dot]
		}
	}
	return "", false
}

func (vm *VM) lookupClass(typeName string) (Class, bool) {
	if class, ok := vm.Classes[typeName]; ok {
		return class, true
	}
	if vm.classLookup == nil {
		vm.rebuildClassLookup()
	}
	if class, ok := vm.classLookup[canonicalClassLookupKey(typeName)]; ok {
		return class, true
	}
	return Class{}, false
}

func (vm *VM) storeClassAliases(class Class) {
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	if vm.classLookup == nil {
		vm.classLookup = make(map[string]Class)
	}
	vm.Classes[class.Name] = class
	vm.enumLookup = nil
	vm.storeClassLookupAlias(class.Name, class)
	if class.Namespace != "" && !strings.Contains(class.Name, ".") {
		qualified := class.Namespace + "." + class.Name
		vm.Classes[qualified] = class
		vm.storeClassLookupAlias(qualified, class)
	}
	if class.Namespace != "" {
		vm.storeClassLookupAlias(class.Namespace+"."+class.Name, class)
	}
}

func (vm *VM) storeClassLookupAlias(name string, class Class) {
	if strings.TrimSpace(name) == "" {
		return
	}
	vm.classLookup[canonicalClassLookupKey(name)] = class
}

func (vm *VM) rebuildClassLookup() {
	vm.classLookup = make(map[string]Class, len(vm.Classes)*2)
	for alias, class := range vm.Classes {
		vm.storeClassLookupAlias(alias, class)
		vm.storeClassLookupAlias(class.Name, class)
		if class.Namespace != "" {
			vm.storeClassLookupAlias(class.Namespace+"."+class.Name, class)
		}
	}
}

func canonicalClassLookupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
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
	page.Fields["parameters"] = pageReferenceParameters(rawURL)
	page.Fields["headers"] = typedMap("Map<String,String>")
	return page
}

func pageReferenceParameters(rawURL string) Value {
	params := typedMap("Map<String,String>")
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return params
	}
	for key, values := range parsed.Query() {
		if key == "" || len(values) == 0 {
			continue
		}
		params.Map[mapKey(String(key))] = String(values[len(values)-1])
	}
	return params
}

func newDomDocument() Value {
	doc := Object("Dom.Document")
	doc.Fields["root"] = Null
	return doc
}

func domXmlNodeTypeValue(name string) (Value, bool) {
	switch strings.ToUpper(name) {
	case "ELEMENT", "TEXT", "COMMENT":
		return Value{Kind: ValueObject, Type: "Dom.XmlNodeType", Text: strings.ToUpper(name)}, true
	default:
		return Null, false
	}
}

func newDomXmlNode(nodeType, name, namespace, text string) Value {
	node := Object("Dom.XmlNode")
	node.Fields["nodeType"] = Value{Kind: ValueObject, Type: "Dom.XmlNodeType", Text: nodeType}
	node.Fields["name"] = String(name)
	node.Fields["namespace"] = domNullableString(namespace)
	node.Fields["text"] = String(text)
	node.Fields["children"] = typedList("List<Dom.XmlNode>")
	node.Fields["attributes"] = typedList("List<Dom.XmlAttribute>")
	node.Fields["namespaces"] = typedMap("Map<String,String>")
	node.Fields["parent"] = Null
	return node
}

func domNullableString(value string) Value {
	if value == "" {
		return Null
	}
	return String(value)
}

func domString(value Value) string {
	if value.Kind == ValueString {
		return value.Text
	}
	return ""
}

func domNodeType(node Value) string {
	if value, ok := node.Fields["nodeType"]; ok && value.Kind == ValueObject {
		return value.Text
	}
	return ""
}

func domNodeList(node Value, field string) Value {
	if value, ok := node.Fields[field]; ok && value.Kind == ValueList {
		return value
	}
	return typedList("List<Dom.XmlNode>")
}

func domSetParent(child, parent Value) Value {
	if child.Kind == ValueObject && child.Type == "Dom.XmlNode" {
		child.Fields["parent"] = parent
	}
	return child
}

func domAppendChild(parent, child Value) Value {
	children := domNodeList(parent, "children")
	child = domSetParent(child, parent)
	children.List = append(children.List, child)
	parent.Fields["children"] = children
	return child
}

func domAttribute(key, value, keyNamespace, valueNamespace string) Value {
	attr := Object("Dom.XmlAttribute")
	attr.Fields["key"] = String(key)
	attr.Fields["value"] = String(value)
	attr.Fields["keyNamespace"] = domNullableString(keyNamespace)
	attr.Fields["valueNamespace"] = domNullableString(valueNamespace)
	return attr
}

func domDocumentXMLString(doc Value) string {
	root, ok := doc.Fields["root"]
	if !ok || root.Kind != ValueObject {
		return `<?xml version="1.0" encoding="UTF-8"?>`
	}
	return `<?xml version="1.0" encoding="UTF-8"?>` + domNodeXMLString(root)
}

func domNodeXMLString(node Value) string {
	switch domNodeType(node) {
	case "TEXT":
		return escapeXMLText(domString(node.Fields["text"]))
	case "COMMENT":
		return "<!--" + strings.ReplaceAll(domString(node.Fields["text"]), "--", "- -") + "-->"
	case "ELEMENT":
		name := domString(node.Fields["name"])
		if name == "" {
			return ""
		}
		var out strings.Builder
		out.WriteByte('<')
		out.WriteString(name)
		for _, attr := range domNodeList(node, "attributes").List {
			key := domString(attr.Fields["key"])
			if key == "" {
				continue
			}
			out.WriteByte(' ')
			out.WriteString(key)
			out.WriteString(`="`)
			out.WriteString(escapeXMLAttr(domString(attr.Fields["value"])))
			out.WriteByte('"')
		}
		children := domNodeList(node, "children").List
		if len(children) == 0 {
			out.WriteString(" />")
			return out.String()
		}
		out.WriteByte('>')
		for _, child := range children {
			out.WriteString(domNodeXMLString(child))
		}
		out.WriteString("</")
		out.WriteString(name)
		out.WriteByte('>')
		return out.String()
	default:
		return ""
	}
}

func escapeXMLText(text string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(text))
	return out.String()
}

func escapeXMLAttr(text string) string {
	escaped := escapeXMLText(text)
	escaped = strings.ReplaceAll(escaped, `"`, "&#34;")
	escaped = strings.ReplaceAll(escaped, "'", "&#39;")
	return escaped
}

func parseDomDocument(source string) (Value, error) {
	source = normalizeHTMLVoidElementsForDOM(source)
	decoder := xml.NewDecoder(strings.NewReader(source))
	var stack []Value
	var root Value
	prefixes := map[string]string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Null, fmt.Errorf("Dom.Document.load invalid XML: %w", err)
		}
		switch typed := token.(type) {
		case xml.StartElement:
			node := newDomXmlNode("ELEMENT", typed.Name.Local, typed.Name.Space, "")
			attrs := typedList("List<Dom.XmlAttribute>")
			namespaces := typedMap("Map<String,String>")
			for _, attr := range typed.Attr {
				if attr.Name.Local == "xmlns" && attr.Name.Space == "" {
					prefixes[""] = attr.Value
					namespaces.Map[mapKey(String(""))] = String(attr.Value)
					continue
				}
				if attr.Name.Space == "xmlns" {
					prefixes[attr.Name.Local] = attr.Value
					namespaces.Map[mapKey(String(attr.Name.Local))] = String(attr.Value)
					continue
				}
				attrs.List = append(attrs.List, domAttribute(attr.Name.Local, attr.Value, attr.Name.Space, ""))
			}
			for prefix, uri := range prefixes {
				if _, ok := namespaces.Map[mapKey(String(prefix))]; !ok {
					namespaces.Map[mapKey(String(prefix))] = String(uri)
				}
			}
			node.Fields["attributes"] = attrs
			node.Fields["namespaces"] = namespaces
			if len(stack) == 0 {
				root = node
			} else {
				parent := stack[len(stack)-1]
				domAppendChild(parent, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			text := string([]byte(typed))
			if len(stack) == 0 || text == "" {
				continue
			}
			textNode := newDomXmlNode("TEXT", "", "", text)
			parent := stack[len(stack)-1]
			domAppendChild(parent, textNode)
		case xml.Comment:
			if len(stack) == 0 {
				continue
			}
			commentNode := newDomXmlNode("COMMENT", "", "", string([]byte(typed)))
			parent := stack[len(stack)-1]
			domAppendChild(parent, commentNode)
		}
	}
	if root.Kind == "" {
		return Null, fmt.Errorf("Dom.Document.load expected root element")
	}
	doc := newDomDocument()
	doc.Fields["root"] = root
	return doc, nil
}

var htmlVoidElementPattern = regexp.MustCompile(`(?i)<(area|base|br|col|embed|hr|img|input|link|meta|param|source|track|wbr)([^<>]*?)>`)

func normalizeHTMLVoidElementsForDOM(source string) string {
	return htmlVoidElementPattern.ReplaceAllStringFunc(source, func(tag string) string {
		trimmed := strings.TrimSpace(tag)
		if strings.HasSuffix(trimmed, "/>") {
			return tag
		}
		return strings.TrimSuffix(tag, ">") + "/>"
	})
}

func callDomDocumentMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "load":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.load expects String")
		}
		doc, err := parseDomDocument(args[0].Text)
		if err != nil {
			return Null, receiver, false, true, newExceptionError("XmlException", err.Error())
		}
		return Null, doc, true, true, nil
	case "getRootElement":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.getRootElement expects 0 arguments")
		}
		if root, ok := receiver.Fields["root"]; ok {
			return root, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "toXmlString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.Document.toXmlString expects 0 arguments")
		}
		return String(domDocumentXMLString(receiver)), receiver, false, true, nil
	}
	return Null, receiver, false, false, nil
}

func callDomXmlNodeMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "toXmlString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.toXmlString expects 0 arguments")
		}
		return String(domNodeXMLString(receiver)), receiver, false, true, nil
	case "getNodeType":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getNodeType expects 0 arguments")
		}
		return receiver.Fields["nodeType"], receiver, false, true, nil
	case "getName":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getName expects 0 arguments")
		}
		return receiver.Fields["name"], receiver, false, true, nil
	case "getNamespace":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getNamespace expects 0 arguments")
		}
		return receiver.Fields["namespace"], receiver, false, true, nil
	case "getText":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getText expects 0 arguments")
		}
		if domNodeType(receiver) != "ELEMENT" {
			return receiver.Fields["text"], receiver, false, true, nil
		}
		var text strings.Builder
		for _, child := range domNodeList(receiver, "children").List {
			if domNodeType(child) == "TEXT" || domNodeType(child) == "COMMENT" {
				text.WriteString(domString(child.Fields["text"]))
			}
		}
		return String(text.String()), receiver, false, true, nil
	case "getChildren":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getChildren expects 0 arguments")
		}
		return domNodeList(receiver, "children"), receiver, false, true, nil
	case "getParent":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getParent expects 0 arguments")
		}
		return receiver.Fields["parent"], receiver, false, true, nil
	case "getAttributeCount":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getAttributeCount expects 0 arguments")
		}
		return Int(int64(len(domNodeList(receiver, "attributes").List))), receiver, false, true, nil
	case "getAttributeKeyAt", "getAttributeKeyNsAt":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s expects Integer", method)
		}
		attrs := domNodeList(receiver, "attributes").List
		index := int(args[0].Int)
		if index < 0 || index >= len(attrs) {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s index out of bounds: %d", method, index)
		}
		field := "key"
		if method == "getAttributeKeyNsAt" {
			field = "keyNamespace"
		}
		return attrs[index].Fields[field], receiver, false, true, nil
	case "getAttributeValue", "getAttributeValueNs":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s expects key and namespace", method)
		}
		key := args[0].Text
		namespace := ""
		if args[1].Kind == ValueString {
			namespace = args[1].Text
		}
		for _, attr := range domNodeList(receiver, "attributes").List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == namespace {
				if method == "getAttributeValueNs" {
					return attr.Fields["valueNamespace"], receiver, false, true, nil
				}
				return attr.Fields["value"], receiver, false, true, nil
			}
		}
		return Null, receiver, false, true, nil
	case "getPrefixFor":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.getPrefixFor expects namespace String")
		}
		namespaces := receiver.Fields["namespaces"]
		if namespaces.Kind == ValueMap {
			for rawKey, value := range namespaces.Map {
				if value.Kind == ValueString && value.Text == args[0].Text {
					return valueFromMapKey(rawKey), receiver, false, true, nil
				}
			}
		}
		return Null, receiver, false, true, nil
	case "setNamespace":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.setNamespace expects prefix and namespace Strings")
		}
		namespaces := receiver.Fields["namespaces"]
		if namespaces.Kind != ValueMap {
			namespaces = typedMap("Map<String,String>")
		}
		namespaces.Map[mapKey(args[0])] = args[1]
		receiver.Fields["namespaces"] = namespaces
		return Null, receiver, true, true, nil
	case "setAttributeNs":
		if len(args) != 4 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.setAttributeNs expects key, value, key namespace, and value namespace")
		}
		key := args[0].Text
		keyNamespace := domString(args[2])
		attrs := domNodeList(receiver, "attributes")
		for i, attr := range attrs.List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == keyNamespace {
				attr.Fields["value"] = args[1]
				attr.Fields["valueNamespace"] = args[3]
				attrs.List[i] = attr
				receiver.Fields["attributes"] = attrs
				return Null, receiver, true, true, nil
			}
		}
		attrs.List = append(attrs.List, domAttribute(key, args[1].Text, keyNamespace, domString(args[3])))
		receiver.Fields["attributes"] = attrs
		return Null, receiver, true, true, nil
	case "removeAttribute":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.removeAttribute expects key and namespace")
		}
		key := args[0].Text
		keyNamespace := domString(args[1])
		attrs := domNodeList(receiver, "attributes")
		filtered := attrs.List[:0]
		for _, attr := range attrs.List {
			if domString(attr.Fields["key"]) == key && domString(attr.Fields["keyNamespace"]) == keyNamespace {
				continue
			}
			filtered = append(filtered, attr)
		}
		attrs.List = filtered
		receiver.Fields["attributes"] = attrs
		return Null, receiver, true, true, nil
	case "addTextNode", "addCommentNode":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.%s expects String", method)
		}
		nodeType := "TEXT"
		if method == "addCommentNode" {
			nodeType = "COMMENT"
		}
		child := newDomXmlNode(nodeType, "", "", args[0].Text)
		child = domAppendChild(receiver, child)
		return child, receiver, true, true, nil
	case "addChildElement":
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.addChildElement expects name, namespace, prefix")
		}
		namespace := domString(args[1])
		child := newDomXmlNode("ELEMENT", args[0].Text, namespace, "")
		if args[2].Kind == ValueString && namespace != "" {
			namespaces := typedMap("Map<String,String>")
			namespaces.Map[mapKey(args[2])] = String(namespace)
			child.Fields["namespaces"] = namespaces
		}
		child = domAppendChild(receiver, child)
		return child, receiver, true, true, nil
	case "removeChild":
		if len(args) != 1 || args[0].Kind != ValueObject {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.removeChild expects XmlNode")
		}
		children := domNodeList(receiver, "children")
		filtered := children.List[:0]
		for _, child := range children.List {
			if child.Equal(args[0]) {
				continue
			}
			filtered = append(filtered, child)
		}
		children.List = filtered
		receiver.Fields["children"] = children
		return Null, receiver, true, true, nil
	case "insertBefore":
		if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueObject {
			return Null, receiver, false, true, fmt.Errorf("Dom.XmlNode.insertBefore expects new child and reference child")
		}
		children := domNodeList(receiver, "children")
		newChild := domSetParent(args[0], receiver)
		inserted := false
		out := make([]Value, 0, len(children.List)+1)
		for _, child := range children.List {
			if !inserted && child.Equal(args[1]) {
				out = append(out, newChild)
				inserted = true
			}
			out = append(out, child)
		}
		if !inserted {
			out = append(out, newChild)
		}
		children.List = out
		receiver.Fields["children"] = children
		return Null, receiver, true, true, nil
	}
	return Null, receiver, false, false, nil
}

func (vm *VM) newPageReference(rawURL string) Value {
	return newPageReference(vm.normalizePageReferenceURL(rawURL))
}

func (vm *VM) normalizePageReferenceURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(strings.ToLower(rawURL), "page.") {
		return rawURL
	}
	rest := rawURL[len("Page."):]
	pageName := rest
	suffix := ""
	for _, sep := range []string{"?", "#"} {
		if idx := strings.Index(pageName, sep); idx >= 0 {
			suffix = pageName[idx:]
			pageName = pageName[:idx]
			break
		}
	}
	if pageName == "" || vm.pageReferences == nil {
		return rawURL
	}
	registered, ok := vm.pageReferences[strings.ToLower(pageName)]
	if !ok {
		return rawURL
	}
	return "/apex/" + registered + suffix
}

func newAuthVerificationResult(redirect, success, message Value) Value {
	result := Object("Auth.VerificationResult")
	result.Fields["redirect"] = redirect
	result.Fields["success"] = success
	result.Fields["message"] = message
	return result
}

func newSelectOption(value, label Value, disabled, escapeItem Value) Value {
	option := Object("SelectOption")
	option.Fields["value"] = value
	option.Fields["label"] = label
	option.Fields["disabled"] = disabled
	option.Fields["escapeItem"] = escapeItem
	return option
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

func (vm *VM) sendEmail(args []Value, result *Result) (Value, error) {
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
	for _, message := range args[0].List {
		vm.capturedEmails = append(vm.capturedEmails, vm.captureEmail(message))
		results = append(results, newSendEmailResult())
	}
	return List(results...), nil
}

func (vm *VM) captureEmail(message Value) CapturedEmail {
	captured := CapturedEmail{Kind: message.Type}
	switch message.Type {
	case "Messaging.SingleEmailMessage":
		captured.ToAddresses = stringsFromList(message.Fields["toAddresses"])
		captured.CcAddresses = stringsFromList(message.Fields["ccAddresses"])
		captured.BccAddresses = stringsFromList(message.Fields["bccAddresses"])
		captured.FileAttachments = stringsFromList(message.Fields["fileAttachments"])
		captured.EntityAttachments = stringsFromList(message.Fields["entityAttachments"])
		captured.DocumentAttachments = stringsFromList(message.Fields["documentAttachments"])
		captured.TargetObjectIDs = stringsFromList(message.Fields["targetObjectIds"])
		captured.Subject = stringValue(message.Fields["subject"])
		captured.PlainTextBody = stringValue(message.Fields["plainTextBody"])
		captured.HTMLBody = stringValue(message.Fields["htmlBody"])
		captured.TemplateID = stringValue(message.Fields["templateId"])
		captured.TargetObjectID = stringValue(message.Fields["targetObjectId"])
		captured.WhatID = stringValue(message.Fields["whatId"])
		captured.SaveAsActivity = boolValue(message.Fields["saveAsActivity"])
		vm.renderCapturedEmailTemplate(&captured)
	case "Messaging.MassEmailMessage":
		captured.TargetObjectIDs = stringsFromList(message.Fields["targetObjectIds"])
		captured.WhatIDs = stringsFromList(message.Fields["whatIds"])
		captured.TemplateID = stringValue(message.Fields["templateId"])
		captured.SaveAsActivity = boolValue(message.Fields["saveAsActivity"])
		if captured.TemplateID != "" && len(captured.TargetObjectIDs) > 0 {
			captured.TargetObjectID = captured.TargetObjectIDs[0]
			if len(captured.WhatIDs) > 0 {
				captured.WhatID = captured.WhatIDs[0]
			}
			vm.renderCapturedEmailTemplate(&captured)
		}
	}
	return captured
}

func (vm *VM) captureWorkflowEmail(alert storage.WorkflowEmailAlert, record storage.Record, result *Result) error {
	if err := vm.incrementLimit("emailInvocations", 1); err != nil {
		return err
	}
	captured := CapturedEmail{
		Kind:   "WorkflowEmailAlert",
		WhatID: string(record.ID),
	}
	captured.ToAddresses, captured.TargetObjectIDs = vm.workflowEmailRecipients(alert, record)
	if len(captured.TargetObjectIDs) > 0 {
		captured.TargetObjectID = captured.TargetObjectIDs[0]
	}
	if template, ok := vm.emailTemplateByName(alert.Template); ok {
		captured.TemplateID = string(template.ID)
		whoID := Null
		if len(captured.TargetObjectIDs) > 0 {
			whoID = String(captured.TargetObjectIDs[0])
		}
		whatID := Null
		if captured.WhatID != "" {
			whatID = String(captured.WhatID)
		}
		captured.Subject = vm.renderEmailTemplateText(storageStringField(template, "Subject"), whoID, whatID)
		captured.HTMLBody = vm.renderEmailTemplateText(storageStringField(template, "HtmlValue"), whoID, whatID)
		captured.PlainTextBody = vm.renderEmailTemplateText(storageStringField(template, "Body"), whoID, whatID)
	}
	vm.capturedEmails = append(vm.capturedEmails, captured)
	appendTrace(result, "apex.email.workflow", "apex.email", map[string]any{
		"alert":      alert.Name,
		"template":   alert.Template,
		"recipients": len(captured.ToAddresses),
		"record":     string(record.ID),
	})
	return nil
}

func (vm *VM) workflowEmailRecipients(alert storage.WorkflowEmailAlert, record storage.Record) ([]string, []string) {
	addresses := make([]string, 0, len(alert.Recipients))
	targetIDs := make([]string, 0, len(alert.Recipients))
	for _, recipient := range alert.Recipients {
		if recipient.Recipient != "" {
			vm.appendWorkflowEmailRecipient(recipient.Type, recipient.Recipient, &addresses, &targetIDs)
			continue
		}
		fieldName := recipient.Field
		if fieldName == "" && strings.EqualFold(strings.TrimSpace(recipient.Type), "owner") {
			fieldName = "OwnerId"
		}
		if fieldName == "" {
			continue
		}
		if vm.Org != nil {
			if objectName, ok := storage.ResolveObjectName(*vm.Org, record.Object); ok {
				if object, ok := vm.Org.Objects[objectName]; ok {
					if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, fieldName); ok {
						fieldName = resolved
					}
				}
			}
		}
		if value, ok := record.Fields[fieldName]; ok {
			vm.appendWorkflowEmailRecipient(recipient.Type, workflowEmailRecipientValue(value), &addresses, &targetIDs)
			continue
		}
		if strings.EqualFold(fieldName, "OwnerId") && record.System.OwnerID != "" {
			vm.appendWorkflowEmailRecipient(recipient.Type, string(record.System.OwnerID), &addresses, &targetIDs)
		}
	}
	return addresses, targetIDs
}

func (vm *VM) appendWorkflowEmailRecipient(recipientType, raw string, addresses, targetIDs *[]string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return
	}
	normalizedType := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(recipientType), " ", ""))
	if workflowRecipientLooksLikeID(value) || (normalizedType == "owner" && !strings.Contains(value, "@")) {
		*targetIDs = append(*targetIDs, value)
		return
	}
	*addresses = append(*addresses, value)
}

func workflowEmailRecipientValue(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return value.String
	default:
		return ""
	}
}

func workflowRecipientLooksLikeID(value string) bool {
	if len(value) != 15 && len(value) != 18 {
		return false
	}
	for _, ch := range value {
		if ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' {
			continue
		}
		return false
	}
	return true
}

func (vm *VM) renderCapturedEmailTemplate(captured *CapturedEmail) {
	if captured == nil || captured.TemplateID == "" || vm.Org == nil {
		return
	}
	template, ok := vm.emailTemplateByID(captured.TemplateID)
	if !ok {
		return
	}
	whoID := Null
	if captured.TargetObjectID != "" {
		whoID = String(captured.TargetObjectID)
	}
	whatID := Null
	if captured.WhatID != "" {
		whatID = String(captured.WhatID)
	}
	if captured.Subject == "" {
		captured.Subject = vm.renderEmailTemplateText(storageStringField(template, "Subject"), whoID, whatID)
	}
	if captured.HTMLBody == "" {
		captured.HTMLBody = vm.renderEmailTemplateText(storageStringField(template, "HtmlValue"), whoID, whatID)
	}
	if captured.PlainTextBody == "" {
		captured.PlainTextBody = vm.renderEmailTemplateText(storageStringField(template, "Body"), whoID, whatID)
	}
}

func stringsFromList(value Value) []string {
	if value.Kind != ValueList {
		return nil
	}
	out := make([]string, 0, len(value.List))
	for _, item := range value.List {
		if item.Kind == ValueString {
			out = append(out, item.Text)
		}
	}
	return out
}

func stringValue(value Value) string {
	if value.Kind == ValueString {
		return value.Text
	}
	return ""
}

func boolValue(value Value) bool {
	return value.Kind == ValueBool && value.Bool
}

func (vm *VM) renderStoredEmailTemplate(args []Value) (Value, error) {
	if len(args) != 3 {
		return Null, fmt.Errorf("Messaging.renderStoredEmailTemplate expects templateId, whoId, whatId")
	}
	for i, arg := range args {
		if arg.Kind == ValueNull {
			continue
		}
		if _, ok := idValueText(arg); !ok {
			names := []string{"templateId", "whoId", "whatId"}
			return Null, fmt.Errorf("Messaging.renderStoredEmailTemplate expects %s String or Id", names[i])
		}
	}
	templateID, _ := idValueText(args[0])
	if templateID == "" {
		return Null, fmt.Errorf("Email template not found: %s", templateID)
	}
	template, ok := vm.emailTemplateByID(templateID)
	if !ok {
		return Null, fmt.Errorf("Email template not found: %s", templateID)
	}

	message := newSingleEmailMessage()
	message.Fields["templateId"] = String(templateID)
	message.Fields["targetObjectId"] = args[1]
	message.Fields["whatId"] = args[2]
	message.Fields["subject"] = String(vm.renderEmailTemplateText(storageStringField(template, "Subject"), args[1], args[2]))
	message.Fields["htmlBody"] = String(vm.renderEmailTemplateText(storageStringField(template, "HtmlValue"), args[1], args[2]))
	message.Fields["plainTextBody"] = String(vm.renderEmailTemplateText(storageStringField(template, "Body"), args[1], args[2]))
	return message, nil
}

func (vm *VM) emailTemplateByID(templateID string) (storage.Record, bool) {
	if vm.Org == nil {
		return storage.Record{}, false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, "EmailTemplate")
	if !ok {
		objectName = "EmailTemplate"
	}
	object := vm.Org.Objects[objectName]
	if record, ok := object.Records[storage.ID(templateID)]; ok {
		return record, true
	}
	for _, record := range object.Records {
		if string(record.ID) == templateID {
			return record, true
		}
		if id, ok := record.Fields["Id"]; ok && string(storageIDFromValue(id)) == templateID {
			return record, true
		}
	}
	return storage.Record{}, false
}

func (vm *VM) emailTemplateByName(name string) (storage.Record, bool) {
	if vm.Org == nil {
		return storage.Record{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return storage.Record{}, false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, "EmailTemplate")
	if !ok {
		objectName = "EmailTemplate"
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return storage.Record{}, false
	}
	for _, record := range object.Records {
		for _, field := range []string{"DeveloperName", "Name"} {
			if strings.EqualFold(storageStringField(record, field), name) {
				return record, true
			}
		}
	}
	return storage.Record{}, false
}

func (vm *VM) renderEmailTemplateText(text string, whoID, whatID Value) string {
	if text == "" || !strings.Contains(text, "{!") {
		return text
	}
	whoRecord, whoOK := vm.recordByIDValue(whoID)
	whatRecord, whatOK := vm.recordByIDValue(whatID)
	var out strings.Builder
	for {
		start := strings.Index(text, "{!")
		if start < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:start])
		text = text[start+2:]
		end := strings.Index(text, "}")
		if end < 0 {
			out.WriteString("{!")
			out.WriteString(text)
			return out.String()
		}
		token := strings.TrimSpace(text[:end])
		if value, ok := vm.emailMergeTokenValue(token, whoRecord, whoOK, whatRecord, whatOK); ok {
			out.WriteString(value)
		} else {
			out.WriteString("{!")
			out.WriteString(text[:end])
			out.WriteString("}")
		}
		text = text[end+1:]
	}
}

func (vm *VM) emailMergeTokenValue(token string, whoRecord storage.Record, whoOK bool, whatRecord storage.Record, whatOK bool) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", false
	}
	root := strings.TrimSpace(parts[0])
	field := strings.TrimSpace(strings.Join(parts[1:], "."))
	if root == "" || field == "" {
		return "", false
	}
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if whoOK && emailMergeRootMatches(root, whoRecord.Object, namespace, "Recipient", "Who", "TargetObject") {
		return vm.storageRecordStringField(whoRecord, field), true
	}
	if whatOK && emailMergeRootMatches(root, whatRecord.Object, namespace, "RelatedTo", "What") {
		return vm.storageRecordStringField(whatRecord, field), true
	}
	return "", false
}

func emailMergeRootMatches(root, objectName, namespace string, aliases ...string) bool {
	for _, alias := range aliases {
		if strings.EqualFold(root, alias) {
			return true
		}
	}
	if strings.EqualFold(root, objectName) {
		return true
	}
	return strings.EqualFold(root, storage.StripNamespaceToken(namespace, objectName))
}

func (vm *VM) storageRecordStringField(record storage.Record, field string) string {
	if strings.EqualFold(field, "Id") {
		return string(record.ID)
	}
	if vm.Org != nil {
		if objectName, ok := storage.ResolveObjectName(*vm.Org, record.Object); ok {
			if object, ok := vm.Org.Objects[objectName]; ok {
				if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
					field = resolved
				}
			}
		}
	}
	return storageStringField(record, field)
}

func (vm *VM) recordByIDValue(value Value) (storage.Record, bool) {
	if value.Kind != ValueString || value.Text == "" || vm.Org == nil {
		return storage.Record{}, false
	}
	id := storage.ID(value.Text)
	if len(value.Text) >= 3 {
		if objectName, ok := vm.sObjectNameForIDPrefix(value.Text[:3]); ok {
			if object, ok := vm.Org.Objects[objectName]; ok {
				if record, ok := object.Records[id]; ok {
					return record, true
				}
			}
		}
	}
	for _, object := range vm.Org.Objects {
		if record, ok := object.Records[id]; ok {
			return record, true
		}
		for _, record := range object.Records {
			if record.ID == id {
				return record, true
			}
			if fieldID, ok := record.Fields["Id"]; ok && storageIDFromValue(fieldID) == id {
				return record, true
			}
		}
	}
	return storage.Record{}, false
}

func storageStringField(record storage.Record, field string) string {
	value, ok := record.Fields[field]
	if !ok {
		return ""
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueInteger:
		return strconv.FormatInt(value.Integer, 10)
	case storage.ValueBoolean:
		if value.Boolean {
			return "true"
		}
		return "false"
	default:
		return ""
	}
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

func typedList(typeName string) Value {
	value := List()
	value.Type = typeName
	return value
}

func canonicalRuntimeTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	for _, known := range []string{
		"HttpRequest", "HttpResponse", "StaticResourceCalloutMock", "MultiStaticResourceCalloutMock",
		"RestRequest", "RestResponse", "Continuation", "PageReference", "VisualEditor.DataRow",
		"VisualEditor.DynamicPickListRows", "Dom.Document", "Auth.UserData", "Auth.VerificationResult",
		"Auth.AuthConfiguration", "Auth.JWT", "Metadata.DeployContainer", "Metadata.CustomMetadata",
		"Metadata.CustomMetadataValue", "Metadata.CustomObject", "Metadata.CustomField", "Metadata.Metadata",
		"Metadata.DeployResult", "Metadata.DeployDetails", "Metadata.DeployMessage", "Metadata.DeployCallbackContext",
		"Metadata.AsyncResult", "SelectOption", "ApexPages.StandardController", "ApexPages.StandardSetController",
		"ApexPages.Message", "Messaging.SendEmailResult", "Messaging.SingleEmailMessage",
		"Messaging.MassEmailMessage", "Messaging.SendEmailOptions", "URL",
	} {
		if strings.EqualFold(typeName, known) {
			return known
		}
	}
	return typeName
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
					return true, newNullDereferenceError("while assigning " + name)
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
	} else {
		typeName = canonicalRuntimeTypeName(typeName)
	}
	switch {
	case collectionBase(typeName) == "List":
		if len(namedArgs) > 0 {
			return Null, fmt.Errorf("List constructor does not accept named fields")
		}
		if len(args) == 1 && args[0].Kind == ValueList {
			if elementType, ok := collectionElementType(typeName); ok && collectionBase(elementType) != "" {
				if element, err := vm.coerceAssignable(elementType, args[0]); err == nil {
					return vm.coerceAssignable(typeName, List(element))
				}
			}
		}
		if len(args) == 1 && (args[0].Kind == ValueList || args[0].Kind == ValueSet) {
			value := List(append([]Value(nil), collectionMembers(args[0])...)...)
			return vm.coerceAssignable(typeName, value)
		}
		return vm.coerceAssignable(typeName, List(args...))
	case collectionBase(typeName) == "Set":
		if len(namedArgs) > 0 {
			return Null, fmt.Errorf("Set constructor does not accept named fields")
		}
		if len(args) == 1 && (args[0].Kind == ValueList || args[0].Kind == ValueSet) {
			value := Set(collectionMembers(args[0])...)
			return vm.coerceAssignable(typeName, value)
		}
		return vm.coerceAssignable(typeName, Set(args...))
	case isMapType(typeName):
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
				encodedKey := mapKey(key)
				value.Map[encodedKey] = coerced
				value.MapKeys[encodedKey] = key
			}
			return value, nil
		}
		if len(args) == 1 && args[0].Kind == ValueMap {
			value := Map()
			value.Type = typeName
			for rawKey, item := range args[0].Map {
				keyValue := mapStoredKey(args[0], rawKey)
				key, coerced, err := vm.coerceMapEntry(typeName, keyValue, item)
				if err != nil {
					return Null, fmt.Errorf("Map constructor: %w", err)
				}
				encodedKey := mapKey(key)
				value.Map[encodedKey] = coerced
				value.MapKeys[encodedKey] = key
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
		if err := vm.ensureClassInitialized(class.Name); err != nil {
			return Null, err
		}
		class, _ = vm.lookupClass(typeName)
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
			if err := vm.callImplicitSuperConstructor(class, ctor, object, result); err != nil {
				return Null, err
			}
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
		} else if err := vm.callImplicitSuperConstructor(class, Method{}, object, result); err != nil {
			return Null, err
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
		return vm.newPageReference(rawURL), nil
	case "VisualEditor.DataRow":
		if len(args) != 2 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("VisualEditor.DataRow constructor expects label and value Strings")
		}
		if args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, fmt.Errorf("VisualEditor.DataRow constructor expects label and value Strings")
		}
		row := Object("VisualEditor.DataRow")
		row.Fields["label"] = args[0]
		row.Fields["value"] = args[1]
		return row, nil
	case "VisualEditor.DynamicPickListRows":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("VisualEditor.DynamicPickListRows constructor expects 0 arguments")
		}
		rows := Object("VisualEditor.DynamicPickListRows")
		rows.Fields["rows"] = typedList("List<VisualEditor.DataRow>")
		return rows, nil
	case "Dom.Document":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Dom.Document constructor expects 0 arguments")
		}
		return newDomDocument(), nil
	case "Auth.UserData":
		if len(args) != 11 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Auth.UserData constructor expects 11 arguments")
		}
		data := Object("Auth.UserData")
		for index, field := range []string{"identifier", "firstName", "lastName", "fullName", "email", "link", "username", "locale", "provider", "siteLoginUrl", "attributeMap"} {
			data.Fields[field] = args[index]
		}
		return data, nil
	case "Auth.VerificationResult":
		if len(args) != 3 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Auth.VerificationResult constructor expects redirect, success, message")
		}
		return newAuthVerificationResult(args[0], args[1], args[2]), nil
	case "Auth.AuthConfiguration":
		if len(args) != 2 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Auth.AuthConfiguration constructor expects community URL and start URL")
		}
		config := Object("Auth.AuthConfiguration")
		config.Fields["communityUrl"] = args[0]
		config.Fields["startUrl"] = args[1]
		authConfig := Object("Auth.AuthConfig")
		authConfig.Fields["Url"] = args[0]
		config.Fields["authConfig"] = authConfig
		return config, nil
	case "Auth.JWT":
		if len(args) != 0 {
			return Null, fmt.Errorf("Auth.JWT constructor expects 0 arguments")
		}
		jwt := Object("Auth.JWT")
		for field, value := range namedArgs {
			jwt.Fields[field] = value
		}
		return jwt, nil
	case "Metadata.DeployContainer":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployContainer constructor expects 0 arguments")
		}
		container := Object("Metadata.DeployContainer")
		container.Fields["metadata"] = List()
		for field, value := range namedArgs {
			container.Fields[field] = value
		}
		return container, nil
	case "Metadata.CustomMetadata":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.CustomMetadata constructor expects 0 arguments")
		}
		metadata := Object("Metadata.CustomMetadata")
		metadata.Fields["values"] = List()
		for field, value := range namedArgs {
			metadata.Fields[field] = value
		}
		return metadata, nil
	case "Metadata.CustomMetadataValue":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.CustomMetadataValue constructor expects 0 arguments")
		}
		value := Object("Metadata.CustomMetadataValue")
		for field, fieldValue := range namedArgs {
			value.Fields[field] = fieldValue
		}
		return value, nil
	case "Metadata.CustomObject":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.CustomObject constructor expects 0 arguments")
		}
		metadata := Object("Metadata.CustomObject")
		for field, value := range namedArgs {
			metadata.Fields[field] = value
		}
		return metadata, nil
	case "Metadata.CustomField":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.CustomField constructor expects 0 arguments")
		}
		field := Object("Metadata.CustomField")
		for name, value := range namedArgs {
			field.Fields[name] = value
		}
		return field, nil
	case "Metadata.Metadata":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.Metadata constructor expects 0 arguments")
		}
		metadata := Object("Metadata.Metadata")
		for field, value := range namedArgs {
			metadata.Fields[field] = value
		}
		return metadata, nil
	case "Metadata.DeployResult":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployResult constructor expects 0 arguments")
		}
		result := Object("Metadata.DeployResult")
		result.Fields["status"] = Value{Kind: ValueObject, Type: "Metadata.DeployStatus", Text: "Succeeded", Fields: map[string]Value{"ordinal": Int(0)}}
		result.Fields["success"] = Bool(true)
		result.Fields["details"] = metadataDeployDetailsObject()
		for field, value := range namedArgs {
			result.Fields[field] = value
		}
		return result, nil
	case "Metadata.DeployDetails":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployDetails constructor expects 0 arguments")
		}
		details := metadataDeployDetailsObject()
		for field, value := range namedArgs {
			details.Fields[field] = value
		}
		return details, nil
	case "Metadata.DeployMessage":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployMessage constructor expects 0 arguments")
		}
		message := Object("Metadata.DeployMessage")
		for field, value := range namedArgs {
			message.Fields[field] = value
		}
		return message, nil
	case "Metadata.DeployCallbackContext":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployCallbackContext constructor expects 0 arguments")
		}
		context := Object("Metadata.DeployCallbackContext")
		for field, value := range namedArgs {
			context.Fields[field] = value
		}
		return context, nil
	case "Metadata.AsyncResult":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.AsyncResult constructor expects 0 arguments")
		}
		result := metadataAsyncResultObject("0Af000000000001", true, "Succeeded", "")
		for field, value := range namedArgs {
			result.Fields[field] = value
		}
		return result, nil
	case "SelectOption":
		if len(args) < 2 || len(args) > 4 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("SelectOption constructor expects value, label[, disabled[, escapeItem]]")
		}
		if args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, fmt.Errorf("SelectOption constructor expects String value and label")
		}
		disabled := Bool(false)
		escapeItem := Bool(true)
		if len(args) >= 3 {
			if args[2].Kind != ValueBool {
				return Null, fmt.Errorf("SelectOption constructor disabled expects Boolean")
			}
			disabled = args[2]
		}
		if len(args) == 4 {
			if args[3].Kind != ValueBool {
				return Null, fmt.Errorf("SelectOption constructor escapeItem expects Boolean")
			}
			escapeItem = args[3]
		}
		return newSelectOption(args[0], args[1], disabled, escapeItem), nil
	case "ApexPages.StandardController":
		if len(args) != 1 || len(namedArgs) != 0 || args[0].Kind != ValueObject {
			return Null, fmt.Errorf("ApexPages.StandardController constructor expects SObject")
		}
		controller := Object("ApexPages.StandardController")
		controller.Fields["record"] = args[0]
		return controller, nil
	case "ApexPages.StandardSetController":
		if len(args) != 1 || len(namedArgs) != 0 || (args[0].Kind != ValueList && !(args[0].Kind == ValueObject && args[0].Type == "Database.QueryLocator")) {
			return Null, fmt.Errorf("ApexPages.StandardSetController constructor expects List or QueryLocator")
		}
		records := args[0]
		if args[0].Kind == ValueObject && args[0].Type == "Database.QueryLocator" {
			if value, ok := args[0].Fields["Records"]; ok {
				records = value
			} else {
				records = List()
			}
		}
		controller := Object("ApexPages.StandardSetController")
		controller.Fields["records"] = records
		controller.Fields["selected"] = List()
		controller.Fields["pageSize"] = Int(20)
		controller.Fields["pageNumber"] = Int(1)
		return controller, nil
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
	class, ok := vm.lookupClass(typeName)
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

func (vm *VM) callImplicitSuperConstructor(class Class, ctor Method, object Value, result *Result) error {
	if class.SuperClass == "" || constructorHasExplicitChain(ctor) {
		return nil
	}
	superClass, ok := vm.lookupClass(class.SuperClass)
	if !ok {
		return nil
	}
	superCtor, found, ambiguous := vm.matchConstructor(superClass, nil)
	if ambiguous {
		return fmt.Errorf("ambiguous %s constructor with 0 argument(s)", superClass.Name)
	}
	if !found {
		return nil
	}
	_, err := vm.callMethodWithReceiver(superCtor, object, nil, result)
	return err
}

func constructorHasExplicitChain(ctor Method) bool {
	if len(ctor.Program.Instructions) == 0 {
		return false
	}
	first := ctor.Program.Instructions[0]
	return first.Op == ir.OpExpr && first.Expr.Kind == ir.ExprCall && (first.Expr.Callee == "this" || first.Expr.Callee == "super")
}

func (vm *VM) callChainedConstructor(callee string, args []Value, result *Result) (Value, error) {
	receiver, ok := vm.Globals["this"]
	if !ok || receiver.Kind != ValueObject {
		return Null, fmt.Errorf("%s constructor call requires instance receiver", callee)
	}
	className := vm.currentMethod.ClassName
	if className == "" {
		className = receiver.Type
	}
	class, ok := vm.lookupClass(className)
	if !ok {
		return Null, fmt.Errorf("%s constructor call requires registered class %q", callee, className)
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
	if vm.activeConstructors[constructorCallKey(target)] > 0 {
		return Null, fmt.Errorf("recursive constructor invocation %s", target.Name)
	}
	_, err := vm.callMethodWithReceiver(target, receiver, args, result)
	return Null, err
}

func constructorCallKey(method Method) string {
	parts := make([]string, 0, len(method.Params)+2)
	parts = append(parts, method.ClassName, method.Name)
	for _, param := range method.Params {
		parts = append(parts, param.Type)
	}
	return strings.Join(parts, "\x00")
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
		class, ok := vm.lookupClass(typeName)
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

func (vm *VM) dispatchAccessMethod(staticType string, target Method, method string, args []Value) Method {
	staticType = vm.resolveTypeNameInClass(vm.currentClass, staticType)
	if staticType == "" {
		return target
	}
	surface, ok, ambiguous := vm.resolveInstanceMethodForArgs(staticType, method, args)
	if !ok || ambiguous {
		return target
	}
	if surface.ClassName == target.ClassName || vm.isSubclass(target.ClassName, surface.ClassName) {
		return surface
	}
	return target
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
		class, ok := vm.lookupClass(typeName)
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
	class, ok := vm.lookupClass(typeName)
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
	class, ok := vm.lookupClass(typeName)
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
		class, ok := vm.lookupClass(typeName)
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
	target, ok, ambiguous := vm.bestMethodBySpecificity(applicable, args)
	if ambiguous {
		vm.lastAmbiguous = &overloadDiagnostic{
			Args:       append([]Value(nil), args...),
			Candidates: append([]Method(nil), applicable...),
		}
	} else {
		vm.lastAmbiguous = nil
	}
	return target, ok, ambiguous
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

func (vm *VM) ambiguousOverloadError(callee string, args []Value) error {
	message := fmt.Sprintf("ambiguous overload for call %q", callee)
	diag := vm.lastAmbiguous
	if diag == nil || len(diag.Candidates) == 0 {
		diag = &overloadDiagnostic{Args: append([]Value(nil), args...)}
	}
	argTypes := runtimeArgTypes(diag.Args)
	if len(argTypes) == 0 && len(args) > 0 {
		argTypes = runtimeArgTypes(args)
	}
	if len(argTypes) > 0 {
		message += "; argument types: " + strings.Join(argTypes, ", ")
	}
	if len(diag.Candidates) > 0 {
		signatures := make([]string, 0, len(diag.Candidates))
		for _, candidate := range diag.Candidates {
			signatures = append(signatures, methodSignature(candidate))
		}
		sort.Strings(signatures)
		message += "; candidates: " + strings.Join(signatures, "; ")
	}
	return fmt.Errorf("%s", message)
}

func runtimeArgTypes(args []Value) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, 0, len(args))
	for i, arg := range args {
		out = append(out, fmt.Sprintf("%d:%s", i+1, valueTypeName(arg)))
	}
	return out
}

func methodSignature(method Method) string {
	name := method.Name
	if name == "" && method.ClassName != "" {
		name = method.ClassName
	}
	params := make([]string, 0, len(method.Params))
	for _, param := range method.Params {
		paramType := strings.TrimSpace(param.Type)
		if paramType == "" {
			paramType = "Object"
		}
		if param.Name != "" {
			params = append(params, param.Name+" "+paramType)
		} else {
			params = append(params, paramType)
		}
	}
	return name + "(" + strings.Join(params, ", ") + ")"
}

func (vm *VM) bestMethodBySpecificity(applicable []Method, args []Value) (Method, bool, bool) {
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
			switch vm.compareMethodSpecificityForArgs(candidate, other, args) {
			case -1, 2:
				moreSpecificThanAll = false
			}
			if !moreSpecificThanAll {
				break
			}
		}
		if moreSpecificThanAll {
			if bestIndex >= 0 && vm.compareMethodSpecificityForArgs(candidate, applicable[bestIndex], args) == 0 {
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

func (vm *VM) compareMethodSpecificityForArgs(left, right Method, args []Value) int {
	if len(args) != len(left.Params) || len(args) != len(right.Params) {
		return vm.compareMethodSpecificity(left, right)
	}
	leftBetter := false
	rightBetter := false
	for i, arg := range args {
		leftType := vm.resolveTypeNameInClass(left.ClassName, left.Params[i].Type)
		rightType := vm.resolveTypeNameInClass(right.ClassName, right.Params[i].Type)
		leftScore := vm.conversionScore(leftType, arg)
		rightScore := vm.conversionScore(rightType, arg)
		switch {
		case leftScore > rightScore:
			leftBetter = true
		case rightScore > leftScore:
			rightBetter = true
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
		return vm.compareMethodSpecificity(left, right)
	}
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
	if platformTokenTypeAlias(from, to) {
		return true
	}
	if (strings.EqualFold(from, "String") && strings.EqualFold(to, "Id")) ||
		(strings.EqualFold(from, "Id") && strings.EqualFold(to, "String")) {
		return true
	}
	if messagingEmailAssignable(from, to) {
		return true
	}
	if strings.EqualFold(from, "Date") && strings.EqualFold(to, "Datetime") {
		return true
	}
	if strings.EqualFold(to, "sObject") && vm.isSObjectLikeType(from) {
		return true
	}
	if collectionBase(from) != "" && strings.EqualFold(collectionBase(from), collectionBase(to)) {
		fromElement, fromOK := collectionElementType(from)
		toElement, toOK := collectionElementType(to)
		if fromOK && toOK {
			return vm.typeAssignableTo(fromElement, toElement)
		}
	}
	if collectionBase(to) == "Iterable" && (collectionBase(from) == "List" || collectionBase(from) == "Set") {
		fromElement, fromOK := collectionElementType(from)
		toElement, toOK := collectionElementType(to)
		if fromOK && toOK {
			return vm.typeAssignableTo(fromElement, toElement)
		}
	}
	if fromKey, fromValue, fromOK := mapTypeArgs(from); fromOK {
		toKey, toValue, toOK := mapTypeArgs(to)
		if toOK {
			return vm.typeAssignableTo(fromKey, toKey) && vm.typeAssignableTo(fromValue, toValue)
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

func messagingEmailAssignable(from, to string) bool {
	if !strings.EqualFold(to, "Messaging.Email") {
		return false
	}
	return strings.EqualFold(from, "Messaging.SingleEmailMessage") ||
		strings.EqualFold(from, "Messaging.MassEmailMessage")
}

func collectionBase(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if strings.HasSuffix(typeName, "[]") && strings.TrimSpace(strings.TrimSuffix(typeName, "[]")) != "" {
		return "List"
	}
	base, ok := genericBaseName(typeName)
	if !ok {
		return ""
	}
	switch {
	case strings.EqualFold(base, "List"):
		return "List"
	case strings.EqualFold(base, "Set"):
		return "Set"
	case strings.EqualFold(base, "Iterator"):
		return "Iterator"
	case strings.EqualFold(base, "Iterable"):
		return "Iterable"
	default:
		return ""
	}
}

func isMapType(typeName string) bool {
	base, ok := genericBaseName(typeName)
	return ok && strings.EqualFold(base, "Map")
}

func genericBaseName(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	open := strings.IndexByte(typeName, '<')
	if open < 0 || !strings.HasSuffix(typeName, ">") {
		return "", false
	}
	base := strings.TrimSpace(typeName[:open])
	if rest, ok := stripLeadingSystemNamespace(base); ok {
		base = rest
	}
	return base, true
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
	if isMapType(valueType) && isMapType(paramType) {
		if vm.typeAssignableTo(valueType, paramType) {
			return 900
		}
		if vm.mapEntriesAssignable(paramType, value) {
			return 850
		}
		return -1
	}
	if score := numericConversionScore(paramType, valueType); score >= 0 {
		return score
	}
	if vm.typeAssignableTo(valueType, paramType) {
		return 900
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
	paramBase := collectionBase(paramType)
	switch value.Kind {
	case ValueList:
		if paramBase != "List" {
			return false
		}
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
		if paramBase != "Set" {
			return false
		}
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

func (vm *VM) mapEntriesAssignable(paramType string, value Value) bool {
	keyType, valueType, ok := mapTypeArgs(paramType)
	if !ok || value.Kind != ValueMap {
		return false
	}
	if len(value.Map) == 0 {
		// Local DTO JSON fields can carry untyped empty maps; overload specificity still picks the surface method.
		return true
	}
	for rawKey, item := range value.Map {
		if err := vm.ensureAssignable(keyType, valueFromMapKey(rawKey)); err != nil {
			return false
		}
		if err := vm.ensureAssignable(valueType, item); err != nil {
			return false
		}
	}
	return true
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
	if strings.EqualFold(typeName, target) {
		return 0, true
	}
	seen[typeName] = true
	class, ok := vm.lookupClass(typeName)
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
		if !strings.Contains(iface, ".") {
			if distance, ok := vm.typeDistance(class.Name+"."+iface, target, seen); ok {
				distance++
				if !found || distance < best {
					best = distance
					found = true
				}
			}
		}
	}
	return best, found
}

func valueTypeName(value Value) string {
	switch value.Kind {
	case ValueInt:
		if value.Type != "" {
			return value.Type
		}
		return "Integer"
	case ValueDecimal:
		return "Decimal"
	case ValueBool:
		return "Boolean"
	case ValueString:
		if value.Type != "" {
			return value.Type
		}
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

func newExceptionErrorWithContext(typeName, message, context string) error {
	value := Object(typeName)
	value.Fields["message"] = String(message)
	if strings.TrimSpace(context) != "" {
		value.Fields["__diagnosticContext"] = String(context)
	}
	return &apexThrowError{value: value}
}

func newNullDereferenceError(context string) error {
	return newExceptionErrorWithContext("NullPointerException", "Attempt to de-reference a null object", context)
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
	if !strings.Contains(target, ".") && strings.EqualFold(shortTypeName(typeName), target) {
		return true
	}
	if platformTokenTypeAlias(typeName, target) {
		return true
	}
	if strings.EqualFold(target, "sObject") && vm.isSObjectLikeType(typeName) {
		return true
	}
	if builtinExceptionTypeMatches(typeName, target) {
		return true
	}
	seen[typeName] = true
	class, ok := vm.lookupClass(typeName)
	if !ok {
		return false
	}
	if vm.typeMatches(class.SuperClass, target, seen) {
		return true
	}
	for _, iface := range class.Interfaces {
		if strings.EqualFold(shortTypeName(iface), shortTypeName(target)) {
			return true
		}
		if vm.typeMatches(iface, target, seen) {
			return true
		}
		if !strings.Contains(iface, ".") && vm.typeMatches(class.Name+"."+iface, target, seen) {
			return true
		}
	}
	return false
}

func platformTokenTypeAlias(typeName, target string) bool {
	switch {
	case strings.EqualFold(typeName, "Schema.SObjectType") && strings.EqualFold(target, "SObjectType"):
		return true
	case strings.EqualFold(typeName, "SObjectType") && strings.EqualFold(target, "Schema.SObjectType"):
		return true
	case strings.EqualFold(typeName, "Schema.SObjectField") && strings.EqualFold(target, "SObjectField"):
		return true
	case strings.EqualFold(typeName, "SObjectField") && strings.EqualFold(target, "Schema.SObjectField"):
		return true
	case strings.EqualFold(typeName, "Schema.DescribeFieldResult") && strings.EqualFold(target, "DescribeFieldResult"):
		return true
	case strings.EqualFold(typeName, "DescribeFieldResult") && strings.EqualFold(target, "Schema.DescribeFieldResult"):
		return true
	case strings.EqualFold(typeName, "Schema.DescribeSObjectResult") && strings.EqualFold(target, "DescribeSObjectResult"):
		return true
	case strings.EqualFold(typeName, "DescribeSObjectResult") && strings.EqualFold(target, "Schema.DescribeSObjectResult"):
		return true
	default:
		return false
	}
}

func shortTypeName(typeName string) string {
	if i := strings.LastIndex(typeName, "."); i >= 0 {
		return typeName[i+1:]
	}
	return typeName
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
	if strings.EqualFold(target, "Id") && value.Kind == ValueString {
		return Bool(validateApexIDShape(value.Text) == nil)
	}
	if strings.EqualFold(target, "Datetime") && value.Kind == ValueObject && strings.EqualFold(value.Type, "Date") {
		return Bool(true)
	}
	if value.Kind == ValueObject {
		return Bool(vm.typeMatches(value.Type, target, make(map[string]bool)))
	}
	if collectionBase(target) != "" || isMapType(target) {
		valueType := valueTypeName(value)
		if vm.typeAssignableTo(valueType, target) {
			return Bool(true)
		}
		if collectionBase(target) != "" && vm.collectionElementsAssignable(target, value) {
			return Bool(true)
		}
		if isMapType(target) && vm.mapEntriesAssignable(target, value) {
			return Bool(true)
		}
		return Bool(false)
	}
	if strings.EqualFold(target, "Object") {
		return Bool(true)
	}
	valueType := valueTypeName(value)
	if strings.EqualFold(valueType, target) {
		return Bool(true)
	}
	return Bool(numericConversionScore(target, valueType) >= 0)
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

func exceptionQualifiedTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if !strings.Contains(typeName, ".") && isBuiltinExceptionType(typeName) {
		return "System." + typeName
	}
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
var metadataDeployStatusNames = []string{"Succeeded", "SUCCEEDED", "Failed", "FAILED", "InProgress", "INPROGRESS", "Pending", "PENDING", "Canceling", "CANCELING", "Canceled", "CANCELED"}
var metadataMetadataTypeNames = []string{"CustomMetadata"}

func isLoggingLevelName(level string) bool {
	_, ok := canonicalLoggingLevelName(level)
	return ok
}

func canonicalLoggingLevelName(level string) (string, bool) {
	for _, name := range loggingLevelNames {
		if strings.EqualFold(level, name) {
			return name, true
		}
	}
	return "", false
}

func isLoggingLevelValue(value Value) bool {
	if value.Kind != ValueObject || value.Type != "LoggingLevel" {
		return false
	}
	return isLoggingLevelName(value.Text)
}

func (vm *VM) coerceAssignable(typeName string, value Value) (Value, error) {
	if value.Type != "" && strings.EqualFold(value.Type, typeName) {
		return value, nil
	}
	if value.Kind == ValueString {
		if strings.EqualFold(typeName, "String") && strings.EqualFold(value.Type, "Id") {
			if len(value.Text) == 15 {
				return String(apexIDTo18(value.Text)), nil
			}
			return String(value.Text), nil
		}
		if class, ok := vm.resolveEnumClass(typeName); ok {
			valueText := value.Text
			if dot := strings.LastIndexByte(valueText, '.'); dot >= 0 {
				valueText = valueText[dot+1:]
			}
			for _, enumValue := range class.EnumValues {
				if enumValue == valueText {
					return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
				}
			}
		}
		if apexIdentifierStartsUpper(typeName) && !isBuiltinTypeName(typeName) && !platformScalarObject(typeName) && (isLikelyEnumValueText(value.Text) || strings.HasSuffix(typeName, "Type")) {
			valueText := value.Text
			if dot := strings.LastIndexByte(valueText, '.'); dot >= 0 {
				valueText = valueText[dot+1:]
			}
			return Value{Kind: ValueObject, Type: typeName, Text: valueText}, nil
		}
		switch typeName {
		case "Id":
			if err := validateApexIDShape(value.Text); err != nil {
				return Null, newExceptionError("StringException", strings.TrimPrefix(err.Error(), "System.StringException: "))
			}
			value.Type = "Id"
			return value, nil
		case "Date":
			parsed, err := parseDateText(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Date", parsed.Format("2006-01-02")), nil
		case "Datetime":
			if strings.EqualFold(value.Type, "Date") {
				parsed, err := parseDateText(value.Text)
				if err != nil {
					return Null, err
				}
				return platformScalar("Datetime", formatPlatformDatetime(parsed)), nil
			}
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
		if class, ok := vm.resolveEnumClass(typeName); ok && len(class.EnumValues) > 0 {
			if strings.EqualFold(value.Type, class.Name) {
				return Value{Kind: ValueObject, Type: class.Name, Text: value.Text}, nil
			}
		}
		if strings.EqualFold(typeName, "Datetime") && strings.EqualFold(value.Type, "Date") {
			text, err := platformScalarText(value, "Date")
			if err != nil {
				return Null, err
			}
			parsed, err := parseDateText(text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Datetime", formatPlatformDatetime(parsed)), nil
		}
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
		if collectionBase(typeName) == "Iterator" && isIteratorValue(value) {
			value.Type = typeName
			return value, nil
		}
		if strings.EqualFold(typeName, "Object") || vm.typeAssignableTo(value.Type, typeName) || vm.typeMatches(value.Type, typeName, make(map[string]bool)) {
			if !strings.EqualFold(typeName, "Object") && !strings.EqualFold(value.Type, typeName) && value.Runtime == "" {
				value.Runtime = value.Type
			}
			return value, nil
		}
		return Null, fmt.Errorf("cannot assign %s to %s", value.Type, typeName)
	}
	if value.Kind == ValueList && vm.isSObjectLikeType(typeName) {
		if len(value.List) != 1 {
			return Null, fmt.Errorf("cannot assign list with %d records to %s", len(value.List), typeName)
		}
		return vm.coerceAssignable(typeName, value.List[0])
	}
	if collectionBase(typeName) == "List" && value.Kind == ValueList {
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
	if collectionBase(typeName) == "List" && value.Kind == ValueMap {
		if records, ok := queryResultRecordsList(value); ok {
			return vm.coerceAssignable(typeName, records)
		}
	}
	if collectionBase(typeName) == "Set" && value.Kind == ValueSet {
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
	if collectionBase(typeName) == "Iterable" && (value.Kind == ValueList || value.Kind == ValueSet) {
		value.Type = typeName
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		items := collectionMembers(value)
		out := make([]Value, 0, len(items))
		for _, item := range items {
			coerced, err := vm.coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			out = append(out, coerced)
		}
		if value.Kind == ValueSet {
			value.Set = out
		} else {
			value.List = out
		}
		return value, nil
	}
	if isMapType(typeName) && value.Kind == ValueMap {
		value.Type = typeName
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			return value, nil
		}
		type coercedEntry struct {
			key   string
			value Value
		}
		entries := make([]coercedEntry, 0, len(value.Map))
	for rawKey, item := range value.Map {
			keyValue := mapStoredKey(value, rawKey)
			coercedKey, err := vm.coerceAssignable(keyType, keyValue)
			if err != nil {
				return Null, fmt.Errorf("key: %w", err)
			}
			coercedValue, err := vm.coerceAssignable(valueType, item)
			if err != nil {
				return Null, fmt.Errorf("value: %w", err)
			}
			entries = append(entries, coercedEntry{key: mapKey(coercedKey), value: coercedValue})
		}
		for rawKey := range value.Map {
			delete(value.Map, rawKey)
		}
	for _, entry := range entries {
		value.Map[entry.key] = entry.value
	}
		return value, nil
	}
	return coerceAssignable(typeName, value)
}

func (vm *VM) resolveEnumClass(typeName string) (Class, bool) {
	cacheKey := vm.currentClass + "|" + typeName
	if vm.enumLookup != nil {
		if cached, ok := vm.enumLookup[cacheKey]; ok {
			return cached.Class, cached.OK
		}
	} else {
		vm.enumLookup = make(map[string]enumClassLookup)
	}
	if enumType, ok := vm.resolveClassName(typeName); ok {
		if class, ok := vm.Classes[enumType]; ok && len(class.EnumValues) > 0 {
			vm.enumLookup[cacheKey] = enumClassLookup{Class: class, OK: true}
			return class, true
		}
	}
	if !strings.Contains(typeName, ".") {
		suffix := "." + typeName
		for _, class := range vm.Classes {
			if strings.HasSuffix(class.Name, suffix) && len(class.EnumValues) > 0 {
				vm.enumLookup[cacheKey] = enumClassLookup{Class: class, OK: true}
				return class, true
			}
		}
	}
	vm.enumLookup[cacheKey] = enumClassLookup{}
	return Class{}, false
}

func isLikelyEnumValueText(text string) bool {
	if text == "" {
		return false
	}
	if dot := strings.LastIndexByte(text, '.'); dot >= 0 {
		text = text[dot+1:]
	}
	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' {
			continue
		}
		return false
	}
	return true
}

func (vm *VM) ensureAssignable(typeName string, value Value) error {
	_, err := vm.coerceAssignable(typeName, value)
	return err
}

func (vm *VM) coerceCast(typeName string, value Value) (Value, error) {
	coerced, err := vm.coerceAssignable(typeName, value)
	if err == nil {
		return coerced, nil
	}
	if value.Kind == ValueDecimal && strings.EqualFold(typeName, "Integer") {
		if value.Decimal < math.MinInt32 || value.Decimal > math.MaxInt32 {
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", valueTypeName(value), typeName))
		}
		return Int(int64(value.Decimal)), nil
	}
	if value.Kind == ValueDecimal && strings.EqualFold(typeName, "Long") {
		if value.Decimal < float64(math.MinInt64) || value.Decimal > float64(math.MaxInt64) {
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", valueTypeName(value), typeName))
		}
		return Int(int64(value.Decimal)), nil
	}
	var thrown *apexThrowError
	if errors.As(err, &thrown) {
		return Null, err
	}
	return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", valueTypeName(value), typeName))
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

func (vm *VM) mapLookupKey(receiver Value, key Value) string {
	keyType, _, ok := mapTypeArgs(receiver.Type)
	if !ok || strings.TrimSpace(keyType) == "" {
		return vm.mapKey(key)
	}
	coerced, err := vm.coerceAssignable(keyType, key)
	if err != nil {
		return vm.mapKey(key)
	}
	return vm.mapKey(coerced)
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
	return value.Kind == ValueObject && (strings.EqualFold(value.Type, "Iterator") || strings.HasPrefix(strings.ToLower(value.Type), "iterator<") || strings.HasPrefix(strings.ToLower(value.Type), "system.iterator<") || value.Type == "Database.QueryLocatorIterator")
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

func (vm *VM) sortComparableValues(values []Value, result *Result) error {
	for _, value := range values {
		switch value.Kind {
		case ValueInt, ValueDecimal, ValueString, ValueBool:
		case ValueObject:
		default:
			return unsupportedCallError("List.sort for non-primitive comparable values")
		}
	}
	if listSortHasObjects(values) {
		return vm.sortApexComparableValues(values, result)
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

func listSortHasObjects(values []Value) bool {
	for _, value := range values {
		if value.Kind == ValueObject {
			return true
		}
	}
	return false
}

func (vm *VM) sortApexComparableValues(values []Value, result *Result) error {
	hasSObject := false
	hasComparable := false
	for _, value := range values {
		if value.Kind != ValueObject {
			return unsupportedCallError("List.sort for mixed primitive and Comparable values")
		}
		runtimeType := runtimeObjectType(value)
		if vm.isSortableSObjectValue(value) {
			hasSObject = true
			continue
		}
		if _, ok, ambiguous := vm.resolveInstanceMethodForArgs(runtimeType, "compareTo", []Value{value}); ambiguous {
			return vm.ambiguousOverloadError(runtimeType+".compareTo", []Value{value})
		} else if !ok {
			return unsupportedCallError("List.sort for non-primitive comparable values")
		}
		hasComparable = true
	}
	if hasSObject && hasComparable {
		return unsupportedCallError("List.sort for mixed sObject and Comparable values")
	}
	var sortErr error
	sort.SliceStable(values, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		if hasSObject {
			return compareSObjectSortValues(values[i], values[j]) < 0
		}
		compare, err := vm.compareApexComparableValues(values[i], values[j], result)
		if err != nil {
			sortErr = err
			return false
		}
		return compare < 0
	})
	return sortErr
}

func (vm *VM) isSortableSObjectValue(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	runtimeType := runtimeObjectType(value)
	if _, ok := vm.lookupClass(runtimeType); ok {
		return false
	}
	return vm.isSObjectLikeType(runtimeType)
}

func compareSObjectSortValues(left, right Value) int {
	if cmp := strings.Compare(strings.ToLower(runtimeObjectType(left)), strings.ToLower(runtimeObjectType(right))); cmp != 0 {
		return cmp
	}
	if leftID, rightID, ok := sObjectSortFieldPair(left, right, "Id"); ok {
		return strings.Compare(leftID, rightID)
	}
	if leftName, rightName, ok := sObjectSortFieldPair(left, right, "Name"); ok {
		return strings.Compare(leftName, rightName)
	}
	return strings.Compare(sObjectStableSortKey(left), sObjectStableSortKey(right))
}

func sObjectSortFieldPair(left, right Value, field string) (string, string, bool) {
	_, leftValue, leftOK := objectFieldValue(left, field)
	_, rightValue, rightOK := objectFieldValue(right, field)
	if !leftOK || !rightOK || leftValue.Kind == ValueNull || rightValue.Kind == ValueNull {
		return "", "", false
	}
	return leftValue.String(), rightValue.String(), true
}

func sObjectStableSortKey(value Value) string {
	fields := make([]string, 0, len(value.Fields))
	for field := range value.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		fields = append(fields, field)
	}
	sort.Strings(fields)
	var out strings.Builder
	for _, field := range fields {
		out.WriteString(strings.ToLower(field))
		out.WriteByte('=')
		out.WriteString(value.Fields[field].String())
		out.WriteByte(';')
	}
	return out.String()
}

func (vm *VM) compareApexComparableValues(left, right Value, result *Result) (int64, error) {
	if left.Kind != ValueObject {
		return 0, unsupportedCallError("List.sort for mixed primitive and Comparable values")
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(runtimeObjectType(left), "compareTo", []Value{right})
	if ambiguous {
		return 0, vm.ambiguousOverloadError(runtimeObjectType(left)+".compareTo", []Value{right})
	}
	if !ok {
		return 0, unsupportedCallError("List.sort for non-primitive comparable values")
	}
	value, err := vm.callMethodWithReceiver(target, left, []Value{right}, result)
	if err != nil {
		return 0, err
	}
	switch value.Kind {
	case ValueInt:
		return value.Int, nil
	case ValueDecimal:
		return int64(value.Decimal), nil
	default:
		return 0, fmt.Errorf("%s returned %s, want Integer", target.Name, valueTypeName(value))
	}
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
		if ok && typeName == "Schema.SObjectType" {
			return sObjectTypeToken(text)
		}
		if ok && typeName == "Schema.SObjectField" {
			objectName, fieldName, hasField := strings.Cut(text, ".")
			if hasField {
				return sObjectFieldToken(objectName, fieldName)
			}
		}
		if ok && typeName == "Schema.ChildRelationship" {
			relationshipName, rest, hasRelationship := strings.Cut(text, "|")
			childName, fieldName, hasField := strings.Cut(rest, "|")
			if hasRelationship && hasField {
				value := Object("Schema.ChildRelationship")
				value.Fields["relationshipName"] = String(relationshipName)
				value.Fields["childSObject"] = sObjectTypeToken(childName)
				value.Fields["field"] = sObjectFieldToken(childName, fieldName)
				value.Fields["cascadeDelete"] = Bool(false)
				value.Fields["restrictedDelete"] = Bool(false)
				return value
			}
		}
		if ok && typeName == "Type" {
			return Value{Kind: ValueObject, Type: "Type", Text: text}
		}
		if ok && platformScalarObject(typeName) {
			return platformScalar(typeName, text)
		}
		if ok && typeName != "" {
			value := Value{Kind: ValueObject, Type: typeName, Text: text, Fields: make(map[string]Value)}
			if looksLikeID(text) {
				value.Fields["Id"] = String(text)
			}
			return value
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
			if context, ok := thrown.Fields["__diagnosticContext"]; ok && context.Kind == ValueString && strings.TrimSpace(context.Text) != "" {
				message += " (context: " + context.Text + ")"
			}
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
		if collectionBase(typeName) != "" || isMapType(typeName) {
			if coerced, err := coerceCollectionValue(typeName, explicit); err == nil {
				return cloneValue(coerced)
			}
		}
		return cloneValue(explicit)
	}
	switch typeName {
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
	if len(vm.callStack) >= maxApexCallDepth {
		return Null, &RuntimeError{Type: "RuntimeError", Message: "maximum Apex call stack depth exceeded", Stack: vm.stackFrames()}
	}
	if len(args) != len(method.Params) {
		return Null, fmt.Errorf("%s expects %d arguments", method.Name, len(method.Params))
	}
	if method.Unsupported != "" {
		return Null, fmt.Errorf("%s is not supported by the local VM: %s", method.Name, method.Unsupported)
	}
	if methodHasModifier(method.Modifiers, "abstract") {
		return Null, fmt.Errorf("cannot execute abstract method %s", method.Name)
	}
	if method.ClassName != "" && !strings.Contains(method.Name, ".<static_") {
		if err := vm.ensureClassInitialized(method.ClassName); err != nil {
			return Null, err
		}
	}
	constructorKey := ""
	if method.IsConstructor {
		constructorKey = constructorCallKey(method)
		if vm.activeConstructors == nil {
			vm.activeConstructors = make(map[string]int)
		}
		vm.activeConstructors[constructorKey]++
		defer func() {
			vm.activeConstructors[constructorKey]--
			if vm.activeConstructors[constructorKey] <= 0 {
				delete(vm.activeConstructors, constructorKey)
			}
		}()
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
		var thrown *apexThrowError
		if errors.As(err, &thrown) {
			if len(thrown.stack) == 0 {
				thrown.stack = vm.rawStackFrames()
			}
			return Null, thrown
		}
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
	switch value.Kind {
	case ValueList:
		return vm.displayList(value.List, result)
	case ValueSet:
		return vm.displayList(value.Set, result)
	case ValueMap:
		return value.String(), nil
	case ValueObject:
	default:
		return value.String(), nil
	}
	if value.Type == "Type" {
		if text := typeValueText(value); text != "" {
			return text, nil
		}
	}
	if strings.EqualFold(value.Type, "Schema.SObjectField") {
		if fieldName, ok := value.Fields["field"]; ok && fieldName.Kind == ValueString {
			return fieldName.Text, nil
		}
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
		return "", vm.ambiguousOverloadError("toString", nil)
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

func (vm *VM) displayList(values []Value, result *Result) (string, error) {
	parts := make([]string, 0, len(values))
	for _, item := range values {
		text, err := vm.displayString(item, result)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return "(" + strings.Join(parts, ", ") + ")", nil
}

func (vm *VM) callMember(callee string, args []Value, result *Result) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) < 2 {
		return Null, false, nil
	}
	receiverName, method := parts[0], parts[1]
	if strings.EqualFold(receiverName, "super") {
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
		class, _ := vm.lookupClass(dispatchClass)
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(class.SuperClass, method, args)
		if ambiguous {
			return Null, true, vm.ambiguousOverloadError(callee, args)
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
		if actual, found := vm.lookupGlobalName(receiverName); found {
			receiverName = actual
			receiver = vm.Globals[actual]
			ok = true
		}
	}
	if !ok {
		if vm.currentClass != "" {
			if field, owner, ok := vm.lookupStaticField(vm.currentClass, receiverName); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+receiverName, field.Modifiers); err != nil {
					return Null, true, err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return Null, true, err
				}
				field, _, _ = vm.lookupStaticField(owner, receiverName)
				receiver := field.Value
				if field.Getter != nil {
					var err error
					receiver, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return Null, true, err
					}
				}
				return vm.callValueMember(receiverName, receiver, method, args, result)
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
		return Null, true, newNullDereferenceError(nullMemberContext(receiverName, method))
	}
	if _, declared := vm.VarTypes[receiverName]; !declared {
		if value, handled, err := vm.callSObjectTypeStaticMember(receiverName, method, args); handled || err != nil {
			return value, true, err
		}
	}
	if receiverType := vm.VarTypes[receiverName]; strings.EqualFold(receiverType, "Id") || idMemberReceiver(receiver, method) {
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
		dispatchType := runtimeObjectType(receiver)
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(dispatchType, method, args)
		if ambiguous {
			return Null, true, vm.ambiguousOverloadError(memberCallName(receiverName, dispatchType, method), args)
		}
		if !ok {
			if value, handled, err := callObjectMember(receiver, method, args); handled || err != nil {
				return value, true, err
			}
			return Null, true, unsupportedCallError(memberCallName(receiverName, dispatchType, method))
		}
		accessMethod := target
		if receiverType := vm.VarTypes[receiverName]; receiverType != "" {
			accessMethod = vm.dispatchAccessMethod(receiverType, target, method, args)
		}
		if err := vm.checkMemberAccess(accessMethod.ClassName, accessMethod.Access, accessMethod.Name, accessMethod.Modifiers); err != nil {
			return Null, true, err
		}
		value, err := vm.callMethodWithReceiver(target, receiver, args, result)
		return value, true, err
	}

	switch receiver.Kind {
	case ValueList:
		method = canonicalCollectionMemberName("List", method)
		switch method {
		case "getSObjectType":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("List.getSObjectType expects 0 arguments")
			}
			if elementType, ok := collectionElementType(receiver.Type); ok && !strings.EqualFold(elementType, "sObject") {
				if token, ok := vm.sObjectTypeTokenForName(elementType); ok {
					return token, true, nil
				}
			}
			for _, item := range receiver.List {
				if item.Kind == ValueObject && vm.isSObjectLikeType(item.Type) {
					if token, ok := vm.sObjectTypeTokenForName(item.Type); ok {
						return token, true, nil
					}
				}
			}
			if elementType, ok := collectionElementType(receiver.Type); ok && strings.EqualFold(elementType, "sObject") {
				return Null, true, nil
			}
			if len(receiver.List) == 0 {
				return Null, true, nil
			}
			if token, ok := vm.sObjectTypeTokenForName("SObject"); ok {
				return token, true, nil
			}
			return Null, true, fmt.Errorf("List.getSObjectType requires SObject list")
		case "add":
			if len(args) != 1 && len(args) != 2 {
				return Null, true, fmt.Errorf("List.add expects 1 or 2 arguments")
			}
			previous := receiver
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
			vm.propagateCollectionMutation(previous, receiver)
			if insertAt >= 0 {
				return Null, true, nil
			}
			return Bool(true), true, nil
		case "addAll":
			if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
				return Null, true, fmt.Errorf("List.addAll expects List or Set")
			}
			previous := receiver
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
			vm.propagateCollectionMutation(previous, receiver)
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
			cloned.Ref = newValueRef()
			cloned.List = append([]Value(nil), receiver.List...)
			return cloned, true, nil
		case "deepClone":
			if len(args) > 3 {
				return Null, true, fmt.Errorf("List.deepClone expects at most 3 Boolean arguments")
			}
			for _, arg := range args {
				if arg.Kind != ValueBool {
					return Null, true, fmt.Errorf("List.deepClone preserve options expect Boolean arguments")
				}
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
			previous := receiver
			sorted := append([]Value(nil), receiver.List...)
			if err := vm.sortComparableValues(sorted, result); err != nil {
				return Null, true, err
			}
			receiver.List = sorted
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutation(previous, receiver)
			return Null, true, nil
		case "remove":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, true, fmt.Errorf("List.remove expects integer index")
			}
			previous := receiver
			i := int(args[0].Int)
			if i < 0 || i >= len(receiver.List) {
				return Null, true, fmt.Errorf("List index out of bounds: %d", i)
			}
			removed := receiver.List[i]
			receiver.List = append(receiver.List[:i], receiver.List[i+1:]...)
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutation(previous, receiver)
			return removed, true, nil
		case "set":
			if len(args) != 2 || args[0].Kind != ValueInt {
				return Null, true, fmt.Errorf("List.set expects integer index and value")
			}
			previous := receiver
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
			vm.propagateCollectionMutation(previous, receiver)
			return Null, true, nil
		}
	case ValueSet:
		method = canonicalCollectionMemberName("Set", method)
		switch method {
		case "add":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Set.add expects 1 argument")
			}
			previous := receiver
			item, err := vm.coerceCollectionElement(receiver.Type, args[0])
			if err != nil {
				return Null, true, fmt.Errorf("Set.add: %w", err)
			}
			if !containsValue(receiver.Set, item) {
				receiver.Set = append(receiver.Set, item)
				if err := vm.storeReceiver(receiverName, receiver); err != nil {
					return Null, true, err
				}
				vm.propagateCollectionMutation(previous, receiver)
				return Bool(true), true, nil
			}
			return Bool(false), true, nil
		case "addAll":
			if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
				return Null, true, fmt.Errorf("Set.addAll expects List or Set")
			}
			previous := receiver
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
				vm.propagateCollectionMutation(previous, receiver)
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
			cloned.Ref = newValueRef()
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
		method = canonicalCollectionMemberName("Map", method)
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
			if existing, ok := receiver.Map[vm.mapKey(key)]; ok {
				previous = existing
			}
			receiver.Map[vm.mapKey(key)] = item
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
				receiver.Map[vm.mapKey(key)] = item
			}
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			return Null, true, nil
		case "get":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Map.get expects 1 argument")
			}
			key := vm.mapLookupKey(receiver, args[0])
			value, ok := receiver.Map[key]
			if !ok {
				value, ok = specialMapLookup(receiver, args[0])
			}
			if !ok {
				return Null, true, nil
			}
			return value, true, nil
		case "containsKey":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Map.containsKey expects 1 argument")
			}
			key := vm.mapLookupKey(receiver, args[0])
			_, ok := receiver.Map[key]
			if !ok {
				_, ok = specialMapLookup(receiver, args[0])
			}
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
			cloned.Ref = newValueRef()
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

func runtimeObjectType(value Value) string {
	if value.Runtime != "" {
		return value.Runtime
	}
	return value.Type
}

func canonicalCollectionMemberName(collection, method string) string {
	var known []string
	switch collection {
	case "List":
		known = []string{"add", "addAll", "size", "get", "contains", "indexOf", "clone", "deepClone", "iterator", "sort", "remove", "set", "getSObjectType"}
	case "Set":
		known = []string{"add", "addAll", "size", "contains", "containsAll", "remove", "removeAll", "retainAll", "clone", "deepClone", "iterator"}
	case "Map":
		known = []string{"put", "putAll", "get", "containsKey", "keySet", "values", "remove", "clear", "size", "isEmpty", "clone", "deepClone"}
	}
	for _, candidate := range known {
		if strings.EqualFold(method, candidate) {
			return candidate
		}
	}
	return method
}

func canonicalPlatformObjectMemberName(typeName, method string) string {
	known := []string{
		"get", "iterator",
		"getQuery",
		"getJobId", "getTriggerId",
		"getAsyncApexJobId", "getRequestId", "getResult", "getException",
		"getDuplicateSignature", "setDuplicateSignature",
		"getMaximumQueueableStackDepth", "setMaximumQueueableStackDepth",
		"getMinimumQueueableDelayInMinutes", "setMinimumQueueableDelayInMinutes",
		"newSObject", "getDescribe", "getRecordTypeInfosByName", "getRecordTypeInfosById",
		"getMap",
		"getName", "getLabel", "getType", "getSOAPType", "getSoapType",
		"isNillable", "isExternalId", "isUnique", "isEncrypted", "isNameField",
		"getReferenceTo", "getRelationshipName", "getPicklistValues", "getSObjectField",
		"getFields", "getFieldPath", "getRequired", "getDbRequired",
		"getController", "getControllerValues", "isAccessible", "isCreateable", "isUpdateable",
		"to15", "to18", "getSObjectType",
		"toStartOfMonth", "format", "toString", "date", "time",
		"equals", "hashCode", "newInstance", "isAssignableFrom",
		"send", "toExternalForm", "getProtocol", "getHost", "getAuthority",
		"getPath", "getQuery", "getRef", "getFile", "getPort", "getDefaultPort",
	}
	if isExceptionType(typeName) {
		known = append(known,
			"getMessage",
			"setMessage",
			"getNumDml",
			"getDmlMessage",
			"getDmlStatusCode",
			"getDmlFields",
			"getDmlId",
			"getDmlIndex",
			"getCause",
			"initCause",
			"getDescription",
			"getIndex",
			"getPattern",
			"getTypeName",
			"getLineNumber",
			"getStackTraceString",
			"toString",
		)
	}
	return canonicalStdlibMemberName(method, known...)
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
	if _, declared := vm.VarTypes[receiverName]; !declared {
		if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
			if _, _, _, ok := vm.lookupThisFieldRoot(this, receiverName); ok {
				return vm.assign(receiverName, value)
			}
		}
	}
	if _, ok := vm.lookupGlobalName(receiverName); ok {
		return vm.assign(receiverName, value)
	}
	if vm.currentClass != "" {
		if _, _, ok := vm.lookupStaticField(vm.currentClass, receiverName); ok {
			return vm.assign(receiverName, value)
		}
	}
	vm.Globals[receiverName] = value
	return nil
}

func (vm *VM) mapKey(value Value) string {
	if key, ok := fflibQualifiedMethodMapKey(value, vm.fflibIndependentMocksEnabled()); ok {
		return key
	}
	return mapKey(value)
}

func specialMapLookup(receiver, key Value) (Value, bool) {
	if receiver.Kind != ValueMap || key.Kind != ValueString {
		return Null, false
	}
	switch receiver.Type {
	case "Schema.GlobalDescribeMap":
		for rawKey, value := range receiver.Map {
			candidate := valueFromMapKey(rawKey)
			if candidate.Kind == ValueString && strings.EqualFold(candidate.Text, key.Text) {
				return value, true
			}
		}
	}
	for _, value := range receiver.Map {
		if value.Kind != ValueObject || value.Type != "Schema.RecordTypeInfo" {
			continue
		}
		developerName, ok := value.Fields["developerName"]
		if ok && developerName.Kind == ValueString && developerName.Text == key.Text {
			return value, true
		}
	}
	return Null, false
}

func (vm *VM) fflibIndependentMocksEnabled() bool {
	class, ok := vm.lookupClass("fflib_ApexMocksConfig")
	if !ok {
		return true
	}
	field, ok := class.StaticFields["HasIndependentMocks"]
	if !ok {
		for name, candidate := range class.StaticFields {
			if strings.EqualFold(name, "HasIndependentMocks") {
				field = candidate
				ok = true
				break
			}
		}
	}
	if !ok || field.Value.Kind != ValueBool {
		return false
	}
	return field.Value.Bool
}

func (vm *VM) propagateCollectionMutation(previous, updated Value) {
	if !sameCollectionType(previous, updated) {
		return
	}
	for name, value := range vm.Globals {
		vm.Globals[name] = replaceCollectionAlias(value, previous, updated, make(map[uint64]bool))
	}
	for className, class := range vm.Classes {
		changed := false
		for fieldName, field := range class.StaticFields {
			replaced := replaceCollectionAlias(field.Value, previous, updated, make(map[uint64]bool))
			if !field.Value.Equal(replaced) {
				field.Value = replaced
				class.StaticFields[fieldName] = field
				changed = true
			}
		}
		if changed {
			vm.Classes[className] = class
			vm.storeClassAliases(class)
		}
	}
}

func replaceCollectionAlias(value, previous, updated Value, seen map[uint64]bool) Value {
	if collectionAliasMatch(previous, value) {
		return updated
	}
	if value.Ref != 0 {
		if seen[value.Ref] {
			return value
		}
		seen[value.Ref] = true
	}
	switch value.Kind {
	case ValueObject:
		for name, child := range value.Fields {
			value.Fields[name] = replaceCollectionAlias(child, previous, updated, seen)
		}
	case ValueMap:
		for key, child := range value.Map {
			value.Map[key] = replaceCollectionAlias(child, previous, updated, seen)
		}
	case ValueList:
		for i, child := range value.List {
			value.List[i] = replaceCollectionAlias(child, previous, updated, seen)
		}
	case ValueSet:
		for i, child := range value.Set {
			value.Set[i] = replaceCollectionAlias(child, previous, updated, seen)
		}
	}
	return value
}

func collectionAliasMatch(left, right Value) bool {
	if !sameCollectionType(left, right) {
		return false
	}
	return left.Ref != 0 && left.Ref == right.Ref
}

func sameCollectionType(left, right Value) bool {
	if left.Kind != right.Kind || left.Type != right.Type {
		return false
	}
	switch left.Kind {
	case ValueList, ValueSet, ValueMap:
		return true
	default:
		return false
	}
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
	if strings.EqualFold(typeName, "AggregateResult") {
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
		return Null, true, vm.ambiguousOverloadError(receiver.Type+"."+method, args)
	}
	if !ok {
		return Null, true, unsupportedCallError("Test.createStub dynamic method " + receiver.Type + "." + method + " without local target method metadata")
	}
	provider := receiver.Fields["__oaerStubProvider"]
	paramTypes := make([]Value, 0, len(target.Params))
	paramNames := make([]Value, 0, len(target.Params))
	for _, param := range target.Params {
		paramTypes = append(paramTypes, platformScalar("Type", vm.resolveTypeNameInClass(target.ClassName, param.Type)))
		paramNames = append(paramNames, String(param.Name))
	}
	returnType := vm.resolveTypeNameInClass(target.ClassName, target.ReturnType)
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
		return Null, true, vm.ambiguousOverloadError(provider.Type+".handleMethodCall", metadataArgs)
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
			if errors.Is(err, errSObjectFieldTokenWrongObject) {
				return Null, true, newExceptionError("SObjectException", err.Error())
			}
			if errors.Is(err, errSObjectFieldTokenNull) {
				return Null, true, newExceptionError("SObjectException", err.Error())
			}
			return Null, true, fmt.Errorf("SObject.get expects field name String or Schema.SObjectField")
		}
		field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
		if err := vm.unqueriedSObjectFieldError(receiver, field); err != nil {
			return Null, true, err
		}
		value, ok := receiver.Fields[field]
		if !ok {
			if value, ok := vm.missingSObjectFieldValue(receiver, field); ok {
				return value, true, nil
			}
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
			if isInternalSObjectField(field) {
				continue
			}
			out.Map[mapKey(String(field))] = value
		}
		return out, true, nil
	case "getSObjectType":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.getSObjectType expects 0 arguments")
		}
		token, ok := vm.sObjectTypeTokenForName(receiver.Type)
		if !ok {
			return Null, false, nil
		}
		return token, true, nil
	case "clone":
		if len(args) != 0 && len(args) != 1 && len(args) != 4 {
			return Null, true, fmt.Errorf("SObject.clone expects 0, 1, or 4 arguments")
		}
		for _, arg := range args {
			if arg.Kind != ValueBool {
				return Null, true, fmt.Errorf("SObject.clone preserve flags must be Boolean")
			}
		}
		cloned := cloneValue(receiver)
		if cloned.Fields == nil {
			cloned.Fields = make(map[string]Value)
		}
		preserveID := len(args) > 0 && args[0].Bool
		if !preserveID {
			delete(cloned.Fields, "Id")
		}
		delete(cloned.Fields, sobjectErrorsField)
		delete(cloned.Fields, sobjectReadOnlyField)
		return cloned, true, nil
	case "getSObject":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("SObject.getSObject expects relationship name String")
		}
		field := vm.resolveSObjectFieldName(receiver.Type, args[0].Text)
		value, ok := receiver.Fields[field]
		if !ok || value.Kind == ValueNull {
			return Null, true, nil
		}
		if value.Kind != ValueObject {
			return Null, true, fmt.Errorf("SObject.getSObject field %s is not an SObject", field)
		}
		return value, true, nil
	case "getSObjects":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("SObject.getSObjects expects relationship name String")
		}
		field := vm.resolveSObjectFieldName(receiver.Type, args[0].Text)
		value, ok := receiver.Fields[field]
		if !ok || value.Kind == ValueNull {
			return List(), true, nil
		}
		if value.Kind != ValueList {
			return Null, true, fmt.Errorf("SObject.getSObjects field %s is not a List", field)
		}
		return value, true, nil
	default:
		return Null, false, nil
	}
}

const (
	sobjectErrorsField        = "__oaer_errors"
	sobjectReadOnlyField      = "__oaer_readonly"
	sobjectQueriedFieldsField = "__oaer_queried_fields"
)

func isInternalSObjectField(field string) bool {
	return field == sobjectErrorsField || field == sobjectReadOnlyField || field == sobjectQueriedFieldsField
}

func queriedSObjectFieldsValue(objectName string, fields map[string]bool) Value {
	value := Map()
	value.Type = "Map<String,Boolean>"
	value.Map[mapKey(String("object"))] = String(objectName)
	for field := range fields {
		value.Map[mapKey(String(field))] = Bool(true)
	}
	return value
}

func dmlVisibleSObjectFields(value *Value) map[string]bool {
	fields := map[string]bool{"id": true}
	if value == nil || value.Kind != ValueObject {
		return fields
	}
	for field := range value.Fields {
		if isInternalSObjectField(field) || isSObjectSystemField(field) {
			continue
		}
		fields[strings.ToLower(field)] = true
	}
	return fields
}

func (vm *VM) unqueriedSObjectFieldError(receiver Value, field string) error {
	if receiver.Kind != ValueObject {
		return nil
	}
	selected, ok := receiver.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return nil
	}
	if _, ok := selected.Map[mapKey(String(strings.ToLower(field)))]; ok {
		return nil
	}
	return newExceptionError("SObjectException", fmt.Sprintf("SObject row was retrieved via SOQL without querying the requested field: %s.%s", receiver.Type, field))
}

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
	_, _, ok := vm.sObjectFieldDefinition(typeName, field)
	return ok
}

func (vm *VM) sObjectFieldDefinition(typeName, field string) (storage.ObjectDefinition, storage.Field, bool) {
	if vm.Org == nil {
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, typeName)
	if !ok {
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	definition := vm.Org.Objects[objectName].Definition
	fieldName, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
	if !ok {
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	return definition, definition.Fields[fieldName], true
}

func (vm *VM) missingSObjectFieldValue(receiver Value, field string) (Value, bool) {
	definition, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field)
	if !ok {
		if value, ok := vm.parentRelationshipValue(receiver, field); ok {
			return value, true
		}
		return Null, false
	}
	if defaultValue, ok := storage.DefaultValueForField(fieldDef); ok {
		return vmValueFromStorage(defaultValue), true
	}
	if fieldDef.Type == storage.FieldCalculated {
		if strings.TrimSpace(fieldDef.Formula) != "" {
			if record, ok := vm.formulaRecordFromSObject(receiver); ok {
				if value, _, ok := dml.EvaluateRecordFormulaValueInOrg(fieldDef.Formula, fieldDef, vm.Org, definition, record); ok {
					return vmValueFromStorage(value), true
				}
			}
		}
		switch strings.ToUpper(fieldDef.DisplayType) {
		case "INTEGER":
			return Int(0), true
		case "DECIMAL", "DOUBLE", "CURRENCY", "PERCENT":
			return Decimal(0), true
		case "BOOLEAN":
			return Bool(false), true
		default:
			return Null, true
		}
	}
	if fieldDef.Type == storage.FieldSummary {
		if value, ok := vm.evaluateSummaryField(receiver, fieldDef); ok {
			return vmValueFromStorage(value), true
		}
		return Null, true
	}
	if fieldDef.Type == storage.FieldBoolean && !storage.IsCustomMetadataDefinition(definition) && !storage.IsCustomSettingDefinition(definition) {
		return Bool(false), true
	}
	return Null, true
}

func (vm *VM) parentRelationshipValue(receiver Value, relationshipName string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) {
			continue
		}
		_, lookupValue, ok := objectFieldValue(receiver, relation.Field)
		if !ok || lookupValue.Kind == ValueNull {
			return Null, true
		}
		lookupID, ok := sObjectIDFromValue(lookupValue)
		if !ok || lookupID == "" {
			return Null, true
		}
		for _, parentName := range relation.ParentObjects {
			parentObject, ok := storage.ResolveObjectName(*vm.Org, parentName)
			if !ok {
				parentObject = parentName
			}
			if record, ok := vm.findOrgRecord(parentObject, lookupID); ok {
				return vm.vmValueFromRecord(record), true
			}
			parent := Object(parentObject)
			parent.Fields["Id"] = String(string(lookupID))
			return parent, true
		}
		return Null, true
	}
	return Null, false
}

func (vm *VM) findOrgRecord(objectName string, id storage.ID) (storage.Record, bool) {
	if vm == nil || vm.Org == nil || id == "" {
		return storage.Record{}, false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return storage.Record{}, false
	}
	if record, ok := object.Records[id]; ok {
		return record, true
	}
	for candidateID, record := range object.Records {
		if apexIDTextEqual(string(candidateID), string(id)) {
			return record, true
		}
	}
	return storage.Record{}, false
}

func (vm *VM) formulaRecordFromSObject(value Value) (storage.Record, bool) {
	record, err := vm.recordFromValue(&value)
	if err != nil {
		return storage.Record{}, false
	}
	if vm.Org == nil || record.ID == "" {
		return record, true
	}
	objectName, ok := storage.ResolveObjectName(*vm.Org, record.Object)
	if !ok {
		return record, true
	}
	if persisted, ok := vm.Org.Objects[objectName].Records[record.ID]; ok {
		for field, fieldValue := range persisted.Fields {
			if _, exists := record.Fields[field]; !exists && !record.ExplicitNulls[field] {
				record.Fields[field] = fieldValue.Clone()
			}
		}
	}
	return record, true
}

func (vm *VM) evaluateSummaryField(receiver Value, fieldDef storage.Field) (storage.Value, bool) {
	if vm.Org == nil || !strings.EqualFold(fieldDef.SummaryOperation, "sum") {
		return storage.Value{}, false
	}
	parent, ok := vm.formulaRecordFromSObject(receiver)
	if !ok || parent.ID == "" {
		return storage.Value{}, false
	}
	childObject, childField := splitQualifiedField(fieldDef.SummarizedField)
	fkObject, fkField := splitQualifiedField(fieldDef.SummaryForeignKey)
	if childObject == "" || childField == "" || fkObject == "" || fkField == "" || !strings.EqualFold(childObject, fkObject) {
		return storage.Value{}, false
	}
	canonicalChild, ok := storage.ResolveObjectName(*vm.Org, childObject)
	if !ok {
		return storage.Value{}, false
	}
	childState := vm.Org.Objects[canonicalChild]
	childFieldName, ok := storage.ResolveFieldName(childState.Definition, vm.Org.Namespace, childField)
	if !ok {
		return storage.Value{}, false
	}
	fkFieldName, ok := storage.ResolveFieldName(childState.Definition, vm.Org.Namespace, fkField)
	if !ok {
		return storage.Value{}, false
	}
	total := 0.0
	matched := false
	for _, child := range childState.Records {
		if !apexIDTextEqual(storageValueIDText(child.Fields[fkFieldName]), string(parent.ID)) {
			continue
		}
		if !vm.summaryFiltersMatch(childState.Definition, child, fieldDef.SummaryFilterItems) {
			continue
		}
		value, ok := vm.summaryRecordFieldValue(childState.Definition, child, childFieldName)
		if !ok {
			continue
		}
		number, ok := storageNumericValue(value)
		if !ok {
			continue
		}
		total += number
		matched = true
	}
	if !matched {
		return storage.DecimalValue("0"), true
	}
	return storage.DecimalValue(strconv.FormatFloat(total, 'f', -1, 64)), true
}

func splitQualifiedField(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return "", name
	}
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func (vm *VM) summaryFiltersMatch(definition storage.ObjectDefinition, record storage.Record, filters []storage.SummaryFilterItem) bool {
	for _, filter := range filters {
		_, fieldName := splitQualifiedField(filter.Field)
		if fieldName == "" {
			fieldName = filter.Field
		}
		canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
		if !ok {
			return false
		}
		value, ok := vm.summaryRecordFieldValue(definition, record, canonical)
		if !ok {
			value = storage.NullValue()
		}
		if !summaryFilterMatches(value, filter) {
			return false
		}
	}
	return true
}

func (vm *VM) summaryRecordFieldValue(definition storage.ObjectDefinition, record storage.Record, fieldName string) (storage.Value, bool) {
	if value, ok := record.Fields[fieldName]; ok {
		return value, true
	}
	fieldDef, ok := definition.Fields[fieldName]
	if !ok || fieldDef.Type != storage.FieldCalculated || strings.TrimSpace(fieldDef.Formula) == "" {
		return storage.Value{}, false
	}
	value, _, ok := dml.EvaluateRecordFormulaValueInOrg(fieldDef.Formula, fieldDef, vm.Org, definition, record)
	return value, ok
}

func summaryFilterMatches(value storage.Value, filter storage.SummaryFilterItem) bool {
	switch strings.ToLower(strings.TrimSpace(filter.Operation)) {
	case "", "equals":
		return storageValueMatchesText(value, filter.Value)
	default:
		return false
	}
}

func storageValueMatchesText(value storage.Value, text string) bool {
	text = strings.TrimSpace(text)
	switch value.Kind {
	case storage.ValueBoolean:
		return strings.EqualFold(strconv.FormatBool(value.Boolean), text)
	case storage.ValueString:
		return strings.EqualFold(value.String, text)
	case storage.ValueID:
		return apexIDTextEqual(string(value.ID), text)
	case storage.ValueInteger:
		parsed, err := strconv.ParseInt(text, 10, 64)
		return err == nil && value.Integer == parsed
	case storage.ValueDecimal:
		return strings.TrimRight(strings.TrimRight(value.Decimal, "0"), ".") == strings.TrimRight(strings.TrimRight(text, "0"), ".")
	case storage.ValueNull:
		return strings.EqualFold(text, "null") || text == ""
	default:
		return false
	}
}

func storageNumericValue(value storage.Value) (float64, bool) {
	switch value.Kind {
	case storage.ValueInteger:
		return float64(value.Integer), true
	case storage.ValueDecimal:
		parsed, err := strconv.ParseFloat(value.Decimal, 64)
		return parsed, err == nil
	case storage.ValueString:
		parsed, err := strconv.ParseFloat(value.String, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (vm *VM) sObjectFieldArg(receiverType string, value Value) (string, error) {
	if value.Kind == ValueString {
		return value.Text, nil
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Schema.SObjectField") {
		if objectValue, ok := value.Fields["object"]; ok && objectValue.Kind == ValueString && receiverType != "" && !strings.EqualFold(receiverType, "SObject") {
			if vm.Org != nil {
				if receiverObject, ok := storage.ResolveObjectName(*vm.Org, receiverType); ok {
					if tokenObject, ok := storage.ResolveObjectName(*vm.Org, objectValue.Text); ok && tokenObject != receiverObject {
						return "", fmt.Errorf("%w: field token belongs to %s, not %s", errSObjectFieldTokenWrongObject, objectValue.Text, receiverType)
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
	if value.Kind == ValueNull && strings.EqualFold(value.Type, "Schema.SObjectField") {
		return "", errSObjectFieldTokenNull
	}
	return "", fmt.Errorf("expected field name")
}

var errSObjectFieldTokenWrongObject = errors.New("field token belongs to another SObject type")
var errSObjectFieldTokenNull = errors.New("field token is null")

func apexPagesSeverityStaticValue(name string) (Value, bool) {
	prefix := "ApexPages.Severity."
	if len(name) < len(prefix) || !strings.EqualFold(name[:len(prefix)], prefix) {
		return Null, false
	}
	severity := name[len(prefix):]
	for i, candidate := range apexPagesSeverityNames {
		if strings.EqualFold(severity, candidate) {
			return Value{Kind: ValueObject, Type: "ApexPages.Severity", Text: candidate, Fields: map[string]Value{"ordinal": Int(int64(i))}}, true
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

func metadataDeployStatusStaticValue(name string) (Value, bool) {
	return namedEnumStaticValue("Metadata.DeployStatus", metadataDeployStatusNames, name)
}

func metadataMetadataTypeStaticValue(name string) (Value, bool) {
	return namedEnumStaticValue("Metadata.MetadataType", metadataMetadataTypeNames, name)
}

func soapTypeForStorageField(field storage.Field) string {
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		return "ID"
	case storage.FieldBoolean:
		return "BOOLEAN"
	case storage.FieldInteger:
		return "INTEGER"
	case storage.FieldDecimal:
		return "DOUBLE"
	case storage.FieldDate:
		return "DATE"
	case storage.FieldDateTime:
		return "DATETIME"
	case storage.FieldBlob:
		return "BASE64BINARY"
	default:
		return "STRING"
	}
}

var schemaSOAPTypeNames = []string{"ID", "STRING", "BOOLEAN", "INTEGER", "DOUBLE", "DATE", "DATETIME", "TIME", "BASE64BINARY", "ANYTYPE"}
var schemaDisplayTypeNames = []string{"BOOLEAN", "CURRENCY", "DATE", "DATETIME", "DOUBLE", "ID", "INTEGER", "PERCENT", "PICKLIST", "REFERENCE", "STRING", "TEXTAREA"}

func schemaSOAPTypeStaticValue(name string) (Value, bool) {
	if value, ok := namedEnumStaticValue("Schema.SOAPType", schemaSOAPTypeNames, name); ok {
		return value, true
	}
	if strings.HasPrefix(name, "Schema.SoapType.") {
		return namedEnumStaticValue("Schema.SOAPType", schemaSOAPTypeNames, "Schema.SOAPType."+strings.TrimPrefix(name, "Schema.SoapType."))
	}
	return Null, false
}

func schemaSOAPTypeValue(name string) Value {
	value, _ := namedEnumStaticValue("Schema.SOAPType", schemaSOAPTypeNames, "Schema.SOAPType."+name)
	return value
}

func schemaDisplayTypeStaticValue(name string) (Value, bool) {
	return namedEnumStaticValue("Schema.DisplayType", schemaDisplayTypeNames, name)
}

func schemaDisplayTypeValue(name string) Value {
	value, ok := namedEnumStaticValue("Schema.DisplayType", schemaDisplayTypeNames, "Schema.DisplayType."+name)
	if ok {
		return value
	}
	return Value{Kind: ValueObject, Type: "Schema.DisplayType", Text: name}
}

func namedEnumStaticValue(typeName string, names []string, name string) (Value, bool) {
	prefix := typeName + "."
	if !strings.HasPrefix(name, prefix) {
		return Null, false
	}
	member := strings.TrimPrefix(name, prefix)
	for i, candidate := range names {
		if member == candidate {
			return Value{Kind: ValueObject, Type: typeName, Text: member, Fields: map[string]Value{"ordinal": Int(int64(i))}}, true
		}
	}
	return Null, false
}

func metadataDeployStatusValues(args []Value) (Value, error) {
	return namedEnumValues("Metadata.DeployStatus", metadataDeployStatusNames, args)
}

func metadataMetadataTypeValues(args []Value) (Value, error) {
	return namedEnumValues("Metadata.MetadataType", metadataMetadataTypeNames, args)
}

func (vm *VM) metadataEnqueueDeployment(args []Value, result *Result) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueObject || args[0].Type != "Metadata.DeployContainer" {
		return Null, fmt.Errorf("Metadata.Operations.enqueueDeployment expects DeployContainer and DeployCallback")
	}
	if args[1].Kind != ValueNull {
		return Null, unsupportedCallError("Metadata.Operations.enqueueDeployment deploy callback invocation")
	}
	deploymentID := "0Af000000000001"
	items := args[0].Fields["metadata"]
	if items.Kind == ValueNull || (items.Kind == ValueList && len(items.List) == 0) {
		vm.recordMetadataDeployment(deploymentID, nil)
		appendTrace(result, "apex.metadata.deploy.enqueue", "apex.metadata", map[string]any{
			"deploymentId": deploymentID,
			"components":   0,
			"success":      true,
		})
		return platformScalar("Id", deploymentID), nil
	}
	if items.Kind != ValueList {
		return Null, fmt.Errorf("Metadata.DeployContainer.metadata must be a list")
	}
	if vm.Org == nil {
		return Null, unsupportedCallError("Metadata.Operations.enqueueDeployment requires org storage for local metadata mutation")
	}
	originalOrg := vm.Org
	candidateOrg := originalOrg.Clone()
	vm.Org = &candidateOrg
	for _, item := range items.List {
		if err := vm.applyMetadataDeployment(item); err != nil {
			vm.Org = originalOrg
			var runtimeErr *RuntimeError
			if errors.As(err, &runtimeErr) && runtimeErr.Type == "UnsupportedFeature" {
				return Null, err
			}
			vm.recordMetadataDeploymentFailure(deploymentID, items.List, item, err)
			appendTrace(result, "apex.metadata.deploy.enqueue", "apex.metadata", map[string]any{
				"deploymentId": deploymentID,
				"components":   len(items.List),
				"success":      false,
				"error":        err.Error(),
			})
			return platformScalar("Id", deploymentID), nil
		}
	}
	*originalOrg = candidateOrg
	vm.Org = originalOrg
	vm.recordMetadataDeployment(deploymentID, items.List)
	appendTrace(result, "apex.metadata.deploy.enqueue", "apex.metadata", map[string]any{
		"deploymentId": deploymentID,
		"components":   len(items.List),
		"success":      true,
	})
	return platformScalar("Id", deploymentID), nil
}

func (vm *VM) metadataCheckDeployStatus(args []Value, result *Result) (Value, error) {
	if len(args) < 1 || len(args) > 2 || !metadataDeploymentIDValue(args[0]) {
		return Null, fmt.Errorf("Metadata.Operations.checkDeployStatus expects deployment Id[, includeDetails]")
	}
	includeDetails := false
	if len(args) == 2 {
		if args[1].Kind != ValueBool {
			return Null, fmt.Errorf("Metadata.Operations.checkDeployStatus includeDetails expects Boolean")
		}
		includeDetails = args[1].Bool
	}
	deploymentID := args[0].Text
	if args[0].Kind == ValueObject {
		var err error
		deploymentID, err = platformScalarText(args[0], "Id")
		if err != nil {
			return Null, err
		}
	}
	if vm.metadataDeploys == nil {
		vm.metadataDeploys = make(map[string]Value)
	}
	storedResult, ok := vm.metadataDeploys[deploymentID]
	if !ok {
		return Null, unsupportedCallError("Metadata.Operations.checkDeployStatus unknown local deployment " + deploymentID)
	}
	deployResult := cloneMetadataDeployResult(storedResult)
	if !includeDetails {
		deployResult.Fields["details"] = Null
	}
	appendTrace(result, "apex.metadata.deploy.status", "apex.metadata", map[string]any{
		"deploymentId":   deploymentID,
		"includeDetails": includeDetails,
		"success":        deployResult.Fields["success"].Bool,
		"status":         deployResult.Fields["status"].Text,
	})
	return deployResult, nil
}

func metadataDeploymentIDValue(value Value) bool {
	return value.Kind == ValueString || (value.Kind == ValueObject && value.Type == "Id")
}

func (vm *VM) recordMetadataDeployment(deploymentID string, items []Value) {
	if vm.metadataDeploys == nil {
		vm.metadataDeploys = make(map[string]Value)
	}
	vm.metadataDeploys[deploymentID] = metadataDeployResultObject(deploymentID, items)
}

func (vm *VM) recordMetadataDeploymentFailure(deploymentID string, items []Value, failedItem Value, err error) {
	if vm.metadataDeploys == nil {
		vm.metadataDeploys = make(map[string]Value)
	}
	vm.metadataDeploys[deploymentID] = metadataDeployFailureResultObject(deploymentID, items, failedItem, err)
}

func (vm *VM) applyMetadataDeployment(item Value) error {
	if item.Kind != ValueObject {
		return unsupportedCallError("Metadata.Operations.enqueueDeployment " + string(item.Kind) + " metadata deploy")
	}
	switch item.Type {
	case "Metadata.CustomMetadata":
		return vm.applyCustomMetadataDeployment(item)
	case "Metadata.CustomObject":
		return vm.applyCustomObjectDeployment(item)
	case "Metadata.CustomField":
		return vm.applyCustomFieldDeployment(item)
	default:
		typeName := item.Type
		if typeName == "" {
			typeName = string(item.Kind)
		}
		return unsupportedCallError("Metadata.Operations.enqueueDeployment " + typeName + " metadata deploy")
	}
}

func (vm *VM) applyCustomMetadataDeployment(item Value) error {
	fullName, ok := metadataStringField(item, "fullName")
	if !ok || strings.TrimSpace(fullName) == "" {
		return fmt.Errorf("Metadata.CustomMetadata.fullName is required")
	}
	objectName, developerName := metadataCustomMetadataNames(fullName)
	if objectName == "" || developerName == "" {
		return fmt.Errorf("Metadata.CustomMetadata.fullName must be Type.Record")
	}
	state := vm.metadataCustomMetadataState(objectName)
	definition := state.Definition
	recordFields := map[string]storage.Value{
		"DeveloperName":    storage.StringValue(developerName),
		"MasterLabel":      storage.StringValue(metadataLabelOrDefault(item, developerName)),
		"Label":            storage.StringValue(metadataLabelOrDefault(item, developerName)),
		"NamespacePrefix":  storage.StringValue(metadataNamespacePrefix(vm.Org.Namespace, definition.APIName)),
		"QualifiedApiName": storage.StringValue(metadataQualifiedAPIName(vm.Org.Namespace, definition.APIName, developerName)),
	}
	values := item.Fields["values"]
	if values.Kind != ValueNull {
		if values.Kind != ValueList {
			return fmt.Errorf("Metadata.CustomMetadata.values must be a list")
		}
		for _, valueItem := range values.List {
			fieldName, fieldValue, err := vm.metadataCustomMetadataValue(definition, valueItem)
			if err != nil {
				return err
			}
			recordFields[fieldName] = fieldValue
		}
	}
	var recordID storage.ID
	for _, existing := range state.Records {
		if customDataRecordMatches(definition, "custom metadata", existing, developerName, vm.Org.Namespace) ||
			customDataRecordMatches(definition, "custom metadata", existing, fullName, vm.Org.Namespace) {
			recordID = existing.ID
			break
		}
	}
	if recordID == "" {
		recordID = nextMetadataRecordID(state)
	}
	record := storage.Record{ID: recordID, Object: definition.APIName, Fields: recordFields}
	record.Fields["Id"] = storage.IDValue(recordID)
	state.Records[recordID] = record
	vm.Org.Objects[definition.APIName] = state
	return nil
}

func (vm *VM) applyCustomObjectDeployment(item Value) error {
	fullName, ok := metadataStringField(item, "fullName")
	if !ok || strings.TrimSpace(fullName) == "" {
		return fmt.Errorf("Metadata.CustomObject.fullName is required")
	}
	objectName := strings.TrimSpace(fullName)
	if !strings.HasSuffix(objectName, "__c") && !strings.HasSuffix(objectName, "__mdt") && !strings.HasSuffix(objectName, "__e") {
		return fmt.Errorf("Metadata.CustomObject.fullName must be a custom object API name")
	}
	objectName = storage.NamespaceTokenName(vm.Org.Namespace, objectName)
	state := vm.Org.Objects[objectName]
	state.Definition.APIName = objectName
	if state.Definition.Label == "" {
		state.Definition.Label = metadataTextFieldOrDefault(item, "label", strings.TrimSuffix(objectName, "__c"))
	}
	if state.Definition.PluralLabel == "" {
		state.Definition.PluralLabel = metadataTextFieldOrDefault(item, "pluralLabel", state.Definition.Label+"s")
	}
	if state.Definition.SharingModel == "" {
		state.Definition.SharingModel = metadataTextFieldOrDefault(item, "sharingModel", "ReadWrite")
	}
	if state.Definition.KeyPrefix == "" {
		state.Definition.KeyPrefix = storage.AssignDeterministicPrefixes([]string{objectName}, nil)[objectName]
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	if _, ok := state.Definition.Fields["Name"]; !ok {
		state.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Name", Type: storage.FieldString}
	}
	if state.Definition.Metadata == nil {
		state.Definition.Metadata = map[string]string{"kind": "customObject"}
	}
	storage.EnsureStandardObjectFields(&state.Definition)
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	vm.Org.Objects[objectName] = state
	return nil
}

func (vm *VM) applyCustomFieldDeployment(item Value) error {
	fullName, ok := metadataStringField(item, "fullName")
	if !ok || strings.TrimSpace(fullName) == "" {
		return fmt.Errorf("Metadata.CustomField.fullName is required")
	}
	objectName, fieldName := metadataCustomFieldNames(fullName)
	if objectName == "" || fieldName == "" {
		return fmt.Errorf("Metadata.CustomField.fullName must be Object.Field")
	}
	objectName = storage.NamespaceTokenName(vm.Org.Namespace, objectName)
	fieldName = storage.NamespaceTokenName(vm.Org.Namespace, fieldName)
	state, ok := vm.Org.Objects[objectName]
	if !ok {
		return fmt.Errorf("Metadata.CustomField.fullName references unknown object %s", objectName)
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	fieldType, displayType := metadataCustomFieldType(item)
	field := state.Definition.Fields[fieldName]
	field.APIName = fieldName
	field.Label = metadataTextFieldOrDefault(item, "label", fieldName)
	field.Type = fieldType
	field.DisplayType = displayType
	field.Required = metadataBoolField(item, "required")
	field.ExternalID = metadataBoolField(item, "externalId")
	field.Unique = metadataBoolField(item, "unique")
	if referenceTo := metadataReferenceTo(item); len(referenceTo) > 0 {
		field.ReferenceTo = referenceTo
	}
	state.Definition.Fields[fieldName] = field
	vm.Org.Objects[objectName] = state
	return nil
}

func (vm *VM) metadataCustomMetadataState(objectName string) storage.ObjectState {
	state := vm.Org.Objects[objectName]
	if state.Definition.APIName == "" {
		state.Definition.APIName = objectName
	}
	if state.Definition.KeyPrefix == "" {
		state.Definition.KeyPrefix = storage.AssignDeterministicPrefixes([]string{objectName}, nil)[objectName]
	}
	if state.Definition.Metadata == nil {
		state.Definition.Metadata = map[string]string{"kind": "customMetadata"}
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	for _, field := range []storage.Field{
		{APIName: "DeveloperName", Type: storage.FieldString},
		{APIName: "MasterLabel", Type: storage.FieldString},
		{APIName: "Label", Type: storage.FieldString},
		{APIName: "NamespacePrefix", Type: storage.FieldString},
		{APIName: "QualifiedApiName", Type: storage.FieldString},
	} {
		if _, ok := state.Definition.Fields[field.APIName]; !ok {
			state.Definition.Fields[field.APIName] = field
		}
	}
	storage.EnsureStandardObjectFields(&state.Definition)
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	vm.Org.Objects[objectName] = state
	return state
}

func (vm *VM) metadataCustomMetadataValue(definition storage.ObjectDefinition, item Value) (string, storage.Value, error) {
	if item.Kind != ValueObject || item.Type != "Metadata.CustomMetadataValue" {
		return "", storage.Value{}, fmt.Errorf("Metadata.CustomMetadata.values expects CustomMetadataValue entries")
	}
	fieldName, ok := metadataStringField(item, "field")
	if !ok || strings.TrimSpace(fieldName) == "" {
		return "", storage.Value{}, fmt.Errorf("Metadata.CustomMetadataValue.field is required")
	}
	resolved, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		value := item.Fields["value"]
		fieldType := metadataFieldTypeFromValue(value)
		resolved = storage.NamespaceTokenName(vm.Org.Namespace, fieldName)
		definition.Fields[resolved] = storage.Field{APIName: resolved, Type: fieldType}
		if state, exists := vm.Org.Objects[definition.APIName]; exists {
			state.Definition.Fields[resolved] = definition.Fields[resolved]
			vm.Org.Objects[definition.APIName] = state
		}
	}
	converted, err := storageValueFromVMForField(item.Fields["value"], definition.Fields[resolved].Type)
	if err != nil {
		return "", storage.Value{}, fmt.Errorf("Metadata.CustomMetadataValue.%s %v", fieldName, err)
	}
	return resolved, converted, nil
}

func metadataFieldTypeFromValue(value Value) storage.FieldType {
	switch value.Kind {
	case ValueBool:
		return storage.FieldBoolean
	case ValueInt:
		return storage.FieldInteger
	case ValueDecimal:
		return storage.FieldDecimal
	case ValueObject:
		switch value.Type {
		case "Date":
			return storage.FieldDate
		case "Datetime", "DateTime":
			return storage.FieldDateTime
		case "Id":
			return storage.FieldID
		}
	}
	return storage.FieldString
}

func (vm *VM) metadataRetrieve(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return Null, fmt.Errorf("Metadata.Operations.retrieve expects metadata type and full names")
	}
	if args[0].Kind != ValueObject || args[0].Type != "Metadata.MetadataType" {
		return Null, fmt.Errorf("Metadata.Operations.retrieve expects metadata type")
	}
	if !strings.EqualFold(args[0].Text, "CustomMetadata") {
		return Null, unsupportedCallError("Metadata.Operations.retrieve " + args[0].Text)
	}
	names, err := metadataStringList(args[1])
	if err != nil {
		return Null, err
	}
	if vm.Org == nil {
		return List(), nil
	}
	out := make([]Value, 0, len(names))
	for _, fullName := range names {
		objectName, developerName := metadataCustomMetadataNames(fullName)
		objectName, ok := storage.ResolveObjectName(*vm.Org, objectName)
		if !ok {
			continue
		}
		state := vm.Org.Objects[objectName]
		if !storage.IsCustomMetadataDefinition(state.Definition) {
			continue
		}
		for _, record := range sortedCustomDataRecords(state.Records, state.Definition, "custom metadata", vm.Org.Namespace) {
			if record.System.IsDeleted {
				continue
			}
			if customDataRecordMatches(state.Definition, "custom metadata", record, developerName, vm.Org.Namespace) ||
				customDataRecordMatches(state.Definition, "custom metadata", record, fullName, vm.Org.Namespace) {
				out = append(out, metadataCustomMetadataObject(state.Definition, record))
				break
			}
		}
	}
	return List(out...), nil
}

func metadataCustomMetadataObject(definition storage.ObjectDefinition, record storage.Record) Value {
	item := Object("Metadata.CustomMetadata")
	developerName := firstStringField(record, "DeveloperName", "Name")
	fullName := strings.TrimSuffix(definition.APIName, "__mdt") + "." + developerName
	item.Fields["fullName"] = String(fullName)
	item.Fields["label"] = String(firstStringField(record, "MasterLabel", "Label", "DeveloperName", "Name"))
	values := make([]Value, 0, len(record.Fields))
	for fieldName, fieldValue := range record.Fields {
		if isCustomMetadataSystemField(fieldName) {
			continue
		}
		value := Object("Metadata.CustomMetadataValue")
		value.Fields["field"] = String(fieldName)
		value.Fields["value"] = vmValueFromStorage(fieldValue)
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].Fields["field"].Text < values[j].Fields["field"].Text
	})
	item.Fields["values"] = List(values...)
	return item
}

func isCustomMetadataSystemField(fieldName string) bool {
	switch fieldName {
	case "Id", "DeveloperName", "MasterLabel", "Label", "NamespacePrefix", "QualifiedApiName", "Name":
		return true
	default:
		return false
	}
}

func metadataStringField(value Value, field string) (string, bool) {
	raw, ok := value.Fields[field]
	if !ok || raw.Kind != ValueString {
		return "", false
	}
	return raw.Text, true
}

func metadataTextFieldOrDefault(value Value, field, fallback string) string {
	if raw, ok := metadataStringField(value, field); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return fallback
}

func metadataBoolField(value Value, field string) bool {
	raw, ok := value.Fields[field]
	return ok && raw.Kind == ValueBool && raw.Bool
}

func metadataCustomFieldNames(fullName string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(fullName), ".", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(fullName)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func metadataCustomFieldType(item Value) (storage.FieldType, string) {
	raw := metadataTextFieldOrDefault(item, "type", "Text")
	switch strings.ToLower(strings.ReplaceAll(raw, "_", "")) {
	case "checkbox", "boolean":
		return storage.FieldBoolean, "BOOLEAN"
	case "number", "integer", "int":
		return storage.FieldInteger, "INTEGER"
	case "currency", "percent", "double", "decimal":
		return storage.FieldDecimal, "DOUBLE"
	case "date":
		return storage.FieldDate, "DATE"
	case "datetime":
		return storage.FieldDateTime, "DATETIME"
	case "picklist", "multipicklist":
		return storage.FieldPicklist, "PICKLIST"
	case "lookup", "masterdetail", "reference":
		return storage.FieldReference, "REFERENCE"
	case "textarea", "longtextarea", "html", "email", "phone", "url", "text":
		return storage.FieldString, "STRING"
	default:
		return storage.FieldString, strings.ToUpper(raw)
	}
}

func metadataReferenceTo(item Value) []string {
	raw, ok := item.Fields["referenceTo"]
	if !ok || raw.Kind == ValueNull {
		return nil
	}
	switch raw.Kind {
	case ValueString:
		if strings.TrimSpace(raw.Text) == "" {
			return nil
		}
		return []string{strings.TrimSpace(raw.Text)}
	case ValueList, ValueSet:
		items := raw.List
		if raw.Kind == ValueSet {
			items = raw.Set
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			if item.Kind == ValueString && strings.TrimSpace(item.Text) != "" {
				out = append(out, strings.TrimSpace(item.Text))
			}
		}
		return out
	default:
		return nil
	}
}

func metadataStringList(value Value) ([]string, error) {
	if value.Kind != ValueList && value.Kind != ValueSet {
		return nil, fmt.Errorf("Metadata.Operations.retrieve expects full names list")
	}
	items := value.List
	if value.Kind == ValueSet {
		items = value.Set
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Kind != ValueString {
			return nil, fmt.Errorf("Metadata.Operations.retrieve expects String full names")
		}
		out = append(out, item.Text)
	}
	return out, nil
}

func metadataCustomMetadataNames(fullName string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(fullName), ".", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(fullName)
	}
	objectName := strings.TrimSpace(parts[0])
	if !strings.HasSuffix(objectName, "__mdt") {
		objectName += "__mdt"
	}
	return objectName, strings.TrimSpace(parts[1])
}

func metadataLabelOrDefault(item Value, developerName string) string {
	if label, ok := metadataStringField(item, "label"); ok && strings.TrimSpace(label) != "" {
		return label
	}
	return developerName
}

func metadataNamespacePrefix(namespace, objectName string) string {
	if namespace == "" {
		return ""
	}
	if strings.HasPrefix(objectName, namespace+"__") {
		return namespace
	}
	return ""
}

func metadataQualifiedAPIName(namespace, objectName, developerName string) string {
	if metadataNamespacePrefix(namespace, objectName) != "" {
		return namespace + "__" + developerName
	}
	return developerName
}

func nextMetadataRecordID(state storage.ObjectState) storage.ID {
	generator := storage.NewIDGenerator(map[string]string{state.Definition.APIName: state.Definition.KeyPrefix})
	for {
		id, err := generator.Next(state.Definition.APIName)
		if err != nil {
			return storage.ID(state.Definition.KeyPrefix + "000000000001")
		}
		if _, exists := state.Records[id]; !exists {
			return id
		}
	}
}

func namedEnumValues(typeName string, names []string, args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("%s.values expects 0 arguments", typeName)
	}
	values := make([]Value, 0, len(names))
	for i, name := range names {
		value := Value{Kind: ValueObject, Type: typeName, Text: name}
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
	if typeName == "Metadata.DeployStatus" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := metadataDeployStatusValues(args)
		return value, true, err
	}
	if typeName == "Metadata.MetadataType" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := metadataMetadataTypeValues(args)
		return value, true, err
	}
	class, ok := vm.Classes[typeName]
	if !ok || len(class.EnumValues) == 0 {
		return Null, false, nil
	}
	if err := vm.ensureClassInitialized(class.Name); err != nil {
		return Null, true, err
	}
	class, _ = vm.lookupClass(class.Name)
	switch method {
	case "values":
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
	case "valueOf":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("%s.valueOf expects String", typeName)
		}
		for i, name := range class.EnumValues {
			if name == args[0].Text {
				value := Value{Kind: ValueObject, Type: class.Name, Text: name}
				value.Fields = map[string]Value{"ordinal": Int(int64(i))}
				return value, true, nil
			}
		}
		return Null, true, fmt.Errorf("No enum constant %s.%s", typeName, args[0].Text)
	default:
		return Null, false, nil
	}
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
	if receiver.Type == "Metadata.DeployStatus" {
		return callNamedEnumMember("Metadata.DeployStatus", metadataDeployStatusNames, receiver, method, args)
	}
	if receiver.Type == "Metadata.MetadataType" {
		return callNamedEnumMember("Metadata.MetadataType", metadataMetadataTypeNames, receiver, method, args)
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

func metadataDeployDetailsObject() Value {
	details := Object("Metadata.DeployDetails")
	details.Fields["componentFailures"] = List()
	details.Fields["componentSuccesses"] = List()
	details.Fields["runTestResult"] = Null
	return details
}

func metadataDeployResultObject(deploymentID string, items []Value) Value {
	result := Object("Metadata.DeployResult")
	result.Fields["id"] = platformScalar("Id", deploymentID)
	result.Fields["status"] = metadataDeployStatusValue("SUCCEEDED")
	result.Fields["success"] = Bool(true)
	result.Fields["done"] = Bool(true)
	result.Fields["numberComponentErrors"] = Int(0)
	result.Fields["numberComponentsDeployed"] = Int(int64(len(items)))
	result.Fields["numberComponentsTotal"] = Int(int64(len(items)))
	result.Fields["numberTestErrors"] = Int(0)
	result.Fields["numberTestsCompleted"] = Int(0)
	result.Fields["checkOnly"] = Bool(false)
	details := metadataDeployDetailsObject()
	successes := make([]Value, 0, len(items))
	for _, item := range items {
		successes = append(successes, metadataDeploySuccessMessage(item))
	}
	details.Fields["componentSuccesses"] = List(successes...)
	result.Fields["details"] = details
	return result
}

func metadataDeployFailureResultObject(deploymentID string, items []Value, failedItem Value, err error) Value {
	result := Object("Metadata.DeployResult")
	result.Fields["id"] = platformScalar("Id", deploymentID)
	result.Fields["status"] = metadataDeployStatusValue("FAILED")
	result.Fields["success"] = Bool(false)
	result.Fields["done"] = Bool(true)
	result.Fields["numberComponentErrors"] = Int(1)
	result.Fields["numberComponentsDeployed"] = Int(0)
	result.Fields["numberComponentsTotal"] = Int(int64(len(items)))
	result.Fields["numberTestErrors"] = Int(0)
	result.Fields["numberTestsCompleted"] = Int(0)
	result.Fields["checkOnly"] = Bool(false)
	details := metadataDeployDetailsObject()
	details.Fields["componentFailures"] = List(metadataDeployFailureMessage(failedItem, err))
	result.Fields["details"] = details
	return result
}

func metadataDeploySuccessMessage(item Value) Value {
	message := Object("Metadata.DeployMessage")
	fullName := metadataDeployItemFullName(item)
	message.Fields["fullName"] = String(fullName)
	message.Fields["fileName"] = String(fullName)
	message.Fields["componentType"] = String(metadataDeployItemComponentType(item))
	message.Fields["success"] = Bool(true)
	message.Fields["problem"] = Null
	return message
}

func metadataDeployFailureMessage(item Value, err error) Value {
	message := Object("Metadata.DeployMessage")
	fullName := metadataDeployItemFullName(item)
	message.Fields["fullName"] = String(fullName)
	message.Fields["fileName"] = String(fullName)
	message.Fields["componentType"] = String(metadataDeployItemComponentType(item))
	message.Fields["success"] = Bool(false)
	if err == nil {
		message.Fields["problem"] = String("metadata deployment failed")
	} else {
		message.Fields["problem"] = String(err.Error())
	}
	return message
}

func metadataDeployItemFullName(item Value) string {
	if item.Kind == ValueObject {
		if fullName, ok := metadataStringField(item, "fullName"); ok {
			return fullName
		}
	}
	return ""
}

func metadataDeployItemComponentType(item Value) string {
	if item.Kind != ValueObject {
		return string(item.Kind)
	}
	switch item.Type {
	case "Metadata.CustomMetadata":
		return "CustomMetadata"
	case "":
		return string(item.Kind)
	default:
		return strings.TrimPrefix(item.Type, "Metadata.")
	}
}

func metadataAsyncResultObject(id string, done bool, state, message string) Value {
	result := Object("Metadata.AsyncResult")
	result.Fields["id"] = platformScalar("Id", id)
	result.Fields["done"] = Bool(done)
	result.Fields["state"] = String(state)
	result.Fields["statusCode"] = Null
	if message == "" {
		result.Fields["message"] = Null
	} else {
		result.Fields["message"] = String(message)
	}
	return result
}

func metadataDeployStatusValue(name string) Value {
	return Value{Kind: ValueObject, Type: "Metadata.DeployStatus", Text: name, Fields: map[string]Value{"ordinal": Int(metadataDeployStatusOrdinal(name))}}
}

func metadataDeployStatusOrdinal(name string) int64 {
	for i, candidate := range metadataDeployStatusNames {
		if candidate == name {
			return int64(i)
		}
	}
	return -1
}

func cloneMetadataDeployResult(result Value) Value {
	cloned := cloneValue(result)
	if cloned.Fields == nil {
		cloned.Fields = make(map[string]Value)
	}
	return cloned
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
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
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
		case "setMessage":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("%s.setMessage expects 1 argument", receiver.Type)
			}
			if args[0].Kind == ValueString || args[0].Kind == ValueNull {
				receiver.Fields["message"] = args[0]
			} else {
				receiver.Fields["message"] = String(args[0].String())
			}
			return Null, receiver, true, true, nil
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
			return String(exceptionQualifiedTypeName(receiver.Type)), receiver, false, true, nil
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
	case "AggregateResult":
		switch method {
		case "get":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("AggregateResult.get expects String field name")
			}
			if value, ok := receiver.Fields[args[0].Text]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
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
		switch method {
		case "newSObject":
			if len(args) > 2 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.newSObject expects optional Id and loadDefaults")
			}
			if len(args) == 2 && args[1].Kind != ValueBool {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.newSObject loadDefaults expects Boolean")
			}
			objectValue, ok := receiver.Fields["object"]
			if !ok || objectValue.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType token missing object")
			}
			objectName := objectValue.Text
			if vm.Org != nil {
				if canonical, ok := storage.ResolveObjectName(*vm.Org, objectValue.Text); ok {
					objectName = canonical
				}
			}
			record := Object(objectName)
			if len(args) >= 1 && args[0].Kind != ValueNull {
				idText, ok := idTextFromValue(args[0])
				if !ok {
					return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.newSObject recordTypeId expects Id")
				}
				recordID := platformScalar("Id", idText)
				if idObject, ok := vm.sObjectNameForIDPrefix(idPrefix(idText)); ok && strings.EqualFold(idObject, objectName) {
					record.Fields["Id"] = recordID
				} else {
					record.Fields["RecordTypeId"] = recordID
				}
			}
			if len(args) == 2 && args[1].Bool && vm.Org != nil {
				if object, ok := vm.Org.Objects[objectName]; ok {
					if _, exists := record.Fields["RecordTypeId"]; !exists {
						if recordTypeID := defaultRecordTypeID(object.Definition); recordTypeID != "" {
							record.Fields["RecordTypeId"] = platformScalar("Id", string(recordTypeID))
						}
					}
					for name, field := range object.Definition.Fields {
						if _, exists := record.Fields[name]; exists {
							continue
						}
						if defaultValue, ok := storage.DefaultValueForField(field); ok {
							putVMFieldPath(record, name, vmValueFromStorage(defaultValue))
						}
					}
				}
			}
			return record, receiver, false, true, nil
		case "getDescribe":
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
				if strings.EqualFold(objectValue.Text, "AggregateResult") {
					return vm.describeSObjectValue("AggregateResult", storage.ObjectDefinition{APIName: "AggregateResult", Label: "Aggregate Result", PluralLabel: "Aggregate Results"}), receiver, false, true, nil
				}
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.getDescribe unknown object %s", objectValue.Text)
			}
			appendTrace(result, "apex.describe.sobject", "apex.describe", map[string]any{
				"operation": "SObjectType.getDescribe",
				"object":    objectName,
			})
			return vm.describeSObjectValue(objectName, vm.Org.Objects[objectName].Definition), receiver, false, true, nil
		case "getRecordTypeInfosByName", "getRecordTypeInfosById":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.%s expects 0 arguments", method)
			}
			describe, _, _, handled, err := vm.callPlatformObjectMember(receiver, "getDescribe", nil, result)
			if err != nil || !handled {
				return describe, receiver, false, true, err
			}
			value, _, _, handled, err := vm.callPlatformObjectMember(describe, method, nil, result)
			if err != nil || !handled {
				return value, receiver, false, true, err
			}
			return value, receiver, false, true, nil
		}
	case "Schema.SObjectFieldMap":
		if method == "getMap" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectFieldMap.getMap expects 0 arguments")
			}
			appendTrace(result, "apex.describe.fields", "apex.describe", map[string]any{"operation": "fields.getMap"})
			return receiver.Fields["map"], receiver, false, true, nil
		}
	case "Schema.FieldSetMap":
		switch method {
		case "getMap":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMap.getMap expects 0 arguments")
			}
			appendTrace(result, "apex.describe.fieldSets", "apex.describe", map[string]any{"operation": "fieldSets.getMap"})
			return receiver.Fields["map"], receiver, false, true, nil
		case "get":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMap.get expects field set name")
			}
			m, ok := receiver.Fields["map"]
			if !ok || m.Kind != ValueMap {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMap is missing map")
			}
			if value, ok := m.Map[mapKey(args[0])]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
	case "Schema.FieldSet":
		switch method {
		case "getFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getFields expects 0 arguments")
			}
			return receiver.Fields["fields"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		}
	case "Schema.FieldSetMember":
		switch method {
		case "getFieldPath":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getFieldPath expects 0 arguments")
			}
			return receiver.Fields["fieldPath"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getRequired":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getRequired expects 0 arguments")
			}
			return receiver.Fields["required"], receiver, false, true, nil
		case "getDbRequired":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getDbRequired expects 0 arguments")
			}
			return receiver.Fields["dbRequired"], receiver, false, true, nil
		case "getType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getType expects 0 arguments")
			}
			return receiver.Fields["type"], receiver, false, true, nil
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
		switch method {
		case "getName", "getLabel", "getType", "getSOAPType", "getSoapType", "isNillable", "isExternalId", "isUnique", "isEncrypted", "isCalculated", "isNameField", "isCustom", "getReferenceTo", "getRelationshipName", "getPicklistValues", "getController", "getControllerValues", "isAccessible", "isCreateable", "isUpdateable":
			describe, _, _, handled, err := vm.callPlatformObjectMember(receiver, "getDescribe", nil, result)
			if err != nil || !handled {
				return describe, receiver, false, true, err
			}
			value, _, _, handled, err := vm.callPlatformObjectMember(describe, method, args, result)
			if err != nil || !handled {
				return value, receiver, false, true, err
			}
			return value, receiver, false, true, nil
		}
	case "Schema.DescribeFieldResult":
		switch method {
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getSObjectField":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getSObjectField expects 0 arguments")
			}
			objectName := ""
			if value, ok := receiver.Fields["sObjectName"]; ok && value.Kind == ValueString {
				objectName = value.Text
			}
			fieldName := ""
			if value, ok := receiver.Fields["name"]; ok && value.Kind == ValueString {
				fieldName = value.Text
			}
			if objectName == "" || fieldName == "" {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult token missing object or field")
			}
			return sObjectFieldToken(objectName, fieldName), receiver, false, true, nil
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
		case "getSOAPType", "getSoapType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			return receiver.Fields["soapType"], receiver, false, true, nil
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
		case "isEncrypted":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isEncrypted expects 0 arguments")
			}
			return receiver.Fields["encrypted"], receiver, false, true, nil
		case "isCalculated":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isCalculated expects 0 arguments")
			}
			return receiver.Fields["calculated"], receiver, false, true, nil
		case "isNameField":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isNameField expects 0 arguments")
			}
			return receiver.Fields["nameField"], receiver, false, true, nil
		case "isCustom":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isCustom expects 0 arguments")
			}
			return receiver.Fields["custom"], receiver, false, true, nil
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
			objectName := ""
			if value, ok := receiver.Fields["sObjectName"]; ok && value.Kind == ValueString {
				objectName = value.Text
			}
			fieldName := ""
			if value, ok := receiver.Fields["name"]; ok && value.Kind == ValueString {
				fieldName = value.Text
			}
			if objectName == "" || fieldName == "" {
				return Bool(true), receiver, false, true, nil
			}
			return Bool(vm.currentUserFieldPermission(objectName, fieldName, method)), receiver, false, true, nil
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
	case "Schema.DescribeTabSetResult":
		switch method {
		case "getTabs":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getTabs expects 0 arguments")
			}
			return receiver.Fields["tabs"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "isSelected":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.isSelected expects 0 arguments")
			}
			return receiver.Fields["selected"], receiver, false, true, nil
		}
	case "Schema.DescribeTabResult":
		switch method {
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getSObjectName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getSObjectName expects 0 arguments")
			}
			return receiver.Fields["sObjectName"], receiver, false, true, nil
		case "isCustom":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.isCustom expects 0 arguments")
			}
			return receiver.Fields["custom"], receiver, false, true, nil
		case "getIconUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getIconUrl expects 0 arguments")
			}
			return receiver.Fields["iconUrl"], receiver, false, true, nil
		case "getUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getUrl expects 0 arguments")
			}
			return receiver.Fields["url"], receiver, false, true, nil
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
		case "monthsBetween":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Date" {
				return Null, receiver, false, true, fmt.Errorf("Date.monthsBetween expects Date")
			}
			start, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			end, err := parsePlatformDate(args[0])
			if err != nil {
				return Null, receiver, false, true, err
			}
			months := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month())
			return Int(int64(months)), receiver, false, true, nil
		case "year", "month", "day", "Year", "Month", "Day":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects 0 arguments", method)
			}
			date, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			switch strings.ToLower(method) {
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
		case "getTime":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.getTime expects 0 arguments")
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return Int(t.UnixNano() / int64(time.Millisecond)), receiver, false, true, nil
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
		if value, handled, err := vm.callIdMember(receiver, method, args); handled || err != nil {
			return value, receiver, false, true, err
		}
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
		case "getRecordTypeInfosById":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfosById expects 0 arguments")
			}
			return receiver.Fields["recordTypeInfosById"], receiver, false, true, nil
		case "getChildRelationships":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getChildRelationships expects 0 arguments")
			}
			return receiver.Fields["childRelationships"], receiver, false, true, nil
		case "getSObjectType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getSObjectType expects 0 arguments")
			}
			name, ok := receiver.Fields["name"]
			if !ok || name.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult token missing object")
			}
			return sObjectTypeToken(name.Text), receiver, false, true, nil
		case "isAccessible", "isCreateable", "isUpdateable", "isDeletable", "isQueryable", "isSearchable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.%s expects 0 arguments", method)
			}
			name, _ := receiver.Fields["name"]
			if name.Kind != ValueString {
				return Bool(true), receiver, false, true, nil
			}
			return Bool(vm.currentUserObjectPermission(name.Text, method)), receiver, false, true, nil
		case "isCustom":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.isCustom expects 0 arguments")
			}
			name, _ := receiver.Fields["name"]
			return Bool(name.Kind == ValueString && (strings.HasSuffix(name.Text, "__c") || strings.HasSuffix(name.Text, "__mdt"))), receiver, false, true, nil
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
			request := args[0]
			if endpoint, ok := request.Fields["endpoint"]; ok && endpoint.Kind == ValueString && vm.Org != nil {
				if resolved, ok := resource.ResolveEndpoint(vm.Org.Metadata, endpoint.Text); ok {
					request.Fields["resolvedEndpoint"] = String(resolved)
				}
			}
			if err := vm.incrementLimit("callouts", 1); err != nil {
				return Null, receiver, false, true, err
			}
			appendTrace(result, "apex.callout.http", "apex.callout", map[string]any{"operation": "Http.send"})
			if vm.testContext != nil && vm.testContext.HTTPMock.Kind == ValueObject {
				if target, ok := vm.resolveInstanceMethod(vm.testContext.HTTPMock.Type, "respond"); ok {
					value, err := vm.callMethodWithReceiver(target, vm.testContext.HTTPMock, []Value{request}, &Result{})
					if err != nil {
						return Null, receiver, false, true, err
					}
					if value.Kind == ValueObject && value.Type == "HttpResponse" {
						return value, receiver, false, true, nil
					}
					return Null, receiver, false, true, fmt.Errorf("HttpCalloutMock.respond must return HttpResponse")
				}
				value, err := vm.localHTTPMockResponse(vm.testContext.HTTPMock, request)
				if err != nil {
					return Null, receiver, false, true, err
				}
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, unsupportedCallError("Http.send real network transport")
		}
	case "Cache.OrgPartition", "Cache.SessionPartition":
		value, updatedReceiver, err := vm.callCachePartitionMember(receiver, method, args)
		return value, updatedReceiver, true, true, err
	case "Auth.AuthConfiguration":
		switch method {
		case "getAuthProviders":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getAuthProviders expects 0 arguments")
			}
			return List(), receiver, false, true, nil
		case "getAuthConfig":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getAuthConfig expects 0 arguments")
			}
			if value, ok := receiver.Fields["authConfig"]; ok {
				return value, receiver, false, true, nil
			}
			config := Object("Auth.AuthConfig")
			config.Fields["Url"] = receiver.Fields["communityUrl"]
			return config, receiver, false, true, nil
		case "getStartUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getStartUrl expects 0 arguments")
			}
			if value, ok := receiver.Fields["startUrl"]; ok {
				return value, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		}
	case "Auth.JWT":
		switch method {
		case "setIss":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Auth.JWT.setIss expects 1 argument")
			}
			receiver.Fields["iss"] = args[0]
			return Null, receiver, true, true, nil
		case "toJSONString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.JWT.toJSONString expects 0 arguments")
			}
			fields := make(map[string]any, len(receiver.Fields))
			for field, value := range receiver.Fields {
				if strings.HasPrefix(field, "__") || value.Kind == ValueNull {
					continue
				}
				fields[field] = jsonFromValue(value, true)
			}
			data, err := json.Marshal(fields)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return String(string(data)), receiver, false, true, nil
		}
	case "Metadata.DeployContainer":
		switch method {
		case "addMetadata":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Metadata.DeployContainer.addMetadata expects metadata")
			}
			values := receiver.Fields["metadata"]
			if values.Kind != ValueList {
				values = List()
			}
			values.List = append(values.List, args[0])
			receiver.Fields["metadata"] = values
			return Null, receiver, true, true, nil
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
	case "VisualEditor.DataRow":
		return callVisualEditorDataRowMember(receiver, method, args)
	case "VisualEditor.DynamicPickListRows":
		return callVisualEditorDynamicPickListRowsMember(receiver, method, args)
	case "SelectOption":
		return callSelectOptionMember(receiver, method, args)
	case "ApexPages.StandardController":
		return vm.callStandardControllerMember(receiver, method, args, result)
	case "ApexPages.StandardSetController":
		return vm.callStandardSetControllerMember(receiver, method, args, result)
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
	case "Dom.Document":
		return callDomDocumentMember(receiver, method, args)
	case "Dom.XmlNode":
		return callDomXmlNodeMember(receiver, method, args)
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
		if !ok {
			if resolved, hasResolved := request.Fields["resolvedEndpoint"]; hasResolved && resolved.Kind == ValueString {
				resource, ok = resources.Map[mapKey(resolved)]
			}
		}
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

func (vm *VM) callCachePartitionMember(receiver Value, method string, args []Value) (Value, Value, error) {
	name, ok := receiver.Fields["name"]
	if !ok || name.Kind != ValueString || strings.TrimSpace(name.Text) == "" {
		return Null, receiver, fmt.Errorf("%s partition missing name", receiver.Type)
	}
	partitionName := strings.ToLower(receiver.Type + ":" + name.Text)
	method = strings.ToLower(method)
	switch method {
	case "get":
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, fmt.Errorf("%s.get expects key or CacheBuilder type and key", receiver.Type)
		}
		keyArg := args[0]
		if len(args) == 2 {
			keyArg = args[1]
		}
		if keyArg.Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.get key expects String", receiver.Type)
		}
		value, ok := vm.cacheGet(partitionName, keyArg.Text)
		if !ok {
			return Null, receiver, nil
		}
		return value, receiver, nil
	case "put":
		if len(args) < 2 || len(args) > 5 || args[0].Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.put expects String key, value[, ttlSeconds[, visibility[, immutable]]]", receiver.Type)
		}
		ttl := int64(0)
		if len(args) >= 3 {
			if args[2].Kind != ValueInt {
				return Null, receiver, fmt.Errorf("%s.put ttl expects Integer seconds", receiver.Type)
			}
			ttl = args[2].Int
		}
		vm.cachePut(partitionName, args[0].Text, args[1], ttl)
		return Null, receiver, nil
	case "remove":
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, fmt.Errorf("%s.remove expects key or CacheBuilder type and key", receiver.Type)
		}
		keyArg := args[0]
		if len(args) == 2 {
			keyArg = args[1]
		}
		if keyArg.Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.remove key expects String", receiver.Type)
		}
		removed, ok := vm.cacheRemove(partitionName, keyArg.Text)
		if !ok {
			return Null, receiver, nil
		}
		return removed, receiver, nil
	case "contains":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.contains expects String key", receiver.Type)
		}
		_, ok := vm.cacheGet(partitionName, args[0].Text)
		return Bool(ok), receiver, nil
	default:
		return Null, receiver, unsupportedCallError(receiver.Type + "." + method)
	}
}

func (vm *VM) cacheGet(partition, key string) (Value, bool) {
	entries := vm.platformCache[partition]
	if entries == nil {
		return Null, false
	}
	entry, ok := entries[key]
	if !ok {
		return Null, false
	}
	if !entry.ExpireAt.IsZero() && !entry.ExpireAt.After(vm.fakeNow) {
		delete(entries, key)
		return Null, false
	}
	return entry.Value, true
}

func (vm *VM) cachePut(partition, key string, value Value, ttlSeconds int64) {
	if vm.platformCache == nil {
		vm.platformCache = make(map[string]map[string]cacheEntry)
	}
	entries := vm.platformCache[partition]
	if entries == nil {
		entries = make(map[string]cacheEntry)
		vm.platformCache[partition] = entries
	}
	entry := cacheEntry{Value: value}
	if ttlSeconds > 0 {
		entry.ExpireAt = vm.fakeNow.Add(time.Duration(ttlSeconds) * time.Second)
	}
	entries[key] = entry
}

func (vm *VM) cacheRemove(partition, key string) (Value, bool) {
	value, ok := vm.cacheGet(partition, key)
	if !ok {
		return Null, false
	}
	delete(vm.platformCache[partition], key)
	return value, true
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
	for _, resource := range vm.Org.Metadata.StaticResources {
		if strings.EqualFold(resource.Name, resourceName) {
			if resource.Content != "" {
				return resource.Content
			}
			break
		}
	}
	for _, asset := range vm.Org.Metadata.ContentAssets {
		if strings.EqualFold(asset.Name, resourceName) {
			if asset.Content != "" {
				return asset.Content
			}
			break
		}
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

func (vm *VM) lookupLabel(name string) (Value, bool) {
	label := strings.TrimPrefix(name, "Label.")
	if label == "" {
		return Null, false
	}
	namespace := ""
	if before, after, ok := strings.Cut(label, "."); ok {
		namespace = before
		label = after
	}
	if vm.Org != nil {
		if value, status := resource.ResolveLabel(vm.Org.Metadata, vm.Org.Namespace, namespace, label); status != resource.LabelLookupMissing {
			return String(value), true
		}
	}
	if namespace == "" && !strings.Contains(label, ".") {
		return String(label), true
	}
	return Null, false
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

func callSelectOptionMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	fieldForGetter := map[string]string{
		"getValue":      "value",
		"getLabel":      "label",
		"getDisabled":   "disabled",
		"getEscapeItem": "escapeItem",
	}
	if field, ok := fieldForGetter[method]; ok {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects 0 arguments", method)
		}
		return receiver.Fields[field], receiver, false, true, nil
	}
	fieldForSetter := map[string]string{
		"setValue":      "value",
		"setLabel":      "label",
		"setDisabled":   "disabled",
		"setEscapeItem": "escapeItem",
	}
	if field, ok := fieldForSetter[method]; ok {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects 1 argument", method)
		}
		if (field == "value" || field == "label") && args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects String", method)
		}
		if (field == "disabled" || field == "escapeItem") && args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects Boolean", method)
		}
		receiver.Fields[field] = args[0]
		return Null, receiver, true, true, nil
	}
	return Null, receiver, false, false, nil
}

func callVisualEditorDataRowMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch {
	case strings.EqualFold(method, "getLabel"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.getLabel expects 0 arguments")
		}
		return receiver.Fields["label"], receiver, false, true, nil
	case strings.EqualFold(method, "getValue"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.getValue expects 0 arguments")
		}
		return receiver.Fields["value"], receiver, false, true, nil
	case strings.EqualFold(method, "setLabel"):
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.setLabel expects String")
		}
		receiver.Fields["label"] = args[0]
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "setValue"):
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.setValue expects String")
		}
		receiver.Fields["value"] = args[0]
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callVisualEditorDynamicPickListRowsMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	rows := receiver.Fields["rows"]
	if rows.Kind != ValueList {
		rows = typedList("List<VisualEditor.DataRow>")
	}
	switch {
	case strings.EqualFold(method, "addRow"):
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "VisualEditor.DataRow") {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.addRow expects VisualEditor.DataRow")
		}
		rows.List = append(rows.List, args[0])
		receiver.Fields["rows"] = rows
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "size"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.size expects 0 arguments")
		}
		return Int(int64(len(rows.List))), receiver, false, true, nil
	case strings.EqualFold(method, "get"):
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.get expects Integer index")
		}
		index := int(args[0].Int)
		if index < 0 || index >= len(rows.List) {
			return Null, receiver, false, true, fmt.Errorf("List index out of bounds: %d", index)
		}
		return rows.List[index], receiver, false, true, nil
	case strings.EqualFold(method, "getRows"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.getRows expects 0 arguments")
		}
		return rows, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) callStandardControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	record, ok := receiver.Fields["record"]
	if !ok || record.Kind != ValueObject {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController has no SObject record")
	}
	switch method {
	case "getId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.getId expects 0 arguments")
		}
		if id, ok := record.Fields["Id"]; ok {
			return id, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getRecord":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.getRecord expects 0 arguments")
		}
		return record, receiver, false, true, nil
	case "save", "quickSave":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.%s expects 0 arguments", method)
		}
		op := "insert"
		if id, ok := record.Fields["Id"]; ok {
			if idText, ok := idValueText(id); ok && idText != "" {
				op = "update"
			}
		}
		appendStandardControllerActionTrace(result, "start", method, record, map[string]any{"dmlOperation": op})
		results, err := vm.applyDML(op, record, true, "", result)
		if err != nil {
			appendStandardControllerErrorTrace(result, method, record, op, err)
			return Null, receiver, false, true, err
		}
		if len(results) > 0 && results[0].ID != "" {
			record.Fields["Id"] = String(string(results[0].ID))
			receiver.Fields["record"] = record
		}
		page := standardControllerPage(record)
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"dmlOperation":  op,
			"pageReference": tracePageReference(page),
		})
		return page, receiver, true, true, nil
	case "delete":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.delete expects 0 arguments")
		}
		appendStandardControllerActionTrace(result, "start", method, record, map[string]any{"dmlOperation": "delete"})
		if _, err := vm.applyDML("delete", record, true, "", result); err != nil {
			appendStandardControllerErrorTrace(result, method, record, "delete", err)
			return Null, receiver, false, true, err
		}
		page := standardControllerPage(record)
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"dmlOperation":  "delete",
			"pageReference": tracePageReference(page),
		})
		return page, receiver, false, true, nil
	case "view", "edit", "cancel", "reset":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.%s expects 0 arguments", method)
		}
		page := standardControllerPage(record)
		appendStandardControllerActionTrace(result, "start", method, record, nil)
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"pageReference": tracePageReference(page),
		})
		return page, receiver, false, true, nil
	case "addFields":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.addFields expects List")
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func appendStandardControllerActionTrace(result *Result, phase, method string, record Value, extra map[string]any) {
	args := standardControllerTraceArgs(method, record)
	for key, value := range extra {
		args[key] = value
	}
	appendTrace(result, "apex.visualforce.standard_controller.action."+phase, "apex.visualforce.standard_controller", args)
}

func appendStandardControllerErrorTrace(result *Result, method string, record Value, dmlOperation string, err error) {
	actionErr := uiInvocationError(err)
	appendStandardControllerActionTrace(result, "error", method, record, map[string]any{
		"dmlOperation": dmlOperation,
		"error":        actionErr.Message,
		"errorType":    actionErr.Type,
	})
}

func standardControllerTraceArgs(method string, record Value) map[string]any {
	args := map[string]any{"method": method}
	if record.Kind == ValueObject {
		args["objectType"] = record.Type
		if id, ok := record.Fields["Id"]; ok && id.Kind == ValueString && id.Text != "" {
			args["recordId"] = id.Text
		}
	}
	return args
}

func (vm *VM) callStandardSetControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	records := receiver.Fields["records"]
	switch method {
	case "getRecords":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getRecords expects 0 arguments")
		}
		return standardSetCurrentPage(receiver, records), receiver, false, true, nil
	case "getSelected":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getSelected expects 0 arguments")
		}
		return receiver.Fields["selected"], receiver, false, true, nil
	case "setSelected":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setSelected expects List")
		}
		receiver.Fields["selected"] = args[0]
		return Null, receiver, true, true, nil
	case "getPageSize":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getPageSize expects 0 arguments")
		}
		return receiver.Fields["pageSize"], receiver, false, true, nil
	case "setPageSize":
		if len(args) != 1 || args[0].Kind != ValueInt || args[0].Int <= 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setPageSize expects positive Integer")
		}
		receiver.Fields["pageSize"] = args[0]
		receiver.Fields["pageNumber"] = Int(1)
		return Null, receiver, true, true, nil
	case "getPageNumber":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getPageNumber expects 0 arguments")
		}
		return receiver.Fields["pageNumber"], receiver, false, true, nil
	case "first":
		receiver.Fields["pageNumber"] = Int(1)
		return Null, receiver, true, true, nil
	case "last":
		receiver.Fields["pageNumber"] = Int(int64(standardSetPageCount(receiver, records)))
		return Null, receiver, true, true, nil
	case "next":
		page := int(receiver.Fields["pageNumber"].Int)
		if page < standardSetPageCount(receiver, records) {
			receiver.Fields["pageNumber"] = Int(int64(page + 1))
		}
		return Null, receiver, true, true, nil
	case "previous":
		page := int(receiver.Fields["pageNumber"].Int)
		if page > 1 {
			receiver.Fields["pageNumber"] = Int(int64(page - 1))
		}
		return Null, receiver, true, true, nil
	case "getHasNext":
		return Bool(int(receiver.Fields["pageNumber"].Int) < standardSetPageCount(receiver, records)), receiver, false, true, nil
	case "getHasPrevious":
		return Bool(receiver.Fields["pageNumber"].Int > 1), receiver, false, true, nil
	case "getCompleteResult":
		return Bool(true), receiver, false, true, nil
	case "save":
		return vm.standardSetDML(receiver, "update", result)
	case "delete":
		return vm.standardSetDML(receiver, "delete", result)
	case "cancel":
		return newPageReference(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func standardControllerPage(record Value) Value {
	if id, ok := record.Fields["Id"]; ok && id.Kind == ValueString && id.Text != "" {
		return newPageReference("/" + id.Text)
	}
	return newPageReference("")
}

func standardSetCurrentPage(controller, records Value) Value {
	if records.Kind != ValueList {
		return List()
	}
	pageSize := int(controller.Fields["pageSize"].Int)
	pageNumber := int(controller.Fields["pageNumber"].Int)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageNumber <= 0 {
		pageNumber = 1
	}
	start := (pageNumber - 1) * pageSize
	if start >= len(records.List) {
		return List()
	}
	end := start + pageSize
	if end > len(records.List) {
		end = len(records.List)
	}
	return List(records.List[start:end]...)
}

func standardSetPageCount(controller, records Value) int {
	if records.Kind != ValueList || len(records.List) == 0 {
		return 1
	}
	pageSize := int(controller.Fields["pageSize"].Int)
	if pageSize <= 0 {
		pageSize = 20
	}
	pages := (len(records.List) + pageSize - 1) / pageSize
	if pages < 1 {
		return 1
	}
	return pages
}

func (vm *VM) standardSetDML(receiver Value, op string, result *Result) (Value, Value, bool, bool, error) {
	records := receiver.Fields["records"]
	if records.Kind != ValueList {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.%s requires records", op)
	}
	if _, err := vm.applyDML(op, records, true, "", result); err != nil {
		return Null, receiver, false, true, err
	}
	return newPageReference(""), receiver, false, true, nil
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
