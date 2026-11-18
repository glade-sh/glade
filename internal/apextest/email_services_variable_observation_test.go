package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Email domain identities differ between native orgs even when their deployed
// metadata is the same. Keep each native row and compare the fixed fields
// exactly; derive per-label bounds from the original and isolation captures.
// email_services_metadata/org.tsv:2, email_domain_recheck/org.tsv:2, and
// carried_conflict_isolation/org.tsv:2,254 provide the API 62 anonymous
// observations; :20,272 provide the API 67 isolation observations.
func assertEmailServicesVariableObservation(t *testing.T, row []string, want, got any) bool {
	t.Helper()
	if row[3] == "missing" {
		return false
	}
	original, ok := want.([]any)
	if !ok || len(original) != 1 {
		t.Fatalf("native email observation: expected one record: %v", want)
	}
	wantRecord, ok := original[0].(map[string]any)
	if !ok {
		t.Fatalf("native email observation is not an object: %v", original[0])
	}
	wantLengths, ok := wantRecord["domainLabelLengths"].([]any)
	if !ok || len(wantLengths) == 0 {
		t.Fatalf("native email observation missing label lengths: %v", wantRecord)
	}
	minimum, maximum := make([]float64, len(wantLengths)), make([]float64, len(wantLengths))
	observations := 0
	add := func(value any) {
		t.Helper()
		lengths, ok := value.([]any)
		if !ok || len(lengths) != len(minimum) {
			t.Fatalf("invalid native domain label lengths: %v", value)
		}
		for i, raw := range lengths {
			n, ok := raw.(float64)
			if !ok || n <= 0 || n != float64(int(n)) {
				t.Fatalf("invalid native label length: %v", raw)
			}
			if observations == 0 || n < minimum[i] {
				minimum[i] = n
			}
			if observations == 0 || n > maximum[i] {
				maximum[i] = n
			}
		}
		observations++
	}
	add(wantLengths)
	readRows := func(name string) [][]string {
		t.Helper()
		file, err := os.Open(filepath.Join("testdata", "conformance", name, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		reader := csv.NewReader(file)
		reader.Comma, reader.LazyQuotes = '\t', true
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	for _, native := range readRows("carried_conflict_isolation") {
		if len(native) != 8 || (native[0] != "plain" && native[0] != "plain2") || native[1] != "baseline" || native[2] != row[0] || native[3] != row[2] || native[4] != "with" || native[5] != "administrator" {
			continue
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(native[7]), &result); err != nil {
			t.Fatal(err)
		}
		records, ok := result[row[3]].([]any)
		if !ok || len(records) != 1 {
			t.Fatalf("invalid isolation email record: %v", result[row[3]])
		}
		record, ok := records[0].(map[string]any)
		if !ok {
			t.Fatalf("invalid isolation email record: %v", records[0])
		}
		if record["developerName"] != wantRecord["developerName"] || record["domainSuffix"] != wantRecord["domainSuffix"] {
			t.Fatalf("isolation capture does not match original metadata: %v", record)
		}
		add(record["domainLabelLengths"])
	}
	for _, native := range readRows("email_domain_recheck") {
		if len(native) != 4 || native[0] != row[0] || native[1] != row[2] {
			continue
		}
		var records []map[string]any
		if err := json.Unmarshal([]byte(native[3]), &records); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, record := range records {
			if record["address"] == wantRecord["developerName"] {
				if record["suffix"] != wantRecord["domainSuffix"] {
					t.Fatalf("recheck suffix differs: %v", record)
				}
				add(record["lengths"])
				found = true
			}
		}
		if !found {
			t.Fatalf("missing recheck for native address %v", wantRecord["developerName"])
		}
	}
	if observations < 4 || reflect.DeepEqual(minimum, maximum) {
		t.Fatalf("insufficient variable native evidence: count=%d bounds=%v/%v", observations, minimum, maximum)
	}
	if err := compareEmailDomainObservation(want, got, minimum, maximum); err != nil {
		t.Fatal(err)
	}
	return true
}

func compareEmailDomainObservation(want, got any, minimum, maximum []float64) error {
	wanted, wok := want.([]any)
	actual, aok := got.([]any)
	if !wok || !aok || len(wanted) != 1 || len(actual) != 1 {
		return fmt.Errorf("native=%v; Glade=%v: expected one address", want, got)
	}
	w, wok := wanted[0].(map[string]any)
	a, aok := actual[0].(map[string]any)
	if !wok || !aok {
		return fmt.Errorf("native=%v; Glade=%v: expected address objects", want, got)
	}
	lengths, ok := a["domainLabelLengths"].([]any)
	if !ok || len(lengths) != len(minimum) || len(minimum) != len(maximum) {
		return fmt.Errorf("unexpected label count: %v", a["domainLabelLengths"])
	}
	for i, raw := range lengths {
		n, ok := raw.(float64)
		if !ok || n != float64(int(n)) || n < minimum[i] || n > maximum[i] {
			return fmt.Errorf("label %d length %v outside observed native range [%v,%v]", i, raw, minimum[i], maximum[i])
		}
	}
	// Do not mutate the native observation or relax fields other than lengths.
	wc, ac := make(map[string]any, len(w)), make(map[string]any, len(a))
	for k, v := range w {
		if k != "domainLabelLengths" {
			wc[k] = v
		}
	}
	for k, v := range a {
		if k != "domainLabelLengths" {
			ac[k] = v
		}
	}
	if !reflect.DeepEqual(wc, ac) {
		return fmt.Errorf("native fixed fields=%v; Glade=%v", wc, ac)
	}
	return nil
}

func TestEmailVariableDomainObservationBounds(t *testing.T) {
	record := func(first float64, suffix string, active bool) []any {
		return []any{map[string]any{"domainLabelLengths": []any{first, float64(11), float64(7)}, "domainSuffix": suffix, "active": active}}
	}
	want := record(50, ".apex.sandbox.salesforce.com", true)
	for _, tc := range []struct {
		name  string
		got   []any
		valid bool
	}{
		{"lower-bound", record(49, ".apex.sandbox.salesforce.com", true), true},
		{"upper-bound", record(52, ".apex.sandbox.salesforce.com", true), true},
		{"below-bound", record(48, ".apex.sandbox.salesforce.com", true), false},
		{"above-bound", record(53, ".apex.sandbox.salesforce.com", true), false},
		{"wrong-suffix", record(50, ".invalid", true), false},
		{"changed-fixed-field", record(50, ".apex.sandbox.salesforce.com", false), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := compareEmailDomainObservation(want, tc.got, []float64{49, 11, 6}, []float64{52, 11, 7})
			if (err == nil) != tc.valid {
				t.Fatalf("got %v, valid=%v", err, tc.valid)
			}
		})
	}
}
