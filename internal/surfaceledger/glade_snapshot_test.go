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

func TestBuildGladeSnapshotMarksGeneratedStandardSObjectShape(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)

	objectID := DataObjectID("AIInsightAction")
	objectRow, ok := byID[objectID]
	if !ok {
		t.Fatalf("missing generated standard object row %s", objectID)
	}
	if objectRow.GladeShape != ShapeGenerated || objectRow.ShapeSource != SourceStandardSObjectGeneratedShape {
		t.Fatalf("object row shape = %s source = %q", objectRow.GladeShape, objectRow.ShapeSource)
	}

	fieldID := DataFieldID("AIInsightAction", "AiRecordInsightId")
	fieldRow, ok := byID[fieldID]
	if !ok {
		t.Fatalf("missing generated standard field row %s", fieldID)
	}
	if fieldRow.GladeShape != ShapeGenerated || fieldRow.ReturnType != "REFERENCE" {
		t.Fatalf("field row shape = %s returnType = %q", fieldRow.GladeShape, fieldRow.ReturnType)
	}
	if fieldRow.GladeBehavior != BehaviorSupported {
		t.Fatalf("field row behavior = %s", fieldRow.GladeBehavior)
	}
}

func TestBuildGladeSnapshotIncludesEmbeddedOrgDescribeStandardSObjectShape(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)

	fieldID := DataFieldID("CareProgram", "ParentProgramId")
	fieldRow, ok := byID[fieldID]
	if !ok {
		t.Fatalf("missing embedded describe-backed standard field row %s", fieldID)
	}
	if fieldRow.GladeShape != ShapeGenerated || fieldRow.ReturnType != "REFERENCE" {
		t.Fatalf("field row shape = %s returnType = %q", fieldRow.GladeShape, fieldRow.ReturnType)
	}
	if fieldRow.GladeBehavior != BehaviorSupported {
		t.Fatalf("field row behavior = %s", fieldRow.GladeBehavior)
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

func TestSurfaceIDKeyNormalizesSystemQualifiedRuntimeParameters(t *testing.T) {
	left := surfaceIDKey(ApexMemberID("System", "Test", "createSoqlStub", []string{"Schema.SObjectType", "System.SoqlStubProvider"}))
	right := surfaceIDKey(ApexMemberID("System", "Test", "createSoqlStub", []string{"Schema.SObjectType", "SoqlStubProvider"}))
	if left != right {
		t.Fatalf("keys differ: %q != %q", left, right)
	}

	left = surfaceIDKey("apex:System.Test.createSoqlStub(Schema.SObjectType,System.SoqlStubProvider)")
	right = surfaceIDKey("apex:System.Test.createSoqlStub(Schema.SObjectType,SoqlStubProvider)")
	if left != right {
		t.Fatalf("raw keys differ: %q != %q", left, right)
	}

	left = surfaceIDKey(ApexMemberID("System", "StubProvider", "handleMethodCall", []string{"Object", "String", "Type", "List<System.Type>", "List<String>", "List<Object>"}))
	right = surfaceIDKey(ApexMemberID("System", "StubProvider", "handleMethodCall", []string{"Object", "String", "Type", "List<Type>", "List<String>", "List<Object>"}))
	if left != right {
		t.Fatalf("generic keys differ: %q != %q", left, right)
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

func TestBuildGladeSnapshotIncludesBatchQueryLocatorOverloads(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)
	for _, id := range []string{
		ApexTypeID("Database", "QueryLocator"),
		ApexMemberID("Database", "QueryLocator", "getQuery", []string{}),
		ApexMemberID("Database", "QueryLocator", "iterator", []string{}),
		ApexMemberID("System", "Database", "getQueryLocator", []string{"List<Object>"}),
		ApexMemberID("System", "Database", "getQueryLocator", []string{"List<Object>", "System.AccessLevel"}),
		ApexMemberID("System", "Database", "getQueryLocator", []string{"Object"}),
		ApexMemberID("System", "Database", "getQueryLocator", []string{"Object", "System.AccessLevel"}),
		ApexMemberID("System", "Database", "getQueryLocatorWithBinds", []string{"String", "Map", "System.AccessLevel"}),
	} {
		row, ok := byID[id]
		if !ok {
			t.Fatalf("missing batch query locator row %s", id)
		}
		if row.GladeShape == ShapeAbsent {
			t.Fatalf("batch query locator row %s has absent shape", id)
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
