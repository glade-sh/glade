package vm

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// One new partial-upsert input and two stored matches retain the field label
// and both actual IDs. Contact.Email is an alternate lookup key rather than an
// external Text key. Keep other field types, cardinalities and modes on their
// existing error-text path.
func storedUpsertDuplicateResult(objectName, field string, metadata storage.Field, key storage.Value, ids []string, singlePartialInput bool) dml.Result {
	message := "External ID matched multiple records"
	textExternalKey := metadata.ExternalID && strings.EqualFold(metadata.DisplayType, "STRING")
	emailLookupKey := strings.EqualFold(objectName, "Contact") && strings.EqualFold(field, "Email") &&
		metadata.IDLookup && !metadata.ExternalID && strings.EqualFold(metadata.DisplayType, "EMAIL")
	if singlePartialInput && len(ids) == 2 && !metadata.Unique &&
		metadata.Type == storage.FieldString && (textExternalKey || emailLookupKey) &&
		metadata.Formula == "" && !metadata.AutoNumber && metadata.Label != "" &&
		key.Kind == storage.ValueString && fieldKeyASCII(key.String) {
		sort.Strings(ids)
		message = metadata.Label + ": more than one record found for external id field: [" + strings.Join(ids, ", ") + "]"
	}
	return dmlFailure("", message, "DUPLICATE_EXTERNAL_ID", []string{field})
}
