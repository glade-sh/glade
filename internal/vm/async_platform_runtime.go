package vm

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) eventBusPublish(args []Value, result *Result) (Value, error) {
	if vm.rejectAsyncActions {
		return Null, vm.rejectSynchronousAsyncAction("EventBus.publish cannot cross the synchronous LWC action boundary")
	}
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("EventBus.publish expects event record or list and optional callback")
	}
	records := []Value{args[0]}
	if args[0].Kind == ValueList {
		records = args[0].List
	}
	if len(records) == 0 {
		return List(), nil
	}
	// R176: duplicate event aliases are rejected before either is published.
	seen := make(map[uint64]bool)
	for _, record := range records {
		if record.Kind == ValueNull {
			if args[0].Kind == ValueList {
				// Native nullListElement/mixedNullElement return unstable hosted
				// internal errors. Keep that outcome as an explicit local boundary.
				return Null, newExceptionError("System.UnsupportedOperationException", "EventBus.publish with null list elements is unavailable locally: Salesforce returned an unstable internal error")
			}
			return Null, newExceptionError("NullPointerException", "Attempt to de-reference a null object")
		}
		if record.Ref != 0 && seen[record.Ref] {
			return Null, newExceptionError("ListException", "Before Insert or Upsert list must not have two identically equal elements")
		}
		seen[record.Ref] = record.Ref != 0
	}
	// P011/P012: native callback publication aborts outside Apex catch.
	// Keep this hosted boundary explicit before any local publication state.
	if len(args) == 2 && args[1].Kind == ValueNull {
		for _, record := range records {
			if record.Kind == ValueObject && !record.projectClass && strings.EqualFold(runtimeObjectType(record), "BatchApexErrorEvent") {
				return Null, newExceptionError("System.UnsupportedOperationException", "EventBus.publish for BatchApexErrorEvent with a null callback requires the hosted event service")
			}
		}
	}
	ordinaryDMLRows := 0
	// Salesforce accounts an erased List<SObject> as ordinary DML, even
	// when its records are immediate-publish platform events.
	elementType, _ := collectionElementType(args[0].Type)
	if args[0].Kind == ValueList && strings.EqualFold(elementType, "SObject") {
		ordinaryDMLRows = len(records)
	} else {
		for _, record := range records {
			if vm.Org == nil {
				break
			}
			if objectName, ok := storage.ResolveObjectName(*vm.Org, record.Type); ok && strings.EqualFold(vm.Org.Objects[objectName].Definition.Metadata["publishBehavior"], "PublishAfterCommit") {
				ordinaryDMLRows++
			}
		}
	}
	if ordinaryDMLRows > 0 {
		if err := vm.incrementLimit("dmlStatements", 1); err != nil {
			return Null, err
		}
		if err := vm.incrementLimit("dmlRows", ordinaryDMLRows); err != nil {
			return Null, err
		}
	}
	if ordinaryDMLRows < len(records) {
		if err := vm.incrementLimit("publishImmediateDml", 1); err != nil {
			return Null, err
		}
	}
	results := make([]Value, 0, len(records))
	triggerRecords := make([]storage.Record, 0, len(records))
	eventUUIDs := make([]string, 0, len(records))
	// Native T/U/V controls: test-only allowance, evaluated in publish chunks.
	quotaExceeded := false
	for recordIndex, record := range records {
		if vm.testContext != nil && !quotaExceeded && recordIndex%200 == 0 {
			eligible := 0
			for _, candidate := range records[recordIndex:min(recordIndex+200, len(records))] {
				if vm.platformEventCanEnqueue(candidate) {
					eligible++
				}
			}
			quotaExceeded = vm.testContext.PlatformEventPublishes+eligible >= 500
		}
		if record.Kind != ValueObject {
			return Null, fmt.Errorf("EventBus.publish expects SObject event record(s)")
		}
		stored, err := vm.recordFromValue(&record)
		if err != nil {
			return Null, err
		}
		// P004-P009: an erased scalar or concrete standard-object list retains
		// the DML rejection; a genuine List<SObject> returns a row failure.
		if strings.EqualFold(stored.Object, "BatchApexErrorEvent") {
			message := "DML operation INSERT not allowed on BatchApexErrorEvent"
			runtimeElementType, _ := collectionElementType(runtimeObjectType(args[0]))
			if args[0].Kind == ValueList && strings.EqualFold(runtimeElementType, "SObject") {
				results = append(results, platformEventSaveResult("", false, "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", message, nil))
				continue
			}
			return Null, newExceptionError("TypeException", message)
		}
		if !vm.hasPlatformEventMetadata(stored.Object) {
			return Null, newExceptionError("UnexpectedException", "The specified sObject or list of sObjects contains objects that aren’t platform events. You can publish only platform event objects using EventBus.publish. Ensure the type of the specified sObject is a platform event.")
		}
		eventUUID, hasEventUUID := vm.platformEventUUID(record)
		if publishedID, ok := record.Fields[sobjectPublishedEventIDField]; ok {
			if text, ok := idTextFromValue(publishedID); ok {
				stored.ID = storage.ID(text)
			}
		}
		if stored.ID != "" {
			results = append(results, platformEventSaveResult(stored.ID, false, "INVALID_FIELD_FOR_INSERT_UPDATE", "The event can't be published because it contains an ID value.", nil))
			continue
		}
		if code, message, fields := vm.platformEventValidationError(stored); code != "" {
			results = append(results, platformEventSaveResult("", false, code, message, fields))
			continue
		}
		if quotaExceeded {
			results = append(results, platformEventSaveResult("", false, "LIMIT_EXCEEDED", "The number of platform event messages published from an Apex test context exceeded the limit of 500.", nil))
			continue
		}
		if vm.testContext != nil {
			vm.testContext.PlatformEventPublishes++
		}
		if !hasEventUUID {
			eventUUID = vm.nextDeterministicUUID()
			putVMFieldPath(record, "EventUuid", String(eventUUID))
		}
		if vm.Org != nil {
			if _, err := vm.assignPendingInsertID(&stored); err != nil {
				return Null, err
			}
			// Publishing assigns event identity without exposing a ReplayId.
			record.Fields[sobjectPublishedEventIDField] = platformScalar("Id", string(stored.ID))
		}
		stored.Fields["ReplayId"] = storage.StringValue(vm.nextEventBusReplayID())
		stored.Fields["EventUuid"] = storage.StringValue(eventUUID)
		triggerRecords = append(triggerRecords, stored)
		row := platformEventSaveResult(stored.ID, true, "OPERATION_ENQUEUED", eventUUID, nil)
		row.Fields[sobjectEventOperationIDField] = String(eventUUID)
		results = append(results, row)
		if len(args) == 2 {
			eventUUIDs = append(eventUUIDs, eventUUID)
		}
	}
	if len(args) == 2 && args[1].Kind != ValueNull {
		if args[1].Kind != ValueObject {
			return Null, fmt.Errorf("EventBus.publish callback expects object")
		}
		if vm.testContext != nil {
			vm.testContext.EventPublishes = append(vm.testContext.EventPublishes, eventPublishCallback{
				Callback:   args[1],
				EventUUIDs: eventUUIDs,
			})
		}
	}
	if vm.testContext != nil && !vm.testContext.Stopped {
		vm.testContext.PlatformEvents = append(vm.testContext.PlatformEvents, triggerRecords...)
	} else {
		if _, err := vm.runTriggers(triggerTimingAfter, "insert", triggerRecords, nil, result); err != nil {
			return Null, err
		}
	}
	appendTrace(result, "apex.eventbus.publish", "apex.eventbus", map[string]any{
		"records":  len(records),
		"delivery": "local-after-insert-trigger",
	})
	if args[0].Kind == ValueList {
		return List(results...), nil
	}
	if len(results) == 0 {
		return Null, nil
	}
	return results[0], nil
}

