package visualforce

import (
	"fmt"
	"html"
	"net/url"
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
	var out strings.Builder
	pos := 0
	for pos < len(raw) {
		next := strings.Index(raw[pos:], "{!")
		if next < 0 {
			out.WriteString(raw[pos:])
			break
		}
		start := pos + next
		out.WriteString(raw[pos:start])
		end := findExpressionTemplateEnd(raw, start+2)
		if end < 0 {
			out.WriteString(raw[start:])
			break
		}
		exprText := strings.TrimSpace(raw[start+2 : end])
		value, err := EvaluateExpression(exprText, ctx)
		if err != nil {
			return "", err
		}
		out.WriteString(value)
		pos = end + 1
	}
	return out.String(), nil
}

func findExpressionTemplateEnd(raw string, offset int) int {
	var quote rune
	escaped := false
	for i, r := range raw[offset:] {
		pos := offset + i
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '}' {
			return pos
		}
	}
	return -1
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
	if strings.EqualFold(expr.parts[0], "$label") {
		label := resolveLabel(expr.parts, ctx)
		return &label
	}
	if strings.EqualFold(expr.parts[0], "$resource") {
		resourceValue := resolveResource(expr.parts, ctx)
		return &resourceValue
	}
	if strings.EqualFold(expr.parts[0], "$currentpage") {
		value := resolveValueByIdentifier(ctx, expr.parts)
		return &value
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
	case "UPPER":
		return evalVisualforceStringFunction(ctx, expr.args, strings.ToUpper)
	case "LOWER":
		return evalVisualforceStringFunction(ctx, expr.args, strings.ToLower)
	case "URLENCODE":
		return evalVisualforceStringFunction(ctx, expr.args, url.QueryEscape)
	case "URLDECODE":
		return evalVisualforceURLDecodeFunction(ctx, expr.args)
	case "JSENCODE":
		return evalVisualforceStringFunction(ctx, expr.args, EscapeVisualforceJavaScriptString)
	case "HTMLENCODE":
		return evalVisualforceStringFunction(ctx, expr.args, html.EscapeString)
	case "ISBLANK":
		if len(expr.args) == 0 {
			out := vm.Bool(true)
			return &out
		}
		value := expr.args[0].Eval(ctx)
		out := vm.Bool(value == nil || isValueNullOrBlank(*value))
		return &out
	case "NOT":
		if len(expr.args) == 0 {
			return &vm.Null
		}
		out := vm.Bool(!isTruthy(expr.args[0].Eval(ctx)))
		return &out
	case "AND":
		for _, arg := range expr.args {
			if !isTruthy(arg.Eval(ctx)) {
				out := vm.Bool(false)
				return &out
			}
		}
		out := vm.Bool(true)
		return &out
	case "OR":
		for _, arg := range expr.args {
			if isTruthy(arg.Eval(ctx)) {
				out := vm.Bool(true)
				return &out
			}
		}
		out := vm.Bool(false)
		return &out
	default:
		if len(expr.args) == 0 {
			return &vm.Null
		}
		return expr.args[0].Eval(ctx)
	}
}

func evalVisualforceStringFunction(ctx *ExpressionContext, args []Expression, transform func(string) string) *vm.Value {
	if len(args) == 0 {
		return &vm.Null
	}
	value := args[0].Eval(ctx)
	if value == nil || value.Kind == vm.ValueNull {
		return &vm.Null
	}
	out := vm.String(transform(value.String()))
	return &out
}

