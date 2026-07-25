package sema

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

var (
	typeContractRawCollectionConstructor = regexp.MustCompile(`\bnew\s+(List|Map|Set)\s*\(\s*\)`)
	typeContractScientificLiteral        = regexp.MustCompile(`\b\d+(?:\.\d+)?[eE][+-]?\d+\b`)
	typeContractIntegerLiteral           = regexp.MustCompile(`\b\d{10,}\b`)
	typeContractSafeAssignment           = regexp.MustCompile(`\?\.\s*[A-Za-z_][A-Za-z0-9_]*\s*=`)
)

func (a *Analyzer) checkSourceTypeContracts(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
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
		source, ok := a.sources.normalizedForType(typ)
		if !ok {
			continue
		}
		spans := newSemaCodeSpans(source)
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
	compatible := func(left, right string) bool {
		return left == "" || right == "" || strings.EqualFold(left, "null") || strings.EqualFold(right, "null") ||
			semaAssignableToType(left, right, model) || semaAssignableToType(right, left, model) ||
			(isSemaNumericType(left) && isSemaNumericType(right))
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
			case "!":
				if operand != "" && !strings.EqualFold(operand, "Boolean") {
					appendDiagnostic("operator ! requires a Boolean operand")
				}
			case "+", "-":
				if operand != "" && !isSemaNumericType(operand) {
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
				if left != "" && right != "" && (!isSemaNumericType(left) || !isSemaNumericType(right)) {
					appendDiagnostic("arithmetic operator requires numeric operands")
				}
			case "+":
				if left != "" && right != "" && !isSemaNumericType(left) && !isSemaNumericType(right) && !strings.EqualFold(left, "String") && !strings.EqualFold(right, "String") {
					appendDiagnostic("operator + requires numeric or String operands")
				}
			case "<", "<=", ">", ">=":
				dateLike := (strings.EqualFold(left, "Date") && strings.EqualFold(right, "Date")) || (strings.EqualFold(left, "Datetime") && strings.EqualFold(right, "Datetime"))
				if left != "" && right != "" && !dateLike && (!isSemaNumericType(left) || !isSemaNumericType(right)) {
					appendDiagnostic("ordering operator requires numeric operands")
				}
			case "instanceof":
				target := strings.TrimSpace(current.Right.Name)
				if left != "" && target != "" && (semaAssignableToType(target, left, model) || !compatible(left, target)) {
					appendDiagnostic("instanceof comparison is always true or impossible")
				}
			}
		case ir.ExprCall:
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
				if target, ok := semaResolveFieldPath(model, receiverType, field); ok && target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyHasAccessor(target.member, "set") {
					appendDiagnostic("property has no setter")
				}
			}
			if strings.HasPrefix(current.Callee, "__field:") && current.Left != nil {
				receiverType := a.inferIRExprType(*current.Left, scope, model, typ.Name)
				field := strings.TrimPrefix(current.Callee, "__field:")
				if target, ok := semaResolveFieldPath(model, receiverType, field); ok && target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyHasAccessor(target.member, "get") {
					appendDiagnostic("property has no getter")
				}
			}
			if strings.HasPrefix(current.Callee, "__cast:") && len(current.Args) == 1 {
				target := strings.TrimPrefix(current.Callee, "__cast:")
				value := a.inferIRExprType(current.Args[0], scope, model, typ.Name)
				if value != "" && !compatible(target, value) {
					appendDiagnostic("cast is incompatible with its operand")
				}
			}
			if strings.EqualFold(current.Callee, "__coalesce") && len(current.Args) == 2 {
				left := a.inferIRExprType(current.Args[0], scope, model, typ.Name)
				right := a.inferIRExprType(current.Args[1], scope, model, typ.Name)
				if !compatible(left, right) {
					appendDiagnostic("coalesce operands do not share a compatible type")
				}
			}
		case ir.ExprVariable:
			if target, ok := semaResolveFieldPath(model, typ.Name, current.Name); ok && target.member.Kind == apexast.DeclarationProperty && !typeContractPropertyHasAccessor(target.member, "get") {
				appendDiagnostic("property has no getter")
			}
		}
	}
	walk(expr)
	return diagnostics
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
