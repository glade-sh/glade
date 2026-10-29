package sema

import (
	"fmt"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"strings"
)

// Complex inline operands use the same lowered expressions as ordinary Apex.
// Bare variable binds retain their existing type/window diagnostics.
func (c querySemanticsChecker) soqlBindExpressionDiagnostics(query soql.Query, ctx queryTextContext, bindings map[string]string) []diagnostic.Diagnostic {
	operands, err := soql.BindExpressions(ctx.queryText)
	if err != nil {
		return nil
	}
	types := make(map[string]string)
	var diagnostics []diagnostic.Diagnostic
	for _, operand := range operands {
		if semaBindName(operand.Text) {
			continue
		}
		expr, err := vm.CompileSOQLBindExpression(operand.Text)
		if err != nil {
			continue
		}
		message := c.soqlBindExpressionProblem(expr, bindings)
		if message != "" {
			diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_BIND", message, ":"+operand.Text, operand.Offset))
			continue
		}
		types[operand.Text] = c.soqlBindExpressionType(expr, bindings)
	}
	var check func(soql.Condition)
	check = func(condition soql.Condition) {
		for _, child := range condition.And {
			check(child)
		}
		for _, child := range condition.Or {
			check(child)
		}
		if condition.Subquery != nil {
			return
		}
		field, ok := c.field(query.Object, condition.Field)
		if !ok {
			return
		}
		values := append([]storage.Value{condition.Value, condition.Value2}, condition.Values...)
		for _, value := range values {
			if value.Kind != storage.ValueID {
				continue
			}
			text := strings.TrimPrefix(string(value.ID), ":")
			typ := types[text]
			if element, ok := queryBindCollectionElementType(typ); ok {
				typ = element
			}
			if typ == "" || queryFieldAcceptsBindType(field.Type, typ) {
				continue
			}
			columnType := field.Type
			switch strings.ToLower(field.Type) {
			case "text", "string", "textarea", "email", "phone", "url", "picklist", "multipicklist":
				columnType = "String"
			case "number", "currency", "percent", "double", "decimal":
				columnType = "Decimal"
			case "checkbox", "boolean":
				columnType = "Boolean"
			}
			message := fmt.Sprintf("Invalid bind expression type of %s for column of type %s", typ, columnType)
			diagnostics = append(diagnostics, ctx.diagnostic("GLADESEMA_QUERY_BIND", message, ":"+text, findQueryIdentifier(ctx.queryText, ":"+text, 0)))
		}
	}
	if query.Where != nil {
		check(*query.Where)
	}
	return diagnostics
}

func (c querySemanticsChecker) soqlBindExpressionProblem(expr ir.Expr, bindings map[string]string) string {
	if expr.Kind == ir.ExprVariable {
		name := strings.Split(expr.Name, ".")[0]
		if _, ok := bindings[strings.ToLower(name)]; !ok && !c.isKnownType(strings.ToLower(name)) && !strings.EqualFold(name, "this") {
			return "Variable does not exist: " + name
		}
	}
	if expr.Left != nil {
		if problem := c.soqlBindExpressionProblem(*expr.Left, bindings); problem != "" {
			return problem
		}
	}
	if expr.Right != nil {
		if problem := c.soqlBindExpressionProblem(*expr.Right, bindings); problem != "" {
			return problem
		}
	}
	for _, arg := range expr.Args {
		if problem := c.soqlBindExpressionProblem(arg, bindings); problem != "" {
			return problem
		}
	}
	if expr.Kind == ir.ExprCall && expr.Left == nil && !strings.HasPrefix(expr.Callee, "__") && !strings.HasPrefix(expr.Callee, "new:") && !strings.HasPrefix(expr.Callee, "newlit:") {
		if root, _, ok := strings.Cut(expr.Callee, "."); ok {
			if _, bound := bindings[strings.ToLower(root)]; !bound && !c.isKnownType(strings.ToLower(root)) {
				return "Variable does not exist: " + root
			}
		}
	}
	if expr.Kind == ir.ExprCall && expr.Operator == "[]" && expr.Left != nil && len(expr.Args) == 1 {
		receiver := c.soqlBindExpressionType(*expr.Left, bindings)
		if strings.HasPrefix(strings.ToLower(strings.ReplaceAll(receiver, " ", "")), "map<") {
			return "Expression must be a list type: " + receiver
		}
		if _, list := queryBindCollectionElementType(receiver); list {
			typ := c.soqlBindExpressionType(expr.Args[0], bindings)
			if typ != "" && !strings.EqualFold(typ, "Integer") {
				return "List index must be of type Integer: " + typ
			}
		}
	}
	return ""
}

