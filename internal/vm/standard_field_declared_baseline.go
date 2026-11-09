package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// standardFieldAbsentFromDeclaredBaseline reports a standard field that a plain
// Developer edition org does not have. Account.IsCustomerPortal appears only
// with Communities or in another edition (standard_describe_profiles
// feature-set-differences: P4, P11 and PU add it, P0 does not). The standard
// schema still lists the field, so a constructor with loadDefaults must not
// give it a default value that user-mode DML would then check
// (standard_user_default_load_dml). Only a project that declares a Developer
// scratch org without those features qualifies; an undeclared shape keeps the
// field.
func (vm *VM) standardFieldAbsentFromDeclaredBaseline(objectName string, field storage.Field) bool {
	if vm == nil || vm.Org == nil || !strings.EqualFold(objectName, "Account") || !strings.EqualFold(field.APIName, "IsCustomerPortal") {
		return false
	}
	return vm.Org.HasFeature(project.StandardFieldEditionDeveloper) &&
		!vm.Org.HasFeature(project.UnspecifiedStandardFieldShape) &&
		!vm.Org.HasFeature("Communities") && !vm.Org.HasFeature("NetworksEnabled")
}
