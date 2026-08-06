package sema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func (a *Analyzer) checkBodyCalls(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, bodyOffset int, source string, scopes semaScopeModel, model *semaTypeMemberView) []diagnostic.Diagnostic {
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
		if scope[semaCurrentTypeScopeKey] == "" {
			scope[semaCurrentTypeScopeKey] = typ.Name
		}
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
			candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, receiverType, method), "instance")
			if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, semaArgTypes(args, scope, model), model); ok && !semaResolvedMembersAllPlatformBacked(model, candidates) {
				if staticDiagnostic, blocked := checkSemaStaticAccessWithModel(typ, member, method, candidate, "instance", bodyOffset+match[2], bodyOffset+match[3], source, model); blocked {
					diagnostics = append(diagnostics, staticDiagnostic)
				}
				if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, method, candidate, bodyOffset+match[2], bodyOffset+match[3], source, model); blocked {
					diagnostics = append(diagnostics, visibilityDiagnostic)
				}
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
			if semaLooksLikeDottedCall(body, match[0]) && semaImmediateDottedCallResolved(body, match[0], args, scope, model) {
				continue
			}
			diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, method, candidates, args, haveArgs, "instance", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
			continue
		}
		if haveArgs {
			argTypes := make([]string, len(args))
			for i, arg := range args {
				argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
			}
			if diag, blocked := diagnoseDatabaseExecuteBatchArg(typ, member, callee, argTypes, bodyOffset+match[2], bodyOffset+match[3], source, model); blocked {
				diagnostics = append(diagnostics, diag)
				continue
			}
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
				if scope[normalizeName(receiverExpr)] == "" && semaKnownPlatformTypeReceiver(receiverExpr) && !semaProjectTypeShadowsPlatform(model, receiverExpr) {
					if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverExpr, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "class"); handled {
						diagnostics = append(diagnostics, platformDiagnostics...)
						continue
					}
				}
				if lookupName, ok := semaStaticContextTypeReceiver(
					model,
					typ,
					member,
					receiverExpr,
					method,
					scopes.localVisibleAt(receiverExpr, match[0]),
				); ok {
					candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, lookupName, method), "class")
					diagnostics = append(diagnostics, a.diagnoseMethodCall(
						typ,
						member,
						callee,
						candidates,
						args,
						haveArgs,
						"class",
						bodyOffset+match[2],
						bodyOffset+match[3],
						source,
						scope,
						model,
					)...)
					continue
				}
				if receiverType := inferSemaFieldAccessType(receiverExpr, scope, model); receiverType != "" {
					receiverMode := "instance"
					if semaTextReceiverExprLooksLikeType(receiverExpr, scope, model) {
						receiverMode = "class"
					}
					if enumDiagnostics, handled := checkSemaEnumCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
						diagnostics = append(diagnostics, enumDiagnostics...)
						continue
					}
					if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, receiverMode); handled {
						diagnostics = append(diagnostics, platformDiagnostics...)
						continue
					}
					if collectionDiagnostics, handled := checkSemaCollectionCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
						diagnostics = append(diagnostics, collectionDiagnostics...)
						continue
					}
					diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, receiverType, method), args, haveArgs, receiverMode, bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
					continue
				}
				if _, scoped := scope[normalizeName(receiverExpr)]; !scoped {
					if classMembers, lookupName, ok := semaClassMembersForReceiver(model, typ, receiverExpr); ok {
						if enumDiagnostics, handled := checkSemaEnumCall(typ, member, classMembers.name, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
							diagnostics = append(diagnostics, enumDiagnostics...)
							continue
						}
						candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, lookupName, method), "class")
						if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, semaArgTypes(args, scope, model), model); ok && !semaResolvedMembersAllPlatformBacked(model, candidates) {
							if staticDiagnostic, blocked := checkSemaStaticAccessWithModel(typ, member, callee, candidate, "class", bodyOffset+match[2], bodyOffset+match[3], source, model); blocked {
								diagnostics = append(diagnostics, staticDiagnostic)
							}
							if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, callee, candidate, bodyOffset+match[2], bodyOffset+match[3], source, model); blocked {
								diagnostics = append(diagnostics, visibilityDiagnostic)
							}
							continue
						}
						if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, lookupName, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "class"); handled {
							diagnostics = append(diagnostics, platformDiagnostics...)
							continue
						}
						diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, candidates, args, haveArgs, "class", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
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
				if classMembers, ok := model.lookup(normalizeName(receiverType)); ok {
					diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveMemberMethods(model, classMembers.name, method), args, haveArgs, "instance", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
				}
				continue
			}
			if classMembers, lookupName, ok := semaClassMembersForReceiver(model, typ, receiver); ok {
				candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, lookupName, method), "class")
				if candidate, ok, _ := bestResolvedMemberByArgTypes(candidates, semaArgTypes(args, scope, model), model); ok && !semaResolvedMembersAllPlatformBacked(model, candidates) {
					if staticDiagnostic, blocked := checkSemaStaticAccessWithModel(typ, member, callee, candidate, "class", bodyOffset+match[2], bodyOffset+match[3], source, model); blocked {
						diagnostics = append(diagnostics, staticDiagnostic)
					}
					if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, callee, candidate, bodyOffset+match[2], bodyOffset+match[3], source, model); blocked {
						diagnostics = append(diagnostics, visibilityDiagnostic)
					}
					continue
				}
				if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, classMembers.name, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "class"); handled {
					diagnostics = append(diagnostics, platformDiagnostics...)
					continue
				}
				diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, candidates, args, haveArgs, "class", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
				continue
			}
			continue
		}
		if a.hasKnown(callee) {
			continue
		}
		_, lookupName, ok := semaCurrentClassMembers(model, typ)
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, callee, resolveImplicitMemberMethods(model, lookupName, callee), args, haveArgs, "implicit", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
	}
	return diagnostics
}

func semaClassMembersForReceiver(model *semaTypeMemberView, current typesys.TypeSymbol, receiver string) (typeMembers, string, bool) {
	if members, ok := model.lookup(normalizeName(receiver)); ok {
		if !semaPlatformReceiverSpellingMatches(receiver, members) {
			return typeMembers{}, "", false
		}
		return members, receiver, true
	}
	if current.Dependency && current.Namespace != "" && !strings.Contains(receiver, ".") {
		qualified := current.Namespace + "." + receiver
		if members, ok := model.lookup(normalizeName(qualified)); ok {
			return members, qualified, true
		}
	}
	return typeMembers{}, "", false
}

func semaStaticContextTypeReceiver(
	model *semaTypeMemberView,
	current typesys.TypeSymbol,
	context typesys.MemberSymbol,
	receiver,
	method string,
	hasNonFieldBinding bool,
) (string, bool) {
	if !hasModifier(context.Modifiers, "static") || hasNonFieldBinding {
		return "", false
	}
	field, found := semaResolveField(model, current.Name, receiver, make(map[string]bool))
	if !found || hasModifier(field.member.Modifiers, "static") {
		return "", false
	}
	_, lookupName, ok := semaClassMembersForReceiver(model, current, receiver)
	if !ok {
		return "", false
	}
	for _, candidate := range resolveMemberMethods(model, lookupName, method) {
		if hasModifier(candidate.member.Modifiers, "static") {
			return lookupName, true
		}
	}
	return "", false
}

func semaUnshadowedPlatformTypeReceiver(model *semaTypeMemberView, scope map[string]string, receiver string) bool {
	if scope[normalizeName(receiver)] != "" || semaProjectTypeShadowsPlatform(model, receiver) {
		return false
	}
	members, ok := model.lookup(normalizeName(receiver))
	return ok && members.dependency && semaKnownPlatformTypeReceiver(members.name) && semaPlatformReceiverSpellingMatches(receiver, members)
}

func semaCurrentClassMembers(model *semaTypeMemberView, current typesys.TypeSymbol) (typeMembers, string, bool) {
	if members, ok := model.lookup(normalizeName(current.Name)); ok {
		return members, current.Name, true
	}
	if current.Dependency && current.Namespace != "" {
		qualified := current.Namespace + "." + current.Name
		if members, ok := model.lookup(normalizeName(qualified)); ok {
			return members, qualified, true
		}
	}
	return typeMembers{}, "", false
}

func semaArgTypes(args []semaArg, scope map[string]string, model *semaTypeMemberView) []string {
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = semaArgType(arg.text, scope, model)
	}
	return argTypes
}

func semaArgType(arg string, scope map[string]string, model *semaTypeMemberView) string {
	if typ := semaSOQLLiteralListType(arg); typ != "" {
		return typ
	}
	return inferSemaArgTypeWithModel(arg, scope, model)
}

