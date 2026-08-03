package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/storage"
)

func apexPagesSeverityStaticValue(name string) (Value, bool) {
	prefix := "ApexPages.Severity."
	if len(name) < len(prefix) || !strings.EqualFold(name[:len(prefix)], prefix) {
		return Null, false
	}
	severity := name[len(prefix):]
	for i, candidate := range apexPagesSeverityNames {
		if strings.EqualFold(severity, candidate) {
			return Value{Kind: ValueObject, Type: "ApexPages.Severity", Text: candidate, Fields: map[string]Value{"ordinal": Int(int64(i))}}, true
		}
	}
	return Null, false
}

func apexPagesSeverityName(value Value) (string, bool) {
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "ApexPages.Severity") && value.Text != "" {
		return value.Text, true
	}
	if value.Kind == ValueString {
		for _, candidate := range apexPagesSeverityNames {
			if strings.EqualFold(value.Text, candidate) {
				return candidate, true
			}
		}
	}
	return "", false
}

func apexPagesSeverityValues(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("ApexPages.Severity.values expects 0 arguments")
	}
	values := make([]Value, 0, len(apexPagesSeverityNames))
	for i, name := range apexPagesSeverityNames {
		value := Value{Kind: ValueObject, Type: "ApexPages.Severity", Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func metadataDeployStatusStaticValue(name string) (Value, bool) {
	return namedEnumStaticValue("Metadata.DeployStatus", metadataDeployStatusNames, name)
}

func metadataMetadataTypeStaticValue(name string) (Value, bool) {
	return namedEnumStaticValue("Metadata.MetadataType", metadataMetadataTypeNames, name)
}

func soapTypeForStorageField(field storage.Field) string {
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		return "ID"
	case storage.FieldBoolean:
		return "BOOLEAN"
	case storage.FieldInteger:
		return "INTEGER"
	case storage.FieldDecimal:
		return "DOUBLE"
	case storage.FieldDate:
		return "DATE"
	case storage.FieldDateTime:
		return "DATETIME"
	case storage.FieldBlob:
		return "BASE64BINARY"
	default:
		return "STRING"
	}
}

var schemaSOAPTypeNames = []string{"ID", "STRING", "BOOLEAN", "INTEGER", "DOUBLE", "DATE", "DATETIME", "TIME", "BASE64BINARY", "ANYTYPE"}

var schemaDisplayTypeNames = []string{"STRING", "BOOLEAN", "DOUBLE", "INTEGER", "PERCENT", "CURRENCY", "DATE", "DATETIME", "TIME", "PICKLIST", "MULTIPICKLIST", "DATACATEGORYGROUPREFERENCE", "BASE64", "ID", "REFERENCE", "TEXTAREA", "PHONE", "COMBOBOX", "URL", "EMAIL", "ANYTYPE", "LOCATION", "ENCRYPTEDSTRING", "COMPLEXVALUE", "ADDRESS", "SOBJECT", "LONG", "JSON", "FLOATARRAY", "TEXTARRAY"}

var schemaFieldDescribeOptionNames = []string{"DEFAULT", "FULL_DESCRIBE"}

var schemaSObjectDescribeOptionNames = []string{"DEFAULT", "FULL", "DEFERRED"}

func schemaSOAPTypeStaticValue(name string) (Value, bool) {
	if value, ok := namedEnumStaticValue("Schema.SOAPType", schemaSOAPTypeNames, name); ok {
		return value, true
	}
	if strings.HasPrefix(name, "Schema.SoapType.") {
		return namedEnumStaticValue("Schema.SOAPType", schemaSOAPTypeNames, "Schema.SOAPType."+strings.TrimPrefix(name, "Schema.SoapType."))
	}
	return Null, false
}

func schemaSOAPTypeValue(name string) Value {
	value, _ := namedEnumStaticValue("Schema.SOAPType", schemaSOAPTypeNames, "Schema.SOAPType."+name)
	return value
}

func schemaDisplayTypeStaticValue(name string) (Value, bool) {
	return namedEnumStaticValue("Schema.DisplayType", schemaDisplayTypeNames, name)
}

func schemaDisplayTypeValue(name string) Value {
	value, ok := namedEnumStaticValue("Schema.DisplayType", schemaDisplayTypeNames, "Schema.DisplayType."+name)
	if ok {
		return value
	}
	return Value{Kind: ValueObject, Type: "Schema.DisplayType", Text: name}
}

func namedEnumStaticValue(typeName string, names []string, name string) (Value, bool) {
	prefix := typeName + "."
	if !hasPrefixFold(name, prefix) {
		return Null, false
	}
	member := name[len(prefix):]
	for i, candidate := range names {
		if member == candidate {
			return Value{Kind: ValueObject, Type: typeName, Text: candidate, Fields: map[string]Value{"ordinal": Int(int64(i))}}, true
		}
	}
	for i, candidate := range names {
		if strings.EqualFold(member, candidate) {
			return Value{Kind: ValueObject, Type: typeName, Text: candidate, Fields: map[string]Value{"ordinal": Int(int64(i))}}, true
		}
	}
	return Null, false
}

func metadataDeployStatusValues(args []Value) (Value, error) {
	return namedEnumValues("Metadata.DeployStatus", metadataDeployStatusNames, args)
}

func metadataMetadataTypeValues(args []Value) (Value, error) {
	return namedEnumValues("Metadata.MetadataType", metadataMetadataTypeNames, args)
}

func (vm *VM) reportsDescribeReport(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("reports.ReportManager.describeReport expects report Id")
	}
	describe := Object("reports.ReportDescribeResult")
	describe.Fields["reportMetadata"] = vm.reportsReportMetadata(args[0], Null)
	describe.Fields["reportExtendedMetadata"] = vm.reportsReportExtendedMetadata()
	describe.Fields["reportTypeMetadata"] = vm.reportsReportTypeMetadata()
	return describe, nil
}

func (vm *VM) reportsDatatypeFilterOperatorMap(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("reports.ReportManager.getDatatypeFilterOperatorMap expects 0 arguments")
	}
	return typedMap("Map<String,List<reports.FilterOperator>>"), nil
}

func (vm *VM) reportsGetReportInstance(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("reports.ReportManager.getReportInstance expects instance Id")
	}
	instanceID := scalarText(args[0])
	if vm.reportInstances != nil {
		if value, ok := vm.reportInstances[instanceID]; ok {
			return cloneValue(value), nil
		}
	}
	return vm.reportsReportInstance(args[0], Null, vm.reportsReportResults(Null, Null, false)), nil
}

func (vm *VM) reportsGetReportInstances(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("reports.ReportManager.getReportInstances expects report Id")
	}
	out := typedList("List<reports.ReportInstance>")
	reportID := scalarText(args[0])
	for _, instance := range vm.reportInstances {
		if _, value, found := objectFieldValue(instance, "reportId"); found && scalarText(value) == reportID {
			out.List = append(out.List, cloneValue(instance))
		}
	}
	return out, nil
}

func (vm *VM) reportsRunAsyncReport(args []Value) (Value, error) {
	reportID, metadata, includeDetails, err := vm.reportsReportArgs(args, "reports.ReportManager.runAsyncReport")
	if err != nil {
		return Null, err
	}
	results := vm.reportsReportResults(reportID, metadata, includeDetails)
	instanceID := platformScalar("Id", vm.nextReportInstanceID())
	instance := vm.reportsReportInstance(instanceID, reportID, results)
	if vm.reportInstances == nil {
		vm.reportInstances = make(map[string]Value)
	}
	vm.reportInstances[scalarText(instanceID)] = cloneValue(instance)
	return instance, nil
}

