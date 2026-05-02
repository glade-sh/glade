package watch

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventJSONShapes(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	started := NewWatchStartedEvent(now, Config{
		Root:     "/tmp/project",
		Debounce: 350 * time.Millisecond,
	})

	data, err := json.Marshal(started)
	if err != nil {
		t.Fatal(err)
	}
	var startedJSON map[string]any
	if err := json.Unmarshal(data, &startedJSON); err != nil {
		t.Fatal(err)
	}
	if startedJSON["event"] != string(EventWatchStarted) {
		t.Fatalf("started event JSON = %s", string(data))
	}
	config := startedJSON["config"].(map[string]any)
	if config["debounceMillis"] != float64(350) {
		t.Fatalf("started config JSON = %s", string(data))
	}

	debounced := NewDebouncedEvent(now, Config{}, []Change{{
		Path: "/tmp/project/InvoiceServiceTest.cls",
		Op:   ChangeModified,
		Kind: FileKindApexClass,
		Name: "InvoiceServiceTest",
	}})
	data, err = json.Marshal(debounced)
	if err != nil {
		t.Fatal(err)
	}
	var debouncedJSON map[string]any
	if err := json.Unmarshal(data, &debouncedJSON); err != nil {
		t.Fatal(err)
	}
	if _, ok := debouncedJSON["Delay"]; ok {
		t.Fatalf("duration leaked into JSON: %s", string(data))
	}
	if debouncedJSON["delayMs"] != float64(DefaultDebounce.Milliseconds()) {
		t.Fatalf("debounced JSON = %s", string(data))
	}
}

func TestTestsSelectedEventJSON(t *testing.T) {
	event := NewTestsSelectedEvent(time.Unix(0, 0).UTC(), TestSelection{
		Mode:        SelectionDirect,
		TestClasses: []string{"InvoiceServiceTest"},
		Reason:      "changed test class matched directly",
	})

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Event     EventType `json:"event"`
		Selection struct {
			Mode        SelectionMode `json:"mode"`
			TestClasses []string      `json:"testClasses"`
		} `json:"selection"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Event != EventTestsSelected || decoded.Selection.Mode != SelectionDirect || decoded.Selection.TestClasses[0] != "InvoiceServiceTest" {
		t.Fatalf("decoded event = %#v from %s", decoded, string(data))
	}
}
