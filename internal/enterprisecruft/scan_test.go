package enterprisecruft

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/enterprisegraph"
)

func TestScanProducesGroupedConservativeFindings(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	ctx, err := enterprise.LoadContext(root)
	if err != nil {
		t.Fatalf("LoadContext: %v", err)
	}

	report := Scan(ctx, enterprisegraph.Graph{})

	if !hasCruftSection(report.Sections, string(BucketSafeDeleteCandidate)) {
		t.Fatalf("missing safe-delete grouped section: %#v", report.Sections)
	}
	for _, finding := range report.Findings {
		if (hasTag(finding.Tags, string(BucketSafeDeleteCandidate)) || hasTag(finding.Tags, string(BucketSafeDeprecateCandidate))) &&
			(finding.Symbol == "LegacyBadgeController" || finding.Symbol == "LegacyFlowAction" || finding.Symbol == "LegacyWorkOrderReviewExtension") {
			t.Fatalf("referenced public symbol marked safe-delete/deprecate: %#v", finding)
		}
	}
}

func TestIncomingReferenceCountsUseGraphEdges(t *testing.T) {
	graph := enterprisegraph.Graph{}
	graph.AddEdge(enterprisegraph.Edge{From: "Caller", To: "UsedService", Kind: enterprisegraph.EdgeKindCalls})
	graph.AddEdge(enterprisegraph.Edge{From: "Page", To: "UsedService", Kind: enterprisegraph.EdgeKindMetadataReferences})

	counts := incomingReferenceCounts(graph)

	if counts["UsedService"] != 2 {
		t.Fatalf("references = %d, want 2", counts["UsedService"])
	}
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

func hasCruftSection(sections []enterprise.Section, id string) bool {
	for _, section := range sections {
		if section.ID == id {
			return true
		}
	}
	return false
}
