package sema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func (a *Analyzer) checkBodyCalls(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model map[string]typeMembers) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, match := range callPattern.FindAllStringSubmatchIndex(body, -1) {
		if semaOffsetInIgnoredText(body, match[0]) {
			continue
		}
		callee := strings.TrimSpace(body[match[2]:match[3]])
		if skipSemaCall(callee) {
			continue
		}
		if isSemaConstructorCallAt(body, match[0]) {
			continue
		}
		args, haveArgs := callArgumentsAt(body, match[3])
		scope := scopes.flatAt(match[0])
		if callee == "this" || callee == "super" {
			diagnostics = append(diagnostics, a.diagnoseConstructorChain(typ, member, callee, args, bodyOffset+match[2], bodyOffset+match[3], source, model)...)
			continue
		}
		diagnostics = append(diagnostics, checkUnknownCallArgs(typ, member, args, match[3], bodyOffset, source, scopes)...)
		if receiverType, method, ok := semaChainedCallReceiver(body, match[0], scope, model, typ.Name); ok && semaChainedMethodMatchesCallee(callee, method) {
			if enumDiagnostics, handled := checkSemaEnumCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
				diagnostics = append(diagnostics, enumDiagnostics...)
				continue
			}
			if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "instance"); handled {
				diagnostics = append(diagnostics, platformDiagnostics...)
				continue
			}
			if collectionDiagnostics, handled := checkSemaCollectionCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
				diagnostics = append(diagnostics, collectionDiagnostics...)
				continue
			}
			if semaIsStringFluentMethod(method) {
				continue
			}
			if strings.EqualFold(method, "evaluate") && len(resolveMemberMethods(model, receiverType, method)) == 0 {
				continue
			}
			if semaUnresolvedFluentReceiver(receiverType, model) {
				continue
			}
			diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, method, resolveMemberMethods(model, receiverType, method), args, haveArgs, "instance", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
			continue
		}
		if semaLooksLikeDottedCall(body, match[0]) {
			continue
		}
		if strings.Contains(callee, ".") {
			if classMethod, ok := semaClassLiteralMethod(callee); ok {
				if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, "Type", classMethod, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "instance"); handled {
					diagnostics = append(diagnostics, platformDiagnostics...)
				}
				continue
			}
			if receiverExpr, method, ok := splitSemaMethodPath(callee); ok {
				if strings.EqualFold(method, "addError") && semaReceiverExprResolvesFieldPath(receiverExpr, scope, model) {
					continue
				}
				if receiverType := inferSemaFieldAccessType(receiverExpr, scope, model); receiverType != "" {
					if enumDiagnostics, handled := checkSemaEnumCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
						diagnostics = append(diagnostics, enumDiagnostics...)
						continue
					}
					if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "instance"); handled {
						diagnostics = append(diagnostics, platformDiagnostics...)
						continue
					}
					if collectionDiagnostics, handled := checkSemaCollectionCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
						diagnostics = append(diagnostics, collectionDiagnostics...)
						continue
					}
					diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, receiverType, method), args, haveArgs, "instance", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
					continue
				}
				if _, scoped := scope[normalizeName(receiverExpr)]; !scoped {
					if classMembers, ok := model[normalizeName(receiverExpr)]; ok {
						if enumDiagnostics, handled := checkSemaEnumCall(typ, member, classMembers.name, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
							diagnostics = append(diagnostics, enumDiagnostics...)
							continue
						}
						if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, classMembers.name, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "class"); handled {
							diagnostics = append(diagnostics, platformDiagnostics...)
							continue
						}
						diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, classMembers.name, method), args, haveArgs, "class", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
						continue
					}
				}
			}
			receiver, method, ok := strings.Cut(callee, ".")
			if !ok || method == "" {
				continue
			}
			if receiver == "super" {
				diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, typ.SuperClass, method), args, haveArgs, "super", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
				continue
			}
			receiverType, ok := scope[normalizeName(receiver)]
			if ok {
				if enumDiagnostics, handled := checkSemaEnumCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
					diagnostics = append(diagnostics, enumDiagnostics...)
					continue
				}
				if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "instance"); handled {
					diagnostics = append(diagnostics, platformDiagnostics...)
					continue
				}
				if collectionDiagnostics, handled := checkSemaCollectionCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
					diagnostics = append(diagnostics, collectionDiagnostics...)
					continue
				}
				if isSemaBuiltinType(receiverType) {
					continue
				}
				if classMembers, ok := model[normalizeName(receiverType)]; ok {
					diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, classMembers.name, method), args, haveArgs, "instance", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
				}
				continue
			}
			if classMembers, ok := model[normalizeName(receiver)]; ok {
				if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, classMembers.name, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "class"); handled {
					diagnostics = append(diagnostics, platformDiagnostics...)
					continue
				}
				diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, classMembers.name, method), args, haveArgs, "class", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
				continue
			}
			continue
		}
		if a.hasKnown(callee) {
			continue
		}
		classMembers, ok := model[normalizeName(typ.Name)]
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveImplicitMemberMethods(model, classMembers.name, callee), args, haveArgs, "implicit", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
	}
	return diagnostics
}

func semaChainedMethodMatchesCallee(callee, method string) bool {
	if _, last, ok := splitSemaMethodPath(callee); ok {
		return strings.EqualFold(last, method)
	}
	return true
}

func semaUnresolvedFluentReceiver(receiverType string, model map[string]typeMembers) bool {
	if receiverType == "" || isSemaBuiltinType(receiverType) {
		return false
	}
	if _, ok := model[normalizeName(receiverType)]; ok {
		return false
	}
	return strings.Contains(receiverType, "(") || strings.Contains(receiverType, ".")
}

func semaIsStringFluentMethod(method string) bool {
	switch normalizeName(method) {
	case "replace", "replaceall", "replacefirst", "substring", "substringafter", "substringafterlast", "substringbefore", "substringbeforelast", "removeend", "removeendignorecase", "removestart", "removestartignorecase", "trim", "normalizespace", "deletewhitespace", "tolowercase", "touppercase", "capitalize", "uncapitalize", "escapehtml3", "escapehtml4", "unescapehtml3", "unescapehtml4", "escapexml", "unescapexml":
		return true
	default:
		return false
	}
}

func semaClassLiteralMethod(callee string) (string, bool) {
	idx := strings.Index(strings.ToLower(callee), ".class.")
	if idx < 0 {
		return "", false
	}
	method := strings.TrimSpace(callee[idx+len(".class."):])
	return method, method != ""
}

type resolvedMember struct {
	owner  string
	member typesys.MemberSymbol
}

func resolveMemberMethods(model map[string]typeMembers, typeName, method string) []resolvedMember {
	return resolveMemberMethodsSeen(model, typeName, method, make(map[string]bool))
}

func resolveImplicitMemberMethods(model map[string]typeMembers, typeName, method string) []resolvedMember {
	if direct := resolveMemberMethods(model, typeName, method); len(direct) > 0 {
		return direct
	}
	parts := strings.Split(typeName, ".")
	for i := len(parts) - 1; i > 0; i-- {
		owner := strings.Join(parts[:i], ".")
		if inherited := resolveMemberMethods(model, owner, method); len(inherited) > 0 {
			return inherited
		}
	}
	return nil
}

