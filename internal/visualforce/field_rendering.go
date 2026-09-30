package visualforce

import (
	"html"
	"net/url"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type FieldRenderKind string

const (
	FieldText     FieldRenderKind = "text"
	FieldCheckbox FieldRenderKind = "checkbox"
	FieldTextarea FieldRenderKind = "textarea"
	FieldSelect   FieldRenderKind = "select"
	FieldDate     FieldRenderKind = "date"
	FieldDatetime FieldRenderKind = "datetime"
	FieldEmail    FieldRenderKind = "email"
	FieldURL      FieldRenderKind = "url"
	FieldPhone    FieldRenderKind = "phone"
	FieldNumber   FieldRenderKind = "number"
)

type FieldBinding struct {
	ObjectName string
	FieldName  string
	Field      storage.Field
	Value      storage.Value
	Kind       FieldRenderKind
	Record     storage.Record
	HasRecord  bool
	Authorized bool
}

func renderFieldOutput(ctx *RenderContext, raw string) (string, bool) {
	binding, ok := resolveFieldBinding(ctx, raw)
	if !ok {
		return "", false
	}
	value, ownerTargetID := fieldOutputText(ctx, binding)
	text := html.EscapeString(value)
	href := fieldOutputHref(binding)
	if ownerTargetID != "" {
		href = "/record/User/" + url.PathEscape(string(ownerTargetID))
	}
	if href != "" {
		return `<a href="` + html.EscapeString(href) + `">` + text + `</a>`, true
	}
	return text, true
}

func fieldOutputHref(binding FieldBinding) string {
	value := storageValueText(binding.Value)
	if binding.Kind != FieldURL || strings.ContainsAny(value, "\\\r\n\t") {
		return ""
	}
	target, err := url.Parse(value)
	if err != nil || target.Hostname() == "" ||
		(!strings.EqualFold(target.Scheme, "http") && !strings.EqualFold(target.Scheme, "https")) {
		return ""
	}
	return value
}

func renderFieldInput(ctx *RenderContext, raw string, id string, required bool) (string, bool) {
	binding, ok := resolveFieldBinding(ctx, raw)
	if !ok {
		return "", false
	}
	name := fieldInputName(binding, "")
	idAttr := fieldInputIDAttr(id)
	switch binding.Kind {
	case FieldCheckbox:
		checked := ""
		if binding.Value.Kind == storage.ValueBoolean && binding.Value.Boolean {
			checked = ` checked="checked"`
		}
		escapedName := html.EscapeString(name)
		// The component attribute is distinct from schema-required metadata here:
		// putting required on a checkbox would make native HTML require it checked.
		requiredAttr := fieldInputRequiredAttr(required)
		return `<input type="hidden" name="` + escapedName + `" value="false" />` +
			`<input type="checkbox" class="inputField" name="` + escapedName + `"` + idAttr + ` value="true"` + requiredAttr + checked + ` />`, true
	case FieldSelect:
		builder := strings.Builder{}
		builder.WriteString(`<select class="inputField" name="`)
		builder.WriteString(html.EscapeString(name))
		builder.WriteString(`"`)
		builder.WriteString(idAttr)
		multiSelect := fieldIsMultiSelect(binding.Field)
		if multiSelect {
			builder.WriteString(` multiple="multiple"`)
		}
		builder.WriteString(fieldInputRequiredAttr(required))
		builder.WriteString(`>`)
		valueText := storageValueText(binding.Value)
		selectedValues := fieldSelectedValues(binding.Value)
		for _, option := range binding.Field.PicklistValues {
			optionValue := strings.TrimSpace(option.Value)
			selected := optionValue == valueText
			if multiSelect {
				selected = selectedValues[optionValue]
			}
			if !option.Active && !selected {
				continue
			}
			if optionValue == "" {
				continue
			}
			label := strings.TrimSpace(option.Label)
			if label == "" {
				label = optionValue
			}
			selectedAttr := ""
			if selected {
				selectedAttr = ` selected="selected"`
			}
			builder.WriteString(`<option value="`)
			builder.WriteString(html.EscapeString(optionValue))
			builder.WriteString(`"`)
			builder.WriteString(selectedAttr)
			builder.WriteString(`>`)
			builder.WriteString(html.EscapeString(label))
			builder.WriteString(`</option>`)
		}
		builder.WriteString(`</select>`)
		return builder.String(), true
	case FieldTextarea:
		return `<textarea class="inputField" name="` + html.EscapeString(name) + `"` + idAttr + fieldInputStateAttrs(binding.Field, true, required) + `>` + html.EscapeString(storageValueText(binding.Value)) + `</textarea>`, true
	default:
		inputType := "text"
		if binding.Kind == FieldDate {
			inputType = "date"
		}
		if binding.Kind == FieldDatetime {
			inputType = "datetime-local"
		}
		if binding.Kind == FieldEmail {
			inputType = "email"
		}
		if binding.Kind == FieldURL {
			inputType = "url"
		}
		if binding.Kind == FieldPhone {
			inputType = "tel"
		}
		if binding.Kind == FieldNumber {
			inputType = "number"
		}
		return `<input type="` + inputType + `" class="inputField" name="` + html.EscapeString(name) + `"` + idAttr + ` value="` + html.EscapeString(storageValueText(binding.Value)) + `"` + fieldNumberAttrs(binding.Field) + fieldInputStateAttrs(binding.Field, binding.Kind != FieldCheckbox, required) + ` />`, true
	}
}

func fieldInputIDAttr(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return ` id="` + html.EscapeString(id) + `"`
}

func fieldInputName(binding FieldBinding, submitted string) string {
	if submitted == "" || submitted == binding.FieldName {
		if binding.ObjectName != "" && binding.FieldName != "" {
			return binding.ObjectName + "." + binding.FieldName
		}
		return binding.FieldName
	}
	return submitted
}

func fieldInputStateAttrs(field storage.Field, allowRequired bool, required bool) string {
	attrs := strings.Builder{}
	if allowRequired {
		attrs.WriteString(fieldInputRequiredAttr(required || fieldIsRequired(field)))
	}
	if fieldIsReadonly(field) {
		attrs.WriteString(` readonly="readonly"`)
	}
	return attrs.String()
}

func fieldInputRequiredAttr(required bool) string {
	if !required {
		return ""
	}
	return ` required="required"`
}

func fieldIsRequired(field storage.Field) bool {
	if field.Required {
		return true
	}
	return field.Nillable != nil && !*field.Nillable && !storage.FieldFlagValue(field.DefaultedOnCreate, false) && !field.AutoNumber
}

func fieldIsReadonly(field storage.Field) bool {
	if field.AutoNumber {
		return true
	}
	switch field.Type {
	case storage.FieldID, storage.FieldCalculated, storage.FieldSummary:
		return true
	}
	if field.Createable != nil && !*field.Createable {
		return true
	}
	if field.Updateable != nil && !*field.Updateable {
		return true
	}
	return false
}

func fieldNumberAttrs(field storage.Field) string {
	if fieldRenderKind(field) != FieldNumber {
		return ""
	}
	displayType := strings.ToUpper(strings.TrimSpace(field.DisplayType))
	if field.Type == storage.FieldInteger || displayType == "INTEGER" {
		return ` step="1"`
	}
	if field.Scale > 0 {
		return ` step="` + html.EscapeString(decimalStep(field.Scale)) + `"`
	}
	return ` step="any"`
}

func fieldIsMultiSelect(field storage.Field) bool {
	return field.Type == storage.FieldMultiPicklist || strings.EqualFold(strings.TrimSpace(field.DisplayType), "MULTIPICKLIST")
}

func fieldSelectedValues(value storage.Value) map[string]bool {
	out := map[string]bool{}
	add := func(text string) {
		for _, part := range strings.Split(text, ";") {
			part = strings.TrimSpace(part)
			if part != "" {
				out[part] = true
			}
		}
	}
	if value.Kind == storage.ValueList {
		for _, item := range value.List {
			add(storageValueText(item))
		}
		return out
	}
	add(storageValueText(value))
	return out
}

func decimalStep(scale int) string {
	if scale <= 0 {
		return "any"
	}
	var builder strings.Builder
	builder.WriteString("0.")
	for i := 1; i < scale; i++ {
		builder.WriteByte('0')
	}
	builder.WriteByte('1')
	return builder.String()
}

func resolveFieldBinding(ctx *RenderContext, raw string) (FieldBinding, bool) {
	if ctx == nil || ctx.VM == nil || ctx.VM.Org == nil {
		return FieldBinding{}, false
	}
	expr := strings.TrimSpace(raw)
	if strings.HasPrefix(expr, "{!") && strings.HasSuffix(expr, "}") {
		expr = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(expr, "{!"), "}"))
	}
	parts := strings.Split(expr, ".")
	if len(parts) < 2 {
		return FieldBinding{}, false
	}
	objectName := strings.TrimSpace(parts[0])
	fieldName := strings.TrimSpace(parts[len(parts)-1])
	if objectName == "" || fieldName == "" {
		return FieldBinding{}, false
	}
	objectKey, ok := storage.ResolveObjectName(*ctx.VM.Org, objectName)
	if !ok {
		return FieldBinding{}, false
	}
	object := ctx.VM.Org.Objects[objectKey]
	resolvedField := "Id"
	field := storage.Field{APIName: "Id", Type: storage.FieldID}
	if !strings.EqualFold(fieldName, "Id") {
		var ok bool
		resolvedField, ok = storage.ResolveFieldName(object.Definition, ctx.Project.Namespace, fieldName)
		if !ok {
			return FieldBinding{ObjectName: objectKey, FieldName: fieldName}, true
		}
		field = object.Definition.Fields[resolvedField]
	}
	binding := FieldBinding{ObjectName: objectKey, FieldName: resolvedField, Field: field, Kind: fieldRenderKind(field)}
	if len(parts) != 2 {
		return binding, true
	}
	if !strings.EqualFold(resolvedField, "Name") && !strings.EqualFold(resolvedField, "Id") {
		if ctx.Expression == nil || strings.TrimSpace(ctx.PageMeta.RecordSetVar) != "" {
			return binding, true
		}
		controllerObject, matchesController := storage.ResolveObjectName(*ctx.VM.Org, ctx.PageMeta.StandardController)
		if !matchesController || !strings.EqualFold(controllerObject, objectKey) {
			return binding, true
		}
		_, authorized := ctx.Expression.AuthorizedStandardFields[strings.ToLower(resolvedField)]
		if !authorized {
			return binding, true
		}
		binding.Authorized = true
		recordID, ok := currentPageRecordID(ctx)
		if !ok {
			return binding, true
		}
		record := ctx.Expression.StandardController.Fields["record"]
		storedID := record.Fields["Id"].Fields["value"]
		if record.Kind != vm.ValueObject || !strings.EqualFold(record.Type, objectKey) ||
			storedID.Kind != vm.ValueString || storedID.Text != recordID {
			return binding, true
		}
		current, present := record.Fields[resolvedField]
		if !present {
			return binding, true
		}
		value, convertible := visualforceFieldStorageValue(current)
		if !convertible {
			return binding, true
		}
		binding.Value = value
		binding.Record = storage.Record{ID: storage.ID(recordID), Object: objectKey, Fields: map[string]storage.Value{resolvedField: value}}
		binding.HasRecord = true
		return binding, true
	}
	record, hasRecord, err := recordForFieldBinding(ctx, objectKey)
	if err != nil || !hasRecord {
		return binding, true
	}
	binding.Record = record
	binding.HasRecord = true
	if strings.EqualFold(resolvedField, "Id") {
		binding.Value = storage.IDValue(record.ID)
	} else if value, ok := record.GetField(resolvedField); ok {
		binding.Value = value
	}
	return binding, true
}

