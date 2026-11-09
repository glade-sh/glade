package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPartialDMLMultifieldNativeFailedIDs(t *testing.T) {
	f, err := os.Open("testdata/partial_dml_multifield/ids.tsv")
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
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			var p struct {
				Results []struct {
					ID     *string `json:"getId"`
					Errors []struct {
						Fields []string `json:"getFields"`
					} `json:"errors"`
				} `json:"results"`
				Inputs []struct {
					ID *string `json:"inputRecordId"`
				} `json:"inputs"`
			}
			if err := json.Unmarshal([]byte(row[3]), &p); err != nil {
				t.Fatal(err)
			}
			op := strings.Split(row[0], "-")[0]
			if strings.HasPrefix(op, "upsert") {
				op = "upsert"
			}
			for i := 0; i < 2; i++ {
				record := Object("SynthPartial__c")
				if p.Inputs[i].ID != nil {
					record.Fields["Id"] = String(*p.Inputs[i].ID)
				}
				got := userModeFieldDMLRowFailure(op, record.Type, record, p.Results[i].Errors[0].Fields)
				want := ""
				if p.Results[i].ID != nil {
					want = *p.Results[i].ID
				}
				if string(got.ID) != want {
					t.Errorf("row %d ID=%q native=%q", i, got.ID, want)
				}
			}
		})
	}
}
