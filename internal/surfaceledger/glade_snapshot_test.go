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

func TestBuildGladeSnapshotFencesLocalTestLWCServiceModules(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)
	for _, id := range []string{
		LWCModuleID("Decorators"),
		LWCModuleID("`lightning/graphql`"),
		LWCModuleID("`lightning/uiGraphQLApi`"),
	} {
		row, ok := byID[id]
		if !ok {
			t.Fatalf("missing LWC row %s", id)
		}
		if row.Product != ProductLWC || row.GladeBehavior != BehaviorUnsupported {
			t.Fatalf("LWC row %s product=%s behavior=%s", id, row.Product, row.GladeBehavior)
		}
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

func TestBuildGladeSnapshotIncludesMessagingLocalTestDTOShapes(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)
	for _, id := range []string{
		ApexMemberID("Messaging", "Email", "setTemplateID", []string{"Id"}),
		ApexTypeID("Messaging", "InboundEmail.AuthenticationResult"),
		ApexMemberID("Messaging", "InboundEmail.AuthenticationResult", "InboundEmail.AuthenticationResult", []string{}),
		ApexMemberID("Messaging", "InboundEmail.AuthenticationResult", "authenticationResultFields", nil),
		ApexMemberID("Messaging", "InboundEmail.AuthenticationResult", "method", nil),
		ApexMemberID("Messaging", "InboundEmail.AuthenticationResult", "result", nil),
		ApexTypeID("Messaging", "InboundEmail.AuthenticationResultField"),
		ApexMemberID("Messaging", "InboundEmail.AuthenticationResultField", "InboundEmail.AuthenticationResultField", []string{}),
		ApexMemberID("Messaging", "InboundEmail.AuthenticationResultField", "name", nil),
		ApexMemberID("Messaging", "InboundEmail.AuthenticationResultField", "value", nil),
		ApexMemberID("Messaging", "InboundEmail.BinaryAttachment", "InboundEmail.BinaryAttachment", []string{}),
		ApexMemberID("Messaging", "InboundEmail.TextAttachment", "InboundEmail.TextAttachment", []string{}),
		ApexMemberID("Messaging", "SingleEmailMessage", "setDocumentAttachments", []string{"List<Id>"}),
		ApexMemberID("Messaging", "SingleEmailMessage", "setFileAttachments", []string{"List<EmailFileAttachment>"}),
	} {
		row, ok := byID[id]
		if !ok {
			t.Fatalf("missing Messaging local-test row %s", id)
		}
		if row.GladeShape == ShapeAbsent {
			t.Fatalf("Messaging local-test row %s has absent shape", id)
		}
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

func TestBuildGladeSnapshotKeepsExplicitUnsupportedOverStubBehavior(t *testing.T) {
	rows := BuildGladeSnapshot()
	byID := rowsByID(rows)
	id := ApexMemberID("", "BusinessHours", "add", []string{"String", "Datetime", "Long"})
	row, ok := byID[id]
	if !ok {
		t.Fatalf("missing BusinessHours row %s", id)
	}
	if row.GladeBehavior != BehaviorUnsupported {
		t.Fatalf("BusinessHours.add behavior = %s, want %s", row.GladeBehavior, BehaviorUnsupported)
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

func TestMergeClosesCoreRuntimeCollectionObjectGenericShapeRows(t *testing.T) {
	docs := []SurfaceLedgerRow{
		docShapeRow("apex:System.Comparator.compare(T,T)", KindMethod),
		docShapeRow("apex:System.Enum", KindType),
		docShapeRow("apex:System.List.List<T>()", KindMethod),
		docShapeRow("apex:System.List.List<T>(List<T>)", KindMethod),
		docShapeRow("apex:System.List.equals(List)", KindMethod),
		docShapeRow("apex:System.Map.Map<ID,sObject>(List<sObject>)", KindMethod),
		docShapeRow("apex:System.Map.Map<T1,T2>()", KindMethod),
		docShapeRow("apex:System.Map.Map<T1,T2>(mapToCopy)", KindMethod),
		docShapeRow("apex:System.Map.equals(Map)", KindMethod),
		docShapeRow("apex:System.Map.remove(Key)", KindMethod),
		docShapeRow("apex:System.Object", KindType),
		docShapeRow("apex:System.Object.equals(Object)", KindMethod),
		docShapeRow("apex:System.Object.hashCode()", KindMethod),
		docShapeRow("apex:System.Object.toString()", KindMethod),
		docShapeRow("apex:System.Set.Set<T>()", KindMethod),
		docShapeRow("apex:System.Set.Set<T>(Set<T>)", KindMethod),
		docShapeRow("apex:System.Set.addAll(List<Object>)", KindMethod),
		docShapeRow("apex:System.Set.containsAll(List<Object>)", KindMethod),
		docShapeRow("apex:System.Set.equals(Set<Object>)", KindMethod),
		docShapeRow("apex:System.Set.removeAll(List<Object>)", KindMethod),
		docShapeRow("apex:System.Set.retainAll(List<Object>)", KindMethod),
	}
	ledger := Merge(docs, nil, BuildGladeSnapshot(), nil)
	byID := rowsByID(ledger.Rows)
	for _, doc := range docs {
		row, ok := byID[doc.SurfaceID]
		if !ok {
			t.Fatalf("missing merged row %s", doc.SurfaceID)
		}
		if row.GladeShape == ShapeAbsent || row.GapClass == GapMissingShape {
			t.Fatalf("%s shape = %s gap = %s", doc.SurfaceID, row.GladeShape, row.GapClass)
		}
	}
}

func docShapeRow(id, kind string) SurfaceLedgerRow {
	return RowFromDocs(SurfaceLedgerRow{
		SurfaceID: id,
		Product:   ProductApex,
		Area:      AreaRuntime,
		Kind:      kind,
	})
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
