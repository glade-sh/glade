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

func (a *Analyzer) checkBodyText(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	member = semaNormalizeMemberTypes(model, typ.Name, member)
	baseScope := make(map[string]string)
	baseScope[semaCurrentTypeScopeKey] = typ.Name
	for name, fieldType := range semaFieldScope(model, typ.Name, make(map[string]bool)) {
		baseScope[name] = fieldType
	}
	for _, param := range member.Parameters {
		baseScope[normalizeName(param.Name)] = param.Type
	}
	scopes, diagnostics := a.collectBodyScopes(typ, member, body, bodyOffset, source, baseScope, model)
	diagnostics = append(diagnostics, a.checkBodyIR(typ, member, body, bodyOffset, source, baseScope, model, constructability)...)
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
	diagnostics = append(diagnostics, a.checkBodyAssignments(typ, member, body, bodyOffset, source, scopes, model)...)
	diagnostics = append(diagnostics, a.checkBodyReturns(typ, member, body, bodyOffset, source, scopes, model)...)
	diagnostics = append(diagnostics, a.checkBodyTernaryConditions(typ, member, body, bodyOffset, source, scopes, model)...)
	diagnostics = append(diagnostics, a.checkBodyExpressionTypeReferences(typ, member, body, bodyOffset, source)...)
	diagnostics = append(diagnostics, a.checkBodyCalls(typ, member, body, bodyOffset, source, scopes, model)...)
	return dedupeBodyDiagnostics(diagnostics)
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
			case "GLADESEMA006", "GLADESEMA008", "GLADESEMA009", "GLADESEMA010", "GLADESEMA011", "GLADESEMA015", "GLADESEMA018", "GLADESEMA019", "GLADESEMA020", "GLADESEMA022", "GLADESEMA023", "GLADESEMA024", "GLADESEMA025", "GLADESEMA026", "GLADESEMA027", "GLADESEMA028":
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

func semaFieldScope(model map[string]typeMembers, typeName string, seen map[string]bool) map[string]string {
	out := make(map[string]string)
	key := normalizeName(typeName)
	if key == "" || seen[key] {
		return out
	}
	seen[key] = true
	members, ok := model[key]
	if !ok {
		return out
	}
	for _, owner := range semaEnclosingTypeNames(members.name) {
		ownerMembers, ok := model[normalizeName(owner)]
		if !ok {
			continue
		}
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

func semaResolveField(model map[string]typeMembers, typeName, fieldName string, seen map[string]bool) (resolvedMember, bool) {
	return semaResolveFieldByKey(model, typeName, normalizeName(fieldName), seen)
}

func semaResolveFieldByKey(model map[string]typeMembers, typeName, fieldKey string, seen map[string]bool) (resolvedMember, bool) {
	key := normalizeName(typeName)
	if key == "" || seen[key] {
		return resolvedMember{}, false
	}
	seen[key] = true
	members, ok := model[key]
	if !ok {
		for _, candidateKey := range semaShortCandidateKeys(model, key) {
			if candidateKey == key || seen[candidateKey] {
				continue
			}
			candidate := model[candidateKey]
			if field, ok := semaResolveFieldByKey(model, candidate.name, fieldKey, seen); ok {
				return field, true
			}
		}
		return resolvedMember{}, false
	}
	if field, ok := members.fields[fieldKey]; ok {
		return resolvedMember{owner: members.name, member: field}, true
	}
	if field, ok := semaResolveFieldByKey(model, members.superClass, fieldKey, seen); ok {
		return field, true
	}
	return resolvedMember{}, false
}

func semaLooksLikeSchemaTokenPath(field string) bool {
	parts := strings.Split(field, ".")
	return len(parts) >= 2 && strings.EqualFold(parts[1], "SObjectType")
}

func semaResolveFieldPath(model map[string]typeMembers, receiverType, fieldPath string) (resolvedMember, bool) {
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
			if sobjectField, fieldOK := semaSObjectFieldMember(currentType, part, model); fieldOK {
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
			Type: "Schema.SObjectFields",
		}}, true
	case strings.EqualFold(currentType, "Schema.SObjectFields") && semaFieldTokenPart(part):
		return resolvedMember{owner: currentType, member: typesys.MemberSymbol{
			Kind: apexast.DeclarationField,
			Name: part,
			Type: "Schema.DescribeFieldResult",
		}}, true
	default:
		return resolvedMember{}, false
	}
}

func semaSObjectFieldMember(typeName, fieldName string, model map[string]typeMembers) (resolvedMember, bool) {
	if !isSemaSObjectLike(typeName, model) {
		return resolvedMember{}, false
	}
	fieldType := ""
	fieldKey := normalizeName(fieldName)
	switch {
	case fieldKey == "recordtype":
		fieldType = "RecordType"
	case fieldKey == "runninguserentityaccess":
		fieldType = "UserEntityAccess"
	case fieldKey == "runninguserfieldaccess":
		fieldType = "UserFieldAccess"
	case fieldKey == "fielddefinition":
		fieldType = "FieldDefinition"
	case fieldKey == "entitydefinition":
		fieldType = "EntityDefinition"
	case strings.HasSuffix(fieldKey, "__r"):
		fieldType = semaRelationshipFieldTypeForModel(model, fieldName)
	case fieldKey == "id" || strings.HasSuffix(fieldKey, "id"):
		fieldType = "Id"
	case fieldKey == "body" || fieldKey == "versiondata":
		fieldType = "Blob"
	case fieldKey == "email":
		fieldType = "String"
	case strings.HasSuffix(fieldKey, "address"):
		fieldType = "Address"
	case fieldKey == "assignee", fieldKey == "owner", fieldKey == "createdby", fieldKey == "lastmodifiedby", fieldKey == "user":
		fieldType = "User"
	case fieldKey == "contact":
		fieldType = "Contact"
	case fieldKey == "account", fieldKey == "parentaccount":
		fieldType = "Account"
	case strings.HasSuffix(fieldKey, "street") ||
		strings.HasSuffix(fieldKey, "city") ||
		strings.HasSuffix(fieldKey, "state") ||
		strings.HasSuffix(fieldKey, "statecode") ||
		strings.HasSuffix(fieldKey, "postalcode") ||
		strings.HasSuffix(fieldKey, "country") ||
		strings.HasSuffix(fieldKey, "countrycode"):
		fieldType = "String"
	case strings.Contains(fieldKey, "file") || strings.Contains(fieldKey, "name") || strings.Contains(fieldKey, "class"):
		fieldType = "String"
	case fieldKey == "name" || fieldKey == "developername" || fieldKey == "masterlabel":
		fieldType = "String"
	case fieldKey == "isdeleted" || strings.HasPrefix(fieldKey, "is") || strings.HasPrefix(fieldKey, "has"):
		fieldType = "Boolean"
	}
	return resolvedMember{owner: typeName, member: typesys.MemberSymbol{
		Kind:      apexast.DeclarationField,
		Name:      fieldName,
		Type:      fieldType,
		Modifiers: []string{"public"},
	}}, true
}

func semaParentRelationshipFieldName(fieldName string) string {
	if strings.HasSuffix(fieldName, "__c") {
		return strings.TrimSuffix(fieldName, "__c") + "__r"
	}
	return ""
}

func semaRelationshipFieldType(fieldName string) string {
	key := normalizeName(strings.TrimSuffix(fieldName, "__r"))
	raw := strings.TrimSuffix(fieldName, "__r")
	switch key {
	case "personcontact", "contact":
		return "Contact"
	case "account", "parentaccount":
		return "Account"
	case "owner", "createdby", "lastmodifiedby", "user":
		return "User"
	case "product2", "product":
		return "Product2"
	case "pricebook2":
		return "Pricebook2"
	case "pricebookentry":
		return "PricebookEntry"
	case "opportunity":
		return "Opportunity"
	case "order":
		return "Order"
	}
	if semaLooksLikeChildRelationship(key) {
		return "List<" + semaChildRelationshipElementType(raw) + ">"
	}
	return "SObject"
}

func semaRelationshipFieldTypeForModel(model map[string]typeMembers, fieldName string) string {
	raw := strings.TrimSuffix(fieldName, "__r")
	key := normalizeName(raw)
	if semaLooksLikeChildRelationship(key) {
		for _, candidate := range semaChildRelationshipTypeCandidates(raw) {
			if _, ok := model[normalizeName(candidate)]; ok {
				return "List<" + candidate + ">"
			}
		}
	}
	return semaRelationshipFieldType(fieldName)
}

func semaChildRelationshipTypeCandidates(name string) []string {
	base := semaChildRelationshipBase(name)
	if base == "" {
		return nil
	}
	return []string{base + "__c", base + "2__c", base + "__mdt", base + "2__mdt"}
}

func semaChildRelationshipBase(name string) string {
	trimmed := strings.TrimRight(name, "0123456789")
	if strings.HasSuffix(trimmed, "ies") {
		return strings.TrimSuffix(trimmed, "ies") + "y"
	}
	return strings.TrimSuffix(trimmed, "s")
}

func semaLooksLikeChildRelationship(key string) bool {
	trimmed := strings.TrimRight(key, "0123456789")
	return trimmed != key || strings.HasSuffix(trimmed, "s")
}

func semaChildRelationshipElementType(name string) string {
	trimmed := semaChildRelationshipBase(name)
	if trimmed == "" {
		return "SObject"
	}
	return trimmed + "__c"
}

type irSemaScope struct {
	frames []map[string]string
}

func newIRSemaScope(base map[string]string) irSemaScope {
	root := make(map[string]string, len(base))
	for name, typ := range base {
		root[normalizeName(name)] = typ
	}
	return irSemaScope{frames: []map[string]string{root}}
}

func (s *irSemaScope) push() {
	s.frames = append(s.frames, make(map[string]string))
}

func (s *irSemaScope) pop() {
	if len(s.frames) > 1 {
		s.frames = s.frames[:len(s.frames)-1]
	}
}

func (s *irSemaScope) declare(name, typ string) {
	if len(s.frames) == 0 {
		s.frames = append(s.frames, make(map[string]string))
	}
	s.frames[len(s.frames)-1][normalizeName(name)] = typ
}

func (s irSemaScope) lookup(name string) (string, bool) {
	key := normalizeName(name)
	for i := len(s.frames) - 1; i >= 0; i-- {
		if typ, ok := s.frames[i][key]; ok {
			return typ, true
		}
	}
	return "", false
}

func (s irSemaScope) flat() map[string]string {
	out := make(map[string]string)
	for _, frame := range s.frames {
		for name, typ := range frame {
			out[name] = typ
		}
	}
	return out
}

func (a *Analyzer) checkBodyIR(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return nil
	}
	scope := newIRSemaScope(base)
	diagnostics := performanceDiagnosticsForProgram(typ, program, bodyOffset, source)
	diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, program.Instructions, &scope, bodyOffset, source, model, constructability)...)
	returnType := strings.TrimSpace(member.Type)
	if returnType != "" && !strings.EqualFold(returnType, "void") && !irInstructionsTerminate(program.Instructions) {
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s on all paths", returnType), member.Range.Start.Offset, member.Range.End.Offset, source))
	}
	return diagnostics
}

