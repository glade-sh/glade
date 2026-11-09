package apextest

import "testing"

func TestCartListRuntimeNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "cart_list_runtime", "CartListSlice", []string{"62.0", "67.0"}, 4)
}
