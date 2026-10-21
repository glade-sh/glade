package sema

import (
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var capturedFinalIntegerInitializer = regexp.MustCompile(`^\s*[0-9]+\s*;?\s*$`)

// A nested subclass constructor can write its direct base's private or protected
// final for the six captured types. Keep other owners,
// initializer blocks, properties and anonymous nesting on the existing path.
func capturedInheritedFinalConstructor(typ typesys.TypeSymbol, member typesys.MemberSymbol, field resolvedMember, models []*semaTypeMemberView, start, end int, source string) bool {
	if typ.NestingDepth != 1 || typ.OwnerName == "" || typ.Dependency ||
		typ.Namespace != "" || !hasModifier(typ.Modifiers, "public") || hasModifier(typ.Modifiers, "abstract") ||
		hasModifier(typ.Modifiers, "virtual") || start < 0 || end > len(source) || start >= end ||
		!typeUsesAPIVersionAtLeast(typ, 62) || len(models) != 1 ||
		hasModifier(typ.Modifiers, vm.AnonymousClassModifier) ||
		member.Kind != apexast.DeclarationConstructor || len(member.Parameters) > 1 ||
		!hasEitherModifier(field.member.Modifiers, "private", "protected") {
		return false
	}
	target := strings.TrimSpace(source[start:end])
	if !strings.EqualFold(target, field.member.Name) && !strings.EqualFold(target, "this."+field.member.Name) {
		return false
	}
	model := models[0]
	if !capturedInheritedFinalType(model, field.owner, typ.OwnerName, field.member.Type) {
		return false
	}
	if len(member.Parameters) == 1 && !sameInheritedSemaParameterType(
		resolveNestedTypeReference(model, typ.Name, member.Parameters[0].Type),
		resolveNestedTypeReference(model, field.owner, field.member.Type)) {
		return false
	}
	declaration := field.member.Range
	if declaration.Start.Offset < 0 || declaration.End.Offset > len(source) || declaration.End.Offset <= declaration.Start.Offset {
		return false
	}
	text := source[declaration.Start.Offset:declaration.End.Offset]
	if _, initializer, present := strings.Cut(text, "="); present &&
		(len(member.Parameters) != 0 || !strings.EqualFold(target, field.member.Name) ||
			!strings.EqualFold(field.member.Type, "Integer") || !capturedFinalIntegerInitializer.MatchString(initializer)) {
		return false // Retain only the earlier no-arg Integer literal initializer shape.
	}
	parent := typ.SuperClass
	if !strings.Contains(parent, ".") {
		parent = typ.OwnerName + "." + parent
	}
	base, _, ok := semaLookupTypeMembers(models[0], parent)
	return ok && base.nestingDepth == 1 && hasModifier(base.modifiers, "virtual") &&
		strings.EqualFold(parent, field.owner) &&
		strings.EqualFold(typ.OwnerName+"."+shortNestedTypeName(field.owner), field.owner)
}

// USER rows name an enclosing sibling class, not a particular nominal type.
// The other five types retain their captured generic arguments exactly.
func capturedInheritedFinalType(model *semaTypeMemberView, scope, outer, raw string) bool {
	resolved := resolveNestedTypeReference(model, scope, raw)
	switch normalizeName(strings.ReplaceAll(semaCanonicalPlatformAlias(resolved), " ", "")) {
	case "integer", "string", "account", "list<string>", "map<string,object>":
		return true
	}
	user, _, ok := semaLookupTypeMembers(model, resolved)
	return ok && !user.dependency && !user.platform && user.namespace == "" && user.kind == apexast.DeclarationClass &&
		user.nestingDepth == 1 && hasModifier(user.modifiers, "public") && !hasModifier(user.modifiers, "abstract") &&
		strings.EqualFold(user.name, outer+"."+shortNestedTypeName(user.name))
}
