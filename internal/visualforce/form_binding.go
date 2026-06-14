package visualforce

import (
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type FormBinding struct {
	SubmittedName string
	FieldName     string
	Value         string
}

func VisualforceFormBindings(values map[string]string) []FormBinding {
	return VisualforceFormBindingsForFields(values, nil)
}

func VisualforceFormBindingsForFields(values map[string]string, allowedFields map[string]bool) []FormBinding {
	if len(values) == 0 {
		return nil
	}
	out := make([]FormBinding, 0, len(values))
	for key, value := range values {
		if strings.TrimSpace(key) == "" || strings.HasPrefix(key, "__") || key == viewStateFieldName {
			continue
		}
		field := formFieldBindingName(key)
		if field == "" {
			continue
		}
		if allowedFields != nil && !allowedFields[key] && !allowedFields[field] {
			continue
		}
		out = append(out, FormBinding{SubmittedName: key, FieldName: field, Value: value})
	}
	return out
}

func VisualforceFormFieldNames(root *MarkupNode) map[string]bool {
	out := map[string]bool{}
	collectVisualforceFormFieldNames(root, out)
	return out
}

func collectVisualforceFormFieldNames(node *MarkupNode, out map[string]bool) {
	if node == nil {
		return
	}
	if node.Type == MarkupNodeElement && strings.EqualFold(node.Namespace, "apex") {
		switch strings.ToLower(node.Name) {
		case "inputtext", "inputsecret", "inputhidden", "inputtextarea", "inputcheckbox", "inputfield":
			addVisualforceFormFieldName(out, inputFieldName(node))
		case "selectlist", "selectcheckboxes", "selectradio":
			addVisualforceFormFieldName(out, fieldName(node))
		case "inputfile":
			addVisualforceFormFieldName(out, inputFileUploadFieldName(node))
			addVisualforceFormFieldName(out, expressionFieldName(node.Attribute("value")))
			addVisualforceFormFieldName(out, expressionFieldName(node.Attribute("filename")))
			addVisualforceFormFieldName(out, expressionFieldName(node.Attribute("contenttype")))
			addVisualforceFormFieldName(out, expressionFieldName(inputFileUploadSizeAttribute(node)))
		}
	}
	for _, child := range node.Children {
		collectVisualforceFormFieldNames(child, out)
	}
}

func addVisualforceFormFieldName(out map[string]bool, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	out[name] = true
	out[formFieldBindingName(name)] = true
}

func visualforceTypedFormValue(raw string, existing vm.Value, field *storage.Field) vm.Value {
	if value, ok := visualforceFormValueFromExisting(raw, existing); ok {
		return value
	}
	if field == nil {
		return vm.String(raw)
	}
	if value, ok := visualforceFormValueFromField(raw, *field); ok {
		return value
	}
	return vm.String(raw)
}

func visualforceFormValueFromExisting(raw string, existing vm.Value) (vm.Value, bool) {
	switch existing.Kind {
	case vm.ValueBool:
		return parseVisualforceFormBool(raw)
	case vm.ValueInt:
		return parseVisualforceFormInt(raw)
	case vm.ValueDecimal:
		return parseVisualforceFormDecimal(raw)
	case vm.ValueObject:
		switch strings.ToLower(existing.Type) {
		case "date":
			return parseVisualforceFormDate(raw)
		case "datetime":
			return parseVisualforceFormDateTime(raw)
		case "id":
			return vmPlatformScalar("Id", strings.TrimSpace(raw)), true
		}
	}
	return vm.Null, false
}

func visualforceFormValueFromField(raw string, field storage.Field) (vm.Value, bool) {
	switch field.Type {
	case storage.FieldBoolean:
		return parseVisualforceFormBool(raw)
	case storage.FieldInteger:
		return parseVisualforceFormInt(raw)
	case storage.FieldDecimal:
		return parseVisualforceFormDecimal(raw)
	case storage.FieldDate:
		return parseVisualforceFormDate(raw)
	case storage.FieldDateTime:
		return parseVisualforceFormDateTime(raw)
	case storage.FieldID, storage.FieldReference:
		return vmPlatformScalar("Id", strings.TrimSpace(raw)), true
	default:
		return vm.Null, false
	}
}

func parseVisualforceFormBool(raw string) (vm.Value, bool) {
	text := strings.TrimSpace(raw)
	if strings.EqualFold(text, "on") {
		return vm.Bool(true), true
	}
	parsed, err := strconv.ParseBool(text)
	if err != nil {
		return vm.Null, false
	}
	return vm.Bool(parsed), true
}

func parseVisualforceFormInt(raw string) (vm.Value, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return vm.Null, false
	}
	return vm.Int(parsed), true
}

func parseVisualforceFormDecimal(raw string) (vm.Value, bool) {
	text := strings.TrimSpace(raw)
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return vm.Null, false
	}
	value := vm.Decimal(parsed)
	value.Text = text
	return value, true
}

func parseVisualforceFormDate(raw string) (vm.Value, bool) {
	text := strings.TrimSpace(raw)
	if _, err := time.Parse("2006-01-02", text); err != nil {
		return vm.Null, false
	}
	return vmPlatformScalar("Date", text), true
}

func parseVisualforceFormDateTime(raw string) (vm.Value, bool) {
	text := strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05-0700"} {
		if _, err := time.Parse(layout, text); err == nil {
			return vmPlatformScalar("Datetime", text), true
		}
	}
	return vm.Null, false
}
