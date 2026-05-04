package capability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMVPReportIsNotReadyUntilRequiredFeaturesAreSupported(t *testing.T) {
	report := MVPReport()
	if report.Ready {
		t.Fatal("MVP report should not be ready while required features are partial or unsupported")
	}
	if report.Required == 0 || report.Incomplete == 0 {
		t.Fatalf("report = %#v", report)
	}
	for _, feature := range report.Features {
		if feature.Required && feature.Status != StatusSupported {
			return
		}
	}
	t.Fatal("expected at least one incomplete required feature")
}

func TestWriteJSON(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, MVPReport()); err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Target != "full-featured aer-parity MVP" {
		t.Fatalf("target = %q", decoded.Target)
	}
}

func TestWriteText(t *testing.T) {
	var out bytes.Buffer
	if err := WriteText(&out, MVPReport()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "MVP readiness: not ready") || !strings.Contains(text, "Trigger invocation") {
		t.Fatalf("text output = %q", text)
	}
}

func TestWriteMarkdown(t *testing.T) {
	var out bytes.Buffer
	if err := WriteMarkdown(&out, MVPReport()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"# Compatibility Dashboard",
		"Generated from `internal/capability`.",
		"Required complete:",
		"| Area | ID | Status | Capability | Notes |",
		"`triggers.runtime`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown output missing %q: %q", want, text)
		}
	}
}

func TestWriteKnownGapsMarkdown(t *testing.T) {
	var out bytes.Buffer
	if err := WriteKnownGapsMarkdown(&out, MVPReport()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"# Known Gaps",
		"Generated from `internal/capability`.",
		"Required incomplete:",
		"### `apex.sema.body`: Method-body semantic analysis",
		"### `release.packaging`: Installable release binaries, checksums, docs",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("known gaps output missing %q: %q", want, text)
		}
	}
}

func TestDatabaseStdlibRowsAreLocallyPromotedOrFenced(t *testing.T) {
	for _, entry := range StdlibMatrix() {
		if entry.Area != "Database" {
			continue
		}
		if entry.Status == StatusPartial {
			t.Fatalf("Database stdlib row %s remains partial: %s", entry.API, entry.Notes)
		}
		if entry.Status == StatusSupported && entry.Notes == "" {
			t.Fatalf("Database stdlib row %s needs local-model notes", entry.API)
		}
	}
}

func TestHTTPStdlibRowsAreLocallyPromotedOrFenced(t *testing.T) {
	watched := map[string]Status{
		"Http.send local mock callouts":    StatusSupported,
		"Http.send real network transport": StatusUnsupported,
	}
	for _, entry := range StdlibMatrix() {
		want, ok := watched[entry.API]
		if !ok {
			continue
		}
		delete(watched, entry.API)
		if entry.Status != want {
			t.Fatalf("%s = %s, want %s: %s", entry.API, entry.Status, want, entry.Notes)
		}
		if entry.Notes == "" {
			t.Fatalf("%s needs local-model notes", entry.API)
		}
	}
	for api := range watched {
		t.Fatalf("missing HTTP stdlib row %s", api)
	}
}

func TestDateDatetimeTimeZoneRowsAreLocallyPromotedOrFenced(t *testing.T) {
	watched := map[string]bool{
		"Date.addMonths":          true,
		"Date.addYears":           true,
		"Date.today":              true,
		"Datetime.addDays":        true,
		"Datetime.addMonths":      true,
		"Datetime.addYears":       true,
		"Datetime.format":         true,
		"Datetime.formatGmt":      true,
		"Datetime.now":            true,
		"TimeZone.getDisplayName": true,
		"TimeZone.getID":          true,
		"TimeZone.getOffset":      true,
		"TimeZone.getTimeZone":    true,
		"UserInfo.getTimeZone":    true,
	}
	for _, entry := range StdlibMatrix() {
		if !watched[entry.API] {
			continue
		}
		if entry.Status != StatusSupported {
			t.Fatalf("%s remains %s: %s", entry.API, entry.Status, entry.Notes)
		}
		if entry.Notes == "" {
			t.Fatalf("%s needs local-model notes", entry.API)
		}
	}
}
