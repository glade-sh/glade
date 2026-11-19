package soql

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Two distinct synthetic data sources establish full null ordering.
func TestTwoDataSourcesNullsNativeOrdering(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/nulls_order_two_datasources/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, row := range rows[1:] {
		var obs map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row[1]), &obs); err != nil {
			t.Fatal(err)
		}
		for _, object := range []string{"Product2", "ContentVersion"} {
			var native map[string]any
			if err := json.Unmarshal(obs[object], &native); err != nil {
				t.Fatal(err)
			}
			checked++
			t.Run(row[0]+"/"+object, func(t *testing.T) {
				org := storage.NewOrgState()
				const field = "ExternalDataSourceId"
				definition := storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Marker__c": {APIName: "Marker__c", Type: storage.FieldString}, field: {APIName: field, Type: storage.FieldReference, ReferenceTo: []string{"ExternalDataSource"}}}}
				state := storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
				for i, label := range []string{"N", "L", "H"} {
					id := storage.ID(fmt.Sprintf("a00%012d", i+1))
					fields := map[string]storage.Value{"Marker__c": storage.StringValue(label)}
					if i > 0 {
						fields[field] = storage.IDValue(storage.ID(fmt.Sprintf("001%012d", i)))
					}
					state.Records[id] = storage.Record{ID: id, Object: object, Fields: fields}
				}
				org.Objects[object] = state
				for _, tail := range []string{"ASC", "DESC", "ASC NULLS FIRST", "ASC NULLS LAST", "DESC NULLS FIRST", "DESC NULLS LAST"} {
					result, err := ParseAndExecute(org, "SELECT Marker__c FROM "+object+" ORDER BY "+field+" "+tail)
					if err != nil {
						t.Fatal(err)
					}
					var got strings.Builder
					for _, record := range result.Records {
						got.WriteString(record.Fields["Marker__c"].String)
					}
					if got.String() != native[tail] {
						t.Errorf("%s order=%s native=%v", tail, got.String(), native[tail])
					}
				}
			})
		}
	}
	if checked != 4 {
		t.Fatalf("classified observations=%d, want 4", checked)
	}
}
