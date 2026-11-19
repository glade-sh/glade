package dml

import "testing"

// BenchmarkCompanyNamePairs scores numbered, plain, hyphenated and qualified
// company pairs, matching and not, through the public scorer.
func BenchmarkCompanyNamePairs(b *testing.B) {
	pairs := [][2]string{
		{"Test Account", "Test Account 1"},
		{"Test Account-1", "Test Account 2"},
		{"Test Account 10", "Test Account 1"},
		{"Test Account", "Test Account"},
		{"Qelthorin Freight", "Qelthorin Freight Services"},
		{"Qelthorin-Freight", "Qelthorin Freights"},
		{"Qelthorin Freight/West", "Qelthorin Freight - West"},
		{"Acme Corporation", "Globex Holdings"},
		{"Northwind Traders", "Northwind Trading"},
		{"Zevrula Inc.", "Zevrula SARL"},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := pairs[i%len(pairs)]
		FuzzyScore("CompanyName", p[0], p[1], false)
	}
}
