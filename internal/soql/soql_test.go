package soql

import (
	"errors"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

func TestSyntheticSystemChildRelationshipVisitorMatchesSliceResult(t *testing.T) {
	definition := storage.ObjectDefinition{
		APIName: "Invoice__c",
		Fields: map[string]storage.Field{
			"OwnerId": {APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User"}},
		},
	}
	fromSlice := syntheticSystemChildRelationships(definition)
	var fromVisitor []storage.Relationship
	visitSyntheticSystemChildRelationships(definition, func(relation storage.Relationship) {
		fromVisitor = append(fromVisitor, relation)
	})
	if !reflect.DeepEqual(fromVisitor, fromSlice) {
		t.Fatalf("visitor relationships = %#v, want %#v", fromVisitor, fromSlice)
	}
}

func TestParseSimpleQuery(t *testing.T) {
	query, err := Parse("SELECT Id, Name FROM Account WHERE Name = 'Acme' WITH SECURITY_ENFORCED ORDER BY Name DESC NULLS LAST LIMIT 10 OFFSET 1 FOR UPDATE")
	if err != nil {
		t.Fatal(err)
	}
	if query.Object != "Account" || len(query.Fields) != 2 || query.Where.Field != "Name" || query.SecurityMode != "SECURITY_ENFORCED" || query.OrderBy != "Name" || !query.OrderDesc || len(query.Order) != 1 || query.Order[0].Nulls != "LAST" || query.Limit != 10 || query.Offset != 1 || !query.ForUpdate {
		t.Fatalf("query = %#v", query)
	}
}

func TestParseSupportsBoundLimitAndOffset(t *testing.T) {
	query, err := Parse("SELECT Id FROM Account LIMIT :limitValue OFFSET :offsetValue")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !query.HasLimit || query.LimitBind != "limitValue" || query.OffsetBind != "offsetValue" {
		t.Fatalf("bound window = %#v", query)
	}
}

func TestParseSupportsMethodCallBindExpression(t *testing.T) {
	query, err := Parse("SELECT FiscalYearStartMonth FROM Organization WHERE Id = :UserInfo.getOrganizationId()")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if query.Where == nil || query.Where.Value.Kind != storage.ValueID || string(query.Where.Value.ID) != ":UserInfo.getOrganizationId()" {
		t.Fatalf("where = %#v", query.Where)
	}
}

func TestParseRootObjectAlias(t *testing.T) {
	query, err := Parse("SELECT a.Id FROM Account a WHERE a.Name = 'Acme' ORDER BY a.Name LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if query.Object != "Account" || len(query.Fields) != 1 || query.Fields[0] != "Id" || query.Where.Field != "Name" || query.OrderBy != "Name" {
		t.Fatalf("query = %#v", query)
	}
}

func TestParseBackslashEscapedSOQLStringLiteral(t *testing.T) {
	query, err := Parse("SELECT Id FROM Account WHERE Name = 'Bob\\'s Shop'")
	if err != nil {
		t.Fatal(err)
	}
	if query.Where == nil || query.Where.Value.String != "Bob's Shop" {
		t.Fatalf("where = %#v", query.Where)
	}

	query, err = Parse("SELECT Id FROM Account WHERE Name = 'C:\\Trail'")
	if err != nil {
		t.Fatal(err)
	}
	if query.Where == nil || query.Where.Value.String != `C:\Trail` {
		t.Fatalf("path where = %#v", query.Where)
	}
}

func TestParseSOQLSkipsComments(t *testing.T) {
	query, err := Parse(`SELECT
  Name,
  // HiddenRevenue__c,
  /* HiddenCost__c, */
  TotalRevenue__c
FROM Event__c`)
	if err != nil {
		t.Fatal(err)
	}
	if query.Object != "Event__c" || len(query.Fields) != 2 || query.Fields[0] != "Name" || query.Fields[1] != "TotalRevenue__c" {
		t.Fatalf("query = %#v", query)
	}
}

func TestIndexedCandidateIDsUsesIndexForOrAndInBranches(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Indexes: []storage.IndexDefinition{{
				Name:   "Account.Rating",
				Object: "Account",
				Fields: []string{"Rating"},
			}},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Rating": storage.StringValue("Hot")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Rating": storage.StringValue("Warm")}},
			"001000000000003": {ID: "001000000000003", Object: "Account", Fields: map[string]storage.Value{"Rating": storage.StringValue("Cold")}},
		},
	}
	storage.RebuildIndexes(&org)
	object := org.Objects["Account"]

	tests := []struct {
		name  string
		where Condition
		want  []string
	}{
		{
			name: "or",
			where: Condition{Or: []Condition{
				{Field: "Rating", Op: "=", Value: storage.StringValue("Hot")},
				{Field: "Rating", Op: "=", Value: storage.StringValue("Cold")},
			}},
			want: []string{"001000000000001", "001000000000003"},
		},
		{
			name:  "in",
			where: Condition{Field: "Rating", Op: "IN", Values: []storage.Value{storage.StringValue("Warm"), storage.StringValue("Cold")}},
			want:  []string{"001000000000002", "001000000000003"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := indexedCandidateIDs(object, &tt.where, false)
			if !ok {
				t.Fatal("condition did not use index")
			}
			gotText := make([]string, 0, len(got))
			for _, id := range got {
				gotText = append(gotText, string(id))
			}
			sort.Strings(gotText)
			if !reflect.DeepEqual(gotText, tt.want) {
				t.Fatalf("candidate IDs = %#v, want %#v", gotText, tt.want)
			}
		})
	}
}

func TestExecuteUsingScopeEverythingReturnsVisibleRows(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Beta")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, Name FROM Account USING SCOPE everything ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Records[0].ID != "001000000000001" || result.Records[1].ID != "001000000000002" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCachedParsedQueryPlainWhereCanBeExecutedTwice(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	input := "SELECT Id FROM Account WHERE Name = 'Acme'"
	first, err := ParseAt(input, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseAt(input, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Where == nil || second.Where == nil {
		t.Fatal("parsed query missing where condition")
	}
	if first.Where == second.Where {
		t.Fatal("cache hit reused top-level where pointer")
	}
	if first.Where.Field != second.Where.Field || first.Where.Value.String != second.Where.Value.String {
		t.Fatalf("cache hit where = %#v, want %#v", second.Where, first.Where)
	}
}

func TestCalculatedFieldValuePlainFieldDoesNotAllocate(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{
		APIName: "Account",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	}
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}}
	if _, ok := calculatedFieldValue(org, definition, record, "Name"); ok {
		t.Fatal("plain field should not resolve as calculated")
	}
	allocs := testing.AllocsPerRun(1000, func() {
		calculatedFieldValue(org, definition, record, "Name")
	})
	if allocs != 0 {
		t.Fatalf("allocs = %.2f, want 0", allocs)
	}
}

func TestParseForView(t *testing.T) {
	query, err := Parse("SELECT Id, Name FROM Account LIMIT 1 FOR VIEW")
	if err != nil {
		t.Fatal(err)
	}
	if query.Object != "Account" || query.Limit != 1 || query.ForUpdate || query.ForReference || !query.ForView {
		t.Fatalf("query = %#v", query)
	}

	query, err = Parse("SELECT Id, Name FROM Account LIMIT 1 FOR REFERENCE")
	if err != nil {
		t.Fatal(err)
	}
	if query.Object != "Account" || query.Limit != 1 || query.ForUpdate || query.ForView || !query.ForReference {
		t.Fatalf("reference query = %#v", query)
	}
}

func TestParseNotEqualAngleOperatorBeforeNull(t *testing.T) {
	query, err := Parse("SELECT Id FROM CartItem__c WHERE OrderItem__c <> NULL")
	if err != nil {
		t.Fatal(err)
	}
	if query.Where.Op != "!=" || query.Where.Value.Kind != storage.ValueNull {
		t.Fatalf("where = %#v, want != null", query.Where)
	}
}

func TestExecuteWhereIncludesExcludesMatchesMultiSelectPicklistValues(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Product__c",
			Fields: map[string]storage.Field{
				"Name":            {APIName: "Name", Type: storage.FieldString},
				"PriceClasses__c": {APIName: "PriceClasses__c", Type: storage.FieldMultiPicklist, DisplayType: "MULTIPICKLIST"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Product__c",
				Fields: map[string]storage.Value{
					"Name":            storage.StringValue("Member Shirt"),
					"PriceClasses__c": storage.StringValue("Member;VIP"),
				},
			},
			"a01000000000002": {
				ID:     "a01000000000002",
				Object: "Product__c",
				Fields: map[string]storage.Value{
					"Name":            storage.StringValue("Public Hat"),
					"PriceClasses__c": storage.StringValue("Public"),
				},
			},
			"a01000000000003": {
				ID:     "a01000000000003",
				Object: "Product__c",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Blank Mug"),
				},
			},
		},
	}

	query, err := Parse("SELECT Id FROM Product__c WHERE PriceClasses__c INCLUDES('Member','Public')")
	if err != nil {
		t.Fatal(err)
	}
	if query.Where.Op != "INCLUDES" || len(query.Where.Values) != 2 {
		t.Fatalf("where = %#v, want INCLUDES two operands", query.Where)
	}
	result, err := Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Records[0].ID != "a01000000000001" || result.Records[1].ID != "a01000000000002" {
		t.Fatalf("records = %#v, want member and public products", result.Records)
	}

	query, err = Parse("SELECT Id FROM Product__c WHERE PriceClasses__c EXCLUDES('Member')")
	if err != nil {
		t.Fatal(err)
	}
	if query.Where.Op != "EXCLUDES" || query.Where.Value.String != "Member" {
		t.Fatalf("where = %#v, want EXCLUDES Member", query.Where)
	}
	result, err = Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Records[0].ID != "a01000000000002" || result.Records[1].ID != "a01000000000003" {
		t.Fatalf("records = %#v, want public and blank products", result.Records)
	}

	if _, err := ParseAndExecute(org, "SELECT Id FROM Product__c WHERE Name INCLUDES('Member')"); err == nil || !strings.Contains(err.Error(), "requires multi-select picklist field") {
		t.Fatalf("text field INCLUDES error = %v, want multi-select picklist error", err)
	}
}

func TestParseCountQuery(t *testing.T) {
	query, err := Parse("SELECT COUNT() FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	if !query.Count || len(query.Fields) != 1 || query.Fields[0] != "COUNT()" {
		t.Fatalf("query = %#v", query)
	}
}

func TestParseIgnoresEmptyGeneratedFieldEntries(t *testing.T) {
	query, err := Parse("SELECT Id,,Name,(SELECT Id,,Name FROM Contacts) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(query.Fields, ","); got != "Id,Name" {
		t.Fatalf("fields = %q", got)
	}
	if len(query.ChildQueries) != 1 {
		t.Fatalf("child queries = %#v", query.ChildQueries)
	}
	if got := strings.Join(query.ChildQueries[0].Query.Fields, ","); got != "Id,Name" {
		t.Fatalf("child fields = %q", got)
	}
}

func TestParseQualifiedChildRelationshipSubquery(t *testing.T) {
	query, err := Parse("SELECT Id, (SELECT LastName FROM Account . Contacts) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	if len(query.ChildQueries) != 1 {
		t.Fatalf("child queries = %#v", query.ChildQueries)
	}
	if got := query.ChildQueries[0].Relationship; got != "Contacts" {
		t.Fatalf("child relationship = %q", got)
	}
	if got := query.ChildQueries[0].Query.Object; got != "Account.Contacts" {
		t.Fatalf("child query object = %q", got)
	}
}

func TestParseEmptyInList(t *testing.T) {
	query, err := Parse("SELECT Id FROM Account WHERE Id IN ()")
	if err != nil {
		t.Fatal(err)
	}
	if query.Where.Op != "IN" || len(query.Where.Values) != 0 {
		t.Fatalf("where = %#v", query.Where)
	}
}

func TestIsSOSLFind(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{input: "FIND {Acme} IN ALL FIELDS RETURNING Account(Id)", want: true},
		{input: " \n\tfind 'Acme' IN ALL FIELDS", want: true},
		{input: "SELECT Id FROM Account WHERE Name = 'Find Me'", want: false},
		{input: "", want: false},
	}
	for _, tc := range cases {
		if got := IsSOSLFind(tc.input); got != tc.want {
			t.Fatalf("IsSOSLFind(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestExecuteResolvesLowercaseIdAsStandardField(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Acme"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT id FROM Account WHERE id = '001000000000001'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	value := result.Records[0].Fields["Id"]
	if value.Kind != storage.ValueID || value.ID != "001000000000001" {
		t.Fatalf("Id field = %#v", value)
	}

	result, err = ParseAndExecute(org, "SELECT id FROM Account WHERE id IN ('001000000000001AAA')")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("18-character Id literal rows = %d", result.Rows)
	}

	result, err = ParseAndExecute(org, "SELECT id FROM Account WHERE id = 001000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("numeric Id literal rows = %d", result.Rows)
	}
}

func TestExecuteProjectsOrganizationTimeZoneDefault(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Organization"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Organization",
			Fields: map[string]storage.Field{
				"TimeZoneSidKey": {APIName: "TimeZoneSidKey", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"00D000000000001": {
				ID:     "00D000000000001",
				Object: "Organization",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Local Org"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, TimeZoneSidKey FROM Organization")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	value := result.Records[0].Fields["TimeZoneSidKey"]
	if value.Kind != storage.ValueString || value.String != "UTC" {
		t.Fatalf("TimeZoneSidKey = %#v", value)
	}
}

func TestExecuteAggregateAliasWithSystemReferenceGroupBy(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["SampleSpecialty__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "SampleSpecialty__c",
			Fields: map[string]storage.Field{
				"Type__c": {APIName: "Type__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "SampleSpecialty__c",
				Fields: map[string]storage.Value{
					"Type__c": storage.StringValue("Primary"),
				},
				System: storage.SystemFields{
					LastModifiedByID: "005000000000001",
				},
			},
			"a01000000000002": {
				ID:     "a01000000000002",
				Object: "SampleSpecialty__c",
				Fields: map[string]storage.Value{
					"Type__c": storage.StringValue("Secondary"),
				},
				System: storage.SystemFields{
					LastModifiedByID: "005000000000001",
				},
			},
		},
	}

	query, err := Parse("SELECT LastModifiedById provider, Type__c type, COUNT(Id) total FROM SampleSpecialty__c WHERE LastModifiedById IN ('005000000000001') AND Id NOT IN ('a01000000000001') AND Type__c != 'Additional' GROUP BY LastModifiedById, Type__c")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("records=%d want 1 (%#v)", len(result.Records), result.Records)
	}
	row := result.Records[0]
	if got := row.Fields["provider"]; got.Kind != storage.ValueID || got.ID != "005000000000001" {
		t.Fatalf("provider=%#v", got)
	}
	if got := row.Fields["type"]; got.Kind != storage.ValueString || got.String != "Secondary" {
		t.Fatalf("type=%#v", got)
	}
	if got := row.Fields["total"]; got.Kind != storage.ValueInteger || got.Integer != 1 {
		t.Fatalf("total=%#v", got)
	}

	query, err = Parse("SELECT LastModifiedBy.Id provider, Type__c type, COUNT(Id) total FROM SampleSpecialty__c WHERE LastModifiedBy.Id IN ('005000000000001') AND Id NOT IN ('a01000000000001') AND Type__c != 'Additional' GROUP BY LastModifiedBy.Id, Type__c")
	if err != nil {
		t.Fatal(err)
	}
	result, err = Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("relationship records=%d want 1 (%#v)", len(result.Records), result.Records)
	}
	row = result.Records[0]
	if got := row.Fields["provider"]; got.Kind != storage.ValueID || got.ID != "005000000000001" {
		t.Fatalf("relationship provider=%#v", got)
	}
}

func TestExecuteFieldReferencesAreCaseInsensitive(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"name": storage.StringValue("Acme"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT NAME FROM account WHERE nAmE = 'Acme'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	value := result.Records[0].Fields["Name"]
	if value.Kind != storage.ValueString || value.String != "Acme" {
		t.Fatalf("Name field = %#v", value)
	}
}

func TestExecuteComparesCustomObjectIDsCaseInsensitively(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["MembershipType__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "MembershipType__c",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a1N000000000001": {
				ID:     "a1N000000000001",
				Object: "MembershipType__c",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Individual"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id FROM MembershipType__c WHERE Id IN ('a1n000000000001')")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("lowercase custom Id rows = %d", result.Rows)
	}
}

func TestExecuteEmailTemplateStandardObjectQuery(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "EmailTemplate")

	result, err := ParseAndExecute(org, "SELECT Id, DeveloperName, IsActive, Name, NamespacePrefix FROM EmailTemplate WHERE DeveloperName = 'ExampleEmailVerify' ORDER BY NamespacePrefix NULLS FIRST")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 0 {
		t.Fatalf("records = %d, want 0", len(result.Records))
	}
}

func TestExecuteProjectsQueriedSystemFieldsAsNullWhenUnset(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Acme"),
				},
			},
		},
	}
	result, err := ParseAndExecute(org, "SELECT Id, LastModifiedById FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(result.Records))
	}
	value, ok := result.Records[0].GetField("LastModifiedById")
	if !ok || value.Kind != storage.ValueNull {
		t.Fatalf("LastModifiedById = %#v, %v; want projected null", value, ok)
	}
}

