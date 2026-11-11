package soql

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func groupedTestOrg(records int) storage.OrgState {
	org := storage.NewOrgState()
	accounts := map[storage.ID]storage.Record{}
	contacts := map[storage.ID]storage.Record{}
	for i := 0; i < 4; i++ {
		id := storage.ID(fmt.Sprintf("001%012d", i))
		accounts[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("Account %d", i))}}
	}
	for i := 0; i < records; i++ {
		id := storage.ID(fmt.Sprintf("003%012d", i))
		account := fmt.Sprintf("001%012d", i%4)
		if i%2 == 0 {
			account += "AAA" // 18-character form groups with the 15-character one
		}
		contacts[id] = storage.Record{ID: id, Object: "Contact", Fields: map[string]storage.Value{
			"LastName":  storage.StringValue(fmt.Sprintf("Name %d", i%7)),
			"AccountId": storage.IDValue(storage.ID(account)),
			"Birthdate": storage.StringValue(fmt.Sprintf("2020-01-%02d", i%3+1)),
			"Level__c":  storage.StringValue([]string{"Low", "Mid", "High"}[i%3]),
		}}
	}
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: accounts}
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
		"LastName":  {APIName: "LastName", Type: storage.FieldString},
		"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		"Birthdate": {APIName: "Birthdate", Type: storage.FieldDate},
		"Level__c":  {APIName: "Level__c", Type: storage.FieldPicklist},
	}}, Records: contacts}
	return org
}

// Grouped fields keep their declared-kind handling (text dates group as
// dates, 18-character Ids group with their 15-character form) with the kind
// resolved once per scan.
func TestGroupedFieldKindsResolveOncePerScan(t *testing.T) {
	org := groupedTestOrg(60)
	for _, text := range []string{
		"SELECT AccountId, COUNT(Id) total FROM Contact GROUP BY AccountId ORDER BY AccountId",
		"SELECT Birthdate, COUNT(Id) total FROM Contact GROUP BY Birthdate ORDER BY Birthdate",
		"SELECT Level__c, LastName, COUNT(Id) total FROM Contact GROUP BY Level__c, LastName ORDER BY Level__c, LastName",
	} {
		query, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ExecuteWithCache(org, query, NewExecutionCache())
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if result.Rows == 0 {
			t.Fatalf("%s: no groups", text)
		}
		for _, record := range result.Records {
			for field, value := range record.Fields {
				kind := declaredGroupFieldKind(org, org.Objects["Contact"].Definition, field)
				if kind.date && value.Kind != storage.ValueDate {
					t.Fatalf("%s: %s grouped as %s, want a date", text, field, value.Kind)
				}
				if kind.id && len(value.ID) != 15 && len(value.ID) != 18 {
					t.Fatalf("%s: %s grouped as %q", text, field, value.ID)
				}
			}
		}
		if query.GroupBy[0] == "AccountId" && result.Rows != 4 {
			t.Fatalf("Id groups = %d, want 4 (15- and 18-character forms share a group)", result.Rows)
		}
	}
	definition := org.Objects["Contact"].Definition
	for field, want := range map[string]groupFieldKind{"AccountId": {id: true}, "Birthdate": {date: true}, "LastName": {}, "Missing__c": {}} {
		if got := declaredGroupFieldKind(org, definition, field); !reflect.DeepEqual(got, want) {
			t.Fatalf("kind(%s) = %+v, want %+v", field, got, want)
		}
	}
}

func BenchmarkGroupByIdScan(b *testing.B) {
	org := groupedTestOrg(2000)
	query, err := Parse("SELECT AccountId, Level__c, COUNT(Id) total FROM Contact GROUP BY AccountId, Level__c")
	if err != nil {
		b.Fatal(err)
	}
	cache := NewExecutionCache()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := ExecuteWithCache(org, query, cache)
		if err != nil || result.Rows != 12 {
			b.Fatalf("rows %d err %v", result.Rows, err)
		}
	}
}
