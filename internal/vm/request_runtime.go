package vm

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) salesforceBaseURL() string {
	if vm.serverBaseURL != "" {
		return vm.serverBaseURL
	}
	return "https://local.glade.example"
}

func (vm *VM) currentRequestURL() string {
	page := vm.currentPage
	if vm.currentPage.Kind == "" {
		page = newPageReference("/apex/current")
	}
	raw := pageReferenceURL(page).String()
	if raw == "" {
		return vm.salesforceBaseURL()
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	if strings.HasPrefix(raw, "/") {
		return vm.salesforceBaseURL() + raw
	}
	return vm.salesforceBaseURL() + "/" + raw
}

func (vm *VM) fileFieldURL(objectID, fieldName Value) string {
	query := url.Values{}
	query.Set("id", stringValueOrEmpty(objectID))
	query.Set("field", stringValueOrEmpty(fieldName))
	return strings.TrimRight(vm.salesforceBaseURL(), "/") + "/servlet/servlet.FileDownload?" + query.Encode()
}

func (vm *VM) currentRequestValue() Value {
	request := Object("Request")
	request.Fields["requestId"] = String("glade-request-000000000001")
	request.Fields["quiddity"] = vm.currentQuiddityValue()
	return request
}

func (vm *VM) currentUIRequestValue() Value {
	request := Object("UIRequest")
	headers := typedMap("Map<String,String>")
	headers.Map[mapKey(String("host"))] = String(strings.TrimPrefix(strings.TrimPrefix(vm.salesforceBaseURL(), "https://"), "http://"))
	headers.MapKeys[mapKey(String("host"))] = String("host")
	request.Fields["headers"] = headers
	return request
}

func (vm *VM) currentQuiddityValue() Value {
	name := "SYNCHRONOUS"
	if vm.testContext != nil {
		name = "RUNTEST_SYNC"
	}
	value := Value{Kind: ValueObject, Type: "Quiddity", Text: name, Ref: newValueRef()}
	value.Fields = map[string]Value{"ordinal": Int(0)}
	return value
}

func quiddityShortCode(name string) string {
	switch name {
	case "SYNCHRONOUS":
		return "R"
	case "RUNTEST_SYNC":
		return "RT"
	case "QUEUEABLE":
		return "QU"
	case "BATCH_APEX", "BATCHAPEX":
		return "BA"
	case "FUTURE":
		return "FU"
	case "SCHEDULED_APEX", "SCHEDULEDAPEX":
		return "SA"
	case "FUNCTION_CALLBACK":
		return "FC"
	default:
		return "SY"
	}
}

func (vm *VM) requestVersionValue() Value {
	version := storage.DefaultRESTAPIVersion
	if vm != nil && vm.Org != nil {
		version = storage.EffectiveRESTAPIVersion(vm.Org.APIVersion)
	}
	major, minor := parseMajorMinorVersion(version)
	out := Object("Version")
	out.Fields["major"] = Int(int64(major))
	out.Fields["minor"] = Int(int64(minor))
	out.Fields["patch"] = Int(0)
	out.Fields["__gladePatchSpecified"] = Bool(true)
	return out
}

func parseMajorMinorVersion(version string) (int, int) {
	parts := strings.Split(storage.NormalizeRESTAPIVersion(version), ".")
	major, minor := 0, 0
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major, minor
}

func (vm *VM) callRequestMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "getRequestId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Request.getRequestId expects 0 arguments")
		}
		if value, ok := receiver.Fields["requestId"]; ok && value.Kind == ValueString {
			return value, receiver, false, true, nil
		}
		return String("glade-request-000000000001"), receiver, false, true, nil
	case "getQuiddity":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Request.getQuiddity expects 0 arguments")
		}
		if value, ok := receiver.Fields["quiddity"]; ok && value.Kind == ValueObject {
			return value, receiver, false, true, nil
		}
		return vm.currentQuiddityValue(), receiver, false, true, nil
	}
	return Null, receiver, false, false, nil
}

func (vm *VM) siteBaseURL() string {
	base := strings.TrimRight(vm.salesforceBaseURL(), "/")
	prefix := strings.Trim(vm.firstOrgRecordString("Site", "UrlPathPrefix", ""), "/")
	if prefix == "" {
		return base
	}
	return base + "/" + prefix
}

func (vm *VM) siteAdminEmail() string {
	adminID := vm.firstOrgRecordIDField("Site", "AdminId", "")
	if adminID != "" && vm.Org != nil {
		if userObject, ok := vm.Org.Objects["User"]; ok {
			if user, ok := userObject.Records[storage.ID(adminID)]; ok {
				if email, ok := user.GetField("Email"); ok && email.Kind == storage.ValueString && email.String != "" {
					return email.String
				}
			}
		}
	}
	return "system@example.invalid"
}

func (vm *VM) hasOrgRecords(objectName string) bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	object, ok := vm.Org.Objects[objectName]
	return ok && len(object.Records) > 0
}

func (vm *VM) orgBool(objectName, field string, fallback bool) bool {
	if vm.Org == nil {
		return fallback
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return fallback
	}
	for _, record := range object.Records {
		value, ok := record.GetField(field)
		if ok && value.Kind == storage.ValueBoolean {
			return value.Boolean
		}
	}
	return fallback
}

