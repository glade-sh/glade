package apexast

import "fmt"

func FormatDiagnostic(diag Diagnostic) string {
	if diag.File == "" || diag.Range == nil {
		return diag.Message
	}
	return fmt.Sprintf("%s:%d:%d: %s", diag.File, diag.Range.Start.Line, diag.Range.Start.Column, diag.Message)
}
