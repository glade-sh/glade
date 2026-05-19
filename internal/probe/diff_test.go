package probe

import (
	"sync"
	"testing"

	"github.com/open-aer/oaer/internal/capability"
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

func TestCompareBlobToPdfAllowsPdfShapeMatch(t *testing.T) {
	golden := ProbeResult{
		ProbeID: "stub.blob.topdf.sig-string",
		Result:  "JVBERi0xLjQKJSVFT0YK",
	}
	local := ProbeResult{
		ProbeID: "stub.blob.topdf.sig-string",
		Result:  "JVBERi0xLjQKMSAwIG9iago8PCAvVHlwZSAvQ2F0YWxvZyA+PgplbmRvYmoKJSVFT0YK",
	}
	if gap := Compare(golden, local); gap != nil {
		t.Fatalf("expected no gap for PDF-shape comparison, got %+v", gap)
	}
}

func TestCompareVolatileProbesIgnoreExactValue(t *testing.T) {
	cases := []string{
		"stub.crypto.getrandominteger",
		"stub.crypto.getrandomlong",
		"stub.date.today",
		"stub.datetime.now",
		"stub.math.random",
		"stub.date.hashcode",
	}
	for _, id := range cases {
		golden := ProbeResult{ProbeID: id, Result: 123.0}
		local := ProbeResult{ProbeID: id, Result: 456.0}
		if gap := Compare(golden, local); gap != nil {
			t.Fatalf("%s expected no gap, got %+v", id, gap)
		}
	}
}

func TestCompareStubContractUnsupportedVsCompileEquivalent(t *testing.T) {
	stubContractProbeByID = map[string]capability.StubContractProbeSpec{
		"stub.connectapi.example": {
			ID:   "stub.connectapi.example",
			Kind: "method",
			Mode: capability.StubContractOrgDiff,
		},
	}
	stubContractProbeOnce = sync.Once{}
	stubContractProbeOnce.Do(func() {})
	golden := ProbeResult{ProbeID: "stub.connectapi.example", ExceptionType: ptr("System.UnsupportedOperationException")}
	local := ProbeResult{ProbeID: "stub.connectapi.example", ExceptionType: ptr("System.CompileException")}
	if gap := Compare(golden, local); gap != nil {
		t.Fatalf("expected no gap for metadata-scoped stub contract equivalence, got %+v", gap)
	}
}

func TestCompareUnsupportedVsCompileNonStubStillGap(t *testing.T) {
	golden := ProbeResult{ProbeID: "p1", ExceptionType: ptr("System.UnsupportedOperationException")}
	local := ProbeResult{ProbeID: "p1", ExceptionType: ptr("System.CompileException")}
	gap := Compare(golden, local)
	if gap == nil {
		t.Fatal("expected gap")
	}
	if gap.GapType != GapTypeBehavioral {
		t.Fatalf("expected behavioral, got %s", gap.GapType)
	}
}

func TestCompareStubContractRuntimeMismatchStillGap(t *testing.T) {
	stubContractProbeByID = map[string]capability.StubContractProbeSpec{
		"stub.connectapi.example.runtime": {
			ID:   "stub.connectapi.example.runtime",
			Kind: "method",
			Mode: capability.StubContractOrgDiff,
		},
	}
	stubContractProbeOnce = sync.Once{}
	stubContractProbeOnce.Do(func() {})
	golden := ProbeResult{ProbeID: "stub.connectapi.example.runtime", ExceptionType: ptr("System.UnsupportedOperationException")}
	local := ProbeResult{ProbeID: "stub.connectapi.example.runtime", ExceptionType: ptr("ExecutionError")}
	gap := Compare(golden, local)
	if gap == nil {
		t.Fatal("expected gap")
	}
	if gap.GapType != GapTypeBehavioral {
		t.Fatalf("expected behavioral, got %s", gap.GapType)
	}
}
