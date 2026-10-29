package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func testSObjectHasViewDateDefaults(objectName string) bool {
	switch strings.ToLower(objectName) {
	case "account", "contact", "pricebook2":
		return true
	default:
		return false
	}
}

// Native named tests initialize these fields on newly created records, while
// anonymous execution leaves them null. Restrict this default to the captured
// object set and available Datetime fields; explicit stored values win.
func applyTestSObjectViewDateDefaults(definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil || record.System.CreatedDate == "" {
		return
	}
	for _, name := range []string{"LastViewedDate", "LastReferencedDate"} {
		canonical, field, ok := storage.ResolveFieldDefinition(definition, "", name)
		if !ok || field.Type != storage.FieldDateTime || record.HasExplicitNull(canonical) {
			continue
		}
		if _, present := record.GetField(canonical); present {
			continue
		}
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		record.Fields[canonical] = storage.DateTimeValue(record.System.CreatedDate)
	}
}
