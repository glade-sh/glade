package vm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	frozenClassLookup        *frozenClassLookup
	sharedClassCopyPlan      *classCopyPlan
	classLookupGeneration    uint64
	classLookupNameCache     map[string]classLookupNameResult
	classLookupNameOrder     []string
	classLookupNameBytes     int
	classLookupNameStats     classLookupNameCacheStats
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
	ownedStaticClasses       map[string]bool
	sharedStaticClasses      bool
	// classMapShared marks a Classes map that another VM may also hold (a
	// frozen-shared clone and its source). The first write copies the map.
	// classMapWritten records any write since this VM was built, after which
	// the frozen copy plan may no longer describe the map's alias values.
	classMapShared  atomic.Bool
	classMapWritten bool
	// classOverlay holds registrations made on top of a frozen generation
	// until the next FreezeClassLookup (see class_lookup_overlay.go).
	classOverlay *classLookupOverlay
	// classValuesWritten records a class value write outside registration
	// since the last freeze. Such writes keep the full rebuild path.
	classValuesWritten bool
	// classLookupBuilds counts O(len(Classes)) class-lookup builds on this VM.
	classLookupBuilds int
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
	currentTrigger          bool
	eventBusTriggerContext  *eventBusTriggerContext
	eventBusReplaySequence  int64
	entrySharingMode        string
	reflectionConstructType string
	testContext             *TestContext
	localAsyncJobs          []AsyncJob
	localAsyncSeq           int
	localAsyncDrain         bool
	localAsyncChain         bool
	rejectAsyncActions      bool
	asyncActionViolation    bool
	executionUser           Value
	// Captured once from the runner's initial org, before test setup inserts.
	// Clones share this immutable ID set; EnableTestContext must not recapture it.
	testDocumentBaseline map[storage.ID]struct{}
	// Traversals belong to this VM, independently of method frames and clones.
	activeCollectionTraversals map[collectionTraversalKey]int
	// Auth validation failures belong to this runtime, never a shared template.
	authTotpFailures map[string]int
	// --- Governor limits ---
	limits          Limits
	limitCaps       LimitCaps
	limitMode       LimitMode
	limitViolations []LimitViolation
	cpuBudgetUsed   int
	cpuStartedAt    time.Time
	cpuRunning      bool
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
	pageMessages              []Value
	currentPage               Value
	vfActionInvoker           VisualforceActionInvoker
	vfStandardControllerReset func(string) (Value, error)
	vfDeleteConfirmationToken func(string) (string, error)
	pageReferences            map[string]string
	siteExperienceID          string
	// --- SOQL / search results and platform cache ---
	// Cursor JSON tokens resolve only within this VM. Allocate on first use;
	// cloneRuntime starts with a fresh registry through newVM.
	queryCursors map[string]Value
	// Alternate query sources belong to this VM, including cursor JSON copies.
	queryHandleSources    map[string]*storage.OrgState
	fixedSearchResults    []Value
	fixedSearchResultsSet bool
	sfsqlqueryRows        []Value
	sfsqlqueryMetadata    []Value
	platformCache         map[string]map[string]cacheEntry
	cacheScanLocators     map[string][]cacheScanItem
	cacheScanSeq          int
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
	debugHooks                  DebugHooks
	hasDebugHooks               bool
	debugOutputSink             func(DebugEvent)
	traceEnabled                bool
	ctx                         context.Context
	activeGetters               map[string]int
	activeSetters               map[string]int
	triggerGlobals              map[string]Value
	frameworkDomainTriggerState map[string][]Value
	cryptoRandomSeq             uint64
	// uuidRandomReader is a per-VM test seam; nil selects crypto/rand.Reader.
	uuidRandomReader     io.Reader
	staticInitState      map[string]staticInitState
	frameworkIDSequences map[string]uint64
	lastAmbiguous        *overloadDiagnostic
	activeConstructors   map[string]int
	// --- Describe caches ---
	describeCache                map[string]Value
	fieldDescribeCache           map[string]Value
	globalDescribeCache          *Value
	describeTabsCache            *Value
	describeDefCache             map[string]storage.ObjectDefinition
	customDataCache              map[string]Value
	soqlExecutionCache           *soql.ExecutionCache
	dmlSummaryByChild            *dml.SummaryRelationCache
	summarySideEffectObjects     *summarySideEffectObjectCache
	summarySideEffectIndex       *summarySideEffectIndexMemo
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
	staticValueRefs       map[uint64]bool
	staticValueRefFields  map[uint64]staticFieldRefSet
	staticValueRefsShared bool
	// Serializes lazy collection/publication by concurrent template clones.
	// Executing a VM concurrently with cloning it is still unsupported.
	staticValueRefsMu         sync.Mutex
	staticAliasChildHints     map[staticAliasChildHintKey]staticAliasChildHint
	staticAliasDirectChildren map[staticFieldRef]staticAliasDirectChildIndex
	localOnlyCollectionRefs   map[uint64]bool
	localOnlyObjectRefs       map[uint64]bool
	// Exceptions to the ordinary SObject field graph are append-only. Unlike
	// local-only provenance, they must survive runtime/static cloning.
	sObjectCollectionAliasRefs  map[uint64]bool
	sObjectAliasTypes           map[string]sObjectAliasTypeVerdict
	sObjectAliasTypeOrg         *storage.OrgState
	collectionMutationSeq       uint64
	aliasContainmentMutationSeq uint64
	aliasContainmentCache       map[aliasContainmentCacheKey]uint64
	frameworkRecorderRollback   *frameworkMethodCountRecorderRollback
	perfRecorder                *PerfRecorder
	classLookupPerf             *classLookupPerfShard
	scopeAliasTraversalObserver scopeAliasTraversalObserver
	runtimeArtifactsShared      bool
	methodOverlay               *methodTableOverlay
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
	Alias      string
	Generation uint64
	OK         bool
}

