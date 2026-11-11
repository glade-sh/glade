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

func TestOutboundTargetNullsNativeOrdering(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/nulls_order_outbound_target/org.tsv")
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
	var absent, native map[string]any
	if err := json.Unmarshal([]byte(rows[1][1]), &absent); err != nil {
		t.Fatal(err)
	}
	if absent["availability"] != "absent" {
		t.Fatalf("API 62 availability = %v", absent)
	}
	if err := json.Unmarshal([]byte(rows[2][1]), &native); err != nil {
		t.Fatal(err)
	}
	if native["savedNull"] != true || native["distinct"] != float64(2) {
		t.Fatalf("invalid native shape: %v", native)
	}
	const object, field = "OutboundEngmtContentRsrc", "TargetReferenceRecordId"
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Marker__c": {APIName: "Marker__c", Type: storage.FieldString}, field: {APIName: field, Type: storage.FieldReference, ReferenceTo: []string{"Promotion"}}}}
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
		for _, rec := range result.Records {
			got.WriteString(rec.Fields["Marker__c"].String)
		}
		if got.String() != native[tail] {
			t.Errorf("%s: got %s native %v", tail, got.String(), native[tail])
		}
	}
}
