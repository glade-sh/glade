package apextest

import (
	"reflect"
	"testing"
)

func TestCaptureCustomPrefixNormalization(t *testing.T) {
	native := map[string]any{"Id": "a00000000000001AAA"}
	local := map[string]any{"Id": "aLa100000000001CAA"}
	if !reflect.DeepEqual(normalizeCaptureIdentities(native), normalizeCaptureIdentities(local)) {
		t.Fatal("custom object prefixes were not mapped within the invocation")
	}
}
