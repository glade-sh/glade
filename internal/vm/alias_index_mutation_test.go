package vm

import "testing"

func warmAliasMutationIndex(t *testing.T, machine *VM, rows Value) {
	t.Helper()
	handled, _ := machine.tryIndexedFlatObjectAliasRoot(rows, snapshotAlias(rows.List[0]), rows.List[0], nil)
	if !handled || len(machine.flatAliasIndex) == 0 {
		t.Fatal("flat list index was not built")
	}
}

func TestFlatObjectAliasIndexSharedBackingMutation(t *testing.T) {
	for _, offset := range []int{0, 1} {
		t.Run(string(rune('0'+offset)), func(t *testing.T) {
			machine := New(nil)
			rows := List(Object("Account"), Object("Account"), Object("Account"))
			rows.Type = "List<Account>"
			warmAliasMutationIndex(t, machine, rows)
			// A different logical list can view the same backing, including a subslice.
			view := List()
			view.Type = rows.Type
			view.List = rows.List[offset:]
			replacement := Object("Account")
			if _, handled, err := machine.callListValueMember("", view, "set", []Value{Int(0), replacement}, &Result{}); err != nil || !handled {
				t.Fatalf("set: handled=%v err=%v", handled, err)
			}
			updated := cloneValuePreserveRefs(replacement)
			updated.Fields["Name"] = String("processed")
			scope := map[string]Value{"rows": rows}
			machine.propagateAliasSnapshotToScope(scope, snapshotAlias(replacement), updated)
			if got := scope["rows"].List[offset].Fields["Name"].Text; got != "processed" {
				t.Fatalf("new occurrence through shared backing = %q, want processed", got)
			}
		})
	}
}

func TestFlatObjectAliasIndexDMLMergeIntroducesNestedAlias(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	row := Object("Account")
	row.Fields["Name"] = String("row")
	rows := List(row)
	rows.Type = "List<Account>"
	warmAliasMutationIndex(t, machine, rows)
	parent := Object("Account")
	parent.Fields["Name"] = String("before")
	alias := cloneValuePreserveRefs(row)
	alias.Fields["Parent"] = cloneValuePreserveRefs(parent)
	machine.Globals["alias"] = alias
	if _, _, err := machine.recordsFromValueUnchecked(rows); err != nil {
		t.Fatal(err)
	}
	if rows.List[0].Fields["Parent"].Ref != parent.Ref {
		t.Fatal("DML merge did not introduce the nested alias")
	}
	updated := cloneValuePreserveRefs(parent)
	updated.Fields["Name"] = String("processed")
	scope := map[string]Value{"rows": rows}
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(parent), updated)
	if got := scope["rows"].List[0].Fields["Parent"].Fields["Name"].Text; got != "processed" {
		t.Fatalf("nested alias after DML merge = %q, want processed", got)
	}
}

func TestFlatObjectAliasIndexCoercionInvalidates(t *testing.T) {
	machine := New(nil)
	rows := List(Object("Account"))
	rows.Type = "List<Account>"
	warmAliasMutationIndex(t, machine, rows)
	if _, err := machine.coerceAssignable("List<Object>", rows); err != nil {
		t.Fatal(err)
	}
	if len(machine.flatAliasIndex) != 0 || machine.flatAliasIndexSlots != 0 {
		t.Fatal("in-place element coercion retained an index")
	}
}

func TestFlatObjectAliasIndexLegacyReplacementInvalidates(t *testing.T) {
	machine := New(nil)
	row := Object("Account")
	rows := List(row)
	rows.Type = "List<Account>"
	warmAliasMutationIndex(t, machine, rows)
	scope := map[string]Value{"rows": rows}
	replacement := Object("Account")
	machine.propagateValueMutationToScope(scope, row, replacement)
	updated := cloneValuePreserveRefs(replacement)
	updated.Fields["Name"] = String("processed")
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(replacement), updated)
	if got := scope["rows"].List[0].Fields["Name"].Text; got != "processed" {
		t.Fatalf("new occurrence after legacy replacement = %q, want processed", got)
	}
}
