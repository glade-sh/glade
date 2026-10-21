package schema

import "strings"

// FieldKeyTextNameLookup identifies the source-backed Text record-name shape
// captured by FieldKey 022/058/105. Settings and other metadata kinds retain
// their separate name-field contracts.
func FieldKeyTextNameLookup(object Object) bool {
	return strings.HasSuffix(strings.ToLower(object.Name), "__c") &&
		object.CustomSettingsType == "" && strings.EqualFold(object.NameField.Type, "Text")
}

// FieldKey 027/029/034/035 capture the omitted metadata default for unique
// Number and Email fields. Explicit flags and other field types stay unchanged.
func fieldKeyCaseSensitive(raw customFieldXML) bool {
	return raw.CaseSensitive != nil && *raw.CaseSensitive
}

func fieldKeyDescribeCaseSensitive(raw customFieldXML) *bool {
	if raw.CaseSensitive == nil && raw.Unique && raw.Formula == "" &&
		(strings.EqualFold(raw.Type, "Number") || strings.EqualFold(raw.Type, "Email")) {
		value := true
		return &value
	}
	return nil
}
