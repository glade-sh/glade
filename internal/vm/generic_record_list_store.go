package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

func (vm *VM) listAddAllMutationType(receiver Value) string {
	mutationType := vm.collectionMutationType(receiver)
	// Named and anonymous source61 observations retain generic SObject backing
	// through a concrete record-list view during addAll. Other source versions
	// and mutations retain their existing boundaries.
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || major != 61 || collectionBase(receiver.Runtime) != "List" {
		return mutationType
	}
	runtimeElement, runtimeOK := collectionElementType(receiver.Runtime)
	declaredElement, declaredOK := collectionElementType(receiver.Type)
	if runtimeOK && declaredOK && strings.EqualFold(runtimeElement, "SObject") && vm.isSObjectLikeType(declaredElement) {
		return receiver.Runtime
	}
	return mutationType
}
