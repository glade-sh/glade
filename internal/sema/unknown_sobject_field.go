package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// A field that a standard SObject does not have is a compile error natively,
// on both routes, for a read, an assignment, a compound assignment, an
// increment and every step of a relationship path: "Variable does not exist:
// <field>" (unknown_sobject_field/org.tsv, 62.0 and 67.0; the constructor form
// "Field does not exist: <field> on <Object>" is checkIRConstructorCall). The
// open fallback for standard objects (semaOpenSObjectFieldMember) cannot tell
// an absent field from one the standard baseline omits, so only the two
// cases the project's own metadata can decide are rejected:
//
//   - a custom field name (a single __c or __r token, or one in the project's
//     own namespace) that the project's declared object metadata does not
//     define. Names in another namespace stay open: an installed package
//     supplies them and the project may not declare it.
//   - a standard field name on an object with a captured closed field
//     surface (standard_field_policy constructor_fields), checked with the
//     same predicate the constructor uses.
//
// A standard field name on any other object stays open, as does a field that
// needs an org feature or a field grant (case_standard_field_fls), and a
// string-keyed put or get, which fails at run time.

// semaUnknownSObjectFieldPath walks a dotted path whose root is a local with
// an SObject type (or a class field that holds one) and returns the first
// segment that an SObject on the path provably lacks.
func (a *Analyzer) semaUnknownSObjectFieldPath(root, path string, scope irSemaScope, model *semaTypeMemberView, currentType string) (missing, owner string, ok bool) {
	if model == nil || model.state == nil || model.state.base == nil {
		return "", "", false
	}
	var current string
	if strings.EqualFold(root, "this") {
		current = currentType
	} else {
		scoped, found := scope.lookup(root)
		if !found {
			return "", "", false
		}
		current = scoped
	}
	// A hop through a custom object's lookup reaches a standard parent whose
	// field list the project may extend partially (relationship_parent_open_field).
	viaCustom := false
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" || strings.ContainsAny(part, "[]()<> ") || strings.EqualFold(part, "class") || strings.EqualFold(part, "SObjectType") {
			return "", "", false
		}
		members, _, known := semaLookupTypeMembers(model, current)
		if !known {
			return "", "", false
		}
		target, resolved := semaResolveField(model, current, part, make(map[string]bool))
		if !resolved && members.sobject {
			target, resolved = semaStandardSObjectFieldMember(current, part)
		}
		if resolved {
			if members.sobject && strings.Contains(members.name, "__") {
				viaCustom = true
			}
			current = strings.TrimSpace(target.member.Type)
			if current == "" {
				return "", "", false
			}
			continue
		}
		if members.sobject && !(viaCustom && !strings.Contains(members.name, "__")) && a.semaStandardSObjectLacksField(model, members, part) {
			return part, members.name, true
		}
		return "", "", false
	}
	return "", "", false
}

// semaStandardSObjectLacksField decides one segment on a standard SObject.
func (a *Analyzer) semaStandardSObjectLacksField(model *semaTypeMemberView, members typeMembers, field string) bool {
	if members.externalPackageSObject || strings.EqualFold(members.name, "SObject") || strings.EqualFold(members.name, "AggregateResult") || strings.EqualFold(members.name, "AuthSession") {
		return false
	}
	if semaHasAPISuffixFold(members.name, "__c") {
		// A custom object's declared metadata is the whole field list (the
		// Widget__c rows). Callers resolve declared fields first.
		return !members.partialSObject && strings.Count(members.name, "__") == 1 && strings.TrimSpace(field) != ""
	}
	if strings.Contains(members.name, "__") {
		return false
	}
	object, standard := storage.ResolveKnownStandardObjectName(members.name)
	if !standard {
		return false
	}
	field = strings.TrimSpace(field)
	if field == "" || storage.StandardObjectFieldRequiresFeature(object, field) {
		return false
	}
	features := model.state.base.orgShapeFeatures
	definition := a.standardConstructorDefinition(object, features, model)
	if _, declared := storage.ResolveFieldName(definition, a.namespace, field); declared {
		return false
	}
	// A project's field files are not the org's complete field list unless a
	// pinned snapshot asserts it (standard_field_access, snapshot rows):
	// custom names on a standard object stay open otherwise.
	closed := members.completeSchemaSObject
	switch strings.Count(field, "__") {
	case 0:
		return !storage.StandardObjectConstructorFieldAllowedForFeatures(object, field, features, definition)
	case 1:
		return closed && (semaHasAPISuffixFold(field, "__c") || semaHasAPISuffixFold(field, "__r"))
	default:
		// ns__Name__c: the project's own namespace is fully declared; another
		// namespace belongs to a package Glade may not see.
		return closed && a.namespace != "" && strings.EqualFold(semaNamespaceFromAPIName(field), a.namespace) &&
			(semaHasAPISuffixFold(field, "__c") || semaHasAPISuffixFold(field, "__r"))
	}
}
