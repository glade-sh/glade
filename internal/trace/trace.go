package trace

import (
	"encoding/json"
	"io"
)

const (
	FormatChromeTraceEvent = "chrome-trace-event"
	Version                = 1
)

type Phase string

const (
	PhaseInstant Phase = "i"
)

type Event struct {
	Name      string         `json:"name"`
	Category  string         `json:"cat,omitempty"`
	Phase     Phase          `json:"ph"`
	Timestamp int64          `json:"ts"`
	ProcessID int            `json:"pid"`
	ThreadID  int            `json:"tid"`
	Scope     string         `json:"s,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
}

type Document struct {
	Format      string         `json:"format"`
	Version     int            `json:"version"`
	TraceEvents []Event        `json:"traceEvents"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func Instant(name, category string, timestamp int64, args map[string]any) Event {
	return Event{
		Name:      name,
		Category:  category,
		Phase:     PhaseInstant,
		Timestamp: timestamp,
		ProcessID: 1,
		ThreadID:  1,
		Scope:     "t",
		Args:      args,
	}
}

func NewDocument(events []Event) Document {
	return Document{
		Format:      FormatChromeTraceEvent,
		Version:     Version,
		TraceEvents: events,
	}
}

func WriteJSON(w io.Writer, doc Document) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
