package sema

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

var (
	typeContractRawCollectionConstructor = regexp.MustCompile(`(?i)\bnew\s+(list|map|set)\s*\(\s*\)`)
	typeContractScientificLiteral        = regexp.MustCompile(`\b\d+(?:\.\d+)?[eE][+-]?\d+\b`)
	typeContractIntegerLiteral           = regexp.MustCompile(`\b\d{10,}\b`)
	typeContractSafeAssignment           = regexp.MustCompile(`\?\.\s*[A-Za-z_][A-Za-z0-9_]*\s*=(?:[^=]|$)`)
)

func (a *Analyzer) checkSourceTypeContracts(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	seenSources := make(map[string]bool)
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		for _, member := range typ.Members {
			diagnostics = append(diagnostics, typeContractTypeDiagnostics(typ, member, member.Type)...)
			for _, parameter := range member.Parameters {
				diagnostics = append(diagnostics, typeContractTypeDiagnostics(typ, member, parameter.Type)...)
			}
		}
		if a.sources == nil {
			continue
		}
		sourceKey := semaSourceCacheKey(typ.File, typ.Namespace, typ.SourceNamespaceRemaps)
		if seenSources[sourceKey] {
			continue
		}
		facts, ok := a.sources.factsForType(typ)
		if !ok {
			continue
		}
		seenSources[sourceKey] = true
		source := facts.sourceText()
		spans := facts.codeSpans()
		for _, match := range typeContractRawCollectionConstructor.FindAllStringIndex(source, -1) {
			if !spans.contains(match[0]) {
				continue
			}
			diagnostics = append(diagnostics, typeContractDiagnostic(typ, typesys.MemberSymbol{}, "raw collection construction requires type arguments", match[0], match[1], source))
		}
		for _, match := range typeContractScientificLiteral.FindAllStringIndex(source, -1) {
			if !spans.contains(match[0]) {
				continue
			}
			diagnostics = append(diagnostics, typeContractDiagnostic(typ, typesys.MemberSymbol{}, "scientific notation is not valid Apex numeric literal syntax", match[0], match[1], source))
		}
		for _, match := range typeContractIntegerLiteral.FindAllStringIndex(source, -1) {
			if !spans.contains(match[0]) {
				continue
			}
			if match[0] > 0 && source[match[0]-1] == '.' || match[1] < len(source) && source[match[1]] == '.' {
				continue
			}
			literal := source[match[0]:match[1]]
			value, err := strconv.ParseInt(literal, 10, 64)
			if err != nil || value > 2147483647 {
				diagnostics = append(diagnostics, typeContractDiagnostic(typ, typesys.MemberSymbol{}, "unsuffixed integer literal exceeds the Integer range", match[0], match[1], source))
			}
		}
		for _, match := range typeContractSafeAssignment.FindAllStringIndex(source, -1) {
			if !spans.contains(match[0]) {
				continue
			}
			diagnostics = append(diagnostics, typeContractDiagnostic(typ, typesys.MemberSymbol{}, "safe navigation cannot be an assignment target", match[0], match[1], source))
		}
	}
	return diagnostics
}

func typeContractTypeDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol, typeName string) []diagnostic.Diagnostic {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.EqualFold(typeName, "void") {
		return nil
	}
	base, args := semaGenericBaseAndArgs(typeName)
	var diagnostics []diagnostic.Diagnostic
	appendDiagnostic := func(detail string) {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA019",
			Message:  fmt.Sprintf("%s", detail),
			File:     typ.File,
			Range:    &member.Range,
		})
	}
	if strings.EqualFold(base, "Currency") {
		appendDiagnostic("Currency is not a source-level Apex type")
	}
	expectedArgs := 0
	switch strings.ToLower(base) {
	case "list", "set", "iterable", "iterator":
		expectedArgs = 1
	case "map":
		expectedArgs = 2
	}
	if expectedArgs != 0 {
		if len(args) == 0 {
			appendDiagnostic(fmt.Sprintf("raw %s type requires type arguments", base))
		} else if len(args) != expectedArgs {
			appendDiagnostic(fmt.Sprintf("%s requires %d type argument(s)", base, expectedArgs))
		}
	}
	if typeContractCollectionDepth(typeName) > 8 {
		appendDiagnostic("collection type nesting exceeds the supported Apex limit")
	}
	return diagnostics
}

