package watch

import "time"

type EventType string

const (
	WatchEventSchemaVersion = 1

	EventWatchStarted   EventType = "watch.started"
	EventChanges        EventType = "watch.changes"
	EventTestsSelected  EventType = "watch.tests_selected"
	EventRunStarted     EventType = "watch.run_started"
	EventRunFinished    EventType = "watch.run_finished"
	EventWatchError     EventType = "watch.error"
	EventWatchDebounced EventType = "watch.debounced"
)

type WatchStartedEvent struct {
	SchemaVersion int            `json:"schemaVersion"`
	Event         EventType      `json:"event"`
	Time          time.Time      `json:"time"`
	Config        ConfigSnapshot `json:"config"`
}

type ChangesEvent struct {
	SchemaVersion int       `json:"schemaVersion"`
	Event         EventType `json:"event"`
	Time          time.Time `json:"time"`
	Changes       []Change  `json:"changes"`
}

type DebouncedEvent struct {
	SchemaVersion int           `json:"schemaVersion"`
	Event         EventType     `json:"event"`
	Time          time.Time     `json:"time"`
	Delay         time.Duration `json:"-"`
	DelayMS       int64         `json:"delayMs"`
	Changes       []Change      `json:"changes"`
}

type TestsSelectedEvent struct {
	SchemaVersion int           `json:"schemaVersion"`
	Event         EventType     `json:"event"`
	Time          time.Time     `json:"time"`
	Selection     TestSelection `json:"selection"`
}

type RunStartedEvent struct {
	SchemaVersion int       `json:"schemaVersion"`
	Event         EventType `json:"event"`
	Time          time.Time `json:"time"`
	RunID         int       `json:"runId"`
	TestClasses   []string  `json:"testClasses"`
}

type RunFinishedEvent struct {
	SchemaVersion int        `json:"schemaVersion"`
	Event         EventType  `json:"event"`
	Time          time.Time  `json:"time"`
	RunID         int        `json:"runId"`
	Summary       RunSummary `json:"summary"`
}

type RunSummary struct {
	Total         int  `json:"total"`
	Passed        int  `json:"passed"`
	Failed        int  `json:"failed"`
	CompileErrors int  `json:"compileErrors"`
	Unsupported   int  `json:"unsupported"`
	PassedAll     bool `json:"passedAll"`
}

type ErrorEvent struct {
	SchemaVersion int       `json:"schemaVersion"`
	Event         EventType `json:"event"`
	Time          time.Time `json:"time"`
	Message       string    `json:"message"`
	Path          string    `json:"path,omitempty"`
}

func NewWatchStartedEvent(now time.Time, cfg Config) WatchStartedEvent {
	return WatchStartedEvent{
		SchemaVersion: WatchEventSchemaVersion,
		Event:         EventWatchStarted,
		Time:          now,
		Config:        cfg.Snapshot(),
	}
}

func NewChangesEvent(now time.Time, changes []Change) ChangesEvent {
	return ChangesEvent{
		SchemaVersion: WatchEventSchemaVersion,
		Event:         EventChanges,
		Time:          now,
		Changes:       changes,
	}
}

func NewDebouncedEvent(now time.Time, cfg Config, changes []Change) DebouncedEvent {
	delay := cfg.Normalized().Debounce
	return DebouncedEvent{
		SchemaVersion: WatchEventSchemaVersion,
		Event:         EventWatchDebounced,
		Time:          now,
		Delay:         delay,
		DelayMS:       delay.Milliseconds(),
		Changes:       changes,
	}
}

func NewTestsSelectedEvent(now time.Time, selection TestSelection) TestsSelectedEvent {
	return TestsSelectedEvent{
		SchemaVersion: WatchEventSchemaVersion,
		Event:         EventTestsSelected,
		Time:          now,
		Selection:     selection,
	}
}

func NewRunStartedEvent(now time.Time, runID int, testClasses []string) RunStartedEvent {
	classes := make([]string, len(testClasses))
	copy(classes, testClasses)
	return RunStartedEvent{
		SchemaVersion: WatchEventSchemaVersion,
		Event:         EventRunStarted,
		Time:          now,
		RunID:         runID,
		TestClasses:   classes,
	}
}

func NewRunFinishedEvent(now time.Time, runID int, summary RunSummary) RunFinishedEvent {
	return RunFinishedEvent{
		SchemaVersion: WatchEventSchemaVersion,
		Event:         EventRunFinished,
		Time:          now,
		RunID:         runID,
		Summary:       summary,
	}
}

func NewErrorEvent(now time.Time, message, path string) ErrorEvent {
	return ErrorEvent{
		SchemaVersion: WatchEventSchemaVersion,
		Event:         EventWatchError,
		Time:          now,
		Message:       message,
		Path:          path,
	}
}