func (vm *VM) firstOrgRecordID(objectName, fallback string) string {
	if vm.Org == nil {
		return fallback
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return fallback
	}
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return fallback
	}
	return ids[0]
}

func (vm *VM) firstOrgRecordIDField(objectName, field string, fallback string) string {
	value := vm.firstOrgRecordValue(objectName, field)
	if value.Kind == storage.ValueID {
		return string(value.ID)
	}
	if value.Kind == storage.ValueString {
		return value.String
	}
	return fallback
}

func (vm *VM) firstOrgRecordString(objectName, field, fallback string) string {
	value := vm.firstOrgRecordValue(objectName, field)
	if value.Kind == storage.ValueString {
		return value.String
	}
	if value.Kind == storage.ValueID {
		return string(value.ID)
	}
	return fallback
}

func (vm *VM) firstOrgRecordValue(objectName, field string) storage.Value {
	if vm.Org == nil {
		return storage.Value{}
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return storage.Value{}
	}
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		record := object.Records[storage.ID(id)]
		if value, ok := record.GetField(field); ok {
			return value
		}
	}
	return storage.Value{}
}

func (vm *VM) currentUserHasPackageLicense(packageID Value) (bool, error) {
	packageIDText := strings.TrimSpace(packageID.String())
	if packageIDText == "" || vm.Org == nil {
		return false, nil
	}
	if !looksLikeID(packageIDText) {
		return false, newExceptionError("System.StringException", "Invalid id: "+packageIDText)
	}
	licenses, ok := vm.Org.Objects["PackageLicense"]
	if !ok {
		return false, newExceptionError("System.TypeException", "Package Not Found")
	}
	if _, ok := licenses.Records[storage.ID(packageIDText)]; !ok {
		return false, newExceptionError("System.TypeException", "Package Not Found")
	}
	userID := vm.currentUserInfoField("Id", "005-local-user")
	assignments, ok := vm.Org.Objects["UserPackageLicense"]
	if !ok {
		return false, nil
	}
	for _, record := range assignments.Records {
		if !storageFieldStringEqual(&record, "PackageLicenseId", packageIDText) {
			continue
		}
		if storageFieldStringEqual(&record, "UserId", userID) || userID == "" {
			return true, nil
		}
	}
	return false, nil
}

func formatLocalPhoneNumber(countryCode, phoneNumber string) string {
	country := strings.TrimSpace(countryCode)
	phone := strings.TrimSpace(phoneNumber)
	if country == "" {
		return phone
	}
	country = strings.TrimPrefix(country, "+")
	if phone == "" {
		return "+" + country
	}
	if strings.HasPrefix(phone, "+") {
		return phone
	}
	return "+" + country + " " + phone
}

func (vm *VM) currentUserLicensedForNamespace(namespace Value) bool {
	namespaceText := strings.TrimSpace(namespace.String())
	if namespaceText == "" || vm.Org == nil {
		return false
	}
	licenses, ok := vm.Org.Objects["PackageLicense"]
	if !ok {
		return false
	}
	for id, record := range licenses.Records {
		if !storageFieldStringEqual(&record, "NamespacePrefix", namespaceText) {
			continue
		}
		if !vm.packageLicenseIsActive(&record) {
			continue
		}
		licensed, err := vm.currentUserHasPackageLicense(String(string(id)))
		if err != nil {
			continue
		}
		if licensed {
			return true
		}
	}
	return false
}

func (vm *VM) packageLicenseIsActive(record *storage.Record) bool {
	if record == nil {
		return false
	}
	if status, ok := record.GetField("Status"); ok && status.Kind == storage.ValueString {
		switch strings.ToLower(strings.TrimSpace(status.String)) {
		case "", "active", "enabled":
			return true
		default:
			return false
		}
	}
	if provisioned, ok := record.GetField("IsProvisioned"); ok && provisioned.Kind == storage.ValueBoolean {
		return provisioned.Boolean
	}
	return true
}

func storageFieldStringEqual(record *storage.Record, field, expected string) bool {
	if record == nil {
		return false
	}
	value, ok := record.GetField(field)
	if !ok {
		return false
	}
	switch value.Kind {
	case storage.ValueID:
		return strings.EqualFold(string(value.ID), expected)
	case storage.ValueString:
		return strings.EqualFold(value.String, expected)
	default:
		return false
	}
}

func exprReceiverName(expr ir.Expr) string {
	if expr.Kind == ir.ExprVariable {
		return expr.Name
	}
	if expr.Kind == ir.ExprCall && expr.Left != nil {
		fieldName := ""
		if strings.HasPrefix(expr.Callee, "__field:") {
			fieldName = strings.TrimPrefix(expr.Callee, "__field:")
		} else if strings.HasPrefix(expr.Callee, "__safe_field:") {
			fieldName = strings.TrimPrefix(expr.Callee, "__safe_field:")
		}
		if fieldName != "" {
			if root := exprReceiverName(*expr.Left); root != "" {
				return root + "." + fieldName
			}
		}
	}
	return ""
}

func nullMemberContext(receiverName, member string) string {
	if receiverName == "" {
		return "while invoking member " + member + " on null receiver"
	}
	return "while invoking " + receiverName + "." + member + " on null receiver " + receiverName
}

