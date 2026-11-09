package apextest

import "testing"

// Families captured in one scratch org for the corpus assertion long tail, tail C.
// Each family has an org.tsv from native Salesforce (anonymous and isTest).

func TestSetHashCodeCustomElementsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "set_hashcode_custom_elements", "SetHashProbeTest", "51.0", "52.0", "62.0", "67.0", "68.0")
}

func TestListAliasAcrossFramesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "list_alias_across_frames", "ListAliasProbeTest", "51.0", "62.0", "67.0")
}

func TestSystemNowBackToBackNativeConformance(t *testing.T) {
	// Native counts vary by a read or two between runs: compare exact values
	// for the stable keys and bounds for the timing counts.
	runInnerTypeNativeConformanceWithComparison(t, "system_now_back_to_back", "NowPairProbeTest", func(t *testing.T, got, want any) {
		g, w := got.(map[string]any), want.(map[string]any)
		for _, key := range []string{"endMinusStartMsIsExact36h", "currentTimeMillisSame", "distinctMsIn2000Reads", "readsPerMsBucket", "more"} {
			if g[key] != w[key] {
				t.Fatalf("%s: native=%v; Glade=%v", key, w[key], g[key])
			}
		}
		for _, key := range []string{"sameInSameStatementPairs", "datetimeNowSame"} {
			if native, glade := w[key].(float64), g[key].(float64); native < 290 || glade < 290 {
				t.Fatalf("%s: native=%v; Glade=%v; back-to-back reads must share a millisecond", key, native, glade)
			}
		}
		for _, key := range []string{"plusOne", "datetimeNowPlusOne"} {
			if native, glade := w[key].(float64), g[key].(float64); native > 10 || glade > 10 {
				t.Fatalf("%s: native=%v; Glade=%v", key, native, glade)
			}
		}
	}, "62.0", "67.0")
}

func TestGlobalDescribeSubscriberBareAliasNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "global_describe_subscriber_bare_alias", "SubscriberDescribeProbeTest", "62.0", "67.0")
}

func TestCMDTMetadataRelationshipAssignmentNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "cmdt_metadata_relationship_assignment", "MdtRelProbeTest", "62.0", "67.0")
}
