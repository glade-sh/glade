package diagnostic

import (
	"encoding/json"
	"fmt"
	"io"
)

type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
)

type Position struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
	Offset int `json:"offset,omitempty"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code,omitempty"`
	Message  string   `json:"message"`
	File     string   `json:"file,omitempty"`
	Range    *Range   `json:"range,omitempty"`
	Excerpt  string   `json:"excerpt,omitempty"`
}

type Report struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func (r Report) HasErrors() bool {
	for _, diag := range r.Diagnostics {
		if diag.Severity == Error {
			return true
		}
	}
	return false
}

func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func (r Report) WriteText(w io.Writer) error {
	for i, diag := range r.Diagnostics {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if diag.File != "" {
			if diag.Range != nil && diag.Range.Start.Line > 0 {
				_, _ = fmt.Fprintf(w, "%s:%d:%d: ", diag.File, diag.Range.Start.Line, diag.Range.Start.Column)
			} else {
				_, _ = fmt.Fprintf(w, "%s: ", diag.File)
			}
		}
		if diag.Code != "" {
			_, _ = fmt.Fprintf(w, "%s[%s]: %s", diag.Severity, diag.Code, diag.Message)
		} else {
			_, _ = fmt.Fprintf(w, "%s: %s", diag.Severity, diag.Message)
		}
	}
	return nil
}
