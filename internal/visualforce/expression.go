package visualforce

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

var errNoExpression = fmt.Errorf("no expression")

func EvaluateExpression(raw string, ctx *ExpressionContext) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	expr, err := parseExpression(raw)
	if err != nil {
		if err.Error() == errNoExpression.Error() {
			return raw, nil
		}
		return "", err
	}
	value := expr.Eval(ctx)
	if value == nil {
		return "", nil
	}
	switch value.Kind {
	case vm.ValueNull:
		return "", nil
	case vm.ValueString:
		return value.Text, nil
	default:
		return value.String(), nil
	}
}

func RenderExpressionTemplate(raw string, ctx *ExpressionContext) (string, error) {
	if !strings.Contains(raw, "{!") {
		return raw, nil
	}
	out := raw
	for {
		start := strings.Index(out, "{!")
		if start < 0 {
			break
		}
		end := strings.Index(out[start:], "}")
		if end < 0 {
			break
		}
		end += start
		exprText := strings.TrimSpace(out[start+2 : end])
		value, err := EvaluateExpression(exprText, ctx)
		if err != nil {
			return "", err
		}
		out = out[:start] + value + out[end+1:]
	}
	return out, nil
}

type ExpressionContext struct {
	Controller         vm.Value
	Extensions         []vm.Value
	StandardController vm.Value
	CurrentPage        vm.Value
	VM                 *vm.VM
	Variables          map[string]vm.Value
	Records            map[string][]vm.Value
	Scope              *ScopeStack
	ProjectNamespace   string
}

type Expression interface {
	Eval(ctx *ExpressionContext) *vm.Value
}

type literalExpr struct {
	value vm.Value
}

func (expr literalExpr) Eval(ctx *ExpressionContext) *vm.Value {
	_ = ctx
	return &expr.value
}

type identifierExpr struct {
	parts []string
}

func (expr identifierExpr) Eval(ctx *ExpressionContext) *vm.Value {
	if ctx == nil {
		return &vm.Null
	}
	if len(expr.parts) == 0 {
		return &vm.Null
	}
	if expr.parts[0] == "$label" {
		label := resolveLabel(expr.parts, ctx)
		return &label
	}
	if expr.parts[0] == "$resource" {
		resourceValue := resolveResource(expr.parts, ctx)
		return &resourceValue
	}
	value := resolveValueByIdentifier(ctx, expr.parts)
	return &value
}

type functionExpr struct {
	name string
	args []Expression
}

func (expr functionExpr) Eval(ctx *ExpressionContext) *vm.Value {
	if expr.name == "" {
		return &vm.Null
	}
	switch strings.ToUpper(expr.name) {
	case "IF":
		if len(expr.args) < 2 {
			return &vm.Null
		}
		truthy := isTruthy(expr.args[0].Eval(ctx))
		if truthy {
			if len(expr.args) >= 2 {
				return expr.args[1].Eval(ctx)
			}
			return &vm.Null
		}
		if len(expr.args) >= 3 {
			return expr.args[2].Eval(ctx)
		}
		return &vm.Null
	case "CASESAFEID", "TEXT":
		if len(expr.args) == 0 {
			return &vm.Null
		}
		value := expr.args[0].Eval(ctx)
		if value == nil {
			return &vm.Null
		}
		return value
	case "JSENCODE", "HTMLENCODE":
		if len(expr.args) == 0 {
			return &vm.Null
		}
		value := expr.args[0].Eval(ctx)
		if value == nil {
			return &vm.Null
		}
		return value
	default:
		if len(expr.args) == 0 {
			return &vm.Null
		}
		return expr.args[0].Eval(ctx)
	}
}

func isTruthy(raw *vm.Value) bool {
	if raw == nil {
		return false
	}
	value := *raw
	switch value.Kind {
	case vm.ValueString:
		return strings.TrimSpace(value.Text) != ""
	case vm.ValueBool:
		return value.Bool
	case vm.ValueInt:
		return value.Int != 0
	case vm.ValueDecimal:
		return value.Decimal != 0
	case vm.ValueNull:
		return false
	default:
		if value.Kind == vm.ValueObject {
			return value.Text != "" || len(value.Fields) > 0 || len(value.List) > 0
		}
	}
	return false
}

type exprParser struct {
	source string
	pos    int
}

func parseExpression(source string) (Expression, error) {
	p := &exprParser{source: strings.TrimSpace(source)}
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos != len(p.source) {
		return nil, fmt.Errorf("unexpected token at %d", p.pos)
	}
	return expr, nil
}

func (p *exprParser) parseExpression() (Expression, error) {
	p.skipSpace()
	if p.pos >= len(p.source) {
		return nil, errNoExpression
	}
	ch := p.source[p.pos]
	switch {
	case ch == '\'' || ch == '"':
		return p.parseString()
	case ch == 't' || ch == 'f':
		if strings.HasPrefix(strings.ToLower(p.source[p.pos:]), "true") {
			p.pos += 4
			return literalExpr{value: vm.Bool(true)}, nil
		}
		if strings.HasPrefix(strings.ToLower(p.source[p.pos:]), "false") {
			p.pos += 5
			return literalExpr{value: vm.Bool(false)}, nil
		}
	case isAlpha(ch) || ch == '_' || ch == '$':
		return p.parseIdentifierOrCall()
	}
	return nil, fmt.Errorf("unsupported expression: %q", p.source[p.pos:])
}

