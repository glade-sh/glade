package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
)

// The wrong-String literal observation is an equals-only user element class.
// Other class signatures keep their existing diagnostic projection.
func capturedEqualsOnlySetElement(element typeMembers) bool {
	if element.platform || element.dependency || element.kind != apexast.DeclarationClass ||
		(element.superClass != "" && !strings.EqualFold(element.superClass, "Object")) || len(element.methods[normalizeName("hashCode")]) != 0 {
		return false
	}
	equals := element.methods[normalizeName("equals")]
	return len(equals) == 1 && !hasModifier(equals[0].Modifiers, "static") &&
		strings.EqualFold(equals[0].Type, "Boolean") && len(equals[0].Parameters) == 1 && strings.EqualFold(equals[0].Parameters[0].Type, "Object")
}
