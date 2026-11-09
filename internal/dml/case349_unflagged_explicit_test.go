package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native business rows in testdata/conformance/case349_review/org.tsv:5-8
// show that names containing Person or Individual do not imply person family.
// A missing family flag is treated as business; a true flag is the twin.
func TestCase349UnflaggedRecordTypeUsesBusinessFamily(t *testing.T) {
	for _, name := range []string{"Person_Commerce", "Individual_Business", "Care Person"} {
		t.Run(name, func(t *testing.T) {
			info := storage.RecordTypeInfo{ID: "012000000000101", DeveloperName: name, Name: name, Active: true, Available: true}
			if recordTypeLooksPersonAccount(info) {
				t.Fatalf("unflagged %q classified as person", name)
			}
			got, ok := businessAccountRecordType([]storage.RecordTypeInfo{info})
			if !ok || got.ID != info.ID {
				t.Fatalf("business selection = %#v, %v", got, ok)
			}
			info.PersonAccount = true
			if !recordTypeLooksPersonAccount(info) {
				t.Fatalf("flagged %q not person", name)
			}
		})
	}
}