func (p *exprParser) parseIdentifierOrCall() (Expression, error) {
	parts, err := p.parseIdentifierParts()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos < len(p.source) && p.source[p.pos] == '(' {
		p.pos++
		args, err := p.parseArgList()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos >= len(p.source) || p.source[p.pos] != ')' {
			return nil, fmt.Errorf("missing ')' in function expression")
		}
		p.pos++
		return functionExpr{name: strings.ToLower(parts[0]), args: args}, nil
	}
	return identifierExpr{parts: parts}, nil
}

func (p *exprParser) parseArgList() ([]Expression, error) {
	args := make([]Expression, 0)
	for {
		p.skipSpace()
		if p.pos >= len(p.source) {
			return nil, fmt.Errorf("missing ')' in function expression")
		}
		if p.source[p.pos] == ')' {
			return args, nil
		}
		arg, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		p.skipSpace()
		if p.pos >= len(p.source) {
			return nil, fmt.Errorf("missing ')' in function expression")
		}
		if p.source[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.source[p.pos] == ')' {
			return args, nil
		}
		return nil, fmt.Errorf("expected ',' or ')' in expression")
	}
}

func (p *exprParser) parseIdentifierParts() ([]string, error) {
	part := p.readIdent()
	if part == "" {
		return nil, fmt.Errorf("expected identifier")
	}
	parts := []string{strings.ToLower(part)}
	for {
		p.skipSpace()
		if p.pos >= len(p.source) || p.source[p.pos] != '.' {
			break
		}
		p.pos++
		next := p.readIdent()
		if next == "" {
			return nil, fmt.Errorf("expected identifier after '.'")
		}
		parts = append(parts, strings.ToLower(next))
	}
	return parts, nil
}

func (p *exprParser) parseString() (Expression, error) {
	quote := p.source[p.pos]
	p.pos++
	start := p.pos
	for p.pos < len(p.source) && p.source[p.pos] != quote {
		if p.source[p.pos] == '\\' && p.pos+1 < len(p.source) {
			p.pos += 2
			continue
		}
		p.pos++
	}
	if p.pos >= len(p.source) {
		return nil, fmt.Errorf("unterminated string literal")
	}
	value := p.source[start:p.pos]
	p.pos++
	return literalExpr{value: vm.String(value)}, nil
}

func (p *exprParser) readIdent() string {
	start := p.pos
	for p.pos < len(p.source) {
		ch := p.source[p.pos]
		if isIdentChar(ch) {
			p.pos++
			continue
		}
		break
	}
	return p.source[start:p.pos]
}

func (p *exprParser) skipSpace() {
	for p.pos < len(p.source) && (p.source[p.pos] == ' ' || p.source[p.pos] == '\t' || p.source[p.pos] == '\n' || p.source[p.pos] == '\r') {
		p.pos++
	}
}

func isAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isIdentChar(ch byte) bool {
	return isAlpha(ch) || (ch >= '0' && ch <= '9') || ch == '_' || ch == '$'
}

func resolveLabel(parts []string, ctx *ExpressionContext) vm.Value {
	if ctx == nil || ctx.VM == nil || len(parts) < 2 {
		return vm.Null
	}
	if ctx.VM.Org == nil {
		return vm.Null
	}
	namespace := ""
	name := strings.TrimSpace(parts[1])
	if len(parts) >= 3 {
		namespace = strings.TrimSpace(parts[1])
		name = strings.TrimSpace(parts[2])
	}
	if name == "" {
		return vm.Null
	}
	value, status := resource.ResolveLabel(storage.MetadataRegistry(ctx.VM.Org.Metadata), ctx.VM.Org.Namespace, namespace, name)
	if status == resource.LabelLookupMissing {
		return vm.Null
	}
	if strings.HasSuffix(name, ".") {
		name = strings.TrimSuffix(name, ".")
	}
	if value == "" {
		value = name
	}
	return vm.String(value)
}

func resolveResource(parts []string, ctx *ExpressionContext) vm.Value {
	if len(parts) < 2 || ctx == nil || ctx.VM == nil || ctx.VM.Org == nil {
		return vm.String("/resource")
	}
	if len(parts) == 2 {
		return vm.String("/resource/" + strings.TrimSpace(parts[1]))
	}
	value := strings.TrimSpace(parts[1])
	path := ""
	if len(parts) > 2 {
		path = strings.TrimSpace(strings.Join(parts[2:], "/"))
	}
	if path == "" {
		return vm.String("/resource/" + value)
	}
	return vm.String("/resource/" + value + "/" + path)
}

func resolveValueByIdentifier(ctx *ExpressionContext, parts []string) vm.Value {
	if ctx == nil {
		return vm.Null
	}
	if len(parts) == 0 {
		return vm.Null
	}
	base := parts[0]
	chain := parts[1:]
	value, ok := resolveRootValue(ctx, base)
	if !ok {
		return vm.Null
	}
	for _, field := range chain {
		if value.Kind != vm.ValueObject {
			return vm.Null
		}
		next, ok := readObjectMember(ctx, value, field)
		if !ok {
			return vm.Null
		}
		value = next
	}
	return value
}

func readObjectMember(ctx *ExpressionContext, value vm.Value, field string) (vm.Value, bool) {
	if next, ok := objectFieldIgnoreCase(value, field); ok {
		return next, true
	}
	if ctx != nil && ctx.VM != nil && isControllerValue(ctx, value) {
		next, ok, err := ctx.VM.ReadInstanceProperty(value, field)
		if err != nil || !ok {
			return vm.Null, false
		}
		return next, true
	}
	return vm.Null, false
}

func isControllerValue(ctx *ExpressionContext, value vm.Value) bool {
	if ctx == nil || value.Kind != vm.ValueObject {
		return false
	}
	if ctx.Controller.Kind == vm.ValueObject && valuesEqual(ctx.Controller, value) {
		return true
	}
	for _, ext := range ctx.Extensions {
		if ext.Kind == vm.ValueObject && valuesEqual(ext, value) {
			return true
		}
	}
	if ctx.StandardController.Kind == vm.ValueObject && valuesEqual(ctx.StandardController, value) {
		return true
	}
	return false
}

func valuesEqual(left, right vm.Value) bool {
	if left.Kind != right.Kind {
		return false
	}
	if left.Type != "" && right.Type != "" {
		return left.Type == right.Type
	}
	return left.Text == right.Text
}

func objectFieldIgnoreCase(value vm.Value, field string) (vm.Value, bool) {
	if value.Kind != vm.ValueObject {
		return vm.Null, false
	}
	if next, ok := value.Fields[field]; ok {
		return next, true
	}
	for key, candidate := range value.Fields {
		if strings.EqualFold(key, field) {
			return candidate, true
		}
	}
	return vm.Null, false
}

func resolveRootValue(ctx *ExpressionContext, name string) (vm.Value, bool) {
	if name == "" {
		return vm.Null, false
	}
	if ctx.VM != nil && strings.EqualFold(name, "this") {
		if ctx.Controller.Kind == vm.ValueObject {
			return ctx.Controller, true
		}
	}
	if ctx.VM != nil && strings.EqualFold(name, "currentpage") {
		if !isValueNullOrBlank(ctx.CurrentPage) {
			return ctx.CurrentPage, true
		}
		page := ctx.VM.CurrentPage()
		if page.Kind != vm.ValueNull {
			return page, true
		}
	}
	if ctx.Controller.Kind == vm.ValueObject {
		if value, ok := objectFieldIgnoreCase(ctx.Controller, name); ok {
			return normalizeNamespaceMergeValue(name, value, ctx), true
		}
		if ctx.VM != nil {
			if value, ok, err := ctx.VM.ReadInstanceProperty(ctx.Controller, name); ok && err == nil {
				return normalizeNamespaceMergeValue(name, value, ctx), true
			}
		}
	}
	for _, ext := range ctx.Extensions {
		if ext.Kind == vm.ValueObject {
			if value, ok := objectFieldIgnoreCase(ext, name); ok {
				return value, true
			}
			if ctx.VM != nil {
				if value, ok, err := ctx.VM.ReadInstanceProperty(ext, name); ok && err == nil {
					return value, true
				}
			}
		}
	}
	if ctx.StandardController.Kind == vm.ValueObject {
		if value, ok := objectFieldIgnoreCase(ctx.StandardController, name); ok {
			return value, true
		}
	}
	if ctx.Scope != nil {
		if value, ok := ctx.Scope.Get(name); ok {
			return value, true
		}
	}
	if ctx.Variables != nil {
		for varName, value := range ctx.Variables {
			if strings.EqualFold(varName, name) {
				return value, true
			}
		}
	}
	if strings.EqualFold(name, "namespace") {
		if ns := strings.TrimSpace(ctx.ProjectNamespace); ns != "" {
			return vm.String(ns), true
		}
	}
	return vm.Null, false
}

func normalizeNamespaceMergeValue(name string, value vm.Value, ctx *ExpressionContext) vm.Value {
	if !strings.EqualFold(name, "namespace") || ctx == nil {
		return value
	}
	if value.Kind == vm.ValueString {
		text := strings.TrimSpace(value.Text)
		if text != "" && !strings.EqualFold(text, "c") {
			return value
		}
	}
	if ns := strings.TrimSpace(ctx.ProjectNamespace); ns != "" {
		return vm.String(ns)
	}
	return value
}

func isValueNullOrBlank(value vm.Value) bool {
	if value.Kind == vm.ValueNull {
		return true
	}
	if value.Kind == vm.ValueString {
		return strings.TrimSpace(value.Text) == ""
	}
	return false
}
