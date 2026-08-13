package vm

import (
	"strings"
	"testing"
)

func TestSelectOptionConstructorRejectsFourArguments(t *testing.T) {
	_, err := New(nil).constructValue(
		"SelectOption",
		[]Value{String("value"), String("label"), Bool(false), Bool(false)},
		nil,
		&Result{},
	)
	if err == nil || !strings.Contains(err.Error(), "SelectOption constructor expects") {
		t.Fatalf("four-argument SelectOption constructor error = %v", err)
	}
}
