package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Captured rows at 62/67/68 on both routes. F is a final local, P/Q a
// parameter/final parameter, S/I an unqualified final static/instance field.
// Each parameter/field row is a sample, so its operator and element type stay
// paired. Consumed final list increments have no native language answer.
func (a *Analyzer) semaFinalCollectionIncrement(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope irSemaScope, start int, source string, model *semaTypeMemberView) (semaIncrementTarget, string, bool) {
	expr := inst.Expr
	if !semaIRIncrementCall(expr) || expr.Left == nil || scope.incrementHeader || typ.Kind != apexast.DeclarationClass || member.Kind != apexast.DeclarationMethod {
		return semaIncrementTarget{}, "", false
	}
	var target semaIncrementTarget
	shape, ok := a.semaFinalCollectionIncrementShape(*expr.Left, typ, member, scope, model, start, source, &target)
	if !ok {
		return target, "", false
	}
	// Anonymous helper methods were captured only with parameter and own
	// final-instance-field roots (033-036,041-044,049-052).
	if hasModifier(typ.Modifiers, vm.AnonymousClassModifier) && !strings.HasPrefix(shape, "P") && !strings.HasPrefix(shape, "Q") && !strings.HasPrefix(shape, "I") {
		return target, "", false
	}
	operand := a.inferIRExprType(*expr.Left, scope, model, typ.Name)
	statement := inst.Op == ir.OpExpr
	if !statement && (inst.Op != ir.OpDeclare || !strings.EqualFold(resolveNestedTypeReference(model, typ.Name, inst.Type), operand)) {
		return target, "", false
	}
	op := expr.Callee
	matched := false
	if statement {
		switch shape {
		case "F[lit]": // 001-008, 025-028, 111-114, 121/123/125/127
			matched = semaWordListContains("Integer Double String", operand) || strings.EqualFold(operand, "Decimal") && op == "__prefix:--"
		case "F[var]": // 103/105/107/109
			matched = semaFinalCollectionSample(operand, op, "Integer", "__postfix:++", "Long", "__prefix:++", "Double", "__postfix:--", "Decimal", "__prefix:--")
		case "F.lget(lit)": // 122/124/126/128
			matched = strings.EqualFold(operand, "Integer")
		case "F.mget(lit):s": // 013-016, 129/130
			// 014/016 need prefix approval for the Integer SObject field;
			// otherwise the statement falls back to the name-only parser.
			matched = strings.EqualFold(operand, "Decimal") || strings.EqualFold(operand, "Integer") && semaWordListContains("__postfix:++ __prefix:++ __prefix:--", op)
		case "L[lit]": // 021-024, 031/032, 047/048
			matched = strings.EqualFold(operand, "Double")
		case "P[lit]": // 035/036
			matched = semaFinalCollectionSample(operand, op, "Double", "__prefix:--", "Decimal", "__postfix:++")
		case "Q[lit]": // 049/050
			matched = semaFinalCollectionSample(operand, op, "Integer", "__prefix:++", "Long", "__postfix:--")
		case "S[lit]": // 039/040 (named only)
			matched = !scope.anonymous && !hasModifier(typ.Modifiers, vm.AnonymousClassModifier) && semaFinalCollectionSample(operand, op, "Double", "__postfix:++", "Decimal", "__prefix:++")
		case "I[lit]": // 043/044
			matched = semaFinalCollectionSample(operand, op, "Double", "__prefix:++", "Decimal", "__postfix:--")
		case "P.mget(lit):s": // 033
			matched = strings.EqualFold(operand, "Integer") && op == "__prefix:++"
		case "P.mget(lit):f": // 034
			matched = strings.EqualFold(operand, "Long") && op == "__postfix:--"
		case "Q.mget(lit):s": // 052
			matched = strings.EqualFold(operand, "Decimal") && op == "__postfix:++"
		case "Q.mget(lit):f": // 051
			matched = strings.EqualFold(operand, "Double") && op == "__prefix:--"
		case "S.mget(lit):s": // 037 (named only)
			matched = !scope.anonymous && !hasModifier(typ.Modifiers, vm.AnonymousClassModifier) && strings.EqualFold(operand, "Integer") && op == "__postfix:--"
		case "S.mget(lit):f": // 038 (named only)
			matched = !scope.anonymous && !hasModifier(typ.Modifiers, vm.AnonymousClassModifier) && strings.EqualFold(operand, "Long") && op == "__prefix:--"
		case "I.mget(lit):s": // 041
			matched = strings.EqualFold(operand, "Integer") && op == "__prefix:--"
		case "I.mget(lit):f": // 042
			matched = strings.EqualFold(operand, "Long") && op == "__postfix:++"
		}
	} else {
		switch shape {
		case "F.mget(lit):f": // 058
			matched = strings.EqualFold(operand, "Long") && op == "__prefix:++"
		case "L": // source syntax and version rules 090/093/096/099
			matched = semaWordListContains("Integer Long Double Decimal", operand)
		}
	}
	target.shape = shape
	return target, operand, matched
}

