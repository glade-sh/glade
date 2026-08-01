package vm

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) testSetMock(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.setMock expects mock type and mock instance")
	}
	if err := vm.requireTestContext("Test.setMock"); err != nil {
		return Null, err
	}
	mockType, ok := testMockTypeName(args[0])
	if !ok {
		return Null, fmt.Errorf("Test.setMock expects mock type")
	}
	if mockType == "WebServiceMock" {
		vm.testContext.WebServiceMock = args[1]
		return Null, nil
	}
	if mockType != "HttpCalloutMock" {
		return Null, unsupportedCallError("Test.setMock " + mockType + " mock surface")
	}
	vm.testContext.HTTPMock = args[1]
	return Null, nil
}
func (vm *VM) testSetContinuationResponse(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "HttpResponse") {
		return Null, fmt.Errorf("Test.setContinuationResponse expects label String and HttpResponse")
	}
	if err := vm.requireTestContext("Test.setContinuationResponse"); err != nil {
		return Null, err
	}
	if vm.testContext.ContinuationResponses == nil {
		vm.testContext.ContinuationResponses = make(map[string]Value)
	}
	vm.testContext.ContinuationResponses[args[0].Text] = args[1]
	return Null, nil
}
func (vm *VM) continuationGetResponse(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("Continuation.getResponse expects label String")
	}
	if vm.testContext == nil || vm.testContext.ContinuationResponses == nil {
		return Null, unsupportedCallError("Continuation.getResponse local continuation callout surface")
	}
	if response, ok := vm.testContext.ContinuationResponses[args[0].Text]; ok {
		return response, nil
	}
	return Null, unsupportedCallError("Continuation.getResponse local continuation callout surface")
}
func (vm *VM) testInvokeContinuationMethod(args []Value, result *Result) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "Continuation") {
		return Null, fmt.Errorf("Test.invokeContinuationMethod expects controller Object and Continuation")
	}
	if err := vm.requireTestContext("Test.invokeContinuationMethod"); err != nil {
		return Null, err
	}
	methodValue, ok := args[1].Fields["ContinuationMethod"]
	if !ok || methodValue.Kind == ValueNull {
		methodValue, ok = args[1].Fields["continuationMethod"]
	}
	if !ok || methodValue.Kind != ValueString || methodValue.Text == "" {
		return Null, fmt.Errorf("Continuation.ContinuationMethod must be set before Test.invokeContinuationMethod")
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(args[0].Type, methodValue.Text, nil)
	if ambiguous {
		return Null, vm.ambiguousOverloadError(args[0].Type+"."+methodValue.Text, nil)
	}
	if !ok {
		return Null, unsupportedCallError(args[0].Type + "." + methodValue.Text)
	}
	return vm.callMethodWithReceiver(target, args[0], nil, result)
}
func (vm *VM) testNotificationActionHandler(args []Value, result *Result) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.testNotificationActionHandler expects handler and actionable notification")
	}
	if err := vm.requireTestContext("Test.testNotificationActionHandler"); err != nil {
		return Null, err
	}
	if args[0].Kind != ValueObject || !vm.typeMatches(args[0].Type, "Messaging.NotificationActionHandler", make(map[string]bool)) {
		return Null, fmt.Errorf("Test.testNotificationActionHandler expects Messaging.NotificationActionHandler")
	}
	if args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "Messaging.ActionableNotification") {
		return Null, fmt.Errorf("Test.testNotificationActionHandler expects Messaging.ActionableNotification")
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(args[0].Type, "executeAction", []Value{args[1]})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(args[0].Type+".executeAction", []Value{args[1]})
	}
	if !ok {
		return Null, fmt.Errorf("Messaging.NotificationActionHandler %s must implement executeAction", args[0].Type)
	}
	value, err := vm.callMethodWithReceiver(target, args[0], []Value{args[1]}, result)
	if err != nil {
		return Null, err
	}
	if value.Kind == ValueNull || (value.Kind == ValueObject && strings.EqualFold(value.Type, "Messaging.ActionResult")) {
		return value, nil
	}
	return Null, fmt.Errorf("Messaging.NotificationActionHandler %s executeAction must return Messaging.ActionResult", args[0].Type)
}
func (vm *VM) testSandboxPostCopyScript(args []Value, result *Result) (Value, error) {
	if len(args) != 4 && len(args) != 5 {
		return Null, fmt.Errorf("Test.testSandboxPostCopyScript expects script, organizationId, sandboxId, sandboxName[, isRunAsAutoProcUser]")
	}
	if err := vm.requireTestContext("Test.testSandboxPostCopyScript"); err != nil {
		return Null, err
	}
	if args[0].Kind != ValueObject || !vm.typeMatches(args[0].Type, "SandboxPostCopy", make(map[string]bool)) {
		return Null, fmt.Errorf("Test.testSandboxPostCopyScript expects SandboxPostCopy")
	}
	if !isApexIDLikeValue(args[1]) || !isApexIDLikeValue(args[2]) || args[3].Kind != ValueString {
		return Null, fmt.Errorf("Test.testSandboxPostCopyScript expects organization Id, sandbox Id, and sandbox name")
	}
	runAsAutoProcUser := Bool(false)
	if len(args) == 5 {
		if args[4].Kind != ValueBool {
			return Null, fmt.Errorf("Test.testSandboxPostCopyScript isRunAsAutoProcUser expects Boolean")
		}
		runAsAutoProcUser = args[4]
	}
	context := Object("SandboxContext")
	context.Fields["organizationId"] = platformScalar("Id", scalarText(args[1]))
	context.Fields["sandboxId"] = platformScalar("Id", scalarText(args[2]))
	context.Fields["sandboxName"] = args[3]
	context.Fields["isRunAsAutoProcUser"] = runAsAutoProcUser
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(args[0].Type, "runApexClass", []Value{context})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(args[0].Type+".runApexClass", []Value{context})
	}
	if !ok {
		return Null, fmt.Errorf("SandboxPostCopy %s must implement runApexClass", args[0].Type)
	}
	_, err := vm.callMethodWithReceiver(target, args[0], []Value{context}, result)
	return Null, err
}
func (vm *VM) canvasTestMockRenderContext(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueMap || args[1].Kind != ValueMap {
		return Null, fmt.Errorf("Canvas.Test.mockRenderContext expects app and environment Map<String,String> values")
	}
	if err := vm.requireTestContext("Canvas.Test.mockRenderContext"); err != nil {
		return Null, err
	}
	app := Object("Canvas.ApplicationContext")
	bindCanvasContextMap(&app, args[0])
	env := Object("Canvas.EnvironmentContext")
	bindCanvasContextMap(&env, args[1])
	env.Fields["entityFields"] = typedList("List<String>")
	ctx := Object("Canvas.RenderContext")
	ctx.Fields["applicationContext"] = app
	ctx.Fields["environmentContext"] = env
	return ctx, nil
}
func (vm *VM) canvasTestCanvasLifecycle(args []Value, result *Result) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "Canvas.RenderContext") {
		return Null, fmt.Errorf("Canvas.Test.testCanvasLifecycle expects CanvasLifecycleHandler and RenderContext")
	}
	if err := vm.requireTestContext("Canvas.Test.testCanvasLifecycle"); err != nil {
		return Null, err
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(args[0].Type, "onRender", []Value{args[1]})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(args[0].Type+".onRender", []Value{args[1]})
	}
	if !ok {
		return Null, nil
	}
	_, err := vm.callMethodWithReceiver(target, args[0], []Value{args[1]}, result)
	return Null, err
}
func bindCanvasContextMap(target *Value, source Value) {
	if target == nil || source.Kind != ValueMap {
		return
	}
	for encoded, value := range source.Map {
		keyValue := mapStoredKey(source, encoded)
		if keyValue.Kind != ValueString {
			continue
		}
		target.Fields[canvasContextFieldName(keyValue.Text)] = value
	}
}
func canvasContextFieldName(key string) string {
	switch strings.ToLower(strings.ReplaceAll(key, "_", "")) {
	case "canvasurl", "keycanvasurl":
		return "canvasUrl"
	case "developername", "keydevelopername":
		return "developerName"
	case "displaylocation", "keydisplaylocation":
		return "displayLocation"
	case "locationurl", "keylocationurl":
		return "locationUrl"
	case "name", "keyname":
		return "name"
	case "namespace", "keynamespace":
		return "namespace"
	case "sublocation", "keysublocation":
		return "sublocation"
	case "version", "keyversion":
		return "version"
	default:
		return key
	}
}
func (vm *VM) testCreateStub(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Test.createStub expects Type and StubProvider")
	}
	if err := vm.requireTestContext("Test.createStub"); err != nil {
		return Null, err
	}
	stubbedType, ok := testMockTypeName(args[0])
	if !ok || stubbedType == "" {
		return Null, fmt.Errorf("Test.createStub expects Type")
	}
	if args[1].Kind != ValueObject || !vm.typeMatches(args[1].Type, "StubProvider", make(map[string]bool)) {
		return Null, fmt.Errorf("Test.createStub expects StubProvider")
	}
	if resolved, ok := vm.resolveClassName(stubbedType); ok {
		stubbedType = resolved
		if class, classOK := vm.lookupClass(resolved); classOK {
			stubbedType = vm.classTypeToken(class)
		}
	} else {
		return Null, unsupportedCallError("Test.createStub local proxy for unknown type " + stubbedType)
	}
	proxy := Object(stubbedType)
	proxy.Fields["__gladeStubProvider"] = args[1]
	proxy.Fields["__gladeStubbedType"] = String(stubbedType)
	if _, ok := vm.lookupClass(stubbedType); ok {
		// Test.createStub should return a proxy without executing user constructors
		// or instance initializers of the stubbed type.
		vm.initializeFields(&proxy, stubbedType)
	}
	return proxy, nil
}
func (vm *VM) testCreateSoqlStub(args []Value) (Value, error) {
	if len(args) != 2 || !isSObjectTypeToken(args[0]) {
		return Null, fmt.Errorf("Test.createSoqlStub expects Schema.SObjectType and SoqlStubProvider")
	}
	if err := vm.requireTestContext("Test.createSoqlStub"); err != nil {
		return Null, err
	}
	if args[1].Kind != ValueObject || !vm.typeMatches(args[1].Type, "SoqlStubProvider", make(map[string]bool)) {
		return Null, fmt.Errorf("Test.createSoqlStub expects SoqlStubProvider")
	}
	objectName, err := vm.schemaDescribeObjectName(args[0])
	if err != nil {
		return Null, err
	}
	if vm.testContext.SoqlStubs == nil {
		vm.testContext.SoqlStubs = make(map[string]Value)
	}
	vm.testContext.SoqlStubs[strings.ToLower(objectName)] = args[1]
	return Null, nil
}
func (vm *VM) testCreateStubQueryRow(args []Value) (Value, error) {
	if len(args) != 2 || !isSObjectTypeToken(args[0]) || args[1].Kind != ValueMap {
		return Null, fmt.Errorf("Test.createStubQueryRow expects Schema.SObjectType and Map<String,Object>")
	}
	if err := vm.requireTestContext("Test.createStubQueryRow"); err != nil {
		return Null, err
	}
	objectName, err := vm.schemaDescribeObjectName(args[0])
	if err != nil {
		return Null, err
	}
	return vm.stubQueryRowFromMap(objectName, args[1])
}
func (vm *VM) testCreateStubQueryRows(args []Value) (Value, error) {
	if len(args) != 2 || !isSObjectTypeToken(args[0]) || args[1].Kind != ValueList {
		return Null, fmt.Errorf("Test.createStubQueryRows expects Schema.SObjectType and List<Map<String,Object>>")
	}
	if err := vm.requireTestContext("Test.createStubQueryRows"); err != nil {
		return Null, err
	}
	objectName, err := vm.schemaDescribeObjectName(args[0])
	if err != nil {
		return Null, err
	}
	rows := typedList("List<" + objectName + ">")
	for _, item := range args[1].List {
		if item.Kind != ValueMap {
			return Null, fmt.Errorf("Test.createStubQueryRows expects List<Map<String,Object>>")
		}
		row, err := vm.stubQueryRowFromMap(objectName, item)
		if err != nil {
			return Null, err
		}
		rows.List = append(rows.List, row)
	}
	return rows, nil
}
func (vm *VM) stubQueryRowFromMap(objectName string, fields Value) (Value, error) {
	row := Object(objectName)
	for rawKey, fieldValue := range fields.Map {
		key := mapStoredKey(fields, rawKey)
		if key.Kind != ValueString || strings.TrimSpace(key.Text) == "" {
			return Null, fmt.Errorf("Test.createStubQueryRow field names must be strings")
		}
		vm.setExplicitSObjectFieldValue(&row, key.Text, fieldValue)
		markQueriedSObjectField(&row, key.Text)
	}
	return row, nil
}
func (vm *VM) testLoadData(args []Value, result *Result) (Value, error) {
	if len(args) != 2 || !isSObjectTypeToken(args[0]) || args[1].Kind != ValueString {
		return Null, fmt.Errorf("Test.loadData expects Schema.SObjectType and static resource name")
	}
	if err := vm.requireTestContext("Test.loadData"); err != nil {
		return Null, err
	}
	objectName, err := vm.schemaDescribeObjectName(args[0])
	if err != nil {
		return Null, err
	}
	content, ok, err := vm.testLoadDataStaticResourceContent(args[1].Text)
	if err != nil {
		return Null, err
	}
	if !ok {
		return Null, newExceptionError("StringException", fmt.Sprintf("Test.loadData static resource %s not found", args[1].Text))
	}
	reader := csv.NewReader(strings.NewReader(content))
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return Null, fmt.Errorf("Test.loadData CSV parse failed: %w", err)
	}
	out := typedList("List<" + objectName + ">")
	if len(rows) == 0 {
		return out, nil
	}
	headers := rows[0]
	if err := vm.validateTestLoadDataHeaders(objectName, headers); err != nil {
		return Null, err
	}
	for rowIndex, csvRow := range rows[1:] {
		record := Object(objectName)
		for i, header := range headers {
			fieldName := strings.TrimSpace(header)
			if fieldName == "" {
				continue
			}
			raw := ""
			if i < len(csvRow) {
				raw = csvRow[i]
			}
			if lookupField, value, handled, err := vm.testLoadDataRelationshipValue(objectName, fieldName, raw); handled || err != nil {
				if err != nil {
					return Null, err
				}
				vm.setExplicitSObjectFieldValue(&record, lookupField, value)
				continue
			}
			value, err := vm.testLoadDataFieldValue(objectName, fieldName, raw)
			if err != nil {
				return Null, fmt.Errorf("Test.loadData row %d %s.%s: %w", rowIndex+2, objectName, fieldName, err)
			}
			vm.setExplicitSObjectFieldValue(&record, fieldName, value)
		}
		out.List = append(out.List, record)
	}
	if len(out.List) == 0 {
		return out, nil
	}
	if _, err := vm.applyDML("insert", out, true, "", dml.Options{}, result); err != nil {
		return Null, err
	}
	return out, nil
}
func (vm *VM) validateTestLoadDataHeaders(objectName string, headers []string) error {
	if vm == nil || vm.Org == nil {
		return nil
	}
	canonicalObject := objectName
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		canonicalObject = resolved
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return nil
	}
	for _, header := range headers {
		fieldName := strings.TrimSpace(header)
		if fieldName == "" {
			return newExceptionError("StringException", fmt.Sprintf("Test.loadData %s CSV header contains a blank field name", objectName))
		}
		if vm.testLoadDataRelationshipHeader(object.Definition, fieldName) {
			continue
		}
		if _, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, fieldName); !ok {
			return newExceptionError("StringException", fmt.Sprintf("Test.loadData %s CSV header contains Unknown field %s", objectName, fieldName))
		}
	}
	return nil
}