// V004: invalid records do not consume the test publication allowance.
func (vm *VM) platformEventCanEnqueue(record Value) bool {
	if _, published := record.Fields[sobjectPublishedEventIDField]; published {
		return false
	}
	stored, err := vm.recordFromValue(&record)
	if err != nil || !vm.hasPlatformEventMetadata(stored.Object) || stored.ID != "" {
		return false
	}
	code, _, _ := vm.platformEventValidationError(stored)
	return code == ""
}

// Platform event publication uses the captured SaveResult shape.
func platformEventSaveResult(id storage.ID, success bool, code, message string, fields []string) Value {
	row := Object("Database.SaveResult")
	row.Fields["success"] = Bool(success)
	row.Fields["id"] = Null
	if id != "" {
		row.Fields["id"] = platformScalar("Id", string(id))
	}
	row.Fields["error"] = String("")
	errValue := Object("Database.Error")
	errValue.Fields["message"] = String(message)
	errValue.Fields["statusCode"] = String(code)
	fieldValues := make([]Value, 0, len(fields))
	for _, field := range fields {
		fieldValues = append(fieldValues, String(field))
	}
	errValue.Fields["fields"] = List(fieldValues...)
	row.Fields["errors"] = List(errValue)
	return row
}

func (vm *VM) platformEventValidationError(record storage.Record) (string, string, []string) {
	if field, missing := vm.missingRequiredPlatformEventField(record); missing {
		return "REQUIRED_FIELD_MISSING", "You must enter a value: " + field, []string{field}
	}
	if vm.Org != nil {
		if objectName, ok := vm.resolveObjectName(record.Object); ok {
			definition := vm.Org.Objects[objectName].Definition
			names := make([]string, 0, len(definition.Fields))
			for name := range definition.Fields {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				field := definition.Fields[name]
				if value, ok := record.GetField(name); ok && field.Type == storage.FieldString && field.Length > 0 && value.Kind == storage.ValueString && len(utf16.Encode([]rune(value.String))) > field.Length {
					return "STRING_TOO_LONG", "Value too long for field", []string{name}
				}
			}
		}
	}
	return "", "", nil
}

