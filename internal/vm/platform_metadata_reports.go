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

// Metadata enum declaration order differs from stub member order.
func metadataEnumSpec(typeName string) (string, []string, bool) {
	var canonical, names string
	switch strings.ToLower(typeName) {
	case "metadata.deploystatus":
		return "Metadata.DeployStatus", metadataDeployStatusNames, true
	case "metadata.metadatatype":
		return "Metadata.MetadataType", metadataMetadataTypeNames, true
	case "metadata.deployproblemtype":
		canonical, names = "Metadata.DeployProblemType", "Warning|Error|Info"
	case "metadata.uibehavior":
		canonical, names = "Metadata.UiBehavior", "Edit|Required|Readonly"
	case "metadata.layoutsectionstyle":
		canonical, names = "Metadata.LayoutSectionStyle", "TwoColumnsTopToBottom|TwoColumnsLeftToRight|OneColumn|CustomLinks"
	case "metadata.sortorder":
		canonical, names = "Metadata.SortOrder", "Asc_x|Desc_x"
	case "metadata.reportchartcomponentsize":
		canonical, names = "Metadata.ReportChartComponentSize", "SMALL|MEDIUM|LARGE"
	case "metadata.summarylayoutstyleenum":
		canonical, names = "Metadata.SummaryLayoutStyleEnum", "Default_x|QuoteTemplate|DefaultQuoteTemplate|ServiceReportTemplate|ChildServiceReportTemplateStyle|DefaultServiceReportTemplate|CaseInteraction|QuickActionLayoutLeftRight|QuickActionLayoutTopDown|PathAssistant"
	case "metadata.feedlayoutcomponenttype":
		canonical, names = "Metadata.FeedLayoutComponentType", "HelpAndToolLinks|CustomButtons|Following|Followers|CustomLinks|Milestones|SimilarCases|CaseExperts|Topics|CaseUnifiedFiles|Visualforce"
	case "metadata.feedlayoutfilterposition":
		canonical, names = "Metadata.FeedLayoutFilterPosition", "CenterDropDown|LeftFixed|LeftFloat"
	case "metadata.feedlayoutfiltertype":
		canonical, names = "Metadata.FeedLayoutFilterType", "AllUpdates|FeedItemType|Custom"
	case "metadata.feeditemtypeenum":
		canonical, names = "Metadata.FeedItemTypeEnum", "TrackedChange|UserStatus|TextPost|AdvancedTextPost|LinkPost|ContentPost|DashboardComponentAlert|PollPost|RypplePost|ProfileSkillPost|DashboardComponentSnapshot|TestItem|ApprovalPost|CaseCommentPost|ReplyPost|EmailMessageEvent|CallLogPost|ChangeStatusPost|AttachArticleEvent|MilestoneEvent|ActivityEvent|ChatTranscriptPost|CollaborationGroupCreated|AttachExternalDocumentEvent|CollaborationGroupUnarchived|SocialPost|QuestionPost|Undefined|FacebookPost|BasicTemplateFeedItem|CreateRecordEvent|CanvasPost|AnnouncementPost"
	case "metadata.layoutheader":
		canonical, names = "Metadata.LayoutHeader", "PersonalTagging|PublicTagging"
	default:
		return "", nil, false
	}
	return canonical, strings.Split(names, "|"), true
}

// E001-E006/E011-E034 capture every admitted DTO below. Keep the defaults
// of neighbouring local-only Metadata mocks outside this measured set.
func metadataCapturedDTOType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "metadata.custommetadata", "metadata.custommetadatavalue",
		"metadata.layout", "metadata.layoutcolumn", "metadata.layoutitem", "metadata.layoutsection",
		"metadata.minilayout", "metadata.quickactionlist", "metadata.quickactionlistitem",
		"metadata.relatedcontent", "metadata.relatedcontentitem", "metadata.relatedlist", "metadata.relatedlistitem",
		"metadata.reportchartcomponentlayoutitem", "metadata.analyticscloudcomponentlayoutitem",
		"metadata.sidebarcomponent", "metadata.container", "metadata.customconsolecomponents",
		"metadata.primarytabcomponents", "metadata.subtabcomponents", "metadata.summarylayout", "metadata.summarylayoutitem",
		"metadata.feedlayout", "metadata.feedlayoutfilter", "metadata.feedlayoutcomponent",
		"metadata.platformactionlist", "metadata.platformactionlistitem",
		"metadata.deploymessage", "metadata.deploydetails", "metadata.deployresult":
		return true
	default:
		return false
	}
}