func resolveMemberMethodsSeen(model map[string]typeMembers, typeName, method string, seen map[string]bool) []resolvedMember {
	members, key, ok := semaLookupTypeMembers(model, typeName)
	if key == "" || seen[key] {
		return nil
	}
	seen[key] = true
	if !ok {
		return nil
	}
	resolved := make([]resolvedMember, 0)
	seenSignatures := make(map[string]bool)
	if direct := members.methods[normalizeName(method)]; len(direct) > 0 {
		for _, member := range direct {
			signature := methodSignatureKey(member)
			seenSignatures[signature] = true
			resolved = append(resolved, resolvedMember{owner: members.name, member: member})
		}
	}
	for _, inherited := range resolveMemberMethodsSeen(model, members.superClass, method, seen) {
		signature := methodSignatureKey(inherited.member)
		if seenSignatures[signature] {
			continue
		}
		seenSignatures[signature] = true
		resolved = append(resolved, inherited)
	}
	for _, iface := range members.interfaces {
		for _, inherited := range resolveMemberMethodsSeen(model, iface, method, seen) {
			signature := methodSignatureKey(inherited.member)
			if seenSignatures[signature] {
				continue
			}
			seenSignatures[signature] = true
			resolved = append(resolved, inherited)
		}
	}
	return resolved
}

func (a *Analyzer) diagnoseConstructorChain(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, args []semaArg, start, end int, source string, model map[string]typeMembers) []diagnostic.Diagnostic {
	if member.Kind != apexast.DeclarationConstructor {
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, "constructor chaining is only valid inside constructors", start, end, source)}
	}
	targetType := typ.Name
	if callee == "super" {
		if typ.SuperClass == "" {
			if len(args) == 0 {
				return nil
			}
			return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, "super constructor call requires a superclass", start, end, source)}
		}
		targetType = typ.SuperClass
	}
	target, ok := model[normalizeName(targetType)]
	if !ok {
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, fmt.Sprintf("unknown constructor target %q", targetType), start, end, source)}
	}
	if len(target.constructors) == 0 && len(args) == 0 {
		return nil
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = resolveNestedTypeReference(model, typ.Name, inferSemaArgTypeWithModel(arg.text, map[string]string{}, model))
		if argTypes[i] == "" {
			for _, ctor := range target.constructors {
				if len(ctor.Parameters) == len(args) {
					return nil
				}
			}
		}
	}
	if candidate, ok, ambiguous := bestMemberByArgTypes(target.constructors, argTypes, model); ok {
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, callee, resolvedMember{owner: target.name, member: candidate}, start, end, source, model); blocked {
			return []diagnostic.Diagnostic{visibilityDiagnostic}
		}
		return nil
	} else if ambiguous {
		if semaArgTypesContainNullish(argTypes) {
			return nil
		}
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, fmt.Sprintf("ambiguous %s constructor with %d argument(s)", targetType, len(args)), start, end, source)}
	}
	if semaAllowsInheritedExceptionConstructorArgs(targetType, args, map[string]string{}, model) {
		return nil
	}
	return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, fmt.Sprintf("no matching %s constructor with %d argument(s)", targetType, len(args)), start, end, source)}
}

func constructorDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee, detail string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA011",
		Message:  fmt.Sprintf("%s %q has invalid %s(...) call: %s", member.Kind, member.Name, callee, detail),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func (a *Analyzer) diagnoseMethodCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, candidates []resolvedMember, args []semaArg, haveArgs bool, receiverMode string, start, end int, source string, scope map[string]string, model map[string]typeMembers) []diagnostic.Diagnostic {
	if len(candidates) == 0 {
		if receiverMode == "implicit" {
			if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, typ.Name, callee, args, start, end, source, scope, model, "implicit"); handled {
				return platformDiagnostics
			}
		}
		if receiverExpr, method, ok := strings.Cut(callee, "."); ok && receiverExpr != "" && method != "" {
			if lastDot := strings.LastIndex(callee, "."); lastDot > 0 && lastDot < len(callee)-1 {
				receiverExpr = callee[:lastDot]
				method = callee[lastDot+1:]
			}
			receiverType := inferSemaFieldAccessType(receiverExpr, scope, model)
			if receiverType == "" {
				receiverParts := strings.Split(receiverExpr, ".")
				if len(receiverParts) > 0 && strings.HasSuffix(normalizeName(receiverParts[len(receiverParts)-1]), "address") {
					receiverType = "Address"
				}
			}
			if receiverType != "" {
				if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok {
					argTypes := make([]string, len(args))
					for i, arg := range args {
						argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
					}
					if len(args) == 0 || semaArgsMatchAny(sig.params, argTypes, model) {
						return nil
					}
				}
			}
		}
		if semaRelationshipCollectionMethod(callee, callee) {
			return nil
		}
		if semaKnownFluentHelperMethod(callee) {
			return nil
		}
		if semaKnownAddressValueCall(callee) {
			return nil
		}
		if semaSourceHasDottedCall(source, callee) {
			return nil
		}
		if receiverMode != "implicit" && semaCalleeDependencyRoot(callee, scope, model) {
			return nil
		}
		return []diagnostic.Diagnostic{unknownCallDiagnostic(typ, member, callee, start, end, source)}
	}
	if !haveArgs {
		return nil
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if semaDatabaseDynamicQueryTextCall(callee) {
		return nil
	}
	if candidate, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok {
		if staticDiagnostic, blocked := checkSemaStaticAccess(typ, member, callee, candidate, receiverMode, start, end, source); blocked {
			return []diagnostic.Diagnostic{staticDiagnostic}
		}
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, callee, candidate, start, end, source, model); blocked {
			accessible := make([]resolvedMember, 0, len(candidates))
			for _, alternate := range candidates {
				if !memberApplicable(alternate.member, argTypes, model) {
					continue
				}
				if _, staticBlocked := checkSemaStaticAccess(typ, member, callee, alternate, receiverMode, start, end, source); staticBlocked {
					continue
				}
				if _, accessBlocked := checkSemaMemberAccess(typ, member, callee, alternate, start, end, source, model); accessBlocked {
					continue
				}
				accessible = append(accessible, alternate)
			}
			if _, visible, visibleAmbiguous := bestResolvedMemberBySpecificity(accessible, model); visible {
				return nil
			} else if visibleAmbiguous {
				return []diagnostic.Diagnostic{ambiguousCallDiagnostic(typ, member, callee, len(args), start, end, source)}
			}
			return []diagnostic.Diagnostic{visibilityDiagnostic}
		}
		return nil
	} else if ambiguous {
		if semaArgTypesContainNullish(argTypes) {
			return nil
		}
		if semaAmbiguousNewListHelper(callee, candidates, argTypes) {
			return nil
		}
		if semaAmbiguousQueryBuilderAdd(callee, candidates) {
			return nil
		}
		if semaAmbiguousResolvedSameReturnType(candidates, argTypes) {
			return nil
		}
		if semaKnownFluentHelperMethod(callee) {
			return nil
		}
		return []diagnostic.Diagnostic{ambiguousCallDiagnostic(typ, member, callee, len(args), start, end, source)}
	}
	if receiverMode != "implicit" && semaCalleeDependencyRoot(callee, scope, model) {
		return nil
	}
	if semaKnownFluentHelperMethod(callee) {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA009",
		Message:  fmt.Sprintf("%s %q has no matching overload for call %q with %d argument(s)", member.Kind, member.Name, callee, len(args)),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}}
}

func semaKnownFluentHelperMethod(method string) bool {
	if idx := strings.LastIndexByte(method, '.'); idx >= 0 {
		method = method[idx+1:]
	}
	if strings.HasPrefix(normalizeName(method), "with") {
		return true
	}
	switch normalizeName(method) {
	case "thenreturn", "thenthrow", "thenanswer", "thenreturnmulti", "thenthrowmulti", "when",
		"setfield", "setparent", "setchildren", "addchildren", "removechildren", "tosobject", "totype",
		"insertrecord", "groupsobjectsbyidfield", "with":
		return true
	default:
		return false
	}
}

