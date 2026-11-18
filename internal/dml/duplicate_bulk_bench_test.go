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

var benchFirstNames = []string{"James", "Mary", "Robert", "Patricia", "John", "Jennifer", "Michael", "Linda", "David", "Elizabeth", "William", "Barbara", "Richard", "Susan", "Joseph", "Jessica", "Thomas", "Sarah", "Charles", "Karen"}
var benchLastNames = []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson", "White", "Harris"}
var benchCompanyWords = []string{"Acme", "Globex", "Initech", "Umbrella", "Stark", "Wayne", "Wonka", "Hooli", "Vandelay", "Cyberdyne", "Soylent", "Tyrell"}
var benchStreetNames = []string{"Main", "Oak", "Pine", "Maple", "Cedar", "Elm", "Washington", "Lake", "Hill", "Park"}
var benchStreetSuffixes = []string{"St", "Ave", "Rd", "Blvd", "Dr", "Ln"}

func benchLeadFields(i int, variant bool) map[string]storage.Value {
	first := benchFirstNames[i%len(benchFirstNames)]
	last := benchLastNames[(i/len(benchFirstNames))%len(benchLastNames)]
	company := fmt.Sprintf("%s %s %d", benchCompanyWords[i%len(benchCompanyWords)], []string{"Inc", "LLC", "Corp", "Ltd"}[i%4], i/7)
	street := fmt.Sprintf("%d %s %s", 100+i, benchStreetNames[(i/3)%len(benchStreetNames)], benchStreetSuffixes[i%len(benchStreetSuffixes)])
	if variant {
		street = fmt.Sprintf("%d %s %s", 100+i, benchStreetNames[(i/3)%len(benchStreetNames)], map[string]string{"St": "Street", "Ave": "Avenue", "Rd": "Road", "Blvd": "Boulevard", "Dr": "Drive", "Ln": "Lane"}[benchStreetSuffixes[i%len(benchStreetSuffixes)]])
	}
	return map[string]storage.Value{"FirstName": storage.StringValue(first), "LastName": storage.StringValue(last), "Company": storage.StringValue(company), "Street": storage.StringValue(street)}
}

// benchFuzzyOrg builds a Lead org with existing records and one custom fuzzy
// duplicate rule on First Name, Last Name, Company Name and Street.
func benchFuzzyOrg(b *testing.B, existing, batch int, rule storage.DuplicateRule, filter string) (storage.OrgState, []storage.Record) {
	b.Helper()
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Lead")
	engine := dml.NewEngine(&org)
	seed := make([]storage.Record, existing)
	for i := range seed {
		seed[i] = storage.Record{Object: "Lead", Fields: benchLeadFields(i, false)}
	}
	for _, result := range engine.Insert(seed) {
		if !result.Success {
			b.Fatalf("seed: %+v", result)
		}
	}
	match := storage.MatchingRule{Object: "Lead", Name: "Bench_Fuzzy", Status: "Active", BooleanFilter: filter, Items: []storage.MatchingRuleItem{
		{Field: "FirstName", Method: "FirstName", BlankBehavior: "NullNotAllowed"},
		{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"},
		{Field: "Company", Method: "CompanyName", BlankBehavior: "NullNotAllowed"},
		{Field: "Street", Method: "Street", BlankBehavior: "NullNotAllowed"},
	}}
	rule.Name, rule.Object, rule.Active, rule.MatchRules = "BenchFuzzyRule", "Lead", true, []storage.MatchingRule{match}
	object := org.Objects["Lead"]
	object.Definition.DuplicateRules = []storage.DuplicateRule{rule}
	org.Objects["Lead"] = object
	rows := make([]storage.Record, batch)
	for i := range rows {
		fields := benchLeadFields(existing+i, false)
		if i%2 == 0 {
			fields = benchLeadFields(i*13, true)
		}
		rows[i] = storage.Record{Object: "Lead", Fields: fields}
	}
	return org, rows
}

func runBenchFuzzyInsert(b *testing.B, rule storage.DuplicateRule, filter string) {
	base, rows := benchFuzzyOrg(b, 5000, 200, rule, filter)
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

// BenchmarkDuplicateInsertBulkFuzzyAllowReport inserts 200 leads against 5000
// existing leads under an Allow+Report custom rule with fuzzy First Name, Last
// Name, Company Name and Street items (all AND); half of the rows match a seed.
func BenchmarkDuplicateInsertBulkFuzzyAllowReport(b *testing.B) {
	runBenchFuzzyInsert(b, storage.DuplicateRule{ActionOnInsert: "Allow", OperationsOnInsert: []string{"Report"}}, "")
}

// BenchmarkDuplicateInsertBulkFuzzyFilterAllow is the same rule with the
// filter (1 AND 2) OR (3 AND 4) under a plain Allow action.
func BenchmarkDuplicateInsertBulkFuzzyFilterAllow(b *testing.B) {
	runBenchFuzzyInsert(b, storage.DuplicateRule{ActionOnInsert: "Allow"}, "(1 AND 2) OR (3 AND 4)")
}
