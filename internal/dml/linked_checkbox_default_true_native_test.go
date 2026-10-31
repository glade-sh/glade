package dml

import (
	"encoding/csv"
	"io"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// TestCustomCheckboxDefaultTrueInsertNative consumes the API 62/67 insert
// and SOQL observations in ../vm/testdata/linked_checkbox_default_true/org.tsv.
func TestCustomCheckboxDefaultTrueInsertNative(t *testing.T) {
	f, err := os.Open("../vm/testdata/linked_checkbox_default_true/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	rows := map[string]map[string]string{}
	for n := 0; ; n++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			continue
		}
		if len(rec) < 4 {
			if len(rec) == 3 && rec[1] == "reject" {
				continue
			}
			t.Fatalf("bad row %d", n+1)
		}
		if rows[rec[0]] == nil {
			rows[rec[0]] = map[string]string{}
		}
		rows[rec[0]][rec[1]+"/"+rec[2]] = rec[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"omitted", "null", "false", "true"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				org := storage.NewOrgState()
				org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", KeyPrefix: "a90", Fields: map[string]storage.Field{
					"Name":    {APIName: "Name", Type: storage.FieldString, Required: true},
					"Flag__c": {APIName: "Flag__c", Label: "Flag", Type: storage.FieldBoolean, DefaultValue: "true", Createable: storage.BoolFlag(true), Nillable: storage.BoolFlag(false)},
				}}, Records: map[storage.ID]storage.Record{}}
				fields := map[string]storage.Value{"Name": storage.StringValue(name)}
				switch name {
				case "null":
					fields["Flag__c"] = storage.NullValue()
				case "false":
					fields["Flag__c"] = storage.BooleanValue(false)
				case "true":
					fields["Flag__c"] = storage.BooleanValue(true)
				}
				engine := NewEngine(&org)
				result := engine.Insert([]storage.Record{{Object: "Parent__c", Fields: fields}})
				if len(result) != 1 {
					t.Fatalf("insert: %#v", result)
				}
				expected, exists := rows[api]["insert/"+name]
				if !exists {
					t.Fatal("missing native insert row")
				}
				if expected != "success" {
					if result[0].Success || result[0].StatusCode != expected {
						t.Fatalf("insert: %#v; native %s", result[0], expected)
					}
					return
				}
				if !result[0].Success {
					t.Fatalf("insert: %#v", result[0])
				}
				expected, exists = rows[api]["soql/"+name]
				if !exists || (expected != "true" && expected != "false") {
					t.Fatalf("missing native queried Boolean: %q", expected)
				}
				got, present := org.Objects["Parent__c"].Records[result[0].ID].GetField("Flag__c")
				if !present || got.Kind != storage.ValueBoolean || got.Boolean != (expected == "true") {
					t.Fatalf("stored = %#v present=%t; native %s", got, present, expected)
				}
			})
		}
	}
}

// Metadata-loaded checkbox fields have a BOOLEAN display type and may omit
// describe nillability. The same native insert rows apply to that route.
func TestCustomCheckboxMetadataNullInsertNative(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", KeyPrefix: "a90", Fields: map[string]storage.Field{
		"Name":    {APIName: "Name", Type: storage.FieldString, Required: true},
		"Flag__c": {APIName: "Flag__c", Label: "Flag", Type: storage.FieldBoolean, DisplayType: "boolean", DefaultValue: "true"},
	}}, Records: map[storage.ID]storage.Record{}}
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{Object: "Parent__c", Fields: map[string]storage.Value{"Name": storage.StringValue("null"), "Flag__c": storage.NullValue()}}})
	if len(result) != 1 || result[0].Success || result[0].StatusCode != "INVALID_TYPE_ON_FIELD_IN_RECORD" {
		t.Fatalf("metadata checkbox insert = %#v", result)
	}
}
