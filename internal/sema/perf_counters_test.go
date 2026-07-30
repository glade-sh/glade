package sema

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestDurationNanosecondsClampsNegativeDurations(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   time.Duration
		want uint64
	}{
		{name: "negative", in: -time.Nanosecond, want: 0},
		{name: "zero", in: 0, want: 0},
		{name: "positive", in: 42 * time.Nanosecond, want: 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := durationNanoseconds(tc.in); got != tc.want {
				t.Fatalf("durationNanoseconds(%s) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestPerfRecorderDisabledStateIsCompact(t *testing.T) {
	// Leave room for counters and scalar snapshots while preventing a full
	// runtime.MemStats value from returning to the disabled call frame.
	const maxDisabledRecorderBytes = 512
	if size := unsafe.Sizeof(perfRecorder{}); size > maxDisabledRecorderBytes {
		t.Fatalf("disabled recorder state is %d bytes, want at most %d", size, maxDisabledRecorderBytes)
	}
}

func TestAnalyzePerfCountersDisabledByDefault(t *testing.T) {
	index := benchmarkIndexWithSources(t, 8)
	counters := PerfCounters{}

	AnalyzeWithOptions(index, AnalyzeOptions{
		Diagnostics:  true,
		ExportTypes:  true,
		PerfCounters: &counters,
	})

	if !reflect.DeepEqual(counters, PerfCounters{}) {
		t.Fatalf("disabled counters changed: %#v", counters)
	}
}

func TestAnalyzePerfCountersCaptureAllPhases(t *testing.T) {
	index := benchmarkIndexWithSources(t, 16)
	counters := PerfCounters{Enabled: true}

	AnalyzeWithOptions(index, AnalyzeOptions{
		Diagnostics:  true,
		ExportTypes:  true,
		PerfCounters: &counters,
	})

	if counters.TotalNS == 0 {
		t.Fatal("total duration was not captured")
	}
	for name, phase := range map[string]PhaseCounters{
		"source/schema enrichment": counters.SourceSchemaEnrichment,
		"platform model":           counters.PlatformModel,
		"type-member model":        counters.TypeMemberModel,
		"method bodies":            counters.MethodBodies,
		"inheritance":              counters.Inheritance,
		"query semantics":          counters.QuerySemantics,
		"export":                   counters.Export,
	} {
		if phase.Calls == 0 {
			t.Errorf("%s calls were not captured", name)
		}
		if phase.DurationNS == 0 {
			t.Errorf("%s duration was not captured", name)
		}
	}
}

func TestAnalyzePerfCountersPreserveResult(t *testing.T) {
	index := benchmarkIndexWithSources(t, 12)
	want := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, ExportTypes: true})
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	counters := PerfCounters{Enabled: true}

	got := AnalyzeWithOptions(index, AnalyzeOptions{
		Diagnostics:  true,
		ExportTypes:  true,
		PerfCounters: &counters,
	})
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("profiled result changed\nwant: %#v\n got: %#v", want, got)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("profiled JSON changed\nwant: %s\n got: %s", wantJSON, gotJSON)
	}
}

func TestAnalyzePerfCountersArePerCall(t *testing.T) {
	index := benchmarkIndexWithSources(t, 8)
	first := PerfCounters{Enabled: true, TotalNS: ^uint64(0)}
	second := PerfCounters{Enabled: true}

	AnalyzeWithOptions(index, AnalyzeOptions{
		Diagnostics:  true,
		ExportTypes:  true,
		PerfCounters: &first,
	})
	firstAfterCall := first
	AnalyzeWithOptions(index, AnalyzeOptions{
		Diagnostics:  true,
		ExportTypes:  true,
		PerfCounters: &second,
	})

	if first.TotalNS == ^uint64(0) || first.TotalNS == 0 {
		t.Fatalf("first call did not replace prior counter values: %#v", first)
	}
	if second.TotalNS == 0 {
		t.Fatalf("second call was not captured: %#v", second)
	}
	if !reflect.DeepEqual(first, firstAfterCall) {
		t.Fatalf("second call mutated first call counters\n before: %#v\n  after: %#v", firstAfterCall, first)
	}
}

