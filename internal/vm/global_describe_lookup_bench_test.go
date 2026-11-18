package vm

import (
	"maps"
	"slices"
	"testing"
)

// BenchmarkGlobalDescribeSpecialMapLookup measures get('account') on the
// pristine global describe, a typed holder sharing its entries, and a private
// detached holder, the only kind that pays the per-instance removal check.
func BenchmarkGlobalDescribeSpecialMapLookup(b *testing.B) {
	machine := describeShareUnprimedFor(false)
	pristine := machine.globalDescribeMapValue()
	typed, ok := machine.sharedGlobalDescribeView(pristine, globalDescribeMapType)
	if !ok {
		typed = pristine
	}
	detached := pristine
	detached.Ref = newValueRef()
	detached.Map = maps.Clone(pristine.Map)
	detached.MapKeys = maps.Clone(pristine.MapKeys)
	detached.MapOrder = slices.Clone(pristine.MapOrder)
	detached.mapBackingShared, detached.mapEntriesShared, detached.globalDescribeEntriesCanonical = false, false, false
	key := String("account")
	for _, view := range []struct {
		name  string
		value Value
	}{{"pristine", pristine}, {"typed", typed}, {"detached", detached}} {
		b.Run(view.name, func(b *testing.B) {
			if _, ok := machine.specialMapLookup(view.value, key); !ok {
				b.Fatal("missing account")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				machine.specialMapLookup(view.value, key)
			}
		})
	}
}
