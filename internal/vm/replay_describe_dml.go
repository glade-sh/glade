package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

const platformEventTestSaveResultAPIVersion = 67

func replayDescribeErasedScalar(value Value) bool {
	return value.Kind == ValueObject && !value.classInstance && strings.EqualFold(value.Static, "SObject") && sObjectIDFromFields(value.Fields) == ""
}

// Blank erased standard events enqueue in tests before the SaveResult gate.
// At and above the gate, publication returns a failed SaveResult.
func (vm *VM) replayDescribeStandardPublishMode(value Value) int {
	if vm.testContext == nil || !replayDescribeErasedScalar(value) || !vm.replayDescribeStandardEvent(value.Type) {
		return 0
	}
	if _, published := value.Fields[sobjectPublishedEventIDField]; published {
		return 0
	}
	for field := range value.Fields {
		if !isInternalSObjectField(field) {
			return 0
		}
	}
	major, ok := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !ok {
		return 0
	}
	if major < platformEventTestSaveResultAPIVersion {
		return 1
	}
	return -1
}

// Blank standard error events have a named-method publication contract that
// also applies when an anonymous caller invokes that method. Standalone
// anonymous publication and Database DML keep their separate contracts.
func (vm *VM) namedStandardEventPublishMode(value Value) int {
	if vm == nil || vm.Org == nil || vm.currentMethod.ClassName == "" || vm.currentTrigger ||
		vm.currentMethod.Namespace != "" || vm.Org.Namespace != "" || value.Kind != ValueObject ||
		value.classInstance || value.projectClass || sObjectIDFromFields(value.Fields) != "" ||
		!vm.replayDescribeStandardEvent(value.Type) {
		return 0
	}
	if _, published := value.Fields[sobjectPublishedEventIDField]; published {
		return 0
	}
	for field := range value.Fields {
		if !isInternalSObjectField(field) && !isDefaultedSObjectField(value, field) {
			return 0
		}
	}
	major, ok := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !ok {
		return 0
	}
	switch {
	case major >= 62 && major < platformEventTestSaveResultAPIVersion:
		if vm.testContext != nil {
			return 1
		}
	case major == platformEventTestSaveResultAPIVersion:
		return -1
	}
	return 0
}

// The partial scalar Database overload has distinct event behavior.
// Typed, list, ID-bearing, access-level and all-or-none
// overloads continue through their existing paths.
func (vm *VM) replayDescribeDatabaseDML(op string, args []Value, result *Result) (Value, bool, error) {
	if len(args) != 2 || args[1].Kind != ValueBool || args[1].Bool || !replayDescribeErasedScalar(args[0]) {
		return Null, false, nil
	}
	record := args[0]
	customEvent := vm.hasPlatformEventMetadata(record.Type)
	standardEvent := vm.replayDescribeStandardEvent(record.Type)
	if !customEvent && !standardEvent {
		return Null, false, nil
	}
	if standardEvent {
		if op != "insert" {
			return Null, false, nil
		}
		if vm.testContext != nil && vm.replayDescribeStandardPublishMode(record) == 0 && vm.namedStandardEventPublishMode(record) == 0 {
			return Null, false, nil
		}
	}
	switch op {
	case "insert":
		// Database.insert does not expose the publication UUID on the input.
		// EventBus.publish does; publishing a private copy shares its result
		// and delivery machinery without mutating the caller's field map.
		// Standard events retain the captured named-method source policy.
		value, err := vm.eventBusPublishWithNamedSource([]Value{cloneValue(record)}, result, standardEvent)
		return value, true, err
	case "update":
		message := "DML operation UPDATE not allowed on " + record.Type
		if vm.testContext != nil {
			major, ok := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
			if !ok {
				return Null, false, nil
			}
			if major >= platformEventTestSaveResultAPIVersion {
				return platformEventSaveResult("", false, "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", message, nil), true, nil
			}
		}
		return Null, true, newExceptionError("System.TypeException", message)
	case "delete":
		return Null, true, newExceptionError("System.DmlException", "Argument must be of internal sObject type. use deleteAsync() or deleteImmediate() instead")
	}
	return Null, false, nil
}

// Standard-event statement inserts retain their scalar/concrete-list boundary.
// An erased list of blank events is accepted only by the pre-gate named test
// contract; publication and partial Database.insert return SaveResults instead.
func (vm *VM) standardEventStatementInsertBoundary(value Value) error {
	eventType := runtimeObjectType(value)
	if value.Kind == ValueList {
		eventType, _ = collectionElementType(eventType)
	}
	if class, ok := vm.lookupClass(eventType); ok && !class.PlatformType {
		return nil
	}
	standardEvent := vm.replayDescribeStandardEvent(eventType)
	if standardEvent && !value.projectClass {
		return newExceptionError("System.DmlException", "Argument must be of internal sObject type. use insertAsync() or insertImmediate() instead")
	}
	if value.Kind == ValueList {
		// Schema classification is constant for a type during this statement.
		// Keep record-specific publication policy checks inside the loop.
		standardEvents := map[string]bool{eventType: standardEvent}
		for _, record := range value.List {
			if record.projectClass {
				continue
			}
			isEvent, checked := standardEvents[record.Type]
			if !checked {
				isEvent = vm.replayDescribeStandardEvent(record.Type)
				standardEvents[record.Type] = isEvent
			}
			if isEvent && vm.namedStandardEventPublishMode(record) != 1 {
				return newExceptionError("System.TypeException", "DML operation INSERT not allowed on "+record.Type)
			}
		}
	}
	return nil
}
