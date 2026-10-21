package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"strings"
	"testing"
)

// Salesforce Object Reference: EmailMessage.Name is derived from the first
// 255 characters of Subject and remains readonly. Empty localized names are excluded.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/object_reference.pdf
func TestEmailMessageNameDerivesFromSubject(t *testing.T) {
	o := testOrg()
	storage.EnsureStandardObject(&o, "EmailMessage")
	e := NewEngine(&o)
	for _, subject := range []string{"Synthetic subject", strings.Repeat("x", 300)} {
		r := e.Insert([]storage.Record{{Object: "EmailMessage", Fields: map[string]storage.Value{"Subject": storage.StringValue(subject), "TextBody": storage.StringValue("body"), "Status": storage.StringValue("3")}}})
		if !r[0].Success {
			t.Fatal(r)
		}
		saved := o.Objects["EmailMessage"].Records[r[0].ID]
		expected := subject
		if len(expected) > 255 {
			expected = expected[:255]
		}
		if saved.Fields["Name"].String != expected {
			t.Fatalf("derived name = %q", saved.Fields["Name"].String)
		}
		u := e.Update([]storage.Record{{Object: "EmailMessage", ID: r[0].ID, Fields: map[string]storage.Value{"Subject": storage.StringValue("Changed subject")}}})
		if !u[0].Success {
			t.Fatal(u)
		}
		if got := o.Objects["EmailMessage"].Records[r[0].ID].Fields["Name"].String; got != "Changed subject" {
			t.Fatal(got)
		}
		blocked := e.Update([]storage.Record{{Object: "EmailMessage", ID: r[0].ID, Fields: map[string]storage.Value{"Name": storage.StringValue("caller override")}}})
		if blocked[0].Success {
			t.Fatal("derived Name became writable")
		}
	}
}