// Collection members are initialized;
// scalar and nested DTO members remain raw null rather than synthetic values.
func metadataDTOFieldDefault(typeName string) Value {
	switch collectionBase(typeName) {
	case "List":
		return typedList(typeName)
	case "Set":
		value := Set()
		value.Type = typeName
		return value
	}
	if isMapType(typeName) {
		return typedMap(typeName)
	}
	return Null
}

// Only the DTO is copied; collection and child identities survive.
func cloneMetadataDTO(receiver Value) Value {
	copy := receiver
	copy.Ref = newValueRef()
	copy.Fields = make(map[string]Value, len(receiver.Fields))
	for name, value := range receiver.Fields {
		copy.Fields[name] = value
	}
	return copy
}

func soapTypeForStorageField(field storage.Field) string {
	if strings.EqualFold(field.DisplayType, "ADDRESS") {
		return "ADDRESS"
	}
	switch field.Type {
	case storage.FieldAddress:
		return "ADDRESS"
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
	case storage.FieldTime:
		return "TIME"
	case storage.FieldBlob:
		return "BASE64BINARY"
	default:
		return "STRING"
	}
}

var schemaSOAPTypeNames = []string{"ID", "STRING", "BOOLEAN", "INTEGER", "DOUBLE", "DATE", "DATETIME", "TIME", "BASE64BINARY", "ANYTYPE", "ADDRESS"}

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
	// Salesforce exposes Blob fields through Schema.DisplayType.BASE64. The
	// storage catalog uses BLOB as its internal field type, so normalize that
	// spelling at the platform boundary.
	if strings.EqualFold(strings.TrimSpace(name), "BLOB") {
		name = "BASE64"
	}
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

// R223-R244: invalid inputs fail before the hosted Reports service boundary.
// Valid report/instance identifiers cannot manufacture local service results.
func reportsReportBoundary(callee string, reportID Value) error {
	if reportID.Kind == ValueNull || !strings.HasPrefix(scalarText(reportID), "00O") {
		return newExceptionError("System.NoDataFoundException", "The data you’re trying to access is unavailable.")
	}
	return newExceptionError("System.UnsupportedOperationException", callee+" requires the hosted Reports service")
}

func (vm *VM) reportsDescribeReport(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("reports.ReportManager.describeReport expects report Id")
	}
	return Null, reportsReportBoundary("reports.ReportManager.describeReport", args[0])
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
	if args[0].Kind == ValueNull {
		return Null, newExceptionError("reports.ReportRunException", "We ran into an error when running this report. Try to re-submit your query.")
	}
	if !strings.HasPrefix(scalarText(args[0]), "0LG") {
		return Null, newExceptionError("System.NoDataFoundException", "The instance you requested does not exist.")
	}
	return Null, newExceptionError("System.UnsupportedOperationException", "reports.ReportManager.getReportInstance requires the hosted Reports service")
}

func (vm *VM) reportsGetReportInstances(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("reports.ReportManager.getReportInstances expects report Id")
	}
	return Null, reportsReportBoundary("reports.ReportManager.getReportInstances", args[0])
}

func (vm *VM) reportsRunAsyncReport(args []Value) (Value, error) {
	reportID, _, _, err := vm.reportsReportArgs(args, "reports.ReportManager.runAsyncReport")
	if err != nil {
		return Null, err
	}
	return Null, reportsReportBoundary("reports.ReportManager.runAsyncReport", reportID)
}

func (vm *VM) reportsRunReport(args []Value) (Value, error) {
	reportID, _, _, err := vm.reportsReportArgs(args, "reports.ReportManager.runReport")
	if err != nil {
		return Null, err
	}
	return Null, reportsReportBoundary("reports.ReportManager.runReport", reportID)
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
		case arg.Kind == ValueNull, arg.Kind == ValueObject && strings.EqualFold(arg.Type, "reports.ReportMetadata"):
			metadata = arg
		default:
			return Null, Null, false, fmt.Errorf("%s expects report Id[, ReportMetadata][, includeDetails]", callee)
		}
	}
	return reportID, metadata, includeDetails, nil
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