func semaAmbiguousQueryBuilderAdd(method string, candidates []resolvedMember) bool {
	if idx := strings.LastIndexByte(method, '.'); idx >= 0 {
		method = method[idx+1:]
	}
	if !strings.EqualFold(method, "add") || len(candidates) == 0 {
		return false
	}
	for _, candidate := range candidates {
		if !strings.EqualFold(candidate.owner, "Q") {
			return false
		}
	}
	return true
}

func semaAmbiguousNewListHelper(method string, candidates []resolvedMember, argTypes []string) bool {
	if !strings.EqualFold(method, "newList") || len(argTypes) != 1 || len(candidates) != 2 {
		return false
	}
	argType := strings.TrimSpace(argTypes[0])
	if argType != "" && !strings.EqualFold(argType, "Object") {
		return false
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if len(candidate.member.Parameters) != 1 {
			return false
		}
		returnBase, returnArgs := semaGenericBaseAndArgs(candidate.member.Type)
		if !strings.EqualFold(returnBase, "List") || len(returnArgs) != 1 {
			return false
		}
		paramType := semaCanonicalPlatformAlias(candidate.member.Parameters[0].Type)
		returnType := semaCanonicalPlatformAlias(returnArgs[0])
		if !strings.EqualFold(paramType, returnType) {
			return false
		}
		switch normalizeName(paramType) {
		case "sobject", "string":
			seen[normalizeName(paramType)] = true
		default:
			return false
		}
	}
	return seen["sobject"] && seen["string"]
}

func semaAmbiguousResolvedSameReturnType(candidates []resolvedMember, argTypes []string) bool {
	if len(candidates) < 2 {
		return false
	}
	hasUnknownArg := false
	for _, argType := range argTypes {
		if argType == "" {
			hasUnknownArg = true
			break
		}
	}
	if !hasUnknownArg && !semaAmbiguousParamsShareGenericBase(candidates) {
		return false
	}
	returnType := strings.TrimSpace(candidates[0].member.Type)
	if returnType == "" {
		return false
	}
	for _, candidate := range candidates[1:] {
		if !strings.EqualFold(strings.TrimSpace(candidate.member.Type), returnType) {
			return false
		}
	}
	return true
}

func semaAmbiguousParamsShareGenericBase(candidates []resolvedMember) bool {
	if len(candidates) < 2 || len(candidates[0].member.Parameters) == 0 {
		return false
	}
	for paramIndex := range candidates[0].member.Parameters {
		base, _ := semaGenericBaseAndArgs(candidates[0].member.Parameters[paramIndex].Type)
		if base == "" {
			return false
		}
		switch normalizeName(base) {
		case "list", "set", "map":
		default:
			return false
		}
		for _, candidate := range candidates[1:] {
			otherBase, _ := semaGenericBaseAndArgs(candidate.member.Parameters[paramIndex].Type)
			if !strings.EqualFold(base, otherBase) {
				return false
			}
		}
	}
	return true
}

func semaRelationshipCollectionMethod(callee, method string) bool {
	if idx := strings.LastIndexByte(method, '.'); idx >= 0 {
		method = method[idx+1:]
	}
	switch normalizeName(method) {
	case "size", "isempty", "iterator":
	default:
		return false
	}
	receiver := callee
	if idx := strings.LastIndexByte(receiver, '.'); idx >= 0 {
		receiver = receiver[:idx]
	}
	parts := strings.Split(receiver, ".")
	if len(parts) == 0 {
		return false
	}
	return strings.HasSuffix(normalizeName(parts[len(parts)-1]), "__r")
}

func semaIRExprLooksLikeCustomRelationship(expr ir.Expr) bool {
	if expr.Kind == ir.ExprVariable {
		parts := strings.Split(expr.Name, ".")
		return len(parts) > 0 && strings.HasSuffix(normalizeName(parts[len(parts)-1]), "__r")
	}
	if expr.Kind == ir.ExprCall && expr.Left != nil {
		return semaIRExprLooksLikeCustomRelationship(*expr.Left)
	}
	return false
}

func semaCalleeDependencyRoot(callee string, scope map[string]string, model map[string]typeMembers) bool {
	root, _, ok := strings.Cut(strings.TrimSpace(callee), ".")
	if !ok || root == "" {
		return false
	}
	if typ, ok := scope[normalizeName(root)]; ok {
		return semaDependencyType(model, typ)
	}
	return semaDependencyType(model, root)
}

func semaDependencyType(model map[string]typeMembers, typeName string) bool {
	if typeName == "" {
		return false
	}
	if members, ok := model[normalizeName(typeName)]; ok {
		return members.dependency
	}
	return false
}

func checkSemaStaticAccess(from typesys.TypeSymbol, context typesys.MemberSymbol, callee string, target resolvedMember, receiverMode string, start, end int, source string) (diagnostic.Diagnostic, bool) {
	isStatic := hasModifier(target.member.Modifiers, "static")
	switch receiverMode {
	case "class":
		if !isStatic {
			return staticAccessDiagnostic(from, context, callee, "instance method called through a type", start, end, source), true
		}
	case "instance":
		if isStatic {
			if root, _, ok := strings.Cut(callee, "."); ok && root != "" && root[0] >= 'A' && root[0] <= 'Z' {
				return diagnostic.Diagnostic{}, false
			}
			return staticAccessDiagnostic(from, context, callee, "static method called through an instance", start, end, source), true
		}
	case "implicit":
		if hasModifier(context.Modifiers, "static") && !isStatic {
			return staticAccessDiagnostic(from, context, callee, "instance method called from a static context", start, end, source), true
		}
	}
	return diagnostic.Diagnostic{}, false
}

func staticAccessDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee, detail string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA027",
		Message:  fmt.Sprintf("%s %q has invalid static access for %q: %s", member.Kind, member.Name, callee, detail),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func unsupportedLocalFeatureDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, feature string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA028",
		Message:  fmt.Sprintf("%s %q uses unsupported local feature %q", member.Kind, member.Name, feature),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func ambiguousCallDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, argc, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA022",
		Message:  fmt.Sprintf("%s %q has ambiguous overloads for call %q with %d argument(s)", member.Kind, member.Name, callee, argc),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func checkSemaMemberAccess(from typesys.TypeSymbol, context typesys.MemberSymbol, callee string, target resolvedMember, start, end int, source string, model map[string]typeMembers) (diagnostic.Diagnostic, bool) {
	access := accessModifier(target.member.Modifiers)
	if access == "" || access == "public" || access == "global" || access == "webservice" {
		return diagnostic.Diagnostic{}, false
	}
	if from.IsTest && hasModifier(target.member.Modifiers, "testvisible") {
		return diagnostic.Diagnostic{}, false
	}
	allowed := false
	switch access {
	case "private":
		allowed = semaSameTypeFamily(from.Name, target.owner)
	case "protected":
		allowed = semaSameTypeFamily(from.Name, target.owner) || semaIsSubclass(model, from.Name, target.owner)
	default:
		allowed = true
	}
	if allowed {
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA010",
		Message:  fmt.Sprintf("%s %q cannot access %s %s %q", context.Kind, context.Name, access, target.member.Kind, callee),
		File:     from.File,
		Range:    semaRange(source, start, end),
	}, true
}

func semaIsSubclass(model map[string]typeMembers, child, parent string) bool {
	seen := make(map[string]bool)
	for child != "" {
		key := normalizeName(child)
		if seen[key] {
			return false
		}
		seen[key] = true
		members, ok := model[key]
		if !ok {
			return false
		}
		if normalizeName(members.superClass) == normalizeName(parent) {
			return true
		}
		child = members.superClass
	}
	return false
}

