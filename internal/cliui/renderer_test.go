package cliui

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEventJSONIsStable(t *testing.T) {
	ev := Event{
		Kind:    EventPhaseStart,
		Phase:   "checking",
		Label:   "Checking Apex semantics",
		Current: 3,
		Total:   7,
		Detail:  "AccountService.cls",
		At:      time.Unix(100, 250000000).UTC(),
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"kind":"phase_start","phase":"checking","label":"Checking Apex semantics","detail":"AccountService.cls","current":3,"total":7,"at":"1970-01-01T00:01:40.25Z"}`
	if string(data) != want {
		t.Fatalf("json = %s, want %s", data, want)
	}
}

func TestNewRendererUsesLineRendererForNonTTY(t *testing.T) {
	var out bytes.Buffer
	r := NewRenderer(RendererOptions{Stderr: &out, Mode: ProgressLine})
	if _, ok := r.(*LineRenderer); !ok {
		t.Fatalf("renderer = %T, want *LineRenderer", r)
	}
}

func TestLineRendererWritesReadableProgress(t *testing.T) {
	var out bytes.Buffer
	r := NewLineRenderer(&out, fixedClock(time.Unix(10, 0).UTC()))
	r.Render(Event{Kind: EventPhaseStart, Phase: "checking", Label: "Checking Apex semantics"})
	r.Render(Event{Kind: EventPhaseTick, Phase: "checking", Label: "Analyzed AccountService", Current: 2, Total: 5})
	r.Render(Event{Kind: EventFail, Phase: "checking", Label: "GLADESEMA002", Detail: "Unknown type MissingType"})
	r.Finish(Result{OK: false, Label: "check failed", ExitCode: 1})

	got := out.String()
	for _, want := range []string{
		"checking · Checking Apex semantics",
		"checking · 2/5 Analyzed AccountService",
		"✗ checking: GLADESEMA002 - Unknown type MissingType",
		"done · check failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("line output missing %q:\n%s", want, got)
		}
	}
}

func TestNDJSONRendererWritesOneEventPerLine(t *testing.T) {
	var out bytes.Buffer
	r := NewNDJSONRenderer(&out)
	r.Render(Event{Kind: EventPhaseStart, Phase: "schema", Label: "Loading metadata"})
	r.Finish(Result{OK: true, Label: "schema loaded"})

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2:\n%s", len(lines), out.String())
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("invalid json line %q", line)
		}
	}
	if !strings.Contains(lines[0], `"kind":"phase_start"`) || !strings.Contains(lines[1], `"kind":"done"`) {
		t.Fatalf("unexpected ndjson:\n%s", out.String())
	}
	if !strings.Contains(lines[1], `"ok":true`) || !strings.Contains(lines[1], `"exitCode":0`) {
		t.Fatalf("done event omitted status fields:\n%s", lines[1])
	}
}

func TestActivityFeedKeepsLastNEvents(t *testing.T) {
	feed := NewActivityFeed(3)
	for _, label := range []string{"one", "two", "three", "four"} {
		feed.Add(Event{Kind: EventInfo, Label: label})
	}
	got := feed.Lines()
	want := []string{"two", "three", "four"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("feed = %#v, want %#v", got, want)
	}
}

func TestProgressBarRendersBoundedAndUnbounded(t *testing.T) {
	if got := RenderProgressBar(5, 10, 10); got != "[===>    ]" {
		t.Fatalf("bounded bar = %q", got)
	}
	if got := RenderProgressBar(3, 0, 10); got != "[   >    ]" {
		t.Fatalf("unbounded bar = %q", got)
	}
}

func TestFormatDurationForProgress(t *testing.T) {
	cases := map[time.Duration]string{
		1500 * time.Millisecond:   "2s",
		65 * time.Second:          "1m05s",
		2*time.Hour + time.Minute: "2h01m",
	}
	for input, want := range cases {
		if got := FormatDuration(input); got != want {
			t.Fatalf("FormatDuration(%s) = %s, want %s", input, got, want)
		}
	}
}

func TestTTYRendererUsesANSIRegionAndActivityFeed(t *testing.T) {
	var out bytes.Buffer
	r := NewTTYRenderer(&out, fixedClock(time.Unix(20, 0).UTC()))
	r.Render(Event{Kind: EventPhaseStart, Phase: "test", Label: "Running tests", Total: 2})
	r.Render(Event{Kind: EventPhaseTick, Phase: "test", Label: "PASS AccountTest.creates", Current: 1, Total: 2})
	r.Render(Event{Kind: EventFail, Phase: "test", Label: "FAIL ContactTest.validates", Detail: "Expected Active"})
	r.Finish(Result{OK: false, Label: "1 passed, 1 failed", ExitCode: 1})

	got := out.String()
	for _, want := range []string{
		"\r\x1b[K",
		"[",
		"Running tests",
		"PASS AccountTest.creates",
		"FAIL ContactTest.validates - Expected Active",
		"1 passed, 1 failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("tty output missing %q:\n%q", want, got)
		}
	}
}

func TestTTYRendererTruncatesLongLabels(t *testing.T) {
	var out bytes.Buffer
	r := NewTTYRenderer(&out, fixedClock(time.Unix(30, 0).UTC()))
	r.SetWidthForTest(50)
	r.Render(Event{
		Kind:    EventPhaseTick,
		Phase:   "test",
		Label:   "Running VeryLongAccountDomainServiceTestName.withAnEquallyLongMethodName",
		Current: 1,
		Total:   2,
	})
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if VisibleWidth(stripANSI(line)) > 50 {
			t.Fatalf("line too wide (%d): %q", VisibleWidth(stripANSI(line)), line)
		}
	}
}

func stripANSI(s string) string {
	replacer := strings.NewReplacer("\r", "", "\x1b[K", "", "\x1b[1A", "")
	return replacer.Replace(s)
}

type fixedClock time.Time

func (c fixedClock) Now() time.Time {
	return time.Time(c)
}