func semaFinalCollectionSample(typ, op string, pairs ...string) bool {
	for i := 0; i < len(pairs); i += 2 {
		if strings.EqualFold(typ, pairs[i]) && op == pairs[i+1] {
			return true
		}
	}
	return false
}

func semaFinalCollectionRoot(name string, typ typesys.TypeSymbol, member typesys.MemberSymbol, scope irSemaScope, model *semaTypeMemberView) (string, string, bool) {
	if name == "" || strings.ContainsAny(name, ".?") {
		return "", "", false
	}
	binding, ok := scope.binding(name)
	if !ok {
		return "", "", false
	}
	switch binding.origin {
	case irSemaOriginLocal:
		return "L", binding.typ, true
	case irSemaOriginFinalLocal:
		return "F", binding.typ, true
	case irSemaOriginParam:
		for _, p := range member.Parameters {
			if strings.EqualFold(p.Name, name) {
				if hasModifier(p.Modifiers, "final") {
					return "Q", binding.typ, true
				}
				return "P", binding.typ, true
			}
		}
	case irSemaOriginField:
		field, found := semaResolveFieldPath(model, typ.Name, name)
		if !found || field.member.Kind != apexast.DeclarationField || !strings.EqualFold(field.owner, typ.Name) || !hasModifier(field.member.Modifiers, "final") {
			return "", "", false
		}
		if hasModifier(field.member.Modifiers, "static") {
			return "S", binding.typ, true
		}
		if !hasModifier(member.Modifiers, "static") {
			return "I", binding.typ, true
		}
	}
	return "", "", false
}

func (a *Analyzer) semaFinalCollectionIncrementShape(node ir.Expr, typ typesys.TypeSymbol, member typesys.MemberSymbol, scope irSemaScope, model *semaTypeMemberView, start int, source string, target *semaIncrementTarget) (string, bool) {
	if node.Kind == ir.ExprVariable {
		shape, _, ok := semaFinalCollectionRoot(node.Name, typ, member, scope, model)
		return shape, ok
	}
	if node.Kind != ir.ExprCall {
		return "", false
	}
	if node.Left == nil {
		name, method, dotted := strings.Cut(node.Callee, ".")
		root, rootType, ok := semaFinalCollectionRoot(name, typ, member, scope, model)
		if !dotted || method != "get" || !ok || len(node.Args) != 1 || node.Args[0].Kind != ir.ExprLiteral {
			return "", false
		}
		base, _ := semaGenericBaseAndArgs(rootType)
		if strings.EqualFold(base, "List") {
			return root + ".lget(lit)", true
		}
		if strings.EqualFold(base, "Map") {
			return root + ".mget(lit)", true
		}
		return "", false
	}
	left, ok := a.semaFinalCollectionIncrementShape(*node.Left, typ, member, scope, model, start, source, target)
	if !ok {
		return "", false
	}
	receiverType := a.inferIRExprType(*node.Left, scope, model, typ.Name)
	if node.Callee == "get" && node.Operator == "[]" && len(node.Args) == 1 && node.Left.Kind == ir.ExprVariable {
		base, _ := semaGenericBaseAndArgs(receiverType)
		arg, ok := semaIncrementArgument(node.Args[0], scope, model)
		if strings.EqualFold(base, "List") && ok && (arg == "lit" || arg == "var") {
			return left + "[" + arg + "]", true
		}
	}
	if strings.HasPrefix(node.Callee, "__field:") && strings.HasSuffix(left, ".mget(lit)") {
		step, ok := semaIncrementFieldStep(":", receiverType, strings.TrimPrefix(node.Callee, "__field:"), false, typ, member, model, start, source, target)
		return left + step, ok
	}
	return "", false
}

