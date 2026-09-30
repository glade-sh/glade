package visualforce

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type FormBinding struct {
	SubmittedName string
	FieldName     string
	Value         string
}

type FormValueConversionDiagnostic struct {
	FieldName string
	FieldType string
	RawValue  string
	Reason    string
	Message   string
}

type visualforceParamAssignment struct {
	SubmittedName string
	TargetName    string
}

func visualforceParamAssignments(root *MarkupNode, action string) ([]visualforceParamAssignment, error) {
	if root == nil || strings.TrimSpace(action) == "" {
		return nil, nil
	}
	var out []visualforceParamAssignment
	var walk func(*MarkupNode) error
	walk = func(node *MarkupNode) error {
		if node == nil {
			return nil
		}
		if node.Type == MarkupNodeElement && strings.EqualFold(node.Namespace, "apex") &&
			strings.TrimSpace(node.Attribute("action")) != "" &&
			strings.EqualFold(actionMethodName(node.Attribute("action")), actionMethodName(action)) {
			for _, child := range node.Children {
				if child == nil || child.Type != MarkupNodeElement || !strings.EqualFold(child.Namespace, "apex") || !strings.EqualFold(child.Name, "param") {
					continue
				}
				submitted := strings.TrimSpace(child.Attribute("name"))
				if submitted == "" || strings.TrimSpace(child.Attribute("assignTo")) == "" {
					continue
				}
				target, err := visualforceAssignmentTarget(child.Attribute("assignTo"))
				if err != nil {
					return fmt.Errorf("apex:param %q assignTo: %w", submitted, err)
				}
				candidate := visualforceParamAssignment{SubmittedName: submitted, TargetName: target}
				duplicate := false
				for _, existing := range out {
					if strings.EqualFold(existing.SubmittedName, submitted) {
						if !strings.EqualFold(existing.TargetName, target) {
							return fmt.Errorf("apex:param name %q maps to multiple assignTo properties for action %q", submitted, action)
						}
						duplicate = true
						break
					}
				}
				if !duplicate {
					out = append(out, candidate)
				}
			}
		}
		for _, child := range node.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	return out, nil
}

func visualforceAssignmentTarget(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if strings.HasPrefix(name, "{!") || strings.HasSuffix(name, "}") {
		if !strings.HasPrefix(name, "{!") || !strings.HasSuffix(name, "}") {
			return "", fmt.Errorf("malformed assignment expression %q", raw)
		}
		name = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(name, "{!"), "}"))
	}
	if name == "" || strings.Contains(name, ".") {
		return "", fmt.Errorf("only a single controller property name is supported, got %q", raw)
	}
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' && r != '$' {
				return "", fmt.Errorf("invalid controller property name %q", raw)
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$' {
			return "", fmt.Errorf("invalid controller property name %q", raw)
		}
	}
	return name, nil
}

func visualforceTypesEqual(left, right string) bool {
	normalize := func(value string) string {
		value = strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), ""))
		return strings.TrimPrefix(value, "system.")
	}
	return normalize(left) == normalize(right)
}

func visualforceAssignmentValue(raw, typeName, fieldName string) (vm.Value, error) {
	var field *storage.Field
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(typeName)), "system.") {
	case "boolean":
		field = &storage.Field{Type: storage.FieldBoolean}
	case "integer", "long":
		field = &storage.Field{Type: storage.FieldInteger}
	case "decimal", "double":
		field = &storage.Field{Type: storage.FieldDecimal}
	}
	value, diagnostic := visualforceTypedFormValueWithDiagnostic(raw, vm.Null, field, fieldName)
	if diagnostic != nil {
		return vm.Null, fmt.Errorf("%s: %s", fieldName, diagnostic.Message)
	}
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(typeName)), "system.") {
	case "long":
		value.Type = "Long"
	case "double":
		value.Static = "Double"
	}
	return value, nil
}

func visualforceFormValuesWithoutParamAssignments(values map[string]string, assignments []visualforceParamAssignment) map[string]string {
	if len(values) == 0 || len(assignments) == 0 {
		return values
	}
	excluded := make(map[string]bool, len(assignments)*2)
	for _, assignment := range assignments {
		if _, submitted, _ := visualforceParamSubmittedValue(values, assignment.SubmittedName); !submitted {
			continue
		}
		excluded[strings.ToLower(formFieldBindingName(assignment.SubmittedName))] = true
		excluded[strings.ToLower(formFieldBindingName(assignment.TargetName))] = true
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		if !excluded[strings.ToLower(formFieldBindingName(key))] {
			out[key] = value
		}
	}
	return out
}

func visualforceParamSubmittedValue(values map[string]string, name string) (string, bool, error) {
	wanted := strings.ToLower(formFieldBindingName(name))
	var value string
	found := false
	for key, candidate := range values {
		if strings.ToLower(formFieldBindingName(key)) != wanted {
			continue
		}
		if found {
			return "", false, fmt.Errorf("multiple submitted form keys resolve to apex:param name %q", name)
		}
		value, found = candidate, true
	}
	return value, found, nil
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
		case "param":
			addVisualforceFormFieldName(out, node.Attribute("name"))
			addVisualforceFormFieldName(out, expressionFieldName(node.Attribute("assignTo")))
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
	value, _ := visualforceTypedFormValueWithDiagnostic(raw, existing, field, "")
	return value
}

