package vm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
)

const defaultMaxLoopIterations = 1000000

var maxLoopIterations = defaultMaxLoopIterations

const (
	triggerTimingBefore = "before"
	triggerTimingAfter  = "after"
)

// VM holds all interpreter state for one execution context. The fields are
// grouped by concern; the groups (in declaration order) are:
//
//   - Class/method/type registries and their lookup caches
//   - Org storage, triggers, and the active execution frame (stacks, current
//     class/namespace/method)
//   - Async/queueable scheduling state
//   - Governor limits (limits, caps, mode, violations)
//   - Exception, statement, and trigger-depth tracking
//   - Transaction state (savepoints, savepoint order/journal)
//   - Visualforce/page and REST/server request context
//   - SOQL/search results and the platform cache
//   - Captured side effects (emails, metadata deploys, reports)
//   - Describe caches (object/field/global/tabs/child-relationship)
//   - Static-field reference tracking for alias invalidation
//
// Fields are not reordered for cache-layout stability; the inline section
// markers below flag the start of a contiguous group. When adding state,
// place it in the matching group and update New.
type VM struct {
	// --- Class/method/type registries and lookup caches ---
	Globals                  map[string]Value
	VarTypes                 map[string]string
	Methods                  map[string]Method
	MethodOverloads          map[string][]Method
	MethodFolded             map[string][]Method
	methodCandidates         map[string][]Method
	methodResolveCache       map[string]methodResolution
	Classes                  map[string]Class
	classLookup              map[string]Class
	sharedClassLookupKeys    map[string]string
	sharedClassCopyPlan      *classCopyPlan
	classLookupNameCache     map[string]classLookupNameResult
	namespaceClassLookup     map[string]map[string]namespaceClassLookup
	classNamespaceCache      map[string]string
	classForAccessCache      map[classForAccessKey]classForAccessLookup
	nestedTypeHierarchyCache map[nestedTypeKey]nestedTypeResult
	enumLookup               map[string]enumClassLookup
	enumSuffixLookup         map[string]enumClassLookup
	uniqueNestedTypeCache    map[string]uniqueNestedTypeLookup
	onlyNestedTypeCache      map[string]uniqueNestedTypeLookup
	topLevelTypeCache        map[string]uniqueNestedTypeLookup
	topLevelClassLookup      map[string]topLevelClassLookup
	classNameSearchCache     []classNameSearchEntry
	// --- Org storage, triggers, and active execution frame ---
	Org                     *storage.OrgState
	Triggers                map[string][]Trigger
	triggerMatchCache       *triggerMatchCache
	triggerNamespaceCache   map[triggerNamespaceLookupKey]string
	Stdout                  io.Writer
	callStack               []callFrame
	scopeStack              []map[string]Value
	currentClass            string
	currentNamespace        string
	currentMethod           Method
	reflectionConstructType string
	testContext             *TestContext
	localAsyncJobs          []AsyncJob
	localAsyncSeq           int
	localAsyncDrain         bool
	localAsyncChain         bool
	executionUser           Value
	// --- Governor limits ---
	limits          Limits
	limitCaps       LimitCaps
	limitMode       LimitMode
	limitViolations []LimitViolation
	fakeNow         time.Time
	lastNow         time.Time
	hasLastNow      bool
	// --- Async/queueable scheduling ---
	currentAsyncKind             string
	currentQueueableDepth        int
	currentQueueableMaxDepth     int
	currentQueueableDelay        int
	queueableDuplicateSignatures map[string]string
	currentFinalizer             Value
	activeExceptions             []activeException
	// --- Exception / statement / trigger-depth tracking ---
	currentStatement        callFrame
	hasStatement            bool
	toolingExecuteAnonymous bool
	triggerDepth            int
	activeTriggerNamespaces []string
	installContextDepth     int
	// --- Transaction state (savepoints) ---
	savepoints      map[string]storage.OrgState
	savepointMarks  map[string]storage.IsolationMark
	emailSavepoints map[string][]CapturedEmail
	savepointOrder  map[string]int
	nextSavepoint   int
	// --- Visualforce / page context ---
	pageMessages     []Value
	currentPage      Value
	vfActionInvoker  VisualforceActionInvoker
	pageReferences   map[string]string
	siteExperienceID string
	// --- SOQL / search results and platform cache ---
	fixedSearchResults []Value
	sfsqlqueryRows     []Value
	sfsqlqueryMetadata []Value
	platformCache      map[string]map[string]cacheEntry
	cacheScanLocators  map[string][]cacheScanItem
	cacheScanSeq       int
	// --- Captured side effects ---
	capturedEmails []CapturedEmail
	// --- REST / server request context ---
	restRequest        Value
	restResponse       Value
	serverBaseURL      string
	metadataDeploys    map[string]Value
	reportInstances    map[string]Value
	subMgmtTestRecords map[string]Value
	subMgmtTestSeq     int
	// --- Debug / trace hooks ---
	debugHooks         DebugHooks
	hasDebugHooks      bool
	debugOutputSink    func(DebugEvent)
	traceEnabled       bool
	ctx                context.Context
	activeGetters      map[string]int
	activeSetters      map[string]int
	triggerGlobals     map[string]Value
	cryptoRandomSeq    uint64
	staticInitState    map[string]staticInitState
	lastAmbiguous      *overloadDiagnostic
	activeConstructors map[string]int
	// --- Describe caches ---
	describeCache                map[string]Value
	fieldDescribeCache           map[string]Value
	globalDescribeCache          *Value
	describeTabsCache            *Value
	describeDefCache             map[string]storage.ObjectDefinition
	customDataCache              map[string]Value
	soqlExecutionCache           *soql.ExecutionCache
	dmlSummaryByChild            *dml.SummaryRelationCache
	managedFeatureValues         map[string]Value
	childRelCache                *childRelationshipCache
	childRelationshipLookupCache *childRelationshipLookupCache
	jsonChildRelTypeCache        *jsonChildRelTypeLookupCache
	sObjectFieldAliasCache       *sObjectFieldAliasLookupCache
	fieldResolveCache            *fieldResolveLookupCache
	loadedChildRelCache          *loadedChildRelationshipLookupCache
	lazyChildRelCache            *lazyChildRelationshipLookupCache
	objectNameCache              map[string]objectNameLookup
	recentlyViewed               map[string]map[storage.ID]recentlyViewedEntry
	metadataCacheStamp           string
	isolationJournal             *storage.IsolationJournal
	// --- Static-field reference tracking (alias invalidation) ---
	staticValueRefs           map[uint64]bool
	staticValueRefFields      map[uint64]staticFieldRefSet
	localOnlyCollectionRefs   map[uint64]bool
	collectionMutationSeq     uint64
	frameworkRecorderRollback *frameworkMethodCountRecorderRollback
	runtimeArtifactsShared    bool
}

