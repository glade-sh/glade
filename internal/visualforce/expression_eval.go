package visualforce

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type binaryExpr struct {
	op    string
	left  Expression
	right Expression
}

type unaryExpr struct {
	op    string
	value Expression
}

type indexExpr struct {
	target Expression
	key    Expression
}

type memberExpr struct {
	target Expression
	field  string
}

type methodCallExpr struct {
	target Expression
	name   string
	args   []Expression
}

type visualforceIdentifierExpr struct {
	parts []string
}

type visualforceFunctionExpr struct {
	name string
	args []Expression
}

func (expr binaryExpr) Eval(ctx *ExpressionContext) *vm.Value {
	left := evalExpressionValue(expr.left, ctx)
	switch expr.op {
	case "&&":
		if !isTruthy(&left) {
			out := vm.Bool(false)
			return &out
		}
		right := evalExpressionValue(expr.right, ctx)
		out := vm.Bool(isTruthy(&right))
		return &out
	case "||":
		if isTruthy(&left) {
			out := vm.Bool(true)
			return &out
		}
		right := evalExpressionValue(expr.right, ctx)
		out := vm.Bool(isTruthy(&right))
		return &out
	}
	right := evalExpressionValue(expr.right, ctx)
	switch expr.op {
	case "+", "-", "*", "/":
		out := evalArithmetic(expr.op, left, right)
		return &out
	case "==", "!=", ">", ">=", "<", "<=":
		out := vm.Bool(compareValues(expr.op, left, right))
		return &out
	default:
		return &vm.Null
	}
}

func (expr unaryExpr) Eval(ctx *ExpressionContext) *vm.Value {
	value := evalExpressionValue(expr.value, ctx)
	switch expr.op {
	case "!":
		out := vm.Bool(!isTruthy(&value))
		return &out
	default:
		return &vm.Null
	}
}

func (expr indexExpr) Eval(ctx *ExpressionContext) *vm.Value {
	target := evalExpressionValue(expr.target, ctx)
	key := evalExpressionValue(expr.key, ctx)
	if key.Kind == vm.ValueNull {
		return &vm.Null
	}
	value, ok := readIndexValue(ctx, target, key)
	if !ok {
		return &vm.Null
	}
	return &value
}

func (expr memberExpr) Eval(ctx *ExpressionContext) *vm.Value {
	target := evalExpressionValue(expr.target, ctx)
	value, ok := readVisualforceExtraMember(ctx, target, expr.field)
	if !ok {
		value, ok = readObjectMember(ctx, target, expr.field)
	}
	if !ok {
		return &vm.Null
	}
	return &value
}

func (expr methodCallExpr) Eval(ctx *ExpressionContext) *vm.Value {
	target := evalExpressionValue(expr.target, ctx)
	if target.Kind == vm.ValueNull {
		return &vm.Null
	}
	args := make([]vm.Value, 0, len(expr.args))
	for _, arg := range expr.args {
		args = append(args, evalExpressionValue(arg, ctx))
	}
	if len(args) == 0 {
		if value, ok := readObjectMember(ctx, target, expr.name); ok {
			return &value
		}
	}
	if ctx == nil || ctx.VM == nil || target.Kind != vm.ValueObject || strings.TrimSpace(target.Type) == "" || len(args) != 0 {
		return &vm.Null
	}
	value, updated, result, err := ctx.VM.InvokeVisualforceActionOnController(target, target.Type, expr.name, "", nil)
	if err != nil || !result.Success {
		return &vm.Null
	}
	writeBackVisualforceReceiver(ctx, target, updated)
	return &value
}

func writeBackVisualforceReceiver(ctx *ExpressionContext, target, updated vm.Value) {
	if ctx == nil {
		return
	}
	if valuesEqual(ctx.Controller, target) {
		ctx.Controller = updated
	}
	for i := range ctx.Extensions {
		if valuesEqual(ctx.Extensions[i], target) {
			ctx.Extensions[i] = updated
			return
		}
	}
	if valuesEqual(ctx.StandardController, target) {
		ctx.StandardController = updated
	}
}

