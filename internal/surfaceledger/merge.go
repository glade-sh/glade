package surfaceledger

import (
	"sort"
	"strings"
)

func Merge(docs, org, glade, evidence []SurfaceLedgerRow) SurfaceLedger {
	byID := map[string]SurfaceLedgerRow{}
	for _, group := range [][]SurfaceLedgerRow{docs, org, glade, evidence} {
		for _, row := range group {
			row = withDefaults(row)
			if row.SurfaceID == "" {
				continue
			}
			key := surfaceIDKey(row.SurfaceID)
			merged := byID[key]
			merged = mergeRow(merged, row)
			Classify(&merged)
			byID[key] = merged
		}
	}
	rows := make([]SurfaceLedgerRow, 0, len(byID))
	for _, row := range byID {
		Classify(&row)
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].SurfaceID < rows[j].SurfaceID
	})
	ledger := SurfaceLedger{SchemaVersion: SchemaVersion, Rows: rows}
	ledger.Summary = Summarize(rows)
	return ledger
}

func mergeRow(base, next SurfaceLedgerRow) SurfaceLedgerRow {
	if base.SurfaceID == "" {
		base = withDefaults(next)
	} else {
		base = fillIdentity(base, next)
		if next.Docs != "" && next.Docs != SourceAbsent {
			base.Docs = next.Docs
		}
		if next.Org != "" && next.Org != SourceAbsent {
			base.Org = next.Org
		}
		if shapeRank(next.GladeShape) > shapeRank(base.GladeShape) {
			base.GladeShape = next.GladeShape
		}
		if behaviorRank(next.GladeBehavior) > behaviorRank(base.GladeBehavior) {
			base.GladeBehavior = next.GladeBehavior
		}
		base.Evidence = mergeEvidence(base.Evidence, next.Evidence)
		if next.DocsSource != "" {
			base.DocsSource = next.DocsSource
		}
		if next.DocsTitle != "" {
			base.DocsTitle = next.DocsTitle
		}
		if next.APIVersion != "" {
			base.APIVersion = next.APIVersion
		}
		if next.Owner != "" {
			base.Owner = next.Owner
		}
		if next.Notes != "" {
			base.Notes = next.Notes
		}
	}
	base.Sources = mergeStrings(base.Sources, next.Sources)
	return withDefaults(base)
}

func fillIdentity(base, next SurfaceLedgerRow) SurfaceLedgerRow {
	if base.Product == "" || base.Product == ProductUnknown {
		base.Product = next.Product
	}
	if base.Area == "" {
		base.Area = next.Area
	}
	if base.Namespace == "" {
		base.Namespace = next.Namespace
	}
	if base.TypeName == "" {
		base.TypeName = next.TypeName
	}
	if base.MemberName == "" {
		base.MemberName = next.MemberName
	}
	if base.Resource == "" {
		base.Resource = next.Resource
	}
	if base.FieldName == "" {
		base.FieldName = next.FieldName
	}
	if base.Kind == "" {
		base.Kind = next.Kind
	}
	if base.Signature == "" {
		base.Signature = next.Signature
	}
	if base.ReturnType == "" {
		base.ReturnType = next.ReturnType
	}
	if len(base.Parameters) == 0 {
		base.Parameters = append([]string(nil), next.Parameters...)
	}
	return base
}

func Classify(row *SurfaceLedgerRow) {
	if row == nil {
		return
	}
	*row = withDefaults(*row)
	row.GapClass = ""
	row.Bucket = ""
	switch {
	case row.Docs == SourcePresent && row.Org == SourcePresent && row.Signature != "" && row.GladeShape == ShapeSignatureKnown && row.GladeBehavior != BehaviorNone && row.Evidence != EvidenceNone && row.SignatureChanged():
		row.GapClass = GapSignatureChanged
		row.Bucket = BucketFailure
	case row.Docs == SourcePresent && row.Org == SourcePresent && row.SignatureChanged():
		row.GapClass = GapSignatureChanged
		row.Bucket = BucketFailure
	case row.GladeBehavior == BehaviorPassive:
		row.Bucket = BucketPassive
	case row.GladeBehavior == BehaviorUnsupported:
		row.Bucket = BucketExplicitUnsupported
	case isGenericObjectHelper(*row):
		row.Bucket = BucketImplemented
	case isGenericEnumHelper(*row):
		row.Bucket = BucketImplemented
	case isFixtureBackedDataReference(*row):
		row.Bucket = BucketImplemented
	case isGeneratedDataReferenceShape(*row):
		row.Bucket = BucketImplemented
	case isFixtureBackedApexType(*row):
		row.Bucket = BucketImplemented
	case isFixtureBackedApexMember(*row):
		row.Bucket = BucketImplemented
	case (row.GladeBehavior == BehaviorSupported || row.GladeBehavior == BehaviorPartial) && row.Evidence == EvidenceNone:
		row.GapClass = GapMissingEvidence
		row.Bucket = BucketGap
	case row.Docs == SourceAbsent && row.Org == SourceAbsent && row.GladeShape != ShapeAbsent:
		row.GapClass = GapStaleGladeShape
		row.Bucket = BucketFailure
	case (row.Docs != SourceAbsent || row.Org != SourceAbsent) && row.GladeShape == ShapeAbsent:
		row.GapClass = GapMissingShape
		row.Bucket = BucketGap
	case (row.MemberName != "" || row.Kind == KindMethod || row.Kind == KindProperty || len(row.Parameters) > 0) && row.GladeShape == ShapeTypeKnown:
		row.GapClass = GapMissingSignature
		row.Bucket = BucketGap
	case row.GladeShape != ShapeAbsent && row.GladeBehavior == BehaviorNone && needsBehavior(*row):
		row.GapClass = GapMissingBehavior
		row.Bucket = BucketGap
	case isPassiveServiceRisk(*row):
		row.GapClass = GapPassiveServiceRisk
		row.Bucket = BucketFailure
	case row.GladeBehavior == BehaviorSupported:
		row.Bucket = BucketImplemented
	case row.GladeBehavior == BehaviorPartial:
		row.Bucket = BucketPartial
	default:
		row.Bucket = BucketGap
	}
}