func TestExecuteCustomMetadataDeveloperNameField(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{
		APIName:  "Feature__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields: map[string]storage.Field{
			"Enabled__c": {APIName: "Enabled__c", Type: storage.FieldBoolean},
		},
	}
	storage.EnsureStandardObjectFields(&definition)
	org.Objects["Feature__mdt"] = storage.ObjectState{
		Definition: definition,
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Feature__mdt",
				Fields: map[string]storage.Value{
					"DeveloperName": storage.StringValue("Default"),
					"Enabled__c":    storage.BooleanValue(true),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, DeveloperName FROM Feature__mdt WHERE DeveloperName = 'Default'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["DeveloperName"].String != "Default" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteCustomMetadataQualifiedApiNameSystemField(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Feature__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "Feature__mdt",
			Metadata: map[string]string{"kind": "customMetadata"},
			Fields: map[string]storage.Field{
				"Enabled__c": {APIName: "Enabled__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Feature__mdt",
				Fields: map[string]storage.Value{
					"DeveloperName":    storage.StringValue("Default"),
					"QualifiedApiName": storage.StringValue("pkg__Default"),
					"Enabled__c":       storage.BooleanValue(true),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, QualifiedApiName FROM Feature__mdt WHERE QualifiedApiName = 'pkg__Default'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["QualifiedApiName"].String != "pkg__Default" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecutePartialCustomMetadataAllowsCustomFields(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["pkg__Feature__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Feature__mdt",
			Fields: map[string]storage.Field{
				"Known__c": {APIName: "Known__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "pkg__Feature__mdt",
				Fields: map[string]storage.Value{
					"Known__c": storage.StringValue("yes"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, QualifiedApiName, Language, pkg__Missing__c FROM pkg__Feature__mdt")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["pkg__Missing__c"].Kind != storage.ValueNull {
		t.Fatalf("result = %#v", result)
	}
	if result.Records[0].Fields["Language"].Kind != storage.ValueNull {
		t.Fatalf("Language = %#v", result.Records[0].Fields["Language"])
	}
}

func TestExecutePartialCustomObjectRequiresLookupMetadataForRelationshipFields(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["pkg__Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "pkg__Line__c", Fields: map[string]storage.Field{}},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "pkg__Line__c", Fields: map[string]storage.Value{}},
		},
	}

	if _, err := ParseAndExecute(org, "SELECT pkg__Parent__r.pkg__ExternalId__c, pkg__Parent__r.RecordType.Name FROM pkg__Line__c"); err == nil || !strings.Contains(err.Error(), "pkg__Parent__r.pkg__ExternalId__c") {
		t.Fatalf("err = %v, want missing relationship field metadata", err)
	}
	if _, err := ParseAndExecute(org, "SELECT pkg__Line__c.pkg__Parent__r.Name FROM pkg__Line__c"); err == nil || !strings.Contains(err.Error(), "pkg__Parent__r.Name") {
		t.Fatalf("err = %v, want missing relationship field metadata", err)
	}
}

func TestExecuteProjectsNestedParentRelationshipFields(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Membership__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Membership__c",
			Fields: map[string]storage.Field{
				"Id":         {APIName: "Id", Type: storage.FieldID},
				"Pending__c": {APIName: "Pending__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Membership__c",
				Fields: map[string]storage.Value{
					"Pending__c": storage.BooleanValue(true),
				},
			},
		},
	}
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Id":            {APIName: "Id", Type: storage.FieldID},
				"Membership__c": {APIName: "Membership__c", Type: storage.FieldReference, ReferenceTo: []string{"Membership__c"}},
			},
			Relations: []storage.Relationship{
				{Field: "Membership__c", ParentObjects: []string{"Membership__c"}, ParentRelationship: "Membership__r"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Membership__c": storage.IDValue("a01000000000001"),
				},
			},
		},
	}
	org.Objects["Affiliation__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Affiliation__c",
			Fields: map[string]storage.Field{
				"Id":               {APIName: "Id", Type: storage.FieldID},
				"ParentAccount__c": {APIName: "ParentAccount__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{
				{Field: "ParentAccount__c", ParentObjects: []string{"Account"}, ParentRelationship: "ParentAccount__r"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a02000000000001": {
				ID:     "a02000000000001",
				Object: "Affiliation__c",
				Fields: map[string]storage.Value{
					"ParentAccount__c": storage.IDValue("001000000000001"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT ParentAccount__r.Membership__r.Pending__c FROM Affiliation__c")
	if err != nil {
		t.Fatal(err)
	}
	got := result.Records[0].Fields["ParentAccount__r.Membership__r.Pending__c"]
	if got.Kind != storage.ValueBoolean || !got.Boolean {
		t.Fatalf("nested relationship field = %#v", got)
	}
}

func TestExecuteCustomMetadataRelationshipProjection(t *testing.T) {
	org := customMetadataRelationshipOrg()

	result, err := ParseAndExecute(org, "SELECT DeveloperName, Target__r.QualifiedApiName FROM Binding__mdt WHERE Target__r.QualifiedApiName = 'Target'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	value := result.Records[0].Fields["Target__r.QualifiedApiName"]
	if value.Kind != storage.ValueString || value.String != "Target" {
		t.Fatalf("relationship projection = %#v", value)
	}
}

func TestExecuteSOQLOverLoadedCustomMetadataRecords(t *testing.T) {
	sch := schema.Schema{Objects: []schema.Object{
		{
			Name: "Target__mdt",
			Fields: []schema.Field{
				{Name: "Description__c", Type: "Text"},
			},
		},
		{
			Name: "Feature__mdt",
			Fields: []schema.Field{
				{Name: "Enabled__c", Type: "Checkbox"},
				{Name: "Target__c", Type: "MetadataRelationship", ReferenceTo: []string{"Target__mdt"}},
			},
		},
	}, CustomMetadataRecords: []schema.CustomMetadataRecord{
		{FullName: "Feature.Default", ObjectName: "Feature__mdt", DeveloperName: "Default", Values: []schema.CustomMetadataValue{{Field: "Enabled__c", Value: "true"}, {Field: "Target__c", Value: "Target"}}},
		{FullName: "Target.Target", ObjectName: "Target__mdt", DeveloperName: "Target", Values: []schema.CustomMetadataValue{{Field: "Description__c", Value: "Target row"}}},
	}}
	org := storage.NewOrgState()
	registry := sobject.BuildDescribeRegistry(sch)
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe), Records: map[storage.ID]storage.Record{}}
	}
	pending := append([]schema.CustomMetadataRecord(nil), sch.CustomMetadataRecords...)
	for len(pending) > 0 {
		next := make([]schema.CustomMetadataRecord, 0, len(pending))
		progressed := false
		for _, record := range pending {
			if err := storage.ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{record}); err != nil {
				next = append(next, record)
				continue
			}
			progressed = true
		}
		if !progressed {
			break
		}
		pending = next
	}

	result, err := ParseAndExecute(org, "SELECT DeveloperName, Enabled__c, Target__r.Description__c FROM Feature__mdt WHERE Target__r.QualifiedApiName = 'Target'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	record := result.Records[0]
	if record.Fields["DeveloperName"].String != "Default" || !record.Fields["Enabled__c"].Boolean || record.Fields["Target__r.Description__c"].String != "Target row" {
		t.Fatalf("record = %#v", record)
	}
}

func TestExecuteCustomMetadataChildRelationshipUsesLocalQualifiedNames(t *testing.T) {
	sch := schema.Schema{Objects: []schema.Object{
		{
			Name: "StateConfiguration__mdt",
			Fields: []schema.Field{
				{Name: "DeveloperName", Type: "Text"},
				{Name: "MasterLabel", Type: "Text"},
				{Name: "Language", Type: "Text"},
				{Name: "NamespacePrefix", Type: "Text"},
				{Name: "Label", Type: "Text"},
				{Name: "IsActive__c", Type: "Checkbox"},
				{Name: "SupportedStates__c", Type: "Text"},
			},
		},
		{
			Name: "StateTransition__mdt",
			Fields: []schema.Field{
				{Name: "IsActive__c", Type: "Checkbox"},
				{Name: "FromStates__c", Type: "Text"},
				{Name: "ToState__c", Type: "Text"},
				{Name: "StateConfiguration__c", Type: "MetadataRelationship", ReferenceTo: []string{"StateConfiguration__mdt"}, RelationshipName: "StateConfiguration__r", ChildRelationshipName: "StateTransitions__r"},
			},
		},
		{
			Name: "StateTransitionCallback__mdt",
			Fields: []schema.Field{
				{Name: "IsActive__c", Type: "Checkbox"},
				{Name: "ActionName__c", Type: "Text"},
				{Name: "ClassName__c", Type: "Text"},
				{Name: "SortOrder__c", Type: "Number"},
				{Name: "StateConfiguration__c", Type: "MetadataRelationship", ReferenceTo: []string{"StateConfiguration__mdt"}, RelationshipName: "StateConfiguration__r", ChildRelationshipName: "StateTransitionCallbacks__r"},
				{Name: "TriggeringTransition__c", Type: "MetadataRelationship", ReferenceTo: []string{"StateTransition__mdt"}, RelationshipName: "TriggeringTransition__r", ChildRelationshipName: "StateTransitionCallbacks__r"},
			},
		},
	}, CustomMetadataRecords: []schema.CustomMetadataRecord{
		{FullName: "StateConfiguration.OrderGraph", ObjectName: "StateConfiguration__mdt", DeveloperName: "OrderGraph", Values: []schema.CustomMetadataValue{{Field: "MasterLabel", Value: "Order Graph"}, {Field: "IsActive__c", Value: "true"}, {Field: "SupportedStates__c", Value: "Cart,Pro forma"}}},
		{FullName: "StateConfiguration.PaymentGraph", ObjectName: "StateConfiguration__mdt", DeveloperName: "PaymentGraph", Values: []schema.CustomMetadataValue{{Field: "MasterLabel", Value: "Payment Graph"}, {Field: "IsActive__c", Value: "true"}, {Field: "SupportedStates__c", Value: "Intent,Authorized"}}},
		{FullName: "StateTransition.order_submit_as_proforma", ObjectName: "StateTransition__mdt", DeveloperName: "order_submit_as_proforma", Values: []schema.CustomMetadataValue{{Field: "IsActive__c", Value: "true"}, {Field: "FromStates__c", Value: "Cart"}, {Field: "ToState__c", Value: "Pro forma"}, {Field: "StateConfiguration__c", Value: "OrderGraph"}}},
		{FullName: "StateTransition.payment_authorize", ObjectName: "StateTransition__mdt", DeveloperName: "payment_authorize", Values: []schema.CustomMetadataValue{{Field: "IsActive__c", Value: "true"}, {Field: "FromStates__c", Value: "Intent"}, {Field: "ToState__c", Value: "Authorized"}, {Field: "StateConfiguration__c", Value: "PaymentGraph"}}},
		{FullName: "StateTransitionCallback.order_convert_carts_to_orders", ObjectName: "StateTransitionCallback__mdt", DeveloperName: "order_convert_carts_to_orders", Values: []schema.CustomMetadataValue{{Field: "IsActive__c", Value: "true"}, {Field: "ActionName__c", Value: "convert_carts_to_orders"}, {Field: "ClassName__c", Value: "ConvertCartsToOrdersCallback"}, {Field: "SortOrder__c", Value: "0.0"}, {Field: "StateConfiguration__c", Value: "OrderGraph"}, {Field: "TriggeringTransition__c", Value: "order_submit_as_proforma"}}},
	}}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	registry := sobject.BuildDescribeRegistry(sch)
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe), Records: map[storage.ID]storage.Record{}}
	}
	if err := storage.ApplyCustomMetadataRecords(&org, sch.CustomMetadataRecords); err != nil {
		t.Fatal(err)
	}

	result, err := ParseAndExecute(org, "SELECT QualifiedApiName, (SELECT QualifiedApiName, FromStates__c FROM StateTransitions__r WHERE IsActive__c = TRUE) FROM StateConfiguration__mdt WHERE QualifiedApiName = 'OrderGraph'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	children := result.Records[0].Children["StateTransitions__r"]
	if len(children) != 1 {
		t.Fatalf("children = %#v", result.Records[0].Children)
	}
	if got := children[0].Fields["QualifiedApiName"].String; got != "order_submit_as_proforma" {
		t.Fatalf("child QualifiedApiName = %q", got)
	}

	result, err = ParseAndExecute(org, "SELECT QualifiedApiName, (SELECT QualifiedApiName, ActionName__c, TriggeringTransition__r.QualifiedApiName FROM StateTransitionCallbacks__r WHERE IsActive__c = TRUE) FROM StateConfiguration__mdt WHERE QualifiedApiName = 'OrderGraph'")
	if err != nil {
		t.Fatal(err)
	}
	callbacks := result.Records[0].Children["StateTransitionCallbacks__r"]
	if len(callbacks) != 1 {
		t.Fatalf("callbacks = %#v", result.Records[0].Children)
	}
	if got := callbacks[0].Fields["QualifiedApiName"].String; got != "order_convert_carts_to_orders" {
		t.Fatalf("callback QualifiedApiName = %q", got)
	}
	if got := callbacks[0].Fields["TriggeringTransition__r.QualifiedApiName"].String; got != "order_submit_as_proforma" {
		t.Fatalf("callback triggering transition = %q", got)
	}

	result, err = ParseAndExecute(org, "SELECT QualifiedApiName, (SELECT QualifiedApiName, FromStates__c FROM StateTransitions__r WHERE IsActive__c = TRUE), (SELECT QualifiedApiName, ActionName__c, TriggeringTransition__r.QualifiedApiName FROM StateTransitionCallbacks__r WHERE IsActive__c = TRUE) FROM StateConfiguration__mdt WHERE QualifiedApiName = 'OrderGraph'")
	if err != nil {
		t.Fatal(err)
	}
	transitions := result.Records[0].Children["StateTransitions__r"]
	callbacks = result.Records[0].Children["StateTransitionCallbacks__r"]
	if len(transitions) != 1 || len(callbacks) != 1 {
		t.Fatalf("combined children = %#v", result.Records[0].Children)
	}
	if got := transitions[0].Fields["QualifiedApiName"].String; got != "order_submit_as_proforma" {
		t.Fatalf("combined transition QualifiedApiName = %q", got)
	}
	if got := callbacks[0].Fields["TriggeringTransition__r.QualifiedApiName"].String; got != "order_submit_as_proforma" {
		t.Fatalf("combined callback triggering transition = %q", got)
	}

	result, err = ParseAndExecute(org, "SELECT Id,DeveloperName,MasterLabel,Language,NamespacePrefix,Label,QualifiedApiName,IsActive__c,SupportedStates__c, (SELECT Id, QualifiedApiName, IsActive__c, FromStates__c, ToState__c FROM StateTransitions__r WHERE IsActive__c = TRUE), (SELECT Id, QualifiedApiName, IsActive__c, ActionName__c, ClassName__c, SortOrder__c, TriggeringTransition__r.QualifiedApiName, Type__c FROM StateTransitionCallbacks__r WHERE IsActive__c = TRUE) FROM StateConfiguration__mdt WHERE IsActive__c = TRUE ORDER BY MasterLabel, NamespacePrefix")
	if err != nil {
		t.Fatal(err)
	}
	var orderGraph storage.Record
	for _, record := range result.Records {
		if record.Fields["QualifiedApiName"].String == "OrderGraph" {
			orderGraph = record
		}
	}
	transitions = orderGraph.Children["StateTransitions__r"]
	callbacks = orderGraph.Children["StateTransitionCallbacks__r"]
	if len(transitions) != 1 || len(callbacks) != 1 {
		t.Fatalf("all-active children = %#v", orderGraph.Children)
	}
	if got := transitions[0].Fields["QualifiedApiName"].String; got != "order_submit_as_proforma" {
		t.Fatalf("all-active transition QualifiedApiName = %q", got)
	}

	clonedOrg := org.CloneRuntime()
	result, err = ParseAndExecute(clonedOrg, "SELECT Id,DeveloperName,MasterLabel,Language,NamespacePrefix,Label,QualifiedApiName,IsActive__c,SupportedStates__c, (SELECT Id, QualifiedApiName, IsActive__c, FromStates__c, ToState__c FROM StateTransitions__r WHERE IsActive__c = TRUE), (SELECT Id, QualifiedApiName, IsActive__c, ActionName__c, ClassName__c, SortOrder__c, TriggeringTransition__r.QualifiedApiName, Type__c FROM StateTransitionCallbacks__r WHERE IsActive__c = TRUE) FROM StateConfiguration__mdt WHERE IsActive__c = TRUE ORDER BY MasterLabel, NamespacePrefix")
	if err != nil {
		t.Fatal(err)
	}
	orderGraph = storage.Record{}
	for _, record := range result.Records {
		if record.Fields["QualifiedApiName"].String == "OrderGraph" {
			orderGraph = record
		}
	}
	transitions = orderGraph.Children["StateTransitions__r"]
	callbacks = orderGraph.Children["StateTransitionCallbacks__r"]
	if len(transitions) != 1 || len(callbacks) != 1 {
		t.Fatalf("cloned all-active children = %#v", orderGraph.Children)
	}

	templateOrg := storage.NewRuntimeTemplate(org).CloneRuntimeOrg()
	methodOrg := templateOrg.CloneRollbackSnapshot()
	result, err = ParseAndExecute(methodOrg, "SELECT Id,DeveloperName,MasterLabel,Language,NamespacePrefix,Label,QualifiedApiName,IsActive__c,SupportedStates__c, (SELECT Id, QualifiedApiName, IsActive__c, FromStates__c, ToState__c FROM StateTransitions__r WHERE IsActive__c = TRUE), (SELECT Id, QualifiedApiName, IsActive__c, ActionName__c, ClassName__c, SortOrder__c, TriggeringTransition__r.QualifiedApiName, Type__c FROM StateTransitionCallbacks__r WHERE IsActive__c = TRUE) FROM StateConfiguration__mdt WHERE IsActive__c = TRUE ORDER BY MasterLabel, NamespacePrefix")
	if err != nil {
		t.Fatal(err)
	}
	orderGraph = storage.Record{}
	for _, record := range result.Records {
		if record.Fields["QualifiedApiName"].String == "OrderGraph" {
			orderGraph = record
		}
	}
	transitions = orderGraph.Children["StateTransitions__r"]
	callbacks = orderGraph.Children["StateTransitionCallbacks__r"]
	if len(transitions) != 1 || len(callbacks) != 1 {
		t.Fatalf("template method children = %#v", orderGraph.Children)
	}
}

func TestExecuteChildRelationshipSubqueryStripsChildObjectQualifier(t *testing.T) {
	org := storage.NewOrgState()
	parentDefinition := storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
		Relations: []storage.Relationship{{
			Field:              "Parent__c",
			ParentObjects:      []string{"Parent__c"},
			ParentRelationship: "Parent__r",
			ChildRelationship:  "Children__r",
		}},
	}
	childDefinition := storage.ObjectDefinition{
		APIName: "Child__c",
		Fields: map[string]storage.Field{
			"Name__c":   {APIName: "Name__c", Type: storage.FieldString},
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
		},
		Relations: []storage.Relationship{{
			Field:              "Parent__c",
			ParentObjects:      []string{"Parent__c"},
			ParentRelationship: "Parent__r",
			ChildRelationship:  "Children__r",
		}},
	}
	storage.EnsureStandardObjectFields(&parentDefinition)
	storage.EnsureStandardObjectFields(&childDefinition)
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: parentDefinition,
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Parent__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Parent")}},
		},
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: childDefinition,
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "Child__c", Fields: map[string]storage.Value{
				"Name__c":   storage.StringValue("Child"),
				"Parent__c": storage.IDValue("a00000000000001"),
			}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Child__c.Id, Child__c.Name__c FROM Children__r) FROM Parent__c")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["Children__r"]
	if len(children) != 1 {
		t.Fatalf("children = %#v", result.Records[0].Children)
	}
	if got := children[0].Fields["Name__c"]; got.Kind != storage.ValueString || got.String != "Child" {
		t.Fatalf("child Name__c = %#v", got)
	}
}

func TestExecuteEntityDefinitionMetadataRelationship(t *testing.T) {
	sch := schema.Schema{Objects: []schema.Object{
		{
			Name: "TriggerStep__mdt",
			Fields: []schema.Field{
				{Name: "Object__c", Type: "MetadataRelationship", ReferenceTo: []string{"EntityDefinition"}},
			},
		},
	}, CustomMetadataRecords: []schema.CustomMetadataRecord{
		{FullName: "TriggerStep.MembershipUpdateAccountStep", ObjectName: "TriggerStep__mdt", DeveloperName: "MembershipUpdateAccountStep", Values: []schema.CustomMetadataValue{{Field: "Object__c", Value: "Membership__c"}}},
	}}
	org := storage.NewOrgState()
	registry := sobject.BuildDescribeRegistry(sch)
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe), Records: map[storage.ID]storage.Record{}}
	}
	if err := storage.ApplyCustomMetadataRecords(&org, sch.CustomMetadataRecords); err != nil {
		t.Fatal(err)
	}

	result, err := ParseAndExecute(org, "SELECT DeveloperName, Object__r.QualifiedApiName FROM TriggerStep__mdt WHERE Object__r.QualifiedApiName = 'Membership__c'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	if value := result.Records[0].Fields["Object__r.QualifiedApiName"]; value.Kind != storage.ValueString || value.String != "Membership__c" {
		t.Fatalf("entity relationship projection = %#v", value)
	}
}

func TestExecuteCustomObjectRelationshipProjectionWithMismatchedRelationshipName(t *testing.T) {
	org := storage.NewOrgState()
	parentDefinition := storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields: map[string]storage.Field{
			"Name__c": {APIName: "Name__c", Type: storage.FieldString},
		},
	}
	storage.EnsureStandardObjectFields(&parentDefinition)
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: parentDefinition,
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Parent__c",
				Fields: map[string]storage.Value{
					"Name__c": storage.StringValue("ParentValue"),
				},
			},
		},
	}
	childDefinition := storage.ObjectDefinition{
		APIName: "Child__c",
		Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parents"},
		},
		Relations: []storage.Relationship{{
			Field:              "Parent__c",
			ParentObjects:      []string{"Parent__c"},
			ParentRelationship: "Parents",
		}},
	}
	storage.EnsureStandardObjectFields(&childDefinition)
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: childDefinition,
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Child__c",
				Fields: map[string]storage.Value{
					"Parent__c": storage.IDValue("a00000000000001AAA"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Parent__r.Name__c FROM Child__c")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	value := result.Records[0].Fields["Parent__r.Name__c"]
	if value.Kind != storage.ValueString || value.String != "ParentValue" {
		t.Fatalf("relationship projection = %#v", value)
	}
}

func TestExecuteParentRelationshipProjectionWithMissingParentReturnsRelationshipNull(t *testing.T) {
	org := storage.NewOrgState()
	parentDefinition := storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields: map[string]storage.Field{
			"Name__c": {APIName: "Name__c", Type: storage.FieldString},
		},
	}
	storage.EnsureStandardObjectFields(&parentDefinition)
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: parentDefinition,
		Records:    map[storage.ID]storage.Record{},
	}
	childDefinition := storage.ObjectDefinition{
		APIName: "Child__c",
		Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
		},
		Relations: []storage.Relationship{{
			Field:              "Parent__c",
			ParentObjects:      []string{"Parent__c"},
			ParentRelationship: "Parent__r",
		}},
	}
	storage.EnsureStandardObjectFields(&childDefinition)
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: childDefinition,
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Child__c",
				Fields: map[string]storage.Value{
					"Parent__c": storage.IDValue("a00000000000001"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Parent__r.Name__c FROM Child__c")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	if value := result.Records[0].Fields["Parent__r"]; value.Kind != storage.ValueNull {
		t.Fatalf("relationship = %#v, want null", value)
	}
	if _, ok := result.Records[0].Fields["Parent__r.Name__c"]; ok {
		t.Fatalf("leaf projection should not be populated for a missing parent: %#v", result.Records[0].Fields)
	}
}

