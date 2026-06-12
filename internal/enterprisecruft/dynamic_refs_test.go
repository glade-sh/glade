package enterprisecruft

import "testing"

func TestDetectDynamicReferencesFindsTypeForNameAndCustomMetadataRouting(t *testing.T) {
	source := "Type.forName('LegacyFlowAction'); String routed = Feature__mdt.getInstance('Default').Handler__c;"
	got := DetectDynamicReferences(source)
	if !got.DynamicApex || !got.CustomMetadataRouting {
		t.Fatalf("dynamic refs = %#v, want both indicators", got)
	}
}
