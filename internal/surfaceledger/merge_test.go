package surfaceledger

import (
	"strings"
	"testing"
)

func TestMergeCombinesSourcesBySurfaceID(t *testing.T) {
	id := ApexMemberID("System", "Label", "get", []string{"String", "String"})
	ledger := Merge(
		[]SurfaceLedgerRow{RowFromDocs(SurfaceLedgerRow{SurfaceID: id, Product: ProductApex, Area: AreaRuntime, Kind: KindMethod})},
		[]SurfaceLedgerRow{RowFromOrg(SurfaceLedgerRow{SurfaceID: id, Product: ProductApex, Area: AreaRuntime, Kind: KindMethod})},
		[]SurfaceLedgerRow{RowFromGladeShape(SurfaceLedgerRow{SurfaceID: id, Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, GladeBehavior: BehaviorSupported})},
		[]SurfaceLedgerRow{RowFromEvidence(SurfaceLedgerRow{SurfaceID: id, Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, Evidence: EvidenceFixture})},
	)

	if len(ledger.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(ledger.Rows))
	}
	row := ledger.Rows[0]
	if row.Docs != SourcePresent || row.Org != SourcePresent || row.GladeShape != ShapeSignatureKnown || row.GladeBehavior != BehaviorSupported || row.Evidence != EvidenceFixture {
		t.Fatalf("merged row states = docs:%s org:%s shape:%s behavior:%s evidence:%s", row.Docs, row.Org, row.GladeShape, row.GladeBehavior, row.Evidence)
	}
	if row.Bucket != BucketImplemented {
		t.Fatalf("bucket = %q, want %q", row.Bucket, BucketImplemented)
	}
}

func TestClassifyGapFromStates(t *testing.T) {
	tests := []struct {
		name string
		row  SurfaceLedgerRow
		gap  string
	}{
		{
			name: "missing shape",
			row:  RowFromDocs(SurfaceLedgerRow{SurfaceID: ApexTypeID("System", "Label"), Product: ProductApex, Area: AreaRuntime, Kind: KindType}),
			gap:  GapMissingShape,
		},
		{
			name: "missing signature",
			row:  RowFromDocs(SurfaceLedgerRow{SurfaceID: ApexMemberID("System", "Label", "get", []string{"String"}), Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, GladeShape: ShapeTypeKnown}),
			gap:  GapMissingSignature,
		},
		{
			name: "missing behavior",
			row:  RowFromGladeShape(RowFromDocs(SurfaceLedgerRow{SurfaceID: ApexMemberID("System", "Label", "get", nil), Product: ProductApex, Area: AreaRuntime, Kind: KindMethod})),
			gap:  GapMissingBehavior,
		},
		{
			name: "missing evidence",
			row:  SurfaceLedgerRow{SurfaceID: ApexMemberID("System", "String", "length", nil), Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, Docs: SourcePresent, GladeShape: ShapeSignatureKnown, GladeBehavior: BehaviorSupported, Evidence: EvidenceNone},
			gap:  GapMissingEvidence,
		},
		{
			name: "stale glade shape",
			row:  SurfaceLedgerRow{SurfaceID: ApexTypeID("System", "OldThing"), Product: ProductApex, Area: AreaRuntime, Kind: KindType, Docs: SourceAbsent, Org: SourceAbsent, GladeShape: ShapeTypeKnown},
			gap:  GapStaleGladeShape,
		},
		{
			name: "passive glade-only shape",
			row:  SurfaceLedgerRow{SurfaceID: ApexMemberID("ApexPages", "Component", "Component", []string{}), Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, Docs: SourceAbsent, Org: SourceAbsent, GladeShape: ShapeSignatureKnown, GladeBehavior: BehaviorPassive},
			gap:  "",
		},
		{
			name: "passive glade-only dto type",
			row:  SurfaceLedgerRow{SurfaceID: ApexTypeID("ConnectApi", "ApplicationFormInput"), Product: ProductApex, Area: AreaRuntime, Kind: KindType, Docs: SourceAbsent, Org: SourceAbsent, GladeShape: ShapeTypeKnown, GladeBehavior: BehaviorPassive},
			gap:  "",
		},
		{
			name: "generic object glade-only method",
			row:  SurfaceLedgerRow{SurfaceID: ApexMemberID("ConnectApi", "ApplicationFormInput", "equals", []string{"Object"}), Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, MemberName: "equals", Parameters: []string{"Object"}, Docs: SourceAbsent, Org: SourceAbsent, GladeShape: ShapeSignatureKnown, GladeBehavior: BehaviorSupported},
			gap:  "",
		},
		{
			name: "explicit unsupported glade-only method",
			row:  SurfaceLedgerRow{SurfaceID: ApexMemberID("ConnectApi", "Billing", "createCreditMemos", []string{"ConnectApi.StandaloneCreditMemoInputRequest"}), Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, MemberName: "createCreditMemos", Parameters: []string{"ConnectApi.StandaloneCreditMemoInputRequest"}, Docs: SourceAbsent, Org: SourceAbsent, GladeShape: ShapeSignatureKnown, GladeBehavior: BehaviorUnsupported},
			gap:  "",
		},
		{
			name: "generic enum glade-only method",
			row:  SurfaceLedgerRow{SurfaceID: ApexMemberID("Database.Cursor", "DeleteFilter", "values", []string{}), Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, MemberName: "values", Docs: SourceAbsent, Org: SourceAbsent, GladeShape: ShapeSignatureKnown, GladeBehavior: BehaviorSupported},
			gap:  "",
		},
		{
			name: "enum constant glade-only property",
			row:  SurfaceLedgerRow{SurfaceID: ApexMemberID("Database.Cursor", "DeleteFilter", "NO_FILTER", nil), Product: ProductApex, Area: AreaRuntime, Kind: KindProperty, MemberName: "NO_FILTER", Docs: SourceAbsent, Org: SourceAbsent, GladeShape: ShapeSignatureKnown, GladeBehavior: BehaviorSupported},
			gap:  "",
		},
		{
			name: "supported glade-only method without evidence",
			row:  SurfaceLedgerRow{SurfaceID: ApexMemberID("System", "Database", "delete", []string{"Object"}), Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, MemberName: "delete", Parameters: []string{"Object"}, Docs: SourceAbsent, Org: SourceAbsent, GladeShape: ShapeSignatureKnown, GladeBehavior: BehaviorSupported, Evidence: EvidenceNone},
			gap:  GapMissingEvidence,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Classify(&tt.row)
			if tt.row.GapClass != tt.gap {
				t.Fatalf("gap = %q, want %q", tt.row.GapClass, tt.gap)
			}
			if strings.HasPrefix(tt.name, "passive glade-only") && tt.row.Bucket != BucketPassive {
				t.Fatalf("bucket = %q, want %q", tt.row.Bucket, BucketPassive)
			}
			if tt.name == "generic object glade-only method" && tt.row.Bucket != BucketImplemented {
				t.Fatalf("bucket = %q, want %q", tt.row.Bucket, BucketImplemented)
			}
			if tt.name == "explicit unsupported glade-only method" && tt.row.Bucket != BucketExplicitUnsupported {
				t.Fatalf("bucket = %q, want %q", tt.row.Bucket, BucketExplicitUnsupported)
			}
			if (tt.name == "generic enum glade-only method" || tt.name == "enum constant glade-only property") && tt.row.Bucket != BucketImplemented {
				t.Fatalf("bucket = %q, want %q", tt.row.Bucket, BucketImplemented)
			}
		})
	}
}
