package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) applyCurrentUserRelationshipFields(parent *Value, parentObject string, lookupID storage.ID) {
	if parent == nil || !strings.EqualFold(parentObject, "User") {
		return
	}
	if storage.ID(vm.currentUserInfoField("Id", "")) != lookupID {
		return
	}
	parent.Fields["Name"] = String(vm.currentUserInfoField("Name", "Test User"))
}
