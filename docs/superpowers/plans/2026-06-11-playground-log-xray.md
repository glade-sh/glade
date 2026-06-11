# Playground Log X-Ray Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a public playground Log X-Ray feature that accepts a Salesforce debug log, parses it server-side without storage, and returns a timeline, limit heat map, failure story, and local `glade debug` install path.

**Architecture:** Keep parsing in base Glade because `internal/apexlog`, `internal/profile`, and `internal/debuglog` are already product-owned. Add a transient `/playground/api/log-xray` endpoint that reads a bounded request body, parses in memory, returns shaped JSON, and writes no files. Add a visible playground tab that lets a visitor paste or drop a log, then renders the X-Ray result and points source-aware workflows to local install commands.

**Tech Stack:** Go standard-library HTTP/JSON tests, `internal/apexlog`, `internal/profile`, React 19, TypeScript, Vitest, Vite, existing playground CSS and lucide icons.

---

## Boundaries

Build this in `/Users/matt/Dev/glade`.

Keep in scope:

- Hosted playground route: `POST /playground/api/log-xray`.
- Server-side transient parsing. No disk write. No database write. No cache write.
- Public rate limit integration.
- Request size cap for logs.
- Timeline, limit heat map, failure story, debug excerpts, and install command suggestions.
- Playground UI tab and docs.

Keep out of scope:

- Uploading project source.
- Hosted `debug explain` source matching.
- Hosted `debug repro` synthesis.
- Storing log history.
- First-party plugin work in `glade-tools`.
- Browser WASM parser.

The install payoff is plain: hosted X-Ray shows the diagnosis; local `glade debug explain` and `glade debug repro` add project context.

## File Map

- Create `internal/playground/log_xray.go`: request/response models, in-memory analyzer, endpoint handler.
- Modify `internal/playground/server.go`: route `/playground/api/log-xray`, add size-aware JSON decoder, rate-limit the endpoint.
- Modify `internal/playground/types.go`: only if shared exported types become clearer there. Prefer keeping X-Ray types in `log_xray.go`.
- Modify `internal/playground/server_test.go`: endpoint and no-workspace-write tests.
- Create `internal/playground/log_xray_test.go`: analyzer unit tests.
- Modify `internal/playground/web/src/App.tsx`: add Log X-Ray tab, file/drop/paste controls, API call, result rendering.
- Modify `internal/playground/web/src/App.test.tsx`: render coverage for the new tab and advanced database tab shape.
- Modify `internal/playground/web/src/index.css`: X-Ray drop zone, timeline, limit bars, and result cards.
- Regenerate `internal/playground/static/index.html` and `internal/playground/static/assets/*` with the web build.
- Modify `site/docs-src/guide/playground.md`: document Log X-Ray and no-storage behavior.
- Modify `docs/EDITOR.md`: add Log X-Ray as hosted preview and local `glade debug` bridge.

## Data Contract

Use one request shape:

```json
{
  "content": "64.0 APEX_CODE,FINEST;...\n00:00:00.001 (1000000)|EXECUTION_STARTED\n"
}
```

Use one response shape:

```json
{
  "privacy": {
    "stored": false,
    "retention": "none",
    "serverWrites": 0
  },
  "summary": {
    "apiVersion": "64.0",
    "entryCount": 11,
    "timelineCount": 5,
    "durationMs": 10,
    "soqlQueries": 1,
    "soqlRows": 1,
    "dmlStatements": 1,
    "dmlRows": 1,
    "callouts": 0,
    "cpuTimeMs": 7,
    "heapBytes": 8120,
    "userDebugs": 2,
    "exceptions": 0,
    "hottestLimit": {
      "name": "Maximum CPU time",
      "used": 7,
      "limit": 10000,
      "percent": 0.1,
      "severity": "ok"
    }
  },
  "timeline": [],
  "limits": [],
  "failure": null,
  "debugLines": [],
  "install": [
    {
      "label": "Profile this log locally",
      "command": "glade debug profile --log your.log",
      "why": "Get the same measured runtime summary from a file on disk."
    }
  ]
}
```

---

### Task 1: Backend X-Ray Analyzer

**Files:**

- Create: `internal/playground/log_xray.go`
- Create: `internal/playground/log_xray_test.go`

- [ ] **Step 1: Write the analyzer tests**

Create `internal/playground/log_xray_test.go`:

