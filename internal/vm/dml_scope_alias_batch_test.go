package vm

import (
	"fmt"
	"maps"
	"reflect"
	"strings"
	"testing"
)

func TestDMLScopeAliasBatchVisitsEachCallerRootOnce(t *testing.T) {
	for _, rowCount := range []int{2, 63, 64, 65, 200} {
		t.Run(fmt.Sprintf("rows=%d", rowCount), func(t *testing.T) {
			machine := New(nil)
			scope := make(map[string]Value, 8)
			for i := range 8 {
				scope[fmt.Sprintf("root%d", i)] = Object("Account")
			}
			mutations := make([]dmlScopeAliasMutation, 0, rowCount)
			for i := range rowCount {
				row := Object("Account")
				row.Fields["Name"] = String(fmt.Sprintf("row-%d", i))
				updated := row
				updated.Fields = map[string]Value{"Name": row.Fields["Name"]}
				updated.Fields["Id"] = platformScalar("Id", fmt.Sprintf("001000000000%03dAAA", i))
				holder := Object("Account")
				holder.Fields["Row"] = row
				scope[fmt.Sprintf("holder%d", i)] = holder
				mutations = append(mutations, dmlScopeAliasMutation{
					previous: snapshotAlias(row),
					updated:  updated,
				})
			}
			observer := &task31ScopeAliasTraversalObserver{}
			machine.scopeAliasTraversalObserver = observer
			if !machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(scope, mutations) {
				t.Fatalf("batch rejected %d row mutations", rowCount)
			}
			if want := uint64(len(scope)); observer.rootVisits != want {
				t.Fatalf("caller roots visited = %d, want %d (each root once)", observer.rootVisits, want)
			}
		})
	}
}

func TestDMLScopeAliasBatchMatchesLegacyPropagation(t *testing.T) {
	for _, rowCount := range []int{1, 2, 65, 200} {
		t.Run(fmt.Sprintf("rows=%d", rowCount), func(t *testing.T) {
			baselineScope, mutations := dmlScopeAliasBatchFixture(rowCount)
			gotScope := cloneDMLScopeAliasBatchScope(baselineScope)
			machine := New(nil)
			machine.Globals = gotScope
			machine.propagateSuccessfulDMLTargetAliases(mutations)

			referenceScope := cloneDMLScopeAliasBatchScope(baselineScope)
			reference := New(nil)
			reference.Globals = referenceScope
			for _, mutation := range mutations {
				updated := mutation.updated
				updated.Fields = maps.Clone(updated.Fields)
				reference.propagateAliasSnapshotToScope(reference.Globals, mutation.previous, updated)
				reference.propagateAliasSnapshotToStatics(mutation.previous, updated)
			}
			if !reflect.DeepEqual(gotScope, referenceScope) {
				t.Fatalf("batch result differs from legacy propagation for %d rows", rowCount)
			}
		})
	}
}

func TestDMLScopeAliasBatchRejectsTopLevelAlias(t *testing.T) {
	row := Object("Account")
	row.Ref = 42
	updated := row
	updated.Fields = maps.Clone(row.Fields)
	updated.Fields["Id"] = platformScalar("Id", "001000000000001AAA")
	scope := map[string]Value{"target": row}
	before := cloneValuePreserveRefs(row)
	mutations := []dmlScopeAliasMutation{{previous: snapshotAlias(row), updated: updated}}
	machine := New(nil)
	observer := &dmlScopeAliasReasonObserver{}
	machine.scopeAliasTraversalObserver = observer
	if machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(scope, mutations) {
		t.Fatal("single mutation should not use batch entrypoint directly")
	}
	other := Object("Account")
	mutations = append(mutations, dmlScopeAliasMutation{previous: snapshotAlias(other), updated: other})
	if machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(scope, mutations) {
		t.Fatal("batch accepted a top-level caller alias target")
	}
	if observer.reason != "top-level caller alias" {
		t.Fatalf("fallback = %q, want top-level caller alias", observer.reason)
	}
	if !reflect.DeepEqual(scope["target"], before) {
		t.Fatal("rejection published an update")
	}
}

func TestDMLScopeAliasBatchRecordsTraversalCounter(t *testing.T) {
	recorder := NewPerfRecorder()
	machine := New(nil)
	machine.SetPerfRecorder(recorder)
	scope, mutations := dmlScopeAliasBatchFixture(5)
	machine.Globals = cloneDMLScopeAliasBatchScope(scope)
	machine.propagateSuccessfulDMLTargetAliases(mutations)
	if got := recorder.Snapshot().DML; got.ScopeAliasBatchCalls != 1 || got.ScopeAliasBatchRows != 5 {
		t.Fatalf("DML batch counters = %#v, want one batch over five rows", got)
	}
	_ = scope
}

func TestExecBulkDMLScopeAliasBatchProjectRoute(t *testing.T) {
	const rowCount = 210
	var builder strings.Builder
	builder.WriteString("List<Account> rows = new List<Account>();\n")
	for i := 0; i < rowCount; i++ {
		builder.WriteString(fmt.Sprintf("rows.add(new Account(Name = 'n%d'));\n", i))
	}
	builder.WriteString("insert rows;\n")
	builder.WriteString("System.assertEquals(")
	builder.WriteString(fmt.Sprint(rowCount))
	builder.WriteString(", [SELECT COUNT() FROM Account]);\n")
	program, err := CompileAnonymous(builder.String())
	if err != nil {
		t.Fatal(err)
	}
	triggerProgram, err := CompileAnonymous(`for (Account a : Trigger.new) { a.Description = a.Name; }`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsert",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	stats := recorder.Snapshot().DML
	if stats.ScopeAliasBatchCalls == 0 {
		t.Fatalf("chunked insert did not record DML scope-alias batch traversal: %#v", stats)
	}
	if stats.ScopeAliasBatchRows < 200 {
		t.Fatalf("batch row counter = %d, want at least one full 200-row chunk", stats.ScopeAliasBatchRows)
	}
}

func dmlScopeAliasBatchFixture(rowCount int) (map[string]Value, []dmlScopeAliasMutation) {
	scope := make(map[string]Value, rowCount+4)
	for i := range 4 {
		scope[fmt.Sprintf("envelope%d", i)] = Object("Account")
	}
	mutations := make([]dmlScopeAliasMutation, 0, rowCount)
	for i := range rowCount {
		row := Object("Account")
		row.Fields["Name"] = String(fmt.Sprintf("acct-%d", i))
		holder := Object("Account")
		holder.Fields["Row"] = row
		scope[fmt.Sprintf("holder%d", i)] = holder
		updated := row
		updated.Fields = map[string]Value{"Name": row.Fields["Name"]}
		updated.Fields["Id"] = platformScalar("Id", fmt.Sprintf("001000000000%03dAAA", i))
		mutations = append(mutations, dmlScopeAliasMutation{
			previous: snapshotAlias(row),
			updated:  updated,
		})
	}
	return scope, mutations
}

func cloneDMLScopeAliasBatchScope(scope map[string]Value) map[string]Value {
	cloned := make(map[string]Value, len(scope))
	for name, value := range scope {
		cloned[name] = cloneValuePreserveRefs(value)
	}
	return cloned
}
