package apextest

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestGetterHelperReentryNativeConformance(t *testing.T) {
	runAccessorConstructorNativeConformance(t, "getter")
}

func TestProtectedReflectionNativeConformance(t *testing.T) {
	runAccessorConstructorNativeConformance(t, "reflection")
}

func runAccessorConstructorNativeConformance(t *testing.T, family string) {
	t.Helper()
	var data struct {
		APIVersion             string              `json:"apiVersion"`
		Route                  string              `json:"route"`
		NativeObservationCount int                 `json:"nativeObservationCount"`
		CapturedMarker         string              `json:"capturedMarker"`
		Source                 nativeBoundaryRoute `json:"source"`
		AdditionalSource       nativeBoundaryRoute `json:"additionalSource"`
		Rows                   []struct {
			ID         string                    `json:"id"`
			SourceRef  string                    `json:"sourceRef"`
			Family     string                    `json:"family"`
			TestMethod string                    `json:"testMethod"`
			Expected   nativeObservationExpected `json:"expected"`
		} `json:"rows"`
		Carries []struct {
			Family          string          `json:"family"`
			Boundary        string          `json:"boundary"`
			Reason          string          `json:"reason"`
			NativeReference json.RawMessage `json:"nativeReference"`
		} `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/accessor_constructor_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.APIVersion != "64.0" || data.Route != "isTest" || data.NativeObservationCount != 8 || len(data.Rows) != 8 || data.CapturedMarker != "OWNED_VM64|" {
		t.Fatal("incomplete native accessor/constructor packet")
	}
	nativeBoundaryCheckSources(t, data.Source)
	nativeBoundaryCheckSources(t, data.AdditionalSource)
	counts := newConformanceCountLog(t, "AccessorConstructor_"+family, data.APIVersion)
	seen := map[string]bool{}
	matched := 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || row.TestMethod == "" {
			t.Fatal("invalid native observation row")
		}
		seen[row.ID] = true
		if row.Family != family {
			continue
		}
		matched++
		t.Run(row.ID, func(t *testing.T) {
			source := data.Source
			if row.SourceRef == "additional" {
				source = data.AdditionalSource
			}
			source.ID, source.TestMethod = row.ID, row.TestMethod
			captured := source.Classes
			source.Classes = make(map[string]string, len(captured))
			// Adapt only the terminal observation marker to the shared JSON comparator.
			// Source hashes above validate the unmodified captured class bytes.
			for name, body := range captured {
				source.Classes[name] = strings.ReplaceAll(body, data.CapturedMarker, "NATIVE_RESULT|")
			}
			counts.TrackKind(t, data.Route, "exact", 1)
			residualObservationObserve(t, data.APIVersion, data.Route, source, row.Expected)
		})
	}
	wantMatched := 3
	if family == "reflection" {
		wantMatched = 5
	}
	if matched != wantMatched {
		t.Fatal("missing native candidate/control observations")
	}
	carries := 0
	for _, carry := range data.Carries {
		if carry.Family != family {
			continue
		}
		if carry.Boundary == "" || carry.Reason == "" || len(carry.NativeReference) == 0 {
			t.Fatal("missing carry boundary/native evidence")
		}
		carries++
		t.Logf("carried %s: %s; observed reference=%s", carry.Boundary, carry.Reason, carry.NativeReference)
	}
	if carries == 0 {
		t.Fatal("missing explicit unobserved boundary")
	}
}