```go
package playground

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeDebugLogTextProducesXRay(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "apexlog", "testdata", "core.log"))
	if err != nil {
		t.Fatal(err)
	}

	result, err := AnalyzeDebugLogText(string(data))
	if err != nil {
		t.Fatal(err)
	}

	if result.Privacy.Stored || result.Privacy.ServerWrites != 0 || result.Privacy.Retention != "none" {
		t.Fatalf("privacy = %#v", result.Privacy)
	}
	if result.Summary.APIVersion != "64.0" {
		t.Fatalf("api version = %q", result.Summary.APIVersion)
	}
	if result.Summary.EntryCount == 0 || result.Summary.TimelineCount == 0 {
		t.Fatalf("summary counts = %#v", result.Summary)
	}
	if result.Summary.SOQLQueries != 1 || result.Summary.SOQLRows != 1 {
		t.Fatalf("soql summary = %#v", result.Summary)
	}
	if result.Summary.DMLStatements != 1 || result.Summary.DMLRows != 1 {
		t.Fatalf("dml summary = %#v", result.Summary)
	}
	if result.Summary.CPUTimeMS != 7 || result.Summary.HeapBytes != 8120 {
		t.Fatalf("resource summary = %#v", result.Summary)
	}
	if len(result.Limits) == 0 || result.Summary.HottestLimit == nil {
		t.Fatalf("limits missing: %#v", result)
	}
	if !containsTimelineKind(result.Timeline, "SOQL") || !containsTimelineKind(result.Timeline, "DML") || !containsTimelineKind(result.Timeline, "USER_DEBUG") {
		t.Fatalf("timeline = %#v", result.Timeline)
	}
	if len(result.DebugLines) != 2 {
		t.Fatalf("debug lines = %#v", result.DebugLines)
	}
	if !containsInstallCommand(result.Install, "glade debug explain --log your.log --project .") {
		t.Fatalf("install commands = %#v", result.Install)
	}
}

func TestAnalyzeDebugLogTextFailureStory(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "apexlog", "testdata", "exception.log"))
	if err != nil {
		t.Fatal(err)
	}

	result, err := AnalyzeDebugLogText(string(data))
	if err != nil {
		t.Fatal(err)
	}

	if result.Failure == nil {
		t.Fatalf("failure missing: %#v", result)
	}
	if result.Summary.Exceptions == 0 {
		t.Fatalf("exception count missing: %#v", result.Summary)
	}
	if result.Failure.Type != "System.AuraHandledException" {
		t.Fatalf("failure type = %q", result.Failure.Type)
	}
	if len(result.Failure.StackFrames) != 2 {
		t.Fatalf("stack frames = %#v", result.Failure.StackFrames)
	}
}

func TestAnalyzeDebugLogTextRejectsEmptyAndOversizedLogs(t *testing.T) {
	if _, err := AnalyzeDebugLogText(" \n\t "); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty err = %v", err)
	}

	tooLarge := strings.Repeat("x", maxLogXRayContentBytes+1)
	if _, err := AnalyzeDebugLogText(tooLarge); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("oversized err = %v", err)
	}
}

func containsTimelineKind(events []LogXRayTimelineEvent, kind string) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func containsInstallCommand(actions []LogXRayInstallAction, command string) bool {
	for _, action := range actions {
		if action.Command == command {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run the tests and verify they fail**

Run:

```bash
go test ./internal/playground -run 'TestAnalyzeDebugLogText' -count=1
```

Expected:

```text
undefined: AnalyzeDebugLogText
```

- [ ] **Step 3: Add the analyzer implementation**

Create `internal/playground/log_xray.go`:

```go
package playground

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/profile"
)

const (
	maxLogXRayContentBytes = 5 * 1024 * 1024
	maxLogXRayBodyBytes    = maxLogXRayContentBytes + 64*1024
	maxLogXRayTimeline     = 120
	maxLogXRayDebugLines   = 12
)

type LogXRayRequest struct {
	Content string `json:"content"`
}

type LogXRayResponse struct {
	Privacy    LogXRayPrivacy         `json:"privacy"`
	Summary    LogXRaySummary         `json:"summary"`
	Timeline   []LogXRayTimelineEvent `json:"timeline"`
	Limits     []LogXRayLimit         `json:"limits,omitempty"`
	Failure    *LogXRayFailure        `json:"failure,omitempty"`
	DebugLines []LogXRayDebugLine     `json:"debugLines,omitempty"`
	Install    []LogXRayInstallAction `json:"install"`
}

type LogXRayPrivacy struct {
	Stored       bool   `json:"stored"`
	Retention    string `json:"retention"`
	ServerWrites int    `json:"serverWrites"`
}

type LogXRaySummary struct {
	APIVersion    string        `json:"apiVersion,omitempty"`
	EntryCount    int           `json:"entryCount"`
	TimelineCount int           `json:"timelineCount"`
	DurationMS    int64         `json:"durationMs,omitempty"`
	SOQLQueries   int           `json:"soqlQueries"`
	SOQLRows      int           `json:"soqlRows"`
	DMLStatements int           `json:"dmlStatements"`
	DMLRows       int           `json:"dmlRows"`
	Callouts      int           `json:"callouts"`
	CPUTimeMS     int           `json:"cpuTimeMs,omitempty"`
	HeapBytes     int           `json:"heapBytes,omitempty"`
	UserDebugs    int           `json:"userDebugs"`
	Exceptions    int           `json:"exceptions"`
	HottestLimit  *LogXRayLimit `json:"hottestLimit,omitempty"`
}

type LogXRayTimelineEvent struct {
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	Detail     string `json:"detail,omitempty"`
	Line       int    `json:"line,omitempty"`
	SourceLine int    `json:"sourceLine,omitempty"`
	OffsetMS   int64  `json:"offsetMs,omitempty"`
	Severity   string `json:"severity"`
}

type LogXRayLimit struct {
	Namespace string  `json:"namespace,omitempty"`
	Name      string  `json:"name"`
	Used      int     `json:"used"`
	Limit     int     `json:"limit"`
	Percent   float64 `json:"percent,omitempty"`
	Severity  string  `json:"severity"`
}

type LogXRayFailure struct {
	Kind       string               `json:"kind"`
	Type       string               `json:"type,omitempty"`
	Message    string               `json:"message,omitempty"`
	Line       int                  `json:"line,omitempty"`
	SourceLine int                  `json:"sourceLine,omitempty"`
	StackFrames []apexlog.StackFrame `json:"stackFrames,omitempty"`
	Story      []LogXRayTimelineEvent `json:"story,omitempty"`
}

type LogXRayDebugLine struct {
	Line       int    `json:"line"`
	SourceLine int    `json:"sourceLine,omitempty"`
	Level      string `json:"level,omitempty"`
	Message    string `json:"message"`
	OffsetMS   int64  `json:"offsetMs,omitempty"`
}

type LogXRayInstallAction struct {
	Label   string `json:"label"`
	Command string `json:"command"`
	Why     string `json:"why"`
}

