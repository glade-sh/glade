package vm

import (
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/ir"
)

// A prefix statement with a local index, and a bare List.get literal target,
// must reach semantic rejection.
// This only exposes candidates: sema still approves each type/binding/operator.
func finalCollectionPrefixCandidate(target ir.Expr, name string, mapGet, field bool) bool {
	if mapGet {
		if field || target.Kind != ir.ExprCall || target.Callee != name || target.Left != nil || len(target.Args) != 1 || target.Args[0].Kind != ir.ExprLiteral {
			return false
		}
		_, err := strconv.ParseInt(target.Args[0].Value, 10, 64)
		return err == nil
	}
	return target.Kind == ir.ExprCall && target.Callee == "get" && target.Operator == "[]" && len(target.Args) == 1 &&
		target.Left != nil && target.Left.Kind == ir.ExprVariable && !strings.Contains(target.Left.Name, ".") &&
		target.Args[0].Kind == ir.ExprVariable && !strings.ContainsAny(target.Args[0].Name, ".?")
}

// The captured rows reject the final numeric List<T>[] declaration,
// before any nested increment. Primitive arrays and List<List<T>> stay intact.
func (p *parser) finalCollectionArrayDeclaration() *ApexSyntaxError {
	if !p.peek(tokenIdent, "final") {
		return nil
	}
	look := *p
	start := look.advance()
	typ, err := look.parseTypeName()
	if err != nil {
		return nil
	}
	switch typ {
	case "List<Integer>[]", "List<Long>[]", "List<Double>[]", "List<Decimal>[]":
	default:
		return nil
	}
	if !look.peek(tokenIdent, "") {
		return nil
	}
	look.advance()
	if !look.match(tokenSymbol, "=") || !look.match(tokenIdent, "new") {
		return nil
	}
	initial, err := look.parseTypeName()
	if err != nil || initial != typ || !look.peek(tokenSymbol, "{") {
		return nil
	}
	end := start.pos
	for i := look.pos; i < len(look.tokens)-1; i++ {
		if look.tokens[i].text == ";" {
			end = look.tokens[i].pos
			break
		}
	}
	return &ApexSyntaxError{Message: "Unexpected token 'final'.", NativeMessage: "Unexpected token 'final'.", Offset: start.pos, EndOffset: end}
}

// FinalCollectionArraySyntaxError also supplies the measured text when a
// declaration parser rejects the captured local syntax before body lowering.
func FinalCollectionArraySyntaxError(source string) *ApexSyntaxError {
	tokens, err := lexWithAnnotations(source, true)
	if err != nil {
		return nil
	}
	for i := 0; i < len(tokens)-1; i++ {
		if i > 0 && tokens[i-1].text != ";" && tokens[i-1].text != "{" && tokens[i-1].text != "}" {
			continue
		}
		p := parser{tokens: tokens, pos: i}
		if syntax := p.finalCollectionArrayDeclaration(); syntax != nil {
			return syntax
		}
	}
	return nil
}

// The captured row consumes ++ on a public Long field of an object reached through
// a final Map<String,T> local. Native throws UnexpectedException rather than
// returning a value. Statement increments, SObjects and carried list results
// never enter this declaration-initializer boundary.
func (vm *VM) finalCollectionIncrementResultError(inst ir.Instruction, result *Result) error {
	expr := inst.Expr
	if inst.Op != ir.OpDeclare || !strings.EqualFold(inst.Type, "Long") || expr.Kind != ir.ExprCall || expr.Callee != "__prefix:++" || expr.Left == nil {
		return nil
	}
	target := expr.Left
	if target.Kind != ir.ExprCall || !strings.HasPrefix(target.Callee, "__field:") || target.Left == nil {
		return nil
	}
	get := target.Left
	if get.Kind != ir.ExprCall || get.Left != nil || len(get.Args) != 1 || get.Args[0].Kind != ir.ExprLiteral || !strings.HasPrefix(get.Args[0].Value, "'") {
		return nil
	}
	root, method, ok := strings.Cut(get.Callee, ".")
	if !ok || method != "get" || strings.Contains(root, ".") {
		return nil
	}
	actual, ok := vm.lookupGlobalName(root)
	if !ok || !vm.finalLocals[actual] {
		return nil
	}
	base, ok := genericBaseName(vm.VarTypes[actual])
	args, typed := genericTypeArgs(vm.VarTypes[actual])
	if !ok || !strings.EqualFold(base, "Map") || !typed || len(args) != 2 || !strings.EqualFold(args[0], "String") {
		return nil
	}
	receiver, err := vm.eval(*get, result)
	if err != nil || receiver.Kind != ValueObject {
		return nil
	}
	field, owner, ok := vm.lookupField(receiver.Type, strings.TrimPrefix(target.Callee, "__field:"))
	if !ok || !strings.EqualFold(owner, receiver.Type) || !strings.EqualFold(field.Type, "Long") || field.Access != "public" || field.Getter != nil || field.Setter != nil || methodHasModifier(field.Modifiers, "final") || methodHasModifier(field.Modifiers, "static") {
		return nil
	}
	return newExceptionError("UnexpectedException", "Illegal arguments")
}