// Captured event contracts use object provenance, rather than API-name spelling.
func (vm *VM) hasPlatformEventMetadata(objectName string) bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	name, ok := vm.resolveObjectName(objectName)
	if !ok {
		return false
	}
	behavior := vm.Org.Objects[name].Definition.Metadata["publishBehavior"]
	return strings.EqualFold(behavior, "PublishImmediately") || strings.EqualFold(behavior, "PublishAfterCommit")
}

func (vm *VM) nextEventBusReplayID() string {
	vm.eventBusReplaySequence++
	return fmt.Sprintf("%d", vm.eventBusReplaySequence)
}

func eventBusGetOperationID(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("EventBus.getOperationId expects result")
	}
	if args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "Database.SaveResult") {
		return Null, nil
	}
	operationID, ok := args[0].Fields[sobjectEventOperationIDField]
	if !ok || operationID.Kind != ValueString || operationID.Text == "" {
		return Null, nil
	}
	return operationID, nil
}

func (vm *VM) eventBusPublishWithAccessLevel(args []Value, result *Result) (Value, error) {
	switch len(args) {
	case 2:
		if !isDatabaseAccessLevelValue(args[1]) {
			return Null, fmt.Errorf("EventBus.publishWithAccessLevel AccessLevel overload expects AccessLevel")
		}
		return vm.eventBusPublish([]Value{args[0]}, result)
	case 3:
		if !isDatabaseAccessLevelValue(args[2]) {
			return Null, fmt.Errorf("EventBus.publishWithAccessLevel AccessLevel overload expects AccessLevel")
		}
		if args[1].Kind == ValueNull {
			return vm.eventBusPublish([]Value{args[0]}, result)
		}
		return vm.eventBusPublish([]Value{args[0], args[1]}, result)
	default:
		return Null, fmt.Errorf("EventBus.publishWithAccessLevel expects event record or list, optional callback, and AccessLevel")
	}
}

func (vm *VM) missingRequiredPlatformEventField(record storage.Record) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	objectName := record.Object
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return "", false
	}
	for name, field := range object.Definition.Fields {
		if !field.Required {
			continue
		}
		if isLocalPlatformEventSystemRequiredField(name) {
			continue
		}
		value, ok := record.GetField(name)
		if !ok && hasSuffixFold(name, "__c") {
			value, ok = record.GetField(name[:len(name)-3])
		}
		if !ok || value.Kind == storage.ValueNull {
			return name, true
		}
		if field.Type == storage.FieldString && strings.TrimSpace(value.String) == "" {
			return name, true
		}
	}
	return "", false
}

func isLocalPlatformEventSystemRequiredField(field string) bool {
	switch strings.ToLower(field) {
	case "id", "eventuuid", "replayid", "createddate", "createdbyid":
		return true
	default:
		return false
	}
}

func (vm *VM) platformEventUUID(record Value) (string, bool) {
	if !vm.hasPlatformEventMetadata(record.Type) {
		return "", false
	}
	_, value, ok := objectFieldValue(record, "EventUuid")
	if !ok || value.Kind == ValueNull {
		return "", false
	}
	text := ""
	switch value.Kind {
	case ValueString:
		text = value.Text
	case ValueObject:
		text, _ = platformScalarObjectText(value)
	}
	return text, text != ""
}

func (vm *VM) connectAPIOrganizationSettings(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("ConnectApi.Organization.getSettings expects 0 arguments")
	}
	orgID := "00D000000000001"
	if vm.Org != nil && vm.Org.OrgID != "" {
		orgID = vm.Org.OrgID
	}
	settings := Object("ConnectApi.OrganizationSettings")
	settings.Fields["id"] = String(orgID)
	settings.Fields["orgId"] = String(orgID)
	settings.Fields["name"] = String(vm.firstOrgRecordString("Organization", "Name", "Local Organization"))
	settings.Fields["defaultLanguage"] = String(vm.currentUserInfoField("LanguageLocaleKey", "en_US"))
	settings.Fields["defaultLocale"] = String(vm.currentUserInfoField("LocaleSidKey", "en_US"))
	settings.Fields["defaultTimeZone"] = connectAPITimeZone(vm.currentUserTimeZoneID())
	settings.Fields["userSettings"] = vm.connectAPIUserSettings()
	return settings, nil
}

