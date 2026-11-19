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

func TestWorkPlanMessagingParentNullsNativeOrdering(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/nulls_order_workplan_messaging/org.tsv")
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
	if len(rows) != 3 {
		t.Fatalf("native rows = %d, want 3", len(rows))
	}
	const object, field = "WorkPlan", "WorkOrderId"
	org := storage.NewOrgState()
	def := storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Marker__c": {APIName: "Marker__c", Type: storage.FieldString}, field: {APIName: field, Type: storage.FieldReference, ReferenceTo: []string{"WorkOrder"}}}}
	state := storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{}}
	for i, label := range []string{"N", "L", "H"} {
		id := storage.ID(fmt.Sprintf("a00%012d", i+1))
		fields := map[string]storage.Value{"Marker__c": storage.StringValue(label)}
		if i > 0 {
			fields[field] = storage.IDValue(storage.ID(fmt.Sprintf("001%012d", i)))
		}
		state.Records[id] = storage.Record{ID: id, Object: object, Fields: fields}
	}
	org.Objects[object] = state
	for _, row := range rows[1:] {
		var native map[string]any
		if err := json.Unmarshal([]byte(row[1]), &native); err != nil {
			t.Fatal(err)
		}
		if native["savedNull"] != true || native["distinct"] != float64(2) {
			t.Fatalf("%s invalid native shape: %v", row[0], native)
		}
		for _, tail := range []string{"ASC", "DESC", "ASC NULLS FIRST", "ASC NULLS LAST", "DESC NULLS FIRST", "DESC NULLS LAST"} {
			result, err := ParseAndExecute(org, "SELECT Marker__c FROM "+object+" ORDER BY "+field+" "+tail)
			if err != nil {
				t.Fatal(err)
			}
			var got strings.Builder
			for _, rec := range result.Records {
				got.WriteString(rec.Fields["Marker__c"].String)
			}
			if got.String() != native[tail] {
				t.Errorf("%s %s: got %s native %v", row[0], tail, got.String(), native[tail])
			}
		}
	}
}
