package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func customCompanyRule() storage.DuplicateRule {
	return storage.DuplicateRule{Name: "CustomCompany", Object: "Account", Active: true, MatchRules: []storage.MatchingRule{{
		Name: "CustomCompany", Object: "Account", Status: "Active",
		Items: []storage.MatchingRuleItem{{Field: "Name", Method: "CompanyName", BlankBehavior: "NullNotAllowed"}},
	}}}
}

// A custom Company Name rule scanning 200 candidates against 2,000 existing
// records: numbered names, plain names and hyphenated names.
func BenchmarkCustomCompanyInsertLoop(b *testing.B) {
	org, rule := benchOrg("Account", 2000), customCompanyRule()
	state := org.Objects["Account"]
	i := 0
	for id, record := range state.Records {
		switch i % 3 {
		case 1:
			record.Fields["Name"] = storage.StringValue(benchLast[i%10] + " " + benchSurname(i))
		case 2:
			record.Fields["Name"] = storage.StringValue(benchLast[i%10] + "-" + benchSurname(i) + " Group")
		}
		state.Records[id] = record
		i++
	}
	names := make([]string, 200)
	for i := range names {
		switch i % 3 {
		case 0:
			names[i] = fmt.Sprintf("Company %d %s Works", 5000+i, benchLast[i%10])
		case 1:
			names[i] = benchLast[(i+3)%10] + " " + benchSurname(i*7+3)
		default:
			names[i] = benchLast[i%10] + "-" + benchSurname(i*7+3)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, name := range names {
			FindDuplicateMatches(org, rule, storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}, nil)
		}
	}
}
