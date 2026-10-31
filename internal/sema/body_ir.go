package sema

import (
	"fmt"
	"maps"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const runtimeLoweringDiagnosticCode = "GLADERUNTIME001"

func (a *Analyzer) checkBodyText(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	// Some catalog reference targets are available only in named Apex source.
	// Keep that visibility local to this body, including its hydration cache.
	if typ.Kind == apexast.DeclarationClass && member.Kind != apexast.DeclarationTrigger && semaNamedApexSourceType(typ) {
		a = a.namedApexSourceAnalyzer()
		bodyModel := *model
		bodyModel.namedApexReferenceObjects = true
		bodyModel.namedApexReferenceMembers = nil
		model = &bodyModel
	}
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
			if !semaTypeReferenceKnownAtVersion(a, model, typ.Name, ref, typ.EffectiveAPIVersion) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity:      diagnostic.Error,
					Code:          "GLADESEMA006",
					Message:       fmt.Sprintf("%s %q constructs unknown type %q", member.Kind, member.Name, ref),
					NativeMessage: "Invalid type: " + ref,
					File:          typ.File,
					Range:         semaRange(source, bodyOffset+ctor.start, bodyOffset+ctor.end),
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
		diagnostics = append(diagnostics, a.checkBodyExpressionTypeReferences(typ, member, bodyScan, bodyOffset, source, model)...)
	}
	diagnostics = append(diagnostics, a.checkBodyCalls(typ, member, body, bodyOffset, source, scopes, model)...)
	diagnostics = semaPlatformEventPrioritizeCallDiagnostics(diagnostics, irDiagnostics)
	diagnostics = preserveApexMetadataAssignmentDiagnostics(diagnostics, irDiagnostics, source, model)
	diagnostics = semaSameSpelledSObjectTypeReadPrecedence(diagnostics, irDiagnostics)
	diagnostics = semaMessagingVisibilityDiagnostics(diagnostics, typ.EffectiveAPIVersion, model)
	return dedupeBodyDiagnostics(semaHTTPVisibilityDiagnostics(diagnostics, model))
}

// semaSameSpelledSObjectTypeReadPrecedence keeps the IR "Variable does not
// exist: SObjectType" rejection as the only error on the rejected X.SObjectType
// expression. The text path cannot tell a local from a parameter or field, so
// it still types X.SObjectType as the type token; a type mismatch it reports
// on a range overlapping that expression would be a second error native never
// gives (M1 S109: Integer token = Contact.SObjectType). Errors on other
// expressions of the same line are kept.
func semaSameSpelledSObjectTypeReadPrecedence(diagnostics, irDiagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	key := func(d diagnostic.Diagnostic) string {
		return fmt.Sprintf("%s|%s|%d|%d|%s", d.File, d.Code, d.Range.Start.Offset, d.Range.End.Offset, d.Message)
	}
	var rejected []diagnostic.Diagnostic
	fromIR := make(map[string]bool, len(irDiagnostics))
	for _, d := range irDiagnostics {
		if d.Range == nil {
			continue
		}
		fromIR[key(d)] = true
		if d.Code == "GLADESEMA021" && d.NativeMessage == "Variable does not exist: SObjectType" {
			rejected = append(rejected, d)
		}
	}
	if len(rejected) == 0 {
		return diagnostics
	}
	out := make([]diagnostic.Diagnostic, 0, len(diagnostics))
	for _, d := range diagnostics {
		if d.Range != nil && !fromIR[key(d)] && (d.Code == "GLADESEMA018" || d.Code == "GLADESEMA019" || d.Code == "GLADESEMA023") {
			overlaps := false
			for _, r := range rejected {
				overlaps = overlaps || (d.File == r.File && d.Range.Start.Offset < r.Range.End.Offset && r.Range.Start.Offset < d.Range.End.Offset)
			}
			if overlaps {
				continue
			}
		}
		out = append(out, d)
	}
	return out
}

// semaIRNameSpan finds the dotted name in source at or after the statement
// offset, token by token: whitespace, line breaks and comments may sit
// between the parts (Contact /*c*/ .SObjectType), and string literals and
// comments are not searched. It returns the expression's start and end, or
// the statement offset and false when the name is not found before the
// statement ends.
func semaIRNameSpan(source string, from int, name string) (int, int, bool) {
	parts := strings.Split(name, ".")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		if parts[i] == "" {
			return from, from, false
		}
	}
	if from < 0 || from >= len(source) {
		return from, from, false
	}
	identifierEnd := func(i int) int {
		for i < len(source) && isApexIdentifierChar(source[i]) {
			i++
		}
		return i
	}
	// trivia skips whitespace and comments; it reports false for an
	// unterminated block comment.
	trivia := func(i int) (int, bool) {
		for i < len(source) {
			switch {
			case source[i] == ' ' || source[i] == '\t' || source[i] == '\n' || source[i] == '\r' || source[i] == '\f':
				i++
			case strings.HasPrefix(source[i:], "//"):
				newline := strings.IndexByte(source[i:], '\n')
				if newline < 0 {
					return len(source), true
				}
				i += newline + 1
			case strings.HasPrefix(source[i:], "/*"):
				closing := strings.Index(source[i+2:], "*/")
				if closing < 0 {
					return len(source), false
				}
				i += closing + 4
			default:
				return i, true
			}
		}
		return i, true
	}
	matchChain := func(i int) (int, bool) {
		end := identifierEnd(i)
		if source[i:end] != parts[0] {
			return 0, false
		}
		for _, part := range parts[1:] {
			next, ok := trivia(end)
			if !ok || next >= len(source) || source[next] != '.' {
				return 0, false
			}
			next, ok = trivia(next + 1)
			if !ok {
				return 0, false
			}
			partEnd := identifierEnd(next)
			if source[next:partEnd] != part {
				return 0, false
			}
			end = partEnd
		}
		return end, true
	}
	previousIsDot := false
	for i := from; i < len(source); {
		next, ok := trivia(i)
		if !ok || next >= len(source) {
			break
		}
		i = next
		c := source[i]
		switch {
		case c == ';':
			return from, from, false
		case c == '\'':
			// A string literal; skip escapes and stop at the closing quote.
			i++
			for i < len(source) && source[i] != '\'' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
			i++
			previousIsDot = false
		case isApexIdentifierChar(c):
			if !previousIsDot {
				if end, ok := matchChain(i); ok {
					return i, end, true
				}
			}
			i = identifierEnd(i)
			previousIsDot = false
		default:
			previousIsDot = c == '.'
			i++
		}
	}
	return from, from, false
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
		if (strings.EqualFold(members.name, "Task") || strings.EqualFold(members.name, "Event")) && semaHasAPISuffixFold(fieldKey, "__c") {
			if field, ok := semaResolveFieldByKey(model, "Schema.Activity", fieldKey, exactName, seen); ok {
				field.owner = members.name
				return field, true
			}
		}
		if fieldKey == "userrecordaccess" && storage.IsKnownStandardObject(members.name) && storage.SupportsUserRecordAccessRelationship(members.name) {
			return resolvedMember{owner: members.name, member: semaPlatformRelationshipMember("UserRecordAccess", "UserRecordAccess")}, true
		}
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

// semaOpenStandardSObjectForConstructor reports whether absent constructor
// fields on this standard object cannot be treated as errors.
func semaOpenStandardSObjectForConstructor(members typeMembers, fieldName string) bool {
	if strings.EqualFold(members.name, "AuthSession") {
		return false
	}
	// Custom and managed objects and custom fields have complete metadata,
	// and a field that exists only under an org feature is a compile error
	// when the feature is off. Neither is a catalog gap.
	if strings.Contains(members.name, "__") || strings.Contains(fieldName, "__") {
		return false
	}
	if storage.StandardObjectFieldRequiresFeature(members.name, fieldName) {
		return false
	}
	_, standard := semaStandardSObjectNameForKey(normalizeName(members.name))
	return standard
}

