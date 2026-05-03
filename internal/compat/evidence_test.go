package compat

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/capability"
)

func TestBuildEvidenceReport(t *testing.T) {
	catalog := capability.Catalog{Entries: []capability.CatalogEntry{{
		Symbol: "String.trim",
		Area:   "Core stdlib",
		Target: capability.TargetExecutableParity,
		Status: capability.StatusSupported,
	}, {
		Symbol: "String.contains",
		Area:   "Core stdlib",
		Target: capability.TargetExecutableParity,
		Status: capability.StatusSupported,
	}, {
		Symbol: "ConnectApi.FeedElement.body",
		Area:   "Product namespaces",
		Target: capability.TargetTypedStub,
		Status: capability.StatusUnknown,
	}}}
	fixtures := []Fixture{{
		Name: "string-fixture",
		Evidence: []FixtureEvidence{{
			Symbol: "String.trim",
			Kind:   "exec",
		}, {
			Symbol: "Nope.missing",
			Kind:   "exec",
		}},
	}}

	report := BuildEvidenceReport(catalog, fixtures)
	if report.CatalogEntries != 3 || report.Fixtures != 1 || report.Evidence != 2 {
		t.Fatalf("report counts = %#v", report)
	}
	if len(report.Covered) != 1 || report.Covered[0].Symbol != "String.trim" {
		t.Fatalf("covered = %#v", report.Covered)
	}
	if len(report.UnmatchedEvidence) != 1 || report.UnmatchedEvidence[0].Symbol != "Nope.missing" {
		t.Fatalf("unmatched = %#v", report.UnmatchedEvidence)
	}
	if len(report.UngatedPromoted) != 1 || report.UngatedPromoted[0].Symbol != "String.contains" {
		t.Fatalf("ungated = %#v", report.UngatedPromoted)
	}
}

func TestWriteEvidenceJSON(t *testing.T) {
	report := EvidenceReport{Covered: []CoveredEvidence{{
		Symbol:   "String.trim",
		Status:   capability.StatusSupported,
		Target:   capability.TargetExecutableParity,
		Area:     "Core stdlib",
		Fixtures: []string{"string-fixture"},
	}}}
	var out bytes.Buffer
	if err := WriteEvidenceJSON(&out, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"symbol": "String.trim"`) {
		t.Fatalf("json = %q", out.String())
	}
	var decoded EvidenceReport
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Covered[0].Fixtures[0] != "string-fixture" {
		t.Fatalf("decoded = %#v", decoded)
	}
}
