//go:build goexperiment.jsonv2

package testreport

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"io"
)

func writeReportJSON(w io.Writer, value Run) error {
	data, err := jsonv2.Marshal(value, json.DefaultOptionsV1(), jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}