func visualforceFieldStorageValue(value vm.Value) (storage.Value, bool) {
	switch value.Kind {
	case vm.ValueNull:
		return storage.NullValue(), true
	case vm.ValueString:
		return storage.StringValue(value.Text), true
	case vm.ValueInt:
		return storage.IntegerValue(value.Int), true
	case vm.ValueDecimal:
		return storage.DecimalValue(value.String()), true
	case vm.ValueBool:
		return storage.BooleanValue(value.Bool), true
	case vm.ValueObject:
		raw := value.Fields["value"]
		if raw.Kind != vm.ValueString {
			return storage.Value{}, false
		}
		switch strings.ToLower(value.Type) {
		case "id":
			return storage.IDValue(storage.ID(raw.Text)), true
		case "date":
			return storage.DateValue(raw.Text), true
		case "datetime":
			return storage.DateTimeValue(raw.Text), true
		case "blob":
			return storage.BlobValue(raw.Text), true
		}
	case vm.ValueList:
		items := make([]storage.Value, 0, len(value.List))
		for _, item := range value.List {
			converted, ok := visualforceFieldStorageValue(item)
			if !ok {
				return storage.Value{}, false
			}
			items = append(items, converted)
		}
		return storage.ListValue(items...), true
	}
	return storage.Value{}, false
}

