package dml

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// standardScanField is one field of a standard-rule grid: a text, the same
// text with a different case and edge spaces, and a different text. Each
// value is valid for the field's scorer.
type standardScanField struct {
	name        string
	text, other string
}

var standardScanAccountFields = []standardScanField{
	{"Name", "Acme Widgets", "Zenith Corp"}, {"Website", "acme.com", "zenith.org"}, {"BillingCity", "Springfield", "Riverton"},
	{"BillingPostalCode", "62701", "90210"}, {"BillingStreet", "100 Main St", "77 Birch Lane"}, {"Phone", "(217) 555-0100", "(415) 555-0199"},
}

var standardScanPersonFields = []standardScanField{
	{"LastName", "Hartwell", "Okafor"}, {"FirstName", "Jonathan", "Marcus"}, {"Email", "jon@example.com", "zed@example.org"}, {"Phone", "(217) 555-0100", "(415) 555-0199"},
}

var standardScanLeadFields = []standardScanField{
	{"LastName", "Hartwell", "Okafor"}, {"FirstName", "Jonathan", "Marcus"}, {"Email", "jon@example.com", "zed@example.org"}, {"Phone", "(217) 555-0100", "(415) 555-0199"},
	{"Company", "Acme Widgets", "Zenith Corp"}, {"Title", "Director of Sales", "Chief Engineer"},
	{"Street", "100 Main St", "77 Birch Lane"}, {"City", "Springfield", "Riverton"}, {"PostalCode", "62701", "90210"},
}

// A shape is absent (0), the text (1), the folded text (2) or the other text (3).
func standardScanRecord(object string, fields []standardScanField, shape []int) storage.Record {
	record := storage.Record{Object: object, Fields: map[string]storage.Value{}}
	for i, field := range fields {
		switch shape[i] {
		case 1:
			record.Fields[field.name] = storage.StringValue(field.text)
		case 2:
			record.Fields[field.name] = storage.StringValue("  " + strings.ToUpper(field.text) + " ")
		case 3:
			record.Fields[field.name] = storage.StringValue(field.other)
		case 4:
			record.Fields[field.name] = storage.NullValue()
		case 5:
			record.Fields[field.name] = storage.StringValue("  ")
		case 6:
			record.Fields[field.name] = storage.IntegerValue(42)
		}
	}
	return record
}

func standardScanGrid(fields []standardScanField, shapes []int) (out [][]int) {
	cur := make([]int, len(fields))
	var walk func(i int)
	walk = func(i int) {
		if i == len(fields) {
			out = append(out, append([]int(nil), cur...))
			return
		}
		for _, s := range shapes {
			cur[i] = s
			walk(i + 1)
		}
	}
	walk(0)
	return out
}

// standardRelation names how two values of one field compare: "eq" for the
// same text, "ne" for different text, "cb" when only the candidate is blank,
// "eb" when only the existing record is, "bb" when both are.
func standardScanRelation(c, e int) string {
	blank := func(s int) bool { return s == 0 || s == 4 || s == 5 }
	switch {
	case blank(c) && blank(e):
		return "bb"
	case blank(c):
		return "cb"
	case blank(e):
		return "eb"
	case c == 6 || e == 6:
		return map[bool]string{true: "eq", false: "ne"}[c == e]
	}
	switch {
	case c == 0 && e == 0:
		return "bb"
	case c == 0:
		return "cb"
	case e == 0:
		return "eb"
	case (c == 3) == (e == 3):
		return "eq"
	}
	return "ne"
}

// referenceStandardStructure is the boolean structure of the standard rules as
// the native truth tables show it, over field relations only.
//
// NATIVE Name path and Website path of the standard Account rule over 729 states
// internal/apextest/testdata/conformance/duplicate_standard_rules/org.tsv:55 anonymous 67.0
// NATIVE standard Contact and Lead rule with blank existing values
// internal/apextest/testdata/conformance/duplicate_standard_rules/org.tsv:63 anonymous 67.0
// NATIVE Lead company branch
// internal/apextest/testdata/conformance/duplicate_standard_rules/org.tsv:65 anonymous 67.0
func referenceStandardStructure(object string, rel map[string]string) bool {
	eq := func(f string) bool { return rel[f] == "eq" }
	switch object {
	case "account":
		cz := eq("BillingCity") || eq("BillingPostalCode")
		return (eq("Name") && cz) || (eq("Website") && cz && (eq("BillingStreet") || eq("Phone")))
	case "contact", "lead":
		if !eq("LastName") {
			return false
		}
		switch rel["FirstName"] {
		case "eq":
			if eq("Email") || eq("Phone") {
				return true
			}
			company := eq("Company") && (eq("Title") || eq("Street") && (eq("City") || eq("PostalCode")))
			return object == "lead" && company
		case "cb", "bb":
			return eq("Email")
		}
	}
	return false
}

