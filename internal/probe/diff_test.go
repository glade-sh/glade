package probe

import (
	"testing"
)

func ptr(s string) *string { return &s }

func TestCompareNoGap(t *testing.T) {
	golden := ProbeResult{ProbeID: "p1", Result: "same"}
	local := ProbeResult{ProbeID: "p1", Result: "same"}
	if gap := Compare(golden, local); gap != nil {
		t.Fatalf("expected no gap, got %+v", gap)
	}
}

func TestCompareBehavioral(t *testing.T) {
	golden := ProbeResult{ProbeID: "p1", Result: "org-value"}
	local := ProbeResult{ProbeID: "p1", Result: "local-value"}
	gap := Compare(golden, local)
	if gap == nil {
		t.Fatal("expected gap")
	}
	if gap.GapType != GapTypeBehavioral {
		t.Fatalf("expected behavioral, got %s", gap.GapType)
	}
	if gap.Severity != "medium" {
		t.Fatalf("expected medium severity, got %s", gap.Severity)
	}
}

func TestCompareUnsupported(t *testing.T) {
	golden := ProbeResult{ProbeID: "p1", Result: "ok"}
	local := ProbeResult{ProbeID: "p1", ExceptionType: ptr("UnsupportedFeatureException")}
	gap := Compare(golden, local)
	if gap == nil {
		t.Fatal("expected gap")
	}
	if gap.GapType != GapTypeUnsupported {
		t.Fatalf("expected unsupported, got %s", gap.GapType)
	}
	if gap.Severity != "high" {
		t.Fatalf("expected high severity, got %s", gap.Severity)
	}
}

func TestCompareOrgExceptionLocalResult(t *testing.T) {
	golden := ProbeResult{ProbeID: "p1", ExceptionType: ptr("System.NullPointerException")}
	local := ProbeResult{ProbeID: "p1", Result: "something"}
	gap := Compare(golden, local)
	if gap == nil {
		t.Fatal("expected gap")
	}
	if gap.GapType != GapTypeBehavioral {
		t.Fatalf("expected behavioral, got %s", gap.GapType)
	}
}

func TestCompareBothExceptionSameType(t *testing.T) {
	golden := ProbeResult{ProbeID: "p1", ExceptionType: ptr("System.NullPointerException")}
	local := ProbeResult{ProbeID: "p1", ExceptionType: ptr("System.NullPointerException")}
	if gap := Compare(golden, local); gap != nil {
		t.Fatalf("expected no gap for same exception type, got %+v", gap)
	}
}

func TestCompareBothExceptionDifferentType(t *testing.T) {
	golden := ProbeResult{ProbeID: "p1", ExceptionType: ptr("System.NullPointerException")}
	local := ProbeResult{ProbeID: "p1", ExceptionType: ptr("System.IllegalArgumentException")}
	gap := Compare(golden, local)
	if gap == nil {
		t.Fatal("expected gap")
	}
	if gap.GapType != GapTypeBehavioral {
		t.Fatalf("expected behavioral, got %s", gap.GapType)
	}
}
