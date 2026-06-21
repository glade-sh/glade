package cliui

import (
	"fmt"
	"io"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

type CheckResultInfo struct {
	ProjectRoot string
	Types       int
	Triggers    int
	Objects     int
	Diagnostics []diagnostic.Diagnostic
}

func WriteCheckResult(w io.Writer, info CheckResultInfo) error {
	t := NewTheme(w)
	if _, err := fmt.Fprintln(w, "Glade check"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if len(info.Diagnostics) == 0 {
		if _, err := fmt.Fprintln(w, t.Green(t.GlyphPass)+" No diagnostics found"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Checked:"); err != nil {
			return err
		}
		if err := FprintlnKV(w, "Apex types", fmt.Sprint(info.Types), 11); err != nil {
			return err
		}
		if err := FprintlnKV(w, "Triggers", fmt.Sprint(info.Triggers), 11); err != nil {
			return err
		}
		if err := FprintlnKV(w, "Objects", fmt.Sprint(info.Objects), 11); err != nil {
			return err
		}
		return nil
	}

	hasErrors := checkResultHasErrors(info.Diagnostics)
	heading := FormatCount(len(info.Diagnostics), "diagnostic", "diagnostics")
	icon := t.Red(t.GlyphFail)
	exitCode := "1"
	if !hasErrors {
		icon = t.Yellow(t.GlyphWarn)
		exitCode = "0"
	}
	if _, err := fmt.Fprintln(w, icon+" "+heading+" found"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	for i, diag := range info.Diagnostics {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		location := formatCheckDiagnosticLocation(info.ProjectRoot, diag)
		if strings.TrimSpace(location) != "" {
			if _, err := fmt.Fprintln(w, location); err != nil {
				return err
			}
		}
		code := strings.TrimSpace(diag.Code)
		severity := string(diag.Severity)
		if severity == "" {
			severity = string(diagnostic.Error)
		}
		if code != "" {
			if _, err := fmt.Fprintf(w, "%s %s %s\n", severity, code, diag.Message); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(w, "%s %s\n", severity, diag.Message); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Try:"); err != nil {
		return err
	}
	for _, step := range []string{"glade schema load --project .", "glade check --project ."} {
		if _, err := fmt.Fprintln(w, "  "+step); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Summary:"); err != nil {
		return err
	}
	if err := FprintlnKV(w, "apex types", fmt.Sprint(info.Types), 13); err != nil {
		return err
	}
	if err := FprintlnKV(w, "triggers", fmt.Sprint(info.Triggers), 13); err != nil {
		return err
	}
	if err := FprintlnKV(w, "objects", fmt.Sprint(info.Objects), 13); err != nil {
		return err
	}
	if err := FprintlnKV(w, "diagnostics", fmt.Sprint(len(info.Diagnostics)), 13); err != nil {
		return err
	}
	return FprintlnKV(w, "exit code", exitCode, 13)
}

func checkResultHasErrors(diagnostics []diagnostic.Diagnostic) bool {
	for _, diag := range diagnostics {
		if diag.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func formatCheckCount(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

func formatCheckDiagnosticLocation(root string, diag diagnostic.Diagnostic) string {
	file := ProjectRelativePath(root, diag.File)
	if file == "" {
		return ""
	}
	if diag.Range != nil && diag.Range.Start.Line > 0 {
		return fmt.Sprintf("%s:%d:%d", file, diag.Range.Start.Line, diag.Range.Start.Column)
	}
	return file
}
