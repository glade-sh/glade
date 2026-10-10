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

// Explicit access arguments have an anonymous-only boundary. Source defaults
// and a WITH SYSTEM_MODE suffix in a dynamic query are separate contracts.
func (vm *VM) validateExplicitAccessLevel(accessLevel Value) error {
	if databaseAccessLevelSecurityMode(accessLevel) == "SYSTEM_MODE" && vm.testContext == nil &&
		vm.currentClass == "" && vm.currentMethod.ClassName == "" && !vm.currentTrigger {
		return newExceptionError("SecurityException", "Cannot use SYSTEM_MODE access level in anonymous execution of Apex.")
	}
	return nil
}

func (vm *VM) validateDynamicQueryAccessLevel(queryText string, accessLevel Value) error {
	if err := vm.validateExplicitAccessLevel(accessLevel); err != nil {
		return err
	}
	if query, err := vm.parseSOQLAt(queryText); err == nil {
		if strings.EqualFold(query.SecurityMode, "USER_MODE") || strings.EqualFold(query.SecurityMode, "SYSTEM_MODE") {
			return newExceptionError("SecurityException", "Cannot use the WITH AccessLevel clause in dynamic queries that also specify an access level.")
		}
		// R089: at the floor USER_MODE cannot combine with SECURITY_ENFORCED.
		// API 67 instead rejects the removed clause in the query route.
		if strings.EqualFold(query.SecurityMode, "SECURITY_ENFORCED") &&
			databaseAccessLevelSecurityMode(accessLevel) == "USER_MODE" &&
			!apexversion.Enabled(vm.currentMethod.APIVersion, apexversion.SecureDefaults) {
			return newExceptionError("SecurityException", "Cannot use the WITH SECURITY_ENFORCED clause in queries using USER_MODE access level.")
		}
	}
	return nil
}

func (vm *VM) currentSharingMode() string {
	if vm == nil {
		return "without sharing"
	}
	if vm.currentTrigger {
		return "without sharing"
	}
	if vm.currentMethod.SourceContextBound {
		switch strings.ToLower(strings.TrimSpace(vm.currentMethod.SharingMode)) {
		case "with sharing", "without sharing":
			return strings.ToLower(strings.TrimSpace(vm.currentMethod.SharingMode))
		case "inherited sharing":
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
	secureDefault := false
	for strings.TrimSpace(className) != "" && !seen[strings.ToLower(className)] {
		seen[strings.ToLower(className)] = true
		class, ok := vm.lookupClass(className)
		if !ok {
			break
		}
		secureDefault = secureDefault || apexversion.Enabled(class.APIVersion, apexversion.SecureDefaults)
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
	if secureDefault {
		return "with sharing", true
	}
	return "", false
}
