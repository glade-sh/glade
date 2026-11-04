package dml

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase349BusinessRecordTypeNativeRows(t *testing.T) {
	f, err := os.Open("../../testdata/conformance/case349_review/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows[1:] {
		if row[1] != "anonymous" || row[3] != "recordtype" {
			continue
		}
		var native struct {
			Name   string `json:"name"`
			Person bool   `json:"personType"`
		}
		if err := json.Unmarshal([]byte(row[4]), &native); err != nil {
			t.Fatal(err)
		}
		definition := storage.ObjectDefinition{RecordTypes: []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: native.Name, PersonAccount: native.Person}}}
		record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001")}}
		if got := AccountHasExplicitBusinessRecordType("Account", definition, record); got != !native.Person {
			t.Errorf("API %s %s: business=%v, native person=%v", row[0], native.Name, got, native.Person)
		}
		count++
	}
	if count != 4 {
		t.Fatalf("business native rows: got %d, want 4", count)
	}
}