func TestExecuteKnowledgeArticleLanguageField(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "FAQ__kav")
	state := org.Objects["FAQ__kav"]
	state.Records["ka0000000000001"] = storage.Record{
		ID:     "ka0000000000001",
		Object: "FAQ__kav",
		Fields: map[string]storage.Value{
			"Language": storage.StringValue("en_US"),
			"Title":    storage.StringValue("Local help"),
		},
	}
	org.Objects["FAQ__kav"] = state

	result, err := ParseAndExecute(org, "SELECT Id, Language, Title FROM FAQ__kav WHERE Language = 'en_US'")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(result.Records))
	}
	if value := result.Records[0].Fields["Language"]; value.Kind != storage.ValueString || value.String != "en_US" {
		t.Fatalf("Language = %#v", value)
	}
}

func customMetadataRelationshipOrg() storage.OrgState {
	org := storage.NewOrgState()
	targetDefinition := storage.ObjectDefinition{
		APIName:  "Target__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields: map[string]storage.Field{
			"Name__c": {APIName: "Name__c", Type: storage.FieldString},
		},
	}
	storage.EnsureStandardObjectFields(&targetDefinition)
	bindingDefinition := storage.ObjectDefinition{
		APIName:  "Binding__mdt",
		Metadata: map[string]string{"kind": "customMetadata"},
		Fields: map[string]storage.Field{
			"Target__c": {APIName: "Target__c", Type: storage.FieldReference, ReferenceTo: []string{"Target__mdt"}},
		},
	}
	storage.EnsureStandardObjectFields(&bindingDefinition)
	org.Objects["Target__mdt"] = storage.ObjectState{
		Definition: targetDefinition,
		Records: map[storage.ID]storage.Record{
			"a10000000000001": {ID: "a10000000000001", Object: "Target__mdt", Fields: map[string]storage.Value{
				"DeveloperName":    storage.StringValue("Target"),
				"QualifiedApiName": storage.StringValue("Target"),
			}},
		},
	}
	org.Objects["Binding__mdt"] = storage.ObjectState{
		Definition: bindingDefinition,
		Records: map[storage.ID]storage.Record{
			"a11000000000001": {ID: "a11000000000001", Object: "Binding__mdt", Fields: map[string]storage.Value{
				"DeveloperName": storage.StringValue("Default"),
				"Target__c":     storage.IDValue("a10000000000001"),
			}},
		},
	}
	return org
}

func TestExecuteAggregateQueries(t *testing.T) {
	org := aggregateTestOrg()

	result, err := ParseAndExecute(org, "SELECT COUNT(Name) namedCount, COUNT_DISTINCT(Rating), SUM(AnnualRevenue) totalRevenue, MIN(AnnualRevenue), MAX(AnnualRevenue), AVG(AnnualRevenue) averageRevenue FROM Account WHERE AnnualRevenue >= 100")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || len(result.Records) != 1 {
		t.Fatalf("result = %#v", result)
	}
	fields := result.Records[0].Fields
	assertStorageInt(t, fields["expr0"], 3)
	assertStorageInt(t, fields["expr1"], 2)
	assertStorageDecimal(t, fields["expr2"], "650")
	assertStorageDecimal(t, fields["expr3"], "100")
	assertStorageDecimal(t, fields["expr4"], "300")
	assertStorageDecimal(t, fields["expr5"], "216.6666666667")
	assertStorageInt(t, fields["namedCount"], 3)
	assertStorageDecimal(t, fields["totalRevenue"], "650")
	assertStorageDecimal(t, fields["averageRevenue"], "216.6666666667")
}

func TestExecuteAggregateGroupByRelationshipFieldAddsLeafAlias(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Event__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Event__c",
			Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
		},
		Records: map[storage.ID]storage.Record{
			"a8G000000000001": {ID: "a8G000000000001", Object: "Event__c"},
		},
	}
	org.Objects["Registration2__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Registration2__c",
			Fields: map[string]storage.Field{
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"Event2__c": {APIName: "Event2__c", Type: storage.FieldReference, ReferenceTo: []string{"Event__c"}, RelationshipName: "Event2__r"},
			},
			Relations: []storage.Relationship{{Field: "Event2__c", ParentObjects: []string{"Event__c"}, ParentRelationship: "Event2__r"}},
		},
		Records: map[storage.ID]storage.Record{
			"a1R000000000001": {ID: "a1R000000000001", Object: "Registration2__c", Fields: map[string]storage.Value{"Event2__c": storage.IDValue("a8G000000000001")}},
		},
	}
	org.Objects["EventBadge__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "EventBadge__c",
			Fields: map[string]storage.Field{
				"Id":               {APIName: "Id", Type: storage.FieldID},
				"Registration2__c": {APIName: "Registration2__c", Type: storage.FieldReference, ReferenceTo: []string{"Registration2__c"}, RelationshipName: "Registration2__r"},
			},
			Relations: []storage.Relationship{{Field: "Registration2__c", ParentObjects: []string{"Registration2__c"}, ParentRelationship: "Registration2__r"}},
		},
		Records: map[storage.ID]storage.Record{
			"a8B000000000001": {ID: "a8B000000000001", Object: "EventBadge__c", Fields: map[string]storage.Value{"Registration2__c": storage.IDValue("a1R000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Registration2__r.Event2__c, COUNT(Id) recordCount FROM EventBadge__c GROUP BY Registration2__r.Event2__c")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("result = %#v", result)
	}
	fields := result.Records[0].Fields
	if got := fields["Registration2__r.Event2__c"].ID; got != "a8G000000000001" {
		t.Fatalf("full path group field = %q", got)
	}
	if got := fields["Event2__c"].ID; got != "a8G000000000001" {
		t.Fatalf("leaf group alias = %q", got)
	}
	assertStorageInt(t, fields["recordCount"], 1)
}

func TestExecuteSelectFieldFunctions(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{
		APIName: "Rating",
		Type:    storage.FieldPicklist,
		PicklistValues: []storage.PicklistValue{
			{Value: "Hot", Label: "Hot Label"},
			{Value: "Warm", Label: "Warm Label"},
		},
	}
	org.Objects["Account"] = account

	result, err := ParseAndExecute(org, "SELECT toLabel(Rating) ratingLabel, FORMAT(AnnualRevenue) formattedRevenue, convertCurrency(AnnualRevenue) convertedRevenue FROM Account WHERE toLabel(Rating) = 'Hot Label' ORDER BY Name LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("result = %#v", result)
	}
	fields := result.Records[0].Fields
	if got := fields["ratingLabel"].String; got != "Hot Label" {
		t.Fatalf("ratingLabel = %q", got)
	}
	if got := fields["formattedRevenue"].String; got != "100" {
		t.Fatalf("formattedRevenue = %q", got)
	}
	assertStorageDecimal(t, fields["convertedRevenue"], "100")
}

func TestExecuteDistanceGeolocationFunction(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	org.Objects["Warehouse__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Warehouse__c",
			Fields: map[string]storage.Field{
				"Name":                   {APIName: "Name", Type: storage.FieldString},
				"Location__latitude__s":  {APIName: "Location__latitude__s", Type: storage.FieldDecimal},
				"Location__longitude__s": {APIName: "Location__longitude__s", Type: storage.FieldDecimal},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "Warehouse__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Near"), "Location__latitude__s": storage.DecimalValue("37.775"), "Location__longitude__s": storage.DecimalValue("-122.418")}},
			"a01000000000002": {ID: "a01000000000002", Object: "Warehouse__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Across Town"), "Location__latitude__s": storage.DecimalValue("37.8044"), "Location__longitude__s": storage.DecimalValue("-122.2712")}},
			"a01000000000003": {ID: "a01000000000003", Object: "Warehouse__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Far"), "Location__latitude__s": storage.DecimalValue("34.0522"), "Location__longitude__s": storage.DecimalValue("-118.2437")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Name, DISTANCE(GEOLOCATION(Location__latitude__s, Location__longitude__s), GEOLOCATION(37.775,-122.418), 'mi') miles FROM Warehouse__c WHERE DISTANCE(GEOLOCATION(Location__latitude__s, Location__longitude__s), GEOLOCATION(37.775,-122.418), 'mi') < 20 ORDER BY DISTANCE(GEOLOCATION(Location__latitude__s, Location__longitude__s), GEOLOCATION(37.775,-122.418), 'mi')")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 {
		t.Fatalf("result = %#v", result)
	}
	if got := result.Records[0].Fields["Name"].String; got != "Near" {
		t.Fatalf("first Name = %q", got)
	}
	if got := result.Records[1].Fields["Name"].String; got != "Across Town" {
		t.Fatalf("second Name = %q", got)
	}
	got, err := strconv.ParseFloat(result.Records[0].Fields["miles"].Decimal, 64)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got) > 0.01 {
		t.Fatalf("miles = %v", got)
	}
}

func TestExecuteDatePartSelectFunctionGrouping(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	for id, record := range account.Records {
		record.Fields["RenewalDate__c"] = storage.DateValue("2026-04-15")
		account.Records[id] = record
	}
	org.Objects["Account"] = account

	result, err := ParseAndExecute(org, "SELECT CALENDAR_YEAR(RenewalDate__c) renewalYear, COUNT(Id) total FROM Account GROUP BY CALENDAR_YEAR(RenewalDate__c)")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("result = %#v", result)
	}
	fields := result.Records[0].Fields
	assertStorageInt(t, fields["CALENDAR_YEAR(RenewalDate__c)"], 2026)
	assertStorageInt(t, fields["renewalYear"], 2026)
	assertStorageInt(t, fields["total"], 3)
}

func TestExecuteDatePartFunctionWithConvertTimezone(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["CreatedDate"] = storage.Field{APIName: "CreatedDate", Type: storage.FieldDateTime}
	for id, record := range account.Records {
		record.Fields["CreatedDate"] = storage.DateTimeValue("2026-02-02T10:30:00Z")
		account.Records[id] = record
	}
	org.Objects["Account"] = account

	result, err := ParseAndExecute(org, "SELECT Id FROM Account WHERE DAY_ONLY(convertTimezone(CreatedDate)) < 2026-02-03 ORDER BY Name ASC NULLS FIRST LIMIT 1000")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 3 {
		t.Fatalf("rows = %d", result.Rows)
	}
}

func TestFiscalDateFunctionsUseOrganizationFiscalStartMonth(t *testing.T) {
	org := aggregateTestOrg()
	org.OrgID = "00D000000000001"
	org.Objects["Organization"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Organization", Fields: map[string]storage.Field{
			"FiscalYearStartMonth":          {APIName: "FiscalYearStartMonth", Type: storage.FieldInteger},
			"UsesStartDateAsFiscalYearName": {APIName: "UsesStartDateAsFiscalYearName", Type: storage.FieldBoolean},
		}},
		Records: map[storage.ID]storage.Record{
			"00D000000000001": {ID: "00D000000000001", Object: "Organization", Fields: map[string]storage.Value{
				"FiscalYearStartMonth":          storage.IntegerValue(2),
				"UsesStartDateAsFiscalYearName": storage.BooleanValue(false),
			}},
		},
	}
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	account.Records["001000000000001"] = storage.Record{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{
		"Name":           storage.StringValue("February"),
		"RenewalDate__c": storage.DateValue("2026-02-15"),
	}}
	account.Records["001000000000002"] = storage.Record{ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{
		"Name":           storage.StringValue("January"),
		"RenewalDate__c": storage.DateValue("2026-01-15"),
	}}
	org.Objects["Account"] = account

	result, err := ParseAndExecute(org, "SELECT Name, FISCAL_MONTH(RenewalDate__c) fiscalMonth, FISCAL_QUARTER(RenewalDate__c) fiscalQuarter, FISCAL_YEAR(RenewalDate__c) fiscalYear FROM Account WHERE FISCAL_MONTH(RenewalDate__c) = 1 ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Name"].String != "February" {
		t.Fatalf("records = %#v, want February fiscal-month row", result.Records)
	}
	assertStorageInt(t, result.Records[0].Fields["fiscalMonth"], 1)
	assertStorageInt(t, result.Records[0].Fields["fiscalQuarter"], 1)
	assertStorageInt(t, result.Records[0].Fields["fiscalYear"], 2027)
}

func aggregateTestOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"AnnualRevenue": {APIName: "AnnualRevenue", Type: storage.FieldDecimal},
				"Rating":        {APIName: "Rating", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":          storage.StringValue("Acme"),
					"AnnualRevenue": storage.DecimalValue("100"),
					"Rating":        storage.StringValue("Hot"),
				},
			},
			"001000000000002": {
				ID:     "001000000000002",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":          storage.StringValue("Beta"),
					"AnnualRevenue": storage.DecimalValue("250"),
					"Rating":        storage.StringValue("Warm"),
				},
			},
			"001000000000003": {
				ID:     "001000000000003",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":          storage.StringValue("Gamma"),
					"AnnualRevenue": storage.DecimalValue("300"),
					"Rating":        storage.StringValue("Hot"),
				},
			},
		},
	}
	return org
}

