package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// The static field is absent at source API 62 and present from 63 onward.
// Native observations: case289_followups ST1-ST4 and static-boundary.tsv.
func objectPermissionsFieldHidden(version, object, field string) bool {
	return apexversion.AtLeast(version, 62) && !apexversion.AtLeast(version, 63) &&
		strings.EqualFold(object, "ObjectPermissions") && strings.EqualFold(field, "PermissionsViewAllFields")
}

// An object-qualified SOQL projection names the same standard field.
func objectPermissionsQueryField(object, field string) string {
	prefix := object + "."
	if len(field) > len(prefix) && strings.EqualFold(field[:len(prefix)], prefix) {
		return field[len(prefix):]
	}
	return field
}
