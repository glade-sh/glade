package sema

import (
	"fmt"
	"maps"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const runtimeLoweringDiagnosticCode = "GLADERUNTIME001"

func (a *Analyzer) checkBodyText(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	member = semaNormalizeMemberTypes(model, typ.Name, member)
	baseScope := semaBodyBaseScope(typ, member, model)
	bodyScan := newSemaBodyExpressionScan(body)
	scopes, diagnostics := a.collectBodyScopesWithLocalDeclMatches(typ, member, body, bodyOffset, source, baseScope, model, bodyScan.localDeclMatches)
	diagnostics = semaAsyncLocalTypeDiagnostics(diagnostics, source, model)
	diagnostics = semaDMLLocalAssignmentDiagnostics(diagnostics, typ.Name, body, bodyOffset, scopes, model, bodyScan.localDeclMatches)
	diagnostics = semaAutomationLocalDiagnostics(diagnostics, typ, bodyScan, bodyOffset, source, model)
	diagnostics = semaTriggerLocalAssignmentDiagnostics(diagnostics, typ.Name, body, bodyOffset, scopes, model, bodyScan.localDeclMatches)
	diagnostics = semaMessagingLocalAssignmentDiagnostics(diagnostics, typ.Name, body, bodyOffset, scopes, model, bodyScan.localDeclMatches)
	diagnostics = semaGovernorLocalAssignmentDiagnostics(diagnostics, typ.Name, body, bodyOffset, scopes, model, bodyScan.localDeclMatches)
	diagnostics = semaVisualforceLocalAssignmentDiagnostics(diagnostics, typ.Name, body, bodyOffset, scopes, model, bodyScan.localDeclMatches)
	diagnostics = append(diagnostics, staticThisDiagnostics(typ, member, body, bodyOffset, source)...)
	irDiagnostics, irOK := a.checkBodyIRWithCompileStatus(typ, member, body, bodyOffset, source, baseScope, model, constructability)
	diagnostics = append(diagnostics, irDiagnostics...)
	for _, ctor := range constructorTypes(body) {
		for _, ref := range extractTypeNames(ctor.text) {
			if !a.hasKnownAtVersion(ref, typ.EffectiveAPIVersion) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA006",
					Message:  fmt.Sprintf("%s %q constructs unknown type %q", member.Kind, member.Name, ref),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+ctor.start, bodyOffset+ctor.end),
				})
				continue
			}
			if ref != constructedTypeName(ctor.text) {
				continue
			}
			if target, ok := constructability[normalizeName(ref)]; ok && !isConstructableType(target) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA015",
					Message:  fmt.Sprintf("%s %q constructs non-instantiable %s %q", member.Kind, member.Name, target.Kind, target.Name),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+ctor.start, bodyOffset+ctor.end),
				})
			}
		}
	}
	diagnostics = append(diagnostics, a.checkBodyAssignments(typ, member, bodyScan, bodyOffset, source, scopes, model)...)
	diagnostics = append(diagnostics, a.checkBodyReturns(typ, member, bodyScan, bodyOffset, source, scopes, model)...)
	diagnostics = append(diagnostics, a.checkBodyTernaryConditions(typ, member, bodyScan, bodyOffset, source, scopes, model)...)
	if !irOK {
		diagnostics = append(diagnostics, a.checkBodyExpressionTypeReferences(typ, member, bodyScan, bodyOffset, source)...)
	}
	diagnostics = append(diagnostics, a.checkBodyCalls(typ, member, body, bodyOffset, source, scopes, model)...)
	diagnostics = semaPlatformEventPrioritizeCallDiagnostics(diagnostics, irDiagnostics)
	diagnostics = preserveApexMetadataAssignmentDiagnostics(diagnostics, irDiagnostics, source, model)
	diagnostics = semaMessagingVisibilityDiagnostics(diagnostics, typ.EffectiveAPIVersion, model)
	return dedupeBodyDiagnostics(semaHTTPVisibilityDiagnostics(diagnostics, model))
}

func semaBodyBaseScope(typ typesys.TypeSymbol, member typesys.MemberSymbol, model *semaTypeMemberView) map[string]string {
	baseScope := make(map[string]string)
	baseScope[semaCurrentTypeScopeKey] = typ.Name
	for name, fieldType := range semaFieldScope(model, typ.Name, make(map[string]bool)) {
		baseScope[name] = fieldType
	}
	for _, param := range member.Parameters {
		baseScope[normalizeName(param.Name)] = param.Type
	}
	// this_dispatch C001: argument applicability uses the lexical class,
	// including the text-inference fallbacks used after IR overload lookup.
	if !hasModifier(member.Modifiers, "static") {
		baseScope["this"] = typ.Name
	}
	return baseScope
}

func staticThisDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string) []diagnostic.Diagnostic {
	if !hasModifier(member.Modifiers, "static") {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	ignored := newSemaIgnoredText(body)
	// Scan the original bytes: Unicode case folding can change byte lengths.
	for offset := 0; offset+len("this") <= len(body); offset++ {
		if !strings.EqualFold(body[offset:offset+len("this")], "this") {
			continue
		}
		end := offset + len("this")
		leftBoundary := offset == 0 || !isApexIdentifierChar(body[offset-1])
		rightBoundary := end == len(body) || !isApexIdentifierChar(body[end])
		if leftBoundary && rightBoundary && !ignored.contains(offset) {
			diagnostics = append(diagnostics, semaFieldAccessDiagnostic(typ, member, "this", "this cannot be referenced from a static method", bodyOffset+offset, bodyOffset+end, source))
		}
	}
	return diagnostics
}

func isApexIdentifierChar(value byte) bool {
	return value == '_' || (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9')
}

func isConstructableType(typ typesys.TypeSymbol) bool {
	return typ.Kind == apexast.DeclarationClass && !hasModifier(typ.Modifiers, "abstract")
}

func dedupeBodyDiagnostics(diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	seen := make(map[string]bool)
	out := make([]diagnostic.Diagnostic, 0, len(diagnostics))
	for _, diag := range diagnostics {
		key := ""
		if diag.Range != nil {
			switch diag.Code {
			case "GLADESEMA_VF001":
				if strings.HasPrefix(diag.Message, "Illegal assignment from ") {
					key = fmt.Sprintf("%s:%s:%d:%s", diag.File, diag.Code, diag.Range.Start.Line, diag.Message)
				}
			case "GLADESEMA006", "GLADESEMA008", "GLADESEMA009", "GLADESEMA010", "GLADESEMA011", "GLADESEMA014", "GLADESEMA015", "GLADESEMA018", "GLADESEMA019", "GLADESEMA020", "GLADESEMA022", "GLADESEMA023", "GLADESEMA024", "GLADESEMA025", "GLADESEMA026", "GLADESEMA027", "GLADESEMA028", visualforceControllerAssignmentCode:
				key = fmt.Sprintf("%s:%s:%d", diag.File, diag.Code, diag.Range.Start.Line)
			}
		}
		if key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, diag)
	}
	return out
}

func semaFieldScope(model *semaTypeMemberView, typeName string, seen map[string]bool) map[string]string {
	out := make(map[string]string)
	key := normalizeName(typeName)
	if key == "" || seen[key] {
		return out
	}
	seen[key] = true
	members, ok := model.lookup(key)
	if !ok {
		return out
	}
	members = semaEnsureStandardSObjectTypeMembers(model, key, members)
	for _, owner := range semaEnclosingTypeNames(members.name) {
		ownerMembers, ok := model.lookup(normalizeName(owner))
		if !ok {
			continue
		}
		ownerMembers = semaEnsureStandardSObjectTypeMembers(model, normalizeName(owner), ownerMembers)
		for name, field := range ownerMembers.fields {
			if hasModifier(field.Modifiers, "static") {
				out[name] = field.Type
			}
		}
	}
	for name, field := range semaFieldScope(model, members.superClass, seen) {
		out[name] = field
	}
	for name, field := range members.fields {
		out[name] = field.Type
	}
	return out
}

func semaEnclosingTypeNames(typeName string) []string {
	parts := strings.Split(typeName, ".")
	if len(parts) <= 1 {
		return nil
	}
	out := make([]string, 0, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		out = append(out, strings.Join(parts[:i], "."))
	}
	return out
}

func semaResolveField(model *semaTypeMemberView, typeName, fieldName string, seen map[string]bool) (resolvedMember, bool) {
	return semaResolveFieldByKey(model, typeName, normalizeName(fieldName), fieldName, seen)
}

// semaResolveFieldByKey resolves fieldKey (a case-normalized field name) starting at
// typeName and walking up the superclass chain. exactName carries the field name as
// written at the reference site: Apex field names are case-insensitive, but a subclass
// can declare its own field that only case-insensitively collides with an inherited
// field of a different declared case (e.g. subclass "jobType" vs superclass "JobType").
// An exact-case match anywhere in the hierarchy is preferred over a same-class
// case-insensitive collision, so the correct field (and its own accessibility) is used.
func semaResolveFieldByKey(model *semaTypeMemberView, typeName, fieldKey, exactName string, seen map[string]bool) (resolvedMember, bool) {
	key := normalizeName(typeName)
	if key == "" || seen[key] {
		return resolvedMember{}, false
	}
	if schemaMembers, _, schemaOK := semaExplicitSchemaSObjectMembers(typeName, model); schemaOK {
		if field, ok := semaResolveFieldFromMembers(model, schemaMembers, fieldKey, exactName, seen); ok {
			return field, true
		}
	}
	seen[key] = true
	members, ok := model.lookup(key)
	if !ok {
		for _, candidateKey := range semaShortCandidateKeys(model, key) {
			if candidateKey == key || seen[candidateKey] {
				continue
			}
			candidate := model.get(candidateKey)
			if field, ok := semaResolveFieldByKey(model, candidate.name, fieldKey, exactName, seen); ok {
				return field, true
			}
		}
		return resolvedMember{}, false
	}
	return semaResolveFieldFromMembers(model, members, fieldKey, exactName, seen)
}

func semaResolveFieldFromMembers(model *semaTypeMemberView, members typeMembers, fieldKey, exactName string, seen map[string]bool) (resolvedMember, bool) {
	members = semaEnsureStandardSObjectTypeMembers(model, normalizeName(members.name), members)
	if field, ok := members.fields[fieldKey]; ok {
		resolved := resolvedMember{owner: members.name, member: field}
		if exactName == "" || field.Name == exactName {
			return resolved, true
		}
		// Case-insensitive collision with this class's own field: prefer an
		// exact-case match further up the hierarchy, but keep this as a fallback.
		if fromSuper, ok := semaResolveFieldByKey(model, members.superClass, fieldKey, exactName, seen); ok {
			return fromSuper, true
		}
		return resolved, true
	}
	if namespaced, ok := semaOwnerNamespacedAPIName(members.name, fieldKey); ok {
		if field, ok := members.fields[normalizeName(namespaced)]; ok {
			return resolvedMember{owner: members.name, member: field}, true
		}
	}
	if members.sobject {
		if field, ok := semaStandardChildRelationshipMemberForKey(members.name, fieldKey); ok {
			fields := make(map[string]typesys.MemberSymbol, len(members.fields)+1)
			for key, member := range members.fields {
				fields[key] = member
			}
			fields[fieldKey] = field
			members.fields = fields
			if key := normalizeName(members.name); key != "" {
				model.storeHydrated(key, members)
			}
			return resolvedMember{owner: members.name, member: field}, true
		}
	}
	if field, ok := semaResolveFieldByKey(model, members.superClass, fieldKey, exactName, seen); ok {
		return field, true
	}
	return resolvedMember{}, false
}

func semaLooksLikeSchemaTokenPath(field string) bool {
	parts := strings.Split(field, ".")
	return len(parts) >= 2 && strings.EqualFold(parts[1], "SObjectType")
}

func semaResolveFieldPath(model *semaTypeMemberView, receiverType, fieldPath string) (resolvedMember, bool) {
	parts := strings.Split(fieldPath, ".")
	if len(parts) == 0 {
		return resolvedMember{}, false
	}
	currentType := receiverType
	var target resolvedMember
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return resolvedMember{}, false
		}
		if resolved, ok := semaResolveSObjectTypeFieldPath(currentType, part); ok {
			target = resolved
			currentType = resolved.member.Type
			continue
		}
		resolved, ok := semaResolveField(model, currentType, part, make(map[string]bool))
		if !ok {
			if standardField, standardOK := semaStandardSObjectFieldMember(currentType, part); standardOK {
				resolved = standardField
				ok = true
			}
		}
		if !ok {
			if sobjectField, fieldOK := semaOpenSObjectFieldMember(currentType, part, model); fieldOK {
				resolved = sobjectField
			} else {
				return resolvedMember{}, false
			}
		}
		target = resolved
		currentType = resolved.member.Type
	}
	return target, true
}

func semaStandardSObjectFieldMember(typeName, fieldName string) (resolvedMember, bool) {
	objectName, ok := semaStandardSObjectNameForKey(normalizeName(typeName))
	if !ok {
		return resolvedMember{}, false
	}
	members := semaBuildStandardSObjectMembers(objectName)
	field, ok := members.fields[normalizeName(fieldName)]
	if !ok {
		return resolvedMember{}, false
	}
	return resolvedMember{owner: objectName, member: field}, true
}

// Resolve only concrete schema relationship paths. Open or partial SObjects,
// dynamic polymorphic parents and fields on Apex classes retain their own rules.
// C004 backs rejection of an unknown field reached through a known parent.
func semaSObjectRelationshipPath(model *semaTypeMemberView, receiverType, fieldPath string) (fieldType, missing string, relationship bool) {
	currentType := receiverType
	for i, part := range strings.Split(fieldPath, ".") {
		members, _, ok := semaLookupTypeMembers(model, currentType)
		if !ok || !members.sobject || members.externalPackageSObject || members.partialSObject || strings.EqualFold(members.name, "SObject") || strings.EqualFold(members.name, "AggregateResult") {
			return "", "", false
		}
		part = strings.TrimSpace(part)
		field, ok := semaResolveField(model, currentType, part, make(map[string]bool))
		if !ok {
			// Relationship targets may be project-generated child aliases with
			// only those aliases in their member map. Resolve known standard
			// fields from the catalog before diagnosing a missing relationship leaf.
			field, ok = semaStandardSObjectFieldMember(members.name, part)
		}
		if !ok {
			if i > 0 && relationship {
				return "", part, true
			}
			return "", "", false
		}
		currentType = field.member.Type
		if i == 0 {
			if !semaConcreteRelationshipFieldType(currentType, model) {
				return "", "", false
			}
			relationship = true
		}
	}
	return currentType, "", relationship
}

func semaConcreteRelationshipFieldType(typeName string, model *semaTypeMemberView) bool {
	base, args := semaGenericBaseAndArgs(typeName)
	if strings.EqualFold(base, "List") && len(args) == 1 {
		typeName = args[0]
	}
	members, _, ok := semaLookupTypeMembers(model, typeName)
	return ok && members.sobject && !strings.EqualFold(members.name, "SObject") && !strings.EqualFold(members.name, "AggregateResult")
}

func semaRelationshipExpressionType(expr string, bindings map[string]string, model *semaTypeMemberView) string {
	root, path, ok := strings.Cut(strings.TrimSpace(expr), ".")
	if !ok {
		return ""
	}
	receiverType := bindings[normalizeName(root)]
	fieldType, missing, relationship := semaSObjectRelationshipPath(model, receiverType, path)
	if !relationship || missing != "" || !semaConcreteRelationshipFieldType(fieldType, model) {
		return ""
	}
	return fieldType
}

// C020: a child relationship initializer uses the runtime singleton/cardinality
// conversion. A local List, list literal or ordinary class field is not this path.
func semaChildRelationshipSingletonAssignable(targetType, valueType, expr string, bindings map[string]string, model *semaTypeMemberView) bool {
	base, args := semaGenericBaseAndArgs(valueType)
	if !strings.EqualFold(base, "List") || len(args) != 1 ||
		!isSemaSObjectLike(targetType, model) || !semaAssignableToType(targetType, args[0], model) {
		return false
	}
	relationshipType := semaRelationshipExpressionType(expr, bindings, model)
	if relationshipType == "" {
		// C020's named route infers a partial Account from source. Its known
		// standard child relationship still has an authoritative list type;
		// unknown fields and parent diagnostics keep the partial-schema guard.
		root, relationship, ok := strings.Cut(strings.TrimSpace(expr), ".")
		if !ok || strings.Contains(relationship, ".") {
			return false
		}
		members, _, ok := semaLookupTypeMembers(model, bindings[normalizeName(root)])
		if !ok || !members.sobject || !members.partialSObject || members.externalPackageSObject {
			return false
		}
		child, ok := semaStandardChildRelationshipMemberForKey(members.name, normalizeName(relationship))
		if !ok {
			return false
		}
		relationshipType = child.Type
	}
	return sameSemaSignatureType(relationshipType, valueType)
}

// C018/C019/C021: preserve the native diagnostic on SOQL result and concrete
// relationship assignments, without rewriting ordinary assignment diagnostics.
func semaRelationshipAssignmentMessage(targetType, valueType, expr string, bindings map[string]string, model *semaTypeMemberView, fallback string) string {
	if message, ok := semaSearchAssignmentMessage(targetType, valueType, expr, model); ok {
		return message
	}
	queryType := ""
	if semaExprLooksLikeSOQLLiteral(expr) {
		literal := strings.TrimSpace(expr)
		queryType = semaSOQLLiteralType(literal[1 : len(literal)-1])
	}
	queryResult := semaConcreteRelationshipFieldType(queryType, model)
	if queryResult || semaRelationshipExpressionType(expr, bindings, model) != "" {
		return fmt.Sprintf("Illegal assignment from %s to %s", valueType, targetType)
	}
	return fallback
}

// C003: only a scalar COUNT() query assigned to aggregate rows uses this
// native message. Ordinary assignments retain their existing diagnostics.
func semaAggregateAssignmentMessage(targetType, valueType, expr string, model *semaTypeMemberView, fallback string) string {
	if !strings.EqualFold(targetType, "List<AggregateResult>") || valueType != "Integer" ||
		semaProjectTypeShadowsPlatform(model, "AggregateResult") || !semaExprLooksLikeSOQLLiteral(expr) {
		return fallback
	}
	literal := strings.TrimSpace(expr)
	if semaSOQLLiteralType(literal[1:len(literal)-1]) != "Integer" {
		return fallback
	}
	return fmt.Sprintf("Illegal assignment from %s to %s", valueType, targetType)
}

func semaResolveSObjectTypeFieldPath(currentType, part string) (resolvedMember, bool) {
	switch {
	case strings.EqualFold(currentType, "Schema.SObjectType") && strings.EqualFold(part, "SObjectType"):
		return resolvedMember{owner: currentType, member: typesys.MemberSymbol{
			Kind: apexast.DeclarationField,
			Name: part,
			Type: "Schema.SObjectType",
		}}, true
	case strings.EqualFold(currentType, "Schema.SObjectType") && strings.EqualFold(part, "fields"):
		return resolvedMember{owner: currentType, member: typesys.MemberSymbol{
			Kind: apexast.DeclarationField,
			Name: part,
			Type: "Schema.SObjectTypeFields",
		}}, true
	case strings.EqualFold(currentType, "Schema.SObjectType") && strings.EqualFold(part, "fieldSets"):
		return resolvedMember{owner: currentType, member: typesys.MemberSymbol{
			Kind: apexast.DeclarationField,
			Name: part,
			Type: "Schema.SObjectTypeFieldSets",
		}}, true
	case strings.EqualFold(currentType, "Schema.DescribeSObjectResult") && strings.EqualFold(part, "fields"):
		return resolvedMember{owner: currentType, member: typesys.MemberSymbol{
			Kind: apexast.DeclarationField,
			Name: part,
			Type: "Schema.SObjectTypeFields",
		}}, true
	case strings.EqualFold(currentType, "Schema.DescribeSObjectResult") && strings.EqualFold(part, "fieldSets"):
		return resolvedMember{owner: currentType, member: typesys.MemberSymbol{
			Kind: apexast.DeclarationField,
			Name: part,
			Type: "Schema.SObjectTypeFieldSets",
		}}, true
	case semaIsSObjectTypeFields(currentType) && semaFieldTokenPart(part):
		return resolvedMember{owner: currentType, member: typesys.MemberSymbol{
			Kind: apexast.DeclarationField,
			Name: part,
			Type: "Schema.DescribeFieldResult",
		}}, true
	case strings.EqualFold(currentType, "Schema.SObjectTypeFieldSets") && semaFieldTokenPart(part):
		return resolvedMember{owner: currentType, member: typesys.MemberSymbol{
			Kind: apexast.DeclarationField,
			Name: part,
			Type: "Schema.FieldSet",
		}}, true
	default:
		return resolvedMember{}, false
	}
}

func semaIsSObjectTypeFields(typeName string) bool {
	return strings.EqualFold(typeName, "Schema.SObjectTypeFields") ||
		strings.EqualFold(typeName, "Schema.SObjectFields")
}

func semaOpenSObjectFieldMember(typeName, fieldName string, model *semaTypeMemberView) (resolvedMember, bool) {
	members, _, ok := semaLookupTypeMembers(model, typeName)
	if !ok || !members.sobject {
		return resolvedMember{}, false
	}
	_, standardObject := semaStandardSObjectNameForKey(normalizeName(members.name))
	if !members.externalPackageSObject && !members.partialSObject &&
		!standardObject && !strings.HasSuffix(normalizeName(members.name), "__mdt") {
		return resolvedMember{}, false
	}
	return resolvedMember{owner: typeName, member: typesys.MemberSymbol{
		Kind:      apexast.DeclarationField,
		Name:      fieldName,
		Type:      "",
		Modifiers: []string{"public"},
	}}, true
}

func semaExternalPackageSObjectType(typeName string, model *semaTypeMemberView) bool {
	members, _, ok := semaLookupTypeMembers(model, typeName)
	return ok && members.sobject && members.externalPackageSObject
}

func semaExternalPackageSObjectFieldPath(expr string, scope map[string]string, model *semaTypeMemberView) bool {
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) < 2 {
		return false
	}
	receiverType := ""
	start := 1
	switch {
	case strings.EqualFold(parts[0], "this"):
		receiverType = scope[semaCurrentTypeScopeKey]
	case strings.EqualFold(parts[0], "super"):
		if members, _, ok := semaLookupTypeMembers(model, scope[semaCurrentTypeScopeKey]); ok {
			receiverType = members.superClass
		}
	default:
		if scoped, ok := scope[normalizeName(parts[0])]; ok {
			receiverType = scoped
		} else if members, _, ok := semaLookupTypeMembers(model, parts[0]); ok {
			receiverType = members.name
		}
	}
	if receiverType == "" {
		return false
	}
	for _, field := range parts[start:] {
		if field == "" {
			return false
		}
		if semaExternalPackageSObjectType(receiverType, model) && semaIsCustomAPIName(field) {
			return true
		}
		resolved, ok := semaResolveFieldPath(model, receiverType, field)
		if !ok {
			return false
		}
		receiverType = resolved.member.Type
	}
	return false
}

func semaParentRelationshipFieldName(fieldName string) string {
	if strings.HasSuffix(fieldName, "__c") {
		return strings.TrimSuffix(fieldName, "__c") + "__r"
	}
	return ""
}

func semaLooksLikeChildRelationship(key string) bool {
	trimmed := strings.TrimRight(key, "0123456789")
	return trimmed != key || strings.HasSuffix(trimmed, "s")
}

type irSemaOrigin int

const (
	irSemaOriginField irSemaOrigin = iota
	irSemaOriginParam
	irSemaOriginLocal
	// A local declared final. Only increment shapes distinguish it.
	irSemaOriginFinalLocal
)