func (vm *VM) reportsRunReport(args []Value) (Value, error) {
	reportID, metadata, includeDetails, err := vm.reportsReportArgs(args, "reports.ReportManager.runReport")
	if err != nil {
		return Null, err
	}
	return vm.reportsReportResults(reportID, metadata, includeDetails), nil
}

func (vm *VM) reportsReportArgs(args []Value, callee string) (Value, Value, bool, error) {
	if len(args) < 1 || len(args) > 3 {
		return Null, Null, false, fmt.Errorf("%s expects report Id[, ReportMetadata][, includeDetails]", callee)
	}
	reportID := args[0]
	metadata := Null
	includeDetails := false
	for _, arg := range args[1:] {
		switch {
		case arg.Kind == ValueBool:
			includeDetails = arg.Bool
		case arg.Kind == ValueObject && strings.EqualFold(arg.Type, "reports.ReportMetadata"):
			metadata = arg
		default:
			return Null, Null, false, fmt.Errorf("%s expects report Id[, ReportMetadata][, includeDetails]", callee)
		}
	}
	return reportID, metadata, includeDetails, nil
}

func (vm *VM) reportsReportResults(reportID, metadata Value, includeDetails bool) Value {
	results := Object("reports.ReportResults")
	results.Fields["allData"] = Bool(false)
	results.Fields["factMap"] = typedMap("Map<String,reports.ReportFact>")
	results.Fields["groupingsAcross"] = Object("reports.Dimension")
	results.Fields["groupingsDown"] = Object("reports.Dimension")
	results.Fields["hasDetailRows"] = Bool(includeDetails)
	results.Fields["reportExtendedMetadata"] = vm.reportsReportExtendedMetadata()
	results.Fields["reportMetadata"] = vm.reportsReportMetadata(reportID, metadata)
	return results
}

func (vm *VM) reportsReportMetadata(reportID, override Value) Value {
	if override.Kind == ValueObject && strings.EqualFold(override.Type, "reports.ReportMetadata") {
		metadata := cloneValue(override)
		if _, _, ok := objectFieldValue(metadata, "id"); !ok && reportID.Kind != ValueNull {
			metadata.Fields["id"] = reportID
		}
		return metadata
	}
	metadata := Object("reports.ReportMetadata")
	if reportID.Kind != ValueNull {
		metadata.Fields["id"] = reportID
	}
	metadata.Fields["name"] = String("Local Report")
	metadata.Fields["developerName"] = String("Local_Report")
	metadata.Fields["groupingsAcross"] = typedList("List<reports.GroupingInfo>")
	metadata.Fields["groupingsDown"] = typedList("List<reports.GroupingInfo>")
	metadata.Fields["aggregates"] = typedList("List<String>")
	metadata.Fields["buckets"] = typedList("List<reports.BucketField>")
	metadata.Fields["detailColumns"] = typedList("List<String>")
	metadata.Fields["reportFilters"] = typedList("List<reports.ReportFilter>")
	metadata.Fields["historicalSnapshotDates"] = typedList("List<String>")
	metadata.Fields["sortBy"] = typedList("List<reports.SortColumn>")
	metadata.Fields["standardFilters"] = typedList("List<reports.StandardFilter>")
	metadata.Fields["customSummaryFormula"] = typedMap("Map<String,reports.ReportCsf>")
	metadata.Fields["crossFilters"] = typedList("List<reports.CrossFilter>")
	metadata.Fields["hasDetailRows"] = Bool(false)
	metadata.Fields["hasRecordCount"] = Bool(false)
	metadata.Fields["showSubtotals"] = Bool(false)
	metadata.Fields["showGrandTotal"] = Bool(false)
	return metadata
}

func (vm *VM) reportsReportExtendedMetadata() Value {
	metadata := Object("reports.ReportExtendedMetadata")
	metadata.Fields["aggregateColumnInfo"] = typedMap("Map<String,reports.AggregateColumn>")
	metadata.Fields["detailColumnInfo"] = typedMap("Map<String,reports.DetailColumn>")
	metadata.Fields["groupingColumnInfo"] = typedMap("Map<String,reports.GroupingColumn>")
	return metadata
}

func (vm *VM) reportsReportTypeMetadata() Value {
	metadata := Object("reports.ReportTypeMetadata")
	metadata.Fields["categories"] = typedList("List<reports.ReportTypeColumnCategory>")
	metadata.Fields["standardDateFilterDurationGroups"] = typedList("List<reports.StandardDateFilterDurationGroup>")
	metadata.Fields["standardFilterInfos"] = typedMap("Map<String,reports.StandardFilterInfo>")
	return metadata
}

func (vm *VM) reportsReportInstance(instanceID, reportID, results Value) Value {
	instance := Object("reports.ReportInstance")
	instance.Fields["id"] = instanceID
	instance.Fields["reportId"] = reportID
	instance.Fields["reportResults"] = results
	instance.Fields["status"] = String("Success")
	instance.Fields["ownerId"] = platformScalar("Id", vm.currentUserInfoField("Id", "005000000000001"))
	now := platformScalar("Datetime", formatPlatformDatetime(vm.fakeNow))
	instance.Fields["requestDate"] = now
	instance.Fields["completionDate"] = now
	return instance
}

func (vm *VM) nextReportInstanceID() string {
	return fmt.Sprintf("0LG000000%06d", len(vm.reportInstances)+1)
}

func prefCenterGenerateToken(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("pref_center.TokenUtility.generateToken expects String[, TokenType]")
	}
	return String(prefCenterLocalToken(args[0], args[1:])), nil
}

func prefCenterGenerateTokens(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 3 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("pref_center.TokenUtility.generateTokens expects List<String>[, TokenType][, DataCloudIdTokenType]")
	}
	out := typedMap("Map<String,String>")
	for _, tokenValue := range args[0].List {
		if tokenValue.Kind != ValueString {
			return Null, fmt.Errorf("pref_center.TokenUtility.generateTokens expects List<String>")
		}
		key := mapKey(tokenValue)
		out.Map[key] = String(prefCenterLocalToken(tokenValue, args[1:]))
		out.MapKeys[key] = tokenValue
	}
	return out, nil
}

func prefCenterLocalToken(tokenValue Value, options []Value) string {
	parts := []string{tokenValue.Text}
	for _, option := range options {
		parts = append(parts, scalarText(option))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "local-token-" + hex.EncodeToString(sum[:])[:24]
}

func functionInvocationSuccess(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
		return Null, fmt.Errorf("functions.MockFunctionInvocationFactory.createSuccessResponse expects invocation Id and response")
	}
	invocation := Object("functions.FunctionInvocation")
	invocation.Fields["invocationId"] = args[0]
	invocation.Fields["response"] = args[1]
	invocation.Fields["status"] = Value{Kind: ValueObject, Type: "functions.FunctionInvocationStatus", Text: "SUCCESS"}
	invocation.Fields["error"] = Null
	return invocation, nil
}

func functionInvocationError(args []Value) (Value, error) {
	if len(args) != 3 || args[0].Kind != ValueString || args[2].Kind != ValueString {
		return Null, fmt.Errorf("functions.MockFunctionInvocationFactory.createErrorResponse expects invocation Id, error type, and message")
	}
	errValue := Object("functions.FunctionInvocationError")
	errValue.Fields["type"] = args[1]
	errValue.Fields["message"] = args[2]
	invocation := Object("functions.FunctionInvocation")
	invocation.Fields["invocationId"] = args[0]
	invocation.Fields["response"] = Null
	invocation.Fields["status"] = Value{Kind: ValueObject, Type: "functions.FunctionInvocationStatus", Text: "ERROR"}
	invocation.Fields["error"] = errValue
	return invocation, nil
}