type classLookupNameCacheStats struct {
	Hits          uint64
	Misses        uint64
	Evictions     uint64
	Entries       int
	RetainedBytes int
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
	Kind                         string            `json:"kind"`
	ToAddresses                  []string          `json:"toAddresses,omitempty"`
	CcAddresses                  []string          `json:"ccAddresses,omitempty"`
	BccAddresses                 []string          `json:"bccAddresses,omitempty"`
	TargetObjectIDs              []string          `json:"targetObjectIds,omitempty"`
	WhatIDs                      []string          `json:"whatIds,omitempty"`
	Subject                      string            `json:"subject,omitempty"`
	PlainTextBody                string            `json:"plainTextBody,omitempty"`
	HTMLBody                     string            `json:"htmlBody,omitempty"`
	TemplateID                   string            `json:"templateId,omitempty"`
	TargetObjectID               string            `json:"targetObjectId,omitempty"`
	WhatID                       string            `json:"whatId,omitempty"`
	SaveAsActivity               bool              `json:"saveAsActivity,omitempty"`
	FileAttachments              []string          `json:"fileAttachments,omitempty"`
	EntityAttachments            []string          `json:"entityAttachments,omitempty"`
	DocumentAttachments          []string          `json:"documentAttachments,omitempty"`
	ReplyTo                      string            `json:"replyTo,omitempty"`
	SenderDisplayName            string            `json:"senderDisplayName,omitempty"`
	Charset                      string            `json:"charset,omitempty"`
	CustomHeaders                map[string]string `json:"customHeaders,omitempty"`
	OrgWideEmailAddressID        string            `json:"orgWideEmailAddressId,omitempty"`
	OptOutPolicy                 string            `json:"optOutPolicy,omitempty"`
	EmailPriority                string            `json:"emailPriority,omitempty"`
	BccSender                    bool              `json:"bccSender,omitempty"`
	UseSignature                 bool              `json:"useSignature,omitempty"`
	TreatBodiesAsTemplate        bool              `json:"treatBodiesAsTemplate,omitempty"`
	TreatTargetObjectAsRecipient bool              `json:"treatTargetObjectAsRecipient,omitempty"`
	TriggerUserEmail             bool              `json:"triggerUserEmail,omitempty"`
	TriggerOtherEmail            bool              `json:"triggerOtherEmail,omitempty"`
	TriggerAutoResponseEmail     bool              `json:"triggerAutoResponseEmail,omitempty"`
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
	// Preserve the Apex message before adding local diagnostic context.
	exceptionMessage *string
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

// ExceptionMessage returns the Apex message without local diagnostic context.
func (e *RuntimeError) ExceptionMessage() string {
	if e.exceptionMessage != nil {
		return *e.exceptionMessage
	}
	return e.Message
}

func unsupportedCallError(callee string) error {
	return &RuntimeError{Type: "UnsupportedFeature", Message: fmt.Sprintf("unsupported call %q", callee)}
}

type callFrame struct {
	Symbol      string
	File        string
	Line        int
	Column      int
	APIVersion  string
	SharingMode string
}

type TestContext struct {
	Started                  bool
	Stopped                  bool
	CurrentUser              Value
	SeeAllData               bool
	SeeAllDataSet            bool
	AsyncJobs                []AsyncJob
	AsyncStartIndex          int
	FlexQueueJobs            []Value
	EventPublishes           []eventPublishCallback
	PlatformEvents           []storage.Record
	PlatformEventRetries     map[string]int
	PlatformEventPublishes   int
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
	ID                  string
	Kind                string
	Object              Value
	Method              Method
	Args                []Value
	BatchSize           int
	Name                string
	Cron                string
	ParentJobID         string
	LastProcessed       string
	LastProcessedOffset int
	Deferred            bool
	// Scheduled test payloads outlive cancellation but require an active replacement to run.
	ScheduledAborted            bool
	ScheduledReusedBy           string
	SuppressWorkerRecords       bool
	QueueableDepth              int
	QueueableMaxDepth           int
	QueueableDelayMinutes       int
	QueueableDuplicateSignature string
	NotBefore                   time.Time
}

type cacheEntry struct {
	Immutable    bool
	Value        Value
	SecondaryKey string
	ExpireAt     time.Time
}

type cacheScanItem struct {
	Key   string
	Value Value
}

type Trigger struct {
	Name       string
	Namespace  string
	Object     string
	Timing     string
	Operation  string
	APIVersion string
	Program    ir.Program
	File       string
	Line       int
	Column     int
}

func New(stdout io.Writer) *VM {
	return newVM(stdout, compatibilityPerfRecorder.Load())
}

func newVM(stdout io.Writer, recorder *PerfRecorder) *VM {
	warmGeneratedPlatformRuntimeIndexes()
	machine := &VM{
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
		activeCollectionTraversals:   make(map[collectionTraversalKey]int),
		limitCaps:                    defaultLimitCaps(),
		limitMode:                    LimitModePermissive,
		fakeNow:                      time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		queueableDuplicateSignatures: make(map[string]string),
		savepoints:                   make(map[string]storage.OrgState),
		savepointMarks:               make(map[string]storage.IsolationMark),
		emailSavepoints:              make(map[string][]CapturedEmail),
		savepointOrder:               make(map[string]int),
		platformCache:                make(map[string]map[string]cacheEntry),
		queryCursors:                 nil,
		queryHandleSources:           nil,
		cacheScanLocators:            make(map[string][]cacheScanItem),
		metadataDeploys:              make(map[string]Value),
		reportInstances:              make(map[string]Value),
		subMgmtTestRecords:           make(map[string]Value),
		traceEnabled:                 true,
		ctx:                          context.Background(),
		activeGetters:                make(map[string]int),
		activeSetters:                make(map[string]int),
		frameworkIDSequences:         make(map[string]uint64),
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
	machine.SetPerfRecorder(recorder)
	return machine
}

func (vm *VM) SetPerfRecorder(recorder *PerfRecorder) {
	if vm == nil {
		return
	}
	if vm.perfRecorder == recorder && (recorder == nil || vm.classLookupPerf != nil) {
		return
	}
	vm.perfRecorder = recorder
	vm.classLookupPerf = recorder.newClassLookupPerfShard()
}

// CloneRuntime returns a fresh VM with the same registered Apex methods,
// classes, and triggers. Mutable runtime state such as globals, limits, org
// state, current user, and static field values remains request-local.
func (vm *VM) CloneRuntime(stdout io.Writer) *VM {
	return vm.cloneRuntime(stdout, false)
}

// CloneRuntimeFrozenShared returns an isolated runtime while sharing frozen
// static templates until their first runtime access. The caller must freeze
// class lookup after registration. Unfrozen machines fall back to CloneRuntime.
func (vm *VM) CloneRuntimeFrozenShared(stdout io.Writer) *VM {
	if vm == nil || vm.frozenClassLookup == nil {
		return vm.CloneRuntime(stdout)
	}
	return vm.cloneRuntime(stdout, true)
}

func (vm *VM) cloneRuntime(stdout io.Writer, shareFrozenStatics bool) *VM {
	clone := newVM(stdout, vm.perfRecorder)
	// Methods, MethodOverloads, MethodFolded, and Triggers are compiled
	// artifacts that are only mutated by Register*/unregister* at setup, never
	// during execution. Share the maps by pointer and mark them shared so
	// CloneRuntime allocates nothing for them. Post-clone RegisterMethod writes
	// to a clone-local overlay; unregister and trigger registration copy-on-write
	// via ensureRuntimeArtifactsOwned, so the source VM (and any cached runtime
	// template) is never corrupted. The shared maps stay read-only across
	// parallel workers, which is safe for concurrent map reads.
	clone.Methods = vm.Methods
	clone.MethodOverloads = vm.MethodOverloads
	clone.MethodFolded = vm.MethodFolded
	clone.Triggers = vm.Triggers
	clone.runtimeArtifactsShared = true
	clone.methodOverlay = vm.methodOverlay.clone()
	// Alias-escape tracking indexes populated Platform Cache entries as alias roots
	// in the static-root index under the partition name. A frozen clone copies
	// cache values to fresh identities, so sharing the template's static refs
	// would otherwise share an index containing the source runtime's refs. Clones
	// with a populated cache therefore collect their own index; the empty-cache
	// fast path is unchanged.
	shareStaticValueRefs := shareFrozenStatics && len(vm.platformCache) == 0
	if shareFrozenStatics && vm.canShareClassMap() {
		// The source map already holds what the planned copy would build:
		// every alias carries its primary's value and statics stay shared
		// until first access. Share it by pointer and copy on first write.
		if !vm.classMapShared.Load() {
			vm.classMapShared.Store(true)
		}
		clone.Classes = vm.Classes
		clone.classMapShared.Store(true)
		clone.sharedClassCopyPlan = vm.sharedClassCopyPlan
		clone.sharedStaticClasses = true
	} else if shareFrozenStatics && vm.sharedClassCopyPlan != nil {
		clone.Classes = copyClassMapSharedStaticsWithPlan(vm.Classes, vm.sharedClassCopyPlan)
		clone.sharedClassCopyPlan = vm.sharedClassCopyPlan
		clone.sharedStaticClasses = true
		// A planned copy can normalize divergent aliases. Only reuse the
		// source index if that leaves every static location unchanged.
		for name, class := range clone.Classes {
			source := vm.Classes[name]
			if !sameMap(class.StaticFields, source.StaticFields) || runtimeClassName(class) != runtimeClassName(source) {
				shareStaticValueRefs = false
				break
			}
		}
	} else if shareFrozenStatics {
		clone.Classes = copyClassMapSharedStatics(vm.Classes)
		clone.sharedStaticClasses = true
	} else if vm.sharedClassCopyPlan != nil {
		clone.Classes = copyClassMapWithPlan(vm.Classes, vm.sharedClassCopyPlan)
		clone.sharedClassCopyPlan = vm.sharedClassCopyPlan
	} else {
		clone.Classes = copyClassMap(vm.Classes)
	}
	if shareFrozenStatics {
		vm.staticValueRefsMu.Lock()
		if shareStaticValueRefs {
			if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
				vm.staticValueRefs, vm.staticValueRefFields = vm.collectStaticValueRefs()
			}
			clone.staticValueRefs = vm.staticValueRefs
			clone.staticValueRefFields = vm.staticValueRefFields
			clone.staticValueRefsShared = true
			vm.staticValueRefsShared = true
		}
		// The source may run again after cloning, including when it was
		// itself a clone with already-owned statics. Both sides must detach.
		vm.sharedStaticClasses = true
		vm.ownedStaticClasses = nil
		vm.staticValueRefsMu.Unlock()
	}
	// frozenClassLookup binds canonical class-name results to one exact runtime
	// generation. Clones share that immutable artifact by pointer and resolve
	// aliases through their own Classes map, preserving clone-local static
	// state. Post-freeze registration invalidates the artifact and falls back
	// to a new private generation with a bounded result overlay.
	if vm.frozenClassLookup != nil {
		clone.frozenClassLookup = vm.frozenClassLookup
		clone.classLookupGeneration = vm.frozenClassLookup.generation
		clone.classLookup = nil
		clone.classNameSearchCache = vm.classNameSearchCache
		clone.topLevelClassLookup = vm.topLevelClassLookup
		clone.classValuesWritten = vm.classValuesWritten
	} else {
		clone.rebuildClassLookup()
	}
	// triggerMatchCache is computed from Triggers; share the pointer so we
	// only populate it once across all clones in a run. Concurrent test
	// methods are protected by the cache's RWMutex.
	clone.triggerMatchCache = vm.triggerMatchCache
	// Relationship describe caches depend only on immutable schema metadata.
	// A non-empty stamp permits tentative sharing during clone construction;
	// SetOrg always clears these pointers before installing any mutable org
	// because the compact stamp is not a complete schema manifest.
	clone.metadataCacheStamp = vm.metadataCacheStamp
	if strings.TrimSpace(vm.metadataCacheStamp) != "" {
		clone.jsonChildRelTypeCache = vm.jsonChildRelTypeCache
		clone.childRelCache = vm.childRelCache
		clone.childRelationshipLookupCache = vm.childRelationshipLookupCache
		clone.sObjectFieldAliasCache = vm.sObjectFieldAliasCache
		clone.fieldResolveCache = vm.fieldResolveCache
		clone.soqlExecutionCache = vm.soqlExecutionCache
		clone.dmlSummaryByChild = vm.dmlSummaryByChild
		clone.summarySideEffectObjects = vm.summarySideEffectObjects
		clone.summarySideEffectIndex = vm.summarySideEffectIndex
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
	clone.rejectAsyncActions = vm.rejectAsyncActions
	clone.asyncActionViolation = vm.asyncActionViolation
	clone.staticInitState = nil
	clone.pageReferences = copyStringMap(vm.pageReferences)
	clone.platformCache = copyCacheMap(vm.platformCache)
	clone.managedFeatureValues = copyValueMap(vm.managedFeatureValues)
	clone.isolationJournal = vm.isolationJournal
	clone.testDocumentBaseline = vm.testDocumentBaseline
	if len(vm.sObjectCollectionAliasRefs) != 0 {
		clone.sObjectCollectionAliasRefs = make(map[uint64]bool, len(vm.sObjectCollectionAliasRefs))
		for ref := range vm.sObjectCollectionAliasRefs {
			clone.sObjectCollectionAliasRefs[ref] = true
		}
		// Static/cache copies can allocate fresh record identities. Resolve
		// their types against the source schema during this private scan;
		// callers still install their own org on the returned runtime.
		clone.Org = vm.Org
		for _, class := range clone.Classes {
			for _, field := range class.StaticFields {
				clone.registerSObjectAliasRecord(field.Value)
				clone.registerSObjectAliasRecord(field.InitialValue)
			}
		}
		for _, partition := range clone.platformCache {
			for _, entry := range partition {
				clone.registerSObjectAliasRecord(entry.Value)
			}
		}
		for _, value := range clone.managedFeatureValues {
			clone.registerSObjectAliasRecord(value)
		}
		clone.Org = nil
		clone.sObjectAliasTypes = nil
		clone.sObjectAliasTypeOrg = nil
	}
	return clone
}

// ensureRuntimeArtifactsOwned performs the copy-on-write step for the compiled
// method and trigger maps shared by CloneRuntime. The first mutation after a
// clone through unregisterMethod or RegisterTrigger gives this VM private copies,
// merged with any method overlay, so the shared source maps stay intact.
// RegisterMethod on a shared clone (per-test registration in the apextest
// runner) writes to the small method overlay instead and copies nothing.
func (vm *VM) ensureRuntimeArtifactsOwned() {
	if vm == nil || !vm.runtimeArtifactsShared {
		return
	}
	vm.runtimeArtifactsShared = false
	vm.Methods = copyMethodMap(vm.Methods)
	vm.MethodOverloads = copyMethodSliceMap(vm.MethodOverloads)
	vm.MethodFolded = copyMethodSliceMap(vm.MethodFolded)
	vm.Triggers = copyTriggerSliceMap(vm.Triggers)
	vm.methodOverlay.applyTo(vm.Methods, vm.MethodOverloads, vm.MethodFolded)
	vm.methodOverlay = nil
}

// canShareClassMap reports whether a frozen-shared clone may take vm.Classes
// by pointer instead of building the planned copy. That needs a frozen plan
// whose aliases all hold their primary's value, and no write since freeze
// that could have made one alias diverge.
func (vm *VM) canShareClassMap() bool {
	return vm.sharedClassCopyPlan != nil && vm.sharedClassCopyPlan.uniformAliases && !vm.classMapWritten
}

// prepareClassMapWrite runs before every write to vm.Classes. A map shared
// with another VM is copied first so neither side sees the other's writes.
func (vm *VM) prepareClassMapWrite() {
	vm.classMapWritten = true
	if !vm.classMapShared.Load() {
		return
	}
	vm.classMapShared.Store(false)
	vm.Classes = copyClassMapSharedStatics(vm.Classes)
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
//
// uniformAliases records that every alias already held a value identical to
// its primary's when the plan was built. The shared-statics planned copy is
// then an exact copy of the map, so frozen-shared clones may share the map.
//
// A derived plan (base != nil) describes a clone that registered a few classes
// over a frozen root: the base plan's names minus skip, followed by its own
// primaries and aliases. It is built in O(registered names) by
// deriveClassCopyPlan and partitions the names exactly as a full build would.
type classCopyPlan struct {
	primaries      []string
	aliases        map[string]string
	uniformAliases bool

	// Root plans only: the primary of each classCopyKey group, the
	// primaries that at least one alias points at, and the aliases whose
	// value differed from their primary's when the plan was built.
	primaryByKey     map[string]string
	aliasedPrimaries map[string]struct{}
	divergentAliases []string

	// Derived plans only.
	base *classCopyPlan
	skip map[string]struct{}
}

func buildClassCopyPlan(in map[string]Class) *classCopyPlan {
	plan := &classCopyPlan{
		primaries:        make([]string, 0, len(in)),
		aliases:          make(map[string]string),
		uniformAliases:   true,
		aliasedPrimaries: make(map[string]struct{}),
	}
	primaryByCanonical := make(map[string]string, len(in))
	for name, class := range in {
		canonical := classCopyKey(name, class)
		if primary, ok := primaryByCanonical[canonical]; ok {
			plan.aliases[name] = primary
			plan.aliasedPrimaries[primary] = struct{}{}
			continue
		}
		primaryByCanonical[canonical] = name
		plan.primaries = append(plan.primaries, name)
	}
	plan.primaryByKey = primaryByCanonical
	for alias, primary := range plan.aliases {
		if !sameClassValue(in[alias], in[primary]) {
			plan.uniformAliases = false
			plan.divergentAliases = append(plan.divergentAliases, alias)
		}
	}
	return plan
}

// deriveClassCopyPlan builds the plan for classes, which equal the root plan's
// map except for the written names. It returns nil when a written name was a
// root primary that other names alias; the caller then builds a full plan.
func deriveClassCopyPlan(root *classCopyPlan, classes map[string]Class, written map[string]struct{}) *classCopyPlan {
	if root == nil || root.base != nil || root.primaryByKey == nil {
		return nil
	}
	for name := range written {
		if _, ok := root.aliasedPrimaries[name]; ok {
			return nil
		}
	}
	plan := &classCopyPlan{
		aliases:        make(map[string]string),
		uniformAliases: true,
		base:           root,
		skip:           written,
	}
	// Root aliases that matched their primary still do: every runtime on
	// this generation holds the root map, a planned copy of it, or a copy
	// with only the written names changed. Recheck the ones that did not.
	for _, alias := range root.divergentAliases {
		if !plan.planSkips(alias) && !sameClassValue(classes[alias], classes[root.aliases[alias]]) {
			plan.uniformAliases = false
			break
		}
	}
	primaryByKey := make(map[string]string, len(written))
	for _, name := range sortedNameSet(written) {
		class, ok := classes[name]
		if !ok {
			continue
		}
		key := classCopyKey(name, class)
		if primary, ok := primaryByKey[key]; ok {
			plan.aliases[name] = primary
			continue
		}
		if primary, ok := root.primaryByKey[key]; ok {
			if _, rewritten := written[primary]; !rewritten {
				plan.aliases[name] = primary
				continue
			}
		}
		primaryByKey[key] = name
		plan.primaries = append(plan.primaries, name)
	}
	if plan.uniformAliases {
		for alias, primary := range plan.aliases {
			if !sameClassValue(classes[alias], classes[primary]) {
				plan.uniformAliases = false
				break
			}
		}
	}
	return plan
}

func sortedNameSet(names map[string]struct{}) []string {
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// planSkips reports whether a derived plan drops name from its base.
func (p *classCopyPlan) planSkips(name string) bool {
	_, ok := p.skip[name]
	return ok
}

// sameClassValue reports whether two Class values are the same value: equal
// scalars, and maps and slices with the same backing storage.
func sameClassValue(a, b Class) bool {
	return a.Name == b.Name &&
		a.Namespace == b.Namespace &&
		a.APIVersion == b.APIVersion &&
		a.SuperClass == b.SuperClass &&
		a.Access == b.Access &&
		a.IsAbstract == b.IsAbstract &&
		a.IsInterface == b.IsInterface &&
		a.IsTest == b.IsTest &&
		a.Dependency == b.Dependency &&
		sameMap(a.Fields, b.Fields) &&
		sameMap(a.StaticFields, b.StaticFields) &&
		sameMap(a.Methods, b.Methods) &&
		sameSlice(a.Interfaces, b.Interfaces) &&
		sameSlice(a.FieldOrder, b.FieldOrder) &&
		sameSlice(a.StaticFieldOrder, b.StaticFieldOrder) &&
		sameSlice(a.Constructors, b.Constructors) &&
		sameSlice(a.StaticInitializers, b.StaticInitializers) &&
		sameSlice(a.InstanceInitializers, b.InstanceInitializers) &&
		sameSlice(a.EnumValues, b.EnumValues) &&
		sameSlice(a.Modifiers, b.Modifiers)
}

func sameMap[K comparable, V any](a, b map[K]V) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func sameSlice[T any](a, b []T) bool {
	if len(a) != len(b) || cap(a) != cap(b) || (a == nil) != (b == nil) {
		return false
	}
	return cap(a) == 0 || reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func copyClassMapWithPlan(in map[string]Class, plan *classCopyPlan) map[string]Class {
	out := make(map[string]Class, len(in))
	if base := plan.base; base != nil {
		for _, name := range base.primaries {
			if !plan.planSkips(name) {
				out[name] = copyClass(in[name])
			}
		}
	}
	for _, name := range plan.primaries {
		out[name] = copyClass(in[name])
	}
	if base := plan.base; base != nil {
		for alias, primary := range base.aliases {
			if !plan.planSkips(alias) {
				out[alias] = out[primary]
			}
		}
	}
	for alias, primary := range plan.aliases {
		out[alias] = out[primary]
	}
	return out
}

func copyClassMapSharedStaticsWithPlan(in map[string]Class, plan *classCopyPlan) map[string]Class {
	out := make(map[string]Class, len(in))
	if base := plan.base; base != nil {
		for _, name := range base.primaries {
			if !plan.planSkips(name) {
				out[name] = in[name]
			}
		}
	}
	for _, name := range plan.primaries {
		out[name] = in[name]
	}
	if base := plan.base; base != nil {
		for alias, primary := range base.aliases {
			if !plan.planSkips(alias) {
				out[alias] = out[primary]
			}
		}
	}
	for alias, primary := range plan.aliases {
		out[alias] = out[primary]
	}
	return out
}

func copyClassMapSharedStatics(in map[string]Class) map[string]Class {
	out := make(map[string]Class, len(in))
	for name, class := range in {
		out[name] = class
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

func (vm *VM) ensureMutableClass(name string) (Class, bool) {
	if !vm.sharedStaticClasses {
		class, ok := vm.Classes[name]
		return class, ok
	}
	class, ok := vm.lookupClass(name)
	if !ok {
		return Class{}, false
	}
	canonical := runtimeClassName(class)
	if vm.ownedStaticClasses != nil && vm.ownedStaticClasses[canonical] {
		return class, true
	}
	previousFields := class.StaticFields
	class.StaticFields = copyFieldMap(previousFields)
	// copyFieldMap renews Value.Ref identities. A shared index describes
	// the old fields; refresh just this class when its statics detach.
	for fieldName, field := range class.StaticFields {
		location := canonicalStaticFieldLocationForClass(class, name, fieldName)
		vm.replaceStaticValueRefsInField(previousFields[fieldName].Value, field.Value, location)
	}
	if vm.ownedStaticClasses == nil {
		vm.ownedStaticClasses = make(map[string]bool)
	}
	vm.ownedStaticClasses[canonical] = true
	vm.storeClassValue(class)
	class, ok = vm.lookupClass(canonical)
	return class, ok
}

func (vm *VM) storeMutableClassAtAlias(alias string, class Class) {
	if vm.sharedStaticClasses {
		vm.storeClassValue(class)
		return
	}
	vm.prepareClassMapWrite()
	vm.classValuesWritten = true
	vm.writeClassValue(alias, class)
}

func staticFieldValueMayMutate(field Field) bool {
	switch field.Value.Kind {
	case ValueObject, ValueList, ValueSet, ValueMap:
		return true
	}
	return field.Value.Fields != nil ||
		field.Value.Map != nil ||
		field.Value.MapKeys != nil ||
		field.Value.MapOrder != nil ||
		field.Value.List != nil ||
		field.Value.Set != nil
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
	if len(in) == 0 {
		return nil
	}
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
			entry.Value = cloneValue(entry.Value)
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
	vm.sObjectAliasTypes = nil
	nextStamp := runtimeSchemaStampHintForOrg(org)
	if nextStamp == "" {
		nextStamp = schemaCacheStampForOrg(org)
	}
	if vm.Org != nil || org != nil {
		// OrgState exposes mutable schema maps. Every SetOrg boundary that can
		// install or retain an org must fail closed: the compact schema stamp is
		// not a complete manifest of every describe-visible field.
		vm.clearMetadataCaches()
		vm.metadataCacheStamp = nextStamp
	}
	vm.Org = org
	// Host code may install schema after supplying values. Recheck those
	// roots before newly resolved SObject types become eligible for pruning.
	for _, value := range vm.Globals {
		vm.registerSObjectAliasRecord(value)
	}
	for _, class := range vm.Classes {
		for _, field := range class.StaticFields {
			vm.registerSObjectAliasRecord(field.Value)
			vm.registerSObjectAliasRecord(field.InitialValue)
		}
	}
	if vm.Org != nil && strings.TrimSpace(vm.metadataCacheStamp) != "" && vm.soqlExecutionCache == nil {
		vm.soqlExecutionCache = soql.NewExecutionCache()
	}
	vm.clearTriggerMatchCache()
	if vm.Org != nil {
		vm.Org.Now = func() time.Time { return vm.fakeNow }
	}
	if vm.testContext != nil && isPlaceholderCurrentUser(vm.testContext.CurrentUser) {
		vm.testContext.CurrentUser = vm.defaultTestCurrentUser()
	}
}

// SetRuntimeTemplateOrg installs a fresh clone of the schema generation that
// this VM was cloned from. All other org changes must use SetOrg.
func (vm *VM) SetRuntimeTemplateOrg(org *storage.OrgState) {
	stamp := runtimeSchemaStampHintForOrg(org)
	if stamp == "" || stamp != vm.metadataCacheStamp {
		vm.SetOrg(org)
		return
	}
	vm.Org = org
	if vm.soqlExecutionCache == nil {
		vm.soqlExecutionCache = soql.NewExecutionCache()
	}
	if vm.Org != nil {
		vm.Org.Now = func() time.Time { return vm.fakeNow }
	}
	if vm.testContext != nil && isPlaceholderCurrentUser(vm.testContext.CurrentUser) {
		vm.testContext.CurrentUser = vm.defaultTestCurrentUser()
	}
}

// PrimeMetadataSchema records the compact schema stamp for org without touching
// the org or clearing caches. Clones may tentatively share the base cache graph,
// but SetOrg always detaches it before installing mutable org state.
func (vm *VM) PrimeMetadataSchema(org *storage.OrgState) {
	if vm == nil {
		return
	}
	stamp := runtimeSchemaStampHintForOrg(org)
	if stamp == "" {
		stamp = schemaCacheStampForOrg(org)
	}
	if stamp != "" {
		vm.metadataCacheStamp = stamp
		// Clones share this pointer; create it here so the first clone whose
		// org still carries this stamp builds the summary index for all.
		if vm.summarySideEffectObjects == nil || vm.summarySideEffectObjects.stamp != stamp {
			vm.summarySideEffectObjects = newSummarySideEffectObjectCache(stamp)
		}
	}
}

func PrimeRuntimeTemplateSchema(template *storage.RuntimeTemplate) {
	if template == nil {
		return
	}
	stamp := schemaCacheStampForOrg(&template.Org)
	template.RuntimeSchemaStamp = stamp
	template.Org.RuntimeSchemaStamp = stamp
	storage.PrimeObjectNameIndex(template.Org)
}

// runtimeSchemaStampHintForOrg returns the stamp computed when an immutable
// runtime template was created. SetOrg never uses this hint to retain caches:
// it clears every shared metadata cache before recording the hint. Avoiding a
// full schema walk here keeps the per-test clone boundary constant-time.
func runtimeSchemaStampHintForOrg(org *storage.OrgState) string {
	if org == nil {
		return ""
	}
	return strings.TrimSpace(org.RuntimeSchemaStamp)
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

// schemaCacheStampForOrg returns a compact structural fingerprint of the org
// schema. Each object's canonical content is hashed into its own 64-bit FNV-1a
// value, and the stamp folds those values in sorted object order. The
// fingerprint is not a complete manifest of describe-visible metadata and must
// not authorize cache retention across a SetOrg boundary.
//
// Known standard objects reuse a memoized per-object hash only after their
// current content matches a private snapshot of every hashed input, so an
// in-place map or slice write is always seen (see schemaStampObjectMemo).
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
		objectHash := schemaStampObjectHash(objectName, org.Objects[objectName].Definition)
		for shift := 0; shift < 64; shift += 8 {
			h = schemaStampHashByte(h, byte(objectHash>>shift))
		}
	}
	return strconv.FormatUint(h, 16)
}

// schemaStampObjectMemos maps an exact object name to the immutable
// *schemaStampObjectMemo of the last standard definition hashed under it.
var schemaStampObjectMemos sync.Map

// schemaStampObjectsHashed counts per-object hashes computed from scratch.
var schemaStampObjectsHashed atomic.Int64

// schemaStampObjectMemo is a private copy of every input the per-object stamp
// hash reads. A memo is reused only when the current definition is equal to it
// input by input, so it is keyed by content, not by map or slice identity:
// frozen definitions share their maps with every runtime clone, and a write
// into a shared map keeps its identity while changing its content.
type schemaStampObjectMemo struct {
	apiName     string
	keyPrefix   string
	label       string
	pluralLabel string
	recordTypes int
	fields      map[string]schemaStampFieldInputs
	relations   []schemaStampRelationInputs
	hash        uint64
}

type schemaStampFieldInputs struct {
	apiName               string
	fieldType             storage.FieldType
	relationshipName      string
	childRelationshipName string
	referenceTo           []string
}

type schemaStampRelationInputs struct {
	inheritedFrom      string
	field              string
	parentRelationship string
	childRelationship  string
	parentObjects      []string
}

// schemaStampObjectHash returns the per-object stamp hash. Standard objects
// are memoized; custom and unknown objects are always hashed.
func schemaStampObjectHash(objectName string, definition storage.ObjectDefinition) uint64 {
	standard := storage.IsKnownStandardObject(objectName)
	if standard {
		if cached, ok := schemaStampObjectMemos.Load(objectName); ok {
			if memo := cached.(*schemaStampObjectMemo); memo.matches(definition) {
				return memo.hash
			}
		}
	}
	hash := schemaStampHashObject(objectName, definition)
	if standard {
		schemaStampObjectMemos.Store(objectName, newSchemaStampObjectMemo(definition, hash))
	}
	return hash
}

func newSchemaStampObjectMemo(definition storage.ObjectDefinition, hash uint64) *schemaStampObjectMemo {
	memo := &schemaStampObjectMemo{
		apiName:     definition.APIName,
		keyPrefix:   definition.KeyPrefix,
		label:       definition.Label,
		pluralLabel: definition.PluralLabel,
		recordTypes: len(definition.RecordTypes),
		fields:      make(map[string]schemaStampFieldInputs, len(definition.Fields)),
		relations:   make([]schemaStampRelationInputs, len(definition.Relations)),
		hash:        hash,
	}
	for name, field := range definition.Fields {
		memo.fields[name] = schemaStampFieldInputs{
			apiName:               field.APIName,
			fieldType:             field.Type,
			relationshipName:      field.RelationshipName,
			childRelationshipName: field.ChildRelationshipName,
			referenceTo:           append([]string(nil), field.ReferenceTo...),
		}
	}
	for i, relation := range definition.Relations {
		memo.relations[i] = schemaStampRelationInputs{
			inheritedFrom:      relation.InheritedFrom,
			field:              relation.Field,
			parentRelationship: relation.ParentRelationship,
			childRelationship:  relation.ChildRelationship,
			parentObjects:      append([]string(nil), relation.ParentObjects...),
		}
	}
	return memo
}

// matches reports whether definition has exactly the hashed inputs recorded
// in the memo. Exact equality is stricter than the trimmed and lowercased
// hash input, so a match always reproduces the memoized hash.
func (memo *schemaStampObjectMemo) matches(definition storage.ObjectDefinition) bool {
	if definition.APIName != memo.apiName ||
		definition.KeyPrefix != memo.keyPrefix ||
		definition.Label != memo.label ||
		definition.PluralLabel != memo.pluralLabel ||
		len(definition.RecordTypes) != memo.recordTypes ||
		len(definition.Fields) != len(memo.fields) ||
		len(definition.Relations) != len(memo.relations) {
		return false
	}
	for name, field := range definition.Fields {
		inputs, ok := memo.fields[name]
		if !ok ||
			field.APIName != inputs.apiName ||
			field.Type != inputs.fieldType ||
			field.RelationshipName != inputs.relationshipName ||
			field.ChildRelationshipName != inputs.childRelationshipName ||
			!slices.Equal(field.ReferenceTo, inputs.referenceTo) {
			return false
		}
	}
	for i, relation := range definition.Relations {
		inputs := memo.relations[i]
		if relation.InheritedFrom != inputs.inheritedFrom ||
			relation.Field != inputs.field ||
			relation.ParentRelationship != inputs.parentRelationship ||
			relation.ChildRelationship != inputs.childRelationship ||
			!slices.Equal(relation.ParentObjects, inputs.parentObjects) {
			return false
		}
	}
	return true
}

// schemaStampHashObject streams one object's canonical stamp content into a
// fresh FNV-1a hash.
func schemaStampHashObject(objectName string, definition storage.ObjectDefinition) uint64 {
	schemaStampObjectsHashed.Add(1)
	h := schemaStampFNVOffset
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
		if relation.InheritedFrom != "" {
			h = schemaStampHashByte(h, ':')
			h = schemaStampHash(h, relation.InheritedFrom)
		}
		h = schemaStampHashByte(h, ';')
	}
	h = schemaStampHashRaw(h, strconv.Itoa(len(definition.RecordTypes)))
	h = schemaStampHashByte(h, '|')
	return h
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
	engine.FlowActionInvokerWithContext = func(action storage.FlowAction, record storage.Record, context dml.FlowActionContext) error {
		return vm.invokeFlowActionWithContext(action, record, context, result)
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
	return vm.invokeFlowActionWithContext(action, record, dml.FlowActionContext{}, result)
}

func (vm *VM) invokeFlowActionWithContext(action storage.FlowAction, record storage.Record, context dml.FlowActionContext, result *Result) error {
	method, ok, err := vm.resolveFlowInvocableMethod(action)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("flow action %s: no static @InvocableMethod found on %s", action.Name, flowActionTargetName(action))
	}
	var args []Value
	if len(method.Params) == 0 {
		// Salesforce packages in the corpus include invocable actions that use
		// the action call only as a transaction boundary and intentionally take
		// no input (for example, a deferred-rollup commit action).
		args = nil
	} else {
		if len(method.Params) != 1 || collectionBase(method.Params[0].Type) != "List" {
			return fmt.Errorf("flow action %s: %s must accept exactly one List parameter", action.Name, method.Name)
		}
		elementType, _ := collectionElementType(method.Params[0].Type)
		// Nested invocable input classes are commonly emitted as an unqualified
		// type in the method signature (for example List<FlowInput> inside
		// Rollup). Runtime values still need the owning class qualification so
		// member lookup sees the nested class fields.
		elementType = vm.qualifyFlowActionElementType(method, elementType)
		item := vm.flowActionInputObject(action, record, context, elementType)
		arg := List(item)
		arg.Type = method.Params[0].Type
		args = []Value{arg}
	}
	appendTrace(result, "apex.flow.action", "apex.flow", map[string]any{
		"action": action.Name,
		"class":  method.ClassName,
		"method": method.Name,
		"record": string(record.ID),
		"object": record.Object,
	})
	_, err = vm.callMethod(method, args, result)
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

func (vm *VM) qualifyFlowActionElementType(method Method, elementType string) string {
	elementType = strings.TrimSpace(elementType)
	if elementType == "" {
		return elementType
	}
	if _, ok := vm.lookupClass(elementType); ok {
		return elementType
	}
	owner := strings.TrimSpace(method.ClassName)
	if owner == "" || strings.Contains(elementType, ".") {
		return elementType
	}
	candidate := owner + "." + elementType
	if _, ok := vm.lookupClass(candidate); ok {
		return candidate
	}
	return elementType
}

func (vm *VM) flowActionInputObject(action storage.FlowAction, record storage.Record, context dml.FlowActionContext, elementType string) Value {
	if len(action.Inputs) == 0 {
		return vm.vmValueFromRecord(record)
	}
	item := Object(elementType)
	vm.initializeFields(&item, elementType)
	for _, input := range action.Inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			continue
		}
		value, ok := vm.flowActionInputValue(input, record, context)
		if !ok {
			value = Null
		}
		item.Fields[name] = value
	}
	return item
}

func (vm *VM) flowActionInputValue(input storage.WorkflowFieldUpdate, record storage.Record, context dml.FlowActionContext) (Value, bool) {
	if source := strings.TrimSpace(input.SourceField); source != "" {
		if value, ok := vm.flowActionContextReference(source, record, context); ok {
			return value, true
		}
	}
	if literal := strings.TrimSpace(input.LiteralValue); literal != "" {
		switch strings.ToLower(literal) {
		case "true":
			return Bool(true), true
		case "false":
			return Bool(false), true
		}
		if decimal, err := decimalFromText(literal); err == nil && strings.ContainsAny(literal, ".eE") {
			return decimal, true
		}
		if integer, err := strconv.ParseInt(literal, 10, 64); err == nil {
			return Int(integer), true
		}
		return String(literal), true
	}
	return Null, true
}

func (vm *VM) flowActionContextReference(reference string, record storage.Record, context dml.FlowActionContext) (Value, bool) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return Value{}, false
	}
	key := strings.ToLower(reference)
	if records, ok := context.Collections[key]; ok {
		return vm.flowActionRecordCollection(records), true
	}
	if records, ok := context.LookupCollections[key]; ok {
		return vm.flowActionRecordCollection(records), true
	}
	if value, ok := context.Scalars[key]; ok {
		return vmValueFromStorage(value), true
	}
	if referenceRecord, ok := context.Records[key]; ok {
		return vm.vmValueFromRecord(referenceRecord), true
	}
	if referenceRecord, ok := context.LookupOutputs[key]; ok {
		return vm.vmValueFromRecord(referenceRecord), true
	}
	if value, ok := vm.flowActionRecordField(vm.flowActionRecordValue(record), reference); ok {
		return value, true
	}
	for root, candidate := range context.Records {
		prefix := root + "."
		if strings.HasPrefix(key, prefix) {
			if value, ok := vm.flowActionRecordField(vm.flowActionRecordValue(candidate), reference[len(prefix):]); ok {
				return value, true
			}
		}
	}
	for root, candidate := range context.LookupOutputs {
		prefix := root + "."
		if strings.HasPrefix(key, prefix) {
			if value, ok := vm.flowActionRecordField(vm.flowActionRecordValue(candidate), reference[len(prefix):]); ok {
				return value, true
			}
		}
	}
	return Value{}, false
}

func (vm *VM) flowActionRecordCollection(records []storage.Record) Value {
	values := make([]Value, 0, len(records))
	for _, record := range records {
		if record.Object == "" && record.ID == "" && len(record.Fields) == 0 && len(record.ExplicitNulls) == 0 {
			values = append(values, Null)
			continue
		}
		values = append(values, vm.flowActionRecordValue(record))
	}
	value := List(values...)
	value.Type = "List<SObject>"
	return value
}

// Flow Apex actions receive records from the Flow interview rather than from
// a SOQL projection. Materialize lightweight parent relationship shells from
// lookup ids so action code that evaluates a relationship path (for example
// Account.Name in a where clause) can read the stored parent fields.
func (vm *VM) flowActionRecordValue(record storage.Record) Value {
	value := vmValueFromRecord(record)
	vm.flowActionParentRelationships(&value, make(map[string]bool), 0)
	return value
}

func (vm *VM) flowActionParentRelationships(value *Value, visited map[string]bool, depth int) {
	if vm == nil || vm.Org == nil || value == nil || value.Kind != ValueObject || depth > 8 {
		return
	}
	objectName, ok := vm.resolveObjectName(value.Type)
	if !ok {
		objectName = value.Type
	}
	id := sObjectIDFromFields(value.Fields)
	key := strings.ToLower(objectName) + ":" + strings.ToLower(string(id))
	if visited[key] {
		return
	}
	visited[key] = true
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return
	}
	for _, relation := range object.Definition.Relations {
		if strings.TrimSpace(relation.ParentRelationship) == "" {
			continue
		}
		if _, existing, exists := objectFieldValue(*value, relation.ParentRelationship); exists && existing.Kind == ValueObject {
			vm.markFlowActionParentProjection(&existing)
			value.Fields[relation.ParentRelationship] = existing
			continue
		}
		_, lookup, exists := objectFieldValue(*value, relation.Field)
		if !exists || lookup.Kind == ValueNull {
			continue
		}
		parent, exists := vm.flowActionParentRelationshipFromLookupID(relation, lookup)
		if !exists || parent.Kind != ValueObject {
			continue
		}
		vm.markFlowActionParentProjection(&parent)
		value.Fields[relation.ParentRelationship] = parent
	}
}

func (vm *VM) markFlowActionParentProjection(value *Value) {
	if value == nil || value.Kind != ValueObject {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	value.Fields[sobjectParentProjectionField] = Bool(true)
	vm.ensureQueriedSObjectFieldMarker(value, value.Type)
	for field := range value.Fields {
		if !isInternalSObjectField(field) {
			markQueriedSObjectField(value, field)
		}
	}
}

func (vm *VM) flowActionParentRelationshipFromLookupID(relation storage.Relationship, lookupValue Value) (Value, bool) {
	if vm == nil || vm.Org == nil {
		return Null, false
	}
	lookupID, ok := sObjectIDFromValue(lookupValue)
	if !ok || lookupID == "" {
		return Null, false
	}
	for _, parentName := range relation.ParentObjects {
		parentObject, ok := vm.resolveObjectName(parentName)
		if !ok {
			parentObject = parentName
		}
		if strings.TrimSpace(parentObject) == "" {
			continue
		}
		if stored, found := vm.findOrgRecord(parentObject, lookupID); found {
			stored.Object = parentObject
			return vmValueFromRecord(stored), true
		}
	}
	return vm.parentRelationshipShellFromLookupID(relation, lookupValue)
}

func (vm *VM) flowActionRecordField(record Value, reference string) (Value, bool) {
	parts := strings.Split(reference, ".")
	current := record
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		_, value, ok := objectFieldValue(current, part)
		if !ok {
			return Value{}, false
		}
		current = value
	}
	return current, true
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
		candidates := vm.registeredOverloads(className + "." + methodName)
		if len(candidates) == 0 {
			candidates = vm.registeredFolded(strings.ToLower(className + "." + methodName))
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
				if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
					return rollbackErr
				}
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
					if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
						return rollbackErr
					}
					vm.restoreSideEffects(sideEffects)
				}
				return err
			}
		}
	}
	return nil
}

