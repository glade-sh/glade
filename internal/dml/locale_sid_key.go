package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// validateCapturedUserLocale applies only rejections observed in
// locale-boundary/org.tsv:2-5 and locale_sid_key/org.tsv:2-3. Describe omits
// valid aliases, so neither the
// describe list nor the sampled successes is an exhaustive DML allowlist.
func validateCapturedUserLocale(objectName, fieldName string, field storage.Field, value storage.Value) (bool, error) {
	if !field.RestrictedPicklistFromCatalog || !strings.EqualFold(objectName, "User") || !strings.EqualFold(fieldName, "LocaleSidKey") {
		return false, nil
	}
	if value.Kind != storage.ValueString {
		return true, nil
	}
	switch value.String {
	case "NotAnAllowedValue", "az", "be", "dz", "en", "en-GB", "en_FOO", "en_XY", "ky", "lo", "lu", "ne", "ps", "rn", "so", "tg", "ti", "uz", "xx", "xx_XX", "yo", "zz", "zz_US":
		return true, labeledRestrictedPicklistValueError(fieldName, field, value.String)
	default:
		return true, nil
	}
}
