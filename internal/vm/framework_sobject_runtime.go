package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) callFrameworkIDGeneratorGenerate(method Method, args []Value) (Value, bool) {
	if !strings.EqualFold(frameworkMockSupportType(method.ClassName), "IDGenerator") ||
		!strings.EqualFold(apexMethodMemberName(method.Name), "generate") ||
		len(args) != 1 || !isSObjectTypeToken(args[0]) {
		return Null, false
	}
	objectName, ok := sObjectTypeTokenObjectName(args[0])
	if !ok {
		return Null, false
	}
	if strings.EqualFold(objectName, "Organization") {
		return platformScalar("Id", vm.nextFrameworkGeneratedID(objectName, "00D", 1)), true
	}
	if strings.EqualFold(objectName, "User") {
		return platformScalar("Id", vm.nextFrameworkGeneratedID(objectName, "005", 998)), true
	}
	return Null, false
}

func (vm *VM) nextFrameworkGeneratedID(objectName, prefix string, floor uint64) string {
	if vm == nil {
		return fmt.Sprintf("%s%012d", prefix, floor+1)
	}
	if vm.frameworkIDSequences == nil {
		vm.frameworkIDSequences = make(map[string]uint64)
	}
	next := vm.frameworkIDSequences[objectName]
	if next < floor {
		next = floor
	}
	if vm.Org != nil && vm.Org.IDSequences != nil {
		if orgNext := vm.Org.IDSequences[objectName]; orgNext > next {
			next = orgNext
		}
	}
	next++
	vm.frameworkIDSequences[objectName] = next
	return fmt.Sprintf("%s%012d", prefix, next)
}

