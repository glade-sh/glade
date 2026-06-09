package perfscan

import "sort"

const SchemaVersion = 1

type Category string

const (
	CategoryApex       Category = "apex"
	CategorySOQL       Category = "soql"
	CategoryDML        Category = "dml"
	CategoryDescribe   Category = "describe"
	CategoryAutomation Category = "automation"
	CategoryUI         Category = "ui"
	CategoryAsync      Category = "async"
	CategoryMeasured   Category = "measured"
)

type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

type Confidence string

const (
	ConfidenceStatic   Confidence = "static"
	ConfidenceMeasured Confidence = "measured"
	ConfidenceCombined Confidence = "combined"
)

type EntryKind string

const (
	EntryTrigger     EntryKind = "trigger"
	EntryBatch       EntryKind = "batch"
	EntryQueueable   EntryKind = "queueable"
	EntrySchedulable EntryKind = "schedulable"
	EntryFuture      EntryKind = "future"
	EntryInvocable   EntryKind = "invocable"
	EntryVisualforce EntryKind = "visualforce"
	EntryAura        EntryKind = "aura"
	EntryLWC         EntryKind = "lwc"
	EntryFlow        EntryKind = "flow"
	EntryWorkflow    EntryKind = "workflow"
	EntryUnknown     EntryKind = "unknown"
)

type Report struct {
	SchemaVersion int           `json:"schemaVersion"`
	Project       string        `json:"project"`
	Summary       Summary       `json:"summary"`
	Findings      []Finding     `json:"findings,omitempty"`
	EntryPoints   []EntryPoint  `json:"entryPoints,omitempty"`
	Measurements  []Measurement `json:"measurements,omitempty"`
}

type Summary struct {
	Findings   int            `json:"findings"`
	High       int            `json:"high"`
	Medium     int            `json:"medium"`
	Low        int            `json:"low"`
	Categories map[string]int `json:"categories,omitempty"`
}

type Finding struct {
	ID         string     `json:"id"`
	Category   Category   `json:"category"`
	Severity   Severity   `json:"severity"`
	Confidence Confidence `json:"confidence"`
	Score      int        `json:"score"`
	EntryPoint EntryPoint `json:"entryPoint,omitempty"`
	Message    string     `json:"message"`
	Location   Location   `json:"location,omitempty"`
	Path       []PathStep `json:"path,omitempty"`
	Evidence   []Evidence `json:"evidence,omitempty"`
	Fix        string     `json:"fix,omitempty"`
}

type EntryPoint struct {
	Kind   EntryKind `json:"kind,omitempty"`
	Name   string    `json:"name,omitempty"`
	File   string    `json:"file,omitempty"`
	Line   int       `json:"line,omitempty"`
	Method string    `json:"method,omitempty"`
}

type Location struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

type PathStep struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

type Evidence struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

type Measurement struct {
	Name       string `json:"name"`
	Category   string `json:"category,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Count      int    `json:"count,omitempty"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
}

func (r *Report) AddFinding(f Finding) {
	r.Findings = append(r.Findings, f)
}

func (r *Report) AddEntryPoint(e EntryPoint) {
	if e.Kind == "" {
		e.Kind = EntryUnknown
	}
	r.EntryPoints = append(r.EntryPoints, e)
}

func (r *Report) AddMeasurement(m Measurement) {
	r.Measurements = append(r.Measurements, m)
}

func (r *Report) Finalize() {
	if r.SchemaVersion == 0 {
		r.SchemaVersion = SchemaVersion
	}
	sort.Slice(r.Findings, func(i, j int) bool {
		if r.Findings[i].Score != r.Findings[j].Score {
			return r.Findings[i].Score > r.Findings[j].Score
		}
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return severityRank(r.Findings[i].Severity) > severityRank(r.Findings[j].Severity)
		}
		return r.Findings[i].ID < r.Findings[j].ID
	})
	sort.Slice(r.EntryPoints, func(i, j int) bool {
		if r.EntryPoints[i].Kind != r.EntryPoints[j].Kind {
			return r.EntryPoints[i].Kind < r.EntryPoints[j].Kind
		}
		if r.EntryPoints[i].Name != r.EntryPoints[j].Name {
			return r.EntryPoints[i].Name < r.EntryPoints[j].Name
		}
		return r.EntryPoints[i].Line < r.EntryPoints[j].Line
	})
	sort.Slice(r.Measurements, func(i, j int) bool {
		if r.Measurements[i].DurationMS != r.Measurements[j].DurationMS {
			return r.Measurements[i].DurationMS > r.Measurements[j].DurationMS
		}
		return r.Measurements[i].Count > r.Measurements[j].Count
	})
	r.Summary = Summary{Findings: len(r.Findings), Categories: map[string]int{}}
	for _, finding := range r.Findings {
		switch finding.Severity {
		case SeverityHigh:
			r.Summary.High++
		case SeverityMedium:
			r.Summary.Medium++
		default:
			r.Summary.Low++
		}
		r.Summary.Categories[string(finding.Category)]++
	}
	if len(r.Summary.Categories) == 0 {
		r.Summary.Categories = nil
	}
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	default:
		return 1
	}
}
