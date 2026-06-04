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

func TestBuildGladeSnapshotUsesSchemaDescribePropertyIDs(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)
	id := ApexMemberID("Schema", "DescribeTabSetResult", "name", nil)
	row, ok := byID[id]
	if !ok {
		t.Fatalf("missing Schema describe property row %s", id)
	}
	if row.Kind != KindProperty || row.GladeShape == ShapeAbsent || row.GladeBehavior != BehaviorSupported {
		t.Fatalf("property row = kind:%s shape:%s behavior:%s", row.Kind, row.GladeShape, row.GladeBehavior)
	}
}

func TestStdlibAPIIDParsesQualifiedSchemaMethods(t *testing.T) {
	got := idFromStdlibAPI("Schema.describeDataCategoryGroups(List<String>)")
	want := ApexMemberID("Schema", "Schema", "describeDataCategoryGroups", []string{"List<String>"})
	if got != want {
		t.Fatalf("id = %q, want %q", got, want)
	}

	got = idFromStdlibAPI("Schema.describeDataCategoryGroupStructures(List<Schema.DataCategoryGroupSobjectTypePair>,Boolean)")
	want = ApexMemberID("Schema", "Schema", "describeDataCategoryGroupStructures", []string{"List<Schema.DataCategoryGroupSobjectTypePair>", "Boolean"})
	if got != want {
		t.Fatalf("id = %q, want %q", got, want)
	}
}

func TestBuildGladeSnapshotUsesParameterizedDataCategoryStdlibRows(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)
	typedID := ApexMemberID("Schema", "Schema", "describeDataCategoryGroups", []string{"List<String>"})
	if _, ok := byID[typedID]; !ok {
		t.Fatalf("missing typed data category row %s", typedID)
	}
	coarseID := ApexMemberID("Schema", "Schema", "describeDataCategoryGroups", nil)
	if _, ok := byID[coarseID]; ok {
		t.Fatalf("found unparameterized data category stdlib row %s", coarseID)
	}
}

func TestBuildGladeSnapshotUsesCanonicalSchemaDescribeStdlibRows(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)
	for _, id := range []string{
		ApexMemberID("Schema", "Schema", "getGlobalDescribe", []string{}),
		ApexMemberID("Schema", "Schema", "describeSObjects", []string{"List<String>"}),
	} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("missing canonical schema stdlib row %s", id)
		}
	}
	for _, id := range []string{
		ApexMemberID("Schema", "Schema", "getGlobalDescribe", nil),
		ApexMemberID("Schema", "Schema", "describeSObjects", nil),
	} {
		if _, ok := byID[id]; ok {
			t.Fatalf("found unparameterized schema stdlib row %s", id)
		}
	}
}

func TestMergeGladeBehaviorKeepsSupportedOverGenericUnsupported(t *testing.T) {
	got := mergeGladeBehavior(BehaviorSupported, BehaviorUnsupported)
	if got != BehaviorSupported {
		t.Fatalf("behavior = %q, want %q", got, BehaviorSupported)
	}
	got = mergeGladeBehavior(BehaviorPartial, BehaviorUnsupported)
	if got != BehaviorPartial {
		t.Fatalf("behavior = %q, want %q", got, BehaviorPartial)
	}
}
