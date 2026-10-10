package vm

import (
	"fmt"
	"strings"
)

// Approval validates its overloads before entering the shared DML path. These
// contracts differ from Database.lock at API 62 and 67.
func (vm *VM) approvalLockInput(value Value, omitNullIDs bool) (Value, error) {
	if value.Kind == ValueNull {
		if strings.HasPrefix(strings.ToLower(value.Type), "list<") {
			return Null, newExceptionError("System.TypeException", "List is null")
		}
		return Null, newExceptionError("System.NullPointerException", "Argument 1 cannot be null")
	}
	if value.Kind == ValueList {
		if element, ok := collectionElementType(value.Type); ok && strings.EqualFold(element, "SObject") {
			return Null, newExceptionError("System.TypeException", "Invalid conversion from runtime type List<SObject> to List<Id>")
		}
		filtered := typedList(value.Type)
		for i, item := range value.List {
			if item.Kind == ValueNull && omitNullIDs {
				continue
			}
			if item.Kind == ValueObject && !isApexIDLikeValue(item) {
				_, id, exists := objectFieldValue(item, "Id")
				if !exists || id.Kind == ValueNull {
					return Null, newExceptionError("System.StringException", fmt.Sprintf("Invalid id at index %d: null", i))
				}
			}
			filtered.List = append(filtered.List, item)
		}
		return filtered, nil
	}
	if value.Kind == ValueObject && !isApexIDLikeValue(value) {
		_, id, exists := objectFieldValue(value, "Id")
		if !exists || id.Kind == ValueNull {
			return Null, newExceptionError("System.StringException", "Invalid id at index 0: null")
		}
	}
	return value, nil
}

func (vm *VM) executeAutomationLock(op string, args []Value, result *Result) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("Approval.%s expects record or Id and optional Boolean", op)
	}
	value, err := vm.approvalLockInput(args[0], true)
	if err != nil {
		return Null, err
	}
	forwarded := append([]Value(nil), args...)
	forwarded[0] = value
	resultType := "Approval.LockResult"
	if op == "unlock" {
		resultType = "Approval.UnlockResult"
	}
	return vm.executeDatabaseRecordAction(op, forwarded, result, resultType)
}

func (vm *VM) executeAutomationIsLocked(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("Approval.isLocked expects record or Id")
	}
	value, err := vm.approvalLockInput(args[0], false)
	if err != nil {
		return Null, err
	}
	return vm.executeApprovalIsLocked([]Value{value})
}

// Request fields are nullable, and the String-valued identity
// accessors preserve their text rather than coercing it through the Id type.
func callApprovalRequestMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch strings.ToLower(receiver.Type) {
	case "approval.processrequest", "approval.processsubmitrequest", "approval.processworkitemrequest":
	default:
		return Null, receiver, false, false, nil
	}
	fields := []string{"Comments", "NextApproverIds"}
	if strings.EqualFold(receiver.Type, "Approval.ProcessSubmitRequest") {
		fields = append(fields, "ObjectId", "ProcessDefinitionNameOrId", "SkipEntryCriteria", "SubmitterId")
	}
	if strings.EqualFold(receiver.Type, "Approval.ProcessWorkitemRequest") {
		fields = append(fields, "Action", "WorkitemId")
	}
	for _, field := range fields {
		if strings.EqualFold(method, "get"+field) && len(args) == 0 {
			if _, value, ok := objectFieldValue(receiver, field); ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
		if strings.EqualFold(method, "set"+field) && len(args) == 1 {
			receiver.Fields[passiveAccessorFieldName(receiver, field)] = args[0]
			return Null, receiver, true, true, nil
		}
	}
	return Null, receiver, false, false, nil
}

func validateInvocableInvocation(action Value) error {
	standard := action.Fields["standard"]
	custom := standard.Kind != ValueBool || !standard.Bool
	kind, name := action.Fields["type"], action.Fields["name"]
	if kind.Kind == ValueNull {
		displayName := "unknown"
		if custom && name.Kind == ValueString {
			displayName = name.Text
		}
		return newExceptionError("System.NullPointerException", "Specify a type for the "+displayName+" custom invocable action.")
	}
	if kind.Kind != ValueString {
		return fmt.Errorf("Invocable.Action.invoke expects a String action type")
	}
	named := strings.EqualFold(kind.Text, "apex") || strings.EqualFold(kind.Text, "flow") || strings.EqualFold(kind.Text, "quickAction")
	if !named && !strings.EqualFold(kind.Text, "chatterPost") {
		return newExceptionError("System.TypeException", fmt.Sprintf("%q isn't a valid action type.", kind.Text))
	}
	if named && name.Kind == ValueNull {
		return newExceptionError("System.NullPointerException", "Specify a name for the "+kind.Text+" custom invocable action type.")
	}
	return nil
}

type invocableMissingActionError struct{ name string }

func (e *invocableMissingActionError) Error() string {
	return "Action name not found: " + e.name
}

func invocablePrimitiveElement(typeName string) bool {
	return strings.EqualFold(typeName, "String")
}

// Only the automation DTOs use this native display syntax.
func automationDTOString(value Value, seen map[uint64]bool) (string, bool) {
	if value.Kind != ValueObject {
		return "", false
	}
	var fields []string
	name := ""
	switch strings.ToLower(value.Type) {
	case "invocable.action":
		name = "Action"
		fields = []string{"apiVersion", "invocations", "invokeFromLightningComponent", "name", "namespace", "type", "version"}
	case "invocable.action.result":
		name = "Result"
		fields = []string{"action", "errors", "invocationParameters", "outputParameters", "success"}
	case "invocable.action.error":
		name = "Error"
		fields = []string{"code", "message"}
	default:
		return "", false
	}
	if value.Ref != 0 {
		if seen[value.Ref] {
			return "(already output)", true
		}
		seen[value.Ref] = true
	}
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		_, item, _ := objectFieldValue(value, field)
		text := automationDTOValueString(item, seen)
		parts = append(parts, field+"="+text)
	}
	return name + ":[" + strings.Join(parts, ", ") + "]", true
}

func automationDTOValueString(value Value, seen map[uint64]bool) string {
	if text, ok := automationDTOString(value, seen); ok {
		return text
	}
	if value.Kind == ValueList || value.Kind == ValueMap {
		if value.Ref != 0 {
			if seen[value.Ref] {
				return "(already output)"
			}
			seen[value.Ref] = true
		}
	}
	if value.Kind == ValueList {
		parts := make([]string, 0, len(value.List))
		for _, item := range value.List {
			parts = append(parts, automationDTOValueString(item, seen))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}
	if value.Kind == ValueMap {
		parts := make([]string, 0, len(value.Map))
		for _, key := range sortedMapKeys(value.Map) {
			parts = append(parts, automationDTOValueString(mapStoredKey(value, key), seen)+"="+automationDTOValueString(value.Map[key], seen))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return valueStringWithSeen(value, seen)
}

func automationID(value Value) (Value, error) {
	if value.Kind == ValueNull {
		return Null, nil
	}
	text, ok := idValueText(value)
	if !ok {
		return Null, newExceptionError("System.StringException", "Invalid id: "+value.String())
	}
	if err := validateApexID(text); err != nil {
		return Null, newExceptionError("System.StringException", "Invalid id: "+text)
	}
	return platformScalar("Id", apexIDTo18(text)), nil
}
