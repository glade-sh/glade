package apextest

import "testing"

func TestTypedNullConstructorStaticOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_constructor_static.json", "constructor-static", 12, 54)
}
