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
	_ = r.enc.Encode(Event{
		Kind:  EventDone,
		Label: result.Label,
		At:    time.Now().UTC(),
	})
}
