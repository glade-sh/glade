package enterprisecruft

import "strings"

type Bucket string

const (
	BucketSafeDeleteCandidate           Bucket = "safe_delete_candidate"
	BucketSafeDeprecateCandidate        Bucket = "safe_deprecate_candidate"
	BucketReviewDynamicReferenceRisk    Bucket = "review_dynamic_reference_risk"
	BucketPackageContractDoNotDelete    Bucket = "package_contract_do_not_delete"
	BucketRuntimeCharacterizationNeeded Bucket = "runtime_characterization_needed"
	BucketTestOnlyCleanup               Bucket = "test_only_cleanup"
	BucketUnknown                       Bucket = "unknown"
)

type SymbolFacts struct {
	Name               string
	Visibility         string
	References         int
	DynamicReferences  bool
	MetadataReferences bool
	IsTest             bool
	RuntimeSurface     bool
}

type Classification struct {
	Bucket         Bucket
	Confidence     string
	Reason         string
	Recommendation string
}

func Classify(facts SymbolFacts) Classification {
	visibility := strings.ToLower(facts.Visibility)
	switch {
	case visibility == "global":
		return Classification{Bucket: BucketPackageContractDoNotDelete, Confidence: "high", Reason: "global symbol is a package contract", Recommendation: "Do not delete without explicit package contract review."}
	case facts.DynamicReferences || facts.MetadataReferences:
		return Classification{Bucket: BucketReviewDynamicReferenceRisk, Confidence: "low", Reason: "dynamic Apex or metadata references reduce static confidence", Recommendation: "Review dynamic routes before deleting or deprecating."}
	case facts.IsTest && facts.References == 0:
		return Classification{Bucket: BucketTestOnlyCleanup, Confidence: "medium", Reason: "unreferenced test-only symbol", Recommendation: "Clean up after confirming no test data contract depends on it."}
	case facts.RuntimeSurface:
		return Classification{Bucket: BucketRuntimeCharacterizationNeeded, Confidence: "medium", Reason: "runtime behavior needs characterization", Recommendation: "Add characterization tests before refactor."}
	case visibility == "public" && facts.References == 0:
		return Classification{Bucket: BucketSafeDeprecateCandidate, Confidence: "medium", Reason: "public symbol has no static references", Recommendation: "Deprecate first; do not safe-delete public API."}
	case visibility == "private" && facts.References == 0:
		return Classification{Bucket: BucketSafeDeleteCandidate, Confidence: "medium", Reason: "private symbol has no static, dynamic, or metadata references", Recommendation: "Candidate for deletion after focused tests pass."}
	default:
		return Classification{Bucket: BucketUnknown, Confidence: "unknown", Reason: "static facts are incomplete", Recommendation: "Review manually."}
	}
}
