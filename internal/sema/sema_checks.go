package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func (a *Analyzer) checkTriggers(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, trigger := range index.Triggers {
		if trigger.ObjectName == "" || a.hasKnown(trigger.ObjectName) {
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA001",
			Message:  fmt.Sprintf("trigger %q references unknown SObject %q", trigger.Name, trigger.ObjectName),
			File:     trigger.File,
			Range:    &trigger.Range,
		})
	}
	return diagnostics
}
func (a *Analyzer) checkMemberTypes(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if typ.Artifact {
			continue
		}
		for _, member := range typ.Members {
			if member.Type == "" || member.Type == "void" {
				continue
			}
			for _, ref := range extractTypeNames(member.Type) {
				if a.hasKnown(ref) {
					continue
				}
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA002",
					Message:  fmt.Sprintf("%s %q references unknown type %q", member.Kind, member.Name, ref),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
	}
	return diagnostics
}
func (a *Analyzer) checkMethodParameters(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if typ.Artifact {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod && member.Kind != apexast.DeclarationConstructor {
				continue
			}
			for _, param := range member.Parameters {
				for _, ref := range extractTypeNames(param.Type) {
					if a.hasKnown(ref) {
						continue
					}
					diagnostics = append(diagnostics, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     "GLADESEMA004",
						Message:  fmt.Sprintf("%s %q parameter %q references unknown type %q", member.Kind, member.Name, param.Name, ref),
						File:     typ.File,
						Range:    &param.Range,
					})
				}
			}
		}
	}
	return diagnostics
}
func (a *Analyzer) checkSchemaReferences(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, object := range index.Objects {
		for _, field := range object.Fields {
			for _, referenceTo := range field.ReferenceTo {
				if referenceTo == "" || a.hasKnown(referenceTo) {
					continue
				}
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA003",
					Message:  fmt.Sprintf("field %s.%s references unknown SObject %q", object.Name, field.Name, referenceTo),
				})
			}
		}
	}
	return diagnostics
}
func (a *Analyzer) checkAnnotations(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if hasModifier(typ.Modifiers, "RestResource") && typ.Kind != apexast.DeclarationClass {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, typ.Range, "RestResource is only valid on classes"))
		}
		for _, member := range typ.Members {
			diagnostics = append(diagnostics, checkMemberAnnotations(typ, member)...)
		}
	}
	return diagnostics
}
func checkMemberAnnotations(typ typesys.TypeSymbol, member typesys.MemberSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if hasModifier(member.Modifiers, "TestSetup") {
		if member.Kind != apexast.DeclarationMethod || !hasModifier(typ.Modifiers, "IsTest") || !hasModifier(member.Modifiers, "static") || !strings.EqualFold(member.Type, "void") || len(member.Parameters) != 0 {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "TestSetup methods must be static void no-arg methods inside IsTest classes"))
		}
	}
	if hasModifier(member.Modifiers, "future") {
		if member.Kind != apexast.DeclarationMethod || !hasModifier(member.Modifiers, "static") || !strings.EqualFold(member.Type, "void") {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "future methods must be static void methods"))
		}
	}
	if hasAnyAnnotation(member.Modifiers, "HttpDelete", "HttpGet", "HttpPatch", "HttpPost", "HttpPut") {
		if member.Kind != apexast.DeclarationMethod {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "HTTP method annotations are only valid on methods"))
		}
		if !hasModifier(typ.Modifiers, "RestResource") {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "HTTP method annotations require a RestResource class"))
		}
	}
	if hasModifier(member.Modifiers, "InvocableMethod") {
		if member.Kind != apexast.DeclarationMethod || !hasModifier(member.Modifiers, "static") {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "InvocableMethod is only valid on static methods"))
		}
	}
	if hasModifier(member.Modifiers, "InvocableVariable") {
		if member.Kind != apexast.DeclarationField && member.Kind != apexast.DeclarationProperty {
			diagnostics = append(diagnostics, annotationDiagnostic(typ.File, member.Range, "InvocableVariable is only valid on fields and properties"))
		}
	}
	return diagnostics
}
func (a *Analyzer) checkVisibility(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if hasAnyModifier(typ.Modifiers, "public", "global") {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA005",
				Message:  fmt.Sprintf("%s %q cannot be both public and global", typ.Kind, typ.Name),
				File:     typ.File,
				Range:    &typ.Range,
			})
		}
		if typ.Kind != apexast.DeclarationInterface {
			continue
		}
		for _, member := range typ.Members {
			if hasModifier(member.Modifiers, "private") || hasModifier(member.Modifiers, "protected") {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA005",
					Message:  fmt.Sprintf("interface method %q cannot be private or protected", member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
	}
	return diagnostics
}
func (a *Analyzer) checkManagedPackageAccess(index typesys.Index) []diagnostic.Diagnostic {
	dependencyNamespaces := make(map[string]typesys.DependencyInfo)
	for _, dep := range index.Dependencies {
		if dep.Status == "loaded" {
			dependencyNamespaces[strings.ToLower(dep.Namespace)] = dep
		}
	}
	if len(dependencyNamespaces) == 0 {
		return nil
	}
	typesByNamespace := make(map[string][]typesys.TypeSymbol)
	for _, typ := range index.Types {
		if typ.Namespace == "" {
			continue
		}
		typesByNamespace[strings.ToLower(typ.Namespace)] = append(typesByNamespace[strings.ToLower(typ.Namespace)], typ)
	}
	var diagnostics []diagnostic.Diagnostic
	sourceCache := make(map[string]string)
	seen := make(map[string]bool)
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		source, ok := readSemaSource(typ.File, sourceCache)
		if !ok {
			continue
		}
		for namespace, dep := range dependencyNamespaces {
			for _, ref := range managedPackageReferences(source, dep.Namespace) {
				key := typ.File + ":" + ref.Namespace + "." + ref.TypeName + ":" + ref.MemberName
				if seen[key] {
					continue
				}
				seen[key] = true
				depType, ok := findManagedPackageType(typesByNamespace[namespace], ref.TypeName)
				if !ok {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     "dependency_unknown_symbol",
						Message:  fmt.Sprintf("managed package dependency %s does not expose type %q", dep.Namespace, ref.TypeName),
						File:     typ.File,
					})
					continue
				}
				if !hasModifier(depType.Modifiers, "global") && !hasModifier(depType.Modifiers, "webservice") {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     "dependency_access_denied",
						Message:  fmt.Sprintf("managed package dependency %s type %q is not global", dep.Namespace, depType.Name),
						File:     typ.File,
					})
					continue
				}
				if ref.MemberName == "" {
					continue
				}
				member, ok := findManagedPackageMember(depType, ref.MemberName)
				if !ok {
					continue
				}
				if !hasModifier(member.Modifiers, "global") && !hasModifier(member.Modifiers, "webservice") {
					diagnostics = append(diagnostics, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     "dependency_member_access_denied",
						Message:  fmt.Sprintf("managed package dependency %s member %q on %q is not global", dep.Namespace, member.Name, depType.Name),
						File:     typ.File,
					})
				}
			}
		}
	}
	return diagnostics
}
func (a *Analyzer) checkInheritanceContracts(index typesys.Index) []diagnostic.Diagnostic {
	model := buildTypeMembers(index)
	defer unregisterSemaShortCandidateIndex(model)
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if typ.Artifact {
			continue
		}
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		abstractClass := hasModifier(typ.Modifiers, "abstract")
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod {
				continue
			}
			if hasModifier(member.Modifiers, "override") && !hasInheritedMethodSignature(model, typ, member) {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA016",
					Message:  fmt.Sprintf("method %q is marked override but no inherited method has the same signature", member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
			if hasModifier(member.Modifiers, "abstract") && !abstractClass {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     "GLADESEMA017",
					Message:  fmt.Sprintf("concrete class %q declares abstract method %q", typ.Name, member.Name),
					File:     typ.File,
					Range:    &member.Range,
				})
			}
		}
		if abstractClass {
			continue
		}
		required := requiredMethodSignatures(model, typ)
		for _, requirement := range required {
			if hasConcreteMethodSignature(model, typ.Name, requirement.member) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA017",
				Message:  fmt.Sprintf("concrete class %q must implement %s method %q from %q", typ.Name, requirement.sourceKind, requirement.member.Name, requirement.owner),
				File:     typ.File,
				Range:    &typ.Range,
			})
		}
		diagnostics = append(diagnostics, checkDatabaseBatchableGenericContract(model, typ)...)
	}
	return diagnostics
}

