package sema

import (
	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// semaNamedApexSourceType reports a class or interface declared in named
// source. Anonymous wrapper types and their nested classes are excluded.
func semaNamedApexSourceType(typ typesys.TypeSymbol) bool {
	return (typ.Kind == apexast.DeclarationClass || typ.Kind == apexast.DeclarationInterface) &&
		!hasModifier(typ.Modifiers, vm.AnonymousClassModifier)
}

// namedApexSourceAnalyzer returns a copy that admits the named-source reference
// object types. The receiver and its shared caches keep the anonymous view.
func (a *Analyzer) namedApexSourceAnalyzer() *Analyzer {
	named := *a
	named.namedApexReferenceObjects = true
	return &named
}

func semaNamedApexReferenceObjectName(name string) (string, bool) {
	if target, qualified := semaSchemaQualifiedTypeName(name); qualified {
		name = target
	}
	return storage.ResolveNamedApexReferenceObjectName(name)
}

// This cache belongs to a named body view. It never adds these context-dependent
// types to the shared platform model or the anonymous body's hydration cache.
func (v *semaTypeMemberView) namedApexReferenceObjectMembers(key string) (typeMembers, bool) {
	if !v.namedApexReferenceObjects {
		return typeMembers{}, false
	}
	name, ok := semaNamedApexReferenceObjectName(key)
	if !ok {
		return typeMembers{}, false
	}
	if members, ok := v.namedApexReferenceMembers[name]; ok {
		return members, true
	}
	members := semaBuildStandardSObjectMembers(name)
	members.platform = true
	if v.namedApexReferenceMembers == nil {
		v.namedApexReferenceMembers = make(map[string]typeMembers)
	}
	v.namedApexReferenceMembers[name] = members
	return members, true
}

func semaNamedApexReferenceInsertMessage(typeName string, model *semaTypeMemberView) string {
	members, _, ok := semaLookupTypeMembers(model, typeName)
	if !ok || !members.platform || !members.sobject {
		return ""
	}
	name, ok := storage.ResolveNamedApexReferenceObjectName(members.name)
	if !ok {
		return ""
	}
	return "DML operation Insert not allowed on " + name
}