func standardScanCheck(t *testing.T, object string, fields []standardScanField, candidateShapes, existingShapes [][]int, fixed map[string][2]string) {
	t.Helper()
	pairs, matched, declined := 0, 0, 0
	for _, cs := range candidateShapes {
		candidate := standardScanRecord(object, fields, cs)
		for k, v := range fixed {
			candidate.Fields[k] = storage.StringValue(v[0])
		}
		matcher := standardDuplicateMatcher(nil, object, object, candidate)
		if matcher == nil {
			declined++
		}
		for _, es := range existingShapes {
			existing := standardScanRecord(object, fields, es)
			for k, v := range fixed {
				existing.Fields[k] = storage.StringValue(v[1])
			}
			rel := map[string]string{}
			for i, f := range fields {
				rel[f.name] = standardScanRelation(cs[i], es[i])
			}
			for k, v := range fixed {
				rel[k] = map[bool]string{true: "eq", false: "ne"}[v[0] == v[1]]
			}
			want := referenceStandardStructure(object, rel)
			got := false
			if matcher != nil {
				got, _ = matcher(existing)
			}
			if got != want {
				t.Fatalf("%s candidate %v existing %v: matcher %v, native structure %v", object, candidate.Fields, existing.Fields, got, want)
			}
			pairs++
			if want {
				matched++
			}
		}
	}
	t.Logf("%s: %d pairs, %d matches, %d of %d candidates declined without a scan", object, pairs, matched, declined, len(candidateShapes))
	if matched == 0 {
		t.Fatalf("%s grid matched nothing", object)
	}
}

func same(values map[string]string) map[string][2]string {
	out := map[string][2]string{}
	for k, v := range values {
		out[k] = [2]string{v, v}
	}
	return out
}

// The scan-level matcher answers as the native structure of each standard
// rule for every combination of field shapes on both sides, including the
// candidates it declines to scan for.
func TestStandardDuplicateMatcherFollowsNativeStructure(t *testing.T) {
	account := standardScanGrid(standardScanAccountFields, []int{0, 1, 3})
	standardScanCheck(t, "account", standardScanAccountFields, account, account, nil)
	person := standardScanGrid(standardScanPersonFields, []int{0, 1, 2, 3})
	standardScanCheck(t, "contact", standardScanPersonFields, person, person, nil)
	// Lead: the company path needs every other lead field to be the same text,
	// which the second grid varies; the first fixes company, title and address.
	leadFields := standardScanLeadFields[:4]
	lead := standardScanGrid(leadFields, []int{0, 1, 2, 3})
	standardScanCheck(t, "lead", leadFields, lead, lead, same(map[string]string{"Company": "Acme Widgets", "Title": "Director of Sales", "Street": "100 Main St", "City": "Springfield", "PostalCode": "62701"}))
	company := standardScanGrid(standardScanLeadFields[4:], []int{0, 1, 3})
	standardScanCheck(t, "lead", standardScanLeadFields[4:], company, company, map[string][2]string{"LastName": {"Hartwell", "Hartwell"}, "FirstName": {"Jonathan", "Jonathan"}, "Email": {"jon@example.com", "zed@example.org"}, "Phone": {"(217) 555-0100", "(415) 555-0199"}})
}

// Absent, null, blank and integer values, the folded text of an Account, a
// no-rule object and candidates the scan declines.
func TestStandardDuplicateMatcherFieldShapes(t *testing.T) {
	// Name, website, city, ZIP, street and phone: the folded text joins the grid
	// on the candidate side.
	candidates := standardScanGrid(standardScanAccountFields, []int{0, 1, 2, 3})
	existing := standardScanGrid(standardScanAccountFields, []int{0, 1, 3})
	standardScanCheck(t, "account", standardScanAccountFields, candidates, existing, nil)
	// Contact and Lead: every shape on the name and email fields.
	for _, object := range []string{"contact", "lead"} {
		fields := standardScanPersonFields
		var fixed map[string][2]string
		if object == "lead" {
			fixed = same(map[string]string{"Company": "Acme Widgets", "Title": "Director of Sales", "Street": "100 Main St", "City": "Springfield", "PostalCode": "62701"})
		}
		var shapes [][]int
		for _, s := range standardScanGrid(fields[:3], []int{0, 1, 2, 3, 4, 5, 6}) {
			shapes = append(shapes, append(s, 1))
		}
		standardScanCheck(t, object, fields, shapes, shapes, fixed)
	}
	if standardDuplicateMatcher(nil, "opportunity", "opportunity", storage.Record{Object: "Opportunity", Fields: map[string]storage.Value{"Name": storage.StringValue("x")}}) != nil {
		t.Fatal("an object with no standard rule scans")
	}
	declined := 0
	for _, shape := range standardScanGrid(standardScanPersonFields, []int{0, 1}) {
		if standardDuplicateMatcher(nil, "contact", "contact", standardScanRecord("Contact", standardScanPersonFields, shape)) == nil {
			declined++
		}
	}
	if declined != 8 {
		t.Fatalf("%d contact candidates declined; the 8 without a last name must", declined)
	}
}

// Case-insensitive stored field names resolve through Record.GetField's
// folded fallback on both sides, as before.
func TestStandardDuplicateMatcherFoldsStoredFieldNames(t *testing.T) {
	candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"lastname": storage.StringValue("Doe"), "email": storage.StringValue("doe@example.com")}}
	existing := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LASTNAME": storage.StringValue("doe"), "EMAIL": storage.StringValue("DOE@example.com")}}
	matcher := standardDuplicateMatcher(nil, "contact", "contact", candidate)
	if matcher == nil {
		t.Fatal("no scan")
	}
	// Neither record has a first name: the email address decides.
	if matched, _ := matcher(existing); !matched {
		t.Fatal("folded field names did not match")
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
