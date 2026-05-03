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
	if startedJSON["schemaVersion"] != float64(WatchEventSchemaVersion) {
		t.Fatalf("started event schema = %s", string(data))
	}
	config := startedJSON["config"].(map[string]any)
	if config["debounceMillis"] != float64(350) {
		t.Fatalf("started config JSON = %s", string(data))
	}
	if config["backend"] != string(BackendAuto) {
		t.Fatalf("started config backend = %s", string(data))
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
	if debouncedJSON["schemaVersion"] != float64(WatchEventSchemaVersion) {
		t.Fatalf("debounced schema = %s", string(data))
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
		SchemaVersion int       `json:"schemaVersion"`
		Event         EventType `json:"event"`
		Selection     struct {
			Mode        SelectionMode `json:"mode"`
			TestClasses []string      `json:"testClasses"`
		} `json:"selection"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != WatchEventSchemaVersion || decoded.Event != EventTestsSelected || decoded.Selection.Mode != SelectionDirect || decoded.Selection.TestClasses[0] != "InvoiceServiceTest" {
		t.Fatalf("decoded event = %#v from %s", decoded, string(data))
	}
}

func TestRunEventJSONShapesAreStable(t *testing.T) {
	now := time.Unix(0, 0).UTC()
	started, err := json.Marshal(NewRunStartedEvent(now, 7, nil))
	if err != nil {
		t.Fatal(err)
	}
	wantStarted := `{"schemaVersion":1,"event":"watch.run_started","time":"1970-01-01T00:00:00Z","runId":7,"testClasses":[]}`
	if string(started) != wantStarted {
		t.Fatalf("started JSON = %s, want %s", string(started), wantStarted)
	}

	finished, err := json.Marshal(NewRunFinishedEvent(now, 7, RunSummary{Total: 2, Passed: 1, Failed: 1}))
	if err != nil {
		t.Fatal(err)
	}
	wantFinished := `{"schemaVersion":1,"event":"watch.run_finished","time":"1970-01-01T00:00:00Z","runId":7,"summary":{"total":2,"passed":1,"failed":1,"compileErrors":0,"unsupported":0,"passedAll":false}}`
	if string(finished) != wantFinished {
		t.Fatalf("finished JSON = %s, want %s", string(finished), wantFinished)
	}
}

func TestChangesAndErrorEventJSONShapesAreStable(t *testing.T) {
	now := time.Unix(0, 0).UTC()
	change := Change{
		Path: "/tmp/project/force-app/main/classes/InvoiceService.cls",
		Op:   ChangeModified,
		Kind: FileKindApexClass,
		Name: "InvoiceService",
		After: &FileMetadata{
			ModTimeUnixNano: 123,
			Size:            456,
		},
	}
	changes, err := json.Marshal(NewChangesEvent(now, []Change{change}))
	if err != nil {
		t.Fatal(err)
	}
	wantChanges := `{"schemaVersion":1,"event":"watch.changes","time":"1970-01-01T00:00:00Z","changes":[{"path":"/tmp/project/force-app/main/classes/InvoiceService.cls","op":"modified","kind":"apex_class","name":"InvoiceService","after":{"modTimeUnixNano":123,"size":456}}]}`
	if string(changes) != wantChanges {
		t.Fatalf("changes JSON = %s, want %s", string(changes), wantChanges)
	}

	errorEvent, err := json.Marshal(NewErrorEvent(now, "index failed", "/tmp/project"))
	if err != nil {
		t.Fatal(err)
	}
	wantError := `{"schemaVersion":1,"event":"watch.error","time":"1970-01-01T00:00:00Z","message":"index failed","path":"/tmp/project"}`
	if string(errorEvent) != wantError {
		t.Fatalf("error JSON = %s, want %s", string(errorEvent), wantError)
	}
}
