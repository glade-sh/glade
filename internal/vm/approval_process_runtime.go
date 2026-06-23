package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) executeApprovalProcess(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("Approval.process expects request and optional allOrNone")
	}
	allOrNone := true
	if len(args) == 2 {
		if args[1].Kind != ValueBool {
			return Null, fmt.Errorf("Approval.process allOrNone expects Boolean")
		}
		allOrNone = args[1].Bool
	}
	request := args[0]
	if request.Kind == ValueList {
		return vm.executeApprovalProcessList(request, allOrNone)
	}
	return vm.executeApprovalProcessRequest(request, allOrNone)
}

func (vm *VM) executeApprovalProcessList(requests Value, allOrNone bool) (Value, error) {
	results := typedList("List<Approval.ProcessResult>")
	for _, request := range requests.List {
		result, err := vm.executeApprovalProcessRequest(request, allOrNone)
		if err != nil {
			return Null, err
		}
		results.List = append(results.List, result)
	}
	return results, nil
}

func (vm *VM) executeApprovalProcessRequest(request Value, allOrNone bool) (Value, error) {
	if request.Kind != ValueObject {
		return Null, unsupportedCallError("Approval.process request type " + runtimeValueTypeName(request))
	}
	switch {
	case strings.EqualFold(request.Type, "Approval.ProcessSubmitRequest") || strings.EqualFold(request.Type, "ProcessSubmitRequest"):
		return vm.approvalProcessSubmitRequest(request, allOrNone)
	case strings.EqualFold(request.Type, "Approval.ProcessWorkitemRequest") || strings.EqualFold(request.Type, "ProcessWorkitemRequest"):
		return vm.approvalProcessWorkitemRequest(request, allOrNone)
	default:
		return Null, unsupportedCallError("Approval.process request type " + request.Type)
	}
}

func (vm *VM) approvalProcessSubmitRequest(request Value, allOrNone bool) (Value, error) {
	objectID := approvalRequestID(request, "ObjectId")
	if objectID.Kind == ValueNull {
		return approvalProcessMissingIDResult(allOrNone, "ObjectId")
	}
	objectIDText, ok := approvalValueIDText(objectID)
	if !ok || strings.TrimSpace(objectIDText) == "" {
		return approvalProcessFailureResult(allOrNone, objectID, "INVALID_FIELD", "ObjectId is invalid", []string{"ObjectId"})
	}
	if skip, ok := approvalRequestBool(request, "SkipEntryCriteria"); ok && !skip {
		return Null, unsupportedCallError("Approval.process hosted approval engine routing")
	}
	objectName, ok := vm.approvalObjectNameForID(storage.ID(objectIDText))
	if !ok {
		return approvalProcessFailureResult(allOrNone, objectID, "INVALID_CROSS_REFERENCE_KEY", "ObjectId record is missing", []string{"ObjectId"})
	}
	definition, ok := vm.approvalProcessDefinitionForRequest(request, objectName)
	if !ok {
		if !vm.approvalHasProcessDefinitionObject() {
			result := approvalProcessResult(true, objectID, nil)
			result.Fields["instanceId"] = platformScalar("Id", "04g000000000001AAA")
			result.Fields["newWorkitemIds"] = List(platformScalar("Id", "04i000000000001AAA"))
			return result, nil
		}
		return approvalProcessFailureResult(allOrNone, objectID, "INVALID_CROSS_REFERENCE_KEY", "ProcessDefinition is required", []string{"ProcessDefinitionId"})
	}
	actorID, err := vm.approvalSubmitActor(request, definition)
	if err != nil {
		return Null, err
	}
	instanceID, err := vm.approvalCreateProcessInstance(storage.ID(objectIDText), definition)
	if err != nil {
		return Null, err
	}
	workitemID, err := vm.approvalCreateWorkitem(instanceID, actorID)
	if err != nil {
		return Null, err
	}
	result := approvalProcessResult(true, objectID, nil)
	result.Fields["instanceId"] = platformScalar("Id", string(instanceID))
	result.Fields["instanceStatus"] = String("Pending")
	result.Fields["actorIds"] = List(platformScalar("Id", string(actorID)))
	result.Fields["newWorkitemIds"] = List(platformScalar("Id", string(workitemID)))
	return result, nil
}

