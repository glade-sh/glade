package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
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

// C137_046/048/050/051/088/090/092/093 capture anonymous query and
// countQuery SYSTEM_MODE argument rejections at 59/62/66/67/68. The access
// boundary precedes query execution and Database.query's scalar COUNT error.
func (vm *VM) validateSOQLAccessLevel(raw string, accessLevel Value) error {
	if databaseAccessLevelSecurityMode(accessLevel) != "SYSTEM_MODE" || vm.testContext != nil ||
		vm.currentClass != "" || vm.currentMethod.ClassName != "" || vm.currentTrigger ||
		!vm.anonymousSystemModeQueryShape(raw) {
		return nil
	}
	switch vm.currentMethod.APIVersion {
	case "59.0", "62.0", "66.0", "67.0", "68.0":
		return vm.validateExplicitAccessLevel(accessLevel)
	default:
		return nil
	}
}

func (vm *VM) validateSOQLCallAccess(callee string, args []Value) error {
	canonical, ok := canonicalBuiltinStaticCall(callee)
	if !ok || (canonical != "Database.query" && canonical != "Database.countQuery") ||
		!vm.shouldUseBuiltinStaticPrecedence(callee, canonical) {
		return nil
	}
	if len(args) == 2 && args[0].Kind == ValueString && isDatabaseAccessLevelValue(args[1]) {
		if canonical == "Database.countQuery" {
			query, err := vm.parseSOQLAt(args[0].Text)
			if err != nil || !query.Count {
				return nil // Non-count validation has no precedence capture.
			}
		}
		return vm.validateSOQLAccessLevel(args[0].Text, args[1])
	}
	return nil
}

// Only the captured flat Id-scoped projection/COUNT shapes opt into the early
// boundary. Invalid syntax, missing binds and other queries keep their existing
// validation order until their rejection twins have native answers.
func (vm *VM) anonymousSystemModeQueryShape(raw string) bool {
	query, err := vm.parseSOQLAt(raw)
	if err != nil || vm.Org == nil || query.SecurityMode != "" || len(query.ChildQueries) != 0 ||
		len(query.Typeofs) != 0 || (!query.Count && len(query.Aggregates) != 0) || len(query.GroupBy) != 0 ||
		query.Having != nil || len(query.Order) != 0 || query.HasLimit || query.HasOffset ||
		query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return false
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || where.Op != "=" || where.Not ||
		len(where.And) != 0 || len(where.Or) != 0 || where.Range || where.Subquery != nil {
		return false
	}
	if where.Value.Kind != storage.ValueID || !strings.HasPrefix(string(where.Value.ID), ":") {
		return false
	}
	bound, err := vm.lookup(strings.TrimPrefix(string(where.Value.ID), ":"))
	boundID := bound.Text
	if bound.Kind == ValueObject && bound.Type == "Id" {
		boundID, err = platformScalarText(bound, "Id")
	} else if bound.Kind != ValueString {
		return false
	}
	if err != nil || validateApexID(boundID) != nil {
		return false // Missing, null and non-Id binds have no precedence capture.
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok {
		return false
	}
	if !query.Count {
		if len(query.Fields) < 1 || len(query.Fields) > 3 || !strings.EqualFold(query.Fields[0], "Id") {
			return false
		}
		definition := vm.Org.Objects[objectName].Definition
		for _, field := range query.Fields[1:] {
			name, found := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
			if !found || definition.Fields[name].Type != storage.FieldString {
				return false
			}
		}
	}
	return vm.validateDynamicSOQLBindSyntax(raw, query.Count) == nil && vm.validateDynamicSOQLBinds(raw, Null) == nil
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

func (vm *VM) validateUserModePermissionSetFeature() error {
	if vm != nil && vm.Org != nil && vm.Org.HasFeature("ApexUserModeWithPermset") {
		return nil
	}
	return newExceptionError("SecurityException", "Custom AccessLevel usage requires ApexUserModeWithPermset enabled in a scratch org.")
}

func (vm *VM) currentSharingMode() string {
	if vm == nil {
		return "without sharing"
	}
	if vm.currentTrigger {
		return "without sharing"
	}
	// Native test invocations enforce sharing in the test method itself,
	// including a class declared without sharing. Ordinary helpers still use
	// their own declarations and the effective caller frame below.
	if entry := vm.testExecutionEntry; entry != nil &&
		strings.EqualFold(vm.currentClass, entry.className) && len(vm.callStack) == entry.depth {
		if class, ok := vm.lookupClass(vm.currentClass); ok && class.IsTest {
			return "with sharing"
		}
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
		if mode, ok := vm.nearestCallStackSharingMode(); ok {
			return mode
		}
		if mode := strings.TrimSpace(vm.entrySharingMode); mode != "" {
			return mode
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