func semaSOQLLiteralListType(arg string) string {
	arg = strings.TrimSpace(arg)
	if !strings.HasPrefix(arg, "[") || !strings.HasSuffix(arg, "]") {
		return ""
	}
	queryText := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(arg, "["), "]"))
	query, err := soql.Parse(queryText)
	if err != nil || query.Count || len(query.Aggregates) > 0 || len(query.GroupBy) > 0 || query.Having != nil || strings.TrimSpace(query.Object) == "" {
		if objectName := semaSOQLLiteralFallbackObject(queryText); objectName != "" {
			return "List<" + objectName + ">"
		}
		return ""
	}
	return "List<" + query.Object + ">"
}

func semaSOQLLiteralFallbackObject(queryText string) string {
	normalized := strings.TrimSpace(queryText)
	lower := strings.ToLower(normalized)
	if !strings.HasPrefix(lower, "select ") || strings.Contains(lower, " count(") || strings.Contains(lower, " group by ") || strings.Contains(lower, " having ") {
		return ""
	}
	from := semaTopLevelSOQLFromIndex(normalized)
	if from < 0 {
		return ""
	}
	rest := strings.TrimSpace(normalized[from+len(" from "):])
	if rest == "" {
		return ""
	}
	end := 0
	for end < len(rest) && (isIdentifierByte(rest[end]) || rest[end] == '.') {
		end++
	}
	if end == 0 {
		return ""
	}
	return rest[:end]
}

func semaTopLevelSOQLFromIndex(queryText string) int {
	lower := strings.ToLower(queryText)
	depth := 0
	for i := 0; i < len(lower); i++ {
		switch lower[i] {
		case '\'':
			i = skipSemaString(lower, i)
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && strings.HasPrefix(lower[i:], " from ") {
				return i
			}
		}
	}
	return -1
}

func semaChainedMethodMatchesCallee(callee, method string) bool {
	if _, last, ok := splitSemaMethodPath(callee); ok {
		return strings.EqualFold(last, method)
	}
	return true
}

func semaUnresolvedFluentReceiver(receiverType string, model *semaTypeMemberView) bool {
	if receiverType == "" || isSemaBuiltinType(receiverType) {
		return false
	}
	if _, ok := model.lookup(normalizeName(receiverType)); ok {
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

func semaResolvedMembersAllPlatformBacked(model *semaTypeMemberView, candidates []resolvedMember) bool {
	if len(candidates) == 0 {
		return false
	}
	for _, candidate := range candidates {
		owner, ok := model.lookup(normalizeName(candidate.owner))
		if !ok || (!owner.dependency && !owner.sobject) {
			return false
		}
	}
	return true
}

func resolveMemberMethods(model *semaTypeMemberView, typeName, method string) []resolvedMember {
	return resolveMemberMethodsSeen(model, typeName, method, make(map[string]bool))
}

func resolveImplicitMemberMethods(model *semaTypeMemberView, typeName, method string) []resolvedMember {
	var out []resolvedMember
	seen := make(map[string]bool)
	for _, candidate := range resolveMemberMethods(model, typeName, method) {
		key := candidate.owner + ":" + methodSignatureKey(candidate.member)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)
	}
	parts := strings.Split(typeName, ".")
	for i := len(parts) - 1; i > 0; i-- {
		owner := strings.Join(parts[:i], ".")
		for _, candidate := range resolveMemberMethods(model, owner, method) {
			key := candidate.owner + ":" + methodSignatureKey(candidate.member)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, candidate)
		}
	}
	return out
}