func waveTemplatesStaticDefault(callee string, args []Value) (Value, error) {
	switch callee {
	case "wave.Templates.cdpQueryMetadata":
		if len(args) != 4 {
			return Null, fmt.Errorf("wave.Templates.cdpQueryMetadata expects 4 arguments")
		}
	case "wave.Templates.getSObject", "wave.Templates.getTemplate", "wave.Templates.getTemplateConfig":
		if len(args) != 1 && len(args) != 3 && len(args) != 4 {
			return Null, fmt.Errorf("%s expects supported template lookup arguments", callee)
		}
	case "wave.Templates.getTemplates":
		if len(args) > 1 {
			return Null, fmt.Errorf("wave.Templates.getTemplates expects optional search options")
		}
	}
	out := typedMap("Map<String,Object>")
	out.Map[mapKey(String("local"))] = Bool(true)
	out.MapKeys[mapKey(String("local"))] = String("local")
	return out, nil
}

func flowInterviewCreate(args []Value) (Value, error) {
	if len(args) != 2 && len(args) != 3 {
		return Null, fmt.Errorf("Flow.Interview.createInterview expects flow name and input variables")
	}
	offset := 0
	if len(args) == 3 {
		if args[0].Kind != ValueString {
			return Null, fmt.Errorf("Flow.Interview.createInterview expects namespace String")
		}
		offset = 1
	}
	if args[offset].Kind != ValueString || args[offset+1].Kind != ValueMap {
		return Null, fmt.Errorf("Flow.Interview.createInterview expects flow name and input variables")
	}
	interview := Object("Flow.Interview")
	if offset == 1 {
		interview.Fields["namespace"] = args[0]
	}
	interview.Fields["flowName"] = args[offset]
	interview.Fields["variables"] = args[offset+1]
	interview.Fields["status"] = String("NotStarted")
	interview.Fields["started"] = Bool(false)
	interview.Fields["outputs"] = Map()
	return interview, nil
}

func (vm *VM) metadataEnqueueDeployment(args []Value, result *Result) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueObject || args[0].Type != "Metadata.DeployContainer" {
		return Null, fmt.Errorf("Metadata.Operations.enqueueDeployment expects DeployContainer and DeployCallback")
	}
	if args[1].Kind != ValueNull {
		if args[1].Kind != ValueObject || !vm.typeAssignableTo(args[1].Type, "Metadata.DeployCallback") {
			return Null, fmt.Errorf("Metadata.Operations.enqueueDeployment expects DeployCallback or null")
		}
		if _, err := vm.metadataDeploymentCallbackMethod(args[1]); err != nil {
			return Null, err
		}
	}
	deploymentID := "0Af000000000001"
	items := args[0].Fields["components"]
	if items.Kind == ValueNull || (items.Kind == ValueList && len(items.List) == 0) {
		vm.recordMetadataDeployment(deploymentID, nil)
		appendTrace(result, "apex.metadata.deploy.enqueue", "apex.metadata", map[string]any{
			"deploymentId": deploymentID,
			"components":   0,
			"success":      true,
		})
		if err := vm.invokeMetadataDeploymentCallback(args[1], deploymentID, result); err != nil {
			return Null, err
		}
		return platformScalar("Id", deploymentID), nil
	}
	if items.Kind != ValueList {
		return Null, fmt.Errorf("Metadata.DeployContainer.components must be a list")
	}
	if vm.Org == nil {
		return Null, unsupportedCallError("Metadata.Operations.enqueueDeployment requires org storage for local metadata mutation")
	}
	originalOrg := vm.Org
	candidateOrg := originalOrg.Clone()
	vm.Org = &candidateOrg
	for _, item := range items.List {
		if err := vm.applyMetadataDeployment(item); err != nil {
			vm.Org = originalOrg
			var runtimeErr *RuntimeError
			if errors.As(err, &runtimeErr) && runtimeErr.Type == "UnsupportedFeature" {
				return Null, err
			}
			vm.recordMetadataDeploymentFailure(deploymentID, items.List, item, err)
			appendTrace(result, "apex.metadata.deploy.enqueue", "apex.metadata", map[string]any{
				"deploymentId": deploymentID,
				"components":   len(items.List),
				"success":      false,
				"error":        err.Error(),
			})
			if callbackErr := vm.invokeMetadataDeploymentCallback(args[1], deploymentID, result); callbackErr != nil {
				return Null, callbackErr
			}
			return platformScalar("Id", deploymentID), nil
		}
	}
	*originalOrg = candidateOrg
	vm.Org = originalOrg
	vm.recordMetadataDeployment(deploymentID, items.List)
	appendTrace(result, "apex.metadata.deploy.enqueue", "apex.metadata", map[string]any{
		"deploymentId": deploymentID,
		"components":   len(items.List),
		"success":      true,
	})
	if err := vm.invokeMetadataDeploymentCallback(args[1], deploymentID, result); err != nil {
		return Null, err
	}
	return platformScalar("Id", deploymentID), nil
}

func (vm *VM) metadataDeploymentCallbackMethod(callback Value) (Method, error) {
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(
		callback.Type,
		"handleResult",
		[]Value{Object("Metadata.DeployResult"), Object("Metadata.DeployCallbackContext")},
	)
	if ambiguous {
		return Method{}, vm.ambiguousOverloadError(callback.Type+".handleResult", []Value{Object("Metadata.DeployResult"), Object("Metadata.DeployCallbackContext")})
	}
	if !ok {
		return Method{}, fmt.Errorf("Metadata.DeployCallback %s has no handleResult method", callback.Type)
	}
	return method, nil
}

func (vm *VM) invokeMetadataDeploymentCallback(callback Value, deploymentID string, result *Result) error {
	if callback.Kind == ValueNull {
		return nil
	}
	method, err := vm.metadataDeploymentCallbackMethod(callback)
	if err != nil {
		return err
	}
	deployResult, ok := vm.metadataDeploys[deploymentID]
	if !ok {
		return fmt.Errorf("Metadata.Operations.enqueueDeployment missing local result %s", deploymentID)
	}
	context := Object("Metadata.DeployCallbackContext")
	context.Fields["__callbackJobId"] = platformScalar("Id", deploymentID)
	_, err = vm.callMethodWithReceiver(method, callback, []Value{cloneMetadataDeployResult(deployResult), context}, result)
	return err
}

func (vm *VM) metadataCheckDeployStatus(args []Value, result *Result) (Value, error) {
	if len(args) < 1 || len(args) > 2 || !metadataDeploymentIDValue(args[0]) {
		return Null, fmt.Errorf("Metadata.Operations.checkDeployStatus expects deployment Id[, includeDetails]")
	}
	includeDetails := false
	if len(args) == 2 {
		if args[1].Kind != ValueBool {
			return Null, fmt.Errorf("Metadata.Operations.checkDeployStatus includeDetails expects Boolean")
		}
		includeDetails = args[1].Bool
	}
	deploymentID := args[0].Text
	if args[0].Kind == ValueObject {
		var err error
		deploymentID, err = platformScalarText(args[0], "Id")
		if err != nil {
			return Null, err
		}
	}
	if vm.metadataDeploys == nil {
		vm.metadataDeploys = make(map[string]Value)
	}
	storedResult, ok := vm.metadataDeploys[deploymentID]
	if !ok && len(deploymentID) == 18 {
		storedResult, ok = vm.metadataDeploys[deploymentID[:15]]
	}
	if !ok {
		return Null, unsupportedCallError("Metadata.Operations.checkDeployStatus unknown local deployment " + deploymentID)
	}
	deployResult := cloneMetadataDeployResult(storedResult)
	if !includeDetails {
		deployResult.Fields["details"] = Null
	}
	appendTrace(result, "apex.metadata.deploy.status", "apex.metadata", map[string]any{
		"deploymentId":   deploymentID,
		"includeDetails": includeDetails,
		"success":        deployResult.Fields["success"].Bool,
		"status":         deployResult.Fields["status"].Text,
	})
	return deployResult, nil
}

