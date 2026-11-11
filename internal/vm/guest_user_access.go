package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/soql"
)

// guestUserReadsOnlySelf reports a query on User by a Guest-type profile user.
// Natively that user sees only its own User row, whatever the class sharing
// mode (guest_user_contract soqlRows, API 62.0 and 67.0).
func (vm *VM) guestUserReadsOnlySelf(objectName string) bool {
	if vm == nil || vm.Org == nil || !strings.EqualFold(strings.TrimSpace(objectName), "User") {
		return false
	}
	if vm.testContext == nil || vm.testContext.RunAsDepth == 0 {
		return false
	}
	user := vm.fieldPermissionPrincipal()
	return vm.currentProfileHasUserType(stringField(user, "ProfileId"), "Guest")
}

// guestUserColumnDenial applies the API 67 column rule of a Guest-type profile
// user reading User: the columns its profile cannot read, and NumberOfFailedLogins,
// are missing columns (guest_user_contract soqlRows, API 67.0). API 62.0 does
// not enforce columns on a plain query.
func (vm *VM) guestUserColumnDenial(query soql.Query) error {
	if !vm.runAsSecureDefaults() || !vm.guestUserReadsOnlySelf(query.Object) {
		return nil
	}
	for _, field := range query.Fields {
		if guestHiddenUserField(field) || strings.EqualFold(field, "NumberOfFailedLogins") {
			return newExceptionError("QueryException", soql.MissingColumnMessage(field, "User"))
		}
	}
	return nil
}
