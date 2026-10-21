package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// SUM exposes a boxed Decimal whose scale comes from the shortest Double
// representation of the result, rather than the source field's scale.
func (vm *VM) aggregateSumNumberValue(value storage.Value, query soql.Query, name string) Value {
	result := vmValueFromStorage(value)
	if result.Kind != ValueDecimal || !apexversion.AtLeast(vm.currentMethod.APIVersion, 62) ||
		len(query.Aggregates) != 1 || len(query.GroupBy) != 0 || query.GroupMode != "" {
		return result
	}
	unnamed := 0
	for _, aggregate := range query.Aggregates {
		alias := aggregate.Alias
		if alias == "" {
			alias = fmt.Sprintf("expr%d", unnamed)
			unnamed++
		}
		if alias != name || !strings.EqualFold(aggregate.Func, "SUM") {
			continue
		}
		definition, field, ok := vm.sObjectFieldDefinition(query.Object, aggregate.Field)
		if !ok || !hasSuffixFold(definition.APIName, "__c") || storage.IsCustomSettingDefinition(definition) ||
			field.Type != storage.FieldDecimal || field.Precision != 18 || !field.ScaleSpecified || strings.TrimSpace(field.Formula) != "" {
			return result
		}
		supported := strings.EqualFold(field.DisplayType, "DOUBLE") && field.Scale >= 0 && field.Scale <= 8 ||
			strings.EqualFold(field.DisplayType, "CURRENCY") && field.Scale == 2 ||
			strings.EqualFold(field.DisplayType, "PERCENT") && field.Scale == 8
		if supported {
			if decimal, err := decimalFromText(doubleDisplayText(result.Decimal)); err == nil {
				return decimal
			}
		}
		return result
	}
	return result
}
