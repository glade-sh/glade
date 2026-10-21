package apextest

import "testing"

func TestCartTestFixtureNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "cart_test_fixture", "CartSliceTest", []string{"62.0", "67.0"}, 2)
}