func (vm *VM) connectAPIUserSettings() Value {
	settings := Object("ConnectApi.UserSettings")
	settings.Fields["approvalPosts"] = Bool(true)
	settings.Fields["canAccessPersonalStreams"] = Bool(true)
	settings.Fields["canFollow"] = Bool(true)
	settings.Fields["canModifyAllData"] = Bool(true)
	settings.Fields["canOwnGroups"] = Bool(true)
	settings.Fields["canViewAllData"] = Bool(true)
	settings.Fields["canViewAllGroups"] = Bool(true)
	settings.Fields["canViewAllUsers"] = Bool(true)
	settings.Fields["canViewCommunitySwitcher"] = Bool(true)
	settings.Fields["canViewFullUserProfile"] = Bool(true)
	settings.Fields["canViewPublicFiles"] = Bool(true)
	settings.Fields["currencySymbol"] = String("$")
	settings.Fields["externalUser"] = Bool(vm.currentUserInfoField("UserType", "") == "Guest")
	settings.Fields["fileSyncLimit"] = Int(0)
	settings.Fields["fileSyncStorageLimit"] = Int(0)
	settings.Fields["folderSyncLimit"] = Int(0)
	settings.Fields["hasAccessToInternalOrg"] = Bool(true)
	settings.Fields["hasChatter"] = Bool(true)
	settings.Fields["hasFileSync"] = Bool(false)
	settings.Fields["hasFieldServiceLocationTracking"] = Bool(false)
	settings.Fields["hasFieldServiceMobileAccess"] = Bool(false)
	settings.Fields["hasFileSyncManagedClientAutoUpdate"] = Bool(false)
	settings.Fields["hasRestDataApiAccess"] = Bool(true)
	settings.Fields["timeZone"] = connectAPITimeZone(vm.currentUserTimeZoneID())
	settings.Fields["userDefaultCurrencyIsoCode"] = String(vm.currentUserInfoField("DefaultCurrencyIsoCode", "USD"))
	settings.Fields["userId"] = String(vm.currentUserInfoField("Id", "005-local-user"))
	settings.Fields["userLocale"] = String(vm.currentUserInfoField("LocaleSidKey", "en_US"))
	return settings
}

func connectAPITimeZone(name string) Value {
	if strings.TrimSpace(name) == "" {
		name = "America/Los_Angeles"
	}
	tz := Object("ConnectApi.TimeZone")
	tz.Fields["id"] = String(name)
	tz.Fields["name"] = String(name)
	tz.Fields["displayName"] = String(name)
	tz.Fields["offset"] = Int(0)
	tz.Fields["gmtOffset"] = Int(0)
	return tz
}

func (vm *VM) callConnectAPICommunitiesStatic(className, methodName string, args []Value) (Value, bool, error) {
	if !strings.EqualFold(className, "ConnectApi.Communities") && !strings.EqualFold(className, "System.ConnectApi.Communities") {
		return Null, false, nil
	}
	switch strings.ToLower(methodName) {
	case "getcommunity":
		value, err := vm.connectAPICommunity(args)
		return value, true, err
	case "getcommunities":
		value, err := vm.connectAPICommunities(args)
		return value, true, err
	default:
		return Null, false, nil
	}
}

func (vm *VM) connectAPICommunity(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("ConnectApi.Communities.getCommunity expects 1 argument")
	}
	networkID := scalarText(args[0])
	if networkID == "" {
		networkID = vm.firstOrgRecordID("Network", "0DB000000000001")
	}
	return vm.connectAPICommunityValue(networkID), nil
}

func (vm *VM) connectAPICommunities(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("ConnectApi.Communities.getCommunities expects 0 arguments")
	}
	page := Object("ConnectApi.CommunityPage")
	communities := typedList("List<ConnectApi.Community>")
	ids := vm.networkRecordIDs()
	if len(ids) == 0 {
		ids = []string{"0DB000000000001"}
	}
	for _, id := range ids {
		communities.List = append(communities.List, vm.connectAPICommunityValue(id))
	}
	page.Fields["communities"] = communities
	return page, nil
}

