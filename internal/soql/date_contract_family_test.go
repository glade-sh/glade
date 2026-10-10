package soql

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDateContractFamilyUsesTypedDateAndExecutionTimezone(t *testing.T) {
	org := dateContractFamilyOrg()
	ungrouped := "SELECT CALENDAR_YEAR(ObservedDate__c) yearPart FROM DateQueryFixture__c WHERE CALENDAR_YEAR(ObservedDate__c) = 2024"
	if _, err := ParseAndExecute(org, ungrouped); err == nil {
		t.Fatalf("ungrouped date-function projection was accepted; source requires matching GROUP BY expressions: %s", ungrouped)
	}

	result, err := ParseAndExecute(org, "SELECT Name, CALENDAR_YEAR(ObservedDate__c) yearPart, CALENDAR_MONTH(ObservedDate__c) monthPart, CALENDAR_QUARTER(ObservedDate__c) quarterPart, DAY_ONLY(ObservedAt__c) dayOnly, COUNT(Id) total FROM DateQueryFixture__c WHERE CALENDAR_YEAR(ObservedDate__c) = 2024 GROUP BY Name, CALENDAR_YEAR(ObservedDate__c), CALENDAR_MONTH(ObservedDate__c), CALENDAR_QUARTER(ObservedDate__c), DAY_ONLY(ObservedAt__c) ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || len(result.Records) != 2 {
		t.Fatalf("calendar/date-only query rows=%d records=%#v, want 2", result.Rows, result.Records)
	}

	leapDay := result.Records[0].Fields
	if got := leapDay["Name"].String; got != "Leap day" {
		t.Fatalf("first Name = %q, want Leap day", got)
	}
	assertStorageInt(t, leapDay["yearPart"], 2024)
	assertStorageInt(t, leapDay["monthPart"], 2)
	assertStorageInt(t, leapDay["quarterPart"], 1)
	assertStorageInt(t, leapDay["total"], 1)
	if got := leapDay["dayOnly"].String; got != "2026-05-01" {
		t.Fatalf("DAY_ONLY(ObservedAt__c) = %q, want 2026-05-01", got)
	}

	march := result.Records[1].Fields
	if got := march["Name"].String; got != "March" {
		t.Fatalf("second Name = %q, want March", got)
	}
	assertStorageInt(t, march["yearPart"], 2024)
	assertStorageInt(t, march["monthPart"], 3)
	assertStorageInt(t, march["quarterPart"], 1)
	assertStorageInt(t, march["total"], 1)

	// The fixed instant is May 1 in Los Angeles but May 2 in UTC. The combined
	// date-function and TODAY predicate distinguishes those calendar days. Add
	// a matching February record on the next local day so month filtering cannot
	// hide a timezone-boundary error.
	fixture := org.Objects["DateQueryFixture__c"]
	fixture.Records["a0D000000000004"] = storage.Record{
		ID:     "a0D000000000004",
		Object: "DateQueryFixture__c",
		Fields: map[string]storage.Value{
			"Name":            storage.StringValue("Late February"),
			"ObservedDate__c": storage.DateValue("2024-02-20"),
			"ObservedAt__c":   storage.DateTimeValue("2026-05-02T08:00:00Z"),
		},
	}
	org.Objects["DateQueryFixture__c"] = fixture
	now := time.Date(2026, 5, 2, 1, 0, 0, 0, time.UTC)
	query, err := ParseAtWithFiscalYearStartMonthAndTimeZone(
		"SELECT Name FROM DateQueryFixture__c WHERE CALENDAR_MONTH(ObservedDate__c) = 2 AND ObservedAt__c = TODAY ORDER BY Name",
		now,
		1,
		"America/Los_Angeles",
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err = Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || len(result.Records) != 1 || result.Records[0].Fields["Name"].String != "Leap day" {
		t.Fatalf("function plus execution-timezone TODAY result=%#v, want only Leap day", result)
	}
}

func TestDateFunctionGroupingUsesFieldTypeAndRejectsConstructedBypass(t *testing.T) {
	org := dateContractFamilyOrg()

	dateRawGroup := "SELECT CALENDAR_YEAR(ObservedDate__c) yearPart, COUNT(Id) total FROM DateQueryFixture__c GROUP BY ObservedDate__c"
	result, err := ParseAndExecute(org, dateRawGroup)
	if err != nil {
		t.Fatalf("Date field raw GROUP BY exception: %v", err)
	}
	if result.Rows != 3 {
		t.Fatalf("Date field raw GROUP BY rows=%d, want 3", result.Rows)
	}

	datetimeRawGroup := "SELECT CALENDAR_YEAR(ObservedAt__c) yearPart, COUNT(Id) total FROM DateQueryFixture__c GROUP BY ObservedAt__c"
	if _, err := ParseAndExecute(org, datetimeRawGroup); err == nil {
		t.Fatalf("DateTime field raw GROUP BY substituted for its date function: %s", datetimeRawGroup)
	}

	constructedMissing := Query{
		Object: "DateQueryFixture__c",
		Fields: []string{"CALENDAR_YEAR(ObservedDate__c) yearPart"},
	}
	if _, err := Execute(org, constructedMissing); err == nil {
		t.Fatal("constructed query bypassed required GROUP BY for a selected date function")
	}

	constructedPartial := Query{
		Object: "DateQueryFixture__c",
		Fields: []string{
			"CALENDAR_YEAR(ObservedDate__c) yearPart",
			"CALENDAR_MONTH(ObservedDate__c) monthPart",
		},
		GroupBy: []string{"CALENDAR_YEAR(ObservedDate__c)"},
	}
	if _, err := Execute(org, constructedPartial); err == nil {
		t.Fatal("constructed query with only one of two selected date functions grouped was accepted")
	}
}

func TestDateFunctionGroupingResolvesParentDateTypes(t *testing.T) {
	org := dateRelationshipContractOrg()
	parentDateQuery := "SELECT CALENDAR_YEAR(DateParent__r.ParentDate__c) yearPart, COUNT(Id) total FROM DateQueryFixture__c GROUP BY DateParent__r.ParentDate__c"
	parsedDateQuery, err := Parse(parentDateQuery)
	if err != nil {
		t.Fatalf("parse parent Date raw GROUP BY: %v", err)
	}
	if _, err := Execute(org, parsedDateQuery); err != nil {
		t.Fatalf("parent Date raw GROUP BY should be accepted: %v", err)
	}

	parentDateTimeQuery := "SELECT CALENDAR_YEAR(DateParent__r.ParentDateTime__c) yearPart, COUNT(Id) total FROM DateQueryFixture__c GROUP BY DateParent__r.ParentDateTime__c"
	parsedDateTimeQuery, err := Parse(parentDateTimeQuery)
	if err != nil {
		t.Fatalf("parse parent DateTime raw GROUP BY: %v", err)
	}
	if _, err := Execute(org, parsedDateTimeQuery); err == nil {
		t.Fatal("parsed query accepted raw parent DateTime GROUP BY as a date-function substitute")
	}

	constructedDateQuery := Query{
		Object:     "DateQueryFixture__c",
		Fields:     []string{"CALENDAR_YEAR(DateParent__r.ParentDate__c) yearPart", "COUNT(Id) total"},
		Aggregates: []Aggregate{{Func: "COUNT", Field: "Id", Alias: "total"}},
		GroupBy:    []string{"DateParent__r.ParentDate__c"},
	}
	if _, err := Execute(org, constructedDateQuery); err != nil {
		t.Fatalf("constructed query parent Date raw GROUP BY should be accepted: %v", err)
	}

	constructedDateTimeQuery := Query{
		Object:     "DateQueryFixture__c",
		Fields:     []string{"CALENDAR_YEAR(DateParent__r.ParentDateTime__c) yearPart", "COUNT(Id) total"},
		Aggregates: []Aggregate{{Func: "COUNT", Field: "Id", Alias: "total"}},
		GroupBy:    []string{"DateParent__r.ParentDateTime__c"},
	}
	if _, err := Execute(org, constructedDateTimeQuery); err == nil {
		t.Fatal("constructed query accepted raw parent DateTime GROUP BY as a date-function substitute")
	}
}

func dateContractFamilyOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["DateQueryFixture__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "DateQueryFixture__c",
			KeyPrefix: "a0D",
			Fields: map[string]storage.Field{
				"Name":            {APIName: "Name", Type: storage.FieldString},
				"ObservedDate__c": {APIName: "ObservedDate__c", Type: storage.FieldDate},
				"ObservedAt__c":   {APIName: "ObservedAt__c", Type: storage.FieldDateTime},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a0D000000000001": {
				ID:     "a0D000000000001",
				Object: "DateQueryFixture__c",
				Fields: map[string]storage.Value{
					"Name":            storage.StringValue("Leap day"),
					"ObservedDate__c": storage.DateValue("2024-02-29"),
					"ObservedAt__c":   storage.DateTimeValue("2026-05-01T20:00:00Z"),
				},
			},
			"a0D000000000002": {
				ID:     "a0D000000000002",
				Object: "DateQueryFixture__c",
				Fields: map[string]storage.Value{
					"Name":            storage.StringValue("March"),
					"ObservedDate__c": storage.DateValue("2024-03-01"),
					"ObservedAt__c":   storage.DateTimeValue("2026-05-02T08:00:00Z"),
				},
			},
			"a0D000000000003": {
				ID:     "a0D000000000003",
				Object: "DateQueryFixture__c",
				Fields: map[string]storage.Value{
					"Name":            storage.StringValue("Prior year"),
					"ObservedDate__c": storage.DateValue("2023-12-31"),
					"ObservedAt__c":   storage.DateTimeValue("2026-05-01T23:30:00Z"),
				},
			},
		},
	}
	return org
}

