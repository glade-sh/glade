package vm

import "github.com/glade-sh/glade/internal/apexversion"

// Keep collection reflection changes within the supported source API window.
func (vm *VM) collectionCastReflectionEnabled() bool {
	return apexversion.AtLeast(vm.currentMethod.APIVersion, 62)
}
