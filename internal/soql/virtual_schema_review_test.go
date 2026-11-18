package soql

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Query shapes over the virtual schema objects: plain, one-hop parents in
// SELECT/WHERE/ORDER/GROUP, functions, aggregates, a child query, a subquery
// and a two-hop path. The last three must fall back to every table.
var virtualSchemaReviewQueries = []struct {
	text   string
	tables []string // nil means every table
}{
	{"SELECT QualifiedApiName, Label FROM EntityDefinition WHERE QualifiedApiName IN ('Account', 'Probe__c') ORDER BY QualifiedApiName", []string{"EntityDefinition"}},
	{"SELECT COUNT() FROM EntityDefinition WHERE QualifiedApiName LIKE 'Probe%'", []string{"EntityDefinition"}},
	{"SELECT QualifiedApiName, EntityDefinition.QualifiedApiName, EntityDefinition.Label FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'Probe__c' ORDER BY EntityDefinition.QualifiedApiName, QualifiedApiName", []string{"FieldDefinition", "EntityDefinition"}},
	{"SELECT toLabel(QualifiedApiName) name, DataType FROM FieldDefinition WHERE EntityDefinitionId = 'Probe__c' ORDER BY QualifiedApiName", []string{"FieldDefinition"}},
	{"SELECT EntityDefinition.QualifiedApiName entity, COUNT(DurableId) total FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName IN ('Account', 'Probe__c') GROUP BY EntityDefinition.QualifiedApiName ORDER BY EntityDefinition.QualifiedApiName", []string{"FieldDefinition", "EntityDefinition"}},
	{"SELECT DurableId, FieldDefinition.QualifiedApiName, EntityDefinition.QualifiedApiName FROM EntityParticle WHERE EntityDefinitionId = 'Probe__c' ORDER BY DurableId", []string{"EntityParticle", "FieldDefinition", "EntityDefinition"}},
	{"SELECT DurableId, IsAccessible FROM UserFieldAccess WHERE EntityDefinitionId = 'Probe__c' ORDER BY DurableId", []string{"UserFieldAccess"}},
	{"SELECT EntityDefinitionId, IsCreatable FROM UserEntityAccess WHERE EntityDefinitionId = 'Probe__c'", []string{"UserEntityAccess"}},
	{"SELECT DurableId, ParentSobjectId, Field.QualifiedApiName FROM RelationshipDomain WHERE ChildSobjectId = 'Probe__c' ORDER BY DurableId", []string{"RelationshipDomain", "FieldDefinition"}},
	{"SELECT QualifiedApiName, (SELECT QualifiedApiName FROM Fields ORDER BY QualifiedApiName) FROM EntityDefinition WHERE QualifiedApiName = 'Probe__c'", nil},
	{"SELECT QualifiedApiName FROM FieldDefinition WHERE EntityDefinitionId IN (SELECT QualifiedApiName FROM EntityDefinition WHERE QualifiedApiName = 'Probe__c') ORDER BY QualifiedApiName", nil},
	{"SELECT DurableId, FieldDefinition.EntityDefinition.QualifiedApiName FROM EntityParticle WHERE EntityDefinitionId = 'Probe__c' ORDER BY DurableId", nil},
}

