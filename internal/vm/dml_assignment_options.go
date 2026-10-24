package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func databaseAssignmentRuleOptions(options Value) (*storage.AssignmentRuleSelection, error) {
	if !databaseDMLOptionConfigured(options, "AssignmentRuleHeader", true) {
		return nil, nil
	}
	unsupported := func() (*storage.AssignmentRuleSelection, error) {
		return nil, unsupportedCallError("Database.DMLOptions.AssignmentRuleHeader local DML option behavior")
	}
	selection := &storage.AssignmentRuleSelection{}
	for name, header := range options.Fields {
		if !strings.EqualFold(name, "AssignmentRuleHeader") || header.Kind == ValueNull {
			continue
		}
		if header.Kind != ValueObject {
			return unsupported()
		}
		for key, value := range header.Fields {
			if value.Kind == ValueNull {
				continue
			}
			switch strings.ToLower(key) {
			case "usedefaultrule":
				if value.Kind != ValueBool {
					return unsupported()
				}
				selection.UseDefault = value.Bool
			case "assignmentruleid":
				if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
					if text, ok := platformScalarObjectText(value); ok {
						value = String(text)
					}
				}
				if value.Kind != ValueString {
					return unsupported()
				}
				selection.ID = storage.ID(value.Text)
			default:
				return unsupported()
			}
		}
	}
	if selection.UseDefault && selection.ID != "" {
		return nil, &RuntimeError{Type: "System.UnexpectedException", Message: "Cannot specify both useDefaultRule=true and a specific assignmentRuleId in the AssignmentRule Header"}
	}
	return selection, nil
}

func (vm *VM) validateAssignmentRuleOptionRoute(op string, records, options Value) error {
	if !databaseDMLOptionConfigured(options, "AssignmentRuleHeader", true) {
		return nil
	}
	if op != "insert" && op != "update" {
		return unsupportedCallError("Database.DMLOptions.AssignmentRuleHeader local DML option behavior")
	}
	items := []Value{records}
	if records.Kind == ValueList {
		items = records.List
	}
	for _, record := range items {
		kind := vm.canonicalSObjectValueType(record)
		if record.Kind != ValueObject || (!strings.EqualFold(kind, "Lead") && !strings.EqualFold(kind, "Case")) {
			return unsupportedCallError("Database.DMLOptions.AssignmentRuleHeader local DML option behavior")
		}
	}
	return nil
}