func evalVisualforceURLDecodeFunction(ctx *ExpressionContext, args []Expression) *vm.Value {
	if len(args) == 0 {
		return &vm.Null
	}
	value := args[0].Eval(ctx)
	if value == nil || value.Kind == vm.ValueNull {
		return &vm.Null
	}
	decoded, err := url.QueryUnescape(value.String())
	if err != nil {
		return value
	}
	out := vm.String(decoded)
	return &out
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

func (p *exprParser) parsePrimary() (Expression, error) {
	p.skipSpace()
	if p.pos >= len(p.source) {
		return nil, errNoExpression
	}
	ch := p.source[p.pos]
	if p.matchWord("TRUE") {
		return literalExpr{value: vm.Bool(true)}, nil
	}
	if p.matchWord("FALSE") {
		return literalExpr{value: vm.Bool(false)}, nil
	}
	switch {
	case ch == '\'' || ch == '"':
		return p.parseString()
	case ch == '(':
		p.pos++
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos >= len(p.source) || p.source[p.pos] != ')' {
			return nil, fmt.Errorf("missing ')' in expression")
		}
		p.pos++
		return expr, nil
	case isDigit(ch):
		return p.parseNumber()
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
	parts := []string{part}
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
		parts = append(parts, next)
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
	if len(parts) < 2 || ctx == nil || ctx.VM == nil {
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
		next, ok := readObjectMember(ctx, value, field)
		if !ok {
			return vm.Null
		}
		value = next
	}
	return value
}

func readObjectMember(ctx *ExpressionContext, value vm.Value, field string) (vm.Value, bool) {
	if ctx != nil && ctx.VM != nil && isControllerValue(ctx, value) {
		next, ok, err := ctx.VM.ReadInstanceProperty(value, field)
		if err == nil && ok {
			return next, true
		}
	}
	if next, ok := objectFieldIgnoreCase(value, field); ok {
		return next, true
	}
	if next, ok := mapMemberIgnoreCase(value, field); ok {
		return next, true
	}
	return vm.Null, false
}

func mapMemberIgnoreCase(value vm.Value, field string) (vm.Value, bool) {
	if value.Kind != vm.ValueMap {
		return vm.Null, false
	}
	for rawKey, candidate := range value.Map {
		if key, ok := value.MapKeys[rawKey]; ok && key.Kind == vm.ValueString && strings.EqualFold(key.Text, field) {
			return candidate, true
		}
		if strings.EqualFold(rawKey, "string:"+field) {
			return candidate, true
		}
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
	if value, ok := resolveVisualforceGlobal(ctx, name); ok {
		return value, true
	}
	if ctx.VM != nil && strings.EqualFold(name, "this") {
		if ctx.Controller.Kind == vm.ValueObject {
			return ctx.Controller, true
		}
	}
	if ctx.VM != nil && (strings.EqualFold(name, "currentpage") || strings.EqualFold(name, "$currentpage")) {
		if ctx.CurrentPage.Kind != "" && !isValueNullOrBlank(ctx.CurrentPage) {
			return ctx.CurrentPage, true
		}
		page := ctx.VM.CurrentPage()
		if page.Kind != vm.ValueNull {
			return page, true
		}
	}
	if ctx.Controller.Kind == vm.ValueObject {
		if ctx.VM != nil {
			if value, ok, err := ctx.VM.ReadInstanceProperty(ctx.Controller, name); ok && err == nil {
				return normalizeNamespaceMergeValue(name, value, ctx), true
			}
		}
		if value, ok := objectFieldIgnoreCase(ctx.Controller, name); ok {
			return normalizeNamespaceMergeValue(name, value, ctx), true
		}
	}
	for _, ext := range ctx.Extensions {
		if ext.Kind == vm.ValueObject {
			if ctx.VM != nil {
				if value, ok, err := ctx.VM.ReadInstanceProperty(ext, name); ok && err == nil {
					return value, true
				}
			}
			if value, ok := objectFieldIgnoreCase(ext, name); ok {
				return value, true
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

func resolveVisualforceGlobal(ctx *ExpressionContext, name string) (vm.Value, bool) {
	if ctx == nil || ctx.VM == nil {
		return vm.Null, false
	}
	switch strings.ToLower(name) {
	case "$user":
		user := vm.Object("User")
		if record, ok := visualforceCurrentUserRecord(ctx); ok {
			user.Fields["Id"] = storageRecordFieldValue(record, "Id")
			user.Fields["Username"] = storageRecordFieldValue(record, "Username")
			user.Fields["Email"] = storageRecordFieldValue(record, "Email")
		}
		return user, true
	case "$profile":
		profile := vm.Object("Profile")
		if record, ok := visualforceCurrentUserRecord(ctx); ok {
			profile.Fields["Id"] = storageRecordFieldValue(record, "ProfileId")
		}
		return profile, true
	case "$organization":
		org := vm.Object("Organization")
		if ctx.VM.Org != nil && strings.TrimSpace(ctx.VM.Org.OrgID) != "" {
			org.Fields["Id"] = vm.String(strings.TrimSpace(ctx.VM.Org.OrgID))
		}
		if record, ok := visualforceOrganizationRecord(ctx); ok {
			if org.Fields["Id"].Kind == "" || org.Fields["Id"].Kind == vm.ValueNull {
				org.Fields["Id"] = storageRecordFieldValue(record, "Id")
			}
			org.Fields["Name"] = storageRecordFieldValue(record, "Name")
		}
		return org, true
	default:
		return vm.Null, false
	}
}

func visualforceCurrentUserRecord(ctx *ExpressionContext) (storage.Record, bool) {
	users, ok := visualforceObjectState(ctx, "User")
	if !ok || len(users.Records) == 0 {
		return storage.Record{}, false
	}
	for _, preferredID := range []storage.ID{storage.ID("005-local-user"), storage.ID("005000000000001")} {
		if record, ok := users.Records[preferredID]; ok && !storageRecordFieldEqual(record, "UserType", "AutomatedProcess") {
			return record, true
		}
	}
	var first storage.ID
	var fallback storage.ID
	for id, record := range users.Records {
		if storageRecordFieldEqual(record, "UserType", "AutomatedProcess") {
			if fallback == "" || id < fallback {
				fallback = id
			}
			continue
		}
		if first == "" || id < first {
			first = id
		}
	}
	if first == "" {
		first = fallback
	}
	if first == "" {
		return storage.Record{}, false
	}
	return users.Records[first], true
}

func visualforceOrganizationRecord(ctx *ExpressionContext) (storage.Record, bool) {
	orgs, ok := visualforceObjectState(ctx, "Organization")
	if !ok || len(orgs.Records) == 0 {
		return storage.Record{}, false
	}
	if ctx != nil && ctx.VM != nil && ctx.VM.Org != nil {
		if orgID := strings.TrimSpace(ctx.VM.Org.OrgID); orgID != "" {
			if record, ok := orgs.Records[storage.ID(orgID)]; ok {
				return record, true
			}
		}
	}
	var first storage.ID
	for id := range orgs.Records {
		if first == "" || id < first {
			first = id
		}
	}
	if first == "" {
		return storage.Record{}, false
	}
	return orgs.Records[first], true
}

func visualforceObjectState(ctx *ExpressionContext, objectName string) (storage.ObjectState, bool) {
	if ctx == nil || ctx.VM == nil || ctx.VM.Org == nil {
		return storage.ObjectState{}, false
	}
	if object, ok := ctx.VM.Org.Objects[objectName]; ok {
		return object, true
	}
	for name, object := range ctx.VM.Org.Objects {
		if strings.EqualFold(name, objectName) {
			return object, true
		}
	}
	return storage.ObjectState{}, false
}

func storageRecordFieldValue(record storage.Record, field string) vm.Value {
	if strings.EqualFold(field, "Id") && record.ID != "" {
		return vm.String(string(record.ID))
	}
	value, ok := record.GetField(field)
	if !ok {
		return vm.Null
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return vm.String(value.String)
	case storage.ValueID:
		return vm.String(string(value.ID))
	case storage.ValueInteger:
		return vm.Int(value.Integer)
	case storage.ValueBoolean:
		return vm.Bool(value.Boolean)
	case storage.ValueDecimal:
		return vm.String(value.Decimal)
	case storage.ValueNull:
		return vm.Null
	default:
		return vm.Null
	}
}

func storageRecordFieldEqual(record storage.Record, field string, want string) bool {
	value := storageRecordFieldValue(record, field)
	return value.Kind == vm.ValueString && strings.EqualFold(strings.TrimSpace(value.Text), want)
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