func (vm *VM) testLoadDataStaticResourceContent(name string) (string, bool, error) {
	rawName := strings.TrimSpace(name)
	resourceNamespace, resourceName, hasNamespace := strings.Cut(rawName, "__")
	if hasNamespace {
		if !strings.EqualFold(resourceNamespace, strings.TrimSpace(vm.Org.Namespace)) {
			return "", false, newExceptionError("StringException", fmt.Sprintf("Test.loadData static resource %s namespace %s does not match local package namespace %s", rawName, resourceNamespace, vm.Org.Namespace))
		}
		content, ok := vm.staticResourceContent(resourceName)
		return content, ok, nil
	}
	content, ok := vm.staticResourceContent(rawName)
	return content, ok, nil
}
func (vm *VM) staticResourceContent(name string) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	resourceName, namespace, hasNamespace := strings.Cut(strings.TrimSpace(name), "__")
	if hasNamespace {
		if namespace == "" {
			return "", false
		}
		if !strings.EqualFold(resourceName, vm.Org.Namespace) {
			return "", false
		}
		name = namespace
	}
	for _, resource := range vm.Org.Metadata.StaticResources {
		if !strings.EqualFold(resource.Name, name) {
			continue
		}
		if hasNamespace && strings.TrimSpace(resource.NamespacePrefix) != "" && !strings.EqualFold(resource.NamespacePrefix, resourceName) {
			continue
		}
		if resource.Content != "" {
			return resource.Content, true
		}
		if resource.ContentPath != "" {
			content, err := os.ReadFile(resource.ContentPath)
			if err == nil {
				return string(content), true
			}
		}
		return "", false
	}
	return "", false
}