func (vm *VM) connectAPICommunityValue(networkID string) Value {
	if strings.TrimSpace(networkID) == "" {
		networkID = "0DB000000000001"
	}
	prefix := strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")
	community := Object("ConnectApi.Community")
	community.Fields["id"] = String(networkID)
	community.Fields["name"] = String(vm.firstOrgRecordString("Network", "Name", "Local Community"))
	community.Fields["urlPathPrefix"] = String(prefix)
	community.Fields["siteUrl"] = String(strings.TrimRight(vm.salesforceBaseURL(), "/") + "/" + prefix)
	return community
}

func (vm *VM) networkRecordIDs() []string {
	if vm == nil || vm.Org == nil {
		return nil
	}
	object, ok := vm.Org.Objects["Network"]
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	return ids
}

func (vm *VM) connectAPINamedCredentialsGetNamedCredentials(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("ConnectApi.NamedCredentials.getNamedCredentials expects 0 arguments")
	}
	return Object("ConnectApi.NamedCredentialList"), nil
}

func (vm *VM) connectAPINamedCredentialsCreateExternalCredential(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("ConnectApi.NamedCredentials.createExternalCredential expects 1 argument")
	}
	if args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "ConnectApi.ExternalCredentialInput") {
		return Null, fmt.Errorf("ConnectApi.NamedCredentials.createExternalCredential expects ConnectApi.ExternalCredentialInput")
	}
	external := Object("ConnectApi.ExternalCredential")
	if developerName, ok := objectFieldFold(args[0], "developerName"); ok && developerName.Kind == ValueString {
		external.Fields["developerName"] = String(developerName.Text)
	}
	if principals, ok := objectFieldFold(args[0], "principals"); ok {
		external.Fields["principals"] = cloneValue(principals)
	}
	return external, nil
}

func (vm *VM) connectAPINamedCredentialsCreateNamedCredential(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("ConnectApi.NamedCredentials.createNamedCredential expects 1 argument")
	}
	if args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "ConnectApi.NamedCredentialInput") {
		return Null, fmt.Errorf("ConnectApi.NamedCredentials.createNamedCredential expects ConnectApi.NamedCredentialInput")
	}
	credential := Object("ConnectApi.NamedCredential")
	if developerName, ok := objectFieldFold(args[0], "developerName"); ok && developerName.Kind == ValueString {
		credential.Fields["developerName"] = String(developerName.Text)
	}
	if externalCredentials, ok := objectFieldFold(args[0], "externalCredentials"); ok {
		credential.Fields["externalCredentials"] = cloneValue(externalCredentials)
	}
	if calloutURL, ok := objectFieldFold(args[0], "calloutUrl"); ok && calloutURL.Kind == ValueString {
		credential.Fields["calloutUrl"] = String(calloutURL.Text)
	}
	return credential, nil
}

func (vm *VM) connectAPINamedCredentialsGetExternalCredential(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("ConnectApi.NamedCredentials.getExternalCredential expects 1 String argument")
	}
	external := Object("ConnectApi.ExternalCredential")
	external.Fields["developerName"] = String(args[0].Text)
	return external, nil
}

func (vm *VM) connectAPIUserProfile(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("ConnectApi.UserProfiles.getUserProfile expects 2 arguments")
	}
	profile := Object("ConnectApi.UserProfile")
	profile.Fields["id"] = String(scalarText(args[1]))
	profile.Fields["communityId"] = String(scalarText(args[0]))
	return profile, nil
}

func (vm *VM) connectAPIUserPhoto(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("ConnectApi.UserProfiles.getPhoto expects 2 arguments")
	}
	photo := Object("ConnectApi.Photo")
	photo.Fields["id"] = String(scalarText(args[1]))
	return photo, nil
}

func (vm *VM) connectAPIUserSetPhoto(args []Value) (Value, error) {
	if len(args) != 3 && len(args) != 4 {
		return Null, fmt.Errorf("ConnectApi.UserProfiles.setPhoto expects 3 or 4 arguments")
	}
	return Object("ConnectApi.Photo"), nil
}

func (vm *VM) connectAPIUserDeletePhoto(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("ConnectApi.UserProfiles.deletePhoto expects 2 arguments")
	}
	return Null, nil
}

func scalarText(value Value) string {
	switch value.Kind {
	case ValueString:
		return value.Text
	case ValueObject:
		if value.Type == "Id" || value.Type == "URL" {
			if text, ok := platformScalarObjectText(value); ok {
				return text
			}
			return value.Text
		}
	}
	return ""
}

func objectFieldFold(value Value, key string) (Value, bool) {
	if value.Kind != ValueObject || value.Fields == nil {
		return Null, false
	}
	for name, field := range value.Fields {
		if strings.EqualFold(name, key) {
			return field, true
		}
	}
	return Null, false
}

