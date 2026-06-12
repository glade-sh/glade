package refactorproof

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/trace"
)

const (
	StageGitDiff         = "git_diff"
	StageParseCheck      = "parse_check"
	StageSemanticCheck   = "semantic_check"
	StageGraphImpact     = "graph_impact"
	StageAffectedTests   = "affected_tests"
	StageRuntimeTrace    = "runtime_trace"
	StageAPISurfaceDelta = "api_surface_delta"
)

const (
	StageStatusPass        = "pass"
	StageStatusWarn        = "warn"
	StageStatusFail        = "fail"
	StageStatusNotRun      = "not_run"
	StageStatusUnsupported = "unsupported"
)

type StageResult struct {
	Name    string         `json:"name"`
	Status  string         `json:"status"`
	Message string         `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

type Options struct {
	Root           string
	Since          string
	TracePath      string
	FailOnAPIBreak bool
}

type Result struct {
	Stages []StageResult     `json:"stages"`
	Report enterprise.Report `json:"report"`
}

func (r Result) Status() string {
	status := StageStatusPass
	for _, stage := range r.Stages {
		switch stage.Status {
		case StageStatusFail:
			return StageStatusFail
		case StageStatusWarn, StageStatusNotRun, StageStatusUnsupported:
			status = StageStatusWarn
		}
	}
	return status
}

func Prove(ctx context.Context, opts Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}

	entCtx, err := enterprise.LoadContext(absRoot)
	if err != nil {
		return Result{}, err
	}
	report := enterprise.NewReport("glade refactor proof", entCtx.Summary())
	result := Result{Report: report}

	parseStage := diagnosticStage(StageParseCheck, entCtx.Index.Diagnostics, "parse and index checks passed")
	result.Stages = append(result.Stages, parseStage)
	result.Stages = append(result.Stages, diagnosticStage(StageSemanticCheck, entCtx.Sema.Diagnostics, "semantic checks passed"))

	changes, gitStage := changedFilesStage(absRoot, opts.Since)
	result.Stages = append(result.Stages, gitStage)

	selection := SelectAffectedTests(entCtx.Index, changes)
	result.Stages = append(result.Stages, StageResult{
		Name:    StageGraphImpact,
		Status:  StageStatusPass,
		Message: "graph impact selected",
		Details: map[string]any{
			"mode":          selection.Mode,
			"test_classes":  selection.TestClasses,
			"changed_files": len(changes),
		},
	})
	result.Stages = append(result.Stages, StageResult{
		Name:    StageAffectedTests,
		Status:  StageStatusNotRun,
		Message: "affected test execution is not wired for refactor proof yet",
		Details: map[string]any{
			"mode":         selection.Mode,
			"test_classes": selection.TestClasses,
		},
	})

	result.Stages = append(result.Stages, runtimeTraceStage(opts.TracePath, &result.Report))
	result.Stages = append(result.Stages, CheckAPISurfaceChanges(absRoot, opts.Since, changes, APISurfaceOptions{FailOnBreak: opts.FailOnAPIBreak}))

	result.Report.Sections = proofSections(result.Stages)
	result.Report.Findings = proofFindings(result.Stages)
	result.Report.RefreshSummary()
	result.Report.Status = enterprise.Status(result.Status())
	return result, nil
}

func changedFilesStage(root, since string) ([]ChangedFile, StageResult) {
	changes, err := ChangedFiles(root, since)
	if err != nil {
		return nil, StageResult{
			Name:    StageGitDiff,
			Status:  StageStatusFail,
			Message: err.Error(),
		}
	}
	return changes, StageResult{
		Name:    StageGitDiff,
		Status:  StageStatusPass,
		Message: fmt.Sprintf("%d changed file(s)", len(changes)),
		Details: map[string]any{"changed_files": changes},
	}
}

func diagnosticStage(name string, diagnostics []diagnostic.Diagnostic, passMessage string) StageResult {
	errCount := 0
	for _, diag := range diagnostics {
		if diag.Severity == diagnostic.Error {
			errCount++
		}
	}
	if errCount == 0 {
		return StageResult{Name: name, Status: StageStatusPass, Message: passMessage}
	}
	return StageResult{
		Name:    name,
		Status:  StageStatusFail,
		Message: fmt.Sprintf("%d error diagnostic(s)", errCount),
		Details: map[string]any{
			"diagnostics": diagnostics,
		},
	}
}

func runtimeTraceStage(tracePath string, report *enterprise.Report) StageResult {
	if tracePath == "" {
		return StageResult{Name: StageRuntimeTrace, Status: StageStatusNotRun, Message: "no trace path supplied"}
	}
	data, err := os.ReadFile(tracePath)
	if err != nil {
		return StageResult{Name: StageRuntimeTrace, Status: StageStatusWarn, Message: err.Error()}
	}
	var doc trace.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return StageResult{Name: StageRuntimeTrace, Status: StageStatusWarn, Message: err.Error()}
	}
	summary := enterprise.SummarizeTrace(doc.TraceEvents)
	report.Trace = &summary
	return StageResult{
		Name:    StageRuntimeTrace,
		Status:  StageStatusPass,
		Message: fmt.Sprintf("%d trace event(s)", summary.Events),
		Details: map[string]any{"trace": summary},
	}
}

func proofSections(stages []StageResult) []enterprise.Section {
	items := make([]enterprise.SectionItem, 0, len(stages))
	for _, stage := range stages {
		items = append(items, enterprise.SectionItem{
			Label: stage.Name,
			Value: stage.Status,
			Details: map[string]any{
				"message": stage.Message,
				"details": stage.Details,
			},
		})
	}
	return []enterprise.Section{{
		ID:      "refactor-proof",
		Title:   "Refactor Proof",
		Summary: "Proof stages for the selected change set.",
		Items:   items,
	}}
}

func proofFindings(stages []StageResult) []enterprise.Finding {
	var findings []enterprise.Finding
	for _, stage := range stages {
		switch stage.Status {
		case StageStatusFail:
			findings = append(findings, proofFinding(stage, enterprise.SeverityCritical))
		case StageStatusWarn, StageStatusNotRun, StageStatusUnsupported:
			findings = append(findings, proofFinding(stage, enterprise.SeverityMedium))
		}
	}
	return findings
}

func proofFinding(stage StageResult, severity enterprise.Severity) enterprise.Finding {
	return enterprise.Finding{
		ID:             "ENT-REFACTOR-PROOF-" + stage.Name,
		Category:       enterprise.CategoryRefactor,
		Severity:       severity,
		Confidence:     enterprise.ConfidenceHigh,
		Title:          stage.Name,
		Summary:        stage.Message,
		Evidence:       []enterprise.Evidence{{Type: enterprise.EvidenceHeuristic, Message: stage.Status, Details: stage.Details}},
		Recommendation: "Inspect the proof stage before merging.",
	}
}
