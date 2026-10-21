package dml

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRecordTypeDefaultUsesCapturedTypeSelection(t *testing.T) {
	native, err := os.ReadFile("../apextest/testdata/conformance/stub_object_collision/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	const typeID storage.ID = "012000000000101AAA"
	field := storage.Field{APIName: "TypeName__c", Type: storage.FieldString, DefaultValue: "$RecordType.Name"}
	definition := storage.ObjectDefinition{
		APIName: "CanopyEntry__c",
		Fields:  map[string]storage.Field{field.APIName: field},
		RecordTypes: []storage.RecordTypeInfo{{
			ID: typeID, Name: "Alpha", DeveloperName: "Alpha", Active: true, Available: true,
		}},
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(native)), "\n") {
		row := strings.SplitN(line, "\t", 3)
		if len(row) != 3 || (row[1] != "observeRecordTypeDefault" && row[1] != "observeExplicitRecordTypeDefault") {
			continue
		}
		key := row[0] + "/" + row[1]
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("duplicate or uncaptured type-selection row %q", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			var observation map[string]any
			if err := json.Unmarshal([]byte(row[2]), &observation); err != nil {
				t.Fatal(err)
			}
			persisted, exists := observation["persisted"]
			if !exists {
				t.Fatalf("missing native persisted value: %s", row[2])
			}
			want := ""
			if persisted != nil {
				var ok bool
				want, ok = persisted.(string)
				if !ok {
					t.Fatalf("unexpected native text value: %v", persisted)
				}
			}
			record := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{}}
			if row[1] == "observeExplicitRecordTypeDefault" {
				record.Fields["RecordTypeId"] = storage.IDValue(typeID)
			}
			value, ok := defaultValueForRecordField(nil, definition, record, field)
			if !ok || value.Kind != storage.ValueString || value.String != want {
				t.Fatalf("native persisted=%v; default=%#v, present=%v", persisted, value, ok)
			}
		})
	}
	if len(seen) != 4 {
		t.Fatalf("native type-selection rows=%d, want 4", len(seen))
	}
}
