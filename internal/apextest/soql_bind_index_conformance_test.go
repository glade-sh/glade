package apextest

import "testing"

func TestSOQLBindIndexOrgConformance(t *testing.T) {
	runSOQLBindNativeConformance(t, "soql_bind_index.json", "SOQLBindIndex", 45, 360)
}
