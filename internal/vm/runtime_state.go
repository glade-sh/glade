package vm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
)

const maxLoopIterations = 1000000

const (
	triggerTimingBefore = "before"
	triggerTimingAfter  = "after"
)

type VM struct {
	Globals                   map[string]Value
	VarTypes                  map[string]string
	Methods                   map[string]Method
	MethodOverloads           map[string][]Method
	MethodFolded              map[string][]Method
	methodCandidates          map[string][]Method
	methodResolveCache        map[string]methodResolution
	Classes                   map[string]Class
	classLookup               map[string]Class
	namespaceClassLookup      map[string]map[string]namespaceClassLookup
	classNamespaceCache       map[string]string
	classForAccessCache       map[string]classForAccessLookup
	enumLookup                map[string]enumClassLookup
	enumSuffixLookup          map[string]enumClassLookup
	uniqueNestedTypeCache     map[string]uniqueNestedTypeLookup
	onlyNestedTypeCache       map[string]uniqueNestedTypeLookup
	topLevelTypeCache         map[string]uniqueNestedTypeLookup
	classNameSearchCache      []classNameSearchEntry
	Org                       *storage.OrgState
	Triggers                  map[string][]Trigger
	triggerMatchCache         map[string][]Trigger
	triggerNamespaceCache     map[triggerNamespaceLookupKey]string
	Stdout                    io.Writer
	callStack                 []callFrame
	scopeStack                []map[string]Value
	currentClass              string
	currentNamespace          string
	currentMethod             Method
	testContext               *TestContext
	localAsyncJobs            []AsyncJob
	localAsyncSeq             int
	localAsyncDrain           bool
	localAsyncChain           bool
	executionUser             Value
	limits                    Limits
	limitCaps                 LimitCaps
	limitMode                 LimitMode
	limitViolations           []LimitViolation
	fakeNow                   time.Time
	currentAsyncKind          string
	currentQueueableDepth     int
	currentQueueableMaxDepth  int
	currentFinalizer          Value
	activeExceptions          []activeException
	currentStatement          callFrame
	hasStatement              bool
	triggerDepth              int
	activeTriggerNamespaces   []string
	installContextDepth       int
	savepoints                map[string]storage.OrgState
	emailSavepoints           map[string][]CapturedEmail
	savepointOrder            map[string]int
	nextSavepoint             int
	pageMessages              []Value
	currentPage               Value
	pageReferences            map[string]string
	fixedSearchResults        []Value
	sfsqlqueryRows            []Value
	sfsqlqueryMetadata        []Value
	platformCache             map[string]map[string]cacheEntry
	cacheScanLocators         map[string][]cacheScanItem
	cacheScanSeq              int
	capturedEmails            []CapturedEmail
	restRequest               Value
	restResponse              Value
	serverBaseURL             string
	metadataDeploys           map[string]Value
	reportInstances           map[string]Value
	pushUpgradeCustoms        map[string]pushUpgradeCustomization
	debugHooks                DebugHooks
	hasDebugHooks             bool
	traceEnabled              bool
	ctx                       context.Context
	activeGetters             map[string]int
	activeSetters             map[string]int
	triggerGlobals            map[string]Value
	cryptoRandomSeq           uint64
	staticInitState           map[string]staticInitState
	lastAmbiguous             *overloadDiagnostic
	activeConstructors        map[string]int
	describeCache             map[string]Value
	fieldDescribeCache        map[string]Value
	globalDescribeCache       *Value
	describeTabsCache         *Value
	describeDefCache          map[string]storage.ObjectDefinition
	customDataCache           map[string]Value
	soqlExecutionCache        *soql.ExecutionCache
	managedFeatureFlags       map[string]bool
	childRelCache             map[string][]Value
	jsonChildRelTypeCache     map[string]jsonRelationshipTypeLookup
	loadedChildRelCache       map[string]loadedChildRelationshipLookup
	lazyChildRelCache         map[string]lazyChildRelationshipLookup
	objectNameCache           map[string]objectNameLookup
	recentlyViewed            map[string]map[storage.ID]recentlyViewedEntry
	metadataCacheStamp        string
	isolationJournal          *storage.IsolationJournal
	staticValueRefs           map[uint64]bool
	staticValueRefFields      map[uint64][]staticFieldRef
	frameworkRecorderRollback *frameworkMethodCountRecorderRollback
}