func (vm *VM) flowInterviewCreate(args []Value) (Value, error) {
	if len(args) != 2 && len(args) != 3 {
		return Null, fmt.Errorf("Flow.Interview.createInterview expects flow name and input variables")
	}
	offset := 0
	if len(args) == 3 {
		if args[0].Kind != ValueString && args[0].Kind != ValueNull {
			return Null, fmt.Errorf("Flow.Interview.createInterview expects namespace String")
		}
		offset = 1
	}
	if args[offset].Kind == ValueNull {
		return Null, newExceptionError("System.NullPointerException", "Argument flowName cannot be null")
	}
	if args[offset+1].Kind == ValueNull {
		return Null, newExceptionError("System.NullPointerException", "Argument initialValues cannot be null")
	}
	if args[offset].Kind != ValueString || args[offset+1].Kind != ValueMap {
		return Null, fmt.Errorf("Flow.Interview.createInterview expects flow name and input variables")
	}
	name := args[offset].Text
	if offset == 1 && args[0].Kind == ValueString && args[0].Text != "" {
		name = args[0].Text + "." + name
	}
	if _, found := vm.autolaunchedFlowRule(name); !found {
		return Null, newExceptionError("System.TypeException", "Invalid type: "+name)
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

func (vm *VM) metadataDeploymentTestRestriction() error {
	if vm.testContext != nil {
		return newExceptionError("System.AsyncException", "Metadata cannot be deployed from within a test")
	}
	return nil
}

func (vm *VM) metadataEnqueueDeployment(args []Value, result *Result) (Value, error) {
	if err := vm.metadataDeploymentTestRestriction(); err != nil {
		return Null, err
	}
	if len(args) == 2 && args[0].Kind == ValueNull {
		return Null, newExceptionError("System.NullPointerException", "Deploy container is null")
	}

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
	// Malformed requests fail before reaching the hosted boundary.
	if args[0].Kind == ValueNull || args[1].Kind == ValueNull {
		typeName := "null"
		if args[0].Kind != ValueNull {
			typeName = args[0].Text
		}
		return Null, newExceptionError("System.TypeException", "Error retrieving metadata for entities of type: "+typeName+". Error message: type and fullNames must be specified for items to read")
	}
	if args[0].Kind != ValueObject || args[0].Type != "Metadata.MetadataType" {
		return Null, fmt.Errorf("Metadata.Operations.retrieve expects metadata type")
	}
	metadataType := strings.TrimSpace(args[0].Text)
	if !strings.EqualFold(metadataType, "CustomMetadata") && !strings.EqualFold(metadataType, "Layout") {
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
		if strings.EqualFold(metadataType, "Layout") {
			if layout := vm.metadataLayoutObject(fullName); layout.Kind == ValueObject {
				out = append(out, layout)
			}
			continue
		}
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
	result := List(out...)
	result.Type = "List<Metadata.Metadata>"
	return result, nil
}

// metadataLayoutObject provides the local shape returned by Metadata.retrieve
// for a layout. The runner has schema fields but no deployable layout store, so
// expose a deterministic single-section layout derived from the target object.
func (vm *VM) metadataLayoutObject(fullName string) Value {
	objectName, _, ok := strings.Cut(strings.TrimSpace(fullName), "-")
	if !ok || strings.TrimSpace(objectName) == "" || vm == nil || vm.Org == nil {
		return Null
	}
	resolved, ok := vm.resolveObjectName(strings.TrimSpace(objectName))
	if !ok {
		return Null
	}
	state, ok := vm.Org.Objects[resolved]
	if !ok {
		return Null
	}
	fieldNames := make([]string, 0, len(state.Definition.Fields))
	for name, field := range state.Definition.Fields {
		if strings.TrimSpace(field.APIName) != "" {
			name = field.APIName
		}
		if strings.TrimSpace(name) != "" {
			fieldNames = append(fieldNames, name)
		}
	}
	sort.Strings(fieldNames)
	items := make([]Value, 0, len(fieldNames))
	for _, fieldName := range fieldNames {
		item := Object("Metadata.LayoutItem")
		item.Fields["field"] = String(fieldName)
		items = append(items, item)
	}
	column := Object("Metadata.LayoutColumn")
	column.Fields["layoutItems"] = List(items...)
	section := Object("Metadata.LayoutSection")
	section.Fields["layoutColumns"] = List(column)
	layout := Object("Metadata.Layout")
	layout.Fields["layoutSections"] = List(section)
	return layout
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
	// R018: values() on an unqualified top-level project enum shadows a
	// platform enum of that name. Only values() is captured; valueOf() keeps
	// the platform-first order. Nested and dependency enums are unchanged.
	if method == "values" && !strings.Contains(typeName, ".") {
		if class, ok := vm.resolveEnumClass(typeName); ok && !class.Dependency && strings.EqualFold(class.Name, typeName) {
			return vm.callProjectEnumStaticMember(class, typeName, method, args)
		}
	}
	if canonical, names, ok := coreEnumSpec(typeName); ok {
		if canonical == "AccessType" && method == "valueOf" && len(args) == 1 && args[0].Kind == ValueNull {
			return Null, true, newExceptionError("NoSuchElementException", "No enum value found called null")
		}
		value, err := callNamedEnumStaticMember(canonical, names, method, args)
		return value, true, err
	}
	if canonical, names, ok := metadataEnumSpec(typeName); ok {
		if canonical == "Metadata.DeployStatus" && method == "valueOf" && len(args) == 1 && args[0].Kind == ValueNull {
			return Null, true, newExceptionError("System.NoSuchElementException", "No enum value found called null")
		}
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
	return vm.callProjectEnumStaticMember(class, typeName, method, args)
}

func (vm *VM) callProjectEnumStaticMember(class Class, typeName, method string, args []Value) (Value, bool, error) {
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
		// The values() result is read-only; copies remain ordinary lists.
		list := List(values...)
		list.Fields = map[string]Value{"__enum_values_readonly": Bool(true)}
		return list, true, nil
	case "valueOf":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.valueOf expects String", typeName)
		}
		if args[0].Kind == ValueNull {
			return Null, true, newExceptionError("NoSuchElementException", "No enum value found called null")
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
	// Other enums retain their own null conversion behavior.
	if typeName == "TriggerOperation" && method == "valueOf" && len(args) == 1 && args[0].Kind == ValueNull {
		return Null, newExceptionError("NoSuchElementException", "No enum value found called null")
	}
	switch method {
	case "values":
		return namedEnumValues(typeName, names, args)
	case "valueOf":
		if len(args) != 1 {
			return Null, fmt.Errorf("%s.valueOf expects String", typeName)
		}
		// Leave other enums' null conversion unchanged.
		if typeName == "LoggingLevel" && args[0].Kind == ValueNull {
			return Null, newExceptionError("NoSuchElementException", "No enum value found called null")
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
	// Captured native order differs from the alphabetized stubs.
	if strings.EqualFold(generated.Name, "Messaging.AttachmentRetrievalOption") {
		return []string{"NONE", "METADATA_ONLY", "METADATA_WITH_BODY"}
	}
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
	if strings.EqualFold(receiverType, "AccessLevel") && strings.EqualFold(method, "withPermissionSetId") {
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull) {
			return Null, true, fmt.Errorf("AccessLevel.withPermissionSetId expects String")
		}
		value := accessLevelClone(receiver)
		value.Fields["permissionSetId"] = cloneValue(args[0])
		if args[0].Kind == ValueNull {
			value.Fields["currentAccessPermissions"] = String(strings.ToUpper(strings.TrimSpace(receiver.Text)))
		} else {
			value.Fields["currentAccessPermissions"] = String("CUSTOM")
		}
		return value, true, nil
	}
	if canonical, names, ok := coreEnumSpec(receiverType); ok {
		return callNamedEnumMember(canonical, names, receiver, method, args)
	}
	if canonical, names, ok := metadataEnumSpec(receiverType); ok {
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
			return Int(int64(javaStringHashCode(generated.Name + "." + receiver.Text))), true, nil
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
	// runTestResult is not an admitted Apex DTO member.
	return details
}

func metadataDeployMessageObject() Value {
	message := Object("Metadata.DeployMessage")
	message.Fields["changed"] = Null
	message.Fields["columnNumber"] = Null
	message.Fields["componentType"] = Null
	message.Fields["created"] = Null
	message.Fields["createdDate"] = Null
	message.Fields["deleted"] = Null
	message.Fields["fileName"] = Null
	message.Fields["fullName"] = Null
	message.Fields["id"] = Null
	message.Fields["lineNumber"] = Null
	message.Fields["problem"] = Null
	message.Fields["problemType"] = Null
	message.Fields["success"] = Null
	return message
}

func metadataDeployResultConstructorObject() Value {
	result := Object("Metadata.DeployResult")
	// E002 captures every admitted field, including the raw-null details member.
	for name := range generatedPlatformTypes()["metadata.deployresult"].Fields {
		result.Fields[name] = Null
	}
	result.Fields["messages"] = List()
	return result
}

func metadataDeployResultObject(deploymentID string, items []Value) Value {
	result := Object("Metadata.DeployResult")
	result.Fields["id"] = platformScalar("Id", deploymentID)
	result.Fields["status"] = metadataDeployStatusValue("Succeeded")
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
	result.Fields["status"] = metadataDeployStatusValue("Failed")
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