type irSemaBinding struct {
	typ    string
	origin irSemaOrigin
}

type irSemaScope struct {
	frames    []map[string]irSemaBinding
	anonymous bool
	// For-loop header parts and multi-variable declarations are outside the
	// captured increment statement and initializer positions.
	incrementHeader bool
	// Set when a ++/-- statement that only the VM prefix fallback lowers has
	// no captured rule; the body then keeps its name-path compile result.
	uncapturedPrefix *bool
	// When set, collects the body offsets of the fallback statements a
	// captured rule covers: the ones the runtime may lower.
	approvedPrefix map[int]bool
	flatMemo       *irSemaScopeFlatMemo
	flatVersion    *uint64
}

// Value copies share the version of their binding maps. Structural changes
// detach the memo so same-depth sibling scopes cannot reuse one another's map.
type irSemaScopeFlatMemo struct {
	bindings map[string]string
	version  uint64
}

func newIRSemaScope(base map[string]string) irSemaScope {
	return newIRSemaScopeWithOrigins(base, nil)
}

func newIRSemaScopeWithOrigins(base map[string]string, params map[string]struct{}) irSemaScope {
	root := make(map[string]irSemaBinding, len(base))
	for name, typ := range base {
		key := normalizeName(name)
		origin := irSemaOriginField
		if _, ok := params[key]; ok {
			origin = irSemaOriginParam
		}
		if key == semaCurrentTypeScopeKey || key == semaInferenceDepthScopeKey {
			origin = irSemaOriginField
		}
		root[key] = irSemaBinding{typ: typ, origin: origin}
	}
	return irSemaScope{frames: []map[string]irSemaBinding{root}, flatMemo: &irSemaScopeFlatMemo{}, flatVersion: new(uint64)}
}

func (s *irSemaScope) push() {
	s.invalidateFlat()
	s.frames = append(s.frames, make(map[string]irSemaBinding))
}

func (s *irSemaScope) pop() {
	if len(s.frames) > 1 {
		s.invalidateFlat()
		s.frames = s.frames[:len(s.frames)-1]
	}
}

func (s *irSemaScope) declare(name, typ string) bool {
	if len(s.frames) == 0 {
		s.push()
	}
	key := normalizeName(name)
	if key == "" {
		return true
	}
	for i, frame := range s.frames {
		// Execute-anonymous root declarations are fields of its implicit type.
		// A block-local may shadow them; duplicates in that root still collide.
		if s.anonymous && i == 0 && len(s.frames) > 1 {
			continue
		}
		if binding, ok := frame[key]; ok && binding.origin != irSemaOriginField {
			return false
		}
	}
	s.invalidateFlat()
	s.frames[len(s.frames)-1][key] = irSemaBinding{typ: typ, origin: irSemaOriginLocal}
	return true
}

func (s irSemaScope) lookup(name string) (string, bool) {
	key := normalizeName(name)
	for i := len(s.frames) - 1; i >= 0; i-- {
		if binding, ok := s.frames[i][key]; ok {
			return binding.typ, true
		}
	}
	return "", false
}

func (s irSemaScope) hasNonFieldBinding(name string) bool {
	key := normalizeName(name)
	for i := len(s.frames) - 1; i >= 0; i-- {
		if binding, ok := s.frames[i][key]; ok {
			return binding.origin != irSemaOriginField
		}
	}
	return false
}

func (s irSemaScope) binding(name string) (irSemaBinding, bool) {
	key := normalizeName(name)
	for i := len(s.frames) - 1; i >= 0; i-- {
		if binding, ok := s.frames[i][key]; ok {
			return binding, true
		}
	}
	return irSemaBinding{}, false
}

// markFinalLocal records that the local just declared in the innermost frame
// was written final.
func (s *irSemaScope) markFinalLocal(name string) {
	if len(s.frames) == 0 {
		return
	}
	key := normalizeName(name)
	frame := s.frames[len(s.frames)-1]
	if binding, ok := frame[key]; ok && binding.origin == irSemaOriginLocal {
		binding.origin = irSemaOriginFinalLocal
		frame[key] = binding
	}
}

func semaStaticInitializer(member typesys.MemberSymbol) bool {
	return member.Kind == apexast.DeclarationInitializer && hasModifier(member.Modifiers, "static")
}

func (a *Analyzer) staticFinalFieldsInitializedElsewhere(typ typesys.TypeSymbol, member typesys.MemberSymbol, source string, model *semaTypeMemberView, baseScope map[string]string) map[string]bool {
	assigned := make(map[string]bool)
	if !semaStaticInitializer(member) {
		return assigned
	}
	for _, candidate := range typ.Members {
		if candidate.Kind == apexast.DeclarationField && hasModifier(candidate.Modifiers, "final") && hasModifier(candidate.Modifiers, "static") {
			if semaStaticFinalFieldHasDeclarationInitializer(candidate, source) {
				assigned[normalizeName(candidate.Name)] = true
			}
			continue
		}
		if candidate.Range.Start.Offset >= member.Range.Start.Offset || !semaStaticInitializer(candidate) {
			continue
		}
		body, _, ok := semaBodyFromRange(source, candidate.BodyRange)
		if !ok {
			continue
		}
		program, err := vm.CompileAnonymous(body)
		if err != nil {
			continue
		}
		scope := newIRSemaScopeWithOrigins(baseScope, nil)
		a.collectStaticFinalFieldWrites(typ, program.Instructions, &scope, model, assigned)
	}
	return assigned
}

func semaStaticFinalFieldHasDeclarationInitializer(member typesys.MemberSymbol, source string) bool {
	declaration, _, ok := memberSource(source, member.Range)
	if !ok {
		return false
	}
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	ignored := newSemaIgnoredText(declaration)
	for index := 0; index < len(declaration); index++ {
		if ignored.contains(index) {
			continue
		}
		switch declaration[index] {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '=':
			if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 ||
				(index > 0 && strings.ContainsRune("!<>=", rune(declaration[index-1]))) ||
				(index+1 < len(declaration) && declaration[index+1] == '=') {
				continue
			}
			return true
		}
		if declaration[index] == ';' && parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
			return false
		}
	}
	return false
}

func (a *Analyzer) collectStaticFinalFieldWrites(typ typesys.TypeSymbol, instructions []ir.Instruction, scope *irSemaScope, model *semaTypeMemberView, assigned map[string]bool) {
	for _, inst := range instructions {
		switch inst.Op {
		case ir.OpDeclare:
			scope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type))
		case ir.OpAssign:
			if scope.hasNonFieldBinding(inst.Name) {
				continue
			}
			field, found := semaResolveField(model, typ.Name, inst.Name, make(map[string]bool))
			if found && hasModifier(field.member.Modifiers, "final") && hasModifier(field.member.Modifiers, "static") {
				assigned[normalizeName(inst.Name)] = true
			}
		case ir.OpBlock:
			scope.push()
			a.collectStaticFinalFieldWrites(typ, inst.Then, scope, model, assigned)
			scope.pop()
		case ir.OpDeclGroup:
			a.collectStaticFinalFieldWrites(typ, inst.Then, scope, model, assigned)
		case ir.OpIf, ir.OpWhile, ir.OpDoWhile, ir.OpRunAs:
			scope.push()
			a.collectStaticFinalFieldWrites(typ, inst.Then, scope, model, assigned)
			scope.pop()
			if inst.Op == ir.OpIf {
				scope.push()
				a.collectStaticFinalFieldWrites(typ, inst.Else, scope, model, assigned)
				scope.pop()
			}
		case ir.OpFor:
			scope.push()
			inits := inst.Inits
			if len(inits) == 0 && inst.Init != nil {
				inits = []ir.Instruction{*inst.Init}
			}
			a.collectStaticFinalFieldWrites(typ, inits, scope, model, assigned)
			a.collectStaticFinalFieldWrites(typ, inst.Then, scope, model, assigned)
			updates := inst.Updates
			if len(updates) == 0 && inst.Update != nil {
				updates = []ir.Instruction{*inst.Update}
			}
			a.collectStaticFinalFieldWrites(typ, updates, scope, model, assigned)
			scope.pop()
		case ir.OpForEach:
			scope.push()
			scope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type))
			a.collectStaticFinalFieldWrites(typ, inst.Then, scope, model, assigned)
			scope.pop()
		case ir.OpTry:
			scope.push()
			a.collectStaticFinalFieldWrites(typ, inst.Then, scope, model, assigned)
			scope.pop()
			for _, catchClause := range catchClauses(inst) {
				scope.push()
				if catchClause.Name != "" {
					catchType := "Exception"
					if len(catchClause.Types) > 0 {
						catchType = catchClause.Types[0]
					}
					scope.declare(catchClause.Name, catchType)
				}
				a.collectStaticFinalFieldWrites(typ, catchClause.Body, scope, model, assigned)
				scope.pop()
			}
			scope.push()
			a.collectStaticFinalFieldWrites(typ, inst.Finally, scope, model, assigned)
			scope.pop()
		case ir.OpSwitch:
			for _, switchCase := range inst.Cases {
				scope.push()
				for _, expr := range switchCase.Exprs {
					if caseType, binding, ok := irSwitchTypeCase(expr); ok {
						scope.declare(binding, caseType)
					}
				}
				a.collectStaticFinalFieldWrites(typ, switchCase.Body, scope, model, assigned)
				scope.pop()
			}
		}
	}
}

func (s *irSemaScope) invalidateFlat() {
	if s.flatVersion == nil {
		s.flatVersion = new(uint64)
	}
	*s.flatVersion++
	s.flatMemo = &irSemaScopeFlatMemo{}
}

// flat returns read-only bindings. Consumers that infer text expressions or
// otherwise write bindings must use flatCopy.
func (s *irSemaScope) flat() map[string]string {
	if s.flatMemo == nil {
		s.flatMemo = &irSemaScopeFlatMemo{}
	}
	if s.flatVersion == nil {
		s.flatVersion = new(uint64)
	}
	if s.flatMemo.bindings != nil && s.flatMemo.version == *s.flatVersion {
		return s.flatMemo.bindings
	}
	size := 0
	for _, frame := range s.frames {
		size += len(frame)
	}
	out := make(map[string]string, size)
	for _, frame := range s.frames {
		for name, binding := range frame {
			out[name] = binding.typ
		}
	}
	s.flatMemo.bindings = out
	s.flatMemo.version = *s.flatVersion
	return out
}

func (s *irSemaScope) flatCopy() map[string]string {
	return maps.Clone(s.flat())
}

func (a *Analyzer) checkBodyIR(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	diagnostics, _ := a.checkBodyIRWithCompileStatus(typ, member, body, bodyOffset, source, base, model, constructability)
	return diagnostics
}

func (a *Analyzer) checkBodyIRWithCompileStatus(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) ([]diagnostic.Diagnostic, bool) {
	return a.checkBodyIRWithScopeOptions(typ, member, body, bodyOffset, source, base, model, constructability, false, nil)
}

// approved, when not nil, receives the body offsets of the prefix candidates a
// captured rule covers; it stays empty for a body that keeps the name path.
func (a *Analyzer) checkBodyIRWithScopeOptions(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol, anonymous bool, approved map[int]bool) ([]diagnostic.Diagnostic, bool) {
	if !anonymous {
		if diagnostics := localAnnotationDiagnostics(typ, body, bodyOffset, source); len(diagnostics) > 0 {
			return diagnostics, false
		}
	}
	if diagnostics := sourceSyntaxDiagnostics(typ, body, bodyOffset, source, anonymous); len(diagnostics) > 0 {
		return diagnostics, false
	}
	if diagnostics := semaForUpdateDiagnostics(typ, member, body, bodyOffset, source); len(diagnostics) > 0 {
		return diagnostics, false
	}
	program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{PrefixStatementCandidates: semaPrefixStatementCandidates(typ, member, anonymous)})
	if err != nil {
		return semaBodyLoweringDiagnostics(typ, body, bodyOffset, source, anonymous, err), false
	}
	scope := newIRSemaScopeWithOrigins(base, semaMemberParameterNames(member))
	scope.anonymous = anonymous
	uncapturedPrefix := false
	scope.uncapturedPrefix = &uncapturedPrefix
	scope.approvedPrefix = approved
	diagnostics := a.apexMetadataAdmissionDiagnostics(typ, program, scope, bodyOffset, source, model)
	if a.includePerformanceDiagnostics {
		diagnostics = append(diagnostics, performanceDiagnosticsForProgram(typ, program, bodyOffset, source)...)
	}
	diagnostics = append(diagnostics, a.checkIRExpressionTypeReferences(typ, member, program.Instructions, body, bodyOffset, source)...)
	diagnostics = append(diagnostics, a.checkIRStatementContracts(typ, member, program.Instructions, scope, bodyOffset, source, model)...)
	diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, program.Instructions, &scope, bodyOffset, source, model, constructability)...)
	if uncapturedPrefix {
		// No captured rule covers the statement: keep the name-path result.
		if _, err := vm.CompileAnonymous(body); err != nil {
			clear(approved)
			return semaBodyLoweringDiagnostics(typ, body, bodyOffset, source, anonymous, err), false
		}
	}
	diagnostics = append(diagnostics, a.checkConstructorChainingIR(typ, member, program.Instructions, bodyOffset, source, model)...)
	returnType := strings.TrimSpace(member.Type)
	memberSource := ""
	if member.Range.Start.Offset >= 0 && member.Range.End.Offset > member.Range.Start.Offset && member.Range.End.Offset <= len(source) {
		memberSource = source[member.Range.Start.Offset:member.Range.End.Offset]
	}
	if returnType != "" && !strings.EqualFold(returnType, "void") && !irInstructionsTerminate(program.Instructions) && !semaBodyEndsWithThrow(body) && !semaBodyEndsWithThrow(memberSource) {
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s on all paths", returnType), member.Range.Start.Offset, member.Range.End.Offset, source))
	}
	diagnostics = semaMessagingVisibilityDiagnostics(diagnostics, typ.EffectiveAPIVersion, model)
	return semaHTTPVisibilityDiagnostics(reconcileApexMetadataAssignmentDiagnostics(diagnostics), model), true
}

// semaBodyLoweringDiagnostics reports a body the VM cannot compile.
func semaBodyLoweringDiagnostics(typ typesys.TypeSymbol, body string, bodyOffset int, source string, anonymous bool, err error) []diagnostic.Diagnostic {
	if syntax, ok := err.(*vm.ApexSyntaxError); ok {
		return []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "GLADESEMA_BODY_PARSE", Message: syntax.Error(), NativeMessage: exceptionSyntaxMessage(syntax.NativeMessage, anonymous), File: typ.File, Range: semaRange(source, bodyOffset+syntax.Offset, bodyOffset+syntax.Offset+1)}}
	}
	if semaApexParserAcceptsBody(body, typ.File) {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Warning,
			Code:     runtimeLoweringDiagnosticCode,
			Message:  fmt.Sprintf("local VM cannot lower this Apex body: %v", err),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset, bodyOffset+len(body)),
		}}
	}
	return nil
}

func semaForUpdateDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string) []diagnostic.Diagnostic {
	// Native for-loop cases reject multiple update expressions;
	// multiple initial declarations are allowed. Check before VM lowering.
	body = maskInlineQueryComments(body)
	var diagnostics []diagnostic.Diagnostic
	ignored := newSemaIgnoredText(body)
	for _, match := range forHeaderPattern.FindAllStringIndex(body, -1) {
		if ignored.contains(match[0]) {
			continue
		}
		header, headerStart, ok := semaBalancedUntil(body, match[1]-1, '(', ')')
		if !ok {
			continue
		}
		firstSemi := semaTopLevelByte(header, ';')
		if firstSemi < 0 {
			continue
		}
		secondSemi := semaTopLevelByte(header[firstSemi+1:], ';')
		if secondSemi < 0 {
			continue
		}
		updateStart := firstSemi + 1 + secondSemi + 1
		// The argument-list reader distinguishes clause separators from commas
		// inside calls, collection expressions, generic types and strings.
		updates, ok := callArgumentsAt("("+header[updateStart:]+")", 0)
		if ok && len(updates) > 1 {
			comma := headerStart + updateStart + updates[0].end - 1
			diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "for update requires one expression", bodyOffset+comma, source))
		}
	}
	return diagnostics
}

func semaApexParserAcceptsBody(body, file string) bool {
	probe := "public class GladeRuntimeLoweringProbe { public void run() {\n" + body + "\n} }"
	parsed := apexast.ParseSource(file, probe)
	for _, diag := range parsed.Diagnostics {
		if diag.Severity == diagnostic.Error {
			return false
		}
	}
	return true
}

func (a *Analyzer) checkConstructorChainingIR(typ typesys.TypeSymbol, member typesys.MemberSymbol, instructions []ir.Instruction, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	if member.Kind != apexast.DeclarationConstructor {
		return nil
	}
	var chainIndexes []int
	for i, inst := range instructions {
		if isConstructorChainInstruction(inst) {
			chainIndexes = append(chainIndexes, i)
		}
	}
	var diagnostics []diagnostic.Diagnostic
	if len(chainIndexes) > 1 {
		inst := instructions[chainIndexes[1]]
		diagnostics = append(diagnostics, constructorDiagnostic(typ, member, irConstructorChainCallee(inst), "constructor may contain at most one this(...)/super(...) call", bodyOffset+inst.Pos, bodyOffset+inst.Pos+1, source))
	}
	if len(chainIndexes) == 1 && chainIndexes[0] != 0 {
		inst := instructions[chainIndexes[0]]
		diagnostics = append(diagnostics, constructorDiagnostic(typ, member, irConstructorChainCallee(inst), "this(...)/super(...) must be the first statement in a constructor", bodyOffset+inst.Pos, bodyOffset+inst.Pos+1, source))
	}
	if len(chainIndexes) == 0 {
		diagnostics = append(diagnostics, a.checkImplicitSuperConstructor(typ, member, bodyOffset, source, model)...)
	}
	return diagnostics
}

func isConstructorChainInstruction(inst ir.Instruction) bool {
	if inst.Op != ir.OpExpr || inst.Expr.Kind != ir.ExprCall {
		return false
	}
	return strings.EqualFold(inst.Expr.Callee, "this") || strings.EqualFold(inst.Expr.Callee, "super")
}

func irConstructorChainCallee(inst ir.Instruction) string {
	if strings.EqualFold(inst.Expr.Callee, "super") {
		return "super"
	}
	return "this"
}

func (a *Analyzer) checkImplicitSuperConstructor(typ typesys.TypeSymbol, member typesys.MemberSymbol, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	superName := strings.TrimSpace(typ.SuperClass)
	if superName == "" {
		return nil
	}
	if resolved := resolveNestedTypeName(model, typ.Name, superName); resolved != "" {
		superName = resolved
	}
	target, ok := model.lookup(normalizeName(superName))
	if !ok {
		return nil
	}
	if len(target.constructors) == 0 {
		return nil
	}
	for _, ctor := range target.constructors {
		if len(ctor.Parameters) != 0 {
			continue
		}
		if _, blocked := checkSemaMemberAccess(typ, member, "super", resolvedMember{owner: target.name, member: ctor}, member.Range.Start.Offset, member.Range.End.Offset, source, model); blocked {
			continue
		}
		return nil
	}
	return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "super", fmt.Sprintf("implicit super() requires an accessible no-argument constructor on %s", superName), member.Range.Start.Offset, member.Range.End.Offset, source)}
}

func (a *Analyzer) checkImplicitDefaultConstructors(index typesys.Index, model *semaTypeMemberView) []diagnostic.Diagnostic {
	if a.sources == nil {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) || typ.Kind != apexast.DeclarationClass || strings.TrimSpace(typ.SuperClass) == "" || !declarationFromParsedSource(typ) {
			continue
		}
		hasConstructor := false
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationConstructor {
				hasConstructor = true
				break
			}
		}
		if hasConstructor {
			continue
		}
		source, ok := a.sources.normalizedForType(typ)
		if !ok {
			continue
		}
		member := typesys.MemberSymbol{Kind: apexast.DeclarationConstructor, Name: typ.LocalName, Range: typ.Range}
		diagnostics = append(diagnostics, a.checkImplicitSuperConstructor(typ, member, typ.Range.Start.Offset, source, model)...)
	}
	return diagnostics
}

func (a *Analyzer) checkIRExpressionTypeReferences(typ typesys.TypeSymbol, member typesys.MemberSymbol, instructions []ir.Instruction, body string, bodyOffset int, source string) []diagnostic.Diagnostic {
	var seen map[string]bool
	var diagnostics []diagnostic.Diagnostic
	var walkExpr func(ir.Expr, int)
	var walkInstruction func(ir.Instruction)
	var walkInstructions func([]ir.Instruction)
	appendTypeDiagnostics := func(typeName string, pos int) {
		if seen == nil {
			seen = make(map[string]bool)
		}
		start := bodyOffset + semaIRExpressionTypeReferenceStart(body, pos, typeName)
		diagnostics = append(diagnostics, a.expressionTypeReferenceDiagnostics(typ, member, typeName, start, source, seen)...)
	}
	walkExpr = func(expr ir.Expr, pos int) {
		if expr.Kind == "" {
			return
		}
		switch expr.Kind {
		case ir.ExprCall:
			if strings.HasPrefix(expr.Callee, "__cast:") {
				typeName := strings.TrimPrefix(expr.Callee, "__cast:")
				appendTypeDiagnostics(typeName, pos)
			}
			if expr.Left != nil {
				walkExpr(*expr.Left, pos)
			}
			for _, arg := range expr.Args {
				walkExpr(arg, pos)
			}
			for _, arg := range expr.NamedArgs {
				walkExpr(arg.Expr, pos)
			}
		case ir.ExprUnary:
			if expr.Left != nil {
				walkExpr(*expr.Left, pos)
			}
			if expr.Right != nil {
				walkExpr(*expr.Right, pos)
			}
		case ir.ExprBinary:
			if expr.Left != nil {
				walkExpr(*expr.Left, pos)
			}
			if strings.EqualFold(expr.Operator, "instanceof") {
				if expr.Right != nil {
					typeName := expr.Right.Name
					appendTypeDiagnostics(typeName, pos)
				}
				return
			}
			if expr.Right != nil {
				walkExpr(*expr.Right, pos)
			}
		}
	}
	walkInstruction = func(inst ir.Instruction) {
		walkExpr(inst.Expr, inst.Pos)
		if inst.Init != nil {
			walkInstruction(*inst.Init)
		}
		walkInstructions(inst.Inits)
		if inst.Update != nil {
			walkInstruction(*inst.Update)
		}
		walkInstructions(inst.Updates)
		walkInstructions(inst.Then)
		walkInstructions(inst.Else)
		walkInstructions(inst.Catch)
		for _, catchClause := range inst.Catches {
			walkInstructions(catchClause.Body)
		}
		walkInstructions(inst.Finally)
		for _, switchCase := range inst.Cases {
			for _, expr := range switchCase.Exprs {
				walkExpr(expr, switchCase.Pos)
			}
			walkInstructions(switchCase.Body)
		}
	}
	walkInstructions = func(instructions []ir.Instruction) {
		for _, inst := range instructions {
			walkInstruction(inst)
		}
	}
	walkInstructions(instructions)
	return diagnostics
}

func semaIRExpressionTypeReferenceStart(body string, pos int, typeName string) int {
	if pos < 0 || pos > len(body) {
		pos = 0
	}
	if typeName == "" {
		return pos
	}
	if idx := strings.Index(body[pos:], typeName); idx >= 0 {
		return pos + idx
	}
	return pos
}

func semaNormalizeMemberTypes(model *semaTypeMemberView, owner string, member typesys.MemberSymbol) typesys.MemberSymbol {
	member.Type = resolveNestedTypeReference(model, owner, member.Type)
	for i := range member.Parameters {
		member.Parameters[i].Type = resolveNestedTypeReference(model, owner, member.Parameters[i].Type)
	}
	return member
}

