package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

var benchCities = []string{"Springfield", "Portland", "Salem", "Eugene", "Boise", "Tucson", "Raleigh", "Madison", "Austin", "Denver"}
var benchFirst = []string{"Nathaniel", "Rosalind", "Marcus", "Priya", "Chen", "Olivia", "Mateo", "Fatima", "Jonas", "Ingrid"}
var benchLast = []string{"Whitfield", "Abernathy", "Okafor", "Raman", "Zhou", "Lindqvist", "Alvarez", "Haddad", "Kowalski", "Brennan"}

var benchSurnamePrefix = []string{"Ash", "Bar", "Cal", "Dun", "Ell", "Far", "Gra", "Hol", "Ing", "Kel", "Lan", "Mor", "Nor", "Os", "Pen", "Quin", "Ros", "Sut", "Tho", "Wil"}
var benchSurnameSuffix = []string{"ton", "ley", "man", "son", "ford", "wick", "worth", "by", "dale", "well", "ham", "field", "stone", "croft", "brook", "ridge", "hurst", "more", "ward", "shaw"}

func benchSurname(i int) string { return benchSurnamePrefix[i%20] + benchSurnameSuffix[(i/20)%20] }

func benchOrg(object string, existing int) *storage.OrgState {
	org := storage.NewOrgState()
	records := make(map[storage.ID]storage.Record, existing)
	prefix := map[string]string{"Account": "001", "Contact": "003"}[object]
	for i := 0; i < existing; i++ {
		id := storage.ID(fmt.Sprintf("%s%012d", prefix, i))
		f := map[string]storage.Value{}
		if object == "Account" {
			f["Name"] = storage.StringValue(fmt.Sprintf("Company %d %s Works", i, benchLast[i%10]))
			f["BillingCity"] = storage.StringValue(benchCities[i%10])
			f["BillingPostalCode"] = storage.StringValue(fmt.Sprintf("%05d", 10000+i%900))
			f["BillingStreet"] = storage.StringValue(fmt.Sprintf("%d Main St", i))
		} else {
			f["FirstName"] = storage.StringValue(benchFirst[i%10])
			f["LastName"] = storage.StringValue(benchSurname(i))
			f["Email"] = storage.StringValue(fmt.Sprintf("u%d@example.com", i))
			f["Phone"] = storage.StringValue(fmt.Sprintf("(503) 555-%04d", i))
		}
		records[id] = storage.Record{ID: id, Object: object, Fields: f}
	}
	org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{}}, Records: records}
	return &org
}

func benchRule(object string) storage.DuplicateRule {
	name := map[string]string{"Account": "Standard_Account_Match_Rule_v1_0", "Contact": "Standard_Contact_Match_Rule_v1_1"}[object]
	return storage.DuplicateRule{Name: "Standard", Object: object, Active: true,
		MatchRules: []storage.MatchingRule{{Name: name, Object: object, Status: "Active"}}}
}

// A realistic insert loop: 200 candidates scanned against 2,000 existing
// records, with a different name, city and ZIP code each.
func BenchmarkStandardAccountInsertLoop(b *testing.B) {
	org, rule := benchOrg("Account", 2000), benchRule("Account")
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		for i := 0; i < 200; i++ {
			c := storage.Record{Object: "Account", Fields: map[string]storage.Value{
				"Name": storage.StringValue(fmt.Sprintf("Candidate %d %s Works", 5000+i, benchLast[i%10])), "BillingCity": storage.StringValue(benchCities[i%10]),
				"BillingPostalCode": storage.StringValue(fmt.Sprintf("%05d", 10000+i%900)), "BillingStreet": storage.StringValue(fmt.Sprintf("%d Elm St", i))}}
			FindDuplicateMatches(org, rule, c, nil)
		}
	}
}

func BenchmarkStandardContactInsertLoop(b *testing.B) {
	org, rule := benchOrg("Contact", 2000), benchRule("Contact")
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		for i := 0; i < 200; i++ {
			c := storage.Record{Object: "Contact", Fields: map[string]storage.Value{
				"FirstName": storage.StringValue(benchFirst[i%10]), "LastName": storage.StringValue(benchSurname(i*7 + 3)),
				"Email": storage.StringValue(fmt.Sprintf("n%d@example.org", i))}}
			FindDuplicateMatches(org, rule, c, nil)
		}
	}
}