func metadataDeploymentIDValue(value Value) bool {
	return value.Kind == ValueString || (value.Kind == ValueObject && value.Type == "Id")
}

func (vm *VM) recordMetadataDeployment(deploymentID string, items []Value) {
	if vm.metadataDeploys == nil {
		vm.metadataDeploys = make(map[string]Value)
	}
	result := metadataDeployResultObject(deploymentID, items)
	vm.metadataDeploys[deploymentID] = result
	if len(deploymentID) == 15 {
		vm.metadataDeploys[apexIDTo18(deploymentID)] = result
	}
}

func (vm *VM) recordMetadataDeploymentFailure(deploymentID string, items []Value, failedItem Value, err error) {
	if vm.metadataDeploys == nil {
		vm.metadataDeploys = make(map[string]Value)
	}
	result := metadataDeployFailureResultObject(deploymentID, items, failedItem, err)
	vm.metadataDeploys[deploymentID] = result
	if len(deploymentID) == 15 {
		vm.metadataDeploys[apexIDTo18(deploymentID)] = result
	}
}

func (vm *VM) applyMetadataDeployment(item Value) error {
	if item.Kind != ValueObject {
		return unsupportedCallError("Metadata.Operations.enqueueDeployment " + string(item.Kind) + " metadata deploy")
	}
	switch item.Type {
	case "Metadata.CustomMetadata":
		return vm.applyCustomMetadataDeployment(item)
	case "Metadata.CustomObject":
		return vm.applyCustomObjectDeployment(item)
	case "Metadata.CustomField":
		return vm.applyCustomFieldDeployment(item)
	default:
		typeName := item.Type
		if typeName == "" {
			typeName = string(item.Kind)
		}
		return unsupportedCallError("Metadata.Operations.enqueueDeployment " + typeName + " metadata deploy")
	}
}

func (vm *VM) applyCustomMetadataDeployment(item Value) error {
	fullName, ok := metadataStringField(item, "fullName")
	if !ok || strings.TrimSpace(fullName) == "" {
		return fmt.Errorf("Metadata.CustomMetadata.fullName is required")
	}
	objectName, developerName := metadataCustomMetadataNames(fullName)
	if objectName == "" || developerName == "" {
		return fmt.Errorf("Metadata.CustomMetadata.fullName must be Type.Record")
	}
	state := vm.metadataCustomMetadataState(objectName)
	definition := state.Definition
	recordFields := map[string]storage.Value{
		"DeveloperName":    storage.StringValue(developerName),
		"MasterLabel":      storage.StringValue(metadataLabelOrDefault(item, developerName)),
		"Label":            storage.StringValue(metadataLabelOrDefault(item, developerName)),
		"NamespacePrefix":  storage.StringValue(metadataNamespacePrefix(vm.Org.Namespace, definition.APIName)),
		"QualifiedApiName": storage.StringValue(metadataQualifiedAPIName(vm.Org.Namespace, definition.APIName, developerName)),
	}
	values := item.Fields["values"]
	if values.Kind != ValueNull {
		if values.Kind != ValueList {
			return fmt.Errorf("Metadata.CustomMetadata.values must be a list")
		}
		for _, valueItem := range values.List {
			fieldName, fieldValue, err := vm.metadataCustomMetadataValue(definition, valueItem)
			if err != nil {
				return err
			}
			recordFields[fieldName] = fieldValue
		}
	}
	var recordID storage.ID
	for _, existing := range state.Records {
		if customDataRecordMatches(definition, "custom metadata", existing, developerName, vm.Org.Namespace) ||
			customDataRecordMatches(definition, "custom metadata", existing, fullName, vm.Org.Namespace) {
			recordID = existing.ID
			break
		}
	}
	if recordID == "" {
		recordID = nextMetadataRecordID(state)
	}
	record := storage.Record{ID: recordID, Object: definition.APIName, Fields: recordFields}
	record.Fields["Id"] = storage.IDValue(recordID)
	if mutable, _ := storage.EnsureMutableObjectRecords(vm.Org, objectName); mutable != nil {
		state = *mutable
	}
	state.Records[recordID] = record
	vm.Org.Objects[definition.APIName] = state
	vm.clearMetadataCaches()
	return nil
}

