package perfscan

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
)

func WriteJSON(w io.Writer, report Report) error {
	report.Finalize()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteMarkdown(w io.Writer, report Report) error {
	report.Finalize()
	if _, err := fmt.Fprintf(w, "# Performance Scan\n\nProject: `%s`\n\nFindings: %d\n\nHigh: %d\n\nMedium: %d\n\nLow: %d\n\n", report.Project, report.Summary.Findings, report.Summary.High, report.Summary.Medium, report.Summary.Low); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		_, err := fmt.Fprintln(w, "No performance findings.")
		return err
	}
	if _, err := fmt.Fprintln(w, "## Findings"); err != nil {
		return err
	}
	for i, finding := range report.Findings {
		location := finding.Location.File
		if finding.Location.Line > 0 {
			location = fmt.Sprintf("%s:%d", filepath.Base(finding.Location.File), finding.Location.Line)
		}
		if _, err := fmt.Fprintf(w, "%d. `%s` [%s/%s] score=%d\n\n", i+1, finding.ID, finding.Severity, finding.Confidence, finding.Score); err != nil {
			return err
		}
		if finding.EntryPoint.Name != "" {
			if _, err := fmt.Fprintf(w, "   Entry point: `%s` `%s`\n\n", finding.EntryPoint.Kind, finding.EntryPoint.Name); err != nil {
				return err
			}
		}
		if location != "" {
			if _, err := fmt.Fprintf(w, "   Location: `%s`\n\n", location); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "   %s\n\n", finding.Message); err != nil {
			return err
		}
		for _, evidence := range finding.Evidence {
			if _, err := fmt.Fprintf(w, "   Evidence: %s", evidence.Message); err != nil {
				return err
			}
			if evidence.Value != "" {
				if _, err := fmt.Fprintf(w, " `%s`", evidence.Value); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if finding.Fix != "" {
			if _, err := fmt.Fprintf(w, "\n   Fix: %s\n\n", finding.Fix); err != nil {
				return err
			}
		}
	}
	return nil
}
