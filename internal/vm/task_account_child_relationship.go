package vm

import (
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// The Account child describe capture has a null relationship for Task.AccountId
// and a distinct named Tasks relationship for Task.WhatId. Preserve that null;
// other unnamed standard relationships retain their existing describe path.
func (vm *VM) capturedTaskAccountChildRelationship(childObject string, relationship storage.Relationship) bool {
	if vm == nil || vm.Org == nil || vm.Org.Namespace != "" || childObject != "Task" ||
		relationship.Field != "AccountId" || relationship.ChildRelationship != "" ||
		len(relationship.ParentObjects) != 1 || relationship.ParentObjects[0] != "Account" {
		return false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	return apexversion.AtLeast(version, 62) && !apexversion.AtLeast(version, 69)
}
