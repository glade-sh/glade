package vm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) eventBusPublish(args []Value, result *Result) (Value, error) {
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
	results := make([]Value, 0, len(records))
	triggerRecords := make([]storage.Record, 0, len(records))
	eventUUIDs := make([]string, 0, len(records))
	for _, record := range records {
		if record.Kind != ValueObject {
			return Null, fmt.Errorf("EventBus.publish expects SObject event record(s)")
		}
		if len(args) == 2 {
			uuid, ok := platformEventUUID(record)
			if !ok {
				return Null, fmt.Errorf("EventBus.publish with callback requires platform event records with EventUuid")
			}
			eventUUIDs = append(eventUUIDs, uuid)
		}
		stored, err := vm.recordFromValue(&record)
		if err != nil {
			return Null, err
		}
		if strings.HasSuffix(strings.ToLower(stored.Object), "__e") {
			if field, ok := vm.missingRequiredPlatformEventField(stored); ok {
				row := Object("Database.SaveResult")
				row.Fields["success"] = Bool(false)
				row.Fields["id"] = Null
				row.Fields["error"] = String("REQUIRED_FIELD_MISSING, required field " + stored.Object + "." + field + " is missing")
				errValue := Object("Database.Error")
				errValue.Fields["message"] = String("required field " + stored.Object + "." + field + " is missing")
				errValue.Fields["statusCode"] = String("REQUIRED_FIELD_MISSING")
				errValue.Fields["fields"] = List(String(field))
				row.Fields["errors"] = List(errValue)
				results = append(results, row)
				continue
			}
		}
		triggerRecords = append(triggerRecords, stored)
		row := Object("Database.SaveResult")
		row.Fields["success"] = Bool(true)
		row.Fields["id"] = Null
		row.Fields["error"] = String("")
		row.Fields["errors"] = List()
		results = append(results, row)
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
		if !ok && strings.HasSuffix(strings.ToLower(name), "__c") {
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

func platformEventUUID(record Value) (string, bool) {
	if !strings.HasSuffix(strings.ToLower(record.Type), "__e") {
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

func (vm *VM) connectAPICommunity(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("ConnectApi.Communities.getCommunity expects 1 argument")
	}
	networkID := scalarText(args[0])
	if networkID == "" {
		networkID = vm.firstOrgRecordID("Network", "0DB000000000001")
	}
	prefix := strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")
	community := Object("ConnectApi.Community")
	community.Fields["id"] = String(networkID)
	community.Fields["name"] = String(vm.firstOrgRecordString("Network", "Name", "Local Community"))
	community.Fields["urlPathPrefix"] = String(prefix)
	community.Fields["siteUrl"] = String(strings.TrimRight(vm.salesforceBaseURL(), "/") + "/" + prefix)
	return community, nil
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
	if len(args) != 4 {
		return Null, fmt.Errorf("ConnectApi.UserProfiles.setPhoto expects 4 arguments")
	}
	return Null, nil
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
	vm.describeCache = make(map[string]Value)
	vm.fieldDescribeCache = make(map[string]Value)
	vm.describeDefCache = make(map[string]storage.ObjectDefinition)
	vm.globalDescribeCache = nil
	vm.describeTabsCache = nil
	vm.childRelCache = make(map[string][]Value)
	vm.jsonChildRelTypeCache = newJSONChildRelTypeLookupCache()
	vm.loadedChildRelCache = make(map[string]loadedChildRelationshipLookup)
	vm.lazyChildRelCache = make(map[string]lazyChildRelationshipLookup)
	vm.objectNameCache = make(map[string]objectNameLookup)
	vm.metadataCacheStamp = ""
	vm.clearCustomDataCache()
}

func (vm *VM) callCustomDataStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	methodKey := strings.ToLower(method)
	objectName, definition, kind, ok := vm.customDataObject(typeName)
	if !ok {
		if (methodKey == "getorgdefaults" || methodKey == "getvalues") && strings.HasSuffix(strings.ToLower(typeName), "__c") {
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
			}
			return Object(typeName), true, nil
		}
		return Null, false, nil
	}
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
		object := vm.Org.Objects[objectName]
		records := make([]storage.Record, 0, len(object.Records))
		for _, record := range object.Records {
			if record.System.IsDeleted {
				continue
			}
			records = append(records, record)
		}
		sort.Slice(records, func(i, j int) bool {
			return customDataRecordLess(definition, kind, records[i], records[j], vm.Org.Namespace)
		})
		for _, record := range records {
			key := customDataRecordKey(definition, kind, record, vm.Org.Namespace)
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
		if cached, ok := vm.customDataCachedValue(cacheKey); ok {
			return cached, true, nil
		}
		record, found, err := vm.customDataGetInstance(objectName, definition, kind, args)
		if err != nil || !found {
			if err == nil && strings.EqualFold(definition.Metadata["customSettingsType"], "Hierarchy") {
				return vm.storeCustomDataCachedValue(cacheKey, vm.readOnlyCustomDataDefaultValue(objectName, kind)), true, nil
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
				if len(args) == 1 && args[0].Kind != ValueString && args[0].Kind != ValueNull {
					return Null, true, fmt.Errorf("%s.getValues expects optional setup owner Id", typeName)
				}
				cacheKey := "getValues:" + strings.ToLower(objectName) + ":" + customDataArgsCacheKey(args)
				if cached, ok := vm.customDataCachedValue(cacheKey); ok {
					return cached, true, nil
				}
				if len(args) == 1 && args[0].Kind == ValueString {
					if record, found := vm.hierarchyCustomSettingRecordForOwner(objectName, args[0].Text); found {
						return vm.storeCustomDataCachedValue(cacheKey, vm.readOnlyCustomDataValue(record, kind)), true, nil
					}
					return vm.storeCustomDataCachedValue(cacheKey, vm.readOnlyCustomDataDefaultValue(objectName, kind)), true, nil
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
			if args[0].Kind == ValueNull {
				return vm.storeCustomDataCachedValue(cacheKey, vm.readOnlyCustomDataDefaultValue(objectName, kind)), true, nil
			}
			record, found, err := vm.customDataGetInstance(objectName, definition, kind, args)
			if err != nil || !found {
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
			"Id":                {APIName: "Id", Type: storage.FieldID},
			"Status":            {APIName: "Status", Type: storage.FieldString},
			"JobType":           {APIName: "JobType", Type: storage.FieldString},
			"ApexClassId":       {APIName: "ApexClassId", Type: storage.FieldReference, ReferenceTo: []string{"ApexClass"}, RelationshipName: "ApexClass"},
			"CronTriggerId":     {APIName: "CronTriggerId", Type: storage.FieldReference, ReferenceTo: []string{"CronTrigger"}, RelationshipName: "CronTrigger"},
			"ApexClassName":     {APIName: "ApexClassName", Type: storage.FieldString},
			"MethodName":        {APIName: "MethodName", Type: storage.FieldString},
			"CreatedDate":       {APIName: "CreatedDate", Type: storage.FieldDateTime},
			"CreatedById":       {APIName: "CreatedById", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"},
			"LastModifiedDate":  {APIName: "LastModifiedDate", Type: storage.FieldDateTime},
			"LastModifiedById":  {APIName: "LastModifiedById", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"},
			"SystemModstamp":    {APIName: "SystemModstamp", Type: storage.FieldDateTime},
			"CompletedDate":     {APIName: "CompletedDate", Type: storage.FieldDateTime},
			"TotalJobItems":     {APIName: "TotalJobItems", Type: storage.FieldInteger},
			"JobItemsProcessed": {APIName: "JobItemsProcessed", Type: storage.FieldInteger},
			"NumberOfErrors":    {APIName: "NumberOfErrors", Type: storage.FieldInteger},
			"ExtendedStatus":    {APIName: "ExtendedStatus", Type: storage.FieldString},
		},
		Relations: []storage.Relationship{{
			Field:              "ApexClassId",
			ParentObjects:      []string{"ApexClass"},
			ParentRelationship: "ApexClass",
		}, {
			Field:              "CronTriggerId",
			ParentObjects:      []string{"CronTrigger"},
			ParentRelationship: "CronTrigger",
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
			"CronJobDetail":   {APIName: "CronJobDetail", Type: storage.FieldString},
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
