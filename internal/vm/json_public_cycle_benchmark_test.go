package vm

import "testing"

func BenchmarkJSONPublicCycleGuard(b *testing.B) {
	small := Map()
	small.Map["value"] = Int(1)
	leaf := List(Int(1), String("ok"))
	for _, tc := range []struct {
		name  string
		value Value
	}{
		{"scalar", Int(1)}, {"small", small}, {"dag", List(leaf, leaf)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			machine := New(nil)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := jsonMarshalNoEscape(machine.jsonFromValueForJSONSerialize(tc.value, false)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
