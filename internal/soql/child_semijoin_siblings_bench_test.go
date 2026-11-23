package soql

import (
	"crypto/sha256"
	"encoding/json"
	"testing"
)

func BenchmarkChildSemiJoinSiblingShape(b *testing.B) {
	queries := map[string]string{
		"captured":           "SELECT Id, (SELECT Id FROM Contacts WHERE Email != null), (SELECT Id, Name FROM Opportunities WHERE StageName = 'Prospecting' AND Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker') LIMIT 1), (SELECT Id FROM Cases LIMIT 1) FROM Account",
		"supported_neighbor": "SELECT Id, (SELECT Name, StageName FROM Opportunities WHERE Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker')) FROM Account",
	}
	for name, source := range queries {
		b.Run(name, func(b *testing.B) {
			var q Query
			var err error
			b.ReportAllocs()
			for b.Loop() {
				q, err = Parse(source)
			}
			if name == "supported_neighbor" && err != nil {
				b.Fatal(err)
			}
			if name == "captured" && err != nil && err.Error() != "soql: semi-join and anti-join subqueries are allowed only in the main WHERE clause" {
				b.Fatal(err)
			}
			if err != nil {
				b.Logf("outcome: rejected: %v", err)
				return
			}
			encoded, e := json.Marshal(q)
			if e != nil {
				b.Fatal(e)
			}
			b.Logf("outcome sha256: %x", sha256.Sum256(encoded))
		})
	}
}