func (vm *VM) approvalProcessWorkitemRequest(request Value, allOrNone bool) (Value, error) {
	workitemID := approvalRequestID(request, "WorkitemId")
	if workitemID.Kind == ValueNull {
		return approvalProcessMissingIDResult(allOrNone, "WorkitemId")
	}
	workitemIDText, ok := approvalValueIDText(workitemID)
	if !ok || strings.TrimSpace(workitemIDText) == "" {
		return approvalProcessFailureResult(allOrNone, workitemID, "INVALID_FIELD", "WorkitemId is invalid", []string{"WorkitemId"})
	}
	action := strings.TrimSpace(approvalRequestString(request, "Action"))
	if action == "" {
		return approvalProcessFailureResult(allOrNone, workitemID, "REQUIRED_FIELD_MISSING", "Action is required", []string{"Action"})
	}
	comments := approvalRequestString(request, "Comments")
	return vm.approvalApplyWorkitemAction(storage.ID(workitemIDText), action, comments, allOrNone)
}

func approvalRequestID(request Value, field string) Value {
	_, value, ok := objectFieldValue(request, field)
	if !ok || value.Kind == ValueNull {
		return Null
	}
	return value
}

func approvalRequestString(request Value, field string) string {
	_, value, ok := objectFieldValue(request, field)
	if !ok || value.Kind == ValueNull {
		return ""
	}
	if value.Kind == ValueString {
		return value.Text
	}
	if text, ok := idValueText(value); ok {
		return text
	}
	return value.String()
}

func approvalRequestBool(request Value, field string) (bool, bool) {
	_, value, ok := objectFieldValue(request, field)
	if !ok || value.Kind == ValueNull || value.Kind != ValueBool {
		return false, false
	}
	return value.Bool, true
}

func approvalProcessMissingIDResult(allOrNone bool, field string) (Value, error) {
	errValue := databaseErrorValue(dml.Error{StatusCode: "REQUIRED_FIELD_MISSING", Message: field + " is required", Fields: []string{field}})
	if allOrNone {
		return Null, newExceptionError("DmlException", field+" is required")
	}
	return approvalProcessResult(false, Null, []Value{errValue}), nil
}

func approvalProcessFailureResult(allOrNone bool, entityID Value, statusCode, message string, fields []string) (Value, error) {
	errValue := databaseErrorValue(dml.Error{StatusCode: statusCode, Message: message, Fields: fields})
	if allOrNone {
		return Null, newExceptionError("DmlException", message)
	}
	return approvalProcessResult(false, entityID, []Value{errValue}), nil
}

func approvalProcessResult(success bool, entityID Value, errors []Value) Value {
	result := Object("Approval.ProcessResult")
	result.Fields["success"] = Bool(success)
	result.Fields["entityId"] = entityID
	result.Fields["instanceId"] = Null
	result.Fields["instanceStatus"] = Null
	result.Fields["actorIds"] = List()
	result.Fields["newWorkitemIds"] = List()
	result.Fields["errors"] = List(errors...)
	return result
}

func (vm *VM) approvalProcessDefinitionForRequest(request Value, objectName string) (storage.Record, bool) {
	nameOrID := strings.TrimSpace(approvalRequestString(request, "ProcessDefinitionNameOrId"))
	if nameOrID != "" {
		return vm.approvalNamedProcessDefinition(nameOrID)
	}
	return vm.approvalLocalProcessDefinition(objectName)
}