func (vm *VM) testLoadDataRelationshipHeader(definition storage.ObjectDefinition, header string) bool {
	relationship, externalField, ok := strings.Cut(strings.TrimSpace(header), ".")
	if !ok || relationship == "" || externalField == "" || vm == nil || vm.Org == nil {
		return false
	}
	_, parentObject, ok := vm.testLoadDataRelationshipField(definition, relationship)
	if !ok {
		return false
	}
	parentState, ok := vm.Org.Objects[parentObject]
	if !ok {
		return false
	}
	parentFieldName, ok := storage.ResolveFieldName(parentState.Definition, vm.Org.Namespace, externalField)
	if !ok {
		return false
	}
	parentField := parentState.Definition.Fields[parentFieldName]
	return parentField.ExternalID || strings.EqualFold(parentFieldName, "Id")
}

func (vm *VM) testLoadDataRelationshipValue(objectName, header, raw string) (string, Value, bool, error) {
	relationship, externalField, ok := strings.Cut(strings.TrimSpace(header), ".")
	if !ok || relationship == "" || externalField == "" || vm == nil || vm.Org == nil {
		return "", Null, false, nil
	}
	canonicalObject := objectName
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		canonicalObject = resolved
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return "", Null, false, nil
	}
	lookupField, parentObject, ok := vm.testLoadDataRelationshipField(object.Definition, relationship)
	if !ok {
		return "", Null, false, nil
	}
	if strings.TrimSpace(raw) == "" {
		return lookupField.APIName, Null, true, nil
	}
	parentState, ok := vm.Org.Objects[parentObject]
	if !ok {
		return "", Null, true, newExceptionError("DmlException", fmt.Sprintf("FIELD_INTEGRITY_EXCEPTION: Test.loadData %s.%s references unknown object %s", objectName, header, parentObject))
	}
	parentFieldName, ok := storage.ResolveFieldName(parentState.Definition, vm.Org.Namespace, externalField)
	if !ok {
		return "", Null, true, newExceptionError("DmlException", fmt.Sprintf("FIELD_INTEGRITY_EXCEPTION: Test.loadData %s.%s references unknown external ID field", objectName, header))
	}
	parentField := parentState.Definition.Fields[parentFieldName]
	if !parentField.ExternalID && !strings.EqualFold(parentFieldName, "Id") {
		return "", Null, true, newExceptionError("DmlException", fmt.Sprintf("FIELD_INTEGRITY_EXCEPTION: Test.loadData %s.%s field is not an external ID", objectName, header))
	}
	matches := make([]storage.Record, 0, 1)
	for _, record := range parentState.Records {
		if strings.EqualFold(storageStringForExternalID(record, parentFieldName), strings.TrimSpace(raw)) {
			matches = append(matches, record)
		}
	}
	if len(matches) == 0 {
		return "", Null, true, newExceptionError("DmlException", fmt.Sprintf("FIELD_INTEGRITY_EXCEPTION: Test.loadData %s.%s external ID %s matched no %s record", objectName, header, raw, parentObject))
	}
	if len(matches) > 1 {
		return "", Null, true, newExceptionError("DmlException", fmt.Sprintf("DUPLICATE_VALUE: Test.loadData %s.%s external ID %s matched multiple %s records", objectName, header, raw, parentObject))
	}
	return lookupField.APIName, platformScalar("Id", string(matches[0].ID)), true, nil
}