func checkDatabaseBatchableGenericContract(model map[string]typeMembers, typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	members, _, ok := semaLookupTypeMembers(model, typ.Name)
	if !ok {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	for _, iface := range members.interfaces {
		base, args := semaGenericBaseAndArgs(iface)
		if !strings.EqualFold(base, "Database.Batchable") && !strings.EqualFold(base, "Batchable") {
			continue
		}
		itemType := "Object"
		if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
			itemType = strings.TrimSpace(args[0])
		}
		for _, methodName := range []string{"start", "execute", "finish"} {
			methods := concreteMethodsByName(model, typ.Name, methodName)
			if len(methods) == 0 {
				continue
			}
			if databaseBatchableMethodCompatible(methodName, itemType, methods, model) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA017",
				Message:  fmt.Sprintf("concrete class %q must implement Database.Batchable<%s> method %q with the matching signature", typ.Name, itemType, methodName),
				File:     typ.File,
				Range:    &typ.Range,
			})
		}
	}
	return diagnostics
}

func concreteMethodsByName(model map[string]typeMembers, typeName, methodName string) []typesys.MemberSymbol {
	var out []typesys.MemberSymbol
	for current := typeName; current != ""; {
		members, ok := model[normalizeName(current)]
		if !ok {
			return out
		}
		for _, method := range members.methods[normalizeName(methodName)] {
			if !hasModifier(method.Modifiers, "abstract") {
				out = append(out, method)
			}
		}
		current = members.superClass
	}
	return out
}

