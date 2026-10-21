package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

// inheritedFieldBindings uses indexed fields and properties rather than names
// in another class's text. The containing source occurrence determines the owner;
// declared fields and locals are appended later and retain their precedence.
func (r *semaBindingResolver) inheritedFieldBindings(typeStart int) []semaScopedBinding {
	owner, ok := r.bindingTypeAt(typeStart)
	if !ok {
		return nil
	}
	var fields []semaScopedBinding
	seen := make(map[string]bool)
	var collect func(typesys.TypeSymbol)
	collect = func(typ typesys.TypeSymbol) {
		key := typ.File + "\x00" + typ.Name
		if seen[key] {
			return
		}
		seen[key] = true
		if parent, ok := r.bindingSuperType(typ); ok {
			collect(parent)
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationField && member.Kind != apexast.DeclarationProperty ||
				!hasModifier(member.Modifiers, "protected") && !hasModifier(member.Modifiers, "public") && !hasModifier(member.Modifiers, "global") {
				continue
			}
			if member.Kind == apexast.DeclarationProperty {
				readable := false
				for _, accessor := range member.Accessors {
					if strings.EqualFold(accessor.Kind, "get") && !hasModifier(accessor.Modifiers, "private") {
						readable = true
						break
					}
				}
				if !readable {
					continue
				}
			}
			fields = append(fields, semaScopedBinding{name: member.Name, typeName: member.Type, field: true, static: hasModifier(member.Modifiers, "static")})
		}
	}
	if parent, ok := r.bindingSuperType(owner); ok {
		collect(parent)
	}
	return fields
}

func (r *semaBindingResolver) bindingTypeAt(offset int) (typesys.TypeSymbol, bool) {
	var owner typesys.TypeSymbol
	found := false
	for _, typ := range r.typeSymbols {
		if typ.File != r.file || typ.Range.Start.Offset > offset || typ.Range.End.Offset <= offset {
			continue
		}
		if !found || typ.Range.Start.Offset >= owner.Range.Start.Offset {
			owner, found = typ, true
		}
	}
	return owner, found
}

func (r *semaBindingResolver) bindingSuperType(owner typesys.TypeSymbol) (typesys.TypeSymbol, bool) {
	if owner.SuperClass == "" {
		return typesys.TypeSymbol{}, false
	}
	var candidate typesys.TypeSymbol
	found := false
	for _, typ := range r.typeSymbols {
		if typ.SourceRoot != owner.SourceRoot || !strings.EqualFold(typ.Namespace, owner.Namespace) {
			continue
		}
		if !strings.EqualFold(typ.Name, owner.SuperClass) && !strings.EqualFold(typ.LocalName, owner.SuperClass) &&
			!(owner.Namespace != "" && strings.EqualFold(typ.Name, owner.Namespace+"."+owner.SuperClass)) {
			continue
		}
		if typ.File == owner.File {
			return typ, true
		}
		if found {
			return typesys.TypeSymbol{}, false // Ambiguous source occurrences must not lend fields.
		}
		candidate, found = typ, true
	}
	return candidate, found
}
