package gladecli

import (
	"fmt"
	"path/filepath"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
)

// projectAutomationDiagnostics returns the warnings produced while loading the
// project's workflow and flow files, the same load the test runtime performs.
func projectAutomationDiagnostics(p project.Project) []diagnostic.Diagnostic {
	if len(p.WorkflowFiles) == 0 && len(p.FlowFiles) == 0 {
		return nil
	}
	idx, err := automation.LoadProject(p)
	if err != nil {
		return nil
	}
	return idx.Diagnostics
}

// withAutomationDiagnostics appends the project's automation warnings to a
// check result. It copies the diagnostics so a cached result is never mutated.
func withAutomationDiagnostics(result sema.Result, p project.Project) sema.Result {
	extra := projectAutomationDiagnostics(p)
	if len(extra) == 0 {
		return result
	}
	diagnostics := make([]diagnostic.Diagnostic, 0, len(result.Diagnostics)+len(extra))
	diagnostics = append(diagnostics, result.Diagnostics...)
	diagnostics = append(diagnostics, extra...)
	result.Diagnostics = diagnostics
	result.Summary.Diagnostics += len(extra)
	return result
}

// automationWarningMessages formats automation warnings for test progress
// output. Each warning includes its relative file, diagnostic code, and detail.
func automationWarningMessages(root string, diagnostics []diagnostic.Diagnostic) []string {
	var messages []string
	for _, diag := range diagnostics {
		file := diag.File
		if root != "" && file != "" {
			if rel, err := filepath.Rel(root, file); err == nil {
				file = rel
			}
		}
		detail := diag.Message
		if diag.Code != "" {
			detail = diag.Code + ": " + detail
		}
		if file == "" {
			messages = append(messages, "automation: "+detail)
			continue
		}
		messages = append(messages, fmt.Sprintf("automation: %s: %s", file, detail))
	}
	return messages
}