func (vm *VM) nextDeterministicCryptoLong() int64 {
	vm.cryptoRandomSeq += 0x9e3779b97f4a7c15
	z := vm.cryptoRandomSeq
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	return int64(z)
}

func (vm *VM) nextDeterministicUUID() string {
	hi := uint64(vm.nextDeterministicCryptoLong())
	lo := uint64(vm.nextDeterministicCryptoLong())
	hi = (hi &^ 0xf000) | 0x4000
	lo = (lo &^ 0xc000000000000000) | 0x8000000000000000
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", uint32(hi>>32), uint16(hi>>16), uint16(hi), uint16(lo>>48), lo&0x0000ffffffffffff)
}

func uuidValue(text string) Value {
	return platformScalar("UUID", strings.ToLower(text))
}

func parseUUIDText(text string) (string, error) {
	if len(text) != 36 {
		return "", fmt.Errorf("UUID.fromString expects canonical UUID text")
	}
	for i, r := range text {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return "", fmt.Errorf("UUID.fromString expects canonical UUID text")
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return "", fmt.Errorf("UUID.fromString expects canonical UUID text")
			}
		}
	}
	return strings.ToLower(text), nil
}

func unsupportedIntegrationSurface(callee string) (string, bool) {
	if canonical, ok := canonicalBuiltinStaticCall(callee); ok {
		callee = canonical
	}
	switch {
	case strings.EqualFold(callee, "Auth.AuthConfiguration.getAuthProviderSsoUrl"),
		strings.EqualFold(callee, "Auth.AuthToken.getAccessToken"),
		strings.EqualFold(callee, "Auth.AuthToken.getAccessTokenMap"),
		strings.EqualFold(callee, "Auth.AuthToken.refreshAccessToken"),
		strings.EqualFold(callee, "Auth.AuthToken.revokeAccess"),
		strings.EqualFold(callee, "Auth.CommunitiesUtil.isGuestUser"),
		strings.EqualFold(callee, "Auth.SessionManagement.getCurrentSession"),
		strings.EqualFold(callee, "Auth.JWTUtil.parseJWTFromStringWithoutValidation"):
		return "", false
	}
	switch callee {
	case "Auth.AuthConfiguration.getAuthProviderSsoUrl", "Auth.AuthToken.getAccessToken", "Auth.AuthToken.getAccessTokenMap", "Auth.AuthToken.refreshAccessToken", "Auth.AuthToken.revokeAccess", "Auth.CommunitiesUtil.isGuestUser", "Auth.SessionManagement.getCurrentSession":
		return "", false
	case "Canvas.Test.mockRenderContext", "Canvas.Test.testCanvasLifecycle",
		"Continuation.getResponse", "Test.invokeContinuationMethod", "Test.setContinuationResponse":
		return "", false
	}
	switch callee {
	case "QuickAction.describeAvailableActions", "QuickAction.describeAvailableQuickActions", "QuickAction.describeQuickActions",
		"QuickAction.retrieveQuickActionTemplate", "QuickAction.retrieveQuickActionTemplates",
		"QuickAction.performQuickAction", "QuickAction.performQuickActions",
		"Test.newSendEmailQuickActionDefaults":
		return "", false
	}
	for _, prefix := range []string{"Approval.", "Auth.", "QuickAction.", "Canvas.", "Continuation.", "ExternalService."} {
		if len(callee) >= len(prefix) && strings.EqualFold(callee[:len(prefix)], prefix) {
			switch prefix {
			case "Approval.":
				if strings.EqualFold(callee, "Approval.lock") || strings.EqualFold(callee, "Approval.unlock") || strings.EqualFold(callee, "Approval.isLocked") {
					return "", false
				}
				return "local approval process metadata", true
			case "Auth.":
				return "local authentication token/cloud API surface", true
			case "QuickAction.":
				return "local quick action UI surface", true
			case "Canvas.":
				return "local canvas app integration surface", true
			case "Continuation.":
				return "local continuation callout surface", true
			case "ExternalService.":
				return "live external service execution surface", true
			}
		}
	}
	return "", false
}

func unsupportedCoreStaticSurface(callee string) (string, bool) {
	if canonical, ok := canonicalBuiltinStaticCall(callee); ok {
		callee = canonical
	}
	switch callee {
	case "Crypto.signXml":
		return "local XML signature surface", true
	case "Ideas.markRead":
		return "local Ideas reply/read-state service surface", true
	case "KbManagement.PublishingService.deleteArchivedArticle",
		"KbManagement.PublishingService.deleteArchivedArticleVersion",
		"KbManagement.PublishingService.deleteDraftArticle",
		"KbManagement.PublishingService.deleteDraftTranslation":
		return "local Knowledge delete surface", true
	case "System.changeOwnPassword", "System.movePassword", "System.resetPassword", "System.resetPasswordWithEmailTemplate":
		return "local password/admin mutation surface", true
	case "System.requestVersion":
		return "unmanaged anonymous API surface", true
	case "System.process", "System.submit":
		return "local approval submit/process surface", true
	default:
		return "", false
	}
}

