package cliui

import (
	"fmt"
	"io"
)

func WriteCLIError(w io.Writer, err error) error {
	if err == nil {
		return nil
	}
	t := NewTheme(w)
	if _, e := fmt.Fprintf(w, "  %s  glade: %s\n\n", t.Red(t.GlyphFail), err.Error()); e != nil {
		return e
	}
	_, e := fmt.Fprintln(w, t.Dim("  Run glade help for usage."))
	return e
}
