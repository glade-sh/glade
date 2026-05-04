package compat

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sema"
	"github.com/open-aer/oaer/internal/typesys"
)

type ReadinessReport struct {
	SchemaVersion int                `json:"schemaVersion"`
	Project       string             `json:"project"`
	OK            bool               `json:"ok"`
	Summary       ReadinessSummary   `json:"summary"`
	Blockers      []ReadinessBlocker `json:"blockers,omitempty"`
}

type ReadinessSummary struct {
	Blockers   int            `json:"blockers"`
	Warnings   int            `json:"warnings"`
	Categories map[string]int `json:"categories"`
}

type ReadinessBlocker struct {
	Category   string `json:"category"`
	Code       string `json:"code,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Capability string `json:"capability,omitempty"`
}

func AnalyzeReadiness(root string) (ReadinessReport, error) {
	report := ReadinessReport{
		SchemaVersion: ReplaySchemaVersion,
		Project:       root,
		OK:            true,
		Summary:       ReadinessSummary{Categories: map[string]int{}},
	}
	proj, err := project.Load(root)
	if err != nil {
		report.addBlocker(blockerFromError("project", "OAERPROJECT001", err))
		return report, nil
	}
	report.Project = proj.Root
	sch, err := schema.LoadProject(proj)
	if err != nil {
		report.addBlocker(blockerFromError("schema", "OAERSCHEMA001", err))
		return report, nil
	}
	result := sema.Analyze(typesys.Build(proj, sch))
	for _, diag := range result.Diagnostics {
		blocker := ClassifyReadinessDiagnostic(diag)
		if diag.Severity == diagnostic.Warning {
			report.Summary.Warnings++
		} else {
			report.addBlocker(blocker)
		}
	}
	sortReadinessBlockers(report.Blockers)
	return report, nil
}

func WriteReadinessJSON(w io.Writer, report ReadinessReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteReadinessText(w io.Writer, report ReadinessReport) {
	state := "ready"
	if !report.OK {
		state = "blocked"
	}
	fmt.Fprintf(w, "Project readiness: %s\n", state)
	fmt.Fprintf(w, "Blockers: %d\n", report.Summary.Blockers)
	if len(report.Summary.Categories) == 0 {
		return
	}
	fmt.Fprintln(w)
	categories := make([]string, 0, len(report.Summary.Categories))
	for category := range report.Summary.Categories {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		fmt.Fprintf(w, "%s: %d\n", category, report.Summary.Categories[category])
		for _, blocker := range report.Blockers {
			if blocker.Category != category {
				continue
			}
			location := blocker.File
			if blocker.Line > 0 {
				location = fmt.Sprintf("%s:%d", blocker.File, blocker.Line)
			}
			if location != "" {
				fmt.Fprintf(w, "  - %s at %s\n", blocker.Message, location)
			} else {
				fmt.Fprintf(w, "  - %s\n", blocker.Message)
			}
		}
	}
}

func ClassifyReadinessDiagnostic(diag diagnostic.Diagnostic) ReadinessBlocker {
	category := classifyReadinessCategory(diag.Code, diag.Message)
	blocker := ReadinessBlocker{
		Category: category,
		Code:     diag.Code,
		Symbol:   extractReadinessSymbol(diag.Message),
		Severity: string(diag.Severity),
		Message:  diag.Message,
		File:     diag.File,
	}
	if blocker.Severity == "" {
		blocker.Severity = "blocking"
	}
	if diag.Range != nil {
		blocker.Line = diag.Range.Start.Line
		blocker.Column = diag.Range.Start.Column
	}
	blocker.Capability = readinessCapability(blocker)
	return blocker
}

func (r *ReadinessReport) addBlocker(blocker ReadinessBlocker) {
	r.OK = false
	r.Blockers = append(r.Blockers, blocker)
	r.Summary.Blockers = len(r.Blockers)
	if r.Summary.Categories == nil {
		r.Summary.Categories = map[string]int{}
	}
	r.Summary.Categories[blocker.Category]++
}

func blockerFromError(category, code string, err error) ReadinessBlocker {
	return ReadinessBlocker{
		Category: category,
		Code:     code,
		Severity: "error",
		Message:  err.Error(),
	}
}

func classifyReadinessCategory(code, message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(code, "PARSE"), strings.Contains(code, "TYPE"):
		return "parser"
	case strings.Contains(code, "SEMA"):
		return "sema"
	case strings.Contains(lower, "system.") || strings.Contains(lower, "string.") || strings.Contains(lower, "json.") || strings.Contains(lower, "unsupported call"):
		return "stdlib"
	case strings.Contains(lower, "soql") || strings.Contains(lower, "sosl") || strings.Contains(lower, " find "):
		return "soql"
	case strings.Contains(lower, " dml") || strings.Contains(lower, "database."):
		return "dml"
	case strings.Contains(lower, "trigger"):
		return "triggers"
	case strings.Contains(lower, "limit"):
		return "limits"
	case strings.Contains(lower, "storage") || strings.Contains(lower, "fixture") || strings.Contains(lower, "sqlite"):
		return "storage"
	case strings.Contains(lower, "server") || strings.Contains(lower, "rest") || strings.Contains(lower, "tooling") || strings.Contains(lower, "composite"):
		return "server"
	default:
		return "unknown"
	}
}

var readinessSymbolPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(System\.[A-Za-z0-9_$.]+)`),
	regexp.MustCompile(`\b(String\.[A-Za-z0-9_$.]+)`),
	regexp.MustCompile(`\b(JSON\.[A-Za-z0-9_$.]+)`),
	regexp.MustCompile(`unsupported call "([^"]+)"`),
	regexp.MustCompile(`unknown type "([^"]+)"`),
	regexp.MustCompile(`unknown SObject "([^"]+)"`),
}

func extractReadinessSymbol(message string) string {
	for _, pattern := range readinessSymbolPatterns {
		match := pattern.FindStringSubmatch(message)
		if len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

func readinessCapability(blocker ReadinessBlocker) string {
	symbol := strings.ToLower(blocker.Symbol)
	switch {
	case blocker.Category == "stdlib" && strings.Contains(symbol, "json"):
		return "stdlib.json"
	case blocker.Category == "stdlib" && strings.Contains(symbol, "string"):
		return "stdlib.string"
	case blocker.Category == "soql":
		return "data.soql"
	case blocker.Category == "dml":
		return "data.dml"
	case blocker.Category == "triggers":
		return "triggers.runtime"
	case blocker.Category == "server":
		return "server.api"
	default:
		return ""
	}
}

func sortReadinessBlockers(blockers []ReadinessBlocker) {
	sort.Slice(blockers, func(i, j int) bool {
		if blockers[i].Category != blockers[j].Category {
			return blockers[i].Category < blockers[j].Category
		}
		if blockers[i].File != blockers[j].File {
			return blockers[i].File < blockers[j].File
		}
		if blockers[i].Line != blockers[j].Line {
			return blockers[i].Line < blockers[j].Line
		}
		return blockers[i].Message < blockers[j].Message
	})
}

func BundleOutFromReadiness(outDir, projectRoot string, report ReadinessReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	var buf strings.Builder
	if err := WriteReadinessJSON(&buf, report); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "readiness.json"), []byte(buf.String()), 0o644); err != nil {
		return err
	}
	return copyReplayBundle(filepath.Join(outDir, "bundle"), projectRoot)
}