func (vm *VM) applyCustomObjectDeployment(item Value) error {
	fullName, ok := metadataStringField(item, "fullName")
	if !ok || strings.TrimSpace(fullName) == "" {
		return fmt.Errorf("Metadata.CustomObject.fullName is required")
	}
	objectName := strings.TrimSpace(fullName)
	if !isCustomObjectLikeName(objectName) {
		return fmt.Errorf("Metadata.CustomObject.fullName must be a custom object API name")
	}
	objectName = storage.NamespaceTokenName(vm.Org.Namespace, objectName)
	state := vm.Org.Objects[objectName]
	state.Definition = state.Definition.Clone()
	state.Definition.APIName = objectName
	if state.Definition.Label == "" {
		state.Definition.Label = metadataTextFieldOrDefault(item, "label", strings.TrimSuffix(objectName, "__c"))
	}
	if state.Definition.PluralLabel == "" {
		state.Definition.PluralLabel = metadataTextFieldOrDefault(item, "pluralLabel", state.Definition.Label+"s")
	}
	if state.Definition.SharingModel == "" {
		state.Definition.SharingModel = metadataTextFieldOrDefault(item, "sharingModel", "ReadWrite")
	}
	if state.Definition.KeyPrefix == "" {
		state.Definition.KeyPrefix = storage.AssignDeterministicPrefixes([]string{objectName}, nil)[objectName]
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	if _, ok := state.Definition.Fields["Name"]; !ok {
		state.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Name", Type: storage.FieldString}
	}
	if state.Definition.Metadata == nil {
		state.Definition.Metadata = map[string]string{"kind": "customObject"}
	}
	storage.EnsureStandardObjectFields(&state.Definition)
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	vm.Org.Objects[objectName] = state
	vm.clearMetadataCaches()
	return nil
}

func (vm *VM) applyCustomFieldDeployment(item Value) error {
	fullName, ok := metadataStringField(item, "fullName")
	if !ok || strings.TrimSpace(fullName) == "" {
		return fmt.Errorf("Metadata.CustomField.fullName is required")
	}
	objectName, fieldName := metadataCustomFieldNames(fullName)
	if objectName == "" || fieldName == "" {
		return fmt.Errorf("Metadata.CustomField.fullName must be Object.Field")
	}
	objectName = storage.NamespaceTokenName(vm.Org.Namespace, objectName)
	fieldName = storage.NamespaceTokenName(vm.Org.Namespace, fieldName)
	state, ok := vm.Org.Objects[objectName]
	if !ok {
		return fmt.Errorf("Metadata.CustomField.fullName references unknown object %s", objectName)
	}
	state.Definition = state.Definition.Clone()
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	fieldType, displayType := metadataCustomFieldType(item)
	field := state.Definition.Fields[fieldName]
	field.APIName = fieldName
	field.Label = metadataTextFieldOrDefault(item, "label", fieldName)
	field.Type = fieldType
	field.DisplayType = displayType
	field.Required = metadataBoolField(item, "required")
	field.ExternalID = metadataBoolField(item, "externalId")
	field.Unique = metadataBoolField(item, "unique")
	if referenceTo := metadataReferenceTo(item); len(referenceTo) > 0 {
		field.ReferenceTo = referenceTo
	}
	state.Definition.Fields[fieldName] = field
	vm.Org.Objects[objectName] = state
	vm.clearMetadataCaches()
	return nil
}

func (vm *VM) metadataCustomMetadataState(objectName string) storage.ObjectState {
	state := vm.Org.Objects[objectName]
	state.Definition = state.Definition.Clone()
	if state.Definition.APIName == "" {
		state.Definition.APIName = objectName
	}
	if state.Definition.KeyPrefix == "" {
		state.Definition.KeyPrefix = storage.AssignDeterministicPrefixes([]string{objectName}, nil)[objectName]
	}
	if state.Definition.Metadata == nil {
		state.Definition.Metadata = map[string]string{"kind": "customMetadata"}
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	for _, field := range []storage.Field{
		{APIName: "DeveloperName", Type: storage.FieldString},
		{APIName: "MasterLabel", Type: storage.FieldString},
		{APIName: "Label", Type: storage.FieldString},
		{APIName: "NamespacePrefix", Type: storage.FieldString},
		{APIName: "QualifiedApiName", Type: storage.FieldString},
	} {
		if _, ok := state.Definition.Fields[field.APIName]; !ok {
			state.Definition.Fields[field.APIName] = field
		}
	}
	storage.EnsureStandardObjectFields(&state.Definition)
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	vm.Org.Objects[objectName] = state
	return state
}

func (vm *VM) metadataCustomMetadataValue(definition storage.ObjectDefinition, item Value) (string, storage.Value, error) {
	if item.Kind != ValueObject || item.Type != "Metadata.CustomMetadataValue" {
		return "", storage.Value{}, fmt.Errorf("Metadata.CustomMetadata.values expects CustomMetadataValue entries")
	}
	fieldName, ok := metadataStringField(item, "field")
	if !ok || strings.TrimSpace(fieldName) == "" {
		return "", storage.Value{}, fmt.Errorf("Metadata.CustomMetadataValue.field is required")
	}
	resolved, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		value := item.Fields["value"]
		fieldType := metadataFieldTypeFromValue(value)
		resolved = storage.NamespaceTokenName(vm.Org.Namespace, fieldName)
		field := storage.Field{APIName: resolved, Type: fieldType}
		if state, exists := vm.Org.Objects[definition.APIName]; exists {
			state.Definition = state.Definition.Clone()
			if state.Definition.Fields == nil {
				state.Definition.Fields = make(map[string]storage.Field)
			}
			state.Definition.Fields[resolved] = field
			vm.Org.Objects[definition.APIName] = state
		}
		definition.Fields = map[string]storage.Field{resolved: field}
	}
	converted, err := storageValueFromVMForField(item.Fields["value"], definition.Fields[resolved])
	if err != nil {
		return "", storage.Value{}, fmt.Errorf("Metadata.CustomMetadataValue.%s %v", fieldName, err)
	}
	return resolved, converted, nil
}

func metadataFieldTypeFromValue(value Value) storage.FieldType {
	switch value.Kind {
	case ValueBool:
		return storage.FieldBoolean
	case ValueInt:
		return storage.FieldInteger
	case ValueDecimal:
		return storage.FieldDecimal
	case ValueObject:
		switch strings.ToLower(value.Type) {
		case "date":
			return storage.FieldDate
		case "datetime":
			return storage.FieldDateTime
		case "id":
			return storage.FieldID
		}
	}
	return storage.FieldString
}

func (vm *VM) metadataRetrieve(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return Null, fmt.Errorf("Metadata.Operations.retrieve expects metadata type and full names")
	}
	if args[0].Kind != ValueObject || args[0].Type != "Metadata.MetadataType" {
		return Null, fmt.Errorf("Metadata.Operations.retrieve expects metadata type")
	}
	if !strings.EqualFold(args[0].Text, "CustomMetadata") {
		return Null, unsupportedCallError("Metadata.Operations.retrieve " + args[0].Text)
	}
	names, err := metadataStringList(args[1])
	if err != nil {
		return Null, err
	}
	if vm.Org == nil {
		return List(), nil
	}
	out := make([]Value, 0, len(names))
	for _, fullName := range names {
		objectName, developerName := metadataCustomMetadataNames(fullName)
		objectName, ok := vm.resolveObjectName(objectName)
		if !ok {
			continue
		}
		state := vm.Org.Objects[objectName]
		if !storage.IsCustomMetadataDefinition(state.Definition) {
			continue
		}
		for _, record := range sortedCustomDataRecords(state.Records, state.Definition, "custom metadata", vm.Org.Namespace) {
			if record.System.IsDeleted {
				continue
			}
			if customDataRecordMatches(state.Definition, "custom metadata", record, developerName, vm.Org.Namespace) ||
				customDataRecordMatches(state.Definition, "custom metadata", record, fullName, vm.Org.Namespace) {
				out = append(out, metadataCustomMetadataObject(state.Definition, record))
				break
			}
		}
	}
	return List(out...), nil
}

func metadataCustomMetadataObject(definition storage.ObjectDefinition, record storage.Record) Value {
	item := Object("Metadata.CustomMetadata")
	developerName := firstStringField(record, "DeveloperName", "Name")
	fullName := strings.TrimSuffix(definition.APIName, "__mdt") + "." + developerName
	item.Fields["fullName"] = String(fullName)
	item.Fields["label"] = String(firstStringField(record, "MasterLabel", "Label", "DeveloperName", "Name"))
	values := make([]Value, 0, len(record.Fields))
	for fieldName, fieldValue := range record.Fields {
		if isCustomMetadataSystemField(fieldName) {
			continue
		}
		value := Object("Metadata.CustomMetadataValue")
		value.Fields["field"] = String(fieldName)
		value.Fields["value"] = vmValueFromStorage(fieldValue)
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].Fields["field"].Text < values[j].Fields["field"].Text
	})
	item.Fields["values"] = List(values...)
	return item
}

func isCustomMetadataSystemField(fieldName string) bool {
	switch fieldName {
	case "Id", "DeveloperName", "MasterLabel", "Label", "NamespacePrefix", "QualifiedApiName", "Name":
		return true
	default:
		return false
	}
}

func metadataStringField(value Value, field string) (string, bool) {
	raw, ok := value.Fields[field]
	if !ok || raw.Kind != ValueString {
		return "", false
	}
	return raw.Text, true
}

func metadataTextFieldOrDefault(value Value, field, fallback string) string {
	if raw, ok := metadataStringField(value, field); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return fallback
}

func metadataBoolField(value Value, field string) bool {
	raw, ok := value.Fields[field]
	return ok && raw.Kind == ValueBool && raw.Bool
}

