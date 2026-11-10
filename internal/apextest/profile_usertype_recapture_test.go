package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// TestProfileUserTypeRecaptureMatchesConformanceRows ties the earlier
// conformance rows to the paired one-day recapture: per API, the same
// Name|UserType|License|Key set.
func TestProfileUserTypeRecaptureMatchesConformanceRows(t *testing.T) {
	read := func(path string) [][]string {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma, r.LazyQuotes = '\t', true
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	old := map[string]map[string]bool{}
	rows := read(filepath.Join("testdata", "conformance", "profile_usertype", "org.tsv"))
	for _, row := range rows[1:] {
		if row[6] != "ROW" || row[3] != "P1" {
			continue
		}
		var rec struct {
			Name, UserType string
			UserLicense    struct{ Name, LicenseDefinitionKey string }
		}
		if err := json.Unmarshal([]byte(row[8]), &rec); err != nil {
			t.Fatal(err)
		}
		if old[row[0]] == nil {
			old[row[0]] = map[string]bool{}
		}
		old[row[0]][rec.Name+"|"+rec.UserType+"|"+rec.UserLicense.Name+"|"+rec.UserLicense.LicenseDefinitionKey] = true
	}
	recap := map[string]map[string]bool{}
	for _, row := range read(filepath.Join("testdata", "native-capture", "profile_usertype_recap", "org.tsv"))[1:] {
		if row[1] != "P1" {
			continue
		}
		if recap[row[0]] == nil {
			recap[row[0]] = map[string]bool{}
		}
		recap[row[0]][row[2]+"|"+row[3]+"|"+row[4]+"|"+row[5]] = true
	}
	keys := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	for _, api := range []string{"62.0", "67.0"} {
		if len(recap[api]) != 38 || !reflect.DeepEqual(keys(old[api]), keys(recap[api])) {
			t.Errorf("API %s: recapture %d rows differs from conformance rows %d", api, len(recap[api]), len(old[api]))
		}
	}
}
