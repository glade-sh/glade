package dml

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestEmailInsertValidationNativeConformance(t *testing.T) {
	f, err := os.Open("testdata/email_insert_validation/org.tsv")
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
	for _, row := range rows[1:] {
		// Length validation is exercised by the engine, not this syntax helper.
		if strings.HasPrefix(row[6], "STRING_TOO_LONG~") {
			continue
		}
		field := strings.TrimPrefix(row[4], "Contact.")
		field = strings.TrimPrefix(field, "Lead.")
		label := "Email"
		if field == "ProbeEmail__c" {
			label = "Probe Email"
		}
		got := validateEmailField(strings.Split(row[4], ".")[0], field, storage.Field{APIName: field, Label: label, DisplayType: "EMAIL"}, storage.StringValue(row[3]))
		if row[6] == "OK" {
			if got != nil {
				t.Errorf("%q %s: got %v, native accepted", row[3], row[4], got)
			}
			continue
		}
		want := strings.SplitN(row[6], "~", 3)
		if len(want) != 3 {
			t.Fatalf("bad native row %q", row[6])
		}
		e, ok := got.(dmlError)
		if !ok || e.code != want[0] || e.message != want[1] || len(e.fields) != 1 || e.fields[0] != field {
			t.Errorf("%q %s: got %#v, native %q", row[3], row[4], got, row[6])
		}
	}
}