func (vm *VM) approvalNamedProcessDefinition(nameOrID string) (storage.Record, bool) {
	if vm == nil || vm.Org == nil {
		return storage.Record{}, false
	}
	object, ok := vm.Org.Objects["ProcessDefinition"]
	if !ok {
		return storage.Record{}, false
	}
	if _, record, ok := storage.LookupRecordByID(object.Records, storage.ID(nameOrID)); ok && approvalProcessDefinitionActive(record) {
		return record, true
	}
	for _, record := range sortedStorageRecords(object.Records) {
		if !approvalProcessDefinitionActive(record) {
			continue
		}
		if strings.EqualFold(storageStringField(record, "DeveloperName"), nameOrID) || strings.EqualFold(storageStringField(record, "Name"), nameOrID) {
			return record, true
		}
	}
	return storage.Record{}, false
}

func (vm *VM) approvalHasProcessDefinitionObject() bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	_, ok := vm.Org.Objects["ProcessDefinition"]
	return ok
}

func (vm *VM) approvalLocalProcessDefinition(objectName string) (storage.Record, bool) {
	if vm == nil || vm.Org == nil {
		return storage.Record{}, false
	}
	object, ok := vm.Org.Objects["ProcessDefinition"]
	if !ok {
		return storage.Record{}, false
	}
	for _, record := range sortedStorageRecords(object.Records) {
		if !approvalProcessDefinitionActive(record) {
			continue
		}
		if strings.EqualFold(storageStringField(record, "TableEnumOrId"), objectName) {
			return record, true
		}
	}
	return storage.Record{}, false
}

func approvalProcessDefinitionActive(record storage.Record) bool {
	state := strings.TrimSpace(storageStringField(record, "State"))
	return state == "" || strings.EqualFold(state, "Active")
}

func (vm *VM) approvalSubmitActor(request Value, definition storage.Record) (storage.ID, error) {
	if actorIDs := approvalRequestIDList(request, "NextApproverIds"); len(actorIDs) != 0 {
		if len(actorIDs) != 1 || strings.HasPrefix(string(actorIDs[0]), "00G") {
			return "", unsupportedCallError("Approval.process hosted approval engine routing")
		}
		return actorIDs[0], nil
	}
	node, ok := vm.approvalFirstProcessNode(definition.ID)
	if ok {
		for _, field := range []string{"ActorId", "ApproverId", "AssignedApproverId"} {
			if actorID := storage.ID(strings.TrimSpace(storageStringField(node, field))); actorID != "" {
				if strings.HasPrefix(string(actorID), "00G") {
					return "", unsupportedCallError("Approval.process hosted approval engine routing")
				}
				return actorID, nil
			}
		}
	}
	if current := vm.currentUserID(); current != "" {
		return storage.ID(current), nil
	}
	return "005000000000001", nil
}

func approvalRequestIDList(request Value, field string) []storage.ID {
	_, value, ok := objectFieldValue(request, field)
	if !ok || value.Kind != ValueList {
		return nil
	}
	out := make([]storage.ID, 0, len(value.List))
	for _, item := range value.List {
		text, ok := approvalValueIDText(item)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		out = append(out, storage.ID(text))
	}
	return out
}

func (vm *VM) approvalFirstProcessNode(definitionID storage.ID) (storage.Record, bool) {
	if vm == nil || vm.Org == nil {
		return storage.Record{}, false
	}
	object, ok := vm.Org.Objects["ProcessNode"]
	if !ok {
		return storage.Record{}, false
	}
	for _, record := range sortedStorageRecords(object.Records) {
		if storage.IDsEqual(storage.ID(strings.TrimSpace(storageStringField(record, "ProcessDefinitionId"))), definitionID) {
			return record, true
		}
	}
	return storage.Record{}, false
}

