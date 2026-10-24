package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestJournalResetReusesGenerationAfterNoopStandardEnsure(t *testing.T) {
	template := runtimeGenerationIndexTemplate(32)
	storage.EnsureStandardObject(&template.Org, "PermissionSetAssignment")
	PrimeRuntimeTemplateSchema(&template)
	org := template.CloneRuntimeOrg()
	journal := storage.NewIsolationJournal(&org)
	wantGeneration := org.RuntimeGenerationID()
	before := jsonChildRelationshipIndexBuilds.Load()
	for i := range 4 {
		mark := journal.Mark()
		machine := New(nil)
		machine.PrimeMetadataSchema(&org)
		machine.SetRuntimeTemplateOrg(&org)
		if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); !ok || got != "List<Child000__c>" {
			t.Fatalf("case %d relationship = %q, %v", i, got, ok)
		}
		storage.EnsureStandardObject(&org, "PermissionSetAssignment")
		if err := journal.Rollback(mark); err != nil {
			t.Fatal(err)
		}
	}
	if got := jsonChildRelationshipIndexBuilds.Load() - before; got != 1 {
		t.Errorf("journal reset JSON index builds = %d, want 1", got)
	}
	if got := org.RuntimeGenerationID(); got != wantGeneration {
		t.Errorf("journal reset generation = %d, want %d", got, wantGeneration)
	}
}

func BenchmarkJournalResetSchemaGeneration(b *testing.B) {
	template := runtimeGenerationIndexTemplate(512)
	storage.EnsureStandardObject(&template.Org, "PermissionSetAssignment")
	PrimeRuntimeTemplateSchema(&template)
	org := template.CloneRuntimeOrg()
	journal := storage.NewIsolationJournal(&org)
	run := func() {
		mark := journal.Mark()
		machine := New(nil)
		machine.PrimeMetadataSchema(&org)
		machine.SetRuntimeTemplateOrg(&org)
		if _, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); !ok {
			b.Fatal("missing relationship")
		}
		storage.EnsureStandardObject(&org, "PermissionSetAssignment")
		if err := journal.Rollback(mark); err != nil {
			b.Fatal(err)
		}
	}
	run()
	before := jsonChildRelationshipIndexBuilds.Load()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
	b.StopTimer()
	b.ReportMetric(float64(jsonChildRelationshipIndexBuilds.Load()-before)/float64(b.N), "json-index-builds/op")
}