func (a *Analyzer) checkIRInstructions(typ typesys.TypeSymbol, member typesys.MemberSymbol, instructions []ir.Instruction, scope *irSemaScope, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, inst := range instructions {
		if inst.Expr.Kind != "" && inst.Op != ir.OpFor {
			diagnostics = append(diagnostics, a.checkIRExpressionContract(typ, member, inst.Expr, *scope, inst.Pos, bodyOffset, source, model)...)
		}
		switch inst.Op {
		case ir.OpDeclare:
			references := extractTypeNames(inst.Type)
			for _, ref := range references {
				resolved := resolveNestedTypeReference(model, typ.Name, ref)
				if message := semaAutomationTypeMessage(resolved, model); message != "" {
					diagnostics = append(diagnostics, semaAutomationDiagnostic(typ, message, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Type)), source))
					continue
				}
				if !a.hasKnown(resolved) || a.qualifiedLifecycleLocalTypeRejected(ref, resolved, model) {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA006", Message: "Invalid type: " + ref, File: typ.File, Range: semaRange(source, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Type)))})
				}
			}
			if message := semaAsyncTypeMessage(inst.Type); message != "" && semaAsyncPlatformType(model, resolveNestedTypeReference(model, typ.Name, inst.Type)) {
				diagnostics = append(diagnostics, semaAsyncDiagnostic(typ, message, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Type)), source))
			} else if (semaAPI67RejectedPlatformType(inst.Type) || semaPlatformTypeUnavailable(typ.EffectiveAPIVersion, inst.Type)) && !semaProjectTypeShadowsPlatform(model, inst.Type) {
				diagnostics = append(diagnostics, unsupportedLocalFeatureDiagnostic(typ, member, inst.Type, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Type)), source))
			}
			diagnostics = append(diagnostics, a.checkIRIncrementOrExprVariables(typ, member, inst, scope, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRAssignmentType(typ, member, inst.Type, inst.Name, inst.Expr, scope, inst.Pos, bodyOffset, source, model, "initializes")...)
			if !scope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type)) {
				diagnostics = append(diagnostics, irRedeclareDiagnostic(typ, member, inst.Name, inst.Pos, bodyOffset, source))
			} else if semaIRDeclaredFinal(source, bodyOffset+inst.Pos) {
				scope.markFinalLocal(inst.Name)
			}
		case ir.OpBlock:
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpDeclGroup:
			// Comma-separated locals share the enclosing scope.
			header := scope.incrementHeader
			scope.incrementHeader = true
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.incrementHeader = header
		case ir.OpAssign:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRAssignmentTarget(typ, member, inst.Name, *scope, inst.Pos, bodyOffset, source, model)...)
			if targetType, ok := irAssignmentTargetType(inst.Name, *scope, model, typ.Name); ok {
				diagnostics = append(diagnostics, a.checkIRAssignmentType(typ, member, targetType, inst.Name, inst.Expr, scope, inst.Pos, bodyOffset, source, model, "assigns")...)
			}
		case ir.OpReturn:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			returnType := strings.TrimSpace(member.Type)
			if returnType != "" && !strings.EqualFold(returnType, "void") {
				diagnostics = append(diagnostics, a.checkIRReturnType(typ, member, returnType, inst.Expr, scope, inst.Pos, bodyOffset, source, model)...)
			}
		case ir.OpExpr, ir.OpThrow, ir.OpDML, ir.OpRunAs:
			if inst.Op == ir.OpExpr {
				diagnostics = append(diagnostics, a.checkIRIncrementOrExprVariables(typ, member, inst, scope, bodyOffset, source, model, constructability)...)
			} else {
				diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			}
			if inst.Op == ir.OpDML {
				diagnostics = append(diagnostics, a.checkIRDMLContract(typ, member, inst, *scope, bodyOffset, source, model)...)
			}
			if inst.Op == ir.OpRunAs {
				argumentType := a.inferIRExprType(inst.Expr, *scope, model, typ.Name)
				if !semaRunAsArgumentAllowed(argumentType, model) {
					diagnostics = append(diagnostics, semaDMLDiagnostic(typ, "runAs requires a single argument of type 'User' or 'Version'", bodyOffset+inst.Pos, bodyOffset+inst.Pos+1, source))
				}
				scope.push()
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
				scope.pop()
			}
		case ir.OpIf:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRConditionType(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model)...)
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Else, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpWhile, ir.OpDoWhile:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRConditionType(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model)...)
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpFor:
			scope.push()
			inits := inst.Inits
			if len(inits) == 0 && inst.Init != nil {
				inits = []ir.Instruction{*inst.Init}
			}
			header := scope.incrementHeader
			scope.incrementHeader = true
			if len(inits) > 0 {
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inits, scope, bodyOffset, source, model, constructability)...)
			}
			scope.incrementHeader = header
			// The initializer establishes loop-local bindings before the condition
			// is checked, including locals that shadow a differently typed member.
			if inst.Expr.Kind != "" {
				diagnostics = append(diagnostics, a.checkIRExpressionContract(typ, member, inst.Expr, *scope, inst.Pos, bodyOffset, source, model)...)
			}
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRConditionType(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model)...)
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			updates := inst.Updates
			if len(updates) == 0 && inst.Update != nil {
				updates = []ir.Instruction{*inst.Update}
			}
			if len(updates) > 0 {
				scope.incrementHeader = true
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, updates, scope, bodyOffset, source, model, constructability)...)
				scope.incrementHeader = header
			}
			scope.pop()
		case ir.OpForEach:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRForEachType(typ, member, inst, *scope, bodyOffset, source, model)...)
			scope.push()
			if !scope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type)) {
				diagnostics = append(diagnostics, irRedeclareDiagnostic(typ, member, inst.Name, inst.Pos, bodyOffset, source))
			}
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpTry:
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
			for _, catchClause := range catchClauses(inst) {
				scope.push()
				if catchClause.Name != "" {
					catchType := "Exception"
					if len(catchClause.Types) > 0 {
						catchType = catchClause.Types[0]
					}
					if !scope.declare(catchClause.Name, catchType) {
						diagnostics = append(diagnostics, irRedeclareDiagnostic(typ, member, catchClause.Name, catchClause.Pos, bodyOffset, source))
					}
				}
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, catchClause.Body, scope, bodyOffset, source, model, constructability)...)
				scope.pop()
			}
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Finally, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpSwitch:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			selectorType := a.inferIRExprType(inst.Expr, *scope, model, typ.Name)
			for _, switchCase := range inst.Cases {
				var typeBindings []struct {
					name string
					typ  string
				}
				for _, expr := range switchCase.Exprs {
					if caseType, binding, ok := irSwitchTypeCase(expr); ok {
						typeBindings = append(typeBindings, struct {
							name string
							typ  string
						}{name: binding, typ: caseType})
						continue
					}
					// V001-V004: every enum label resolves against the selector,
					// including labels following a comma in the same when clause.
					if semaSwitchSelectorEnumCaseType(selectorType, expr, model) != "" {
						continue
					}
					diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, expr, scope, switchCase.Pos, bodyOffset, source, model, constructability)...)
				}
				scope.push()
				for _, binding := range typeBindings {
					if !scope.declare(binding.name, binding.typ) {
						diagnostics = append(diagnostics, irRedeclareDiagnostic(typ, member, binding.name, switchCase.Pos, bodyOffset, source))
					}
				}
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, switchCase.Body, scope, bodyOffset, source, model, constructability)...)
				scope.pop()
			}
		}
	}
	return diagnostics
}

// R201/C055/C056 and N005-N008/N010-N011, including named controls: a qualified local
// type must resolve to a real namespace type. Built-in value/collection types
// captured as String, Integer and List cannot be qualified with System here;
// qualified call receivers still can. Other built-ins retain their current rules.
func (a *Analyzer) qualifiedLifecycleLocalTypeRejected(spelling, resolved string, model *semaTypeMemberView) bool {
	root, localName, qualified := strings.Cut(spelling, ".")
	if !qualified {
		return false
	}
	if members, known := model.lookupName(spelling); known && !members.platform && !members.dependency {
		return false
	}
	if strings.EqualFold(root, "System") && (strings.EqualFold(localName, "String") || strings.EqualFold(localName, "Integer") || strings.EqualFold(localName, "List")) {
		return true
	}
	if _, knownRoot := a.known[a.canonicalName(root)]; knownRoot {
		return false
	}
	return semaUnknownExternalType(model, resolved) && !a.hasExternalDependencyName(resolved)
}

func (a *Analyzer) checkIRDMLContract(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope irSemaScope, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	operands := []ir.Expr{inst.Expr}
	if strings.EqualFold(inst.Name, "merge") && inst.Expr.Kind == ir.ExprCall && len(inst.Expr.Args) >= 2 {
		operands = inst.Expr.Args[:2]
	}
	operandTypes := make([]string, len(operands))
	mergeIDDuplicates := false
	for i, operand := range operands {
		operandTypes[i] = a.inferIRExprType(operand, scope, model, typ.Name)
		if i == 1 && strings.EqualFold(inst.Name, "merge") && len(operandTypes) == 2 {
			mergeIDDuplicates = semaDMLMergeIDDuplicateTypesCompatible(operandTypes[0], operandTypes[1], model)
		}
		if !semaDMLTargetType(operandTypes[i], model) && !(i == 1 && mergeIDDuplicates) {
			return []diagnostic.Diagnostic{irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, "DML requires SObject or SObject list type: "+operandTypes[i])}
		}
		// Typed events cannot be updated or
		// deleted. Generic SObject operands retain their existing DML path.
		if semaPlatformEventType(semaDMLObjectType(operandTypes[i]), model) {
			operation := ""
			switch strings.ToLower(inst.Name) {
			case "update":
				operation = "Update"
			case "delete":
				operation = "Delete"
			}
			if operation != "" {
				message := "DML operation " + operation + " not allowed on " + operandTypes[i]
				d := irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, message)
				d.NativeMessage = message
				return []diagnostic.Diagnostic{d}
			}
		}
	}
	if strings.EqualFold(inst.Name, "merge") && len(operandTypes) > 0 {
		objectType := semaDMLObjectType(operandTypes[0])
		switch normalizeName(objectType) {
		case "account", "contact", "lead", "sobject":
		default:
			return []diagnostic.Diagnostic{irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, "Specified type "+objectType+" cannot be merged")}
		}
	}
	if strings.EqualFold(inst.Name, "merge") && len(operandTypes) == 2 && !semaDMLMergeTypesCompatible(operandTypes[0], operandTypes[1], model) {
		return []diagnostic.Diagnostic{irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, "merge requires master and duplicate operands of the same SObject type")}
	}
	if strings.EqualFold(inst.Name, "upsert") && inst.Field != "" && !a.semaUpsertSelectorAllowed(operandTypes[0], inst.Field) {
		field := inst.Field
		if _, name, qualified := strings.Cut(field, "."); qualified {
			field = name
		}
		return []diagnostic.Diagnostic{irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, "Invalid field for upsert, must be an External Id custom or standard indexed field: "+field)}
	}
	return nil
}

func (a *Analyzer) semaUpsertSelectorAllowed(operandType, selector string) bool {
	objectName := semaDMLObjectType(operandType)
	selectorObject, fieldName, qualified := strings.Cut(strings.TrimSpace(selector), ".")
	if objectName == "" {
		return false
	}
	if !qualified {
		fieldName = selectorObject
	} else if !semaProjectReferencedSchemaAPINamesMatch(a.namespace, selectorObject, objectName) {
		return false
	}
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		return false
	}
	fieldSchemaFound := false
	objectSchemaFound := false
	for _, object := range a.queryDeclaredObjects {
		if !semaProjectReferencedSchemaAPINamesMatch(a.namespace, object.Name, objectName) {
			continue
		}
		objectSchemaFound = objectSchemaFound || len(object.Fields) > 0
		for _, field := range object.Fields {
			if semaProjectReferencedSchemaAPINamesMatch(a.namespace, field.Name, fieldName) {
				fieldSchemaFound = true
				if field.ExternalID || field.IDLookup {
					return true
				}
			}
		}
	}
	if provider := semaStandardSObjectFieldProviderFor(objectName); provider != nil && provider.hasFields() {
		objectSchemaFound = true
		field, ok := provider.lookup(fieldName)
		if ok {
			fieldSchemaFound = true
			if field.ExternalID || field.IDLookup {
				return true
			}
		}
	}
	if fieldSchemaFound || objectSchemaFound {
		return false
	}
	// Do not infer a selector capability from a field name. An unavailable
	// describe source remains unknown rather than a synthetic rejection.
	return true
}

func semaDMLObjectType(typeName string) string {
	if base, args := semaGenericBaseAndArgs(typeName); (strings.EqualFold(base, "List") || strings.EqualFold(base, "Set")) && len(args) == 1 {
		return strings.TrimSpace(args[0])
	}
	return strings.TrimSpace(typeName)
}

func semaDMLTargetType(typeName string, model *semaTypeMemberView) bool {
	typeName = strings.TrimSpace(typeName)
	if semaDMLRecordType(typeName, model) {
		return true
	}
	base, args := semaGenericBaseAndArgs(typeName)
	return (strings.EqualFold(base, "List") || strings.EqualFold(base, "Set")) && len(args) == 1 && semaDMLRecordType(args[0], model)
}

func semaDMLMergeTypesCompatible(left, right string, model *semaTypeMemberView) bool {
	if !semaDMLRecordType(left, model) {
		return false
	}
	if semaDMLMergeIDDuplicateTypesCompatible(left, right, model) {
		return true
	}
	rightObject := right
	if !semaDMLRecordType(right, model) {
		base, args := semaGenericBaseAndArgs(right)
		if !strings.EqualFold(base, "List") || len(args) != 1 || !semaDMLRecordType(args[0], model) {
			return false
		}
		rightObject = args[0]
	}
	return strings.EqualFold(normalizeName(left), normalizeName(rightObject))
}

func semaDMLMergeIDDuplicateTypesCompatible(master, duplicates string, model *semaTypeMemberView) bool {
	if !semaDMLConcreteRecordType(master, model) {
		return false
	}
	base, args := semaGenericBaseAndArgs(duplicates)
	return strings.EqualFold(base, "List") && len(args) == 1 && strings.EqualFold(strings.TrimSpace(args[0]), "Id")
}

func semaDMLConcreteRecordType(typeName string, model *semaTypeMemberView) bool {
	typeName = strings.TrimSpace(typeName)
	if schemaName, ok := semaSchemaQualifiedTypeName(typeName); ok {
		typeName = schemaName
	}
	return !strings.EqualFold(typeName, "SObject") && semaDMLRecordType(typeName, model)
}

func semaDMLRecordType(typeName string, model *semaTypeMemberView) bool {
	typeName = strings.TrimSpace(typeName)
	if schemaName, ok := semaSchemaQualifiedTypeName(typeName); ok {
		typeName = schemaName
	}
	return !strings.EqualFold(typeName, "AggregateResult") && isSemaSObjectLike(typeName, model)
}

func irDMLContractDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, bodyOffset int, source, message string) diagnostic.Diagnostic {
	start := bodyOffset + inst.Pos
	return semaDMLDiagnostic(typ, message, start, start+max(1, len(inst.Name)), source)
}

func irSwitchTypeCase(expr ir.Expr) (string, string, bool) {
	if expr.Kind != ir.ExprVariable || !strings.HasPrefix(expr.Name, "__typecase:") {
		return "", "", false
	}
	rest := strings.TrimPrefix(expr.Name, "__typecase:")
	typeName, binding, ok := strings.Cut(rest, ":")
	return typeName, binding, ok && typeName != "" && binding != ""
}

func irInstructionsTerminate(instructions []ir.Instruction) bool {
	for _, inst := range instructions {
		if irInstructionTerminates(inst) {
			return true
		}
	}
	return false
}

func irInstructionTerminates(inst ir.Instruction) bool {
	switch inst.Op {
	case ir.OpReturn, ir.OpThrow:
		return true
	case ir.OpBlock, ir.OpDeclGroup:
		return irInstructionsTerminate(inst.Then)
	case ir.OpIf:
		return len(inst.Then) > 0 && len(inst.Else) > 0 && irInstructionsTerminate(inst.Then) && irInstructionsTerminate(inst.Else)
	case ir.OpTry:
		if irInstructionsTerminate(inst.Finally) {
			return true
		}
		if !irInstructionsTerminate(inst.Then) {
			return false
		}
		clauses := catchClauses(inst)
		for _, catchClause := range clauses {
			if !irInstructionsTerminate(catchClause.Body) {
				return false
			}
		}
		return true
	case ir.OpSwitch:
		hasElse := false
		if len(inst.Cases) == 0 {
			return false
		}
		for _, switchCase := range inst.Cases {
			if switchCase.Else {
				hasElse = true
			}
			if !irInstructionsTerminate(switchCase.Body) {
				return false
			}
		}
		return hasElse
	default:
		return false
	}
}

func irRedeclareDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, name string, pos, bodyOffset int, source string) diagnostic.Diagnostic {
	start := bodyOffset + pos
	end := start + len(name)
	if end > len(source) {
		end = len(source)
	}
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA014",
		Message:  fmt.Sprintf("%s %q redeclares local variable %q in the same scope", member.Kind, member.Name, name),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func catchClauses(inst ir.Instruction) []ir.CatchClause {
	if len(inst.Catches) > 0 {
		return inst.Catches
	}
	if len(inst.Catch) == 0 {
		return nil
	}
	return []ir.CatchClause{{Types: catchTypes(inst), Name: inst.Name, Body: inst.Catch, Pos: inst.Pos}}
}

func catchTypes(inst ir.Instruction) []string {
	if len(inst.CatchTypes) > 0 {
		return inst.CatchTypes
	}
	if inst.Type == "" {
		return nil
	}
	return []string{inst.Type}
}

func (a *Analyzer) checkIRExprVariables(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	if expr.Kind == "" {
		return nil
	}
	if diag, rejected := semaVisualforceSeverityDiagnostic(typ, expr, *scope, model, bodyOffset+pos, source); rejected {
		return []diagnostic.Diagnostic{diag}
	}
	if diag, rejected := a.irStandardControllerRecordMemberDiagnostic(typ, expr, *scope, model, bodyOffset+pos, source); rejected {
		return []diagnostic.Diagnostic{diag}
	}
	var diagnostics []diagnostic.Diagnostic
	switch expr.Kind {
	case ir.ExprVariable:
		if semaExprAtSwitchWhenLabel(source, bodyOffset+pos, expr.Name) {
			return nil
		}
		if message := a.securityAccessFieldMessage(expr.Name, *scope, model); message != "" {
			return []diagnostic.Diagnostic{semaDMLDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Name)), source)}
		}
		fieldReceiver := expr.Name
		fieldPath := expr.Name
		if strings.HasSuffix(strings.ToLower(expr.Name), ".class") {
			if hidden := semaHTTPReferencedInvisibleType(expr.Name, model); hidden != "" {
				return []diagnostic.Diagnostic{semaHTTPDiagnostic(typ, "Type is not visible: "+hidden, bodyOffset+pos, bodyOffset+pos+len(expr.Name), source)}
			}
		}
		if dot := strings.LastIndexByte(fieldReceiver, '.'); dot > 0 {
			fieldReceiver = fieldReceiver[:dot]
			if receiverType, ok := scope.lookup(fieldReceiver); ok {
				fieldPath = receiverType + expr.Name[dot:]
				fieldReceiver = receiverType
			}
		}
		if message := semaAsyncFieldMessage(fieldPath); message != "" && !semaProjectTypeShadowsPlatform(model, fieldReceiver) {
			return []diagnostic.Diagnostic{semaAsyncDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Name)), source)}
		}
		if (semaAPI67RejectedPlatformField(fieldPath) || semaPlatformFieldPathUnavailable(typ.EffectiveAPIVersion, fieldPath)) && !semaProjectTypeShadowsPlatform(model, fieldReceiver) {
			if field := semaHTTPFieldName(fieldPath, false); field != "" {
				return []diagnostic.Diagnostic{semaHTTPDiagnostic(typ, "Variable is not visible: "+field, bodyOffset+pos, bodyOffset+pos+len(expr.Name), source)}
			}
			return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, expr.Name, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Name)), source)}
		}
		if diag, ok := a.irVariableDiagnostic(typ, member, expr.Name, *scope, model, bodyOffset+pos, source, true); ok {
			diagnostics = append(diagnostics, diag)
		} else if !a.irVariableKnown(expr.Name, *scope, model, typ.Name) && (scope.anonymous || !strings.Contains(expr.Name, ".") || !isLikelyTypeReference(expr.Name)) {
			message := fmt.Sprintf("%s %q reads unknown variable %q", member.Kind, member.Name, expr.Name)
			if scope.anonymous {
				message = "Variable does not exist: " + expr.Name
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA013",
				Message:  message,
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Name))),
			})
		}
	case ir.ExprCall:
		if expr.Left != nil {
			diagnostics = append(diagnostics, a.checkIRHTTPReceiverCalls(typ, member, *expr.Left, *scope, pos, bodyOffset, source, model, constructability)...)
		}
		// A fluent call still has to validate the constructor
		// and calls in its receiver; inferring their return type is not enough.
		if expr.Left != nil && expr.Left.Kind == ir.ExprCall {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, *expr.Left, scope, pos, bodyOffset, source, model, constructability)...)
		}
		if lastDot := strings.LastIndex(expr.Callee, "."); lastDot > 0 && lastDot < len(expr.Callee)-1 {
			typeName := expr.Callee[:lastDot]
			memberName := expr.Callee[lastDot+1:]
			if strings.EqualFold(typeName, "Search") && !scope.hasNonFieldBinding(typeName) && !strings.EqualFold(memberName, "query") && !strings.EqualFold(memberName, "find") && !strings.EqualFold(memberName, "suggest") {
				return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, expr.Callee+" local search/SOSL surface", bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
			}
		}
		for _, arg := range expr.Args {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, arg, scope, pos, bodyOffset, source, model, constructability)...)
		}
		for _, arg := range expr.NamedArgs {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, arg.Expr, scope, pos, bodyOffset, source, model, constructability)...)
		}
		diagnostics = append(diagnostics, a.checkIRCall(typ, member, expr, *scope, pos, bodyOffset, source, model, constructability)...)
	case ir.ExprUnary:
		if expr.Left != nil {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, *expr.Left, scope, pos, bodyOffset, source, model, constructability)...)
		}
	case ir.ExprBinary:
		if expr.Left != nil && expr.Right != nil {
			left := a.inferIRExprType(*expr.Left, *scope, model, typ.Name)
			right := a.inferIRExprType(*expr.Right, *scope, model, typ.Name)
			message := ""
			switch expr.Operator {
			case "instanceof":
				right = expr.Right.Name
				if strings.EqualFold(left, "AggregateResult") && strings.EqualFold(right, "SObject") && !semaProjectTypeShadowsPlatform(model, left) {
					// Retain the native text without the generic
					// source-contract prefix, in anonymous and named code.
					nativeMessage := "Operation instanceof is always true since an instance of AggregateResult is always an instance of SObject"
					diag := typeContractDiagnostic(typ, member, nativeMessage, bodyOffset+pos, bodyOffset+pos+1, source)
					diag.Message = nativeMessage
					diagnostics = append(diagnostics, diag)
				} else if strings.EqualFold(left, "Exception") && strings.EqualFold(right, "Exception") {
					// Preserve canonical R084's structured exception diagnostic.
					d := typeContractDiagnostic(typ, member, "instanceof is always true for the declared type "+left, bodyOffset+pos, bodyOffset+pos+1, source)
					d.NativeMessage = "Operation instanceof is always true since an instance of Exception is always an instance of Exception"
					diagnostics = append(diagnostics, d)
				} else if native := semaInstanceofAlwaysTrueMessage(left, right, model); native != "" {
					diagnostics = append(diagnostics, typeContractNativeDiagnostic(typ, native, bodyOffset+pos, bodyOffset+pos+1, source))
				}
			case "==", "!=":
				// Describe shorthand is a result, not the corresponding token
				// at both API endpoints.
				if shorthand := semaSchemaDescribeShorthandType(*expr.Left, *scope, model); shorthand != "" {
					left = shorthand
				}
				if shorthand := semaSchemaDescribeShorthandType(*expr.Right, *scope, model); shorthand != "" {
					right = shorthand
				}
				if semaSchemaDescribeTokenComparison(left, right) && !semaProjectTypeShadowsPlatform(model, left) && !semaProjectTypeShadowsPlatform(model, right) {
					message = "Comparison arguments must be compatible types: " + left + ", " + right
				}
			case "===", "!==":
				// Native R005/R006 accept Object/String in either order;
				// C005-C007 still reject homogeneous Strings at both APIs.
				mixedObjectString := expr.Operator == "===" && (strings.EqualFold(left, "Object") && strings.EqualFold(right, "String") ||
					strings.EqualFold(left, "String") && strings.EqualFold(right, "Object"))
				if expr.Operator == "===" && strings.EqualFold(left, "String") && strings.EqualFold(right, "String") {
					diagnostics = append(diagnostics, typeContractNativeDiagnostic(typ, "Exact equality operator only allowed for reference types: String", bodyOffset+pos, bodyOffset+pos+1, source))
				} else if !mixedObjectString && (isSemaNumericType(left) || isSemaNumericType(right) || strings.EqualFold(left, "Boolean") || strings.EqualFold(right, "Boolean") || strings.EqualFold(left, "String") || strings.EqualFold(right, "String")) {
					message = "exact equality is only allowed for reference types"
				}
			case "+", "-", "*", "/":
				if (left == "null" || right == "null") && left != "String" && right != "String" {
					message = "arithmetic expressions require numeric operands"
				}
			case "&", "|", "^":
				if left != "" && right != "" && !(isSemaIntegralType(left) && isSemaIntegralType(right)) && !(left == "Boolean" && right == "Boolean") {
					message = "bitwise operators require two integral or two Boolean operands"
				}
			}
			if message != "" {
				d := typeContractDiagnostic(typ, member, message, bodyOffset+pos, bodyOffset+pos+1, source)
				if (expr.Operator == "===" || expr.Operator == "!==") && left == "Integer" && right == "Integer" {
					d.NativeMessage = "Exact equality operator only allowed for reference types: Integer"
				}
				if (expr.Operator == "+" || expr.Operator == "-" || expr.Operator == "*" || expr.Operator == "/") && (left == "null" || right == "null") {
					d.NativeMessage = "Arithmetic expressions must use numeric arguments"
				}
				diagnostics = append(diagnostics, d)
			}
		}
		if expr.Left != nil {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, *expr.Left, scope, pos, bodyOffset, source, model, constructability)...)
		}
		if expr.Right != nil && !strings.EqualFold(expr.Operator, "instanceof") {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, *expr.Right, scope, pos, bodyOffset, source, model, constructability)...)
		}
	case ir.ExprSOQL:
		return nil
	}
	return diagnostics
}

