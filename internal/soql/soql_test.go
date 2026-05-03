package soql

import (
	"sort"
	"strings"
	"testing"
	"time"

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
