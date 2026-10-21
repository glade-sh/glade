package sema

import "strings"

// Keep the pending String payload candidate on the platform property. This
// does not change the assignability or declared type of LocaleOptions itself.
func semaDMLStringLocaleProperty(targetType, target, valueType string, scope irSemaScope, model *semaTypeMemberView, currentType string) bool {
	if !strings.EqualFold(valueType, "String") || !strings.EqualFold(targetType, "Database.LocaleOptions") {
		return false
	}
	receiver, field, qualified := strings.Cut(target, ".")
	if !qualified || !strings.EqualFold(field, "localeOptions") {
		return false
	}
	receiverType := semaIRReceiverType(receiver, scope, model, currentType)
	if !strings.EqualFold(receiverType, "Database.DMLOptions") {
		return false
	}
	owner, known := model.lookup(normalizeName(receiverType))
	return known && owner.platform
}
