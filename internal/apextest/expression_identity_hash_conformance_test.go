package apextest

import "testing"

func TestExpressionIdentityHashOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "expression_identity_hash.json", "ExpressionIdentityHash", 6)
}