// 068/070/072/074 capture the skipped arm only. They do not authorize
// executing a consumed final list increment (the taken twins are carried).
func (a *Analyzer) semaFinalCollectionSkippedIncrement(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope *irSemaScope, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) ([]diagnostic.Diagnostic, bool) {
	expr := inst.Expr
	if hasModifier(typ.Modifiers, vm.AnonymousClassModifier) {
		return nil, false
	}
	if inst.Op != ir.OpDeclare || expr.Kind != ir.ExprCall || expr.Callee != "__ternary" || len(expr.Args) != 3 || expr.Args[0].Kind != ir.ExprLiteral || expr.Args[0].Value != "false" || expr.Args[2].Kind != ir.ExprLiteral || expr.Args[2].Value != "0" {
		return nil, false
	}
	increment := expr.Args[1]
	if !semaIRIncrementCall(increment) || increment.Left == nil {
		return nil, false
	}
	var target semaIncrementTarget
	shape, ok := a.semaFinalCollectionIncrementShape(*increment.Left, typ, member, *scope, model, bodyOffset+inst.Pos, source, &target)
	operand := a.inferIRExprType(*increment.Left, *scope, model, typ.Name)
	if !ok || shape != "F[lit]" || !strings.EqualFold(inst.Type, operand) || !semaFinalCollectionSample(operand, increment.Callee, "Integer", "__postfix:++", "Long", "__prefix:++", "Double", "__postfix:--", "Decimal", "__prefix:--") {
		return nil, false
	}
	return a.checkIRExprVariables(typ, member, *increment.Left, scope, inst.Pos, bodyOffset, source, model, constructability), true
}

// 085-088 and 115-118 reject unary operators before the VM's statement
// lowering (assignment of a binary +/-) loses the source operator spelling.
func semaFinalCollectionUnaryAssignment(typ typesys.TypeSymbol, inst ir.Instruction, scope irSemaScope, start int, source string) (diagnostic.Diagnostic, bool) {
	if inst.Op != ir.OpAssign || strings.Contains(inst.Name, ".") || inst.Expr.Kind != ir.ExprBinary || (inst.Expr.Operator != "+" && inst.Expr.Operator != "-") || start < 0 || start >= len(source) {
		return diagnostic.Diagnostic{}, false
	}
	binding, ok := scope.binding(inst.Name)
	if !ok {
		return diagnostic.Diagnostic{}, false
	}
	operand := ""
	base, args := semaGenericBaseAndArgs(binding.typ)
	if binding.origin == irSemaOriginFinalLocal && strings.EqualFold(base, "List") && len(args) == 1 && strings.EqualFold(args[0], "Integer") {
		operand = "List<Integer>"
	}
	if binding.origin == irSemaOriginCatch && strings.EqualFold(binding.typ, "Exception") {
		operand = "Exception"
	}
	if operand == "" {
		return diagnostic.Diagnostic{}, false
	}
	end := strings.IndexByte(source[start:], ';')
	if end < 0 {
		return diagnostic.Diagnostic{}, false
	}
	spelling := strings.Join(strings.Fields(sourceSyntaxCodeText(source[start:start+end])), "")
	operator := ""
	switch {
	case strings.EqualFold(spelling, inst.Name+"++"), strings.EqualFold(spelling, inst.Name+"--"):
		operator = "postfix increment/decrement"
	case strings.EqualFold(spelling, "++"+inst.Name):
		operator = "prefix increment"
	case strings.EqualFold(spelling, "--"+inst.Name):
		operator = "prefix decrement"
	default:
		return diagnostic.Diagnostic{}, false
	}
	return typeContractNativeDiagnostic(typ, "Unary "+operator+" can only be applied to numeric expressions: "+operand, start, start+end, source), true
}

func (s *irSemaScope) markFinalCollectionCatch(name string) {
	frame := s.frames[len(s.frames)-1]
	key := normalizeName(name)
	binding := frame[key]
	binding.origin = irSemaOriginCatch
	frame[key] = binding
}
