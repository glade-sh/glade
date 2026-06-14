package visualforce

import (
	"html"
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
}

func renderFieldOutput(ctx *RenderContext, raw string) (string, bool) {
	binding, ok := resolveFieldBinding(ctx, raw)
	if !ok {
		return "", false
	}
	return html.EscapeString(fieldOutputText(binding)), true
}

func renderFieldInput(ctx *RenderContext, raw string, id string) (string, bool) {
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
		return `<input type="checkbox" class="inputField" name="` + html.EscapeString(name) + `"` + idAttr + ` value="true"` + checked + ` />`, true
	case FieldSelect:
		builder := strings.Builder{}
		builder.WriteString(`<select class="inputField" name="`)
		builder.WriteString(html.EscapeString(name))
		builder.WriteString(`"`)
		builder.WriteString(idAttr)
		builder.WriteString(`>`)
		valueText := storageValueText(binding.Value)
		for _, option := range binding.Field.PicklistValues {
			if !option.Active && strings.TrimSpace(option.Value) != valueText {
				continue
			}
			optionValue := strings.TrimSpace(option.Value)
			if optionValue == "" {
				continue
			}
			label := strings.TrimSpace(option.Label)
			if label == "" {
				label = optionValue
			}
			selected := ""
			if optionValue == valueText {
				selected = ` selected="selected"`
			}
			builder.WriteString(`<option value="`)
			builder.WriteString(html.EscapeString(optionValue))
			builder.WriteString(`"`)
			builder.WriteString(selected)
			builder.WriteString(`>`)
			builder.WriteString(html.EscapeString(label))
			builder.WriteString(`</option>`)
		}
		builder.WriteString(`</select>`)
		return builder.String(), true
	case FieldTextarea:
		return `<textarea class="inputField" name="` + html.EscapeString(name) + `"` + idAttr + fieldInputStateAttrs(binding.Field, true) + `>` + html.EscapeString(storageValueText(binding.Value)) + `</textarea>`, true
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
		return `<input type="` + inputType + `" class="inputField" name="` + html.EscapeString(name) + `"` + idAttr + ` value="` + html.EscapeString(storageValueText(binding.Value)) + `"` + fieldNumberAttrs(binding.Field) + fieldInputStateAttrs(binding.Field, binding.Kind != FieldCheckbox) + ` />`, true
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

func fieldInputStateAttrs(field storage.Field, allowRequired bool) string {
	attrs := strings.Builder{}
	if allowRequired && fieldIsRequired(field) {
		attrs.WriteString(` required="required"`)
	}
	if fieldIsReadonly(field) {
		attrs.WriteString(` readonly="readonly"`)
	}
	return attrs.String()
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
	resolvedField, ok := storage.ResolveFieldName(object.Definition, ctx.Project.Namespace, fieldName)
	if !ok {
		return FieldBinding{}, false
	}
	field := object.Definition.Fields[resolvedField]
	record, hasRecord := recordForFieldBinding(ctx, object)
	value := storage.Value{}
	if hasRecord {
		if stored, ok := record.GetField(resolvedField); ok {
			value = stored
		} else if defaultValue, ok := storage.DefaultValueForRecordField(object.Definition, record, field); ok {
			value = defaultValue
		}
	}
	return FieldBinding{ObjectName: objectKey, FieldName: resolvedField, Field: field, Value: value, Kind: fieldRenderKind(field), Record: record, HasRecord: hasRecord}, true
}

func recordForFieldBinding(ctx *RenderContext, object storage.ObjectState) (storage.Record, bool) {
	if recordID, ok := currentPageRecordID(ctx); ok {
		record, found := object.Records[storage.ID(recordID)]
		return record, found
	}
	return storage.Record{}, false
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

func fieldOutputText(binding FieldBinding) string {
	if value, ok := referenceDisplayText(binding); ok {
		return value
	}
	return storageValueText(binding.Value)
}

func referenceDisplayText(binding FieldBinding) (string, bool) {
	if !binding.HasRecord || !fieldIsReference(binding.Field) {
		return "", false
	}
	for _, relationship := range referenceRelationshipNames(binding.FieldName, binding.Field) {
		if value, ok := parentRelationshipDisplayText(binding.Record, relationship); ok {
			return value, true
		}
		if value, ok := flattenedRelationshipDisplayText(binding.Record, relationship); ok {
			return value, true
		}
	}
	return "", false
}

func fieldIsReference(field storage.Field) bool {
	return field.Type == storage.FieldReference || strings.EqualFold(strings.TrimSpace(field.DisplayType), "REFERENCE")
}

func referenceRelationshipNames(fieldName string, field storage.Field) []string {
	names := make([]string, 0, 3)
	if relationship := strings.TrimSpace(field.RelationshipName); relationship != "" {
		names = append(names, relationship)
	}
	fieldName = strings.TrimSpace(fieldName)
	if strings.HasSuffix(fieldName, "__c") {
		names = append(names, strings.TrimSuffix(fieldName, "__c")+"__r")
	} else if strings.HasSuffix(fieldName, "Id") && len(fieldName) > len("Id") {
		names = append(names, strings.TrimSuffix(fieldName, "Id"))
	}
	return uniqueStringsFold(names)
}

func uniqueStringsFold(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen := false
		for _, existing := range out {
			if strings.EqualFold(existing, value) {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, value)
		}
	}
	return out
}

func parentRelationshipDisplayText(record storage.Record, relationship string) (string, bool) {
	if record.ParentRelationships == nil {
		return "", false
	}
	for name, parent := range record.ParentRelationships {
		if !strings.EqualFold(name, relationship) {
			continue
		}
		return recordDisplayText(parent)
	}
	return "", false
}

func flattenedRelationshipDisplayText(record storage.Record, relationship string) (string, bool) {
	for _, field := range []string{relationship + ".Name", relationship + ".Id"} {
		if value, ok := record.GetField(field); ok {
			text := strings.TrimSpace(storageValueText(value))
			if text != "" {
				return text, true
			}
		}
	}
	return "", false
}

func recordDisplayText(record storage.Record) (string, bool) {
	for _, field := range []string{"Name", "DeveloperName", "Id"} {
		value, ok := record.GetField(field)
		if !ok && field == "Id" && record.ID != "" {
			value, ok = storage.IDValue(record.ID), true
		}
		if !ok {
			continue
		}
		text := strings.TrimSpace(storageValueText(value))
		if text != "" {
			return text, true
		}
	}
	return "", false
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
