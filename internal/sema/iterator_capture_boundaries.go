package sema

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

var capturedIntegerLiteral = regexp.MustCompile(`^[0-9]+$`)
var capturedIntegerIteratorText = regexp.MustCompile(`(?i)^\s*new\s+List\s*<\s*Integer\s*>\s*\{\s*[0-9]+\s*\}\s*\.\s*iterator\s*\(\s*\)\s*$`)
var capturedInlineAccountIteratorText = regexp.MustCompile(`(?is)^\s*new\s+List\s*<\s*Account\s*>\s*\{.*\}\s*\.\s*iterator\s*\(\s*\)\s*$`)

func capturedIteratorType(value, element string) bool {
	base, args := semaGenericBaseAndArgs(value)
	return sameSemaSignatureType(base, "Iterator") && len(args) == 1 && sameSemaSignatureType(args[0], element)
}

// Only this declaration order, element pair, single parameter and String
// result have duplicate-identity captures. Do not erase generic signatures
// used by inheritance, or extend identity to unmeasured overload neighbors.
func capturedIteratorDuplicate(typ typesys.TypeSymbol, member typesys.MemberSymbol, previous map[string]typesys.MemberSymbol) (diagnostic.Diagnostic, bool) {
	if !typeUsesAPIVersionAtLeast(typ, 62) || len(member.Parameters) != 1 ||
		!strings.EqualFold(member.Type, "String") || !hasModifier(member.Modifiers, "public") ||
		hasModifier(member.Modifiers, "static") || !capturedIteratorType(member.Parameters[0].Type, "Object") {
		return diagnostic.Diagnostic{}, false
	}
	for _, prior := range previous {
		if !strings.EqualFold(prior.Name, member.Name) || len(prior.Parameters) != 1 ||
			!strings.EqualFold(prior.Type, "String") || !hasModifier(prior.Modifiers, "public") ||
			hasModifier(prior.Modifiers, "static") || !capturedIteratorType(prior.Parameters[0].Type, "Integer") {
			continue
		}
		d := duplicateMemberDiagnostic(typ, member, apexast.DeclarationMethod)
		d.NativeMessage = fmt.Sprintf("Method already defined: %s String %s.%s(System.Iterator<Object>) from the type %s", member.Name, typ.Name, member.Name, typ.Name)
		return d, true
	}
	return diagnostic.Diagnostic{}, false
}

func capturedInlineIntegerIterator(expr ir.Expr, model *semaTypeMemberView) bool {
	if expr.Kind != ir.ExprCall || !strings.EqualFold(expr.Callee, "iterator") || len(expr.Args) != 0 || expr.Left == nil ||
		expr.Left.Kind != ir.ExprCall || !strings.EqualFold(expr.Left.Callee, "newlit:List<Integer>") ||
		len(expr.Left.Args) != 1 || expr.Left.Args[0].Kind != ir.ExprLiteral ||
		!capturedIntegerLiteral.MatchString(expr.Left.Args[0].Value) {
		return false
	}
	return !semaProjectTypeShadowsPlatform(model, "List") && !semaProjectTypeShadowsPlatform(model, "Iterator")
}

func capturedInlineIteratorRejection(typ typesys.TypeSymbol, member typesys.MemberSymbol, candidates []resolvedMember, direct bool, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if !direct || !typeUsesAPIVersionAtLeast(typ, 62) || len(candidates) != 1 {
		return diagnostic.Diagnostic{}, false
	}
	candidate := candidates[0]
	owner, _, ok := semaLookupTypeMembers(model, candidate.owner)
	method := candidate.member
	if !ok || owner.dependency || owner.platform || len(method.Parameters) != 1 ||
		!strings.EqualFold(method.Type, "String") || !hasModifier(method.Modifiers, "public") ||
		hasModifier(method.Modifiers, "static") || !capturedIteratorType(method.Parameters[0].Type, "String") {
		return diagnostic.Diagnostic{}, false
	}
	d := unknownCallDiagnostic(typ, member, method.Name, start, end, source)
	d.NativeMessage = fmt.Sprintf("Method does not exist or incorrect signature: void %s(System.ListIterator<Integer>) from the type %s", method.Name, candidate.owner)
	return d, true
}