func (vm *VM) callFrameworkSObjectDescribeMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if strings.EqualFold(receiver.Type, "framework_SObjectDescribe") {
		return vm.callFrameworkSObjectDescribe(receiver, method, args, result)
	}
	if strings.EqualFold(receiver.Type, "framework_SObjectDescribe.FieldsMap") ||
		strings.EqualFold(receiver.Type, "framework_SObjectDescribe.GlobalDescribeMap") {
		return vm.callFrameworkNamespacedAttributeMap(receiver, method, args)
	}
	return Null, false, nil
}
func (vm *VM) callFrameworkSObjectDomainTriggerHandler(args []Value) (Value, bool, error) {
	if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "Type") {
		return Null, true, fmt.Errorf("framework_SObjectDomain.triggerHandler expects Type")
	}
	domainClassName := typeValueName(args[0])
	if domainClassName == "" {
		return Null, true, fmt.Errorf("framework_SObjectDomain.triggerHandler Type is blank")
	}
	if ctx, ok := vm.frameworkMockDatabaseContext(false); ok {
		afterCtx, hasAfterCtx := vm.frameworkMockDatabaseContext(true)
		saved := vm.triggerGlobals
		vm.triggerGlobals = ctx
		beforeDomain, _, err := vm.callFrameworkSObjectDomainTriggerHandlerForContext(domainClassName)
		if err != nil {
			vm.triggerGlobals = saved
			return Null, true, err
		}
		if hasAfterCtx {
			vm.triggerGlobals = afterCtx
			domainOverride := Null
			if frameworkDomainTriggerStateEnabled(beforeDomain) {
				domainOverride = beforeDomain
			}
			if _, _, err := vm.callFrameworkSObjectDomainTriggerHandlerForContextWithDomain(domainClassName, domainOverride); err != nil {
				vm.triggerGlobals = saved
				return Null, true, err
			}
		}
		vm.triggerGlobals = saved
		return Null, true, nil
	}
	return vm.callFrameworkSObjectDomainTriggerHandlerForContext(domainClassName)
}
func (vm *VM) callFrameworkSObjectDomainTriggerHandlerForContext(domainClassName string) (Value, bool, error) {
	return vm.callFrameworkSObjectDomainTriggerHandlerForContextWithDomain(domainClassName, Null)
}
func (vm *VM) callFrameworkSObjectDomainTriggerHandlerForContextWithDomain(domainClassName string, domainOverride Value) (Value, bool, error) {
	if !vm.frameworkTriggerEventEnabled(domainClassName) {
		return Null, true, nil
	}
	records := vm.triggerGlobals["Trigger.new"]
	if records.Kind == ValueNull || records.Kind == "" {
		records = vm.triggerGlobals["Trigger.old"]
	}
	if records.Kind != ValueList {
		records = List()
		records.Type = "List<SObject>"
	}
	records = withConcreteSObjectListRuntime(records)
	domain := domainOverride
	var err error
	if domain.Kind != ValueObject {
		domain, err = vm.constructValue(domainClassName, []Value{records}, nil, resultForLookup())
		if err != nil && hasSuffixFold(domainClassName, ".constructor") {
			domain, err = vm.constructDomainThroughFrameworkConstructor(domainClassName, records)
		}
		if err != nil {
			constructorName := domainClassName
			if !hasSuffixFold(constructorName, "constructor") {
				constructorName += ".Constructor"
			}
			domain, err = vm.constructDomainThroughFrameworkConstructor(constructorName, records)
		}
		if err != nil {
			return Null, true, err
		}
	}
	handler := ""
	switch {
	case triggerBool(vm.triggerGlobals, "Trigger.isBefore") && triggerBool(vm.triggerGlobals, "Trigger.isInsert"):
		handler = "handleBeforeInsert"
	case triggerBool(vm.triggerGlobals, "Trigger.isBefore") && triggerBool(vm.triggerGlobals, "Trigger.isUpdate"):
		handler = "handleBeforeUpdate"
	case triggerBool(vm.triggerGlobals, "Trigger.isBefore") && triggerBool(vm.triggerGlobals, "Trigger.isDelete"):
		handler = "handleBeforeDelete"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isInsert"):
		handler = "handleAfterInsert"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isUpdate"):
		handler = "handleAfterUpdate"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isDelete"):
		handler = "handleAfterDelete"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isUndelete"):
		handler = "handleAfterUndelete"
	}
	if handler == "" {
		return domain, true, nil
	}
	handlerArgs := []Value(nil)
	if strings.EqualFold(handler, "handleBeforeUpdate") || strings.EqualFold(handler, "handleAfterUpdate") {
		oldMap := vm.triggerGlobals["Trigger.oldMap"]
		if oldMap.Kind == ValueNull || oldMap.Kind == "" {
			oldMap = typedMap("Map<Id,SObject>")
		} else if oldMap.Kind == ValueMap && strings.EqualFold(oldMap.Runtime, "Map<Id,SObject>") {
			oldMap.Type = "Map<Id,SObject>"
		}
		handlerArgs = []Value{oldMap}
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(domain.Type, handler, handlerArgs)
	if ambiguous {
		return Null, true, vm.ambiguousOverloadError(domain.Type+"."+handler, handlerArgs)
	}
	if !ok {
		if handled, err := vm.callFrameworkSObjectDomainBaseHandler(domain, handler, handlerArgs, resultForLookup()); handled || err != nil {
			return Null, true, err
		}
		return Null, true, fmt.Errorf("%s.%s not found", domain.Type, handler)
	}
	if _, err := vm.callMethodWithReceiver(target, domain, handlerArgs, resultForLookup()); err != nil {
		return Null, true, err
	}
	if triggerBool(vm.triggerGlobals, "Trigger.isBefore") && !triggerBool(vm.triggerGlobals, "Trigger.isDelete") {
		if records, ok := frameworkDomainRecords(domain); ok {
			vm.triggerGlobals["Trigger.new"] = records
		}
	}
	return domain, true, nil
}
func (vm *VM) callFrameworkSObjectDomainBaseHandler(domain Value, handler string, handlerArgs []Value, result *Result) (bool, error) {
	call := func(name string, args []Value) error {
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(domain.Type, name, args)
		if ambiguous {
			return vm.ambiguousOverloadError(domain.Type+"."+name, args)
		}
		if !ok {
			return nil
		}
		_, err := vm.callMethodWithReceiver(target, domain, args, result)
		return err
	}
	switch strings.ToLower(handler) {
	case "handlebeforeinsert":
		if err := call("onApplyDefaults", nil); err != nil {
			return true, err
		}
		return true, call("onBeforeInsert", nil)
	case "handlebeforeupdate":
		return true, call("onBeforeUpdate", handlerArgs)
	case "handlebeforedelete":
		return true, call("onBeforeDelete", nil)
	case "handleafterinsert":
		if err := call("onValidate", nil); err != nil {
			return true, err
		}
		return true, call("onAfterInsert", nil)
	case "handleafterupdate":
		if err := call("onValidate", nil); err != nil {
			return true, err
		}
		if err := call("onValidate", handlerArgs); err != nil {
			return true, err
		}
		return true, call("onAfterUpdate", handlerArgs)
	case "handleafterdelete":
		return true, call("onAfterDelete", nil)
	case "handleafterundelete":
		return true, call("onAfterUndelete", nil)
	default:
		return false, nil
	}
}
func frameworkDomainTriggerStateEnabled(domain Value) bool {
	if domain.Kind != ValueObject {
		return false
	}
	_, config, ok := objectFieldValue(domain, "Configuration")
	if !ok || config.Kind != ValueObject {
		return false
	}
	_, enabled, ok := objectFieldValue(config, "TriggerStateEnabled")
	return ok && enabled.Kind == ValueBool && enabled.Bool
}
func (vm *VM) constructDomainThroughFrameworkConstructor(constructorName string, records Value) (Value, error) {
	constructor, err := vm.constructValue(constructorName, nil, nil, resultForLookup())
	if err != nil {
		return Null, err
	}
	construct, ok, ambiguous := vm.resolveInstanceMethodForArgs(constructor.Type, "construct", []Value{records})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(constructor.Type+".construct", []Value{records})
	}
	if !ok {
		return Null, fmt.Errorf("%s.construct(List<SObject>) not found", constructor.Type)
	}
	return vm.callMethodWithReceiver(construct, constructor, []Value{records}, resultForLookup())
}
func frameworkDomainRecords(domain Value) (Value, bool) {
	if domain.Kind != ValueObject {
		return Null, false
	}
	for name, value := range domain.Fields {
		if strings.EqualFold(name, "objects") && value.Kind == ValueList {
			return value, true
		}
	}
	return Null, false
}
func (vm *VM) callFrameworkSObjectDescribe(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	token, ok := frameworkSObjectDescribeToken(receiver)
	if !ok {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "getsobjecttype":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getSObjectType expects 0 arguments")
		}
		return token, true, nil
	case "getdescribe":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getDescribe expects 0 arguments")
		}
		describe, _, _, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
		if err != nil || !handled {
			return describe, true, err
		}
		return describe, true, nil
	case "getfieldsmap":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getFieldsMap expects 0 arguments")
		}
		fields, err := vm.frameworkSObjectDescribeFields(token, result)
		return fields, true, err
	case "getfield":
		if len(args) != 1 && len(args) != 2 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getField expects field name")
		}
		if args[0].Kind == ValueNull {
			return Null, true, nil
		}
		if args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getField expects String")
		}
		implyNamespace := true
		if len(args) == 2 {
			if args[1].Kind != ValueBool {
				return Null, true, fmt.Errorf("framework_SObjectDescribe.getField implyNamespace expects Boolean")
			}
			implyNamespace = args[1].Bool
		}
		field, err := vm.frameworkSObjectDescribeField(token, args[0].Text, implyNamespace, result)
		return field, true, err
	case "getnamefield":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getNameField expects 0 arguments")
		}
		field, err := vm.frameworkSObjectDescribeNameField(token, result)
		return field, true, err
	}
	return Null, false, nil
}
func frameworkSObjectDescribeToken(receiver Value) (Value, bool) {
	_, token, ok := objectFieldValue(receiver, "token")
	return token, ok && token.Kind == ValueObject && strings.EqualFold(token.Type, "Schema.SObjectType")
}
func (vm *VM) frameworkSObjectDescribeFields(token Value, result *Result) (Value, error) {
	describe, _, _, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
	if err != nil || !handled {
		return describe, err
	}
	fields, ok := describe.Fields["fields"]
	if !ok {
		return Null, fmt.Errorf("framework_SObjectDescribe fields are not available")
	}
	value, _, _, _, err := vm.callPlatformObjectMember(fields, "getMap", nil, result)
	return value, err
}
func (vm *VM) frameworkSObjectDescribeField(token Value, fieldName string, implyNamespace bool, result *Result) (Value, error) {
	fields, err := vm.frameworkSObjectDescribeFields(token, result)
	if err != nil {
		return Null, err
	}
	lookupName := fieldName
	if hasSuffixFold(lookupName, "__r") {
		lookupName = lookupName[:len(lookupName)-len("__r")] + "__c"
	}
	value := vm.frameworkNamespacedAttributeMapGet(fields, lookupName, implyNamespace, "")
	if value.Kind == ValueNull {
		value = vm.frameworkNamespacedAttributeMapGet(fields, fieldName+"Id", implyNamespace, "")
	}
	return value, nil
}
func (vm *VM) frameworkSObjectDescribeNameField(token Value, result *Result) (Value, error) {
	fields, err := vm.frameworkSObjectDescribeFields(token, result)
	if err != nil {
		return Null, err
	}
	for _, key := range sortedMapKeys(fields.Map) {
		field := fields.Map[key]
		describe, _, _, handled, err := vm.callPlatformObjectMember(field, "getDescribe", nil, result)
		if err != nil || !handled {
			return Null, err
		}
		if isNameField, ok := describe.Fields["nameField"]; ok && isNameField.Kind == ValueBool && isNameField.Bool {
			return field, nil
		}
	}
	return Null, nil
}
func (vm *VM) callFrameworkSimpleDMLMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if !strings.EqualFold(receiver.Type, "framework_SObjectUnitOfWork.SimpleDML") {
		return Null, false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.SimpleDML.%s expects List<SObject>", method)
	}
	switch strings.ToLower(method) {
	case "dmlinsert":
		_, err := vm.executeDatabaseDML("insert", []Value{args[0]}, result)
		return Null, true, err
	case "dmlupdate":
		_, err := vm.executeDatabaseDML("update", []Value{args[0]}, result)
		return Null, true, err
	case "dmldelete":
		_, err := vm.executeDatabaseDML("delete", []Value{args[0]}, result)
		return Null, true, err
	case "eventpublish":
		_, err := vm.eventBusPublish([]Value{args[0]}, result)
		return Null, true, err
	case "emptyrecyclebin":
		if len(args[0].List) == 0 {
			return Null, true, nil
		}
		_, err := vm.executeDatabaseRecordAction("emptyRecycleBin", []Value{args[0]}, result, "Database.EmptyRecycleBinResult")
		return Null, true, err
	default:
		return Null, false, nil
	}
}
func (vm *VM) callFrameworkSObjectUnitOfWorkMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if strings.EqualFold(frameworkMockSupportType(receiver.Type), "SObjectUnitOfWork.SendEmailWork") {
		switch strings.ToLower(method) {
		case "registeremail":
			if len(args) != 1 || !messagingEmailAssignable(args[0].Type, "Messaging.Email") {
				return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.SendEmailWork.registerEmail expects Messaging.Email")
			}
			emails, ok := receiver.Fields["emails"]
			if !ok || emails.Kind != ValueList {
				emails = typedList("List<Messaging.Email>")
			}
			emails.List = append(emails.List, args[0])
			receiver.Fields["emails"] = emails
			return Null, true, nil
		case "dowork":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.SendEmailWork.doWork expects 0 arguments")
			}
			emails, ok := receiver.Fields["emails"]
			if !ok || emails.Kind != ValueList || len(emails.List) == 0 {
				return Null, true, nil
			}
			_, err := vm.sendEmail([]Value{emails}, result)
			return Null, true, err
		}
	}
	if frameworkSObjectUnitOfWorkRelationshipsType(receiver.Type) {
		switch strings.ToLower(method) {
		case "add":
			var relationship Value
			if len(args) == 1 && args[0].Kind == ValueObject {
				relationship = args[0]
			} else if len(args) == 2 && args[0].Kind == ValueObject && args[1].Kind == ValueObject {
				// fflib email relationship: add(Messaging.SingleEmailMessage, SObject)
				if !strings.Contains(strings.ToLower(args[0].Type), "singleemailmessage") {
					return Null, true, fmt.Errorf("%s.add expects relationship object or record, field, relatedTo", receiver.Type)
				}
				relationship = Object("framework_SObjectUnitOfWork.EmailRelationship")
				relationship.Fields["email"] = args[0]
				relationship.Fields["relatedTo"] = args[1]
			} else if len(args) == 3 && args[0].Kind == ValueObject && args[1].Kind == ValueObject && args[2].Kind == ValueObject {
				fieldName, err := vm.sObjectFieldArg(args[0].Type, args[1])
				if err != nil {
					return Null, true, err
				}
				relationship = Object("framework_SObjectUnitOfWork.Relationship")
				relationship.Fields["Record"] = args[0]
				relationship.Fields["RelatedToField"] = sObjectFieldToken(vm.canonicalSObjectValueType(args[0]), fieldName)
				relationship.Fields["RelatedTo"] = args[2]
			} else if len(args) == 4 && args[0].Kind == ValueObject && args[1].Kind == ValueObject && args[2].Kind == ValueObject {
				// fflib external-id relationship: add(SObject, SObjectField, SObjectField, Object)
				fieldName, err := vm.sObjectFieldArg(args[0].Type, args[1])
				if err != nil {
					return Null, true, err
				}
				relationshipName := fieldName
				if hasSuffixFold(relationshipName, "__c") {
					relationshipName = relationshipName[:len(relationshipName)-3] + "__r"
				} else if hasSuffixFold(relationshipName, "id") {
					relationshipName = relationshipName[:len(relationshipName)-2]
				}
				relationship = Object("framework_SObjectUnitOfWork.RelationshipByExternalId")
				relationship.Fields["Record"] = args[0]
				relationship.Fields["RelatedToField"] = sObjectFieldToken(vm.canonicalSObjectValueType(args[0]), fieldName)
				relationship.Fields["ExternalIdField"] = args[2]
				relationship.Fields["ExternalId"] = args[3]
				relationship.Fields["RelationshipName"] = String(relationshipName)
			} else {
				return Null, true, fmt.Errorf("%s.add expects relationship object or record, field, relatedTo", receiver.Type)
			}
			relationships, ok := receiver.Fields["m_relationships"]
			if !ok || relationships.Kind != ValueList {
				relationships = typedList("List<framework_SObjectUnitOfWork.IRelationship>")
			}
			relationships.List = append(relationships.List, relationship)
			receiver.Fields["m_relationships"] = relationships
			return Null, true, nil
		}
	}
	if !vm.isSObjectUnitOfWorkRuntimeType(receiver.Type) {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "commitwork":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.commitWork expects 0 arguments")
		}
		return Null, true, vm.commitFrameworkSObjectUnitOfWork(receiver, result)
	case "handleregistertype":
		return vm.callFrameworkSObjectUnitOfWorkHandleRegisterType(receiver, args)
	case "registernew":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkNew(receiver, args)
	case "registerdirty":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkDirty(receiver, args)
	case "registerdeleted":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_deletedMapByType", args, true)
	case "registerupsert":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkUpsert(receiver, args)
	case "registeremptyrecyclebin":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_emptyRecycleBinMapByType", args, true)
	case "registerpermanentlydeleted":
		if err := vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_emptyRecycleBinMapByType", args, true); err != nil {
			return Null, true, err
		}
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_deletedMapByType", args, true)
	case "registerpublishbeforetransaction":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_publishBeforeListByType", args, false)
	case "registerpublishaftersuccesstransaction":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_publishAfterSuccessListByType", args, false)
	case "registerpublishafterfailuretransaction":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_publishAfterFailureListByType", args, false)
	default:
		return Null, false, nil
	}
}
func (vm *VM) callFrameworkSObjectUnitOfWorkHandleRegisterType(receiver Value, args []Value) (Value, bool, error) {
	if len(args) != 1 || !isSObjectTypeToken(args[0]) {
		return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.handleRegisterType expects Schema.SObjectType")
	}
	objectName, ok := sObjectTypeTokenObjectName(args[0])
	if !ok || objectName == "" {
		return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.handleRegisterType requires SObjectType object name")
	}
	if vm.Org != nil {
		if canonical, resolved := vm.resolveObjectName(objectName); resolved && canonical != "" {
			objectName = canonical
		}
	}
	receiver.Fields["m_newListByType"] = frameworkMapPut(receiver.Fields["m_newListByType"], String(objectName), typedList("List<SObject>"))
	receiver.Fields["m_dirtyMapByType"] = frameworkMapPut(receiver.Fields["m_dirtyMapByType"], String(objectName), typedMap("Map<Id,SObject>"))
	receiver.Fields["m_upsertRecordsPerType"] = frameworkMapPut(receiver.Fields["m_upsertRecordsPerType"], String(objectName), typedList("List<SObject>"))
	receiver.Fields["m_deletedMapByType"] = frameworkMapPut(receiver.Fields["m_deletedMapByType"], String(objectName), typedMap("Map<Id,SObject>"))
	receiver.Fields["m_emptyRecycleBinMapByType"] = frameworkMapPut(receiver.Fields["m_emptyRecycleBinMapByType"], String(objectName), typedMap("Map<Id,SObject>"))
	receiver.Fields["m_relationships"] = frameworkMapPut(receiver.Fields["m_relationships"], String(objectName), frameworkUnitOfWorkRelationships())
	receiver.Fields["m_publishBeforeListByType"] = frameworkMapPut(receiver.Fields["m_publishBeforeListByType"], String(objectName), typedList("List<SObject>"))
	receiver.Fields["m_publishAfterSuccessListByType"] = frameworkMapPut(receiver.Fields["m_publishAfterSuccessListByType"], String(objectName), typedList("List<SObject>"))
	receiver.Fields["m_publishAfterFailureListByType"] = frameworkMapPut(receiver.Fields["m_publishAfterFailureListByType"], String(objectName), typedList("List<SObject>"))
	// Derived UoW implementations rely on this callback during registration.
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onRegisterType", []Value{args[0]}, &Result{}); err != nil {
		return Null, true, err
	}
	return Null, true, nil
}
func (vm *VM) registerFrameworkSObjectUnitOfWorkUpsert(receiver Value, args []Value) error {
	records, err := vm.frameworkSObjectUnitOfWorkRegisterRecords(args)
	if err != nil {
		return err
	}
	if len(args) >= 2 {
		if args[1].Kind == ValueNull {
			return frameworkSObjectUnitOfWorkException("Invalid argument: externalIdField. If you want to upsert by id, use the registerUpsert method that has only one argument")
		}
		if args[1].Kind != ValueObject || !isSObjectFieldTokenType(args[1].Type) {
			return frameworkSObjectUnitOfWorkException("Invalid argument: externalIdField. Field supplied is not a known field on the target sObject.")
		}
		var relationshipField Value
		var relatedTo Value
		hasRelationship := false
		if len(args) >= 4 && args[2].Kind == ValueObject && isSObjectFieldTokenType(args[2].Type) && args[3].Kind == ValueObject && sObjectValueType(args[3].Type) {
			relationshipField = args[2]
			relatedTo = args[3]
			hasRelationship = true
		}
		for _, record := range records {
			objectName := vm.canonicalSObjectValueType(record)
			fieldName, field, err := vm.frameworkSObjectUnitOfWorkUpsertField(objectName, args[1])
			if err != nil {
				return err
			}
			registeredExternalID := vm.frameworkSObjectUnitOfWorkExternalIDField(receiver, objectName)
			if registeredExternalID.Kind != ValueNull {
				registeredField, err := vm.sObjectFieldArg(objectName, registeredExternalID)
				if err != nil {
					return err
				}
				if strings.EqualFold(registeredField, fieldName) {
					if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_upsertRecordsPerType", record, false); err != nil {
						return err
					}
					if hasRelationship {
						if err := vm.addFrameworkSObjectUnitOfWorkRelationship(receiver, objectName, record, relationshipField, relatedTo); err != nil {
							return err
						}
					}
					continue
				}
				return frameworkSObjectUnitOfWorkException(fmt.Sprintf("SObject type %s has already registered an upsert by external id %s, you cannot use another in this unit of work.", objectName, registeredField))
			}
			if !vm.frameworkSObjectUnitOfWorkDMLSupportsUpsert(receiver) {
				return frameworkSObjectUnitOfWorkException("Upsert by external ID requires IDMLUpsertable implementation. Current IDML implementation does not support this feature.")
			}
			if id := sObjectIDValue(record); id.Kind != ValueNull && !strings.EqualFold(fieldName, "Id") {
				return frameworkSObjectUnitOfWorkException("When upserting by external id, the record cannot already have the standard Id populated")
			}
			if !frameworkSObjectUnitOfWorkFieldCanUpsert(field) {
				return frameworkSObjectUnitOfWorkException("Invalid argument: externalIdField. Field supplied cannot be used with upsert.")
			}
			if err := vm.setFrameworkSObjectUnitOfWorkExternalIDField(receiver, objectName, args[1]); err != nil {
				return err
			}
			if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_upsertRecordsPerType", record, false); err != nil {
				return err
			}
			if hasRelationship {
				if err := vm.addFrameworkSObjectUnitOfWorkRelationship(receiver, objectName, record, relationshipField, relatedTo); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, record := range records {
		if sObjectIDValue(record).Kind == ValueNull {
			if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_newListByType", record, false); err != nil {
				return err
			}
		} else if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_dirtyMapByType", record, true); err != nil {
			return err
		}
	}
	return nil
}