func databaseBatchableMethodCompatible(methodName, itemType string, methods []typesys.MemberSymbol, model map[string]typeMembers) bool {
	for _, method := range methods {
		switch strings.ToLower(methodName) {
		case "start":
			if len(method.Parameters) != 1 || !sameSemaSignatureType(method.Parameters[0].Type, "Database.BatchableContext") {
				continue
			}
			if strings.EqualFold(method.Type, "Database.QueryLocator") || databaseBatchableStartReturnCompatible(itemType, method.Type) {
				return true
			}
		case "execute":
			if len(method.Parameters) != 2 ||
				!sameSemaSignatureType(method.Parameters[0].Type, "Database.BatchableContext") ||
				!sameSemaSignatureType(method.Parameters[1].Type, "List<"+itemType+">") {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(method.Type), "void") || strings.TrimSpace(method.Type) == "" {
				return true
			}
		case "finish":
			if len(method.Parameters) != 1 || !sameSemaSignatureType(method.Parameters[0].Type, "Database.BatchableContext") {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(method.Type), "void") || strings.TrimSpace(method.Type) == "" {
				return true
			}
		}
	}
	return false
}

func databaseBatchableStartReturnCompatible(itemType, returnType string) bool {
	base, args := semaGenericBaseAndArgs(returnType)
	if (!strings.EqualFold(base, "Iterable") && !strings.EqualFold(base, "List")) || len(args) != 1 {
		return false
	}
	return sameSemaSignatureType(args[0], itemType)
}