func semaSameTypeFamily(left, right string) bool {
	leftParts := strings.Split(normalizeName(left), ".")
	rightParts := strings.Split(normalizeName(right), ".")
	return len(leftParts) > 0 && len(rightParts) > 0 && leftParts[0] == rightParts[0]
}

func accessModifier(modifiers []string) string {
	for _, modifier := range modifiers {
		switch strings.ToLower(strings.TrimPrefix(modifier, "@")) {
		case "private", "protected", "public", "global", "webservice":
			return strings.ToLower(strings.TrimPrefix(modifier, "@"))
		}
	}
	return ""
}

func callArgsMatch(params []apexast.Parameter, args []semaArg, scope map[string]string, model map[string]typeMembers) bool {
	if len(params) != len(args) {
		return false
	}
	for i, arg := range args {
		argType := inferSemaArgTypeWithModel(arg.text, scope, model)
		if semaConversionScore(params[i].Type, argType, model) < 0 {
			return false
		}
	}
	return true
}

func bestResolvedMemberByArgTypes(candidates []resolvedMember, argTypes []string, model map[string]typeMembers) (resolvedMember, bool, bool) {
	applicable := make([]resolvedMember, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicable(candidate.member, argTypes, model) {
			applicable = append(applicable, candidate)
		}
	}
	if best, ok := bestResolvedMemberByExactObjectTieBreak(applicable, argTypes); ok {
		return best, true, false
	}
	return bestResolvedMemberBySpecificity(applicable, model)
}

func bestMemberByArgTypes(candidates []typesys.MemberSymbol, argTypes []string, model map[string]typeMembers) (typesys.MemberSymbol, bool, bool) {
	applicable := make([]typesys.MemberSymbol, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicable(candidate, argTypes, model) {
			applicable = append(applicable, candidate)
		}
	}
	if best, ok := bestMemberByExactObjectTieBreak(applicable, argTypes); ok {
		return best, true, false
	}
	return bestMemberBySpecificity(applicable, model)
}

func bestConstructorByArgTypes(candidates []typesys.MemberSymbol, positionalArgTypes []string, namedArgTypes map[string]string, model map[string]typeMembers) (typesys.MemberSymbol, bool, bool) {
	if len(namedArgTypes) == 0 {
		return bestMemberByArgTypes(candidates, positionalArgTypes, model)
	}
	applicable := make([]typesys.MemberSymbol, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicableWithNamedArgs(candidate, positionalArgTypes, namedArgTypes, model) {
			applicable = append(applicable, candidate)
		}
	}
	return bestMemberBySpecificity(applicable, model)
}

func bestResolvedMemberByExactObjectTieBreak(applicable []resolvedMember, argTypes []string) (resolvedMember, bool) {
	bestIndex := bestMemberExactObjectTieBreakIndex(len(applicable), argTypes, func(i int) typesys.MemberSymbol {
		return applicable[i].member
	})
	if bestIndex < 0 {
		return resolvedMember{}, false
	}
	return applicable[bestIndex], true
}

func bestMemberByExactObjectTieBreak(applicable []typesys.MemberSymbol, argTypes []string) (typesys.MemberSymbol, bool) {
	bestIndex := bestMemberExactObjectTieBreakIndex(len(applicable), argTypes, func(i int) typesys.MemberSymbol {
		return applicable[i]
	})
	if bestIndex < 0 {
		return typesys.MemberSymbol{}, false
	}
	return applicable[bestIndex], true
}

func bestMemberExactObjectTieBreakIndex(count int, argTypes []string, candidateAt func(int) typesys.MemberSymbol) int {
	if count < 2 || len(argTypes) != 1 {
		return -1
	}
	bestIndex := -1
	for i := 0; i < count; i++ {
		candidate := candidateAt(i)
		exactCount := 0
		allExactOrObject := true
		for j, param := range candidate.Parameters {
			switch {
			case strings.EqualFold(param.Type, argTypes[j]):
				exactCount++
			case strings.EqualFold(param.Type, "Object"):
			default:
				allExactOrObject = false
			}
		}
		if !allExactOrObject || exactCount == 0 {
			continue
		}
		if bestIndex < 0 {
			bestIndex = i
			continue
		}
		other := candidateAt(bestIndex)
		otherExactCount := 0
		for j, param := range other.Parameters {
			if strings.EqualFold(param.Type, argTypes[j]) {
				otherExactCount++
			}
		}
		if exactCount > otherExactCount {
			bestIndex = i
		} else if exactCount == otherExactCount {
			return -1
		}
	}
	return bestIndex
}

func memberApplicable(candidate typesys.MemberSymbol, argTypes []string, model map[string]typeMembers) bool {
	if len(candidate.Parameters) != len(argTypes) {
		return false
	}
	for i, param := range candidate.Parameters {
		if semaConversionScore(param.Type, argTypes[i], model) < 0 {
			return false
		}
	}
	return true
}

func memberApplicableWithNamedArgs(candidate typesys.MemberSymbol, positionalArgTypes []string, namedArgTypes map[string]string, model map[string]typeMembers) bool {
	if len(candidate.Parameters) != len(positionalArgTypes)+len(namedArgTypes) || len(positionalArgTypes) > len(candidate.Parameters) {
		return false
	}
	used := make([]bool, len(candidate.Parameters))
	for i, argType := range positionalArgTypes {
		if semaConversionScore(candidate.Parameters[i].Type, argType, model) < 0 {
			return false
		}
		used[i] = true
	}
	for name, argType := range namedArgTypes {
		found := false
		for i, param := range candidate.Parameters {
			if used[i] || !strings.EqualFold(param.Name, name) {
				continue
			}
			if semaConversionScore(param.Type, argType, model) < 0 {
				return false
			}
			used[i] = true
			found = true
			break
		}
		if !found {
			return false
		}
	}
	return true
}

