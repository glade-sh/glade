package vm

import (
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// FORMAT without an alias writes the selected field's slot. Explicit aliases
// remain extra slots. Native sobject_json2 and sobject_json3 rows cover en_US
// and fr_FR query projections, respectively.
func (vm *VM) applyJSONQueryFunctions(value *Value, fields []string) {
	for _, expression := range fields {
		parts := strings.Fields(expression)
		if len(parts) == 0 {
			continue
		}
		raw := parts[0]
		open := strings.IndexByte(raw, '(')
		if open < 0 || !strings.HasSuffix(raw, ")") {
			continue
		}
		function := raw[:open]
		selected := raw[open+1 : len(raw)-1]
		name := raw
		if len(parts) == 2 {
			name = parts[1]
		}
		if strings.EqualFold(function, "TOLABEL") {
			// Resolve root qualification and walk the selected parent path.
			path := strings.Split(selected, ".")
			if len(path) > 1 && vm.soqlFieldQualifierMatchesObject(value.Type, path[0]) {
				path = path[1:]
				selected = strings.Join(path, ".")
				if len(parts) == 1 {
					name = "toLabel(" + selected + ")"
				}
			}
			if len(path) > 1 {
				parentName := path[len(path)-1]
				if len(parts) == 2 {
					parentName = name
				}
				vm.applyRelationshipStandardValueLabel(value, path, parentName)
				continue
			}
			// A selected null picklist alias remains readable through SObject.get,
			// but is absent from JSON. The query executor may omit its null slot.
			if len(parts) == 2 && !strings.Contains(selected, ".") {
				_, field, known := vm.jsonActiveSObjectField(value.Type, selected)
				if _, _, present := objectFieldValue(*value, name); known && field.Type == storage.FieldPicklist && !present {
					value.Fields[name] = Null
				}
			}
			vm.applyStandardValueLabel(value, selected, name)
			continue
		}
		if !strings.EqualFold(function, "FORMAT") {
			continue
		}
		locale := vm.currentUserInfoField("LocaleSidKey", "en_US")
		_, field, hasField := vm.jsonActiveSObjectField(value.Type, selected)
		actual, item, present := objectFieldValue(*value, name)
		usdSingleCurrency := func() bool {
			return vm.currentUserInfoField("DefaultCurrencyIsoCode", "USD") == "USD" && (vm.Org == nil || !vm.Org.HasFeature("MultiCurrency"))
		}
		if !present {
			// The French null-currency alias is an empty string in the captured
			// API 62/67 query results, even with suppressNulls enabled.
			if locale == "fr_FR" && len(parts) == 2 && hasField && strings.EqualFold(field.DisplayType, "CURRENCY") && usdSingleCurrency() {
				value.Fields[name] = String("")
			}
			continue
		}
		if len(parts) == 1 {
			delete(value.Fields, actual)
			name = vm.resolveSObjectFieldName(value.Type, selected)
		}
		if locale == "fr_FR" && len(parts) == 2 && hasField && strings.EqualFold(field.DisplayType, "CURRENCY") && item.Kind == ValueNull && usdSingleCurrency() {
			item = String("")
		}
		if locale == "fr_FR" && hasField && strings.EqualFold(field.DisplayType, "DATE") && item.Kind == ValueString {
			if date, err := time.Parse("1/2/2006", item.Text); err == nil {
				item = String(date.Format("02/01/2006"))
			}
		}
		if (locale == "en_US" || locale == "fr_FR") && usdSingleCurrency() {
			if hasField && strings.EqualFold(field.DisplayType, "CURRENCY") && item.Kind == ValueString {
				if decimal, err := decimalFromText(item.Text); err == nil {
					if rounded, err := roundDecimalValueToScale("FORMAT", decimal, int64(field.Scale), "HALF_UP"); err == nil {
						text := decimalDisplayText(rounded)
						if locale == "fr_FR" {
							text = decimalPlainText(rounded)
						}
						text = formatDecimalTextWithGrouping(text)
						if locale == "fr_FR" && strings.HasPrefix(text, "-") {
							item = String("-$" + strings.TrimPrefix(text, "-"))
						} else {
							item = String("$" + text)
						}
					}
				}
			}
		}
		value.Fields[name] = item
	}
}

// applyRelationshipStandardValueLabel walks materialized parent values without
// changing null or absent relationships.
func (vm *VM) applyRelationshipStandardValueLabel(value *Value, path []string, name string) {
	if len(path) == 1 {
		vm.applyStandardValueLabel(value, path[0], name)
		return
	}
	actual, parent, present := objectFieldValue(*value, path[0])
	if !present || parent.Kind != ValueObject {
		return
	}
	vm.applyRelationshipStandardValueLabel(&parent, path[1:], name)
	value.Fields[actual] = parent
}

// Standard value set bindings must be explicit: field names alone do not
// identify the metadata value set. Industry and OpportunityStage are covered
// by boundary-fr-org.tsv and boundary-de-org.tsv; the three Status bindings
// are also used by schema.bindProjectStandardValueSets.
func (vm *VM) applyStandardValueLabel(value *Value, selected, name string) {
	if vm.Org == nil {
		return
	}
	var valueSet string
	switch {
	case strings.EqualFold(value.Type, "Account") && strings.EqualFold(selected, "Industry"):
		valueSet = "Industry"
	case strings.EqualFold(value.Type, "Opportunity") && strings.EqualFold(selected, "StageName"):
		valueSet = "OpportunityStage"
	case strings.EqualFold(selected, "Status") && strings.EqualFold(value.Type, "Contract"):
		valueSet = "ContractStatus"
	case strings.EqualFold(selected, "Status") && strings.EqualFold(value.Type, "Order"):
		valueSet = "OrderStatus"
	case strings.EqualFold(selected, "Status") && strings.EqualFold(value.Type, "Task"):
		valueSet = "TaskStatus"
	default:
		return
	}
	_, field, ok := vm.jsonActiveSObjectField(value.Type, selected)
	if !ok || field.Type != storage.FieldPicklist {
		return
	}
	language := vm.currentUserInfoField("LanguageLocaleKey", "en_US")
	if !vm.Org.HasFeature("TranslationLanguage-" + language) {
		return
	}
	actual, item, present := objectFieldValue(*value, name)
	if !present && strings.EqualFold(name, "toLabel("+selected+")") {
		// An unaliased toLabel projection is stored in the source field slot.
		actual, item, present = objectFieldValue(*value, selected)
	}
	if !present || item.Kind != ValueString {
		return
	}
	// The SOQL executor already projects the master label for toLabel.
	// Looking it up as an API value again can select a different option.
	masterLabel := item.Text
	for _, translation := range vm.Org.Metadata.ValueSetTranslations {
		if strings.EqualFold(translation.ValueSet, valueSet) && strings.EqualFold(translation.Language, language) && translation.MasterLabel == masterLabel {
			value.Fields[actual] = String(translation.Translation)
			return
		}
	}
}