func semaNormalizeMemberTypes(model map[string]typeMembers, owner string, member typesys.MemberSymbol) typesys.MemberSymbol {
	member.Type = resolveNestedTypeReference(model, owner, member.Type)
	for i := range member.Parameters {
		member.Parameters[i].Type = resolveNestedTypeReference(model, owner, member.Parameters[i].Type)
	}
	return member
}

func (a *Analyzer) checkIRInstructions(typ typesys.TypeSymbol, member typesys.MemberSymbol, instructions []ir.Instruction, scope *irSemaScope, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, inst := range instructions {
		switch inst.Op {
		case ir.OpDeclare:
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)...)
			diagnostics = append(diagnostics, a.checkIRAssignmentType(typ, member, inst.Type, inst.Name, inst.Expr, scope, inst.Pos, bodyOffset, source, model, "initializes")...)
			scope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type))
		case ir.OpBlock:
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
			scope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type))
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
					scope.declare(catchClause.Name, catchType)
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
					scope.declare(binding.name, binding.typ)
				}
				diagnostics = append(diagnostics, a.checkIRInstructions(typ, member, switchCase.Body, scope, bodyOffset, source, model, constructability)...)
				scope.pop()
			}
		}
	}
	return diagnostics
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
	case ir.OpBlock:
		return irInstructionsTerminate(inst.Then)
	case ir.OpIf:
		return len(inst.Then) > 0 && len(inst.Else) > 0 && irInstructionsTerminate(inst.Then) && irInstructionsTerminate(inst.Else)
	case ir.OpTry:
		if irInstructionsTerminate(inst.Finally) {
			return true
		}
		clauses := catchClauses(inst)
		if len(clauses) == 0 {
			return false
		}
		for _, catchClause := range clauses {
			if !irInstructionsTerminate(catchClause.Body) {
				return false
			}
		}
		return irInstructionsTerminate(inst.Then)
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