func semaSchemaDescribeShorthandType(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView) string {
	if expr.Kind != ir.ExprVariable {
		return ""
	}
	parts := strings.Split(expr.Name, ".")
	if scope.hasNonFieldBinding(parts[0]) || semaProjectTypeShadowsPlatform(model, parts[0]) {
		return ""
	}
	if len(parts) > 0 && strings.EqualFold(parts[0], "Schema") {
		parts = parts[1:]
	}
	if len(parts) == 2 && strings.EqualFold(parts[0], "SObjectType") {
		return "Schema.DescribeSObjectResult"
	}
	if len(parts) == 4 && strings.EqualFold(parts[0], "SObjectType") && strings.EqualFold(parts[2], "fields") {
		return "Schema.DescribeFieldResult"
	}
	return ""
}

func semaSchemaDescribeTokenComparison(left, right string) bool {
	for _, pair := range [][2]string{
		{"Schema.DescribeSObjectResult", "Schema.SObjectType"},
		{"Schema.DescribeFieldResult", "Schema.SObjectField"},
	} {
		if strings.EqualFold(left, pair[0]) && strings.EqualFold(right, pair[1]) ||
			strings.EqualFold(right, pair[0]) && strings.EqualFold(left, pair[1]) {
			return true
		}
	}
	return false
}

func (a *Analyzer) checkIRAssignmentTarget(typ typesys.TypeSymbol, member typesys.MemberSymbol, name string, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	if field, ok := semaTriggerContextField(name, scope, model); ok && semaKnownTriggerContextField(field) {
		// Context properties themselves cannot be assigned.
		return []diagnostic.Diagnostic{semaTriggerDiagnostic(typ, "Expression cannot be assigned", bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
	}
	if strings.Contains(name, "?.") {
		return []diagnostic.Diagnostic{typeContractDiagnostic(typ, member, "safe navigation cannot be an assignment target", bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
	}
	if !strings.Contains(name, ".") && scope.hasNonFieldBinding(name) {
		return nil
	}
	if !strings.Contains(name, ".") {
		if field, resolved := semaResolveFieldPath(model, typ.Name, name); resolved {
			if d, rejected := semaInstanceFinalFieldAssignmentDiagnostic(typ, member, field, true, bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source); rejected {
				return []diagnostic.Diagnostic{d}
			}
		}
	}
	if !strings.Contains(name, ".") {
		if _, ok := scope.lookup(name); !ok {
			return []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: "Variable does not exist: " + name, File: typ.File, Range: semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(name)))}}
		}
	}
	if root, field, ok := strings.Cut(name, "."); ok {
		if receiverType := semaIRReceiverType(root, scope, model, typ.Name); receiverType != "" {
			if message := semaPlatformEventWriteMessage(receiverType, field, model); message != "" {
				return []diagnostic.Diagnostic{semaPlatformEventDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
			}
			if fieldName, readOnly := semaFormulaReadOnlyField(model, receiverType, field); readOnly {
				item := semaFieldAccessDiagnostic(typ, member, name, "field is not writeable", bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)
				item.Message = "Field is not writeable: " + fieldName
				return []diagnostic.Diagnostic{item}
			}
			if !semaProjectTypeShadowsPlatform(model, receiverType) {
				if fieldName := semaHTTPFieldName(receiverType+"."+field, true); fieldName != "" {
					return []diagnostic.Diagnostic{semaHTTPDiagnostic(typ, "Variable is not visible: "+fieldName, bodyOffset+pos, bodyOffset+pos+len(name), source)}
				}
			}
			if semaStandardFieldAssignmentReadOnly(model, receiverType, field) {
				return []diagnostic.Diagnostic{semaFieldAccessDiagnostic(typ, member, name, "field is not writeable", bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
			}
			if target, resolved := semaResolveFieldPath(model, receiverType, field); resolved {
				if d, rejected := semaInstanceFinalFieldAssignmentDiagnostic(typ, member, target, strings.EqualFold(root, "this"), bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source); rejected {
					return []diagnostic.Diagnostic{d}
				}
				if target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyAssignmentAllowed(typ, member, target, false, semaReceiverExprLooksLikeType(name[:strings.LastIndex(name, ".")], scope, model), model) {
					return []diagnostic.Diagnostic{typeContractPropertyAssignmentDiagnostic(typ, member, target, false, bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
				}
				if !semaProjectTypeShadowsPlatform(model, receiverType) && semaAPI67ReadOnlyPlatformField(target.owner+"."+target.member.Name) {
					return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, name, bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
				}
			}
		}
	}
	if target, ok := semaResolveFieldPath(model, typ.Name, name); ok && target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyAssignmentAllowed(typ, member, target, !strings.Contains(name, "."), false, model) {
		return []diagnostic.Diagnostic{typeContractPropertyAssignmentDiagnostic(typ, member, target, !strings.Contains(name, "."), bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
	}
	if diag, ok := a.irVariableDiagnostic(typ, member, name, scope, model, bodyOffset+pos, source, false); ok {
		return []diagnostic.Diagnostic{diag}
	}
	return nil
}

func semaExprAtSwitchWhenLabel(source string, pos int, name string) bool {
	if pos < 0 || pos > len(source) {
		return false
	}
	lineStart := strings.LastIndexAny(source[:pos], "\r\n") + 1
	lineEnd := pos
	for lineEnd < len(source) && source[lineEnd] != '\n' && source[lineEnd] != '\r' {
		lineEnd++
	}
	line := source[lineStart:lineEnd]
	label := strings.ToLower(strings.TrimSpace(name))
	lowerLine := strings.ToLower(line)
	if strings.Contains(lowerLine, "when "+label+" {") || strings.Contains(lowerLine, "when "+label+",") {
		return true
	}
	start := pos - 1
	for start >= 0 && isWhitespace(source[start]) {
		start--
	}
	for start >= 0 && isIdentifierByte(source[start]) {
		start--
	}
	prefix := strings.TrimSpace(source[max(0, start-8):pos])
	if !strings.HasSuffix(strings.ToLower(prefix), "when") {
		return false
	}
	end := pos + len(name)
	for end < len(source) && isWhitespace(source[end]) {
		end++
	}
	return end < len(source) && (source[end] == '{' || source[end] == ',')
}

func (a *Analyzer) checkIRCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	// Schema.SObjectType.<object> exposes a describe result, not the token
	// returned by <object>.SObjectType or getGlobalDescribe().get(...).
	if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok && strings.EqualFold(method, "newSObject") && !scope.hasNonFieldBinding("Schema") {
		parts := strings.Split(receiver, ".")
		if len(parts) == 3 && strings.EqualFold(parts[0], "Schema") && strings.EqualFold(parts[1], "SObjectType") && isSemaSObjectLike(parts[2], model) {
			message := fmt.Sprintf("Method does not exist or incorrect signature: void newSObject(%s) from the type Schema.DescribeSObjectResult", strings.Join(irCallArgTypes(a, expr.Args, scope, model, typ.Name), ","))
			d := collectionCallDiagnostic(typ, member, method, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)
			d.Message, d.NativeMessage = message, message
			return []diagnostic.Diagnostic{d}
		}
	}
	if receiver, _, ok := splitSemaMethodPath(expr.Callee); ok {
		if item, rejected := a.pageReferenceDiagnostic(typ, receiver, scope, model, bodyOffset+pos, source); rejected {
			return []diagnostic.Diagnostic{item}
		}
	}
	if message := semaAutomationChainedTypeMessage(expr, scope, model); message != "" {
		return []diagnostic.Diagnostic{semaAutomationDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	if message := a.securityAccessCallMessage(expr, scope, model, typ.Name); message != "" {
		return []diagnostic.Diagnostic{semaDMLDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	if strings.HasPrefix(expr.Callee, "newlit:") {
		typeName := strings.TrimPrefix(expr.Callee, "newlit:")
		diagnostics, _ := a.checkIRCollectionConstructor(typ, member, typeName, expr.Args, scope, pos, bodyOffset, source, model)
		// Collection row C001 is a literal initializer, not a constructor overload.
		if typeName == "List<Integer>" && len(diagnostics) == 1 && len(expr.Args) == 1 && a.inferIRExprType(expr.Args[0], scope, model, typ.Name) == "String" {
			diagnostics[0].NativeMessage = "Initial expression is of incorrect type, expected: Integer but was: String"
		}
		return diagnostics
	}
	if strings.HasPrefix(expr.Callee, "new:") {
		return a.checkIRConstructorCall(typ, member, expr, scope, pos, bodyOffset, source, model, constructability)
	}
	if strings.HasPrefix(expr.Callee, "__assign:") {
		return a.checkIRAssignmentTarget(typ, member, strings.TrimPrefix(expr.Callee, "__assign:"), scope, pos, bodyOffset, source, model)
	}
	if diag, rejected := a.semaVisualforceSetControllerField(typ, expr, scope, model, bodyOffset+pos, source); rejected {
		return []diagnostic.Diagnostic{diag}
	}
	if strings.HasPrefix(expr.Callee, "__field:") ||
		strings.HasPrefix(expr.Callee, "__safe_field:") ||
		strings.HasPrefix(expr.Callee, "__assignField:") ||
		strings.HasPrefix(expr.Callee, "newlit:") ||
		strings.HasPrefix(expr.Callee, "__newArray:") ||
		strings.HasPrefix(expr.Callee, "__cast:") ||
		expr.Callee == "__ternary" {
		return nil
	}
	// System.debug normally bypasses semantic call checks. C027/C031 need
	// its real overloads, and qualified static receivers need the same gate.
	if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok && semaGovernorCallCandidate(receiver, method, model) {
		// S001-S004: value bindings, including fields, take precedence over
		// platform type names. Resolve them before checking static contracts.
		receiverType, receiverMode := "", "instance"
		if scoped, ok := scope.lookup(receiver); ok {
			receiverType = scoped
		} else if field, ok := semaResolveFieldPath(model, typ.Name, receiver); ok {
			receiverType = field.member.Type
		} else {
			receiverType = semaIRReceiverType(receiver, scope, model, typ.Name)
			receiverMode = "class"
		}
		if semaGovernorCallCandidate(receiverType, method, model) {
			if message := semaGovernorCallMessage(typ.EffectiveAPIVersion, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, receiverMode); message != "" {
				return []diagnostic.Diagnostic{semaGovernorDiagnostic(typ, "GLADESEMA009", message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
			}
		}
	}
	if expr.Callee == "" || expr.Callee == "this" || expr.Callee == "super" || skipSemaCall(expr.Callee) {
		// These native comparison diagnostics must precede
		// the assertEquals skip; Object arguments and shadowed receivers stay
		// on their existing paths (J082-J084).
		if strings.EqualFold(expr.Callee, "System.assertEquals") && len(expr.Args) == 2 {
			if _, bound := scope.lookup("System"); !bound {
				receiverType := resolveNestedTypeName(model, typ.Name, "System")
				if d, rejected := semaTestCallDiagnostic(typ, receiverType, "assertEquals", irCallArgTypes(a, expr.Args, scope, model, typ.Name), bodyOffset+pos, bodyOffset+pos+len(expr.Callee), source, model); rejected {
					return []diagnostic.Diagnostic{d}
				}
			}
		}
		return nil
	}
	// The qualified namespace receiver can exit through the
	// permissive dotted-name path before the ordinary DML contract check.
	if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok && strings.EqualFold(receiver, "Datacloud.FindDuplicates") {
		root, _, _ := strings.Cut(receiver, ".")
		_, scoped := scope.lookup(root)
		rootMembers, rootType := model.lookup(normalizeName(root))
		members, projectType := model.lookup(normalizeName(receiver))
		if !scoped && (!rootType || rootMembers.dependency) && (!projectType || members.dependency) {
			if message := a.semaDMLCallMessage(receiver, method, expr.Args, scope, model, typ.Name); message != "" {
				return []diagnostic.Diagnostic{semaDMLDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
			}
		}
	}
	if d, rejected := a.semaIRShadowReceiverMissingMethod(typ, member, expr, scope, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
		return []diagnostic.Diagnostic{d}
	}
	// C034's named control declares the enum inside the test class. Resolve
	// this static enum receiver before a bare-name fallback can accept it.
	if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok && !scope.hasNonFieldBinding(receiver) && (strings.EqualFold(method, "values") || strings.EqualFold(method, "valueOf")) {
		receiverType := resolveNestedTypeName(model, typ.Name, receiver)
		if members, ok := model.lookupName(receiverType); ok && members.kind == apexast.DeclarationEnum && !members.platform {
			if sig, ok := semaEnumMethodSignature(model, receiverType, method); ok {
				argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
				if !semaArgsMatchAny(sig.params, argTypes, model) {
					d := collectionCallDiagnostic(typ, member, method, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)
					d.NativeMessage = fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(argTypes, ","), receiverType)
					return []diagnostic.Diagnostic{d}
				}
				return nil
			}
		}
	}
	// Keep this pre-resolution guard for rejected qualified platform receivers.
	// The later platform path is shared, but unknown dotted receivers can exit
	// through permissive fallback before it is reached.
	if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok && !scope.hasNonFieldBinding(receiver) && !semaProjectTypeShadowsPlatform(model, receiver) {
		if message := semaPlatformEventCallMessage(receiver, method, "class", irCallArgTypes(a, expr.Args, scope, model, typ.Name), model); message != "" {
			return []diagnostic.Diagnostic{semaPlatformEventDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
		}
		root, _, _ := strings.Cut(receiver, ".")
		if _, scoped := scope.lookup(root); !scoped {
			if message := a.semaAutomationCallMessage(receiver, method, expr.Args, scope, model, typ.Name); message != "" {
				return []diagnostic.Diagnostic{semaAutomationDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
			}
		}
		if d, rejected := semaMessagingMethodDiagnostic(typ, receiver, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, bodyOffset+pos, bodyOffset+pos+len(expr.Callee), source); rejected {
			return []diagnostic.Diagnostic{d}
		}
		if hidden := semaHTTPReferencedInvisibleType(receiver, model); hidden != "" {
			return []diagnostic.Diagnostic{semaHTTPDiagnostic(typ, "Type is not visible: "+hidden, bodyOffset+pos, bodyOffset+pos+len(expr.Callee), source)}
		}
		if semaAsyncCallCandidate(receiver, method) {
			if message := semaAsyncCallMessage(receiver, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name)); message != "" {
				return []diagnostic.Diagnostic{semaAsyncDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
			}
		}
		if semaAPI67RejectedPlatformCallAtVersion(typ.EffectiveAPIVersion, receiver, method, "class") || (strings.EqualFold(method, "validateKeys") && semaAPI67RejectedPlatformCallArgs(typ.EffectiveAPIVersion, receiver, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name))) {
			return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, receiver+"."+method, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
		}
		// C033/C035: namespaced platform types can reach the dotted fallback
		// before ordinary call validation. A bound namespace is a value receiver
		// instead (K080/K081), even for anonymous root declarations.
		if _, reports := semaReportsTypeName(receiver, model); reports {
			root, _, _ := strings.Cut(receiver, ".")
			_, bound := scope.lookup(root)
			rootType := resolveNestedTypeName(model, typ.Name, root)
			if rootMembers, ok := model.lookup(normalizeName(rootType)); ok && !rootMembers.platform {
				bound = true
			}
			if !bound {
				if d, rejected := semaReportsCallDiagnostic(typ, member, receiver, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), "class", bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
					return []diagnostic.Diagnostic{d}
				}
			}
		}
	}
	receiverType := typ.Name
	method := expr.Callee
	explicitReceiver := false
	classLiteralReceiver := false
	platformClassReceiver := false
	projectClassReceiver := false
	if expr.Left != nil {
		explicitReceiver = true
		if inferred := a.inferIRExprType(*expr.Left, scope, model, typ.Name); inferred != "" {
			receiverType = inferred
		} else {
			return nil
		}
	}
	if receiver, callee, ok := strings.Cut(expr.Callee, "."); ok {
		explicitReceiver = true
		method = callee
		if receiverExpr, methodName, ok := splitSemaMethodPath(expr.Callee); ok {
			_, receiverScoped := scope.lookup(receiverExpr)
			if !receiverScoped && semaKnownPlatformTypeReceiver(receiverExpr) && !semaProjectTypeShadowsPlatform(model, receiverExpr) {
				if _, ok := semaPlatformMethodSignatureForMode(model, receiverExpr, methodName, "class"); ok {
					receiverType = receiverExpr
					method = methodName
					platformClassReceiver = true
				}
			}
		}
		if !platformClassReceiver {
			if classMethod, ok := semaClassLiteralMethod(expr.Callee); ok {
				receiverType = "Type"
				method = classMethod
				classLiteralReceiver = true
			} else {
				switch {
				case strings.EqualFold(receiver, "this"):
					receiverType = typ.Name
				case strings.EqualFold(receiver, "super"):
					if members, ok := model.lookup(normalizeName(typ.Name)); ok {
						receiverType = members.superClass
					}
				default:
					if lookupName, ok := semaStaticContextTypeReceiver(
						model,
						typ,
						member,
						receiver,
						method,
						scope.hasNonFieldBinding(receiver),
					); ok {
						receiverType = lookupName
						projectClassReceiver = true
					} else if scoped, ok := scope.lookup(receiver); ok {
						receiverType = scoped
					} else if members, ok := model.lookup(normalizeName(receiver)); ok {
						if !semaPlatformReceiverSpellingMatches(receiver, members) {
							return nil
						}
						receiverType = receiver
					} else if a.hasKnown(receiver) {
						return nil
					} else {
						if strings.Count(expr.Callee, ".") == 1 {
							d := unknownCallDiagnostic(typ, member, expr.Callee, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)
							// C034: an exception method cannot bring its receiver
							// back into scope after the catch block ends.
							if strings.EqualFold(method, "getMessage") {
								d.NativeMessage = "Variable does not exist: " + receiver
							}
							return []diagnostic.Diagnostic{d}
						}
						return nil
					}
				}
			}
		}
	}
	if strings.HasPrefix(method, "__safe_call:") {
		method = strings.TrimPrefix(method, "__safe_call:")
	}
	if strings.EqualFold(receiverType, "AggregateResult") && strings.EqualFold(method, "get") && len(expr.Args) == 1 && !semaProjectTypeShadowsPlatform(model, receiverType) {
		argType := a.inferIRExprType(expr.Args[0], scope, model, typ.Name)
		message := ""
		switch strings.ToLower(argType) {
		case "null":
			message = "Ambiguous method signature: void get(NULL)"
		case "integer":
			message = "Method does not exist or incorrect signature: void get(Integer) from the type AggregateResult"
		}
		if message != "" {
			return []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "GLADESEMA_CALL", Message: message, File: typ.File, Range: semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)))}}
		}
	}
	if receiverType == "" {
		return nil
	}
	if message := a.semaAutomationCallMessage(receiverType, method, expr.Args, scope, model, typ.Name); message != "" {
		return []diagnostic.Diagnostic{semaAutomationDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	// addError on a captured SObject field is a record
	// validation operation, even when that field's value has type Decimal.
	if a.semaIRSObjectFieldAddError(expr, scope, model, typ.Name) {
		return nil
	}
	if message := a.semaDMLCallMessage(receiverType, method, expr.Args, scope, model, typ.Name); message != "" {
		return []diagnostic.Diagnostic{semaDMLDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	if semaSystemRunAsBlockCall(receiverType, method, expr.Callee, expr.Args) {
		return nil
	}
	receiverMode := "implicit"
	if explicitReceiver {
		receiverMode = "instance"
		if platformClassReceiver || projectClassReceiver {
			receiverMode = "class"
		}
		if expr.Left != nil && semaIRExprLooksLikeTypeReceiver(*expr.Left, scope, model) {
			receiverMode = "class"
		}
		if receiver, _, ok := strings.Cut(expr.Callee, "."); ok && !classLiteralReceiver {
			if _, scoped := scope.lookup(receiver); !scoped {
				if members, ok := model.lookup(normalizeName(receiver)); ok && semaPlatformReceiverSpellingMatches(receiver, members) {
					receiverMode = "class"
				}
			}
		}
	}
	// Project enums have their own static signatures even though
	// their declarations shadow the permissive platform-call fallback.
	if members, ok := model.lookup(normalizeName(receiverType)); ok && members.kind == apexast.DeclarationEnum && !members.platform && receiverMode == "class" && (strings.EqualFold(method, "values") || strings.EqualFold(method, "valueOf")) {
		if sig, ok := semaEnumMethodSignature(model, receiverType, method); ok {
			argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
			if !semaArgsMatchAny(sig.params, argTypes, model) {
				d := collectionCallDiagnostic(typ, member, method, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)
				d.NativeMessage = fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(argTypes, ","), receiverType)
				return []diagnostic.Diagnostic{d}
			}
			return nil
		}
	}
	if d, handled := semaSearchCallDiagnostic(typ, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), receiverMode, bodyOffset+pos, source, model); handled {
		if d != nil {
			return []diagnostic.Diagnostic{*d}
		}
		return nil
	}
	// Instance receivers can bypass checkIRPlatformCall through its permissive
	// fallback after IR infers their platform type, so guard that path here.
	if semaGovernorCallCandidate(receiverType, method, model) {
		if message := semaGovernorCallMessage(typ.EffectiveAPIVersion, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, receiverMode); message != "" {
			return []diagnostic.Diagnostic{semaGovernorDiagnostic(typ, "GLADESEMA009", message, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}
		}
	}
	if _, reports := semaReportsTypeName(receiverType, model); reports {
		if d, rejected := semaReportsCallDiagnostic(typ, member, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), receiverMode, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source, model); rejected {
			return []diagnostic.Diagnostic{d}
		}
	}
	if semaQueryLocatorCall(receiverType, method) {
		if d, rejected := semaQueryLocatorCallDiagnostic(typ, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source, model); rejected {
			return []diagnostic.Diagnostic{d}
		}
	}
	if _, cache := semaPlatformCacheTypeName(receiverType, model); cache {
		if d, rejected := semaPlatformCacheCallDiagnostic(typ, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), receiverMode, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source, model); rejected {
			return []diagnostic.Diagnostic{d}
		}
	}
	if d, rejected := semaHTTPMethodDiagnostic(typ, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, receiverMode, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source); rejected {
		return []diagnostic.Diagnostic{d}
	}
	if d, rejected := semaTestCallDiagnostic(typ, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
		return []diagnostic.Diagnostic{d}
	}
	if d, rejected := semaMessagingMethodDiagnostic(typ, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source); rejected {
		return []diagnostic.Diagnostic{d}
	}
	if message := semaPlatformEventCallMessage(receiverType, method, receiverMode, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model); message != "" {
		return []diagnostic.Diagnostic{semaPlatformEventDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	if semaAsyncCallCandidate(receiverType, method) && !semaProjectTypeShadowsPlatform(model, receiverType) {
		if message := semaAsyncCallMessage(receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name)); message != "" {
			return []diagnostic.Diagnostic{semaAsyncDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
		}
	}
	if !semaProjectTypeShadowsPlatform(model, receiverType) && semaNumericCallRejected(receiverType, method, expr.Args) {
		return []diagnostic.Diagnostic{valueCollectionCallDiagnostic(collectionCallDiagnostic(typ, member, method, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source), receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model)}
	}
	if !semaProjectTypeShadowsPlatform(model, receiverType) && semaAPI67RejectedPlatformCallAtVersion(typ.EffectiveAPIVersion, receiverType, method, receiverMode) {
		return []diagnostic.Diagnostic{valueCollectionCallDiagnostic(unsupportedLocalFeatureDiagnostic(typ, member, receiverType+"."+method, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source), receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model)}
	}
	if !semaProjectTypeShadowsPlatform(model, receiverType) && semaAPI67RejectedPlatformCallArgs(typ.EffectiveAPIVersion, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name)) {
		return []diagnostic.Diagnostic{valueCollectionCallDiagnostic(collectionCallDiagnostic(typ, member, method, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source), receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model)}
	}
	if !semaProjectTypeShadowsPlatform(model, receiverType) &&
		strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "Database") &&
		strings.EqualFold(method, "getQueryLocator") &&
		len(expr.Args) > 0 && expr.Args[0].Kind != ir.ExprSOQL {
		argType := a.inferIRExprType(expr.Args[0], scope, model, typ.Name)
		argBase, _ := semaGenericBaseAndArgs(argType)
		if strings.EqualFold(argBase, "List") || strings.EqualFold(argType, "Database.QueryResult") {
			return []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA009",
				Message:  "Argument must be an inline query",
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(method))),
			}}
		}
	}
	if diag, rejected := a.checkIRVisualforceCall(typ, expr, receiverType, method, receiverMode, scope, model, bodyOffset+pos, source); rejected {
		return []diagnostic.Diagnostic{diag}
	}
	if strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "String") && strings.EqualFold(method, "valueOf") {
		if diag, rejected := a.unresolvedIRStringValueOfDiagnostic(typ, member, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), bodyOffset+pos, source, model); rejected {
			return []diagnostic.Diagnostic{diag}
		}
	}
	// Generic collection signatures carry the receiver's element/key types.
	// Generated Object overloads cannot erase those types before validation.
	if strings.HasSuffix(strings.ToLower(receiverType), "exception") {
		if d, rejected := exceptionCallDiagnostic(typ, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source); rejected {
			return []diagnostic.Diagnostic{d}
		}
	}
	if !semaProjectTypeShadowsPlatform(model, receiverType) {
		if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
			return diagnostics
		}
	}
	// Native provenance R023/C023: check the explicitly selected platform
	// receiver before canonical catalog owners can resolve to a project shadow.
	if strings.HasPrefix(strings.ToLower(receiverType), "system.") && strings.Count(receiverType, ".") > 1 {
		if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model, receiverMode); handled {
			return diagnostics
		}
	}
	candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, receiverType, method), receiverMode)
	if !explicitReceiver {
		candidates = resolveImplicitMemberMethods(model, receiverType, method)
	}
	if len(candidates) == 0 {
		if semaCallMayBelongToMissingSuperclass(model, typ, expr.Callee, receiverMode, receiverType) {
			return nil
		}
		// C003/C004 capture only a receiver named by the callee path. A chained
		// or indexed receiver typed from a platform return keeps canonical flow.
		if expr.Left == nil {
			if d, rejected := semaShadowMissingMethodDiagnostic(typ, member, receiverType, method, expr.Callee, irCallArgTypes(a, expr.Args, scope, model, typ.Name), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
				return []diagnostic.Diagnostic{d}
			}
		}
		if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model, receiverMode); handled {
			return diagnostics
		}
		if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
			return diagnostics
		}
	}
	if len(candidates) == 0 && !strings.Contains(expr.Callee, ".") && bodyOffset >= 0 && bodyOffset <= len(source) {
		if chainedReceiver, chainedMethod, ok := semaChainedCallReceiverNear(source[bodyOffset:], pos, method, scope.flatCopy(), model, typ.Name); ok && strings.EqualFold(chainedMethod, method) {
			receiverType = chainedReceiver
			explicitReceiver = true
			receiverMode = "instance"
			candidates = preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, receiverType, method), receiverMode)
			argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
			if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok && !semaResolvedMembersAllPlatformBacked(model, candidates) {
				if staticDiagnostic, blocked := checkSemaStaticAccessWithModel(typ, member, expr.Callee, candidate, receiverMode, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); blocked {
					return []diagnostic.Diagnostic{staticDiagnostic}
				}
				if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, expr.Callee, candidate, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); blocked {
					return []diagnostic.Diagnostic{visibilityDiagnostic}
				}
				return nil
			}
			if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model, "instance"); handled {
				return diagnostics
			}
			if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
				return diagnostics
			}
		}
	}
	if len(candidates) == 0 {
		if semaKnownAddressValueCall(expr.Callee) {
			return nil
		}
		if strings.Contains(expr.Callee, ".") {
			receiverExpr := expr.Callee
			methodName := method
			if lastDot := strings.LastIndex(expr.Callee, "."); lastDot > 0 && lastDot < len(expr.Callee)-1 {
				receiverExpr = expr.Callee[:lastDot]
				methodName = expr.Callee[lastDot+1:]
			}
			if semaExternalPackageSObjectFieldPath(receiverExpr, scope.flat(), model) {
				return nil
			}
			if semaCallReceiverEntersDependencyType(model, typ.Name, receiverExpr, scope) {
				return nil
			}
			receiverTyp := inferSemaFieldAccessType(receiverExpr, scope.flatCopy(), model)
			if receiverTyp == "" {
				receiverParts := strings.Split(receiverExpr, ".")
				if len(receiverParts) > 0 && strings.HasSuffix(normalizeName(receiverParts[len(receiverParts)-1]), "address") {
					receiverTyp = "Address"
				}
			}
			if receiverTyp != "" {
				receiverMode := "instance"
				if semaReceiverExprLooksLikeType(receiverExpr, scope, model) {
					receiverMode = "class"
				}
				if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverTyp, methodName, expr.Args, scope, pos, bodyOffset, source, model, receiverMode); handled {
					return diagnostics
				}
				if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverTyp, methodName, expr.Args, scope, pos, bodyOffset, source, model); handled {
					return diagnostics
				}
				if a.hasKnown(receiverTyp) {
					return nil
				}
			}
		}
		if semaRelationshipCollectionMethod(expr.Callee, method) {
			return nil
		}
		if semaKnownFluentHelperMethod(method) {
			return nil
		}
		if expr.Left == nil && strings.Count(expr.Callee, ".") != 1 && semaSourceHasDottedCall(source, method) {
			return nil
		}
		if explicitReceiver && a.hasKnown(receiverType) {
			return nil
		}
		if semaCallMayBelongToMissingSuperclass(model, typ, expr.Callee, receiverMode, receiverType) {
			return nil
		}
		d := unknownCallDiagnostic(typ, member, expr.Callee, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)
		return []diagnostic.Diagnostic{valueCollectionCallDiagnostic(d, receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model)}
	}
	argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
	if candidate, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok {
		if staticDiagnostic, blocked := checkSemaStaticAccessWithModel(typ, member, expr.Callee, candidate, receiverMode, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); blocked {
			return []diagnostic.Diagnostic{staticDiagnostic}
		}
		return nil
	} else if ambiguous {
		if semaArgTypesContainUnknown(argTypes) {
			return nil
		}
		if semaAmbiguousNewListHelper(method, candidates, argTypes) {
			return nil
		}
		if semaAmbiguousQueryBuilderAdd(method, candidates) {
			return nil
		}
		if semaAmbiguousResolvedSameReturnType(candidates, argTypes) {
			return nil
		}
		if semaKnownFluentHelperMethod(method) {
			return nil
		}
		if message := semaAsyncBatchableAmbiguityMessage(receiverType, method, argTypes, model); message != "" {
			return []diagnostic.Diagnostic{semaAsyncDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
		}
		return []diagnostic.Diagnostic{nativeSourceOverloadDiagnostic(ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source), receiverType, method, argTypes, candidates, model)}
	}
	if semaResolvedMembersAllPlatformBacked(model, candidates) {
		if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model, receiverMode); handled {
			return diagnostics
		}
		if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
			return diagnostics
		}
	}
	if semaObjectMethodName(method) {
		if sig, ok := semaPlatformMethodSignatureForMode(model, receiverType, method, receiverMode); ok {
			argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
			if semaArgsMatchAny(sig.params, argTypes, model) {
				return nil
			}
		}
	}
	// C012/C018/C019: an expression receiver is already resolved by the IR.
	// A textual dotted call elsewhere cannot excuse its invalid signature.
	if expr.Left == nil && strings.Count(expr.Callee, ".") != 1 && semaSourceHasDottedCall(source, method) {
		return nil
	}
	if semaKnownFluentHelperMethod(method) {
		return nil
	}
	preserveFieldTypes := false
	for i, arg := range expr.Args {
		if arg.Kind == ir.ExprCall && (strings.HasPrefix(arg.Callee, "__field:") || strings.HasPrefix(arg.Callee, "__safe_field:")) && argTypes[i] != "" {
			preserveFieldTypes = true
		}
	}
	if textArgTypes := irVariableTextArgTypes(expr.Args, scope, model); len(textArgTypes) == len(expr.Args) {
		if _, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, textArgTypes, model); ok &&
			(!preserveFieldTypes || !semaArgTypesContainUnknown(textArgTypes)) {
			return nil
		} else if ambiguous && !semaArgTypesContainUnknown(textArgTypes) {
			return []diagnostic.Diagnostic{nativeSourceOverloadDiagnostic(ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source), receiverType, method, textArgTypes, candidates, model)}
		}
	}
	if sourceArgTypes := irSourceArgTypesForCall(source, bodyOffset+pos, expr.Callee, scope, model); len(sourceArgTypes) == len(expr.Args) {
		// Native provenance C009-C013/C020-C021: an unknown text fallback
		// cannot erase the known final type of an IR field/safe-navigation read.
		if _, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, sourceArgTypes, model); ok &&
			(!preserveFieldTypes || !semaArgTypesContainUnknown(sourceArgTypes)) {
			return nil
		} else if ambiguous && !semaArgTypesContainUnknown(sourceArgTypes) {
			return []diagnostic.Diagnostic{nativeSourceOverloadDiagnostic(ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source), receiverType, method, sourceArgTypes, candidates, model)}
		}
	}
	d := diagnostic.Diagnostic{
		Severity:      diagnostic.Error,
		Code:          "GLADESEMA009",
		NativeMessage: nativeNarrowedBatchableCallRejection(model, candidates, argTypes),
		Message:       fmt.Sprintf("%s %q has no matching overload for call %q with %d argument(s)", member.Kind, member.Name, expr.Callee, len(expr.Args)),
		File:          typ.File,
		Range:         semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee))),
	}
	return []diagnostic.Diagnostic{nativeSourceOverloadDiagnostic(d, receiverType, method, argTypes, candidates, model)}
}