func (vm *VM) quickActionDescribeAvailable(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("QuickAction.describeAvailableQuickActions expects parentType String")
	}
	parentType := strings.TrimSpace(args[0].Text)
	out := typedList("List<QuickAction.DescribeAvailableQuickActionResult>")
	for _, action := range vm.quickActionMetadata() {
		if parentType != "" && action.TargetObject != "" && !strings.EqualFold(action.TargetObject, parentType) {
			continue
		}
		result := Object("QuickAction.DescribeAvailableQuickActionResult")
		result.Fields["name"] = String(action.Name)
		result.Fields["label"] = String(firstNonEmptyString(action.Label, action.Name))
		result.Fields["type"] = String(firstNonEmptyString(action.Type, "Action"))
		result.Fields["actionenumorid"] = String(action.Name)
		out.List = append(out.List, result)
	}
	return out, nil
}

func (vm *VM) quickActionDescribe(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("QuickAction.describeQuickActions expects List<String>")
	}
	out := typedList("List<QuickAction.DescribeQuickActionResult>")
	for _, name := range args[0].List {
		if name.Kind != ValueString {
			return Null, fmt.Errorf("QuickAction.describeQuickActions expects List<String>")
		}
		action, ok := vm.quickActionByName(name.Text)
		if !ok {
			action = storage.QuickActionMetadata{Name: name.Text, Label: name.Text, Type: "Action"}
		}
		result := Object("QuickAction.DescribeQuickActionResult")
		result.Fields["name"] = String(action.Name)
		result.Fields["label"] = String(firstNonEmptyString(action.Label, action.Name))
		result.Fields["type"] = String(firstNonEmptyString(action.Type, "Action"))
		result.Fields["actionenumorid"] = String(action.Name)
		result.Fields["contextsobjecttype"] = String(action.TargetObject)
		result.Fields["targetsobjecttype"] = String(action.TargetObject)
		result.Fields["defaultvalues"] = quickActionDescribeDefaultValues(action)
		result.Fields["parameters"] = typedList("List<QuickAction.DescribeQuickActionParameter>")
		result.Fields["colors"] = typedList("List<Schema.DescribeColorResult>")
		result.Fields["icons"] = typedList("List<Schema.DescribeIconResult>")
		result.Fields["showquickactionlcheader"] = Bool(false)
		result.Fields["showquickactionvfheader"] = Bool(false)
		out.List = append(out.List, result)
	}
	return out, nil
}

func (vm *VM) quickActionRetrieveTemplate(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || !isApexIDLikeValue(args[1]) {
		return Null, fmt.Errorf("QuickAction.retrieveQuickActionTemplate expects quickActionName String and contextId Id")
	}
	return vm.quickActionTemplateResult(args[0].Text, args[1]), nil
}

func (vm *VM) quickActionRetrieveTemplates(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueList || !isApexIDLikeValue(args[1]) {
		return Null, fmt.Errorf("QuickAction.retrieveQuickActionTemplates expects List<String> and contextId Id")
	}
	out := typedList("List<QuickAction.QuickActionTemplateResult>")
	for _, name := range args[0].List {
		if name.Kind != ValueString {
			return Null, fmt.Errorf("QuickAction.retrieveQuickActionTemplates expects List<String> and contextId Id")
		}
		out.List = append(out.List, vm.quickActionTemplateResult(name.Text, args[1]))
	}
	return out, nil
}

func (vm *VM) quickActionPerform(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("QuickAction.performQuickAction expects QuickActionRequest and optional Boolean")
	}
	if args[0].Kind != ValueObject {
		return Null, fmt.Errorf("QuickAction.performQuickAction expects QuickActionRequest")
	}
	if len(args) == 2 && args[1].Kind != ValueBool {
		return Null, fmt.Errorf("QuickAction.performQuickAction expects optional Boolean")
	}
	return vm.quickActionResult(args[0]), nil
}

func (vm *VM) quickActionPerformMany(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("QuickAction.performQuickActions expects List<QuickActionRequest> and optional Boolean")
	}
	if args[0].Kind != ValueList {
		return Null, fmt.Errorf("QuickAction.performQuickActions expects List<QuickActionRequest>")
	}
	if len(args) == 2 && args[1].Kind != ValueBool {
		return Null, fmt.Errorf("QuickAction.performQuickActions expects optional Boolean")
	}
	out := typedList("List<QuickAction.QuickActionResult>")
	for _, request := range args[0].List {
		if request.Kind != ValueObject {
			return Null, fmt.Errorf("QuickAction.performQuickActions expects List<QuickActionRequest>")
		}
		out.List = append(out.List, vm.quickActionResult(request))
	}
	return out, nil
}

func (vm *VM) quickActionResult(request Value) Value {
	result := Object("QuickAction.QuickActionResult")
	result.Fields["success"] = Bool(true)
	result.Fields["created"] = Bool(false)
	result.Fields["errors"] = typedList("List<Database.Error>")
	result.Fields["ids"] = typedList("List<Id>")
	result.Fields["successmessage"] = String("")
	if _, contextID, ok := objectFieldValue(request, "contextId"); ok {
		result.Fields["contextid"] = contextID
	}
	return result
}

