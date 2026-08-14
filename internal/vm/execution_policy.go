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

func (vm *VM) recordSharingApplies(accessLevel Value) bool {
	if databaseAccessLevelSecurityMode(accessLevel) == "USER_MODE" {
		return true
	}
	return vm != nil && !vm.currentTrigger && strings.EqualFold(vm.currentSharingMode(), "with sharing")
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
	if vm == nil {
		return "without sharing"
	}
	if vm.currentTrigger {
		return "without sharing"
	}
	if vm.currentClass == "" && len(vm.callStack) == 0 && vm.entrySharingMode == "" {
		if vm.currentMethod.APIVersion == "" || !apexversion.Enabled(vm.currentMethod.APIVersion, apexversion.SecureDefaults) {
			return "without sharing"
		}
		return "with sharing"
	}
	if mode, ok := vm.classSharingMode(vm.currentClass); ok {
		if mode != "inherited sharing" {
			return mode
		}
		if mode, ok := vm.nearestCallStackSharingMode(); ok {
			return mode
		}
		if mode := strings.TrimSpace(vm.entrySharingMode); mode != "" {
			return mode
		}
		if apexversion.Enabled(vm.currentMethod.APIVersion, apexversion.SecureDefaults) {
			return "with sharing"
		}
		return "without sharing"
	}
	if !apexversion.Enabled(vm.currentMethod.APIVersion, apexversion.SecureDefaults) {
		if mode, ok := vm.nearestCallStackSharingMode(); ok {
			return mode
		}
		if mode := strings.TrimSpace(vm.entrySharingMode); mode != "" {
			return mode
		}
	}
	if apexversion.Enabled(vm.currentMethod.APIVersion, apexversion.SecureDefaults) {
		return "with sharing"
	}
	return "without sharing"
}

func (vm *VM) classSharingMode(className string) (string, bool) {
	if hasSuffixFold(className, ".withsharing") {
		return "with sharing", true
	}
	seen := map[string]bool{}
	for strings.TrimSpace(className) != "" && !seen[strings.ToLower(className)] {
		seen[strings.ToLower(className)] = true
		class, ok := vm.lookupClass(className)
		if !ok {
			return "", false
		}
		switch {
		case methodHasModifier(class.Modifiers, "with sharing"):
			return "with sharing", true
		case methodHasModifier(class.Modifiers, "without sharing"):
			return "without sharing", true
		case methodHasModifier(class.Modifiers, "inherited sharing"):
			return "inherited sharing", true
		}
		className = vm.resolvedSuperClassName(class)
	}
	return "", false
}