func metadataCustomFieldNames(fullName string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(fullName), ".", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(fullName)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func metadataCustomFieldType(item Value) (storage.FieldType, string) {
	raw := metadataTextFieldOrDefault(item, "type", "Text")
	switch strings.ToLower(strings.ReplaceAll(raw, "_", "")) {
	case "checkbox", "boolean":
		return storage.FieldBoolean, "BOOLEAN"
	case "number", "integer", "int":
		return storage.FieldInteger, "INTEGER"
	case "currency", "percent", "double", "decimal":
		return storage.FieldDecimal, "DOUBLE"
	case "date":
		return storage.FieldDate, "DATE"
	case "datetime":
		return storage.FieldDateTime, "DATETIME"
	case "picklist":
		return storage.FieldPicklist, "PICKLIST"
	case "multipicklist":
		return storage.FieldMultiPicklist, "MULTIPICKLIST"
	case "lookup", "masterdetail", "reference":
		return storage.FieldReference, "REFERENCE"
	case "textarea", "longtextarea", "html", "email", "phone", "url", "text":
		return storage.FieldString, "STRING"
	default:
		return storage.FieldString, strings.ToUpper(raw)
	}
}

func metadataReferenceTo(item Value) []string {
	raw, ok := item.Fields["referenceTo"]
	if !ok || raw.Kind == ValueNull {
		return nil
	}
	switch raw.Kind {
	case ValueString:
		if strings.TrimSpace(raw.Text) == "" {
			return nil
		}
		return []string{strings.TrimSpace(raw.Text)}
	case ValueList, ValueSet:
		items := raw.List
		if raw.Kind == ValueSet {
			items = raw.Set
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			if item.Kind == ValueString && strings.TrimSpace(item.Text) != "" {
				out = append(out, strings.TrimSpace(item.Text))
			}
		}
		return out
	default:
		return nil
	}
}

func metadataStringList(value Value) ([]string, error) {
	if value.Kind != ValueList && value.Kind != ValueSet {
		return nil, fmt.Errorf("Metadata.Operations.retrieve expects full names list")
	}
	items := value.List
	if value.Kind == ValueSet {
		items = value.Set
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Kind != ValueString {
			return nil, fmt.Errorf("Metadata.Operations.retrieve expects String full names")
		}
		out = append(out, item.Text)
	}
	return out, nil
}

func metadataCustomMetadataNames(fullName string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(fullName), ".", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(fullName)
	}
	objectName := strings.TrimSpace(parts[0])
	if !hasSuffixFold(objectName, "__mdt") {
		objectName += "__mdt"
	}
	return objectName, strings.TrimSpace(parts[1])
}

func metadataLabelOrDefault(item Value, developerName string) string {
	if label, ok := metadataStringField(item, "label"); ok && strings.TrimSpace(label) != "" {
		return label
	}
	return developerName
}

func metadataNamespacePrefix(namespace, objectName string) string {
	if namespace == "" {
		return ""
	}
	if strings.HasPrefix(objectName, namespace+"__") {
		return namespace
	}
	return ""
}

func metadataQualifiedAPIName(namespace, objectName, developerName string) string {
	if metadataNamespacePrefix(namespace, objectName) != "" {
		return namespace + "__" + developerName
	}
	return developerName
}

func nextMetadataRecordID(state storage.ObjectState) storage.ID {
	generator := storage.NewIDGenerator(map[string]string{state.Definition.APIName: state.Definition.KeyPrefix})
	for {
		id, err := generator.Next(state.Definition.APIName)
		if err != nil {
			return storage.ID(state.Definition.KeyPrefix + "000000000001")
		}
		if _, exists := state.Records[id]; !exists {
			return id
		}
	}
}

func namedEnumValues(typeName string, names []string, args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("%s.values expects 0 arguments", typeName)
	}
	values := make([]Value, 0, len(names))
	for i, name := range names {
		value := Value{Kind: ValueObject, Type: typeName, Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func loggingLevelValues(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("LoggingLevel.values expects 0 arguments")
	}
	values := make([]Value, 0, len(loggingLevelNames))
	for i, name := range loggingLevelNames {
		value := Value{Kind: ValueObject, Type: "LoggingLevel", Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func (vm *VM) callEnumStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	method = canonicalStdlibMemberName(method, "values", "valueOf")
	if method != "values" && method != "valueOf" {
		return Null, false, nil
	}
	if canonical, names, ok := coreEnumSpec(typeName); ok {
		value, err := callNamedEnumStaticMember(canonical, names, method, args)
		return value, true, err
	}
	if value, handled, err := vm.callGeneratedPlatformEnumStaticMember(typeName, method, args); handled || err != nil {
		return value, handled, err
	}
	if typeName == "LoggingLevel" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := loggingLevelValues(args)
		return value, true, err
	}
	if typeName == "RoundingMode" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := roundingModeValues(args)
		return value, true, err
	}
	if strings.EqualFold(typeName, "Schema.DisplayType") || strings.EqualFold(typeName, "DisplayType") {
		value, err := callNamedEnumStaticMember("Schema.DisplayType", schemaDisplayTypeNames, method, args)
		return value, true, err
	}
	if strings.EqualFold(typeName, "Schema.FieldDescribeOptions") || strings.EqualFold(typeName, "FieldDescribeOptions") {
		value, err := callNamedEnumStaticMember("Schema.FieldDescribeOptions", schemaFieldDescribeOptionNames, method, args)
		return value, true, err
	}
	if strings.EqualFold(typeName, "Schema.SObjectDescribeOptions") || strings.EqualFold(typeName, "SObjectDescribeOptions") {
		value, err := callNamedEnumStaticMember("Schema.SObjectDescribeOptions", schemaSObjectDescribeOptionNames, method, args)
		return value, true, err
	}
	if strings.EqualFold(typeName, "Schema.SOAPType") || strings.EqualFold(typeName, "SOAPType") {
		value, err := callNamedEnumStaticMember("Schema.SOAPType", schemaSOAPTypeNames, method, args)
		return value, true, err
	}
	if typeName == "Metadata.DeployStatus" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := metadataDeployStatusValues(args)
		return value, true, err
	}
	if typeName == "Metadata.MetadataType" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := metadataMetadataTypeValues(args)
		return value, true, err
	}
	class, ok := vm.resolveEnumClass(typeName)
	if !ok || len(class.EnumValues) == 0 {
		return Null, false, nil
	}
	if err := vm.ensureClassInitialized(class.Name); err != nil {
		return Null, true, err
	}
	class, _ = vm.lookupClass(class.Name)
	switch method {
	case "values":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.values expects 0 arguments", typeName)
		}
		values := make([]Value, 0, len(class.EnumValues))
		for i, name := range class.EnumValues {
			value := Value{Kind: ValueObject, Type: class.Name, Text: name}
			value.Fields = map[string]Value{"ordinal": Int(int64(i))}
			values = append(values, value)
		}
		return List(values...), true, nil
	case "valueOf":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.valueOf expects String", typeName)
		}
		argText, ok := stringLikeValueText(args[0])
		if !ok {
			return Null, true, newExceptionError("TypeException", fmt.Sprintf("%s.valueOf expects String", typeName))
		}
		for i, name := range class.EnumValues {
			if strings.EqualFold(name, argText) {
				value := Value{Kind: ValueObject, Type: class.Name, Text: name}
				value.Fields = map[string]Value{"ordinal": Int(int64(i))}
				return value, true, nil
			}
		}
		return Null, true, newExceptionError("NoSuchElementException", fmt.Sprintf("No enum value found called %s", argText))
	default:
		return Null, false, nil
	}
}

func callNamedEnumStaticMember(typeName string, names []string, method string, args []Value) (Value, error) {
	switch method {
	case "values":
		return namedEnumValues(typeName, names, args)
	case "valueOf":
		if len(args) != 1 {
			return Null, fmt.Errorf("%s.valueOf expects String", typeName)
		}
		argText, ok := stringLikeValueText(args[0])
		if !ok {
			return Null, newExceptionError("TypeException", fmt.Sprintf("%s.valueOf expects String", typeName))
		}
		if value, ok := namedEnumStaticValue(typeName, names, typeName+"."+argText); ok {
			return value, nil
		}
		return Null, newExceptionError("NoSuchElementException", fmt.Sprintf("No enum value found called %s", argText))
	default:
		return Null, fmt.Errorf("%s.%s is not supported", typeName, method)
	}
}

