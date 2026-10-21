package apextest

import "testing"

func TestMinePaginationNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "mine_pagination", "MinePaginationTest", []string{"62.0", "67.0"}, 3)
}
