package storage

import "strings"

// CurrencyDefaultForField resolves the platform currency default only for an
// enabled multi-currency org. It does not default arbitrary currency amounts.
func CurrencyDefaultForField(org *OrgState, userID ID, field Field) (Value, bool) {
	if org == nil || !strings.EqualFold(field.APIName, "CurrencyIsoCode") {
		return Value{}, false
	}
	enabled := false
	for _, record := range org.Objects["Organization"].Records {
		value, _ := record.GetField("IsMultiCurrencyEnabled")
		enabled = enabled || value.Kind == ValueBoolean && value.Boolean
	}
	if !enabled {
		return Value{}, false
	}
	if _, user, ok := LookupRecordByID(org.Objects["User"].Records, userID); ok {
		if value, ok := user.GetField("DefaultCurrencyIsoCode"); ok && value.Kind == ValueString && strings.TrimSpace(value.String) != "" {
			return value, true
		}
	}
	for _, record := range org.Objects["CurrencyType"].Records {
		corporate, _ := record.GetField("IsCorporate")
		code, _ := record.GetField("IsoCode")
		if corporate.Kind == ValueBoolean && corporate.Boolean && code.Kind == ValueString && strings.TrimSpace(code.String) != "" {
			return code, true
		}
	}
	return Value{}, false
}
