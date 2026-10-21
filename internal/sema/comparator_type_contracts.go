package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/typesys"
)

// A qualified Datetime argument in an inner platform Comparator declaration
// is rejected independently of ordinary qualified type references.
func semaComparatorQualifiedDatetimeRejected(typ typesys.TypeSymbol, model *semaTypeMemberView) bool {
	if typ.Kind != apexast.DeclarationClass || typ.NestingDepth != 1 || typ.Namespace != "" ||
		!apexversion.AtLeast(typ.EffectiveAPIVersion, 62) || apexversion.AtLeast(typ.EffectiveAPIVersion, 69) {
		return false
	}
	for _, iface := range typ.Interfaces {
		base, args := semaGenericBaseAndArgs(iface)
		if !strings.EqualFold(base, "System.Comparator") || len(args) != 1 || !strings.EqualFold(args[0], "System.Datetime") {
			continue
		}
		members, _, known := semaLookupTypeMembers(model, base)
		if known && members.platform && members.kind == apexast.DeclarationInterface {
			return true
		}
	}
	return false
}
