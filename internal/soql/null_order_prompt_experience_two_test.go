package soql

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// This fixture distinguishes null placement using two distinct populated contexts.
func TestPromptExperienceTwoSitesNullsNativeOrdering(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/nulls_order_prompt_experience_two/org.tsv")
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
	org := storage.NewOrgState()
	const object, field = "PromptVersion", "ExperienceContextId"
	definition := storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Marker__c": {APIName: "Marker__c", Type: storage.FieldString}, field: {APIName: field, Type: storage.FieldReference, ReferenceTo: []string{"Site", "CommunityTemplateDefinition"}}}}
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
	checked := 0
	for _, row := range rows[1:] {
		if len(row[2]) != 3 || !strings.Contains(row[2], "N") || !strings.Contains(row[2], "L") || !strings.Contains(row[2], "H") {
			continue
		}
		checked++
		result, err := ParseAndExecute(org, "SELECT Marker__c FROM "+object+" ORDER BY "+field+" "+row[1])
		if err != nil {
			t.Fatalf("%s %s: %v", row[0], row[1], err)
		}
		var got strings.Builder
		for _, rec := range result.Records {
			got.WriteString(rec.Fields["Marker__c"].String)
		}
		if got.String() != row[2] {
			t.Errorf("%s %s: got %s native %s", row[0], row[1], got.String(), row[2])
		}
	}
	if checked != 12 {
		t.Fatalf("checked %d ordering rows, want 12", checked)
	}
}
