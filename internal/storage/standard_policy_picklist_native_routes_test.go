package storage

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The rows separately record named Apex and Visualforce observations; route
// differences and API errors remain evidence rather than inferred parity.
func TestStandardPolicyPicklistCapturedRouteValues(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_policy_picklist_routes/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, row := range rows[1:] {
		if row[1] != "named" && row[1] != "visualforce" {
			continue
		}
		key := row[0] + "/" + row[1]
		counts[key]++
		t.Run(key+"/"+row[2], func(t *testing.T) {
			parts := strings.SplitN(row[2], ".", 2)
			if len(parts) != 2 {
				t.Fatal("invalid field")
			}
			def, ok := StandardObjectDefinition(parts[0])
			if !ok {
				t.Fatal("missing object")
			}
			field, ok := def.Fields[parts[1]]
			if !ok {
				t.Fatal("missing field")
			}
			field.PicklistValues = nil
			field.PicklistValuesConfigured = false
			def.Fields[parts[1]] = field
			EnsureStandardObjectFields(&def)
			var want string
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			var values []string
			for _, value := range def.Fields[parts[1]].PicklistValues {
				values = append(values, fmt.Sprintf("%s:%t:%t", value.Value, value.Active, value.Default))
			}
			if got := strings.Join(values, "~"); got != want {
				t.Fatalf("repaired list=%q; native %s=%q", got, key, want)
			}
		})
	}
	for _, key := range []string{"62.0/named", "67.0/named", "59.0/visualforce", "67.0/visualforce"} {
		if counts[key] != 8 {
			t.Errorf("%s rows=%d want 8", key, counts[key])
		}
	}
}