func TestExecuteGroupedAggregateQueries(t *testing.T) {
	org := aggregateTestOrg()

	result, err := ParseAndExecute(org, "SELECT Rating, COUNT(Id) accountCount, SUM(AnnualRevenue) totalRevenue FROM Account GROUP BY Rating HAVING accountCount > 1 ORDER BY totalRevenue LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || len(result.Records) != 1 {
		t.Fatalf("result = %#v", result)
	}
	fields := result.Records[0].Fields
	if got := fields["Rating"].String; got != "Hot" {
		t.Fatalf("Rating = %q", got)
	}
	assertStorageInt(t, fields["expr0"], 2)
	assertStorageDecimal(t, fields["expr1"], "400")
	assertStorageInt(t, fields["accountCount"], 2)
	assertStorageDecimal(t, fields["totalRevenue"], "400")

	result, err = ParseAndExecute(org, "SELECT Rating, COUNT(Id) accountCount, SUM(AnnualRevenue) totalRevenue FROM Account GROUP BY Rating ORDER BY totalRevenue DESC LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("desc aggregate result = %#v", result)
	}
	if got := result.Records[0].Fields["Rating"].String; got != "Hot" {
		t.Fatalf("desc aggregate Rating = %q", got)
	}

	result, err = ParseAndExecute(org, "SELECT Rating, COUNT(Id) accountCount, SUM(AnnualRevenue) totalRevenue FROM Account GROUP BY Rating ORDER BY accountCount DESC, Rating DESC LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("multi-order aggregate result = %#v", result)
	}
	if got := result.Records[0].Fields["Rating"].String; got != "Hot" {
		t.Fatalf("multi-order aggregate Rating = %q", got)
	}

	account := org.Objects["Account"]
	account.Records["001000000000004"] = storage.Record{
		ID:     "001000000000004",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":          storage.StringValue("No Rating"),
			"AnnualRevenue": storage.DecimalValue("50"),
		},
	}
	org.Objects["Account"] = account
	result, err = ParseAndExecute(org, "SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating ORDER BY Rating ASC NULLS FIRST LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Rating"].Kind != storage.ValueNull {
		t.Fatalf("nulls first aggregate result = %#v", result)
	}

	result, err = ParseAndExecute(org, "SELECT Rating ratingKey, MAX(AnnualRevenue) maxRevenue FROM Account GROUP BY Rating ORDER BY ratingKey")
	if err != nil {
		t.Fatal(err)
	}
	if result.Records[0].Fields["ratingKey"].Kind != storage.ValueNull || result.Records[1].Fields["ratingKey"].String != "Hot" {
		t.Fatalf("grouped field alias result = %#v", result)
	}
	assertStorageDecimal(t, result.Records[1].Fields["maxRevenue"], "300")
}

func TestExecuteValidatesAggregateHavingAndAliases(t *testing.T) {
	org := aggregateTestOrg()

	result, err := ParseAndExecute(org, "SELECT Rating, COUNT(Id) accountCount, SUM(AnnualRevenue) totalRevenue FROM Account GROUP BY Rating HAVING Rating = 'Hot' AND totalRevenue > 100")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Rating"].String != "Hot" {
		t.Fatalf("valid HAVING result = %#v", result)
	}
	result, err = ParseAndExecute(org, "SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING SUM(AnnualRevenue) > 300")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Rating"].String != "Hot" {
		t.Fatalf("unselected aggregate HAVING result = %#v", result)
	}
	if _, ok := result.Records[0].Fields["expr1"]; ok {
		t.Fatalf("unselected aggregate leaked expr1 field: %#v", result.Records[0].Fields)
	}
	for field := range result.Records[0].Fields {
		if strings.Contains(field, "havingAggregate") {
			t.Fatalf("unselected aggregate leaked hidden field %q: %#v", field, result.Records[0].Fields)
		}
	}
	result, err = ParseAndExecute(org, "SELECT Rating FROM Account GROUP BY Rating HAVING SUM(AnnualRevenue) > 300")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || len(result.Records[0].Fields) != 1 || result.Records[0].Fields["Rating"].String != "Hot" {
		t.Fatalf("grouped field with unselected aggregate HAVING result = %#v", result)
	}
	result, err = ParseAndExecute(org, "SELECT Rating FROM Account GROUP BY Rating HAVING SUM(AnnualRevenue) > 300 AND SUM(AnnualRevenue) < 500")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || len(result.Records[0].Fields) != 1 || result.Records[0].Fields["Rating"].String != "Hot" {
		t.Fatalf("deduped unselected aggregate HAVING result = %#v", result)
	}

	cases := []struct {
		query string
		want  string
	}{
		{query: "SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING Missing__c = 'x'", want: "Missing__c"},
		{query: "SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING Name = 'Acme'", want: "must be grouped or aggregated"},
		{query: "SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING SUM(Missing__c) > 0", want: "Missing__c"},
		{query: "SELECT SUM(Name) bad FROM Account", want: "SUM requires numeric field Name"},
		{query: "SELECT SUM(Id) bad FROM Account", want: "SUM requires numeric field Id"},
		{query: "SELECT Rating, COUNT(Id) Rating FROM Account GROUP BY Rating", want: "conflicts with grouped field"},
		{query: "SELECT COUNT(Id) sameAlias, SUM(AnnualRevenue) sameAlias FROM Account", want: "duplicate aggregate alias"},
		{query: "SELECT COUNT(Id) expr0 FROM Account", want: "conflicts with generated aggregate field"},
	}
	for _, tc := range cases {
		if _, err := ParseAndExecute(org, tc.query); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("expected %q for %q, got %v", tc.want, tc.query, err)
		}
	}
}

func TestExecuteDateLiteralPredicates(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	account.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Acme"),
			"RenewalDate__c": storage.DateValue("2026-05-02"),
		},
		System: storage.SystemFields{CreatedDate: "2026-05-02T13:00:00Z"},
	}
	account.Records["001000000000002"] = storage.Record{
		ID:     "001000000000002",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Beta"),
			"RenewalDate__c": storage.DateValue("2026-04-30"),
		},
		System: storage.SystemFields{CreatedDate: "2026-04-30T13:00:00Z"},
	}
	org.Objects["Account"] = account
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)

	result, err := ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE RenewalDate__c = TODAY", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000001" {
		t.Fatalf("TODAY result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE CreatedDate = LAST_N_DAYS:2 ORDER BY Id", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 {
		t.Fatalf("LAST_N_DAYS result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE RenewalDate__c = 2026-04-30", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000002" {
		t.Fatalf("ISO date result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE CreatedDate <= 2026-05-01T00:00:00Z", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000002" {
		t.Fatalf("ISO datetime result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE CreatedDate = 2026-05-02T14:00:00+01:00", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000001" {
		t.Fatalf("positive offset datetime result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE CreatedDate = 2026-04-30T05:00:00-08:00", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000002" {
		t.Fatalf("negative offset datetime result = %#v", result)
	}
}

func TestExecuteDateLiteralInListPredicates(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	account.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Today"),
			"RenewalDate__c": storage.DateValue("2026-05-02"),
		},
	}
	account.Records["001000000000002"] = storage.Record{
		ID:     "001000000000002",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Yesterday"),
			"RenewalDate__c": storage.DateValue("2026-05-01"),
		},
	}
	account.Records["001000000000003"] = storage.Record{
		ID:     "001000000000003",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Older"),
			"RenewalDate__c": storage.DateValue("2026-04-30"),
		},
	}
	org.Objects["Account"] = account
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)

	result, err := ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE RenewalDate__c IN (TODAY, YESTERDAY) ORDER BY Id", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Records[0].ID != "001000000000001" || result.Records[1].ID != "001000000000002" {
		t.Fatalf("date literal IN result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE RenewalDate__c NOT IN (TODAY, YESTERDAY) ORDER BY Id", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000003" {
		t.Fatalf("date literal NOT IN result = %#v", result)
	}
}

func TestExecuteExtendedDateLiteralPredicates(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	account.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("March"),
			"RenewalDate__c": storage.DateValue("2026-03-15"),
		},
	}
	account.Records["001000000000002"] = storage.Record{
		ID:     "001000000000002",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Quarter"),
			"RenewalDate__c": storage.DateValue("2026-04-01"),
		},
	}
	account.Records["001000000000003"] = storage.Record{
		ID:     "001000000000003",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Yesterday"),
			"RenewalDate__c": storage.DateValue("2026-05-01"),
		},
	}
	account.Records["001000000000004"] = storage.Record{
		ID:     "001000000000004",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("LastWeek"),
			"RenewalDate__c": storage.DateValue("2026-04-25"),
		},
	}
	account.Records["001000000000005"] = storage.Record{
		ID:     "001000000000005",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("NextWeek"),
			"RenewalDate__c": storage.DateValue("2026-05-03"),
		},
	}
	org.Objects["Account"] = account
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)

	result, err := ParseAndExecuteAt(org, "SELECT Id, Name FROM Account WHERE RenewalDate__c = LAST_N_MONTHS:2 ORDER BY Name", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 3 || result.Records[0].Fields["Name"].String != "LastWeek" || result.Records[1].Fields["Name"].String != "March" || result.Records[2].Fields["Name"].String != "Quarter" {
		t.Fatalf("LAST_N_MONTHS result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id, Name FROM Account WHERE RenewalDate__c = THIS_QUARTER ORDER BY Name", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 4 || result.Records[0].Fields["Name"].String != "LastWeek" || result.Records[1].Fields["Name"].String != "NextWeek" || result.Records[2].Fields["Name"].String != "Quarter" || result.Records[3].Fields["Name"].String != "Yesterday" {
		t.Fatalf("THIS_QUARTER result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id, Name FROM Account WHERE RenewalDate__c = N_DAYS_AGO:1", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Name"].String != "Yesterday" {
		t.Fatalf("N_DAYS_AGO result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id, Name FROM Account WHERE RenewalDate__c = THIS_WEEK ORDER BY Name", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Name"].String != "Yesterday" {
		t.Fatalf("THIS_WEEK result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id, Name FROM Account WHERE RenewalDate__c = LAST_WEEK ORDER BY Name", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Name"].String != "LastWeek" {
		t.Fatalf("LAST_WEEK result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id, Name FROM Account WHERE RenewalDate__c = NEXT_WEEK ORDER BY Name", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Name"].String != "NextWeek" {
		t.Fatalf("NEXT_WEEK result = %#v", result)
	}
}

func TestDateLiteralReferenceCoverage(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		literal string
		start   string
		end     string
	}{
		{"YESTERDAY", "2026-05-01", "2026-05-02"},
		{"TODAY", "2026-05-02", "2026-05-03"},
		{"TOMORROW", "2026-05-03", "2026-05-04"},
		{"LAST_WEEK", "2026-04-19", "2026-04-26"},
		{"THIS_WEEK", "2026-04-26", "2026-05-03"},
		{"NEXT_WEEK", "2026-05-03", "2026-05-10"},
		{"LAST_MONTH", "2026-04-01", "2026-05-01"},
		{"THIS_MONTH", "2026-05-01", "2026-06-01"},
		{"NEXT_MONTH", "2026-06-01", "2026-07-01"},
		{"LAST_90_DAYS", "2026-02-01", "2026-05-03"},
		{"NEXT_90_DAYS", "2026-05-03", "2026-08-01"},
		{"LAST_N_DAYS:5", "2026-04-27", "2026-05-03"},
		{"NEXT_N_DAYS:5", "2026-05-03", "2026-05-08"},
		{"N_DAYS_AGO:3", "2026-04-29", "2026-04-30"},
		{"NEXT_N_WEEKS:2", "2026-05-03", "2026-05-17"},
		{"LAST_N_WEEKS:2", "2026-04-12", "2026-04-26"},
		{"N_WEEKS_AGO:2", "2026-04-12", "2026-04-19"},
		{"NEXT_N_MONTHS:2", "2026-06-01", "2026-08-01"},
		{"LAST_N_MONTHS:2", "2026-03-01", "2026-05-01"},
		{"N_MONTHS_AGO:2", "2026-03-01", "2026-04-01"},
		{"THIS_QUARTER", "2026-04-01", "2026-07-01"},
		{"LAST_QUARTER", "2026-01-01", "2026-04-01"},
		{"NEXT_QUARTER", "2026-07-01", "2026-10-01"},
		{"NEXT_N_QUARTERS:2", "2026-07-01", "2027-01-01"},
		{"LAST_N_QUARTERS:2", "2025-10-01", "2026-04-01"},
		{"N_QUARTERS_AGO:2", "2025-10-01", "2026-01-01"},
		{"THIS_YEAR", "2026-01-01", "2027-01-01"},
		{"LAST_YEAR", "2025-01-01", "2026-01-01"},
		{"NEXT_YEAR", "2027-01-01", "2028-01-01"},
		{"NEXT_N_YEARS:2", "2027-01-01", "2029-01-01"},
		{"LAST_N_YEARS:2", "2024-01-01", "2026-01-01"},
		{"N_YEARS_AGO:2", "2024-01-01", "2025-01-01"},
		{"THIS_FISCAL_QUARTER", "2026-04-01", "2026-07-01"},
		{"LAST_FISCAL_QUARTER", "2026-01-01", "2026-04-01"},
		{"NEXT_FISCAL_QUARTER", "2026-07-01", "2026-10-01"},
		{"NEXT_N_FISCAL_QUARTERS:2", "2026-07-01", "2027-01-01"},
		{"LAST_N_FISCAL_QUARTERS:2", "2025-10-01", "2026-04-01"},
		{"N_FISCAL_QUARTERS_AGO:2", "2025-10-01", "2026-01-01"},
		{"THIS_FISCAL_YEAR", "2026-01-01", "2027-01-01"},
		{"LAST_FISCAL_YEAR", "2025-01-01", "2026-01-01"},
		{"NEXT_FISCAL_YEAR", "2027-01-01", "2028-01-01"},
		{"NEXT_N_FISCAL_YEARS:2", "2027-01-01", "2029-01-01"},
		{"LAST_N_FISCAL_YEARS:2", "2024-01-01", "2026-01-01"},
		{"N_FISCAL_YEARS_AGO:2", "2024-01-01", "2025-01-01"},
	}
	for _, tc := range cases {
		start, end, ok := dateLiteral(tc.literal, now)
		if !ok {
			t.Fatalf("%s was not recognized", tc.literal)
		}
		if start.String != tc.start || end.String != tc.end {
			t.Fatalf("%s range = %s..%s, want %s..%s", tc.literal, start.String, end.String, tc.start, tc.end)
		}
	}
}

func TestFiscalDateLiteralsUseOrganizationFiscalStartMonth(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	org := storage.NewOrgState()
	org.OrgID = "00D000000000001"
	org.Objects["Organization"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Organization", Fields: map[string]storage.Field{
			"FiscalYearStartMonth": {APIName: "FiscalYearStartMonth", Type: storage.FieldInteger},
		}},
		Records: map[storage.ID]storage.Record{
			"00D000000000001": {ID: "00D000000000001", Object: "Organization", Fields: map[string]storage.Value{
				"FiscalYearStartMonth": storage.IntegerValue(2),
			}},
		},
	}
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
			"Name":           {APIName: "Name", Type: storage.FieldString},
			"RenewalDate__c": {APIName: "RenewalDate__c", Type: storage.FieldDate},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("April"), "RenewalDate__c": storage.DateValue("2026-04-30")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("May"), "RenewalDate__c": storage.DateValue("2026-05-01")}},
			"001000000000003": {ID: "001000000000003", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("July"), "RenewalDate__c": storage.DateValue("2026-07-31")}},
		},
	}

	result, err := ParseAndExecuteAt(org, "SELECT Name FROM Account WHERE RenewalDate__c = THIS_FISCAL_QUARTER ORDER BY Name", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Records[0].Fields["Name"].String != "July" || result.Records[1].Fields["Name"].String != "May" {
		t.Fatalf("records = %#v, want May and July fiscal-quarter rows", result.Records)
	}
}

