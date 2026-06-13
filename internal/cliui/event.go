package cliui

import "time"

type EventKind string

const (
	EventPhaseStart EventKind = "phase_start"
	EventPhaseTick  EventKind = "phase_tick"
	EventPhaseEnd   EventKind = "phase_end"
	EventInfo       EventKind = "info"
	EventWarn       EventKind = "warn"
	EventFail       EventKind = "fail"
	EventDone       EventKind = "done"
)

type Event struct {
	Kind     EventKind `json:"kind"`
	Phase    string    `json:"phase,omitempty"`
	Label    string    `json:"label,omitempty"`
	Detail   string    `json:"detail,omitempty"`
	Current  int       `json:"current,omitempty"`
	Total    int       `json:"total,omitempty"`
	OK       *bool     `json:"ok,omitempty"`
	ExitCode *int      `json:"exitCode,omitempty"`
	At       time.Time `json:"at,omitempty"`
}

type Result struct {
	OK       bool          `json:"ok"`
	Label    string        `json:"label,omitempty"`
	Elapsed  time.Duration `json:"-"`
	ExitCode int           `json:"exitCode"`
}
