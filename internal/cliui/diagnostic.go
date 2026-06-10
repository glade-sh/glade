package cliui

import (
	"fmt"
	"io"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

func WriteDiagnostics(w io.Writer, t Theme, diags []diagnostic.Diagnostic) error {
	for i, diag := range diags {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		icon := t.GlyphFail
		severity := t.Red(string(diag.Severity))
		if !t.Color {
			severity = string(diag.Severity)
		}
		switch diag.Severity {
		case diagnostic.Warning:
			icon = t.GlyphWarn
			severity = t.Yellow(string(diag.Severity))
			if !t.Color {
				severity = string(diag.Severity)
			}
		case diagnostic.Info:
			icon = "·"
			severity = t.Dim(string(diag.Severity))
			if !t.Color {
				severity = string(diag.Severity)
			}
		}
		location := formatDiagnosticLocation(diag)
		code := ""
		if diag.Code != "" {
			code = t.Magenta("[" + diag.Code + "]")
			if !t.Color {
				code = "[" + diag.Code + "]"
			}
		}
		if _, err := fmt.Fprintf(w, "  %s  %s\n", icon, location); err != nil {
			return err
		}
		msg := diag.Message
		if code != "" {
			msg = severity + code + ": " + diag.Message
		} else {
			msg = severity + ": " + diag.Message
		}
		if _, err := fmt.Fprintf(w, "     %s\n", msg); err != nil {
			return err
		}
	}
	return nil
}

func formatDiagnosticLocation(diag diagnostic.Diagnostic) string {
	if diag.File == "" {
		return ""
	}
	if diag.Range != nil && diag.Range.Start.Line > 0 {
		return fmt.Sprintf("%s:%d:%d", diag.File, diag.Range.Start.Line, diag.Range.Start.Column)
	}
	return diag.File + ":"
}

func CountDiagnosticsBySeverity(diags []diagnostic.Diagnostic) (errors, warnings int) {
	for _, d := range diags {
		switch d.Severity {
		case diagnostic.Error:
			errors++
		case diagnostic.Warning:
			warnings++
		}
	}
	return errors, warnings
}

func FormatDiagnosticSummary(total, errors, warnings int) string {
	if total == 0 {
		return "no diagnostics"
	}
	parts := []string{fmt.Sprintf("%d diagnostic", total)}
	if total != 1 {
		parts[0] = fmt.Sprintf("%d diagnostics", total)
	}
	detail := make([]string, 0, 2)
	if errors > 0 {
		detail = append(detail, fmt.Sprintf("%d error", errors))
		if errors != 1 {
			detail[len(detail)-1] = fmt.Sprintf("%d errors", errors)
		}
	}
	if warnings > 0 {
		detail = append(detail, fmt.Sprintf("%d warning", warnings))
		if warnings != 1 {
			detail[len(detail)-1] = fmt.Sprintf("%d warnings", warnings)
		}
	}
	if len(detail) > 0 {
		return parts[0] + " (" + strings.Join(detail, ", ") + ")"
	}
	return parts[0]
}