type VisualforceActionInvoker func(actionExpr string, pageURL string) (Value, error)

type VisualforcePageContext struct {
	CurrentPage   Value
	PageMessages  []Value
	ActionInvoker VisualforceActionInvoker
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
	ID           storage.ID
	ObjectName   string
	Name         string
	ViewedAt     string
	ReferencedAt string
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

type classLookupNameResult struct {
	Alias string
	OK    bool
}

// classForAccessKey is the cache key for classForAccess. Using the raw
// (whitespace-trimmed) name components as a struct key avoids the per-call
// allocation of canonicalizing and concatenating three strings just to probe
// the cache. Case variants get distinct entries, each resolved correctly by the
// case-insensitive lookup logic, so correctness is unchanged.
type classForAccessKey struct {
	ClassName        string
	CurrentClass     string
	CurrentNamespace string
}

// nestedTypeKey memoizes resolveNestedTypeInClassHierarchy. The resolution is a
// pure function of the (immutable) compiled class hierarchy, so per-VM caching
// is safe: per-test clones never register new classes, and the access caches are
// reset whenever class registration changes.
type nestedTypeKey struct {
	ClassName string
	TypeName  string
}

type nestedTypeResult struct {
	Name string
	OK   bool
}

type enumClassLookup struct {
	Class Class
	OK    bool
}

type uniqueNestedTypeLookup struct {
	Name string
	OK   bool
}

type topLevelClassLookup struct {
	ByNamespace map[string]string
	Unique      string
	Ambiguous   bool
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
	DebugEvents     []DebugEvent     `json:"debugEvents,omitempty"`
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
	Kind                         string   `json:"kind"`
	ToAddresses                  []string `json:"toAddresses,omitempty"`
	CcAddresses                  []string `json:"ccAddresses,omitempty"`
	BccAddresses                 []string `json:"bccAddresses,omitempty"`
	TargetObjectIDs              []string `json:"targetObjectIds,omitempty"`
	WhatIDs                      []string `json:"whatIds,omitempty"`
	Subject                      string   `json:"subject,omitempty"`
	PlainTextBody                string   `json:"plainTextBody,omitempty"`
	HTMLBody                     string   `json:"htmlBody,omitempty"`
	TemplateID                   string   `json:"templateId,omitempty"`
	TargetObjectID               string   `json:"targetObjectId,omitempty"`
	WhatID                       string   `json:"whatId,omitempty"`
	SaveAsActivity               bool     `json:"saveAsActivity,omitempty"`
	FileAttachments              []string `json:"fileAttachments,omitempty"`
	EntityAttachments            []string `json:"entityAttachments,omitempty"`
	DocumentAttachments          []string `json:"documentAttachments,omitempty"`
	ReplyTo                      string   `json:"replyTo,omitempty"`
	SenderDisplayName            string   `json:"senderDisplayName,omitempty"`
	Charset                      string   `json:"charset,omitempty"`
	OrgWideEmailAddressID        string   `json:"orgWideEmailAddressId,omitempty"`
	OptOutPolicy                 string   `json:"optOutPolicy,omitempty"`
	EmailPriority                string   `json:"emailPriority,omitempty"`
	BccSender                    bool     `json:"bccSender,omitempty"`
	UseSignature                 bool     `json:"useSignature,omitempty"`
	TreatBodiesAsTemplate        bool     `json:"treatBodiesAsTemplate,omitempty"`
	TreatTargetObjectAsRecipient bool     `json:"treatTargetObjectAsRecipient,omitempty"`
	TriggerUserEmail             bool     `json:"triggerUserEmail,omitempty"`
	TriggerOtherEmail            bool     `json:"triggerOtherEmail,omitempty"`
	TriggerAutoResponseEmail     bool     `json:"triggerAutoResponseEmail,omitempty"`
}

type sideEffectSnapshot struct {
	capturedEmails []CapturedEmail
}

// DebugEvent records a System.debug invocation with its logging level and the
// trace position at which it occurred. TracePos is len(Trace) at emit time, so
// a Salesforce-style log formatter can interleave debug output with SOQL/DML
// trace events in true execution order without mutating the trace stream
// itself (which the oracle/parity tooling consumes). Captured only when tracing
// is enabled.
type DebugEvent struct {
	Level    string `json:"level"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
	TracePos int    `json:"tracePos"`
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
	Started                  bool
	Stopped                  bool
	CurrentUser              Value
	SeeAllData               bool
	SeeAllDataSet            bool
	AsyncJobs                []AsyncJob
	AsyncStartIndex          int
	EventPublishes           []eventPublishCallback
	PlatformEvents           []storage.Record
	PlatformEventStartIndex  int
	ChangeDataCaptureEnabled bool
	Draining                 bool
	HTTPMock                 Value
	WebServiceMock           Value
	ContinuationResponses    map[string]Value
	ConnectAPIFixtures       map[string]Value
	SoqlStubs                map[string]Value
	ParentLimits             Limits
	ParentViolations         []LimitViolation
	CurrentPackageVersion    Value
	RunAsDepth               int
	PackageRunAsDepth        int
	SetupDML                 bool
	NonSetupDML              bool
	JobSeq                   int
	ChainEnqueued            bool
	PreserveAsyncStatics     bool
}

type eventPublishCallback struct {
	Callback   Value
	EventUUIDs []string
	Fail       bool
}

type AsyncJob struct {
	ID                          string
	Kind                        string
	Object                      Value
	Method                      Method
	Args                        []Value
	BatchSize                   int
	Name                        string
	Cron                        string
	ParentJobID                 string
	LastProcessed               string
	LastProcessedOffset         int
	Deferred                    bool
	SuppressWorkerRecords       bool
	QueueableDepth              int
	QueueableMaxDepth           int
	QueueableDelayMinutes       int
	QueueableDuplicateSignature string
	NotBefore                   time.Time
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
	warmGeneratedPlatformRuntimeIndexes()
	return &VM{
		Globals:                      make(map[string]Value),
		VarTypes:                     make(map[string]string),
		Methods:                      make(map[string]Method),
		MethodOverloads:              make(map[string][]Method),
		MethodFolded:                 make(map[string][]Method),
		methodCandidates:             make(map[string][]Method),
		methodResolveCache:           make(map[string]methodResolution),
		Classes:                      make(map[string]Class),
		classLookup:                  make(map[string]Class),
		namespaceClassLookup:         make(map[string]map[string]namespaceClassLookup),
		classNamespaceCache:          make(map[string]string),
		classForAccessCache:          make(map[classForAccessKey]classForAccessLookup),
		enumLookup:                   make(map[string]enumClassLookup),
		enumSuffixLookup:             make(map[string]enumClassLookup),
		uniqueNestedTypeCache:        make(map[string]uniqueNestedTypeLookup),
		onlyNestedTypeCache:          make(map[string]uniqueNestedTypeLookup),
		topLevelTypeCache:            make(map[string]uniqueNestedTypeLookup),
		Triggers:                     make(map[string][]Trigger),
		triggerMatchCache:            newTriggerMatchCache(),
		triggerNamespaceCache:        make(map[triggerNamespaceLookupKey]string),
		Stdout:                       stdout,
		limitCaps:                    defaultLimitCaps(),
		limitMode:                    LimitModePermissive,
		fakeNow:                      time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		queueableDuplicateSignatures: make(map[string]string),
		savepoints:                   make(map[string]storage.OrgState),
		savepointMarks:               make(map[string]storage.IsolationMark),
		emailSavepoints:              make(map[string][]CapturedEmail),
		savepointOrder:               make(map[string]int),
		platformCache:                make(map[string]map[string]cacheEntry),
		cacheScanLocators:            make(map[string][]cacheScanItem),
		metadataDeploys:              make(map[string]Value),
		reportInstances:              make(map[string]Value),
		subMgmtTestRecords:           make(map[string]Value),
		traceEnabled:                 true,
		ctx:                          context.Background(),
		activeGetters:                make(map[string]int),
		activeSetters:                make(map[string]int),
		staticInitState:              make(map[string]staticInitState),
		describeCache:                make(map[string]Value),
		fieldDescribeCache:           make(map[string]Value),
		describeDefCache:             make(map[string]storage.ObjectDefinition),
		customDataCache:              make(map[string]Value),
		managedFeatureValues:         make(map[string]Value),
		dmlSummaryByChild:            dml.NewSummaryRelationCache(),
		childRelCache:                newChildRelationshipCache(),
		childRelationshipLookupCache: newChildRelationshipLookupCache(),
		jsonChildRelTypeCache:        newJSONChildRelTypeLookupCache(),
		sObjectFieldAliasCache:       newSObjectFieldAliasLookupCache(),
		fieldResolveCache:            newFieldResolveLookupCache(),
		loadedChildRelCache:          newLoadedChildRelationshipLookupCache(),
		lazyChildRelCache:            newLazyChildRelationshipLookupCache(),
		objectNameCache:              make(map[string]objectNameLookup),
		recentlyViewed:               make(map[string]map[storage.ID]recentlyViewedEntry),
	}
}

// CloneRuntime returns a fresh VM with the same registered Apex methods,
// classes, and triggers. Mutable runtime state such as globals, limits, org
// state, current user, and static field values remains request-local.
func (vm *VM) CloneRuntime(stdout io.Writer) *VM {
	clone := New(stdout)
	// Methods, MethodOverloads, MethodFolded, and Triggers are compiled
	// artifacts that are only mutated by Register*/unregister* at setup, never
	// during execution. Share the maps by pointer and mark them shared so
	// CloneRuntime allocates nothing for them. The (rare) post-clone
	// registration path copies-on-write via ensureRuntimeArtifactsOwned, so the
	// source VM (and any cached runtime template) is never corrupted. Per-test
	// clones never register, so they keep sharing read-only across parallel
	// workers, which is safe for concurrent map reads.
	clone.Methods = vm.Methods
	clone.MethodOverloads = vm.MethodOverloads
	clone.MethodFolded = vm.MethodFolded
	clone.Triggers = vm.Triggers
	clone.runtimeArtifactsShared = true
	if vm.sharedClassCopyPlan != nil {
		clone.Classes = copyClassMapWithPlan(vm.Classes, vm.sharedClassCopyPlan)
		clone.sharedClassCopyPlan = vm.sharedClassCopyPlan
	} else {
		clone.Classes = copyClassMap(vm.Classes)
	}
	// classLookup is a case/alias-normalized index over Classes. The canonical
	// key -> live Classes key mapping is pure compiled metadata, identical for
	// every clone of the same base. When the base has been frozen
	// (FreezeClassLookup), share the immutable string index by pointer and skip
	// the per-clone rebuild (re-canonicalizing ~2x len(Classes) keys). Per-test
	// clones resolve through their own Classes map, so clone-local static field
	// state stays isolated. The (setup-only) registration path copies-on-write
	// via unshareClassLookup, so the shared index is never mutated.
	if vm.sharedClassLookupKeys != nil {
		clone.sharedClassLookupKeys = vm.sharedClassLookupKeys
		clone.classLookup = nil
		clone.classNameSearchCache = vm.classNameSearchCache
		clone.topLevelClassLookup = vm.topLevelClassLookup
	} else {
		clone.rebuildClassLookup()
	}
	// triggerMatchCache is computed from Triggers; share the pointer so we
	// only populate it once across all clones in a run. Concurrent test
	// methods are protected by the cache's RWMutex.
	clone.triggerMatchCache = vm.triggerMatchCache
	// Relationship describe caches depend only on immutable schema metadata.
	// Share them only when a schema stamp proves the shape. A later SetOrg or
	// metadata overlay mutation that changes the stamp clears these pointers and
	// gives the clone private caches before it answers from stale metadata.
	clone.metadataCacheStamp = vm.metadataCacheStamp
	if strings.TrimSpace(vm.metadataCacheStamp) != "" {
		clone.jsonChildRelTypeCache = vm.jsonChildRelTypeCache
		clone.childRelCache = vm.childRelCache
		clone.childRelationshipLookupCache = vm.childRelationshipLookupCache
		clone.sObjectFieldAliasCache = vm.sObjectFieldAliasCache
		clone.fieldResolveCache = vm.fieldResolveCache
		clone.dmlSummaryByChild = vm.dmlSummaryByChild
		clone.loadedChildRelCache = vm.loadedChildRelCache
		clone.lazyChildRelCache = vm.lazyChildRelCache
	} else {
		clone.jsonChildRelTypeCache = newJSONChildRelTypeLookupCache()
		clone.childRelCache = newChildRelationshipCache()
		clone.childRelationshipLookupCache = newChildRelationshipLookupCache()
		clone.sObjectFieldAliasCache = newSObjectFieldAliasLookupCache()
		clone.fieldResolveCache = newFieldResolveLookupCache()
		clone.dmlSummaryByChild = dml.NewSummaryRelationCache()
		clone.loadedChildRelCache = newLoadedChildRelationshipLookupCache()
		clone.lazyChildRelCache = newLazyChildRelationshipLookupCache()
	}
	clone.traceEnabled = vm.traceEnabled
	clone.toolingExecuteAnonymous = vm.toolingExecuteAnonymous
	clone.staticInitState = copyStaticInitStateMap(vm.staticInitState)
	clone.pageReferences = copyStringMap(vm.pageReferences)
	clone.platformCache = copyCacheMap(vm.platformCache)
	clone.managedFeatureValues = copyValueMap(vm.managedFeatureValues)
	clone.isolationJournal = vm.isolationJournal
	return clone
}

// ensureRuntimeArtifactsOwned performs the copy-on-write step for the compiled
// method and trigger maps shared by CloneRuntime. The first mutation after a
// clone (only the setup-time Register*/unregister* path reaches here) gives this
// VM private copies so the shared source maps stay intact. Per-test clones never
// register, so the flag remains set and no copy is made.
func (vm *VM) ensureRuntimeArtifactsOwned() {
	if vm == nil || !vm.runtimeArtifactsShared {
		return
	}
	vm.runtimeArtifactsShared = false
	vm.Methods = copyMethodMap(vm.Methods)
	vm.MethodOverloads = copyMethodSliceMap(vm.MethodOverloads)
	vm.MethodFolded = copyMethodSliceMap(vm.MethodFolded)
	vm.Triggers = copyTriggerSliceMap(vm.Triggers)
}

func (vm *VM) SetIsolationJournal(journal *storage.IsolationJournal) {
	if vm != nil {
		vm.isolationJournal = journal
	}
}

func (vm *VM) recordIsolationJournalMutation(objectName string, id storage.ID, before storage.Record, exists bool) {
	if vm == nil || vm.isolationJournal == nil || objectName == "" || id == "" {
		return
	}
	if exists {
		vm.isolationJournal.RecordUpdate(objectName, id, before)
		return
	}
	vm.isolationJournal.RecordInsert(objectName, id)
}

func (vm *VM) recordIsolationJournalSequence(objectName string) {
	if vm == nil || vm.isolationJournal == nil || objectName == "" {
		return
	}
	vm.isolationJournal.RecordSequence(objectName)
}

func (vm *VM) SetTraceEnabled(enabled bool) {
	if vm != nil {
		vm.traceEnabled = enabled
	}
}

func (vm *VM) SetDebugOutputSink(sink func(DebugEvent)) {
	if vm != nil {
		vm.debugOutputSink = sink
	}
}

func (vm *VM) SetToolingExecuteAnonymous(enabled bool) {
	if vm != nil {
		vm.toolingExecuteAnonymous = enabled
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

var classCopyDedupPool = sync.Pool{
	New: func() any { return make(map[string]Class) },
}

// classCopyPlan precomputes the per-clone class-copy work that is identical for
// every clone of a frozen base machine: which alias names own a fresh copyClass
// (primaries) and which alias names share an already-copied class (aliases ->
// primary). This avoids rebuilding the canonical-dedup map and re-running
// classCopyKey (strings.ToLower) on every per-test clone. It is pure compiled
// metadata; per-test static isolation is preserved because copyClass still
// copies each primary's mutable StaticFields per clone, and aliases share the
// same copied Class exactly as the unplanned path does.
type classCopyPlan struct {
	primaries []string
	aliases   map[string]string
}

func buildClassCopyPlan(in map[string]Class) *classCopyPlan {
	plan := &classCopyPlan{
		primaries: make([]string, 0, len(in)),
		aliases:   make(map[string]string),
	}
	primaryByCanonical := make(map[string]string, len(in))
	for name, class := range in {
		canonical := classCopyKey(name, class)
		if primary, ok := primaryByCanonical[canonical]; ok {
			plan.aliases[name] = primary
			continue
		}
		primaryByCanonical[canonical] = name
		plan.primaries = append(plan.primaries, name)
	}
	return plan
}

func copyClassMapWithPlan(in map[string]Class, plan *classCopyPlan) map[string]Class {
	out := make(map[string]Class, len(in))
	for _, name := range plan.primaries {
		out[name] = copyClass(in[name])
	}
	for alias, primary := range plan.aliases {
		out[alias] = out[primary]
	}
	return out
}

func copyClassMap(in map[string]Class) map[string]Class {
	out := make(map[string]Class, len(in))
	// byCanonicalName dedups aliases that resolve to the same class so every
	// alias entry shares one copied Class (and thus one mutable StaticFields
	// map, preserving per-test static isolation across aliases). It is transient
	// per clone, so it is pooled to avoid re-growing a large backing array on
	// every test clone.
	byCanonicalName := classCopyDedupPool.Get().(map[string]Class)
	for name, class := range in {
		canonical := classCopyKey(name, class)
		copied, ok := byCanonicalName[canonical]
		if !ok {
			copied = copyClass(class)
			byCanonicalName[canonical] = copied
		}
		out[name] = copied
	}
	clear(byCanonicalName)
	classCopyDedupPool.Put(byCanonicalName)
	return out
}

func copyClass(class Class) Class {
	// Instance Fields are immutable templates after registration (construction
	// clones their values into each new instance), so the map is shared by
	// reference across clones. StaticFields hold mutable per-test static state
	// and may even gain entries at runtime, so they are always copied to keep
	// each test isolated.
	class.StaticFields = copyFieldMap(class.StaticFields)
	return class
}

func copyFieldMap(in map[string]Field) map[string]Field {
	if len(in) == 0 {
		// Classes with no static fields (the common case across a large
		// codebase) would otherwise allocate an empty map per class on every
		// per-test clone. Return nil: reads (range/lookup) behave identically,
		// and the runtime static-write paths already nil-check and allocate on
		// demand, so this only removes wasted allocations.
		return nil
	}
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

func copyValueMap(in map[string]Value) map[string]Value {
	if in == nil {
		return nil
	}
	out := make(map[string]Value, len(in))
	for key, value := range in {
		out[key] = cloneValue(value)
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
	if hasPrefixFold(name, "page.") {
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
	vm.siteExperienceID = ""
}

func (vm *VM) SnapshotVisualforcePageContext() VisualforcePageContext {
	if vm == nil {
		return VisualforcePageContext{}
	}
	return VisualforcePageContext{
		CurrentPage:   cloneValue(vm.currentPage),
		PageMessages:  cloneValueSlice(vm.pageMessages),
		ActionInvoker: vm.vfActionInvoker,
	}
}

func (vm *VM) RestoreVisualforcePageContext(ctx VisualforcePageContext) {
	if vm == nil {
		return
	}
	vm.currentPage = cloneValue(ctx.CurrentPage)
	vm.pageMessages = cloneValueSlice(ctx.PageMessages)
	vm.vfActionInvoker = ctx.ActionInvoker
}

func (vm *VM) CurrentPage() Value {
	if vm == nil || vm.currentPage.Kind == "" {
		return Null
	}
	return vm.currentPage
}

func (vm *VM) SetVisualforceActionInvoker(invoker VisualforceActionInvoker) {
	if vm == nil {
		return
	}
	vm.vfActionInvoker = invoker
}

func (vm *VM) ClearVisualforceActionInvoker() {
	if vm == nil {
		return
	}
	vm.vfActionInvoker = nil
}

func (vm *VM) PageMessages() []Value {
	if vm == nil {
		return nil
	}
	return vm.pageMessages
}

func (vm *VM) SetCurrentPageURL(rawURL string) {
	if vm == nil {
		return
	}
	vm.currentPage = vm.newPageReference(rawURL)
}

func (vm *VM) SetCurrentPageURLNull() {
	if vm == nil {
		return
	}
	page := vm.newPageReference("")
	page.Fields["url"] = typedNull("String")
	vm.currentPage = page
}

func (vm *VM) SetOrg(org *storage.OrgState) {
	currentStamp := vm.schemaCacheStamp()
	nextStamp := trustedSchemaCacheStampForOrg(org)
	schemaChanged := currentStamp != "" && nextStamp != "" && currentStamp != nextStamp
	if vm.Org != org || schemaChanged {
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
	return trustedSchemaCacheStampForOrg(vm.Org)
}

// PrimeMetadataSchema records the schema stamp for org without touching the org
// or clearing caches. Priming the base machine before cloning lets every clone
// inherit a non-empty stamp so SetOrg reuses the shared schema-describe caches
// (CloneRuntime) instead of clearing them on each clone's first SetOrg.
func (vm *VM) PrimeMetadataSchema(org *storage.OrgState) {
	if vm == nil {
		return
	}
	if stamp := trustedSchemaCacheStampForOrg(org); stamp != "" {
		vm.metadataCacheStamp = stamp
	}
}

func PrimeRuntimeTemplateSchema(template *storage.RuntimeTemplate) {
	if template == nil {
		return
	}
	stamp := schemaCacheStampForOrg(&template.Org)
	template.RuntimeSchemaStamp = stamp
	template.Org.RuntimeSchemaStamp = stamp
}

func trustedSchemaCacheStampForOrg(org *storage.OrgState) string {
	if org == nil {
		return ""
	}
	if stamp := strings.TrimSpace(org.RuntimeSchemaStamp); stamp != "" {
		return stamp
	}
	return schemaCacheStampForOrg(org)
}

const (
	schemaStampFNVOffset uint64 = 1469598103934665603
	schemaStampFNVPrime  uint64 = 1099511628211
)

func schemaStampHashByte(h uint64, b byte) uint64 {
	return (h ^ uint64(b)) * schemaStampFNVPrime
}

func schemaStampHashRaw(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h = (h ^ uint64(s[i])) * schemaStampFNVPrime
	}
	return h
}

// schemaStampHash hashes strings.TrimSpace(s) verbatim into the running stamp.
func schemaStampHash(h uint64, s string) uint64 {
	return schemaStampHashRaw(h, strings.TrimSpace(s))
}

// schemaStampHashLower hashes the lowercased, trimmed form of s, matching the
// previous strings.ToLower(strings.TrimSpace(s)) stamp content byte-for-byte.
// ASCII (the universal case for schema identifiers) is lowered inline without
// allocation; any non-ASCII input falls back to strings.ToLower so the hashed
// byte stream stays identical to the original stamp.
func schemaStampHashLower(h uint64, s string) uint64 {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return schemaStampHashRaw(h, strings.ToLower(s))
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		h = (h ^ uint64(c)) * schemaStampFNVPrime
	}
	return h
}

// schemaCacheStampForOrg returns a compact content fingerprint of the org schema
// used only for equality comparison (schema-unchanged detection for the
// metadata describe caches). It streams the same canonical content the previous
// implementation serialized, but accumulates it into a 64-bit FNV-1a hash
// instead of materializing a multi-megabyte string on every SetOrg, eliminating
// the dominant allocation on the per-test clone path. Distinct schemas yield
// distinct stamps; the comparison semantics are unchanged.
func schemaCacheStampForOrg(org *storage.OrgState) string {
	if org == nil {
		return ""
	}
	h := schemaStampFNVOffset
	h = schemaStampHashLower(h, org.Namespace)
	h = schemaStampHashByte(h, '|')
	objectNames := make([]string, 0, len(org.Objects))
	for objectName, object := range org.Objects {
		if strings.TrimSpace(object.Definition.APIName) == "" {
			continue
		}
		objectNames = append(objectNames, objectName)
	}
	sort.Strings(objectNames)
	for _, objectName := range objectNames {
		object := org.Objects[objectName]
		definition := object.Definition
		h = schemaStampHashLower(h, objectName)
		h = schemaStampHashByte(h, '=')
		h = schemaStampHashLower(h, definition.APIName)
		h = schemaStampHashByte(h, ',')
		h = schemaStampHash(h, definition.KeyPrefix)
		h = schemaStampHashByte(h, ',')
		h = schemaStampHash(h, definition.Label)
		h = schemaStampHashByte(h, ',')
		h = schemaStampHash(h, definition.PluralLabel)
		h = schemaStampHashByte(h, ';')

		fieldNames := make([]string, 0, len(definition.Fields))
		for fieldName := range definition.Fields {
			fieldNames = append(fieldNames, fieldName)
		}
		sort.Strings(fieldNames)
		for _, fieldName := range fieldNames {
			field := definition.Fields[fieldName]
			h = schemaStampHashLower(h, fieldName)
			h = schemaStampHashByte(h, ':')
			h = schemaStampHash(h, field.APIName)
			h = schemaStampHashByte(h, ':')
			h = schemaStampHashRaw(h, string(field.Type))
			h = schemaStampHashByte(h, ':')
			h = schemaStampHash(h, field.RelationshipName)
			h = schemaStampHashByte(h, ':')
			h = schemaStampHash(h, field.ChildRelationshipName)
			h = schemaStampHashByte(h, ':')
			h = schemaStampHashReferenceList(h, field.ReferenceTo)
			h = schemaStampHashByte(h, ';')
		}

		for _, relation := range definition.Relations {
			h = schemaStampHash(h, relation.Field)
			h = schemaStampHashByte(h, ':')
			h = schemaStampHash(h, relation.ParentRelationship)
			h = schemaStampHashByte(h, ':')
			h = schemaStampHash(h, relation.ChildRelationship)
			h = schemaStampHashByte(h, ':')
			h = schemaStampHashReferenceList(h, relation.ParentObjects)
			h = schemaStampHashByte(h, ';')
		}
		h = schemaStampHashRaw(h, strconv.Itoa(len(definition.RecordTypes)))
		h = schemaStampHashByte(h, '|')
	}
	return strconv.FormatUint(h, 16)
}

// schemaStampHashReferenceList hashes a sorted, comma-joined string list,
// matching strings.Join(sortedStrings(values), ",") byte-for-byte without
// allocating the joined string.
func schemaStampHashReferenceList(h uint64, values []string) uint64 {
	sorted := sortedStrings(values)
	for i, value := range sorted {
		if i > 0 {
			h = schemaStampHashByte(h, ',')
		}
		h = schemaStampHashRaw(h, value)
	}
	return h
}

func sortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
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

func (vm *VM) SetCurrentNamespace(namespace string) {
	vm.currentNamespace = strings.TrimSpace(namespace)
}

func (vm *VM) SetContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	vm.ctx = ctx
}

func (vm *VM) newDMLEngine(result *Result) dml.Engine {
	engine := dml.NewEngine(vm.Org)
	if vm.dmlSummaryByChild == nil {
		vm.dmlSummaryByChild = dml.NewSummaryRelationCache()
	}
	engine.SummaryByChild = vm.dmlSummaryByChild
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

func (vm *VM) applyBeforeSaveFlows(records []storage.Record, result *Result) error {
	if len(records) == 0 {
		return nil
	}
	engine := vm.newDeferredAutomationDMLEngine(result)
	return engine.ApplyBeforeSaveFlows(records)
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

func (vm *VM) applyDeferredAutomation(engine *dml.Engine, records, oldRecords []storage.Record, allOrNone bool, rollback vmDMLRollbackPoint, result *Result) error {
	if engine == nil {
		return nil
	}
	sideEffects := vm.snapshotSideEffects()
	for i, record := range records {
		if record.Object == "" || record.ID == "" {
			continue
		}
		if result != nil {
			appendTrace(result, "apex.automation.apply", "apex.automation", map[string]any{
				"object": record.Object,
				"id":     string(record.ID),
			})
		}
		outcome, err := engine.ApplyAutomation(record.Object, record.ID)
		if err != nil {
			if allOrNone {
				vm.restoreDMLRollbackPoint(rollback)
				vm.restoreSideEffects(sideEffects)
				appendTrace(result, "apex.automation.rollback", "apex.automation", map[string]any{
					"object": record.Object,
					"id":     string(record.ID),
					"reason": err.Error(),
				})
			}
			return err
		}
		if outcome.WorkflowUpdated || outcome.FlowUpdated {
			oldRecord := record
			if i < len(oldRecords) {
				oldRecord = oldRecords[i]
			}
			if err := vm.refireAutomationUpdateTriggers(record.Object, record.ID, oldRecord, allOrNone, rollback, result); err != nil {
				if allOrNone {
					vm.restoreDMLRollbackPoint(rollback)
					vm.restoreSideEffects(sideEffects)
				}
				return err
			}
		}
	}
	return nil
}

func (vm *VM) refireAutomationUpdateTriggers(objectName string, id storage.ID, oldRecord storage.Record, allOrNone bool, rollback vmDMLRollbackPoint, result *Result) error {
	if vm == nil || vm.Org == nil || objectName == "" || id == "" {
		return nil
	}
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return nil
	}
	_, stored, ok := storage.LookupRecordByID(object.Records, id)
	if !ok {
		return nil
	}
	stored.Object = objectName
	if oldRecord.Object == "" {
		oldRecord.Object = objectName
	}
	if oldRecord.ID == "" {
		oldRecord.ID = id
	}
	before := []storage.Record{oldRecord}
	triggerRecords := vm.hydrateUpdateTriggerRecords([]storage.Record{stored.Clone()}, before)
	vm.applyBeforeDMLDerivedFields(triggerRecords)
	failures, err := vm.runTriggers(triggerTimingBefore, "update", triggerRecords, before, result)
	if err != nil {
		return dmlExceptionFromTriggerError("update", err)
	}
	if hasDMLFailures(failures) {
		if allOrNone {
			vm.restoreDMLRollbackPoint(rollback)
		}
		return fmt.Errorf("workflow update trigger failed for %s: %s", objectName, failures[0].Error)
	}
	if err := vm.storeTriggerRecords(objectName, triggerRecords); err != nil {
		return err
	}
	if _, err := vm.runTriggers(triggerTimingAfter, "update", triggerRecords, before, result); err != nil {
		return dmlExceptionFromTriggerError("update", err)
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
	vm.ensureRuntimeArtifactsOwned()
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
	vm.triggerMatchCache.reset()
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
	vm.testContext.SeeAllDataSet = true
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
	resetClasses := make(map[string]Class)
	for className, class := range vm.Classes {
		if len(class.StaticFields) == 0 {
			continue
		}
		resetAny := classHasStaticCollectionField(class)
		if resetAny {
			for fieldName, field := range class.StaticFields {
				if !resetTestAsyncStaticField(field) && !resetTestAsyncStaticFieldForReinitialization(class, field) {
					continue
				}
				field.Value = defaultStaticCollectionFieldValue(className, fieldName, field)
				class.StaticFields[fieldName] = field
			}
		}
		vm.Classes[className] = class
		if resetAny {
			resetClasses[runtimeClassName(class)] = class
		}
	}
	for _, class := range resetClasses {
		vm.markStaticInitializationUninitialized(class)
	}
	vm.invalidateStaticValueRefs()
	return nil
}

func classHasStaticCollectionField(class Class) bool {
	for _, field := range class.StaticFields {
		if isStaticCollectionField(field) {
			return true
		}
	}
	return false
}

func resetTestAsyncStaticField(field Field) bool {
	return isStaticCollectionField(field)
}

func resetTestAsyncStaticFieldForReinitialization(class Class, field Field) bool {
	return classHasStaticCollectionField(class) &&
		field.Getter == nil &&
		staticFieldHasDefaultNullInitialValue(field) &&
		staticFieldIsBoolean(field)
}

func staticFieldHasDefaultNullInitialValue(field Field) bool {
	if field.InitialValue.Kind == "" {
		return true
	}
	if field.InitialValue.Kind != ValueNull {
		return false
	}
	if field.InitialValue.Type == "" {
		return true
	}
	return strings.EqualFold(field.InitialValue.Type, field.Type)
}

func staticFieldIsBoolean(field Field) bool {
	return strings.EqualFold(strings.TrimSpace(field.Type), "Boolean")
}

func (vm *VM) markStaticInitializationUninitialized(class Class) {
	if vm.staticInitState == nil {
		return
	}
	canonical := runtimeClassName(class)
	if canonical != "" {
		delete(vm.staticInitState, canonical)
	}
	if class.Name != "" {
		delete(vm.staticInitState, class.Name)
	}
}

func defaultStaticCollectionFieldValue(className, fieldName string, field Field) Value {
	return defaultStaticFieldValue(className, fieldName, field.Type, field.InitialValue)
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
		appendTraceLazy(&result, "apex.limits", "apex.limits", func() map[string]any {
			return limitTraceArgs(vm.limits)
		})
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
	var out execOutcome
	_, err = vm.withQueueableDuplicateSignatureTransaction(func() (Value, error) {
		var runErr error
		out, runErr = vm.executeProgram(program, &result)
		return Null, runErr
	})
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

func (vm *VM) AdvanceDeterministicTime(delta time.Duration) {
	vm.fakeNow = vm.fakeNow.Add(delta)
}

func (vm *VM) DrainAsync(result *Result) error {
	if vm.testContext != nil {
		return vm.drainTestAsync(result)
	}
	return vm.drainLocalAsync(result)
}

type controlSignal string