func (c querySemanticsChecker) soqlBindExpressionType(expr ir.Expr, bindings map[string]string) string {
	switch expr.Kind {
	case ir.ExprLiteral:
		return inferSemaArgType(expr.Value, bindings)
	case ir.ExprVariable:
		if typ, ok := bindings[strings.ToLower(expr.Name)]; ok {
			return typ
		}
		if root, field, ok := strings.Cut(expr.Name, "."); ok {
			return c.soqlBindMemberType(bindings[strings.ToLower(root)], field)
		}
	case ir.ExprUnary:
		if expr.Left != nil {
			return c.soqlBindExpressionType(*expr.Left, bindings)
		}
		if expr.Right != nil {
			return c.soqlBindExpressionType(*expr.Right, bindings)
		}
	case ir.ExprBinary:
		if expr.Left != nil && expr.Right != nil {
			return semaBinaryType(expr.Operator, c.soqlBindExpressionType(*expr.Left, bindings), c.soqlBindExpressionType(*expr.Right, bindings))
		}
	case ir.ExprCall:
		if strings.HasPrefix(expr.Callee, "__cast:") {
			return strings.TrimPrefix(expr.Callee, "__cast:")
		}
		if strings.HasPrefix(expr.Callee, "new:") {
			return strings.TrimPrefix(expr.Callee, "new:")
		}
		if strings.HasPrefix(expr.Callee, "newlit:") {
			return strings.TrimPrefix(expr.Callee, "newlit:")
		}
		if expr.Callee == "__ternary" && len(expr.Args) == 3 {
			return c.soqlBindExpressionType(expr.Args[1], bindings)
		}
		if expr.Callee == "__coalesce" && len(expr.Args) == 2 {
			typ := c.soqlBindExpressionType(expr.Args[0], bindings)
			if typ == "null" {
				return c.soqlBindExpressionType(expr.Args[1], bindings)
			}
			return typ
		}
		if expr.Left != nil {
			receiver := c.soqlBindExpressionType(*expr.Left, bindings)
			if strings.HasPrefix(expr.Callee, "__field:") || strings.HasPrefix(expr.Callee, "__safe_field:") {
				field := strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__safe_field:"), "__field:")
				return c.soqlBindMemberType(receiver, field)
			}
			method := strings.TrimPrefix(expr.Callee, "__safe_call:")
			if sig, ok := semaCollectionMethodSignature(receiver, method); ok {
				return sig.returnTypeForArity(len(expr.Args))
			}
			if strings.EqualFold(receiver, "String") && (strings.EqualFold(method, "toUpperCase") || strings.EqualFold(method, "toLowerCase")) {
				return "String"
			}
		} else if receiver, method, ok := strings.Cut(expr.Callee, "."); ok {
			typ := bindings[strings.ToLower(receiver)]
			if sig, ok := semaCollectionMethodSignature(typ, method); ok {
				return sig.returnTypeForArity(len(expr.Args))
			}
			if strings.EqualFold(typ, "String") && (strings.EqualFold(method, "toUpperCase") || strings.EqualFold(method, "toLowerCase")) {
				return "String"
			}
		}
	}
	return ""
}

func (c querySemanticsChecker) soqlBindMemberType(receiver, field string) string {
	if f, ok := c.field(receiver, field); ok {
		switch strings.ToLower(f.Type) {
		case "text", "string", "textarea", "email", "phone", "url", "picklist":
			return "String"
		case "id", "reference":
			return "Id"
		case "integer":
			return "Integer"
		}
	}
	return ""
}