// Keep the local explanation while exposing the native signature for rejected
// calls to resolved source methods. Platform contracts retain their own text.
func nativeSourceOverloadDiagnostic(d diagnostic.Diagnostic, receiverType, method string, argTypes []string, candidates []resolvedMember, model *semaTypeMemberView) diagnostic.Diagnostic {
	if d.NativeMessage != "" || len(candidates) == 0 || semaArgTypesContainUnknown(argTypes) {
		return d
	}
	for _, candidate := range candidates {
		owner, _, ok := semaLookupTypeMembers(model, candidate.owner)
		if !ok || owner.platform || owner.dependency || owner.sobject {
			return d
		}
	}
	args := append([]string(nil), argTypes...)
	for i, arg := range args {
		if strings.EqualFold(arg, "null") {
			args[i] = "NULL"
		}
	}
	signature := "void " + method + "(" + strings.Join(args, ", ") + ")"
	switch d.Code {
	case "GLADESEMA022":
		d.NativeMessage = "Ambiguous method signature: " + signature
	case "GLADESEMA009":
		if receiverType != "" {
			d.NativeMessage = "Method does not exist or incorrect signature: " + signature + " from the type " + receiverType
		}
	}
	return d
}

func irVariableTextArgTypes(args []ir.Expr, scope irSemaScope, model *semaTypeMemberView) []string {
	argTypes := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.Kind != ir.ExprVariable || strings.TrimSpace(arg.Name) == "" {
			return nil
		}
		argTypes = append(argTypes, inferSemaArgTypeWithModel(arg.Name, scope.flatCopy(), model))
	}
	return argTypes
}

func irSourceArgTypesForCall(source string, calleeStart int, callee string, scope irSemaScope, model *semaTypeMemberView) []string {
	if calleeStart < 0 || calleeStart >= len(source) {
		return nil
	}
	if calleeStart+len(callee) > len(source) || !strings.EqualFold(source[calleeStart:calleeStart+len(callee)], callee) {
		windowStart := max(0, calleeStart-16)
		windowEnd := min(len(source), calleeStart+len(callee)+32)
		window := source[windowStart:windowEnd]
		found := -1
		lowerCallee := strings.ToLower(callee)
		lowerWindow := strings.ToLower(window)
		for offset := strings.Index(lowerWindow, lowerCallee); offset >= 0; {
			end := offset + len(callee)
			for end < len(window) && isWhitespace(window[end]) {
				end++
			}
			if end < len(window) && window[end] == '(' {
				found = offset
				break
			}
			next := strings.Index(lowerWindow[offset+1:], lowerCallee)
			if next < 0 {
				break
			}
			offset += next + 1
		}
		if found < 0 {
			return nil
		}
		calleeStart = windowStart + found
	}
	args, ok := callArgumentsAt(source, calleeStart+len(callee))
	if !ok {
		return nil
	}
	argTypes := make([]string, len(args))
	flat := scope.flatCopy()
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, flat, model)
	}
	return argTypes
}

func (a *Analyzer) semaIRSObjectFieldAddError(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) bool {
	if !strings.EqualFold(expr.Callee, "addError") || expr.Left == nil || len(expr.Args) != 1 || !strings.EqualFold(a.inferIRExprType(expr.Args[0], scope, model, currentType), "String") {
		return false
	}
	field := *expr.Left
	if field.Kind != ir.ExprCall || field.Left == nil || !strings.HasPrefix(field.Callee, "__field:") {
		return false
	}
	recordType := a.inferIRExprType(*field.Left, scope, model, currentType)
	if !isSemaSObjectLike(recordType, model) || semaIRExprLooksLikeTypeReceiver(*field.Left, scope, model) {
		return false
	}
	_, resolved := semaResolveFieldPath(model, recordType, strings.TrimPrefix(field.Callee, "__field:"))
	return resolved
}

func semaReceiverExprLooksLikeType(receiverExpr string, scope irSemaScope, model *semaTypeMemberView) bool {
	receiverExpr = strings.TrimSpace(receiverExpr)
	if receiverExpr == "" {
		return false
	}
	root, _, _ := strings.Cut(receiverExpr, ".")
	if root != "" {
		if _, scoped := scope.lookup(root); scoped {
			return false
		}
	}
	if semaModelHasType(model, receiverExpr) {
		if members, ok := model.lookup(normalizeName(receiverExpr)); ok && !semaPlatformReceiverSpellingMatches(receiverExpr, members) {
			return false
		}
		return true
	}
	canonical := semaCanonicalPlatformAlias(receiverExpr)
	return !strings.EqualFold(canonical, receiverExpr) && semaKnownPlatformTypeReceiver(receiverExpr) && semaModelHasType(model, canonical)
}

func semaIRExprLooksLikeTypeReceiver(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView) bool {
	path, ok := semaIRExprTypeReceiverPath(expr)
	if !ok {
		return false
	}
	root, _, _ := strings.Cut(path, ".")
	if root != "" {
		if _, scoped := scope.lookup(root); scoped {
			return false
		}
	}
	if semaModelHasType(model, path) {
		return true
	}
	canonical := semaCanonicalPlatformAlias(path)
	return !strings.EqualFold(canonical, path) && semaKnownPlatformTypeReceiver(path) && semaModelHasType(model, canonical)
}

func semaIRCallReceiverMode(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView) string {
	if expr.Left != nil {
		if semaIRExprLooksLikeTypeReceiver(*expr.Left, scope, model) {
			return "class"
		}
		return "instance"
	}
	receiver, _, ok := strings.Cut(expr.Callee, ".")
	if !ok || receiver == "" {
		return "implicit"
	}
	if strings.EqualFold(receiver, "super") {
		return "super"
	}
	if semaReceiverExprLooksLikeType(receiver, scope, model) {
		return "class"
	}
	return "instance"
}

func semaIRExprTypeReceiverPath(expr ir.Expr) (string, bool) {
	switch expr.Kind {
	case ir.ExprVariable:
		name := strings.TrimSpace(expr.Name)
		return name, name != ""
	case ir.ExprCall:
		if expr.Left == nil {
			return "", false
		}
		field := strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__safe_field:"), "__field:")
		if field == expr.Callee || strings.TrimSpace(field) == "" {
			return "", false
		}
		left, ok := semaIRExprTypeReceiverPath(*expr.Left)
		if !ok {
			return "", false
		}
		return left + "." + field, true
	default:
		return "", false
	}
}

func (a *Analyzer) checkIRCollectionCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) ([]diagnostic.Diagnostic, bool) {
	sig, ok := semaCollectionMethodSignature(receiverType, method)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = a.inferIRExprType(arg, scope, model, typ.Name)
	}
	if strings.EqualFold(method, "addError") && semaAddErrorArgsAccepted(argTypes, model) {
		return nil, true
	}
	if semaArgsMatchAny(sig.params, argTypes, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{valueCollectionCallDiagnostic(collectionCallDiagnostic(typ, member, method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source), receiverType, method, argTypes, model)}, true
}

func (a *Analyzer) checkIRPlatformCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, receiverMode string) ([]diagnostic.Diagnostic, bool) {
	if semaProjectTypeShadowsPlatform(model, receiverType) {
		return nil, false
	}
	if semaNumericCallRejected(receiverType, method, args) {
		return []diagnostic.Diagnostic{valueCollectionCallDiagnostic(collectionCallDiagnostic(typ, member, method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source), receiverType, method, irCallArgTypes(a, args, scope, model, typ.Name), model)}, true
	}
	if semaPlatformTypeUnavailable(typ.EffectiveAPIVersion, receiverType) || semaPlatformMemberUnavailable(typ.EffectiveAPIVersion, receiverType, method) {
		return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, receiverType+"."+method, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}, true
	}
	if semaDatabaseDynamicQueryCall(receiverType, method) {
		return nil, true
	}
	if _, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return nil, false
	}
	candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, receiverType, method), receiverMode)
	var sig semaCollectionSignature
	qualifiedPlatform := false
	// Native provenance R023/C023: nested System types are catalogued without
	// the namespace. Resolve their signatures in the platform-only view so a
	// project nested type cannot replace the explicitly qualified receiver.
	if strings.HasPrefix(strings.ToLower(receiverType), "system.") && strings.Count(receiverType, ".") > 1 {
		sig, qualifiedPlatform = semaUnshadowedGeneratedPlatformMethodSignature(model, receiverType[len("System."):], method, receiverMode)
	}
	if len(candidates) != 0 && !semaResolvedMembersAllPlatformBacked(model, candidates) && !qualifiedPlatform {
		return nil, false
	}
	ok := qualifiedPlatform
	if !ok {
		sig, ok = semaPlatformMethodSignatureForMode(model, receiverType, method, receiverMode)
	}
	if !ok {
		if strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "String") {
			return []diagnostic.Diagnostic{valueCollectionCallDiagnostic(unknownCallDiagnostic(typ, member, receiverType+"."+method, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source), receiverType, method, irCallArgTypes(a, args, scope, model, typ.Name), model)}, true
		}
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(arg, scope, model, typ.Name))
	}
	if d, rejected := a.unresolvedIRStringValueOfDiagnostic(typ, member, receiverType, method, argTypes, bodyOffset+pos, source, model); rejected {
		return []diagnostic.Diagnostic{d}, true
	}
	if d, rejected := semaPlatformCacheCallDiagnostic(typ, receiverType, method, argTypes, receiverMode, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source, model); rejected {
		return []diagnostic.Diagnostic{d}, true
	}
	if d, rejected := semaQueryLocatorCallDiagnostic(typ, receiverType, method, argTypes, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source, model); rejected {
		return []diagnostic.Diagnostic{d}, true
	}
	if semaAmbiguousSObjectNullCall(receiverType, method, argTypes, model) {
		return []diagnostic.Diagnostic{ambiguousCallDiagnostic(typ, member, receiverType+"."+method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}, true
	}
	if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok && semaResolvedMembersAllPlatformBacked(model, candidates) && semaPlatformResolvedMemberUnavailable(typ.EffectiveAPIVersion, candidate) {
		return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, receiverType+"."+method, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}, true
	}
	if semaAPI67RejectedPlatformCallArgs(typ.EffectiveAPIVersion, receiverType, method, argTypes) {
		return []diagnostic.Diagnostic{valueCollectionCallDiagnostic(collectionCallDiagnostic(typ, member, method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source), receiverType, method, irCallArgTypes(a, args, scope, model, typ.Name), model)}, true
	}
	if semaDatabaseDMLObjectCall(receiverType, method, argTypes, model) {
		return nil, true
	}
	if semaDatabaseUpsertObjectFieldCall(receiverType, method, argTypes, model) {
		return nil, true
	}
	if semaDatabaseDMLReturnType(receiverType, method, argTypes) != "" && len(args) <= 4 && semaFirstArgMatchesAny(sig.params, argTypes[0], model) {
		return nil, true
	}
	if semaSearchSuggestObjectOverload(receiverType, method, argTypes) {
		return nil, true
	}
	if semaArgsMatchAny(sig.params, argTypes, model) {
		return nil, true
	}
	item := collectionCallDiagnostic(typ, member, method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)
	// overload_dispatch C030: an interface's implicit equals(Object) member
	// rejects the wrong arity with the declared receiver's native signature.
	// Keep the existing local code and explanation for platform-call errors.
	owner, _, found := semaLookupTypeMembers(model, receiverType)
	if !semaArgTypesContainUnknown(argTypes) && (qualifiedPlatform ||
		(found && owner.kind == apexast.DeclarationInterface && !owner.platform && !owner.dependency && strings.EqualFold(method, "equals"))) {
		item.NativeMessage = fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(argTypes, ", "), receiverType)
	}
	item = valueCollectionCallDiagnostic(item, receiverType, method, argTypes, model)
	return []diagnostic.Diagnostic{semaFormulaMethodDiagnostic(item, receiverType, method, argTypes)}, true
}