func evalExpressionValue(expr Expression, ctx *ExpressionContext) vm.Value {
	if expr == nil {
		return vm.Null
	}
	value := expr.Eval(ctx)
	if value == nil {
		return vm.Null
	}
	return *value
}

func (expr visualforceIdentifierExpr) Eval(ctx *ExpressionContext) *vm.Value {
	if len(expr.parts) == 0 {
		return &vm.Null
	}
	value, ok := resolveVisualforceExtraGlobal(ctx, expr.parts[0])
	if !ok {
		return identifierExpr{parts: expr.parts}.Eval(ctx)
	}
	for _, field := range expr.parts[1:] {
		next, ok := readVisualforceExtraMember(ctx, value, field)
		if !ok {
			return &vm.Null
		}
		value = next
	}
	return &value
}

func (expr visualforceFunctionExpr) Eval(ctx *ExpressionContext) *vm.Value {
	switch strings.ToUpper(expr.name) {
	case "CASE":
		return evalVisualforceCase(ctx, expr.args)
	case "BLANKVALUE":
		return evalVisualforceBlankValue(ctx, expr.args)
	case "NULLVALUE":
		return evalVisualforceNullValue(ctx, expr.args)
	case "VALUE":
		return evalVisualforceValue(ctx, expr.args)
	case "URLFOR":
		return evalVisualforceURLFor(ctx, expr.args)
	default:
		return functionExpr{name: expr.name, args: expr.args}.Eval(ctx)
	}
}

func evalVisualforceCase(ctx *ExpressionContext, args []Expression) *vm.Value {
	if len(args) < 3 {
		return &vm.Null
	}
	target := evalExpressionValue(args[0], ctx)
	limit := len(args)
	var fallback Expression
	if (len(args)-1)%2 == 1 {
		fallback = args[len(args)-1]
		limit = len(args) - 1
	}
	for i := 1; i+1 < limit; i += 2 {
		candidate := evalExpressionValue(args[i], ctx)
		if valuesComparableEqual(target, candidate) {
			return args[i+1].Eval(ctx)
		}
	}
	if fallback != nil {
		return fallback.Eval(ctx)
	}
	return &vm.Null
}

func evalVisualforceBlankValue(ctx *ExpressionContext, args []Expression) *vm.Value {
	if len(args) < 2 {
		return &vm.Null
	}
	value := evalExpressionValue(args[0], ctx)
	if isValueNullOrBlank(value) {
		return args[1].Eval(ctx)
	}
	return &value
}

func evalVisualforceNullValue(ctx *ExpressionContext, args []Expression) *vm.Value {
	if len(args) < 2 {
		return &vm.Null
	}
	value := evalExpressionValue(args[0], ctx)
	if value.Kind == vm.ValueNull {
		return args[1].Eval(ctx)
	}
	return &value
}

func evalVisualforceValue(ctx *ExpressionContext, args []Expression) *vm.Value {
	if len(args) == 0 {
		return &vm.Null
	}
	value := evalExpressionValue(args[0], ctx)
	text := strings.TrimSpace(value.String())
	if text == "" {
		return &vm.Null
	}
	if !strings.ContainsAny(text, ".eE") {
		if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
			out := vm.Int(parsed)
			return &out
		}
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return &vm.Null
	}
	out := vm.Decimal(parsed)
	out.Text = text
	return &out
}

func resolveVisualforceExtraGlobal(ctx *ExpressionContext, name string) (vm.Value, bool) {
	switch strings.ToLower(name) {
	case "$objecttype":
		return vm.Object("$ObjectType"), true
	case "$permission":
		return vm.Object("$Permission"), true
	case "$setup":
		return vm.Object("$Setup"), true
	case "$site":
		return visualforceSiteValue(ctx), true
	case "$component":
		return vm.Object("$Component"), true
	case "$remoteaction":
		value := vm.Object("$RemoteAction")
		value.Fields["value"] = vm.String("{!$RemoteAction}")
		return value, true
	default:
		return vm.Null, false
	}
}

