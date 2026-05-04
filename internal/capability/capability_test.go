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
