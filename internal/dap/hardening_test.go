package dap

import (
	"encoding/json"
	"testing"
)

func TestNoPanicOnMalformedDAPRequests(t *testing.T) {
	handler := NewHandler(Snapshot{})
	requests := []Request{
		{Seq: 1, Type: MessageTypeRequest, Command: CommandSetBreakpoints, Arguments: json.RawMessage(`{`)},
		{Seq: 2, Type: MessageTypeRequest, Command: CommandStackTrace, Arguments: json.RawMessage(`{"startFrame":-10,"levels":1}`)},
		{Seq: 3, Type: MessageTypeRequest, Command: CommandVariables, Arguments: json.RawMessage(`{"variablesReference":999}`)},
		{Seq: 4, Type: MessageTypeRequest, Command: CommandEvaluate, Arguments: json.RawMessage(`{"expression":"missing"}`)},
		{Seq: 5, Type: MessageTypeRequest, Command: "unsupported", Arguments: json.RawMessage(`null`)},
	}
	for _, request := range requests {
		assertNoPanic(t, func() {
			_ = handler.Handle(request)
		})
	}
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
