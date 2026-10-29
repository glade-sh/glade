package vm

import "strings"

// FORMAT without an alias writes the selected field's slot. Explicit aliases
// remain extra slots. Native format/formatAlias rows use en_US currency output;
// other locales retain the existing formatter until they are captured.
func (vm *VM) applyJSONQueryFunctions(value *Value, fields []string) {
	for _, expression := range fields {
		parts := strings.Fields(expression)
		if len(parts) == 0 {
			continue
		}
		raw := parts[0]
		open := strings.IndexByte(raw, '(')
		if open < 0 || !strings.HasSuffix(raw, ")") || !strings.EqualFold(raw[:open], "FORMAT") {
			continue
		}
		selected := raw[open+1 : len(raw)-1]
		name := raw
		if len(parts) == 2 {
			name = parts[1]
		}
		actual, item, present := objectFieldValue(*value, name)
		if !present {
			continue
		}
		if len(parts) == 1 {
			delete(value.Fields, actual)
			name = vm.resolveSObjectFieldName(value.Type, selected)
		}
		if vm.currentUserInfoField("LocaleSidKey", "en_US") == "en_US" && vm.currentUserInfoField("DefaultCurrencyIsoCode", "USD") == "USD" && (vm.Org == nil || !vm.Org.HasFeature("MultiCurrency")) {
			if _, field, ok := vm.jsonActiveSObjectField(value.Type, selected); ok && strings.EqualFold(field.DisplayType, "CURRENCY") && item.Kind == ValueString {
				if decimal, err := decimalFromText(item.Text); err == nil {
					if rounded, err := roundDecimalValueToScale("FORMAT", decimal, int64(field.Scale), "HALF_UP"); err == nil {
						item = String("$" + formatDecimalTextWithGrouping(decimalDisplayText(rounded)))
					}
				}
			}
		}
		value.Fields[name] = item
	}
}
