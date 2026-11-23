//go:build goexperiment.jsonv2

package server

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"io"
)

func writeServerJSON(w io.Writer, value any) error {
	data, err := jsonv2.Marshal(value, json.DefaultOptionsV1())
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}
