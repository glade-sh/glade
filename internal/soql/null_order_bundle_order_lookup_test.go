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

// Each native row supplies five orderings for a null and two parent IDs.
// Build an owned local object so this checks query ordering, not just the
// classification table. Native setup skips are not treated as results.
func TestBundleOrderLookupNullsNativeOrdering(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/nulls_order_bundle_order_lookup/org.tsv")
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
	tails := []string{"NULLS FIRST", "NULLS LAST", "ASC", "DESC", "DESC NULLS LAST"}
	checked := 0
	for _, row := range rows[1:] {
		row := row
		var obs map[string]any
		if err := json.Unmarshal([]byte(row[3]), &obs); err != nil {
			t.Fatal(err)
		}
		if _, skip := obs["skip"]; skip {
			continue
		}
		checked++
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			object, field := row[1], row[2]
			org := storage.NewOrgState()
			definition := storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Marker__c": {APIName: "Marker__c", Type: storage.FieldString}, field: {APIName: field, Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}}
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
			for _, tail := range tails {
				result, err := ParseAndExecute(org, "SELECT Marker__c FROM "+object+" ORDER BY "+field+" "+tail)
				if err != nil {
					t.Fatal(err)
				}
				var got strings.Builder
				for _, record := range result.Records {
					got.WriteString(record.Fields["Marker__c"].String)
				}
				if got.String() != obs[tail] {
					t.Errorf("%s order=%s native=%v", tail, got.String(), obs[tail])
				}
			}
		})
	}
	if checked != 2 {
		t.Fatalf("classified observations=%d, want 2", checked)
	}
}
