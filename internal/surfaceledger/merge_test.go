package surfaceledger

import "testing"

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Classify(&tt.row)
			if tt.row.GapClass != tt.gap {
				t.Fatalf("gap = %q, want %q", tt.row.GapClass, tt.gap)
			}
		})
	}
}
