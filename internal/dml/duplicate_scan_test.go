package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// referenceStandardDuplicateMatch is the per-pair form the matcher replaced:
// every discriminator is read from both records on every comparison.
func referenceStandardDuplicateMatch(lowerObject string, candidate, existing storage.Record) bool {
	switch lowerObject {
	case "account":
		for _, field := range []string{"Name", "BillingStreet", "BillingCity", "BillingPostalCode"} {
			if !duplicateFieldsEqual(candidate, existing, field) {
				return false
			}
		}
		return true
	case "contact", "lead":
		if !duplicateFieldsEqual(candidate, existing, "LastName") {
			return false
		}
		first, firstOK := candidate.GetField("FirstName")
		other, otherOK := existing.GetField("FirstName")
		bothFirst := !duplicateBlank(first, firstOK) && !duplicateBlank(other, otherOK)
		if bothFirst && !duplicateFieldsEqual(candidate, existing, "FirstName") {
			return false
		}
		if duplicateFieldsEqual(candidate, existing, "Email") {
			return true
		}
		return bothFirst && duplicateFieldsEqual(candidate, existing, "Phone")
	}
	return false
}

// Every discriminator shape a field can take for the standard rules: absent,
// explicit null, blank text, text, the same text in another case and with
// surrounding whitespace, different text, and an integer rendered as text.
var duplicateScanFieldShapes = []struct {
	name  string
	value *storage.Value
}{
	{"absent", nil},
	{"null", ptr(storage.NullValue())},
	{"blank", ptr(storage.StringValue("  "))},
	{"text", ptr(storage.StringValue("Acme"))},
	{"folded", ptr(storage.StringValue(" ACME "))},
	{"other", ptr(storage.StringValue("Globex"))},
	{"integer", ptr(storage.IntegerValue(42))},
}

func ptr(value storage.Value) *storage.Value { return &value }

func duplicateScanRecord(object string, fields []string, shape []int) storage.Record {
	record := storage.Record{Object: object, Fields: map[string]storage.Value{}}
	for i, field := range fields {
		if value := duplicateScanFieldShapes[shape[i]].value; value != nil {
			record.Fields[field] = *value
		}
	}
	return record
}

func duplicateScanShapes(fields int, visit func(shape []int)) {
	shape := make([]int, fields)
	var walk func(i int)
	walk = func(i int) {
		if i == fields {
			visit(shape)
			return
		}
		for s := range duplicateScanFieldShapes {
			shape[i] = s
			walk(i + 1)
		}
	}
	walk(0)
}

// The scan-level matcher answers exactly as the per-pair comparison for every
// combination of discriminator shapes on both sides, including the candidates
// it declines to scan for.
func TestStandardDuplicateMatcherMatchesPerPairComparison(t *testing.T) {
	cases := []struct {
		object string
		fields []string
	}{
		{"account", []string{"Name", "BillingStreet", "BillingCity", "BillingPostalCode"}},
		{"contact", []string{"LastName", "FirstName", "Email", "Phone"}},
		{"lead", []string{"LastName", "FirstName", "Email", "Phone"}},
		{"opportunity", []string{"Name"}},
	}
	for _, tc := range cases {
		var candidates, existings []storage.Record
		duplicateScanShapes(len(tc.fields), func(shape []int) {
			candidates = append(candidates, duplicateScanRecord(tc.object, tc.fields, shape))
		})
		existings = candidates
		pairs, matched, declined := 0, 0, 0
		for _, candidate := range candidates {
			matcher := standardDuplicateMatcher(tc.object, candidate)
			if matcher == nil {
				declined++
			}
			for _, existing := range existings {
				want := referenceStandardDuplicateMatch(tc.object, candidate, existing)
				got := matcher != nil && matcher(existing)
				if got != want {
					t.Fatalf("%s candidate %v existing %v: matcher %v, per-pair %v", tc.object, candidate.Fields, existing.Fields, got, want)
				}
				pairs++
				if want {
					matched++
				}
			}
		}
		t.Logf("%s: %d pairs, %d matches, %d of %d candidates declined without a scan", tc.object, pairs, matched, declined, len(candidates))
		if tc.object != "opportunity" && (matched == 0 || declined == 0 || declined == len(candidates)) {
			t.Fatalf("%s grid exercised matched=%d declined=%d of %d; expected both outcomes", tc.object, matched, declined, len(candidates))
		}
	}
}

// Case-insensitive stored field names resolve through Record.GetField's
// folded fallback on both sides, as before.
func TestStandardDuplicateMatcherFoldsStoredFieldNames(t *testing.T) {
	candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"lastname": storage.StringValue("Doe"), "email": storage.StringValue("doe@example.com")}}
	existing := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LASTNAME": storage.StringValue("doe"), "EMAIL": storage.StringValue("DOE@example.com")}}
	matcher := standardDuplicateMatcher("contact", candidate)
	if matcher == nil || !matcher(existing) {
		t.Fatal("folded field names did not match")
	}
	if !referenceStandardDuplicateMatch("contact", candidate, existing) {
		t.Fatal("reference comparison disagrees")
	}
}

func duplicateScanOrg(existing int, withBilling bool) *storage.OrgState {
	org := storage.NewOrgState()
	records := make(map[storage.ID]storage.Record, existing)
	for i := 0; i < existing; i++ {
		id := storage.ID(fmt.Sprintf("001%012d", i))
		fields := map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("Account %d", i))}
		if withBilling {
			fields["BillingStreet"] = storage.StringValue("1 Main St")
			fields["BillingCity"] = storage.StringValue("Springfield")
			fields["BillingPostalCode"] = storage.StringValue("12345")
		}
		records[id] = storage.Record{ID: id, Object: "Account", Fields: fields}
	}
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    records,
	}
	return &org
}

func standardAccountDuplicateRule() storage.DuplicateRule {
	return storage.DuplicateRule{
		Name: "Standard_Account_Duplicate_Rule", Object: "Account", Active: true,
		MatchRules: []storage.MatchingRule{{Name: "Standard_Account_Match_Rule_v1_0", Object: "Account", Status: "Active"}},
	}
}

func BenchmarkFindDuplicateMatchesAccountNameOnly(b *testing.B) {
	org := duplicateScanOrg(5000, false)
	rule := standardAccountDuplicateRule()
	if !storage.IsStandardMatchingRule("Account", rule.MatchRules[0].Name) {
		b.Skip("standard account matching rule name not recognised")
	}
	candidate := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Account 4999")}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, supported := FindDuplicateMatches(org, rule, candidate, nil)
		if !supported || result.HasRecords() {
			b.Fatalf("supported %v records %v", supported, result.HasRecords())
		}
	}
}

func BenchmarkFindDuplicateMatchesAccountFullAddress(b *testing.B) {
	org := duplicateScanOrg(5000, true)
	rule := standardAccountDuplicateRule()
	if !storage.IsStandardMatchingRule("Account", rule.MatchRules[0].Name) {
		b.Skip("standard account matching rule name not recognised")
	}
	candidate := storage.Record{Object: "Account", Fields: map[string]storage.Value{
		"Name": storage.StringValue("account 4999"), "BillingStreet": storage.StringValue("1 Main St"),
		"BillingCity": storage.StringValue("Springfield"), "BillingPostalCode": storage.StringValue("12345"),
	}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, supported := FindDuplicateMatches(org, rule, candidate, nil)
		if !supported || !result.HasRecords() || len(result.Matches[0].Records) != 1 {
			b.Fatalf("supported %v matches %v", supported, result.Matches)
		}
	}
}
