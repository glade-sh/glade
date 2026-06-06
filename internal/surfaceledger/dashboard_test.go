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

func TestProgressMarkdownShowsVerticalsAndUnmatchedRows(t *testing.T) {
	ledger := SurfaceLedger{SchemaVersion: SchemaVersion, Rows: []SurfaceLedgerRow{
		{SurfaceID: ApexMemberID("System", "Database", "executeBatch", []string{"Database.Batchable"}), Product: ProductApex, Namespace: "System", TypeName: "Database", MemberName: "executeBatch", Bucket: BucketImplemented, Owner: "runtime"},
		{SurfaceID: "rest:/services/data/vXX.X/sobjects", Product: ProductREST, SalesforceSurfaceFamily: "rest-api", Bucket: BucketGap, GapClass: GapMissingBehavior, Owner: "server", Priority: 3},
		{SurfaceID: "odd:unclaimed", Product: ProductUnknown, Bucket: BucketGap, GapClass: GapMissingShape, Priority: 1},
	}}
	ledger.Summary = Summarize(ledger.Rows)
	md := ProgressMarkdown(ledger)
	for _, want := range []string{
		"Salesforce Surface Progress",
		"`Core.Runtime.Database.Batchable`",
		"`Server.RESTResources`",
		"Rows not claimed by a vertical packet: 1",
		"`odd:unclaimed`",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("progress markdown missing %q:\n%s", want, md)
		}
	}
}

func TestProgressMarkdownCountsOnlyImplementedRowsAsDone(t *testing.T) {
	ledger := SurfaceLedger{SchemaVersion: SchemaVersion, Rows: []SurfaceLedgerRow{
		{SurfaceID: ApexTypeID("Schema", "DescribeFieldResult"), Product: ProductApex, Namespace: "Schema", TypeName: "DescribeFieldResult", Bucket: BucketImplemented, Owner: "data-runtime"},
		{SurfaceID: ApexTypeID("Schema", "SObjectType"), Product: ProductApex, Namespace: "Schema", TypeName: "SObjectType", Bucket: BucketPassive, Owner: "data-runtime"},
		{SurfaceID: ApexTypeID("Schema", "UnsupportedThing"), Product: ProductApex, Namespace: "Schema", TypeName: "UnsupportedThing", Bucket: BucketExplicitUnsupported, Owner: "data-runtime"},
		{SurfaceID: ApexMemberID("Context", "IndustriesContext", "addRecordsToContext", []string{"Map<String,Object>"}), Product: ProductApex, Namespace: "Context", TypeName: "IndustriesContext", MemberName: "addRecordsToContext", Bucket: "stubNoOp", Owner: "runtime"},
	}}
	ledger.Summary = Summarize(ledger.Rows)
	md := ProgressMarkdown(ledger)
	for _, want := range []string{
		"Progress is `implemented / total`",
		"| scope | total | implemented | partial | passive | unsupported | stub/no-op | remaining | progress |",
		"| all surfaces | 4 | 1 | 0 | 1 | 1 | 1 | 0 | [##........] 25.0% |",
		"| `Data.Runtime.SchemaDescribe` | internal/vm describe runtime | [###.......] 33.3% | 1/3 | 0 | 1 | 1 | 0 | 0 | - |",
		"| `Core.Runtime.Context.IndustriesContext` | internal/vm context runtime | [..........] 0.0% | 0/1 | 0 | 0 | 0 | 1 | 0 | - |",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("progress markdown missing %q:\n%s", want, md)
		}
	}
}

func TestProgressHTMLRendersBars(t *testing.T) {
	ledger := SurfaceLedger{SchemaVersion: SchemaVersion, Rows: []SurfaceLedgerRow{
		{SurfaceID: ApexTypeID("System", "FeatureManagement"), Product: ProductApex, Namespace: "System", TypeName: "FeatureManagement", Bucket: BucketImplemented, Owner: "runtime"},
	}}
	ledger.Summary = Summarize(ledger.Rows)
	html := ProgressHTML(ledger)
	for _, want := range []string{
		"<!DOCTYPE html>",
		"Salesforce Surface Progress",
		"Core.Runtime.System.FeatureManagement",
		`class="bar"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("progress html missing %q:\n%s", want, html)
		}
	}
}

func TestPacketRowsCoversGenericVerticals(t *testing.T) {
	ledger := SurfaceLedger{Rows: []SurfaceLedgerRow{
		{SurfaceID: "rest:/services/data/vXX.X/query", Product: ProductREST, SalesforceSurfaceFamily: "rest-api"},
		{SurfaceID: "apex:System.Schema.DescribeSObjectResult", Product: ProductApex, Namespace: "Schema", TypeName: "DescribeSObjectResult"},
		{SurfaceID: "apex:System.ConnectApi.ManagedContent", Product: ProductConnectAPI, SalesforceSurfaceFamily: ProductConnectAPI},
	}}
	for _, area := range []string{"Server.RESTResources", "Data.Runtime.SchemaDescribe", "ConnectApi.PassiveDTOs"} {
		packet, ok := AreaPacketByName(area)
		if !ok {
			t.Fatalf("missing packet %s", area)
		}
		if got := len(PacketRows(ledger, packet)); got != 1 {
			t.Fatalf("PacketRows(%s)=%d, want 1", area, got)
		}
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
