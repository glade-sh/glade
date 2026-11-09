package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"
)

// The row helper is shared by partial USER_MODE DML routes. The fields and
// exact message come from the unmodified captured rows, not a guessed join.
func TestPartialDMLMultifieldNativeRowMessage(t *testing.T) {
	f, err := os.Open("testdata/partial_dml_multifield/org.tsv")
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
	if len(rows) != 4 {
		t.Fatalf("got %d rows", len(rows))
	}
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			var payload struct {
				Results []struct {
					Errors []struct {
						Fields  []string `json:"getFields"`
						Message string   `json:"getMessage"`
						Status  string   `json:"getStatusCode"`
					} `json:"errors"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(row[3]), &payload); err != nil {
				t.Fatal(err)
			}
			observed := payload.Results[0].Errors[0]
			got := userModeFieldDMLRowFailure("insert", "SynthPartial__c", Object("SynthPartial__c"), observed.Fields)
			if got.Error != observed.Message || got.StatusCode != observed.Status {
				t.Errorf("got %q (%s); native %q (%s)", got.Error, got.StatusCode, observed.Message, observed.Status)
			}
		})
	}
}
