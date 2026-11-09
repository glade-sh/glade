package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native controls: testdata/locale_sid_key/org.tsv:4 and the API 62/67
// acceptance controls in testdata/locale_sid_key/ORG-RUN.txt.
func TestNativeLocaleAPI68DMLControls(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "User"}
	storage.EnsureStandardObjectFields(&definition)
	field := definition.Fields["LocaleSidKey"]
	for _, row := range []struct {
		value string
		valid bool
	}{
		{"en_BR", true}, {"en_MX", true}, {"en_SA", true}, {"en-GB", false},
	} {
		t.Run(row.value, func(t *testing.T) {
			err := validateRestrictedPicklistField("User", "LocaleSidKey", field, storage.StringValue(row.value))
			if (err == nil) != row.valid {
				t.Fatalf("locale %s valid=%v: %v", row.value, row.valid, err)
			}
		})
	}
}
