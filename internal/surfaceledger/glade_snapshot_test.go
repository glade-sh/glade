package surfaceledger

import "testing"

func TestBuildGladeSnapshotIncludesKnownStdlibBehavior(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)
	id := ApexMemberID("System", "String", "contains", []string{"String"})
	row, ok := byID[id]
	if !ok {
		t.Fatalf("missing %s", id)
	}
	if row.GladeShape == ShapeAbsent || row.GladeBehavior == BehaviorNone {
		t.Fatalf("String.contains states = shape:%s behavior:%s", row.GladeShape, row.GladeBehavior)
	}
}