func (vm *VM) testLoadDataRelationshipField(definition storage.ObjectDefinition, relationship string) (storage.Field, string, bool) {
	for _, field := range definition.Fields {
		if field.Type != storage.FieldReference {
			continue
		}
		if !strings.EqualFold(storage.ParentRelationshipName(field), relationship) && !strings.EqualFold(field.RelationshipName, relationship) {
			continue
		}
		for _, target := range field.ReferenceTo {
			parentObject := target
			if resolved, ok := vm.resolveObjectName(target); ok {
				parentObject = resolved
			}
			return field, parentObject, true
		}
	}
	return storage.Field{}, "", false
}

func storageStringForExternalID(record storage.Record, field string) string {
	value, ok := record.GetField(field)
	if !ok {
		return ""
	}
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString, storage.ValueBlob, storage.ValueDate, storage.ValueDateTime:
		return value.String
	default:
		return ""
	}
}
func (vm *VM) testLoadDataFieldValue(objectName, fieldName, raw string) (Value, error) {
	if strings.TrimSpace(raw) == "" {
		return Null, nil
	}
	if vm == nil || vm.Org == nil {
		return String(raw), nil
	}
	canonicalObject := objectName
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		canonicalObject = resolved
	}
	object, ok := vm.Org.Objects[canonicalObject]
	if !ok {
		return String(raw), nil
	}
	canonicalField := fieldName
	if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, fieldName); ok {
		canonicalField = resolved
	}
	field, ok := object.Definition.Fields[canonicalField]
	if !ok {
		return String(raw), nil
	}
	switch field.Type {
	case storage.FieldBoolean:
		return Bool(strings.EqualFold(raw, "true") || raw == "1"), nil
	case storage.FieldInteger:
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return Null, err
		}
		return Int(parsed), nil
	case storage.FieldDecimal:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return Null, err
		}
		return Decimal(parsed), nil
	case storage.FieldID, storage.FieldReference:
		return platformScalar("Id", raw), nil
	case storage.FieldDate:
		return platformScalar("Date", raw), nil
	case storage.FieldDateTime:
		return platformScalar("Datetime", raw), nil
	case storage.FieldBlob:
		return platformScalar("Blob", raw), nil
	default:
		return String(raw), nil
	}
}
func (vm *VM) testInstall(args []Value, result *Result) (Value, error) {
	if len(args) != 2 && len(args) != 3 {
		return Null, fmt.Errorf("Test.testInstall expects InstallHandler, previousVersion[, isPush]")
	}
	if err := vm.requireTestContext("Test.testInstall"); err != nil {
		return Null, err
	}
	handler := args[0]
	if handler.Kind != ValueObject || handler.Type == "" {
		return Null, fmt.Errorf("Test.testInstall expects InstallHandler")
	}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(handler.Type, "onInstall", []Value{Object("InstallContext")})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(handler.Type+".onInstall", []Value{Object("InstallContext")})
	}
	if !ok {
		return Null, fmt.Errorf("Test.testInstall expects InstallHandler with onInstall")
	}
	context := Object("InstallContext")
	context.Fields["PreviousVersion"] = args[1]
	context.Fields["InstallerId"] = Null
	context.Fields["installerId"] = Null
	if len(args) == 3 {
		context.Fields["IsPush"] = args[2]
	}
	vm.installContextDepth++
	defer func() {
		vm.installContextDepth--
	}()
	if _, err := vm.callMethodWithReceiver(method, handler, []Value{context}, result); err != nil {
		return Null, err
	}
	return Null, nil
}

