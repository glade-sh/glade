package oracle

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type OracleReport struct {
	Target          string        `json:"target"`
	Ready           bool          `json:"ready"`
	Project         string        `json:"project,omitempty"`
	RunID           string        `json:"runId,omitempty"`
	ArtifactDir     string        `json:"artifactDir,omitempty"`
	GoldenOnly      bool          `json:"goldenOnly,omitempty"`
	Summary         OracleSummary `json:"summary"`
	Comparisons     []OracleDiff  `json:"comparisons,omitempty"`
	SalesforceCount int           `json:"salesforceCount,omitempty"`
	LocalCount      int           `json:"localCount,omitempty"`
	Messages        []string      `json:"messages,omitempty"`
}

type OracleSummary struct {
	Total               int `json:"total"`
	Pass                int `json:"pass"`
	TraceMismatch       int `json:"traceMismatch,omitempty"`
	StateMismatch       int `json:"stateMismatch,omitempty"`
	ExceptionMismatch   int `json:"exceptionMismatch,omitempty"`
	Unsupported         int `json:"unsupported,omitempty"`
	CompileGap          int `json:"compileGap,omitempty"`
	InfrastructureError int `json:"infrastructureError,omitempty"`
}

type OracleCorpusBaseline struct {
	Target string             `json:"target"`
	Cases  []OracleCorpusCase `json:"cases"`
}

type OracleCorpusCase struct {
	Name          string                  `json:"name"`
	Project       string                  `json:"project,omitempty"`
	SalesforceRun string                  `json:"salesforceRun"`
	LocalRun      string                  `json:"localRun"`
	Ready         bool                    `json:"ready"`
	Summary       OracleSummary           `json:"summary"`
	Outcomes      []OracleExpectedOutcome `json:"outcomes,omitempty"`
}

type OracleExpectedOutcome struct {
	Class   string        `json:"class,omitempty"`
	Method  string        `json:"method,omitempty"`
	Outcome OracleOutcome `json:"outcome"`
}

type OracleCorpusReport struct {
	Target   string                   `json:"target"`
	Ready    bool                     `json:"ready"`
	Baseline string                   `json:"baseline"`
	Cases    []OracleCorpusCaseResult `json:"cases"`
	Failures []string                 `json:"failures,omitempty"`
}

type OracleCorpusCaseResult struct {
	Name     string                  `json:"name"`
	Project  string                  `json:"project,omitempty"`
	Ready    bool                    `json:"ready"`
	Summary  OracleSummary           `json:"summary"`
	Outcomes []OracleExpectedOutcome `json:"outcomes,omitempty"`
}

func NewReport(project, runID, artifactDir string, diffs []OracleDiff, salesforceCount, localCount int, goldenOnly bool) OracleReport {
	report := OracleReport{
		Target:          "Salesforce oracle parity",
		Project:         project,
		RunID:           runID,
		ArtifactDir:     artifactDir,
		GoldenOnly:      goldenOnly,
		Comparisons:     diffs,
		SalesforceCount: salesforceCount,
		LocalCount:      localCount,
	}
	for _, diff := range diffs {
		report.Summary.Total++
		switch diff.Outcome {
		case OracleOutcomePass:
			report.Summary.Pass++
		case OracleOutcomeTraceMismatch:
			report.Summary.TraceMismatch++
		case OracleOutcomeStateMismatch:
			report.Summary.StateMismatch++
		case OracleOutcomeExceptionMismatch:
			report.Summary.ExceptionMismatch++
		case OracleOutcomeUnsupported:
			report.Summary.Unsupported++
		case OracleOutcomeCompileGap:
			report.Summary.CompileGap++
		case OracleOutcomeInfrastructureError:
			report.Summary.InfrastructureError++
		}
	}
	if goldenOnly && len(diffs) == 0 {
		report.Summary.Total = salesforceCount
		report.Summary.Pass = 0
	}
	report.Ready = !goldenOnly && report.Summary.Total > 0 && report.Summary.Total == report.Summary.Pass
	return report
}