func (a *Analyzer) checkIRConstructorCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	typeName := strings.TrimPrefix(expr.Callee, "new:")
	resolvedTypeName := resolveNestedTypeReference(model, typ.Name, typeName)
	if message := semaPlatformEventConstructorMessage(resolvedTypeName, irCallArgTypes(a, expr.Args, scope, model, typ.Name), len(expr.NamedArgs), model); message != "" {
		return []diagnostic.Diagnostic{semaPlatformEventDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if message := semaAutomationConstructorMessage(resolvedTypeName, model); message != "" {
		return []diagnostic.Diagnostic{semaAutomationDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if d, rejected := semaMessagingConstructorDiagnostic(typ, resolvedTypeName, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source); rejected {
		return []diagnostic.Diagnostic{d}
	}
	if name := semaGovernorType(resolvedTypeName, model); name == "Limit" || name == "System.OrgLimit" {
		if message := semaGovernorConstructorMessage(typeName, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model); message != "" {
			return []diagnostic.Diagnostic{semaGovernorDiagnostic(typ, "GLADESEMA011", message, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
		}
	}
	// Object is a declared type, not a constructor.
	if strings.EqualFold(semaCanonicalPlatformAlias(typeName), "Object") && !semaProjectTypeShadowsPlatform(model, typeName) {
		return []diagnostic.Diagnostic{typeContractNativeDiagnostic(typ, "Type cannot be constructed: Object", bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if message := semaAsyncConstructorMessage(typeName); message != "" && semaAsyncPlatformType(model, resolvedTypeName) {
		return []diagnostic.Diagnostic{semaAsyncDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if name := semaDMLNonconstructibleType(resolvedTypeName); name != "" && semaDMLPlatformType(model, resolvedTypeName) {
		return []diagnostic.Diagnostic{semaDMLDiagnostic(typ, "Type cannot be constructed: "+name, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if d, rejected := semaSearchConstructorDiagnostic(typ, resolvedTypeName, expr.Args, bodyOffset+pos, source, model); rejected {
		return []diagnostic.Diagnostic{d}
	}
	if len(expr.NamedArgs) == 0 && strings.HasSuffix(strings.ToLower(resolvedTypeName), "exception") {
		if d, rejected := exceptionConstructorDiagnostic(typ, member, resolvedTypeName, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source); rejected {
			return []diagnostic.Diagnostic{d}
		}
	}
	if d, rejected := semaPlatformCacheConstructorDiagnostic(typ, resolvedTypeName, len(expr.Args)+len(expr.NamedArgs), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source, model); rejected {
		return []diagnostic.Diagnostic{d}
	}
	rejectedConstructor := semaAPI67RejectedPlatformConstructor(typeName)
	if strings.EqualFold(typeName, "Cache.CacheBuilder") {
		// A source-defined Cache.CacheBuilder is not the platform interface.
		_, rejectedConstructor = semaPlatformCacheTypeName(resolvedTypeName, model)
	}
	if strings.EqualFold(resolvedTypeName, "AggregateResult") && !semaProjectTypeShadowsPlatform(model, typeName) {
		return []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "GLADESEMA015", Message: "SObject is not constructable: AggregateResult", File: typ.File, Range: semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)))}}
	}
	if !semaProjectTypeShadowsPlatform(model, typeName) {
		if hidden := semaHTTPInvisibleType(typeName); hidden != "" {
			return []diagnostic.Diagnostic{semaHTTPDiagnostic(typ, "Type is not visible: "+hidden, bodyOffset+pos, bodyOffset+pos+len(typeName), source)}
		}
		constructorName := ""
		if semaHTTPReceiver(typeName) == "Continuation" {
			constructorName = "System.Continuation"
		} else if strings.EqualFold(typeName, "commercepayments.PaymentGatewayContext") {
			constructorName = "commercepayments.PaymentGatewayContext"
		}
		if constructorName != "" {
			argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
			params, known := semaPlatformConstructorSignatures(typeName)
			if known && len(expr.NamedArgs) == 0 && !semaArgsMatchAny(params, argTypes, model) {
				message := "Constructor not defined: [" + constructorName + "].<Constructor>(" + semaHTTPArgNames(argTypes) + ")"
				return []diagnostic.Diagnostic{semaHTTPDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+len(typeName), source)}
			}
		}
	}
	if (rejectedConstructor || semaPlatformTypeUnavailable(typ.EffectiveAPIVersion, typeName)) && !semaProjectTypeShadowsPlatform(model, typeName) {
		return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, "new "+typeName, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	constructorName := resolvedTypeName
	if dot := strings.LastIndexByte(constructorName, '.'); dot >= 0 {
		constructorName = constructorName[dot+1:]
	}
	if semaPlatformMemberUnavailable(typ.EffectiveAPIVersion, resolvedTypeName, constructorName) && !semaProjectTypeShadowsPlatform(model, typeName) {
		return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, "new "+typeName, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	for _, ref := range extractTypeNames(typeName) {
		if !a.hasKnownAtVersion(ref, typ.EffectiveAPIVersion) {
			return []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA006",
				Message:  fmt.Sprintf("%s %q constructs unknown type %q", member.Kind, member.Name, ref),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName))),
			}}
		}
	}
	if target, ok := constructability[normalizeName(resolvedTypeName)]; ok && !isConstructableType(target) {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA015",
			Message:  fmt.Sprintf("%s %q constructs non-instantiable %s %q", member.Kind, member.Name, target.Kind, target.Name),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName))),
		}}
	}
	if diagnostics, handled := a.checkIRCollectionConstructor(typ, member, typeName, expr.Args, scope, pos, bodyOffset, source, model); handled {
		return diagnostics
	}
	if isSemaSObjectLike(resolvedTypeName, model) && len(expr.Args) == 0 {
		for _, arg := range expr.NamedArgs {
			if fieldName, readOnly := semaFormulaReadOnlyField(model, resolvedTypeName, arg.Name); readOnly {
				item := semaFieldAccessDiagnostic(typ, member, fieldName, "field is not writeable", bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)
				item.Message = "Field is not writeable: " + fieldName
				return []diagnostic.Diagnostic{item}
			}
		}
		return nil
	}
	argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
	namedArgTypes := irCallNamedArgTypes(a, expr.NamedArgs, scope, model, typ.Name)
	if target, ok := model.lookup(normalizeName(resolvedTypeName)); ok && target.platform {
		if _, diagnosticName := semaFormulaPlatformType(resolvedTypeName); diagnosticName != "" {
			if params, ok := semaPlatformConstructorSignatures(resolvedTypeName); ok && len(params) == 0 {
				item := constructorDiagnostic(typ, member, "new "+typeName, "", bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)
				item.Message = fmt.Sprintf("Constructor not defined: [%s].<Constructor>(%s)", diagnosticName, strings.Join(argTypes, ", "))
				return []diagnostic.Diagnostic{item}
			}
		}
	}
	if len(expr.NamedArgs) == 0 {
		if diag, rejected := semaVisualforceConstructorDiagnostic(typ, resolvedTypeName, argTypes, model, bodyOffset+pos, source); rejected {
			return []diagnostic.Diagnostic{diag}
		}
	}
	if semaExplicitPlatformQualifiedName(typeName) {
		if params, ok := semaPlatformConstructorSignatures(resolvedTypeName); ok {
			if len(params) == 0 {
				return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("no matching %s constructor with %d argument(s)", typeName, len(expr.Args)+len(expr.NamedArgs)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
			}
			if len(namedArgTypes) == 0 && semaArgsMatchAny(params, argTypes, model) {
				return nil
			}
			return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("no matching %s constructor with %d argument(s)", typeName, len(expr.Args)+len(expr.NamedArgs)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
		}
	}
	target, ok := model.lookup(normalizeName(resolvedTypeName))
	if !ok {
		return nil
	}
	if len(target.constructors) == 0 {
		if target.constructorsAuthoritative {
			return []diagnostic.Diagnostic{nativeIRConstructorDiagnostic(typ, member, typeName, resolvedTypeName, argTypes, len(expr.NamedArgs), false, target.platform, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
		}
		if len(expr.Args) == 0 || a.allowsInheritedExceptionConstructor(resolvedTypeName, expr.Args, scope, model, typ.Name) {
			return nil
		}
		return []diagnostic.Diagnostic{nativeIRConstructorDiagnostic(typ, member, typeName, resolvedTypeName, argTypes, len(expr.NamedArgs), false, target.platform, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if len(namedArgTypes) == 0 {
		if candidate, ok, ambiguous := bestConstructorByIRSOQLSingletonArgs(target.constructors, argTypes, expr.Args, model); ok {
			if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, "new "+typeName, resolvedMember{owner: target.name, member: candidate}, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source, model); blocked {
				return []diagnostic.Diagnostic{visibilityDiagnostic}
			}
			return nil
		} else if ambiguous {
			if semaAllowAmbiguousPlatformConstructor(resolvedTypeName, argTypes) || semaArgTypesContainUnknown(argTypes) {
				return nil
			}
			return []diagnostic.Diagnostic{nativeIRConstructorDiagnostic(typ, member, typeName, resolvedTypeName, argTypes, len(expr.NamedArgs), true, target.platform, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
		}
	}
	if candidate, ok, ambiguous := bestConstructorByArgTypes(target.constructors, argTypes, namedArgTypes, model); ok {
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, "new "+typeName, resolvedMember{owner: target.name, member: candidate}, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source, model); blocked {
			return []diagnostic.Diagnostic{visibilityDiagnostic}
		}
		return nil
	} else if ambiguous {
		if semaAllowAmbiguousPlatformConstructor(resolvedTypeName, argTypes) || semaArgTypesContainUnknown(argTypes) {
			return nil
		}
		return []diagnostic.Diagnostic{nativeIRConstructorDiagnostic(typ, member, typeName, resolvedTypeName, argTypes, len(expr.NamedArgs), true, target.platform, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if !target.constructorsAuthoritative && a.allowsInheritedExceptionConstructor(resolvedTypeName, expr.Args, scope, model, typ.Name) {
		return nil
	}
	return []diagnostic.Diagnostic{nativeIRConstructorDiagnostic(typ, member, typeName, resolvedTypeName, argTypes, len(expr.NamedArgs), false, target.platform, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
}

func semaAllowAmbiguousPlatformConstructor(typeName string, argTypes []string) bool {
	if !strings.EqualFold(typeName, "ApexPages.StandardSetController") || len(argTypes) != 1 {
		return false
	}
	return argTypes[0] == ""
}

func (a *Analyzer) allowsInheritedExceptionConstructor(typeName string, args []ir.Expr, scope irSemaScope, model *semaTypeMemberView, ownerType string) bool {
	if !semaTypeMatches(model, typeName, "Exception", make(map[string]bool)) {
		return false
	}
	argTypes := irCallArgTypes(a, args, scope, model, ownerType)
	return semaArgsMatchAny(inheritedExceptionConstructorSignatures(typeName), argTypes, model)
}

func semaAllowsInheritedExceptionConstructorArgs(typeName string, args []semaArg, scope map[string]string, model *semaTypeMemberView) bool {
	if !semaTypeMatches(model, typeName, "Exception", make(map[string]bool)) {
		return false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	return semaArgsMatchAny(inheritedExceptionConstructorSignatures(typeName), argTypes, model)
}

func inheritedExceptionConstructorSignatures(typeName string) [][]string {
	if strings.EqualFold(strings.TrimPrefix(typeName, "System."), "TouchHandledException") {
		return [][]string{{"String"}}
	}
	return [][]string{{}, {"String"}, {"Exception"}, {"String", "Exception"}}
}

func (a *Analyzer) checkIRCollectionConstructor(typ typesys.TypeSymbol, member typesys.MemberSymbol, typeName string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) ([]diagnostic.Diagnostic, bool) {
	base, params := semaGenericBaseAndArgs(typeName)
	baseKey := normalizeName(base)
	if baseKey != "list" && baseKey != "set" && baseKey != "map" {
		return nil, false
	}
	if len(args) == 0 {
		return nil, true
	}
	if (baseKey == "list" || baseKey == "set") && len(params) == 1 {
		if len(args) == 1 {
			argType := a.inferIRExprType(args[0], scope, model, typ.Name)
			if baseKey == "list" && strings.EqualFold(argType, "Integer") {
				return nil, true
			}
			if argType == "" || strings.EqualFold(argType, "null") || semaAssignableToType(typeName, argType, model) || semaCollectionCopyConstructorAccepts(baseKey, params[0], argType, model) {
				return nil, true
			}
		}
		for _, arg := range args {
			argType := a.inferIRExprType(arg, scope, model, typ.Name)
			if argType != "" && !strings.EqualFold(argType, "null") && !semaAssignableToType(params[0], argType, model) {
				return []diagnostic.Diagnostic{collectionConstructorDiagnostic(typ, member, typeName, len(args), bodyOffset+pos, source)}, true
			}
		}
		return nil, true
	}
	if baseKey == "map" && len(params) == 2 {
		// The captured String/Object literal reports its unresolved value type
		// even though ordinary Object assignment accepts all known value types.
		if strings.EqualFold(params[0], "String") && strings.EqualFold(params[1], "Object") {
			for _, arg := range args {
				if arg.Kind != ir.ExprCall || arg.Callee != "__mapEntry" || len(arg.Args) != 2 {
					continue
				}
				valueType := a.inferIRExprType(arg.Args[1], scope, model, typ.Name)
				if a.unresolvedIRValueType(valueType, typ, model) {
					d := collectionConstructorDiagnostic(typ, member, typeName, len(args), bodyOffset+pos, source)
					d.NativeMessage = "Invalid value type " + valueType + " for Object"
					return []diagnostic.Diagnostic{d}, true
				}
			}
		}
		if len(args) == 1 {
			argType := a.inferIRExprType(args[0], scope, model, typ.Name)
			if argType == "" || strings.EqualFold(argType, "null") || semaAssignableToType(typeName, argType, model) || semaMapConstructorAccepts(params[0], params[1], argType, model) {
				return nil, true
			}
		}
		if semaMapEntriesAssignable(a, params[0], params[1], args, scope, model, typ.Name) {
			return nil, true
		}
	}
	return []diagnostic.Diagnostic{collectionConstructorDiagnostic(typ, member, typeName, len(args), bodyOffset+pos, source)}, true
}

func (a *Analyzer) unresolvedIRValueType(typeName string, typ typesys.TypeSymbol, model *semaTypeMemberView) bool {
	if typeName == "" || strings.EqualFold(typeName, "null") {
		return false
	}
	resolved := resolveNestedTypeReference(model, typ.Name, typeName)
	for _, ref := range extractTypeNames(resolved) {
		if !a.hasKnownAtVersion(ref, typ.EffectiveAPIVersion) {
			return true
		}
	}
	return false
}

// The Object overload must not hide an unresolved declared field type.
// Native compiler captures show this recovery for String.valueOf and valid twins.
func (a *Analyzer) unresolvedIRStringValueOfDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, argTypes []string, start int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if semaProjectTypeShadowsPlatform(model, receiverType) || !strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "String") || !strings.EqualFold(method, "valueOf") || len(argTypes) != 1 || !a.unresolvedIRValueType(argTypes[0], typ, model) {
		return diagnostic.Diagnostic{}, false
	}
	d := collectionCallDiagnostic(typ, member, method, len(argTypes), start, start+max(1, len(method)), source)
	d.NativeMessage = fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type String", method, argTypes[0])
	return d, true
}

func semaMapEntriesAssignable(a *Analyzer, keyType, valueType string, args []ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) bool {
	if len(args) == 0 {
		return true
	}
	for _, arg := range args {
		if arg.Kind != ir.ExprCall || arg.Callee != "__mapEntry" || len(arg.Args) != 2 {
			return false
		}
		entryKeyType := a.inferIRExprType(arg.Args[0], scope, model, currentType)
		if entryKeyType != "" && !strings.EqualFold(entryKeyType, "null") && !semaAssignableToType(keyType, entryKeyType, model) {
			return false
		}
		entryValueType := a.inferIRExprType(arg.Args[1], scope, model, currentType)
		if entryValueType != "" && !strings.EqualFold(entryValueType, "null") && !semaAssignableToType(valueType, entryValueType, model) {
			return false
		}
	}
	return true
}

func semaCollectionCopyConstructorAccepts(targetBase, targetElement, argType string, model *semaTypeMemberView) bool {
	sourceBase, sourceArgs := semaGenericBaseAndArgs(argType)
	sourceBaseKey := normalizeName(sourceBase)
	if sourceBaseKey != "list" && sourceBaseKey != "set" {
		return false
	}
	if (targetBase != "list" && targetBase != "set") || len(sourceArgs) != 1 {
		return false
	}
	return semaAssignableToType(targetElement, sourceArgs[0], model)
}

func semaMapConstructorAccepts(keyType, valueType, argType string, model *semaTypeMemberView) bool {
	if strings.EqualFold(argType, "Database.QueryResult") && strings.EqualFold(keyType, "Id") {
		return strings.EqualFold(valueType, "SObject") || isSemaSObjectLike(valueType, model)
	}
	sourceBase, sourceArgs := semaGenericBaseAndArgs(argType)
	sourceBaseKey := normalizeName(sourceBase)
	if sourceBaseKey == "map" && len(sourceArgs) == 2 {
		return semaAssignableToType(keyType, sourceArgs[0], model) && semaAssignableToType(valueType, sourceArgs[1], model)
	}
	if sourceBaseKey == "list" && len(sourceArgs) == 1 && (strings.EqualFold(keyType, "Id") || strings.EqualFold(keyType, "String")) {
		return semaAssignableToType(valueType, sourceArgs[0], model)
	}
	return false
}

func irCallArgTypes(a *Analyzer, args []ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) []string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argType := resolveNestedTypeReference(model, currentType, a.inferIRExprType(arg, scope, model, currentType))
		argTypes[i] = semaIRSObjectConstructorPrecedence(model, argType, arg)
	}
	return argTypes
}

func irCallNamedArgTypes(a *Analyzer, args []ir.NamedArg, scope irSemaScope, model *semaTypeMemberView, currentType string) map[string]string {
	if len(args) == 0 {
		return nil
	}
	argTypes := make(map[string]string, len(args))
	for _, arg := range args {
		if arg.Name == "" {
			continue
		}
		argType := resolveNestedTypeReference(model, currentType, a.inferIRExprType(arg.Expr, scope, model, currentType))
		argTypes[arg.Name] = semaIRSObjectConstructorPrecedence(model, argType, arg.Expr)
	}
	return argTypes
}

func irCallArgsMatch(a *Analyzer, params []apexast.Parameter, args []ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) bool {
	if len(params) != len(args) {
		return false
	}
	for i, param := range params {
		argType := a.inferIRExprType(args[i], scope, model, currentType)
		if semaConversionScore(param.Type, argType, model) < 0 {
			return false
		}
	}
	return true
}

func (a *Analyzer) checkIRAssignmentType(typ typesys.TypeSymbol, member typesys.MemberSymbol, targetType, target string, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, verb string) []diagnostic.Diagnostic {
	targetType = resolveNestedTypeReference(model, typ.Name, targetType)
	valueType := resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(expr, *scope, model, typ.Name))
	valueType = semaIRSObjectConstructorPrecedence(model, valueType, expr)
	if strings.EqualFold(valueType, "void") && semaIRExprLooksLikeIndexAssignmentValue(expr, source, bodyOffset+pos) {
		valueType = resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(expr.Args[1], *scope, model, typ.Name))
	}
	if valueType == "" || valueType == "null" || semaAssignableToType(targetType, valueType, model) || semaIRPlatformEnumTypeFieldAssignable(targetType, target, valueType, model) || (expr.Kind == ir.ExprSOQL && semaSOQLSingletonAssignable(targetType, valueType, "["+expr.Value+"]", model)) {
		return nil
	}
	expression := ""
	switch expr.Kind {
	case ir.ExprSOQL:
		expression = "[" + expr.Value + "]"
	case ir.ExprVariable:
		expression = expr.Name
	}
	bindings := scope.flat()
	if verb == "initializes" && semaChildRelationshipSingletonAssignable(targetType, valueType, expression, bindings, model) {
		return nil
	}
	if a.semaIRPlatformCacheAssignment(typ, targetType, valueType, expr, *scope, model) {
		message := semaPlatformCacheAssignmentMessage(targetType, valueType, model)
		return []diagnostic.Diagnostic{semaPlatformCacheDiagnostic(typ, "GLADESEMA018", message, bodyOffset+pos, bodyOffset+pos+max(1, len(target)), source)}
	}
	if message, reports := semaReportsAssignmentMessage(targetType, valueType, model); reports {
		return []diagnostic.Diagnostic{semaReportsDiagnostic(typ, "GLADESEMA018", message, bodyOffset+pos, bodyOffset+pos+max(1, len(target)), source)}
	}
	if method, standard := a.irStandardControllerCall(expr, *scope, model, typ.Name); standard {
		if message, captured := standardControllerAssignmentMessage(targetType, valueType, method); captured {
			diag := visualforceControllerDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(target)), source)
			diag.Code = visualforceControllerAssignmentCode
			return []diagnostic.Diagnostic{diag}
		}
	}
	if diag, rejected := a.semaVisualforceSetControllerAssignment(typ, targetType, valueType, expr, *scope, model, bodyOffset+pos, bodyOffset+pos+max(1, len(target)), source); rejected {
		return []diagnostic.Diagnostic{diag}
	}
	message := fmt.Sprintf("%s %q %s %s with %s", member.Kind, member.Name, verb, target, valueType)
	if native := a.semaIRGovernorAssignmentMessage(targetType, valueType, expr, *scope, model, typ.Name); native != "" {
		message = native
	}
	message = semaRelationshipAssignmentMessage(targetType, valueType, expression, bindings, model, message)
	message = semaAggregateAssignmentMessage(targetType, valueType, expression, model, message)
	if root, _, qualified := strings.Cut(target, "."); qualified && strings.EqualFold(semaCanonicalPlatformAlias(semaIRReceiverType(root, *scope, model, typ.Name)), "AsyncOptions") && !semaProjectTypeShadowsPlatform(model, "AsyncOptions") {
		message = fmt.Sprintf("Illegal assignment from %s to %s", valueType, targetType)
	}
	nativeMessage := semaDMLAssignmentMessage(targetType, valueType, "", model)
	if triggerMessage := semaTriggerAssignmentMessage(targetType, valueType, expr, *scope, model); triggerMessage != "" {
		nativeMessage = triggerMessage
	}
	if nativeMessage == "" {
		if root, _, qualified := strings.Cut(target, "."); qualified {
			nativeMessage = semaDMLAssignmentMessage(targetType, valueType, semaIRReceiverType(root, *scope, model, typ.Name), model)
		}
	}
	if nativeMessage != "" {
		message = nativeMessage
	}
	if native, ok := semaIRSearchAssignmentMessage(targetType, valueType, expr, *scope, model); ok {
		message = native
	}
	if nativeMessage := semaMessagingAssignmentMessage(targetType, valueType, "", model); nativeMessage != "" {
		message = nativeMessage
	} else if root, _, qualified := strings.Cut(target, "."); qualified {
		if nativeMessage := semaMessagingAssignmentMessage(targetType, valueType, semaIRReceiverType(root, *scope, model, typ.Name), model); nativeMessage != "" {
			message = nativeMessage
		}
	}
	if strings.EqualFold(targetType, "Blob") && strings.EqualFold(valueType, "String") {
		if root, field, ok := strings.Cut(target, "."); ok && strings.EqualFold(field, "responseBody") {
			receiver := semaIRReceiverType(root, *scope, model, typ.Name)
			if semaHTTPReceiver(receiver) == "RestResponse" && !semaProjectTypeShadowsPlatform(model, receiver) {
				message = "Illegal assignment from String to Blob"
			}
		}
	}
	d := diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA018",
		Message:  message,
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(target))),
	}
	d.NativeMessage = valueCollectionAssignmentMessage(targetType, valueType, model)
	if strings.EqualFold(targetType, "Integer") && strings.EqualFold(valueType, "String") {
		d.NativeMessage = "Illegal assignment from String to Integer"
	}
	if target, ok := model.lookupName(targetType); ok && target.kind == apexast.DeclarationEnum && !target.platform {
		d.NativeMessage = "Illegal assignment from " + valueType + " to " + targetType
	}
	return []diagnostic.Diagnostic{d}
}

func semaIRPlatformEnumTypeFieldAssignable(targetType, target, valueType string, model *semaTypeMemberView) bool {
	if !strings.EqualFold(targetType, "String") {
		return false
	}
	lastDot := strings.LastIndex(target, ".")
	if lastDot < 0 || !strings.EqualFold(target[lastDot+1:], "type") {
		return false
	}
	members, _, ok := semaLookupTypeMembers(model, valueType)
	return ok && members.dependency && members.kind == apexast.DeclarationEnum
}

func semaIRExprLooksLikeIndexAssignmentValue(expr ir.Expr, source string, pos int) bool {
	if expr.Kind != ir.ExprCall || !strings.EqualFold(expr.Callee, "set") || expr.Left == nil || len(expr.Args) != 2 {
		return false
	}
	if pos < 0 || pos >= len(source) {
		return false
	}
	end := semaStatementEnd(source, pos)
	if end <= pos || end > len(source) {
		return false
	}
	statement := source[pos:end]
	eq := strings.IndexByte(statement, '=')
	if eq < 0 {
		return false
	}
	for i := eq + 1; i < len(statement); i++ {
		if statement[i] != ']' {
			continue
		}
		next := i + 1
		for next < len(statement) && isWhitespace(statement[next]) {
			next++
		}
		if next < len(statement) && statement[next] == '=' {
			return true
		}
	}
	return false
}

func (a *Analyzer) checkIRReturnType(typ typesys.TypeSymbol, member typesys.MemberSymbol, returnType string, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	valueType := resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(expr, *scope, model, typ.Name))
	valueType = semaIRSObjectConstructorPrecedence(model, valueType, expr)
	if valueType == "" || valueType == "null" || semaAssignableToType(returnType, valueType, model) || (expr.Kind == ir.ExprSOQL && semaSOQLSingletonAssignable(returnType, valueType, "["+expr.Value+"]", model)) {
		return nil
	}
	if strings.EqualFold(returnType, "Boolean") && semaMemberReturnSourceLooksBoolean(source, 0, len(source)) {
		return nil
	}
	return []diagnostic.Diagnostic{returnTypeDiagnostic(typ, member, fmt.Sprintf("returns %s from %s method", valueType, returnType), bodyOffset+pos, bodyOffset+pos+max(1, len(valueType)), source)}
}