func typeContractCollectionDepth(typeName string) int {
	base, args := semaGenericBaseAndArgs(typeName)
	depth := 0
	switch strings.ToLower(base) {
	case "list", "set", "map":
		depth = 1
	}
	for _, argument := range args {
		if nested := typeContractCollectionDepth(argument) + depth; nested > depth {
			depth = nested
		}
	}
	return depth
}

func typeContractDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, detail string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA019",
		Message:  fmt.Sprintf("%s has invalid source contract: %s", typ.Name, detail),
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

func typeContractNativeDiagnostic(typ typesys.TypeSymbol, message string, start, end int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA019",
		Message:  message,
		File:     typ.File,
		Range:    semaRange(source, start, end),
	}
}

// Async marker-interface and Object/same-type controls reject these
// statically certain tests, including the captured async marker interfaces.
func semaInstanceofAlwaysTrueMessage(left, target string, model *semaTypeMemberView) string {
	if left == "" {
		return ""
	}
	always := strings.EqualFold(left, target) || strings.EqualFold(semaCanonicalPlatformAlias(target), "Object")
	// Platform event records are statically SObjects.
	if strings.EqualFold(target, "SObject") && semaPlatformEventType(left, model) {
		always = true
	}
	marker := strings.EqualFold(target, "Database.Stateful") || strings.EqualFold(target, "Database.AllowsCallouts")
	if !always && marker && !semaProjectTypeShadowsPlatform(model, target) {
		always = semaTypeMatches(model, left, target, make(map[string]bool))
	}
	if !always {
		return ""
	}
	return fmt.Sprintf("Operation instanceof is always true since an instance of %s is always an instance of %s", left, target)
}

// Salesforce reports an own static getter write introduced after API 41 as a
// visibility failure, even though the underlying contract is a read-only
// property. Keep the structured local contract code while preserving that
// source-compatible diagnostic wording for the exact boundary.
func typeContractPropertyAssignmentDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, target resolvedMember, unqualified bool, start, end int, source string) diagnostic.Diagnostic {
	if unqualified && !apexversion.Before(typ.EffectiveAPIVersion, 42) &&
		strings.EqualFold(target.owner, typ.Name) &&
		strings.EqualFold(member.Name, target.member.Name+".get") &&
		hasModifier(member.Modifiers, "static") && hasModifier(target.member.Modifiers, "static") {
		return diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA019",
			Message:  fmt.Sprintf("Variable is not visible: %s.%s", target.owner, target.member.Name),
			File:     typ.File,
			Range:    semaRange(source, start, end),
		}
	}
	d := typeContractDiagnostic(typ, member, "property has no setter", start, end, source)
	d.NativeMessage = "Variable is not visible: " + target.owner + "." + target.member.Name
	return d
}