func (vm *VM) callGeneratedPlatformEnumStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	generated, ok := generatedPlatformTypes()[strings.ToLower(typeName)]
	if !ok || generated.Kind != apexast.DeclarationEnum {
		return Null, false, nil
	}
	names := generatedPlatformEnumNames(generated)
	if len(names) == 0 {
		return Null, false, nil
	}
	switch method {
	case "values":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.values expects 0 arguments", generated.Name)
		}
		values := make([]Value, 0, len(names))
		for i, name := range names {
			value := Value{Kind: ValueObject, Type: generated.Name, Text: name}
			value.Fields = map[string]Value{"ordinal": Int(int64(i))}
			values = append(values, value)
		}
		return List(values...), true, nil
	case "valueOf":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.valueOf expects String", generated.Name)
		}
		argText, ok := stringLikeValueText(args[0])
		if !ok {
			return Null, true, newExceptionError("TypeException", fmt.Sprintf("%s.valueOf expects String", generated.Name))
		}
		for i, name := range names {
			if strings.EqualFold(name, argText) {
				value := Value{Kind: ValueObject, Type: generated.Name, Text: name}
				value.Fields = map[string]Value{"ordinal": Int(int64(i))}
				return value, true, nil
			}
		}
		return Null, true, newExceptionError("NoSuchElementException", fmt.Sprintf("No enum value found called %s", argText))
	default:
		return Null, false, nil
	}
}

func generatedPlatformEnumNames(generated generatedPlatformType) []string {
	names := make([]string, 0, len(generated.StaticFields))
	seen := make(map[string]bool, len(generated.StaticFields))
	for _, name := range generated.StaticFieldOrder {
		field, ok := generated.StaticFields[name]
		if !ok {
			continue
		}
		if field.Type == "" || strings.EqualFold(field.Type, generated.Name) {
			names = append(names, name)
			seen[strings.ToLower(name)] = true
		}
	}
	remaining := make([]string, 0)
	for name, field := range generated.StaticFields {
		if seen[strings.ToLower(name)] {
			continue
		}
		if field.Type == "" || strings.EqualFold(field.Type, generated.Name) {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	names = append(names, remaining...)
	return names
}

func stringLikeValueText(value Value) (string, bool) {
	if value.Kind == ValueString {
		return value.Text, true
	}
	if value.Kind == ValueObject && value.Text != "" {
		return value.Text, true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "String") {
		if text, ok := platformScalarObjectText(value); ok {
			return text, true
		}
	}
	return "", false
}

func generatedPlatformEnumOrdinal(generated generatedPlatformType, value string) int {
	for i, name := range generatedPlatformEnumNames(generated) {
		if strings.EqualFold(name, value) {
			return i
		}
	}
	return -1
}

func roundingModeValues(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("RoundingMode.values expects 0 arguments")
	}
	values := make([]Value, 0, len(roundingModeNames))
	for i, name := range roundingModeNames {
		value := Value{Kind: ValueObject, Type: "RoundingMode", Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func (vm *VM) callEnumMember(receiver Value, method string, args []Value) (Value, bool, error) {
	method = canonicalStdlibMemberName(method, "equals", "hashCode", "name", "ordinal", "toString")
	receiverType := receiver.Type
	if rest, ok := stripLeadingSystemNamespace(receiverType); ok {
		receiverType = rest
	}
	if canonical, names, ok := coreEnumSpec(receiverType); ok {
		return callNamedEnumMember(canonical, names, receiver, method, args)
	}
	if receiverType == "JSONToken" {
		if method == "equals" {
			if len(args) != 1 {
				return Null, true, fmt.Errorf("JSONToken.equals expects 1 argument")
			}
			return Bool(enumValuesEqual(receiver, args[0])), true, nil
		}
		if len(args) != 0 {
			return Null, true, fmt.Errorf("JSONToken.%s expects 0 arguments", method)
		}
		switch method {
		case "name", "toString":
			return String(receiver.Text), true, nil
		case "ordinal":
			for i, name := range jsonTokenNames {
				if name == receiver.Text {
					return Int(int64(i)), true, nil
				}
			}
			return Int(-1), true, nil
		default:
			return Null, false, nil
		}
	}
	if generated, ok := generatedPlatformTypes()[strings.ToLower(receiverType)]; ok && generated.Kind == apexast.DeclarationEnum && generated.EnumHashBase != nil {
		return callGeneratedPlatformEnumMember(generated, receiver, method, args)
	}
	if receiver.Type == "ApexPages.Severity" {
		return callNamedEnumMember("ApexPages.Severity", apexPagesSeverityNames, receiver, method, args)
	}
	if receiverType == "LoggingLevel" {
		return callNamedEnumMember("LoggingLevel", loggingLevelNames, receiver, method, args)
	}
	if receiverType == "RoundingMode" {
		return callNamedEnumMember("RoundingMode", roundingModeNames, receiver, method, args)
	}
	if receiverType == "AccessType" {
		return callNamedEnumMember("AccessType", accessTypeNames, receiver, method, args)
	}
	if receiverType == "TriggerOperation" {
		return callNamedEnumMember("TriggerOperation", triggerOperationNames, receiver, method, args)
	}
	if receiverType == "StatusCode" {
		return callStatusCodeMember(receiver, method, args)
	}
	if receiver.Type == "Schema.DisplayType" {
		return callNamedEnumMember("Schema.DisplayType", schemaDisplayTypeNames, receiver, method, args)
	}
	if receiver.Type == "Schema.FieldDescribeOptions" {
		return callNamedEnumMember("Schema.FieldDescribeOptions", schemaFieldDescribeOptionNames, receiver, method, args)
	}
	if receiver.Type == "Schema.SObjectDescribeOptions" || receiver.Type == "SObjectDescribeOptions" {
		return callNamedEnumMember("Schema.SObjectDescribeOptions", schemaSObjectDescribeOptionNames, receiver, method, args)
	}
	if receiver.Type == "Schema.SOAPType" {
		return callNamedEnumMember("Schema.SOAPType", schemaSOAPTypeNames, receiver, method, args)
	}
	if receiver.Type == "Metadata.DeployStatus" {
		return callNamedEnumMember("Metadata.DeployStatus", metadataDeployStatusNames, receiver, method, args)
	}
	if receiver.Type == "Metadata.MetadataType" {
		return callNamedEnumMember("Metadata.MetadataType", metadataMetadataTypeNames, receiver, method, args)
	}
	if generated, ok := generatedPlatformTypes()[strings.ToLower(receiver.Type)]; ok && generated.Kind == apexast.DeclarationEnum {
		return callGeneratedPlatformEnumMember(generated, receiver, method, args)
	}
	class, ok := vm.resolveEnumClass(receiver.Type)
	if !ok || len(class.EnumValues) == 0 {
		if receiver.Text != "" && looksManagedQualifiedType(receiver.Type) {
			return callManagedEnumMember(receiver, method, args)
		}
		return Null, false, nil
	}
	if method == "equals" {
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.equals expects 1 argument", receiver.Type)
		}
		equal, _ := vm.resolvedEnumValuesEqual(receiver, args[0])
		return Bool(equal), true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		for i, name := range class.EnumValues {
			if name == receiver.Text {
				return Int(int64(i)), true, nil
			}
		}
		return Int(-1), true, nil
	default:
		return Null, false, nil
	}
}

func callManagedEnumMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if method == "equals" {
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.equals expects 1 argument", receiver.Type)
		}
		return Bool(enumValuesEqual(receiver, args[0])), true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		if ordinal, ok := receiver.Fields["ordinal"]; ok && ordinal.Kind == ValueInt {
			return ordinal, true, nil
		}
		return Int(-1), true, nil
	default:
		return Null, false, nil
	}
}

func callGeneratedPlatformEnumMember(generated generatedPlatformType, receiver Value, method string, args []Value) (Value, bool, error) {
	method = canonicalStdlibMemberName(method, "equals", "hashCode", "name", "ordinal", "toString")
	if method == "equals" {
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.equals expects 1 argument", receiver.Type)
		}
		return Bool(enumValuesEqual(receiver, args[0])), true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		return Int(int64(generatedPlatformEnumOrdinal(generated, receiver.Text))), true, nil
	case "hashCode":
		if generated.EnumHashBase == nil {
			return Null, false, nil
		}
		ordinal := generatedPlatformEnumOrdinal(generated, receiver.Text)
		if ordinal < 0 {
			return Int(-1), true, nil
		}
		return Int(*generated.EnumHashBase + int64(ordinal)), true, nil
	default:
		return Null, false, nil
	}
}

