package cliui

import (
	"encoding/json"
	"io"
	"time"
)

type NDJSONRenderer struct {
	enc *json.Encoder
}

func NewNDJSONRenderer(w io.Writer) *NDJSONRenderer {
	return &NDJSONRenderer{enc: json.NewEncoder(w)}
}

func (r *NDJSONRenderer) Render(ev Event) {
	if r == nil || r.enc == nil {
		return
	}
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	_ = r.enc.Encode(ev)
}

func (r *NDJSONRenderer) Finish(result Result) {
	if r == nil || r.enc == nil {
		return
	}
	ok := result.OK
	exitCode := result.ExitCode
	if !result.OK && exitCode == 0 {
		exitCode = 1
	}
	_ = r.enc.Encode(Event{
		Kind:     EventDone,
		Label:    result.Label,
		OK:       &ok,
		ExitCode: &exitCode,
		At:       time.Now().UTC(),
	})
}
