package soql

import (
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sobject"
	"github.com/open-aer/oaer/internal/storage"
)

func TestParseSimpleQuery(t *testing.T) {
	query, err := Parse("SELECT Id, Name FROM Account WHERE Name = 'Acme' WITH SECURITY_ENFORCED ORDER BY Name DESC NULLS LAST LIMIT 10 OFFSET 1 FOR UPDATE")
	if err != nil {
		t.Fatal(err)
	}
	if query.Object != "Account" || len(query.Fields) != 2 || query.Where.Field != "Name" || query.SecurityMode != "SECURITY_ENFORCED" || query.OrderBy != "Name" || !query.OrderDesc || len(query.Order) != 1 || query.Order[0].Nulls != "LAST" || query.Limit != 10 || query.Offset != 1 || !query.ForUpdate {
		t.Fatalf("query = %#v", query)
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
	if err := storage.ApplyCustomMetadataRecords(&org, sch.CustomMetadataRecords); err != nil {
		t.Fatal(err)
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
	value := result.Records[0].Fields["Parent__r.Name__c"]
	if value.Kind != storage.ValueString || value.String != "ParentValue" {
		t.Fatalf("relationship projection = %#v", value)
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
	org.Objects["Account"] = account
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)

	result, err := ParseAndExecuteAt(org, "SELECT Id, Name FROM Account WHERE RenewalDate__c = LAST_N_MONTHS:2 ORDER BY Name", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Records[0].Fields["Name"].String != "March" || result.Records[1].Fields["Name"].String != "Quarter" {
		t.Fatalf("LAST_N_MONTHS result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id, Name FROM Account WHERE RenewalDate__c = THIS_QUARTER ORDER BY Name", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Records[0].Fields["Name"].String != "Quarter" || result.Records[1].Fields["Name"].String != "Yesterday" {
		t.Fatalf("THIS_QUARTER result = %#v", result)
	}
	result, err = ParseAndExecuteAt(org, "SELECT Id, Name FROM Account WHERE RenewalDate__c = N_DAYS_AGO:1", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].Fields["Name"].String != "Yesterday" {
		t.Fatalf("N_DAYS_AGO result = %#v", result)
	}
	_, err = ParseAndExecuteAt(org, "SELECT Id FROM Account WHERE RenewalDate__c = THIS_WEEK", now)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Message != "soql: date literal THIS_WEEK is not supported" {
		t.Fatalf("unsupported date literal err = %#v", err)
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

	locked := org.Objects["Account"].Records["001000000000001"]
	locked.System.Locked = true
	org.Objects["Account"].Records["001000000000001"] = locked
	_, err = ParseAndExecute(org, "SELECT Id FROM Account WHERE Id = '001000000000001' FOR UPDATE")
	if err == nil || !strings.Contains(err.Error(), "unable to lock row 001000000000001") {
		t.Fatalf("expected lock error, got %v", err)
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
	task.Records["00T000000000001"] = storage.Record{ID: "00T000000000001", Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("Call"), "AccountId": storage.IDValue("001000000000001")}}
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
		{"SELECT Id FROM Account WHERE Name NOT IN ('Acme', 'Beta')", []string{"001000000000003", "001000000000004"}},
		{"SELECT Id FROM Account WHERE Name LIKE 'A%'", []string{"001000000000001"}},
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