func (vm *VM) customDataCachedValue(key string) (Value, bool) {
	if key == "" || vm.customDataCache == nil {
		return Null, false
	}
	value, ok := vm.customDataCache[key]
	return value, ok
}

func (vm *VM) storeCustomDataCachedValue(key string, value Value) Value {
	if key == "" {
		return value
	}
	if vm.customDataCache == nil {
		vm.customDataCache = make(map[string]Value)
	}
	vm.customDataCache[key] = value
	return value
}

func (vm *VM) clearCustomDataCache() {
	if len(vm.customDataCache) > 0 {
		vm.customDataCache = make(map[string]Value)
	}
}

func (vm *VM) clearMetadataCaches() {
	if vm != nil && vm.Org != nil {
		vm.Org.ClearRuntimeSchemaStamp()
	}
	vm.describeCache = make(map[string]Value)
	vm.fieldDescribeCache = make(map[string]Value)
	vm.describeDefCache = make(map[string]storage.ObjectDefinition)
	vm.globalDescribeCache = nil
	vm.describeTabsCache = nil
	vm.childRelCache = newChildRelationshipCache()
	vm.childRelationshipLookupCache = newChildRelationshipLookupCache()
	vm.jsonChildRelTypeCache = newJSONChildRelTypeLookupCache()
	vm.sObjectFieldAliasCache = newSObjectFieldAliasLookupCache()
	vm.fieldResolveCache = newFieldResolveLookupCache()
	vm.soqlExecutionCache = nil
	vm.dmlSummaryByChild = dml.NewSummaryRelationCache()
	vm.summarySideEffectObjects = nil
	vm.summarySideEffectIndex = nil
	vm.loadedChildRelCache = newLoadedChildRelationshipLookupCache()
	vm.lazyChildRelCache = newLazyChildRelationshipLookupCache()
	vm.objectNameCache = make(map[string]objectNameLookup)
	vm.metadataCacheStamp = ""
	vm.clearCustomDataCache()
}

func (vm *VM) callCustomDataStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	methodKey := strings.ToLower(method)
	objectName, definition, kind, ok := vm.customDataObject(typeName)
	if !ok {
		if objectName, definition, kind, ok = vm.metadataLightCustomSettingObject(typeName, methodKey); ok {
			goto dispatchCustomData
		}
		if (methodKey == "getorgdefaults" || methodKey == "getvalues") && hasSuffixFold(typeName, "__c") {
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
			}
			return Object(typeName), true, nil
		}
		return Null, false, nil
	}
