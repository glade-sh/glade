package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The captured rows are anonymous-local declarations. They are physically
// top-level parser inputs, so apply the captured static priority only once
// the anonymous context is known. Keep every original reserved diagnostic.
func anonymousReservedStaticPriority(index typesys.Index, diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	out := append([]diagnostic.Diagnostic(nil), diagnostics...)
	for _, typ := range index.Types {
		if !hasModifier(typ.Modifiers, vm.AnonymousClassModifier) || typ.NestingDepth != 0 || !strings.EqualFold(typ.LocalName, "Inner") ||
			len(typ.Members) != 1 || typ.SuperClass != "" || len(typ.Interfaces) != 0 || len(typ.TypeParameters) != 0 || len(typ.Annotations) != 0 {
			continue
		}
		member := typ.Members[0]
		message := ""
		if member.Type == "Integer" && len(member.Parameters) == 0 && len(member.Modifiers) == 2 && len(member.Annotations) == 0 && hasModifier(member.Modifiers, "public") && hasModifier(member.Modifiers, "static") {
			switch member.Kind {
			case apexast.DeclarationMethod:
				message = "static can only be used on methods of a top level type"
			case apexast.DeclarationField:
				message = "static can only be used on fields of a top level type"
			}
		}
		if message == "" {
			continue
		}
		for i := range out {
			d := &out[i]
			if d.Severity == diagnostic.Error && d.Code == "APEXPARSE002" && d.File == typ.File && strings.EqualFold(d.Message, "Identifier name is reserved: Inner") && d.Range != nil &&
				d.Range.Start.Offset >= typ.Range.Start.Offset && d.Range.Start.Offset < member.Range.Start.Offset {
				d.NativeMessage = message
			}
		}
	}
	return out
}