func (row SurfaceLedgerRow) SignatureChanged() bool {
	return row.Docs == SourceChanged || row.Org == SourceChanged
}

func needsBehavior(row SurfaceLedgerRow) bool {
	return row.Area == AreaRuntime || row.Area == AreaServer || row.Kind == KindMethod || row.Kind == KindResource
}

func isGenericObjectHelper(row SurfaceLedgerRow) bool {
	if row.GladeShape == ShapeAbsent || row.GladeBehavior != BehaviorSupported {
		return false
	}
	switch row.MemberName {
	case "clone", "hashCode", "toString":
		return len(row.Parameters) == 0
	case "equals":
		return len(row.Parameters) == 1 && row.Parameters[0] == "Object"
	default:
		return false
	}
}

func isGenericEnumHelper(row SurfaceLedgerRow) bool {
	if row.GladeShape == ShapeAbsent || row.GladeBehavior != BehaviorSupported {
		return false
	}
	if row.Kind == KindProperty && row.MemberName == strings.ToUpper(row.MemberName) && row.MemberName != "" {
		return true
	}
	switch row.MemberName {
	case "ordinal", "values":
		return len(row.Parameters) == 0
	case "valueOf":
		return len(row.Parameters) == 1 && row.Parameters[0] == "String"
	default:
		return false
	}
}

func isFixtureBackedDataReference(row SurfaceLedgerRow) bool {
	return row.Product == ProductDataRef &&
		row.GladeShape != ShapeAbsent &&
		row.GladeBehavior == BehaviorSupported &&
		row.Evidence != EvidenceNone
}

func isGeneratedDataReferenceShape(row SurfaceLedgerRow) bool {
	return row.Product == ProductDataRef &&
		row.GladeShape == ShapeGenerated &&
		row.GladeBehavior == BehaviorSupported &&
		(row.ShapeSource == SourceStandardSObjectGeneratedShape || hasSource(row.Sources, SourceStandardSObjectGeneratedShape))
}

func isFixtureBackedApexType(row SurfaceLedgerRow) bool {
	return row.Product == ProductApex &&
		row.Kind == KindType &&
		row.GladeShape != ShapeAbsent &&
		row.GladeBehavior == BehaviorSupported &&
		row.Evidence != EvidenceNone
}

func isFixtureBackedApexMember(row SurfaceLedgerRow) bool {
	return row.Product == ProductApex &&
		(row.Kind == KindMethod || row.Kind == KindProperty || row.Kind == KindField) &&
		row.GladeShape != ShapeAbsent &&
		row.GladeBehavior == BehaviorSupported &&
		row.Evidence != EvidenceNone
}

func isPassiveServiceRisk(row SurfaceLedgerRow) bool {
	return row.GladeBehavior == BehaviorPassive && (row.Area == AreaServer || row.Kind == KindResource)
}

func Summarize(rows []SurfaceLedgerRow) LedgerSummary {
	summary := LedgerSummary{
		Gaps:     map[string]int{},
		Failures: map[string]int{},
		Total:    len(rows),
	}
	for _, row := range rows {
		switch row.Bucket {
		case BucketImplemented:
			summary.Implemented++
		case BucketPartial:
			summary.Partial++
		case BucketPassive:
			summary.Passive++
		case BucketExplicitUnsupported:
			summary.ExplicitUnsupported++
		case BucketFailure:
			summary.Failures[row.GapClass]++
		default:
			summary.Gaps[row.GapClass]++
		}
	}
	return summary
}

func shapeRank(state ShapeState) int {
	switch state {
	case ShapeGenerated:
		return 3
	case ShapeSignatureKnown:
		return 2
	case ShapeTypeKnown:
		return 1
	default:
		return 0
	}
}

func behaviorRank(state BehaviorState) int {
	switch state {
	case BehaviorSupported:
		return 4
	case BehaviorPartial:
		return 3
	case BehaviorUnsupported:
		return 2
	case BehaviorPassive:
		return 1
	default:
		return 0
	}
}

func mergeEvidence(a, b EvidenceState) EvidenceState {
	if a == EvidenceFixtureAndOracle || b == EvidenceFixtureAndOracle {
		return EvidenceFixtureAndOracle
	}
	if (a == EvidenceFixture && b == EvidenceOracle) || (a == EvidenceOracle && b == EvidenceFixture) {
		return EvidenceFixtureAndOracle
	}
	if b != "" && b != EvidenceNone {
		return b
	}
	if a != "" {
		return a
	}
	return EvidenceNone
}

func mergeStrings(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range append(append([]string(nil), a...), b...) {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func hasSource(values []string, source string) bool {
	for _, value := range values {
		if value == source {
			return true
		}
	}
	return false
}
