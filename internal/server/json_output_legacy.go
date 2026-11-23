//go:build !goexperiment.jsonv2

package server

import (
	"encoding/json"
	"io"
)

func writeServerJSON(w io.Writer, value any) error {
	return json.NewEncoder(w).Encode(value)
}
