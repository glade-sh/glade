package soql

import (
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func virtualSchemaSharedTestOrg(t *testing.T) storage.OrgState {
	t.Helper()
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	org.Objects["Probe__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Probe__c", Label: "Probe", Fields: map[string]storage.Field{
			"Name":        {APIName: "Name", Type: storage.FieldString, Label: "Probe Name"},
			"Account__c":  {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
			"Amount__c":   {APIName: "Amount__c", Type: storage.FieldDecimal, Length: 18},
			"Computed__c": {APIName: "Computed__c", Type: storage.FieldCalculated, Formula: "Amount__c * 2"},
		}, Relations: []storage.Relationship{{Field: "Account__c", ParentObjects: []string{"Account"}, ParentRelationship: "Account__r", ChildRelationship: "Probes__r"}}},
		Records: map[storage.ID]storage.Record{},
	}
	return org
}

var virtualSchemaSharedQueries = []string{
	"SELECT QualifiedApiName, Label, IsCustomizable FROM EntityDefinition WHERE QualifiedApiName IN ('Account', 'Probe__c') ORDER BY QualifiedApiName",
	"SELECT QualifiedApiName, DataType, IsCalculated, Length FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'Probe__c' ORDER BY QualifiedApiName",
	"SELECT DurableId, QualifiedApiName, EntityDefinitionId FROM EntityParticle WHERE EntityDefinitionId = 'Probe__c' ORDER BY QualifiedApiName",
	"SELECT DurableId, IsAccessible FROM UserFieldAccess WHERE EntityDefinitionId = 'Probe__c' ORDER BY DurableId",
	"SELECT EntityDefinitionId, IsCreatable FROM UserEntityAccess WHERE EntityDefinitionId = 'Probe__c'",
	"SELECT DurableId, ParentSobjectId, ChildSobjectId, RelationshipName FROM RelationshipDomain WHERE ChildSobjectId = 'Probe__c' ORDER BY DurableId",
	"SELECT COUNT() FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'Contact'",
}

func sortedRecords(result Result) []storage.Record {
	records := append([]storage.Record(nil), result.Records...)
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}

// A frozen generation's virtual schema queries answer exactly as the private
// per-query hydration, for every virtual object, and leave the caller's org
// without virtual objects.
func TestVirtualSchemaSharedViewMatchesPrivateHydration(t *testing.T) {
	frozen := virtualSchemaSharedTestOrg(t)
	frozen.RuntimeSchemaStamp = "generation-1"
	private := virtualSchemaSharedTestOrg(t)
	cache := NewExecutionCache()
	for round := 0; round < 2; round++ {
		for _, text := range virtualSchemaSharedQueries {
			query, err := Parse(text)
			if err != nil {
				t.Fatal(err)
			}
			want, err := ExecuteWithCache(private, query, NewExecutionCache())
			if err != nil {
				t.Fatalf("%s: %v", text, err)
			}
			got, err := ExecuteWithCache(frozen, query, cache)
			if err != nil {
				t.Fatalf("%s: %v", text, err)
			}
			if got.Rows != want.Rows || !reflect.DeepEqual(sortedRecords(got), sortedRecords(want)) {
				t.Fatalf("round %d %s:\n shared  %d %+v\n private %d %+v", round, text, got.Rows, sortedRecords(got), want.Rows, sortedRecords(want))
			}
			if want.Rows == 0 && !queryHasAggregates(query) {
				t.Fatalf("%s matched no rows; the fixture does not exercise it", text)
			}
		}
		if cache.virtualSchema == nil || cache.virtualSchema.runtimeStamp != "generation-1" {
			t.Fatal("frozen generation did not keep its shared virtual schema")
		}
	}
	for _, name := range virtualSchemaObjectNames {
		if _, ok := frozen.Objects[name]; ok {
			t.Fatalf("shared view added %s to the caller org", name)
		}
	}
	if resolved, ok := storage.ResolveObjectName(frozen, "EntityDefinition"); ok {
		t.Fatalf("caller org resolves EntityDefinition to %q after a shared query", resolved)
	}
}

// The shared view is built once per generation: a repeated query hydrates
// nothing, while the private path hydrates every time.
func TestVirtualSchemaSharedViewHydratesOncePerGeneration(t *testing.T) {
	frozen := virtualSchemaSharedTestOrg(t)
	frozen.RuntimeSchemaStamp = "generation-1"
	private := virtualSchemaSharedTestOrg(t)
	query, err := Parse(virtualSchemaSharedQueries[1])
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	if _, err := ExecuteWithCache(frozen, query, cache); err != nil {
		t.Fatal(err)
	}
	shared := testing.AllocsPerRun(20, func() {
		if _, err := ExecuteWithCache(frozen, query, cache); err != nil {
			t.Fatal(err)
		}
	})
	privateAllocs := testing.AllocsPerRun(20, func() {
		if _, err := ExecuteWithCache(private, query, NewExecutionCache()); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("allocations per FieldDefinition query: shared view %.0f, private hydration %.0f", shared, privateAllocs)
	if shared*2 > privateAllocs {
		t.Fatalf("shared view allocated %.0f against %.0f for private hydration; the generation must hydrate once", shared, privateAllocs)
	}
}

// Explicit records for a virtual object, an org without a runtime stamp, a
// changed stamp and a changed schema all leave the shared view alone or
// rebuild it.
func TestVirtualSchemaSharedViewFallsBackAndInvalidates(t *testing.T) {
	query, err := Parse("SELECT QualifiedApiName FROM EntityDefinition WHERE QualifiedApiName LIKE 'Probe%' ORDER BY QualifiedApiName")
	if err != nil {
		t.Fatal(err)
	}
	names := func(result Result) []string {
		var out []string
		for _, record := range result.Records {
			out = append(out, record.Fields["QualifiedApiName"].String)
		}
		return out
	}

	unstamped := virtualSchemaSharedTestOrg(t)
	cache := NewExecutionCache()
	if _, err := ExecuteWithCache(unstamped, query, cache); err != nil {
		t.Fatal(err)
	}
	if cache.virtualSchema != nil {
		t.Fatal("an org without a runtime stamp used the shared view")
	}

	explicit := virtualSchemaSharedTestOrg(t)
	explicit.RuntimeSchemaStamp = "generation-1"
	storage.EnsureStandardObject(&explicit, "EntityDefinition")
	state := explicit.Objects["EntityDefinition"]
	state.Records = map[storage.ID]storage.Record{"ProbeExplicit__c": {ID: "ProbeExplicit__c", Object: "EntityDefinition", Fields: map[string]storage.Value{"QualifiedApiName": storage.StringValue("ProbeExplicit__c")}}}
	explicit.Objects["EntityDefinition"] = state
	result, err := ExecuteWithCache(explicit, query, cache)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(result); !reflect.DeepEqual(got, []string{"Probe__c", "ProbeExplicit__c"}) {
		t.Fatalf("explicit records: %v", got)
	}
	if cache.virtualSchema != nil {
		t.Fatal("an org with explicit virtual records used the shared view")
	}

	frozen := virtualSchemaSharedTestOrg(t)
	frozen.RuntimeSchemaStamp = "generation-1"
	if result, err = ExecuteWithCache(frozen, query, cache); err != nil {
		t.Fatal(err)
	}
	if got := names(result); !reflect.DeepEqual(got, []string{"Probe__c"}) {
		t.Fatalf("frozen: %v", got)
	}
	first := cache.virtualSchema
	if first == nil {
		t.Fatal("frozen generation did not build the shared view")
	}
	// A new object in the same generation changes the hydration stamp.
	frozen.Objects["Probe2__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe2__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{}}
	if result, err = ExecuteWithCache(frozen, query, cache); err != nil {
		t.Fatal(err)
	}
	if got := names(result); !reflect.DeepEqual(got, []string{"Probe2__c", "Probe__c"}) {
		t.Fatalf("after adding an object: %v", got)
	}
	if cache.virtualSchema == first {
		t.Fatal("a changed schema reused the shared view")
	}
	// A new generation rebuilds, a cleared stamp hydrates privately.
	second := cache.virtualSchema
	frozen.RuntimeSchemaStamp = "generation-2"
	if _, err = ExecuteWithCache(frozen, query, cache); err != nil {
		t.Fatal(err)
	}
	if cache.virtualSchema == second || cache.virtualSchema.runtimeStamp != "generation-2" {
		t.Fatal("a new generation did not rebuild the shared view")
	}
	third := cache.virtualSchema
	frozen.ClearRuntimeSchemaStamp()
	if result, err = ExecuteWithCache(frozen, query, cache); err != nil {
		t.Fatal(err)
	}
	if got := names(result); !reflect.DeepEqual(got, []string{"Probe2__c", "Probe__c"}) || cache.virtualSchema != third {
		t.Fatalf("cleared stamp: %v (shared view replaced: %v)", got, cache.virtualSchema != third)
	}
}

// Concurrent queries on one generation share one view without racing.
func TestVirtualSchemaSharedViewConcurrentQueries(t *testing.T) {
	frozen := virtualSchemaSharedTestOrg(t)
	frozen.RuntimeSchemaStamp = "generation-1"
	cache := NewExecutionCache()
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, text := range virtualSchemaSharedQueries[:4] {
				query, err := Parse(text)
				if err != nil {
					errs <- err
					return
				}
				if result, err := ExecuteWithCache(frozen, query, cache); err != nil || result.Rows == 0 {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent query: %v", err)
	}
}
