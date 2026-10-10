package vm_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

type floorCompilerMismatch struct {
	Family     string            `json:"family"`
	RowID      string            `json:"rowId"`
	NativeText string            `json:"nativeText"`
	LocalTexts map[string]string `json:"localTexts"`
	Reason     string            `json:"reason"`
}

type floorCompilerMismatches struct {
	family  string
	rows    map[string]floorCompilerMismatch
	framing map[string]map[string]string
}

type floorCompilerLineFraming struct {
	Family string   `json:"family"`
	RowID  string   `json:"rowId"`
	Routes []string `json:"routes"`
	Reason string   `json:"reason"`
}

// Keep the transport prefix, punctuation, spacing and diagnostic text exact;
// only the leading wrapper's numeric line is eligible for normalization.
var floorCompilerWrapperLine = regexp.MustCompile(`^(COMPILE_ERROR\tline )[0-9]+(: )`)

func floorCompilerSameTextAfterWrapperLine(native, local string) bool {
	return floorCompilerWrapperLine.MatchString(native) && floorCompilerWrapperLine.MatchString(local) &&
		floorCompilerWrapperLine.ReplaceAllString(native, "${1}<wrapper>${2}") ==
			floorCompilerWrapperLine.ReplaceAllString(local, "${1}<wrapper>${2}")
}

// The manifest freezes observed diagnostic gaps, not desired product behavior.
// Rejection remains asserted by each caller. A new or repaired gap must change
// this reviewed list explicitly; it cannot silently become an exact match.
func newFloorCompilerMismatches(t *testing.T, family string, nativeRows map[string]string, routes ...string) *floorCompilerMismatches {
	t.Helper()
	raw, err := os.ReadFile("../apextest/testdata/conformance/apex_floor_diagnostic_mismatches.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		APIVersion  string                     `json:"apiVersion"`
		Mismatches  []floorCompilerMismatch    `json:"mismatches"`
		LineFraming []floorCompilerLineFraming `json:"lineFraming"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.APIVersion != "62.0" {
		t.Fatalf("mismatch manifest API %q, want 62.0", manifest.APIVersion)
	}
	allowedRoutes := map[string]bool{}
	for _, route := range routes {
		allowedRoutes[route] = true
	}
	result := &floorCompilerMismatches{family: family, rows: map[string]floorCompilerMismatch{}, framing: map[string]map[string]string{}}
	seen := map[string]bool{}
	for _, row := range manifest.Mismatches {
		key := row.Family + "/" + row.RowID
		if seen[key] || row.RowID == "" || strings.TrimSpace(row.Reason) == "" || strings.ContainsAny(row.Reason, "\n\r\t") {
			t.Fatalf("invalid or repeated mismatch %s", key)
		}
		seen[key] = true
		if row.Family != family {
			continue
		}
		if nativeRows[row.RowID] != row.NativeText || !strings.HasPrefix(row.NativeText, "COMPILE_ERROR\t") {
			t.Fatalf("%s mismatch native text does not equal its floor compiler oracle", key)
		}
		if len(row.LocalTexts) != len(allowedRoutes) {
			t.Fatalf("%s mismatch routes %v, want %v", key, row.LocalTexts, allowedRoutes)
		}
		for route, local := range row.LocalTexts {
			if !allowedRoutes[route] || !strings.HasPrefix(local, "COMPILE_ERROR\t") {
				t.Fatalf("%s invalid mismatch route/text %q <%s>", key, route, local)
			}
		}
		result.rows[row.RowID] = row
	}
	seenFraming := map[string]bool{}
	for _, row := range manifest.LineFraming {
		key := row.Family + "/" + row.RowID
		if seenFraming[key] || row.RowID == "" || len(row.Routes) == 0 || strings.TrimSpace(row.Reason) == "" || strings.ContainsAny(row.Reason, "\n\r\t") {
			t.Fatalf("invalid or repeated line-framing row %s", key)
		}
		seenFraming[key] = true
		if row.Family != family {
			continue
		}
		if !floorCompilerWrapperLine.MatchString(nativeRows[row.RowID]) {
			t.Fatalf("%s line-framing row has no floor compiler line oracle", key)
		}
		result.framing[row.RowID] = map[string]string{}
		for _, route := range row.Routes {
			if !allowedRoutes[route] || result.framing[row.RowID][route] != "" {
				t.Fatalf("%s invalid or repeated line-framing route %q", key, route)
			}
			if frozen, ok := result.rows[row.RowID]; ok && !floorCompilerSameTextAfterWrapperLine(frozen.NativeText, frozen.LocalTexts[route]) {
				t.Fatalf("%s %s frozen wording mismatch cannot count as line-framing", key, route)
			}
			result.framing[row.RowID][route] = row.Reason
		}
	}
	return result
}

func (m *floorCompilerMismatches) Kind(rowID, route string) string {
	if m.framing[rowID][route] != "" {
		return "line-framing"
	}
	if row, ok := m.rows[rowID]; ok && row.LocalTexts[route] != "" {
		return "mismatch"
	}
	return "exact"
}

func (m *floorCompilerMismatches) Check(t *testing.T, rowID, route, native, local string) {
	t.Helper()
	reason := m.framing[rowID][route]
	framed := reason != ""
	match := native == local
	if framed {
		if native == local {
			t.Errorf("%s %s API 62.0 %s line-framing difference is fixed; remove the reviewed framing entry", m.family, rowID, route)
		}
		match = floorCompilerSameTextAfterWrapperLine(native, local)
		if !match {
			t.Errorf("%s %s API 62.0 %s diagnostic differs beyond the leading wrapper line: native <%s> local <%s>", m.family, rowID, route, native, local)
		}
		t.Logf("%s %s API 62.0 %s line-framing: native <%s> local <%s>; %s", m.family, rowID, route, native, local, reason)
	}
	row, listed := m.rows[rowID]
	if !listed {
		if !framed && !match {
			t.Errorf("%s %s API 62.0 %s new compiler mismatch: native <%s> local <%s>", m.family, rowID, route, native, local)
		}
		return
	}
	if !framed && match {
		t.Errorf("%s %s API 62.0 %s recorded compiler mismatch is fixed; remove the reviewed entry", m.family, rowID, route)
	}
	if row.NativeText != native || row.LocalTexts[route] != local {
		t.Errorf("%s %s API 62.0 %s compiler mismatch changed: native <%s> local <%s>; recorded native <%s> local <%s>", m.family, rowID, route, native, local, row.NativeText, row.LocalTexts[route])
	}
	if !framed {
		t.Logf("%s %s API 62.0 %s compiler mismatch: native <%s> local <%s>; %s", m.family, rowID, route, native, local, row.Reason)
	}
}