func (a *Analyzer) checkIRExpressionContract(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	var walk func(ir.Expr)
	appendDiagnostic := func(detail string) {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESEMA019",
			Message:  fmt.Sprintf("%s %q has invalid expression: %s", member.Kind, member.Name, detail),
			File:     typ.File,
			Range:    semaRange(source, bodyOffset+pos, bodyOffset+pos+1),
		})
	}
	// C037/P001/P004: a setter-only property has no readable value, even
	// within its declaring type. Preserve the local detail and native text.
	appendPropertyReadDiagnostic := func(target resolvedMember) {
		appendDiagnostic("property has no getter")
		diagnostics[len(diagnostics)-1].NativeMessage = "Variable is not visible: " + target.owner + "." + target.member.Name
	}
	compatible := func(left, right string) bool {
		return left == "" || right == "" || strings.EqualFold(left, "null") || strings.EqualFold(right, "null") ||
			semaAssignableToType(left, right, model) || semaAssignableToType(right, left, model) ||
			(isSemaNumericType(left) && isSemaNumericType(right))
	}
	runtimeCompatible := func(left, right string) bool {
		return compatible(left, right) || semaRuntimeTypeTestCompatible(typ.Name, left, right, model)
	}
	walk = func(current ir.Expr) {
		if current.Left != nil {
			walk(*current.Left)
		}
		if current.Right != nil && !strings.EqualFold(current.Operator, "instanceof") {
			walk(*current.Right)
		}
		for _, arg := range current.Args {
			walk(arg)
		}
		for _, arg := range current.NamedArgs {
			walk(arg.Expr)
		}
		switch current.Kind {
		case ir.ExprUnary:
			if current.Left == nil {
				return
			}
			operand := a.inferIRExprType(*current.Left, scope, model, typ.Name)
			switch current.Operator {
			case "~":
				if operand != "" && !strings.EqualFold(operand, "Integer") && !strings.EqualFold(operand, "Long") {
					appendDiagnostic("operator ~ requires an Integer or Long operand")
				}
			case "!":
				if operand != "" && !strings.EqualFold(operand, "Boolean") {
					appendDiagnostic("operator ! requires a Boolean operand")
				}
			case "+", "-":
				if operand != "" && !isSemaNumericType(operand) && !(current.Operator == "+" && (strings.EqualFold(operand, "String") || strings.EqualFold(operand, "Id"))) {
					appendDiagnostic("unary numeric operator requires a numeric operand")
				}
			}
		case ir.ExprBinary:
			if current.Left == nil || current.Right == nil {
				return
			}
			left := a.inferIRExprType(*current.Left, scope, model, typ.Name)
			right := a.inferIRExprType(*current.Right, scope, model, typ.Name)
			switch current.Operator {
			case "*", "/", "%", "-":
				if left != "" && right != "" && !semaDateDayArithmetic(current.Operator, left, right) && (!isSemaNumericType(left) || !isSemaNumericType(right)) {
					appendDiagnostic("arithmetic operator requires numeric operands")
				}
			case "+":
				if left != "" && right != "" && !semaDateDayArithmetic(current.Operator, left, right) && !isSemaNumericType(left) && !isSemaNumericType(right) && !strings.EqualFold(left, "String") && !strings.EqualFold(right, "String") {
					appendDiagnostic("operator + requires numeric or String operands")
				}
			case "&", "|", "^":
				booleanPair := strings.EqualFold(left, "Boolean") && strings.EqualFold(right, "Boolean")
				if left != "" && right != "" && !booleanPair && (!isSemaIntegralType(left) || !isSemaIntegralType(right)) {
					appendDiagnostic("bitwise operator requires Integer or Long operands")
					if current.Operator == "&" {
						diagnostics[len(diagnostics)-1].NativeMessage = "& operator can only be applied to Boolean expressions or to Integer or Long expressions"
					}
				}
			case "<", "<=", ">", ">=":
				if left != "" && right != "" && !semaOrderablePrimitivePair(left, right) && (!isSemaNumericType(left) || !isSemaNumericType(right)) {
					appendDiagnostic("ordering operator requires numeric operands")
				}
			case "instanceof":
				target := strings.TrimSpace(current.Right.Name)
				if strings.EqualFold(left, "null") {
					// C006: a null literal is rejected at compile time; typed null
					// variables remain valid runtime tests.
					appendDiagnostic("instanceof comparison is always true for null literal")
				} else if left != "" && target != "" && !runtimeCompatible(left, target) {
					appendDiagnostic("instanceof comparison is impossible")
				} else if typeUsesAPIVersionAtLeast(typ, 60) && semaNestedIterableInstanceofAlwaysTrue(left, target, typ.Name, model) {
					appendDiagnostic("instanceof comparison is always true")
				}
			}
		case ir.ExprCall:
			// Native Map bracket reads are invalid; Map.get remains a method call.
			if current.Operator == "[]" && current.Left != nil {
				receiverType := a.inferIRExprType(*current.Left, scope, model, typ.Name)
				base, _ := semaGenericBaseAndArgs(receiverType)
				if strings.EqualFold(base, "Map") {
					diagnostics = append(diagnostics, typeContractNativeDiagnostic(typ, "Expression must be a list type: "+receiverType, bodyOffset+pos, bodyOffset+pos+1, source))
				}
			}
			if (strings.HasPrefix(current.Callee, "__safe_field:") || strings.HasPrefix(current.Callee, "__safe_call:")) && current.Left != nil {
				if semaIRExprLooksLikeTypeReceiver(*current.Left, scope, model) {
					appendDiagnostic("safe navigation cannot use a static receiver")
				}
			}
			if strings.HasPrefix(current.Callee, "__assignField:") && current.Left != nil {
				if strings.HasPrefix(current.Left.Callee, "__safe_field:") {
					appendDiagnostic("safe navigation cannot be an assignment target")
				}
				receiverType := a.inferIRExprType(*current.Left, scope, model, typ.Name)
				field := strings.TrimPrefix(current.Callee, "__assignField:")
				if target, ok := semaResolveFieldPath(model, receiverType, field); ok && target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyAssignmentAllowed(typ, member, target, false, semaIRExprLooksLikeTypeReceiver(*current.Left, scope, model), model) {
					appendDiagnostic("property has no setter")
				}
			}
			if strings.HasPrefix(current.Callee, "__field:") && current.Left != nil {
				receiverType := a.inferIRExprType(*current.Left, scope, model, typ.Name)
				field := strings.TrimPrefix(current.Callee, "__field:")
				if target, ok := semaResolveFieldPath(model, receiverType, field); ok && target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyHasAccessor(target.member, "get") {
					appendPropertyReadDiagnostic(target)
				}
			}
			if strings.HasPrefix(current.Callee, "__cast:") && len(current.Args) == 1 {
				target := strings.TrimPrefix(current.Callee, "__cast:")
				value := a.inferIRExprType(current.Args[0], scope, model, typ.Name)
				if value != "" && !runtimeCompatible(target, value) {
					if message, reports := semaReportsCastMessage(target, value, model); reports {
						diagnostics = append(diagnostics, semaReportsDiagnostic(typ, "GLADESEMA019", message, bodyOffset+pos, bodyOffset+pos+1, source))
					} else {
						appendDiagnostic("cast is incompatible with its operand")
						if target == "Integer" && value == "String" && !semaProjectTypeShadowsPlatform(model, target) && !semaProjectTypeShadowsPlatform(model, value) {
							diagnostics[len(diagnostics)-1].NativeMessage = "Incompatible types since an instance of String is never an instance of Integer"
						}
					}
				}
			}
			if strings.EqualFold(current.Callee, "__coalesce") && len(current.Args) == 2 {
				left := a.inferIRExprType(current.Args[0], scope, model, typ.Name)
				right := a.inferIRExprType(current.Args[1], scope, model, typ.Name)
				if !compatible(left, right) && !semaCoalesceSOQLSingletonAssignable(current.Args[0], left, right, model) {
					appendDiagnostic("coalesce operands do not share a compatible type")
				}
			}
		case ir.ExprVariable:
			if target, ok := semaResolveFieldPath(model, typ.Name, current.Name); ok && target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyHasAccessor(target.member, "get") {
				appendPropertyReadDiagnostic(target)
			}
		}
	}
	walk(expr)
	return diagnostics
}