func frameworkSObjectUnitOfWorkException(message string) error {
	return newExceptionError("framework_SObjectUnitOfWork.UnitOfWorkException", message)
}

func (vm *VM) frameworkSObjectUnitOfWorkUpsertField(objectName string, token Value) (string, storage.Field, error) {
	fieldName, err := vm.sObjectFieldArg(objectName, token)
	if err != nil {
		return "", storage.Field{}, frameworkSObjectUnitOfWorkException("Invalid argument: externalIdField. Field supplied is not a known field on the target sObject.")
	}
	_, definition, ok := vm.describeObjectDefinition(objectName)
	if !ok {
		return "", storage.Field{}, frameworkSObjectUnitOfWorkException("Invalid argument: externalIdField. Field supplied is not a known field on the target sObject.")
	}
	canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return "", storage.Field{}, frameworkSObjectUnitOfWorkException("Invalid argument: externalIdField. Field supplied is not a known field on the target sObject.")
	}
	return canonical, definition.Fields[canonical], nil
}

func (vm *VM) frameworkSObjectUnitOfWorkDMLSupportsUpsert(receiver Value) bool {
	dmlValue, ok := receiver.Fields["m_dml"]
	if !ok || dmlValue.Kind != ValueObject {
		return false
	}
	if strings.EqualFold(dmlValue.Type, "framework_SObjectUnitOfWork.SimpleDML") {
		return true
	}
	return vm.typeAssignableTo(dmlValue.Type, "framework_SObjectUnitOfWork.IDMLUpsertable")
}