func TestAnalyzePerfCountersConcurrentDistinctOutputs(t *testing.T) {
	firstIndex := benchmarkIndexWithSources(t, 8)
	secondIndex := benchmarkIndexWithSources(t, 8)
	first := PerfCounters{Enabled: true, TotalNS: ^uint64(0)}
	second := PerfCounters{Enabled: true, TotalNS: ^uint64(0)}

	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		AnalyzeWithOptions(firstIndex, AnalyzeOptions{
			Diagnostics:  true,
			ExportTypes:  true,
			PerfCounters: &first,
		})
	}()
	go func() {
		defer wait.Done()
		AnalyzeWithOptions(secondIndex, AnalyzeOptions{
			Diagnostics:  true,
			ExportTypes:  true,
			PerfCounters: &second,
		})
	}()
	wait.Wait()

	for name, counters := range map[string]PerfCounters{"first": first, "second": second} {
		if counters.TotalNS == 0 || counters.TotalNS == ^uint64(0) {
			t.Errorf("%s output was not populated: %#v", name, counters)
		}
		if counters.MethodBodies.Calls == 0 || counters.Export.Calls == 0 {
			t.Errorf("%s output is missing phase counters: %#v", name, counters)
		}
	}
	secondAfterCalls := second
	first.TotalNS = 1
	if !reflect.DeepEqual(second, secondAfterCalls) {
		t.Fatalf("mutating first output changed second output\n before: %#v\n  after: %#v", secondAfterCalls, second)
	}
}

func TestAnalyzeOptionsFingerprintCoversEveryOptionField(t *testing.T) {
	fields := reflect.VisibleFields(reflect.TypeOf(AnalyzeOptions{}))
	gotNames := make([]string, 0, len(fields))
	for _, field := range fields {
		gotNames = append(gotNames, field.Name)
	}
	wantNames := []string{
		"Diagnostics",
		"ExportTypes",
		"SuppressPerformanceDiagnostics",
		"PerfCounters",
		"BuildArtifacts",
		"CapturedSource",
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("AnalyzeOptions fields changed; classify every new field in AnalyzeOptionsFingerprint\nwant: %v\n got: %v", wantNames, gotNames)
	}

	base := AnalyzeOptionsFingerprint(AnalyzeOptions{})
	for _, tc := range []struct {
		name string
		opts AnalyzeOptions
	}{
		{name: "diagnostics", opts: AnalyzeOptions{Diagnostics: true}},
		{name: "export types", opts: AnalyzeOptions{ExportTypes: true}},
		{name: "performance diagnostics", opts: AnalyzeOptions{SuppressPerformanceDiagnostics: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := AnalyzeOptionsFingerprint(tc.opts); got == base {
				t.Fatalf("behavior-affecting option did not change fingerprint: %q", got)
			}
		})
	}

	instrumented := AnalyzeOptions{
		PerfCounters:   &PerfCounters{Enabled: true},
		BuildArtifacts: &typesys.BuildArtifacts{},
		CapturedSource: func(string) (string, bool) { return "", false },
	}
	if got := AnalyzeOptionsFingerprint(instrumented); got != base {
		t.Fatalf("request transport or instrumentation changed fingerprint\nbase: %s\n got: %s", base, got)
	}
}

func TestAnalyzeOptionsFingerprintIsStable(t *testing.T) {
	opts := AnalyzeOptions{
		Diagnostics:                    true,
		ExportTypes:                    true,
		SuppressPerformanceDiagnostics: true,
	}
	const want = "009b9f699f5eee6dba2ab4eff186ab640b33535f061cf97338331e7254f50556"
	if got := AnalyzeOptionsFingerprint(opts); got != want {
		t.Fatalf("AnalyzeOptionsFingerprint() = %q, want %q", got, want)
	}
}

func TestSemanticABIIsVersioned(t *testing.T) {
	if SemanticABI == "" {
		t.Fatal("SemanticABI must not be empty")
	}
}