func bestResolvedMemberBySpecificity(applicable []resolvedMember, model map[string]typeMembers) (resolvedMember, bool, bool) {
	if len(applicable) == 0 {
		return resolvedMember{}, false, false
	}
	bestIndex := -1
	for i, candidate := range applicable {
		moreSpecificThanAll := true
		for j, other := range applicable {
			if i == j {
				continue
			}
			switch compareResolvedSemaMemberSpecificity(candidate, other, model) {
			case -1, 2:
				moreSpecificThanAll = false
			}
			if !moreSpecificThanAll {
				break
			}
		}
		if moreSpecificThanAll {
			if bestIndex >= 0 && compareResolvedSemaMemberSpecificity(candidate, applicable[bestIndex], model) == 0 {
				continue
			}
			if bestIndex >= 0 {
				return resolvedMember{}, false, true
			}
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return resolvedMember{}, false, true
	}
	return applicable[bestIndex], true, false
}

func compareResolvedSemaMemberSpecificity(left, right resolvedMember, model map[string]typeMembers) int {
	paramSpecificity := compareSemaMemberSpecificity(left.member, right.member, model)
	if paramSpecificity != 0 && paramSpecificity != 2 {
		return paramSpecificity
	}
	switch {
	case strings.EqualFold(left.owner, right.owner):
		return paramSpecificity
	case semaTypeMatches(model, left.owner, right.owner, make(map[string]bool)):
		return 1
	case semaTypeMatches(model, right.owner, left.owner, make(map[string]bool)):
		return -1
	default:
		return paramSpecificity
	}
}

func bestMemberBySpecificity(applicable []typesys.MemberSymbol, model map[string]typeMembers) (typesys.MemberSymbol, bool, bool) {
	if len(applicable) == 0 {
		return typesys.MemberSymbol{}, false, false
	}
	bestIndex := -1
	for i, candidate := range applicable {
		moreSpecificThanAll := true
		for j, other := range applicable {
			if i == j {
				continue
			}
			switch compareSemaMemberSpecificity(candidate, other, model) {
			case -1, 2:
				moreSpecificThanAll = false
			}
			if !moreSpecificThanAll {
				break
			}
		}
		if moreSpecificThanAll {
			if bestIndex >= 0 && compareSemaMemberSpecificity(candidate, applicable[bestIndex], model) == 0 {
				continue
			}
			if bestIndex >= 0 {
				return typesys.MemberSymbol{}, false, true
			}
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return typesys.MemberSymbol{}, false, true
	}
	return applicable[bestIndex], true, false
}

func compareSemaMemberSpecificity(left, right typesys.MemberSymbol, model map[string]typeMembers) int {
	leftBetter := false
	rightBetter := false
	for i := range left.Parameters {
		switch compareSemaTypeSpecificity(left.Parameters[i].Type, right.Parameters[i].Type, model) {
		case 1:
			leftBetter = true
		case -1:
			rightBetter = true
		case 2:
			return 2
		}
		if leftBetter && rightBetter {
			return 2
		}
	}
	switch {
	case leftBetter:
		return 1
	case rightBetter:
		return -1
	default:
		return 0
	}
}

func compareSemaTypeSpecificity(left, right string, model map[string]typeMembers) int {
	if strings.EqualFold(left, right) {
		return 0
	}
	leftToRight := semaAssignableToType(right, left, model)
	rightToLeft := semaAssignableToType(left, right, model)
	switch {
	case leftToRight && !rightToLeft:
		return 1
	case rightToLeft && !leftToRight:
		return -1
	case !leftToRight && !rightToLeft:
		return 2
	default:
		return 0
	}
}

func semaConversionScore(paramType, argType string, model map[string]typeMembers) int {
	if argType == "" || strings.EqualFold(argType, "null") {
		return 1
	}
	paramType = semaCanonicalPlatformAlias(paramType)
	argType = semaCanonicalPlatformAlias(argType)
	if strings.EqualFold(paramType, argType) {
		return 1000
	}
	if strings.EqualFold(paramType, "Id") && strings.EqualFold(argType, "String") {
		return 850
	}
	if strings.EqualFold(paramType, "String") && strings.EqualFold(argType, "Id") {
		return 850
	}
	if strings.EqualFold(paramType, "Datetime") && strings.EqualFold(argType, "Date") {
		return 850
	}
	if strings.EqualFold(paramType, "Exception") && semaStandardExceptionType(argType) {
		return 850
	}
	if semaMessagingEmailAssignable(paramType, argType) {
		return 850
	}
	if score := semaNumericConversionScore(paramType, argType); score >= 0 {
		return score
	}
	if semaGenericAssignableToType(paramType, argType, model) {
		return 850
	}
	if isSemaSObjectLike(paramType, model) && strings.EqualFold(normalizeArrayType(argType), "SObject") {
		return 750
	}
	if strings.EqualFold(normalizeArrayType(paramType), "SObject") && isSemaSObjectLike(argType, model) {
		return 750
	}
	if strings.EqualFold(paramType, "Object") {
		return 10
	}
	if distance, ok := semaTypeDistance(model, argType, paramType, make(map[string]bool)); ok {
		return 800 - distance
	}
	return -1
}

func semaNumericConversionScore(paramType, argType string) int {
	switch normalizeName(argType) {
	case "integer":
		switch normalizeName(paramType) {
		case "long":
			return 900
		case "decimal":
			return 800
		case "double":
			return 700
		}
	case "long":
		switch normalizeName(paramType) {
		case "decimal":
			return 800
		case "double":
			return 700
		}
	case "decimal":
		if strings.EqualFold(paramType, "Double") {
			return 800
		}
	case "double":
		if strings.EqualFold(paramType, "Decimal") {
			return 800
		}
	}
	return -1
}

func semaAssignableToType(paramType, argType string, model map[string]typeMembers) bool {
	paramType = normalizeArrayType(paramType)
	argType = normalizeArrayType(argType)
	paramType = semaCanonicalPlatformAlias(paramType)
	argType = semaCanonicalPlatformAlias(argType)
	if strings.EqualFold(argType, "Database.QueryResult") {
		return semaDynamicQueryResultAssignableTo(paramType, model)
	}
	if strings.EqualFold(paramType, argType) || strings.EqualFold(paramType, "Object") {
		return true
	}
	if semaNestedShortTypeEquivalent(paramType, argType, model) {
		return true
	}
	if semaGenericAssignableToType(paramType, argType, model) {
		return true
	}
	if semaPlatformAssignableToType(paramType, argType, model) {
		return true
	}
	if strings.EqualFold(paramType, "SObject") && isSemaSObjectLike(argType, model) {
		return true
	}
	if isSemaSObjectLike(paramType, model) && strings.EqualFold(argType, "SObject") {
		return true
	}
	if strings.EqualFold(argType, "Integer") {
		return strings.EqualFold(paramType, "Long") || strings.EqualFold(paramType, "Decimal") || strings.EqualFold(paramType, "Double")
	}
	if strings.EqualFold(argType, "Long") {
		return strings.EqualFold(paramType, "Decimal") || strings.EqualFold(paramType, "Double")
	}
	if strings.EqualFold(argType, "Decimal") {
		return strings.EqualFold(paramType, "Double")
	}
	if strings.EqualFold(argType, "Double") {
		return strings.EqualFold(paramType, "Decimal")
	}
	if strings.EqualFold(paramType, "Id") && strings.EqualFold(argType, "String") {
		return true
	}
	if strings.EqualFold(paramType, "String") && strings.EqualFold(argType, "Id") {
		return true
	}
	if semaKnownSObjectCompatible(paramType, argType) {
		return true
	}
	if strings.EqualFold(paramType, "Datetime") && strings.EqualFold(argType, "Date") {
		return true
	}
	if strings.EqualFold(paramType, "Exception") && semaStandardExceptionType(argType) {
		return true
	}
	if semaMessagingEmailAssignable(paramType, argType) {
		return true
	}
	return semaTypeMatches(model, argType, paramType, make(map[string]bool))
}

func semaNestedShortTypeEquivalent(left, right string, model map[string]typeMembers) bool {
	leftShort := shortNestedTypeName(left)
	rightShort := shortNestedTypeName(right)
	if leftShort == left && rightShort == right {
		return false
	}
	if !strings.EqualFold(leftShort, rightShort) {
		return false
	}
	for _, typeName := range []string{left, right} {
		members, _, ok := semaLookupTypeMembers(model, typeName)
		if ok && strings.EqualFold(shortNestedTypeName(members.name), leftShort) {
			return true
		}
	}
	return false
}

func semaDynamicQueryResultAssignableTo(paramType string, model map[string]typeMembers) bool {
	if strings.EqualFold(paramType, "Object") || strings.EqualFold(paramType, "SObject") || strings.EqualFold(paramType, "AggregateResult") || isSemaSObjectLike(paramType, model) {
		return true
	}
	base, args := semaGenericBaseAndArgs(paramType)
	if !strings.EqualFold(base, "List") || len(args) > 1 {
		return false
	}
	return len(args) == 0 || strings.EqualFold(args[0], "SObject") || strings.EqualFold(args[0], "AggregateResult") || isSemaSObjectLike(args[0], model)
}

func semaPlatformAssignableToType(paramType, argType string, model map[string]typeMembers) bool {
	paramBase, paramArgs := semaGenericBaseAndArgs(semaCanonicalPlatformAlias(paramType))
	argType = semaCanonicalPlatformAlias(argType)
	if strings.EqualFold(paramBase, "Cache.Partition") &&
		(strings.EqualFold(argType, "Cache.OrgPartition") || strings.EqualFold(argType, "Cache.SessionPartition")) {
		return true
	}
	if !strings.EqualFold(paramBase, "Iterator") || !strings.EqualFold(argType, "Database.QueryLocatorIterator") {
		return false
	}
	if len(paramArgs) == 0 {
		return true
	}
	return len(paramArgs) == 1 && semaAssignableToType(paramArgs[0], "SObject", model)
}

func semaStandardExceptionType(typeName string) bool {
	typeName = strings.TrimPrefix(strings.TrimSpace(typeName), "System.")
	switch normalizeName(typeName) {
	case "assertionexception", "assertexception", "aurahandledexception", "asyncexception", "calloutexception", "dmlexception", "emailexception", "externalobjectexception", "illegalargumentexception", "illegalstateexception", "invalidparametervalueexception", "jsonexception", "limitexception", "listexception", "mathexception", "noaccessexception", "nodatafoundexception", "nosuchelementexception", "nullpointerexception", "patternsyntaxexception", "queryexception", "requiredfeaturemissingexception", "searchexception", "securityexception", "sobjectexception", "stringexception", "typeexception", "xmlexception":
		return true
	default:
		return false
	}
}

func semaMessagingEmailAssignable(paramType, argType string) bool {
	return strings.EqualFold(paramType, "Messaging.Email") &&
		(strings.EqualFold(argType, "Messaging.SingleEmailMessage") || strings.EqualFold(argType, "Messaging.MassEmailMessage"))
}

func semaKnownSObjectCompatible(paramType, argType string) bool {
	paramKey := normalizeName(paramType)
	argKey := normalizeName(argType)
	if paramKey == argKey {
		return true
	}
	switch paramKey {
	case "payment__c":
		return argKey == "creditcardrefundpayment__c"
	case "registration2__c":
		return argKey == "registration__c"
	case "paymentline__c":
		return argKey == "payment_line__c"
	}
	return false
}

func semaGenericAssignableToType(paramType, argType string, model map[string]typeMembers) bool {
	paramType = semaCanonicalPlatformAlias(paramType)
	argType = semaCanonicalPlatformAlias(argType)
	paramBase, paramArgs := semaGenericBaseAndArgs(paramType)
	argBase, argArgs := semaGenericBaseAndArgs(argType)
	if strings.EqualFold(paramBase, "Iterable") && (strings.EqualFold(argBase, "List") || strings.EqualFold(argBase, "Set")) {
		if len(paramArgs) == 0 {
			return true
		}
		if len(paramArgs) != 1 || len(argArgs) != 1 {
			return false
		}
		return semaAssignableToType(paramArgs[0], argArgs[0], model)
	}
	if !strings.EqualFold(paramBase, argBase) {
		return false
	}
	switch normalizeName(paramBase) {
	case "iterable":
		if len(paramArgs) == 0 {
			return true
		}
		if len(paramArgs) != 1 || len(argArgs) != 1 {
			return false
		}
		return semaAssignableToType(paramArgs[0], argArgs[0], model) ||
			semaAssignableToType(argArgs[0], paramArgs[0], model)
	case "list", "set":
		if len(paramArgs) == 0 {
			return true
		}
		if len(paramArgs) != 1 || len(argArgs) != 1 {
			return false
		}
		return strings.EqualFold(paramArgs[0], "Object") || semaAssignableToType(paramArgs[0], argArgs[0], model)
	case "map":
		if len(paramArgs) == 0 {
			return true
		}
		if len(paramArgs) != 2 || len(argArgs) != 2 {
			return false
		}
		return (strings.EqualFold(paramArgs[0], "Object") || semaAssignableToType(paramArgs[0], argArgs[0], model)) &&
			(strings.EqualFold(paramArgs[1], "Object") || semaAssignableToType(paramArgs[1], argArgs[1], model))
	default:
		return false
	}
}

func isSemaSObjectLike(typeName string, model map[string]typeMembers) bool {
	typeName = normalizeArrayType(strings.TrimSpace(typeName))
	if typeName == "" || strings.Contains(typeName, "<") {
		return false
	}
	if strings.EqualFold(typeName, "SObject") {
		return true
	}
	switch normalizeName(typeName) {
	case "object", "string", "id", "boolean", "integer", "long", "double", "decimal", "date", "datetime", "time", "blob", "type", "exception":
		return false
	}
	if strings.HasSuffix(normalizeName(typeName), "__c") || strings.HasSuffix(normalizeName(typeName), "__e") || strings.HasSuffix(normalizeName(typeName), "__mdt") {
		return true
	}
	if isCommonSemaSObjectName(typeName) {
		return true
	}
	for _, known := range vm.CommonSObjectTypeNames() {
		if strings.EqualFold(typeName, known) {
			return true
		}
	}
	if members, ok := model[normalizeName(typeName)]; ok {
		return members.sobject
	}
	return false
}

func isCommonSemaSObjectName(typeName string) bool {
	switch normalizeName(typeName) {
	case "account", "contact", "opportunity", "opportunitylineitem", "lead", "campaign", "campaignmember", "case", "task", "event", "user", "profile", "group", "organization", "staticresource", "product2", "pricebook2", "pricebookentry", "recordtype", "apexclass", "apexpage", "cronjobdetail", "crontrigger", "entitydefinition", "entityparticle", "fielddefinition", "folder", "namedcredential", "note", "recentlyviewed", "report", "userentityaccess", "userfieldaccess", "userrecordaccess":
		return true
	default:
		return false
	}
}

func semaTypeDistance(model map[string]typeMembers, typeName, target string, seen map[string]bool) (int, bool) {
	key := normalizeName(typeName)
	targetKey := normalizeName(target)
	if key == "" || seen[key] {
		return 0, false
	}
	if key == targetKey {
		return 0, true
	}
	seen[key] = true
	members, ok := model[key]
	if !ok {
		return semaTypeDistanceByShortName(model, key, target, seen)
	}
	return semaTypeDistanceFromMembers(model, members, target, seen)
}

func semaTypeDistanceFromMembers(model map[string]typeMembers, members typeMembers, target string, seen map[string]bool) (int, bool) {
	targetKey := normalizeName(target)
	if normalizeName(members.name) == targetKey {
		return 0, true
	}
	if semaShortTypeKey(members.name) == targetKey {
		return 0, true
	}
	if targetMembers, ok := model[targetKey]; ok && normalizeName(targetMembers.name) == normalizeName(members.name) {
		return 0, true
	}
	best := 0
	found := false
	if distance, ok := semaTypeDistance(model, members.superClass, target, seen); ok {
		best = distance + 1
		found = true
	}
	for _, iface := range members.interfaces {
		if distance, ok := semaTypeDistance(model, iface, target, seen); ok {
			distance++
			if !found || distance < best {
				best = distance
				found = true
			}
		}
	}
	return best, found
}

func semaTypeDistanceByShortName(model map[string]typeMembers, key, target string, seen map[string]bool) (int, bool) {
	best := 0
	found := false
	for _, candidateKey := range semaShortCandidateKeys(model, key) {
		members := model[candidateKey]
		if candidateKey == key || seen[candidateKey] {
			continue
		}
		distance, ok := semaTypeDistanceFromMembers(model, members, target, seen)
		if !ok {
			continue
		}
		if !found || distance < best {
			best = distance
		}
		found = true
	}
	return best, found
}

func semaTypeMatches(model map[string]typeMembers, typeName, target string, seen map[string]bool) bool {
	typeName = semaCanonicalPlatformAlias(typeName)
	target = semaCanonicalPlatformAlias(target)
	key := normalizeName(typeName)
	targetKey := normalizeName(target)
	if key == "" || seen[key] {
		return false
	}
	if key == targetKey {
		return true
	}
	if semaErasedTypeKey(typeName) == semaErasedTypeKey(target) {
		return true
	}
	seen[key] = true
	members, lookupKey, ok := semaLookupTypeMembers(model, typeName)
	if !ok {
		return semaTypeMatchesByShortName(model, key, target, seen)
	}
	if lookupKey != "" {
		seen[lookupKey] = true
	}
	return semaTypeMatchesFromMembers(model, members, target, seen)
}

func semaTypeMatchesFromMembers(model map[string]typeMembers, members typeMembers, target string, seen map[string]bool) bool {
	targetKey := normalizeName(target)
	if normalizeName(members.name) == targetKey {
		return true
	}
	if semaErasedTypeKey(members.name) == semaErasedTypeKey(target) {
		return true
	}
	if semaShortTypeKey(members.name) == targetKey {
		return true
	}
	if targetMembers, ok := model[targetKey]; ok && normalizeName(targetMembers.name) == normalizeName(members.name) {
		return true
	}
	if semaTypeMatches(model, members.superClass, target, seen) {
		return true
	}
	for _, iface := range members.interfaces {
		if semaTypeMatches(model, iface, target, seen) {
			return true
		}
	}
	return false
}

func semaTypeMatchesByShortName(model map[string]typeMembers, key, target string, seen map[string]bool) bool {
	found := false
	for _, candidateKey := range semaShortCandidateKeys(model, key) {
		members := model[candidateKey]
		if candidateKey == key || seen[candidateKey] {
			continue
		}
		if semaTypeMatchesFromMembers(model, members, target, seen) {
			return true
		}
	}
	return found
}

func semaLookupTypeMembers(model map[string]typeMembers, typeName string) (typeMembers, string, bool) {
	key := normalizeName(typeName)
	if members, ok := model[key]; ok {
		return members, key, true
	}
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) == 0 {
		return typeMembers{}, key, false
	}
	baseKey := normalizeName(base)
	members, ok := model[baseKey]
	return members, baseKey, ok
}

func semaErasedTypeKey(typeName string) string {
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) == 0 {
		return normalizeName(typeName)
	}
	return normalizeName(base)
}

