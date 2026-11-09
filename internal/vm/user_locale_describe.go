package vm

import (
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// userLocaleDescribeValues projects the captured source-version view without
// modifying the catalog used to validate DML. The caller caches the result.
func (vm *VM) userLocaleDescribeValues(objectName string, field storage.Field) []storage.PicklistValue {
	original := field.PicklistValues
	if objectName != "User" || field.APIName != "LocaleSidKey" || !field.RestrictedPicklistFromCatalog || field.PicklistValuesConfigured {
		return original
	}
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || (major != 62 && major != 68) {
		return original
	}
	catalog, ok := storage.StandardObjectDefinition("User")
	if !ok {
		return original
	}
	canonical := catalog.Fields["LocaleSidKey"].PicklistValues
	if len(original) != len(canonical) || len(canonical) != 279 {
		return original
	}
	for i := range canonical {
		if original[i] != canonical[i] {
			return original
		}
	}
	removed := map[string]bool{
		"de_LI": true, "en_AD": true, "en_AL": true, "en_AT": true, "en_BA": true, "en_BG": true, "en_DM": true, "en_EE": true, "en_FI": true, "en_GD": true, "en_GR": true, "en_HR": true, "en_IS": true, "en_JP": true, "en_KN": true, "en_KR": true, "en_LC": true, "en_LI": true, "en_LT": true, "en_LU": true, "en_LV": true, "en_MC": true, "en_ME": true, "en_MK": true, "en_PT": true, "en_RS": true, "en_SI": true, "en_TH": true, "en_TR": true, "en_TW": true, "en_VC": true, "es_AD": true, "sv_FI": true,
	}
	insert := map[string]storage.PicklistValue{
		"en_BG": {Value: "en_BR", Label: "English (Brazil)", Active: true},
		"en_MC": {Value: "en_MX", Label: "English (Mexico)", Active: true},
		"en_RS": {Value: "en_SA", Label: "English (Saudi Arabia)", Active: true},
	}
	out := make([]storage.PicklistValue, 0, len(original)+3)
	for _, value := range original {
		if major == 62 && removed[value.Value] {
			continue
		}
		if major == 68 {
			if extra, ok := insert[value.Value]; ok {
				out = append(out, extra)
			}
		}
		out = append(out, value)
	}
	return out
}
