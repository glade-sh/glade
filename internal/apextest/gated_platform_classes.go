package apextest

import (
	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The standard platform view leaves the ConnectApi and Slack stub types out
// until the process names those namespaces. A VM registered from that view
// binds a left-out stub class when a lookup first needs it; this source builds
// the class the passive templates built for it before the gate: the same
// stub-only symbol, the same passive declaration.
func init() {
	vm.SetGatedPlatformClassSource(gatedPlatformStubClass)
}

func gatedPlatformStubClass(runtimeName string) (vm.Class, bool) {
	symbol, ok := typesys.GatedPlatformStubSymbol(runtimeName)
	if !ok {
		return vm.Class{}, false
	}
	switch symbol.Kind {
	case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
	default:
		return vm.Class{}, false
	}
	name := typeSymbolRuntimeName(*symbol)
	if symbol.Kind != apexast.DeclarationEnum && !isPassiveStandardRuntimeType(name) {
		return vm.Class{}, false
	}
	return passiveRuntimeClassFromTypeSymbol(*symbol, name), true
}