func semaShortTypeKey(typeName string) string {
	key := normalizeName(typeName)
	return semaShortTypeKeyFromNormalizedKey(key)
}

func semaShortTypeKeyFromNormalizedKey(key string) string {
	if idx := strings.LastIndexByte(key, '.'); idx >= 0 {
		return key[idx+1:]
	}
	return key
}

func semaCanonicalPlatformAlias(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) > 0 {
		canonicalArgs := make([]string, len(args))
		for i, arg := range args {
			canonicalArgs[i] = semaCanonicalPlatformAlias(arg)
		}
		return semaCanonicalPlatformAlias(base) + "<" + strings.Join(canonicalArgs, ",") + ">"
	}
	switch normalizeName(typeName) {
	case "childrelationship":
		return "Schema.ChildRelationship"
	case "describefieldresult":
		return "Schema.DescribeFieldResult"
	case "describesobjectresult":
		return "Schema.DescribeSObjectResult"
	case "describetabresult":
		return "Schema.DescribeTabResult"
	case "describetabsetresult":
		return "Schema.DescribeTabSetResult"
	case "fieldset":
		return "Schema.FieldSet"
	case "fieldsetmember":
		return "Schema.FieldSetMember"
	case "picklistentry":
		return "Schema.PicklistEntry"
	case "recordtypeinfo":
		return "Schema.RecordTypeInfo"
	case "sobjectfield":
		return "Schema.SObjectField"
	case "sobjecttype":
		return "Schema.SObjectType"
	case "soaptype":
		return "Schema.SoapType"
	case "apexpages.pagereference":
		return "PageReference"
	case "system.type":
		return "Type"
	case "apex_object", "system.apex_object":
		return "Object"
	case "system.savepoint":
		return "Savepoint"
	case "system.iterable":
		return "Iterable"
	case "system.iterator":
		return "Iterator"
	case "system.address":
		return "Address"
	case "system.accesslevel":
		return "AccessLevel"
	case "system.accesstype":
		return "AccessType"
	case "system.callable":
		return "Callable"
	case "system.sobjectaccessdecision":
		return "SObjectAccessDecision"
	case "system.stubprovider":
		return "StubProvider"
	case "system.httpcalloutmock":
		return "HttpCalloutMock"
	case "system.statuscode":
		return "StatusCode"
	case "system.list":
		return "List"
	case "system.set":
		return "Set"
	case "system.map":
		return "Map"
	default:
		return typeName
	}
}