func readIndexValue(ctx *ExpressionContext, target, key vm.Value) (vm.Value, bool) {
	switch target.Kind {
	case vm.ValueList:
		index, ok := listIndexFromValue(key)
		if !ok || index < 0 || index >= len(target.List) {
			return vm.Null, false
		}
		return target.List[index], true
	case vm.ValueMap:
		if key.Kind == vm.ValueString {
			for candidate, value := range vm.StringValueMapEntries(target) {
				if strings.EqualFold(candidate, key.Text) {
					return value, true
				}
			}
		}
	}
	return readObjectMember(ctx, target, key.String())
}

func listIndexFromValue(value vm.Value) (int, bool) {
	switch value.Kind {
	case vm.ValueInt:
		return int(value.Int), true
	case vm.ValueDecimal:
		if math.Trunc(value.Decimal) != value.Decimal {
			return 0, false
		}
		return int(value.Decimal), true
	case vm.ValueString:
		parsed, err := strconv.Atoi(strings.TrimSpace(value.Text))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func readVisualforceExtraMember(ctx *ExpressionContext, value vm.Value, field string) (vm.Value, bool) {
	switch strings.ToLower(value.Type) {
	case "$objecttype":
		return visualforceObjectTypeValue(ctx, field)
	case "schema.sobjecttype":
		if strings.EqualFold(field, "fields") {
			objectName, ok := visualforceTokenObjectName(value)
			if !ok {
				return vm.Null, false
			}
			return visualforceObjectFieldsValue(ctx, objectName), true
		}
	case "schema.sobjectfieldmap":
		objectName, ok := visualforceTokenObjectName(value)
		if !ok {
			return vm.Null, false
		}
		return visualforceObjectFieldValue(ctx, objectName, field)
	case "$permission":
		return vm.Bool(visualforceCurrentUserHasPermission(ctx, field)), true
	case "$setup":
		return visualforceSetupValue(ctx, field)
	case "$component":
		return vm.String(field), true
	case "$remoteaction":
		path := visualforceRemoteActionPath(value)
		next := vm.Object("$RemoteAction")
		next.Fields["value"] = vm.String("{!" + path + "." + field + "}")
		return next, true
	}
	if next, ok := readObjectMember(ctx, value, field); ok {
		return next, true
	}
	return vm.Null, false
}

func visualforceRemoteActionPath(value vm.Value) string {
	path := strings.TrimSpace(value.String())
	path = strings.TrimPrefix(path, "{!")
	path = strings.TrimSuffix(path, "}")
	if path == "" || !strings.HasPrefix(strings.ToLower(path), "$remoteaction") {
		return "$RemoteAction"
	}
	return path
}

func visualforceObjectTypeValue(ctx *ExpressionContext, objectName string) (vm.Value, bool) {
	if ctx == nil || ctx.VM == nil || ctx.VM.Org == nil {
		return vm.Null, false
	}
	resolved, ok := storage.ResolveObjectName(*ctx.VM.Org, objectName)
	if !ok {
		return vm.Null, false
	}
	definition := ctx.VM.Org.Objects[resolved].Definition
	token := vm.Object("Schema.SObjectType")
	token.Fields["object"] = vm.String(resolved)
	token.Fields["Name"] = vm.String(resolved)
	token.Fields["Label"] = vm.String(firstNonBlank(definition.Label, resolved))
	token.Fields["KeyPrefix"] = vm.String(definition.KeyPrefix)
	return token, true
}

func visualforceObjectFieldsValue(ctx *ExpressionContext, objectName string) vm.Value {
	fields := vm.Object("Schema.SObjectFieldMap")
	fields.Fields["object"] = vm.String(objectName)
	if ctx == nil || ctx.VM == nil || ctx.VM.Org == nil {
		return fields
	}
	resolved, ok := storage.ResolveObjectName(*ctx.VM.Org, objectName)
	if !ok {
		return fields
	}
	definition := ctx.VM.Org.Objects[resolved].Definition
	for name, field := range definition.Fields {
		value := visualforceFieldToken(resolved, field)
		fields.Fields[name] = value
		fields.Fields[strings.ToLower(name)] = value
	}
	return fields
}

func visualforceObjectFieldValue(ctx *ExpressionContext, objectName, fieldName string) (vm.Value, bool) {
	if ctx == nil || ctx.VM == nil || ctx.VM.Org == nil {
		return vm.Null, false
	}
	resolvedObject, ok := storage.ResolveObjectName(*ctx.VM.Org, objectName)
	if !ok {
		return vm.Null, false
	}
	definition := ctx.VM.Org.Objects[resolvedObject].Definition
	resolvedField, ok := storage.ResolveFieldName(definition, ctx.VM.Org.Namespace, fieldName)
	if !ok {
		return vm.Null, false
	}
	return visualforceFieldToken(resolvedObject, definition.Fields[resolvedField]), true
}

func visualforceFieldToken(objectName string, field storage.Field) vm.Value {
	token := vm.Object("Schema.SObjectField")
	token.Fields["object"] = vm.String(objectName)
	token.Fields["field"] = vm.String(field.APIName)
	token.Fields["Name"] = vm.String(field.APIName)
	token.Fields["Label"] = vm.String(firstNonBlank(field.Label, field.APIName))
	token.Fields["Type"] = vm.String(string(field.Type))
	return token
}

func visualforceTokenObjectName(value vm.Value) (string, bool) {
	objectName, ok := value.Fields["object"]
	if !ok || objectName.Kind != vm.ValueString || strings.TrimSpace(objectName.Text) == "" {
		return "", false
	}
	return objectName.Text, true
}

func visualforceCurrentUserHasPermission(ctx *ExpressionContext, permission string) bool {
	if ctx == nil {
		return false
	}
	if record, ok := visualforceCurrentUserRecord(ctx); ok && visualforceStorageRecordHasPermission(record, permission) {
		return true
	}
	if ctx.VM == nil || ctx.VM.Org == nil {
		return false
	}
	userID := ""
	if record, ok := visualforceCurrentUserRecord(ctx); ok {
		userID = string(record.ID)
	}
	for _, permissionSetID := range visualforceAssignedPermissionSetIDs(ctx.VM.Org, userID) {
		if visualforcePermissionSetHasPermission(ctx.VM.Org, permissionSetID, permission) {
			return true
		}
	}
	return false
}

func visualforceStorageRecordHasPermission(record storage.Record, permission string) bool {
	for _, field := range []string{"Permissions", "PermissionSets", "CustomPermissions"} {
		value, ok := record.GetField(field)
		if !ok {
			continue
		}
		if visualforceStoragePermissionValueMatches(value, permission) {
			return true
		}
	}
	return false
}

func visualforceStoragePermissionValueMatches(value storage.Value, permission string) bool {
	if value.Kind == storage.ValueString && strings.EqualFold(strings.TrimSpace(value.String), strings.TrimSpace(permission)) {
		return true
	}
	if value.Kind == storage.ValueList {
		for _, item := range value.List {
			if visualforceStoragePermissionValueMatches(item, permission) {
				return true
			}
		}
	}
	return false
}

func visualforceAssignedPermissionSetIDs(org *storage.OrgState, userID string) []storage.ID {
	if org == nil || strings.TrimSpace(userID) == "" {
		return nil
	}
	assignments, ok := org.Objects["PermissionSetAssignment"]
	if !ok {
		return nil
	}
	var out []storage.ID
	for _, record := range assignments.Records {
		assignee, ok := record.GetField("AssigneeId")
		if !ok || !storageValueStringEqual(assignee, userID) {
			continue
		}
		permissionSet, ok := record.GetField("PermissionSetId")
		if !ok {
			continue
		}
		if id := storageIDText(permissionSet); id != "" {
			out = append(out, storage.ID(id))
		}
	}
	return out
}

func visualforcePermissionSetHasPermission(org *storage.OrgState, permissionSetID storage.ID, permission string) bool {
	if org == nil || permissionSetID == "" {
		return false
	}
	state, ok := org.Objects["PermissionSet"]
	if !ok {
		return false
	}
	record, ok := state.Records[permissionSetID]
	if !ok {
		return false
	}
	return visualforceStorageRecordHasPermission(record, permission)
}

func visualforceSetupValue(ctx *ExpressionContext, objectName string) (vm.Value, bool) {
	if ctx == nil || ctx.VM == nil || ctx.VM.Org == nil {
		return vm.Null, false
	}
	resolved, ok := storage.ResolveObjectName(*ctx.VM.Org, objectName)
	if !ok {
		return vm.Null, false
	}
	state := ctx.VM.Org.Objects[resolved]
	if !storage.IsCustomSettingDefinition(state.Definition) {
		return vm.Null, false
	}
	if len(state.Records) == 0 {
		return vm.Object(resolved), true
	}
	ids := make([]string, 0, len(state.Records))
	for id := range state.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	return vmValueFromStorageRecord(state.Records[storage.ID(ids[0])]), true
}

func visualforceSiteValue(ctx *ExpressionContext) vm.Value {
	site := vm.Object("Site")
	state, ok := visualforceObjectState(ctx, "Site")
	if !ok || len(state.Records) == 0 {
		return site
	}
	ids := make([]string, 0, len(state.Records))
	for id := range state.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	record := state.Records[storage.ID(ids[0])]
	site.Fields["Id"] = storageRecordFieldValue(record, "Id")
	for name := range record.Fields {
		site.Fields[name] = storageRecordFieldValue(record, name)
	}
	return site
}

func storageValueStringEqual(value storage.Value, text string) bool {
	return strings.EqualFold(storageIDText(value), strings.TrimSpace(text))
}

func storageIDText(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return strings.TrimSpace(value.String)
	default:
		return ""
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func evalArithmetic(op string, left, right vm.Value) vm.Value {
	leftNumber, leftOK := numericValue(left)
	rightNumber, rightOK := numericValue(right)
	if !leftOK || !rightOK {
		if op == "+" {
			return vm.String(left.String() + right.String())
		}
		return vm.Null
	}
	switch op {
	case "+":
		return numericResult(left, right, leftNumber+rightNumber)
	case "-":
		return numericResult(left, right, leftNumber-rightNumber)
	case "*":
		return numericResult(left, right, leftNumber*rightNumber)
	case "/":
		if rightNumber == 0 {
			return vm.Null
		}
		return numericResult(left, right, leftNumber/rightNumber)
	default:
		return vm.Null
	}
}

func numericResult(left, right vm.Value, value float64) vm.Value {
	if left.Kind == vm.ValueInt && right.Kind == vm.ValueInt && math.Trunc(value) == value {
		return vm.Int(int64(value))
	}
	return vm.Decimal(value)
}

func numericValue(value vm.Value) (float64, bool) {
	switch value.Kind {
	case vm.ValueInt:
		return float64(value.Int), true
	case vm.ValueDecimal:
		return value.Decimal, true
	case vm.ValueString:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value.Text), 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func compareValues(op string, left, right vm.Value) bool {
	if leftNumber, leftOK := numericValue(left); leftOK {
		if rightNumber, rightOK := numericValue(right); rightOK {
			return compareFloat(op, leftNumber, rightNumber)
		}
	}
	if op == "==" || op == "!=" {
		equal := valuesComparableEqual(left, right)
		if op == "!=" {
			return !equal
		}
		return equal
	}
	return compareString(op, left.String(), right.String())
}

func compareFloat(op string, left, right float64) bool {
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case ">=":
		return left >= right
	case "<":
		return left < right
	case "<=":
		return left <= right
	default:
		return false
	}
}

func compareString(op string, left, right string) bool {
	switch op {
	case ">":
		return left > right
	case ">=":
		return left >= right
	case "<":
		return left < right
	case "<=":
		return left <= right
	default:
		return false
	}
}

func valuesComparableEqual(left, right vm.Value) bool {
	if left.Kind == vm.ValueBool || right.Kind == vm.ValueBool {
		return isTruthy(&left) == isTruthy(&right)
	}
	return left.String() == right.String()
}
