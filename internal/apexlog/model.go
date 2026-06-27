package apexlog

type Log struct {
	APIVersion    string       `json:"apiVersion,omitempty"`
	Header        string       `json:"header,omitempty"`
	AnonymousApex string       `json:"anonymousApex,omitempty"`
	Entries       []Entry      `json:"entries"`
	Limits        []LimitUsage `json:"limits,omitempty"`
}

type Entry struct {
	Raw       string    `json:"raw"`
	Timestamp string    `json:"timestamp,omitempty"`
	Line      int       `json:"line"`
	ByteStart int       `json:"byteStart"`
	ByteEnd   int       `json:"byteEnd"`
	Kind      EntryKind `json:"kind"`
	Payload   string    `json:"payload,omitempty"`
	Data      EntryData `json:"data,omitempty"`
}

type EntryKind string

const (
	EntryOther                   EntryKind = "OTHER"
	EntryUserInfo                EntryKind = "USER_INFO"
	EntryExecutionStarted        EntryKind = "EXECUTION_STARTED"
	EntryExecutionFinished       EntryKind = "EXECUTION_FINISHED"
	EntryCodeUnitStarted         EntryKind = "CODE_UNIT_STARTED"
	EntryCodeUnitFinished        EntryKind = "CODE_UNIT_FINISHED"
	EntryMethodEntry             EntryKind = "METHOD_ENTRY"
	EntryMethodExit              EntryKind = "METHOD_EXIT"
	EntryConstructorEntry        EntryKind = "CONSTRUCTOR_ENTRY"
	EntryConstructorExit         EntryKind = "CONSTRUCTOR_EXIT"
	EntrySystemMethodEntry       EntryKind = "SYSTEM_METHOD_ENTRY"
	EntrySystemMethodExit        EntryKind = "SYSTEM_METHOD_EXIT"
	EntryVariableScopeBegin      EntryKind = "VARIABLE_SCOPE_BEGIN"
	EntryVariableAssignment      EntryKind = "VARIABLE_ASSIGNMENT"
	EntryStatementExecute        EntryKind = "STATEMENT_EXECUTE"
	EntryHeapAllocate            EntryKind = "HEAP_ALLOCATE"
	EntryUserDebug               EntryKind = "USER_DEBUG"
	EntrySOQLExecuteBegin        EntryKind = "SOQL_EXECUTE_BEGIN"
	EntrySOQLExecuteEnd          EntryKind = "SOQL_EXECUTE_END"
	EntryDMLBegin                EntryKind = "DML_BEGIN"
	EntryDMLEnd                  EntryKind = "DML_END"
	EntryValidationRule          EntryKind = "VALIDATION_RULE"
	EntryExceptionThrown         EntryKind = "EXCEPTION_THROWN"
	EntryFatalError              EntryKind = "FATAL_ERROR"
	EntryEnteringManagedPackage  EntryKind = "ENTERING_MANAGED_PKG"
	EntryCumulativeLimitUsage    EntryKind = "CUMULATIVE_LIMIT_USAGE"
	EntryCumulativeLimitUsageEnd EntryKind = "CUMULATIVE_LIMIT_USAGE_END"
	EntryLimitUsageForNamespace  EntryKind = "LIMIT_USAGE_FOR_NS"
	EntryCalloutRequest          EntryKind = "CALLOUT_REQUEST"
	EntryCalloutResponse         EntryKind = "CALLOUT_RESPONSE"
)

type EntryData struct {
	SourceLine      int          `json:"sourceLine,omitempty"`
	MethodSymbol    string       `json:"methodSymbol,omitempty"`
	VariableName    string       `json:"variableName,omitempty"`
	VariableType    string       `json:"variableType,omitempty"`
	VariableValue   string       `json:"variableValue,omitempty"`
	HeapBytes       int          `json:"heapBytes,omitempty"`
	ValidationRule  string       `json:"validationRule,omitempty"`
	DebugLevel      string       `json:"debugLevel,omitempty"`
	DebugMessage    string       `json:"debugMessage,omitempty"`
	SOQLQuery       string       `json:"soqlQuery,omitempty"`
	SOQLRows        int          `json:"soqlRows,omitempty"`
	DMLOperation    string       `json:"dmlOperation,omitempty"`
	DMLType         string       `json:"dmlType,omitempty"`
	DMLRows         int          `json:"dmlRows,omitempty"`
	ExceptionType   string       `json:"exceptionType,omitempty"`
	ExceptionText   string       `json:"exceptionText,omitempty"`
	StackFrames     []StackFrame `json:"stackFrames,omitempty"`
	CodeUnit        string       `json:"codeUnit,omitempty"`
	Namespace       string       `json:"namespace,omitempty"`
	CalloutEndpoint string       `json:"calloutEndpoint,omitempty"`
	CalloutStatus   string       `json:"calloutStatus,omitempty"`
}

type StackFrame struct {
	Namespace string `json:"namespace,omitempty"`
	Class     string `json:"class,omitempty"`
	Method    string `json:"method,omitempty"`
	Line      int    `json:"line,omitempty"`
	Raw       string `json:"raw"`
}

type LimitUsage struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Used      int    `json:"used"`
	Limit     int    `json:"limit"`
}
