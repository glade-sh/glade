package vm

import (
	"testing"
	"unsafe"
)

// Value is copied on every evaluation, so its size is a performance contract.
// The Datetime week term lives in padding; a new field must find padding too or
// raise this bound on purpose, with benchmarks.
func TestValueSizeGuard(t *testing.T) {
	if size := unsafe.Sizeof(Value{}); size > 312 {
		t.Fatalf("vm.Value is %d bytes; it was 312 on 64-bit platforms", size)
	}
}
