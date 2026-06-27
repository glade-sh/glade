package debuglog

type EditorAnalysis struct {
	Version      int                 `json:"version"`
	LogFile      string              `json:"logFile,omitempty"`
	ProjectRoot  string              `json:"projectRoot,omitempty"`
	Language     string              `json:"language"`
	GeneratedAt  string              `json:"generatedAt"`
	Entries      []EditorEntry       `json:"entries"`
	Symbols      []EditorSymbol      `json:"symbols"`
	Folds        []EditorFold        `json:"folds"`
	Links        []EditorLink        `json:"links"`
	Hovers       []EditorHover       `json:"hovers"`
	CodeLenses   []EditorCodeLens    `json:"codeLenses"`
	Semantic     []EditorToken       `json:"semanticTokens"`
	Diagnostics  []EditorDiagnostic  `json:"diagnostics"`
	Variables    []EditorVariable    `json:"variables"`
	ReplayFrames []EditorReplayFrame `json:"replayFrames"`
	Coverage     EditorCoverage      `json:"coverage"`
}

type EditorRange struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

type EditorLocation struct {
	File       string  `json:"file,omitempty"`
	Line       int     `json:"line,omitempty"`
	Column     int     `json:"column,omitempty"`
	Symbol     string  `json:"symbol,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	Confidence float64 `json:"confidence"`
}

type EditorEntry struct {
	Index     int            `json:"index"`
	Kind      string         `json:"kind"`
	Raw       string         `json:"raw"`
	Range     EditorRange    `json:"range"`
	Depth     int            `json:"depth"`
	FrameID   string         `json:"frameId,omitempty"`
	ParentID  string         `json:"parentId,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
	Source    EditorLocation `json:"source,omitempty"`
	LowDetail bool           `json:"lowDetail,omitempty"`
}

type EditorSymbol struct {
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	Range    EditorRange    `json:"range"`
	Select   EditorRange    `json:"selectionRange"`
	Detail   string         `json:"detail,omitempty"`
	Source   EditorLocation `json:"source,omitempty"`
	Children []EditorSymbol `json:"children,omitempty"`
}

type EditorFold struct {
	Kind      string      `json:"kind"`
	Range     EditorRange `json:"range"`
	Collapsed string      `json:"collapsedText,omitempty"`
	Depth     int         `json:"depth"`
}

type EditorLink struct {
	Kind    string         `json:"kind"`
	Range   EditorRange    `json:"range"`
	Target  EditorLocation `json:"target,omitempty"`
	Command string         `json:"command,omitempty"`
	Title   string         `json:"title,omitempty"`
}

type EditorHover struct {
	Range    EditorRange `json:"range"`
	Markdown string      `json:"markdown"`
}

type EditorCodeLens struct {
	Range     EditorRange `json:"range"`
	Command   string      `json:"command"`
	Title     string      `json:"title"`
	Arguments []string    `json:"arguments,omitempty"`
}

type EditorToken struct {
	Range     EditorRange `json:"range"`
	TokenType string      `json:"tokenType"`
	Modifiers []string    `json:"modifiers,omitempty"`
}

type EditorDiagnostic struct {
	Range    EditorRange `json:"range"`
	Severity string      `json:"severity"`
	Code     string      `json:"code"`
	Message  string      `json:"message"`
}

type EditorVariable struct {
	Name       string         `json:"name"`
	Type       string         `json:"type,omitempty"`
	Value      string         `json:"value,omitempty"`
	ScopeID    string         `json:"scopeId,omitempty"`
	Range      EditorRange    `json:"range"`
	LogDef     EditorLocation `json:"logDefinition,omitempty"`
	SourceDef  EditorLocation `json:"sourceDefinition,omitempty"`
	Assignment EditorLocation `json:"assignment,omitempty"`
}

type EditorReplayFrame struct {
	FrameID    string      `json:"frameId"`
	EntryIndex int         `json:"entryIndex"`
	Range      EditorRange `json:"range"`
	CanReplay  bool        `json:"canReplay"`
	Reason     string      `json:"reason,omitempty"`
}

type EditorCoverage struct {
	TotalEntries       int `json:"totalEntries"`
	ResolvedSources    int `json:"resolvedSources"`
	ResolvedVariables  int `json:"resolvedVariables"`
	ResolvedSchemaRefs int `json:"resolvedSchemaRefs"`
	ParserWarnings     int `json:"parserWarnings"`
}