func (a *Analyzer) checkIRExprVariables(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	if expr.Kind == "" {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	switch expr.Kind {
	case ir.ExprVariable:
		if semaExprAtSwitchWhenLabel(source, bodyOffset+pos, expr.Name) {
			return nil
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

func (a *Analyzer) checkIRAssignmentTarget(typ typesys.TypeSymbol, member typesys.MemberSymbol, name string, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
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

func (a *Analyzer) checkIRCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
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
	receiverType := typ.Name
	method := expr.Callee
	explicitReceiver := false
	classLiteralReceiver := false
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
		if classMethod, ok := semaClassLiteralMethod(expr.Callee); ok {
			receiverType = "Type"
			method = classMethod
			classLiteralReceiver = true
		} else {
			switch {
			case strings.EqualFold(receiver, "this"):
				receiverType = typ.Name
			case strings.EqualFold(receiver, "super"):
				if members, ok := model[normalizeName(typ.Name)]; ok {
					receiverType = members.superClass
				}
			default:
				if scoped, ok := scope.lookup(receiver); ok {
					receiverType = scoped
				} else if _, ok := model[normalizeName(receiver)]; ok {
					receiverType = receiver
				} else if a.hasKnown(receiver) {
					return nil
				} else {
					return nil
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
		if receiver, _, ok := strings.Cut(expr.Callee, "."); ok && !classLiteralReceiver {
			if _, scoped := scope.lookup(receiver); !scoped {
				if _, ok := model[normalizeName(receiver)]; ok {
					receiverMode = "class"
				}
			}
		}
	}
	if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model, receiverMode); handled {
		return diagnostics
	}
	if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
		return diagnostics
	}
	candidates := resolveMemberMethods(model, receiverType, method)
	if !explicitReceiver {
		candidates = resolveImplicitMemberMethods(model, receiverType, method)
	}
	if len(candidates) == 0 && !strings.Contains(expr.Callee, ".") && bodyOffset >= 0 && bodyOffset <= len(source) {
		if chainedReceiver, chainedMethod, ok := semaChainedCallReceiverNear(source[bodyOffset:], pos, method, scope.flat(), model, typ.Name); ok && strings.EqualFold(chainedMethod, method) {
			receiverType = chainedReceiver
			explicitReceiver = true
			receiverMode = "instance"
			if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model, "instance"); handled {
				return diagnostics
			}
			if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverType, method, expr.Args, scope, pos, bodyOffset, source, model); handled {
				return diagnostics
			}
			candidates = resolveMemberMethods(model, receiverType, method)
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
			receiverTyp := inferSemaFieldAccessType(receiverExpr, scope.flat(), model)
			if receiverTyp == "" {
				receiverParts := strings.Split(receiverExpr, ".")
				if len(receiverParts) > 0 && strings.HasSuffix(normalizeName(receiverParts[len(receiverParts)-1]), "address") {
					receiverTyp = "Address"
				}
			}
			if receiverTyp != "" {
				if diagnostics, handled := a.checkIRPlatformCall(typ, member, receiverTyp, methodName, expr.Args, scope, pos, bodyOffset, source, model, "instance"); handled {
					return diagnostics
				}
				if diagnostics, handled := a.checkIRCollectionCall(typ, member, receiverTyp, methodName, expr.Args, scope, pos, bodyOffset, source, model); handled {
					return diagnostics
				}
			}
		}
		if semaRelationshipCollectionMethod(expr.Callee, method) {
			return nil
		}
		if semaKnownFluentHelperMethod(method) {
			return nil
		}
		if semaSourceHasDottedCall(source, method) {
			return nil
		}
		if explicitReceiver && a.hasKnown(receiverType) {
			return nil
		}
		return []diagnostic.Diagnostic{unknownCallDiagnostic(typ, member, expr.Callee, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)}
	}
	argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
	if candidate, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok {
		if staticDiagnostic, blocked := checkSemaStaticAccess(typ, member, expr.Callee, candidate, receiverMode, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source); blocked {
			return []diagnostic.Diagnostic{staticDiagnostic}
		}
		return nil
	} else if ambiguous {
		if semaArgTypesContainNullish(argTypes) {
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
	if semaKnownFluentHelperMethod(method) {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA009",
		Message:  fmt.Sprintf("%s %q has no matching overload for call %q with %d argument(s)", member.Kind, member.Name, expr.Callee, len(expr.Args)),
		File:     typ.File,
		Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee))),
	}}
}

