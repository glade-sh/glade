package soql

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"testing"
)

func BenchmarkChildJoinValidation(b *testing.B) {
	query, err := Parse("SELECT Id, (SELECT Id FROM Contacts WHERE Id IN (SELECT ContactId FROM Case WHERE ContactId != null)) FROM Account")
	if err != nil {
		b.Fatal(err)
	}
	encoded, err := json.Marshal(query)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("control_sha256=%x", sha256.Sum256(encoded))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := validateSemiAntiQuery(query); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCapturedChildJoinValidation(b *testing.B) {
	source, err := os.ReadFile("testdata/child_join_conjunction/query.soql")
	if err != nil {
		b.Fatal(err)
	}
	query, err := Parse(string(source))
	if err != nil {
		b.Fatal(err)
	}
	encoded, err := json.Marshal(query)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("captured_sha256=%x", sha256.Sum256(encoded))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := validateSemiAntiQuery(query); err != nil {
			b.Fatal(err)
		}
	}
}
