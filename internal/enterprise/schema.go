package enterprise

import "time"

const SchemaVersion = "glade.enterprise.report/v0"

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

func (s Severity) Valid() bool {
	switch s {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo:
		return true
	default:
		return false
	}
}

type Confidence string

const (
	ConfidenceHigh    Confidence = "high"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceLow     Confidence = "low"
	ConfidenceUnknown Confidence = "unknown"
)

func (c Confidence) Valid() bool {
	switch c {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow, ConfidenceUnknown:
		return true
	default:
		return false
	}
}

type Category string

const (
	CategoryArchitecture Category = "architecture"
	CategoryCruft        Category = "cruft"
	CategoryRefactor     Category = "refactor_proof"
	CategoryRuntime      Category = "runtime"
	CategorySupport      Category = "support"
	CategoryInventory    Category = "inventory"
)

type EvidenceType string

const (
	EvidenceGraph     EvidenceType = "graph"
	EvidenceMetadata  EvidenceType = "metadata"
	EvidenceRuntime   EvidenceType = "runtime"
	EvidenceSema      EvidenceType = "sema"
	EvidenceGit       EvidenceType = "git"
	EvidenceHeuristic EvidenceType = "heuristic"
)

type Status string

const (
	StatusPass        Status = "pass"
	StatusWarn        Status = "warn"
	StatusFail        Status = "fail"
	StatusNotRun      Status = "not_run"
	StatusUnsupported Status = "unsupported"
)

type Location struct {
	File        string `json:"file,omitempty"`
	LineStart   int    `json:"line_start,omitempty"`
	LineEnd     int    `json:"line_end,omitempty"`
	ColumnStart int    `json:"column_start,omitempty"`
	ColumnEnd   int    `json:"column_end,omitempty"`
}

type Evidence struct {
	Type     EvidenceType   `json:"type"`
	Message  string         `json:"message"`
	Location *Location      `json:"location,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
}

type Finding struct {
	ID             string     `json:"id"`
	Category       Category   `json:"category"`
	Severity       Severity   `json:"severity"`
	Confidence     Confidence `json:"confidence"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Symbol         string     `json:"symbol,omitempty"`
	Location       Location   `json:"location,omitempty"`
	Evidence       []Evidence `json:"evidence"`
	Recommendation string     `json:"recommendation"`
	NextActions    []string   `json:"next_actions,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
}

type ProjectSummary struct {
	Root             string `json:"root"`
	Namespace        string `json:"namespace,omitempty"`
	SourceAPIVersion string `json:"source_api_version,omitempty"`
	ApexClasses      int    `json:"apex_classes"`
	Triggers         int    `json:"triggers"`
	Tests            int    `json:"tests"`
	MetadataFiles    int    `json:"metadata_files"`
}

type Summary struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

type Section struct {
	ID      string        `json:"id"`
	Title   string        `json:"title"`
	Summary string        `json:"summary,omitempty"`
	Items   []SectionItem `json:"items,omitempty"`
}

type SectionItem struct {
	Label   string         `json:"label"`
	Value   string         `json:"value,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

type Artifact struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type TraceSummary struct {
	Events         int            `json:"events"`
	ByCategory     map[string]int `json:"by_category,omitempty"`
	ByName         map[string]int `json:"by_name,omitempty"`
	SOQLStatements int            `json:"soql_statements,omitempty"`
	DMLOperations  int            `json:"dml_operations,omitempty"`
	AsyncEvents    int            `json:"async_events,omitempty"`
	Callouts       int            `json:"callouts,omitempty"`
}

type Report struct {
	SchemaVersion string         `json:"schema_version"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Command       string         `json:"command"`
	Project       ProjectSummary `json:"project"`
	Status        Status         `json:"status,omitempty"`
	Summary       Summary        `json:"summary"`
	Sections      []Section      `json:"sections"`
	Findings      []Finding      `json:"findings"`
	Artifacts     []Artifact     `json:"artifacts,omitempty"`
	Trace         *TraceSummary  `json:"trace,omitempty"`
	Limitations   []string       `json:"limitations,omitempty"`
}

func NewReport(command string, project ProjectSummary) Report {
	return Report{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Command:       command,
		Project:       project,
		Status:        StatusPass,
	}
}

func (r *Report) RefreshSummary() {
	r.Summary = Summary{}
	status := StatusPass
	for _, finding := range r.Findings {
		switch finding.Severity {
		case SeverityCritical:
			r.Summary.Critical++
			status = StatusFail
		case SeverityHigh:
			r.Summary.High++
			if status != StatusFail {
				status = StatusWarn
			}
		case SeverityMedium:
			r.Summary.Medium++
			if status == StatusPass {
				status = StatusWarn
			}
		case SeverityLow:
			r.Summary.Low++
		case SeverityInfo:
			r.Summary.Info++
		}
	}
	r.Status = status
}