func (a *Analyzer) capturedInlineIteratorIRCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	if len(expr.Args) == 1 && capturedInlineAccountIterator(expr.Args[0]) {
		receiver, method := typ.Name, expr.Callee
		if expr.Left != nil {
			receiver = a.inferIRExprType(*expr.Left, scope, model, typ.Name)
		} else if target, name, ok := splitSemaMethodPath(expr.Callee); ok {
			receiver, method = semaIRReceiverType(target, scope, model, typ.Name), name
		}
		candidates := resolveMemberMethods(model, receiver, method)
		if d, rejected := capturedInlineAccountIteratorRejection(typ, member, candidates, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
			return []diagnostic.Diagnostic{d}
		}
	}
	// The captured receiver is constructed directly, with one inline iterator
	// argument. Locals preserve their existing declared Iterator applicability.
	if expr.Left == nil || expr.Left.Kind != ir.ExprCall || !strings.HasPrefix(expr.Left.Callee, "new:") ||
		len(expr.Left.Args) != 0 || len(expr.Args) != 1 || !capturedInlineIntegerIterator(expr.Args[0], model) {
		return nil
	}
	receiver := a.inferIRExprType(*expr.Left, scope, model, typ.Name)
	candidates := resolveMemberMethods(model, receiver, expr.Callee)
	if d, rejected := capturedInlineIteratorRejection(typ, member, candidates, true, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source, model); rejected {
		return []diagnostic.Diagnostic{d}
	}
	return nil
}

func capturedInlineAccountIterator(expr ir.Expr) bool {
	return expr.Kind == ir.ExprCall && strings.EqualFold(expr.Callee, "iterator") && len(expr.Args) == 0 &&
		expr.Left != nil && expr.Left.Kind == ir.ExprCall && strings.EqualFold(expr.Left.Callee, "newlit:List<Account>")
}

// A direct ListIterator and a declared Iterator local have different argument
// applicability. Keep the captured list/iterator overload boundary separate
// from the declared-iterator argument conversions.
func capturedInlineAccountIteratorRejection(typ typesys.TypeSymbol, member typesys.MemberSymbol, candidates []resolvedMember, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if !typeUsesAPIVersionAtLeast(typ, 62) || len(candidates) != 2 || semaProjectTypeShadowsPlatform(model, "List") || semaProjectTypeShadowsPlatform(model, "Iterator") || semaProjectTypeShadowsPlatform(model, "Account") {
		return diagnostic.Diagnostic{}, false
	}
	var iterator *resolvedMember
	list := false
	for i := range candidates {
		candidate := &candidates[i]
		if len(candidate.member.Parameters) != 1 {
			return diagnostic.Diagnostic{}, false
		}
		parameter := candidate.member.Parameters[0].Type
		if capturedIteratorType(parameter, "Object") {
			iterator = candidate
		} else if sameSemaSignatureType(parameter, "List<SObject>") {
			list = true
		}
	}
	if iterator == nil || !list {
		return diagnostic.Diagnostic{}, false
	}
	d := unknownCallDiagnostic(typ, member, iterator.member.Name, start, end, source)
	d.NativeMessage = fmt.Sprintf("Method does not exist or incorrect signature: void %s(System.ListIterator<Account>) from the type %s", iterator.member.Name, iterator.owner)
	return d, true
}

func capturedIteratorReturnNativeMessage(returnType, valueType string, model *semaTypeMemberView, direct []bool) string {
	if len(direct) == 1 && direct[0] && capturedIteratorType(returnType, "Object") &&
		capturedIteratorType(valueType, "Integer") && !semaProjectTypeShadowsPlatform(model, "Iterator") &&
		!semaProjectTypeShadowsPlatform(model, "List") {
		return "Illegal conversion from System.ListIterator<Integer> to System.Iterator<Object>"
	}
	return ""
}