type staticFieldRef struct {
	ClassName string
	FieldName string
}

type frameworkMethodCountRecorderRollback struct {
	previous *frameworkMethodCountRecorderRollback
	values   map[string]Value
}

type recentlyViewedEntry struct {
	ID         storage.ID
	ObjectName string
	Name       string
	ViewedAt   string
}

type lazyChildRelationshipLookup struct {
	ChildType string
	Targets   []lazyChildRelationshipTarget
	OK        bool
}

type lazyChildRelationshipTarget struct {
	ChildName   string
	LookupField string
}

type jsonRelationshipTypeLookup struct {
	Type string
	OK   bool
}

type loadedChildRelationshipLookup struct {
	ParentRelationshipExists bool
	ChildRelationshipNames   []string
	CandidateNames           []string
}

type objectNameLookup struct {
	Name string
	OK   bool
}

type triggerNamespaceLookupKey struct {
	CurrentNamespace string
	Name             string
}

type classForAccessLookup struct {
	Class Class
	OK    bool
}

type pushUpgradeCustomization struct {
	ID                   string
	PackageID            string
	SubscriberOrgID      string
	CustomUpgradeAllowed bool
}

type enumClassLookup struct {
	Class Class
	OK    bool
}

type uniqueNestedTypeLookup struct {
	Name string
	OK   bool
}

type classNameSearchEntry struct {
	Name  string
	Lower string
}

type namespaceClassLookup struct {
	Class Class
	OK    bool
}

type methodResolution struct {
	Method Method
	OK     bool
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
	traceEnabled    bool
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

const maxApexCallDepth = 1000

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
	Started                 bool
	Stopped                 bool
	CurrentUser             Value
	SeeAllData              bool
	AsyncJobs               []AsyncJob
	AsyncStartIndex         int
	EventPublishes          []eventPublishCallback
	PlatformEvents          []storage.Record
	PlatformEventStartIndex int
	Draining                bool
	HTTPMock                Value
	WebServiceMock          Value
	ContinuationResponses   map[string]Value
	ConnectAPIFixtures      map[string]Value
	SoqlStubs               map[string]Value
	ParentLimits            Limits
	ParentViolations        []LimitViolation
	RunAsDepth              int
	SetupDML                bool
	NonSetupDML             bool
	JobSeq                  int
	ChainEnqueued           bool
	PreserveAsyncStatics    bool
}

type eventPublishCallback struct {
	Callback   Value
	EventUUIDs []string
	Fail       bool
}

type AsyncJob struct {
	ID                string
	Kind              string
	Object            Value
	Method            Method
	Args              []Value
	BatchSize         int
	Name              string
	Cron              string
	Deferred          bool
	QueueableDepth    int
	QueueableMaxDepth int
}

type cacheEntry struct {
	Value        Value
	SecondaryKey string
	ExpireAt     time.Time
}