func (vm *VM) testNewSendEmailQuickActionDefaults(args []Value) (Value, error) {
	if len(args) != 2 || !isApexIDLikeValue(args[0]) || !isApexIDLikeValue(args[1]) {
		return Null, fmt.Errorf("Test.newSendEmailQuickActionDefaults expects contextId Id and replyToId Id")
	}
	if err := vm.requireTestContext("Test.newSendEmailQuickActionDefaults"); err != nil {
		return Null, err
	}
	defaults := Object("QuickAction.SendEmailQuickActionDefaults")
	defaults.Fields["actionName"] = String("SendEmail")
	defaults.Fields["actionType"] = String("SendEmail")
	defaults.Fields["contextId"] = args[0]
	defaults.Fields["inReplyToId"] = args[1]
	defaults.Fields["fromAddressList"] = typedList("List<String>")
	defaults.Fields["targetSObject"] = Object("EmailMessage")
	return defaults, nil
}

func (vm *VM) quickActionTemplateResult(name string, contextID Value) Value {
	result := Object("QuickAction.QuickActionTemplateResult")
	result.Fields["contextid"] = String(contextID.String())
	result.Fields["success"] = Bool(true)
	result.Fields["errors"] = typedList("List<Database.Error>")
	result.Fields["defaultvalues"] = vm.quickActionTemplateSObject(name)
	result.Fields["defaultvalueformulas"] = Object("SObject")
	return result
}

func (vm *VM) quickActionTemplateSObject(name string) Value {
	action, ok := vm.quickActionByName(name)
	objectName := ""
	if ok {
		objectName = action.TargetObject
	}
	if objectName == "" {
		objectName = quickActionObjectFromName(name)
	}
	if objectName == "" {
		objectName = "SObject"
	}
	record := Object(objectName)
	record.Fields["QuickActionName"] = String(name)
	for _, fieldValue := range action.PredefinedFieldValues {
		field := strings.TrimSpace(fieldValue.Field)
		if field == "" {
			continue
		}
		record.Fields[field] = String(fieldValue.Value)
	}
	return record
}

func quickActionDescribeDefaultValues(action storage.QuickActionMetadata) Value {
	out := typedList("List<QuickAction.DescribeQuickActionDefaultValue>")
	for _, fieldValue := range action.PredefinedFieldValues {
		field := strings.TrimSpace(fieldValue.Field)
		if field == "" {
			continue
		}
		item := Object("QuickAction.DescribeQuickActionDefaultValue")
		item.Fields["field"] = String(field)
		item.Fields["defaultvalue"] = String(fieldValue.Value)
		out.List = append(out.List, item)
	}
	return out
}

func (vm *VM) quickActionByName(name string) (storage.QuickActionMetadata, bool) {
	for _, action := range vm.quickActionMetadata() {
		if strings.EqualFold(action.Name, name) {
			return action, true
		}
	}
	return storage.QuickActionMetadata{}, false
}

func (vm *VM) quickActionMetadata() []storage.QuickActionMetadata {
	if vm.Org == nil {
		return nil
	}
	return vm.Org.Metadata.QuickActions
}

func quickActionObjectFromName(name string) string {
	if dot := strings.IndexByte(name, '.'); dot > 0 {
		return name[:dot]
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func callQuickActionMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	if strings.HasPrefix(method, "get") || strings.HasPrefix(method, "is") {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
		}
		prefix := "get"
		if strings.HasPrefix(method, "is") {
			prefix = "is"
		}
		field := passiveAccessorFieldName(receiver, strings.TrimPrefix(method, prefix))
		if _, value, ok := objectFieldValue(receiver, field); ok {
			return value, receiver, false, true, nil
		}
		return quickActionDefaultGetterValue(receiver.Type, method), receiver, false, true, nil
	}
	if strings.HasPrefix(method, "set") {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 1 argument", receiver.Type, method)
		}
		field := passiveAccessorFieldName(receiver, strings.TrimPrefix(method, "set"))
		receiver.Fields[field] = args[0]
		return Null, receiver, true, true, nil
	}
	return Null, receiver, false, false, nil
}

func quickActionDefaultGetterValue(typeName, method string) Value {
	switch method {
	case "getDefaultValueFormulas", "getTargetSObject":
		return Object("SObject")
	case "getDefaultValues":
		if strings.EqualFold(typeName, "QuickAction.QuickActionTemplateResult") {
			return Object("SObject")
		}
		return typedList("List<QuickAction.DescribeQuickActionDefaultValue>")
	case "getErrors":
		return typedList("List<Database.Error>")
	case "getIds":
		return typedList("List<Id>")
	case "getFromAddressList":
		return typedList("List<String>")
	case "getParameters":
		return typedList("List<QuickAction.DescribeQuickActionParameter>")
	case "getColors":
		return typedList("List<Schema.DescribeColorResult>")
	case "getIcons":
		return typedList("List<Schema.DescribeIconResult>")
	case "isSuccess":
		return Bool(strings.EqualFold(typeName, "QuickAction.QuickActionTemplateResult"))
	case "isCreated", "isEditableForNew", "isEditableForUpdate", "isPlaceholder", "isRequired", "isCollapsed", "isUseCollapsibleSection", "isUseHeading":
		return Bool(false)
	default:
		return Null
	}
}

func (vm *VM) kbPublishingServiceVoid(callee string, args []Value) (Value, error) {
	if err := vm.validateKbPublishingServiceArgs(callee, args); err != nil {
		return Null, err
	}
	return Null, nil
}

