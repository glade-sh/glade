package vm

import (
	"strings"
	"testing"
	"unicode/utf16"
)

var utf16BenchmarkHash int32
var utf16BenchmarkUnits []uint16

func BenchmarkStringUTF16Conversion(b *testing.B) {
	for _, tc := range []struct{ name, text string }{
		{"ascii", strings.Repeat("Abc123", 64)},
		{"unicode", strings.Repeat("Abc😀é", 64)},
	} {
		b.Run(tc.name+"/baseline", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				utf16BenchmarkUnits = utf16.Encode([]rune(tc.text))
			}
		})
		b.Run(tc.name+"/candidate", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				utf16BenchmarkUnits = apexStringUTF16Units(tc.text)
			}
		})
	}
}

func BenchmarkStringUTF16Hash(b *testing.B) {
	for _, tc := range []struct{ name, text string }{
		{"ascii", strings.Repeat("Abc123", 64)},
		{"unicode", strings.Repeat("Abc😀é", 64)},
	} {
		b.Run(tc.name+"/baseline", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var hash int32
				for _, unit := range utf16.Encode([]rune(tc.text)) {
					hash = 31*hash + int32(unit)
				}
				utf16BenchmarkHash = hash
			}
		})
		b.Run(tc.name+"/candidate", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				utf16BenchmarkHash = javaStringHashCode(tc.text)
			}
		})
	}
}