// semaIRSObjectConstructorPrecedence mirrors semaResolveConstructedExpressionType for the
// IR-based type inference path: when expr is a `new Type(field = value, ...)` call, that
// SObject field-initializer syntax only exists for real SObjects, so a genuine standard
// SObject named Type takes precedence over a same-named nested Apex class that nested-class
// resolution would otherwise prefer.
func semaIRSObjectConstructorPrecedence(model *semaTypeMemberView, resolved string, expr ir.Expr) string {
	if expr.Kind != ir.ExprCall || !strings.HasPrefix(expr.Callee, "new:") || len(expr.NamedArgs) == 0 {
		return resolved
	}
	bareName := strings.TrimPrefix(expr.Callee, "new:")
	return semaSObjectConstructorPrecedence(model, resolved, bareName, true)
}

func semaMemberReturnSourceLooksBoolean(source string, start, end int) bool {
	if start < 0 {
		start = 0
	}
	if end <= start || end > len(source) {
		end = len(source)
	}
	body := source[start:end]
	for offset := 0; ; {
		idx := strings.Index(body[offset:], "return")
		if idx < 0 {
			return false
		}
		returnStart := offset + idx
		exprStart := returnStart + len("return")
		if returnStart > 0 && isIdentifierByte(body[returnStart-1]) || exprStart < len(body) && isIdentifierByte(body[exprStart]) {
			offset = exprStart
			continue
		}
		exprEnd := strings.Index(body[exprStart:], ";")
		if exprEnd < 0 {
			return false
		}
		expr := strings.TrimSpace(body[exprStart : exprStart+exprEnd])
		for _, op := range []string{"&&", "||", "==", "!=", "<=", ">=", "<", ">"} {
			if strings.Contains(expr, op) {
				return true
			}
		}
		if strings.EqualFold(expr, "true") || strings.EqualFold(expr, "false") {
			return true
		}
		offset = exprStart + exprEnd + 1
		if offset >= len(body) {
			return false
		}
	}
}

func (a *Analyzer) checkIRConditionType(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	valueType := a.inferIRExprType(expr, *scope, model, typ.Name)
	if valueType == "" || strings.EqualFold(valueType, "Boolean") {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA020",
		Message:  fmt.Sprintf("%s %q uses %s expression as a Boolean condition", member.Kind, member.Name, valueType),
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(valueType))),
	}}
}

func (a *Analyzer) checkIRForEachType(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope irSemaScope, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	iterableType := a.inferIRExprType(inst.Expr, scope, model, typ.Name)
	if iterableType == "" {
		return nil
	}
	if strings.EqualFold(iterableType, "Object") {
		return nil
	}
	elementType, ok := semaIterableElementTypeInModel(iterableType, model)
	if !ok && strings.EqualFold(iterableType, "SObject") && semaIRExprLooksLikeCustomRelationship(inst.Expr) {
		elementType, ok = "SObject", true
	}
	if !ok {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA024",
			Message:  fmt.Sprintf("%s %q enhanced-for iterates non-collection type %s", member.Kind, member.Name, iterableType),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(iterableType))),
		}}
	}
	targetType := resolveNestedTypeReference(model, typ.Name, inst.Type)
	if strings.EqualFold(iterableType, "Database.QueryResult") {
		targetBase, targetArgs := semaGenericBaseAndArgs(targetType)
		if strings.EqualFold(targetBase, "List") && (len(targetArgs) == 0 || strings.EqualFold(targetArgs[0], "SObject") || isSemaSObjectLike(targetArgs[0], model)) {
			return nil
		}
	}
	if inst.Expr.Kind == ir.ExprSOQL {
		targetBase, targetArgs := semaGenericBaseAndArgs(targetType)
		if strings.EqualFold(targetBase, "List") && (len(targetArgs) == 0 || semaAssignableToType(targetArgs[0], elementType, model)) {
			return nil
		}
	}
	if elementType == "" || semaAssignableToType(targetType, elementType, model) {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA024",
		Message:  fmt.Sprintf("%s %q enhanced-for assigns %s elements to %s variable %q", member.Kind, member.Name, elementType, targetType, inst.Name),
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Name))),
	}}
}

func (a *Analyzer) inferIRExprType(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	switch expr.Kind {
	case ir.ExprLiteral:
		if len(expr.Value) > 1 && strings.HasSuffix(strings.ToLower(expr.Value), "l") && intLiteralPattern.MatchString(expr.Value[:len(expr.Value)-1]) {
			return "Long"
		}
		return inferSemaArgType(expr.Value, scope.flat())
	case ir.ExprVariable:
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(expr.Name)), ".class") {
			return "Type"
		}
		if typ, ok := scope.lookup(expr.Name); ok {
			return typ
		}
		if semaLooksLikeSObjectFieldStringPropertyPath(expr.Name) {
			return "String"
		}
		if semaLooksLikeCustomShareRowCauseToken(expr.Name, model) {
			return "String"
		}
		if root, field, ok := strings.Cut(expr.Name, "."); ok {
			if _, scoped := scope.lookup(root); !scoped {
				if target, staticOK := semaStaticClassFieldPathMemberInContext(model, currentType, root, field); staticOK && !hasModifier(target.member.Modifiers, semaSyntheticStandardSObjectFieldModifier) {
					return target.member.Type
				}
			}
		}
		if semaIRExprLooksLikeStaticSObjectToken(expr.Name, scope, model) {
			if semaLooksLikeSObjectDescribeFieldResultPath(expr.Name) {
				return "Schema.DescribeFieldResult"
			}
			if semaLooksLikeSObjectFieldTokenInModel(expr.Name, model) {
				return "Schema.SObjectField"
			}
			if semaLooksLikeSObjectTypeTokenInModel(expr.Name, model) {
				return "Schema.SObjectType"
			}
		}
		if root, _, hasMember := strings.Cut(expr.Name, "."); !hasMember || root == "" {
			if typ := semaEnumValuePathType(model, expr.Name); typ != "" {
				return typ
			}
		} else if _, scoped := scope.lookup(root); !scoped {
			if typ := semaEnumValuePathType(model, expr.Name); typ != "" {
				return typ
			}
		}
		if root, field, ok := strings.Cut(expr.Name, "."); ok {
			if receiverType := semaIRReceiverType(root, scope, model, currentType); receiverType != "" {
				if target, ok := semaResolveFieldPath(model, receiverType, field); ok {
					return target.member.Type
				}
			}
		}
	case ir.ExprCall:
		if strings.EqualFold(expr.Callee, "__coalesce") && len(expr.Args) == 2 {
			leftType := a.inferIRExprType(expr.Args[0], scope, model, currentType)
			rightType := a.inferIRExprType(expr.Args[1], scope, model, currentType)
			if semaCoalesceSOQLSingletonAssignable(expr.Args[0], leftType, rightType, model) {
				return rightType
			}
			return semaCommonType(leftType, rightType, model)
		}
		if strings.EqualFold(expr.Callee, "__ternary") && len(expr.Args) == 3 {
			trueType := a.inferIRExprType(expr.Args[1], scope, model, currentType)
			falseType := a.inferIRExprType(expr.Args[2], scope, model, currentType)
			return semaCommonType(trueType, falseType, model)
		}
		if (strings.HasPrefix(expr.Callee, "__field:") || strings.HasPrefix(expr.Callee, "__safe_field:")) && expr.Left != nil {
			receiverType := resolveNestedTypeReference(model, currentType, a.inferIRExprType(*expr.Left, scope, model, currentType))
			if receiverType == "" && semaIRExprLooksLikeTypeReceiver(*expr.Left, scope, model) {
				if receiverPath, ok := semaIRExprTypeReceiverPath(*expr.Left); ok {
					receiverType = resolveNestedTypeReference(model, currentType, receiverPath)
				}
			}
			if receiverType == "" {
				return ""
			}
			field := strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__safe_field:"), "__field:")
			if semaIRExprLooksLikeTypeReceiver(*expr.Left, scope, model) && isSemaSObjectLike(receiverType, model) && semaFieldTokenPart(field) {
				if strings.EqualFold(field, "SObjectType") {
					return "Schema.SObjectType"
				}
				return "Schema.SObjectField"
			}
			if target, ok := semaResolveFieldPath(model, receiverType, field); ok {
				// Native provenance C009-C013/C020-C021: null safe reads
				// still have the final field type resolved in its declaring owner.
				return resolveNestedTypeReference(model, target.owner, target.member.Type)
			}
			return ""
		}
		if strings.HasPrefix(expr.Callee, "__assign:") {
			name := strings.TrimPrefix(expr.Callee, "__assign:")
			if typ, ok := scope.lookup(name); ok {
				return typ
			}
			if len(expr.Args) == 1 {
				return a.inferIRExprType(expr.Args[0], scope, model, currentType)
			}
			return ""
		}
		if strings.HasPrefix(expr.Callee, "__cast:") {
			return resolveNestedTypeReference(model, currentType, strings.TrimPrefix(expr.Callee, "__cast:"))
		}
		if strings.HasPrefix(expr.Callee, "new:") {
			return resolveNestedTypeReference(model, currentType, strings.TrimPrefix(expr.Callee, "new:"))
		}
		if strings.HasPrefix(expr.Callee, "newlit:") {
			return resolveNestedTypeReference(model, currentType, strings.TrimPrefix(expr.Callee, "newlit:"))
		}
		if typ := a.inferFlattenedIRCallType(expr, scope, model, currentType); typ != "" {
			return typ
		}
		if expr.Left != nil {
			receiverType := a.inferIRExprType(*expr.Left, scope, model, currentType)
			return a.inferIRCallTypeWithReceiver(expr, receiverType, scope, model, currentType)
		}
		if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok {
			receiverType := semaTextReceiverType(receiver, scope.flatCopy(), model)
			return semaResolvedIRCallReturnType(a, model, receiverType, method, expr.Args, scope, currentType, semaTextCallReceiverMode(receiver, scope.flat(), model))
		}
		return semaResolvedIRCallReturnType(a, model, currentType, expr.Callee, expr.Args, scope, currentType, "implicit")
	case ir.ExprUnary:
		switch expr.Operator {
		case "!":
			return "Boolean"
		case "-", "~":
			if expr.Left != nil {
				return a.inferIRExprType(*expr.Left, scope, model, currentType)
			}
			if expr.Right != nil {
				return a.inferIRExprType(*expr.Right, scope, model, currentType)
			}
		}
	case ir.ExprBinary:
		leftType := ""
		rightType := ""
		if expr.Left != nil {
			leftType = a.inferIRExprType(*expr.Left, scope, model, currentType)
		}
		if expr.Right != nil {
			rightType = a.inferIRExprType(*expr.Right, scope, model, currentType)
		}
		return semaBinaryType(expr.Operator, leftType, rightType)
	case ir.ExprSOQL:
		return semaSOQLLiteralType(expr.Value)
	}
	return ""
}

// inferIRCallTypeWithReceiver is the ordinary receiver-call branch of inference.
// A caller may reuse a receiver type only while its scope and model are unchanged.
func (a *Analyzer) inferIRCallTypeWithReceiver(expr ir.Expr, receiverType string, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	method := expr.Callee
	if _, cutMethod, ok := strings.Cut(expr.Callee, "."); ok {
		method = cutMethod
	}
	method = strings.TrimPrefix(method, "__safe_call:")
	if sig, ok := semaEnumMethodSignature(model, receiverType, method); ok {
		return sig.returnType
	}
	if typ := semaResolvedIRCallReturnType(a, model, receiverType, method, expr.Args, scope, currentType, semaIRCallReceiverMode(expr, scope, model)); typ != "" {
		return typ
	}
	if sig, ok := semaSObjectCloneSignature(model, receiverType, method); ok {
		return sig.returnType
	}
	if sig, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return sig.returnType
	}
	if strings.EqualFold(method, "set") && len(expr.Args) == 2 && isSemaSObjectLike(receiverType, model) {
		return a.inferIRExprType(expr.Args[1], scope, model, currentType)
	}
	if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok {
		return sig.returnType
	}
	return ""
}

func semaSOQLLiteralType(queryText string) string {
	if soql.IsSOSLFind(queryText) {
		return "List<List<SObject>>"
	}
	if semaLooksLikeSOQLCountLiteral(queryText) {
		return "Integer"
	}
	query, err := soql.Parse(queryText)
	if err == nil && query.Count {
		return "Integer"
	}
	if err == nil && (len(query.Aggregates) > 0 || len(query.GroupBy) > 0 || query.Having != nil) {
		return "List<AggregateResult>"
	}
	if semaLooksLikeSOQLAggregateLiteral(queryText) {
		return "List<AggregateResult>"
	}
	if err == nil && strings.TrimSpace(query.Object) != "" {
		return "List<" + query.Object + ">"
	}
	if objectName := semaSOQLLiteralFallbackObject(queryText); objectName != "" {
		return "List<" + objectName + ">"
	}
	return "Database.QueryResult"
}

func semaCoalesceSOQLSingletonAssignable(left ir.Expr, leftType, rightType string, model *semaTypeMemberView) bool {
	return left.Kind == ir.ExprSOQL && semaSOQLSingletonAssignable(rightType, leftType, "["+left.Value+"]", model)
}

func semaLooksLikeSOQLCountLiteral(queryText string) bool {
	queryText = strings.TrimSpace(queryText)
	if strings.HasPrefix(queryText, "[") && strings.HasSuffix(queryText, "]") {
		queryText = strings.TrimSpace(queryText[1 : len(queryText)-1])
	}
	normalized := strings.NewReplacer("(", " ( ", ")", " ) ").Replace(queryText)
	tokens := strings.Fields(normalized)
	if len(tokens) < 5 {
		return false
	}
	return strings.EqualFold(tokens[0], "SELECT") &&
		strings.EqualFold(tokens[1], "COUNT") &&
		tokens[2] == "(" &&
		tokens[3] == ")" &&
		strings.EqualFold(tokens[4], "FROM")
}

func semaLooksLikeSOQLAggregateLiteral(queryText string) bool {
	lower := strings.ToLower(queryText)
	if strings.Contains(lower, " group by ") || strings.Contains(lower, " having ") {
		return true
	}
	for _, fn := range []string{"count", "count_distinct", "sum", "avg", "min", "max", "grouping"} {
		if strings.Contains(lower, fn+"(") || strings.Contains(lower, fn+" (") {
			return true
		}
	}
	return false
}

func semaIRExprLooksLikeStaticSObjectToken(expr string, scope irSemaScope, model *semaTypeMemberView) bool {
	root, _, ok := strings.Cut(strings.TrimSpace(expr), ".")
	if !ok || root == "" {
		return false
	}
	if scopedType, scoped := scope.lookup(root); scoped {
		return root == scopedType
	}
	return semaLooksLikeSObjectFieldTokenInModel(expr, model) || semaLooksLikeSObjectTypeTokenInModel(expr, model)
}

func semaIRReceiverType(receiver string, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	switch {
	case strings.EqualFold(receiver, "this"):
		return currentType
	case strings.EqualFold(receiver, "super"):
		if members, ok := model.lookup(normalizeName(currentType)); ok {
			return members.superClass
		}
	case receiver == "":
		return ""
	default:
		if scoped, ok := scope.lookup(receiver); ok {
			return scoped
		}
		if members, ok := model.lookup(normalizeName(receiver)); ok && semaPlatformReceiverSpellingMatches(receiver, members) {
			return receiver
		}
	}
	return ""
}

func (a *Analyzer) inferFlattenedIRCallType(expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	// Both text helpers require a parenthesized call. Ordinary IR callees
	// cannot reach inference and need no mutable copy of the flat scope.
	if expr.Left != nil || !strings.Contains(expr.Callee, "(") {
		return ""
	}
	scopeMap := scope.flatCopy()
	if typ := inferSemaDescribeFieldChainType(expr.Callee, scopeMap, model); typ != "" {
		return typ
	}
	if typ := inferSemaMethodCallType(expr.Callee, scopeMap, model); typ != "" {
		return typ
	}
	return ""
}

func semaResolvedIRCallReturnType(a *Analyzer, model *semaTypeMemberView, receiverType, method string, args []ir.Expr, scope irSemaScope, currentType, receiverMode string) string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = a.inferIRExprType(arg, scope, model, currentType)
	}
	if stubbedType := semaCreateStubReturnTypeFromIR(model, receiverType, method, args, currentType); stubbedType != "" {
		return stubbedType
	}
	if sig, ok := semaEnumMethodSignature(model, receiverType, method); ok {
		return sig.returnType
	}
	if returnType := semaApprovalActionReturnType(receiverType, method, argTypes); returnType != "" {
		return returnType
	}
	candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, receiverType, method), receiverMode)
	platformBackedCandidates := semaResolvedMembersAllPlatformBacked(model, candidates)
	if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok && !platformBackedCandidates {
		return semaResolvedMemberReturnType(model, candidate)
	}
	if semaProjectTypeShadowsPlatform(model, receiverType) {
		return ""
	}
	if sig, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return sig.returnType
	}
	if semaDatabaseDynamicQueryResultCall(receiverType, method) {
		return "Database.QueryResult"
	}
	if returnType := semaDatabaseDMLReturnType(receiverType, method, argTypes); returnType != "" {
		return returnType
	}
	if sig, ok := semaSObjectCloneSignature(model, receiverType, method); ok {
		return sig.returnType
	}
	if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok {
		return semaResolvedMemberReturnType(model, candidate)
	}
	if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok {
		return sig.returnType
	}
	return ""
}

func semaBinaryType(op, leftType, rightType string) string {
	switch op {
	case "&&", "||":
		if strings.EqualFold(leftType, "Boolean") && strings.EqualFold(rightType, "Boolean") {
			return "Boolean"
		}
	case "==", "!=", "<=", ">=", "<", ">":
		return "Boolean"
	case "+":
		if strings.EqualFold(leftType, "String") || strings.EqualFold(rightType, "String") {
			return "String"
		}
		return semaNumericResultType(leftType, rightType)
	case "-", "*", "/", "%":
		return semaNumericResultType(leftType, rightType)
	case "&", "|", "^":
		if strings.EqualFold(leftType, "Boolean") && strings.EqualFold(rightType, "Boolean") {
			return "Boolean"
		}
		return semaIntegralResultType(leftType, rightType)
	case "<<", ">>", ">>>":
		if isSemaIntegralType(leftType) && isSemaIntegralType(rightType) {
			return leftType
		}
	}
	return ""
}

func semaIntegralResultType(leftType, rightType string) string {
	if !isSemaIntegralType(leftType) || !isSemaIntegralType(rightType) {
		return ""
	}
	if strings.EqualFold(leftType, "Long") || strings.EqualFold(rightType, "Long") {
		return "Long"
	}
	return "Integer"
}

func semaNumericResultType(leftType, rightType string) string {
	for _, typ := range []string{"Decimal", "Double", "Long", "Integer"} {
		if strings.EqualFold(leftType, typ) || strings.EqualFold(rightType, typ) {
			if isSemaNumericType(leftType) && isSemaNumericType(rightType) {
				return typ
			}
		}
	}
	return ""
}

func irAssignmentTargetType(name string, scope irSemaScope, model *semaTypeMemberView, currentType string) (string, bool) {
	if typ, ok := scope.lookup(name); ok {
		return typ, true
	}
	root, field, ok := strings.Cut(name, ".")
	if !ok {
		return "", false
	}
	receiverType := semaIRReceiverType(root, scope, model, currentType)
	if receiverType == "" {
		return "", false
	}
	fieldType := semaFieldScope(model, receiverType, make(map[string]bool))[normalizeName(field)]
	return fieldType, fieldType != ""
}

