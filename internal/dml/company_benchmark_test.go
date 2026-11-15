package dml

import "testing"

func BenchmarkCompanyNameNumbered(b *testing.B) {
	pairs := [][2]string{
		{"Test Account", "Test Account 1"},
		{"Test Account-1", "Test Account 2"},
		{"Test Account 10", "Test Account 1"},
		{"Test Account", "Test Account"},
	}
	for _, variant := range []struct {
		name  string
		match func(string, string, bool) (bool, float64)
	}{{"base", legacyFuzzyCompany}, {"candidate", fuzzyCompany}} {
		b.Run(variant.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				p := pairs[i%len(pairs)]
				variant.match(p[0], p[1], false)
			}
		})
	}
}
