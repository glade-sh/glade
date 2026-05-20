package oracle

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	case "salesforce", "oaer":
	default:
		return fmt.Errorf("source must be salesforce or oaer")
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
