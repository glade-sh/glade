//go:build !goexperiment.jsonv2

package testreport

import (
	"encoding/json"
	"io"
)

func writeReportJSON(w io.Writer, value Run) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