type cacheScanItem struct {
	Key   string
	Value Value
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
		Globals:               make(map[string]Value),
		VarTypes:              make(map[string]string),
		Methods:               make(map[string]Method),
		MethodOverloads:       make(map[string][]Method),
		MethodFolded:          make(map[string][]Method),
		methodCandidates:      make(map[string][]Method),
		methodResolveCache:    make(map[string]methodResolution),
		Classes:               make(map[string]Class),
		classLookup:           make(map[string]Class),
		namespaceClassLookup:  make(map[string]map[string]namespaceClassLookup),
		classNamespaceCache:   make(map[string]string),
		classForAccessCache:   make(map[string]classForAccessLookup),
		enumLookup:            make(map[string]enumClassLookup),
		enumSuffixLookup:      make(map[string]enumClassLookup),
		uniqueNestedTypeCache: make(map[string]uniqueNestedTypeLookup),
		onlyNestedTypeCache:   make(map[string]uniqueNestedTypeLookup),
		topLevelTypeCache:     make(map[string]uniqueNestedTypeLookup),
		Triggers:              make(map[string][]Trigger),
		triggerMatchCache:     make(map[string][]Trigger),
		triggerNamespaceCache: make(map[triggerNamespaceLookupKey]string),
		Stdout:                stdout,
		limitCaps:             defaultLimitCaps(),
		limitMode:             LimitModePermissive,
		fakeNow:               time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		savepoints:            make(map[string]storage.OrgState),
		emailSavepoints:       make(map[string][]CapturedEmail),
		savepointOrder:        make(map[string]int),
		platformCache:         make(map[string]map[string]cacheEntry),
		cacheScanLocators:     make(map[string][]cacheScanItem),
		metadataDeploys:       make(map[string]Value),
		reportInstances:       make(map[string]Value),
		pushUpgradeCustoms:    make(map[string]pushUpgradeCustomization),
		traceEnabled:          true,
		ctx:                   context.Background(),
		activeGetters:         make(map[string]int),
		activeSetters:         make(map[string]int),
		staticInitState:       make(map[string]staticInitState),
		describeCache:         make(map[string]Value),
		fieldDescribeCache:    make(map[string]Value),
		describeDefCache:      make(map[string]storage.ObjectDefinition),
		customDataCache:       make(map[string]Value),
		managedFeatureFlags:   make(map[string]bool),
		childRelCache:         make(map[string][]Value),
		jsonChildRelTypeCache: make(map[string]jsonRelationshipTypeLookup),
		loadedChildRelCache:   make(map[string]loadedChildRelationshipLookup),
		lazyChildRelCache:     make(map[string]lazyChildRelationshipLookup),
		objectNameCache:       make(map[string]objectNameLookup),
		recentlyViewed:        make(map[string]map[storage.ID]recentlyViewedEntry),
	}
}

// CloneRuntime returns a fresh VM with the same registered Apex methods,
// classes, and triggers. Mutable runtime state such as globals, limits, org
// state, current user, and static field values remains request-local.
func (vm *VM) CloneRuntime(stdout io.Writer) *VM {
	clone := New(stdout)
	clone.Methods = vm.Methods
	clone.MethodOverloads = vm.MethodOverloads
	clone.MethodFolded = vm.MethodFolded
	clone.Classes = copyClassMap(vm.Classes)
	clone.rebuildClassLookup()
	clone.Triggers = copyTriggerSliceMap(vm.Triggers)
	clone.triggerMatchCache = make(map[string][]Trigger)
	clone.traceEnabled = vm.traceEnabled
	clone.staticInitState = copyStaticInitStateMap(vm.staticInitState)
	clone.pageReferences = copyStringMap(vm.pageReferences)
	clone.platformCache = copyCacheMap(vm.platformCache)
	clone.isolationJournal = vm.isolationJournal
	return clone
}

func (vm *VM) SetIsolationJournal(journal *storage.IsolationJournal) {
	if vm != nil {
		vm.isolationJournal = journal
	}
}

func (vm *VM) SetTraceEnabled(enabled bool) {
	if vm != nil {
		vm.traceEnabled = enabled
	}
}

func (vm *VM) DeterministicRandomState() uint64 {
	if vm == nil {
		return 0
	}
	return vm.cryptoRandomSeq
}

func (vm *VM) SetDeterministicRandomState(seq uint64) {
	if vm != nil {
		vm.cryptoRandomSeq = seq
	}
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
		canonical := classCopyKey(name, class)
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
	class.Fields = copyFieldMap(class.Fields)
	class.StaticFields = copyFieldMap(class.StaticFields)
	return class
}

func copyFieldMap(in map[string]Field) map[string]Field {
	out := make(map[string]Field, len(in))
	for name, field := range in {
		field.Value = cloneValue(field.Value)
		field.InitialValue = cloneValue(field.InitialValue)
		out[name] = field
	}
	return out
}