func TestDateLiteralReferenceCoverageWithSpacedNumberSuffix(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	for id, record := range account.Records {
		record.Fields["RenewalDate__c"] = storage.DateValue("2026-03-15")
		if id == "001000000000001" {
			record.Fields["RenewalDate__c"] = storage.DateValue("2026-04-15")
		}
		account.Records[id] = record
	}
	org.Objects["Account"] = account

	result, err := ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE RenewalDate__c = N_MONTHS_AGO : 1", now)
	if err != nil {
		t.Fatalf("spaced numeric date literal parse failed: %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("N_MONTHS_AGO : 1 records = %d, want 1", len(result.Records))
	}
}

func TestExecuteRollupAggregateQueries(t *testing.T) {
	org := aggregateTestOrg()

	result, err := ParseAndExecute(org, "SELECT Rating, COUNT(Id) accountCount, GROUPING(Rating) ratingGrouped FROM Account GROUP BY ROLLUP(Rating) ORDER BY ratingGrouped")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 3 || len(result.Records) != 3 {
		t.Fatalf("result = %#v", result)
	}
	fields := result.Records[2].Fields
	if fields["Rating"].Kind != storage.ValueNull {
		t.Fatalf("subtotal Rating = %#v", fields["Rating"])
	}
	assertStorageInt(t, fields["accountCount"], 3)
	assertStorageInt(t, fields["ratingGrouped"], 1)
	assertStorageInt(t, fields["expr1"], 1)
}

func TestExecuteCubeAggregateQueries(t *testing.T) {
	org := aggregateTestOrg()

	result, err := ParseAndExecute(org, "SELECT Rating, Name, COUNT(Id) accountCount, GROUPING(Rating) ratingGrouped, GROUPING(Name) nameGrouped FROM Account GROUP BY CUBE(Rating, Name) HAVING accountCount >= 2")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || len(result.Records) != 2 {
		t.Fatalf("result = %#v", result)
	}
	var hotSubtotal, grandTotal bool
	for _, record := range result.Records {
		fields := record.Fields
		if fields["Rating"].String == "Hot" && fields["Name"].Kind == storage.ValueNull {
			assertStorageInt(t, fields["accountCount"], 2)
			assertStorageInt(t, fields["ratingGrouped"], 0)
			assertStorageInt(t, fields["nameGrouped"], 1)
			hotSubtotal = true
		}
		if fields["Rating"].Kind == storage.ValueNull && fields["Name"].Kind == storage.ValueNull {
			assertStorageInt(t, fields["accountCount"], 3)
			assertStorageInt(t, fields["ratingGrouped"], 1)
			assertStorageInt(t, fields["nameGrouped"], 1)
			grandTotal = true
		}
	}
	if !hotSubtotal || !grandTotal {
		t.Fatalf("missing cube rows: hotSubtotal=%v grandTotal=%v records=%#v", hotSubtotal, grandTotal, result.Records)
	}
}

func TestExecuteFiltersProjectsAndOrders(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name":   {APIName: "Name", Type: storage.FieldString},
				"Active": {APIName: "Active", Type: storage.FieldBoolean},
				"Rating": {APIName: "Rating", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000002": {
				ID:     "001000000000002",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":   storage.StringValue("Beta"),
					"Active": storage.BooleanValue(true),
					"Rating": storage.StringValue("Hot"),
				},
			},
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":   storage.StringValue("Acme"),
					"Active": storage.BooleanValue(true),
					"Rating": storage.StringValue("Hot"),
				},
			},
			"001000000000003": {
				ID:     "001000000000003",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Active": storage.BooleanValue(false),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Name LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d", result.Rows)
	}
	record := result.Records[0]
	if record.ID != "001000000000001" || record.Fields["Name"].String != "Acme" {
		t.Fatalf("record = %#v", record)
	}
	if _, ok := record.Fields["Active"]; ok {
		t.Fatalf("unprojected field leaked: %#v", record.Fields)
	}

	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Name LIMIT 0")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 0 || len(result.Records) != 0 {
		t.Fatalf("limit zero result = %#v", result)
	}

	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Name DESC LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000002" {
		t.Fatalf("desc result = %#v", result)
	}

	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Rating ASC, Name DESC LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000002" {
		t.Fatalf("multi-order result = %#v", result)
	}

	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Name LIMIT 1 OFFSET 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000002" {
		t.Fatalf("offset window result = %#v", result)
	}

	org.Objects["Account"].Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":   storage.StringValue("Edit scheduled payment"),
			"Active": storage.BooleanValue(true),
			"Rating": storage.StringValue("Warm"),
		},
	}
	org.Objects["Account"].Records["001000000000002"] = storage.Record{
		ID:     "001000000000002",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":   storage.StringValue("Edit or Cancel Another"),
			"Active": storage.BooleanValue(true),
			"Rating": storage.StringValue("Warm"),
		},
	}
	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Name LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000002" {
		t.Fatalf("case-insensitive text order result = %#v", result)
	}
	org.Objects["Account"].Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":   storage.StringValue("Acme"),
			"Active": storage.BooleanValue(true),
			"Rating": storage.StringValue("Warm"),
		},
	}
	org.Objects["Account"].Records["001000000000002"] = storage.Record{
		ID:     "001000000000002",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":   storage.StringValue("Beta"),
			"Active": storage.BooleanValue(true),
			"Rating": storage.StringValue("Hot"),
		},
	}

	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account ORDER BY Name ASC NULLS LAST LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000001" {
		t.Fatalf("nulls last result = %#v", result)
	}

	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account ORDER BY Name ASC NULLS FIRST LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000003" {
		t.Fatalf("nulls first result = %#v", result)
	}

	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Active = true ORDER BY Name FOR UPDATE")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Records[0].ID != "001000000000001" || result.Records[1].ID != "001000000000002" {
		t.Fatalf("for update result = %#v", result)
	}
	if !result.Records[0].System.Locked || !result.Records[1].System.Locked {
		t.Fatalf("for update did not mark rows locked: %#v", result.Records)
	}
	if org.Objects["Account"].Records["001000000000001"].System.Locked ||
		org.Objects["Account"].Records["001000000000002"].System.Locked {
		t.Fatalf("for update persisted transaction locks: %#v", org.Objects["Account"].Records)
	}
	result, err = ParseAndExecute(org, "SELECT Id FROM Account WHERE Id = '001000000000001' FOR UPDATE")
	if err != nil {
		t.Fatalf("same transaction for update should be reentrant: %v", err)
	}
	if result.Rows != 1 || !result.Records[0].System.Locked {
		t.Fatalf("for update reentrant result = %#v", result)
	}

	_, err = ParseAndExecute(org, "SELECT COUNT(Id) FROM Account FOR UPDATE")
	if err == nil || !strings.Contains(err.Error(), "FOR UPDATE is not supported with aggregate queries") {
		t.Fatalf("expected aggregate for update error, got %v", err)
	}

	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Active = true WITH USER_MODE ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 {
		t.Fatalf("user mode result = %#v", result)
	}

	query, err := Parse("SELECT Id FROM Account WITH SYSTEM_MODE")
	if err != nil {
		t.Fatal(err)
	}
	if query.SecurityMode != "SYSTEM_MODE" {
		t.Fatalf("query = %#v", query)
	}
	if _, err := ParseAndExecute(org, "SELECT Missing__c FROM Account WITH SECURITY_ENFORCED"); err == nil || !strings.Contains(err.Error(), "Missing__c") {
		t.Fatalf("expected security projection error, got %v", err)
	}
}

func TestCompareSOQLTextDoesNotAllocateForMixedCase(t *testing.T) {
	allocs := testing.AllocsPerRun(1000, func() {
		_ = compareSOQLText("Beta Account", "alpha account")
		_ = compareSOQLText("Acme", "acme")
	})
	if allocs != 0 {
		t.Fatalf("compareSOQLText allocations = %v, want 0", allocs)
	}
}

func TestExecuteUsesSingleFieldIndexCandidates(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
			Indexes: []storage.IndexDefinition{{Name: "Account.Name", Object: "Account", Fields: []string{"Name"}}},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Beta")}},
		},
	}
	storage.RebuildIndexes(&org)
	result, err := ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Name = 'Acme'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000001" {
		t.Fatalf("indexed result = %#v", result)
	}
}

func TestExecuteUsesSingleFieldIndexCandidatesInsideAnd(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Status__c": {APIName: "Status__c", Type: storage.FieldString},
			},
			Indexes: []storage.IndexDefinition{{Name: "Account.Name", Object: "Account", Fields: []string{"Name"}}},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme"), "Status__c": storage.StringValue("Active")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Beta"), "Status__c": storage.StringValue("Active")}},
			"001000000000003": {ID: "001000000000003", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme"), "Status__c": storage.StringValue("Inactive")}},
		},
	}
	storage.RebuildIndexes(&org)
	result, err := ParseAndExecute(org, "SELECT Id FROM Account WHERE Status__c = 'Active' AND Name = 'Acme'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000001" {
		t.Fatalf("indexed AND result = %#v", result)
	}
}

func TestExecuteStringEqualityIsCaseInsensitiveWithIndex(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["CouponCode__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CouponCode__c",
			Fields: map[string]storage.Field{
				"Code__c": {APIName: "Code__c", Type: storage.FieldString},
			},
			Indexes: []storage.IndexDefinition{{Name: "CouponCode.Code", Object: "CouponCode__c", Fields: []string{"Code__c"}}},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "CouponCode__c", Fields: map[string]storage.Value{"Code__c": storage.StringValue("TESTCODE")}},
		},
	}
	storage.RebuildIndexes(&org)
	result, err := ParseAndExecute(org, "SELECT Id FROM CouponCode__c WHERE Code__c = 'testcode'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "a01000000000001" {
		t.Fatalf("case-insensitive indexed result = %#v", result)
	}
}