func (vm *VM) testUninstall(args []Value, result *Result) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("Test.testUninstall expects UninstallHandler")
	}
	if err := vm.requireTestContext("Test.testUninstall"); err != nil {
		return Null, err
	}
	handler := args[0]
	if handler.Kind != ValueObject || handler.Type == "" {
		return Null, fmt.Errorf("Test.testUninstall expects UninstallHandler")
	}
	context := Object("UninstallContext")
	context.Fields["organizationId"] = platformScalar("Id", vm.orgID())
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(handler.Type, "onUninstall", []Value{context})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(handler.Type+".onUninstall", []Value{context})
	}
	if !ok {
		return Null, fmt.Errorf("Test.testUninstall expects UninstallHandler with onUninstall")
	}
	vm.installContextDepth++
	defer func() {
		vm.installContextDepth--
	}()
	if _, err := vm.callMethodWithReceiver(method, handler, []Value{context}, result); err != nil {
		return Null, err
	}
	return Null, nil
}

func testMockTypeName(value Value) (string, bool) {
	switch value.Kind {
	case ValueString:
		if value.Text == "" {
			return "", false
		}
		return value.Text, true
	case ValueObject:
		if (strings.EqualFold(value.Type, "Type") || strings.EqualFold(value.Static, "Type") || strings.EqualFold(value.Runtime, "Type")) && value.Text != "" {
			return value.Text, true
		}
	}
	return "", false
}
