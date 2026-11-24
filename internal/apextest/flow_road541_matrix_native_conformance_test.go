package apextest

import "testing"

func TestFlowRoad541MatrixNativeConformance(t *testing.T) {
	for _, c := range []struct {
		name string
		rows int
	}{{"criteria", 30}, {"lookup", 8}, {"relationship", 4}} {
		t.Run(c.name, func(t *testing.T) { runFlowCriteriaFixture(t, "flow_road541_matrix/"+c.name, c.rows, nil) })
	}
}
