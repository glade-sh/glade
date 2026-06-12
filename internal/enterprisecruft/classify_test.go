package enterprisecruft

import "testing"

func TestClassifyProtectsGlobalPrivateNoRefsAndDynamicRisk(t *testing.T) {
	cases := []struct {
		name string
		in   SymbolFacts
		want Bucket
	}{
		{name: "global protected", in: SymbolFacts{Name: "GlobalApi", Visibility: "global", References: 0}, want: BucketPackageContractDoNotDelete},
		{name: "private no refs", in: SymbolFacts{Name: "DeadPrivate", Visibility: "private", References: 0}, want: BucketSafeDeleteCandidate},
		{name: "dynamic risk", in: SymbolFacts{Name: "Routed", Visibility: "private", References: 0, DynamicReferences: true}, want: BucketReviewDynamicReferenceRisk},
		{name: "runtime before delete", in: SymbolFacts{Name: "DMLWorker", Visibility: "private", References: 0, RuntimeSurface: true}, want: BucketRuntimeCharacterizationNeeded},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.in)
			if got.Bucket != tt.want {
				t.Fatalf("bucket = %q, want %q: %#v", got.Bucket, tt.want, got)
			}
			if tt.in.DynamicReferences && got.Confidence != "low" {
				t.Fatalf("dynamic risk confidence = %q, want low", got.Confidence)
			}
		})
	}
}