func resolveMemberMethodsSeen(model *semaTypeMemberView, typeName, method string, seen map[string]bool) []resolvedMember {
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

func (a *Analyzer) diagnoseConstructorChain(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, args []semaArg, start, end int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
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
	if resolved := resolveNestedTypeName(model, typ.Name, targetType); resolved != "" {
		targetType = resolved
	}
	target, ok := model.lookup(normalizeName(targetType))
	if !ok {
		if callee == "super" {
			return nil
		}
		return []diagnostic.Diagnostic{constructorDiagnostic(typ, member, callee, fmt.Sprintf("unknown constructor target %q", targetType), start, end, source)}
	}
	if len(target.constructors) == 0 && len(args) == 0 {
		return nil
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = semaResolveConstructedExpressionType(model, typ.Name, arg.text, map[string]string{})
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
		if semaArgTypesContainUnknown(argTypes) {
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

func (a *Analyzer) diagnoseMethodCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, candidates []resolvedMember, args []semaArg, haveArgs bool, receiverMode string, start, end int, source string, scope map[string]string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	candidates = preferResolvedMethodsByReceiverMode(candidates, receiverMode)
	var argTypes []string
	if haveArgs {
		argTypes = make([]string, len(args))
		for i, arg := range args {
			argTypes[i] = semaArgType(arg.text, scope, model)
		}
		if diag, blocked := diagnoseDatabaseExecuteBatchArg(typ, member, callee, argTypes, start, end, source, model); blocked {
			return []diagnostic.Diagnostic{diag}
		}
	}
	if len(candidates) == 0 {
		if semaCallMayBelongToMissingSuperclass(model, typ, callee, receiverMode, "") {
			return nil
		}
		if receiverMode == "implicit" {
			if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, typ.Name, callee, args, start, end, source, scope, model, "implicit"); handled {
				return platformDiagnostics
			}
		}
		if receiverExpr, method, ok := splitSemaMethodPath(callee); ok && semaUnshadowedPlatformTypeReceiver(model, scope, receiverExpr) {
			if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverExpr, method, args, start, end, source, scope, model, receiverMode); handled {
				return platformDiagnostics
			}
			return []diagnostic.Diagnostic{unknownCallDiagnostic(typ, member, callee, start, end, source)}
		}
		if receiverExpr, method, ok := strings.Cut(callee, "."); ok && receiverExpr != "" && method != "" {
			if lastDot := strings.LastIndex(callee, "."); lastDot > 0 && lastDot < len(callee)-1 {
				receiverExpr = callee[:lastDot]
				method = callee[lastDot+1:]
			}
			if semaExternalPackageSObjectFieldPath(receiverExpr, scope, model) {
				return nil
			}
			if semaKnownChildRelationshipCollectionCall(receiverExpr, method, scope, model) {
				return nil
			}
			if semaTextCallReceiverEntersDependencyType(model, receiverExpr, scope) {
				return nil
			}
			receiverType := inferSemaFieldAccessType(receiverExpr, scope, model)
			if receiverType == "" {
				if scoped, ok := scope[normalizeName(receiverExpr)]; ok {
					receiverType = scoped
				}
			}
			if receiverType == "" {
				receiverParts := strings.Split(receiverExpr, ".")
				if len(receiverParts) > 0 && strings.HasSuffix(normalizeName(receiverParts[len(receiverParts)-1]), "address") {
					receiverType = "Address"
				}
			}
			if receiverType != "" {
				if semaCallMayBelongToMissingSuperclass(model, typ, callee, receiverMode, receiverType) {
					return nil
				}
				if resolved := resolveMemberMethods(model, receiverType, method); len(resolved) != 0 {
					return a.diagnoseMethodCall(typ, member, callee, resolved, args, haveArgs, receiverMode, start, end, source, scope, model)
				}
				if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok {
					argTypes := make([]string, len(args))
					for i, arg := range args {
						argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
					}
					if len(args) == 0 || semaArgsMatchAny(sig.params, argTypes, model) {
						return nil
					}
				}
				if a.hasKnown(receiverType) {
					return nil
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
		if strings.Count(callee, ".") != 1 && semaSourceHasDottedCall(source, callee) {
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
	if receiver, method, ok := splitSemaMethodPath(callee); ok && semaDatabaseDynamicQueryCall(semaTextReceiverType(receiver, scope, model), method) {
		return nil
	}
	if candidate, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, argTypes, model); ok {
		if staticDiagnostic, blocked := checkSemaStaticAccessWithModel(typ, member, callee, candidate, receiverMode, start, end, source, model); blocked {
			return []diagnostic.Diagnostic{staticDiagnostic}
		}
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, callee, candidate, start, end, source, model); blocked {
			accessible := make([]resolvedMember, 0, len(candidates))
			for _, alternate := range candidates {
				if !memberApplicable(alternate.member, argTypes, model) {
					continue
				}
				if _, staticBlocked := checkSemaStaticAccessWithModel(typ, member, callee, alternate, receiverMode, start, end, source, model); staticBlocked {
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
		if semaArgTypesContainUnknown(argTypes) {
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
	if candidate, ok := bestResolvedMemberBySOQLSingletonArgs(candidates, argTypes, args, model); ok {
		if staticDiagnostic, blocked := checkSemaStaticAccessWithModel(typ, member, callee, candidate, receiverMode, start, end, source, model); blocked {
			return []diagnostic.Diagnostic{staticDiagnostic}
		}
		if visibilityDiagnostic, blocked := checkSemaMemberAccess(typ, member, callee, candidate, start, end, source, model); blocked {
			return []diagnostic.Diagnostic{visibilityDiagnostic}
		}
		return nil
	}
	if receiverMode != "implicit" && semaCalleeDependencyRoot(callee, scope, model) {
		return nil
	}
	if receiverExpr, method, ok := splitSemaMethodPath(callee); ok && semaObjectMethodName(method) {
		receiverType := semaTextReceiverType(receiverExpr, scope, model)
		if sig, ok := semaPlatformMethodSignatureForMode(model, receiverType, method, receiverMode); ok {
			if semaArgsMatchAny(sig.params, semaArgTypes(args, scope, model), model) {
				return nil
			}
		}
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

func diagnoseDatabaseExecuteBatchArg(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, argTypes []string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if len(argTypes) == 0 || !isBatchEnqueueCallee(callee) {
		return diagnostic.Diagnostic{}, false
	}
	argType := strings.TrimSpace(argTypes[0])
	if argType == "" || strings.EqualFold(argType, "null") {
		return diagnostic.Diagnostic{}, false
	}
	if semaAssignableToType("Database.Batchable", argType, model) || semaAssignableToType("Batchable", argType, model) {
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA009",
		Message:  fmt.Sprintf("%s %q has no matching overload for call %q: first argument must implement Database.Batchable", member.Kind, member.Name, callee),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}, true
}

func isBatchEnqueueCallee(callee string) bool {
	parts := strings.Split(callee, ".")
	if len(parts) < 2 {
		return false
	}
	member := parts[len(parts)-1]
	owner := parts[len(parts)-2]
	switch {
	case strings.EqualFold(member, "executeBatch"):
		return strings.EqualFold(owner, "Database")
	case strings.EqualFold(member, "scheduleBatch"):
		return strings.EqualFold(owner, "System")
	default:
		return false
	}
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

func semaImmediateDottedCallResolved(body string, callStart int, args []semaArg, scope map[string]string, model *semaTypeMemberView) bool {
	receiverExpr, method, ok := semaImmediateDottedCallReceiverExpr(body, callStart)
	if !ok {
		return false
	}
	receiverType := semaTextReceiverType(receiverExpr, scope, model)
	if receiverType == "" {
		return false
	}
	argTypes := semaArgTypes(args, scope, model)
	if _, ok, _ := bestResolvedMemberByArgTypes(preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, receiverType, method), "instance"), argTypes, model); ok {
		return true
	}
	if sig, ok := semaPlatformMethodSignatureFor(model, receiverType, method); ok && semaArgsMatchAny(sig.params, argTypes, model) {
		return true
	}
	if sig, ok := semaCollectionMethodSignature(receiverType, method); ok && semaArgsMatchAny(sig.params, argTypes, model) {
		return true
	}
	return false
}

func semaImmediateDottedCallReceiverExpr(body string, callStart int) (string, string, bool) {
	if callStart < 0 || callStart >= len(body) || !isIdentifierByte(body[callStart]) {
		return "", "", false
	}
	methodEnd := callStart
	for methodEnd < len(body) && isIdentifierByte(body[methodEnd]) {
		methodEnd++
	}
	dot := callStart - 1
	for dot >= 0 && isWhitespace(body[dot]) {
		dot--
	}
	if dot <= 0 || body[dot] != '.' {
		return "", "", false
	}
	receiverEnd := dot - 1
	for receiverEnd >= 0 && isWhitespace(body[receiverEnd]) {
		receiverEnd--
	}
	if receiverEnd < 0 {
		return "", "", false
	}
	receiverStart := receiverEnd
	if body[receiverEnd] == ')' {
		open := matchingOpenParenBefore(body, receiverEnd)
		if open < 0 {
			return "", "", false
		}
		receiverStart = open
		for receiverStart > 0 && isIdentifierByte(body[receiverStart-1]) {
			receiverStart--
		}
	} else {
		for receiverStart > 0 {
			ch := body[receiverStart-1]
			if isIdentifierByte(ch) || ch == '.' || ch == '?' {
				receiverStart--
				continue
			}
			break
		}
	}
	receiverExpr := strings.TrimSpace(body[receiverStart : receiverEnd+1])
	method := strings.TrimSpace(body[callStart:methodEnd])
	return receiverExpr, method, receiverExpr != "" && method != ""
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

func semaCalleeDependencyRoot(callee string, scope map[string]string, model *semaTypeMemberView) bool {
	root, _, ok := strings.Cut(strings.TrimSpace(callee), ".")
	if !ok || root == "" {
		return false
	}
	if typ, ok := scope[normalizeName(root)]; ok {
		return semaDependencyType(model, typ)
	}
	return semaDependencyType(model, root)
}

func semaCallMayBelongToMissingSuperclass(model *semaTypeMemberView, typ typesys.TypeSymbol, callee, receiverMode, receiverType string) bool {
	trimmed := strings.TrimSpace(callee)
	if receiverMode == "implicit" || receiverMode == "super" || strings.HasPrefix(strings.ToLower(trimmed), "super.") {
		return semaTypeHasMissingSuperclass(model, semaTypeMembersName(typ), map[string]bool{})
	}
	if receiverType == "" {
		return false
	}
	return semaTypeHasMissingSuperclass(model, receiverType, map[string]bool{})
}

func semaTypeHasMissingSuperclass(model *semaTypeMemberView, typeName string, seen map[string]bool) bool {
	if typeName == "" {
		return false
	}
	key := normalizeName(typeName)
	if seen[key] {
		return false
	}
	seen[key] = true
	members, ok := model.lookup(key)
	if !ok || members.superClass == "" {
		return false
	}
	superName := members.superClass
	if resolved := resolveNestedTypeName(model, members.name, superName); resolved != "" {
		superName = resolved
	}
	if _, ok := model.lookup(normalizeName(superName)); !ok {
		return true
	}
	return semaTypeHasMissingSuperclass(model, superName, seen)
}

func semaDependencyType(model *semaTypeMemberView, typeName string) bool {
	if typeName == "" {
		return false
	}
	if members, ok := model.lookup(normalizeName(typeName)); ok {
		return members.dependency
	}
	return false
}

func semaKnownChildRelationshipCollectionCall(receiverExpr, method string, scope map[string]string, model *semaTypeMemberView) bool {
	parentExpr, relationship, ok := strings.Cut(strings.TrimSpace(receiverExpr), ".")
	if !ok || strings.Contains(relationship, ".") {
		return false
	}
	parentType := inferSemaArgTypeWithModel(parentExpr, scope, model)
	if parentType == "" {
		return false
	}
	if typ := semaKnownChildRelationshipListType(parentType, relationship, model); typ != "" {
		if sig, ok := semaCollectionMethodSignature(typ, method); ok && sig.returnType != "" {
			return true
		}
	}
	return false
}

func semaKnownChildRelationshipListType(parentType, relationship string, model *semaTypeMemberView) string {
	if target, ok := semaResolveFieldPath(model, parentType, relationship); ok {
		base, _ := semaGenericBaseAndArgs(target.member.Type)
		if strings.EqualFold(base, "List") {
			return target.member.Type
		}
	}
	for _, relationshipMember := range semaStandardChildRelationshipMembers(parentType) {
		if strings.EqualFold(relationshipMember.name, relationship) {
			return relationshipMember.typ
		}
	}
	return ""
}

func semaAssignmentOperatorNeighbor(arg string, i int) bool {
	return (i > 0 && strings.ContainsRune("=!<>", rune(arg[i-1]))) ||
		(i+1 < len(arg) && strings.ContainsRune("=>", rune(arg[i+1])))
}

func checkSemaStaticAccessWithModel(from typesys.TypeSymbol, context typesys.MemberSymbol, callee string, target resolvedMember, receiverMode string, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if receiverMode == "instance" && hasModifier(target.member.Modifiers, "static") {
		if owner, ok := model.lookup(normalizeName(target.owner)); ok {
			if !owner.dependency && !owner.sobject {
				return diagnostic.Diagnostic{}, false
			}
			return checkSemaStaticAccessStrict(from, context, callee, target, receiverMode, start, end, source)
		}
	}
	return checkSemaStaticAccess(from, context, callee, target, receiverMode, start, end, source)
}

func checkSemaStaticAccess(from typesys.TypeSymbol, context typesys.MemberSymbol, callee string, target resolvedMember, receiverMode string, start, end int, source string) (diagnostic.Diagnostic, bool) {
	if receiverMode == "instance" && hasModifier(target.member.Modifiers, "static") && !semaGeneratedPlatformStaticOwner(target.owner) {
		return diagnostic.Diagnostic{}, false
	}
	return checkSemaStaticAccessStrict(from, context, callee, target, receiverMode, start, end, source)
}

func checkSemaStaticAccessStrict(from typesys.TypeSymbol, context typesys.MemberSymbol, callee string, target resolvedMember, receiverMode string, start, end int, source string) (diagnostic.Diagnostic, bool) {
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
			if semaStaticAccessLooksTypeQualifiedAt(source, start) {
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

func semaGeneratedPlatformStaticOwner(ownerName string) bool {
	ownerName = strings.TrimSpace(ownerName)
	if ownerName == "" {
		return false
	}
	normalized := normalizeName(ownerName)
	for _, symbol := range typesys.StandardPlatformSymbolView() {
		if normalizeName(symbol.Name) == normalized {
			return true
		}
	}
	return false
}

func semaStaticAccessLooksTypeQualifiedAt(source string, start int) bool {
	if start <= 0 || start > len(source) {
		return false
	}
	i := start - 1
	for i >= 0 && isWhitespace(source[i]) {
		i--
	}
	if i < 0 || source[i] != '.' {
		return false
	}
	i--
	for i >= 0 && isWhitespace(source[i]) {
		i--
	}
	end := i + 1
	for i >= 0 && (isIdentifierByte(source[i]) || source[i] == '.') {
		i--
	}
	receiver := strings.TrimSpace(source[i+1 : end])
	if receiver == "" {
		return false
	}
	root, _, _ := strings.Cut(receiver, ".")
	return root != "" && root[0] >= 'A' && root[0] <= 'Z'
}

func semaTextReceiverExprLooksLikeType(receiverExpr string, scope map[string]string, model *semaTypeMemberView) bool {
	receiverExpr = strings.TrimSpace(receiverExpr)
	if receiverExpr == "" {
		return false
	}
	root, _, _ := strings.Cut(receiverExpr, ".")
	if root != "" {
		if _, scoped := scope[normalizeName(root)]; scoped {
			return false
		}
	}
	if root, fieldPath, ok := strings.Cut(receiverExpr, "."); ok && root != "" && fieldPath != "" {
		if _, staticOK := semaStaticClassFieldPathMemberInContext(model, scope[semaCurrentTypeScopeKey], root, fieldPath); staticOK {
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

func checkSemaMemberAccess(from typesys.TypeSymbol, context typesys.MemberSymbol, callee string, target resolvedMember, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
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

func semaIsSubclass(model *semaTypeMemberView, child, parent string) bool {
	seen := make(map[string]bool)
	for child != "" {
		key := normalizeName(child)
		if seen[key] {
			return false
		}
		seen[key] = true
		members, ok := model.lookup(key)
		if !ok {
			return false
		}
		if semaTypeNameMatches(model, members.name, members.superClass, parent) {
			return true
		}
		resolved := resolveNestedTypeName(model, members.name, members.superClass)
		if resolved == "" {
			resolved = members.superClass
		}
		child = resolved
	}
	return false
}

func semaTypeNameMatches(model *semaTypeMemberView, context, left, right string) bool {
	if normalizeName(left) == normalizeName(right) {
		return true
	}
	if semaShortTypeKey(left) != "" && semaShortTypeKey(left) == semaShortTypeKey(right) {
		return true
	}
	leftResolved := resolveNestedTypeName(model, context, left)
	rightResolved := resolveNestedTypeName(model, context, right)
	switch {
	case leftResolved != "" && normalizeName(leftResolved) == normalizeName(right):
		return true
	case rightResolved != "" && normalizeName(left) == normalizeName(rightResolved):
		return true
	case leftResolved != "" && rightResolved != "" && normalizeName(leftResolved) == normalizeName(rightResolved):
		return true
	default:
		return false
	}
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

func callArgsMatch(params []apexast.Parameter, args []semaArg, scope map[string]string, model *semaTypeMemberView) bool {
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

func bestResolvedMemberByArgTypes(candidates []resolvedMember, argTypes []string, model *semaTypeMemberView) (resolvedMember, bool, bool) {
	applicable := make([]resolvedMember, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicable(candidate.member, argTypes, model) {
			applicable = append(applicable, candidate)
		}
	}
	if best, ok := bestResolvedMemberByExactObjectTieBreak(applicable, argTypes); ok {
		return best, true, false
	}
	if best, ok := bestResolvedMemberByConversionScore(applicable, argTypes, model); ok {
		return best, true, false
	}
	return bestResolvedMemberBySpecificity(applicable, model)
}

func bestMemberByArgTypes(candidates []typesys.MemberSymbol, argTypes []string, model *semaTypeMemberView) (typesys.MemberSymbol, bool, bool) {
	applicable := make([]typesys.MemberSymbol, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicable(candidate, argTypes, model) {
			applicable = append(applicable, candidate)
		}
	}
	if best, ok := bestMemberByExactObjectTieBreak(applicable, argTypes); ok {
		return best, true, false
	}
	if best, ok := bestMemberByConversionScore(applicable, argTypes, model); ok {
		return best, true, false
	}
	return bestMemberBySpecificity(applicable, model)
}

func bestResolvedMemberBySOQLSingletonArgs(candidates []resolvedMember, argTypes []string, args []semaArg, model *semaTypeMemberView) (resolvedMember, bool) {
	applicable := make([]resolvedMember, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicableWithSOQLSingletonArgs(candidate.member, argTypes, args, model) {
			applicable = append(applicable, candidate)
		}
	}
	best, ok, ambiguous := bestResolvedMemberBySpecificity(applicable, model)
	return best, ok && !ambiguous
}

func memberApplicableWithSOQLSingletonArgs(candidate typesys.MemberSymbol, argTypes []string, args []semaArg, model *semaTypeMemberView) bool {
	if len(candidate.Parameters) != len(argTypes) || len(args) != len(argTypes) {
		return false
	}
	usedSingleton := false
	for i, param := range candidate.Parameters {
		if semaConversionScore(param.Type, argTypes[i], model) >= 0 {
			continue
		}
		if semaSOQLSingletonArgAssignable(param.Type, argTypes[i], args[i].text, model) {
			usedSingleton = true
			continue
		}
		return false
	}
	return usedSingleton
}

func bestConstructorByArgTypes(candidates []typesys.MemberSymbol, positionalArgTypes []string, namedArgTypes map[string]string, model *semaTypeMemberView) (typesys.MemberSymbol, bool, bool) {
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

func bestConstructorByIRSOQLSingletonArgs(candidates []typesys.MemberSymbol, argTypes []string, args []ir.Expr, model *semaTypeMemberView) (typesys.MemberSymbol, bool, bool) {
	applicable := make([]typesys.MemberSymbol, 0, len(candidates))
	for _, candidate := range candidates {
		if memberApplicableWithIRSOQLSingletonArgs(candidate, argTypes, args, model) {
			applicable = append(applicable, candidate)
		}
	}
	if best, ok := bestMemberByExactObjectTieBreak(applicable, argTypes); ok {
		return best, true, false
	}
	if best, ok := bestMemberByConversionScore(applicable, argTypes, model); ok {
		return best, true, false
	}
	return bestMemberBySpecificity(applicable, model)
}

func memberApplicableWithIRSOQLSingletonArgs(candidate typesys.MemberSymbol, argTypes []string, args []ir.Expr, model *semaTypeMemberView) bool {
	if len(candidate.Parameters) != len(argTypes) || len(args) != len(argTypes) {
		return false
	}
	usedSingleton := false
	for i, param := range candidate.Parameters {
		if semaConversionScore(param.Type, argTypes[i], model) >= 0 {
			continue
		}
		if args[i].Kind == ir.ExprSOQL && semaSOQLSingletonAssignable(param.Type, argTypes[i], "["+args[i].Value+"]", model) {
			usedSingleton = true
			continue
		}
		return false
	}
	return usedSingleton
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

func bestResolvedMemberByConversionScore(applicable []resolvedMember, argTypes []string, model *semaTypeMemberView) (resolvedMember, bool) {
	bestIndex, ok := bestMemberConversionScoreIndex(len(applicable), argTypes, model, func(i int) typesys.MemberSymbol {
		return applicable[i].member
	})
	if !ok {
		return resolvedMember{}, false
	}
	return applicable[bestIndex], true
}

func bestMemberByConversionScore(applicable []typesys.MemberSymbol, argTypes []string, model *semaTypeMemberView) (typesys.MemberSymbol, bool) {
	bestIndex, ok := bestMemberConversionScoreIndex(len(applicable), argTypes, model, func(i int) typesys.MemberSymbol {
		return applicable[i]
	})
	if !ok {
		return typesys.MemberSymbol{}, false
	}
	return applicable[bestIndex], true
}

func bestMemberConversionScoreIndex(count int, argTypes []string, model *semaTypeMemberView, candidateAt func(int) typesys.MemberSymbol) (int, bool) {
	bestIndex := -1
	bestScore := 0
	bestExact := 0
	scores := make([]int, count)
	exacts := make([]int, count)
	for i := 0; i < count; i++ {
		candidate := candidateAt(i)
		if len(candidate.Parameters) != len(argTypes) {
			scores[i] = -1
			continue
		}
		score := 0
		exact := 0
		for j, param := range candidate.Parameters {
			conversion := semaConversionScore(param.Type, argTypes[j], model)
			if conversion < 0 {
				score = -1
				break
			}
			if strings.EqualFold(semaCanonicalPlatformAlias(param.Type), semaCanonicalPlatformAlias(argTypes[j])) {
				exact++
			}
			score += conversion
		}
		scores[i] = score
		exacts[i] = exact
		if score < 0 {
			continue
		}
		switch {
		case bestIndex < 0 || score > bestScore:
			bestIndex = i
			bestScore = score
			bestExact = exact
		}
	}
	if bestIndex < 0 || bestExact == 0 {
		return -1, false
	}
	for i := 0; i < count; i++ {
		if i == bestIndex || scores[i] < 0 {
			continue
		}
		if scores[i] == bestScore || exacts[i] >= bestExact {
			return -1, false
		}
	}
	return bestIndex, true
}

func bestMemberExactObjectTieBreakIndex(count int, argTypes []string, candidateAt func(int) typesys.MemberSymbol) int {
	if count < 2 || len(argTypes) != 1 {
		return -1
	}
	if strings.EqualFold(argTypes[0], "Database.QueryResult") {
		bestIndex := -1
		for i := 0; i < count; i++ {
			candidate := candidateAt(i)
			if len(candidate.Parameters) != 1 {
				continue
			}
			base, _ := semaGenericBaseAndArgs(candidate.Parameters[0].Type)
			if !strings.EqualFold(base, "List") {
				continue
			}
			if bestIndex >= 0 {
				return -1
			}
			bestIndex = i
		}
		return bestIndex
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

func memberApplicable(candidate typesys.MemberSymbol, argTypes []string, model *semaTypeMemberView) bool {
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

func memberApplicableWithNamedArgs(candidate typesys.MemberSymbol, positionalArgTypes []string, namedArgTypes map[string]string, model *semaTypeMemberView) bool {
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

func bestResolvedMemberBySpecificity(applicable []resolvedMember, model *semaTypeMemberView) (resolvedMember, bool, bool) {
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

func compareResolvedSemaMemberSpecificity(left, right resolvedMember, model *semaTypeMemberView) int {
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

func bestMemberBySpecificity(applicable []typesys.MemberSymbol, model *semaTypeMemberView) (typesys.MemberSymbol, bool, bool) {
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

func compareSemaMemberSpecificity(left, right typesys.MemberSymbol, model *semaTypeMemberView) int {
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

func compareSemaTypeSpecificity(left, right string, model *semaTypeMemberView) int {
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

func semaConversionScore(paramType, argType string, model *semaTypeMemberView) int {
	if argType == "" || strings.EqualFold(argType, "null") {
		return 1
	}
	paramType = semaCanonicalAssignableType(paramType)
	argType = semaCanonicalAssignableType(argType)
	if strings.EqualFold(argType, "void") {
		return -1
	}
	if strings.EqualFold(paramType, argType) {
		return 1000
	}
	if strings.EqualFold(argType, "Database.QueryResult") && semaDynamicQueryResultAssignableTo(paramType, model) {
		if base, _ := semaGenericBaseAndArgs(paramType); strings.EqualFold(base, "List") {
			return 875
		}
		return 840
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
	if semaAssignableToType(paramType, argType, model) {
		return 700
	}
	if strings.EqualFold(paramType, "Object") {
		return 10
	}
	if distance, ok := semaTypeDistance(model, argType, paramType, make(map[string]bool)); ok {
		return 800 - distance
	}
	return -1
}

func semaSOQLSingletonArgAssignable(paramType, argType, argText string, model *semaTypeMemberView) bool {
	argText = strings.TrimSpace(argText)
	if !strings.HasPrefix(argText, "[") || !strings.HasSuffix(argText, "]") {
		return false
	}
	if !semaSOQLSingletonAssignable(paramType, argType, argText, model) {
		return false
	}
	base, args := semaGenericBaseAndArgs(argType)
	if !strings.EqualFold(base, "List") || len(args) != 1 {
		return false
	}
	return semaAssignableToType(paramType, args[0], model)
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

func semaAssignableToType(paramType, argType string, model *semaTypeMemberView) bool {
	paramType = semaCanonicalAssignableType(paramType)
	argType = semaCanonicalAssignableType(argType)
	if strings.EqualFold(argType, "void") {
		return false
	}
	if strings.EqualFold(argType, "Database.QueryResult") {
		return semaDynamicQueryResultAssignableTo(paramType, model)
	}
	if semaSObjectTypeTokenAssignableToDescribe(paramType, argType, model) {
		return true
	}
	if strings.EqualFold(paramType, argType) || strings.EqualFold(paramType, "Object") {
		return true
	}
	if semaCustomAPITypeLocalNamesMatch(paramType, argType) {
		return true
	}
	if paramSchemaName, paramSchemaOK := semaSchemaQualifiedTypeName(paramType); paramSchemaOK {
		if argSchemaName, argSchemaOK := semaSchemaQualifiedTypeName(argType); argSchemaOK {
			return strings.EqualFold(paramSchemaName, argSchemaName) ||
				(strings.EqualFold(paramSchemaName, "SObject") && isSemaSObjectLike(argSchemaName, model)) ||
				(isSemaSObjectLike(paramSchemaName, model) && strings.EqualFold(argSchemaName, "SObject"))
		}
		if strings.EqualFold(paramSchemaName, argType) ||
			(strings.EqualFold(paramSchemaName, "SObject") && isSemaSObjectLike(argType, model)) ||
			(isSemaSObjectLike(paramSchemaName, model) && strings.EqualFold(argType, "SObject")) {
			return true
		}
	}
	if argSchemaName, argSchemaOK := semaSchemaQualifiedTypeName(argType); argSchemaOK {
		if strings.EqualFold(paramType, argSchemaName) ||
			(strings.EqualFold(paramType, "SObject") && isSemaSObjectLike(argSchemaName, model)) ||
			(isSemaSObjectLike(paramType, model) && strings.EqualFold(argSchemaName, "SObject")) {
			return true
		}
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
	if semaSchemaDescribeMapAssignable(paramType, argType) {
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

func semaSObjectTypeTokenAssignableToDescribe(paramType, argType string, model *semaTypeMemberView) bool {
	if !strings.EqualFold(paramType, "Schema.DescribeSObjectResult") && !strings.EqualFold(paramType, "DescribeSObjectResult") {
		return false
	}
	if strings.EqualFold(argType, "Schema.SObjectType") || strings.EqualFold(argType, "SObjectType") {
		return true
	}
	if semaLooksLikeSObjectTypeTokenInModel(argType, model) {
		return true
	}
	if schemaName, ok := semaSchemaQualifiedTypeName(argType); ok {
		return semaLooksLikeSObjectTypeTokenInModel(schemaName, model)
	}
	return false
}

func semaSchemaDescribeMapAssignable(paramType, argType string) bool {
	paramBase, paramArgs := semaGenericBaseAndArgs(paramType)
	if !strings.EqualFold(paramBase, "Map") || len(paramArgs) != 2 || !strings.EqualFold(paramArgs[0], "String") {
		return false
	}
	switch {
	case strings.EqualFold(argType, "Schema.SObjectTypeFields"):
		return strings.EqualFold(paramArgs[1], "Schema.SObjectField")
	case strings.EqualFold(argType, "Schema.SObjectTypeFieldSets"):
		return strings.EqualFold(paramArgs[1], "Schema.FieldSet")
	default:
		return false
	}
}

func semaNestedShortTypeEquivalent(left, right string, model *semaTypeMemberView) bool {
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

func semaDynamicQueryResultAssignableTo(paramType string, model *semaTypeMemberView) bool {
	if strings.EqualFold(paramType, "Object") || strings.EqualFold(paramType, "SObject") || strings.EqualFold(paramType, "AggregateResult") || isSemaSObjectLike(paramType, model) {
		return true
	}
	base, args := semaGenericBaseAndArgs(paramType)
	if !strings.EqualFold(base, "List") || len(args) > 1 {
		return false
	}
	return len(args) == 0 || strings.EqualFold(args[0], "Object") || strings.EqualFold(args[0], "SObject") || strings.EqualFold(args[0], "AggregateResult") || isSemaSObjectLike(args[0], model)
}

func semaPlatformAssignableToType(paramType, argType string, model *semaTypeMemberView) bool {
	paramBase, paramArgs := semaGenericBaseAndArgs(semaCanonicalPlatformAlias(paramType))
	argType = semaCanonicalPlatformAlias(argType)
	if strings.EqualFold(paramBase, "Database.BatchableContext") && strings.EqualFold(argType, "Database.BatchableContextImpl") {
		return true
	}
	if strings.EqualFold(paramBase, "System.FinalizerContext") && strings.EqualFold(argType, "System.FinalizerContextImpl") {
		return true
	}
	if strings.EqualFold(paramBase, "Cache.Partition") &&
		(strings.EqualFold(argType, "Cache.OrgPartition") || strings.EqualFold(argType, "Cache.SessionPartition")) {
		return true
	}
	if strings.EqualFold(paramType, "ApexPages.Component") && semaVisualforceComponentType(argType) {
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
	case "assertionexception", "assertexception", "aurahandledexception", "asyncexception", "bigobjectexception", "calloutexception", "canvasexception", "dmlexception", "emailexception", "externalobjectexception", "illegalargumentexception", "illegalstateexception", "invalidparametervalueexception", "jsonexception", "limitexception", "listexception", "mathexception", "noaccessexception", "nodatafoundexception", "nosuchelementexception", "nullpointerexception", "patternsyntaxexception", "queryexception", "requiredfeaturemissingexception", "searchexception", "securityexception", "sobjectexception", "stringexception", "typeexception", "xmlexception":
		return true
	default:
		base := shortNestedTypeName(typeName)
		return strings.Contains(typeName, ".") && strings.HasSuffix(normalizeName(base), "exception")
	}
}

func semaVisualforceComponentType(typeName string) bool {
	parts := strings.Split(strings.TrimSpace(typeName), ".")
	return len(parts) >= 2 && strings.EqualFold(parts[0], "Component")
}

func semaMessagingEmailAssignable(paramType, argType string) bool {
	return strings.EqualFold(paramType, "Messaging.Email") &&
		(strings.EqualFold(argType, "Messaging.SingleEmailMessage") || strings.EqualFold(argType, "Messaging.MassEmailMessage"))
}

func semaGenericAssignableToType(paramType, argType string, model *semaTypeMemberView) bool {
	paramType = semaCanonicalAssignableType(paramType)
	argType = semaCanonicalAssignableType(argType)
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
	case "comparator":
		if len(paramArgs) == 0 {
			return true
		}
		if len(paramArgs) != 1 || len(argArgs) != 1 {
			return false
		}
		return semaAssignableToType(paramArgs[0], argArgs[0], model) ||
			semaAssignableToType(argArgs[0], paramArgs[0], model)
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
		return strings.EqualFold(paramArgs[0], "Object") ||
			semaKnownStandardObjectListAssignable(paramArgs[0], argArgs[0], model) ||
			semaTypeMatches(model, argArgs[0], paramArgs[0], make(map[string]bool)) ||
			semaAssignableToType(paramArgs[0], argArgs[0], model) ||
			semaAssignableToType(argArgs[0], paramArgs[0], model) ||
			semaCustomAPITypeLocalNamesMatch(paramArgs[0], argArgs[0])
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

func semaKnownStandardObjectListAssignable(paramElementType, argElementType string, model *semaTypeMemberView) bool {
	if !strings.EqualFold(paramElementType, "SObject") {
		return false
	}
	canonical, ok := storage.ResolveKnownStandardObjectName(shortNestedTypeName(argElementType))
	if !ok {
		return false
	}
	if members, _, ok := semaLookupTypeMembers(model, argElementType); ok {
		return members.sobject
	}
	if members, _, ok := semaLookupTypeMembers(model, canonical); ok {
		return members.sobject
	}
	return ok
}

func semaResolvedMemberReturnType(model *semaTypeMemberView, candidate resolvedMember) string {
	return semaQualifyStandardSObjectType(resolveNestedTypeReference(model, candidate.owner, candidate.member.Type), model)
}

func semaQualifyStandardSObjectType(typeName string, model *semaTypeMemberView) string {
	typeName = strings.TrimSpace(typeName)
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) > 0 {
		resolvedArgs := make([]string, len(args))
		for i, arg := range args {
			resolvedArgs[i] = semaQualifyStandardSObjectType(arg, model)
		}
		return strings.TrimSpace(base) + "<" + strings.Join(resolvedArgs, ",") + ">"
	}
	if strings.EqualFold(typeName, "SObject") || strings.Contains(typeName, ".") {
		return typeName
	}
	members, _, ok := semaLookupTypeMembers(model, typeName)
	if !ok || !members.sobject || !storage.IsKnownStandardObject(members.name) {
		return typeName
	}
	return "Schema." + members.name
}

func semaCanonicalAssignableType(typeName string) string {
	typeName = semaCanonicalPlatformAlias(normalizeArrayType(strings.TrimSpace(typeName)))
	base, args := semaGenericBaseAndArgs(typeName)
	if base == "" || len(args) == 0 {
		return typeName
	}
	for i := range args {
		args[i] = semaCanonicalAssignableType(args[i])
	}
	return strings.TrimSpace(base) + "<" + strings.Join(args, ",") + ">"
}

func semaCustomAPITypeLocalNamesMatch(left, right string) bool {
	left = semaCustomAPITypeLocalName(left)
	right = semaCustomAPITypeLocalName(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func semaCustomAPITypeLocalName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if !semaIsCustomAPIName(typeName) {
		return ""
	}
	parts := strings.Split(typeName, "__")
	if len(parts) >= 3 && parts[0] != "" {
		return strings.Join(parts[1:], "__")
	}
	return typeName
}

func isSemaSObjectLike(typeName string, model *semaTypeMemberView) bool {
	typeName = normalizeArrayType(strings.TrimSpace(typeName))
	if typeName == "" || strings.Contains(typeName, "<") {
		return false
	}
	if schemaName, ok := semaSchemaQualifiedTypeName(typeName); ok {
		return isSemaSObjectLike(schemaName, model)
	}
	if strings.EqualFold(typeName, "SObject") {
		return true
	}
	if strings.EqualFold(typeName, "AggregateResult") {
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
	if members, ok := model.lookup(normalizeName(typeName)); ok {
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

func semaTypeDistance(model *semaTypeMemberView, typeName, target string, seen map[string]bool) (int, bool) {
	key := normalizeName(typeName)
	targetKey := normalizeName(target)
	if key == "" || seen[key] {
		return 0, false
	}
	if key == targetKey {
		return 0, true
	}
	seen[key] = true
	members, ok := model.lookup(key)
	if !ok {
		return semaTypeDistanceByShortName(model, key, target, seen)
	}
	return semaTypeDistanceFromMembers(model, members, target, seen)
}

func semaTypeDistanceFromMembers(model *semaTypeMemberView, members typeMembers, target string, seen map[string]bool) (int, bool) {
	targetKey := normalizeName(target)
	if normalizeName(members.name) == targetKey {
		return 0, true
	}
	if semaShortTypeKey(members.name) == targetKey {
		return 0, true
	}
	if targetMembers, ok := model.lookup(targetKey); ok && normalizeName(targetMembers.name) == normalizeName(members.name) {
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

func semaTypeDistanceByShortName(model *semaTypeMemberView, key, target string, seen map[string]bool) (int, bool) {
	best := 0
	found := false
	for _, candidateKey := range semaShortCandidateKeys(model, key) {
		members := model.get(candidateKey)
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

func semaTypeMatches(model *semaTypeMemberView, typeName, target string, seen map[string]bool) bool {
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
		typeBase, typeArgs := semaGenericBaseAndArgs(typeName)
		targetBase, targetArgs := semaGenericBaseAndArgs(target)
		if !strings.EqualFold(typeBase, targetBase) {
			return true
		}
		if len(typeArgs) == 0 || len(targetArgs) == 0 {
			return true
		}
		if strings.EqualFold(typeBase, "Comparator") && len(typeArgs) == 1 && len(targetArgs) == 1 {
			return semaAssignableToType(targetArgs[0], typeArgs[0], model) ||
				semaAssignableToType(typeArgs[0], targetArgs[0], model)
		}
		return false
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

func semaTypeMatchesFromMembers(model *semaTypeMemberView, members typeMembers, target string, seen map[string]bool) bool {
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
	if targetMembers, ok := model.lookup(targetKey); ok && normalizeName(targetMembers.name) == normalizeName(members.name) {
		return true
	}
	if semaTypeMatches(model, members.superClass, target, seen) {
		return true
	}
	for _, iface := range members.interfaces {
		if semaImplementedInterfaceMatchesTarget(model, members.name, iface, target) {
			return true
		}
		if semaTypeMatches(model, iface, target, seen) {
			return true
		}
	}
	return false
}

func semaImplementedInterfaceMatchesTarget(model *semaTypeMemberView, owner, iface, target string) bool {
	ifaceBase, ifaceArgs := semaGenericBaseAndArgs(iface)
	targetBase, targetArgs := semaGenericBaseAndArgs(target)
	if len(ifaceArgs) == 0 || len(ifaceArgs) != len(targetArgs) || !strings.EqualFold(ifaceBase, targetBase) {
		return false
	}
	for i := range ifaceArgs {
		left := resolveNestedTypeReference(model, owner, ifaceArgs[i])
		right := resolveNestedTypeReference(model, owner, targetArgs[i])
		if strings.EqualFold(left, right) {
			continue
		}
		if semaAssignableToType(right, left, model) || semaAssignableToType(left, right, model) {
			continue
		}
		return false
	}
	return true
}

func semaTypeMatchesByShortName(model *semaTypeMemberView, key, target string, seen map[string]bool) bool {
	found := false
	for _, candidateKey := range semaShortCandidateKeys(model, key) {
		members := model.get(candidateKey)
		if candidateKey == key || seen[candidateKey] {
			continue
		}
		if semaTypeMatchesFromMembers(model, members, target, seen) {
			return true
		}
	}
	return found
}

func semaLookupTypeMembers(model *semaTypeMemberView, typeName string) (typeMembers, string, bool) {
	if members, schemaKey, ok := semaExplicitSchemaSObjectMembers(typeName, model); ok {
		return members, schemaKey, true
	}
	key := normalizeName(typeName)
	if members, ok := model.lookup(key); ok {
		return semaEnsureStandardSObjectTypeMembers(model, key, members), key, true
	}
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) == 0 {
		return typeMembers{}, key, false
	}
	baseKey := normalizeName(base)
	members, ok := model.lookup(baseKey)
	if ok {
		members = semaEnsureStandardSObjectTypeMembers(model, baseKey, members)
	}
	return members, baseKey, ok
}

func semaExplicitSchemaSObjectMembers(typeName string, model *semaTypeMemberView) (typeMembers, string, bool) {
	schemaName, ok := semaSchemaQualifiedTypeName(typeName)
	if !ok || schemaName == "" {
		return typeMembers{}, "", false
	}
	qualifiedKey := normalizeName("Schema." + schemaName)
	if members, ok := model.lookup(qualifiedKey); ok && members.sobject {
		return semaEnsureStandardSObjectTypeMembers(model, qualifiedKey, members), qualifiedKey, true
	}
	schemaKey := normalizeName(schemaName)
	if members, ok := model.lookup(schemaKey); ok && members.sobject {
		return semaEnsureStandardSObjectTypeMembers(model, qualifiedKey, members), qualifiedKey, true
	}
	if objectName, ok := semaStandardSObjectNameForKey(schemaKey); ok {
		members := semaBuildStandardSObjectMembers(objectName)
		model.storeHydrated(qualifiedKey, members)
		return members, qualifiedKey, true
	}
	return typeMembers{}, qualifiedKey, false
}

func semaSchemaQualifiedTypeName(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if len(typeName) <= len("Schema.") || !strings.EqualFold(typeName[:len("Schema.")], "Schema.") {
		return "", false
	}
	return strings.TrimSpace(typeName[len("Schema."):]), true
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

// semaResolveConstructedExpressionType infers expr's type and resolves nested-class
// names relative to owner, same as resolveNestedTypeReference(model, owner,
// inferSemaArgTypeWithModel(expr, scope, model)). When expr is a `new Type(field =
// value, ...)` SObject field-initializer call, that constructor syntax only exists for
// real SObjects, so a genuine standard SObject named Type takes precedence over a
// same-named nested Apex class that nested-class resolution would otherwise prefer.
// This applies at every expression-typing site (returns, local declarations,
// assignments, call arguments), not just returns: the ambiguity is intrinsic to the
// expression's syntax, not to where the expression happens to appear.
func semaResolveConstructedExpressionType(model *semaTypeMemberView, owner, expr string, scope map[string]string) string {
	resolved := resolveNestedTypeReference(model, owner, inferSemaArgTypeWithModel(expr, scope, model))
	match := newExprPattern.FindStringSubmatch(strings.TrimSpace(expr))
	if len(match) != 2 {
		return resolved
	}
	return semaSObjectConstructorPrecedence(model, resolved, match[1], newExprSObjectFieldArgPattern.MatchString(expr))
}

// semaSObjectConstructorPrecedence holds the narrow precedence rule shared by the
// regex-based (semaResolveConstructedExpressionType) and IR-based
// (semaIRSObjectConstructorPrecedence) expression-typing paths: a `new Type(field =
// value, ...)` SObject field-initializer only exists for real SObjects, so a genuine
// standard SObject named bareName takes precedence over a same-named non-SObject nested
// Apex class that ordinary nested-class resolution would otherwise prefer. This does not
// broadly invert nested-class-wins: it only fires when the constructor call used named
// field arguments and the nested-resolved type is not itself an SObject.
func semaSObjectConstructorPrecedence(model *semaTypeMemberView, resolved, bareName string, hasSObjectFieldArgs bool) string {
	if bareName == "" || strings.EqualFold(resolved, bareName) || !hasSObjectFieldArgs {
		return resolved
	}
	standard, ok := model.lookup(normalizeName(bareName))
	if !ok || !standard.sobject {
		return resolved
	}
	if nested, ok := model.lookup(normalizeName(resolved)); ok && nested.sobject {
		return resolved
	}
	return bareName
}

func inferSemaArgTypeWithModel(arg string, scope map[string]string, model *semaTypeMemberView) string {
	if !enterSemaInference(scope) {
		return ""
	}
	defer leaveSemaInference(scope)
	arg = strings.TrimSpace(arg)
	arg = semaTrimSafeNavigationReceiverSuffix(arg)
	if scope != nil && semaInferenceTrackableArg(arg) {
		activeKey := semaInferenceActiveKey(arg)
		if scope[activeKey] != "" {
			return ""
		}
		scope[activeKey] = "1"
		defer delete(scope, activeKey)
	}
	return inferSemaArgTypeWithModelUncached(arg, scope, model)
}

func inferSemaArgTypeWithModelUncached(arg string, scope map[string]string, model *semaTypeMemberView) string {
	arg = semaTrimExpressionPrefix(arg)
	if semaContainsStatementSeparator(arg) {
		return ""
	}
	if semaLooksLikeSObjectFieldStringPropertyPath(arg) {
		return "String"
	}
	if semaLooksLikeSObjectDescribeFieldResultPath(arg) {
		return "Schema.DescribeFieldResult"
	}
	if inner, ok := trimSemaOuterParens(arg); ok {
		return inferSemaArgTypeWithModel(inner, scope, model)
	}
	if condition, whenTrue, whenFalse, ok := splitSemaTernary(strings.TrimSpace(arg)); ok {
		inferSemaArgTypeWithModel(condition, scope, model)
		trueType := inferSemaArgTypeWithModel(whenTrue, scope, model)
		falseType := inferSemaArgTypeWithModel(whenFalse, scope, model)
		return semaCommonType(trueType, falseType, model)
	}
	if left, right, ok := splitSemaBinary(arg, "??"); ok {
		return semaCommonType(inferSemaArgTypeWithModel(left, scope, model), inferSemaArgTypeWithModel(right, scope, model), model)
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
	if typ := inferSemaBinaryTypeWithModel(arg, scope, model); typ != "" {
		return typ
	}
	if castType, _, ok := splitSemaCast(arg); ok {
		if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
			return resolveNestedTypeReference(model, currentType, castType)
		}
		return castType
	}
	if typ := inferSemaMethodCallType(arg, scope, model); typ != "" {
		return typ
	}
	if receiver, ok := splitSemaIndexExpression(arg); ok {
		receiverType := inferSemaArgTypeWithModel(receiver, scope, model)
		if elementType, elementOK := semaIterableElementType(receiverType); elementOK {
			return elementType
		}
	}
	if strings.HasPrefix(arg, "'") {
		return "String"
	}
	if literalType := semaKeywordLiteralType(arg); literalType != "" {
		return literalType
	}
	if decimalLiteralPattern.MatchString(arg) {
		return "Decimal"
	}
	if intLiteralPattern.MatchString(arg) {
		return "Integer"
	}
	if strings.HasSuffix(strings.ToLower(arg), ".class") {
		return "Type"
	}
	if receiver, name, ok := strings.Cut(arg, "."); ok && strings.EqualFold(receiver, "Page") && scope[normalizeName(receiver)] == "" && semaPageReferenceTokenName(name) {
		return "PageReference"
	}
	if typ := inferSemaDescribeFieldChainType(arg, scope, model); typ != "" {
		return typ
	}
	if typ := inferSemaFieldAccessType(arg, scope, model); typ != "" {
		return typ
	}
	if typ := inferSemaArgType(arg, scope); typ != "" {
		return typ
	}
	if simpleIdentifierPattern.MatchString(arg) {
		if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
			if target, ok := semaResolveFieldPath(model, currentType, arg); ok {
				return target.member.Type
			}
		}
	}
	return ""
}

func semaTrimExpressionPrefix(arg string) string {
	arg = strings.TrimSpace(arg)
	for {
		switch {
		case strings.HasPrefix(arg, "return "):
			arg = strings.TrimSpace(strings.TrimPrefix(arg, "return "))
		case strings.HasPrefix(arg, "!"):
			arg = strings.TrimSpace(strings.TrimPrefix(arg, "!"))
		default:
			return arg
		}
	}
}

func semaContainsStatementSeparator(arg string) bool {
	for i := 0; i < len(arg); i++ {
		switch arg[i] {
		case '\'':
			i = skipSemaString(arg, i)
		case '/':
			if end, ok := skipSemaComment(arg, i); ok {
				i = end
			}
		case ';':
			return true
		}
	}
	return false
}

func semaTrimSafeNavigationReceiverSuffix(arg string) string {
	arg = strings.TrimSpace(arg)
	if strings.HasSuffix(arg, "?") {
		return strings.TrimSpace(strings.TrimSuffix(arg, "?"))
	}
	return arg
}

func semaInferenceTrackableArg(arg string) bool {
	return arg != "" && len(arg) <= 2048
}

func semaInferenceActiveKey(arg string) string {
	return "__glade_infer_active:" + arg
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

func inferSemaBinaryTypeWithModel(arg string, scope map[string]string, model *semaTypeMemberView) string {
	for _, op := range []string{"&&", "||"} {
		if left, right, ok := splitSemaBinary(arg, op); ok {
			if strings.EqualFold(inferSemaArgTypeWithModel(left, scope, model), "Boolean") && strings.EqualFold(inferSemaArgTypeWithModel(right, scope, model), "Boolean") {
				return "Boolean"
			}
			return ""
		}
	}
	for _, op := range []string{"<<", ">>", ">>>", "|", "&", "^"} {
		left, right, ok := splitSemaBinary(arg, op)
		if !ok {
			continue
		}
		return semaIntegralResultType(inferSemaArgTypeWithModel(left, scope, model), inferSemaArgTypeWithModel(right, scope, model))
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

func inferSemaDescribeFieldChainType(arg string, scope map[string]string, model *semaTypeMemberView) string {
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

func inferSemaFieldAccessType(expr string, scope map[string]string, model *semaTypeMemberView) string {
	if semaLooksLikeLabelReference(expr) {
		return "String"
	}
	if semaLooksLikeCustomShareRowCauseToken(expr, model) {
		return "String"
	}
	if semaLooksLikeSObjectFieldStringPropertyPath(expr) {
		return "String"
	}
	if receiverExpr, field, ok := splitSemaMethodPath(expr); ok {
		if castType, _, castOK := splitSemaCast(receiverExpr); castOK {
			if currentType := scope[semaCurrentTypeScopeKey]; currentType != "" {
				castType = resolveNestedTypeReference(model, currentType, castType)
			}
			if target, ok := semaResolveFieldPath(model, castType, field); ok {
				return target.member.Type
			}
		}
	}
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) < 2 {
		return ""
	}
	if semaLooksLikeSObjectDescribeFieldResultPath(expr) {
		return "Schema.DescribeFieldResult"
	}
	_, firstPartScoped := scope[normalizeName(parts[0])]
	if !firstPartScoped {
		if target, staticOK := semaStaticClassFieldPathMemberInContext(model, scope[semaCurrentTypeScopeKey], parts[0], strings.Join(parts[1:], ".")); staticOK && !hasModifier(target.member.Modifiers, semaSyntheticStandardSObjectFieldModifier) {
			return target.member.Type
		}
		if semaExprLooksLikeStaticSObjectTokenInModel(expr, scope, model) {
			if semaLooksLikeSObjectFieldTokenInModel(expr, model) {
				return "Schema.SObjectField"
			}
			if semaLooksLikeSObjectTypeTokenInModel(expr, model) {
				return "Schema.SObjectType"
			}
		}
		if typ := semaEnumValuePathType(model, expr); typ != "" {
			return typ
		}
	}
	if receiverExpr, field, ok := splitSemaMethodPath(expr); ok && semaFieldReceiverNeedsInference(receiverExpr) {
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
			if members, ok := model.lookup(normalizeName(currentType)); ok {
				receiverType = members.superClass
				startIndex = 1
			}
		}
	} else if scoped, ok := scope[normalizeName(parts[0])]; ok {
		receiverType = scoped
	} else {
		currentType := scope[semaCurrentTypeScopeKey]
		if resolved := resolveNestedTypeName(model, currentType, parts[0]); resolved != "" {
			if members, ok := model.lookup(normalizeName(resolved)); ok {
				receiverType = members.name
			}
		}
		if receiverType == "" {
			if members, ok := model.lookup(normalizeName(parts[0])); ok {
				if !semaPlatformReceiverSpellingMatches(parts[0], members) {
					return ""
				}
				receiverType = members.name
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
		if fallback := semaFallbackFieldPathType(parts[len(parts)-1]); fallback != "" && !semaUnknownExternalDottedType(receiverType, model) {
			return fallback
		}
	}
	return ""
}

func semaLooksLikeCustomShareRowCauseToken(expr string, model *semaTypeMemberView) bool {
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) != 4 || !strings.EqualFold(parts[0], "Schema") || !strings.EqualFold(parts[2], "RowCause") {
		return false
	}
	if !strings.HasSuffix(normalizeName(parts[1]), "__share") || !semaFieldTokenPart(parts[3]) {
		return false
	}
	_, _, ok := semaLookupTypeMembers(model, parts[1])
	return ok
}

func semaUnknownExternalDottedType(typeName string, model *semaTypeMemberView) bool {
	base, _ := semaGenericBaseAndArgs(strings.TrimSpace(typeName))
	if !strings.Contains(base, ".") {
		return false
	}
	if _, ok := model.lookup(normalizeName(base)); ok {
		return false
	}
	root, _, ok := strings.Cut(base, ".")
	if !ok || root == "" {
		return false
	}
	if _, ok := model.lookup(normalizeName(root)); ok {
		return false
	}
	return true
}

func semaFieldReceiverNeedsInference(receiverExpr string) bool {
	return strings.ContainsAny(receiverExpr, "()")
}

func semaLooksLikeSObjectDescribeFieldResultPath(expr string) bool {
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) == 5 && strings.EqualFold(parts[0], "Schema") && strings.EqualFold(parts[1], "SObjectType") && semaFieldTokenPart(parts[2]) && strings.EqualFold(parts[3], "fields") && semaFieldTokenPart(parts[4]) {
		return true
	}
	if len(parts) == 4 && strings.EqualFold(parts[0], "SObjectType") && semaFieldTokenPart(parts[1]) && strings.EqualFold(parts[2], "fields") && semaFieldTokenPart(parts[3]) {
		return true
	}
	return false
}

func semaLooksLikeSObjectFieldStringPropertyPath(expr string) bool {
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) < 5 || !semaDescribeFieldStringProperty(parts[len(parts)-1]) {
		return false
	}
	for i := 0; i < len(parts); i++ {
		if !strings.EqualFold(parts[i], "SObjectType") {
			continue
		}
		if i+3 < len(parts) &&
			strings.EqualFold(parts[i+1], "fields") &&
			semaFieldTokenPart(parts[i+2]) &&
			semaDescribeFieldStringProperty(parts[i+3]) &&
			i+4 == len(parts) {
			return true
		}
		if i+4 < len(parts) &&
			semaFieldTokenPart(parts[i+1]) &&
			strings.EqualFold(parts[i+2], "fields") &&
			semaFieldTokenPart(parts[i+3]) &&
			semaDescribeFieldStringProperty(parts[i+4]) &&
			i+5 == len(parts) {
			return true
		}
	}
	return false
}

func semaDescribeFieldStringProperty(part string) bool {
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "label", "name", "relationshipname":
		return true
	default:
		return false
	}
}