func (vm *VM) validateKbPublishingServiceArgs(callee string, args []Value) error {
	specs := map[string][]ValueKind{
		"KbManagement.PublishingService.archiveOnlineArticle":                {ValueString, ValueObject},
		"KbManagement.PublishingService.assignDraftArticleTask":              {ValueString, ValueString, ValueString, ValueObject, ValueBool},
		"KbManagement.PublishingService.assignDraftTranslationTask":          {ValueString, ValueString, ValueString, ValueObject, ValueBool},
		"KbManagement.PublishingService.cancelScheduledArchivingOfArticle":   {ValueString},
		"KbManagement.PublishingService.cancelScheduledPublicationOfArticle": {ValueString},
		"KbManagement.PublishingService.completeTranslation":                 {ValueString},
		"KbManagement.PublishingService.editArchivedArticle":                 {ValueString},
		"KbManagement.PublishingService.editOnlineArticle":                   {ValueString, ValueBool},
		"KbManagement.PublishingService.editPublishedTranslation":            {ValueString, ValueString, ValueBool},
		"KbManagement.PublishingService.publishArticle":                      {ValueString, ValueBool},
		"KbManagement.PublishingService.restoreOldVersion":                   {ValueString, ValueInt},
		"KbManagement.PublishingService.scheduleForPublication":              {ValueString, ValueObject},
		"KbManagement.PublishingService.setTranslationToIncomplete":          {ValueString},
		"KbManagement.PublishingService.submitForTranslation":                {ValueString, ValueString, ValueString, ValueObject},
	}
	want, ok := specs[callee]
	if !ok {
		return fmt.Errorf("unsupported KbManagement.PublishingService call %s", callee)
	}
	if len(args) != len(want) {
		return fmt.Errorf("%s expects %d arguments", callee, len(want))
	}
	for i, kind := range want {
		if kind == ValueObject && args[i].Kind == ValueObject && args[i].Type == "Datetime" {
			continue
		}
		if args[i].Kind != kind {
			return fmt.Errorf("%s argument %d has wrong type", callee, i+1)
		}
	}
	return nil
}

func remoteObjectControllerResult(callee string, args []Value) (Value, error) {
	switch callee {
	case "RemoteObjectController.retrieve":
		if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueList || args[2].Kind != ValueMap {
			return Null, fmt.Errorf("RemoteObjectController.retrieve expects object name, field list, and criteria map")
		}
	case "RemoteObjectController.create", "RemoteObjectController.updat":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, fmt.Errorf("%s expects object name and values map", callee)
		}
	case "RemoteObjectController.update":
		if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueList || args[2].Kind != ValueMap {
			return Null, fmt.Errorf("RemoteObjectController.update expects object name, Id list, and values map")
		}
	case "RemoteObjectController.del":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueList {
			return Null, fmt.Errorf("RemoteObjectController.del expects object name and Id list")
		}
	default:
		return Null, fmt.Errorf("unsupported RemoteObjectController call %s", callee)
	}
	result := Map()
	for key, value := range map[string]Value{
		"success": Bool(true),
		"records": List(),
		"errors":  List(),
	} {
		encoded := mapKey(String(key))
		result.Map[encoded] = value
		result.MapKeys[encoded] = String(key)
	}
	return result, nil
}

func (vm *VM) callIndustryControllerStatic(callee string, args []Value) (Value, bool, error) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok {
		dot := strings.LastIndex(callee, ".")
		if dot <= 0 || dot == len(callee)-1 {
			return Null, false, nil
		}
		className, methodName = callee[:dot], callee[dot+1:]
	}
	if industryControllerUnsupportedStatic(className, methodName) {
		return Null, true, unsupportedCallError(callee + " local industry service mutation surface")
	}
	if !industryControllerDefaultStatic(className, methodName) {
		return Null, false, nil
	}
	return vm.industryControllerDefaultReturn(className, methodName, args, true)
}

func (vm *VM) callIndustryControllerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	className := receiver.Type
	if industryControllerUnsupportedInstance(className, method) {
		return Null, receiver, false, true, unsupportedCallError(className + "." + method + " local industry service mutation surface")
	}
	if !industryControllerDefaultInstance(className, method) {
		return Null, receiver, false, false, nil
	}
	value, handled, err := vm.industryControllerDefaultReturn(className, method, args, false)
	return value, receiver, false, handled, err
}