func (a *Analyzer) irVariableDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, name string, scope irSemaScope, model *semaTypeMemberView, start int, source string, reading bool) (diagnostic.Diagnostic, bool) {
	if item, rejected := a.pageReferenceDiagnostic(typ, name, scope, model, start, source); rejected {
		return item, true
	}
	if field, ok := semaTriggerContextField(name, scope, model); ok && !semaKnownTriggerContextField(field) {
		return semaTriggerDiagnostic(typ, "Variable does not exist: "+field, start, start+max(1, len(name)), source), true
	}
	if semaUnavailableTriggerOperation(name, scope, model) {
		return semaTriggerDiagnostic(typ, "Variable does not exist: BEFORE_UNDELETE", start, start+max(1, len(name)), source), true
	}
	root, field, hasMember := strings.Cut(name, ".")
	if !hasMember {
		return diagnostic.Diagnostic{}, false
	}
	if semaIRExprLooksLikeStaticSObjectToken(name, scope, model) {
		return diagnostic.Diagnostic{}, false
	}
	receiverType := ""
	valueReceiver := false
	switch {
	case strings.EqualFold(root, "this"):
		receiverType = typ.Name
		valueReceiver = true
	case strings.EqualFold(root, "super"):
		if members, ok := model.lookup(normalizeName(typ.Name)); ok {
			receiverType = members.superClass
			valueReceiver = true
		}
	default:
		if scoped, ok := scope.lookup(root); ok {
			receiverType = scoped
			valueReceiver = true
		} else if resolved := resolveNestedTypeName(model, typ.Name, root); resolved != "" {
			if _, ok := model.lookup(normalizeName(resolved)); ok {
				receiverType = resolved
			}
		} else if _, ok := model.lookup(normalizeName(root)); ok {
			receiverType = root
		}
	}
	if receiverType == "" {
		return diagnostic.Diagnostic{}, false
	}
	if valueReceiver && strings.EqualFold(receiverType, "AggregateResult") && !strings.EqualFold(field, "Id") && !strings.EqualFold(field, "SObjectType") && !semaProjectTypeShadowsPlatform(model, receiverType) {
		return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA021", Message: "Variable does not exist: " + field, File: typ.File, Range: semaRange(source, start, start+max(1, len(name)))}, true
	}
	if missing := semaDMLMissingField(model, receiverType, field); missing != "" {
		return semaDMLDiagnostic(typ, "Variable does not exist: "+missing, start, start+max(1, len(name)), source), true
	}
	if !semaProjectTypeShadowsPlatform(model, receiverType) && semaRejectedDataWeaveResultField(receiverType, field) {
		return semaFieldAccessDiagnostic(typ, member, name, "variable is not visible", start, start+max(1, len(name)), source), true
	}
	if valueReceiver && strings.Contains(field, ".") {
		if _, missing, relationship := semaSObjectRelationshipPath(model, receiverType, field); relationship && missing != "" {
			return diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA021",
				Message:  "Variable does not exist: " + missing,
				File:     typ.File,
				Range:    semaRange(source, start, start+max(1, len(name))),
			}, true
		}
	}
	if _, ok := model.lookup(normalizeName(receiverType)); !ok {
		return diagnostic.Diagnostic{}, false
	}
	if strings.EqualFold(field, "class") {
		return diagnostic.Diagnostic{}, false
	}
	if strings.HasSuffix(strings.ToLower(field), ".class") {
		nestedType := strings.TrimSpace(field[:len(field)-len(".class")])
		if resolved := resolveNestedTypeName(model, receiverType, nestedType); resolved != "" {
			if _, ok := model.lookup(normalizeName(resolved)); ok {
				return diagnostic.Diagnostic{}, false
			}
		}
	}
	if _, ok := model.lookup(normalizeName(receiverType + "." + field)); ok {
		return diagnostic.Diagnostic{}, false
	}
	if strings.EqualFold(receiverType, "Schema") && semaLooksLikeSchemaTokenPath(field) {
		return diagnostic.Diagnostic{}, false
	}
	if valueReceiver {
		if target, ok := semaResolveFieldPath(model, receiverType, field); ok {
			// C037/P001/P004 reject reads even within the declaring family;
			// R180/R181/P002 allow writes and P003 preserves explicit getters.
			if reading && target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyHasAccessor(target.member, "get") {
				return semaFieldAccessDiagnostic(typ, member, name, "Variable is not visible: "+target.owner+"."+target.member.Name, start, start+max(1, len(name)), source), true
			}
			if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, field, target, start, start+max(1, len(name)), source, model); blocked {
				return visibilityDiagnostic, true
			}
			return diagnostic.Diagnostic{}, false
		}
	}
	if target, ok := semaResolveNestedStaticField(model, receiverType, field); ok {
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, field, target, start, start+max(1, len(name)), source, model); blocked {
			return visibilityDiagnostic, true
		}
		return diagnostic.Diagnostic{}, false
	}
	if target, ok := semaResolveFieldPath(model, receiverType, field); ok {
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, field, target, start, start+max(1, len(name)), source, model); blocked {
			return visibilityDiagnostic, true
		}
		return diagnostic.Diagnostic{}, false
	}
	// C020: the declared platform interface exposes its own members, even
	// when generated symbols are marked as dependencies.
	if members, ok := model.lookup(normalizeName(receiverType)); ok && members.platform && members.kind == apexast.DeclarationInterface {
		return semaFieldAccessDiagnostic(typ, member, name, "Variable does not exist: "+field, start, start+max(1, len(name)), source), true
	}
	if semaDependencyType(model, receiverType) {
		return diagnostic.Diagnostic{}, false
	}
	if semaFieldPathEntersDependencyType(model, receiverType, field) {
		return diagnostic.Diagnostic{}, false
	}
	if semaDynamicFlowInterviewType(receiverType) {
		return diagnostic.Diagnostic{}, false
	}
	if semaTypeHasMissingSuperclass(model, receiverType, map[string]bool{}) {
		return diagnostic.Diagnostic{}, false
	}
	if members, ok := model.lookup(normalizeName(receiverType)); ok && members.kind == apexast.DeclarationEnum && !members.platform {
		return semaFieldAccessDiagnostic(typ, member, name, "Variable does not exist: "+field, start, start+max(1, len(name)), source), true
	}
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA021",
		Message:  fmt.Sprintf("%s %q references unknown field %q on %s", member.Kind, member.Name, field, receiverType),
		File:     typ.File,
		Range:    semaRange(source, start, start+max(1, len(name))),
	}, true
}

func semaDynamicFlowInterviewType(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	return strings.HasPrefix(strings.ToLower(typeName), "flow.interview.")
}

func semaFieldPathEntersDependencyType(model *semaTypeMemberView, receiverType, fieldPath string) bool {
	currentType := strings.TrimSpace(receiverType)
	for _, part := range strings.Split(fieldPath, ".") {
		if semaDependencyType(model, currentType) {
			return true
		}
		if semaUnknownCascadeType(model, currentType) {
			return true
		}
		part = strings.TrimSpace(part)
		if part == "" {
			return false
		}
		target, ok := semaResolveField(model, currentType, part, make(map[string]bool))
		if !ok {
			if sobjectField, fieldOK := semaOpenSObjectFieldMember(currentType, part, model); fieldOK {
				target = sobjectField
			} else {
				return false
			}
		}
		currentType = target.member.Type
	}
	return semaDependencyType(model, currentType) || semaUnknownCascadeType(model, currentType)
}

func semaCallReceiverEntersDependencyType(model *semaTypeMemberView, currentType, receiverExpr string, scope irSemaScope) bool {
	parts := strings.Split(strings.TrimSpace(receiverExpr), ".")
	if len(parts) < 2 {
		return false
	}
	receiverType := ""
	startIndex := 1
	switch {
	case strings.EqualFold(parts[0], "this"):
		receiverType = currentType
	case strings.EqualFold(parts[0], "super"):
		if members, ok := model.lookup(normalizeName(currentType)); ok {
			receiverType = members.superClass
		}
	case parts[0] != "":
		if scoped, ok := scope.lookup(parts[0]); ok {
			receiverType = scoped
		} else if resolved := resolveNestedTypeName(model, currentType, parts[0]); resolved != "" {
			if members, ok := model.lookup(normalizeName(resolved)); ok {
				receiverType = members.name
			}
		} else if members, ok := model.lookup(normalizeName(parts[0])); ok {
			if !semaPlatformReceiverSpellingMatches(parts[0], members) {
				return false
			}
			receiverType = members.name
		}
	}
	if receiverType == "" || startIndex >= len(parts) {
		return false
	}
	return semaFieldPathEntersDependencyType(model, receiverType, strings.Join(parts[startIndex:], "."))
}

func semaTextCallReceiverEntersDependencyType(model *semaTypeMemberView, receiverExpr string, scope map[string]string) bool {
	parts := strings.Split(strings.TrimSpace(receiverExpr), ".")
	if len(parts) < 2 {
		return false
	}
	currentType := scope[semaCurrentTypeScopeKey]
	receiverType := ""
	startIndex := 1
	switch {
	case strings.EqualFold(parts[0], "this"):
		receiverType = currentType
	case strings.EqualFold(parts[0], "super"):
		if members, ok := model.lookup(normalizeName(currentType)); ok {
			receiverType = members.superClass
		}
	case parts[0] != "":
		if scoped, ok := scope[normalizeName(parts[0])]; ok {
			receiverType = scoped
		} else if resolved := resolveNestedTypeName(model, currentType, parts[0]); resolved != "" {
			if members, ok := model.lookup(normalizeName(resolved)); ok {
				receiverType = members.name
			}
		} else if members, ok := model.lookup(normalizeName(parts[0])); ok {
			if !semaPlatformReceiverSpellingMatches(parts[0], members) {
				return false
			}
			receiverType = members.name
		}
	}
	if receiverType == "" || startIndex >= len(parts) {
		return false
	}
	return semaFieldPathEntersDependencyType(model, receiverType, strings.Join(parts[startIndex:], "."))
}

func semaUnknownExternalType(model *semaTypeMemberView, typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return false
	}
	if _, ok := model.lookup(normalizeName(typeName)); ok {
		return false
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	if !strings.EqualFold(canonical, typeName) {
		if _, ok := model.lookup(normalizeName(canonical)); ok {
			return false
		}
	}
	return strings.Contains(typeName, ".")
}

func semaUnknownCascadeType(model *semaTypeMemberView, typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return false
	}
	base, _ := semaGenericBaseAndArgs(typeName)
	if base != "" {
		typeName = base
	}
	if _, ok := model.lookup(normalizeName(typeName)); ok {
		return false
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	if !strings.EqualFold(canonical, typeName) {
		if _, ok := model.lookup(normalizeName(canonical)); ok {
			return false
		}
	}
	return true
}

func semaResolveNestedStaticField(model *semaTypeMemberView, receiverType, fieldPath string) (resolvedMember, bool) {
	parts := strings.Split(fieldPath, ".")
	if len(parts) < 2 {
		return resolvedMember{}, false
	}
	for i := len(parts) - 1; i > 0; i-- {
		typeName := resolveNestedTypeName(model, receiverType, strings.Join(parts[:i], "."))
		if _, ok := model.lookup(normalizeName(typeName)); !ok {
			continue
		}
		fieldName := strings.Join(parts[i:], ".")
		if strings.Contains(fieldName, ".") {
			continue
		}
		if target, ok := semaResolveField(model, typeName, fieldName, make(map[string]bool)); ok {
			return target, true
		}
	}
	return resolvedMember{}, false
}

func (a *Analyzer) irVariableKnown(name string, scope irSemaScope, model *semaTypeMemberView, currentType string) bool {
	if name == "" || name == "this" || name == "super" {
		return true
	}
	if semaKeywordLiteralType(name) != "" {
		return true
	}
	if strings.HasSuffix(strings.ToLower(name), ".class") {
		typeName := strings.TrimSpace(name[:len(name)-len(".class")])
		if _, args := semaGenericBaseAndArgs(typeName); len(args) > 0 {
			// JSON R134/R146/R207 and the formula evaluator use collection
			// class literals. Resolve their type arguments rather than looking
			// up the parameterized type text as an anonymous variable name.
			for _, ref := range extractTypeNames(resolveNestedTypeReference(model, currentType, typeName)) {
				if !a.hasKnown(ref) {
					return false
				}
			}
			return true
		}
	}
	root, _, hasMember := strings.Cut(name, ".")
	if hasMember {
		if strings.EqualFold(root, "this") || strings.EqualFold(root, "super") {
			return true
		}
		if strings.EqualFold(root, "trigger") {
			return true
		}
		if _, ok := scope.lookup(root); ok {
			return true
		}
		if _, ok := model.lookup(normalizeName(root)); ok {
			return true
		}
		if semaEnumValuePathType(model, name) != "" {
			return true
		}
		if root, fieldPath, ok := strings.Cut(name, "."); ok {
			if target, staticOK := semaStaticClassFieldPathMemberInContext(model, currentType, root, fieldPath); staticOK && !hasModifier(target.member.Modifiers, semaSyntheticStandardSObjectFieldModifier) {
				return true
			}
		}
	}
	if _, ok := scope.lookup(root); ok {
		return true
	}
	if _, ok := scope.lookup(name); ok {
		return true
	}
	if hasMember && (a.hasKnown(root) || model.get(normalizeName(root)).name != "") {
		return true
	}
	if a.hasKnown(name) {
		return true
	}
	return false
}

func constructedTypeName(text string) string {
	name := strings.TrimSpace(text)
	if idx := strings.IndexByte(name, '<'); idx >= 0 {
		name = strings.TrimSpace(name[:idx])
	}
	return name
}

type semaLocal struct {
	name       string
	key        string
	typeName   string
	start      int
	scopeStart int
	scopeEnd   int
}

type semaScopeModel struct {
	base        map[string]string
	locals      []semaLocal
	canonical   *semaCanonicalNames
	flatMemo    *semaScopeFlatMemo
	flatVersion *uint64
}

func (s semaScopeModel) localVisibleAt(name string, pos int) bool {
	key := s.canonicalName(name)
	for _, local := range s.locals {
		if s.localKey(local) == key && pos >= local.start && pos <= local.scopeEnd {
			return true
		}
	}
	return false
}

func (a *Analyzer) collectBodyScopes(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model *semaTypeMemberView) (semaScopeModel, []diagnostic.Diagnostic) {
	return a.collectBodyScopesWithLocalDeclMatches(typ, member, body, bodyOffset, source, base, model, nil)
}

func (a *Analyzer) collectBodyScopesWithLocalDeclMatches(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model *semaTypeMemberView, localDeclMatches [][]int) (semaScopeModel, []diagnostic.Diagnostic) {
	scopes := semaScopeModel{base: base, canonical: a.canonicalNames, flatMemo: &semaScopeFlatMemo{}, flatVersion: new(uint64)}
	var diagnostics []diagnostic.Diagnostic
	if localDeclMatches == nil {
		localDeclMatches = findSemaLocalDeclMatches(body)
	}
	diagnostics = append(diagnostics, declareSemaParameters(typ, member, body, bodyOffset, source, &scopes)...)
	ignored := newSemaIgnoredText(body)
	for _, match := range enhancedForLocalPattern.FindAllStringSubmatchIndex(body, -1) {
		if ignored.contains(match[0]) {
			continue
		}
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		name := strings.TrimSpace(body[match[4]:match[5]])
		if isSemaKeyword(typeName) {
			continue
		}
		scopeStart, scopeEnd := statementOrBlockBoundsAfter(body, semaEnhancedForBodySearchStart(body, match[0], match[1]))
		if scopeStart < match[1] {
			scopeStart = match[1]
		}
		for _, ref := range extractTypeNames(typeName) {
			if !a.hasKnownAtVersion(ref, typ.EffectiveAPIVersion) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA006",
					Message:  fmt.Sprintf("%s %q declares enhanced-for local %q with unknown type %q", member.Kind, member.Name, name, ref),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+match[2], bodyOffset+match[3]),
				})
			}
		}
		diagnostics = append(diagnostics, scopes.declareLocal(typ, member, name, resolveNestedTypeReference(model, typ.Name, typeName), match[5], scopeStart, scopeEnd, bodyOffset, source, match[4], match[5])...)
	}
	for _, match := range lineLocalDeclPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaLocalDeclMatchInIgnoredText(ignored, match) {
			continue
		}
		diagnostics = append(diagnostics, a.collectSemaLocalDecl(typ, member, body, bodyOffset, source, &scopes, model, match)...)
	}
	for _, match := range wrappedLocalDeclPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaLocalDeclMatchInIgnoredText(ignored, match) {
			continue
		}
		diagnostics = append(diagnostics, a.collectSemaLocalDecl(typ, member, body, bodyOffset, source, &scopes, model, match)...)
	}
	for _, match := range noSpaceGenericLocalDeclPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaLocalDeclMatchInIgnoredText(ignored, match) {
			continue
		}
		diagnostics = append(diagnostics, a.collectSemaLocalDecl(typ, member, body, bodyOffset, source, &scopes, model, match)...)
	}
	for _, match := range localDeclMatches {
		if semaLocalDeclMatchInIgnoredText(ignored, match) {
			continue
		}
		diagnostics = append(diagnostics, a.collectSemaLocalDecl(typ, member, body, bodyOffset, source, &scopes, model, match)...)
		diagnostics = append(diagnostics, a.collectAdditionalSemaLocalDecls(typ, member, body, bodyOffset, source, &scopes, model, match)...)
	}
	for _, local := range collectClassicForLocals(body) {
		if isSemaKeyword(local.typeName) {
			continue
		}
		for _, ref := range extractTypeNames(local.typeName) {
			if !a.hasKnownAtVersion(ref, typ.EffectiveAPIVersion) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA006",
					Message:  fmt.Sprintf("%s %q declares for local %q with unknown type %q", member.Kind, member.Name, local.name, ref),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+local.start, bodyOffset+local.start+len(local.typeName)),
				})
			}
		}
		local.typeName = resolveNestedTypeReference(model, typ.Name, local.typeName)
		nameEnd := local.start
		nameStart := nameEnd - len(local.name)
		if nameStart < 0 {
			nameStart = 0
		}
		diagnostics = append(diagnostics, scopes.declareLocal(typ, member, local.name, local.typeName, local.start, local.scopeStart, local.scopeEnd, bodyOffset, source, nameStart, nameEnd)...)
	}
	for _, match := range catchLocalPattern.FindAllStringSubmatchIndex(body, -1) {
		if ignored.contains(match[0]) {
			continue
		}
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		name := strings.TrimSpace(body[match[4]:match[5]])
		scopeStart, scopeEnd := blockBoundsAfter(body, match[1])
		for _, ref := range extractTypeNames(strings.ReplaceAll(typeName, "|", ",")) {
			if !a.hasKnownAtVersion(ref, typ.EffectiveAPIVersion) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA006",
					Message:  fmt.Sprintf("%s %q declares catch local %q with unknown type %q", member.Kind, member.Name, name, ref),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+match[2], bodyOffset+match[3]),
				})
			}
		}
		diagnostics = append(diagnostics, scopes.declareCatchLocal(typ, member, name, resolveNestedTypeReference(model, typ.Name, firstCatchType(typeName)), match[4], scopeStart, scopeEnd, bodyOffset, source, match[4], match[5])...)
	}
	return scopes, diagnostics
}

func declareSemaParameters(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes *semaScopeModel) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	seen := make(map[string]apexast.Parameter)
	for _, param := range member.Parameters {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			continue
		}
		key := normalizeName(name)
		if previous, ok := seen[key]; ok {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA014",
				Message:  fmt.Sprintf("%s %q redeclares local variable %q in the same scope", member.Kind, member.Name, name),
				File:     typ.File,
				Range:    parameterNameRange(param, previous, source),
			})
			continue
		}
		seen[key] = param
		scopes.invalidateFlat()
		scopes.locals = append(scopes.locals, semaLocal{
			name:       name,
			key:        scopes.canonicalName(name),
			typeName:   param.Type,
			start:      -1,
			scopeStart: 0,
			scopeEnd:   len(body),
		})
	}
	return diagnostics
}

func parameterNameRange(param, _ apexast.Parameter, source string) *diagnostic.Range {
	r := param.Range
	if r.Start.Offset >= 0 && r.End.Offset > r.Start.Offset && r.End.Offset <= len(source) {
		out := diagnostic.Range{
			Start: r.Start,
			End:   r.End,
		}
		return &out
	}
	return nil
}

func (s *semaScopeModel) declareLocal(typ typesys.TypeSymbol, member typesys.MemberSymbol, name, typeName string, start, scopeStart, scopeEnd, bodyOffset int, source string, nameStart, nameEnd int) []diagnostic.Diagnostic {
	key := s.canonicalName(name)
	if existing, exists := s.conflictingLocalKey(key, scopeStart, scopeEnd); exists {
		if existing.start == start {
			return nil
		}
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA014",
			Message:  fmt.Sprintf("%s %q redeclares local variable %q in the same scope", member.Kind, member.Name, name),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+nameStart, bodyOffset+nameEnd),
		}}
	}
	s.invalidateFlat()
	s.locals = append(s.locals, semaLocal{name: name, key: key, typeName: typeName, start: start, scopeStart: scopeStart, scopeEnd: scopeEnd})
	return nil
}

// declareCatchLocal keeps a catch parameter within its catch block. A local
// declared in the enclosing block after that catch is not in scope at the
// catch header and may reuse its name.
func (s *semaScopeModel) declareCatchLocal(typ typesys.TypeSymbol, member typesys.MemberSymbol, name, typeName string, start, scopeStart, scopeEnd, bodyOffset int, source string, nameStart, nameEnd int) []diagnostic.Diagnostic {
	key := s.canonicalName(name)
	if existing, exists := s.conflictingCatchLocalKey(key, start, scopeStart, scopeEnd); exists {
		if existing.start == start {
			return nil
		}
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA014",
			Message:  fmt.Sprintf("%s %q redeclares local variable %q in the same scope", member.Kind, member.Name, name),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+nameStart, bodyOffset+nameEnd),
		}}
	}
	s.invalidateFlat()
	s.locals = append(s.locals, semaLocal{name: name, key: key, typeName: typeName, start: start, scopeStart: scopeStart, scopeEnd: scopeEnd})
	return nil
}

func (s semaScopeModel) conflictingLocal(name string, scopeStart, scopeEnd int) (semaLocal, bool) {
	return s.conflictingLocalKey(s.canonicalName(name), scopeStart, scopeEnd)
}

func (s semaScopeModel) conflictingLocalKey(key string, scopeStart, scopeEnd int) (semaLocal, bool) {
	for _, local := range s.locals {
		if s.localKey(local) != key {
			continue
		}
		// Same block, or an existing parent scope that encloses this declaration.
		if local.scopeStart <= scopeStart && local.scopeEnd >= scopeEnd {
			return local, true
		}
	}
	return semaLocal{}, false
}

func (s semaScopeModel) conflictingCatchLocalKey(key string, start, scopeStart, scopeEnd int) (semaLocal, bool) {
	for _, local := range s.locals {
		if s.localKey(local) != key {
			continue
		}
		// A local in an enclosing scope conflicts only if it was declared at
		// the catch header. Sibling catch blocks remain separate.
		if local.scopeStart <= scopeStart && local.scopeEnd >= scopeEnd && local.start <= start {
			return local, true
		}
	}
	return semaLocal{}, false
}

func (s semaScopeModel) canonicalName(name string) string {
	if s.canonical == nil {
		return normalizeName(name)
	}
	return s.canonical.canonical(name)
}

func (s semaScopeModel) localKey(local semaLocal) string {
	if local.key != "" {
		return local.key
	}
	return s.canonicalName(local.name)
}