dispatchCustomData:
	switch methodKey {
	case "getall":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getAll expects 0 arguments", typeName)
		}
		if err := unsupportedHierarchyCustomSettingStatic(definition, typeName, method); err != nil {
			return Null, true, err
		}
		cacheKey := "getAll:" + strings.ToLower(objectName)
		if cached, ok := vm.customDataCachedValue(cacheKey); ok {
			return cached, true, nil
		}
		out := Map()
		out.Type = "Map<String," + objectName + ">"
		namespace := ""
		var records []storage.Record
		if vm.Org != nil {
			namespace = vm.Org.Namespace
			object := vm.Org.Objects[objectName]
			records = make([]storage.Record, 0, len(object.Records))
			for _, record := range object.Records {
				if record.System.IsDeleted {
					continue
				}
				records = append(records, record)
			}
		}
		sort.Slice(records, func(i, j int) bool {
			return customDataRecordLess(definition, kind, records[i], records[j], namespace)
		})
		for _, record := range records {
			key := customDataRecordKey(definition, kind, record, namespace)
			if key == "" {
				continue
			}
			out.Map[mapKey(String(key))] = vm.readOnlyCustomDataValue(record, kind)
		}
		return vm.storeCustomDataCachedValue(cacheKey, out), true, nil
	case "getinstance":
		if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
			if len(args) > 1 {
				return Null, true, fmt.Errorf("%s.getInstance expects optional setup owner Id", typeName)
			}
		} else if err := unsupportedHierarchyCustomSettingStatic(definition, typeName, method); err != nil {
			return Null, true, err
		}
		cacheKey := "getInstance:" + strings.ToLower(objectName) + ":" + customDataArgsCacheKey(args)
		if len(args) == 1 && args[0].Kind == ValueNull {
			// The retained legacy null-name lookup differs from R263 at the
			// supported source APIs, so it cannot share the same cache entry.
			cacheKey += ":" + vm.currentMethod.APIVersion
		}
		if cached, ok := vm.customDataCachedValue(cacheKey); ok {
			return cached, true, nil
		}
		if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
			value, err := vm.hierarchyCustomSettingInstance(objectName, kind, args)
			if err != nil {
				return Null, true, err
			}
			return vm.storeCustomDataCachedValue(cacheKey, value), true, nil
		}
		record, found, err := vm.customDataGetInstance(objectName, definition, kind, args)
		if err != nil || !found {
			if err == nil {
				return vm.storeCustomDataCachedValue(cacheKey, typedNull(objectName)), true, nil
			}
			return Null, true, err
		}
		return vm.storeCustomDataCachedValue(cacheKey, vm.readOnlyCustomDataValue(record, kind)), true, nil
	case "getorgdefaults", "getvalues":
		if strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
			switch methodKey {
			case "getorgdefaults":
				if len(args) != 0 {
					return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
				}
				cacheKey := "getOrgDefaults:" + strings.ToLower(objectName)
				if cached, ok := vm.customDataCachedValue(cacheKey); ok {
					return cached, true, nil
				}
				return vm.storeCustomDataCachedValue(cacheKey, vm.hierarchyCustomSettingOrgDefaults(objectName, kind)), true, nil
			case "getvalues":
				if len(args) > 1 {
					return Null, true, fmt.Errorf("%s.getValues expects optional setup owner Id", typeName)
				}
				var ownerID string
				if len(args) == 1 {
					var ok bool
					var err error
					ownerID, ok, err = customSettingOwnerIDArg(args[0])
					if err != nil {
						return Null, true, err
					}
					if !ok {
						return Null, true, fmt.Errorf("%s.getValues expects optional setup owner Id", typeName)
					}
					if err := validateCustomSettingOwnerID(ownerID); err != nil {
						return Null, true, err
					}
				}
				cacheKey := "getValues:" + strings.ToLower(objectName) + ":" + customDataArgsCacheKey(args)
				if cached, ok := vm.customDataCachedValue(cacheKey); ok {
					return cached, true, nil
				}
				if len(args) == 1 && ownerID != "" {
					if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, ownerID); found {
						return vm.storeCustomDataCachedValue(cacheKey, vm.readOnlyCustomDataValue(record, kind)), true, nil
					}
					return vm.storeCustomDataCachedValue(cacheKey, typedNull(objectName)), true, nil
				}
				return vm.storeCustomDataCachedValue(cacheKey, vm.hierarchyCustomSettingOrgDefaults(objectName, kind)), true, nil
			}
		}
		if methodKey == "getvalues" && len(args) == 1 {
			if args[0].Kind != ValueString && args[0].Kind != ValueNull {
				return Null, true, fmt.Errorf("%s.getValues expects optional String name", typeName)
			}
			cacheKey := "getValues:" + strings.ToLower(objectName) + ":" + customDataArgsCacheKey(args)
			if cached, ok := vm.customDataCachedValue(cacheKey); ok {
				return cached, true, nil
			}
			// A null name never selects a populated list setting.
			if args[0].Kind == ValueNull {
				return vm.storeCustomDataCachedValue(cacheKey, typedNull(objectName)), true, nil
			}
			record, found, err := vm.customDataGetInstance(objectName, definition, kind, args)
			if err != nil || !found {
				if err == nil {
					return vm.storeCustomDataCachedValue(cacheKey, typedNull(objectName)), true, nil
				}
				return Null, true, err
			}
			return vm.storeCustomDataCachedValue(cacheKey, vm.readOnlyCustomDataValue(record, kind)), true, nil
		}
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
		}
		cacheKey := methodKey + ":" + strings.ToLower(objectName)
		if cached, ok := vm.customDataCachedValue(cacheKey); ok {
			return cached, true, nil
		}
		record, found := vm.customDataOrgDefaultRecord(objectName)
		if !found {
			return vm.storeCustomDataCachedValue(cacheKey, vm.readOnlyCustomDataDefaultValue(objectName, kind)), true, nil
		}
		return vm.storeCustomDataCachedValue(cacheKey, vm.readOnlyCustomDataValue(record, kind)), true, nil
	default:
		return Null, false, nil
	}
}

