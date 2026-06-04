package surfaceledger

import (
	"fmt"
	"os"
	"path/filepath"
)

type RefreshOptions struct {
	DocsSource          string
	ToolingCompletions  string
	TargetOrg           string
	OutputDir           string
	Release             string
	DiffFrom            string
	EvidenceFixtureGlob []string
}

type RefreshResult struct {
	OutputDir string        `json:"outputDir"`
	Summary   LedgerSummary `json:"summary"`
	Ledger    SurfaceLedger `json:"ledger,omitempty"`
}

func OutputFileNames() []string {
	return []string{
		"DOCS_SNAPSHOT.json",
		"ORG_SNAPSHOT.json",
		"GLADE_SNAPSHOT.json",
		"EVIDENCE_SNAPSHOT.json",
		"SURFACE_LEDGER.json",
		"SURFACE_DASHBOARD.md",
		"SURFACE_PROGRESS.md",
		"SURFACE_PROGRESS.html",
		"SURFACE_GAPS.md",
		"SURFACE_FAILURES.md",
		"SURFACE_RELEASE_DIFF.md",
	}
}

func Refresh(options RefreshOptions) (RefreshResult, error) {
	if options.DocsSource == "" {
		return RefreshResult{}, fmt.Errorf("docs source is required")
	}
	if options.OutputDir == "" {
		options.OutputDir = filepath.Join("docs", "generated", "salesforce")
	}
	docsRows, err := BuildDocsSnapshot(options.DocsSource)
	if err != nil {
		return RefreshResult{}, err
	}
	orgRows, err := buildOrgRows(options)
	if err != nil {
		return RefreshResult{}, err
	}
	gladeRows := BuildGladeSnapshot()
	evidenceRows, err := BuildEvidenceSnapshot(defaultEvidenceFixtures(options.EvidenceFixtureGlob))
	if err != nil {
		return RefreshResult{}, err
	}
	ledger := Merge(docsRows, orgRows, gladeRows, evidenceRows)
	AssignPriorities(ledger.Rows)
	ledger.Summary = Summarize(ledger.Rows)
	if err := writeRefreshOutputs(options.OutputDir, docsRows, orgRows, gladeRows, evidenceRows, ledger, options.DiffFrom); err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{OutputDir: options.OutputDir, Summary: ledger.Summary, Ledger: ledger}, nil
}

func buildOrgRows(options RefreshOptions) ([]SurfaceLedgerRow, error) {
	if options.TargetOrg != "" && options.ToolingCompletions == "" {
		return BuildOrgSnapshotFromTargetOrg(options.TargetOrg, options.Release)
	}
	if options.ToolingCompletions == "" {
		return nil, nil
	}
	return BuildOrgSnapshotFromToolingCompletions(options.ToolingCompletions)
}

func defaultEvidenceFixtures(patterns []string) []string {
	if len(patterns) == 0 {
		matches, _ := filepath.Glob(filepath.Join("docs", "fixtures", "*.json"))
		return matches
	}
	var out []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			out = append(out, matches...)
			continue
		}
		out = append(out, pattern)
	}
	return out
}

func writeRefreshOutputs(out string, docsRows, orgRows, gladeRows, evidenceRows []SurfaceLedgerRow, ledger SurfaceLedger, diffFrom string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	files := map[string]any{
		"DOCS_SNAPSHOT.json":     docsRows,
		"ORG_SNAPSHOT.json":      orgRows,
		"GLADE_SNAPSHOT.json":    gladeRows,
		"EVIDENCE_SNAPSHOT.json": evidenceRows,
		"SURFACE_LEDGER.json":    ledger,
	}
	for name, value := range files {
		data, err := marshalPretty(value)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(out, "SURFACE_DASHBOARD.md"), []byte(DashboardMarkdown(ledger)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "SURFACE_PROGRESS.md"), []byte(ProgressMarkdown(ledger)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "SURFACE_PROGRESS.html"), []byte(ProgressHTML(ledger)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "SURFACE_GAPS.md"), []byte(GapsMarkdown(ledger)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "SURFACE_FAILURES.md"), []byte(FailuresMarkdown(ledger)), 0o644); err != nil {
		return err
	}
	diff := "# Salesforce Surface Release Diff\n\nNo previous ledger supplied.\n"
	if diffFrom != "" {
		oldLedger, err := ReadLedgerJSON(diffFrom)
		if err != nil {
			return err
		}
		diff = ReleaseDiffMarkdown(oldLedger, ledger)
	}
	return os.WriteFile(filepath.Join(out, "SURFACE_RELEASE_DIFF.md"), []byte(diff), 0o644)
}
