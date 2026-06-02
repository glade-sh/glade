package oracle

import (
	"testing"

	"github.com/glade-sh/glade/internal/surfaceledger"
)

func TestBuildInventoryFromLedgerFiltersGapClass(t *testing.T) {
	ledger := surfaceledger.SurfaceLedger{Rows: []surfaceledger.SurfaceLedgerRow{
		{SurfaceID: surfaceledger.ApexMemberID("System", "Label", "get", []string{"String", "String"}), Product: surfaceledger.ProductApex, Area: surfaceledger.AreaRuntime, Namespace: "System", TypeName: "Label", MemberName: "get", Kind: surfaceledger.KindMethod, Parameters: []string{"String", "String"}, GapClass: surfaceledger.GapMissingBehavior},
		{SurfaceID: surfaceledger.ApexTypeID("System", "Object"), Product: surfaceledger.ProductApex, Area: surfaceledger.AreaRuntime, Namespace: "System", TypeName: "Object", Kind: surfaceledger.KindType, GapClass: surfaceledger.GapMissingShape},
	}}
	inv := BuildInventoryFromLedger(ledger, surfaceledger.GapMissingBehavior, 25)
	if len(inv.Surfaces) != 1 {
		t.Fatalf("surfaces = %d, want 1", len(inv.Surfaces))
	}
	if inv.Surfaces[0].SurfaceID != surfaceledger.ApexMemberID("System", "Label", "get", []string{"String", "String"}) {
		t.Fatalf("surface = %#v", inv.Surfaces[0])
	}
	if inv.Surfaces[0].Status != SurfaceCompileShapeKnown {
		t.Fatalf("status = %q", inv.Surfaces[0].Status)
	}
}
