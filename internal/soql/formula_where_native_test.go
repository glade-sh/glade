package soql

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSOQLFormulaWhereDirectNativeRows(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/soql_formula_where"
	read := func(name string) []byte {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	var queries []struct{ ID, Query string }
	if err := json.Unmarshal(read("queries.json"), &queries); err != nil || len(queries) != 40 {
		t.Fatalf("native query table: %d queries, %v", len(queries), err)
	}
	reader := csv.NewReader(strings.NewReader(string(read("org.tsv"))))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 241 {
		t.Fatalf("native rows: %d rows, %v", len(rows), err)
	}
	want := map[string]string{}
	for _, row := range rows[1:] {
		if row[1] == "istest" {
			want[row[0]+"/"+row[2]] = row[3]
		}
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	object := org.Objects["Account"]
	for _, name := range []string{"Start__c", "Finish__c"} {
		object.Definition.Fields[name] = storage.Field{APIName: name, Type: storage.FieldDate}
	}
	for _, name := range []string{"StartTime__c", "FinishTime__c"} {
		object.Definition.Fields[name] = storage.Field{APIName: name, Type: storage.FieldDateTime}
	}
	object.Records = map[storage.ID]storage.Record{}
	for i, fields := range []map[string]storage.Value{
		{"Name": storage.StringValue("Arithmetic A"), "AnnualRevenue": storage.DecimalValue("12.5"), "NumberOfEmployees": storage.IntegerValue(2), "Start__c": storage.DateValue("2026-01-01"), "Finish__c": storage.DateValue("2026-01-04"), "StartTime__c": storage.DateTimeValue("2026-01-01T00:00:00Z"), "FinishTime__c": storage.DateTimeValue("2026-01-02T12:00:00Z")},
		{"Name": storage.StringValue("Arithmetic B"), "AnnualRevenue": storage.DecimalValue("3"), "NumberOfEmployees": storage.IntegerValue(5)},
		{"Name": storage.StringValue("Arithmetic C"), "NumberOfEmployees": storage.IntegerValue(4)},
		{"Name": storage.StringValue("Arithmetic D"), "AnnualRevenue": storage.DecimalValue("0")},
	} {
		id := storage.ID([]string{"001000000000001", "001000000000002", "001000000000003", "001000000000004"}[i])
		object.Records[id] = storage.Record{ID: id, Object: "Account", Fields: fields}
	}
	org.Objects["Account"] = object
	// Revisit both lower versions after warming the shared parsed-query cache.
	for _, api := range []string{"68.0", "62.0", "67.0", "68.0"} {
		org.APIVersion = api
		for _, query := range queries {
			t.Run(api+"/direct/"+query.ID, func(t *testing.T) {
				parsed, err := Parse(query.Query)
				var result Result
				if err == nil {
					result, err = ExecuteWithCache(org, parsed, NewExecutionCache())
				}
				got := "System.QueryException"
				if err != nil {
					var queryErr *QueryError
					if !errors.As(err, &queryErr) {
						t.Fatalf("unexpected error kind: %T: %v", err, err)
					}
				} else {
					var names []string
					for _, record := range result.Records {
						names = append(names, record.Fields["Name"].String)
					}
					got = "OK|" + strings.Join(names, ",")
				}
				expected, exists := want[api+"/"+query.ID]
				if !exists {
					t.Fatal("missing native row")
				}
				if !strings.HasPrefix(expected, "OK|") {
					expected, _, _ = strings.Cut(expected, "|")
				}
				if got != expected {
					t.Fatalf("native=%q; direct SOQL=%q", expected, got)
				}
			})
		}
	}
}
