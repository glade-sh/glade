package vm

import "testing"

// BenchmarkStaticWriteback measures repeated static-field writeback on one
// runtime: each CallStatic of Registry.bump writes Count back through
// storeClassValue.
func BenchmarkStaticWriteback(b *testing.B) {
	for _, tc := range []struct {
		name   string
		frozen bool
	}{{"frozen-clone", true}, {"plain", false}} {
		b.Run(tc.name, func(b *testing.B) {
			base := classMapShareBase(b, 2000)
			machine := base
			if tc.frozen {
				machine = base.CloneRuntimeFrozenShared(nil)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := machine.CallStatic("Registry.bump", nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStoreClassValue measures the writeback entry point alone.
func BenchmarkStoreClassValue(b *testing.B) {
	machine := classMapShareBase(b, 2000).CloneRuntimeFrozenShared(nil)
	class, ok := machine.lookupClass("pkg.Registry")
	if !ok {
		b.Fatal("class lookup miss")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine.storeClassValue(class)
	}
}
