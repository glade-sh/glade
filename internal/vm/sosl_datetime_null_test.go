package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned named rows D01-D17 at API62/67/68 distinguish a null record operand
// from a literal null criterion. NOT preserves the unknown ordered result.
func TestSOSLDatetimeNullOrderedPredicates(t *testing.T) {
	for _, tc := range []struct {
		name, predicate, want  string
		desc, noFixed, invalid bool
	}{
		{name: "greater", predicate: "RecordedAt__c > :cutoff", want: "DLate"},
		{name: "greater equal", predicate: "RecordedAt__c >= :cutoff", want: "CEqual|DLate"},
		{name: "less", predicate: "RecordedAt__c < :cutoff", want: "BEarly"},
		{name: "less equal", predicate: "RecordedAt__c <= :cutoff", want: "BEarly|CEqual"},
		{name: "not greater", predicate: "NOT (RecordedAt__c > :cutoff)", want: "BEarly|CEqual"},
		{name: "not greater equal", predicate: "NOT (RecordedAt__c >= :cutoff)", want: "BEarly"},
		{name: "not less", predicate: "NOT (RecordedAt__c < :cutoff)", want: "CEqual|DLate"},
		{name: "not less equal", predicate: "NOT (RecordedAt__c <= :cutoff)", want: "DLate"},
		{name: "null equality", predicate: "RecordedAt__c = null", want: "ABlank"},
		{name: "nonnull equality", predicate: "RecordedAt__c != null", want: "BEarly|CEqual|DLate"},
		{name: "or true", predicate: "RecordedAt__c > :cutoff OR Name = 'ABlank'", want: "ABlank|DLate"},
		{name: "not or true", predicate: "NOT (RecordedAt__c > :cutoff OR Name = 'ABlank')", want: "BEarly|CEqual"},
		{name: "and true", predicate: "RecordedAt__c > :cutoff AND Name = 'ABlank'"},
		{name: "not and false", predicate: "NOT (RecordedAt__c > :cutoff AND Name = 'DLate')", want: "ABlank|BEarly|CEqual"},
		{name: "nonnull descending", predicate: "RecordedAt__c != null", want: "DLate|CEqual|BEarly", desc: true},
		{name: "no fixed candidates", predicate: "RecordedAt__c > :cutoff", noFixed: true},
		{name: "literal null invalid", predicate: "RecordedAt__c > null", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["SearchMoment__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "SearchMoment__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
					"Id":            {APIName: "Id", Type: storage.FieldID},
					"Name":          {APIName: "Name", Type: storage.FieldString},
					"RecordedAt__c": {APIName: "RecordedAt__c", Type: storage.FieldDateTime},
				}},
				Records: map[storage.ID]storage.Record{},
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			fixed := "Test.setFixedSearchResults(new List<Id>{seed[3].Id,seed[0].Id,seed[1].Id,seed[2].Id});"
			if tc.noFixed {
				fixed = ""
			}
			order := "Name ASC"
			if tc.desc {
				order = "RecordedAt__c DESC"
			}
			query := "FIND {unindexedneedle} IN NAME FIELDS RETURNING SearchMoment__c(Name WHERE " + tc.predicate + " ORDER BY " + order + ")"
			wantType, wantMessage := "null", "null"
			if tc.invalid {
				wantType, wantMessage = "'System.QueryException'", "'invalid operator'"
			}
			source := fmt.Sprintf(`
Datetime cutoff = Datetime.newInstanceGmt(2020,1,2,0,0,0);
List<SearchMoment__c> seed = new List<SearchMoment__c>{
    new SearchMoment__c(Name='ABlank'),
    new SearchMoment__c(Name='BEarly', RecordedAt__c=cutoff.addDays(-1)),
    new SearchMoment__c(Name='CEqual', RecordedAt__c=cutoff),
    new SearchMoment__c(Name='DLate', RecordedAt__c=cutoff.addDays(1))
};
Database.insert(seed, true, AccessLevel.SYSTEM_MODE);
%s
List<String> labels = new List<String>();
String exceptionType;
String message;
try {
    for (SObject record : Search.query('%s', AccessLevel.SYSTEM_MODE)[0]) labels.add((String)record.get('Name'));
} catch (Exception e) { exceptionType=e.getTypeName(); message=e.getMessage(); }
System.assertEquals(%s, exceptionType);
System.assertEquals(%s, message);
System.assertEquals('%s', String.join(labels, '|'));
`, fixed, strings.ReplaceAll(query, "'", "\\'"), wantType, wantMessage, tc.want)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