func semaOrderablePrimitivePair(left, right string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	if (left == "date" || left == "datetime") && (right == "date" || right == "datetime") {
		return true
	}
	if left != right {
		return false
	}
	switch left {
	case "time", "string", "id":
		return true
	default:
		return false
	}
}

func semaDateDayArithmetic(operator, left, right string) bool {
	if operator != "+" && operator != "-" {
		return false
	}
	return (strings.EqualFold(left, "Date") || strings.EqualFold(left, "Datetime")) &&
		strings.EqualFold(right, "Integer")
}

func semaRuntimeTypeTestCompatible(owner, left, right string, model *semaTypeMemberView) bool {
	left = semaCanonicalAssignableType(resolveNestedTypeReference(model, owner, left))
	right = semaCanonicalAssignableType(resolveNestedTypeReference(model, owner, right))
	leftBase, leftArgs := semaGenericBaseAndArgs(left)
	rightBase, rightArgs := semaGenericBaseAndArgs(right)
	leftIterable := strings.EqualFold(leftBase, "Iterable") && len(leftArgs) == 1
	rightIterable := strings.EqualFold(rightBase, "Iterable") && len(rightArgs) == 1
	leftQueryLocator := strings.EqualFold(leftBase, "Database.QueryLocator") && len(leftArgs) == 0
	rightQueryLocator := strings.EqualFold(rightBase, "Database.QueryLocator") && len(rightArgs) == 0
	if (leftIterable && rightQueryLocator) || (leftQueryLocator && rightIterable) {
		return true
	}
	if len(leftArgs) > 0 || len(rightArgs) > 0 {
		if len(leftArgs) == 0 || len(leftArgs) != len(rightArgs) || !strings.EqualFold(leftBase, rightBase) {
			return false
		}
		for i := range leftArgs {
			if strings.EqualFold(leftArgs[i], rightArgs[i]) ||
				semaAssignableToType(leftArgs[i], rightArgs[i], model) ||
				semaAssignableToType(rightArgs[i], leftArgs[i], model) {
				continue
			}
			if !semaRuntimeTypeTestCompatible(owner, leftArgs[i], rightArgs[i], model) {
				return false
			}
		}
		return true
	}
	leftMembers, leftOK := model.lookup(normalizeName(left))
	rightMembers, rightOK := model.lookup(normalizeName(right))
	if !leftOK || !rightOK {
		return true
	}
	// An Apex class need not implement these platform
	// marker interfaces for the runtime test to compile and return false.
	if !leftMembers.platform && !leftMembers.sobject && leftMembers.kind == apexast.DeclarationClass &&
		rightMembers.platform && rightMembers.kind == apexast.DeclarationInterface &&
		(strings.EqualFold(right, "Database.Stateful") || strings.EqualFold(right, "Database.AllowsCallouts")) {
		return true
	}
	if leftMembers.kind == apexast.DeclarationInterface {
		return rightMembers.kind == apexast.DeclarationInterface ||
			hasModifier(rightMembers.modifiers, "abstract") ||
			hasModifier(rightMembers.modifiers, "virtual") ||
			semaAssignableToType(left, right, model) ||
			semaRuntimeTypesShareImplementation(left, right, model)
	}
	if rightMembers.kind == apexast.DeclarationInterface {
		return leftMembers.kind == apexast.DeclarationInterface ||
			hasModifier(leftMembers.modifiers, "abstract") ||
			hasModifier(leftMembers.modifiers, "virtual") ||
			semaAssignableToType(right, left, model) ||
			semaRuntimeTypesShareImplementation(left, right, model)
	}
	return false
}

