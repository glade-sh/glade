package gladecli

import (
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
)

func withProjectSourceWarnings(result sema.Result, p project.Project) sema.Result {
	if len(p.SourceWarnings) == 0 {
		return result
	}
	diagnostics := make([]diagnostic.Diagnostic, 0, len(result.Diagnostics)+len(p.SourceWarnings))
	diagnostics = append(diagnostics, result.Diagnostics...)
	for _, warning := range p.SourceWarnings {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Warning,
			Code:     "GLADEPROJECT001",
			Message:  warning,
		})
	}
	result.Diagnostics = diagnostics
	result.Summary.Diagnostics += len(p.SourceWarnings)
	return result
}
