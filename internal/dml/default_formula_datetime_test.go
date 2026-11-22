package dml

import (
	"encoding/csv"
	"html"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE default formulas: internal/vm/testdata/default_formula_datetime/org.tsv:2 anonymous 62.0
// NATIVE default formulas: internal/vm/testdata/default_formula_datetime/org.tsv:3 anonymous 67.0
// NATIVE default formulas: internal/vm/testdata/default_formula_datetime/org.tsv:4 istest 62.0
// NATIVE default formulas: internal/vm/testdata/default_formula_datetime/org.tsv:5 istest 67.0
func TestDefaultFormulaDatetimeInsert(t *testing.T) {
	f, err := os.Open("../vm/testdata/default_formula_datetime/org.tsv")
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
	if len(rows) != 5 {
		t.Fatalf("incomplete capture: %d rows", len(rows))
	}
	for _, row := range rows[1:] {
		text := html.UnescapeString(row[5])
		if !strings.Contains(text, "plainInsert=") || strings.Count(text, "offsetNear=true|minuteNow=true|tomorrow=true|number=7.00|blankNull=true") != 4 {
			t.Fatalf("unexpected native row %v", row)
		}
	}
	org := storage.NewOrgState()
	now := time.Date(2026, 5, 15, 10, 7, 0, 0, time.UTC)
	org.Now = func() time.Time { return now }
	def := storage.ObjectDefinition{APIName: "DefaultProbe__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
		"Name":        {APIName: "Name", Type: storage.FieldString},
		"Moment__c":   {APIName: "Moment__c", Type: storage.FieldDateTime, DefaultValue: "NOW()"},
		"Offset__c":   {APIName: "Offset__c", Type: storage.FieldDateTime, DefaultValue: "NOW()+MOD(MINUTE(TIMEVALUE(NOW())),15)/1440"},
		"Minute__c":   {APIName: "Minute__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", DefaultValue: "MINUTE(TIMEVALUE(NOW()))"},
		"Tomorrow__c": {APIName: "Tomorrow__c", Type: storage.FieldDate, DefaultValue: "TODAY()+1"},
		"Number__c":   {APIName: "Number__c", Type: storage.FieldDecimal, DefaultValue: "1+2*3"},
		"Blank__c":    {APIName: "Blank__c", Type: storage.FieldDateTime},
	}}
	org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{}}
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{Object: def.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("probe")}}})
	if len(result) != 1 || !result[0].Success {
		t.Fatalf("insert: %+v", result)
	}
	record := org.Objects[def.APIName].Records[result[0].ID]
	for name, want := range map[string]string{"Moment__c": "2026-05-15T10:07:00Z", "Offset__c": "2026-05-15T10:14:00Z", "Minute__c": "7", "Tomorrow__c": "2026-05-16", "Number__c": "7"} {
		got, ok := record.Fields[name]
		actual := got.String
		if got.Kind == storage.ValueDecimal {
			actual = got.Decimal
		}
		if name == "Minute__c" || name == "Number__c" {
			n, valid := new(big.Rat).SetString(actual)
			if ok && valid && n.Cmp(big.NewRat(7, 1)) == 0 {
				continue
			}
		}
		if !ok || actual != want {
			t.Errorf("%s: got %+v; want %s", name, got, want)
		}
	}
	if value, ok := record.Fields["Blank__c"]; ok && value.Kind != storage.ValueNull {
		t.Errorf("blank: %+v", value)
	}
}