// R137: a known subclass can implement an interface even when its base does
// not. That concrete implementation witnesses a possible runtime comparison.
func semaRuntimeTypesShareImplementation(left, right string, model *semaTypeMemberView) bool {
	if model == nil || model.state == nil || model.state.base == nil {
		return false
	}
	for _, members := range []map[string]typeMembers{model.current, model.state.base.members} {
		for key := range members {
			candidate, ok := model.lookup(key)
			// Sema takes the destination first, unlike VM assignability.
			if ok && !candidate.platform && candidate.kind == apexast.DeclarationClass &&
				semaAssignableToType(left, candidate.name, model) && semaAssignableToType(right, candidate.name, model) {
				return true
			}
		}
	}
	return false
}

func semaNestedIterableInstanceofAlwaysTrue(left, target, owner string, model *semaTypeMemberView) bool {
	leftBase, leftArgs := semaGenericBaseAndArgs(left)
	targetBase, targetArgs := semaGenericBaseAndArgs(target)
	if len(leftArgs) != 1 || len(targetArgs) != 1 || !strings.EqualFold(leftBase, "List") || !strings.EqualFold(targetBase, "Iterable") {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(targetArgs[0]), "SObject") {
		return false
	}
	targetArgument := resolveNestedTypeName(model, owner, targetArgs[0])
	if !strings.Contains(targetArgument, ".") {
		return false
	}
	targetMembers, ok := model.lookup(normalizeName(targetArgument))
	if !ok || targetMembers.dependency || targetMembers.nestingDepth == 0 {
		return false
	}
	return semaAssignableToType(targetArgument, leftArgs[0], model)
}

func typeContractPropertyAssignmentAllowed(typ typesys.TypeSymbol, member typesys.MemberSymbol, target resolvedMember, unqualified, typeReceiver bool, model *semaTypeMemberView) bool {
	if typeContractPropertyHasAccessor(target.member, "set") {
		return true
	}
	// Legacy callers can replace another component's static getter value only
	// when both component versions predate API 42. Resolve the declaring version
	// from the same member model that supplied the property, including inheritance.
	if typeReceiver && !strings.EqualFold(target.owner, typ.Name) &&
		hasModifier(target.member.Modifiers, "static") && apexversion.Before(typ.EffectiveAPIVersion, 42) {
		if owner, ok := model.lookup(normalizeName(target.owner)); ok && apexversion.Before(owner.effectiveAPIVersion, 42) {
			return true
		}
	}
	// Before API 42, a static getter can initialize its own backing value through
	// an unqualified assignment. The existing accessor body context retains the
	// property name as "property.get" and its declaring component API version.
	return unqualified && apexversion.Before(typ.EffectiveAPIVersion, 42) &&
		strings.EqualFold(target.owner, typ.Name) &&
		strings.EqualFold(member.Name, target.member.Name+".get") &&
		hasModifier(member.Modifiers, "static") && hasModifier(target.member.Modifiers, "static")
}

func typeContractPropertyHasAccessor(member typesys.MemberSymbol, kind string) bool {
	if len(member.Accessors) == 0 {
		return true
	}
	for _, accessor := range member.Accessors {
		if strings.EqualFold(accessor.Kind, kind) {
			return true
		}
	}
	return false
}