func (vm *VM) industryControllerDefaultReturn(className, methodName string, args []Value, static bool) (Value, bool, error) {
	switch {
	case industryTypeName(className, "LoyaltyManagement.WidgetVisibility") && strings.EqualFold(methodName, "checkVisibility"):
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, true, fmt.Errorf("LoyaltyManagement.WidgetVisibility.checkVisibility expects String and Map<String,Object>")
		}
		return Bool(false), true, nil
	case industryEventManagementReadMethod(className, methodName):
		if len(args) != 3 || args[0].Kind != ValueMap || args[1].Kind != ValueMap || args[2].Kind != ValueMap {
			return Null, true, fmt.Errorf("%s.%s expects input, output, and options maps", className, methodName)
		}
		return industryMapResult(), true, nil
	case industryWidgetCallMethod(className, methodName):
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, true, fmt.Errorf("%s.call expects action String and arguments map", className)
		}
		return industryMapResult(), true, nil
	case industryTypeName(className, "inventorypricing.GetInventoryPricing"):
		switch strings.ToLower(methodName) {
		case "getinventory", "getinventoryandpricing", "getpricing":
			if len(args) != 1 || args[0].Kind != ValueObject {
				return Null, true, fmt.Errorf("%s.%s expects InventoryPricingData", className, methodName)
			}
			return args[0], true, nil
		case "handleinventorypricingserviceexception":
			if len(args) != 2 || args[1].Kind != ValueObject {
				return Null, true, fmt.Errorf("%s.%s expects Exception and InventoryPricingData", className, methodName)
			}
			return args[1], true, nil
		case "processinput":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("%s.processInput expects input object", className)
			}
			return Object("inventorypricing.InventoryPricingData"), true, nil
		case "createresponse":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("%s.createResponse expects InventoryPricingData", className)
			}
			return industryMapResult(), true, nil
		}
	}
	method, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, static)
	if !ok {
		return Null, false, nil
	}
	if strings.EqualFold(className, "healthcloudext.IntegratedCareManagementApexHelper") && strings.EqualFold(methodName, "convertMultiLineToHtml") {
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("%s.%s expects String", className, methodName)
		}
		text := strings.ReplaceAll(args[0].Text, "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		return String(strings.ReplaceAll(text, "\n", "<br/>")), true, nil
	}
	return vm.generatedPlatformMethodDefaultReturn(method, Null, args), true, nil
}

func industryControllerDefaultStatic(className, methodName string) bool {
	name := strings.ToLower(methodName)
	switch className {
	case "healthcloudext.AppointmentBookingSelfService":
		switch name {
		case "findassets", "findavailableappointmentslots", "findavailableassetslots", "findproviders",
			"getgeolocationcoordinates", "logselfserviceinstrumentation", "validateslotstatusselfservice":
			return true
		}
	case "healthcloudext.IntegratedCareManagementApexHelper":
		switch name {
		case "checkentity", "checkobjectcreationaccess", "convertmultilinetohtml",
			"fetchsuggestedassessmentsforpatient", "getcarebarrierdetails", "getmaxaccesslevel",
			"getmru", "getpicklist", "getsoslsearch":
			return true
		}
	case "healthcloudext.IntegratedCareManagementApexUtil":
		return name == "checkcaregapaccess" || name == "checkcreateaccess"
	case "LoyaltyManagement.LoyaltyResources":
		switch name {
		case "getloyaltypromotionbasedonsalesforcecdp", "getloyaltypromotions", "getpointsbalance", "gettier":
			return true
		}
	case "LoyaltyManagement.WidgetVisibility":
		return name == "checkvisibility"
	case "LoyaltyManagement.WidgetCumulativePromotions", "LoyaltyManagement.WidgetMemberBadges", "LoyaltyManagement.WidgetReferMember":
		return name == "call"
	case "industries_docgen.DocGenPermsAndAccessChecksService":
		return strings.HasPrefix(name, "has") || strings.HasPrefix(name, "is")
	}
	return false
}

func industryControllerUnsupportedStatic(className, methodName string) bool {
	name := strings.ToLower(methodName)
	switch className {
	case "healthcloudext.AppointmentBookingSelfService":
		switch name {
		case "bookselfserviceappointment", "cancelselfserviceappointment", "createpatient", "publisheventforpft":
			return true
		}
	case "healthcloudext.ATMCRMAuthenticationPortalUserDelegator":
		return name == "executeauthenticationforportaluser"
	case "LoyaltyManagement.LoyaltyResources":
		switch name {
		case "changetier", "creditpoints", "debitpoints", "issuevoucher",
			"transfermemberpointstogroups", "updateprogressforcumulativepromotionusage":
			return true
		}
	case "RevSalesTrxn.PlaceSalesTransactionExecutor":
		return name == "execute"
	}
	return false
}

func industryControllerDefaultInstance(className, methodName string) bool {
	name := strings.ToLower(methodName)
	if industryWidgetCallMethod(className, methodName) {
		return true
	}
	if industryControllerMapCallback(className, methodName) {
		return true
	}
	if industryTypeName(className, "inventorypricing.GetInventoryPricing") {
		switch name {
		case "createresponse", "getinventory", "getinventoryandpricing", "getpricing", "handleinventorypricingserviceexception", "processinput":
			return true
		}
	}
	switch className {
	case "fscwmgen.RecordAlertBatchProvider":
		return name == "getalertsbyparentidbatch" || name == "getalertsbywhatidbatch"
	case "fscwmgen.RecordAlertProvider":
		return name == "getalertsbyparentid" || name == "getalertsbywhatid" || name == "getalertsbywhatidandparentid"
	case "healthcloudext.AppointmentBookingInterop", "healthcloudext.AppointmentBookingInteropFhirAdapter":
		return name == "findslots" || name == "getslotstatus"
	case "healthcloudext.IQuotasAndAllocation":
		return name == "validateslotchain"
	case "id_verification.IdentityVerificationExt":
		return name == "getverifiers" || name == "search"
	case "ind_docgen_api.EnvelopeStatusScheduler":
		return name == "execute"
	case "service_cloud_voice.GroupSetup":
		return name == "listgroups"
	case "service_cloud_voice.PhoneNumberProvider":
		return name == "listphonenumbers"
	case "service_cloud_voice.QueueManager":
		return name == "supportsqueueusergrouping"
	case "service_cloud_voice.QueueSetup":
		return name == "listqueues"
	}
	return industryEventManagementReadMethod(className, methodName)
}

