package oracle

const SchemaVersion = 1

type OracleStatus string

const (
	OracleStatusPass                OracleStatus = "pass"
	OracleStatusFail                OracleStatus = "fail"
	OracleStatusSkipped             OracleStatus = "skipped"
	OracleStatusUnsupported         OracleStatus = "unsupported"
	OracleStatusCompileError        OracleStatus = "compile_error"
	OracleStatusRuntimeError        OracleStatus = "runtime_error"
	OracleStatusInfrastructureError OracleStatus = "infrastructure_error"
)

type OracleEventType string

const (
	OracleEventMethodCall  OracleEventType = "method_call"
	OracleEventSOQL        OracleEventType = "soql"
	OracleEventDML         OracleEventType = "dml"
	OracleEventTrigger     OracleEventType = "trigger"
	OracleEventFlow        OracleEventType = "flow"
	OracleEventWorkflow    OracleEventType = "workflow"
	OracleEventEmail       OracleEventType = "email"
	OracleEventFile        OracleEventType = "file"
	OracleEventAsync       OracleEventType = "async"
	OracleEventLimit       OracleEventType = "limit"
	OracleEventAssert      OracleEventType = "assert"
	OracleEventException   OracleEventType = "exception"
	OracleEventDebug       OracleEventType = "debug"
	OracleEventUnsupported OracleEventType = "unsupported"
)

type OracleOutcome string

const (
	OracleOutcomePass                OracleOutcome = "pass"
	OracleOutcomeTraceMismatch       OracleOutcome = "trace_mismatch"
	OracleOutcomeStateMismatch       OracleOutcome = "state_mismatch"
	OracleOutcomeExceptionMismatch   OracleOutcome = "exception_mismatch"
	OracleOutcomeUnsupported         OracleOutcome = "unsupported"
	OracleOutcomeCompileGap          OracleOutcome = "compile_gap"
	OracleOutcomeInfrastructureError OracleOutcome = "infrastructure_error"
)

type OracleRun struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Source        string               `json:"source,omitempty"`
	Project       string               `json:"project,omitempty"`
	OrgAlias      string               `json:"orgAlias,omitempty"`
	TestClass     string               `json:"testClass,omitempty"`
	TestMethod    string               `json:"testMethod,omitempty"`
	Status        OracleStatus         `json:"status"`
	Exception     *OracleException     `json:"exception,omitempty"`
	Stack         []OracleStackFrame   `json:"stack,omitempty"`
	DebugPayloads []OracleDebugPayload `json:"debugPayloads,omitempty"`
	Events        []OracleEvent        `json:"events,omitempty"`
	Limits        []OracleLimit        `json:"limits,omitempty"`
	SideEffects   []OracleSideEffect   `json:"sideEffects,omitempty"`
	FinalRecords  []OracleRecord       `json:"finalRecords,omitempty"`
	Timings       []OracleTiming       `json:"timings,omitempty"`
	DurationMS    int64                `json:"durationMs,omitempty"`
	RawArtifacts  []OracleArtifact     `json:"rawArtifacts,omitempty"`
}

type OracleException struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
	Stack   string `json:"stack,omitempty"`
}

type OracleStackFrame struct {
	Symbol string `json:"symbol,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

type OracleDebugPayload struct {
	Label    string `json:"label,omitempty"`
	Sequence int    `json:"sequence,omitempty"`
	Value    any    `json:"value,omitempty"`
	Raw      string `json:"raw,omitempty"`
}

type OracleEvent struct {
	Type          OracleEventType `json:"type"`
	Sequence      int             `json:"sequence,omitempty"`
	Name          string          `json:"name,omitempty"`
	Operation     string          `json:"operation,omitempty"`
	Object        string          `json:"object,omitempty"`
	Query         string          `json:"query,omitempty"`
	Fields        []string        `json:"fields,omitempty"`
	Values        map[string]any  `json:"values,omitempty"`
	Result        any             `json:"result,omitempty"`
	ExceptionType string          `json:"exceptionType,omitempty"`
	Message       string          `json:"message,omitempty"`
	Payload       map[string]any  `json:"payload,omitempty"`
	Raw           string          `json:"raw,omitempty"`
}

type OracleLimit struct {
	Name     string `json:"name"`
	Used     int64  `json:"used,omitempty"`
	Max      int64  `json:"max,omitempty"`
	Sequence int    `json:"sequence,omitempty"`
}

type OracleSideEffect struct {
	Type     OracleEventType `json:"type"`
	Name     string          `json:"name,omitempty"`
	Object   string          `json:"object,omitempty"`
	ID       string          `json:"id,omitempty"`
	Fields   map[string]any  `json:"fields,omitempty"`
	Sequence int             `json:"sequence,omitempty"`
}

type OracleRecord struct {
	Object string         `json:"object"`
	ID     string         `json:"id,omitempty"`
	Fields map[string]any `json:"fields,omitempty"`
}

type OracleTiming struct {
	Name       string `json:"name"`
	DurationMS int64  `json:"durationMs"`
}

type OracleArtifact struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
	Raw  string `json:"raw,omitempty"`
}

type OracleDiff struct {
	TestClass        string        `json:"testClass,omitempty"`
	TestMethod       string        `json:"testMethod,omitempty"`
	Outcome          OracleOutcome `json:"outcome"`
	Details          []string      `json:"details,omitempty"`
	SalesforceStatus OracleStatus  `json:"salesforceStatus,omitempty"`
	LocalStatus      OracleStatus  `json:"localStatus,omitempty"`
}
