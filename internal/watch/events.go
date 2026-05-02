package watch

import "time"

type EventType string

const (
	EventWatchStarted   EventType = "watch.started"
	EventChanges        EventType = "watch.changes"
	EventTestsSelected  EventType = "watch.tests_selected"
	EventRunStarted     EventType = "watch.run_started"
	EventRunFinished    EventType = "watch.run_finished"
	EventWatchError     EventType = "watch.error"
	EventWatchDebounced EventType = "watch.debounced"
)

type WatchStartedEvent struct {
	Event  EventType      `json:"event"`
	Time   time.Time      `json:"time"`
	Config ConfigSnapshot `json:"config"`
}

type ChangesEvent struct {
	Event   EventType `json:"event"`
	Time    time.Time `json:"time"`
	Changes []Change  `json:"changes"`
}

type DebouncedEvent struct {
	Event   EventType     `json:"event"`
	Time    time.Time     `json:"time"`
	Delay   time.Duration `json:"-"`
	DelayMS int64         `json:"delayMs"`
	Changes []Change      `json:"changes"`
}

type TestsSelectedEvent struct {
	Event     EventType     `json:"event"`
	Time      time.Time     `json:"time"`
	Selection TestSelection `json:"selection"`
}

type RunStartedEvent struct {
	Event       EventType `json:"event"`
	Time        time.Time `json:"time"`
	TestClasses []string  `json:"testClasses,omitempty"`
}

type RunFinishedEvent struct {
	Event   EventType  `json:"event"`
	Time    time.Time  `json:"time"`
	Summary RunSummary `json:"summary"`
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
	Event   EventType `json:"event"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
	Path    string    `json:"path,omitempty"`
}

func NewWatchStartedEvent(now time.Time, cfg Config) WatchStartedEvent {
	return WatchStartedEvent{
		Event:  EventWatchStarted,
		Time:   now,
		Config: cfg.Snapshot(),
	}
}

func NewChangesEvent(now time.Time, changes []Change) ChangesEvent {
	return ChangesEvent{
		Event:   EventChanges,
		Time:    now,
		Changes: changes,
	}
}

func NewDebouncedEvent(now time.Time, cfg Config, changes []Change) DebouncedEvent {
	delay := cfg.Normalized().Debounce
	return DebouncedEvent{
		Event:   EventWatchDebounced,
		Time:    now,
		Delay:   delay,
		DelayMS: delay.Milliseconds(),
		Changes: changes,
	}
}

func NewTestsSelectedEvent(now time.Time, selection TestSelection) TestsSelectedEvent {
	return TestsSelectedEvent{
		Event:     EventTestsSelected,
		Time:      now,
		Selection: selection,
	}
}

func NewErrorEvent(now time.Time, message, path string) ErrorEvent {
	return ErrorEvent{
		Event:   EventWatchError,
		Time:    now,
		Message: message,
		Path:    path,
	}
}