func visualforceTypedFormValueWithDiagnostic(raw string, existing vm.Value, field *storage.Field, fieldName string) (vm.Value, *FormValueConversionDiagnostic) {
	if value, ok := visualforceFormValueFromExisting(raw, existing); ok {
		return value, nil
	}
	if target := visualforceExistingTargetName(existing); target != "" {
		return vm.String(raw), visualforceConversionDiagnostic(fieldName, target, raw, "could not convert submitted Visualforce value")
	}
	if field == nil {
		return vm.String(raw), nil
	}
	if value, ok := visualforceFormValueFromField(raw, *field); ok {
		return value, nil
	}
	if visualforceFieldTargetName(*field) != "" {
		return vm.String(raw), visualforceConversionDiagnostic(fieldName, visualforceFieldTargetName(*field), raw, "could not convert submitted Visualforce value")
	}
	return vm.String(raw), nil
}

func visualforceFormValueFromExisting(raw string, existing vm.Value) (vm.Value, bool) {
	switch existing.Kind {
	case vm.ValueBool:
		return parseVisualforceFormBool(raw)
	case vm.ValueInt:
		return parseVisualforceFormInt(raw)
	case vm.ValueDecimal:
		return parseVisualforceFormDecimal(raw)
	case vm.ValueList:
		return parseVisualforceFormStringList(raw)
	case vm.ValueSet:
		return parseVisualforceFormStringSet(raw)
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
	case storage.FieldMultiPicklist:
		return parseVisualforceFormMultiPicklist(raw)
	case storage.FieldID, storage.FieldReference:
		return vmPlatformScalar("Id", strings.TrimSpace(raw)), true
	default:
		return vm.Null, false
	}
}

func parseVisualforceFormBool(raw string) (vm.Value, bool) {
	text := strings.TrimSpace(raw)
	switch {
	case text == "":
		return vm.Bool(false), true
	case strings.EqualFold(text, "on"), strings.EqualFold(text, "checked"), strings.EqualFold(text, "yes"):
		return vm.Bool(true), true
	case strings.EqualFold(text, "off"), strings.EqualFold(text, "unchecked"), strings.EqualFold(text, "no"):
		return vm.Bool(false), true
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
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05-0700"} {
		if _, err := time.Parse(layout, text); err == nil {
			return vmPlatformScalar("Datetime", text), true
		}
	}
	return vm.Null, false
}

func parseVisualforceFormStringList(raw string) (vm.Value, bool) {
	values := visualforceSplitMultiValue(raw)
	out := make([]vm.Value, 0, len(values))
	for _, value := range values {
		out = append(out, vm.String(value))
	}
	return vm.List(out...), true
}

func parseVisualforceFormStringSet(raw string) (vm.Value, bool) {
	values := visualforceSplitMultiValue(raw)
	out := make([]vm.Value, 0, len(values))
	for _, value := range values {
		out = append(out, vm.String(value))
	}
	return vm.Set(out...), true
}

func parseVisualforceFormMultiPicklist(raw string) (vm.Value, bool) {
	return vm.String(strings.Join(visualforceSplitMultiValue(raw), ";")), true
}

func visualforceSplitMultiValue(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func visualforceExistingTargetName(existing vm.Value) string {
	switch existing.Kind {
	case vm.ValueBool:
		return "BOOLEAN"
	case vm.ValueInt:
		return "INTEGER"
	case vm.ValueDecimal:
		return "DECIMAL"
	case vm.ValueList:
		return "LIST"
	case vm.ValueSet:
		return "SET"
	case vm.ValueObject:
		switch strings.ToLower(strings.TrimSpace(existing.Type)) {
		case "date":
			return "DATE"
		case "datetime":
			return "DATETIME"
		case "id":
			return "ID"
		}
	}
	return ""
}

func visualforceFieldTargetName(field storage.Field) string {
	switch field.Type {
	case storage.FieldBoolean, storage.FieldInteger, storage.FieldDecimal, storage.FieldDate, storage.FieldDateTime, storage.FieldMultiPicklist, storage.FieldID, storage.FieldReference:
		return string(field.Type)
	default:
		return ""
	}
}

func visualforceConversionDiagnostic(fieldName, fieldType, raw, reason string) *FormValueConversionDiagnostic {
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		fieldName = "<unknown>"
	}
	fieldType = strings.TrimSpace(fieldType)
	if fieldType == "" {
		fieldType = "<unknown>"
	}
	message := "Visualforce form value " + strconv.Quote(raw) + " for " + fieldName + " could not be converted to " + fieldType + ": " + reason
	return &FormValueConversionDiagnostic{
		FieldName: fieldName,
		FieldType: fieldType,
		RawValue:  raw,
		Reason:    reason,
		Message:   message,
	}
}
