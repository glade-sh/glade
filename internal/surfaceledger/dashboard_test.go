package surfaceledger

import (
	"strings"
	"testing"
)

func TestDashboardStartsWithBuckets(t *testing.T) {
	ledger := SurfaceLedger{SchemaVersion: SchemaVersion, Rows: []SurfaceLedgerRow{
		{SurfaceID: ApexTypeID("System", "Label"), Product: ProductApex, Bucket: BucketGap, GapClass: GapMissingShape, Owner: "runtime"},
	}}
	ledger.Summary = Summarize(ledger.Rows)
	md := DashboardMarkdown(ledger)
	if !strings.Contains(md, "| implemented |") || !strings.Contains(md, "Top Gaps") {
		t.Fatalf("dashboard missing buckets: %s", md)
	}
}

func TestCheckLedgerRatchets(t *testing.T) {
	ledger := SurfaceLedger{Rows: []SurfaceLedgerRow{
		{SurfaceID: "apex:System.Missing", Bucket: BucketGap, GapClass: GapMissingShape},
	}}
	ledger.Summary = Summarize(ledger.Rows)
	if err := CheckLedger(ledger, CheckOptions{MaxMissingShape: 1}); err != nil {
		t.Fatalf("check with matching ratchet failed: %v", err)
	}
	if err := CheckLedger(ledger, CheckOptions{MaxMissingShape: 0}); err == nil {
		t.Fatalf("check passed with too-low ratchet")
	}
}