func cloneStorageRecords(records []storage.Record) []storage.Record {
	if len(records) == 0 {
		return nil
	}
	out := make([]storage.Record, 0, len(records))
	for _, record := range records {
		out = append(out, record.Clone())
	}
	return out
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
			if rollbackErr := vm.restoreDMLRollbackPoint(rollback); rollbackErr != nil {
				return rollbackErr
			}
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

// SetTestDocumentBaseline captures the pre-test Document rows. The runner calls
// it on its private base VM before static initializers or @TestSetup execute.
// Documents subsequently inserted by tests remain visible in ordinary SOQL.
func (vm *VM) SetTestDocumentBaseline(org *storage.OrgState) {
	vm.testDocumentBaseline = make(map[storage.ID]struct{})
	if org == nil {
		return
	}
	name, ok := storage.ResolveObjectName(*org, "Document")
	if !ok {
		return
	}
	for id := range org.Objects[name].Records {
		vm.testDocumentBaseline[id] = struct{}{}
	}
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
	record, ok := vm.defaultOrgUserRecord()
	if !ok {
		return Value{}
	}
	return vmValueFromRecord(record)
}

// defaultOrgUserID returns userInfoFieldValue(defaultOrgUser(), "Id") without
// converting the whole User record. It answers only when the record's Id slot
// cannot be shadowed by a stored field, child relationship, explicit null or
// explicit-field marker; otherwise ok is false and callers convert the record.
func (vm *VM) defaultOrgUserID() (string, bool) {
	record, ok := vm.defaultOrgUserRecord()
	if !ok || record.ID == "" {
		return "", false
	}
	for field := range record.Fields {
		if defaultOrgUserIDShadowed(field) {
			return "", false
		}
	}
	for relationship := range record.Children {
		if defaultOrgUserIDShadowed(relationship) {
			return "", false
		}
	}
	for field := range record.ExplicitNulls {
		if defaultOrgUserIDShadowed(field) {
			return "", false
		}
	}
	return string(record.ID), true
}

func defaultOrgUserIDShadowed(field string) bool {
	head, _, _ := strings.Cut(field, ".")
	return strings.EqualFold(head, "Id") || head == sobjectExplicitFieldsField
}

func (vm *VM) defaultOrgUserRecord() (storage.Record, bool) {
	if vm.Org == nil {
		return storage.Record{}, false
	}
	users, ok := vm.Org.Objects["User"]
	if !ok || len(users.Records) == 0 {
		return storage.Record{}, false
	}
	for _, preferredID := range []storage.ID{storage.ID("005-local-user"), storage.ID("005000000000001")} {
		if preferred, ok := users.Records[preferredID]; ok {
			if !strings.EqualFold(recordFieldString(preferred, "UserType"), "AutomatedProcess") {
				return preferred, true
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
	return users.Records[first], true
}

func (vm *VM) ResetStatics() error {
	for className, class := range vm.Classes {
		if len(class.StaticFields) == 0 {
			continue
		}
		class, _ = vm.ensureMutableClass(className)
		for fieldName, field := range class.StaticFields {
			field.Value = defaultStaticCollectionFieldValue(className, fieldName, field)
			class.StaticFields[fieldName] = field
		}
		vm.storeMutableClassAtAlias(className, class)
	}
	vm.invalidateStaticValueRefs()
	vm.staticInitState = nil
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
			class, _ = vm.ensureMutableClass(className)
			for fieldName, field := range class.StaticFields {
				if !resetTestAsyncStaticField(field) && !resetTestAsyncStaticFieldForReinitialization(class, field) {
					continue
				}
				field.Value = defaultStaticCollectionFieldValue(className, fieldName, field)
				class.StaticFields[fieldName] = field
			}
		}
		vm.storeMutableClassAtAlias(className, class)
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
	vm.startCPUClock()
	result = Result{Vars: vm.Globals, traceEnabled: vm.traceEnabled}
	if vm.traceEnabled {
		result.TraceFormat = trace.FormatChromeTraceEvent
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("internal VM panic: %v", recovered)
		}
		result.CapturedEmails = append([]CapturedEmail(nil), vm.capturedEmails...)
		vm.sampleCPUTimeMS()
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

// SetSynchronousActionBoundary makes framework actions fail closed when they
// attempt to cross an asynchronous delivery boundary that the caller cannot
// drain and commit as part of the same request.
func (vm *VM) SetSynchronousActionBoundary(enabled bool) {
	if vm != nil {
		vm.rejectAsyncActions = enabled
		if !enabled {
			vm.asyncActionViolation = false
		}
	}
}

// rejectSynchronousAsyncAction records a boundary violation even when Apex
// catches and converts the UnsupportedFeature into an ordinary return value.
// The request owner must still reject the transaction before publishing any
// earlier DML. This flag is intentionally outside org/savepoint state.
func (vm *VM) rejectSynchronousAsyncAction(message string) error {
	if vm != nil {
		vm.asyncActionViolation = true
	}
	return UnsupportedFeature(message)
}

// HasRejectedAsyncAction reports an attempted asynchronous action that was
// caught or otherwise converted into a successful-looking Apex result.
func (vm *VM) HasRejectedAsyncAction() bool {
	return vm != nil && vm.asyncActionViolation
}

// HasPendingAsyncWork reports asynchronous work that would outlive the
// current VM invocation if the caller committed only the org state.
func (vm *VM) HasPendingAsyncWork() bool {
	if vm == nil {
		return false
	}
	if len(vm.localAsyncJobs) > 0 {
		return true
	}
	if vm.testContext == nil {
		return false
	}
	return len(vm.testContext.AsyncJobs) > 0 || len(vm.testContext.PlatformEvents) > 0 || len(vm.testContext.EventPublishes) > 0
}

func (vm *VM) DrainAsync(result *Result) error {
	if vm.testContext != nil {
		return vm.drainTestAsync(result)
	}
	return vm.drainLocalAsync(result)
}

type controlSignal string
