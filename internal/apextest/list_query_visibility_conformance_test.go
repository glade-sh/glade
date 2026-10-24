package apextest

import "testing"

func TestRunAsListQueryVisibilityNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "list_query_visibility", "QueryProbeVisibilityTest", []string{"62.0", "67.0"}, 1)
}

func TestSeeAllDataListQueryNativeObservation(t *testing.T) {
	runNativeObservationProjectWithCarries(t, "list_query_see_all_data", "QueryProbeSeeAllDataTest", []string{"62.0", "67.0"}, 1, map[string]nativeObservationCarry{
		"62.0/seeAllData": {LocalJSON: `{"seeAllDataTrueCount":0}`, Reason: "the local capture project has no pre-existing Account row in its org state"},
		"67.0/seeAllData": {LocalJSON: `{"seeAllDataTrueCount":0}`, Reason: "the local capture project has no pre-existing Account row in its org state"},
	})
}
