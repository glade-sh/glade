package vm

import "testing"

func TestPropagateTopLevelAliasSnapshotToScopeCountUsesUnsignedCount(t *testing.T) {
	original := Value{Kind: ValueList, Type: "List<String>", Ref: 7}
	updated := Value{Kind: ValueList, Type: "List<String>", Ref: 8}
	scope := map[string]Value{
		"first":  original,
		"second": original,
	}

	var got uint64 = propagateTopLevelAliasSnapshotToScopeCount(scope, snapshotAlias(original), updated)
	if got != 2 {
		t.Fatalf("changed count = %d, want 2", got)
	}
	for name, value := range scope {
		if value.Ref != updated.Ref {
			t.Fatalf("scope[%q].Ref = %d, want %d", name, value.Ref, updated.Ref)
		}
	}
}
