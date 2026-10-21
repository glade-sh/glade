package apextest

import "testing"

func TestAuthResponseFieldsNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "auth_response_fields", "SurfaceSliceTest", []string{"62.0", "67.0"}, 4)
}
func TestChangeEventReflectionNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "change_event_reflection", "SurfaceSliceTest", []string{"62.0", "67.0"}, 1)
}
func TestConditionalQueryRecordNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "conditional_query_record", "SurfaceSliceTest", []string{"62.0", "67.0"}, 1)
}
