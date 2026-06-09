package trace

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteJSONChromeTraceDocument(t *testing.T) {
	doc := NewDocument([]Event{
		Instant("apex.statement.expr", "apex.statement", 7, map[string]any{
			"op":           "expr",
			"sourceOffset": 12,
		}),
	})

	var out bytes.Buffer
	if err := WriteJSON(&out, doc); err != nil {
		t.Fatal(err)
	}

	var decoded Document
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Format != FormatChromeTraceEvent || decoded.Version != Version {
		t.Fatalf("document header = %#v", decoded)
	}
	if len(decoded.TraceEvents) != 1 {
		t.Fatalf("events = %d", len(decoded.TraceEvents))
	}
	event := decoded.TraceEvents[0]
	if event.Phase != PhaseInstant || event.ProcessID != 1 || event.ThreadID != 1 {
		t.Fatalf("event = %#v", event)
	}
}

func TestDurationEventUsesChromeCompletePhase(t *testing.T) {
	event := Duration("apex.method.AccountService.save", "apex.method", 1000, 250, map[string]any{"line": 7})
	if event.Phase != PhaseComplete || event.Duration != 250 {
		t.Fatalf("event = %#v", event)
	}
	if event.Timestamp != 1000 || event.Category != "apex.method" || event.Args["line"] != 7 {
		t.Fatalf("event metadata = %#v", event)
	}
}