func TestExecuteProjectsFutureCalculatedFormulaField(t *testing.T) {
	codeStatus := storage.Field{APIName: "Status__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `IF (ISBLANK(Code__c), 'Inactive',
    IF(OR(!ISBLANK(EndDate__c) && EndDate__c < TODAY(), CouponRule__r.Status__c == 'Inactive'), 'Expired',
        IF(!ISBLANK(StartDate__c) && StartDate__c > TODAY(), 'Future', 'Active')
    )
)`}
	ruleStatus := storage.Field{APIName: "Status__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `IF(!ISBLANK(EndDate__c) && EndDate__c < TODAY(), 'Inactive', IF(!ISBLANK(StartDate__c) && StartDate__c > TODAY(), 'Future', 'Active'))`}
	org := storage.NewOrgState()
	org.Objects["CouponCode__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CouponCode__c",
			Fields: map[string]storage.Field{
				"Code__c":       {APIName: "Code__c", Type: storage.FieldString},
				"CouponRule__c": {APIName: "CouponRule__c", Type: storage.FieldReference, ReferenceTo: []string{"CouponRule__c"}},
				"StartDate__c":  {APIName: "StartDate__c", Type: storage.FieldDate},
				"EndDate__c":    {APIName: "EndDate__c", Type: storage.FieldDate},
				"Status__c":     codeStatus,
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "CouponCode__c", Fields: map[string]storage.Value{
				"Code__c":       storage.StringValue("TESTCODE"),
				"CouponRule__c": storage.IDValue("a02000000000001"),
				"StartDate__c":  storage.DateValue("2999-01-01"),
				"Status__c":     storage.StringValue("Active"),
			}},
		},
	}
	org.Objects["CouponRule__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CouponRule__c",
			Fields: map[string]storage.Field{
				"StartDate__c": {APIName: "StartDate__c", Type: storage.FieldDate},
				"EndDate__c":   {APIName: "EndDate__c", Type: storage.FieldDate},
				"Status__c":    ruleStatus,
			},
		},
		Records: map[storage.ID]storage.Record{
			"a02000000000001": {ID: "a02000000000001", Object: "CouponRule__c", Fields: map[string]storage.Value{}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, Status__c FROM CouponCode__c WHERE Code__c = 'testcode' AND Status__c = 'Future'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Status__c"].String != "Future" {
		t.Fatalf("future status result = %#v", result)
	}
}

func TestExecuteFieldsFunctionProjection(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Rating":    {APIName: "Rating", Type: storage.FieldString},
				"Score__c":  {APIName: "Score__c", Type: storage.FieldInteger},
				"Hidden__c": {APIName: "Hidden__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":      storage.StringValue("Acme"),
					"Rating":    storage.StringValue("Hot"),
					"Score__c":  storage.IntegerValue(7),
					"Hidden__c": storage.StringValue("kept"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT FIELDS(STANDARD) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	fields := result.Records[0].Fields
	if _, ok := fields["Id"]; !ok {
		t.Fatalf("Id missing from standard fields: %#v", fields)
	}
	if fields["Name"].String != "Acme" || fields["Rating"].String != "Hot" {
		t.Fatalf("standard fields = %#v", fields)
	}
	if _, ok := fields["Score__c"]; ok {
		t.Fatalf("custom field leaked into standard fields: %#v", fields)
	}

	result, err = ParseAndExecute(org, "SELECT FIELDS(CUSTOM) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	fields = result.Records[0].Fields
	if _, ok := fields["Name"]; ok {
		t.Fatalf("standard field leaked into custom fields: %#v", fields)
	}
	if fields["Score__c"].Integer != 7 || fields["Hidden__c"].String != "kept" {
		t.Fatalf("custom fields = %#v", fields)
	}

	result, err = ParseAndExecute(org, "SELECT Name, FIELDS(CUSTOM) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	fields = result.Records[0].Fields
	if fields["Name"].String != "Acme" || fields["Score__c"].Integer != 7 {
		t.Fatalf("mixed fields = %#v", fields)
	}
}

func TestExecuteAllRowsIncludesDeletedRecords(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Active")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Deleted")}, System: storage.SystemFields{IsDeleted: true}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, IsDeleted FROM Account ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000001" {
		t.Fatalf("default rows = %#v", result)
	}
	result, err = ParseAndExecute(org, "SELECT Id, IsDeleted FROM Account ORDER BY Name ALL ROWS")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Records[1].Fields["IsDeleted"].Kind != storage.ValueBoolean || !result.Records[1].Fields["IsDeleted"].Boolean {
		t.Fatalf("all rows = %#v", result)
	}
	query, err := Parse("SELECT Id FROM Account ALL ROWS FOR UPDATE")
	if err != nil {
		t.Fatal(err)
	}
	if !query.AllRows || !query.ForUpdate {
		t.Fatalf("query = %#v", query)
	}
}

func TestExecuteProjectsParentRelationshipField(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Relations: []storage.Relationship{{
				Field:              "ParentId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Parent",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Acme"),
				},
			},
			"001000000000002": {
				ID:     "001000000000002",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":     storage.StringValue("Child"),
					"ParentId": storage.IDValue("001000000000001"),
				},
			},
			"001000000000003": {
				ID:     "001000000000003",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("NoParent"),
				},
			},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {
				ID:     "003000000000001",
				Object: "Contact",
				Fields: map[string]storage.Value{
					"AccountId": storage.IDValue("001000000000001"),
				},
			},
			"003000000000002": {
				ID:     "003000000000002",
				Object: "Contact",
				Fields: map[string]storage.Value{
					"AccountId": storage.IDValue("001000000000002"),
				},
			},
			"003000000000003": {
				ID:     "003000000000003",
				Object: "Contact",
				Fields: map[string]storage.Value{
					"AccountId": storage.IDValue("001000000000003"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, Account.Name FROM Contact WHERE Account.Name = 'Acme'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d records=%#v", result.Rows, result.Records)
	}
	if got := result.Records[0].Fields["Account.Name"].String; got != "Acme" {
		t.Fatalf("Account.Name = %q", got)
	}
	if value := result.Records[0].Fields["Account.Id"]; value.Kind != storage.ValueID || value.ID != "001000000000001" {
		t.Fatalf("Account.Id = %#v", value)
	}
	result, err = ParseAndExecute(org, "SELECT Id, Account.Parent.Name FROM Contact WHERE Account.Parent.Name = 'Acme'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "003000000000002" {
		t.Fatalf("multi-hop rows = %#v", result.Records)
	}
	if got := result.Records[0].Fields["Account.Parent.Name"].String; got != "Acme" {
		t.Fatalf("Account.Parent.Name = %q", got)
	}
	if value := result.Records[0].Fields["Account.Id"]; value.Kind != storage.ValueID || value.ID != "001000000000002" {
		t.Fatalf("multi-hop Account.Id = %#v", value)
	}
	if value := result.Records[0].Fields["Account.Parent.Id"]; value.Kind != storage.ValueID || value.ID != "001000000000001" {
		t.Fatalf("multi-hop Account.Parent.Id = %#v", value)
	}
	result, err = ParseAndExecute(org, "SELECT Id, Account.Parent.Name FROM Contact WHERE Id = '003000000000003'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("missing parent relationship rows = %#v", result.Records)
	}
	if value := result.Records[0].Fields["Account.Parent"]; value.Kind != storage.ValueNull {
		t.Fatalf("missing parent relationship = %#v", value)
	}
	if _, ok := result.Records[0].Fields["Account.Parent.Name"]; ok {
		t.Fatalf("missing parent field should not be projected as a nested value: %#v", result.Records[0].Fields)
	}
}

func TestExecuteTypeofRelationshipProjection(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
		},
	}
	org.Objects["Opportunity"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Opportunity", KeyPrefix: "006", Fields: map[string]storage.Field{"Amount": {APIName: "Amount", Type: storage.FieldDecimal}}},
		Records: map[storage.ID]storage.Record{
			"006000000000001": {ID: "006000000000001", Object: "Opportunity", Fields: map[string]storage.Value{"Amount": storage.DecimalValue("42")}},
		},
	}
	org.Objects["Task"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Task",
			Fields: map[string]storage.Field{
				"Subject": {APIName: "Subject", Type: storage.FieldString},
				"WhatId":  {APIName: "WhatId", Type: storage.FieldReference, ReferenceTo: []string{"Account", "Opportunity"}, RelationshipName: "What"},
			},
			Relations: []storage.Relationship{{
				Field:              "WhatId",
				ParentObjects:      []string{"Account", "Opportunity"},
				ParentRelationship: "What",
				Polymorphic:        true,
			}},
		},
		Records: map[storage.ID]storage.Record{
			"00T000000000001": {ID: "00T000000000001", Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("A"), "WhatId": storage.IDValue("001000000000001")}},
			"00T000000000002": {ID: "00T000000000002", Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("B"), "WhatId": storage.IDValue("006000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, TYPEOF What WHEN Account THEN Name WHEN Opportunity THEN Amount END FROM Task ORDER BY Subject")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 {
		t.Fatalf("result = %#v", result)
	}
	if got := result.Records[0].Fields["What.Name"].String; got != "Acme" {
		t.Fatalf("What.Name = %q", got)
	}
	if got := result.Records[1].Fields["What.Amount"].Decimal; got != "42" {
		t.Fatalf("What.Amount = %q", got)
	}
}

func TestExecuteSecurityModeRequiresRelationshipFieldOnAllParentTargets(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Secret__c": {APIName: "Secret__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Opportunity"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Opportunity",
			KeyPrefix: "006",
			Fields: map[string]storage.Field{
				"Amount": {APIName: "Amount", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Task"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Task",
			Fields: map[string]storage.Field{
				"WhatId": {APIName: "WhatId", Type: storage.FieldReference, ReferenceTo: []string{"Account", "Opportunity"}, RelationshipName: "What"},
			},
			Relations: []storage.Relationship{{
				Field:              "WhatId",
				ParentObjects:      []string{"Account", "Opportunity"},
				ParentRelationship: "What",
				Polymorphic:        true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}

	if _, err := ParseAndExecute(org, "SELECT Id, What.Secret__c FROM Task WITH SECURITY_ENFORCED"); err == nil || !strings.Contains(err.Error(), "What.Secret__c") {
		t.Fatalf("expected relationship security projection error, got %v", err)
	}
}

func TestExecuteSecurityRelationshipWhereRequiresAllParentTargets(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Secret__c": {APIName: "Secret__c", Type: storage.FieldString}}},
		Records:    make(map[storage.ID]storage.Record),
	}
	org.Objects["Opportunity"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Opportunity", Fields: map[string]storage.Field{"Amount": {APIName: "Amount", Type: storage.FieldDecimal}}},
		Records:    make(map[storage.ID]storage.Record),
	}
	org.Objects["Task"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Task",
			Fields: map[string]storage.Field{
				"WhatId": {APIName: "WhatId", Type: storage.FieldReference, ReferenceTo: []string{"Account", "Opportunity"}, RelationshipName: "What"},
			},
			Relations: []storage.Relationship{{
				Field:              "WhatId",
				ParentObjects:      []string{"Account", "Opportunity"},
				ParentRelationship: "What",
				Polymorphic:        true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}

	if _, err := ParseAndExecute(org, "SELECT Id FROM Task WHERE What.Secret__c = 'hidden' WITH SECURITY_ENFORCED"); err == nil || !strings.Contains(err.Error(), "What.Secret__c") || !strings.Contains(err.Error(), "SECURITY_ENFORCED") {
		t.Fatalf("expected relationship security where error, got %v", err)
	}
}

func TestExecuteValidatesReferencesAcrossClauses(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name":     {APIName: "Name", Type: storage.FieldString},
				"Active":   {APIName: "Active", Type: storage.FieldBoolean},
				"Score__c": {APIName: "Score__c", Type: storage.FieldCalculated},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Low"), "Active": storage.BooleanValue(true), "Score__c": storage.IntegerValue(1)}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("High"), "Active": storage.BooleanValue(true), "Score__c": storage.IntegerValue(5)}},
		},
	}

	cases := []string{
		"SELECT Missing__c FROM Account",
		"SELECT Id FROM Account WHERE Missing__c = 'x'",
		"SELECT Id FROM Account ORDER BY Missing__c",
		"SELECT COUNT(Id) total FROM Account GROUP BY Active ORDER BY Name",
	}
	for _, query := range cases {
		if _, err := ParseAndExecute(org, query); err == nil || !strings.Contains(err.Error(), "Missing__c") && !strings.Contains(err.Error(), "must be grouped or aggregated") {
			t.Fatalf("expected validation error for %q, got %v", query, err)
		}
	}

	result, err := ParseAndExecute(org, "SELECT Id, Name, Score__c FROM Account WHERE Score__c > 1 WITH USER_MODE ORDER BY Score__c DESC")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Name"].String != "High" || result.Records[0].Fields["Score__c"].Integer != 5 {
		t.Fatalf("calculated field result = %#v", result)
	}
}

func TestExecuteRejectsUnknownCustomFieldOnKnownCustomObject(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["pkg__Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Line__c",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Type: storage.FieldString},
				"pkg__Status__c": {APIName: "pkg__Status__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "pkg__Line__c",
				Fields: map[string]storage.Value{
					"Name":           storage.StringValue("Line"),
					"pkg__Status__c": storage.StringValue("Active"),
				},
			},
		},
	}

	if _, err := ParseAndExecute(org, "SELECT pkg_invalid__c FROM pkg__Line__c"); err == nil || !strings.Contains(err.Error(), "pkg_invalid__c") {
		t.Fatalf("expected unknown custom field error, got %v", err)
	}
}

func TestExecuteEvaluatesFormulaFieldsInWhere(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Member__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Member__c",
			Fields: map[string]storage.Field{
				"StartDate__c": {APIName: "StartDate__c", Type: storage.FieldDate},
				"EndDate__c":   {APIName: "EndDate__c", Type: storage.FieldDate},
				"Pending__c":   {APIName: "Pending__c", Type: storage.FieldBoolean},
				"Status__c": {
					APIName: "Status__c",
					Type:    storage.FieldCalculated,
					Formula: "IF(Pending__c, 'Pending', IF(StartDate__c > TODAY(), 'Future', IF(EndDate__c >= TODAY(), 'Current', 'Expired')))",
				},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Member__c",
				Fields: map[string]storage.Value{
					"StartDate__c": storage.DateValue("2000-01-01"),
					"EndDate__c":   storage.DateValue("2999-12-31"),
				},
			},
			"a00000000000002": {
				ID:     "a00000000000002",
				Object: "Member__c",
				Fields: map[string]storage.Value{
					"StartDate__c": storage.DateValue("2000-01-01"),
					"EndDate__c":   storage.DateValue("2000-12-31"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, Status__c FROM Member__c WHERE Status__c IN ('Current') ORDER BY Id")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "a00000000000001" || result.Records[0].Fields["Status__c"].String != "Current" {
		t.Fatalf("formula where result = %#v", result)
	}
}

func TestExecuteEvaluatesTextFormulaFieldsInWhere(t *testing.T) {
	org := storage.NewOrgState()
	org.Now = func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	org.Objects["Member__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Member__c",
			Fields: map[string]storage.Field{
				"StartDate__c":    {APIName: "StartDate__c", Type: storage.FieldDate},
				"EndDate__c":      {APIName: "EndDate__c", Type: storage.FieldDate},
				"StampedState__c": {APIName: "StampedState__c", Type: storage.FieldString},
				"State__c": {
					APIName: "State__c",
					Type:    storage.FieldString,
					Formula: `IF(TODAY() >= StartDate__c && TODAY() > EndDate__c, "Past", "Current")`,
				},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Member__c",
				Fields: map[string]storage.Value{
					"StartDate__c":    storage.DateValue("2026-01-01"),
					"EndDate__c":      storage.DateValue("2026-05-01"),
					"StampedState__c": storage.StringValue("Current"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, State__c FROM Member__c WHERE StampedState__c = 'Current' AND State__c != 'Current'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["State__c"].String != "Past" {
		t.Fatalf("text formula where result = %#v", result)
	}
}

func TestExecuteEvaluatesDateDifferenceFormulaFields(t *testing.T) {
	org := storage.NewOrgState()
	org.Now = func() time.Time { return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC) }
	org.Objects["Order__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Order__c",
			Fields: map[string]storage.Field{
				"InvoiceDate__c": {APIName: "InvoiceDate__c", Type: storage.FieldDate},
				"Balance__c":     {APIName: "Balance__c", Type: storage.FieldDecimal},
				"InvoiceDaysOutstanding__c": {
					APIName:     "InvoiceDaysOutstanding__c",
					Type:        storage.FieldCalculated,
					DisplayType: "DOUBLE",
					Formula:     "IF(IsBlank(InvoiceDate__c) || Balance__c == 0, 0, Today() - InvoiceDate__c)",
				},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Order__c",
				Fields: map[string]storage.Value{
					"InvoiceDate__c": storage.DateValue("2026-04-21"),
					"Balance__c":     storage.DecimalValue("25"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, InvoiceDaysOutstanding__c FROM Order__c WHERE InvoiceDaysOutstanding__c <= 30")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["InvoiceDaysOutstanding__c"].Decimal != "30" {
		t.Fatalf("date difference formula result = %#v", result)
	}
}

func TestExecuteEvaluatesDateFormulaOverrideFieldsInWhere(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "PKG"
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"LapsedOnOverride__c": {APIName: "LapsedOnOverride__c", Type: storage.FieldDate},
				"LapsedOn__c": {
					APIName:     "LapsedOn__c",
					Type:        storage.FieldCalculated,
					DisplayType: "DATE",
					Formula: `IF(ISBLANK(LapsedOnOverride__c), (IF(Membership__r.Pending__c,
            null,
            IF(!ISBLANK(Membership__r.EndDateOverride__c),
            Membership__r.EndDate__c + 1,
            IF(ISPICKVAL(Membership__r.MembershipType2__r.GracePeriodUnit__c, 'Day') ,
            Membership__r.EndDate__c + Membership__r.MembershipType2__r.GracePeriod__c + 1,
            DATE(
            YEAR(Membership__r.EndDate__c) + FLOOR((MONTH(Membership__r.EndDate__c) + Membership__r.MembershipType2__r.GracePeriod__c) / 12),
            MOD(MONTH(Membership__r.EndDate__c) + Membership__r.MembershipType2__r.GracePeriod__c, 12) + 1,
            1
            )
            )
            ))), LapsedOnOverride__c)`,
				},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"PKG__LapsedOnOverride__c": storage.DateValue("2026-05-02"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, LapsedOn__c FROM Account WHERE LapsedOn__c = 2026-05-02")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["LapsedOn__c"].String != "2026-05-02" {
		t.Fatalf("date formula override result = %#v", result)
	}

	result, err = ParseAndExecute(org, "SELECT Id, LapsedOn__c FROM Account WHERE PKG__LapsedOn__c = 2026-05-02")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["LapsedOn__c"].String != "2026-05-02" {
		t.Fatalf("namespaced date formula override result = %#v", result)
	}

	result, err = ParseAndExecute(org, "SELECT Id FROM Account WHERE PKG__LapsedOn__c = 2026-05-02")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("namespaced date formula filter-only result = %#v", result)
	}
}

func TestExecuteSupportsFromRelationshipAlias(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CartItem__c",
			Fields: map[string]storage.Field{
				"Cart__c": {APIName: "Cart__c", Type: storage.FieldReference, ReferenceTo: []string{"Cart__c"}, RelationshipName: "Cart__r"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "CartItem__c",
				Fields: map[string]storage.Value{
					"Cart__c": storage.IDValue("a01000000000001"),
				},
			},
		},
	}
	org.Objects["Cart__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Cart__c",
			Fields: map[string]storage.Field{
				"BillTo__c": {APIName: "BillTo__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "BillTo__r"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Cart__c",
				Fields: map[string]storage.Value{
					"BillTo__c": storage.IDValue("001000000000001"),
				},
			},
		},
	}
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"TaxExempt__c": {APIName: "TaxExempt__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"TaxExempt__c": storage.BooleanValue(true),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, `SELECT BillTo.Id, BillTo.TaxExempt__c FROM CartItem__c, CartItem__c.Cart__r.BillTo__r BillTo WHERE Cart__c = 'a01000000000001'`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d, want 1", result.Rows)
	}
	if got := result.Records[0].Fields["Cart__r.BillTo__r.Id"].ID; got != "001000000000001" {
		t.Fatalf("aliased parent id = %s fields=%#v", got, result.Records[0].Fields)
	}
	if got := result.Records[0].Fields["Cart__r.BillTo__r.TaxExempt__c"].Boolean; !got {
		t.Fatalf("aliased parent TaxExempt__c = %v", got)
	}
}

func TestExecuteEvaluatesParentFormulaField(t *testing.T) {
	org := storage.NewOrgState()
	batchDefinition := storage.ObjectDefinition{
		APIName:   "Batch__c",
		KeyPrefix: "a00",
		Fields: map[string]storage.Field{
			"Status__c": {APIName: "Status__c", Type: storage.FieldPicklist},
		},
	}
	transactionDefinition := storage.ObjectDefinition{
		APIName:   "Transaction__c",
		KeyPrefix: "a01",
		Fields: map[string]storage.Field{
			"Batch__c":  {APIName: "Batch__c", Type: storage.FieldReference, ReferenceTo: []string{"Batch__c"}, RelationshipName: "Transactions"},
			"Status__c": {APIName: "Status__c", Type: storage.FieldCalculated, Formula: "Text(Batch__r.Status__c)"},
		},
	}
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: batchDefinition,
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Batch__c",
				Fields: map[string]storage.Value{"Status__c": storage.StringValue("Open")},
			},
		},
	}
	org.Objects["Transaction__c"] = storage.ObjectState{
		Definition: transactionDefinition,
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Transaction__c",
				Fields: map[string]storage.Value{"Batch__c": storage.IDValue("a00000000000001")},
			},
		},
	}
	result, err := ParseAndExecute(org, "SELECT Id, Status__c FROM Transaction__c WHERE Status__c = 'Open'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Status__c"].String != "Open" {
		t.Fatalf("parent formula result = %#v", result)
	}
}

func TestExecuteParentFormulaPrefersCurrentLookupRecordOverEmbeddedParent(t *testing.T) {
	org := storage.NewOrgState()
	lineDefinition := storage.ObjectDefinition{
		APIName:   "OrderItemLine__c",
		KeyPrefix: "a02",
		Fields: map[string]storage.Field{
			"Status__c": {APIName: "Status__c", Type: storage.FieldPicklist},
		},
	}
	merchandiseDefinition := storage.ObjectDefinition{
		APIName:   "Merchandise__c",
		KeyPrefix: "a03",
		Fields: map[string]storage.Field{
			"OrderItemLine__c": {
				APIName:          "OrderItemLine__c",
				Type:             storage.FieldReference,
				ReferenceTo:      []string{"OrderItemLine__c"},
				RelationshipName: "Merchandises",
			},
			"Status__c": {
				APIName: "Status__c",
				Type:    storage.FieldCalculated,
				Formula: "IF(ISBLANK(TEXT(OrderItemLine__r.Status__c)),'Imported',TEXT(OrderItemLine__r.Status__c))",
			},
		},
	}
	org.Objects["OrderItemLine__c"] = storage.ObjectState{
		Definition: lineDefinition,
		Records: map[storage.ID]storage.Record{
			"a02000000000001": {
				ID:     "a02000000000001",
				Object: "OrderItemLine__c",
				Fields: map[string]storage.Value{"Status__c": storage.StringValue("Cancelled")},
			},
		},
	}
	org.Objects["Merchandise__c"] = storage.ObjectState{
		Definition: merchandiseDefinition,
		Records: map[storage.ID]storage.Record{
			"a03000000000001": {
				ID:     "a03000000000001",
				Object: "Merchandise__c",
				Fields: map[string]storage.Value{
					"OrderItemLine__c": storage.IDValue("a02000000000001"),
				},
				ParentRelationships: map[string]storage.Record{
					"OrderItemLine__r": {
						ID:     "a02000000000001",
						Object: "OrderItemLine__c",
						Fields: map[string]storage.Value{"Status__c": storage.StringValue("Active")},
					},
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, Status__c FROM Merchandise__c WHERE Id = 'a03000000000001'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Status__c"].String != "Cancelled" {
		t.Fatalf("parent formula used stale embedded parent = %#v", result)
	}
}

func TestExecuteNotInIgnoresNullCandidates(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Parent__c",
			Fields:  map[string]storage.Field{"Name__c": {APIName: "Name__c", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	childDefinition := storage.ObjectDefinition{
		APIName: "Child__c",
		Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent"},
		},
		Relations: []storage.Relationship{{
			Field:              "Parent__c",
			ParentObjects:      []string{"Parent__c"},
			ParentRelationship: "Parent__r",
		}},
	}
	storage.EnsureStandardObjectFields(&childDefinition)
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: childDefinition,
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Child__c",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("orphan"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id FROM Child__c WHERE Parent__r.Name__c NOT IN (null)")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d, want 1", result.Rows)
	}
}

func TestExecuteValidatesRelationshipReferencesAcrossClauses(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Secret__c": {APIName: "Secret__c", Type: storage.FieldString}}},
		Records:    make(map[storage.ID]storage.Record),
	}
	org.Objects["Opportunity"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Opportunity", Fields: map[string]storage.Field{"Amount": {APIName: "Amount", Type: storage.FieldDecimal}}},
		Records:    make(map[storage.ID]storage.Record),
	}
	org.Objects["Task"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Task",
			Fields: map[string]storage.Field{
				"WhatId": {APIName: "WhatId", Type: storage.FieldReference, ReferenceTo: []string{"Account", "Opportunity"}, RelationshipName: "What"},
			},
			Relations: []storage.Relationship{{
				Field:              "WhatId",
				ParentObjects:      []string{"Account", "Opportunity"},
				ParentRelationship: "What",
				Polymorphic:        true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}

	for _, query := range []string{
		"SELECT Id, What.Missing__c FROM Task",
		"SELECT Id FROM Task WHERE What.Missing__c = 'hidden'",
		"SELECT Id FROM Task ORDER BY What.Missing__c",
	} {
		if _, err := ParseAndExecute(org, query); err == nil || !strings.Contains(err.Error(), "What.Missing__c") {
			t.Fatalf("expected relationship validation error for %q, got %v", query, err)
		}
	}
}

func TestExecuteValidatesSystemParentFieldsWithLazyStandardParent(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	delete(org.Objects, "User")

	if _, err := ParseAndExecute(org, "SELECT Id, CreatedBy.Email, CreatedBy.Title FROM Account"); err != nil {
		t.Fatalf("expected lazy standard User parent fields to validate, got %v", err)
	}
	if _, ok := org.Objects["User"]; ok {
		t.Fatal("query validation should not mutate the caller org")
	}

	org.Objects["Audit_Target__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Audit_Target__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    make(map[storage.ID]storage.Record),
	}
	if _, err := ParseAndExecute(org, "SELECT Id, CreatedBy.Email, LastModifiedBy.Title, Owner.Username FROM Audit_Target__c"); err != nil {
		t.Fatalf("expected virtual system parent fields to validate, got %v", err)
	}
}

func TestExecuteVirtualSchemaHydrationDoesNotMutateCallerOrg(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")

	if _, err := ParseAndExecute(org, "SELECT Id FROM Account"); err != nil {
		t.Fatal(err)
	}
	for _, objectName := range []string{"EntityDefinition", "EntityParticle", "RelationshipDomain", "UserEntityAccess", "UserFieldAccess"} {
		if _, ok := org.Objects[objectName]; ok {
			t.Fatalf("query should not add %s to the caller org", objectName)
		}
	}
}

func TestExecuteProjectsChildRelationshipSubquery(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Beta")}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Zulu"), "AccountId": storage.IDValue("001000000000001")}},
			"003000000000002": {ID: "003000000000002", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Alpha"), "AccountId": storage.IDValue("001000000000001")}},
			"003000000000003": {ID: "003000000000003", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Beta"), "AccountId": storage.IDValue("001000000000002")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, Name, (SELECT Id, LastName FROM Contacts WHERE LastName != 'Zulu' ORDER BY LastName LIMIT 1) FROM Account ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 {
		t.Fatalf("Rows = %d", result.Rows)
	}
	acmeChildren := result.Records[0].Children["Contacts"]
	if len(acmeChildren) != 1 || acmeChildren[0].Fields["LastName"].String != "Alpha" {
		t.Fatalf("Acme children = %#v", acmeChildren)
	}
	betaChildren := result.Records[1].Children["Contacts"]
	if len(betaChildren) != 1 || betaChildren[0].Fields["LastName"].String != "Beta" {
		t.Fatalf("Beta children = %#v", betaChildren)
	}

	result, err = ParseAndExecute(org, "SELECT Id, (SELECT Id, LastName FROM Contacts ORDER BY AccountId ASC, LastName DESC NULLS LAST LIMIT 1) FROM Account WHERE Name = 'Acme'")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["Contacts"]
	if len(children) != 1 || children[0].Fields["LastName"].String != "Zulu" {
		t.Fatalf("multi-order children = %#v", children)
	}

	contactObject := org.Objects["Contact"]
	contactObject.Records["003000000000004"] = storage.Record{ID: "003000000000004", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000001")}}
	org.Objects["Contact"] = contactObject
	result, err = ParseAndExecute(org, "SELECT Id, (SELECT Id, LastName FROM Contacts ORDER BY LastName ASC NULLS FIRST LIMIT 1) FROM Account WHERE Name = 'Acme'")
	if err != nil {
		t.Fatal(err)
	}
	children = result.Records[0].Children["Contacts"]
	if len(children) != 1 || children[0].ID != "003000000000004" {
		t.Fatalf("nulls first children = %#v", children)
	}

	result, err = ParseAndExecute(org, "SELECT Id, (SELECT FIELDS(STANDARD) FROM Contacts ORDER BY LastName ASC NULLS LAST LIMIT 1) FROM Account WHERE Name = 'Acme'")
	if err != nil {
		t.Fatal(err)
	}
	children = result.Records[0].Children["Contacts"]
	if len(children) != 1 || children[0].Fields["LastName"].String != "Alpha" || children[0].Fields["AccountId"].ID != "001000000000001" {
		t.Fatalf("child FIELDS() rows = %#v", children)
	}

	result, err = ParseAndExecute(org, "SELECT Id, (SELECT LastName FROM Account.Contacts ORDER BY LastName ASC NULLS LAST LIMIT 1) FROM Account WHERE Name = 'Acme'")
	if err != nil {
		t.Fatal(err)
	}
	children = result.Records[0].Children["Contacts"]
	if len(children) != 1 || children[0].Fields["LastName"].String != "Alpha" {
		t.Fatalf("qualified child relationship rows = %#v", children)
	}
}

func TestExecuteProjectsNestedChildRelationshipSubquery(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Smith"), "AccountId": storage.IDValue("001000000000001")}},
		},
	}
	org.Objects["Case"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Case",
			Fields: map[string]storage.Field{
				"Subject":   {APIName: "Subject", Type: storage.FieldString},
				"ContactId": {APIName: "ContactId", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}},
			},
			Relations: []storage.Relationship{{
				Field:              "ContactId",
				ParentObjects:      []string{"Contact"},
				ParentRelationship: "Contact",
				ChildRelationship:  "Cases",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"500000000000001": {ID: "500000000000001", Object: "Case", Fields: map[string]storage.Value{"Subject": storage.StringValue("Open"), "ContactId": storage.IDValue("003000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Id, (SELECT Id, Subject FROM Cases) FROM Contacts) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	contacts := result.Records[0].Children["Contacts"]
	if len(contacts) != 1 {
		t.Fatalf("contacts = %#v", contacts)
	}
	cases := contacts[0].Children["Cases"]
	if len(cases) != 1 || cases[0].Fields["Subject"].String != "Open" {
		t.Fatalf("cases = %#v", cases)
	}
}

func TestExecuteChildRelationshipSubqueryPrefersCurrentPackageObject(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "PKG"
	org.Objects["PKG__RecurringPayment__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "PKG__RecurringPayment__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a10000000000001": {ID: "a10000000000001", Object: "PKG__RecurringPayment__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Recurring")}},
		},
	}
	org.Objects["Membership__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Membership__c",
			Fields: map[string]storage.Field{
				"RecurringPayment__c": {APIName: "RecurringPayment__c", Type: storage.FieldReference, ReferenceTo: []string{"PKG__RecurringPayment__c"}, RelationshipName: "RecurringPayment__r", ChildRelationshipName: "Memberships__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "RecurringPayment__c",
				ParentObjects:      []string{"PKG__RecurringPayment__c"},
				ParentRelationship: "RecurringPayment__r",
				ChildRelationship:  "Memberships__r",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["PKG__Membership__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "PKG__Membership__c",
			KeyPrefix: "a11",
			Fields: map[string]storage.Field{
				"PKG__Balance__c":          {APIName: "PKG__Balance__c", Type: storage.FieldDecimal},
				"PKG__RecurringPayment__c": {APIName: "PKG__RecurringPayment__c", Type: storage.FieldReference, ReferenceTo: []string{"PKG__RecurringPayment__c"}, RelationshipName: "PKG__RecurringPayment__r", ChildRelationshipName: "PKG__Memberships__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "PKG__RecurringPayment__c",
				ParentObjects:      []string{"PKG__RecurringPayment__c"},
				ParentRelationship: "PKG__RecurringPayment__r",
				ChildRelationship:  "PKG__Memberships__r",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a11000000000001": {ID: "a11000000000001", Object: "PKG__Membership__c", Fields: map[string]storage.Value{"PKG__Balance__c": storage.DecimalValue("25"), "PKG__RecurringPayment__c": storage.IDValue("a10000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Id, Balance__c FROM Memberships__r WHERE Balance__c > 0) FROM RecurringPayment__c")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["Memberships__r"]
	if len(children) != 1 || children[0].Object != "PKG__Membership__c" || children[0].Fields["PKG__Balance__c"].Decimal != "25" {
		t.Fatalf("current package children = %#v", children)
	}
}

func TestExecuteChildRelationshipSubqueryUsesSyntheticAuditRelationship(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "User",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"Name": storage.StringValue("Admin")}},
		},
	}
	org.Objects["Invoice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Invoice__c",
			PluralLabel: "Invoices",
			Fields:      map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Invoice__c",
				Fields: map[string]storage.Value{"Name": storage.StringValue("INV-1")},
				System: storage.SystemFields{CreatedByID: "005000000000001"},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Id, Name, CreatedById FROM Invoices) FROM User")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["Invoices"]
	if len(children) != 1 || children[0].Fields["Name"].String != "INV-1" {
		t.Fatalf("children = %#v", children)
	}
}

func TestExecuteWithCacheReusesChildRelationshipResolutionOnly(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Alpha"), "AccountId": storage.IDValue("001000000000001")}},
		},
	}
	cache := NewExecutionCache()
	firstQuery, err := Parse("SELECT Id, (SELECT Id, LastName FROM Contacts) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	first, err := ExecuteWithCache(org, firstQuery, cache)
	if err != nil {
		t.Fatal(err)
	}
	if got := first.Records[0].Children["Contacts"][0].Fields["LastName"].String; got != "Alpha" {
		t.Fatalf("first child LastName = %q", got)
	}
	contactState := org.Objects["Contact"]
	contactState.Records["003000000000002"] = storage.Record{ID: "003000000000002", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Beta"), "AccountId": storage.IDValue("001000000000001")}}
	org.Objects["Contact"] = contactState
	secondQuery, err := Parse("SELECT Id, (SELECT Id, LastName FROM Contacts ORDER BY LastName DESC LIMIT 1) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExecuteWithCache(org, secondQuery, cache)
	if err != nil {
		t.Fatal(err)
	}
	children := second.Records[0].Children["Contacts"]
	if len(children) != 1 || children[0].Fields["LastName"].String != "Beta" {
		t.Fatalf("second child rows = %#v", children)
	}
	if len(cache.childRelationships) != 1 {
		t.Fatalf("child relationship cache entries = %d, want 1", len(cache.childRelationships))
	}
}

func TestExecuteWithCacheConcurrentChildRelationshipResolution(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Alpha"), "AccountId": storage.IDValue("001000000000001")}},
		},
	}
	query, err := Parse("SELECT Id, (SELECT Id, LastName FROM Contacts) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 16; j++ {
				result, err := ExecuteWithCache(org, query, cache)
				if err != nil {
					errs <- err
					return
				}
				children := result.Records[0].Children["Contacts"]
				if len(children) != 1 || children[0].Fields["LastName"].String != "Alpha" {
					errs <- errors.New("unexpected child relationship rows")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestValidateQueryReferencesUsesChildRelationshipExecutionCache(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
			}},
		},
	}
	query, err := Parse("SELECT Id, (SELECT Id, LastName FROM Contacts) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	childCache := newChildRelationshipQueryCache(cache)
	account := org.Objects["Account"]

	if err := validateQueryReferencesWithChildCache(org, account.Definition, query, query.SecurityMode, childCache); err != nil {
		t.Fatal(err)
	}
	if len(cache.childRelationships) != 1 {
		t.Fatalf("child relationship cache entries = %d, want 1", len(cache.childRelationships))
	}
}

func TestNormalizeChildRelationshipSelectFieldsUsesExecutionCache(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields:  map[string]storage.Field{"LastName": {APIName: "LastName", Type: storage.FieldString}},
		},
	}
	cache := NewExecutionCache()
	childCache := newChildRelationshipQueryCache(cache)
	contact := org.Objects["Contact"]

	fields := normalizeChildRelationshipSelectFields(org, "Contact", contact.Definition, []string{"Contact.LastName"}, childCache)
	if len(fields) != 1 || fields[0] != "LastName" {
		t.Fatalf("normalized fields = %#v, want LastName", fields)
	}
	if len(cache.childFieldQualifierMatches) != 1 {
		t.Fatalf("field qualifier cache entries = %d, want 1", len(cache.childFieldQualifierMatches))
	}
}

func TestExecuteChildRelationshipUsesStandardRelationshipWithoutMetadataRelation(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Alpha"), "AccountId": storage.IDValue("001000000000001")}},
		},
	}
	query, err := Parse("SELECT Id, (SELECT Id, LastName FROM Contacts) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["Contacts"]
	if len(children) != 1 || children[0].Fields["LastName"].String != "Alpha" {
		t.Fatalf("children = %#v", children)
	}
}

func TestExecuteChildRelationshipPrefersExplicitCustomRelationship(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
		},
	}
	org.Objects["Order"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Order",
			Fields: map[string]storage.Field{
				"AccountId":  {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
				"GrandTotal": {APIName: "GrandTotal", Type: storage.FieldDecimal},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
			}},
			PluralLabel: "Orders",
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Order__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Order__c",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
				"Balance__c": {APIName: "Balance__c", Type: storage.FieldDecimal},
			},
			Relations: []storage.Relationship{{
				Field:              "Account__c",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account__r",
				ChildRelationship:  "Orders__r",
			}},
			PluralLabel: "Orders",
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "Order__c", Fields: map[string]storage.Value{"Account__c": storage.IDValue("001000000000001"), "Balance__c": storage.DecimalValue("42")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Balance__c FROM Orders__r) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["Orders__r"]
	if len(children) != 1 || children[0].Fields["Balance__c"].Decimal != "42" {
		t.Fatalf("children = %#v", children)
	}
}

func TestExecuteCustomChildRelationshipSubqueryAcceptsRuntimeSuffix(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Cart__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Cart__c",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a0S000000000001": {ID: "a0S000000000001", Object: "Cart__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Cart")}},
		},
	}
	org.Objects["CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CartItem__c",
			Fields: map[string]storage.Field{
				"Name":    {APIName: "Name", Type: storage.FieldString},
				"Cart__c": {APIName: "Cart__c", Type: storage.FieldReference, ReferenceTo: []string{"Cart__c"}},
			},
			Relations: []storage.Relationship{{
				Field:              "Cart__c",
				ParentObjects:      []string{"Cart__c"},
				ParentRelationship: "Cart__r",
				ChildRelationship:  "CartItems",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a0I000000000001": {ID: "a0I000000000001", Object: "CartItem__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Line"), "Cart__c": storage.IDValue("a0S000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Id, Name FROM CartItems__r) FROM Cart__c")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["CartItems__r"]
	if len(children) != 1 || children[0].Fields["Name"].String != "Line" {
		t.Fatalf("children = %#v", children)
	}
}

func TestExecuteCustomChildRelationshipSubqueryInfersPrefixedLinkObject(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["pkg__MembershipType__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__MembershipType__c",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"a0r000000000001": {ID: "a0r000000000001", Object: "pkg__MembershipType__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Regular")}},
		},
	}
	org.Objects["pkg__MembershipTypeProductLink__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__MembershipTypeProductLink__c",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"a0q000000000001": {ID: "a0q000000000001", Object: "pkg__MembershipTypeProductLink__c", Fields: map[string]storage.Value{
				"Name":                   storage.StringValue("Link"),
				"pkg__MembershipType__c": storage.IDValue("a0r000000000001"),
			}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Id, Name FROM pkg__MembershipTypeProductLinks__r) FROM pkg__MembershipType__c")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["pkg__MembershipTypeProductLinks__r"]
	if len(children) != 1 || children[0].Fields["Name"].String != "Link" {
		t.Fatalf("children = %#v", children)
	}
}

func TestExecuteCustomParentRelationshipFilterUsesDerivedName(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Cart__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Cart__c",
			Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
		},
		Records: map[storage.ID]storage.Record{
			"a0S000000000001": {ID: "a0S000000000001", Object: "Cart__c"},
		},
	}
	org.Objects["CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CartItem__c",
			Fields: map[string]storage.Field{
				"Id":      {APIName: "Id", Type: storage.FieldID},
				"Cart__c": {APIName: "Cart__c", Type: storage.FieldReference, ReferenceTo: []string{"Cart__c"}, RelationshipName: "CartItems"},
			},
			Relations: []storage.Relationship{{
				Field:              "Cart__c",
				ParentObjects:      []string{"Cart__c"},
				ParentRelationship: "CartItems",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a0I000000000001": {ID: "a0I000000000001", Object: "CartItem__c", Fields: map[string]storage.Value{"Cart__c": storage.IDValue("a0S000000000001")}},
		},
	}
	org.Objects["CartItemLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CartItemLine__c",
			Fields: map[string]storage.Field{
				"Id":          {APIName: "Id", Type: storage.FieldID},
				"CartItem__c": {APIName: "CartItem__c", Type: storage.FieldReference, ReferenceTo: []string{"CartItem__c"}, RelationshipName: "CartItemLines"},
			},
			Relations: []storage.Relationship{{
				Field:              "CartItem__c",
				ParentObjects:      []string{"CartItem__c"},
				ParentRelationship: "CartItemLines",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a0L000000000001": {ID: "a0L000000000001", Object: "CartItemLine__c", Fields: map[string]storage.Value{"CartItem__c": storage.IDValue("a0I000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id FROM CartItemLine__c WHERE CartItem__r . Cart__c = 'a0S000000000001'")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 || result.Records[0].ID != "a0L000000000001" {
		t.Fatalf("records = %#v", result.Records)
	}
}

func TestExecuteNamespacedCustomParentRelationshipFilterUsesUnqualifiedName(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	assertNamespacedCustomParentRelationshipFilterUsesUnqualifiedName(t, org)
}

func TestExecutePackagedCustomParentRelationshipFilterWithoutOrgNamespace(t *testing.T) {
	org := storage.NewOrgState()
	assertNamespacedCustomParentRelationshipFilterUsesUnqualifiedName(t, org)
}

func TestExecuteMetadataCustomParentRelationshipFilterUsesLookupField(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Cart__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Cart__c",
			Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
		},
		Records: map[storage.ID]storage.Record{
			"a0S000000000001": {ID: "a0S000000000001", Object: "Cart__c"},
		},
	}
	org.Objects["CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CartItem__c",
			Fields: map[string]storage.Field{
				"Id":      {APIName: "Id", Type: storage.FieldID},
				"Cart__c": {APIName: "Cart__c", Type: storage.FieldReference, ReferenceTo: []string{"Cart__c"}},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a0I000000000001": {ID: "a0I000000000001", Object: "CartItem__c", Fields: map[string]storage.Value{"Cart__c": storage.IDValue("a0S000000000001")}},
		},
	}
	org.Objects["CartItemLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CartItemLine__c",
			Fields: map[string]storage.Field{
				"Id":          {APIName: "Id", Type: storage.FieldID},
				"CartItem__c": {APIName: "CartItem__c", Type: storage.FieldReference, ReferenceTo: []string{"CartItem__c"}},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a0L000000000001": {ID: "a0L000000000001", Object: "CartItemLine__c", Fields: map[string]storage.Value{"CartItem__c": storage.IDValue("a0I000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id FROM CartItemLine__c WHERE CartItem__r.Cart__c = 'a0S000000000001'")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 || result.Records[0].ID != "a0L000000000001" {
		t.Fatalf("records = %#v", result.Records)
	}
}

func assertNamespacedCustomParentRelationshipFilterUsesUnqualifiedName(t *testing.T, org storage.OrgState) {
	t.Helper()
	org.Objects["pkg__Cart__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Cart__c",
			Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
		},
		Records: map[storage.ID]storage.Record{
			"a0S000000000001": {ID: "a0S000000000001", Object: "Cart__c"},
		},
	}
	org.Objects["pkg__CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__CartItem__c",
			Fields: map[string]storage.Field{
				"Id":           {APIName: "Id", Type: storage.FieldID},
				"pkg__Cart__c": {APIName: "pkg__Cart__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Cart__c"}, RelationshipName: "pkg__Cart__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "pkg__Cart__c",
				ParentObjects:      []string{"pkg__Cart__c"},
				ParentRelationship: "pkg__Cart__r",
				ChildRelationship:  "pkg__CartItems__r",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a0I000000000001": {ID: "a0I000000000001", Object: "CartItem__c", Fields: map[string]storage.Value{"Cart__c": storage.IDValue("a0S000000000001")}},
		},
	}
	org.Objects["pkg__CartItemLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__CartItemLine__c",
			Fields: map[string]storage.Field{
				"Id":               {APIName: "Id", Type: storage.FieldID},
				"pkg__CartItem__c": {APIName: "pkg__CartItem__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__CartItem__c"}, RelationshipName: "pkg__CartItem__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "pkg__CartItem__c",
				ParentObjects:      []string{"pkg__CartItem__c"},
				ParentRelationship: "pkg__CartItem__r",
				ChildRelationship:  "pkg__CartItemLines__r",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a0L000000000001": {ID: "a0L000000000001", Object: "CartItemLine__c", Fields: map[string]storage.Value{"CartItem__c": storage.IDValue("a0I000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id FROM CartItemLine__c WHERE CartItem__r.Cart__c = 'a0S000000000001'")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 || result.Records[0].ID != "a0L000000000001" {
		t.Fatalf("records = %#v", result.Records)
	}
}

func TestExecuteCustomChildRelationshipSubqueryAcceptsNamespacedRelationship(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Cart__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Cart__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records: map[storage.ID]storage.Record{
			"a0S000000000001": {ID: "a0S000000000001", Object: "Cart__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Cart")}},
		},
	}
	org.Objects["CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CartItem__c",
			Fields: map[string]storage.Field{
				"Name":    {APIName: "Name", Type: storage.FieldString},
				"Cart__c": {APIName: "Cart__c", Type: storage.FieldReference, ReferenceTo: []string{"Cart__c"}},
			},
			Relations: []storage.Relationship{{
				Field:              "Cart__c",
				ParentObjects:      []string{"Cart__c"},
				ParentRelationship: "Cart__r",
				ChildRelationship:  "CartItems__r",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a0I000000000001": {ID: "a0I000000000001", Object: "CartItem__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Line"), "Cart__c": storage.IDValue("a0S000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Id, Name FROM pkg__CartItems__r) FROM pkg__Cart__c")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["pkg__CartItems__r"]
	if len(children) != 1 || children[0].Fields["Name"].String != "Line" {
		t.Fatalf("children = %#v", children)
	}
}

func TestExecuteChildRelationshipSubqueryMatchesDependencyNamespacedParentAlias(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "otherpkg"
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Product__c",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"a0P000000000001": {ID: "a0P000000000001", Object: "Product__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Membership")}},
		},
	}
	org.Objects["pkg__SpecialPrice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__SpecialPrice__c",
			Fields: map[string]storage.Field{
				"Name":            {APIName: "Name", Type: storage.FieldString},
				"pkg__Product__c": {APIName: "pkg__Product__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Product__c"}},
			},
			Relations: []storage.Relationship{{
				Field:              "pkg__Product__c",
				ParentObjects:      []string{"pkg__Product__c"},
				ParentRelationship: "pkg__Product__r",
				ChildRelationship:  "pkg__SpecialPrices__r",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a0S000000000001": {ID: "a0S000000000001", Object: "pkg__SpecialPrice__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Early"), "pkg__Product__c": storage.IDValue("a0P000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Id, Name FROM SpecialPrices__r) FROM Product__c")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["SpecialPrices__r"]
	if len(children) != 1 || children[0].Fields["Name"].String != "Early" {
		t.Fatalf("children = %#v", children)
	}
}

func TestExecuteChildRelationshipSubqueryPrefersLocalRelationshipOverDependencyNamespaceCollision(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "otherpkg"
	org.Objects["StateConfiguration__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "StateConfiguration__mdt",
			Fields: map[string]storage.Field{
				"IsActive__c": {APIName: "IsActive__c", Type: storage.FieldBoolean},
			},
			Metadata: map[string]string{"kind": "customMetadata"},
		},
		Records: map[storage.ID]storage.Record{
			"a0j000000000001": {
				ID:     "a0j000000000001",
				Object: "StateConfiguration__mdt",
				Fields: map[string]storage.Value{
					"QualifiedApiName": storage.StringValue("OrderGraph"),
					"IsActive__c":      storage.BooleanValue(true),
				},
			},
		},
	}
	localDefinition := storage.ObjectDefinition{
		APIName: "StateTransition__mdt",
		Fields: map[string]storage.Field{
			"IsActive__c":           {APIName: "IsActive__c", Type: storage.FieldBoolean},
			"FromStates__c":         {APIName: "FromStates__c", Type: storage.FieldString},
			"StateConfiguration__c": {APIName: "StateConfiguration__c", Type: storage.FieldReference, ReferenceTo: []string{"StateConfiguration__mdt"}, RelationshipName: "StateConfiguration__r", ChildRelationshipName: "StateTransitions__r"},
		},
		Relations: []storage.Relationship{{
			Field:              "StateConfiguration__c",
			ParentObjects:      []string{"StateConfiguration__mdt"},
			ParentRelationship: "StateConfiguration__r",
			ChildRelationship:  "StateTransitions__r",
		}},
		Metadata: map[string]string{"kind": "customMetadata"},
	}
	org.Objects["pkg__StateTransition__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__StateTransition__mdt",
			Fields: map[string]storage.Field{
				"pkg__StateConfiguration__c": {APIName: "pkg__StateConfiguration__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__StateConfiguration__mdt"}, RelationshipName: "pkg__StateConfiguration__r", ChildRelationshipName: "pkg__StateTransitions__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "pkg__StateConfiguration__c",
				ParentObjects:      []string{"pkg__StateConfiguration__mdt"},
				ParentRelationship: "pkg__StateConfiguration__r",
				ChildRelationship:  "pkg__StateTransitions__r",
			}},
			Metadata: map[string]string{"kind": "customMetadata"},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["StateTransition__mdt"] = storage.ObjectState{
		Definition: localDefinition,
		Records: map[storage.ID]storage.Record{
			"a0l000000000002": {
				ID:     "a0l000000000002",
				Object: "StateTransition__mdt",
				Fields: map[string]storage.Value{
					"QualifiedApiName":      storage.StringValue("order_submit_as_proforma"),
					"IsActive__c":           storage.BooleanValue(true),
					"FromStates__c":         storage.StringValue("Cart"),
					"StateConfiguration__c": storage.IDValue("a0j000000000001"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT QualifiedApiName, (SELECT QualifiedApiName, FromStates__c FROM StateTransitions__r WHERE IsActive__c = TRUE) FROM StateConfiguration__mdt")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["StateTransitions__r"]
	if len(children) != 1 || children[0].Fields["QualifiedApiName"].String != "order_submit_as_proforma" {
		t.Fatalf("children = %#v", result.Records[0].Children)
	}
}

func TestExecuteDerivedStandardChildRelationshipSubquery(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureStandardObject(&org, "Task")
	account := org.Objects["Account"]
	account.Records["001000000000001"] = storage.Record{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}}
	org.Objects["Account"] = account
	contact := org.Objects["Contact"]
	contact.Records["003000000000001"] = storage.Record{ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Smith"), "AccountId": storage.IDValue("001000000000001")}}
	org.Objects["Contact"] = contact
	task := org.Objects["Task"]
	task.Records["00T000000000001"] = storage.Record{ID: "00T000000000001", Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("Call"), "WhatId": storage.IDValue("001000000000001")}}
	org.Objects["Task"] = task

	result, err := ParseAndExecute(org, "SELECT Id, (SELECT Id, LastName FROM Contacts), (SELECT Id, Subject FROM Tasks) FROM Account")
	if err != nil {
		t.Fatal(err)
	}
	children := result.Records[0].Children["Contacts"]
	if len(children) != 1 || children[0].Fields["LastName"].String != "Smith" {
		t.Fatalf("children = %#v", children)
	}
	tasks := result.Records[0].Children["Tasks"]
	if len(tasks) != 1 || tasks[0].Fields["Subject"].String != "Call" {
		t.Fatalf("tasks = %#v", tasks)
	}

	child := org.Objects["Account"]
	child.Records["001000000000002"] = storage.Record{ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Sub"), "ParentId": storage.IDValue("001000000000001")}}
	org.Objects["Account"] = child
	result, err = ParseAndExecute(org, "SELECT Id, (SELECT Id, Name FROM ChildAccounts) FROM Account WHERE Id = '001000000000001'")
	if err != nil {
		t.Fatal(err)
	}
	childAccounts := result.Records[0].Children["ChildAccounts"]
	if len(childAccounts) != 1 || childAccounts[0].Fields["Name"].String != "Sub" {
		t.Fatalf("child accounts = %#v", childAccounts)
	}
}

func TestExecuteStandardIsDeletedWhereWithoutExplicitFieldDefinition(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Custom__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Custom__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Custom__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Live")}},
			"a00000000000002": {ID: "a00000000000002", Object: "Custom__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Deleted")}, System: storage.SystemFields{IsDeleted: true}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, IsDeleted FROM Custom__c WHERE IsDeleted = false")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["IsDeleted"].Boolean {
		t.Fatalf("live rows = %#v", result)
	}
	result, err = ParseAndExecute(org, "SELECT Id, IsDeleted FROM Custom__c WHERE IsDeleted = true ALL ROWS")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || !result.Records[0].Fields["IsDeleted"].Boolean {
		t.Fatalf("deleted rows = %#v", result)
	}
}

func TestExecuteSystemFieldsAreCaseInsensitive(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Acme"),
				},
				System: storage.SystemFields{
					CreatedDate: "2026-05-01T12:00:00Z",
					OwnerID:     "005000000000001",
					IsDeleted:   true,
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT createddate, OWNERID, isdeleted FROM Account WHERE isdeleted = true ORDER BY createddate ALL ROWS")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %#v", result)
	}
	fields := result.Records[0].Fields
	if fields["CreatedDate"].String != "2026-05-01T12:00:00Z" || fields["OwnerId"].ID != "005000000000001" || !fields["IsDeleted"].Boolean {
		t.Fatalf("system fields = %#v", fields)
	}
}

func TestExecuteChildRelationshipSubqueryErrors(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account"},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account"},
		},
	}

	_, err := ParseAndExecute(org, "SELECT Id, (SELECT Id FROM Contacts) FROM Account")
	if err == nil || !strings.Contains(err.Error(), "unknown child relationship Contacts") {
		t.Fatalf("child relationship error = %v", err)
	}
}

func TestParseUnsupportedReturnsError(t *testing.T) {
	if _, err := Parse("SELECT Name FROM Account WHERE Name ILIKE 'A%'"); err == nil {
		t.Fatal("expected unsupported operator error")
	}
}

func TestExecuteSemiJoinAndAntiJoinPredicates(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Beta")}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Smith"), "AccountId": storage.IDValue("001000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE LastName = 'Smith')")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000001" {
		t.Fatalf("semi join result = %#v", result)
	}
	result, err = ParseAndExecute(org, "SELECT Id, Name FROM Account WHERE Id NOT IN (SELECT AccountId FROM Contact WHERE LastName = 'Smith')")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000002" {
		t.Fatalf("anti join result = %#v", result)
	}
	result, err = ParseAndExecute(org, "SELECT Id FROM Contact WHERE AccountId IN (SELECT Id FROM Account WHERE Name = 'Acme')")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "003000000000001" {
		t.Fatalf("reverse semi join result = %#v", result)
	}
}

func TestExecuteRepeatedSemiJoinDoesNotReuseResolvedValues(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}

	query := "SELECT Id, Name FROM Account WHERE Name = 'Acme' AND Id IN (SELECT AccountId FROM Contact WHERE LastName = 'Smith')"
	result, err := ParseAndExecute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 0 {
		t.Fatalf("initial semi join result = %#v", result)
	}

	contact := org.Objects["Contact"]
	contact.Records["003000000000001"] = storage.Record{
		ID:     "003000000000001",
		Object: "Contact",
		Fields: map[string]storage.Value{
			"LastName":  storage.StringValue("Smith"),
			"AccountId": storage.IDValue("001000000000001"),
		},
	}
	org.Objects["Contact"] = contact

	result, err = ParseAndExecute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000001" {
		t.Fatalf("repeated semi join result = %#v", result)
	}
}

func TestExecuteSemiJoinSubqueryErrors(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
		},
	}

	if _, err := ParseAndExecute(org, "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Missing__c)"); err == nil || !strings.Contains(err.Error(), "unknown object Missing__c") {
		t.Fatalf("missing object error = %v", err)
	}
	if _, err := ParseAndExecute(org, "SELECT Id FROM Account WHERE Id IN (SELECT COUNT() FROM Account)"); err == nil || !strings.Contains(err.Error(), "semi-join subquery must select exactly one field") {
		t.Fatalf("aggregate subquery error = %v", err)
	}
}

func TestExecuteComplexPredicates(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":          storage.StringValue("Acme"),
					"Rating":        storage.StringValue("Hot"),
					"AnnualRevenue": storage.IntegerValue(100),
				},
			},
			"001000000000002": {
				ID:     "001000000000002",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":          storage.StringValue("Beta"),
					"Rating":        storage.StringValue("Warm"),
					"AnnualRevenue": storage.IntegerValue(200),
				},
			},
			"001000000000003": {
				ID:     "001000000000003",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":          storage.StringValue("Gamma"),
					"Rating":        storage.StringValue("Cold"),
					"AnnualRevenue": storage.IntegerValue(300),
				},
			},
			"001000000000004": {
				ID:     "001000000000004",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":          storage.StringValue("Delta"),
					"Rating":        storage.StringValue("Cold"),
					"AnnualRevenue": storage.DecimalValue("20"),
				},
			},
		},
	}

	cases := []struct {
		query string
		want  []string
	}{
		{"SELECT Id FROM Account WHERE Name IN ('Acme', 'Beta')", []string{"001000000000001", "001000000000002"}},
		{"SELECT Id FROM Account WHERE Name IN 'Acme'", []string{"001000000000001"}},
		{"SELECT Id FROM Account WHERE Name NOT IN ('Acme', 'Beta')", []string{"001000000000003", "001000000000004"}},
		{"SELECT Id FROM Account WHERE Name LIKE 'A%'", []string{"001000000000001"}},
		{"SELECT Id FROM Account WHERE Name LIKE ('A%')", []string{"001000000000001"}},
		{"SELECT Id FROM Account WHERE Name LIKE ('A%', 'B%')", []string{"001000000000001", "001000000000002"}},
		{"SELECT Id FROM Account WHERE Name LIKE 'a%'", []string{"001000000000001"}},
		{"SELECT Id FROM Account WHERE Name LIKE '%a%'", []string{"001000000000001", "001000000000002", "001000000000003", "001000000000004"}},
		{"SELECT Id FROM Account WHERE Name LIKE '_CME'", []string{"001000000000001"}},
		{"SELECT Id FROM Account WHERE AnnualRevenue > 150", []string{"001000000000002", "001000000000003"}},
		{"SELECT Id FROM Account WHERE AnnualRevenue >= 200", []string{"001000000000002", "001000000000003"}},
		{"SELECT Id FROM Account WHERE AnnualRevenue < 200", []string{"001000000000001", "001000000000004"}},
		{"SELECT Id FROM Account WHERE AnnualRevenue <= 100", []string{"001000000000001", "001000000000004"}},
		{"SELECT Id FROM Account WHERE Name = 'Acme' AND Rating = 'Hot'", []string{"001000000000001"}},
		{"SELECT Id FROM Account WHERE Name = 'Acme' OR Name = 'Gamma'", []string{"001000000000001", "001000000000003"}},
		{"SELECT Id FROM Account WHERE Name = 'Acme' AND Rating = 'Hot' OR Rating = 'Cold'", []string{"001000000000001", "001000000000003", "001000000000004"}},
		{"SELECT Id FROM Account WHERE (Name = 'Acme' OR Name = 'Beta') AND Rating = 'Warm'", []string{"001000000000002"}},
		{"SELECT Id FROM Account WHERE NOT Name = 'Acme'", []string{"001000000000002", "001000000000003", "001000000000004"}},
		{"SELECT Id FROM Account WHERE Name NOT LIKE 'a%'", []string{"001000000000002", "001000000000003", "001000000000004"}},
		{"SELECT Id FROM Account WHERE AnnualRevenue = 20", []string{"001000000000004"}},
	}

	for _, tc := range cases {
		result, err := ParseAndExecute(org, tc.query)
		if err != nil {
			t.Fatalf("query %q: %v", tc.query, err)
		}
		got := make([]string, 0, len(result.Records))
		for _, r := range result.Records {
			got = append(got, string(r.ID))
		}
		sort.Strings(got)
		want := append([]string(nil), tc.want...)
		sort.Strings(want)
		if len(got) != len(want) {
			t.Fatalf("query %q: got %v, want %v", tc.query, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("query %q: got %v, want %v", tc.query, got, want)
			}
		}
	}
}

func TestExecuteNamespacedCustomFieldPredicate(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Thing__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Thing__c",
			Fields: map[string]storage.Field{
				"Name__c": {APIName: "Name__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Thing__c",
				Fields: map[string]storage.Value{
					"Name__c": storage.StringValue("Changed"),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id, pkg__Name__c FROM pkg__Thing__c WHERE pkg__Name__c = 'Changed'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d, result = %#v", result.Rows, result)
	}
	if got, ok := result.Records[0].Fields["Name__c"]; !ok || got.Kind != storage.ValueString || got.String != "Changed" {
		t.Fatalf("projected namespaced field = %#v ok=%v", got, ok)
	}
}

func TestExecuteNamespacedCustomFieldPredicateWithUnqualifiedCondition(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Webhook_Event__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Webhook_Event__c",
			Fields: map[string]storage.Field{
				"Status__c":                 {APIName: "Status__c", Type: storage.FieldPicklist},
				"NextProcessingDateTime__c": {APIName: "NextProcessingDateTime__c", Type: storage.FieldDateTime},
				"Priority__c":               {APIName: "Priority__c", Type: storage.FieldInteger},
				"TriggeredAt__c":            {APIName: "TriggeredAt__c", Type: storage.FieldDateTime},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Webhook_Event__c",
				Fields: map[string]storage.Value{
					"Status__c": storage.StringValue("Pending"),
				},
			},
		},
	}

	result, err := ParseAndExecuteAt(org, "SELECT Id, pkg__Status__c FROM pkg__Webhook_Event__c WHERE Status__c In('Pending', 'Retry') AND (NextProcessingDateTime__c = null OR NextProcessingDateTime__c <2026-05-02T12:00:00Z) ORDER BY pkg__Priority__c ASC NULLS LAST, pkg__TriggeredAt__c ASC NULLS LAST, pkg__NextProcessingDateTime__c ASC NULLS LAST LIMIT 200", time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d, result = %#v", result.Rows, result)
	}
	if got, ok := result.Records[0].Fields["Status__c"]; !ok || got.Kind != storage.ValueString || got.String != "Pending" {
		t.Fatalf("projected status = %#v ok=%v", got, ok)
	}
}

func TestExecuteNamespacedCustomMetadataAPINamePredicate(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Config__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Config__mdt",
			Fields: map[string]storage.Field{
				"ObjectName__c": {APIName: "ObjectName__c", Type: storage.FieldString},
				"Active__c":     {APIName: "Active__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Config__mdt",
				Fields: map[string]storage.Value{
					"ObjectName__c": storage.StringValue("pkg__Widget__c"),
					"Active__c":     storage.BooleanValue(true),
				},
			},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id FROM Config__mdt WHERE Active__c = TRUE AND ObjectName__c = 'Widget__c'")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("rows = %d, result = %#v", result.Rows, result)
	}
}

func assertStorageInt(t *testing.T, value storage.Value, want int64) {
	t.Helper()
	if value.Kind != storage.ValueInteger || value.Integer != want {
		t.Fatalf("integer value = %#v, want %d", value, want)
	}
}

func assertStorageDecimal(t *testing.T, value storage.Value, want string) {
	t.Helper()
	if value.Kind != storage.ValueDecimal || value.Decimal != want {
		t.Fatalf("decimal value = %#v, want %s", value, want)
	}
}