func (vm *VM) ensureAsyncObjects() {
	if vm.Org == nil {
		return
	}
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "AsyncApexJob",
		Label:     "Async Apex Job",
		KeyPrefix: "707",
		Fields: map[string]storage.Field{
			"Id":                  {APIName: "Id", Type: storage.FieldID},
			"Status":              {APIName: "Status", Type: storage.FieldString},
			"JobType":             {APIName: "JobType", Type: storage.FieldString},
			"ApexClassId":         {APIName: "ApexClassId", Type: storage.FieldReference, ReferenceTo: []string{"ApexClass"}, RelationshipName: "ApexClass"},
			"CronTriggerId":       {APIName: "CronTriggerId", Type: storage.FieldReference, ReferenceTo: []string{"CronTrigger"}, RelationshipName: "CronTrigger"},
			"ApexClassName":       {APIName: "ApexClassName", Type: storage.FieldString},
			"MethodName":          {APIName: "MethodName", Type: storage.FieldString},
			"CreatedDate":         {APIName: "CreatedDate", Type: storage.FieldDateTime},
			"CreatedById":         {APIName: "CreatedById", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"},
			"LastModifiedDate":    {APIName: "LastModifiedDate", Type: storage.FieldDateTime},
			"LastModifiedById":    {APIName: "LastModifiedById", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"},
			"SystemModstamp":      {APIName: "SystemModstamp", Type: storage.FieldDateTime},
			"CompletedDate":       {APIName: "CompletedDate", Type: storage.FieldDateTime},
			"TotalJobItems":       {APIName: "TotalJobItems", Type: storage.FieldInteger},
			"JobItemsProcessed":   {APIName: "JobItemsProcessed", Type: storage.FieldInteger},
			"NumberOfErrors":      {APIName: "NumberOfErrors", Type: storage.FieldInteger},
			"ExtendedStatus":      {APIName: "ExtendedStatus", Type: storage.FieldString},
			"ParentJobId":         {APIName: "ParentJobId", Type: storage.FieldReference, ReferenceTo: []string{"AsyncApexJob"}, RelationshipName: "ParentJob"},
			"LastProcessed":       {APIName: "LastProcessed", Type: storage.FieldString},
			"LastProcessedOffset": {APIName: "LastProcessedOffset", Type: storage.FieldInteger},
		},
		Relations: []storage.Relationship{{
			Field:              "ApexClassId",
			ParentObjects:      []string{"ApexClass"},
			ParentRelationship: "ApexClass",
		}, {
			Field:              "CronTriggerId",
			ParentObjects:      []string{"CronTrigger"},
			ParentRelationship: "CronTrigger",
		}, {
			Field:              "ParentJobId",
			ParentObjects:      []string{"AsyncApexJob"},
			ParentRelationship: "ParentJob",
		}},
	})
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "ApexClass",
		Label:     "Apex Class",
		KeyPrefix: "01p",
		Fields: map[string]storage.Field{
			"Id":              {APIName: "Id", Type: storage.FieldID},
			"Name":            {APIName: "Name", Type: storage.FieldString},
			"NamespacePrefix": {APIName: "NamespacePrefix", Type: storage.FieldString},
		},
	})
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "CronTrigger",
		Label:     "Cron Trigger",
		KeyPrefix: "08e",
		Fields: map[string]storage.Field{
			"Id":              {APIName: "Id", Type: storage.FieldID},
			"State":           {APIName: "State", Type: storage.FieldString},
			"CronExpression":  {APIName: "CronExpression", Type: storage.FieldString},
			"CronJobDetailId": {APIName: "CronJobDetailId", Type: storage.FieldReference, ReferenceTo: []string{"CronJobDetail"}, RelationshipName: "CronJobDetail"},
			"NextFireTime":    {APIName: "NextFireTime", Type: storage.FieldDateTime},
			"TimesTriggered":  {APIName: "TimesTriggered", Type: storage.FieldInteger},
		},
		Relations: []storage.Relationship{{
			Field:              "CronJobDetailId",
			ParentObjects:      []string{"CronJobDetail"},
			ParentRelationship: "CronJobDetail",
		}},
	})
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "CronJobDetail",
		Label:     "Cron Job Detail",
		KeyPrefix: "08a",
		Fields: map[string]storage.Field{
			"Id":      {APIName: "Id", Type: storage.FieldID},
			"Name":    {APIName: "Name", Type: storage.FieldString},
			"JobType": {APIName: "JobType", Type: storage.FieldString},
		},
	})
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "User",
		Label:     "User",
		KeyPrefix: "005",
		Fields: map[string]storage.Field{
			"Id":        {APIName: "Id", Type: storage.FieldID},
			"Username":  {APIName: "Username", Type: storage.FieldString},
			"ProfileId": {APIName: "ProfileId", Type: storage.FieldString},
		},
	})
	ensureObject(vm.Org, storage.ObjectDefinition{
		APIName:   "Profile",
		Label:     "Profile",
		KeyPrefix: "00e",
		Fields: map[string]storage.Field{
			"Id":   {APIName: "Id", Type: storage.FieldID},
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	})
}