func semaOpenSObjectFieldMember(typeName, fieldName string, model *semaTypeMemberView) (resolvedMember, bool) {
	members, _, ok := semaLookupTypeMembers(model, typeName)
	if !ok || !members.sobject {
		return resolvedMember{}, false
	}
	// AuthSession has a complete captured standard field surface. Missing
	// members must not enter the compatibility fallback for partial schemas.
	if strings.EqualFold(members.name, "AuthSession") {
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
	// Nonnumeric catch-variable unary operators are distinguished.
	irSemaOriginCatch
)

type irSemaBinding struct {
	typ                string
	origin             irSemaOrigin
	rejectedCollection bool
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

func (s irSemaScope) rejectedCollection(name string) bool {
	key := normalizeName(name)
	for i := len(s.frames) - 1; i >= 0; i-- {
		if binding, ok := s.frames[i][key]; ok {
			return binding.rejectedCollection
		}
	}
	return false
}

func (a *Analyzer) rejectedCollectionType(name string, expr ir.Expr, model *semaTypeMemberView, owner string) bool {
	name = resolveNestedTypeReference(model, owner, name)
	base, args := semaGenericBaseAndArgs(name)
	if !isCollectionType(base) {
		return false
	}
	unknown := false
	for _, arg := range args {
		if a.hasKnown(arg) {
			continue
		}
		for _, ref := range extractTypeNames(arg) {
			if !semaTypeReferenceKnownAtVersion(a, model, owner, ref, "") {
				unknown = true
				break
			}
		}
		if unknown {
			break
		}
	}
	if !unknown {
		return false
	}
	if expr.Kind == ir.ExprSOQL {
		element, collection := queryBindCollectionElementType(name)
		query, err := soql.Parse(expr.Value)
		return isListType(name) && collection && err == nil && strings.EqualFold(query.Object, element)
	}
	return expr.Kind == ir.ExprCall && len(expr.Args) == 0 && len(expr.NamedArgs) == 0 &&
		strings.HasPrefix(expr.Callee, "new:") && strings.EqualFold(resolveNestedTypeReference(model, owner, strings.TrimPrefix(expr.Callee, "new:")), name)
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
	diagnostics = append(diagnostics, a.checkIRExpressionTypeReferences(typ, member, program.Instructions, body, bodyOffset, source, model)...)
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
	// exception_implicit_super: an exception keeps its inherited public
	// no-argument constructor after it declares other constructors.
	if _, isException := exceptionTypeMembers(target.name, model); isException {
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

func (a *Analyzer) checkIRExpressionTypeReferences(typ typesys.TypeSymbol, member typesys.MemberSymbol, instructions []ir.Instruction, body string, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
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
		diagnostics = append(diagnostics, a.expressionTypeReferenceDiagnostics(typ, member, typeName, start, source, seen, model)...)
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
	for instructionIndex, inst := range instructions {
		if d, rejected := semaFinalCollectionUnaryAssignment(typ, inst, *scope, bodyOffset+inst.Pos, source); rejected {
			diagnostics = append(diagnostics, d)
			continue
		}
		if d, rejected := a.semaCapturedUnaryAssignment(typ, member, inst, *scope, bodyOffset+inst.Pos, source, model); rejected {
			diagnostics = append(diagnostics, d)
			continue
		}
		rejectedCollection := inst.Op == ir.OpDeclare && a.rejectedCollectionType(inst.Type, inst.Expr, model, typ.Name)
		if inst.Expr.Kind != "" && inst.Op != ir.OpFor && !rejectedCollection {
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
				if (!strings.Contains(ref, ".") && !a.hasKnown(resolved)) || !semaTypeReferenceKnownAtVersion(a, model, typ.Name, ref, typ.EffectiveAPIVersion) || a.qualifiedLifecycleLocalTypeRejected(typ.Name, ref, model) {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA006", Message: "Invalid type: " + ref, NativeMessage: "Invalid type: " + ref, File: typ.File, Range: semaRange(source, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Type)))})
				}
			}
			if message := semaAsyncTypeMessage(inst.Type); message != "" && semaAsyncPlatformType(model, resolveNestedTypeReference(model, typ.Name, inst.Type)) {
				diagnostics = append(diagnostics, semaAsyncDiagnostic(typ, message, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Type)), source))
			} else if (semaAPI67RejectedPlatformType(inst.Type) || semaPlatformTypeUnavailableInModel(typ.EffectiveAPIVersion, inst.Type, model)) && !semaProjectTypeShadowsPlatform(model, inst.Type) {
				diagnostics = append(diagnostics, unsupportedLocalFeatureDiagnostic(typ, member, inst.Type, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Type)), source))
			}
			if !rejectedCollection {
				diagnostics = append(diagnostics, a.checkIRIncrementOrExprVariables(typ, member, inst, scope, bodyOffset, source, model, constructability)...)
				diagnostics = append(diagnostics, a.checkIRAssignmentType(typ, member, inst.Type, inst.Name, inst.Expr, scope, inst.Pos, bodyOffset, source, model, "initializes")...)
			}
			if !scope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type)) {
				diagnostics = append(diagnostics, irRedeclareDiagnostic(typ, member, inst.Name, inst.Pos, bodyOffset, source))
			} else if rejectedCollection {
				frame := scope.frames[len(scope.frames)-1]
				binding := frame[normalizeName(inst.Name)]
				binding.rejectedCollection = true
				frame[normalizeName(inst.Name)] = binding
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
					} else if strings.EqualFold(catchType, "Exception") && !semaProjectTypeShadowsPlatform(model, catchType) {
						scope.markFinalCollectionCatch(catchClause.Name)
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
			diagnostics = append(diagnostics, forLoopScopeSwitchDiagnostics(typ, member, inst, instructions[instructionIndex+1:], *scope, selectorType, bodyOffset, source, model)...)
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
// type retains captured local-spelling and inheritance restrictions when
// its owner is a complete project declaration. Built-in value/collection types
// captured as String, Integer and List cannot be qualified with System here;
// qualified call receivers still can. Other built-ins retain their current rules.
func (a *Analyzer) qualifiedLifecycleLocalTypeRejected(currentType, spelling string, model *semaTypeMemberView) bool {
	root, localName, qualified := strings.Cut(spelling, ".")
	if !qualified {
		return false
	}
	if members, known := model.lookupName(spelling); known && !members.platform && !members.dependency && semaSourceTypeInCallingScope(model, currentType, members) {
		return false
	}
	if strings.EqualFold(root, "System") && (strings.EqualFold(localName, "String") || strings.EqualFold(localName, "Integer") || strings.EqualFold(localName, "List")) {
		return true
	}
	// A declared dependency namespace is open even when its source is opaque
	// (v0.3.1 a recorded revision), and a qualifier that also names a source subclass
	// still denotes that namespace.
	if a.deps[normalizeName(root)] {
		return false
	}
	// The project's own or a loaded sub-project's namespace names project
	// source; only a type that source declares resolves through it.
	if semaQualifierNamesProjectSource(a, root) {
		if members, known := model.lookupName(localName); known && !members.platform && !members.dependency {
			return false
		}
		return true
	}
	if members, known := model.lookupName(root); known && members.dependency {
		return false
	}
	// Inner types inherited into an unqualified scope cannot be named
	// through the subclass as though declared there.
	if end := strings.LastIndexByte(spelling, '.'); end >= 0 {
		owner, name := spelling[:end], spelling[end+1:]
		if members, known := model.lookupName(owner); known && semaSourceTypeInCallingScope(model, currentType, members) {
			if inherited := resolveNestedTypeNameFromSuperclasses(model, owner, name); inherited != "" {
				return true
			}
		}
	}
	if _, knownRoot := a.known[a.canonicalName(root)]; knownRoot {
		return false
	}
	// FormulaEval's platform catalog is partial; a miss proves nothing.
	if semaFormulaEvalQualifiedType(spelling) {
		return false
	}
	// R201/C055/N010/N011: a qualified local type under a namespace that is
	// neither project source, a declared dependency, nor platform is invalid.
	// Other positions keep v0.3.1's open managed-package rule.
	return semaUnknownExternalType(model, resolveNestedTypeReference(model, currentType, spelling))
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
		// Dynamic query results are record lists in a DML operand context.
		// Keep the polymorphic marker for other uses, including singleton
		// assignments, and do not reinterpret a project type with that name.
		if strings.EqualFold(operandTypes[i], "Database.QueryResult") {
			if members, known := model.lookupName(operandTypes[i]); !known || members.platform {
				operandTypes[i] = "List<SObject>"
			}
		}
		if i == 1 && strings.EqualFold(inst.Name, "merge") && len(operandTypes) == 2 {
			mergeIDDuplicates = semaDMLMergeIDDuplicateTypesCompatible(operandTypes[0], operandTypes[1], model)
		}
		if !semaDMLTargetType(operandTypes[i], model) && !(i == 1 && mergeIDDuplicates) {
			return []diagnostic.Diagnostic{irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, "DML requires SObject or SObject list type: "+operandTypes[i])}
		}
		if strings.EqualFold(inst.Name, "insert") {
			if message := semaNamedApexReferenceInsertMessage(semaDMLObjectType(operandTypes[i]), model); message != "" {
				d := irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, message)
				d.NativeMessage = message
				return []diagnostic.Diagnostic{d}
			}
		}
		// Typed events and generated history tables cannot be updated or
		// deleted. Generic SObject operands retain runtime validation.
		if semaPlatformEventType(semaDMLObjectType(operandTypes[i]), model) || semaGeneratedHistoryType(semaDMLObjectType(operandTypes[i]), model) {
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
	if len(operandTypes) > 0 {
		if message := fieldKeyTypedUserUpsert(inst.Name, operandTypes[0], model); message != "" {
			d := irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, message)
			d.NativeMessage = message
			return []diagnostic.Diagnostic{d}
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
	if qualified {
		if qualifier, name, found := strings.Cut(fieldName, "."); found && strings.EqualFold(qualifier, "Fields") {
			fieldName = name
		}
	}
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		return false
	}
	// The identity selector uses ordinary upsert semantics, including on
	// custom settings whose source metadata does not declare the Id field.
	if strings.EqualFold(fieldName, "Id") {
		return true
	}
	fieldSchemaFound := false
	objectSchemaFound := false
	for _, object := range a.queryDeclaredObjects {
		// Activity owns the custom fields shared by Task and Event, including
		// their explicitly declared external-ID capability.
		activityField := (strings.EqualFold(objectName, "Task") || strings.EqualFold(objectName, "Event")) &&
			strings.EqualFold(object.Name, "Activity") && semaHasAPISuffixFold(fieldName, "__c")
		if !activityField && !semaProjectReferencedSchemaAPINamesMatch(a.namespace, object.Name, objectName) {
			continue
		}
		objectSchemaFound = objectSchemaFound || len(object.Fields) > 0
		if (strings.EqualFold(fieldName, "Name") || strings.EqualFold(fieldName, "Id")) && schema.FieldKeyTextNameLookup(object) {
			return true
		}
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
	if diag, rejected := a.dependencyGateMissingSObjectTypeDiagnostic(typ, member, expr, *scope, model, bodyOffset+pos, source); rejected {
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
		if item, rejected := a.queryBindMapVariableDiagnostic(typ, expr, *scope, model, bodyOffset+pos, source); rejected {
			return []diagnostic.Diagnostic{item}
		}
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
			if d, rejected := semaInstanceFinalFieldAssignmentDiagnostic(typ, member, field, true, bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source, model); rejected {
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
			if semaStandardFieldAssignmentReadOnly(model, receiverType, field, scope.anonymous) {
				item := semaFieldAccessDiagnostic(typ, member, name, "field is not writeable", bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)
				item.NativeMessage = "Field is not writeable: " + receiverType + "." + field
				return []diagnostic.Diagnostic{item}
			}
			if target, resolved := semaResolveFieldPath(model, receiverType, field); resolved {
				if d, rejected := semaInstanceFinalFieldAssignmentDiagnostic(typ, member, target, strings.EqualFold(root, "this"), bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source, model); rejected {
					return []diagnostic.Diagnostic{d}
				}
				if target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyAssignmentAllowed(typ, member, target, false, semaReceiverExprLooksLikeType(name[:strings.LastIndex(name, ".")], scope, model), model) {
					return []diagnostic.Diagnostic{typeContractPropertyAssignmentDiagnostic(typ, member, target, false, bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
				}
				if !semaProjectTypeShadowsPlatform(model, receiverType) && semaAPI67ReadOnlyPlatformField(target.owner+"."+target.member.Name) {
					return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, name, bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
				}
			} else if scope.hasNonFieldBinding(root) && !strings.Contains(field, ".") &&
				strings.EqualFold(semaCanonicalPlatformAlias(receiverType), "Integer") &&
				!semaProjectTypeShadowsPlatform(model, receiverType) {
				// source syntax and version rules C072: an unresolved field on an Integer local is rejected
				// on both compilation routes throughout the captured API window.
				return []diagnostic.Diagnostic{semaFieldAccessDiagnostic(typ, member, name, "Variable does not exist: "+field, bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
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
	receiverPath, _, _ := splitSemaMethodPath(expr.Callee)
	if expr.Left != nil && expr.Left.Kind == ir.ExprVariable {
		receiverPath = expr.Left.Name
	}
	if strings.Contains(receiverPath, ".") {
		root, _, _ := strings.Cut(receiverPath, ".")
		if _, scoped := scope.lookup(root); !scoped {
			if d, rejected := semaInnerEnumReceiverFieldAccess(typ, member, receiverPath, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
				return []diagnostic.Diagnostic{d}
			}
		}
	}
	if diagnostics := a.capturedInlineIteratorIRCall(typ, member, expr, scope, pos, bodyOffset, source, model); len(diagnostics) != 0 {
		return diagnostics
	}
	if receiverExpr, method, ok := splitSemaMethodPath(expr.Callee); ok && strings.Contains(receiverExpr, ".") {
		root, _, _ := strings.Cut(receiverExpr, ".")
		_, scoped := scope.lookup(root)
		if !scoped && semaShortSObjectFieldsCall(a, receiverExpr, method, model) {
			message := fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type Schema.SObjectField", method, strings.Join(irCallArgTypes(a, expr.Args, scope, model, typ.Name), ","))
			d := collectionCallDiagnostic(typ, member, method, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)
			d.Message, d.NativeMessage = message, message
			return []diagnostic.Diagnostic{d}
		}
		_, rootType := model.lookupName(resolveNestedTypeName(model, typ.Name, root))
		if !scoped && (rootType || a.hasExternalDependencyName(receiverExpr)) && !semaQualifiedCallReceiverKnown(a, typ.Name, receiverExpr, scope, model, typ.EffectiveAPIVersion) {
			d := unknownCallDiagnostic(typ, member, expr.Callee, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)
			d.NativeMessage = "Variable does not exist: " + receiverExpr
			parts := strings.Split(receiverExpr, ".")
			if strings.EqualFold(parts[0], "Schema") {
				parts = parts[1:]
			}
			if len(parts) == 2 && strings.EqualFold(parts[1], "fields") &&
				!semaProjectTypeShadowsPlatform(model, root) && semaSObjectExistsInModel(parts[0], model) {
				d.NativeMessage = fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type Schema.SObjectField", method, strings.Join(irCallArgTypes(a, expr.Args, scope, model, typ.Name), ","))
			}
			return []diagnostic.Diagnostic{d}
		}
		// The qualified SOAP platform receiver can otherwise be deferred before
		// its captured beginInvoke signature is checked. A local System binding
		// or project type keeps its own receiver semantics.
		if !scoped && strings.EqualFold(receiverExpr, "System.WebServiceCallout") && strings.EqualFold(method, "beginInvoke") && semaLexicalShadowingInner(model, typ.Name, root) == "" {
			rootMembers, rootKnown := model.lookupName(root)
			if !rootKnown || rootMembers.platform || rootMembers.dependency {
				if d, rejected := semaHTTPMethodDiagnostic(typ, receiverExpr, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, "class", bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source); rejected {
					return []diagnostic.Diagnostic{d}
				}
			}
		}
		if semaQualifiedCallReceiverDeferred(a, typ.Name, receiverExpr, scope, model, typ.EffectiveAPIVersion) {
			// No indexed type or value binds this path. An incomplete type
			// catalog cannot make the later static-root fallback reject it.
			return nil
		}
	}
	literalCall := expr.Callee
	if expr.Left != nil && expr.Left.Kind == ir.ExprVariable && strings.HasSuffix(strings.ToLower(expr.Left.Name), ".class") {
		literalCall = expr.Left.Name + "." + expr.Callee
	}
	if d, rejected := semaStubNamespaceLiteralDiagnostic(typ, literalCall, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
		return []diagnostic.Diagnostic{d}
	}
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
		// Native H1XT9-A@62.0/67.0: RecordType.SObjectType is the SobjectType
		// field token as a receiver too, and a field token has no newSObject().
		if _, scoped := scope.lookup(strings.TrimSpace(parts[0])); !scoped && semaSObjectTypeFieldTokenParts(parts, model) {
			message := fmt.Sprintf("Method does not exist or incorrect signature: void newSObject(%s) from the type Schema.SObjectField", strings.Join(irCallArgTypes(a, expr.Args, scope, model, typ.Name), ","))
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
	// Native metadata recovery rejects the list declaration before its receiver
	// can participate in a later call (authorization and request context missing-object/collection controls).
	receiver, _, qualified := strings.Cut(expr.Callee, ".")
	if expr.Left != nil && expr.Left.Kind == ir.ExprVariable {
		receiver, qualified = expr.Left.Name, true
	}
	if qualified && scope.rejectedCollection(receiver) {
		return []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: "Variable does not exist: " + receiver, File: typ.File, Range: semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(receiver)))}}
	}
	if strings.HasPrefix(expr.Callee, "newlit:") {
		typeName := strings.TrimPrefix(expr.Callee, "newlit:")
		diagnostics, _ := a.checkIRCollectionConstructor(typ, member, typeName, expr.Args, scope, pos, bodyOffset, source, model)
		// Collection row C001 is a literal initializer, not a constructor overload.
		if typeName == "List<Integer>" && len(diagnostics) == 1 && len(expr.Args) == 1 && a.inferIRExprType(expr.Args[0], scope, model, typ.Name) == "String" {
			diagnostics[0].NativeMessage = "Initial expression is of incorrect type, expected: Integer but was: String"
		}
		// The captured row changes only the captured rejected user-class Set literal.
		base, elements := semaGenericBaseAndArgs(typeName)
		if strings.EqualFold(base, "Set") && len(elements) == 1 && len(diagnostics) == 1 && len(expr.Args) == 1 && expr.Args[0].Kind == ir.ExprLiteral && a.inferIRExprType(expr.Args[0], scope, model, typ.Name) == "String" {
			if element, ok := model.lookup(normalizeName(elements[0])); ok && capturedEqualsOnlySetElement(element) {
				diagnostics[0].NativeMessage = "Initial expression is of incorrect type, expected: " + elements[0] + " but was: String"
			}
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
	if (strings.HasPrefix(expr.Callee, "__field:") || strings.HasPrefix(expr.Callee, "__safe_field:") || strings.HasPrefix(expr.Callee, "__assignField:")) && expr.Left != nil {
		receiverType := resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(*expr.Left, scope, model, typ.Name))
		field := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__field:"), "__safe_field:"), "__assignField:")
		// A complete custom object or pinned standard-object snapshot closes
		// its custom fields. Partial and external-package schemas stay open.
		if members, _, known := semaLookupTypeMembers(model, receiverType); known && members.sobject && !members.partialSObject && !members.externalPackageSObject && semaHasAPISuffixFold(field, "__c") && (semaHasAPISuffixFold(members.name, "__c") || members.completeSchemaSObject && storage.IsKnownStandardObject(members.name) && strings.HasPrefix(expr.Callee, "__field:")) {
			_, found := semaResolveFieldPath(model, receiverType, field)
			if storage.IsKnownStandardObject(members.name) {
				_, found = semaResolveField(model, receiverType, field, make(map[string]bool))
			}
			if !found {
				message := "Variable does not exist: " + field
				item := semaFieldAccessDiagnostic(typ, member, field, message, bodyOffset+pos, bodyOffset+pos+max(1, len(field)), source)
				item.NativeMessage = message
				return []diagnostic.Diagnostic{item}
			}
		}
		return nil
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
	if d, rejected := semaCapturedCacheLocalReceiverDiagnostic(typ, member, expr, scope, bodyOffset+pos, source, model); rejected {
		return []diagnostic.Diagnostic{d}
	}
	if expr.Left == nil && len(expr.NamedArgs) == 0 {
		if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok && !scope.hasNonFieldBinding(receiver) {
			if d, rejected := semaCacheLexicalRequestCall(typ, member, receiver, method, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
				return []diagnostic.Diagnostic{d}
			}
		}
	}
	if d, rejected := a.semaIRShadowReceiverMissingMethod(typ, member, expr, scope, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
		return []diagnostic.Diagnostic{d}
	}
	// Resolve a constant's enum owner before permissive dotted/static-value
	// fallbacks can accept the call without checking the method.
	if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok {
		if diagnostics, handled := a.checkIREnumConstantCall(typ, member, receiver, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
			return diagnostics
		}
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
	if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok && !scope.hasNonFieldBinding(receiver) && !semaProjectTypeShadowsPlatform(model, receiver) && semaLexicalShadowingInner(model, typ.Name, receiver) == "" {
		if message := semaPlatformEventCallMessage(receiver, method, "class", irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, typ.File != ""); message != "" {
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
	// Field validation depends on the record reference, not its primitive value.
	if a.semaIRSObjectFieldAddError(expr, scope, model, typ.Name) {
		return nil
	}
	// A scoped field path supplies the value receiver before static-type
	// fallbacks can interpret its first component as an SObject name.
	if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok && strings.Contains(receiver, ".") {
		root, _, _ := strings.Cut(receiver, ".")
		if _, scoped := scope.lookup(root); scoped {
			valueType := a.inferIRExprType(ir.Expr{Kind: ir.ExprVariable, Name: receiver}, scope, model, typ.Name)
			if diagnostics, handled := a.checkIRPlatformCall(typ, member, valueType, method, expr.Args, scope, pos, bodyOffset, source, model, "instance"); handled {
				return diagnostics
			}
		}
	}
	receiverType := typ.Name
	method := expr.Callee
	explicitReceiver := false
	classLiteralReceiver := false
	platformClassReceiver := false
	projectClassReceiver := false
	lexicalShadowReceiver := false
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
			if !receiverScoped && semaKnownPlatformTypeReceiver(receiverExpr) && !semaProjectTypeShadowsPlatform(model, receiverExpr) && semaLexicalShadowingInner(model, typ.Name, receiverExpr) == "" {
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
					} else if nested := semaLexicalShadowingInner(model, typ.Name, receiver); nested != "" {
						// A dotted value path (Type.B.name()) is an enum or field
						// value, never a static call on the inner type.
						if strings.Contains(method, ".") {
							return nil
						}
						receiverType = nested
						projectClassReceiver = true
						lexicalShadowReceiver = true
					} else if members, ok := model.lookup(normalizeName(receiver)); ok {
						if !semaPlatformReceiverSpellingMatches(receiver, members) {
							return nil
						}
						receiverType = receiver
					} else if enum := semaLexicalSourceInnerEnum(model, typ.Name, receiver); enum != "" && !strings.Contains(method, ".") {
						receiverType = enum
						projectClassReceiver = true
					} else if a.hasKnown(receiver) {
						return nil
					} else {
						receiverExpr, _, _ := splitSemaMethodPath(expr.Callee)
						// A namespace need not have its own symbol. Resolve both
						// qualified types and static field values rooted in them.
						if _, known := model.lookupName(resolveNestedTypeName(model, typ.Name, receiverExpr)); known {
							return nil
						}
						if semaIRStaticValueReceiver(model, typ.Name, receiverExpr) {
							return nil
						}
						if semaIRUnresolvedDeclaredReceiver(receiverExpr, scope, model) || semaQualifiedCallReceiverKnown(a, typ.Name, receiverExpr, scope, model, typ.EffectiveAPIVersion) {
							return nil
						}
						d := unknownCallDiagnostic(typ, member, expr.Callee, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)
						d.NativeMessage = "Variable does not exist: " + receiverExpr
						return []diagnostic.Diagnostic{d}
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
	if receiverDependencyRejectionAPI(typ) && len(receiverDependencyCallPeers(typ, receiverType, expr.Callee, model)) != 0 {
		if d, rejected := receiverDependencyCallDiagnostic(typ, member, receiverType, method, expr.Callee, receiverMode, irCallArgTypes(a, expr.Args, scope, model, typ.Name), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
			return []diagnostic.Diagnostic{d}
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
	if message := semaPlatformEventCallMessage(receiverType, method, receiverMode, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model, typ.File != ""); message != "" {
		return []diagnostic.Diagnostic{semaPlatformEventDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	if strings.EqualFold(receiverType, "Finalizer") || strings.EqualFold(receiverType, "System.Finalizer") {
		if message := semaFinalizerCallMessage(receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name), model); message != "" {
			return []diagnostic.Diagnostic{semaAsyncDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
		}
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
	if diagnostics, handled := a.checkIRVisualforceCall(typ, expr, receiverType, method, receiverMode, scope, model, bodyOffset+pos, source); handled {
		return diagnostics
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
		if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr, scope, pos, bodyOffset, source, model); handled {
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
		if selected, rejected := semaInnerReceiverMissingMethodType(receiverType, method, model, expr.Left != nil && expr.Left.Kind == ir.ExprCall && strings.HasPrefix(expr.Left.Callee, "new:")); rejected {
			d := semaShadowMissingMethod(typ, member, expr.Callee, method, selected, irCallArgTypes(a, expr.Args, scope, model, typ.Name), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)
			return []diagnostic.Diagnostic{d}
		}
		// A lexical inner type shadows the platform type it is named after; a
		// missing method cannot fall back to the platform signature.
		if !lexicalShadowReceiver && expr.Left == nil && !strings.Contains(method, ".") {
			// A missing static method on a lexical inner enum has no platform
			// fallback (testdata/inner_error_order/org.tsv). Methods
			// every enum declares keep their existing resolution.
			if receiver, _, ok := strings.Cut(expr.Callee, "."); ok {
				if _, scoped := scope.lookup(receiver); !scoped {
					if enum := semaLexicalSourceInnerEnum(model, typ.Name, receiver); enum != "" && !semaEnumTypeHasMethod(model, enum, method) {
						receiverType = enum
						lexicalShadowReceiver = true
					}
				}
			}
		}
		if lexicalShadowReceiver {
			return []diagnostic.Diagnostic{semaShadowMissingMethod(typ, member, expr.Callee, method, receiverType, irCallArgTypes(a, expr.Args, scope, model, typ.Name), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
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
		if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr, scope, pos, bodyOffset, source, model); handled {
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
			if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr, scope, pos, bodyOffset, source, model); handled {
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
				if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverTyp, methodName, expr, scope, pos, bodyOffset, source, model); handled {
					return diagnostics
				}
				if semaTypeReferenceKnownAtVersion(a, model, typ.Name, receiverTyp, typ.EffectiveAPIVersion) {
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
		if explicitReceiver && semaTypeReferenceKnownAtVersion(a, model, typ.Name, receiverType, typ.EffectiveAPIVersion) {
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
			if lexicalShadowReceiver {
				staticDiagnostic = semaLexicalShadowStaticAccessNative(staticDiagnostic, candidate, receiverMode)
			}
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
		return []diagnostic.Diagnostic{nativeSourceOverloadDiagnostic(ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source), receiverType, method, argTypes, candidates, model, typ)}
	}
	if semaResolvedMembersAllPlatformBacked(model, candidates) {
		if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model, receiverMode); handled {
			return diagnostics
		}
		if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr, scope, pos, bodyOffset, source, model); handled {
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
			return []diagnostic.Diagnostic{nativeSourceOverloadDiagnostic(ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source), receiverType, method, textArgTypes, candidates, model, typ)}
		}
	}
	if sourceArgTypes := irSourceArgTypesForCall(source, bodyOffset+pos, expr.Callee, scope, model); len(sourceArgTypes) == len(expr.Args) {
		// Native provenance C009-C013/C020-C021: an unknown text fallback
		// cannot erase the known final type of an IR field/safe-navigation read.
		if _, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, sourceArgTypes, model); ok &&
			(!preserveFieldTypes || !semaArgTypesContainUnknown(sourceArgTypes)) {
			return nil
		} else if ambiguous && !semaArgTypesContainUnknown(sourceArgTypes) {
			return []diagnostic.Diagnostic{nativeSourceOverloadDiagnostic(ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source), receiverType, method, sourceArgTypes, candidates, model, typ)}
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
	return []diagnostic.Diagnostic{nativeSourceOverloadDiagnostic(d, receiverType, method, argTypes, candidates, model, typ)}
}

func (a *Analyzer) checkIREnumConstantCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiver, method string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) ([]diagnostic.Diagnostic, bool) {
	root, _, _ := strings.Cut(receiver, ".")
	if _, bound := scope.lookup(root); bound {
		return nil, false
	}
	enumPath, label, ok := splitSemaMethodPath(receiver)
	if !ok {
		return nil, false
	}
	enumType := resolveNestedTypeName(model, typ.Name, enumPath)
	valueType := semaEnumValuePathType(model, enumType+"."+label)
	if valueType == "" {
		return nil, false
	}
	// Keep the explicit System spelling in native diagnostics; source inner
	// enums use their resolved lexical owner instead.
	displayType := valueType
	if semaExplicitPlatformEnumType(enumPath) {
		displayType = enumPath
	}
	// Look up the bare name first, then the explicit System spelling: a project
	// class with the bare enum name must not hide the platform enum's methods,
	// and methods catalogued only under the bare name stay visible when the
	// call is System.-qualified.
	sig, known := semaEnumMethodSignature(model, valueType, method)
	if !known && displayType != valueType {
		sig, known = semaEnumMethodSignature(model, displayType, method)
	}
	if !known {
		// Enums inherit Object's instance methods in addition to name/ordinal.
		_, known = semaPlatformMethodSignatureForMode(model, valueType, method, "instance")
		if !known && displayType != valueType {
			_, known = semaPlatformMethodSignatureForMode(model, displayType, method, "instance")
		}
	}
	if known && !strings.EqualFold(method, "values") && !strings.EqualFold(method, "valueOf") {
		return nil, true
	}
	// Native rows cover only values() and valueOf(String). Other arities have
	// no captured answer, so leave them to the existing call checks.
	if known && len(sig.params) > 0 && len(args) != len(sig.params[0]) {
		return nil, false
	}
	d := unknownCallDiagnostic(typ, member, receiver+"."+method, bodyOffset+pos, bodyOffset+pos+max(1, len(receiver)+1+len(method)), source)
	if known {
		returnType := strings.ReplaceAll(sig.returnType, valueType, displayType)
		d.NativeMessage = fmt.Sprintf("Static method cannot be referenced from a non static context: %s %s.%s(%s)", returnType, displayType, method, strings.Join(sig.params[0], ", "))
	} else {
		d.NativeMessage = fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(irCallArgTypes(a, args, scope, model, typ.Name), ", "), displayType)
	}
	d.Message = d.NativeMessage
	return []diagnostic.Diagnostic{d}, true
}

// A type-qualified value path starts with a static field. Once that field
// supplies a value, subsequent fields belong to the value and may be instance
// fields. Checking only the final member incorrectly rejects those chains.
func semaIRStaticValueReceiver(model *semaTypeMemberView, currentType, receiver string) bool {
	parts := strings.Split(receiver, ".")
	for i := 1; i < len(parts); i++ {
		root := strings.Join(parts[:i], ".")
		target, ok := semaStaticClassFieldPathMemberInContext(model, currentType, root, parts[i])
		if !ok {
			continue
		}
		if i == len(parts)-1 {
			return true
		}
		valueType := resolveNestedTypeName(model, target.owner, target.member.Type)
		_, ok = semaResolveFieldPath(model, valueType, strings.Join(parts[i+1:], "."))
		return ok
	}
	return false
}

// A declaration can identify an unavailable package type without supplying its
// members. Keep calls on that same owner unresolved for dependency handling;
// a known suffix or the depth of an otherwise undeclared path is not evidence.
func semaIRUnresolvedDeclaredReceiver(receiver string, scope irSemaScope, model *semaTypeMemberView) bool {
	parts := strings.Split(receiver, ".")
	if len(parts) < 2 {
		return false
	}
	owner := strings.Join(parts[:2], ".")
	if !semaLooksLikeUnconfiguredManagedPackageType(owner) || semaModelHasType(model, owner) {
		return false
	}
	owner = normalizeName(owner)
	for _, declared := range scope.flat() {
		for _, ref := range extractTypeNames(declared) {
			ref = normalizeName(ref)
			if ref == owner || strings.HasPrefix(ref, owner+".") {
				return true
			}
		}
	}
	return false
}

// Keep the local explanation while exposing the native signature for rejected
// calls to resolved source methods. Platform contracts retain their own text.
func nativeSourceOverloadDiagnostic(d diagnostic.Diagnostic, receiverType, method string, argTypes []string, candidates []resolvedMember, model *semaTypeMemberView, typ typesys.TypeSymbol) diagnostic.Diagnostic {
	if d.NativeMessage != "" || len(candidates) == 0 || semaArgTypesContainUnknown(argTypes) {
		return d
	}
	if captured, rejected := namespaceInterfaceOverloadDiagnostic(d, typ, receiverType, method, argTypes, candidates, model); rejected {
		return captured
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
	if expr.Left == nil {
		receiver, method, ok := splitSemaMethodPath(expr.Callee)
		if !ok || !strings.EqualFold(method, "addError") {
			return false
		}
		record, field, ok := splitSemaMethodPath(receiver)
		if !ok {
			return false
		}
		expr.Callee = method
		expr.Left = &ir.Expr{Kind: ir.ExprCall, Callee: "__field:" + field, Left: &ir.Expr{Kind: ir.ExprVariable, Name: record}}
	}
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
	root, field, _ := strings.Cut(path, ".")
	if root != "" {
		if _, scoped := scope.lookup(root); scoped {
			return false
		}
	}
	// A static field holds a value even when its name case-insensitively
	// matches a nested type. Safe navigation applies to that value.
	if field != "" {
		if target, ok := semaStaticClassFieldPathMember(model, root, field); ok && !hasModifier(target.member.Modifiers, semaSyntheticStandardSObjectFieldModifier) {
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

func (a *Analyzer) checkIRCollectionCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) ([]diagnostic.Diagnostic, bool) {
	args := expr.Args
	sig, ok := semaCollectionMethodSignature(receiverType, method)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = a.inferIRExprType(arg, scope, model, typ.Name)
	}
	if diagnostics, handled := inlineQueryListSetDiagnostic(typ, member, receiverType, method, args, argTypes, bodyOffset+pos, source, model, capturedIndexedQueryAssignment(expr, source, bodyOffset+pos)); handled {
		return diagnostics, true
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
	var argTypes []string
	observedPagination := false
	if observedPaginationSourceCallCandidate(typ.EffectiveAPIVersion, receiverType, method, receiverMode) {
		argTypes = make([]string, len(args))
		for i, arg := range args {
			argTypes[i] = resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(arg, scope, model, typ.Name))
		}
		observedPagination = observedPaginationSourceCall(typ.EffectiveAPIVersion, receiverType, method, argTypes, receiverMode, model)
	}
	if (semaPlatformTypeUnavailableInModel(typ.EffectiveAPIVersion, receiverType, model) ||
		semaPlatformMemberUnavailable(typ.EffectiveAPIVersion, receiverType, method)) && !observedPagination {
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
		platformType := semaCanonicalPlatformAlias(receiverType)
		// source syntax and version rules C071 captures unresolved zero-argument Integer instance calls.
		// Lowered increment operators keep the existing increment checks and
		// must not be diagnosed as missing source methods.
		increment := semaIRIncrementCall(ir.Expr{Kind: ir.ExprCall, Callee: method})
		if strings.EqualFold(platformType, "String") ||
			(receiverMode == "instance" && len(args) == 0 && strings.EqualFold(platformType, "Integer") && !increment) {
			return []diagnostic.Diagnostic{valueCollectionCallDiagnostic(unknownCallDiagnostic(typ, member, receiverType+"."+method, bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source), receiverType, method, irCallArgTypes(a, args, scope, model, typ.Name), model)}, true
		}
		return nil, false
	}
	if argTypes == nil {
		argTypes = make([]string, len(args))
		for i, arg := range args {
			argTypes[i] = resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(arg, scope, model, typ.Name))
		}
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
	if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok && semaResolvedMembersAllPlatformBacked(model, candidates) && semaPlatformResolvedMemberUnavailable(typ.EffectiveAPIVersion, candidate) && !observedPagination {
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
	if item.NativeMessage == "" {
		item.NativeMessage = capturedMathAbsAccountNativeMessage(receiverType, method, argTypes, receiverMode, model)
	}
	return []diagnostic.Diagnostic{semaFormulaMethodDiagnostic(item, receiverType, method, argTypes)}, true
}

func (a *Analyzer) checkIRConstructorCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	typeName := strings.TrimPrefix(expr.Callee, "new:")
	resolvedTypeName := resolveNestedTypeReference(model, typ.Name, typeName)
	if members, _, known := semaLookupTypeMembers(model, resolvedTypeName); known && members.sobject {
		features := model.state.base.orgShapeFeatures
		message := ""
		if !storage.StandardObjectConstructorAvailableForFeatures(members.name, features) {
			message = "Invalid type: " + resolvedTypeName
		} else if len(expr.NamedArgs) > 0 {
			definition := a.standardConstructorDefinition(members.name, features, model)
			for _, arg := range expr.NamedArgs {
				if !storage.StandardObjectConstructorFieldAllowedForFeatures(members.name, arg.Name, features, definition) {
					message = "Field does not exist: " + arg.Name + " on " + resolvedTypeName
					break
				}
			}
		}
		if message != "" {
			item := typeContractNativeDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)
			return []diagnostic.Diagnostic{item}
		}
	}
	// These abstract platform classes live in the lazy member model rather
	// than the project-only constructability map.
	if strings.HasPrefix(strings.ToLower(resolvedTypeName), "cartextension.") {
		if target, ok := model.lookupName(resolvedTypeName); ok && target.platform && hasModifier(target.modifiers, "abstract") {
			d := typeContractNativeDiagnostic(typ, "Abstract classes cannot be constructed: "+resolvedTypeName, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)
			d.Code = "GLADESEMA015"
			return []diagnostic.Diagnostic{d}
		}
	}
	if d, rejected := receiverDependencyConstructionDiagnostic(typ, typeName, resolvedTypeName, len(expr.Args)+len(expr.NamedArgs), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source, model); rejected {
		return []diagnostic.Diagnostic{d}
	}
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
	if members, _, ok := semaLookupTypeMembers(model, resolvedTypeName); ok && !members.platform && !members.sobject {
		// Dotted source classes retain their own constructor contract.
		rejectedConstructor = false
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
	if (rejectedConstructor || semaPlatformTypeUnavailableInModel(typ.EffectiveAPIVersion, typeName, model)) && !semaProjectTypeShadowsPlatform(model, typeName) {
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
		if !semaTypeReferenceKnownAtVersion(a, model, typ.Name, ref, typ.EffectiveAPIVersion) {
			return []diagnostic.Diagnostic{{
				Severity:      diagnostic.Error,
				Code:          "GLADESEMA006",
				Message:       fmt.Sprintf("%s %q constructs unknown type %q", member.Kind, member.Name, ref),
				NativeMessage: "Invalid type: " + ref,
				File:          typ.File,
				Range:         semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName))),
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
		var diagnostics []diagnostic.Diagnostic
		for _, arg := range expr.NamedArgs {
			if message := semaReplayDescribeConstructorField(resolvedTypeName, arg.Name, model); message != "" {
				return []diagnostic.Diagnostic{semaPlatformEventDiagnostic(typ, message, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
			}
			if fieldName, readOnly := semaFormulaReadOnlyField(model, resolvedTypeName, arg.Name); readOnly {
				item := semaFieldAccessDiagnostic(typ, member, fieldName, "field is not writeable", bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)
				item.Message = "Field is not writeable: " + fieldName
				return []diagnostic.Diagnostic{item}
			}
			// Constructor initializers require a declared field. Field-path
			// resolution can instead return an opaque compatibility fallback.
			field, exists := semaResolveField(model, resolvedTypeName, arg.Name, make(map[string]bool))
			// Feature compatibility fields in the shared provider are not
			// declarations in this org's metadata or enabled feature profile.
			if exists && storage.StandardObjectFieldRequiresFeature(resolvedTypeName, arg.Name) && !hasModifier(field.member.Modifiers, semaDeclaredFeatureFieldModifier) {
				exists = false
			}
			if !exists {
				definition := a.standardConstructorDefinition(resolvedTypeName, model.state.base.orgShapeFeatures, model)
				if name, declared := storage.ResolveFieldName(definition, "", arg.Name); declared {
					declaredField := definition.Fields[name]
					if declaredField.Formula != "" {
						fieldName := resolvedTypeName + "." + name
						item := semaFieldAccessDiagnostic(typ, member, fieldName, "field is not writeable", bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)
						item.Message = "Field is not writeable: " + fieldName
						return []diagnostic.Diagnostic{item}
					}
					field.member.Type = semaApexTypeForStorageField(declaredField)
					exists = true
				}
			}
			if !exists {
				// The standard baseline is one clean org's describe export, so it
				// can omit real standard fields (for example Account.Tradestyle).
				// Standard objects keep the open fallback, as field reads do.
				// AuthSession has a complete captured surface and stays strict.
				// Person custom fields come from Contact metadata even when the
				// project omits the Account object definition.
				personCustomField := semaHasAPISuffixFold(arg.Name, "__pc")
				if members, _, known := semaLookupTypeMembers(model, resolvedTypeName); known && members.sobject && (!members.partialSObject || personCustomField && strings.EqualFold(members.name, "Account")) && !strings.EqualFold(members.name, "SObject") && !semaOpenStandardSObjectForConstructor(members, arg.Name) {
					message := "Field does not exist: " + arg.Name + " on " + resolvedTypeName
					item := semaFieldAccessDiagnostic(typ, member, arg.Name, message, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)
					item.NativeMessage = message
					diagnostics = append(diagnostics, item)
				}
				continue
			}
			// Partial schema fields may exist without a known Apex value type.
			if field.member.Type != "" {
				target := resolvedTypeName + "." + arg.Name
				items := a.checkIRAssignmentType(typ, member, field.member.Type, target, arg.Expr, &scope, pos, bodyOffset, source, model, "initializes")
				for i := range items {
					valueType := resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(arg.Expr, scope, model, typ.Name))
					items[i].NativeMessage = "Illegal assignment from " + valueType + " to " + field.member.Type
				}
				diagnostics = append(diagnostics, items...)
			}
		}
		return diagnostics
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
		// Native API 62/67/68, both routes: a String entry cannot initialize
		// a Schema.SObjectField value. Check the captured String-key shape
		// before the copy-constructor shortcut: __mapEntry itself has no
		// inferred type, so a single entry otherwise looks like an unknown arg.
		if len(args) == 1 && strings.EqualFold(params[0], "String") && strings.EqualFold(params[1], "Schema.SObjectField") {
			arg := args[0]
			if arg.Kind == ir.ExprCall && arg.Callee == "__mapEntry" && len(arg.Args) == 2 {
				entryKeyType := a.inferIRExprType(arg.Args[0], scope, model, typ.Name)
				entryValueType := a.inferIRExprType(arg.Args[1], scope, model, typ.Name)
				if strings.EqualFold(entryKeyType, "String") && strings.EqualFold(entryValueType, "String") {
					d := collectionConstructorDiagnostic(typ, member, typeName, len(args), bodyOffset+pos, source)
					d.Message = "Invalid value type String for Schema.SObjectField"
					d.NativeMessage = d.Message
					return []diagnostic.Diagnostic{d}, true
				}
			}
		}
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
		if !semaTypeReferenceKnownAtVersion(a, model, typ.Name, ref, typ.EffectiveAPIVersion) {
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
	if member.Kind != apexast.DeclarationField && verb == "initializes" && !strings.Contains(target, ".") && semaGenericInterfaceDirectAssignmentRejected(targetType, valueType, expr, model) {
		return []diagnostic.Diagnostic{typeContractDiagnostic(typ, member, "generic interface initialization requires an explicit cast", bodyOffset+pos, bodyOffset+pos+max(1, len(target)), source)}
	}
	if expr.Kind == ir.ExprVariable && strings.EqualFold(targetType, "Object") && !strings.EqualFold(valueType, "null") && !semaTypeReferenceKnownAtVersion(a, model, typ.Name, valueType, typ.EffectiveAPIVersion) {
		refs := extractTypeNames(valueType)
		if len(refs) == 1 && strings.EqualFold(refs[0], valueType) {
			return []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "GLADESEMA018", Message: "Illegal assignment from " + valueType + " to Object", File: typ.File, Range: semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(target)))}}
		}
	}
	if strings.EqualFold(valueType, "void") && semaIRExprLooksLikeIndexAssignmentValue(expr, source, bodyOffset+pos) {
		valueType = resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(expr.Args[1], *scope, model, typ.Name))
	}
	if valueType == "" || valueType == "null" || semaAssignableToType(targetType, valueType, model) || semaIRPlatformEnumTypeFieldAssignable(targetType, target, valueType, model) || (expr.Kind == ir.ExprSOQL && semaSOQLSingletonAssignable(targetType, valueType, "["+expr.Value+"]", model)) || semaIRSOQLNullCoalesceAssignable(targetType, valueType, expr, model) || a.semaIRSOQLConditionalAssignable(targetType, expr, *scope, model, typ.Name) {
		return nil
	}
	if semaDMLStringLocaleProperty(targetType, target, valueType, *scope, model, typ.Name) {
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
	if strings.HasPrefix(message, "Illegal assignment from ") {
		d.NativeMessage = message
	} else {
		d.NativeMessage = assignmentDiagnosticMessage(targetType, valueType, model)
	}
	// The captured row preserves rejection of Account assigned to the List-setting
	// Id owner field; the field provider now resolves on both routes.
	if targetType == "Id" && valueType == "Account" && !semaProjectTypeShadowsPlatform(model, valueType) && a.listSettingOwnerAssignment(target, *scope, model, typ.Name) {
		d.NativeMessage = "Illegal assignment from Account to Id"
	}
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
	if valueType == "" || valueType == "null" || semaAssignableToType(returnType, valueType, model) || semaQueryLocatorReturnsIterable(returnType, valueType) || (expr.Kind == ir.ExprSOQL && semaSOQLSingletonAssignable(returnType, valueType, "["+expr.Value+"]", model)) || semaIRSOQLNullCoalesceAssignable(returnType, valueType, expr, model) || a.semaIRSOQLConditionalAssignable(returnType, expr, *scope, model, typ.Name) {
		return nil
	}
	if strings.EqualFold(returnType, "Boolean") && semaMemberReturnSourceLooksBoolean(source, 0, len(source)) {
		return nil
	}
	d := returnTypeDiagnostic(typ, member, fmt.Sprintf("returns %s from %s method", valueType, returnType), bodyOffset+pos, bodyOffset+pos+max(1, len(valueType)), source)
	if native := semaQueryLocatorReturnNativeMessage(returnType, valueType, model, typeUsesAPIVersionAtLeast(typ, 62) && capturedInlineIntegerIterator(expr, model)); native != "" {
		d.NativeMessage = native
	} else if native := semaInnerReturnConversion(returnType, valueType, model); native != "" {
		d.NativeMessage = native
	} else if member.Kind == apexast.DeclarationMethod && len(member.Parameters) == 0 {
		d.NativeMessage = capturedListReturnConversion(returnType, valueType, model)
	}
	return []diagnostic.Diagnostic{d}
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
	elementType, ok := semaInheritedIterableElementType(iterableType, model)
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
		if semaIRCustomLabelReference(expr.Name, scope, model, currentType) {
			return "String"
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
			if semaIRSameSpelledLocalSObjectTypeRead(expr.Name, scope, model) {
				return ""
			}
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
		if expr.Operator == "[]=" && expr.Callee == "set" && expr.Left != nil && len(expr.Args) == 2 {
			// Assignment has the slot's declared type; an explicit List.set
			// remains Void and retains its ordinary argument checks.
			target := expr
			target.Callee, target.Operator = "get", "[]"
			target.Args = target.Args[:1]
			return a.inferIRExprType(target, scope, model, currentType)
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
		return sig.returnTypeForArity(len(expr.Args))
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
	if err == nil && strings.TrimSpace(query.Object) != "" {
		return "List<" + query.Object + ">"
	}
	if semaLooksLikeSOQLAggregateLiteral(queryText) {
		return "List<AggregateResult>"
	}
	if objectName := semaSOQLLiteralFallbackObject(queryText); objectName != "" {
		return "List<" + objectName + ">"
	}
	return "Database.QueryResult"
}

func (a *Analyzer) semaIRSOQLConditionalAssignable(targetType string, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) bool {
	if expr.Kind != ir.ExprCall || expr.Callee != "__ternary" || len(expr.Args) != 3 {
		return false
	}
	hasQuery := false
	for _, branch := range expr.Args[1:] {
		if branch.Kind == ir.ExprSOQL {
			hasQuery = true
			if !semaSOQLSingletonAssignable(targetType, semaSOQLLiteralType(branch.Value), "["+branch.Value+"]", model) {
				return false
			}
			continue
		}
		branchType := resolveNestedTypeReference(model, currentType, a.inferIRExprType(branch, scope, model, currentType))
		if !strings.EqualFold(branchType, "null") && !semaAssignableToType(targetType, branchType, model) {
			return false
		}
	}
	return hasQuery
}

// semaIRSOQLNullCoalesceAssignable preserves direct-query singleton context only
// when the fallback is the null literal. Stored lists keep ordinary list typing.
func semaIRSOQLNullCoalesceAssignable(targetType, valueType string, expr ir.Expr, model *semaTypeMemberView) bool {
	if expr.Kind != ir.ExprCall || !strings.EqualFold(expr.Callee, "__coalesce") || len(expr.Args) != 2 {
		return false
	}
	query, fallback := expr.Args[0], expr.Args[1]
	if query.Kind != ir.ExprSOQL || fallback.Kind != ir.ExprLiteral || !strings.EqualFold(strings.TrimSpace(fallback.Value), "null") {
		return false
	}
	return semaSOQLSingletonAssignable(targetType, valueType, "["+query.Value+"]", model)
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
	if _, missing := semaMissingSObjectFieldsObject(nil, expr, scope, model); missing {
		return false
	}
	root, _, ok := strings.Cut(strings.TrimSpace(expr), ".")
	if !ok || root == "" {
		return false
	}
	if _, scoped := scope.lookup(root); scoped {
		// A local, loop variable, parameter or field is the receiver even when
		// it is spelled like its SObject type: native M1 S01-S08 and
		// H2S1X/H2S5/H2S6 at API 62.0 and 67.0, both routes. The text path
		// (semaExprLooksLikeStaticSObjectTokenInModel) has the same rule.
		return false
	}
	return semaLooksLikeSObjectFieldTokenInModel(expr, model) || semaLooksLikeSObjectTypeTokenInModel(expr, model)
}

// semaIRSObjectSpelledLocal reports a root that is a local written exactly as
// an SObject type name (Contact, Account) and returns the local's concrete
// SObject type. The local is the receiver, whatever its own type: M1 S01-S04
// and H2R1/H2R1L (an Account local spelled Contact) at 62.0 and 67.0.
// A lowercase root, a parameter or a field has no row for the rules below.
func semaIRSObjectSpelledLocal(root string, scope irSemaScope, model *semaTypeMemberView) (string, bool) {
	root = strings.TrimSpace(root)
	binding, scoped := scope.binding(root)
	if !scoped || (binding.origin != irSemaOriginLocal && binding.origin != irSemaOriginFinalLocal) {
		return "", false
	}
	if spelled, ok := model.lookup(normalizeName(root)); !ok || !spelled.sobject || spelled.name != root {
		return "", false
	}
	local, ok := model.lookup(normalizeName(binding.typ))
	if !ok || !local.sobject || strings.EqualFold(local.name, "SObject") || strings.EqualFold(local.name, "AggregateResult") {
		return "", false
	}
	return binding.typ, true
}

// semaIRSameSpelledLocalSObjectTypeRead reports X.SObjectType where X is such
// a local. A record has no SObjectType member, so native reports "Variable
// does not exist: SObjectType": M1 S09/S109, H2S2X and H2R1T (an Account
// local spelled Contact) at API 62.0 and 67.0, abstract-override R014/C014 at
// 62.0, 65.0, 66.0 and 67.0, both routes. A parameter, a field, another
// spelling or another local name has no row and keeps the type token.
func semaIRSameSpelledLocalSObjectTypeRead(name string, scope irSemaScope, model *semaTypeMemberView) bool {
	root, field, ok := strings.Cut(strings.TrimSpace(name), ".")
	if !ok || !strings.EqualFold(field, "SObjectType") {
		return false
	}
	localType, ok := semaIRSObjectSpelledLocal(root, scope, model)
	return ok && !semaSObjectHasSObjectTypeField(model, localType)
}

// semaIRSObjectSpelledLocalMissingField reports a standard field read through
// such a local of another standard object type that its own object lacks:
// native H2R1B (an Account local spelled Contact reading Contact.LastName)
// gives "Variable does not exist: LastName" at 62.0 and 67.0, both routes.
// The field must be in the object's base standard definition; a field only an
// org feature adds (Account.LastName with Person Accounts) counts as missing,
// as in the native org and the runtime's default org shape. Unqualified custom
// fields use the project's declared schema; namespaced fields remain open.
func semaIRSObjectSpelledLocalMissingField(name, namespace string, scope irSemaScope, model *semaTypeMemberView, declared []schema.Object) (string, bool) {
	root, path, ok := strings.Cut(strings.TrimSpace(name), ".")
	if !ok {
		return "", false
	}
	field, _, _ := strings.Cut(path, ".")
	field = strings.TrimSpace(field)
	customField := semaHasAPISuffixFold(field, "__c") && strings.Count(field, "__") == 1
	if field == "" || (strings.Contains(field, "__") && !customField) || strings.EqualFold(field, "SObjectType") || strings.EqualFold(field, "class") {
		return "", false
	}
	localType, ok := semaIRSObjectSpelledLocal(root, scope, model)
	if !ok || (!customField && strings.EqualFold(localType, strings.TrimSpace(root))) {
		return "", false
	}
	// A project that references a standard object without its metadata
	// marks it partial; the base standard definition still decides here.
	members, ok := model.lookup(normalizeName(localType))
	if !ok || members.externalPackageSObject {
		return "", false
	}
	definition, standard := storage.StandardObjectDefinition(localType)
	if !standard {
		return "", false
	}
	for apiName := range definition.Fields {
		if strings.EqualFold(apiName, field) {
			return "", false
		}
	}
	for _, relation := range definition.Relations {
		if strings.EqualFold(relation.ParentRelationship, field) {
			return "", false
		}
	}
	if _, ok := semaStandardChildRelationshipMemberForKey(members.name, normalizeName(field)); ok {
		return "", false
	}
	// A field the project's own object metadata declares is not a gap.
	qualifiedField, _ := semaProjectNamespacedAPIName(namespace, field)
	for _, object := range declared {
		if !strings.EqualFold(object.Name, members.name) {
			continue
		}
		for _, declaredField := range object.Fields {
			if strings.EqualFold(declaredField.Name, field) || (qualifiedField != "" && strings.EqualFold(declaredField.Name, qualifiedField)) {
				return "", false
			}
		}
	}
	return field, true
}

// semaSObjectHasSObjectTypeField reports an object with a real SobjectType
// field (RecordType, for one). Reading it through a record is a field read,
// so the rule above does not apply; no row covers that shape.
func semaSObjectHasSObjectTypeField(model *semaTypeMemberView, typeName string) bool {
	target, ok := semaResolveFieldPath(model, typeName, "SObjectType")
	return ok && !strings.EqualFold(target.member.Type, "Schema.SObjectType")
}

// semaSetterValueShadowsType reports a dotted name rooted at the implicit
// value parameter of a property setter while a project type of that name is
// visible. The parameter is the receiver, in any case: native GATE3-01/01C
// (named) and H2V-A1/H2V-A3 (anonymous) at API 62.0 and 67.0. Another name,
// such as Valuex, stays the type (H2V-A2).
func semaSetterValueShadowsType(typ typesys.TypeSymbol, member typesys.MemberSymbol, root string, scope irSemaScope, model *semaTypeMemberView) bool {
	if !strings.EqualFold(strings.TrimSpace(root), "value") || len(member.Name) < len(".set") || !strings.EqualFold(member.Name[len(member.Name)-len(".set"):], ".set") || len(member.Parameters) != 1 || !strings.EqualFold(member.Parameters[0].Name, "value") {
		return false
	}
	if binding, ok := scope.binding(root); !ok || binding.origin != irSemaOriginParam {
		return false
	}
	resolved := resolveNestedTypeName(model, typ.Name, strings.TrimSpace(root))
	if resolved == "" {
		return false
	}
	members, ok := model.lookup(normalizeName(resolved))
	return ok && !members.platform && !members.dependency
}

// semaIRCustomLabelReference reports a custom label read written Label.X or
// System.Label.X: a String (M1 rows and H2L1-H2L4 at API 62.0 and 67.0,
// both routes). A project type named Label keeps its own members (M1 row,
// L112), and other spellings (namespaced labels) have no row.
func semaIRCustomLabelReference(name string, scope irSemaScope, model *semaTypeMemberView, currentType string) bool {
	parts := strings.Split(strings.TrimSpace(name), ".")
	switch {
	case len(parts) == 2 && strings.EqualFold(parts[0], "Label"):
	case len(parts) == 3 && strings.EqualFold(parts[0], "System") && strings.EqualFold(parts[1], "Label"):
	default:
		return false
	}
	if _, scoped := scope.lookup(parts[0]); scoped || strings.TrimSpace(parts[len(parts)-1]) == "" || semaProjectTypeShadowsPlatform(model, "Label") {
		return false
	}
	if resolved := resolveNestedTypeName(model, currentType, "Label"); resolved != "" {
		if members, ok := model.lookup(normalizeName(resolved)); ok && !members.platform && !members.dependency {
			return false
		}
	}
	return true
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
	if len(candidates) == 0 && receiverMode == "implicit" {
		candidates = preferResolvedMethodsByReceiverMode(resolveImplicitMemberMethods(model, receiverType, method), receiverMode)
	}
	platformBackedCandidates := semaResolvedMembersAllPlatformBacked(model, candidates)
	if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok && !platformBackedCandidates {
		return semaResolvedMemberReturnType(model, candidate)
	}
	if returnType := enclosingListHelperReturnType(model, receiverType, method, currentType, receiverMode, candidates, argTypes); returnType != "" {
		return returnType
	}
	if semaProjectTypeShadowsPlatform(model, receiverType) {
		return ""
	}
	if sig, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return sig.returnTypeForArity(len(args))
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
	if item, rejected := semaCustomLabelDiagnostic(typ, name, scope, model, start, source); rejected {
		return item, true
	}
	if message := a.customSettingTokenMessage(name, scope, model); message != "" {
		return semaFieldAccessDiagnostic(typ, member, name, message, start, start+max(1, len(name)), source), true
	}
	if item, rejected := a.pageReferenceDiagnostic(typ, name, scope, model, start, source); rejected {
		return item, true
	}
	if missing, ok := semaMissingSObjectFieldsObject(a, name, scope, model); ok {
		message := "Variable does not exist: " + missing
		return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, start+max(1, len(name)))}, true
	}
	if field, ok := semaTriggerContextField(name, scope, model); ok && !semaKnownTriggerContextField(field) {
		return semaTriggerDiagnostic(typ, "Variable does not exist: "+field, start, start+max(1, len(name)), source), true
	}
	if semaUnavailableTriggerOperation(name, scope, model) {
		return semaTriggerDiagnostic(typ, "Variable does not exist: BEFORE_UNDELETE", start, start+max(1, len(name)), source), true
	}
	if strings.HasSuffix(strings.ToLower(name), ".class") {
		typeName := name[:len(name)-len(".class")]
		prefix, invalid := semaClassLiteralPrefix(model, typ.Name, typeName)
		if _, args := semaGenericBaseAndArgs(typeName); len(args) == 0 && !strings.Contains(typeName, ".") && !a.hasKnown(resolveNestedTypeName(model, typ.Name, typeName)) {
			invalid = true
		}
		if invalid {
			message := "Invalid type: " + typeName
			return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA006", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, start+len(name))}, true
		}
		if prefix != "" {
			return diagnostic.Diagnostic{}, false
		}
	}
	root, field, hasMember := strings.Cut(name, ".")
	if !hasMember {
		return diagnostic.Diagnostic{}, false
	}
	recordType := root
	if scoped, ok := scope.lookup(root); ok {
		recordType = scoped
	}
	firstField, _, _ := strings.Cut(field, ".")
	schemaFieldsPath := !scope.hasNonFieldBinding(root) && strings.EqualFold(root, "AuthSession") && strings.EqualFold(firstField, "Fields")
	if !schemaFieldsPath && (strings.EqualFold(recordType, "AuthSession") || strings.EqualFold(recordType, "Schema.AuthSession")) {
		if members, _, known := semaLookupTypeMembers(model, recordType); known && members.sobject {
			if _, found := semaResolveFieldPath(model, recordType, field); !found {
				missing, _, _ := strings.Cut(field, ".")
				message := "Variable does not exist: " + missing
				item := semaFieldAccessDiagnostic(typ, member, name, message, start, start+max(1, len(name)), source)
				item.NativeMessage = message
				return item, true
			}
		}
	}
	if semaIRExprLooksLikeStaticSObjectToken(name, scope, model) {
		return diagnostic.Diagnostic{}, false
	}
	if semaIRSameSpelledLocalSObjectTypeRead(name, scope, model) {
		message := "Variable does not exist: " + field
		at, end, found := semaIRNameSpan(source, start, name)
		if !found {
			end = at + max(1, len(name))
		}
		return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA021", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, at, end)}, true
	}
	if missing, ok := semaIRSObjectSpelledLocalMissingField(name, a.namespace, scope, model, a.queryDeclaredObjects); ok && (reading || semaHasAPISuffixFold(missing, "__c")) {
		message := "Variable does not exist: " + missing
		return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA021", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, start+max(1, len(name)))}, true
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
		if !a.irVariableKnown(name, scope, model, typ.Name) && !strings.HasSuffix(normalizeName(name), ".class") {
			message := "Variable does not exist: " + root
			return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, start+max(1, len(root)))}, true
		}
		return diagnostic.Diagnostic{}, false
	}
	if valueReceiver && semaSetterValueShadowsType(typ, member, root, scope, model) {
		if _, ok := semaResolveFieldPath(model, receiverType, field); !ok {
			missing, _, _ := strings.Cut(field, ".")
			message := "Variable does not exist: " + strings.TrimSpace(missing)
			return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA021", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, start+max(1, len(name)))}, true
		}
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
		// A missing suffix names the nearest enclosing type, in any spelling
		// case (class_literal_prefix/org.tsv). Static values stay invalid.
		if prefix, invalid := semaClassLiteralPrefix(model, currentType, typeName); prefix != "" && !invalid {
			return true
		}
		for _, ref := range extractTypeNames(resolveNestedTypeReference(model, currentType, typeName)) {
			if !semaTypeReferenceKnownAtVersion(a, model, currentType, ref, "") {
				return false
			}
		}
		return true
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
			if !semaTypeReferenceKnownAtVersion(a, model, typ.Name, ref, typ.EffectiveAPIVersion) {
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
			if !semaTypeReferenceKnownAtVersion(a, model, typ.Name, ref, typ.EffectiveAPIVersion) {
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
		diagnostics = append(diagnostics, scopes.declareScopedLocal(typ, member, local.name, local.typeName, local.start, local.scopeStart, local.scopeEnd, bodyOffset, source, nameStart, nameEnd, true)...)
	}
	for _, match := range catchLocalPattern.FindAllStringSubmatchIndex(body, -1) {
		if ignored.contains(match[0]) {
			continue
		}
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		name := strings.TrimSpace(body[match[4]:match[5]])
		scopeStart, scopeEnd := blockBoundsAfter(body, match[1])
		for _, ref := range extractTypeNames(strings.ReplaceAll(typeName, "|", ",")) {
			if !semaTypeReferenceKnownAtVersion(a, model, typ.Name, ref, typ.EffectiveAPIVersion) {
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
	collectSemaSwitchTypeBindings(typ, member, body, &scopes, model)
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
	return s.declareScopedLocal(typ, member, name, typeName, start, scopeStart, scopeEnd, bodyOffset, source, nameStart, nameEnd, false)
}

func (s *semaScopeModel) declareScopedLocal(typ typesys.TypeSymbol, member typesys.MemberSymbol, name, typeName string, start, scopeStart, scopeEnd, bodyOffset int, source string, nameStart, nameEnd int, classicFor bool) []diagnostic.Diagnostic {
	key := s.canonicalName(name)
	if existing, exists := s.forLoopScopeConflictingLocal(typ.EffectiveAPIVersion, key, start, scopeStart, scopeEnd, classicFor); exists {
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