func classCopyKey(alias string, class Class) string {
	name := class.Name
	if name == "" {
		name = alias
	}
	return strings.ToLower(strings.TrimSpace(class.Namespace)) + "\x00" + strings.ToLower(strings.TrimSpace(name))
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
	if vm.Org != org {
		currentStamp := vm.schemaCacheStamp()
		nextStamp := schemaCacheStampForOrg(org)
		if currentStamp != "" && nextStamp != "" && currentStamp == nextStamp {
			vm.metadataCacheStamp = nextStamp
		} else {
			vm.clearMetadataCaches()
			vm.metadataCacheStamp = nextStamp
		}
	}
	vm.Org = org
	vm.clearTriggerMatchCache()
	if vm.Org != nil {
		vm.Org.Now = func() time.Time { return vm.fakeNow }
	}
	if vm.testContext != nil && isPlaceholderCurrentUser(vm.testContext.CurrentUser) {
		vm.testContext.CurrentUser = vm.defaultTestCurrentUser()
	}
}

func (vm *VM) schemaCacheStamp() string {
	if strings.TrimSpace(vm.metadataCacheStamp) != "" {
		return vm.metadataCacheStamp
	}
	return schemaCacheStampForOrg(vm.Org)
}

func schemaCacheStampForOrg(org *storage.OrgState) string {
	if org == nil {
		return ""
	}
	objectCount := 0
	fieldCount := 0
	relationCount := 0
	recordTypeCount := 0
	for _, object := range org.Objects {
		if strings.TrimSpace(object.Definition.APIName) == "" {
			continue
		}
		objectCount++
		fieldCount += len(object.Definition.Fields)
		relationCount += len(object.Definition.Relations)
		recordTypeCount += len(object.Definition.RecordTypes)
	}
	return strings.ToLower(strings.TrimSpace(org.Namespace)) + "|" +
		strconv.Itoa(objectCount) + "|" +
		strconv.Itoa(fieldCount) + "|" +
		strconv.Itoa(relationCount) + "|" +
		strconv.Itoa(recordTypeCount)
}

func isPlaceholderCurrentUser(user Value) bool {
	return user.Kind == ValueString && strings.EqualFold(strings.TrimSpace(user.Text), "system")
}

func (vm *VM) SetServerBaseURL(rawURL string) {
	vm.serverBaseURL = strings.TrimRight(rawURL, "/")
}

