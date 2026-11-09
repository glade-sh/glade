package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Compare a fresh native capture with the oracle used by the CDC runtime tests.
func TestCDCRefreshNativeObservations(t *testing.T) {
	read := func(path string) [][]string {
		t.Helper()
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var rows [][]string
		for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
			row := strings.SplitN(line, "\t", 4)
			if len(row) != 4 {
				t.Fatalf("malformed row in %s", path)
			}
			rows = append(rows, row)
		}
		return rows
	}
	base := filepath.Join("testdata", "conformance")
	old := map[string]any{}
	for _, row := range read(filepath.Join(base, "cdc_journal_delivery", "org.tsv"))[1:] {
		var value any
		if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
			t.Fatal(err)
		}
		old[row[0]+"/"+row[2]] = value
	}
	rows := read(filepath.Join(base, "b7cdc_refresh", "org.tsv"))[1:]
	if len(rows) != 10 {
		t.Fatalf("got %d rows, want 10", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows {
		key := row[0] + "/" + row[1] + "/" + row[2]
		if seen[key] {
			t.Fatalf("duplicate row %s", key)
		}
		seen[key] = true
		var got any
		if err := json.Unmarshal([]byte(row[3]), &got); err != nil {
			t.Fatal(err)
		}
		var want any
		switch row[1] {
		case "istest":
			var ok bool
			want, ok = old[row[0]+"/"+row[2]]
			if !ok {
				t.Fatalf("missing original row %s", key)
			}
		case "anonymous":
			want = "System.TypeException"
		default:
			t.Fatalf("unknown route %s", key)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", key, got, want)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range []string{"istest/cdcExplicit", "istest/cdcDisabledDelivery", "istest/cdcHeaders", "anonymous/enable", "anonymous/deliver"} {
			if !seen[api+"/"+c] {
				t.Errorf("missing row %s/%s", api, c)
			}
		}
	}
}
