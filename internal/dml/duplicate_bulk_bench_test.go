package dml_test

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// benchDuplicateOrg builds a Contact org with existing records and one
// duplicate rule on Email, then returns it with the rows to insert.
func benchDuplicateOrg(b *testing.B, existing, batch int, rule storage.DuplicateRule, matching bool) (storage.OrgState, []storage.Record) {
	b.Helper()
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Contact")
	engine := dml.NewEngine(&org)
	seed := make([]storage.Record, existing)
	for i := range seed {
		seed[i] = storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue(fmt.Sprintf("Seed%d", i)), "Email": storage.StringValue(fmt.Sprintf("seed%d@example.invalid", i))}}
	}
	for _, result := range engine.Insert(seed) {
		if !result.Success {
			b.Fatalf("seed: %+v", result)
		}
	}
	match := storage.MatchingRule{Object: "Contact", Name: "Bench_Exact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}
	rule.Name, rule.Object, rule.Active, rule.MatchRules = "BenchRule", "Contact", true, []storage.MatchingRule{match}
	object := org.Objects["Contact"]
	object.Definition.DuplicateRules = []storage.DuplicateRule{rule}
	org.Objects["Contact"] = object
	rows := make([]storage.Record, batch)
	for i := range rows {
		email := fmt.Sprintf("new%d@example.invalid", i)
		if matching && i%2 == 0 {
			email = fmt.Sprintf("seed%d@example.invalid", i)
		}
		rows[i] = storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue(fmt.Sprintf("New%d", i)), "Email": storage.StringValue(email)}}
	}
	return org, rows
}

func runBenchDuplicateInsert(b *testing.B, rule storage.DuplicateRule, matching bool) {
	base, rows := benchDuplicateOrg(b, 5000, 200, rule, matching)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		org := storage.SnapshotRuntimeOrg(&base)
		engine := dml.NewEngine(&org)
		engine.Options.EnforceDuplicateRules = true
		batch := make([]storage.Record, len(rows))
		for j := range rows {
			batch[j] = rows[j].Clone()
		}
		b.StartTimer()
		for _, result := range engine.Insert(batch) {
			if !result.Success {
				b.Fatalf("insert: %+v", result)
			}
		}
	}
}

// BenchmarkDuplicateInsertBulkAllowReport inserts 200 records against 5000
// existing records under an Allow+Report rule; half of the rows match a seed.
func BenchmarkDuplicateInsertBulkAllowReport(b *testing.B) {
	runBenchDuplicateInsert(b, storage.DuplicateRule{ActionOnInsert: "Allow", OperationsOnInsert: []string{"Report"}}, true)
}

// BenchmarkDuplicateInsertBulkAllowReportNoMatch is the same rule with no match.
func BenchmarkDuplicateInsertBulkAllowReportNoMatch(b *testing.B) {
	runBenchDuplicateInsert(b, storage.DuplicateRule{ActionOnInsert: "Allow", OperationsOnInsert: []string{"Report"}}, false)
}

// BenchmarkDuplicateInsertBulkBlockNoMatch is the plain duplicate-rule insert
// path: a Block rule that never matches.
func BenchmarkDuplicateInsertBulkBlockNoMatch(b *testing.B) {
	runBenchDuplicateInsert(b, storage.DuplicateRule{ActionOnInsert: "Block"}, false)
}

// BenchmarkDuplicateInsertBulkAllowMatch is a plain Allow rule whose rows match.
func BenchmarkDuplicateInsertBulkAllowMatch(b *testing.B) {
	runBenchDuplicateInsert(b, storage.DuplicateRule{ActionOnInsert: "Allow"}, true)
}
