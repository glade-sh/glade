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

func TestStableOperationCorrelationArgs(t *testing.T) {
	firstID := StableOperationID("force-app/main/default/classes/Slow.cls", 7, "soql", "SELECT Id\nFROM Account")
	secondID := StableOperationID("force-app/main/default/classes/Slow.cls", 7, "soql", " SELECT Id FROM   Account ")
	otherID := StableOperationID("force-app/main/default/classes/Slow.cls", 8, "soql", "SELECT Id FROM Account")
	if firstID == "" || firstID != secondID || firstID == otherID {
		t.Fatalf("operation ids: first=%q second=%q other=%q", firstID, secondID, otherID)
	}

	firstHash := StableQueryHash("SELECT Id\nFROM Account")
	secondHash := StableQueryHash(" SELECT Id FROM   Account ")
	if firstHash == "" || firstHash != secondHash {
		t.Fatalf("query hashes: first=%q second=%q", firstHash, secondHash)
	}
	literalHash := StableQueryHash("SELECT Id FROM Account WHERE Name = 'A  B'")
	collapsedLiteralHash := StableQueryHash("SELECT Id FROM Account WHERE Name = 'A B'")
	spacedLiteralHash := StableQueryHash(" SELECT   Id FROM Account WHERE Name = 'A  B' ")
	if literalHash != spacedLiteralHash {
		t.Fatalf("query hash changed for outside whitespace: first=%q second=%q", literalHash, spacedLiteralHash)
	}
	if literalHash == collapsedLiteralHash {
		t.Fatalf("query hash collapsed string literal whitespace: %q", literalHash)
	}
	if len(firstHash) != 64 {
		t.Fatalf("query hash length = %d, want sha256 hex", len(firstHash))
	}
}

func TestSourceArgsUsesStableArgNames(t *testing.T) {
	args := SourceArgs("classes/Slow.cls", 7, 11)
	for _, key := range []string{ArgFile, ArgLine, ArgColumn} {
		if _, ok := args[key]; !ok {
			t.Fatalf("missing %s in %#v", key, args)
		}
	}
	if args[ArgFile] != "classes/Slow.cls" || args[ArgLine] != 7 || args[ArgColumn] != 11 {
		t.Fatalf("source args = %#v", args)
	}
}

func TestWriteJSONPreservesStableArgsInsideChromeTraceEvent(t *testing.T) {
	operationID := StableOperationID("classes/Slow.cls", 7, "soql", "SELECT Id FROM Account")
	doc := NewDocument([]Event{
		Duration("apex.soql", "apex.soql", 100, 25, map[string]any{
			ArgOperationID: operationID,
			ArgQueryHash:   StableQueryHash("SELECT Id FROM Account"),
			ArgObject:      "Account",
			ArgRows:        3,
			ArgFile:        "classes/Slow.cls",
			ArgLine:        7,
			ArgColumn:      11,
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
	if decoded.Format != FormatChromeTraceEvent || len(decoded.TraceEvents) != 1 {
		t.Fatalf("document = %#v", decoded)
	}
	args := decoded.TraceEvents[0].Args
	if args[ArgOperationID] != operationID || args[ArgObject] != "Account" {
		t.Fatalf("stable args = %#v", args)
	}
}
