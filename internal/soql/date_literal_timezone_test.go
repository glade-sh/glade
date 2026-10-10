package soql

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecuteDateLiteralUsesExecutionTimezoneForDatetimeFields(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["When__c"] = storage.Field{APIName: "When__c", Type: storage.FieldDateTime}
	account.Records["001000000000001"].Fields["When__c"] = storage.DateTimeValue("2026-05-02T02:00:00Z")
	account.Records["001000000000002"].Fields["When__c"] = storage.DateTimeValue("2026-05-01T13:30:00Z")
	account.Records["001000000000003"].Fields["When__c"] = storage.DateTimeValue("2026-05-02T14:01:00Z")
	org.Objects["Account"] = account

	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		operator string
		want     int
	}{
		{"=", 1},
		{">", 1},
		{">=", 2},
		{"<", 1},
		{"<=", 2},
	} {
		query, err := ParseAtWithFiscalYearStartMonthAndTimeZone(
			"SELECT Id FROM Account WHERE When__c "+tc.operator+" TODAY",
			now,
			1,
			"Australia/Sydney",
		)
		if err != nil {
			t.Fatalf("parse %s: %v", tc.operator, err)
		}
		result, err := Execute(org, query)
		if err != nil {
			t.Fatalf("execute %s: %v", tc.operator, err)
		}
		if result.Rows != tc.want {
			t.Fatalf("%s TODAY rows=%d, want %d", tc.operator, result.Rows, tc.want)
		}
	}
}

func TestDateLiteralUsesLocalCalendarDateAcrossUTCBoundary(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["When__c"] = storage.Field{APIName: "When__c", Type: storage.FieldDateTime}
	account.Records["001000000000001"].Fields["When__c"] = storage.DateTimeValue("2026-05-01T20:00:00Z")
	account.Records["001000000000002"].Fields["When__c"] = storage.DateTimeValue("2026-05-02T08:00:00Z")
	org.Objects["Account"] = account

	// At 01:00Z, the execution user in Los Angeles is still on May 1.
	now := time.Date(2026, 5, 2, 1, 0, 0, 0, time.UTC)
	query, err := ParseAtWithFiscalYearStartMonthAndTimeZone(
		"SELECT Id FROM Account WHERE When__c = TODAY",
		now,
		1,
		"America/Los_Angeles",
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || result.Records[0].ID != "001000000000001" {
		t.Fatalf("local TODAY rows=%d records=%#v", result.Rows, result.Records)
	}
}
