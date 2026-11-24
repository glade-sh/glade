package soql

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestLocaleOrder466NativeRows(t *testing.T) {
	values := []string{"", "a", "A", "ä", "å", "æ", "ö", "ø", "z", "Z", "é", "e", "É", "œ", "ß", "ss", "中", "α", "Я", "2", "10", "-", "_", "!", "ñ", "n", "á", "à", "Å", "Ä", "Æ", "Ö", "Ø", "あ", "ア"}
	org := storage.NewOrgState()
	state := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
		"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString}, "AccountNumber": {APIName: "AccountNumber", Type: storage.FieldString}, "BillingCity": {APIName: "BillingCity", Type: storage.FieldString},
	}}, Records: map[storage.ID]storage.Record{}}
	for i, value := range values {
		id := storage.ID(fmt.Sprintf("001%012d", i+1))
		fields := map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("capture %d", i)), "AccountNumber": storage.StringValue(fmt.Sprintf("%03d", i))}
		if i != 0 {
			fields["BillingCity"] = storage.StringValue(value)
		}
		state.Records[id] = storage.Record{ID: id, Object: "Account", Fields: fields}
	}
	org.Objects["Account"] = state
	raw, err := os.ReadFile("testdata/conformance/locale_order_466/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 13 {
		t.Fatalf("got %d rows, want 12", len(rows)-1)
	}
	for _, line := range rows[1:] {
		cols := strings.Split(line, "\t")
		if len(cols) != 3 {
			t.Fatal("invalid row")
		}
		t.Run(cols[0]+"/"+cols[1], func(t *testing.T) {
			q, err := Parse("SELECT AccountNumber FROM Account WHERE Name LIKE 'capture %' ORDER BY BillingCity ASC NULLS FIRST, AccountNumber ASC")
			if err != nil {
				t.Fatal(err)
			}
			result, err := ExecuteWithCacheAndContext(org, q, nil, ExecutionContext{Locale: cols[0], APIVersion: cols[1]})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, rec := range result.Records {
				got = append(got, rec.Fields["AccountNumber"].String)
			}
			if strings.Join(got, ",") != cols[2] {
				t.Fatalf("got %s\nwant %s", strings.Join(got, ","), cols[2])
			}
		})
	}
}