func (vm *VM) approvalCreateProcessInstance(objectID storage.ID, definition storage.Record) (storage.ID, error) {
	submitterID := storage.ID(vm.currentUserID())
	if submitterID == "" {
		submitterID = "005000000000001"
	}
	id, err := vm.approvalNextID("ProcessInstance", "04g")
	if err != nil {
		return "", err
	}
	record := storage.Record{
		ID:     id,
		Object: "ProcessInstance",
		Fields: map[string]storage.Value{
			"Id":                  storage.IDValue(id),
			"TargetObjectId":      storage.IDValue(objectID),
			"ProcessDefinitionId": storage.IDValue(definition.ID),
			"Status":              storage.StringValue("Pending"),
			"SubmittedById":       storage.IDValue(submitterID),
			"IsDeleted":           storage.BooleanValue(false),
		},
	}
	vm.approvalStoreRecord(record)
	return id, nil
}

func (vm *VM) approvalCreateWorkitem(instanceID storage.ID, actorID storage.ID) (storage.ID, error) {
	id, err := vm.approvalNextID("ProcessInstanceWorkitem", "04i")
	if err != nil {
		return "", err
	}
	record := storage.Record{
		ID:     id,
		Object: "ProcessInstanceWorkitem",
		Fields: map[string]storage.Value{
			"Id":                storage.IDValue(id),
			"ProcessInstanceId": storage.IDValue(instanceID),
			"ActorId":           storage.IDValue(actorID),
			"OriginalActorId":   storage.IDValue(actorID),
			"IsDeleted":         storage.BooleanValue(false),
		},
	}
	vm.approvalStoreRecord(record)
	return id, nil
}

func (vm *VM) approvalApplyWorkitemAction(workitemID storage.ID, action string, comments string, allOrNone bool) (Value, error) {
	status := ""
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "approve", "approved":
		status = "Approved"
	case "reject", "rejected":
		status = "Rejected"
	default:
		return approvalProcessFailureResult(allOrNone, platformScalar("Id", string(workitemID)), "INVALID_FIELD", "Action is invalid", []string{"Action"})
	}
	workitem, ok := vm.approvalRecordByID("ProcessInstanceWorkitem", workitemID)
	if !ok {
		return approvalProcessFailureResult(allOrNone, platformScalar("Id", string(workitemID)), "INVALID_CROSS_REFERENCE_KEY", "WorkitemId record is missing", []string{"WorkitemId"})
	}
	instanceID := storage.ID(strings.TrimSpace(storageStringField(workitem, "ProcessInstanceId")))
	instance, ok := vm.approvalRecordByID("ProcessInstance", instanceID)
	if !ok {
		return approvalProcessFailureResult(allOrNone, platformScalar("Id", string(workitemID)), "INVALID_CROSS_REFERENCE_KEY", "ProcessInstance record is missing", []string{"ProcessInstanceId"})
	}
	actorID := storage.ID(strings.TrimSpace(storageStringField(workitem, "ActorId")))
	instanceBefore := instance.Clone()
	instance.Fields["Status"] = storage.StringValue(status)
	instance.Fields["LastActorId"] = storage.IDValue(actorID)
	vm.approvalUpdateRecord(instance, instanceBefore)

	workitemBefore := workitem.Clone()
	workitem.Fields["IsDeleted"] = storage.BooleanValue(true)
	vm.approvalUpdateRecord(workitem, workitemBefore)
	vm.approvalCreateStep(instanceID, actorID, status, comments)

	result := approvalProcessResult(true, platformScalar("Id", string(workitemID)), nil)
	result.Fields["instanceId"] = platformScalar("Id", string(instanceID))
	result.Fields["instanceStatus"] = String(status)
	result.Fields["actorIds"] = List(platformScalar("Id", string(actorID)))
	return result, nil
}