func frameworkSObjectUnitOfWorkFieldCanUpsert(field storage.Field) bool {
	return field.Type == storage.FieldID || field.IDLookup || field.ExternalID
}

func (vm *VM) setFrameworkSObjectUnitOfWorkExternalIDField(receiver Value, objectName string, field Value) error {
	if _, err := vm.sObjectFieldArg(objectName, field); err != nil {
		return err
	}
	bucket, ok := receiver.Fields["m_externalIdToUpsertPerType"]
	if !ok || bucket.Kind != ValueMap {
		bucket = typedMap("Map<String,Schema.SObjectField>")
	}
	receiver.Fields["m_externalIdToUpsertPerType"] = frameworkMapPut(bucket, String(objectName), field)
	return nil
}
func (vm *VM) registerFrameworkSObjectUnitOfWorkNew(receiver Value, args []Value) error {
	records, err := vm.frameworkSObjectUnitOfWorkRegisterRecords(args)
	if err != nil {
		return err
	}
	var relationshipField Value
	var relatedTo Value
	hasRelationship := false
	if len(args) >= 3 && args[1].Kind == ValueObject && isSObjectFieldTokenType(args[1].Type) && args[2].Kind == ValueObject && sObjectValueType(args[2].Type) {
		relationshipField = args[1]
		relatedTo = args[2]
		hasRelationship = true
	}
	for _, record := range records {
		if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_newListByType", record, false); err != nil {
			return err
		}
		if hasRelationship {
			objectName := vm.canonicalSObjectValueType(record)
			if err := vm.addFrameworkSObjectUnitOfWorkRelationship(receiver, objectName, record, relationshipField, relatedTo); err != nil {
				return err
			}
		}
	}
	return nil
}
func (vm *VM) registerFrameworkSObjectUnitOfWorkDirty(receiver Value, args []Value) error {
	records, err := vm.frameworkSObjectUnitOfWorkRegisterRecords(args)
	if err != nil {
		return err
	}
	dirtyFields := frameworkSObjectUnitOfWorkDirtyFields(args)
	for _, record := range records {
		if err := vm.addFrameworkSObjectUnitOfWorkDirtyRecord(receiver, record, dirtyFields); err != nil {
			return err
		}
	}
	return nil
}
func frameworkSObjectUnitOfWorkDirtyFields(args []Value) []Value {
	if len(args) < 2 || args[1].Kind != ValueList || len(args[1].List) == 0 {
		return nil
	}
	fields := make([]Value, 0, len(args[1].List))
	for _, field := range args[1].List {
		if field.Kind == ValueObject && isSObjectFieldTokenType(field.Type) {
			fields = append(fields, field)
		}
	}
	return fields
}
func (vm *VM) registerFrameworkSObjectUnitOfWorkRecords(receiver Value, fieldName string, args []Value, requireID bool) error {
	records, err := vm.frameworkSObjectUnitOfWorkRegisterRecords(args)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, fieldName, record, requireID); err != nil {
			return err
		}
	}
	return nil
}
func (vm *VM) addFrameworkSObjectUnitOfWorkDirtyRecord(receiver Value, record Value, dirtyFields []Value) error {
	if len(dirtyFields) == 0 {
		return vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_dirtyMapByType", record, true)
	}
	existing, ok, err := vm.frameworkSObjectUnitOfWorkRegisteredRecord(receiver, "m_dirtyMapByType", record)
	if err != nil {
		return err
	}
	if !ok {
		return vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_dirtyMapByType", record, true)
	}
	for _, fieldToken := range dirtyFields {
		field, err := vm.sObjectFieldArg(record.Type, fieldToken)
		if err != nil {
			return err
		}
		actual, value, ok := objectFieldValue(record, field)
		if !ok {
			value = Null
			actual = field
		}
		setExplicitSObjectField(&existing, actual, value)
	}
	return vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_dirtyMapByType", existing, true)
}
func (vm *VM) frameworkSObjectUnitOfWorkRegisteredRecord(receiver Value, fieldName string, record Value) (Value, bool, error) {
	id := sObjectIDValue(record)
	if id.Kind == ValueNull {
		return Null, false, newExceptionError("framework_SObjectUnitOfWork.UnitOfWorkException", frameworkSObjectUnitOfWorkMissingIDMessage(fieldName))
	}
	objectName := vm.canonicalSObjectValueType(record)
	bucket, ok := receiver.Fields[fieldName]
	if !ok || bucket.Kind != ValueMap {
		return Null, false, fmt.Errorf("framework_SObjectUnitOfWork missing %s", fieldName)
	}
	value, ok := bucket.Map[vm.resolveObjectBucketKey(bucket, objectName)]
	if !ok || value.Kind != ValueMap {
		return Null, false, nil
	}
	registered, ok := value.Map[vm.mapKey(id)]
	return registered, ok, nil
}
func (vm *VM) frameworkSObjectUnitOfWorkRegisterRecords(args []Value) ([]Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("framework_SObjectUnitOfWork register method expects SObject or List<SObject>")
	}
	value := args[0]
	if value.Kind == ValueList {
		return append([]Value(nil), value.List...), nil
	}
	if value.Kind == ValueObject && sObjectValueType(value.Type) {
		return []Value{value}, nil
	}
	return nil, fmt.Errorf("framework_SObjectUnitOfWork register method expects SObject or List<SObject>")
}
func (vm *VM) addFrameworkSObjectUnitOfWorkRecord(receiver Value, fieldName string, record Value, requireID bool) error {
	if record.Kind != ValueObject || !sObjectValueType(record.Type) {
		return fmt.Errorf("framework_SObjectUnitOfWork register method expects SObject")
	}
	id := sObjectIDValue(record)
	if requireID && id.Kind == ValueNull {
		return newExceptionError("framework_SObjectUnitOfWork.UnitOfWorkException", frameworkSObjectUnitOfWorkMissingIDMessage(fieldName))
	}
	if !requireID && id.Kind != ValueNull && strings.EqualFold(fieldName, "m_newListByType") {
		return newExceptionError("framework_SObjectUnitOfWork.UnitOfWorkException", "Only new records can be registered as new")
	}
	objectName := vm.canonicalSObjectValueType(record)
	bucket, ok := receiver.Fields[fieldName]
	if !ok || bucket.Kind != ValueMap {
		return fmt.Errorf("framework_SObjectUnitOfWork missing %s", fieldName)
	}
	key := vm.resolveObjectBucketKey(bucket, objectName)
	value, ok := bucket.Map[key]
	if !ok {
		return frameworkSObjectUnitOfWorkException(fmt.Sprintf("SObject type %s is not supported by this unit of work", objectName))
	}
	if value.Kind == ValueNull {
		if strings.Contains(strings.ToLower(bucket.Type), "list") || strings.Contains(strings.ToLower(fieldName), "publish") || strings.Contains(strings.ToLower(fieldName), "newlist") {
			value = typedList("List<SObject>")
		} else {
			value = typedMap("Map<Id,SObject>")
		}
	}
	if value.Kind == ValueList {
		value.List = append(value.List, record)
	} else if value.Kind == ValueMap {
		recordKey := vm.mapKey(id)
		if id.Kind == ValueNull {
			recordKey = vm.mapKey(record)
		}
		if _, exists := value.Map[recordKey]; !exists {
			value.MapOrder = append(value.MapOrder, recordKey)
		}
		value.Map[recordKey] = record
		if value.MapKeys == nil {
			value.MapKeys = make(map[string]Value)
		}
		value.MapKeys[recordKey] = id
	} else {
		return fmt.Errorf("framework_SObjectUnitOfWork %s bucket must be list or map", fieldName)
	}
	bucket.Map[key] = value
	if bucket.MapKeys == nil {
		bucket.MapKeys = make(map[string]Value)
	}
	bucket.MapKeys[key] = String(objectName)
	if !containsString(bucket.MapOrder, key) {
		bucket.MapOrder = append(bucket.MapOrder, key)
	}
	receiver.Fields[fieldName] = bucket
	return nil
}
func frameworkSObjectUnitOfWorkMissingIDMessage(fieldName string) string {
	switch strings.ToLower(fieldName) {
	case "m_deletedmapbytype", "m_emptyrecyclebinmapbytype":
		return "New records cannot be registered for deletion"
	case "m_dirtymapbytype":
		return "New records cannot be registered as dirty"
	default:
		return "New records cannot be registered for this operation"
	}
}
func (vm *VM) addFrameworkSObjectUnitOfWorkRelationship(receiver Value, objectName string, record Value, relatedToField Value, relatedTo Value) error {
	fieldName, err := vm.sObjectFieldArg(record.Type, relatedToField)
	if err != nil {
		return err
	}
	bucket, ok := receiver.Fields["m_relationships"]
	if !ok || bucket.Kind != ValueMap {
		return fmt.Errorf("framework_SObjectUnitOfWork missing m_relationships")
	}
	relationshipBucket, ok := bucket.Map[mapKey(String(objectName))]
	if !ok || relationshipBucket.Kind != ValueObject {
		relationshipBucket = frameworkUnitOfWorkRelationships()
	}
	relationships, ok := relationshipBucket.Fields["m_relationships"]
	if !ok || relationships.Kind != ValueList {
		relationships = typedList("List<framework_SObjectUnitOfWork.IRelationship>")
	}
	relationship := Object("framework_SObjectUnitOfWork.Relationship")
	relationship.Fields["Record"] = record
	relationship.Fields["RelatedToField"] = sObjectFieldToken(vm.canonicalSObjectValueType(record), fieldName)
	relationship.Fields["RelatedTo"] = relatedTo
	relationships.List = append(relationships.List, relationship)
	relationshipBucket.Fields["m_relationships"] = relationships
	receiver.Fields["m_relationships"] = frameworkMapPut(bucket, String(objectName), relationshipBucket)
	return nil
}
func (vm *VM) constructFrameworkSObjectUnitOfWork(args []Value, namedArgs map[string]Value) (Value, error) {
	if len(namedArgs) != 0 || len(args) < 1 || len(args) > 3 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("framework_SObjectUnitOfWork constructor expects List<Schema.SObjectType>[, IDML][, UnresolvedRelationshipBehavior]")
	}
	uow := Object("framework_SObjectUnitOfWork")
	sObjectTypes := List(append([]Value(nil), args[0].List...)...)
	sObjectTypes.Type = args[0].Type
	uow.Fields["m_sObjectTypes"] = sObjectTypes
	uow.Fields["m_newListByType"] = typedMap("Map<String,List<SObject>>")
	uow.Fields["m_dirtyMapByType"] = typedMap("Map<String,Map<Id,SObject>>")
	uow.Fields["m_upsertRecordsPerType"] = typedMap("Map<String,List<SObject>>")
	uow.Fields["m_externalIdToUpsertPerType"] = typedMap("Map<String,Schema.SObjectField>")
	uow.Fields["m_deletedMapByType"] = typedMap("Map<String,Map<Id,SObject>>")
	uow.Fields["m_emptyRecycleBinMapByType"] = typedMap("Map<String,Map<Id,SObject>>")
	uow.Fields["m_relationships"] = typedMap("Map<String,framework_SObjectUnitOfWork.Relationships>")
	uow.Fields["m_publishBeforeListByType"] = typedMap("Map<String,List<SObject>>")
	uow.Fields["m_publishAfterSuccessListByType"] = typedMap("Map<String,List<SObject>>")
	uow.Fields["m_publishAfterFailureListByType"] = typedMap("Map<String,List<SObject>>")
	uow.Fields["m_workList"] = typedList("List<framework_SObjectUnitOfWork.IDoWork>")
	emailWork := Object("framework_SObjectUnitOfWork.SendEmailWork")
	emailWork.Fields["emails"] = typedList("List<Messaging.Email>")
	uow.Fields["m_emailWork"] = emailWork
	uow.Fields["m_dml"] = Object("framework_SObjectUnitOfWork.SimpleDML")
	uow.Fields["m_unresolvedRelationshipBehaviour"] = frameworkSObjectUnitOfWorkRelationshipBehaviorValue("IgnoreOutOfOrder")
	switch len(args) {
	case 2:
		if behavior, ok := frameworkSObjectUnitOfWorkRelationshipBehaviorName(args[1]); ok {
			uow.Fields["m_unresolvedRelationshipBehaviour"] = frameworkSObjectUnitOfWorkRelationshipBehaviorValue(behavior)
		} else {
			uow.Fields["m_dml"] = args[1]
		}
	case 3:
		uow.Fields["m_dml"] = args[1]
		behavior, ok := frameworkSObjectUnitOfWorkRelationshipBehaviorName(args[2])
		if !ok {
			return Null, fmt.Errorf("framework_SObjectUnitOfWork constructor expects UnresolvedRelationshipBehavior as third argument")
		}
		uow.Fields["m_unresolvedRelationshipBehaviour"] = frameworkSObjectUnitOfWorkRelationshipBehaviorValue(behavior)
	}
	for _, sObjectType := range args[0].List {
		if _, handled, err := vm.callFrameworkSObjectUnitOfWorkHandleRegisterType(uow, []Value{sObjectType}); err != nil || !handled {
			if err != nil {
				return Null, err
			}
			return Null, fmt.Errorf("framework_SObjectUnitOfWork constructor expects Schema.SObjectType entries")
		}
	}
	uow.Fields["m_relationships"] = frameworkMapPut(uow.Fields["m_relationships"], String("Messaging.SingleEmailMessage"), frameworkUnitOfWorkRelationships())
	return uow, nil
}
func frameworkSObjectUnitOfWorkRelationshipBehaviorName(value Value) (string, bool) {
	text := strings.TrimSpace(value.Text)
	if text == "" && value.Kind == ValueString {
		text = strings.TrimSpace(value.Text)
	}
	switch strings.ToLower(text) {
	case "attemptresolveoutoforder":
		return "AttemptResolveOutOfOrder", true
	case "ignoreoutoforder":
		return "IgnoreOutOfOrder", true
	default:
		return "", false
	}
}
func frameworkSObjectUnitOfWorkRelationshipBehaviorValue(name string) Value {
	return Value{Kind: ValueObject, Type: "framework_SObjectUnitOfWork.UnresolvedRelationshipBehavior", Text: name}
}
func frameworkSObjectUnitOfWorkAttemptsOutOfOrder(receiver Value) bool {
	if behavior, ok := receiver.Fields["m_unresolvedRelationshipBehaviour"]; ok {
		name, ok := frameworkSObjectUnitOfWorkRelationshipBehaviorName(behavior)
		return ok && strings.EqualFold(name, "AttemptResolveOutOfOrder")
	}
	return false
}
func (vm *VM) commitFrameworkSObjectUnitOfWork(receiver Value, result *Result) error {
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onCommitWorkStarting", nil, result); err != nil {
		return err
	}
	wasSuccessful := false
	defer func() {
		_ = vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onCommitWorkFinished", []Value{Bool(wasSuccessful)}, result)
	}()
	fail := func(cause error) error {
		if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishAfterFailureEventsStarting", nil, result); err != nil {
			return err
		}
		if err := vm.publishFrameworkSObjectUnitOfWorkEvents(receiver, "m_publishAfterFailureListByType", result); err != nil {
			return err
		}
		if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishAfterFailureEventsFinished", nil, result); err != nil {
			return err
		}
		return cause
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishBeforeEventsStarting", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.publishFrameworkSObjectUnitOfWorkEvents(receiver, "m_publishBeforeListByType", result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishBeforeEventsFinished", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onDMLStarting", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.applyFrameworkSObjectUnitOfWorkDML(receiver, "m_newListByType", "insert", result); err != nil {
		return fail(err)
	}
	if err := vm.applyFrameworkSObjectUnitOfWorkDML(receiver, "m_upsertRecordsPerType", "upsert", result); err != nil {
		return fail(err)
	}
	if frameworkSObjectUnitOfWorkAttemptsOutOfOrder(receiver) {
		if err := vm.updateFrameworkSObjectUnitOfWorkOutOfOrderRelationships(receiver, result); err != nil {
			return fail(err)
		}
	}
	if err := vm.applyFrameworkSObjectUnitOfWorkDML(receiver, "m_dirtyMapByType", "update", result); err != nil {
		return fail(err)
	}
	if err := vm.applyFrameworkSObjectUnitOfWorkDML(receiver, "m_deletedMapByType", "delete", result); err != nil {
		return fail(err)
	}
	if err := vm.applyFrameworkSObjectUnitOfWorkRecordAction(receiver, "m_emptyRecycleBinMapByType", result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onDMLFinished", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onDoWorkStarting", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.runFrameworkSObjectUnitOfWorkWorkItems(receiver, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onDoWorkFinished", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onCommitWorkFinishing", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishAfterSuccessEventsStarting", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.publishFrameworkSObjectUnitOfWorkEvents(receiver, "m_publishAfterSuccessListByType", result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishAfterSuccessEventsFinished", nil, result); err != nil {
		return fail(err)
	}
	wasSuccessful = true
	return nil
}
func (vm *VM) callFrameworkSObjectUnitOfWorkLifecycle(receiver Value, methodName string, args []Value, result *Result) error {
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(receiver.Type, methodName, args)
	if ambiguous {
		return vm.ambiguousOverloadError(receiver.Type+"."+methodName, args)
	}
	if !ok {
		return nil
	}
	_, err := vm.callMethodWithReceiver(method, receiver, args, result)
	return err
}
func (vm *VM) replayFrameworkSObjectUnitOfWorkTypeRegistrations(receiver Value) error {
	typesField, ok := receiver.Fields["m_newListByType"]
	if !ok || typesField.Kind != ValueMap {
		return nil
	}
	onRegisterArgs := []Value{sObjectTypeToken("")}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(receiver.Type, "onRegisterType", onRegisterArgs)
	if ambiguous || !ok || vm.isSObjectUnitOfWorkBaseType(method.ClassName) {
		return nil
	}
	keys := typesField.MapOrder
	if len(keys) == 0 {
		for key := range typesField.Map {
			keys = append(keys, key)
		}
		sort.Strings(keys)
	}
	result := &Result{}
	for _, key := range keys {
		name, exists := typesField.MapKeys[key]
		if !exists || name.Kind != ValueString {
			continue
		}
		if _, err := vm.callMethodWithReceiver(method, receiver, []Value{sObjectTypeToken(name.Text)}, result); err != nil {
			return err
		}
	}
	return nil
}
func (vm *VM) applyFrameworkSObjectUnitOfWorkDML(receiver Value, fieldName, op string, result *Result) error {
	buckets := frameworkSObjectUnitOfWorkRecordBuckets(receiver, fieldName)
	if op == "delete" {
		reverseFrameworkSObjectUnitOfWorkRecordBuckets(buckets)
	}
	for _, bucket := range buckets {
		records := bucket.records
		if len(records.List) == 0 {
			continue
		}
		if op == "insert" || op == "update" || op == "upsert" {
			if err := vm.resolveFrameworkSObjectUnitOfWorkRelationships(receiver, bucket.objectName, result); err != nil {
				return err
			}
		}
		if op == "upsert" {
			externalIDField := vm.frameworkSObjectUnitOfWorkExternalIDField(receiver, bucket.objectName)
			if externalIDField.Kind == ValueNull {
				return fmt.Errorf("framework_SObjectUnitOfWork upsert missing external id field for %s", bucket.objectName)
			}
			if handled, err := vm.callFrameworkSObjectUnitOfWorkCustomUpsert(receiver, records, externalIDField, result); err != nil {
				return err
			} else if handled {
				continue
			}
			if _, err := vm.executeDatabaseDML("upsert", []Value{records, externalIDField}, result); err != nil {
				return err
			}
			continue
		}
		if handled, err := vm.callFrameworkSObjectUnitOfWorkCustomDML(receiver, op, records, result); err != nil {
			return err
		} else if handled {
			// Custom IDML implementations own persistence behavior for this operation.
			continue
		}
		if _, err := vm.executeDatabaseDML(op, []Value{records}, result); err != nil {
			return err
		}
	}
	return nil
}
func (vm *VM) frameworkSObjectUnitOfWorkExternalIDField(receiver Value, objectName string) Value {
	bucket, ok := receiver.Fields["m_externalIdToUpsertPerType"]
	if !ok || bucket.Kind != ValueMap {
		return Null
	}
	value, ok := bucket.Map[vm.resolveObjectBucketKey(bucket, objectName)]
	if !ok {
		return Null
	}
	return value
}
func reverseFrameworkSObjectUnitOfWorkRecordBuckets(buckets []frameworkSObjectUnitOfWorkRecordBucket) {
	for i, j := 0, len(buckets)-1; i < j; i, j = i+1, j-1 {
		buckets[i], buckets[j] = buckets[j], buckets[i]
	}
}
func frameworkCustomDMLHandlesPersistence(receiver Value) bool {
	dmlValue, ok := receiver.Fields["m_dml"]
	if !ok || dmlValue.Kind != ValueObject {
		return false
	}
	_, hasOptions := dmlValue.Fields["dmlOptions"]
	_, hasParsedErrors := dmlValue.Fields["parsedErrors"]
	return hasOptions && hasParsedErrors
}
func frameworkSObjectUnitOfWorkRecordsHaveIDs(records Value) bool {
	if records.Kind != ValueList || len(records.List) == 0 {
		return false
	}
	for _, record := range records.List {
		if sObjectIDValue(record).Kind == ValueNull {
			return false
		}
	}
	return true
}
func frameworkSObjectUnitOfWorkRecordBuckets(receiver Value, fieldName string) []frameworkSObjectUnitOfWorkRecordBucket {
	field, ok := receiver.Fields[fieldName]
	if !ok || field.Kind != ValueMap {
		return nil
	}
	out := make([]frameworkSObjectUnitOfWorkRecordBucket, 0, len(field.MapOrder)+len(field.Map))
	seen := make(map[string]bool, len(field.Map))
	appendBucket := func(key string, value Value) {
		list := frameworkSObjectUnitOfWorkRecordList(value)
		if list.Kind != ValueList {
			return
		}
		objectName := ""
		if field.MapKeys != nil {
			if keyValue, ok := field.MapKeys[key]; ok && keyValue.Kind == ValueString {
				objectName = keyValue.Text
			}
		}
		if objectName == "" {
			objectName = strings.TrimPrefix(key, "s:")
		}
		out = append(out, frameworkSObjectUnitOfWorkRecordBucket{objectName: objectName, records: list})
	}
	for _, key := range field.MapOrder {
		if value, ok := field.Map[key]; ok {
			appendBucket(key, value)
			seen[key] = true
		}
	}
	keys := make([]string, 0, len(field.Map))
	for key := range field.Map {
		if seen[key] {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := field.Map[key]
		appendBucket(key, value)
	}
	return out
}
func (vm *VM) applyFrameworkSObjectUnitOfWorkRecordAction(receiver Value, fieldName string, result *Result) error {
	for _, records := range frameworkSObjectUnitOfWorkRecordLists(receiver, fieldName) {
		if len(records.List) == 0 {
			continue
		}
		if handled, err := vm.callFrameworkSObjectUnitOfWorkCustomDML(receiver, "emptyRecycleBin", records, result); handled || err != nil {
			return err
		}
		if _, err := vm.executeDatabaseRecordAction("emptyRecycleBin", []Value{records}, result, "Database.EmptyRecycleBinResult"); err != nil {
			return err
		}
	}
	return nil
}
func (vm *VM) publishFrameworkSObjectUnitOfWorkEvents(receiver Value, fieldName string, result *Result) error {
	for _, records := range frameworkSObjectUnitOfWorkRecordLists(receiver, fieldName) {
		if len(records.List) == 0 {
			continue
		}
		if handled, err := vm.callFrameworkSObjectUnitOfWorkCustomDML(receiver, "eventPublish", records, result); handled || err != nil {
			return err
		}
		if _, err := vm.eventBusPublish([]Value{records}, result); err != nil {
			return err
		}
	}
	return nil
}
func (vm *VM) callFrameworkSObjectUnitOfWorkCustomDML(receiver Value, op string, records Value, result *Result) (bool, error) {
	dmlValue, ok := receiver.Fields["m_dml"]
	if !ok || dmlValue.Kind != ValueObject || strings.EqualFold(dmlValue.Type, "framework_SObjectUnitOfWork.SimpleDML") {
		return false, nil
	}
	methodName := ""
	switch op {
	case "insert":
		methodName = "dmlInsert"
	case "update":
		methodName = "dmlUpdate"
	case "delete":
		methodName = "dmlDelete"
	case "eventPublish", "emptyRecycleBin":
		methodName = op
	default:
		return false, nil
	}
	methodArgs := []Value{records}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(dmlValue.Type, methodName, methodArgs)
	if !ok && !ambiguous {
		if widened, err := coerceCollectionValue("List<SObject>", records); err == nil {
			methodArgs = []Value{widened}
			method, ok, ambiguous = vm.resolveInstanceMethodForArgs(dmlValue.Type, methodName, methodArgs)
		}
	}
	if ambiguous {
		return true, vm.ambiguousOverloadError(dmlValue.Type+"."+methodName, methodArgs)
	}
	if !ok {
		return false, nil
	}
	_, err := vm.callMethodWithReceiver(method, dmlValue, methodArgs, result)
	return true, err
}
func (vm *VM) callFrameworkSObjectUnitOfWorkCustomUpsert(receiver Value, records Value, externalIDField Value, result *Result) (bool, error) {
	dmlValue, ok := receiver.Fields["m_dml"]
	if !ok || dmlValue.Kind != ValueObject || strings.EqualFold(dmlValue.Type, "framework_SObjectUnitOfWork.SimpleDML") {
		return false, nil
	}
	methodArgs := []Value{records, externalIDField}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(dmlValue.Type, "dmlUpsert", methodArgs)
	if !ok && !ambiguous {
		if widened, err := coerceCollectionValue("List<SObject>", records); err == nil {
			methodArgs = []Value{widened, externalIDField}
			method, ok, ambiguous = vm.resolveInstanceMethodForArgs(dmlValue.Type, "dmlUpsert", methodArgs)
		}
	}
	if ambiguous {
		return true, vm.ambiguousOverloadError(dmlValue.Type+".dmlUpsert", methodArgs)
	}
	if !ok {
		return false, nil
	}
	_, err := vm.callMethodWithReceiver(method, dmlValue, methodArgs, result)
	return true, err
}
func (vm *VM) resolveFrameworkSObjectUnitOfWorkRelationships(receiver Value, objectName string, result *Result) error {
	bucket, ok := receiver.Fields["m_relationships"]
	if !ok || bucket.Kind != ValueMap {
		return nil
	}
	relationshipsValue, ok := bucket.Map[vm.resolveObjectBucketKey(bucket, objectName)]
	if !ok || relationshipsValue.Kind != ValueObject {
		return nil
	}
	_, relationships, ok := objectFieldValue(relationshipsValue, "m_relationships")
	if !ok || relationships.Kind != ValueList {
		return nil
	}
	for _, relationship := range relationships.List {
		if relationship.Kind != ValueObject {
			continue
		}
		if strings.EqualFold(relationship.Type, "framework_SObjectUnitOfWork.Relationship") {
			if err := vm.resolveFrameworkSObjectUnitOfWorkRelationship(receiver, relationship); err != nil {
				return err
			}
			continue
		}
		method, ok, ambiguous := vm.resolveInstanceMethodForArgs(relationship.Type, "resolve", nil)
		if ambiguous {
			return vm.ambiguousOverloadError(relationship.Type+".resolve", nil)
		}
		if !ok {
			continue
		}
		if _, err := vm.callMethodWithReceiver(method, relationship, nil, result); err != nil {
			return err
		}
	}
	return nil
}
func (vm *VM) resolveFrameworkSObjectUnitOfWorkRelationship(receiver Value, relationship Value) error {
	_, record, hasRecord := objectFieldValue(relationship, "Record")
	_, relatedToField, hasField := objectFieldValue(relationship, "RelatedToField")
	_, relatedTo, hasRelated := objectFieldValue(relationship, "RelatedTo")
	if !hasRecord || !hasField || !hasRelated || record.Kind != ValueObject || relatedTo.Kind != ValueObject {
		return nil
	}
	relatedID := sObjectIDValue(relatedTo)
	if relatedID.Kind == ValueNull {
		return nil
	}
	fieldName, err := vm.sObjectFieldArg(record.Type, relatedToField)
	if err != nil {
		return err
	}
	previous := snapshotAlias(record)
	setExplicitSObjectField(&record, fieldName, relatedID)
	relationship.Fields["Record"] = record
	replaced, changed := replaceAliasSnapshot(receiver, previous, record, make(map[uint64]bool))
	if changed && replaced.Kind == ValueObject {
		receiver.Fields = replaced.Fields
	}
	vm.propagateAliasSnapshotToScope(vm.Globals, previous, record)
	vm.propagateAliasSnapshotToStatics(previous, record)
	return nil
}
func (vm *VM) updateFrameworkSObjectUnitOfWorkOutOfOrderRelationships(receiver Value, result *Result) error {
	bucket, ok := receiver.Fields["m_relationships"]
	if !ok || bucket.Kind != ValueMap {
		return nil
	}
	updates := typedList("List<SObject>")
	seen := make(map[string]int)
	bucketKeys := append([]string(nil), bucket.MapOrder...)
	if len(bucketKeys) == 0 {
		for key := range bucket.Map {
			bucketKeys = append(bucketKeys, key)
		}
		sort.Strings(bucketKeys)
	}
	for _, bucketKey := range bucketKeys {
		relationshipsValue, ok := bucket.Map[bucketKey]
		if !ok || relationshipsValue.Kind != ValueObject {
			continue
		}
		_, relationships, ok := objectFieldValue(relationshipsValue, "m_relationships")
		if !ok || relationships.Kind != ValueList {
			continue
		}
		for _, relationship := range relationships.List {
			updateRecord, ok, err := vm.frameworkSObjectUnitOfWorkOutOfOrderUpdate(relationship)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			updateKey := vm.mapKey(sObjectIDValue(updateRecord))
			if existingIndex, exists := seen[updateKey]; exists {
				updates.List[existingIndex] = mergeSparseSObjectUpdate(updates.List[existingIndex], updateRecord)
				continue
			}
			seen[updateKey] = len(updates.List)
			updates.List = append(updates.List, updateRecord)
		}
	}
	if len(updates.List) == 0 {
		return nil
	}
	if handled, err := vm.callFrameworkSObjectUnitOfWorkCustomDML(receiver, "update", updates, result); handled || err != nil {
		return err
	}
	_, err := vm.executeDatabaseDML("update", []Value{updates}, result)
	return err
}
func (vm *VM) frameworkSObjectUnitOfWorkOutOfOrderUpdate(relationship Value) (Value, bool, error) {
	if relationship.Kind != ValueObject || !strings.EqualFold(relationship.Type, "framework_SObjectUnitOfWork.Relationship") {
		return Null, false, nil
	}
	_, record, hasRecord := objectFieldValue(relationship, "Record")
	_, relatedToField, hasField := objectFieldValue(relationship, "RelatedToField")
	_, relatedTo, hasRelated := objectFieldValue(relationship, "RelatedTo")
	if !hasRecord || !hasField || !hasRelated || record.Kind != ValueObject || relatedTo.Kind != ValueObject {
		return Null, false, nil
	}
	recordID := sObjectIDValue(record)
	relatedID := sObjectIDValue(relatedTo)
	if recordID.Kind == ValueNull || relatedID.Kind == ValueNull {
		return Null, false, nil
	}
	fieldName, err := vm.sObjectFieldArg(record.Type, relatedToField)
	if err != nil {
		return Null, false, err
	}
	if persisted, hasField, hasRecord := vm.persistedSObjectFieldValue(record, fieldName); hasRecord {
		if hasField && persisted.Equal(relatedID) {
			return Null, false, nil
		}
	} else {
		current, _, err := vm.callSObjectMember(record, "get", []Value{relatedToField})
		if err != nil {
			return Null, false, err
		}
		if current.Equal(relatedID) {
			return Null, false, nil
		}
	}
	updateRecord := Object(record.Type)
	updateRecord.Fields["Id"] = recordID
	setExplicitSObjectField(&updateRecord, fieldName, relatedID)
	return updateRecord, true, nil
}
func (vm *VM) persistedSObjectFieldValue(record Value, fieldName string) (Value, bool, bool) {
	if vm == nil || vm.Org == nil || record.Kind != ValueObject || strings.TrimSpace(fieldName) == "" {
		return Null, false, false
	}
	idText, ok := comparableIDText(sObjectIDValue(record))
	if !ok || idText == "" {
		return Null, false, false
	}
	objectName := record.Type
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false, false
	}
	_, stored, ok := storage.LookupRecordByID(object.Records, storage.ID(idText))
	if !ok {
		return Null, false, false
	}
	if value, ok := stored.GetField(fieldName); ok {
		return vmValueFromStorage(value), true, true
	}
	if stored.HasExplicitNull(fieldName) {
		return Null, true, true
	}
	return Null, false, true
}
func frameworkSObjectUnitOfWorkRecordLists(receiver Value, fieldName string) []Value {
	field, ok := receiver.Fields[fieldName]
	if !ok || field.Kind != ValueMap {
		return nil
	}
	out := make([]Value, 0, len(field.MapOrder)+len(field.Map))
	seen := make(map[string]bool, len(field.Map))
	for _, key := range field.MapOrder {
		if value, ok := field.Map[key]; ok {
			if list := frameworkSObjectUnitOfWorkRecordList(value); list.Kind == ValueList {
				out = append(out, list)
			}
			seen[key] = true
		}
	}
	keys := make([]string, 0, len(field.Map))
	for key := range field.Map {
		if seen[key] {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := field.Map[key]
		if list := frameworkSObjectUnitOfWorkRecordList(value); list.Kind == ValueList {
			out = append(out, list)
		}
	}
	return out
}
func frameworkSObjectUnitOfWorkRecordList(value Value) Value {
	switch value.Kind {
	case ValueList:
		return value
	case ValueMap:
		list := typedList("List<SObject>")
		if len(value.MapOrder) != 0 {
			seen := make(map[string]bool, len(value.Map))
			for _, key := range value.MapOrder {
				if item, ok := value.Map[key]; ok {
					list.List = append(list.List, item)
					seen[key] = true
				}
			}
			for key, item := range value.Map {
				if !seen[key] {
					list.List = append(list.List, item)
				}
			}
			return list
		}
		for _, item := range value.Map {
			list.List = append(list.List, item)
		}
		return list
	default:
		return Null
	}
}
func (vm *VM) runFrameworkSObjectUnitOfWorkWorkItems(receiver Value, result *Result) error {
	workList, ok := receiver.Fields["m_workList"]
	if !ok || workList.Kind != ValueList {
		workList = typedList("List<framework_SObjectUnitOfWork.IDoWork>")
	}
	if emailWork, ok := receiver.Fields["m_emailWork"]; ok && emailWork.Kind == ValueObject {
		workList.List = append(workList.List, emailWork)
		receiver.Fields["m_workList"] = workList
	}
	for _, work := range workList.List {
		if work.Kind != ValueObject {
			continue
		}
		if _, handled, err := vm.callFrameworkSObjectUnitOfWorkMember(work, "doWork", nil, result); handled || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		method, ok, ambiguous := vm.resolveInstanceMethodForArgs(work.Type, "doWork", nil)
		if ambiguous {
			return vm.ambiguousOverloadError(work.Type+".doWork", nil)
		}
		if !ok {
			continue
		}
		if _, err := vm.callMethodWithReceiver(method, work, nil, result); err != nil {
			return err
		}
	}
	return nil
}
func (vm *VM) callFrameworkQueryFactoryMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if !strings.EqualFold(receiver.Type, "framework_QueryFactory.Ordering") {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "getfield":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_QueryFactory.Ordering.getField expects 0 arguments")
		}
		_, field, _ := objectFieldValue(receiver, "field")
		return field, true, nil
	case "getdirection":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_QueryFactory.Ordering.getDirection expects 0 arguments")
		}
		_, direction, _ := objectFieldValue(receiver, "direction")
		return direction, true, nil
	case "tosoql":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_QueryFactory.Ordering.toSOQL expects 0 arguments")
		}
		field := ""
		if _, value, ok := objectFieldValue(receiver, "field"); ok {
			field = scalarText(value)
		}
		direction := "DESC"
		if _, value, ok := objectFieldValue(receiver, "direction"); ok && frameworkQueryFactorySortAscending(value) {
			direction = "ASC"
		}
		nulls := " NULLS FIRST "
		if _, value, ok := objectFieldValue(receiver, "nullsLast"); ok && value.Kind == ValueBool && value.Bool {
			nulls = " NULLS LAST "
		}
		return String(field + " " + direction + nulls), true, nil
	default:
		return Null, false, nil
	}
}
func frameworkQueryFactorySortAscending(value Value) bool {
	text := value.Text
	if text == "" {
		text = value.String()
	}
	return strings.EqualFold(text, "ASCENDING") || strings.EqualFold(text, "framework_QueryFactory.SortOrder.ASCENDING")
}
func frameworkUnitOfWorkRelationships() Value {
	relationships := Object("framework_SObjectUnitOfWork.Relationships")
	relationships.Fields["m_relationships"] = typedList("List<framework_SObjectUnitOfWork.IRelationship>")
	return relationships
}
func frameworkSObjectUnitOfWorkRelationshipsType(typeName string) bool {
	return strings.EqualFold(typeName, "framework_SObjectUnitOfWork.Relationships") ||
		strings.EqualFold(frameworkMockSupportType(typeName), "SObjectUnitOfWork.Relationships")
}
func (vm *VM) constructFrameworkFastDTO(typeName string, args []Value, namedArgs map[string]Value) (Value, bool, error) {
	if len(namedArgs) != 0 {
		return Null, false, nil
	}
	switch strings.ToLower(typeName) {
	case "framework_methodargvalues", "fflib_methodargvalues":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, false, nil
		}
		value := Object(typeName)
		value.Fields["argValues"] = args[0]
		return value, true, nil
	case "framework_invocationonmock", "fflib_invocationonmock":
		if len(args) != 3 {
			return Null, false, nil
		}
		value := Object(typeName)
		value.Fields["qm"] = args[0]
		value.Fields["methodArg"] = args[1]
		value.Fields["mockInstance"] = args[2]
		return value, true, nil
	case "framework_qualifiedmethod", "fflib_qualifiedmethod":
		if len(args) != 3 && len(args) != 4 {
			return Null, false, nil
		}
		value := Object(typeName)
		value.Fields["typeName"] = args[0]
		value.Fields["methodName"] = args[1]
		value.Fields["methodArgTypes"] = args[2]
		if len(args) == 4 {
			value.Fields["mockInstance"] = args[3]
		} else {
			value.Fields["mockInstance"] = Null
		}
		return value, true, nil
	default:
		return Null, false, nil
	}
}
