package dml

import (
	"encoding/csv"
	"github.com/glade-sh/glade/internal/storage"
	"os"
	"testing"
	"time"
)

func TestR6FlowCapturedAssignmentActiveState(t *testing.T) {
	f, err := os.Open("testdata/r6flow_assignment_active/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			if row[3] == "SETUP" {
				if row[0] != "67.0" || row[1] != "isTest" || row[2] != "revoked" {
					t.Fatal("unexpected setup result")
				}
				return
			}
			fields := map[string]storage.Value{"IsActive": storage.BooleanValue(row[3] == "true"), "IsRevoked": storage.BooleanValue(row[4] == "true")}
			if row[2] == "future" {
				fields["ExpirationDate"] = storage.DateTimeValue("2030-01-04T12:00:00Z")
			}
			if row[2] == "past" {
				fields["ExpirationDate"] = storage.DateTimeValue("2029-12-31T12:00:00Z")
			}
			active, known := PermissionAssignmentState(storage.Record{Fields: fields}, func() time.Time { return now })
			if !known || active != (row[3] == "true") {
				t.Fatalf("active=%v known=%v native=%s", active, known, row[3])
			}
		})
	}
}
