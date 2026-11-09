package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPartialDMLMultifieldNativeAssignmentOrder(t *testing.T) {
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
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			var payload struct {
				Results []struct {
					Errors []struct {
						Fields []string `json:"getFields"`
					} `json:"errors"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(row[3]), &payload); err != nil {
				t.Fatal(err)
			}
			machine := recordSharingControlMachine(t, "Private")
			state := machine.Org.Objects["Widget__c"]
			for _, field := range []string{"Zeta__c", "Alpha__c", "Mid__c"} {
				state.Definition.Fields[field] = storage.Field{APIName: field, Type: storage.FieldString}
			}
			machine.Org.Objects["Widget__c"] = state
			machine.currentMethod = Method{APIVersion: row[1]}
			record := Object("Widget__c")
			for _, field := range []string{"Zeta__c", "Alpha__c", "Mid__c"} {
				record.Fields[field] = String("value")
				markExplicitSObjectField(&record, field)
			}
			got, err := machine.userModeDMLRecordAccess("insert", record, "isCreateable", "isCreateable", "")
			if err != nil {
				t.Fatal(err)
			}
			if want := payload.Results[0].Errors[0].Fields; !reflect.DeepEqual(got, want) {
				t.Fatalf("fields=%v, native=%v", got, want)
			}
		})
	}
}
