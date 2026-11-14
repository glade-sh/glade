package dml

import (
	"encoding/csv"
	"os"
	"reflect"
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
		got := validateInsertEmailField(strings.Split(row[4], ".")[0], field, storage.Field{APIName: field, Label: label, DisplayType: "EMAIL"}, storage.StringValue(row[3]))
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

// These uncaptured shapes retain the origin/main result, including its message.
func TestEmailInsertValidationUncapturedNeighborsMatchBase(t *testing.T) {
	field := storage.Field{APIName: "Email", Label: "Email", DisplayType: "EMAIL"}
	for _, text := range []string{"user@例え.テスト.", "a@@example.com.", "a@sub.example.com.", "a@example1.com.", `""quoted local""@example.com`, "not-an-address", "x@y.example"} {
		value := storage.StringValue(text)
		var base error
		if InvalidEmailValue(value) {
			base = dmlErrorf("INVALID_EMAIL_ADDRESS", []string{"Email"}, "dml: invalid email address for field %s.%s", "Contact", "Email")
		}
		got := validateInsertEmailField("Contact", "Email", field, value)
		if got == nil && base != nil {
			got = validateEmailField("Contact", "Email", field, value)
		}
		if !reflect.DeepEqual(got, base) {
			t.Errorf("%q: got %#v, base %#v", text, got, base)
		}
	}
}

func TestEmailInsertValidationLeavesUpdatesUnchanged(t *testing.T) {
	field := storage.Field{APIName: "Email", Label: "Email", Type: storage.FieldString, DisplayType: "EMAIL"}
	definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"Email": field}}
	existing := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"Email": storage.StringValue("old@example.com")}}
	for _, address := range []string{"a@example.com.", "a@example..com", "\"quoted local\"@example.com"} {
		record := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"Email": storage.StringValue(address)}}
		if err := validateFields(definition, "", record, existing); err != nil {
			t.Errorf("update %q: baseline accepted, got %v", address, err)
		}
	}
}

func TestEmailInsertValidationLeavesUpsertsUnchanged(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	engine := NewEngine(&org)
	for _, address := range []string{"a@example.com.", "a@example..com", "\"quoted local\"@example.com"} {
		got := engine.Upsert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Probe"), "Email": storage.StringValue(address)}}})
		if len(got) != 1 || !got[0].Success {
			t.Errorf("upsert %q: baseline accepted, got %+v", address, got)
		}
	}
}

func TestEmailInsertValidationLeavesTextFieldUnchanged(t *testing.T) {
	field := storage.Field{APIName: "Email", Type: storage.FieldString, DisplayType: "TEXT"}
	if err := validateInsertEmailField("Custom__c", "Email", field, storage.StringValue("a@example.com.")); err != nil {
		t.Fatalf("uncaptured text field: baseline accepted, got %v", err)
	}
}

func TestEmailInsertValidationLeavesNormalizedNeighborsUnchanged(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	engine := NewEngine(&org)
	for _, address := range []string{" a@example.com. ", "A@example.com.", "a@EXAMPLE..com"} {
		got := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Probe"), "Email": storage.StringValue(address)}}})
		if len(got) != 1 || !got[0].Success {
			t.Errorf("normalized neighbor %q: baseline accepted, got %+v", address, got)
		}
	}
}