func (vm *VM) SetCurrentUser(record storage.Record) {
	if record.ID == "" {
		if id, ok := record.GetField("Id"); ok {
			record.ID = storageIDFromValue(id)
		}
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
	engine.IsolationJournal = vm.isolationJournal
	engine.Now = func() time.Time { return vm.fakeNow }
	if userID := vm.currentUserID(); userID != "" {
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
	appendTrace(result, "apex.flow.action", "apex.flow", map[string]any{
		"action": action.Name,
		"class":  method.ClassName,
		"method": method.Name,
		"record": string(record.ID),
		"object": record.Object,
	})
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
	vm.clearTriggerMatchCache()
	return nil
}

func (vm *VM) clearTriggerMatchCache() {
	if vm == nil {
		return
	}
	vm.triggerMatchCache = make(map[string][]Trigger)
	vm.triggerNamespaceCache = make(map[triggerNamespaceLookupKey]string)
}

func (vm *VM) EnableTestContext() {
	vm.testContext = &TestContext{
		CurrentUser:           vm.defaultTestCurrentUser(),
		ContinuationResponses: make(map[string]Value),
		ConnectAPIFixtures:    make(map[string]Value),
		SoqlStubs:             make(map[string]Value),
	}
	vm.ensureAsyncObjects()
}

func (vm *VM) SetTestSeeAllData(enabled bool) {
	if vm.testContext == nil {
		vm.EnableTestContext()
	}
	vm.testContext.SeeAllData = enabled
}

func (vm *VM) defaultTestCurrentUser() Value {
	if vm.executionUser.Kind != "" && vm.executionUser.Kind != ValueNull {
		return vm.executionUser
	}
	if user := vm.defaultOrgUser(); user.Kind != "" {
		return user
	}
	return String("system")
}

func (vm *VM) defaultOrgUser() Value {
	if vm.Org == nil {
		return Value{}
	}
	users, ok := vm.Org.Objects["User"]
	if !ok || len(users.Records) == 0 {
		return Value{}
	}
	for _, preferredID := range []storage.ID{storage.ID("005-local-user"), storage.ID("005000000000001")} {
		if preferred, ok := users.Records[preferredID]; ok {
			if !strings.EqualFold(recordFieldString(preferred, "UserType"), "AutomatedProcess") {
				return vmValueFromRecord(preferred)
			}
		}
	}
	var first storage.ID
	var fallback storage.ID
	for id := range users.Records {
		record := users.Records[id]
		if strings.EqualFold(recordFieldString(record, "UserType"), "AutomatedProcess") {
			if fallback == "" || id < fallback {
				fallback = id
			}
			continue
		}
		if first == "" || id < first {
			first = id
		}
	}
	if first == "" {
		first = fallback
	}
	return vmValueFromRecord(users.Records[first])
}

func (vm *VM) ResetStatics() error {
	for className, class := range vm.Classes {
		if len(class.StaticFields) == 0 {
			continue
		}
		for fieldName, field := range class.StaticFields {
			field.Value = defaultStaticCollectionFieldValue(className, fieldName, field)
			class.StaticFields[fieldName] = field
		}
		vm.Classes[className] = class
	}
	vm.invalidateStaticValueRefs()
	vm.staticInitState = make(map[string]staticInitState)
	vm.ResetApexPageState()
	return nil
}

func (vm *VM) ResetTestAsyncStaticCollections() error {
	for className, class := range vm.Classes {
		if len(class.StaticFields) == 0 {
			continue
		}
		for fieldName, field := range class.StaticFields {
			if !isStaticCollectionField(field) {
				continue
			}
			field.Value = defaultStaticCollectionFieldValue(className, fieldName, field)
			class.StaticFields[fieldName] = field
		}
		vm.Classes[className] = class
	}
	vm.invalidateStaticValueRefs()
	return nil
}

func defaultStaticCollectionFieldValue(className, fieldName string, field Field) Value {
	value := defaultStaticFieldValue(className, fieldName, field.Type, field.InitialValue)
	if value.Kind != ValueNull && value.Kind != "" {
		return value
	}
	switch staticCollectionBase(field.Type) {
	case "List":
		return typedList(field.Type)
	case "Set":
		return typedSet(field.Type)
	case "Map":
		return typedMap(field.Type)
	default:
		return value
	}
}

func isStaticCollectionField(field Field) bool {
	if field.Value.Kind == ValueSet {
		return true
	}
	return staticCollectionBase(field.Type) == "Set"
}

func staticCollectionBase(typeName string) string {
	base := collectionBase(typeName)
	if base != "" && base != "Iterator" && base != "Iterable" {
		return base
	}
	genericBase, ok := genericBaseName(typeName)
	if !ok {
		return ""
	}
	if i := strings.LastIndex(genericBase, "."); i >= 0 {
		genericBase = genericBase[i+1:]
	}
	switch {
	case strings.EqualFold(genericBase, "List"):
		return "List"
	case strings.EqualFold(genericBase, "Set"):
		return "Set"
	case strings.EqualFold(genericBase, "Map"):
		return "Map"
	default:
		return ""
	}
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
	result = Result{Vars: vm.Globals, traceEnabled: vm.traceEnabled}
	if vm.traceEnabled {
		result.TraceFormat = trace.FormatChromeTraceEvent
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("internal VM panic: %v", recovered)
		}
		result.CapturedEmails = append([]CapturedEmail(nil), vm.capturedEmails...)
		result.Limits = vm.limits
		result.LimitMode = vm.limitMode
		result.LimitViolations = append([]LimitViolation(nil), vm.limitViolations...)
		vm.appendTrace(&result, "apex.limits", "apex.limits", limitTraceArgs(vm.limits))
	}()
	callerClass := vm.currentClass
	if className != "" {
		vm.currentClass = className
		defer func() {
			vm.currentClass = callerClass
		}()
	}
	if vm.ctx != nil {
		if err := vm.ctx.Err(); err != nil {
			return result, err
		}
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
