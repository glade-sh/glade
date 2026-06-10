package testdaemon

import (
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

const (
	OpPing         = "ping"
	OpPong         = "pong"
	OpRun          = "run"
	OpRunResult    = "run_result"
	OpShutdown     = "shutdown"
	OpShutdownAck  = "shutdown_ack"
	OpError        = "error"
)

type Request struct {
	Op           string `json:"op"`
	ID           string `json:"id,omitempty"`
	Filter       string `json:"filter,omitempty"`
	ChangedSince string `json:"changedSince,omitempty"`
}

type Response struct {
	Op        string                `json:"op"`
	ID        string                `json:"id,omitempty"`
	OK        bool                  `json:"ok"`
	Error     string                `json:"error,omitempty"`
	Ready     bool                  `json:"ready,omitempty"`
	Warming   bool                  `json:"warming,omitempty"`
	Project   string                `json:"project,omitempty"`
	Run       *testreport.Run       `json:"run,omitempty"`
	Selection *watch.TestSelection  `json:"selection,omitempty"`
}