func recordForFieldBinding(ctx *RenderContext, objectName string) (storage.Record, bool, error) {
	if recordID, ok := currentPageRecordID(ctx); ok {
		return ctx.VM.ReadVisualforceRecord(objectName, storage.ID(recordID))
	}
	return storage.Record{}, false, nil
}

func currentPageRecordID(ctx *RenderContext) (string, bool) {
	if ctx == nil {
		return "", false
	}
	page := vm.Null
	if ctx.Expression != nil {
		page = ctx.Expression.CurrentPage
	}
	if page.Kind == vm.ValueNull && ctx.VM != nil {
		page = ctx.VM.CurrentPage()
	}
	return pageParameterString(page, "id")
}

func pageParameterString(page vm.Value, name string) (string, bool) {
	if page.Kind != vm.ValueObject {
		return "", false
	}
	params, ok := page.Fields["parameters"]
	if !ok || params.Kind != vm.ValueMap {
		return "", false
	}
	for key, value := range params.Map {
		if value.Kind != vm.ValueString {
			continue
		}
		rawKey := key
		if original, ok := params.MapKeys[key]; ok && original.Kind == vm.ValueString {
			rawKey = original.Text
		}
		if !strings.EqualFold(rawKey, name) {
			continue
		}
		text := strings.TrimSpace(value.Text)
		if text == "" {
			return "", false
		}
		return text, true
	}
	return "", false
}

