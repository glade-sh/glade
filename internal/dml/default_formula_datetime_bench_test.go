package dml

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// Benchmark the insert-time default pass with the captured formula and a
// representative group of additional fields. The digest excludes only the
// two defaults whose results the captured fix changes.
func BenchmarkDefaultFormulaDatetime(b *testing.B) {
	org := storage.NewOrgState()
	org.Now = func() time.Time { return time.Date(2026, 5, 15, 10, 7, 0, 0, time.UTC) }
	fields := map[string]storage.Field{
		"Moment__c":   {APIName: "Moment__c", Type: storage.FieldDateTime, DefaultValue: "NOW()"},
		"Offset__c":   {APIName: "Offset__c", Type: storage.FieldDateTime, DefaultValue: "NOW()+MOD(MINUTE(TIMEVALUE(NOW())),15)/1440"},
		"Minute__c":   {APIName: "Minute__c", Type: storage.FieldDecimal, DefaultValue: "MINUTE(TIMEVALUE(NOW()))"},
		"Tomorrow__c": {APIName: "Tomorrow__c", Type: storage.FieldDate, DefaultValue: "TODAY()+1"},
		"Number__c":   {APIName: "Number__c", Type: storage.FieldDecimal, DefaultValue: "1+2*3"},
		"Blank__c":    {APIName: "Blank__c", Type: storage.FieldDateTime},
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("Extra%02d__c", i)
		fields[name] = storage.Field{APIName: name, Type: storage.FieldString, DefaultValue: "'value'"}
	}
	definition := storage.ObjectDefinition{APIName: "DefaultProbe__c", Fields: fields}
	var record storage.Record
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		record = storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{}}
		applyFieldDefaults(&org, definition, &record, "", nil)
	}
	b.StopTimer()
	_, offset := record.Fields["Offset__c"]
	_, minute := record.Fields["Minute__c"]
	delete(record.Fields, "Offset__c")
	delete(record.Fields, "Minute__c")
	encoded, err := json.Marshal(record.Fields)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("unchanged fields sha256=%x; offset=%t minute=%t", sha256.Sum256(encoded), offset, minute)
}