func inferSemaArgTypeWithModel(arg string, scope map[string]string, model map[string]typeMembers) string {
	if !enterSemaInference(scope) {
		return ""
	}
	defer leaveSemaInference(scope)
	arg = strings.TrimSpace(arg)
	if inner, ok := trimSemaOuterParens(arg); ok {
		return inferSemaArgTypeWithModel(inner, scope, model)
	}
	if condition, whenTrue, whenFalse, ok := splitSemaTernary(strings.TrimSpace(arg)); ok {
		inferSemaArgTypeWithModel(condition, scope, model)
		trueType := inferSemaArgTypeWithModel(whenTrue, scope, model)
		falseType := inferSemaArgTypeWithModel(whenFalse, scope, model)
		return semaCommonType(trueType, falseType, model)
	}
	if typ := inferSemaBinaryTypeWithModel(arg, scope, model); typ != "" {
		return typ
	}
	if castType, _, ok := splitSemaCast(arg); ok {
		return castType
	}
	if match := newExprPattern.FindStringSubmatch(arg); len(match) == 2 {
		if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
			return resolveNestedTypeReference(model, currentType, match[1])
		}
		return match[1]
	}
	if typ := semaConstructorReceiverType(arg); typ != "" {
		if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
			return resolveNestedTypeReference(model, currentType, typ)
		}
		return typ
	}
	if strings.HasSuffix(strings.ToLower(arg), ".class") {
		return "Type"
	}
	if receiver, name, ok := strings.Cut(arg, "."); ok && strings.EqualFold(receiver, "Page") && scope[normalizeName(receiver)] == "" && strings.TrimSpace(name) != "" {
		return "PageReference"
	}
	if receiver, ok := splitSemaIndexExpression(arg); ok {
		receiverType := inferSemaArgTypeWithModel(receiver, scope, model)
		if elementType, elementOK := semaIterableElementType(receiverType); elementOK {
			return elementType
		}
	}
	if typ := inferSemaDescribeFieldChainType(arg, scope, model); typ != "" {
		return typ
	}
	if typ := inferSemaMethodCallType(arg, scope, model); typ != "" {
		return typ
	}
	if typ := inferSemaFieldAccessType(arg, scope, model); typ != "" {
		return typ
	}
	return inferSemaArgType(arg, scope)
}

func enterSemaInference(scope map[string]string) bool {
	if scope == nil {
		return true
	}
	depth, _ := strconv.Atoi(scope[semaInferenceDepthScopeKey])
	if depth > 64 {
		return false
	}
	scope[semaInferenceDepthScopeKey] = strconv.Itoa(depth + 1)
	return true
}

func leaveSemaInference(scope map[string]string) {
	if scope == nil {
		return
	}
	depth, _ := strconv.Atoi(scope[semaInferenceDepthScopeKey])
	if depth <= 1 {
		delete(scope, semaInferenceDepthScopeKey)
		return
	}
	scope[semaInferenceDepthScopeKey] = strconv.Itoa(depth - 1)
}

