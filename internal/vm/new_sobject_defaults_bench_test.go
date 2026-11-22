package vm_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Measures newSObject(recordTypeId, true) on an object with ordinary string,
// date and arithmetic defaults.
func BenchmarkNewSObjectLoadDefaults(b *testing.B) {
	org := storage.NewOrgState()
	org.Now = func() time.Time { return time.Date(2026, 5, 15, 10, 7, 0, 0, time.UTC) }
	generator := storage.NewStandardIDGenerator()
	rtID, err := generator.Next("RecordType")
	if err != nil {
		b.Fatal(err)
	}
	fields := map[string]storage.Field{
		"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
		"Tomorrow__c":  {APIName: "Tomorrow__c", Type: storage.FieldDate, DefaultValue: "TODAY()+1"},
		"Number__c":    {APIName: "Number__c", Type: storage.FieldDecimal, DefaultValue: "1+2*3"},
		"Moment__c":    {APIName: "Moment__c", Type: storage.FieldDateTime, DefaultValue: "NOW()"},
		"Offset__c":    {APIName: "Offset__c", Type: storage.FieldDateTime, DefaultValue: "NOW()+(30-MOD(MINUTE(TIMEVALUE(NOW())),30))/1440"},
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("Extra%02d__c", i)
		fields[name] = storage.Field{APIName: name, Type: storage.FieldString, DefaultValue: "'value'"}
	}
	definition := storage.ObjectDefinition{APIName: "DefaultProbe__c", KeyPrefix: "a01", Fields: fields,
		RecordTypes: []storage.RecordTypeInfo{{ID: rtID, Active: true, Available: true, Default: true}}}
	org.Objects[definition.APIName] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	source := fmt.Sprintf("for (Integer i = 0; i < 200; i++) { SObject row = DefaultProbe__c.SObjectType.newSObject('%s', true); }", rtID)
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "62.0"})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine := vm.New(nil)
		machine.SetOrg(&org)
		if _, err := machine.Execute(program); err != nil {
			b.Fatal(err)
		}
	}
}
