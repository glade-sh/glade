package enterprisecruft

import (
	"fmt"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/enterprisegraph"
)

func Scan(ctx enterprise.Context, graph enterprisegraph.Graph) enterprise.Report {
	if len(graph.Nodes) == 0 && len(graph.Edges) == 0 {
		if built, err := enterprisegraph.Build(ctx); err == nil {
			graph = built
		}
	}
	references := incomingReferenceCounts(graph)
	metadataReferences := metadataReferenceSet(graph)
	report := enterprise.NewReport("glade enterprise cruft scan", ctx.Summary())
	grouped := map[Bucket][]enterprise.SectionItem{}
	for _, typ := range ctx.Index.Types {
		if typ.Dependency {
			continue
		}
		source := readText(typ.File)
		dynamic := DetectDynamicReferences(source)
		facts := SymbolFacts{
			Name:               typ.Name,
			Visibility:         visibility(typ.Modifiers),
			References:         references[typ.Name],
			DynamicReferences:  dynamic.DynamicApex,
			MetadataReferences: dynamic.CustomMetadataRouting || metadataReferences[typ.Name],
			IsTest:             typ.IsTest,
			RuntimeSurface:     strings.Contains(strings.ToLower(typ.Name), "trigger") || strings.Contains(strings.ToLower(source), "database."),
		}
		classification := Classify(facts)
		grouped[classification.Bucket] = append(grouped[classification.Bucket], enterprise.SectionItem{Label: typ.Name, Value: classification.Reason})
		if classification.Bucket == BucketUnknown {
			continue
		}
		report.Findings = append(report.Findings, enterprise.Finding{
			ID:             fmt.Sprintf("enterprise.cruft.%s.%s", classification.Bucket, strings.ToLower(typ.Name)),
			Category:       enterprise.CategoryCruft,
			Severity:       severityForBucket(classification.Bucket),
			Confidence:     confidence(classification.Confidence),
			Title:          "Cruft classification: " + string(classification.Bucket),
			Summary:        classification.Reason,
			Symbol:         typ.Name,
			Location:       enterprise.Location{File: typ.File, LineStart: typ.Range.Start.Line},
			Evidence:       []enterprise.Evidence{{Type: enterprise.EvidenceHeuristic, Message: classification.Reason}},
			Recommendation: classification.Recommendation,
			Tags:           []string{string(classification.Bucket)},
		})
	}
	for _, bucket := range []Bucket{BucketSafeDeleteCandidate, BucketSafeDeprecateCandidate, BucketReviewDynamicReferenceRisk, BucketPackageContractDoNotDelete, BucketRuntimeCharacterizationNeeded, BucketTestOnlyCleanup, BucketUnknown} {
		report.Sections = append(report.Sections, enterprise.Section{
			ID:      string(bucket),
			Title:   strings.ReplaceAll(string(bucket), "_", " "),
			Summary: fmt.Sprintf("%d symbols", len(grouped[bucket])),
			Items:   grouped[bucket],
		})
	}
	report.Limitations = []string{
		"static graph references are conservative",
		"dynamic Apex/custom metadata routing reduce confidence",
		"public/global symbols are never marked safe-delete",
	}
	report.RefreshSummary()
	return report
}

func incomingReferenceCounts(graph enterprisegraph.Graph) map[string]int {
	counts := make(map[string]int)
	for _, edge := range graph.Edges {
		if edge.To == "" || edge.From == "" || edge.From == edge.To {
			continue
		}
		switch edge.Kind {
		case enterprisegraph.EdgeKindReferences, enterprisegraph.EdgeKindCalls, enterprisegraph.EdgeKindExtends, enterprisegraph.EdgeKindImplements, enterprisegraph.EdgeKindTestCovers, enterprisegraph.EdgeKindMetadataReferences:
			counts[edge.To]++
		}
	}
	return counts
}

func metadataReferenceSet(graph enterprisegraph.Graph) map[string]bool {
	refs := make(map[string]bool)
	for _, edge := range graph.Edges {
		if edge.Kind == enterprisegraph.EdgeKindMetadataReferences && edge.To != "" {
			refs[edge.To] = true
		}
	}
	return refs
}

func visibility(modifiers []string) string {
	for _, modifier := range modifiers {
		switch strings.ToLower(modifier) {
		case "global", "public", "private":
			return strings.ToLower(modifier)
		}
	}
	return "private"
}

func severityForBucket(bucket Bucket) enterprise.Severity {
	switch bucket {
	case BucketPackageContractDoNotDelete, BucketReviewDynamicReferenceRisk, BucketRuntimeCharacterizationNeeded:
		return enterprise.SeverityMedium
	case BucketSafeDeleteCandidate, BucketSafeDeprecateCandidate, BucketTestOnlyCleanup:
		return enterprise.SeverityLow
	default:
		return enterprise.SeverityInfo
	}
}

func confidence(value string) enterprise.Confidence {
	switch value {
	case "high":
		return enterprise.ConfidenceHigh
	case "medium":
		return enterprise.ConfidenceMedium
	case "low":
		return enterprise.ConfidenceLow
	default:
		return enterprise.ConfidenceUnknown
	}
}

func readText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