func dateRelationshipContractOrg() storage.OrgState {
	org := dateContractFamilyOrg()
	const parentObjectName = "DateParentFixture__c"
	const parentID storage.ID = "a1P000000000001"
	org.Objects[parentObjectName] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   parentObjectName,
			KeyPrefix: "a1P",
			Fields: map[string]storage.Field{
				"ParentDate__c":     {APIName: "ParentDate__c", Type: storage.FieldDate},
				"ParentDateTime__c": {APIName: "ParentDateTime__c", Type: storage.FieldDateTime},
			},
		},
		Records: map[storage.ID]storage.Record{
			parentID: {
				ID:     parentID,
				Object: parentObjectName,
				Fields: map[string]storage.Value{
					"ParentDate__c":     storage.DateValue("2024-02-29"),
					"ParentDateTime__c": storage.DateTimeValue("2024-02-29T12:00:00Z"),
				},
			},
		},
	}
	child := org.Objects["DateQueryFixture__c"]
	child.Definition.Fields["DateParent__c"] = storage.Field{
		APIName:          "DateParent__c",
		Type:             storage.FieldReference,
		ReferenceTo:      []string{parentObjectName},
		RelationshipName: "DateParent__r",
	}
	child.Definition.Relations = append(child.Definition.Relations, storage.Relationship{
		Field:              "DateParent__c",
		ParentObjects:      []string{parentObjectName},
		ParentRelationship: "DateParent__r",
	})
	for id, record := range child.Records {
		record.Fields["DateParent__c"] = storage.IDValue(parentID)
		child.Records[id] = record
	}
	org.Objects["DateQueryFixture__c"] = child
	return org
}