func (vm *VM) approvalCreateStep(instanceID storage.ID, actorID storage.ID, status string, comments string) {
	id, err := vm.approvalNextID("ProcessInstanceStep", "04h")
	if err != nil {
		return
	}
	record := storage.Record{
		ID:     id,
		Object: "ProcessInstanceStep",
		Fields: map[string]storage.Value{
			"Id":                storage.IDValue(id),
			"ProcessInstanceId": storage.IDValue(instanceID),
			"ActorId":           storage.IDValue(actorID),
			"OriginalActorId":   storage.IDValue(actorID),
			"StepStatus":        storage.StringValue(status),
			"Comments":          storage.StringValue(comments),
		},
	}
	vm.approvalStoreRecord(record)
}

func (vm *VM) approvalObjectNameForID(id storage.ID) (string, bool) {
	if vm == nil || vm.Org == nil || id == "" {
		return "", false
	}
	for objectName, object := range vm.Org.Objects {
		if _, _, ok := storage.LookupRecordByID(object.Records, id); ok {
			return objectName, true
		}
	}
	return vm.sObjectNameForID(string(id))
}

func (vm *VM) approvalRecordByID(objectName string, id storage.ID) (storage.Record, bool) {
	if vm == nil || vm.Org == nil || id == "" {
		return storage.Record{}, false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return storage.Record{}, false
	}
	_, record, ok := storage.LookupRecordByID(object.Records, id)
	return record, ok
}

func (vm *VM) approvalStoreRecord(record storage.Record) {
	if vm == nil || vm.Org == nil || record.Object == "" || record.ID == "" {
		return
	}
	storage.EnsureStandardObject(vm.Org, record.Object)
	object := vm.Org.Objects[record.Object]
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	vm.recordIsolationJournalMutation(record.Object, record.ID, storage.Record{}, false)
	object.Records[record.ID] = record
	vm.Org.Objects[record.Object] = object
}

func (vm *VM) approvalUpdateRecord(record storage.Record, before storage.Record) {
	if vm == nil || vm.Org == nil || record.Object == "" || record.ID == "" {
		return
	}
	storage.EnsureStandardObject(vm.Org, record.Object)
	object := vm.Org.Objects[record.Object]
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	vm.recordIsolationJournalMutation(record.Object, record.ID, before, true)
	object.Records[record.ID] = record
	vm.Org.Objects[record.Object] = object
}

func (vm *VM) approvalNextID(objectName string, fallbackPrefix string) (storage.ID, error) {
	if vm == nil || vm.Org == nil {
		return "", fmt.Errorf("Approval.process requires org storage")
	}
	storage.EnsureStandardObject(vm.Org, objectName)
	storage.EnsureUniqueKeyPrefixes(vm.Org)
	object := vm.Org.Objects[objectName]
	prefix := strings.TrimSpace(object.Definition.KeyPrefix)
	if prefix == "" {
		prefix = fallbackPrefix
	}
	if vm.Org.IDSequences == nil {
		vm.Org.IDSequences = make(map[string]uint64)
	}
	vm.recordIsolationJournalSequence(objectName)
	generator := storage.NewRuntimeIDGenerator(map[string]string{objectName: prefix})
	generator.Sequences = vm.Org.IDSequences
	id, err := generator.Next(objectName)
	if err != nil {
		return "", err
	}
	vm.Org.IDSequences[objectName] = generator.Sequences[objectName]
	return id, nil
}

func approvalValueIDText(value Value) (string, bool) {
	if text, ok := idValueText(value); ok {
		return text, true
	}
	if value.Kind == ValueObject {
		if _, id, ok := objectFieldValue(value, "Id"); ok {
			return idValueText(id)
		}
	}
	return "", false
}

func sortedStorageRecords(records map[storage.ID]storage.Record) []storage.Record {
	keys := make([]string, 0, len(records))
	byID := make(map[string]storage.Record, len(records))
	for id, record := range records {
		key := string(id)
		keys = append(keys, key)
		byID[key] = record
	}
	sort.Strings(keys)
	out := make([]storage.Record, 0, len(keys))
	for _, key := range keys {
		out = append(out, byID[key])
	}
	return out
}
