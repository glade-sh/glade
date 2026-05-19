package probe

// ProbeResult is the normalized response shape from both golden and local runs.
type ProbeResult struct {
	ProbeID          string      `json:"probeId"`
	Category         string      `json:"category"`
	Result           interface{} `json:"result"`
	ExceptionType    *string     `json:"exceptionType"`
	ExceptionMessage *string     `json:"exceptionMessage"`
}

// GoldenResponse and LocalResponse are aliases for clarity.
type GoldenResponse = ProbeResult
type LocalResponse = ProbeResult

// GapType categorizes the kind of discrepancy between org and local behavior.
type GapType string

const (
	GapTypeBehavioral  GapType = "behavioral_gap"
	GapTypeUnsupported GapType = "unsupported_gap"
	GapTypePanic       GapType = "panic_gap"
	GapTypeLimit       GapType = "limit_gap"
	GapTypeMetadata    GapType = "metadata_gap"
)

// GapEntry is a single discovered difference.
type GapEntry struct {
	ProbeID      string      `json:"probeId"`
	Category     string      `json:"category"`
	GapType      GapType     `json:"gapType"`
	Severity     string      `json:"severity"`
	CapabilityID string      `json:"capabilityId,omitempty"`
	Golden       interface{} `json:"golden"`
	Local        interface{} `json:"local"`
	Diff         string      `json:"diff"`
}

// GapReport is the top-level output of a probe run.
type GapReport struct {
	ProbesRun    int                    `json:"probesRun"`
	GapsFound    int                    `json:"gapsFound"`
	Panics       int                    `json:"panics"`
	Unsupported  int                    `json:"unsupported"`
	Behavioral   int                    `json:"behavioral"`
	Entries      []GapEntry             `json:"entries"`
	Timings      []Timing               `json:"timings,omitempty"`
	ProbeTimings []ProbeTiming          `json:"probeTimings,omitempty"`
	OrgShape     map[string]interface{} `json:"orgShape,omitempty"`
	RunMeta      RunMeta                `json:"runMeta"`
}

// LocalRunReport records local-only probe results without implying an org diff.
type LocalRunReport struct {
	ProbesRun int           `json:"probesRun"`
	Results   []ProbeResult `json:"results"`
}

// Config drives a single probe run.
type Config struct {
	ProbeDir       string // path to probes/sfdx
	OrgAlias       string // sfdx target org alias or username
	OutputDir      string // where to write gap-report.json
	ProbeIDs       []string
	Features       []string // org shape features (e.g. MultiCurrency)
	Tier           string
	GoldenCache    string
	UseGoldenCache bool
	GoldenExecutor string // rest (default) or sf
}

// Timing records elapsed time for a top-level probe runner phase.
type Timing struct {
	Phase      string `json:"phase"`
	DurationMS int64  `json:"durationMs"`
}

// ProbeTiming records elapsed time for an individual probe or batch.
type ProbeTiming struct {
	Phase      string   `json:"phase"`
	ProbeID    string   `json:"probeId,omitempty"`
	ProbeIDs   []string `json:"probeIds,omitempty"`
	Mode       string   `json:"mode"`
	DurationMS int64    `json:"durationMs"`
}

// RunMeta records the shape inputs used by a probe run.
type RunMeta struct {
	StartedAtUTC    string   `json:"startedAtUtc"`
	ProbeVersion    string   `json:"probeVersion"`
	ManifestVersion string   `json:"manifestVersion"`
	SeedVersion     string   `json:"seedVersion"`
	Tier            string   `json:"tier"`
	Features        []string `json:"features,omitempty"`
	GoldenSource    string   `json:"goldenSource"`
}

// GoldenCache stores reusable org responses for local iteration.
type GoldenCache struct {
	RunMeta  RunMeta                `json:"runMeta"`
	OrgShape map[string]interface{} `json:"orgShape"`
	Manifest []ProbeSpec            `json:"manifest"`
	Results  []ProbeResult          `json:"results"`
}

// TrendEntry is appended after every org probe run.
type TrendEntry struct {
	StartedAtUTC     string `json:"startedAtUtc"`
	ProbeVersion     string `json:"probeVersion"`
	ManifestVersion  string `json:"manifestVersion"`
	SeedVersion      string `json:"seedVersion"`
	Tier             string `json:"tier"`
	ProbesRun        int    `json:"probesRun"`
	GapsFound        int    `json:"gapsFound"`
	Unsupported      int    `json:"unsupported"`
	Behavioral       int    `json:"behavioral"`
	Panics           int    `json:"panics"`
	GoldenDurationMS int64  `json:"goldenDurationMs"`
	LocalDurationMS  int64  `json:"localDurationMs"`
	GoldenSource     string `json:"goldenSource"`
	OrganizationID   string `json:"organizationId,omitempty"`
}
