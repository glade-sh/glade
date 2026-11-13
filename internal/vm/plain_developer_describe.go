package vm

import (
	"slices"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// These describe projections were observed in named source at two source APIs
// in an explicitly declared plain Developer org.
func (vm *VM) plainDeveloperDescribe(object string) bool {
	switch strings.ToLower(object) {
	case "account", "case", "contact", "lead":
	default:
		return false
	}
	if vm == nil || vm.Org == nil || vm.currentTrigger || !slices.Contains(vm.Org.Features, project.PlainDeveloperDescribeShape) {
		return false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	if version != "62.0" && version != "67.0" {
		return false
	}
	return vm.currentUserProfileName(vm.fieldPermissionPrincipal()) == "System Administrator" &&
		strings.EqualFold(vm.currentUserInfoField("LocaleSidKey", "en_US"), "en_US") &&
		strings.EqualFold(vm.currentUserInfoField("LanguageLocaleKey", "en_US"), "en_US")
}

func (vm *VM) plainDeveloperNamedDescribe(object string) bool {
	if !vm.plainDeveloperDescribe(object) || vm.currentClass == "" {
		return false
	}
	class, ok := vm.lookupClass(vm.currentClass)
	return ok && !class.PlatformType && !methodHasModifier(class.Modifiers, AnonymousClassModifier)
}

// Facets not covered by the native fixture retain synthetic-field defaults;
// they are not qualified as native behavior.
func plainDeveloperTierField() storage.Field {
	return storage.Field{APIName: "Tier", Label: "Einstein Account Tier", Type: storage.FieldString, DisplayType: "STRING", Length: 2, Nillable: storage.BoolFlag(true), Accessible: storage.BoolFlag(false), Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false), Permissionable: storage.BoolFlag(false), Filterable: storage.BoolFlag(true), Sortable: storage.BoolFlag(true), Groupable: storage.BoolFlag(true)}
}

func plainDeveloperPicklist(object string, field storage.Field) ([]storage.PicklistValue, bool) {
	if field.PicklistValuesConfigured {
		return nil, false
	}
	var entries []string
	defaultIndex := -1
	switch strings.ToLower(object + "." + field.APIName) {
	case "account.type":
		entries = []string{"Prospect", "Customer - Direct", "Customer - Channel", "Channel Partner / Reseller", "Installation Partner", "Technology Partner", "Other"}
	case "case.origin":
		entries = []string{"Phone", "Email", "Web"}
	case "case.status":
		entries = []string{"New", "Working", "Escalated", "Closed"}
		defaultIndex = 0
	case "contact.leadsource":
		entries = []string{"Web", "Phone Inquiry", "Partner Referral", "Purchased List", "Other"}
	case "lead.status":
		entries = []string{"Open - Not Contacted", "Working - Contacted", "Closed - Converted", "Closed - Not Converted"}
		defaultIndex = 0
	default:
		return nil, false
	}
	values := make([]storage.PicklistValue, 0, len(entries))
	for i, value := range entries {
		values = append(values, storage.PicklistValue{Value: value, Label: value, Active: true, Default: i == defaultIndex})
	}
	return values, true
}