func virtualSchemaReviewTables(t *testing.T, cache *ExecutionCache) []string {
	t.Helper()
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if cache.virtualSchema == nil {
		return nil
	}
	var names []string
	for name := range cache.virtualSchema.tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Each query hydrates only the tables it can read, answers exactly as full
// private hydration, and a later query adds tables without rebuilding earlier
// ones.
func TestVirtualSchemaSharedViewHydratesOnlyReachableTables(t *testing.T) {
	frozen := virtualSchemaSharedTestOrg(t)
	frozen.RuntimeSchemaStamp = "generation-1"
	private := virtualSchemaSharedTestOrg(t)
	for _, tc := range virtualSchemaReviewQueries {
		t.Run(tc.text[:40], func(t *testing.T) {
			query, err := Parse(tc.text)
			if err != nil {
				t.Fatal(err)
			}
			cache := NewExecutionCache()
			want, wantErr := ExecuteWithCache(private, query, NewExecutionCache())
			got, gotErr := ExecuteWithCache(frozen, query, cache)
			if (wantErr == nil) != (gotErr == nil) || (wantErr != nil && wantErr.Error() != gotErr.Error()) {
				t.Fatalf("shared error %v, private error %v", gotErr, wantErr)
			}
			if wantErr == nil {
				if got.Rows != want.Rows || !reflect.DeepEqual(sortedRecords(got), sortedRecords(want)) {
					t.Fatalf("shared %d %+v\nprivate %d %+v", got.Rows, sortedRecords(got), want.Rows, sortedRecords(want))
				}
				if want.Rows == 0 && !queryHasAggregates(query) {
					t.Fatal("query matched no rows; the fixture does not exercise it")
				}
			} else {
				t.Logf("both paths reject the query: %v", wantErr)
			}
			expected := tc.tables
			if expected == nil {
				expected = append([]string(nil), virtualSchemaObjectNames[:]...)
			}
			sort.Strings(expected)
			if hydrated := virtualSchemaReviewTables(t, cache); !reflect.DeepEqual(hydrated, expected) {
				t.Fatalf("hydrated tables %v, want %v", hydrated, expected)
			}
		})
	}
	// Tables accumulate across queries on one cache; earlier tables are kept.
	cache := NewExecutionCache()
	before := virtualSchemaBuilds.Load()
	for _, text := range []string{virtualSchemaReviewQueries[0].text, virtualSchemaReviewQueries[2].text, virtualSchemaReviewQueries[0].text, virtualSchemaReviewQueries[2].text} {
		query, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ExecuteWithCache(frozen, query, cache); err != nil {
			t.Fatal(err)
		}
	}
	if got := virtualSchemaReviewTables(t, cache); !reflect.DeepEqual(got, []string{"EntityDefinition", "FieldDefinition"}) {
		t.Fatalf("accumulated tables %v", got)
	}
	if builds := virtualSchemaBuilds.Load() - before; builds != 2 {
		t.Fatalf("four queries over two tables built %d tables, want 2", builds)
	}
}

// Concurrent first queries of a generation wait for one build per table
// instead of each hydrating its own copy.
func TestVirtualSchemaSharedViewBuildsEachTableOnceUnderConcurrency(t *testing.T) {
	frozen := virtualSchemaSharedTestOrg(t)
	frozen.RuntimeSchemaStamp = "generation-1"
	query, err := Parse(virtualSchemaReviewQueries[2].text)
	if err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 5; round++ {
		cache := NewExecutionCache()
		before := virtualSchemaBuilds.Load()
		var wg sync.WaitGroup
		errs := make(chan error, 16)
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, err := ExecuteWithCache(frozen, query, cache)
				if err != nil || result.Rows == 0 {
					errs <- fmt.Errorf("rows %d err %v", result.Rows, err)
				}
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
		if builds := virtualSchemaBuilds.Load() - before; builds != 2 {
			t.Fatalf("round %d: 16 concurrent first queries built %d tables, want 2 (FieldDefinition and EntityDefinition)", round, builds)
		}
	}
}

// A callback that mutates the record it is handed changes nothing in the
// shared view: later and concurrent queries keep their rows.
func TestVirtualSchemaSharedViewCallbacksReceivePrivateRecords(t *testing.T) {
	frozen := virtualSchemaSharedTestOrg(t)
	frozen.RuntimeSchemaStamp = "generation-1"
	private := virtualSchemaSharedTestOrg(t)
	cache := NewExecutionCache()
	windowed, err := Parse("SELECT QualifiedApiName, Label, EntityDefinition.Label FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'Probe__c' ORDER BY QualifiedApiName LIMIT 3")
	if err != nil {
		t.Fatal(err)
	}
	child, err := Parse("SELECT QualifiedApiName, (SELECT QualifiedApiName, Label FROM Fields ORDER BY QualifiedApiName) FROM EntityDefinition WHERE QualifiedApiName = 'Probe__c'")
	if err != nil {
		t.Fatal(err)
	}
	poison := func(objectName string, record storage.Record) bool {
		record.Fields["Label"] = storage.StringValue("poisoned")
		record.Fields["QualifiedApiName"] = storage.StringValue("poisoned")
		delete(record.Fields, "DurableId")
		record.Fields["Injected__c"] = storage.StringValue("x")
		return true
	}
	context := ExecutionContext{WindowRecordVisible: poison, ChildRecordVisible: poison}
	check := func(label string) {
		for _, query := range []Query{windowed, child} {
			want, err := ExecuteWithCacheAndContext(private, query, NewExecutionCache(), ExecutionContext{})
			if err != nil {
				t.Fatal(err)
			}
			got, err := ExecuteWithCacheAndContext(frozen, query, cache, ExecutionContext{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(sortedRecords(got), sortedRecords(want)) {
				t.Fatalf("%s: shared rows changed:\n shared  %+v\n private %+v", label, sortedRecords(got), sortedRecords(want))
			}
		}
	}
	for round := 0; round < 3; round++ {
		for _, query := range []Query{windowed, child} {
			if _, err := ExecuteWithCacheAndContext(frozen, query, cache, context); err != nil {
				t.Fatal(err)
			}
		}
		check(fmt.Sprintf("after sequential round %d", round))
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				for _, query := range []Query{windowed, child} {
					if _, err := ExecuteWithCacheAndContext(frozen, query, cache, context); err != nil {
						t.Error(err)
						return
					}
					if _, err := ExecuteWithCacheAndContext(frozen, query, cache, ExecutionContext{}); err != nil {
						t.Error(err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	check("after concurrent callbacks")
}

// A shallow org copy marks every copied state shared, so the copy-on-write
// helper clones a record map before a writer touches it.
func TestCloneObjectsShallowMarksCopiedStatesShared(t *testing.T) {
	org := virtualSchemaSharedTestOrg(t)
	probe := org.Objects["Probe__c"]
	probe.Records["a01000000000001"] = storage.Record{ID: "a01000000000001", Object: "Probe__c", Fields: map[string]storage.Value{"Name": storage.StringValue("one")}}
	org.Objects["Probe__c"] = probe
	view := org.CloneObjectsShallow(0)
	state, cloned := storage.EnsureMutableObjectRecords(&view, "Probe__c")
	if !cloned {
		t.Fatal("copied state was not marked shared")
	}
	state.Records["a01000000000001"] = storage.Record{ID: "a01000000000001", Object: "Probe__c", Fields: map[string]storage.Value{"Name": storage.StringValue("changed")}}
	if got := org.Objects["Probe__c"].Records["a01000000000001"].Fields["Name"].String; got != "one" {
		t.Fatalf("write through the view reached the receiver: %q", got)
	}
}