func fieldRenderKind(field storage.Field) FieldRenderKind {
	switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
	case "TEXTAREA", "LONGTEXTAREA", "HTML":
		return FieldTextarea
	case "EMAIL":
		return FieldEmail
	case "URL":
		return FieldURL
	case "PHONE":
		return FieldPhone
	case "INTEGER", "DOUBLE", "CURRENCY", "PERCENT":
		return FieldNumber
	case "BOOLEAN":
		return FieldCheckbox
	case "PICKLIST", "MULTIPICKLIST":
		return FieldSelect
	case "DATE":
		return FieldDate
	case "DATETIME":
		return FieldDatetime
	}
	switch field.Type {
	case storage.FieldBoolean:
		return FieldCheckbox
	case storage.FieldPicklist, storage.FieldMultiPicklist:
		return FieldSelect
	case storage.FieldDate:
		return FieldDate
	case storage.FieldDateTime:
		return FieldDatetime
	case storage.FieldInteger, storage.FieldDecimal:
		return FieldNumber
	default:
		return FieldText
	}
}

func fieldOutputText(ctx *RenderContext, binding FieldBinding) (string, storage.ID) {
	if value, targetID := ownerDisplayText(ctx, binding); targetID != "" {
		return value, targetID
	}
	// A denied or missing target never supplies a label. The authorized source
	// field's ID remains plain escaped text, without a guessed record route.
	return storageValueText(binding.Value), ""
}

func ownerDisplayText(ctx *RenderContext, binding FieldBinding) (string, storage.ID) {
	if ctx == nil || ctx.VM == nil || ctx.VM.Org == nil || ctx.Expression == nil ||
		!strings.EqualFold(binding.FieldName, "OwnerId") || !binding.Authorized || !binding.HasRecord ||
		!fieldIsReference(binding.Field) || binding.Value.Kind != storage.ValueID {
		return "", ""
	}
	projected, ok := ctx.Expression.AuthorizedStandardFields[strings.ToLower(binding.FieldName)]
	if !ok || projected.Kind != storage.ValueID || !storage.IDsEqual(projected.ID, binding.Value.ID) {
		return "", ""
	}
	var userObject string
	for _, candidate := range binding.Field.ReferenceTo {
		resolved, ok := storage.ResolveObjectName(*ctx.VM.Org, candidate)
		if ok && strings.EqualFold(resolved, "User") {
			userObject = resolved
			break
		}
	}
	if userObject == "" {
		return "", ""
	}
	user := ctx.VM.Org.Objects[userObject]
	prefix := strings.TrimSpace(user.Definition.KeyPrefix)
	if prefix == "" || !strings.HasPrefix(string(binding.Value.ID), prefix) {
		return "", ""
	}
	target, found, err := ctx.VM.ReadVisualforceRecordFields(userObject, binding.Value.ID, []string{"Name"})
	if err != nil || !found {
		return "", ""
	}
	name, ok := target.GetField("Name")
	if !ok || name.Kind != storage.ValueString || strings.TrimSpace(name.String) == "" {
		return "", ""
	}
	return name.String, target.ID
}

func fieldIsReference(field storage.Field) bool {
	return field.Type == storage.FieldReference || strings.EqualFold(strings.TrimSpace(field.DisplayType), "REFERENCE")
}

func storageValueText(value storage.Value) string {
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueInteger:
		return strconvFormatInt(value.Integer)
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueBoolean:
		if value.Boolean {
			return "true"
		}
		return "false"
	case storage.ValueList:
		parts := make([]string, 0, len(value.List))
		for _, item := range value.List {
			if text := strings.TrimSpace(storageValueText(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ";")
	default:
		return ""
	}
}

func strconvFormatInt(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