func (a *Analyzer) checkIRCollectionCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) ([]diagnostic.Diagnostic, bool) {
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

func (a *Analyzer) checkIRPlatformCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers, receiverMode string) ([]diagnostic.Diagnostic, bool) {
	if semaDatabaseDynamicQueryCall(receiverType, method) {
		return nil, true
	}
	if _, ok := semaCollectionMethodSignature(receiverType, method); ok {
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
	if semaDatabaseDMLReturnType(receiverType, method, argTypes) != "" && len(args) <= 4 {
		return nil, true
	}
	if semaArgsMatchAny(sig.params, argTypes, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), bodyOffset+pos, bodyOffset+pos+max(1, len(method)), source)}, true
}

func (a *Analyzer) checkIRConstructorCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	typeName := strings.TrimPrefix(expr.Callee, "new:")
	resolvedTypeName := resolveNestedTypeReference(model, typ.Name, typeName)
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
	target, ok := model[normalizeName(resolvedTypeName)]
	if !ok {
		return nil
	}
	if len(target.constructors) == 0 {
		if len(expr.Args) == 0 || a.allowsInheritedExceptionConstructor(resolvedTypeName, expr.Args, scope, model, typ.Name) {
			return nil
		}
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("no matching %s constructor with %d argument(s)", typeName, len(expr.Args)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	argTypes := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
	namedArgTypes := irCallNamedArgTypes(a, expr.NamedArgs, scope, model, typ.Name)
	if candidate, ok, ambiguous := bestConstructorByArgTypes(target.constructors, argTypes, namedArgTypes, model); ok {
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, "new "+typeName, resolvedMember{owner: target.name, member: candidate}, bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source, model); blocked {
			return []diagnostic.Diagnostic{visibilityDiagnostic}
		}
		return nil
	} else if ambiguous {
		if semaAllowAmbiguousPlatformConstructor(resolvedTypeName, argTypes) || semaArgTypesContainNullish(argTypes) {
			return nil
		}
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, "new "+typeName, fmt.Sprintf("ambiguous %s constructor with %d argument(s)", typeName, len(expr.Args)+len(expr.NamedArgs)), bodyOffset+pos, bodyOffset+pos+max(1, len(typeName)), source)}
	}
	if a.allowsInheritedExceptionConstructor(resolvedTypeName, expr.Args, scope, model, typ.Name) {
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

func (a *Analyzer) allowsInheritedExceptionConstructor(typeName string, args []ir.Expr, scope irSemaScope, model map[string]typeMembers, ownerType string) bool {
	if !semaTypeMatches(model, typeName, "Exception", make(map[string]bool)) {
		return false
	}
	argTypes := irCallArgTypes(a, args, scope, model, ownerType)
	return semaArgsMatchAny([][]string{
		{},
		{"String"},
		{"Exception"},
		{"String", "Exception"},
	}, argTypes, model)
}

func semaAllowsInheritedExceptionConstructorArgs(typeName string, args []semaArg, scope map[string]string, model map[string]typeMembers) bool {
	if !semaTypeMatches(model, typeName, "Exception", make(map[string]bool)) {
		return false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	return semaArgsMatchAny([][]string{
		{},
		{"String"},
		{"Exception"},
		{"String", "Exception"},
	}, argTypes, model)
}

func (a *Analyzer) checkIRCollectionConstructor(typ typesys.TypeSymbol, member typesys.MemberSymbol, typeName string, args []ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) ([]diagnostic.Diagnostic, bool) {
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

func semaMapEntriesAssignable(a *Analyzer, keyType, valueType string, args []ir.Expr, scope irSemaScope, model map[string]typeMembers, currentType string) bool {
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

func semaCollectionCopyConstructorAccepts(targetBase, targetElement, argType string, model map[string]typeMembers) bool {
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

func semaMapConstructorAccepts(keyType, valueType, argType string, model map[string]typeMembers) bool {
	if strings.EqualFold(argType, "Database.QueryResult") && strings.EqualFold(keyType, "Id") {
		return strings.EqualFold(valueType, "SObject") || isSemaSObjectLike(valueType, model)
	}
	sourceBase, sourceArgs := semaGenericBaseAndArgs(argType)
	sourceBaseKey := normalizeName(sourceBase)
	if sourceBaseKey == "map" && len(sourceArgs) == 2 {
		return semaAssignableToType(keyType, sourceArgs[0], model) && semaAssignableToType(valueType, sourceArgs[1], model)
	}
	if sourceBaseKey == "list" && len(sourceArgs) == 1 && strings.EqualFold(keyType, "Id") {
		return semaAssignableToType(valueType, sourceArgs[0], model)
	}
	return false
}

func irCallArgTypes(a *Analyzer, args []ir.Expr, scope irSemaScope, model map[string]typeMembers, currentType string) []string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = resolveNestedTypeReference(model, currentType, a.inferIRExprType(arg, scope, model, currentType))
	}
	return argTypes
}

func irCallNamedArgTypes(a *Analyzer, args []ir.NamedArg, scope irSemaScope, model map[string]typeMembers, currentType string) map[string]string {
	if len(args) == 0 {
		return nil
	}
	argTypes := make(map[string]string, len(args))
	for _, arg := range args {
		if arg.Name == "" {
			continue
		}
		argTypes[arg.Name] = resolveNestedTypeReference(model, currentType, a.inferIRExprType(arg.Expr, scope, model, currentType))
	}
	return argTypes
}

func irCallArgsMatch(a *Analyzer, params []apexast.Parameter, args []ir.Expr, scope irSemaScope, model map[string]typeMembers, currentType string) bool {
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

func (a *Analyzer) checkIRAssignmentType(typ typesys.TypeSymbol, member typesys.MemberSymbol, targetType, target string, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers, verb string) []diagnostic.Diagnostic {
	targetType = resolveNestedTypeReference(model, typ.Name, targetType)
	valueType := resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(expr, *scope, model, typ.Name))
	if valueType == "" || valueType == "null" || semaAssignableToType(targetType, valueType, model) {
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

func (a *Analyzer) checkIRReturnType(typ typesys.TypeSymbol, member typesys.MemberSymbol, returnType string, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
	valueType := resolveNestedTypeReference(model, typ.Name, a.inferIRExprType(expr, *scope, model, typ.Name))
	if valueType == "" || valueType == "null" || semaAssignableToType(returnType, valueType, model) {
		return nil
	}
	if strings.EqualFold(returnType, "Boolean") && semaMemberReturnSourceLooksBoolean(source, 0, len(source)) {
		return nil
	}
	return []diagnostic.Diagnostic{returnTypeDiagnostic(typ, member, fmt.Sprintf("returns %s from %s method", valueType, returnType), bodyOffset+pos, bodyOffset+pos+max(1, len(valueType)), source)}
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

func (a *Analyzer) checkIRConditionType(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope *irSemaScope, pos, bodyOffset int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
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

func (a *Analyzer) checkIRForEachType(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope irSemaScope, bodyOffset int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
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

func (a *Analyzer) inferIRExprType(expr ir.Expr, scope irSemaScope, model map[string]typeMembers, currentType string) string {
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
		if typ := semaEnumValuePathType(model, expr.Name); typ != "" {
			return typ
		}
		if semaLooksLikeSObjectFieldStringPropertyPath(expr.Name) {
			return "String"
		}
		if root, field, ok := strings.Cut(expr.Name, "."); ok {
			if _, scoped := scope.lookup(root); !scoped {
				if target, staticOK := semaStaticClassFieldPathMemberInContext(model, currentType, root, field); staticOK && !hasModifier(target.member.Modifiers, semaSyntheticStandardSObjectFieldModifier) {
					if owner, ok := model[normalizeName(target.owner)]; !ok || !owner.sobject {
						return target.member.Type
					}
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
		if root, field, ok := strings.Cut(expr.Name, "."); ok {
			if _, scoped := scope.lookup(root); !scoped {
				if target, staticOK := semaStaticClassFieldPathMemberInContext(model, currentType, root, field); staticOK && !hasModifier(target.member.Modifiers, semaSyntheticStandardSObjectFieldModifier) {
					return target.member.Type
				}
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
		if (strings.HasPrefix(expr.Callee, "__field:") || strings.HasPrefix(expr.Callee, "__safe_field:")) && expr.Left != nil {
			receiverType := a.inferIRExprType(*expr.Left, scope, model, currentType)
			if receiverType == "" {
				return ""
			}
			field := strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__safe_field:"), "__field:")
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
			if sig, ok := semaCollectionMethodSignature(receiverType, method); ok {
				return sig.returnType
			}
			if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok {
				return sig.returnType
			}
			if sig, ok := semaEnumMethodSignature(model, receiverType, method); ok {
				return sig.returnType
			}
			return semaResolvedIRCallReturnType(a, model, receiverType, method, expr.Args, scope, currentType)
		}
		if receiver, method, ok := splitSemaMethodPath(expr.Callee); ok {
			receiverType := semaTextReceiverType(receiver, scope.flat(), model)
			return semaResolvedIRCallReturnType(a, model, receiverType, method, expr.Args, scope, currentType)
		}
		return semaResolvedIRCallReturnType(a, model, currentType, expr.Callee, expr.Args, scope, currentType)
	case ir.ExprUnary:
		switch expr.Operator {
		case "!":
			return "Boolean"
		case "-":
			if expr.Left != nil {
				return a.inferIRExprType(*expr.Left, scope, model, currentType)
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

func semaIRExprLooksLikeStaticSObjectToken(expr string, scope irSemaScope, model map[string]typeMembers) bool {
	root, _, ok := strings.Cut(strings.TrimSpace(expr), ".")
	if !ok || root == "" {
		return false
	}
	if scopedType, scoped := scope.lookup(root); scoped {
		return root == scopedType
	}
	return semaLooksLikeSObjectFieldTokenInModel(expr, model) || semaLooksLikeSObjectTypeTokenInModel(expr, model)
}

func semaIRReceiverType(receiver string, scope irSemaScope, model map[string]typeMembers, currentType string) string {
	switch {
	case strings.EqualFold(receiver, "this"):
		return currentType
	case strings.EqualFold(receiver, "super"):
		if members, ok := model[normalizeName(currentType)]; ok {
			return members.superClass
		}
	case receiver == "":
		return ""
	default:
		if scoped, ok := scope.lookup(receiver); ok {
			return scoped
		}
		if _, ok := model[normalizeName(receiver)]; ok {
			return receiver
		}
	}
	return ""
}

func (a *Analyzer) inferFlattenedIRCallType(expr ir.Expr, scope irSemaScope, model map[string]typeMembers, currentType string) string {
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

func semaResolvedIRCallReturnType(a *Analyzer, model map[string]typeMembers, receiverType, method string, args []ir.Expr, scope irSemaScope, currentType string) string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = a.inferIRExprType(arg, scope, model, currentType)
	}
	if stubbedType := semaCreateStubReturnTypeFromIR(model, receiverType, method, args, currentType); stubbedType != "" {
		return stubbedType
	}
	if semaDatabaseDynamicQueryCall(receiverType, method) {
		return "Database.QueryResult"
	}
	if returnType := semaDatabaseDMLReturnType(receiverType, method, argTypes); returnType != "" {
		return returnType
	}
	if sig, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return sig.returnType
	}
	if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok {
		return sig.returnType
	}
	if sig, ok := semaEnumMethodSignature(model, receiverType, method); ok {
		return sig.returnType
	}
	if candidate, ok, _ := bestResolvedMemberByArgTypes(resolveMemberMethods(model, receiverType, method), argTypes, model); ok {
		return candidate.member.Type
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
	}
	return ""
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

func irAssignmentTargetType(name string, scope irSemaScope, model map[string]typeMembers, currentType string) (string, bool) {
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

func (a *Analyzer) irVariableDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, name string, scope irSemaScope, model map[string]typeMembers, start int, source string) (diagnostic.Diagnostic, bool) {
	root, field, hasMember := strings.Cut(name, ".")
	if !hasMember {
		return diagnostic.Diagnostic{}, false
	}
	receiverType := ""
	switch {
	case strings.EqualFold(root, "this"):
		receiverType = typ.Name
	case strings.EqualFold(root, "super"):
		if members, ok := model[normalizeName(typ.Name)]; ok {
			receiverType = members.superClass
		}
	default:
		if scoped, ok := scope.lookup(root); ok {
			receiverType = scoped
		} else if resolved := resolveNestedTypeName(model, typ.Name, root); resolved != "" {
			if _, ok := model[normalizeName(resolved)]; ok {
				receiverType = resolved
			}
		} else if _, ok := model[normalizeName(root)]; ok {
			receiverType = root
		}
	}
	if receiverType == "" {
		return diagnostic.Diagnostic{}, false
	}
	if _, ok := model[normalizeName(receiverType)]; !ok {
		return diagnostic.Diagnostic{}, false
	}
	if strings.EqualFold(field, "class") {
		return diagnostic.Diagnostic{}, false
	}
	if strings.HasSuffix(strings.ToLower(field), ".class") {
		nestedType := strings.TrimSpace(field[:len(field)-len(".class")])
		if resolved := resolveNestedTypeName(model, receiverType, nestedType); resolved != "" {
			if _, ok := model[normalizeName(resolved)]; ok {
				return diagnostic.Diagnostic{}, false
			}
		}
	}
	if _, ok := model[normalizeName(receiverType+"."+field)]; ok {
		return diagnostic.Diagnostic{}, false
	}
	if strings.EqualFold(receiverType, "Schema") && semaLooksLikeSchemaTokenPath(field) {
		return diagnostic.Diagnostic{}, false
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
	if members, ok := model[normalizeName(receiverType)]; ok && members.kind == apexast.DeclarationEnum {
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

func semaResolveNestedStaticField(model map[string]typeMembers, receiverType, fieldPath string) (resolvedMember, bool) {
	parts := strings.Split(fieldPath, ".")
	if len(parts) < 2 {
		return resolvedMember{}, false
	}
	for i := len(parts) - 1; i > 0; i-- {
		typeName := resolveNestedTypeName(model, receiverType, strings.Join(parts[:i], "."))
		if _, ok := model[normalizeName(typeName)]; !ok {
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

func (a *Analyzer) irVariableKnown(name string, scope irSemaScope, model map[string]typeMembers, currentType string) bool {
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
		if _, ok := model[normalizeName(root)]; ok {
			return true
		}
	}
	if _, ok := scope.lookup(root); ok {
		return true
	}
	if _, ok := scope.lookup(name); ok {
		return true
	}
	if hasMember && (a.hasKnown(root) || model[normalizeName(root)].name != "") {
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
	typeName   string
	start      int
	scopeStart int
	scopeEnd   int
}

type semaScopeModel struct {
	base   map[string]string
	locals []semaLocal
}

func (a *Analyzer) collectBodyScopes(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, base map[string]string, model map[string]typeMembers) (semaScopeModel, []diagnostic.Diagnostic) {
	scopes := semaScopeModel{base: base}
	var diagnostics []diagnostic.Diagnostic
	for _, match := range enhancedForLocalPattern.FindAllStringSubmatchIndex(body, -1) {
		typeName := strings.TrimSpace(body[match[2]:match[3]])
		name := strings.TrimSpace(body[match[4]:match[5]])
		if isSemaKeyword(typeName) {
			continue
		}
		scopeStart, scopeEnd := blockBoundsAt(body, match[0])
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
		scopes.locals = append(scopes.locals, semaLocal{name: name, typeName: resolveNestedTypeReference(model, typ.Name, typeName), start: match[5], scopeStart: scopeStart, scopeEnd: scopeEnd})
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
	for _, match := range localDeclPattern.FindAllStringSubmatchIndex(body, -1) {
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
		scopes.locals = append(scopes.locals, local)
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
		scopes.locals = append(scopes.locals, semaLocal{name: name, typeName: resolveNestedTypeReference(model, typ.Name, firstCatchType(typeName)), start: scopeStart, scopeStart: scopeStart, scopeEnd: scopeEnd})
	}
	return scopes, diagnostics
}
