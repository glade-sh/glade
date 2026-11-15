package dml

import (
	"fmt"
	"testing"
)

func BenchmarkCompanyNameNumbered(b *testing.B) {
	pairs := [][2]string{
		{"Test Account", "Test Account 1"},
		{"Test Account-1", "Test Account 2"},
		{"Test Account 10", "Test Account 1"},
		{"Test Account", "Test Account"},
	}
	for round := 1; round <= 3; round++ {
		for _, variant := range []struct {
			name  string
			match func(string, string, bool) (bool, float64)
		}{{"base", legacyFuzzyCompany}, {"candidate", fuzzyCompany}} {
			b.Run(fmt.Sprintf("round%d_%s", round, variant.name), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					p := pairs[i%len(pairs)]
					variant.match(p[0], p[1], false)
				}
			})
		}
	}
}