func industryControllerUnsupportedInstance(className, methodName string) bool {
	name := strings.ToLower(methodName)
	switch className {
	case "fscwmgen.BranchManagementAssociationHandler":
		return name == "handleassociation"
	case "fscwmgen.RecordAlertBatchProvider":
		return name == "dismissalertsbatch" || name == "snoozealertsbatch"
	case "fscwmgen.RecordAlertProvider":
		return name == "dismissalert" || name == "snoozealert"
	case "healthcloudext.AppointmentBookingInterop", "healthcloudext.AppointmentBookingInteropFhirAdapter":
		return name == "bookappointment" || name == "cancelappointment"
	case "healthcloudext.IBenefitsVerificationInterOp":
		return name == "verifybenefits"
	case "healthcloudext.IQuotasAndAllocation":
		return name == "fetchquotaavailability"
	case "healthcloudext.IUnifiedHealthScore":
		return name == "saveactiondetail"
	case "healthcloudext.RosterFileRelatedObjectsCreationService":
		return name == "createcaserelatedfiles"
	case "healthcloudext.UMBookAppointmentSlotService":
		return name == "bookslotremoteaction"
	case "ime_mrm.EventManagementBudgetApi", "ime_mrm.EventManagementManagedEventApi",
		"ime_mrm.EventManagementParticipantApi", "ime_mrm.EventManagementProductApi", "ime_mrm.EventManagementSubjectApi":
		return strings.HasPrefix(name, "create") || strings.HasPrefix(name, "update") || strings.HasPrefix(name, "delete")
	case "service_cloud_voice.GroupSetup":
		return name == "associateuserswithgroup" || name == "creategroup"
	case "service_cloud_voice.PartnerConnector":
		return name == "connect"
	case "service_cloud_voice.QueueSetup":
		return name == "associateusersandgroupswithqueue" || name == "createqueue" || name == "removequeue"
	case "service_cloud_voice.UpdateOrgDomainProvider":
		return name == "updateorgdomainvalues"
	case "service_cloud_voice.UserSyncing":
		return name == "adduserstocontactcenter" || name == "removeusersfromcontactcenter"
	default:
		return false
	}
}

func industryControllerMapCallback(className, methodName string) bool {
	name := strings.ToLower(methodName)
	switch className {
	case "fschousehold.FSCFinancialAccountService", "fschousehold.FSCGoalService",
		"fschousehold.FSCHouseholdService", "fschousehold.FSCPlanService",
		"fschousehold.RetrievalSummaryDataRefresh",
		"healthcloudext.AppointmentBookingSelfServiceWrapper", "healthcloudext.CommunityHelper",
		"healthcloudext.HealthCloudICMCareGapUtil", "healthcloudext.HealthCloudICMDiscoveryFrameworkUtil",
		"healthcloudext.IntegratedCareManagementApexUtil", "healthcloudext.IntegratedCareManagementCPTApexUtil",
		"healthcloudext.IntegratedCareManagementUtil_250", "healthcloudext.ProviderSearchCardUtil",
		"healthcloudext.ReferralManagementUtil", "healthcloudext.SuggestedResponseAssessmentService",
		"healthcloudext.UtilizationManagementWrapper",
		"ind_docgen_api.OpenInterface",
		"industries_docgen.ApryseReplacementService", "industries_docgen.DocumentGenerationProcess",
		"industries_docgen.DocumentTemplate":
		return name == "call" || name == "invokemethod"
	default:
		return false
	}
}

func industryWidgetCallMethod(className, methodName string) bool {
	if !strings.EqualFold(methodName, "call") {
		return false
	}
	switch className {
	case "LoyaltyManagement.WidgetCumulativePromotions", "LoyaltyManagement.WidgetMemberBadges", "LoyaltyManagement.WidgetReferMember",
		"WidgetCumulativePromotions", "WidgetMemberBadges", "WidgetReferMember":
		return true
	default:
		return false
	}
}

func industryEventManagementReadMethod(className, methodName string) bool {
	name := strings.ToLower(methodName)
	if !strings.HasPrefix(name, "get") {
		return false
	}
	switch className {
	case "ime_mrm.EventManagementBudgetApi", "ime_mrm.EventManagementManagedEventApi",
		"ime_mrm.EventManagementParticipantApi", "ime_mrm.EventManagementProductApi", "ime_mrm.EventManagementSubjectApi",
		"EventManagementBudgetApi", "EventManagementManagedEventApi",
		"EventManagementParticipantApi", "EventManagementProductApi", "EventManagementSubjectApi":
		return true
	default:
		return false
	}
}

func industryTypeName(actual, qualified string) bool {
	if strings.EqualFold(actual, qualified) {
		return true
	}
	_, short, ok := strings.Cut(qualified, ".")
	return ok && strings.EqualFold(actual, short)
}
