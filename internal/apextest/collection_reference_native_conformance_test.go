package apextest

import "testing"

func TestCollectionReferenceNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "collection_reference", "ReferenceProbeTest")
}

func TestCallbackReferenceNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "callback_reference", "CallbackProbeTest")
}
