package storage

import (
	"fmt"
	"testing"
)

func BenchmarkOrgStateCloneRuntime(b *testing.B) {
	org := benchmarkOrgState(60, 450)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloned := org.CloneRuntime()
		if len(cloned.Objects) == 0 {
			b.Fatal("expected cloned objects")
		}
	}
}

func BenchmarkOrgStateCloneRollbackSnapshot(b *testing.B) {
	org := benchmarkOrgState(60, 450)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloned := org.CloneRollbackSnapshot()
		if len(cloned.Objects) == 0 {
			b.Fatal("expected cloned objects")
		}
	}
}

func BenchmarkOrgStateClone(b *testing.B) {
	org := benchmarkOrgState(60, 450)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloned := org.Clone()
		if len(cloned.Objects) == 0 {
			b.Fatal("expected cloned objects")
		}
	}
}

func BenchmarkEnsureStandardObjectFields(b *testing.B) {
	def := ObjectDefinition{APIName: "Account"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EnsureStandardObjectFields(&def)
	}
}

func BenchmarkEnsureStandardObjectFieldsFreshDefinition(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		def := ObjectDefinition{APIName: "Account"}
		EnsureStandardObjectFields(&def)
	}
}

func benchmarkOrgState(objectCount, recordsPerObject int) OrgState {
	org := NewOrgState()
	for i := 0; i < objectCount; i++ {
		objectName := fmt.Sprintf("PerfObject%d__c", i)
		definition := ObjectDefinition{
			APIName: objectName,
			Fields: map[string]Field{
				"Id":        {APIName: "Id", Type: FieldID},
				"Name":      {APIName: "Name", Type: FieldString},
				"Lookup__c": {APIName: "Lookup__c", Type: FieldReference, ReferenceTo: []string{"PerfObject0__c"}},
				"Flag__c":   {APIName: "Flag__c", Type: FieldBoolean},
				"Score__c":  {APIName: "Score__c", Type: FieldInteger},
			},
			Indexes: []IndexDefinition{
				{Name: objectName + ".Name", Object: objectName, Fields: []string{"Name"}},
				{Name: objectName + ".Lookup", Object: objectName, Fields: []string{"Lookup__c"}},
			},
			Metadata: map[string]string{
				"kind": "customObject",
			},
		}
		state := ObjectState{
			Definition: definition,
			Records:    make(map[ID]Record, recordsPerObject),
		}
		for r := 0; r < recordsPerObject; r++ {
			id := ID(fmt.Sprintf("a%014d", i*recordsPerObject+r))
			state.Records[id] = Record{
				ID:     id,
				Object: objectName,
				Fields: map[string]Value{
					"Id":        IDValue(id),
					"Name":      StringValue(fmt.Sprintf("row-%d-%d", i, r)),
					"Lookup__c": IDValue(ID("a000000000000000")),
					"Flag__c":   BooleanValue((r % 2) == 0),
					"Score__c":  IntegerValue(int64(r)),
				},
			}
		}
		org.Objects[objectName] = state
	}
	RebuildIndexes(&org)
	return org
}
