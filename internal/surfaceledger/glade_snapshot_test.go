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

func TestBuildGladeSnapshotUsesPropertyIDWithoutCallParens(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)
	propertyID := ApexMemberID("ApexPages", "Component", "childComponents", nil)
	callID := ApexMemberID("ApexPages", "Component", "childComponents", []string{})
	if byID[propertyID].GladeShape == ShapeAbsent {
		t.Fatalf("missing property row %s", propertyID)
	}
	if _, ok := byID[callID]; ok {
		t.Fatalf("property should not also appear as zero-arg call %s", callID)
	}
}
