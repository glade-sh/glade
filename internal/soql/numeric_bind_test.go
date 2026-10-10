package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// SF185 accepted this unchanged API58 colon-numeric query.
func TestParseNumericLiteralBind(t *testing.T) {
	q, err := Parse("SELECT Id FROM Account WHERE NumberOfEmployees = :1")
	if err != nil {
		t.Fatal(err)
	}
	if q.Where == nil || q.Where.Value.Kind != storage.ValueID || string(q.Where.Value.ID) != ":1" {
		t.Fatalf("bind = %#v", q.Where)
	}
}

func TestNumericBindDoesNotConsumeDateLiteralSuffix(t *testing.T) {
	q, err := Parse("SELECT Id FROM Account WHERE CreatedDate = N_DAYS_AGO : 1")
	if err != nil {
		t.Fatal(err)
	}
	if q.Where == nil || !q.Where.Range {
		t.Fatalf("date range = %#v", q.Where)
	}
}

func TestParseCronTriggerIDWithNumericExponentPrefix(t *testing.T) {
	q, err := Parse("SELECT Id FROM CronTrigger WHERE Id = 08e000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if q.Where == nil || q.Where.Value.Kind != storage.ValueID || string(q.Where.Value.ID) != "08e000000000001" {
		t.Fatalf("CronTrigger Id literal = %#v, want ValueID 08e000000000001", q.Where)
	}

	decimal, err := Parse("SELECT Id FROM Account WHERE AnnualRevenue = 1e3")
	if err != nil {
		t.Fatal(err)
	}
	if decimal.Where == nil || decimal.Where.Value.Kind != storage.ValueDecimal || decimal.Where.Value.Decimal != "1e3" {
		t.Fatalf("scientific decimal literal = %#v, want ValueDecimal 1e3", decimal.Where)
	}
}