func (a *Analyzer) checkBodyAssignments(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, match := range assignmentPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaOffsetInIgnoredText(body, match[0]) {
			continue
		}
		target := strings.TrimSpace(body[match[2]:match[3]])
		if semaAssignmentLooksLikeNamedArg(body, match[2]) {
			continue
		}
		if semaAssignmentLooksLikeMapEntry(body, match[1]) {
			continue
		}
		if semaAssignmentLooksLikeLocalDeclaration(body, match[2]) {
			continue
		}
		targetType, ok := scopes.visibleAt(target, match[2])
		if ok {
			value := trimSemaArg(body, match[1], semaStatementEnd(body, match[1]))
			valueType := inferSemaArgTypeWithModel(value.text, scopes.flat(), model)
			if valueType == "" || valueType == "null" || semaAssignableToType(targetType, valueType, model) {
				continue
			}
			diagnostics = append(diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA018",
				Message:  fmt.Sprintf("%s %q assigns %s to %s variable %q", member.Kind, member.Name, valueType, targetType, target),
				File:     typ.File,
				Range:    semaRange(source, bodyOffset+value.start, bodyOffset+value.end),
			})
			continue
		}
		if semaAnyKnownField(model, target) {
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA013",
			Message:  fmt.Sprintf("%s %q assigns unknown variable %q", member.Kind, member.Name, target),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+match[2], bodyOffset+match[3]),
		})
	}
	return diagnostics
}
func (a *Analyzer) checkBodyReturns(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	if member.Type == "" {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	returnType := strings.TrimSpace(member.Type)
	foundReturn := false
	for _, match := range returnPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaReturnMatchInIgnoredText(body, match) {
			continue
		}
		foundReturn = true
		hasValue := match[2] >= 0
		if strings.EqualFold(returnType, "void") {
			if hasValue {
				diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, "void method cannot return a value", bodyOffset+match[2], bodyOffset+match[3], source))
			}
			continue
		}
		if !hasValue {
			diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s", returnType), bodyOffset+match[0], bodyOffset+match[1], source))
			continue
		}
		value := trimSemaArg(body, match[2], match[3])
		valueType := resolveNestedTypeReference(model, typ.Name, inferSemaArgTypeWithModel(value.text, scopes.flat(), model))
		if strings.EqualFold(returnType, "Boolean") && semaExprContainsComparison(value.text) {
			valueType = "Boolean"
		}
		if valueType == "" || valueType == "null" || semaAssignableToType(returnType, valueType, model) {
			continue
		}
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("returns %s from %s method", valueType, returnType), bodyOffset+value.start, bodyOffset+value.end, source))
	}
	if !foundReturn && !strings.EqualFold(returnType, "void") && !semaBodyEndsWithThrow(body) {
		diagnostics = append(diagnostics, returnTypeDiagnostic(typ, member, fmt.Sprintf("method must return %s", returnType), member.Range.Start.Offset, member.Range.End.Offset, source))
	}
	return diagnostics
}
func (a *Analyzer) checkBodyTernaryConditions(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	seen := make(map[int]bool)
	for _, expr := range semaBodyExpressions(body) {
		if seen[expr.start] {
			continue
		}
		seen[expr.start] = true
		diagnostics = append(diagnostics, checkSemaTernaryCondition(typ, member, expr.text, bodyOffset+expr.start, source, scopes.flat(), model)...)
	}
	return diagnostics
}
func checkSemaTernaryCondition(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr string, exprStart int, source string, scope map[string]string, model map[string]typeMembers) []diagnostic.Diagnostic {
	question, colon, ok := semaTernaryPositions(expr)
	if !ok {
		return nil
	}
	var diagnostics []diagnostic.Diagnostic
	condition := strings.TrimSpace(expr[:question])
	conditionStart := exprStart + leadingWhitespaceLen(expr[:question])
	conditionType := inferSemaArgTypeWithModel(condition, scope, model)
	if conditionType != "" && !strings.EqualFold(conditionType, "Boolean") {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA020",
			Message:  fmt.Sprintf("%s %q uses %s expression as a ternary condition", member.Kind, member.Name, conditionType),
			File:     typ.File,
			Range:    semaRange(source, conditionStart, conditionStart+max(1, len(condition))),
		})
	}
	whenTrue := strings.TrimSpace(expr[question+1 : colon])
	trueStart := exprStart + question + 1 + leadingWhitespaceLen(expr[question+1:colon])
	whenFalse := strings.TrimSpace(expr[colon+1:])
	falseStart := exprStart + colon + 1 + leadingWhitespaceLen(expr[colon+1:])
	diagnostics = append(diagnostics, checkSemaTernaryCondition(typ, member, whenTrue, trueStart, source, scope, model)...)
	diagnostics = append(diagnostics, checkSemaTernaryCondition(typ, member, whenFalse, falseStart, source, scope, model)...)
	return diagnostics
}
func (a *Analyzer) checkBodyExpressionTypeReferences(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	seen := make(map[string]bool)
	for _, expr := range semaBodyExpressions(body) {
		diagnostics = append(diagnostics, a.checkSemaExpressionTypeReferences(typ, member, expr.text, bodyOffset+expr.start, source, seen)...)
	}
	return diagnostics
}
func (a *Analyzer) checkSemaExpressionTypeReferences(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr string, exprStart int, source string, seen map[string]bool) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if _, whenTrue, whenFalse, ok := splitSemaTernary(expr); ok {
		question, colon, _ := semaTernaryPositions(expr)
		condition := strings.TrimSpace(expr[:question])
		conditionStart := exprStart + leadingWhitespaceLen(expr[:question])
		trueStart := exprStart + question + 1 + leadingWhitespaceLen(expr[question+1:colon])
		falseStart := exprStart + colon + 1 + leadingWhitespaceLen(expr[colon+1:])
		diagnostics = append(diagnostics, a.checkSemaExpressionTypeReferences(typ, member, condition, conditionStart, source, seen)...)
		diagnostics = append(diagnostics, a.checkSemaExpressionTypeReferences(typ, member, strings.TrimSpace(whenTrue), trueStart, source, seen)...)
		diagnostics = append(diagnostics, a.checkSemaExpressionTypeReferences(typ, member, strings.TrimSpace(whenFalse), falseStart, source, seen)...)
		return diagnostics
	}
	if castType, _, ok := splitSemaCast(expr); ok {
		diagnostics = append(diagnostics, a.expressionTypeReferenceDiagnostics(typ, member, castType, exprStart+1, source, seen)...)
	}
	if _, instanceType, ok := splitSemaInstanceOf(expr); ok {
		typeStart := strings.LastIndex(expr, instanceType)
		if typeStart < 0 {
			typeStart = len(expr) - len(instanceType)
		}
		diagnostics = append(diagnostics, a.expressionTypeReferenceDiagnostics(typ, member, instanceType, exprStart+typeStart, source, seen)...)
	}
	return diagnostics
}
func checkSemaPlatformCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []semaArg, start, end int, source string, scope map[string]string, model map[string]typeMembers, receiverMode string) ([]diagnostic.Diagnostic, bool) {
	if strings.EqualFold(receiverType, "System") && strings.EqualFold(method, "runAs") && len(args) == 1 {
		return nil, true
	}
	if semaDatabaseDynamicQueryCall(receiverType, method) {
		return nil, true
	}
	if _, ok := semaCollectionMethodSignature(receiverType, method); ok {
		return nil, false
	}
	if staticDiagnostic, blocked := checkGeneratedPlatformStaticAccess(typ, member, receiverType, method, receiverMode, start, end, source, model); blocked {
		return []diagnostic.Diagnostic{staticDiagnostic}, true
	}
	sig, ok := semaPlatformMethodSignatureForMode(model, receiverType, method, receiverMode)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if semaDatabaseDMLReturnType(receiverType, method, argTypes) != "" && len(args) <= 4 {
		return nil, true
	}
	if semaArgsMatchAny(sig.params, argTypes, model) || semaCollectionFieldPathArgsMatch(sig.params, args, scope, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), start, end, source)}, true
}
func checkGeneratedPlatformStaticAccess(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method, receiverMode string, start, end int, source string, model map[string]typeMembers) (diagnostic.Diagnostic, bool) {
	switch receiverMode {
	case "class", "instance", "implicit":
	default:
		return diagnostic.Diagnostic{}, false
	}
	candidates := resolveMemberMethods(model, receiverType, method)
	if len(candidates) == 0 {
		canonical := semaCanonicalPlatformAlias(receiverType)
		if !strings.EqualFold(canonical, receiverType) {
			candidates = resolveMemberMethods(model, canonical, method)
		}
	}
	if len(candidates) == 0 {
		return diagnostic.Diagnostic{}, false
	}
	if owner, ok := model[normalizeName(candidates[0].owner)]; !ok || (!owner.dependency && !owner.sobject) {
		return diagnostic.Diagnostic{}, false
	}
	if len(filterGeneratedPlatformMethodsByReceiverMode(candidates, receiverMode)) != 0 {
		return diagnostic.Diagnostic{}, false
	}
	return checkSemaStaticAccess(typ, member, method, candidates[0], receiverMode, start, end, source)
}
func checkUnknownCallArgs(typ typesys.TypeSymbol, member typesys.MemberSymbol, args []semaArg, pos, bodyOffset int, source string, scopes semaScopeModel) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, arg := range args {
		name := strings.TrimSpace(arg.text)
		if strings.EqualFold(name, "this") || strings.EqualFold(name, "super") {
			continue
		}
		if inferSemaArgType(name, scopes.flat()) != "" {
			continue
		}
		if !simpleIdentifierPattern.MatchString(name) {
			continue
		}
		if _, ok := scopes.visibleAt(name, pos); ok {
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA013",
			Message:  fmt.Sprintf("%s %q references unknown variable %q", member.Kind, member.Name, name),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+arg.start, bodyOffset+arg.end),
		})
	}
	return diagnostics
}
