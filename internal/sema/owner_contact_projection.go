package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/soql"
)

// userGroupContactProjection accepts only the one-hop contact fields whose
// USER_MODE projection is covered by owned native observations.
func (c querySemanticsChecker) userGroupContactProjection(objectName, fieldPath string) bool {
	parts := strings.Split(semaSOQLFieldReference(fieldPath), ".")
	if len(parts) == 3 && c.apiNamesMatch(parts[0], objectName) {
		parts = parts[1:]
	}
	if len(parts) != 2 {
		return false
	}
	field, _, ok := c.relationshipField(objectName, parts[0])
	if !ok || !soql.IsUserGroupContactField(field.ReferenceTo, parts[1]) {
		return false
	}
	_, ok = c.field("User", parts[1])
	return ok
}
