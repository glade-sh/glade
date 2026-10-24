package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func runtimeGenerationIndexTemplate(objectCount int) storage.RuntimeTemplate {
	org := storage.NewOrgState()
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
	}}
	for i := 0; i < objectCount; i++ {
		childName := fmt.Sprintf("Child%03d__c", i)
		fieldName := fmt.Sprintf("Parent%03d__c", i)
		relationshipName := fmt.Sprintf("Children%03d__r", i)
		org.Objects[childName] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: childName,
			Fields: map[string]storage.Field{
				fieldName: {
					APIName: fieldName, Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"},
					RelationshipName: "Parent__r", ChildRelationshipName: relationshipName,
				},
			},
		}}
	}
	storage.EnsureUniqueKeyPrefixes(&org)
	template := storage.NewRuntimeTemplate(org)
	PrimeRuntimeTemplateSchema(&template)
	return template
}

func runtimeGenerationIndexVM(template *storage.RuntimeTemplate) *VM {
	machine := New(nil)
	machine.PrimeMetadataSchema(&template.Org)
	org := template.CloneRuntimeOrg()
	machine.SetRuntimeTemplateOrg(&org)
	return machine
}

func TestRuntimeTemplateClonesReuseJSONChildRelationshipIndex(t *testing.T) {
	template := runtimeGenerationIndexTemplate(8)
	before := jsonChildRelationshipIndexBuilds.Load()
	for i := 0; i < 2; i++ {
		machine := runtimeGenerationIndexVM(&template)
		got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children000__r")
		if !ok || got != "List<Child000__c>" {
			t.Fatalf("clone %d child relationship type = %q, %v", i, got, ok)
		}
	}
	if got := jsonChildRelationshipIndexBuilds.Load() - before; got != 1 {
		t.Fatalf("frozen org clone JSON index builds = %d, want 1", got)
	}
}

func TestRuntimeTemplateClonesReuseSchemaStamp(t *testing.T) {
	template := storage.NewRuntimeTemplate(schemaStampMemoTestOrg())
	PrimeRuntimeTemplateSchema(&template)
	before := schemaStampObjectsHashed.Load()
	for i := 0; i < 3; i++ {
		clone := template.CloneRuntimeOrg()
		if got := schemaCacheStampForOrg(&clone); got != template.RuntimeSchemaStamp {
			t.Fatalf("clone %d schema stamp = %q, want %q", i, got, template.RuntimeSchemaStamp)
		}
	}
	if got := schemaStampObjectsHashed.Load() - before; got != 0 {
		t.Fatalf("frozen org clone schema stamp object hashes = %d, want 0", got)
	}
}

func TestSetOrgInvalidationPreservesSiblingGenerationIndexes(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprintf("template_fallback=%v", fallback), func(t *testing.T) {
			template := runtimeGenerationIndexTemplate(8)
			first := runtimeGenerationIndexVM(&template)
			if _, ok := first.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); !ok {
				t.Fatal("missing initial child relationship")
			}
			wantID := template.Org.RuntimeGenerationID()
			before := jsonChildRelationshipIndexBuilds.Load()
			clone := template.CloneRuntimeOrg()
			machine := New(nil)
			if fallback {
				machine.SetRuntimeTemplateOrg(&clone)
			} else {
				machine.SetOrg(&clone)
			}
			if clone.RuntimeGenerationID() == wantID {
				t.Error("SetOrg did not detach the installed clone's generation")
			}
			if got := template.Org.RuntimeGenerationID(); got != wantID {
				t.Errorf("SetOrg changed template generation: got %d, want %d", got, wantID)
			}
			sibling := runtimeGenerationIndexVM(&template)
			if got, ok := sibling.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); !ok || got != "List<Child000__c>" {
				t.Fatalf("sibling child relationship type = %q, %v", got, ok)
			}
			if got := jsonChildRelationshipIndexBuilds.Load() - before; got != 0 {
				t.Errorf("sibling JSON index rebuilds after SetOrg = %d, want 0", got)
			}
		})
	}
}

func BenchmarkRuntimeGenerationIndexesAcrossVMClones(b *testing.B) {
	template := runtimeGenerationIndexTemplate(512)
	run := func() {
		machine := runtimeGenerationIndexVM(&template)
		org := machine.Org
		storage.EnsureUniqueKeyPrefixes(org)
		if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); !ok || got != "List<Child000__c>" {
			b.Fatalf("child relationship type = %q, %v", got, ok)
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
	builds := jsonChildRelationshipIndexBuilds.Load() - before
	b.ReportMetric(float64(builds)/float64(b.N), "json-index-builds/op")
}