func enumValuesEqual(left, right Value) bool {
	if right.Kind != ValueObject {
		return false
	}
	return (strings.EqualFold(left.Type, right.Type) || namespaceQualifiedTypeEquivalent(left.Type, right.Type)) && left.Text == right.Text
}

func (vm *VM) resolvedEnumValuesEqual(left, right Value) (bool, bool) {
	if left.Kind != ValueObject || right.Kind != ValueObject || left.Text == "" || right.Text == "" {
		return false, false
	}
	leftClass, leftOK := vm.resolveEnumClass(left.Type)
	rightClass, rightOK := vm.resolveEnumClass(right.Type)
	if leftOK || rightOK {
		if !leftOK || !rightOK {
			return false, true
		}
		return strings.EqualFold(leftClass.Name, rightClass.Name) && left.Text == right.Text, true
	}
	if strings.EqualFold(left.Type, right.Type) || namespaceQualifiedTypeEquivalent(left.Type, right.Type) {
		return left.Text == right.Text, true
	}
	return false, false
}

func metadataDeployDetailsObject() Value {
	details := Object("Metadata.DeployDetails")
	details.Fields["componentFailures"] = typedList("List<Metadata.DeployMessage>")
	details.Fields["componentSuccesses"] = typedList("List<Metadata.DeployMessage>")
	details.Fields["runTestResult"] = Null
	return details
}

func metadataDeployResultObject(deploymentID string, items []Value) Value {
	result := Object("Metadata.DeployResult")
	result.Fields["id"] = platformScalar("Id", deploymentID)
	result.Fields["status"] = metadataDeployStatusValue("SUCCEEDED")
	result.Fields["success"] = Bool(true)
	result.Fields["done"] = Bool(true)
	result.Fields["numberComponentErrors"] = Int(0)
	result.Fields["numberComponentsDeployed"] = Int(int64(len(items)))
	result.Fields["numberComponentsTotal"] = Int(int64(len(items)))
	result.Fields["numberTestErrors"] = Int(0)
	result.Fields["numberTestsCompleted"] = Int(0)
	result.Fields["checkOnly"] = Bool(false)
	result.Fields["messages"] = List()
	details := metadataDeployDetailsObject()
	successes := make([]Value, 0, len(items))
	for _, item := range items {
		successes = append(successes, metadataDeploySuccessMessage(item))
	}
	details.Fields["componentSuccesses"] = List(successes...)
	result.Fields["details"] = details
	return result
}

func metadataDeployFailureResultObject(deploymentID string, items []Value, failedItem Value, err error) Value {
	result := Object("Metadata.DeployResult")
	result.Fields["id"] = platformScalar("Id", deploymentID)
	result.Fields["status"] = metadataDeployStatusValue("FAILED")
	result.Fields["success"] = Bool(false)
	result.Fields["done"] = Bool(true)
	result.Fields["numberComponentErrors"] = Int(1)
	result.Fields["numberComponentsDeployed"] = Int(0)
	result.Fields["numberComponentsTotal"] = Int(int64(len(items)))
	result.Fields["numberTestErrors"] = Int(0)
	result.Fields["numberTestsCompleted"] = Int(0)
	result.Fields["checkOnly"] = Bool(false)
	result.Fields["messages"] = List()
	details := metadataDeployDetailsObject()
	details.Fields["componentFailures"] = List(metadataDeployFailureMessage(failedItem, err))
	result.Fields["details"] = details
	return result
}

func metadataDeploySuccessMessage(item Value) Value {
	message := Object("Metadata.DeployMessage")
	fullName := metadataDeployItemFullName(item)
	message.Fields["fullName"] = String(fullName)
	message.Fields["fileName"] = String(fullName)
	message.Fields["componentType"] = String(metadataDeployItemComponentType(item))
	message.Fields["success"] = Bool(true)
	message.Fields["problem"] = Null
	return message
}

func metadataDeployFailureMessage(item Value, err error) Value {
	message := Object("Metadata.DeployMessage")
	fullName := metadataDeployItemFullName(item)
	message.Fields["fullName"] = String(fullName)
	message.Fields["fileName"] = String(fullName)
	message.Fields["componentType"] = String(metadataDeployItemComponentType(item))
	message.Fields["success"] = Bool(false)
	if err == nil {
		message.Fields["problem"] = String("metadata deployment failed")
	} else {
		message.Fields["problem"] = String(err.Error())
	}
	return message
}

func metadataDeployItemFullName(item Value) string {
	if item.Kind == ValueObject {
		if fullName, ok := metadataStringField(item, "fullName"); ok {
			return fullName
		}
	}
	return ""
}

func metadataDeployItemComponentType(item Value) string {
	if item.Kind != ValueObject {
		return string(item.Kind)
	}
	switch item.Type {
	case "Metadata.CustomMetadata":
		return "CustomMetadata"
	case "":
		return string(item.Kind)
	default:
		return strings.TrimPrefix(item.Type, "Metadata.")
	}
}

func metadataAsyncResultObject(id string, done bool, state, message string) Value {
	result := Object("Metadata.AsyncResult")
	result.Fields["id"] = platformScalar("Id", id)
	result.Fields["done"] = Bool(done)
	result.Fields["state"] = String(state)
	result.Fields["statusCode"] = Null
	if message == "" {
		result.Fields["message"] = Null
	} else {
		result.Fields["message"] = String(message)
	}
	return result
}

func metadataDeployStatusValue(name string) Value {
	return Value{Kind: ValueObject, Type: "Metadata.DeployStatus", Text: name, Fields: map[string]Value{"ordinal": Int(metadataDeployStatusOrdinal(name))}}
}

func metadataDeployStatusOrdinal(name string) int64 {
	for i, candidate := range metadataDeployStatusNames {
		if candidate == name {
			return int64(i)
		}
	}
	return -1
}

func cloneMetadataDeployResult(result Value) Value {
	cloned := cloneValue(result)
	if cloned.Fields == nil {
		cloned.Fields = make(map[string]Value)
	}
	return cloned
}

func callNamedEnumMember(typeName string, names []string, receiver Value, method string, args []Value) (Value, bool, error) {
	if method == "equals" {
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.equals expects 1 argument", typeName)
		}
		return Bool(enumValuesEqual(receiver, args[0])), true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		for i, name := range names {
			if name == receiver.Text {
				return Int(int64(i)), true, nil
			}
		}
		return Int(-1), true, nil
	case "hashCode":
		return Int(int64(javaStringHashCode(typeName + "." + receiver.Text))), true, nil
	default:
		return Null, false, nil
	}
}

func callStatusCodeMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if method == "equals" {
		if len(args) != 1 {
			return Null, true, fmt.Errorf("StatusCode.equals expects 1 argument")
		}
		return Bool(enumValuesEqual(receiver, args[0])), true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("StatusCode.%s expects 0 arguments", method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		return Int(0), true, nil
	default:
		return Null, false, nil
	}
}
