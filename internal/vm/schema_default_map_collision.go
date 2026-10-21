package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The captured lookup probes use an unmutated, Id-less custom-record key and
// a distinct record whose zero Number or empty Text has become explicit.
// Do not change insertion, hash encoding, Id-backed keys or class instances.
func (vm *VM) schemaDefaultMapPresenceCollision(stored, key Value) bool {
	if stored.Kind != ValueObject || key.Kind != ValueObject || stored.classInstance || key.classInstance || !hasSuffixFold(stored.Type, "__c") || !strings.EqualFold(stored.Type, key.Type) {
		return false
	}
	if _, exists := sObjectIDFromValue(stored); exists {
		return false
	}
	if _, exists := sObjectIDFromValue(key); exists {
		return false
	}
	for field, value := range stored.Fields {
		_, other, exists := objectFieldValue(key, field)
		if !exists || !value.Equal(other) || sObjectEqualityFieldIncluded(stored, field, value) == sObjectEqualityFieldIncluded(key, field, other) {
			continue
		}
		object, column, known := vm.sObjectFieldDefinition(stored.Type, field)
		if !known || storage.IsCustomSettingDefinition(object) {
			continue
		}
		zero := (value.Kind == ValueInt && value.Int == 0 || value.Kind == ValueDecimal && value.Decimal == 0) && (column.Type == storage.FieldInteger || column.Type == storage.FieldDecimal) && strings.TrimSpace(column.DefaultValue) == "0"
		empty := value.Kind == ValueString && value.Text == "" && column.Type == storage.FieldString && strings.TrimSpace(column.DefaultValue) == `""`
		if (zero || empty) && !stored.Equal(key) {
			return true
		}
	}
	return false
}
