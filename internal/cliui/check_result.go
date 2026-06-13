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
	errors, warnings := CountDiagnosticsBySeverity(info.Diagnostics)
	summary := FormatDiagnosticSummary(len(info.Diagnostics), errors, warnings)
	body := strings.Join([]string{
		"project: " + info.ProjectRoot,
		fmt.Sprintf("%s · %s · %s", formatCheckCount(info.Types, "type"), formatCheckCount(info.Triggers, "trigger"), formatCheckCount(info.Objects, "object")),
		summary,
	}, "\n")
	if _, err := fmt.Fprintln(w, FormatBox(t, "Check", body, defaultBoxWidth)); err != nil {
		return err
	}
	if len(info.Diagnostics) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return WriteDiagnostics(w, t, info.Diagnostics)
}

func formatCheckCount(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