func InfrastructureReport(project, runID, artifactDir string, message string) OracleReport {
	return NewReport(project, runID, artifactDir, []OracleDiff{{
		Outcome: OracleOutcomeInfrastructureError,
		Details: []string{message},
	}}, 0, 0, false)
}

func WriteReportJSON(w io.Writer, report OracleReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func ReadRuns(path string) ([]OracleRun, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var runs []OracleRun
	if err := json.Unmarshal(data, &runs); err == nil {
		if err := validateRuns(path, runs); err != nil {
			return nil, err
		}
		return normalizeRuns(runs), nil
	}
	var wrapped struct {
		Runs []OracleRun `json:"runs"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Runs != nil {
		if err := validateRuns(path, wrapped.Runs); err != nil {
			return nil, err
		}
		return normalizeRuns(wrapped.Runs), nil
	}
	var run OracleRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("read oracle runs %s: %w", path, err)
	}
	if err := validateRuns(path, []OracleRun{run}); err != nil {
		return nil, err
	}
	return []OracleRun{NormalizeRun(run)}, nil
}

func CheckCorpus(path string) (OracleCorpusReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OracleCorpusReport{}, err
	}
	var baseline OracleCorpusBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return OracleCorpusReport{}, err
	}
	if len(baseline.Cases) == 0 {
		return OracleCorpusReport{}, fmt.Errorf("oracle corpus %s: at least one case is required", path)
	}
	if strings.TrimSpace(baseline.Target) == "" {
		baseline.Target = "oracle parity corpus"
	}
	absPath, _ := filepath.Abs(path)
	baseDir := filepath.Dir(absPath)
	report := OracleCorpusReport{
		Target:   baseline.Target,
		Ready:    true,
		Baseline: absPath,
	}
	for _, expected := range baseline.Cases {
		if strings.TrimSpace(expected.Name) == "" {
			return report, fmt.Errorf("oracle corpus %s: case name is required", path)
		}
		salesforcePath := resolveCorpusPath(baseDir, expected.SalesforceRun)
		localPath := resolveCorpusPath(baseDir, expected.LocalRun)
		salesforceRuns, err := ReadRuns(salesforcePath)
		if err != nil {
			return report, err
		}
		localRuns, err := ReadRuns(localPath)
		if err != nil {
			return report, err
		}
		diffs := DiffRunSets(salesforceRuns, localRuns)
		actual := NewReport(expected.Project, "", "", diffs, len(salesforceRuns), len(localRuns), false)
		result := OracleCorpusCaseResult{
			Name:     expected.Name,
			Project:  expected.Project,
			Ready:    actual.Ready,
			Summary:  actual.Summary,
			Outcomes: stableOracleOutcomes(diffs),
		}
		report.Cases = append(report.Cases, result)
		if result.Ready != expected.Ready {
			report.Failures = append(report.Failures, fmt.Sprintf("%s ready = %t, want %t", expected.Name, result.Ready, expected.Ready))
		}
		if result.Summary != expected.Summary {
			report.Failures = append(report.Failures, fmt.Sprintf("%s summary = %+v, want %+v", expected.Name, result.Summary, expected.Summary))
		}
		compareOracleOutcomes(expected.Name, result.Outcomes, stableExpectedOracleOutcomes(expected.Outcomes), &report.Failures)
	}
	report.Ready = len(report.Failures) == 0
	if !report.Ready {
		return report, fmt.Errorf("oracle corpus baseline mismatch: %s", strings.Join(report.Failures, "; "))
	}
	return report, nil
}

func PersistArtifacts(dir string, salesforceRuns, localRuns []OracleRun, diffs []OracleDiff, report OracleReport) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if salesforceRuns != nil {
		if err := writeJSONFile(filepath.Join(dir, "salesforce.json"), normalizeRuns(salesforceRuns)); err != nil {
			return err
		}
	}
	if localRuns != nil {
		if err := writeJSONFile(filepath.Join(dir, "local.json"), normalizeRuns(localRuns)); err != nil {
			return err
		}
	}
	if err := writeJSONFile(filepath.Join(dir, "diff.json"), diffs); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(dir, "report.json"), report)
}

func normalizeRuns(runs []OracleRun) []OracleRun {
	out := make([]OracleRun, len(runs))
	for i := range runs {
		out[i] = NormalizeRun(runs[i])
	}
	return out
}

func validateRuns(path string, runs []OracleRun) error {
	if len(runs) == 0 {
		return fmt.Errorf("read oracle runs %s: at least one run is required", path)
	}
	for i, run := range runs {
		if err := validateRun(run); err != nil {
			return fmt.Errorf("read oracle runs %s: run[%d]: %w", path, i, err)
		}
	}
	return nil
}

func validateRun(run OracleRun) error {
	if run.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schemaVersion = %d, want %d", run.SchemaVersion, SchemaVersion)
	}
	switch strings.ToLower(strings.TrimSpace(run.Source)) {
	case "salesforce", "glade":
	default:
		return fmt.Errorf("source must be salesforce or glade")
	}
	switch run.Status {
	case OracleStatusPass, OracleStatusFail, OracleStatusSkipped, OracleStatusUnsupported, OracleStatusCompileError, OracleStatusRuntimeError, OracleStatusInfrastructureError:
	default:
		return fmt.Errorf("status is required")
	}
	if strings.TrimSpace(run.TestClass) == "" && strings.TrimSpace(run.TestMethod) == "" {
		return fmt.Errorf("testClass or testMethod is required")
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func WriteCorpusJSON(w io.Writer, report OracleCorpusReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteCorpusText(w io.Writer, report OracleCorpusReport) {
	state := "ready"
	if !report.Ready {
		state = "not ready"
	}
	fmt.Fprintf(w, "Oracle corpus: %s\n", state)
	fmt.Fprintf(w, "baseline: %s\n", report.Baseline)
	for _, c := range report.Cases {
		fmt.Fprintf(w, "- %s: ready=%t pass=%d trace_mismatch=%d state_mismatch=%d exception_mismatch=%d unsupported=%d compile_gap=%d infrastructure_error=%d total=%d\n",
			c.Name,
			c.Ready,
			c.Summary.Pass,
			c.Summary.TraceMismatch,
			c.Summary.StateMismatch,
			c.Summary.ExceptionMismatch,
			c.Summary.Unsupported,
			c.Summary.CompileGap,
			c.Summary.InfrastructureError,
			c.Summary.Total,
		)
	}
	for _, failure := range report.Failures {
		fmt.Fprintf(w, "! %s\n", failure)
	}
}

func resolveCorpusPath(baseDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Clean(filepath.Join(baseDir, p))
}

func stableOracleOutcomes(diffs []OracleDiff) []OracleExpectedOutcome {
	out := make([]OracleExpectedOutcome, 0, len(diffs))
	for _, diff := range diffs {
		out = append(out, OracleExpectedOutcome{
			Class:   diff.TestClass,
			Method:  diff.TestMethod,
			Outcome: diff.Outcome,
		})
	}
	return stableExpectedOracleOutcomes(out)
}

func stableExpectedOracleOutcomes(outcomes []OracleExpectedOutcome) []OracleExpectedOutcome {
	out := append([]OracleExpectedOutcome(nil), outcomes...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Class == out[j].Class {
			if out[i].Method == out[j].Method {
				return out[i].Outcome < out[j].Outcome
			}
			return out[i].Method < out[j].Method
		}
		return out[i].Class < out[j].Class
	})
	return out
}

func compareOracleOutcomes(name string, actual, expected []OracleExpectedOutcome, failures *[]string) {
	if len(actual) != len(expected) {
		*failures = append(*failures, fmt.Sprintf("%s outcomes length = %d, want %d", name, len(actual), len(expected)))
		return
	}
	for i := range expected {
		if actual[i] != expected[i] {
			*failures = append(*failures, fmt.Sprintf("%s outcome[%d] = %+v, want %+v", name, i, actual[i], expected[i]))
		}
	}
}