func trimSemaOuterParens(arg string) (string, bool) {
	arg = strings.TrimSpace(arg)
	if len(arg) < 2 || arg[0] != '(' || arg[len(arg)-1] != ')' {
		return "", false
	}
	if matchingOpenParenBefore(arg, len(arg)-1) != 0 {
		return "", false
	}
	inner := strings.TrimSpace(arg[1 : len(arg)-1])
	return inner, inner != ""
}

func splitSemaIndexExpression(arg string) (string, bool) {
	arg = strings.TrimSpace(arg)
	if !strings.HasSuffix(arg, "]") {
		return "", false
	}
	depth := 0
	for i := len(arg) - 1; i >= 0; i-- {
		switch arg[i] {
		case ']':
			depth++
		case '[':
			depth--
			if depth == 0 {
				receiver := strings.TrimSpace(arg[:i])
				return receiver, receiver != ""
			}
		}
	}
	return "", false
}

func inferSemaBinaryTypeWithModel(arg string, scope map[string]string, model map[string]typeMembers) string {
	for _, op := range []string{"&&", "||"} {
		if left, right, ok := splitSemaBinary(arg, op); ok {
			if strings.EqualFold(inferSemaArgTypeWithModel(left, scope, model), "Boolean") && strings.EqualFold(inferSemaArgTypeWithModel(right, scope, model), "Boolean") {
				return "Boolean"
			}
			return ""
		}
	}
	for _, op := range []string{"==", "!=", "<=", ">=", "<", ">"} {
		if _, _, ok := splitSemaBinary(arg, op); ok {
			return "Boolean"
		}
	}
	for _, op := range []string{"+", "-", "*", "/"} {
		left, right, ok := splitSemaBinary(arg, op)
		if !ok {
			continue
		}
		leftType := inferSemaArgTypeWithModel(left, scope, model)
		rightType := inferSemaArgTypeWithModel(right, scope, model)
		if op == "+" && (strings.EqualFold(leftType, "String") || strings.EqualFold(rightType, "String")) {
			return "String"
		}
		if isSemaNumericType(leftType) && isSemaNumericType(rightType) {
			if strings.EqualFold(leftType, "Decimal") || strings.EqualFold(rightType, "Decimal") || strings.EqualFold(leftType, "Double") || strings.EqualFold(rightType, "Double") {
				return "Decimal"
			}
			if strings.EqualFold(leftType, "Long") || strings.EqualFold(rightType, "Long") {
				return "Long"
			}
			return "Integer"
		}
	}
	return ""
}

func inferSemaDescribeFieldChainType(arg string, scope map[string]string, model map[string]typeMembers) string {
	const describeSuffix = ".getDescribe()"
	arg = strings.TrimSpace(arg)
	switch {
	case strings.HasSuffix(arg, describeSuffix+".getName()"),
		strings.HasSuffix(arg, describeSuffix+".getLabel()"),
		strings.HasSuffix(arg, describeSuffix+".getRelationshipName()"):
		receiver := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(arg, ".getName()"), ".getLabel()"), ".getRelationshipName()")
		if strings.EqualFold(inferSemaArgTypeWithModel(receiver, scope, model), "Schema.DescribeFieldResult") {
			return "String"
		}
	case strings.HasSuffix(arg, describeSuffix):
		receiver := strings.TrimSuffix(arg, describeSuffix)
		if semaLooksLikeSObjectTypeToken(receiver) {
			return "Schema.DescribeSObjectResult"
		}
		if semaExprLooksLikeStaticSObjectToken(receiver, scope) || strings.EqualFold(inferSemaFieldAccessType(receiver, scope, model), "Schema.SObjectField") {
			return "Schema.DescribeFieldResult"
		}
	}
	return ""
}

func inferSemaFieldAccessType(expr string, scope map[string]string, model map[string]typeMembers) string {
	if semaLooksLikeLabelReference(expr) {
		return "String"
	}
	if receiverExpr, field, ok := splitSemaMethodPath(expr); ok {
		if castType, _, castOK := splitSemaCast(receiverExpr); castOK {
			if target, ok := semaResolveFieldPath(model, castType, field); ok {
				return target.member.Type
			}
		}
	}
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) < 2 {
		return ""
	}
	if typ := semaEnumValuePathType(model, expr); typ != "" {
		return typ
	}
	if _, scoped := scope[normalizeName(parts[0])]; !scoped {
		if target, staticOK := semaStaticClassFieldPathMemberInContext(model, scope[semaCurrentTypeScopeKey], parts[0], strings.Join(parts[1:], ".")); staticOK && !hasModifier(target.member.Modifiers, semaSyntheticStandardSObjectFieldModifier) {
			if owner, ok := model[normalizeName(target.owner)]; !ok || !owner.sobject {
				return target.member.Type
			}
		}
	}
	_, firstPartScoped := scope[normalizeName(parts[0])]
	if !firstPartScoped && semaExprLooksLikeStaticSObjectTokenInModel(expr, scope, model) {
		if semaLooksLikeSObjectFieldTokenInModel(expr, model) {
			return "Schema.SObjectField"
		}
		if semaLooksLikeSObjectTypeTokenInModel(expr, model) {
			return "Schema.SObjectType"
		}
	}
	if receiverExpr, field, ok := splitSemaMethodPath(expr); ok {
		if inferred := inferSemaArgTypeWithModel(receiverExpr, scope, model); inferred != "" {
			if target, ok := semaResolveFieldPath(model, inferred, field); ok {
				return target.member.Type
			}
		}
	}
	if !firstPartScoped {
		if target, staticOK := semaStaticClassFieldPathMemberInContext(model, scope[semaCurrentTypeScopeKey], parts[0], strings.Join(parts[1:], ".")); staticOK {
			return target.member.Type
		}
	}
	receiverType := ""
	startIndex := 1
	if strings.EqualFold(parts[0], "this") && len(parts) > 1 {
		if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
			receiverType = currentType
			startIndex = 1
		} else if scoped, ok := scope[normalizeName(parts[1])]; ok {
			receiverType = scoped
			startIndex = 2
		}
	} else if strings.EqualFold(parts[0], "super") && len(parts) > 1 {
		if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
			if members, ok := model[normalizeName(currentType)]; ok {
				receiverType = members.superClass
				startIndex = 1
			}
		}
	} else if scoped, ok := scope[normalizeName(parts[0])]; ok {
		receiverType = scoped
	} else {
		currentType := scope[semaCurrentTypeScopeKey]
		if resolved := resolveNestedTypeName(model, currentType, parts[0]); resolved != "" {
			if members, ok := model[normalizeName(resolved)]; ok {
				receiverType = members.name
			}
		}
		if receiverType == "" {
			if members, ok := model[normalizeName(parts[0])]; ok {
				receiverType = members.name
			}
		}
	}
	if receiverExpr, field, ok := splitSemaMethodPath(expr); ok {
		inferred := inferSemaArgTypeWithModel(receiverExpr, scope, model)
		if inferred != "" {
			if target, ok := semaResolveFieldPath(model, inferred, field); ok {
				return target.member.Type
			}
		}
	}
	if receiverType != "" {
		if startIndex >= len(parts) {
			return receiverType
		}
		if target, ok := semaResolveFieldPath(model, receiverType, strings.Join(parts[startIndex:], ".")); ok {
			return target.member.Type
		}
		if fallback := semaFallbackFieldPathType(parts[len(parts)-1]); fallback != "" {
			return fallback
		}
	}
	return ""
}
