package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func (a *Analyzer) checkBodyText(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	member = semaNormalizeMemberTypes(model, typ.Name, member)
	baseScope := semaBodyBaseScope(typ, member, model)
	bodyScan := newSemaBodyExpressionScan(body)
	scopes, diagnostics := a.collectBodyScopes(typ, member, body, bodyOffset, source, baseScope, model)
	diagnostics = append(diagnostics, staticThisDiagnostics(typ, member, body, bodyOffset, source)...)
	irDiagnostics, irOK := a.checkBodyIRWithCompileStatus(typ, member, body, bodyOffset, source, baseScope, model, constructability)
	diagnostics = append(diagnostics, irDiagnostics...)
	for _, ctor := range constructorTypes(body) {
		for _, ref := range extractTypeNames(ctor.text) {
			if !a.hasKnown(ref) {
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
	return dedupeBodyDiagnostics(diagnostics)
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
	return baseScope
}

func staticThisDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string) []diagnostic.Diagnostic {
	if !hasModifier(member.Modifiers, "static") {
		return nil
	}
	lower := strings.ToLower(body)
	var diagnostics []diagnostic.Diagnostic
	for cursor := 0; cursor < len(lower); {
		offset := strings.Index(lower[cursor:], "this")
		if offset < 0 {
			break
		}
		offset += cursor
		end := offset + len("this")
		leftBoundary := offset == 0 || !isApexIdentifierChar(lower[offset-1])
		rightBoundary := end == len(lower) || !isApexIdentifierChar(lower[end])
		if leftBoundary && rightBoundary && !semaOffsetInIgnoredText(body, offset) {
			diagnostics = append(diagnostics, semaFieldAccessDiagnostic(typ, member, "this", "this cannot be referenced from a static method", bodyOffset+offset, bodyOffset+end, source))
		}
		cursor = end
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
			case "GLADESEMA006", "GLADESEMA008", "GLADESEMA009", "GLADESEMA010", "GLADESEMA011", "GLADESEMA014", "GLADESEMA015", "GLADESEMA018", "GLADESEMA019", "GLADESEMA020", "GLADESEMA022", "GLADESEMA023", "GLADESEMA024", "GLADESEMA025", "GLADESEMA026", "GLADESEMA027", "GLADESEMA028":
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
	if !isSemaSObjectLike(typeName, model) {
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
)

type irSemaBinding struct {
	typ    string
	origin irSemaOrigin
}

type irSemaScope struct {
	frames []map[string]irSemaBinding
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
	return irSemaScope{frames: []map[string]irSemaBinding{root}}
}

func (s *irSemaScope) push() {
	s.frames = append(s.frames, make(map[string]irSemaBinding))
}

func (s *irSemaScope) pop() {
	if len(s.frames) > 1 {
		s.frames = s.frames[:len(s.frames)-1]
	}
}

func (s *irSemaScope) declare(name, typ string) bool {
	if len(s.frames) == 0 {
		s.frames = append(s.frames, make(map[string]irSemaBinding))
	}
	key := normalizeName(name)
	if key == "" {
		return true
	}
	for _, frame := range s.frames {
		if binding, ok := frame[key]; ok && binding.origin != irSemaOriginField {
			return false
		}
	}
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
		body, _, ok := extractBodyForSema(source, candidate.Range)
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
	for index := 0; index < len(declaration); index++ {
		if semaOffsetInIgnoredText(declaration, index) {
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

func (s irSemaScope) flat() map[string]string {
	out := make(map[string]string)
	for _, frame := range s.frames {
		for name, binding := range frame {
			out[name] = binding.typ
		}
	}
	return out
}

func (a *Analyzer) checkBodyIR(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	diagnostics, _ := a.checkBodyIRWithCompileStatus(typ, member, body, bodyOffset, source, base, model, constructability)
	return diagnostics
}

func (a *Analyzer) checkBodyIRWithCompileStatus(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) ([]diagnostic.Diagnostic, bool) {
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return nil, false
	}
	scope := newIRSemaScopeWithOrigins(base, semaMemberParameterNames(member))
	var diagnostics []diagnostic.Diagnostic
	if a.includePerformanceDiagnostics {
		diagnostics = append(diagnostics, performanceDiagnosticsForProgram(typ, program, bodyOffset, source)...)
	}
	diagnostics = append(diagnostics, a.checkIRExpressionTypeReferences(typ, member, program.Instructions, body, bodyOffset, source)...)
	diagnostics = append(diagnostics, a.checkIRStatementContracts(typ, member, program.Instructions, scope, bodyOffset, source, model)...)
	diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, program.Instructions, &scope, bodyOffset, source, model, constructability)...)
	diagnostics = append(diagnostics, a.checkConstructorChainingIR(typ, member, program.Instructions, bodyOffset, source, model)...)
	returnType := strings.TrimSpace(member.Type)
	memberSource := ""
	if member.Range.Start.Offset >= 0 && member.Range.End.Offset > member.Range.Start.Offset && member.Range.End.Offset <= len(source) {
		memberSource = source[member.Range.Start.Offset:member.Range.End.Offset]
	}
	if returnType != "" && !strings.EqualFold(returnType, "void") && !irInstructionsTerminate(program.Instructions) && !semaBodyEndsWithThrow(body) && !semaBodyEndsWithThrow(memberSource) {
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s on all paths", returnType), member.Range.Start.Offset, member.Range.End.Offset, source))
	}
	return diagnostics, true
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
		if inst.Expr.Kind != "" {
			diagnostics = append(diagnostics, a.checkIRExpressionContract(typ, member, inst.Expr, *scope, inst.Pos, bodyOffset, source, model)...)
		}
		switch inst.Op {
		case ir.OpDeclare:
			if semaAPI67RejectedPlatformType(inst.Type) && !semaProjectTypeShadowsPlatform(model, inst.Type) {
				diagnostics = append(diagnostics, unsupportedLocalFeatureDiagnostic(typ, member, inst.Type, bodyOffset+inst.Pos, bodyOffset+inst.Pos+max(1, len(inst.Type)), source))
			}
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRAssignmentType(typ, member, inst.Type, inst.Name, inst.Expr, scope, inst.Pos, bodyOffset, source, model, "initializes")...)
			if !scope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type)) {
				diagnostics = append(diagnostics, irRedeclareDiagnostic(typ, member, inst.Name, inst.Pos, bodyOffset, source))
			}
		case ir.OpBlock:
			scope.push()
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			scope.pop()
		case ir.OpDeclGroup:
			// Comma-separated locals share the enclosing scope.
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
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
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			if inst.Op == ir.OpDML {
				diagnostics = append(diagnostics, a.checkIRDMLContract(typ, member, inst, *scope, bodyOffset, source, model)...)
			}
			if inst.Op == ir.OpRunAs {
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
			if len(inits) > 0 {
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inits, scope, bodyOffset, source, model, constructability)...)
			}
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRConditionType(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model)...)
			diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, inst.Then, scope, bodyOffset, source, model, constructability)...)
			updates := inst.Updates
			if len(updates) == 0 && inst.Update != nil {
				updates = []ir.Instruction{*inst.Update}
			}
			if len(updates) > 0 {
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, updates, scope, bodyOffset, source, model, constructability)...)
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
			return []diagnostic.Diagnostic{irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, fmt.Sprintf("%s requires an SObject or SObject collection operand, got %s", inst.Name, operandTypes[i]))}
		}
	}
	if strings.EqualFold(inst.Name, "merge") && len(operandTypes) == 2 && !semaDMLMergeTypesCompatible(operandTypes[0], operandTypes[1], model) {
		return []diagnostic.Diagnostic{irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, "merge requires master and duplicate operands of the same SObject type")}
	}
	if strings.EqualFold(inst.Name, "upsert") && inst.Field != "" && !a.semaUpsertSelectorAllowed(operandTypes[0], inst.Field) {
		return []diagnostic.Diagnostic{irDMLContractDiagnostic(typ, member, inst, bodyOffset, source, fmt.Sprintf("upsert field %q must be an external ID or idLookup field for %s", inst.Field, semaDMLObjectType(operandTypes[0])))}
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
	for _, object := range a.queryDeclaredObjects {
		if !semaProjectReferencedSchemaAPINamesMatch(a.namespace, object.Name, objectName) {
			continue
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
		field, ok := provider.lookup(fieldName)
		if ok {
			fieldSchemaFound = true
			if field.ExternalID || field.IDLookup {
				return true
			}
		}
	}
	if fieldSchemaFound {
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
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA034", Message: fmt.Sprintf("%s %q %s", member.Kind, member.Name, message), File: typ.File, Range: semaRange(source, start, start+max(1, len(inst.Name)))}
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
	var diagnostics []diagnostic.Diagnostic
	switch expr.Kind {
	case ir.ExprVariable:
		if semaExprAtSwitchWhenLabel(source, bodyOffset+pos, expr.Name) {
			return nil
		}
		fieldReceiver := expr.Name
		fieldPath := expr.Name
		if dot := strings.LastIndexByte(fieldReceiver, '.'); dot > 0 {
			fieldReceiver = fieldReceiver[:dot]
			if receiverType, ok := scope.lookup(fieldReceiver); ok {
				fieldPath = receiverType + expr.Name[dot:]
			}
		}
		if semaAPI67RejectedPlatformField(fieldPath) && !semaProjectTypeShadowsPlatform(model, fieldReceiver) {
			return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, expr.Name, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Name)), source)}
		}
		if diag, ok := a.irVariableDiagnostic(typ, member, expr.Name, *scope, model, bodyOffset+pos, source); ok {
			diagnostics = append(diagnostics, diag)
		} else if !a.irVariableKnown(expr.Name, *scope, model, typ.Name) && !isLikelyTypeReference(expr.Name) {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA013",
				Message:  fmt.Sprintf("%s %q reads unknown variable %q", member.Kind, member.Name, expr.Name),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Name))),
			})
		}
	case ir.ExprCall:
		if lastDot := strings.LastIndex(expr.Callee, "."); lastDot > 0 && lastDot < len(expr.Callee)-1 {
			typeName := expr.Callee[:lastDot]
			memberName := expr.Callee[lastDot+1:]
			if strings.EqualFold(typeName, "Search") && !strings.EqualFold(memberName, "query") && !strings.EqualFold(memberName, "find") && !strings.EqualFold(memberName, "suggest") {
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

func (a *Analyzer) checkIRAssignmentTarget(typ typesys.TypeSymbol, member typesys.MemberSymbol, name string, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	if strings.Contains(name, "?.") {
		return []diagnostic.Diagnostic{typeContractDiagnostic(typ, member, "safe navigation cannot be an assignment target", bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
	}
	if !strings.Contains(name, ".") && scope.hasNonFieldBinding(name) {
		return nil
	}
	if root, field, ok := strings.Cut(name, "."); ok {
		if receiverType := semaIRReceiverType(root, scope, model, typ.Name); receiverType != "" && !semaProjectTypeShadowsPlatform(model, receiverType) {
			if target, resolved := semaResolveFieldPath(model, receiverType, field); resolved && semaAPI67ReadOnlyPlatformField(target.owner+"."+target.member.Name) {
				return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, name, bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
			}
		}
	}
	if target, ok := semaResolveFieldPath(model, typ.Name, name); ok && target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyHasAccessor(target.member, "set") {
		return []diagnostic.Diagnostic{typeContractDiagnostic(typ, member, "property has no setter", bodyOffset+pos, bodyOffset+pos+max(1, len(name)), source)}
	}
	if diag, ok := a.irVariableDiagnostic(typ, member, name, scope, model, bodyOffset+pos, source); ok {
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
	if strings.HasPrefix(expr.Callee, "new:") {
		return a.checkIRConstructorCall(typ, member, expr, scope, pos, bodyOffset, source, model, constructability)
	}
	if strings.HasPrefix(expr.Callee, "__assign:") {
		return a.checkIRAssignmentTarget(typ, member, strings.TrimPrefix(expr.Callee, "__assign:"), scope, pos, bodyOffset, source, model)
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
	if expr.Callee == "" || expr.Callee == "this" || expr.Callee == "super" || skipSemaCall(expr.Callee) {
		return nil
	}
	// Keep this pre-resolution guard for rejected qualified platform receivers.
	// The later platform path is shared, but unknown dotted receivers can exit
	// through permissive fallback before it is reached.
	if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok && !scope.hasNonFieldBinding(receiver) && !semaProjectTypeShadowsPlatform(model, receiver) && semaAPI67RejectedPlatformCall(receiver, method, "class") {
		return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, receiver+"."+method, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
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
							return []diagnostic.Diagnostic{unknownCallDiagnostic(typ, member, expr.Callee, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
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
	if receiverType == "" {
		return nil
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
	// Instance receivers can bypass checkIRPlatformCall through its permissive
	// fallback after IR infers their platform type, so guard that path here.
	if !semaProjectTypeShadowsPlatform(model, receiverType) && semaAPI67RejectedPlatformCall(receiverType, method, receiverMode) {
		return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, receiverType+"."+method, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	if !semaProjectTypeShadowsPlatform(model, receiverType) && semaAPI67RejectedPlatformCallArgs(receiverType, method, irCallArgTypes(a, expr.Args, scope, model, typ.Name)) {
		return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}
	}
	candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, receiverType, method), receiverMode)
	if !explicitReceiver {
		candidates = resolveImplicitMemberMethods(model, receiverType, method)
	}
	if len(candidates) == 0 {
		if semaCallMayBelongToMissingSuperclass(model, typ, expr.Callee, receiverMode, receiverType) {
			return nil
		}
		if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model, receiverMode); handled {
			return diagnostics
		}
		if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
			return diagnostics
		}
	}
	if len(candidates) == 0 && !strings.Contains(expr.Callee, ".") && bodyOffset >= 0 && bodyOffset <= len(source) {
		if chainedReceiver, chainedMethod, ok := semaChainedCallReceiverNear(source[bodyOffset:], pos, method, scope.flat(), model, typ.Name); ok && strings.EqualFold(chainedMethod, method) {
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
			receiverTyp := inferSemaFieldAccessType(receiverExpr, scope.flat(), model)
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
		if strings.Count(expr.Callee, ".") != 1 && semaSourceHasDottedCall(source, method) {
			return nil
		}
		if explicitReceiver && a.hasKnown(receiverType) {
			return nil
		}
		if semaCallMayBelongToMissingSuperclass(model, typ, expr.Callee, receiverMode, receiverType) {
			return nil
		}
		return []diagnostic.Diagnostic{unknownCallDiagnostic(typ, member, expr.Callee, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
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
		return []diagnostic.Diagnostic{ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
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
	if strings.Count(expr.Callee, ".") != 1 && semaSourceHasDottedCall(source, method) {
		return nil
	}
	if semaKnownFluentHelperMethod(method) {
		return nil
	}
	if textArgTypes := irVariableTextArgTypes(expr.Args, scope, model); len(textArgTypes) == len(expr.Args) {
		if _, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, textArgTypes, model); ok {
			return nil
		} else if ambiguous && !semaArgTypesContainUnknown(textArgTypes) {
			return []diagnostic.Diagnostic{ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
		}
	}
	if sourceArgTypes := irSourceArgTypesForCall(source, bodyOffset+pos, expr.Callee, scope, model); len(sourceArgTypes) == len(expr.Args) {
		if _, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, sourceArgTypes, model); ok {
			return nil
		} else if ambiguous && !semaArgTypesContainUnknown(sourceArgTypes) {
			return []diagnostic.Diagnostic{ambiguousCallDiagnostic(typ, member, expr.Callee, len(expr.Args), bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
		}
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA009",
		Message:  fmt.Sprintf("%s %q has no matching overload for call %q with %d argument(s)", member.Kind, member.Name, expr.Callee, len(expr.Args)),
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee))),
	}}
}

func irVariableTextArgTypes(args []ir.Expr, scope irSemaScope, model *semaTypeMemberView) []string {
	argTypes := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.Kind != ir.ExprVariable || strings.TrimSpace(arg.Name) == "" {
			return nil
		}
		argTypes = append(argTypes, inferSemaArgTypeWithModel(arg.Name, scope.flat(), model))
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
	flat := scope.flat()
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, flat, model)
	}
	return argTypes
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
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}, true
}

func (a *Analyzer) checkIRPlatformCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, receiverMode string) ([]diagnostic.Diagnostic, bool) {
	if semaProjectTypeShadowsPlatform(model, receiverType) {
		return nil, false
	}
	if semaDatabaseDynamicQueryCall(receiverType, method) {
		return nil, true
	}
	if _, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return nil, false
	}
	if candidates := resolveMemberMethods(model, receiverType, method); len(candidates) != 0 && !semaResolvedMembersAllPlatformBacked(model, candidates) {
		return nil, false
	}
	sig, ok := semaPlatformMethodSignatureForMode(model, receiverType, method, receiverMode)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = a.inferIRExprType(arg, scope, model, typ.Name)
	}
	if semaAPI67RejectedPlatformCallArgs(receiverType, method, argTypes) {
		return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}, true
	}
	if semaDatabaseDMLReturnType(receiverType, method, argTypes) != "" && len(args) <= 4 {
		return nil, true
	}
	if semaSearchSuggestObjectOverload(receiverType, method, argTypes) {
		return nil, true
	}
	if semaArgsMatchAny(sig.params, argTypes, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}, true
}

func (a *Analyzer) checkIRConstructorCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	typeName := strings.TrimPrefix(expr.Callee, "new:")
	resolvedTypeName := resolveNestedTypeReference(model, typ.Name, typeName)
	if semaAPI67RejectedPlatformConstructor(typeName) && !semaProjectTypeShadowsPlatform(model, typeName) {
		return []diagnostic.Diagnostic{unsupportedLocalFeatureDiagnostic(typ, member, "new "+typeName, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	for _, ref := range extractTypeNames(typeName) {
		if !a.hasKnown(ref) {
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
		return nil
	}
	argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
	namedArgTypes := irCallNamedArgTypes(a, expr.NamedArgs, scope, model, typ.Name)
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
			return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("no matching %s constructor with %d argument(s)", typeName, len(expr.Args)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
		}
		if len(expr.Args) == 0 || a.allowsInheritedExceptionConstructor(resolvedTypeName, expr.Args, scope, model, typ.Name) {
			return nil
		}
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("no matching %s constructor with %d argument(s)", typeName, len(expr.Args)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
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
			return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("ambiguous %s constructor with %d argument(s)", typeName, len(expr.Args)+len(expr.NamedArgs)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
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
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("ambiguous %s constructor with %d argument(s)", typeName, len(expr.Args)+len(expr.NamedArgs)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if !target.constructorsAuthoritative && a.allowsInheritedExceptionConstructor(resolvedTypeName, expr.Args, scope, model, typ.Name) {
		return nil
	}
	return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("no matching %s constructor with %d argument(s)", typeName, len(expr.Args)+len(expr.NamedArgs)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
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
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA018",
		Message:  fmt.Sprintf("%s %q %s %s with %s", member.Kind, member.Name, verb, target, valueType),
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(target))),
	}}
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
			return semaCommonType(leftType, rightType, model)
		}
		if (strings.HasPrefix(expr.Callee, "__field:") || strings.HasPrefix(expr.Callee, "__safe_field:")) && expr.Left != nil {
			receiverType := a.inferIRExprType(*expr.Left, scope, model, currentType)
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
				return target.member.Type
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
		if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok {
			receiverType := semaTextReceiverType(receiver, scope.flat(), model)
			return semaResolvedIRCallReturnType(a, model, receiverType, method, expr.Args, scope, currentType, semaTextCallReceiverMode(receiver, scope.flat(), model))
		}
		return semaResolvedIRCallReturnType(a, model, currentType, expr.Callee, expr.Args, scope, currentType, "implicit")
	case ir.ExprUnary:
		switch expr.Operator {
		case "!":
			return "Boolean"
		case "-":
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

func semaLooksLikeSOQLCountLiteral(queryText string) bool {
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
	if expr.Callee == "" || expr.Left != nil {
		return ""
	}
	scopeMap := scope.flat()
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
	if semaDatabaseDynamicQueryCall(receiverType, method) {
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
	case "<<", ">>", ">>>", "|", "&", "^":
		return semaIntegralResultType(leftType, rightType)
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
	for _, typ := range []string{"Double", "Decimal", "Long", "Integer"} {
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

func (a *Analyzer) irVariableDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, name string, scope irSemaScope, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
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
	if members, ok := model.lookup(normalizeName(receiverType)); ok && members.kind == apexast.DeclarationEnum {
		return diagnostic.Diagnostic{}, false
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
	base      map[string]string
	locals    []semaLocal
	canonical *semaCanonicalNames
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
	scopes := semaScopeModel{base: base, canonical: a.canonicalNames}
	var diagnostics []diagnostic.Diagnostic
	diagnostics = append(diagnostics, declareSemaParameters(typ, member, body, bodyOffset, source, &scopes)...)
	for _, match := range enhancedForLocalPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaOffsetInIgnoredText(body, match[0]) {
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
			if !a.hasKnown(ref) {
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
		if semaLocalDeclMatchInIgnoredText(body, match) {
			continue
		}
		diagnostics = append(diagnostics, a.collectSemaLocalDecl(typ, member, body, bodyOffset, source, &scopes, model, match)...)
	}
	for _, match := range wrappedLocalDeclPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaLocalDeclMatchInIgnoredText(body, match) {
			continue
		}
		diagnostics = append(diagnostics, a.collectSemaLocalDecl(typ, member, body, bodyOffset, source, &scopes, model, match)...)
	}
	for _, match := range noSpaceGenericLocalDeclPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaLocalDeclMatchInIgnoredText(body, match) {
			continue
		}
		diagnostics = append(diagnostics, a.collectSemaLocalDecl(typ, member, body, bodyOffset, source, &scopes, model, match)...)
	}
	for _, match := range findSemaLocalDeclMatches(body) {
		if semaLocalDeclMatchInIgnoredText(body, match) {
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
			if !a.hasKnown(ref) {
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
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		name := strings.TrimSpace(body[match[4]:match[5]])
		scopeStart, scopeEnd := blockBoundsAfter(body, match[1])
		for _, ref := range extractTypeNames(strings.ReplaceAll(typeName, "|", ",")) {
			if !a.hasKnown(ref) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA006",
					Message:  fmt.Sprintf("%s %q declares catch local %q with unknown type %q", member.Kind, member.Name, name, ref),
					File:     typ.File,
					Range:    semaRange(source, bodyOffset+match[2], bodyOffset+match[3]),
				})
			}
		}
		diagnostics = append(diagnostics, scopes.declareLocal(typ, member, name, resolveNestedTypeReference(model, typ.Name, firstCatchType(typeName)), scopeStart, scopeStart, scopeEnd, bodyOffset, source, match[4], match[5])...)
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
