package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// SaveResult/DescribeFieldResult receivers (inner_type_receivers/org.tsv) keep
// their source identity through constructors, including qualified constructors.
// Callers have already checked source/inherited methods and missing superclasses.
// Returning the selected type lets callers defer argument inference until eligible.
func semaInnerReceiverMissingMethodType(receiver, method string, model *semaTypeMemberView, constructed bool) (string, bool) {
	if model == nil || strings.Contains(method, ".") || semaObjectMethodName(method) || strings.EqualFold(method, "getClass") {
		return "", false
	}
	selected, ok := model.lookupName(receiver)
	if !ok || selected.kind != apexast.DeclarationClass || selected.nestingDepth == 0 || selected.platform || selected.dependency || selected.sobject {
		return "", false
	}
	// Native fluent-argument rejection also covers a direct source inner
	// constructor with no platform namesake.
	if constructed {
		return selected.name, true
	}
	for _, platform := range semaPlatformTypesNamed(model, semaShortTypeKey(selected.name)) {
		if platform.kind == apexast.DeclarationClass {
			return selected.name, true
		}
	}
	return "", false
}

// duplicate_reject captures the native return diagnostic for unrelated
// source inner classes. Other conversion families retain their existing wording.
func semaInnerReturnConversion(target, value string, model *semaTypeMemberView) string {
	if model == nil {
		return ""
	}
	to, toOK := model.lookupName(target)
	from, fromOK := model.lookupName(value)
	if !toOK || !fromOK || to.nestingDepth == 0 || from.nestingDepth == 0 ||
		to.kind != apexast.DeclarationClass || from.kind != apexast.DeclarationClass ||
		to.platform || from.platform || to.dependency || from.dependency || to.sobject || from.sobject {
		return ""
	}
	return fmt.Sprintf("Illegal conversion from %s to %s", from.name, to.name)
}

// field_enum_direct_reject: a static field takes precedence over the
// same case-insensitive inner enum name when used as a qualified call receiver.
func semaInnerEnumReceiverFieldAccess(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiver string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if model == nil {
		return diagnostic.Diagnostic{}, false
	}
	owner, name, ok := splitSemaMethodPath(receiver)
	if !ok {
		return diagnostic.Diagnostic{}, false
	}
	selected, ok := model.lookupName(receiver)
	if !ok || selected.kind != apexast.DeclarationEnum || !strings.Contains(selected.name, ".") || selected.platform || selected.dependency {
		return diagnostic.Diagnostic{}, false
	}
	field, ok := semaResolveField(model, owner, name, make(map[string]bool))
	if !ok || !hasModifier(field.member.Modifiers, "static") {
		return diagnostic.Diagnostic{}, false
	}
	// An omitted access modifier is private for this source field. Copy before
	// filling the default so the shared model remains immutable.
	if accessModifier(field.member.Modifiers) == "" {
		field.member.Modifiers = append(append([]string(nil), field.member.Modifiers...), "private")
	}
	if d, rejected := checkSemaMemberAccess(typ, member, receiver, field, start, end, source, model); rejected {
		d.NativeMessage = "Variable is not visible: " + field.owner + "." + field.member.Name
		return d, true
	}
	return diagnostic.Diagnostic{}, false
}

// A rejected argument naming an unrelated same-named source inner class uses
// the same native signature diagnostic on the text and IR routes.
func semaInnerArgumentCollision(candidates []resolvedMember, args []string, model *semaTypeMemberView) bool {
	if model == nil {
		return false
	}
	for _, candidate := range candidates {
		if len(candidate.member.Parameters) != len(args) {
			continue
		}
		for i, param := range candidate.member.Parameters {
			to, toOK := model.lookupName(resolveNestedTypeReference(model, candidate.owner, param.Type))
			from, fromOK := model.lookupName(args[i])
			if toOK && fromOK && to.nestingDepth > 0 && from.nestingDepth > 0 &&
				to.kind == apexast.DeclarationClass && from.kind == apexast.DeclarationClass &&
				!to.platform && !from.platform && !to.dependency && !from.dependency && !to.sobject && !from.sobject &&
				strings.EqualFold(semaShortTypeKey(to.name), semaShortTypeKey(from.name)) && !strings.EqualFold(to.name, from.name) {
				return true
			}
		}
	}
	return false
}

// semaDirectInnerConstructorReceiver confirms the text call's immediate
// receiver is a complete constructor, rather than a surrounding fluent chain.
func semaDirectInnerConstructorReceiver(body string, callStart int, receiver, currentType string, model *semaTypeMemberView) bool {
	dot, ok := semaPreviousDottedChainDot(body, callStart)
	if !ok {
		return false
	}
	start := semaExpressionStart(body[:callStart])
	if start > dot {
		return false
	}
	constructed := semaConstructorReceiverType(strings.TrimSpace(body[start:dot]))
	if constructed == "" {
		return false
	}
	actual, actualOK := model.lookupName(resolveNestedTypeReference(model, currentType, constructed))
	selected, selectedOK := model.lookupName(receiver)
	return actualOK && selectedOK && strings.EqualFold(actual.name, selected.name)
}
