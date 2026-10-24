package vm

import "testing"

func TestFlatObjectAliasRepeatedUpdatesScaleLinearly(t *testing.T) {
	visitsByCount := make(map[int]uint64, 3)
	for _, recordCount := range []int{200, 800, 4000} {
		machine := New(nil)
		rows := List()
		rows.Type = "List<Account>"
		for range recordCount {
			row := Object("Account")
			row.Fields["Name"] = String("Synthetic")
			row.Fields["Description"] = String("before")
			duplicate := row
			duplicate.Fields = cloneAliasOccurrenceFields(row.Fields)
			rows.List = append(rows.List, row, duplicate)
		}
		scope := map[string]Value{"rows": rows}
		recorder := NewPerfRecorder()
		machine.SetPerfRecorder(recorder)

		for i := range recordCount {
			previous := scope["rows"].List[2*i]
			updated := previous
			updated.Fields = cloneAliasOccurrenceFields(previous.Fields)
			updated.Fields["Description"] = String("processed")
			machine.propagateAliasSnapshotToScope(scope, snapshotAlias(previous), updated)

			for _, position := range []int{2 * i, 2*i + 1} {
				got := scope["rows"].List[position]
				if got.Ref != previous.Ref || got.Fields["Description"].Text != "processed" {
					t.Fatalf("record %d occurrence %d = %#v, want refreshed duplicate alias", i, position, got)
				}
			}
		}

		stats := recorder.Snapshot().ScopeAlias
		visits := stats.Roots + stats.RecursiveVisits + stats.ShortcutVisits + stats.ElisionFallbackWalks
		visitsByCount[recordCount] = visits
		t.Logf("records=%d occurrences=%d visits=%d roots=%d recursive=%d shortcut=%d fallback=%d", recordCount, len(rows.List), visits, stats.Roots, stats.RecursiveVisits, stats.ShortcutVisits, stats.ElisionFallbackWalks)
		if limit := uint64(40 * recordCount); visits > limit {
			t.Errorf("records=%d counted visits=%d, want <= %d", recordCount, visits, limit)
		}
	}
	if visitsByCount[800] > 5*visitsByCount[200] {
		t.Errorf("counted visits grew from %d to %d, want at most 5x", visitsByCount[200], visitsByCount[800])
	}
	if visitsByCount[4000] > 5*visitsByCount[800] {
		t.Errorf("counted visits grew from %d to %d, want at most 5x", visitsByCount[800], visitsByCount[4000])
	}
}

func TestFlatObjectAliasIndexRebuildsAfterListMutation(t *testing.T) {
	machine := New(nil)
	first := Object("Account")
	first.Fields["Name"] = String("first")
	firstDuplicate := first
	firstDuplicate.Fields = cloneAliasOccurrenceFields(first.Fields)
	second := Object("Account")
	second.Fields["Name"] = String("second")
	rows := List(first, firstDuplicate, second)
	rows.Type = "List<Account>"
	scope := map[string]Value{"rows": rows}

	firstUpdated := first
	firstUpdated.Fields = cloneAliasOccurrenceFields(first.Fields)
	firstUpdated.Fields["Description"] = String("initial")
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(first), firstUpdated)

	rows = scope["rows"]
	replacement := Object("Account")
	replacement.Fields["Name"] = String("replacement")
	rows.List[0], rows.List[2] = rows.List[2], rows.List[0]
	rows.List[1] = replacement
	machine.recordCollectionMutation(rows.Ref)
	scope["rows"] = rows

	replacementUpdated := replacement
	replacementUpdated.Fields = cloneAliasOccurrenceFields(replacement.Fields)
	replacementUpdated.Fields["Description"] = String("after mutation")
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(replacement), replacementUpdated)
	if got := scope["rows"].List[1].Fields["Description"]; got.Kind != ValueString || got.Text != "after mutation" {
		t.Fatalf("replacement row = %#v, want updated value", got)
	}
}

func TestFlatObjectAliasIndexFallsBackForNestedUpdate(t *testing.T) {
	machine := New(nil)
	target := Object("Account")
	target.Fields["Name"] = String("target")
	nested := Object("Account")
	nested.Fields["Name"] = String("nested")
	rows := List(target, nested)
	rows.Type = "List<Account>"
	scope := map[string]Value{"rows": rows}

	targetUpdated := target
	targetUpdated.Fields = cloneAliasOccurrenceFields(target.Fields)
	targetUpdated.Fields["Description"] = String("initial")
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), targetUpdated)

	rows = scope["rows"]
	nested = rows.List[1]
	nestedUpdated := nested
	nestedUpdated.Fields = cloneAliasOccurrenceFields(nested.Fields)
	nestedUpdated.Fields["Parent"] = targetUpdated
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(nested), nestedUpdated)

	rows = scope["rows"]
	target = rows.List[0]
	targetUpdated = target
	targetUpdated.Fields = cloneAliasOccurrenceFields(target.Fields)
	targetUpdated.Fields["Description"] = String("after nested update")
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), targetUpdated)
	parent := scope["rows"].List[1].Fields["Parent"]
	if got := parent.Fields["Description"]; parent.Ref != target.Ref || got.Kind != ValueString || got.Text != "after nested update" {
		t.Fatalf("nested alias = %#v, Description=%#v; want refreshed target %d", parent, got, target.Ref)
	}
}

func TestFlatObjectAliasIndexClearsAfterNestedFieldAssignment(t *testing.T) {
	machine := New(nil)
	target := Object("Account")
	target.Fields["Name"] = String("target")
	neighbor := Object("Account")
	neighbor.Fields["Name"] = String("neighbor")
	rows := List(target, neighbor)
	rows.Type = "List<Account>"
	scope := map[string]Value{"rows": rows}

	targetUpdated := target
	targetUpdated.Fields = cloneAliasOccurrenceFields(target.Fields)
	targetUpdated.Fields["Description"] = String("initial")
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), targetUpdated)

	rows = scope["rows"]
	machine.setGraphFieldValue(&rows.List[1], "Parent", targetUpdated)
	scope["rows"] = rows

	target = rows.List[0]
	targetUpdated = target
	targetUpdated.Fields = cloneAliasOccurrenceFields(target.Fields)
	targetUpdated.Fields["Description"] = String("after nested assignment")
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), targetUpdated)

	parent := scope["rows"].List[1].Fields["Parent"]
	if got := parent.Fields["Description"]; parent.Ref != target.Ref || got.Kind != ValueString || got.Text != "after nested assignment" {
		t.Fatalf("nested alias = %#v, Description=%#v; want refreshed target %d", parent, got, target.Ref)
	}
}

func cloneAliasOccurrenceFields(fields map[string]Value) map[string]Value {
	clone := make(map[string]Value, len(fields))
	for name, value := range fields {
		clone[name] = value
	}
	return clone
}
