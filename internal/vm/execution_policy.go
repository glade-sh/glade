package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/ir"
)

type sourceExecutionPolicy struct {
	APIVersion  string
	DataMode    string
	SharingMode string
	Trigger     bool
}

func (vm *VM) currentSourceExecutionPolicy() sourceExecutionPolicy {
	version := ""
	if vm != nil {
		version = strings.TrimSpace(vm.currentMethod.APIVersion)
	}
	dataMode := "SYSTEM_MODE"
	if apexversion.Enabled(version, apexversion.SecureDefaults) {
		dataMode = "USER_MODE"
	}
	trigger := vm != nil && vm.currentTrigger
	sharingMode := "without sharing"
	if vm != nil {
		sharingMode = vm.currentSharingMode()
	}
	return sourceExecutionPolicy{
		APIVersion:  version,
		DataMode:    dataMode,
		SharingMode: sharingMode,
		Trigger:     trigger,
	}
}

func (vm *VM) defaultAccessLevelMode() string {
	return vm.currentSourceExecutionPolicy().DataMode
}

func (vm *VM) defaultAccessLevel() Value {
	return accessLevelValue(vm.defaultAccessLevelMode())
}

func (vm *VM) resolveDMLMode(mode ir.DMLMode) string {
	switch mode {
	case ir.DMLModeUser:
		return "USER_MODE"
	case ir.DMLModeSystem:
		return "SYSTEM_MODE"
	default:
		return vm.defaultAccessLevelMode()
	}
}

func (vm *VM) currentSharingMode() string {
	if vm.currentTrigger {
		return "without sharing"
	}
	if hasSuffixFold(vm.currentClass, ".withsharing") {
		return "with sharing"
	}
	if class, ok := vm.lookupClass(vm.currentClass); ok {
		switch {
		case methodHasModifier(class.Modifiers, "with sharing"):
			return "with sharing"
		case methodHasModifier(class.Modifiers, "without sharing"):
			return "without sharing"
		case methodHasModifier(class.Modifiers, "inherited sharing"):
			if mode, ok := vm.nearestCallStackSharingMode(); ok {
				return mode
			}
		}
	}
	if !apexversion.Enabled(vm.currentMethod.APIVersion, apexversion.SecureDefaults) {
		if mode, ok := vm.nearestCallStackSharingMode(); ok {
			return mode
		}
	}
	if apexversion.Enabled(vm.currentMethod.APIVersion, apexversion.SecureDefaults) {
		return "with sharing"
	}
	return "without sharing"
}
