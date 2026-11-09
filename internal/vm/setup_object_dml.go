package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// setupObjectDMLMessage returns the native System.TypeException text for DML
// on a setup object that Apex cannot write (dml_not_allowed_setup_objects).
func (vm *VM) setupObjectDMLMessage(objectName, op string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(objectName))
	switch name {
	case "userlicense":
		switch op {
		case "insert", "upsert":
			// Anonymous code reads "INSERT" at every API; a test reads it from API 67.
			if apexversion.AtLeast(vm.currentMethod.APIVersion, 67) || vm.testContext == nil {
				return "DML operation INSERT not allowed on UserLicense", true
			}
			return "DML not allowed on UserLicense", true
		case "update":
			return "DML operation UPDATE not allowed on UserLicense", true
		default:
			return "DML operation DELETE not allowed on UserLicense", true
		}
	case "recordtype":
		if op == "delete" {
			return "DML operation DELETE not allowed on RecordType", true
		}
		return "DML not allowed on RecordType", true
	case "profile", "apexclass", "apextrigger":
		return "DML not allowed on " + canonicalSetupObjectName(name), true
	case "apexlog":
		switch op {
		case "insert", "upsert":
			return "DML operation INSERT not allowed on ApexLog", true
		case "update":
			return "DML operation UPDATE not allowed on ApexLog", true
		default:
			return "DML not allowed on ApexLog", true
		}
	case "organization":
		switch op {
		case "insert", "upsert":
			return "DML operation INSERT not allowed on Organization", true
		case "delete":
			return "DML operation DELETE not allowed on Organization", true
		default:
			return "DML not allowed on Organization", true
		}
	}
	return "", false
}

func canonicalSetupObjectName(lower string) string {
	switch lower {
	case "profile":
		return "Profile"
	case "apexclass":
		return "ApexClass"
	case "apextrigger":
		return "ApexTrigger"
	default:
		return "RecordType"
	}
}

// setupObjectDMLRefusal reports the native refusal of DML on Profile, ApexClass,
// ApexTrigger, RecordType, Organization, ApexLog and UserLicense. All-or-none
// calls raise a TypeException. A partial-success Database call with a list of
// records returns one failed row per record at every API (list_partial_setup_dml);
// a partial-success call with a single record, or a list of Ids for delete,
// raises through API 66 and returns a failed row from API 67.
func (vm *VM) setupObjectDMLRefusal(op string, records Value, allOrNone, idListInput bool) (message string, partialRow bool, err error) {
	if vm == nil {
		return "", false, nil
	}
	for _, record := range dmlAccessRecords(records) {
		if record.Kind != ValueObject {
			continue
		}
		if message, ok := vm.setupObjectDMLMessage(vm.canonicalSObjectValueType(record), op); ok {
			listRows := records.Kind == ValueList && !idListInput
			if !allOrNone && (listRows || apexversion.AtLeast(vm.currentMethod.APIVersion, 67)) {
				return message, true, nil
			}
			return message, false, newExceptionError("System.TypeException", message)
		}
	}
	return "", false, nil
}
