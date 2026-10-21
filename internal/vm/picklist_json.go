package vm

import (
	"encoding/base64"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

const picklistJSONValidForField = "__glade_picklist_valid_for"

// Larger and inactive controller sets retain their existing serialization until
// their bitmap boundary is established. The observed active sets reach 101.
const picklistJSONControllerLimit = 101

func picklistJSONValidFor(controller storage.Field, field storage.Field, entry storage.PicklistValue, hasController bool) (Value, bool) {
	if field.Type != storage.FieldPicklist || !entry.Active {
		return Null, false
	}
	if !hasController {
		return Null, true
	}
	if controller.Type != storage.FieldPicklist || len(controller.PicklistValues) == 0 || len(controller.PicklistValues) > picklistJSONControllerLimit {
		return Null, false
	}
	positions := make(map[string]int, len(controller.PicklistValues))
	for i, value := range controller.PicklistValues {
		if !value.Active || value.Value == "" {
			return Null, false
		}
		if _, duplicate := positions[value.Value]; duplicate {
			return Null, false
		}
		positions[value.Value] = i
	}
	var selected *storage.PicklistSetting
	for i := range field.PicklistValueSettings {
		setting := &field.PicklistValueSettings[i]
		if setting.ValueName == entry.Value {
			if selected != nil {
				return Null, false
			}
			selected = setting
		}
	}
	if selected == nil {
		return String(""), true
	}
	if len(selected.ControllingFieldValues) == 0 {
		return Null, false
	}
	indices := make([]int, 0, len(selected.ControllingFieldValues))
	seen := make(map[string]bool, len(selected.ControllingFieldValues))
	highest := 0
	for _, name := range selected.ControllingFieldValues {
		index, known := positions[name]
		if !known || seen[name] {
			return Null, false
		}
		seen[name] = true
		indices = append(indices, index)
		if index > highest {
			highest = index
		}
	}
	// Native JSON emits MSB-first bits in complete 24-bit Base64 groups through
	// the highest selected controller, independent of the total controller count.
	bits := make([]byte, (highest/24+1)*3)
	for _, index := range indices {
		bits[index/8] |= 1 << uint(7-index%8)
	}
	return String(base64.StdEncoding.EncodeToString(bits)), true
}

// Build a private serialization view so cached entries keep their public
// property surface and callers at other source versions cannot inherit a view.
func (vm *VM) picklistEntryJSONView(value Value, suppressObjectNulls bool) Value {
	if !isPicklistJSONValidForField(value, picklistJSONValidForField) {
		return value
	}
	encoded, present := value.Fields[picklistJSONValidForField]
	if !present {
		return value
	}
	out := value
	out.Fields = make(map[string]Value, len(value.Fields))
	for name, item := range value.Fields {
		if name != picklistJSONValidForField {
			out.Fields[name] = item
		}
	}
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if known && (major == 61 || major == 62 || major == 67) && !suppressObjectNulls && (encoded.Kind == ValueNull || encoded.Kind == ValueString) {
		out.Fields["validFor"] = encoded
	}
	return out
}

func isPicklistJSONValidForField(value Value, field string) bool {
	return field == picklistJSONValidForField && value.Kind == ValueObject && !value.classInstance && strings.EqualFold(value.Type, "Schema.PicklistEntry")
}