func (s *Server) handleLogXRay(w http.ResponseWriter, r *http.Request) {
	var req LogXRayRequest
	if err := decodeJSONMax(r, &req, maxLogXRayBodyBytes); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := AnalyzeDebugLogText(req.Content)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "too large") {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func AnalyzeDebugLogText(raw string) (LogXRayResponse, error) {
	if strings.TrimSpace(raw) == "" {
		return LogXRayResponse{}, errors.New("debug log is empty")
	}
	if len(raw) > maxLogXRayContentBytes {
		return LogXRayResponse{}, fmt.Errorf("debug log is too large: max %d bytes", maxLogXRayContentBytes)
	}

	log, err := apexlog.Parse(strings.NewReader(raw))
	if err != nil {
		return LogXRayResponse{}, err
	}
	if len(log.Entries) == 0 {
		return LogXRayResponse{}, errors.New("no Salesforce debug log events found")
	}

	report := profile.Analyze(apexlog.TraceDocument(log))
	limits := buildLogXRayLimits(log)
	timeline := buildLogXRayTimeline(log)
	debugLines := buildLogXRayDebugLines(log, maxLogXRayDebugLines)

	return LogXRayResponse{
		Privacy: LogXRayPrivacy{
			Stored:       false,
			Retention:    "none",
			ServerWrites: 0,
		},
		Summary:    buildLogXRaySummary(log, report, limits, timeline),
		Timeline:   timeline,
		Limits:     limits,
		Failure:    buildLogXRayFailure(log, timeline),
		DebugLines: debugLines,
		Install:    defaultLogXRayInstallActions(),
	}, nil
}

func buildLogXRaySummary(log apexlog.Log, report profile.Report, limits []LogXRayLimit, timeline []LogXRayTimelineEvent) LogXRaySummary {
	summary := LogXRaySummary{
		APIVersion:    log.APIVersion,
		EntryCount:    len(log.Entries),
		TimelineCount: len(timeline),
		DurationMS:    logDurationMS(log),
		SOQLQueries:   report.Limits.SOQLQueries,
		SOQLRows:      report.Limits.SOQLRows,
		DMLStatements: report.Limits.DML,
		DMLRows:       report.Limits.DMLRows,
		Callouts:      report.Limits.Callouts,
		CPUTimeMS:     report.Limits.CPUTimeMS,
		HeapBytes:     report.Limits.HeapSize,
	}
	for _, entry := range log.Entries {
		switch entry.Kind {
		case apexlog.EntryUserDebug:
			summary.UserDebugs++
		case apexlog.EntryExceptionThrown, apexlog.EntryFatalError:
			summary.Exceptions++
		}
	}
	if len(limits) > 0 {
		limit := limits[0]
		summary.HottestLimit = &limit
	}
	return summary
}

func buildLogXRayLimits(log apexlog.Log) []LogXRayLimit {
	out := make([]LogXRayLimit, 0, len(log.Limits))
	for _, limit := range log.Limits {
		percent := 0.0
		if limit.Limit > 0 {
			percent = math.Round((float64(limit.Used)/float64(limit.Limit))*1000) / 10
		}
		out = append(out, LogXRayLimit{
			Namespace: limit.Namespace,
			Name:      limit.Name,
			Used:      limit.Used,
			Limit:     limit.Limit,
			Percent:   percent,
			Severity:  logXRayLimitSeverity(percent),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Percent != out[j].Percent {
			return out[i].Percent > out[j].Percent
		}
		if out[i].Used != out[j].Used {
			return out[i].Used > out[j].Used
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func logXRayLimitSeverity(percent float64) string {
	switch {
	case percent >= 80:
		return "critical"
	case percent >= 60:
		return "warning"
	default:
		return "ok"
	}
}

func buildLogXRayTimeline(log apexlog.Log) []LogXRayTimelineEvent {
	first, hasFirst := firstLogNanos(log)
	out := make([]LogXRayTimelineEvent, 0, minInt(len(log.Entries), maxLogXRayTimeline))
	for _, entry := range log.Entries {
		event, ok := logXRayTimelineEvent(entry, first, hasFirst)
		if !ok {
			continue
		}
		out = append(out, event)
		if len(out) >= maxLogXRayTimeline {
			break
		}
	}
	return out
}

func logXRayTimelineEvent(entry apexlog.Entry, first int64, hasFirst bool) (LogXRayTimelineEvent, bool) {
	event := LogXRayTimelineEvent{
		Line:       entry.Line,
		SourceLine: entry.Data.SourceLine,
		OffsetMS:   offsetMS(entry, first, hasFirst),
		Severity:   "info",
	}
	switch entry.Kind {
	case apexlog.EntryCodeUnitStarted:
		event.Kind = "CODE_UNIT"
		event.Label = "Code unit"
		event.Detail = trimLogXRayDetail(entry.Data.CodeUnit)
	case apexlog.EntryUserDebug:
		event.Kind = "USER_DEBUG"
		event.Label = "Debug"
		event.Detail = trimLogXRayDetail(entry.Data.DebugMessage)
		event.Severity = "debug"
	case apexlog.EntrySOQLExecuteBegin:
		event.Kind = "SOQL"
		event.Label = "SOQL"
		event.Detail = trimLogXRayDetail(entry.Data.SOQLQuery)
		event.Severity = "data"
	case apexlog.EntryDMLBegin:
		event.Kind = "DML"
		event.Label = strings.TrimSpace(strings.ToUpper(entry.Data.DMLOperation))
		if event.Label == "" {
			event.Label = "DML"
		}
		if entry.Data.DMLType != "" || entry.Data.DMLRows > 0 {
			event.Detail = strings.TrimSpace(fmt.Sprintf("%s %d rows", entry.Data.DMLType, entry.Data.DMLRows))
		}
		event.Severity = "mutation"
	case apexlog.EntryCalloutRequest:
		event.Kind = "CALLOUT"
		event.Label = "Callout"
		event.Detail = trimLogXRayDetail(entry.Data.CalloutEndpoint)
		event.Severity = "callout"
	case apexlog.EntryExceptionThrown, apexlog.EntryFatalError:
		event.Kind = "EXCEPTION"
		event.Label = string(entry.Kind)
		event.Detail = trimLogXRayDetail(strings.TrimSpace(entry.Data.ExceptionType + ": " + entry.Data.ExceptionText))
		event.Severity = "danger"
	default:
		return LogXRayTimelineEvent{}, false
	}
	return event, true
}

func buildLogXRayFailure(log apexlog.Log, timeline []LogXRayTimelineEvent) *LogXRayFailure {
	for i := len(log.Entries) - 1; i >= 0; i-- {
		entry := log.Entries[i]
		if entry.Kind != apexlog.EntryExceptionThrown && entry.Kind != apexlog.EntryFatalError {
			continue
		}
		failure := &LogXRayFailure{
			Kind:        string(entry.Kind),
			Type:        entry.Data.ExceptionType,
			Message:     entry.Data.ExceptionText,
			Line:        entry.Line,
			SourceLine:  entry.Data.SourceLine,
			StackFrames: entry.Data.StackFrames,
			Story:       precedingLogXRayEvents(timeline, entry.Line, 5),
		}
		return failure
	}
	return nil
}

func buildLogXRayDebugLines(log apexlog.Log, maxLines int) []LogXRayDebugLine {
	first, hasFirst := firstLogNanos(log)
	out := make([]LogXRayDebugLine, 0, maxLines)
	for _, entry := range log.Entries {
		if entry.Kind != apexlog.EntryUserDebug {
			continue
		}
		out = append(out, LogXRayDebugLine{
			Line:       entry.Line,
			SourceLine: entry.Data.SourceLine,
			Level:      entry.Data.DebugLevel,
			Message:    entry.Data.DebugMessage,
			OffsetMS:   offsetMS(entry, first, hasFirst),
		})
		if len(out) >= maxLines {
			break
		}
	}
	return out
}

func precedingLogXRayEvents(timeline []LogXRayTimelineEvent, line int, maxEvents int) []LogXRayTimelineEvent {
	out := make([]LogXRayTimelineEvent, 0, maxEvents)
	for i := len(timeline) - 1; i >= 0; i-- {
		if timeline[i].Line > line {
			continue
		}
		out = append([]LogXRayTimelineEvent{timeline[i]}, out...)
		if len(out) >= maxEvents {
			return out
		}
	}
	return out
}

func defaultLogXRayInstallActions() []LogXRayInstallAction {
	return []LogXRayInstallAction{
		{
			Label:   "Profile this log locally",
			Command: "glade debug profile --log your.log",
			Why:     "Get the same measured runtime summary from a file on disk.",
		},
		{
			Label:   "Explain it against source",
			Command: "glade debug explain --log your.log --project .",
			Why:     "Match log evidence back to local Apex classes with conservative confidence.",
		},
		{
			Label:   "Draft a repro test",
			Command: "glade debug repro --log your.log --project . > ReproTest.cls",
			Why:     "Turn log evidence into a local test starting point for review.",
		},
	}
}

func logDurationMS(log apexlog.Log) int64 {
	first, ok := firstLogNanos(log)
	if !ok {
		return 0
	}
	last := first
	for _, entry := range log.Entries {
		if ts, ok := logEntryNanos(entry); ok && ts > last {
			last = ts
		}
	}
	return (last - first) / int64(timeMillisecondNanos())
}

func firstLogNanos(log apexlog.Log) (int64, bool) {
	for _, entry := range log.Entries {
		if ts, ok := logEntryNanos(entry); ok {
			return ts, true
		}
	}
	return 0, false
}

func offsetMS(entry apexlog.Entry, first int64, hasFirst bool) int64 {
	if !hasFirst {
		return 0
	}
	ts, ok := logEntryNanos(entry)
	if !ok || ts < first {
		return 0
	}
	return (ts - first) / int64(timeMillisecondNanos())
}

func logEntryNanos(entry apexlog.Entry) (int64, bool) {
	open := strings.LastIndex(entry.Timestamp, "(")
	close := strings.LastIndex(entry.Timestamp, ")")
	if open < 0 || close <= open {
		return 0, false
	}
	value := strings.TrimSpace(entry.Timestamp[open+1 : close])
	ts, err := strconv.ParseInt(value, 10, 64)
	return ts, err == nil
}

func timeMillisecondNanos() int {
	return 1_000_000
}

func trimLogXRayDetail(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 180 {
		return value
	}
	return value[:177] + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run focused analyzer tests**

Run:

```bash
go test ./internal/playground -run 'TestAnalyzeDebugLogText' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/playground
```

- [ ] **Step 5: Commit backend analyzer**

```bash
git add internal/playground/log_xray.go internal/playground/log_xray_test.go
git commit -m "feat: add playground log xray analyzer"
```

---

### Task 2: Playground API Route

**Files:**

- Modify: `internal/playground/server.go`
- Modify: `internal/playground/server_test.go`

- [ ] **Step 1: Add failing route tests**

Append to `internal/playground/server_test.go`:

```go
func TestServerLogXRayRouteAnalyzesWithoutWorkspaceWrites(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test", Public: true})
	logData, err := os.ReadFile(filepath.Join("..", "apexlog", "testdata", "core.log"))
	if err != nil {
		t.Fatal(err)
	}
	before, err := ws.Metadata()
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(LogXRayRequest{Content: string(logData)})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/playground/api/log-xray", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5000"
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("log xray status = %d body=%s", rec.Code, rec.Body.String())
	}
	var result LogXRayResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode xray: %v", err)
	}
	if result.Privacy.Stored || result.Privacy.ServerWrites != 0 {
		t.Fatalf("privacy = %#v", result.Privacy)
	}

	after, err := ws.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Files) != len(before.Files) || after.WorkspaceHash != before.WorkspaceHash {
		t.Fatalf("workspace changed before=%#v after=%#v", before, after)
	}
}

func TestServerLogXRayRouteRejectsOversizedContent(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})

	body, _ := json.Marshal(LogXRayRequest{Content: strings.Repeat("x", maxLogXRayContentBytes+1)})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/playground/api/log-xray", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPublicRateLimiterAppliesToLogXRayRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/playground/api/log-xray", nil)
	if !isPublicLimitedEndpoint(req) {
		t.Fatalf("log xray route is not public-rate-limited")
	}
}
```

- [ ] **Step 2: Run route tests and verify they fail**

Run:

```bash
go test ./internal/playground -run 'TestServerLogXRay|TestPublicRateLimiterAppliesToLogXRayRoute' -count=1
```

Expected:

```text
log xray status = 404
```

- [ ] **Step 3: Wire the route and bounded decoder**

In `internal/playground/server.go`, add this route before `/playground/api/run` or near the other POST API routes:

```go
	case r.Method == http.MethodPost && r.URL.Path == "/playground/api/log-xray":
		s.handleLogXRay(w, r)
```

Change `isPublicLimitedEndpoint`:

```go
	case "/playground/api/run", "/playground/api/log-xray", "/playground/api/examples/load", "/playground/api/files", "/playground/api/reset", "/playground/api/seed":
		return r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete
```

Replace `decodeJSON` at the bottom of `internal/playground/server.go` with:

```go
func decodeJSON(r *http.Request, out any) error {
	return decodeJSONMax(r, out, 1<<20)
}

func decodeJSONMax(r *http.Request, out any, maxBytes int64) error {
	defer r.Body.Close()
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBytes)).Decode(out)
}
```

- [ ] **Step 4: Run focused route tests**

Run:

```bash
go test ./internal/playground -run 'TestServerLogXRay|TestPublicRateLimiterAppliesToLogXRayRoute' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/playground
```

- [ ] **Step 5: Commit API route**

```bash
git add internal/playground/server.go internal/playground/server_test.go
git commit -m "feat: expose transient log xray playground route"
```

---

### Task 3: Playground UI Tab

**Files:**

- Modify: `internal/playground/web/src/App.tsx`
- Modify: `internal/playground/web/src/App.test.tsx`
- Modify: `internal/playground/web/src/index.css`

- [ ] **Step 1: Add failing render tests**

Modify `internal/playground/web/src/App.test.tsx`:

```tsx
test("renders the Log X-Ray tab for public users", () => {
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => (key === "glade-playground-theme" ? "dark" : "false"),
    setItem: () => undefined,
  })

  const html = renderToString(<App />)

  expect(html).toContain("Log X-Ray")
  expect(html).toContain('data-testid="workspace-log-xray-tab"')
  expect(html).toContain("Drop debug log")
})
```

Update the existing non-advanced database test to expect the new public tab:

```tsx
expect(html).toContain("Log X-Ray")
expect(html).toContain('data-testid="workspace-log-xray-tab"')
expect(html).not.toContain('data-testid="workspace-database-tab"')
```

- [ ] **Step 2: Run UI tests and verify they fail**

Run:

```bash
npm --prefix internal/playground/web test -- App.test.tsx
```

Expected:

```text
expected rendered HTML to contain "Log X-Ray"
```

- [ ] **Step 3: Add TypeScript models and state**

In `internal/playground/web/src/App.tsx`, add `type DragEvent` to the React import and add the `Upload`, `ShieldCheck`, `Flame`, and `FileSearch` lucide imports.

Add these types below `DatabaseRow`:

```tsx
type LogXRayResponse = {
  privacy: LogXRayPrivacy
  summary: LogXRaySummary
  timeline: LogXRayTimelineEvent[]
  limits?: LogXRayLimit[]
  failure?: LogXRayFailure | null
  debugLines?: LogXRayDebugLine[]
  install: LogXRayInstallAction[]
}

type LogXRayPrivacy = {
  stored: boolean
  retention: string
  serverWrites: number
}

type LogXRaySummary = {
  apiVersion?: string
  entryCount: number
  timelineCount: number
  durationMs?: number
  soqlQueries: number
  soqlRows: number
  dmlStatements: number
  dmlRows: number
  callouts: number
  cpuTimeMs?: number
  heapBytes?: number
  userDebugs: number
  exceptions: number
  hottestLimit?: LogXRayLimit
}

type LogXRayTimelineEvent = {
  kind: string
  label: string
  detail?: string
  line?: number
  sourceLine?: number
  offsetMs?: number
  severity: "info" | "debug" | "data" | "mutation" | "callout" | "danger" | string
}

type LogXRayLimit = {
  namespace?: string
  name: string
  used: number
  limit: number
  percent?: number
  severity: "ok" | "warning" | "critical" | string
}

type LogXRayFailure = {
  kind: string
  type?: string
  message?: string
  line?: number
  sourceLine?: number
  stackFrames?: { namespace?: string; class?: string; method?: string; line?: number; raw: string }[]
  story?: LogXRayTimelineEvent[]
}

type LogXRayDebugLine = {
  line: number
  sourceLine?: number
  level?: string
  message: string
  offsetMs?: number
}

type LogXRayInstallAction = {
  label: string
  command: string
  why: string
}
```

Inside `App`, add state:

```tsx
const [workspaceTab, setWorkspaceTab] = useState<"apex" | "log-xray" | "database">("apex")
const [logXRayText, setLogXRayText] = useState("")
const [logXRayFile, setLogXRayFile] = useState("")
const [logXRay, setLogXRay] = useState<LogXRayResponse | null>(null)
const [logXRayRunning, setLogXRayRunning] = useState(false)
const [logXRayError, setLogXRayError] = useState("")
```

- [ ] **Step 4: Add API and file handlers**

Add these functions inside `App`, near `runAndHandle`:

```tsx
const analyzeLogXRay = useCallback(async (content = logXRayText, fileName = logXRayFile) => {
  const text = content.trim()
  if (!text) {
    setLogXRayError("Paste or drop a Salesforce debug log")
    setLogXRay(null)
    return
  }
  setLogXRayRunning(true)
  setLogXRayError("")
  setStatus("Analyzing")
  try {
    const result = await api<LogXRayResponse>("log-xray", {
      method: "POST",
      body: JSON.stringify({ content }),
    })
    setLogXRay(result)
    setLogXRayFile(fileName)
    setStatus("X-Ray")
  } catch (error) {
    setLogXRay(null)
    setLogXRayError(error instanceof Error ? error.message : String(error))
    setStatus("Error")
  } finally {
    setLogXRayRunning(false)
  }
}, [logXRayFile, logXRayText])

const readLogXRayFile = useCallback(async (file: File) => {
  const content = await file.text()
  setLogXRayText(content)
  setLogXRayFile(file.name)
  await analyzeLogXRay(content, file.name)
}, [analyzeLogXRay])

const onLogXRayDrop = useCallback((event: DragEvent<HTMLLabelElement>) => {
  event.preventDefault()
  const file = event.dataTransfer.files.item(0)
  if (file) {
    void readLogXRayFile(file)
  }
}, [readLogXRayFile])
```

Add a helper outside `App`:

```tsx
function formatBytes(value?: number) {
  if (!value) return "-"
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${Math.round(value / 102.4) / 10} KB`
  return `${Math.round(value / 1024 / 102.4) / 10} MB`
}
```

- [ ] **Step 5: Render the Log X-Ray work surface**

Inside `App`, create this `logXRaySurface` constant before `return`:

```tsx
const logXRaySurface = (
  <div className="log-xray-surface grid h-full min-h-0 gap-3">
    <section className="pane flex min-h-0 flex-col">
      <header className="pane-header">
        <div className="flex min-w-0 items-center gap-2">
          <FileSearch className="size-4 text-primary" />
          <h2 className="truncate text-sm font-semibold">Log X-Ray</h2>
        </div>
        <Badge variant="outline">{logXRayFile || "transient"}</Badge>
      </header>
      <div className="grid min-h-0 flex-1 grid-rows-[auto_minmax(0,1fr)_auto] gap-3 p-3">
        <label
          className="log-xray-drop"
          data-testid="log-xray-drop"
          onDragOver={(event) => event.preventDefault()}
          onDrop={onLogXRayDrop}
        >
          <Upload className="size-5 text-primary" />
          <span>Drop debug log</span>
          <input
            className="sr-only"
            type="file"
            accept=".log,.txt,text/plain"
            onChange={(event) => {
              const file = event.currentTarget.files?.[0]
              if (file) void readLogXRayFile(file)
            }}
          />
        </label>
        <textarea
          className="log-xray-textarea"
          value={logXRayText}
          onChange={(event) => setLogXRayText(event.target.value)}
          spellCheck={false}
          placeholder="Paste Salesforce debug log text"
        />
        <Button onClick={() => void analyzeLogXRay()} disabled={logXRayRunning}>
          <Flame />
          {logXRayRunning ? "Analyzing" : "Analyze"}
        </Button>
      </div>
    </section>
  </div>
)
```

Replace the center workspace section with tabs that always include Apex and Log X-Ray, and include Database only when `advanced` is true:

```tsx
<Tabs value={workspaceTab} onValueChange={(value) => setWorkspaceTab(value as "apex" | "log-xray" | "database")} className="flex h-full min-h-0 flex-col">
  <TabsList className={cn("grid w-full", advanced ? "grid-cols-3" : "grid-cols-2")}>
    <TabsTrigger value="apex">Apex</TabsTrigger>
    <TabsTrigger value="log-xray" data-testid="workspace-log-xray-tab">Log X-Ray</TabsTrigger>
    {advanced ? (
      <TabsTrigger value="database" data-testid="workspace-database-tab">Database</TabsTrigger>
    ) : null}
  </TabsList>
  <TabsContent value="apex" className="min-h-0 flex-1">
    <div className="editor-grid grid h-full min-h-0 gap-3">
      <CodeEditor
        title="Apex Source"
        contextLabel={sourcePath ? fileName(sourcePath) : "No source"}
        contextTitle={sourcePath || "No source file selected"}
        value={sourceContent}
        onChange={onSourceChange}
        readOnly={sourceReadOnly}
      />
      <CodeEditor
        title="Execute Anonymous"
        value={anonymous}
        onChange={onAnonymousChange}
        runLabel={running ? "Running" : "Run"}
        running={running}
        onRun={runAndHandle}
      />
    </div>
  </TabsContent>
  <TabsContent value="log-xray" className="min-h-0 flex-1">
    {logXRaySurface}
  </TabsContent>
  {advanced ? (
    <TabsContent value="database" className="min-h-0 flex-1">
      <div className="h-full min-h-0">{databaseBrowser}</div>
    </TabsContent>
  ) : null}
</Tabs>
```

Keep the `CodeEditor` prop list shown above. This step moves editor JSX into a tab; it does not rewrite editor behavior.

- [ ] **Step 6: Render the X-Ray output panel**

Create `logXRayOutput` before `return`:

```tsx
const logXRayOutput = (
  <div className="flex min-h-0 flex-1 flex-col">
    <div className="grid grid-cols-4 gap-2 p-3">
      <div className="metric"><span>Entries</span><strong>{logXRay?.summary.entryCount ?? "-"}</strong></div>
      <div className="metric"><span>CPU</span><strong>{logXRay?.summary.cpuTimeMs ?? 0} ms</strong></div>
      <div className="metric"><span>SOQL</span><strong>{logXRay?.summary.soqlQueries ?? 0}</strong></div>
      <div className="metric"><span>Heap</span><strong>{formatBytes(logXRay?.summary.heapBytes)}</strong></div>
    </div>
    <ScrollArea className="result-box mx-3 mb-3">
      <div className="space-y-3 p-3">
        {logXRayError ? <div className="problem danger">{logXRayError}</div> : null}
        {logXRay ? (
          <>
            <div className="xray-privacy">
              <ShieldCheck className="size-4 text-primary" />
              <span>No server storage</span>
              <code>{`writes=${logXRay.privacy.serverWrites}`}</code>
            </div>
            {logXRay.failure ? (
              <section className="xray-card danger">
                <span>Failure story</span>
                <strong>{logXRay.failure.type || logXRay.failure.kind}</strong>
                <p>{logXRay.failure.message || "No message"}</p>
              </section>
            ) : (
              <section className="xray-card">
                <span>Failure story</span>
                <strong>No exception found</strong>
              </section>
            )}
            <section className="xray-card">
              <span>Governor limits</span>
              <div className="xray-limits">
                {(logXRay.limits ?? []).slice(0, 8).map((limit) => (
                  <div className="xray-limit" key={`${limit.namespace}-${limit.name}`}>
                    <div>
                      <strong>{limit.name}</strong>
                      <code>{limit.used}/{limit.limit}</code>
                    </div>
                    <span className={cn("xray-limit-bar", limit.severity)} style={{ width: `${Math.min(100, limit.percent ?? 0)}%` }} />
                  </div>
                ))}
              </div>
            </section>
            <section className="xray-card">
              <span>Hot path timeline</span>
              <div className="xray-timeline">
                {logXRay.timeline.map((event, index) => (
                  <div className={cn("xray-event", event.severity)} key={`${event.line}-${index}`}>
                    <code>{event.offsetMs ?? 0}ms</code>
                    <strong>{event.label}</strong>
                    <span>{event.detail || `log line ${event.line ?? "-"}`}</span>
                  </div>
                ))}
              </div>
            </section>
            <section className="xray-card">
              <span>Install payoff</span>
              <div className="space-y-2">
                {logXRay.install.map((action) => (
                  <div className="xray-command" key={action.command}>
                    <strong>{action.label}</strong>
                    <code>{action.command}</code>
                    <p>{action.why}</p>
                  </div>
                ))}
              </div>
            </section>
          </>
        ) : !logXRayError ? (
          <div className="text-sm text-muted-foreground">No log analyzed</div>
        ) : null}
      </div>
    </ScrollArea>
  </div>
)
```

In the output pane body, render `logXRayOutput` when `workspaceTab === "log-xray"`. Render the existing Apex output block otherwise. Do not remove existing output tabs.

- [ ] **Step 7: Add CSS**

Append to `internal/playground/web/src/index.css`:

```css
.log-xray-surface {
  grid-template-columns: minmax(0, 1fr);
}

.log-xray-drop {
  display: flex;
  min-height: 74px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px dashed var(--selection-border);
  border-radius: 4px;
  background: var(--selection-background-soft);
  color: var(--foreground);
  cursor: pointer;
}

.log-xray-drop:hover {
  border-color: var(--primary);
  background: var(--selection-background);
}

.log-xray-textarea {
  min-height: 0;
  resize: none;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--well);
  color: var(--foreground);
  padding: 12px;
  font: inherit;
  line-height: 1.6;
  outline: none;
}

.xray-privacy,
.xray-card,
.xray-command {
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--pane-raised);
  padding: 10px;
}

.xray-privacy {
  display: flex;
  align-items: center;
  gap: 8px;
}

.xray-privacy code {
  margin-left: auto;
}

.xray-card > span {
  display: block;
  margin-bottom: 6px;
  color: var(--muted-foreground);
  text-transform: uppercase;
  font-size: 10px;
}

.xray-card.danger {
  border-color: color-mix(in oklch, var(--destructive) 45%, var(--border));
}

.xray-limits {
  display: grid;
  gap: 8px;
}

.xray-limit {
  position: relative;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--well);
  padding: 7px;
}

.xray-limit > div {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.xray-limit-bar {
  position: absolute;
  inset: 0 auto 0 0;
  background: rgba(55, 217, 255, 0.14);
}

.xray-limit-bar.warning {
  background: rgba(210, 153, 34, 0.22);
}

.xray-limit-bar.critical {
  background: rgba(248, 81, 73, 0.25);
}

.xray-timeline {
  display: grid;
  gap: 7px;
}

.xray-event {
  display: grid;
  grid-template-columns: 48px 82px minmax(0, 1fr);
  gap: 8px;
  align-items: center;
  border-left: 3px solid var(--primary);
  padding: 6px 8px;
  background: var(--well);
}

.xray-event.debug {
  border-left-color: #bc8cff;
}

.xray-event.data {
  border-left-color: #39d2c0;
}

.xray-event.mutation {
  border-left-color: #d29922;
}

.xray-event.danger {
  border-left-color: var(--destructive);
}

.xray-event span,
.xray-command p {
  min-width: 0;
  margin: 0;
  color: var(--muted-foreground);
  overflow-wrap: anywhere;
}

.xray-command {
  display: grid;
  gap: 5px;
}

.xray-command code {
  overflow-wrap: anywhere;
}
```

- [ ] **Step 8: Run UI tests**

Run:

```bash
npm --prefix internal/playground/web test -- App.test.tsx
```

Expected:

```text
PASS  src/App.test.tsx
```

- [ ] **Step 9: Commit UI source**

```bash
git add internal/playground/web/src/App.tsx internal/playground/web/src/App.test.tsx internal/playground/web/src/index.css
git commit -m "feat: add playground log xray tab"
```

---

### Task 4: Docs And Embedded Assets

**Files:**

- Modify: `site/docs-src/guide/playground.md`
- Modify: `docs/EDITOR.md`
- Modify: `internal/playground/static/index.html`
- Modify: `internal/playground/static/assets/*`

- [ ] **Step 1: Update hosted playground docs**

In `site/docs-src/guide/playground.md`, under `## What to use it for`, add:

```markdown
- Drop a Salesforce debug log into Log X-Ray to inspect limits, hot events, failure paths, and local `glade debug` next steps. The hosted playground parses the request in memory and does not store the log.
```

After the examples section, add:

````markdown
## Log X-Ray

Use **Log X-Ray** when you have one Salesforce debug log and need a fast read on what happened. Paste or drop the log in the hosted playground. Glade parses it in memory and returns a hot path timeline, governor-limit heat map, failure story, debug excerpts, and install commands for deeper local work.

The hosted route does not write the log to the workspace, SQLite, cache, or server disk. Source-aware matching stays local:

```bash
glade debug profile --log your.log
glade debug explain --log your.log --project .
glade debug repro --log your.log --project . > ReproTest.cls
```
````

- [ ] **Step 2: Update editor docs**

In `docs/EDITOR.md`, in `## Offline Debug Log Analysis`, add this paragraph before the command block:

```markdown
The public playground also has **Log X-Ray** for a fast hosted preview. It accepts one log, parses it in memory, and returns a timeline, limit heat map, failure story, and install commands. It does not store logs. Use local `glade debug explain` and `glade debug repro` when source matching or repro synthesis matters.
```

- [ ] **Step 3: Build embedded playground assets**

Run:

```bash
npm --prefix internal/playground/web run build
```

Expected:

```text
built
```

Confirm `internal/playground/static/index.html` and `internal/playground/static/assets/*` changed.

- [ ] **Step 4: Run docs and asset smoke tests**

Run:

```bash
go test ./internal/playground -run 'TestServerServesEmbeddedPlaygroundUI' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/playground
```

- [ ] **Step 5: Commit docs and assets**

```bash
git add site/docs-src/guide/playground.md docs/EDITOR.md internal/playground/static/index.html internal/playground/static/assets
git commit -m "docs: document playground log xray"
```

---

### Task 5: Final Verification

**Files:**

- No new files.

- [ ] **Step 1: Run focused Go tests**

Run:

```bash
go test ./internal/apexlog ./internal/profile ./internal/playground -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/apexlog
ok  	github.com/glade-sh/glade/internal/profile
ok  	github.com/glade-sh/glade/internal/playground
```

- [ ] **Step 2: Run playground web tests**

Run:

```bash
npm --prefix internal/playground/web test
```

Expected:

```text
Test Files passed
```

- [ ] **Step 3: Run web build**

Run:

```bash
npm --prefix internal/playground/web run build
```

Expected:

```text
built
```

- [ ] **Step 4: Run product smoke**

Run:

```bash
scripts/smoke.sh
```

Expected:

```text
smoke ok
```

- [ ] **Step 5: Manual no-storage check**

Run:

```bash
tmp="$(mktemp -d)"
go run ./cmd/glade playground --examples --public --addr 127.0.0.1:1789 --data-root "$tmp/data" --db "$tmp/org.sqlite" >"$tmp/server.log" 2>&1 &
pid=$!
trap 'kill $pid 2>/dev/null || true; rm -rf "$tmp"' EXIT
sleep 2
payload="$(jq -Rs '{content: .}' internal/apexlog/testdata/core.log)"
curl -sS -X POST http://127.0.0.1:1789/playground/api/log-xray -H 'content-type: application/json' -d "$payload" >"$tmp/xray.json"
jq '.privacy, .summary.soqlQueries, .summary.dmlStatements' "$tmp/xray.json"
find "$tmp/data" -type f | sort
```

Expected:

```text
{
  "stored": false,
  "retention": "none",
  "serverWrites": 0
}
1
1
```

The `find` output shows only normal playground workspace files. It does not show an uploaded log, cache file, or X-Ray artifact.

- [ ] **Step 6: Final commit if verification changed assets**

If the build rewrote static assets after Task 4, run:

```bash
git add internal/playground/static/index.html internal/playground/static/assets
git commit -m "build: refresh playground assets"
```

Skip this commit when there is no asset diff.

## Self-Review

Spec coverage:

- Server-side transient parsing: Task 1 and Task 2.
- No storage: Task 1 privacy response, Task 2 workspace hash test, Task 5 manual check.
- Public playground wow result: Task 3.
- Install bridge: Task 1 response and Task 3 render.
- Docs: Task 4.

Placeholder scan:

- No placeholder markers found.
- No open-ended error handling step.
- No project-specific exception.

Type consistency:

- Go response fields match TypeScript response fields.
- Endpoint path is `/playground/api/log-xray` in server, UI, and docs.
- Request field is `content` in Go and TypeScript.

Plan complete. Execute one task at a time. Keep commits small. The log is handled like a hot coal: pick it up, read it, set it down. Nothing into the shed.
