package apextest

import "testing"

func TestImplicitObjectSuperNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "implicit_object_super", "ObjectSuperShapesTest")
}
