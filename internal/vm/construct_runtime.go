package vm

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) constructValueWithLiteral(typeName string, args []Value, namedArgs map[string]Value, result *Result, literalArgs bool) (Value, error) {
	if resolved := vm.resolveTypeNameInClass(vm.currentClass, typeName); resolved != "" && !strings.EqualFold(resolved, typeName) {
		typeName = resolved
	} else if resolved, ok := vm.resolveClassName(typeName); ok {
		if !strings.Contains(typeName, ".") && !strings.Contains(resolved, ".") {
			if nested, nestedOK := vm.resolveUniqueNestedTypeName(typeName); nestedOK {
				typeName = nested
			} else {
				typeName = resolved
			}
		} else {
			typeName = resolved
		}
	} else if resolved, ok := vm.resolveUniqueNestedTypeName(typeName); ok {
		typeName = resolved
	} else {
		typeName = canonicalRuntimeTypeName(typeName)
	}
	switch {
	case collectionBase(typeName) == "List":
		if len(namedArgs) > 0 {
			return Null, fmt.Errorf("List constructor does not accept named fields")
		}
		if !literalArgs && len(args) == 1 && args[0].Kind == ValueList {
			if elementType, ok := collectionElementType(typeName); ok && collectionBase(elementType) != "" {
				if element, err := vm.coerceAssignable(elementType, args[0]); err == nil {
					return vm.coerceAssignable(typeName, List(element))
				}
			}
		}
		if !literalArgs && len(args) == 1 && (args[0].Kind == ValueList || args[0].Kind == ValueSet) {
			value := List(append([]Value(nil), collectionMembers(args[0])...)...)
			return vm.coerceAssignable(typeName, value)
		}
		return vm.coerceAssignable(typeName, List(args...))
	case collectionBase(typeName) == "Set":
		if len(namedArgs) > 0 {
			return Null, fmt.Errorf("Set constructor does not accept named fields")
		}
		if !literalArgs && len(args) == 1 && (args[0].Kind == ValueList || args[0].Kind == ValueSet) {
			value := Set(collectionMembers(args[0])...)
			return vm.coerceAssignable(typeName, value)
		}
		return vm.coerceAssignable(typeName, Set(args...))
	case isMapType(typeName):
		if len(namedArgs) != 0 {
			return Null, fmt.Errorf("Map constructor does not accept named fields")
		}
		if len(args) > 0 && allMapEntryValues(args) {
			value := Map()
			value.Type = typeName
			for _, entry := range args {
				keyValue := entry.Fields["__key"]
				item := entry.Fields["__value"]
				key, coerced, err := vm.coerceMapEntry(typeName, keyValue, item)
				if err != nil {
					return Null, fmt.Errorf("Map constructor: %w", err)
				}
				encodedKey := vm.mapKey(key)
				if _, exists := value.Map[encodedKey]; !exists {
					value.MapOrder = append(value.MapOrder, encodedKey)
				}
				value.Map[encodedKey] = coerced
				value.MapKeys[encodedKey] = key
			}
			return value, nil
		}
		if len(args) == 1 && args[0].Kind == ValueMap {
			value := Map()
			value.Type = typeName
			for _, rawKey := range orderedValueMapKeys(args[0]) {
				item := args[0].Map[rawKey]
				keyValue := mapStoredKey(args[0], rawKey)
				key, coerced, err := vm.coerceMapEntry(typeName, keyValue, item)
				if err != nil {
					return Null, fmt.Errorf("Map constructor: %w", err)
				}
				encodedKey := vm.mapKey(key)
				if _, exists := value.Map[encodedKey]; !exists {
					value.MapOrder = append(value.MapOrder, encodedKey)
				}
				value.Map[encodedKey] = coerced
				value.MapKeys[encodedKey] = key
			}
			return value, nil
		}
		if len(args) == 1 && (args[0].Kind == ValueList || args[0].Kind == ValueMap || typedNullCollectionBase(args[0]) == "List") {
			source := args[0]
			if source.Kind == ValueNull {
				source = Value{Kind: ValueList, Type: source.Type}
			}
			if source.Kind == ValueMap {
				records, ok := queryResultRecordsList(source)
				if !ok {
					return Null, fmt.Errorf("Map constructor does not accept positional values")
				}
				source = records
			}
			value, err := vm.mapFromSObjectList(typeName, source)
			if err != nil {
				return Null, err
			}
			return value, nil
		}
		if len(args) != 0 {
			return Null, fmt.Errorf("Map constructor does not accept positional values (%s)", valueShape(args[0]))
		}
		value := Map()
		value.Type = typeName
		return value, nil
	}
	if strings.EqualFold(typeName, "XmlStreamReader") {
		if len(args) != 1 || len(namedArgs) != 0 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("XmlStreamReader constructor expects XML String")
		}
		return newXmlStreamReader(args[0].Text)
	}
	if strings.EqualFold(typeName, "Continuation") {
		return newContinuation(args, namedArgs)
	}
	if vm.isSObjectUnitOfWorkBaseType(typeName) {
		uow, err := vm.constructFrameworkSObjectUnitOfWork(args, namedArgs)
		if err != nil {
			return Null, err
		}
		uow.Type = typeName
		return uow, nil
	}
	if strings.EqualFold(typeName, "VisualEditor.DataRow") {
		if (len(args) != 2 && len(args) != 3) || len(namedArgs) != 0 {
			return Null, fmt.Errorf("VisualEditor.DataRow constructor expects label, value, and optional selected")
		}
		if args[0].Kind != ValueString {
			return Null, fmt.Errorf("VisualEditor.DataRow constructor expects label String")
		}
		if len(args) == 3 && args[2].Kind != ValueBool {
			return Null, fmt.Errorf("VisualEditor.DataRow constructor selected expects Boolean")
		}
		row := Object("VisualEditor.DataRow")
		row.Fields["label"] = args[0]
		row.Fields["value"] = args[1]
		row.Fields["selected"] = Bool(false)
		if len(args) == 3 {
			row.Fields["selected"] = args[2]
		}
		return row, nil
	}
	if class, ok := vm.Classes[typeName]; ok {
		if class.IsInterface {
			return Null, fmt.Errorf("cannot instantiate interface %s", typeName)
		}
		// Standard/custom SObject constructor forms (for example `new Product2(Name='x')`)
		// should not be blocked by unrelated package class aliases that share the same name.
		if !vm.isSObjectType(typeName) {
			if err := vm.checkClassAccess(class.Name, typeName); err != nil {
				return Null, err
			}
		}
		if class.IsAbstract {
			return Null, fmt.Errorf("cannot instantiate abstract class %s", typeName)
		}
		if len(class.EnumValues) > 0 {
			return Null, fmt.Errorf("cannot instantiate enum %s", typeName)
		}
		if value, handled, err := vm.constructFrameworkFastDTO(typeName, args, namedArgs); handled || err != nil {
			return value, err
		}
		if err := vm.ensureClassInitialized(class.Name); err != nil {
			return Null, err
		}
		class, _ = vm.lookupClass(typeName)
		passiveDTO := passiveRuntimeClass(class) && vm.isPassivePlatformDTOType(typeName)
		frameworkUOWRuntime := vm.isSObjectUnitOfWorkRuntimeType(typeName)
		object := Object(typeName)
		if !passiveDTO {
			for field, value := range namedArgs {
				value = coerceLikelyCustomNumberRuntimeValue(field, value)
				object.Fields[field] = value
			}
		}
		vm.initializeFields(&object, typeName)
		if passiveDTO || frameworkUOWRuntime {
			vm.initializePassiveCollectionFields(&object, typeName)
		}
		if passiveDTO {
			vm.bindPassiveNamedConstructorFields(&object, namedArgs)
		} else {
			for field, value := range namedArgs {
				value = coerceLikelyCustomNumberRuntimeValue(field, value)
				object.Fields[field] = value
			}
		}
		if err := vm.runInstanceInitializers(class, object, result); err != nil {
			return Null, err
		}
		isSObjectCtor := vm.isSObjectType(typeName)
		if !passiveDTO {
			delete(object.Fields, sobjectExplicitFieldsField)
			for field, value := range namedArgs {
				value = coerceLikelyCustomNumberRuntimeValue(field, value)
				if isSObjectCtor {
					setExplicitSObjectField(&object, field, value)
				} else {
					object.Fields[field] = value
				}
			}
		}
		ctorArgs := args
		ctor, ok, ambiguous := vm.matchConstructor(class, args)
		if passiveDTO && len(namedArgs) != 0 {
			ctor, ctorArgs, ok, ambiguous = vm.matchConstructorWithNamedArgs(class, args, namedArgs)
		}
		if ok {
			if err := vm.callImplicitSuperConstructor(class, ctor, object, result); err != nil {
				return Null, err
			}
			constructed, err := vm.callConstructorWithReceiver(class, ctor, object, ctorArgs, result)
			if err != nil {
				return Null, err
			}
			if constructed.Kind == ValueObject {
				object = constructed
			}
			if passiveDTO {
				bindPassiveConstructorArgs(&object, ctor, ctorArgs)
			}
		} else if ambiguous {
			return Null, fmt.Errorf("ambiguous %s constructor with %d argument(s)", typeName, len(args))
		} else if len(args) != 0 {
			if isExceptionType(typeName) {
				handled, err := applyExceptionConstructorArgs(&object, args)
				if err != nil {
					return Null, err
				}
				if handled {
					if err := vm.callImplicitSuperConstructor(class, Method{}, object, result); err != nil {
						return Null, err
					}
					return object, nil
				}
			}
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		} else if err := vm.callImplicitSuperConstructor(class, Method{}, object, result); err != nil {
			return Null, err
		}
		if !passiveDTO {
			delete(object.Fields, sobjectExplicitFieldsField)
			for field, value := range namedArgs {
				if isSObjectCtor {
					setExplicitSObjectField(&object, field, value)
				} else {
					object.Fields[field] = value
				}
			}
		}
		return object, nil
	}
	switch typeName {
	case "Apex.Stack":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Apex.Stack constructor expects 0 arguments")
		}
		stack := Object("Apex.Stack")
		stack.Fields["values"] = List()
		return stack, nil
	case "ApexPages.Action":
		if len(args) != 1 || len(namedArgs) != 0 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("ApexPages.Action constructor expects expression String")
		}
		action := Object("ApexPages.Action")
		action.Fields["expression"] = args[0]
		return action, nil
	case "ApexPages.Component", "ApexPages.ComponentIteration":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return Object(typeName), nil
	case "HttpRequest":
		if len(args) != 0 {
			return Null, fmt.Errorf("HttpRequest constructor expects 0 arguments")
		}
		request := newHttpRequest()
		for field, value := range namedArgs {
			request.Fields[field] = value
		}
		return request, nil
	case "HttpResponse":
		if len(args) != 0 {
			return Null, fmt.Errorf("HttpResponse constructor expects 0 arguments")
		}
		response := newHttpResponse()
		for field, value := range namedArgs {
			response.Fields[field] = value
		}
		return response, nil
	case "Database.DMLOptions", "DMLOptions":
		if len(args) != 0 {
			return Null, fmt.Errorf("Database.DMLOptions constructor expects 0 arguments")
		}
		options := newDatabaseDMLOptions()
		for field, value := range namedArgs {
			options.Fields[field] = value
		}
		return options, nil
	case "Database.UnitOfWork":
		if len(args) != 0 {
			return Null, fmt.Errorf("Database.UnitOfWork constructor expects 0 arguments")
		}
		if len(namedArgs) != 0 {
			return Null, fmt.Errorf("Database.UnitOfWork constructor does not accept named arguments")
		}
		return newDatabaseUnitOfWork(), nil
	case "Database.AssignmentRuleHeader":
		if len(args) != 0 {
			return Null, fmt.Errorf("Database.AssignmentRuleHeader constructor expects 0 arguments")
		}
		return newDatabaseHeaderObject("Database.AssignmentRuleHeader"), nil
	case "Database.DuplicateRuleHeader":
		if len(args) != 0 {
			return Null, fmt.Errorf("Database.DuplicateRuleHeader constructor expects 0 arguments")
		}
		return newDatabaseHeaderObject("Database.DuplicateRuleHeader"), nil
	case "Database.EmailHeader":
		if len(args) != 0 {
			return Null, fmt.Errorf("Database.EmailHeader constructor expects 0 arguments")
		}
		return newDatabaseHeaderObject("Database.EmailHeader"), nil
	case "Database.LocaleOptions":
		if len(args) != 0 {
			return Null, fmt.Errorf("Database.LocaleOptions constructor expects 0 arguments")
		}
		return Object("Database.LocaleOptions"), nil
	case "AsyncOptions":
		if len(args) != 0 {
			return Null, fmt.Errorf("AsyncOptions constructor expects 0 arguments")
		}
		options := Object("AsyncOptions")
		for field, value := range namedArgs {
			options.Fields[field] = value
		}
		return options, nil
	case "StaticResourceCalloutMock", "MultiStaticResourceCalloutMock":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		mock := Object(typeName)
		mock.Fields["headers"] = typedMap("Map<String,String>")
		mock.Fields["statusCode"] = Int(200)
		if typeName == "MultiStaticResourceCalloutMock" {
			mock.Fields["staticResources"] = typedMap("Map<String,String>")
		}
		for field, value := range namedArgs {
			mock.Fields[field] = value
		}
		return mock, nil
	case "RestRequest":
		if len(args) != 0 {
			return Null, fmt.Errorf("RestRequest constructor expects 0 arguments")
		}
		request := newRestRequest()
		for field, value := range namedArgs {
			request.Fields[field] = value
		}
		return request, nil
	case "RestResponse":
		if len(args) != 0 {
			return Null, fmt.Errorf("RestResponse constructor expects 0 arguments")
		}
		response := newRestResponse()
		for field, value := range namedArgs {
			response.Fields[field] = value
		}
		return response, nil
	case "Continuation":
		return newContinuation(args, namedArgs)
	case "PageReference", "ApexPages.PageReference":
		if len(args) > 1 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("PageReference constructor expects optional URL String")
		}
		rawURL := ""
		if len(args) == 1 {
			if args[0].Kind != ValueString {
				return Null, fmt.Errorf("PageReference constructor expects URL String")
			}
			rawURL = args[0].Text
		}
		return vm.newPageReference(rawURL), nil
	case "Cookie", "System.Cookie":
		if len(namedArgs) != 0 {
			return Null, fmt.Errorf("Cookie constructor does not support named arguments")
		}
		return newCookie(args)
	case "Domain":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Domain constructor expects 0 arguments")
		}
		return newDomainFromHostname(localDomainHostname("ORG_MY_DOMAIN", "")), nil
	case "DomainCreator":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("DomainCreator constructor expects 0 arguments")
		}
		return Object("DomainCreator"), nil
	case "VisualEditor.DataRow":
		if (len(args) != 2 && len(args) != 3) || len(namedArgs) != 0 {
			return Null, fmt.Errorf("VisualEditor.DataRow constructor expects label, value, and optional selected")
		}
		if args[0].Kind != ValueString {
			return Null, fmt.Errorf("VisualEditor.DataRow constructor expects label String")
		}
		if len(args) == 3 && args[2].Kind != ValueBool {
			return Null, fmt.Errorf("VisualEditor.DataRow constructor selected expects Boolean")
		}
		row := Object("VisualEditor.DataRow")
		row.Fields["label"] = args[0]
		row.Fields["value"] = args[1]
		row.Fields["selected"] = Bool(false)
		if len(args) == 3 {
			row.Fields["selected"] = args[2]
		}
		return row, nil
	case "VisualEditor.DynamicPickListRows":
		if len(args) > 2 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("VisualEditor.DynamicPickListRows constructor expects rows and optional containsAllRows")
		}
		rows := Object("VisualEditor.DynamicPickListRows")
		list := typedList("List<VisualEditor.DataRow>")
		if len(args) > 0 {
			if args[0].Kind != ValueList {
				return Null, fmt.Errorf("VisualEditor.DynamicPickListRows constructor expects List<VisualEditor.DataRow>")
			}
			list = args[0]
		}
		rows.Fields["rows"] = list
		rows.Fields["containsAllRows"] = Bool(false)
		if len(args) == 2 {
			if args[1].Kind != ValueBool {
				return Null, fmt.Errorf("VisualEditor.DynamicPickListRows constructor expects Boolean containsAllRows")
			}
			rows.Fields["containsAllRows"] = args[1]
		}
		return rows, nil
	case "compression.ZipWriter":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("compression.ZipWriter constructor expects 0 arguments")
		}
		return newCompressionZipWriter(), nil
	case "compression.ZipReader":
		if len(args) != 1 || len(namedArgs) != 0 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
			return Null, fmt.Errorf("compression.ZipReader constructor expects Blob archive")
		}
		return newCompressionZipReader(args[0])
	case "UserProvisioning.ProvisioningBatchable", "UserProvisioning.PluginBatchable":
		if len(args) != 1 || len(namedArgs) != 0 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("%s constructor expects List<SObject>", typeName)
		}
		value := Object(typeName)
		value.Fields["newRows"] = args[0]
		return value, nil
	case "UserProvisioning.CollectingBatchable":
		if len(args) != 3 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("UserProvisioning.CollectingBatchable constructor expects reconOffset, uprId, connectedAppId")
		}
		value := Object(typeName)
		value.Fields["reconOffset"] = args[0]
		value.Fields["uprId"] = args[1]
		value.Fields["connectedAppId"] = args[2]
		return value, nil
	case "UserProvisioning.LinkingBatchable":
		if len(args) != 1 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("UserProvisioning.LinkingBatchable constructor expects uprId")
		}
		value := Object(typeName)
		value.Fields["uprId"] = args[0]
		return value, nil
	case "Dom.Document":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Dom.Document constructor expects 0 arguments")
		}
		return newDomDocument(), nil
	case "Auth.UserData":
		if len(args) != 11 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Auth.UserData constructor expects 11 arguments")
		}
		data := Object("Auth.UserData")
		for index, field := range []string{"identifier", "firstName", "lastName", "fullName", "email", "link", "username", "locale", "provider", "siteLoginUrl", "attributeMap"} {
			data.Fields[field] = args[index]
		}
		return data, nil
	case "Auth.VerificationResult":
		if len(args) != 3 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Auth.VerificationResult constructor expects redirect, success, message")
		}
		return newAuthVerificationResult(args[0], args[1], args[2]), nil
	case "Auth.AuthConfiguration":
		if len(args) != 2 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Auth.AuthConfiguration constructor expects community URL and start URL")
		}
		config := Object("Auth.AuthConfiguration")
		config.Fields["communityUrl"] = args[0]
		config.Fields["startUrl"] = args[1]
		authConfig := Object("Auth.AuthConfig")
		authConfig.Fields["Url"] = args[0]
		config.Fields["authConfig"] = authConfig
		return config, nil
	case "Auth.JWT":
		if len(args) != 0 {
			return Null, fmt.Errorf("Auth.JWT constructor expects 0 arguments")
		}
		jwt := Object("Auth.JWT")
		for field, value := range namedArgs {
			jwt.Fields[field] = value
		}
		return jwt, nil
	case "Version":
		if len(args) != 2 && len(args) != 3 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Version constructor expects major, minor[, patch]")
		}
		version := Object("Version")
		fields := []string{"major", "minor", "patch"}
		for i, arg := range args {
			if arg.Kind != ValueInt {
				return Null, fmt.Errorf("Version constructor expects Integer components")
			}
			version.Fields[fields[i]] = arg
		}
		if len(args) == 2 {
			version.Fields["patch"] = Int(0)
		}
		version.Fields["__gladePatchSpecified"] = Bool(len(args) == 3)
		return version, nil
	case "Metadata.DeployContainer":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployContainer constructor expects 0 arguments")
		}
		container := Object("Metadata.DeployContainer")
		container.Fields["metadata"] = typedList("List<Metadata.Metadata>")
		for field, value := range namedArgs {
			container.Fields[field] = value
		}
		return container, nil
	case "Metadata.CustomMetadata":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.CustomMetadata constructor expects 0 arguments")
		}
		metadata := Object("Metadata.CustomMetadata")
		metadata.Fields["description"] = Null
		metadata.Fields["fullName"] = Null
		metadata.Fields["label"] = Null
		metadata.Fields["protected_x"] = Bool(false)
		metadata.Fields["values"] = typedList("List<Metadata.CustomMetadataValue>")
		for field, value := range namedArgs {
			metadata.Fields[field] = value
		}
		return metadata, nil
	case "Metadata.CustomMetadataValue":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.CustomMetadataValue constructor expects 0 arguments")
		}
		value := Object("Metadata.CustomMetadataValue")
		value.Fields["field"] = Null
		value.Fields["value"] = Null
		for field, fieldValue := range namedArgs {
			value.Fields[field] = fieldValue
		}
		return value, nil
	case "Metadata.CustomObject":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.CustomObject constructor expects 0 arguments")
		}
		metadata := Object("Metadata.CustomObject")
		metadata.Fields["deploymentStatus"] = Null
		metadata.Fields["description"] = Null
		metadata.Fields["enableActivities"] = Bool(false)
		metadata.Fields["enableReports"] = Bool(false)
		metadata.Fields["fullName"] = Null
		metadata.Fields["label"] = Null
		metadata.Fields["pluralLabel"] = Null
		for field, value := range namedArgs {
			metadata.Fields[field] = value
		}
		return metadata, nil
	case "Metadata.CustomField":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.CustomField constructor expects 0 arguments")
		}
		field := Object("Metadata.CustomField")
		field.Fields["description"] = Null
		field.Fields["externalId"] = Bool(false)
		field.Fields["fullName"] = Null
		field.Fields["label"] = Null
		field.Fields["required"] = Bool(false)
		field.Fields["type"] = Null
		field.Fields["unique"] = Bool(false)
		for name, value := range namedArgs {
			field.Fields[name] = value
		}
		return field, nil
	case "Metadata.Metadata":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.Metadata constructor expects 0 arguments")
		}
		metadata := Object("Metadata.Metadata")
		metadata.Fields["fullName"] = Null
		for field, value := range namedArgs {
			metadata.Fields[field] = value
		}
		return metadata, nil
	case "Metadata.DeployResult":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployResult constructor expects 0 arguments")
		}
		result := Object("Metadata.DeployResult")
		result.Fields["id"] = Null
		result.Fields["status"] = metadataDeployStatusValue("SUCCEEDED")
		result.Fields["success"] = Bool(true)
		result.Fields["done"] = Bool(true)
		result.Fields["numberComponentErrors"] = Int(0)
		result.Fields["numberComponentsDeployed"] = Int(0)
		result.Fields["numberComponentsTotal"] = Int(0)
		result.Fields["numberTestErrors"] = Int(0)
		result.Fields["numberTestsCompleted"] = Int(0)
		result.Fields["checkOnly"] = Bool(false)
		result.Fields["details"] = metadataDeployDetailsObject()
		for field, value := range namedArgs {
			result.Fields[field] = value
		}
		return result, nil
	case "Metadata.DeployDetails":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployDetails constructor expects 0 arguments")
		}
		details := metadataDeployDetailsObject()
		for field, value := range namedArgs {
			details.Fields[field] = value
		}
		return details, nil
	case "Metadata.DeployMessage":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployMessage constructor expects 0 arguments")
		}
		message := Object("Metadata.DeployMessage")
		message.Fields["changed"] = Bool(false)
		message.Fields["columnNumber"] = Int(0)
		message.Fields["componentType"] = Null
		message.Fields["created"] = Bool(false)
		message.Fields["createdDate"] = Null
		message.Fields["deleted"] = Bool(false)
		message.Fields["fileName"] = Null
		message.Fields["fullName"] = Null
		message.Fields["id"] = Null
		message.Fields["lineNumber"] = Int(0)
		message.Fields["problem"] = Null
		message.Fields["problemType"] = Null
		message.Fields["success"] = Bool(false)
		for field, value := range namedArgs {
			message.Fields[field] = value
		}
		return message, nil
	case "Metadata.DeployCallbackContext":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.DeployCallbackContext constructor expects 0 arguments")
		}
		context := Object("Metadata.DeployCallbackContext")
		for field, value := range namedArgs {
			context.Fields[field] = value
		}
		return context, nil
	case "Metadata.AsyncResult":
		if len(args) != 0 {
			return Null, fmt.Errorf("Metadata.AsyncResult constructor expects 0 arguments")
		}
		result := metadataAsyncResultObject("0Af000000000001", true, "Succeeded", "")
		for field, value := range namedArgs {
			result.Fields[field] = value
		}
		return result, nil
	case "SelectOption":
		if len(args) < 2 || len(args) > 4 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("SelectOption constructor expects value, label[, disabled[, escapeItem]]")
		}
		value, err := vm.coerceAssignable("String", args[0])
		if err != nil {
			return Null, fmt.Errorf("SelectOption constructor value expects String: %w", err)
		}
		label, err := vm.coerceAssignable("String", args[1])
		if err != nil {
			return Null, fmt.Errorf("SelectOption constructor label expects String: %w", err)
		}
		disabled := Bool(false)
		escapeItem := Bool(true)
		if len(args) >= 3 {
			if args[2].Kind != ValueBool {
				return Null, fmt.Errorf("SelectOption constructor disabled expects Boolean")
			}
			disabled = args[2]
		}
		if len(args) == 4 {
			if args[3].Kind != ValueBool {
				return Null, fmt.Errorf("SelectOption constructor escapeItem expects Boolean")
			}
			escapeItem = args[3]
		}
		return newSelectOption(value, label, disabled, escapeItem), nil
	case "ApexPages.StandardController":
		if len(args) != 1 || len(namedArgs) != 0 || args[0].Kind != ValueObject {
			return Null, fmt.Errorf("ApexPages.StandardController constructor expects SObject")
		}
		controller := Object("ApexPages.StandardController")
		controller.Fields["record"] = args[0]
		return controller, nil
	case "ApexPages.StandardSetController":
		if len(args) != 1 || len(namedArgs) != 0 || (args[0].Kind != ValueList && !(args[0].Kind == ValueObject && args[0].Type == "Database.QueryLocator")) {
			return Null, fmt.Errorf("ApexPages.StandardSetController constructor expects List or QueryLocator")
		}
		records := args[0]
		if args[0].Kind == ValueObject && args[0].Type == "Database.QueryLocator" {
			if value, ok := args[0].Fields["Records"]; ok {
				records = value
			} else {
				records = List()
			}
		}
		controller := Object("ApexPages.StandardSetController")
		controller.Fields["records"] = records
		controller.Fields["selected"] = List()
		controller.Fields["pageSize"] = Int(20)
		controller.Fields["pageNumber"] = Int(1)
		return controller, nil
	case "ApexPages.Message":
		if len(args) < 2 || len(args) > 3 {
			return Null, fmt.Errorf("ApexPages.Message constructor expects severity, summary[, detail]")
		}
		message := Object("ApexPages.Message")
		message.Fields["severity"] = args[0]
		message.Fields["summary"] = args[1]
		if len(args) == 3 {
			message.Fields["detail"] = args[2]
		}
		return message, nil
	case "Messaging.SendEmailResult":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Messaging.SendEmailResult constructor expects 0 arguments")
		}
		return newSendEmailResult(), nil
	case "Messaging.EmailFileAttachment":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Messaging.EmailFileAttachment constructor expects 0 arguments")
		}
		return newEmailFileAttachment(), nil
	case "Messaging.SingleEmailMessage":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return newSingleEmailMessage(), nil
	case "Messaging.MassEmailMessage":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return newMassEmailMessage(), nil
	case "Messaging.SendEmailOptions":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return Object(typeName), nil
	case "Messaging.CustomNotification":
		if len(namedArgs) != 0 || (len(args) != 0 && len(args) != 6) {
			return Null, fmt.Errorf("Messaging.CustomNotification constructor expects 0 or 6 arguments")
		}
		for _, arg := range args {
			if arg.Kind != ValueString && !(arg.Kind == ValueObject && strings.EqualFold(arg.Type, "Id")) {
				return Null, fmt.Errorf("Messaging.CustomNotification constructor expects String arguments")
			}
		}
		return newCustomNotification(args), nil
	case "Messaging.PushNotification":
		if len(namedArgs) != 0 || len(args) > 1 {
			return Null, fmt.Errorf("Messaging.PushNotification constructor expects optional payload")
		}
		if len(args) == 1 && args[0].Kind != ValueMap {
			return Null, fmt.Errorf("Messaging.PushNotification constructor expects Map<String,Object> payload")
		}
		return newPushNotification(args), nil
	case "Messaging.ActionResult":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Messaging.ActionResult constructor expects 0 arguments")
		}
		return newActionResult(), nil
	case "Messaging.ActionResult.Builder", "Messaging.Builder":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return newActionResultBuilder(), nil
	case "Messaging.ActionableNotification":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Messaging.ActionableNotification constructor expects 0 arguments")
		}
		return newActionableNotification(), nil
	case "Messaging.ActionableNotification.Builder":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Messaging.ActionableNotification.Builder constructor expects 0 arguments")
		}
		return newActionableNotificationBuilder(), nil
	case "Messaging.InboundEmail":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Messaging.InboundEmail constructor expects 0 arguments")
		}
		return newInboundEmail(), nil
	case "Messaging.InboundEnvelope":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Messaging.InboundEnvelope constructor expects 0 arguments")
		}
		return newInboundEnvelope(), nil
	case "Messaging.InboundEmailResult":
		if len(args) != 0 || len(namedArgs) != 0 {
			return Null, fmt.Errorf("Messaging.InboundEmailResult constructor expects 0 arguments")
		}
		return newInboundEmailResult(), nil
	case "URL":
		if len(namedArgs) != 0 {
			return Null, fmt.Errorf("URL constructor does not accept named fields")
		}
		var raw string
		switch len(args) {
		case 1:
			if args[0].Kind != ValueString {
				return Null, fmt.Errorf("URL constructor expects String")
			}
			raw = args[0].Text
		case 2:
			if args[0].Kind != ValueObject || args[0].Type != "URL" || args[1].Kind != ValueString {
				return Null, fmt.Errorf("URL constructor expects URL context and String spec")
			}
			baseRaw, err := platformScalarText(args[0], "URL")
			if err != nil {
				return Null, err
			}
			base, err := url.Parse(baseRaw)
			if err != nil {
				return Null, err
			}
			ref, err := url.Parse(args[1].Text)
			if err != nil {
				return Null, err
			}
			raw = base.ResolveReference(ref).String()
		case 3, 4:
			if args[0].Kind != ValueString || args[1].Kind != ValueString || args[len(args)-1].Kind != ValueString {
				return Null, fmt.Errorf("URL constructor expects protocol, host, [port,] file")
			}
			protocol, host, file := args[0].Text, args[1].Text, args[len(args)-1].Text
			if len(args) == 4 {
				if args[2].Kind != ValueInt {
					return Null, fmt.Errorf("URL constructor port expects Integer")
				}
				host = fmt.Sprintf("%s:%d", host, args[2].Int)
			}
			raw = protocol + "://" + host + file
		default:
			return Null, fmt.Errorf("URL constructor expects spec, context and spec, or protocol, host, [port,] file")
		}
		if err := validateURLConstructorValue(raw); err != nil {
			return Null, err
		}
		return platformScalar("URL", raw), nil
	}
	if value, handled, err := vm.constructGeneratedPlatformValue(typeName, args, namedArgs); handled || err != nil {
		return value, err
	}
	if componentApexRuntimeType(typeName) {
		if len(args) != 0 {
			return Null, fmt.Errorf("%s constructor expects 0 arguments", typeName)
		}
		return newComponentApexValue(typeName, namedArgs), nil
	}
	objectType := typeName
	var definition storage.ObjectDefinition
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(typeName); ok {
			objectType = canonical
			definition = vm.Org.Objects[canonical].Definition
		}
	}
	object := Object(objectType)
	for field, value := range namedArgs {
		explicitField := false
		if definition.APIName != "" {
			if canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field); ok {
				field = canonical
				explicitField = true
			}
		}
		if explicitField {
			value = coerceSObjectFieldRuntimeValue(value, definition.Fields[field])
			setExplicitSObjectField(&object, field, value)
		} else {
			value = coerceLikelyCustomNumberRuntimeValue(field, value)
			object.Fields[field] = value
		}
	}
	if !vm.isSObjectLikeType(objectType) {
		vm.initializeFields(&object, objectType)
	}
	if looksManagedQualifiedType(objectType) {
		for i, arg := range args {
			object.Fields[fmt.Sprintf("__arg%d", i)] = arg
		}
		return object, nil
	}
	if len(args) != 0 {
		if isExceptionType(typeName) {
			handled, err := applyExceptionConstructorArgs(&object, args)
			if err != nil {
				return Null, err
			}
			if handled {
				return object, nil
			}
		}
		return Null, fmt.Errorf("%s constructor does not accept arguments", typeName)
	}
	return object, nil
}

func applyExceptionConstructorArgs(object *Value, args []Value) (bool, error) {
	switch len(args) {
	case 0:
		return true, nil
	case 1:
		if args[0].Kind == ValueObject && isExceptionType(args[0].Type) {
			object.Fields["__causeInitialized"] = Bool(true)
			object.Fields["__cause"] = args[0]
			return true, nil
		}
		setExceptionMessage(object, args[0])
		return true, nil
	case 2:
		setExceptionMessage(object, args[0])
		if args[1].Kind != ValueNull && (args[1].Kind != ValueObject || !isExceptionType(args[1].Type)) {
			return false, fmt.Errorf("%s constructor expects Exception cause", object.Type)
		}
		object.Fields["__causeInitialized"] = Bool(true)
		object.Fields["__cause"] = args[1]
		return true, nil
	default:
		return false, nil
	}
}

func setExceptionMessage(object *Value, value Value) {
	if value.Kind == ValueString {
		object.Fields["message"] = value
	} else if value.Kind != ValueNull {
		object.Fields["message"] = String(value.String())
	}
}

func (vm *VM) constructArrayValue(typeName string, size Value) (Value, error) {
	if size.Kind != ValueInt {
		return Null, fmt.Errorf("array size must be Integer")
	}
	count := size.Int
	if count < 0 {
		return Null, fmt.Errorf("array size cannot be negative")
	}
	if count > 1000000 {
		return Null, fmt.Errorf("array size too large")
	}
	values := make([]Value, int(count))
	for i := range values {
		values[i] = Null
	}
	list := List(values...)
	list.Type = typeName
	return vm.coerceAssignable(typeName, list)
}

func allMapEntryValues(values []Value) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if value.Kind != ValueObject || value.Type != "__mapEntry" {
			return false
		}
	}
	return true
}

func validateURLConstructorValue(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("URL constructor invalid URL: %w", err)
	}
	if parsed.Scheme == "" {
		return fmt.Errorf("URL constructor invalid URL: missing protocol")
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL constructor invalid URL: missing host")
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.ParseInt(port, 10, 64)
		if err != nil || value < 0 || value > 65535 {
			return fmt.Errorf("URL constructor invalid URL: invalid port")
		}
	}
	return nil
}

func (vm *VM) runInstanceInitializers(class Class, object Value, result *Result) error {
	if class.SuperClass != "" {
		if superClass, ok := vm.Classes[class.SuperClass]; ok {
			if err := vm.runInstanceInitializers(superClass, object, result); err != nil {
				return err
			}
		}
	}
	for _, initializer := range class.InstanceInitializers {
		if initializer.Name == "" {
			initializer.Name = class.Name + ".<init_block>"
		}
		if initializer.ClassName == "" {
			initializer.ClassName = class.Name
		}
		recorderRollback := vm.beginFrameworkMethodCountRecorderRollback()
		if _, err := vm.callMethodWithReceiver(initializer, object, nil, result); err != nil {
			vm.endFrameworkMethodCountRecorderRollback(recorderRollback, true)
			return err
		}
		vm.endFrameworkMethodCountRecorderRollback(recorderRollback, false)
	}
	return nil
}

func isExceptionType(typeName string) bool {
	typeName = exceptionTypeName(typeName)
	return isBuiltinExceptionType(typeName) || strings.HasSuffix(typeName, "Exception")
}

func isThrownAuraHandledException(value Value) bool {
	if !strings.EqualFold(value.Type, "AuraHandledException") && !strings.EqualFold(value.Type, "System.AuraHandledException") {
		return false
	}
	thrown, ok := value.Fields["__thrown"]
	return ok && thrown.Kind == ValueBool && thrown.Bool
}

func isThrownAuraHandledExceptionWithoutExplicitMessage(value Value) bool {
	if !isThrownAuraHandledException(value) {
		return false
	}
	explicit, ok := value.Fields["__messageSet"]
	return !ok || explicit.Kind != ValueBool || !explicit.Bool
}

func typeNewInstanceAllowsDottedBuiltin(typeName string) bool {
	return isExceptionType(typeName) ||
		strings.HasPrefix(typeName, "Schema.") ||
		typeName == "ApexPages.Message"
}

func typeNewInstanceUnsupportedBuiltin(typeName string) (string, bool) {
	canonical := strings.TrimPrefix(typeName, "System.")
	switch canonical {
	case "Object", "String", "Boolean", "Integer", "Long", "Decimal", "Double",
		"Date", "Datetime", "Time", "TimeZone", "Blob", "Id", "Type", "URL",
		"LoggingLevel", "RestContext":
		return canonical, true
	default:
		return "", false
	}
}

func (vm *VM) initializeFields(object *Value, typeName string) {
	class, ok := vm.Classes[typeName]
	if !ok {
		return
	}
	if class.SuperClass != "" {
		vm.initializeFields(object, class.SuperClass)
	}
	for _, name := range orderedFieldNames(class.Fields, class.FieldOrder) {
		field := class.Fields[name]
		value := defaultValue(field.Type, field.InitialValue)
		if value.Kind == ValueObject && !strings.EqualFold(value.Type, field.Type) &&
			(vm.typeAssignableTo(value.Type, field.Type) || vm.typeMatches(value.Type, field.Type, make(map[string]bool))) {
			if value.Runtime == "" {
				value.Runtime = value.Type
			}
			value.Static = field.Type
		}
		object.Fields[name] = value
	}
}

func (vm *VM) initializePassiveCollectionFields(object *Value, typeName string) {
	class, ok := vm.Classes[typeName]
	if !ok {
		return
	}
	if class.SuperClass != "" {
		vm.initializePassiveCollectionFields(object, class.SuperClass)
	}
	for _, name := range orderedFieldNames(class.Fields, class.FieldOrder) {
		field := class.Fields[name]
		value := object.Fields[name]
		if value.Kind != ValueNull {
			continue
		}
		switch {
		case collectionBase(field.Type) == "List":
			list := List()
			list.Type = field.Type
			object.Fields[name] = list
		case collectionBase(field.Type) == "Set":
			set := Set()
			set.Type = field.Type
			object.Fields[name] = set
		case isMapType(field.Type):
			m := Map()
			m.Type = field.Type
			object.Fields[name] = m
		}
	}
}

func (vm *VM) matchConstructor(class Class, args []Value) (Method, bool, bool) {
	return vm.matchMethodByArgs(class.Constructors, args)
}

func (vm *VM) matchConstructorWithNamedArgs(class Class, args []Value, namedArgs map[string]Value) (Method, []Value, bool, bool) {
	type candidateMatch struct {
		method Method
		args   []Value
		score  int
	}
	matches := make([]candidateMatch, 0, len(class.Constructors))
	seen := make(map[string]int, len(class.Constructors))
	for _, candidate := range class.Constructors {
		orderedArgs, ok := vm.constructorArgsWithNamed(candidate, args, namedArgs)
		if !ok {
			orderedArgs, ok = vm.passiveConstructorArgsWithNamedFields(class, candidate, args, namedArgs)
		}
		if !ok {
			continue
		}
		score := 0
		applicable := true
		for i, param := range candidate.Params {
			paramType := vm.resolveTypeNameInClass(candidate.ClassName, param.Type)
			paramScore := vm.conversionScore(paramType, orderedArgs[i])
			if paramScore < 0 {
				applicable = false
				break
			}
			score += paramScore
		}
		if !applicable {
			continue
		}
		signature := methodSignature(candidate)
		if index, exists := seen[signature]; exists {
			if matches[index].method.Dependency && !candidate.Dependency {
				matches[index] = candidateMatch{method: candidate, args: orderedArgs, score: score}
			}
			continue
		}
		seen[signature] = len(matches)
		matches = append(matches, candidateMatch{method: candidate, args: orderedArgs, score: score})
	}
	if len(matches) == 0 {
		return Method{}, nil, false, false
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})
	if len(matches) > 1 && matches[0].score == matches[1].score {
		return Method{}, nil, false, true
	}
	return matches[0].method, matches[0].args, true, false
}

func (vm *VM) passiveConstructorArgsWithNamedFields(class Class, ctor Method, args []Value, namedArgs map[string]Value) ([]Value, bool) {
	if len(ctor.Params) != len(args)+len(namedArgs) {
		return nil, false
	}
	orderedArgs := make([]Value, len(ctor.Params))
	copy(orderedArgs, args)
	openParams := make([]int, 0, len(ctor.Params)-len(args))
	for i := len(args); i < len(ctor.Params); i++ {
		if !passiveGeneratedPlaceholderParam(ctor.Params[i].Name) {
			return nil, false
		}
		openParams = append(openParams, i)
	}
	usedParams := make(map[int]bool, len(openParams))
	for name, value := range namedArgs {
		if strings.TrimSpace(name) == "" {
			return nil, false
		}
		field, _, ok := vm.lookupField(class.Name, name)
		if !ok {
			return nil, false
		}
		fieldType := vm.resolveTypeNameInClass(class.Name, field.Type)
		if vm.conversionScore(fieldType, value) < 0 {
			return nil, false
		}
		matchedParam := -1
		matchedScore := -1
		for _, paramIndex := range openParams {
			if usedParams[paramIndex] {
				continue
			}
			paramType := vm.resolveTypeNameInClass(class.Name, ctor.Params[paramIndex].Type)
			score := vm.conversionScore(paramType, value)
			if score < 0 {
				continue
			}
			if score > matchedScore {
				matchedParam = paramIndex
				matchedScore = score
			}
		}
		if matchedParam < 0 {
			return nil, false
		}
		orderedArgs[matchedParam] = value
		usedParams[matchedParam] = true
	}
	return orderedArgs, len(usedParams) == len(namedArgs) && len(usedParams) == len(openParams)
}

func (vm *VM) constructorArgsWithNamed(ctor Method, args []Value, namedArgs map[string]Value) ([]Value, bool) {
	if len(ctor.Params) != len(args)+len(namedArgs) {
		return nil, false
	}
	orderedArgs := make([]Value, len(ctor.Params))
	copy(orderedArgs, args)
	valuesByName := make(map[string]Value, len(namedArgs))
	for name, value := range namedArgs {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return nil, false
		}
		if _, exists := valuesByName[key]; exists {
			return nil, false
		}
		valuesByName[key] = value
	}
	for i := len(args); i < len(ctor.Params); i++ {
		key := strings.ToLower(strings.TrimSpace(ctor.Params[i].Name))
		value, ok := valuesByName[key]
		if !ok {
			return nil, false
		}
		orderedArgs[i] = value
		delete(valuesByName, key)
	}
	return orderedArgs, len(valuesByName) == 0
}

func (vm *VM) bindPassiveNamedConstructorFields(object *Value, namedArgs map[string]Value) {
	if object == nil || object.Kind != ValueObject {
		return
	}
	for field, value := range namedArgs {
		object.Fields[vm.passiveConstructorFieldName(object.Type, *object, field)] = value
	}
}

func (vm *VM) passiveConstructorFieldName(typeName string, object Value, field string) string {
	if def, _, ok := vm.lookupField(typeName, field); ok && def.Name != "" {
		return def.Name
	}
	return passiveAccessorFieldName(object, field)
}

func passiveGeneratedPlaceholderParam(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(name, "arg") {
		return passiveGeneratedPlaceholderSuffix(name[len("arg"):])
	}
	if strings.HasPrefix(name, "param") {
		return passiveGeneratedPlaceholderSuffix(name[len("param"):])
	}
	return false
}

func passiveGeneratedPlaceholderSuffix(suffix string) bool {
	if suffix == "" {
		return false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (vm *VM) callImplicitSuperConstructor(class Class, ctor Method, object Value, result *Result) error {
	if class.SuperClass == "" || constructorHasExplicitChain(ctor) {
		return nil
	}
	superClass, ok := vm.lookupClass(class.SuperClass)
	if !ok {
		return nil
	}
	superCtor, found, ambiguous := vm.matchConstructor(superClass, nil)
	if ambiguous {
		return fmt.Errorf("ambiguous %s constructor with 0 argument(s)", superClass.Name)
	}
	if !found {
		return vm.callImplicitSuperConstructor(superClass, Method{}, object, result)
	}
	if err := vm.callImplicitSuperConstructor(superClass, superCtor, object, result); err != nil {
		return err
	}
	_, err := vm.callMethodWithReceiver(superCtor, object, nil, result)
	return err
}

func constructorHasExplicitChain(ctor Method) bool {
	if len(ctor.Program.Instructions) == 0 {
		return false
	}
	first := ctor.Program.Instructions[0]
	return first.Op == ir.OpExpr && first.Expr.Kind == ir.ExprCall && (first.Expr.Callee == "this" || first.Expr.Callee == "super")
}

func (vm *VM) callChainedConstructor(callee string, args []Value, result *Result) (Value, error) {
	receiver, ok := vm.Globals["this"]
	if !ok || receiver.Kind != ValueObject {
		return Null, fmt.Errorf("%s constructor call requires instance receiver", callee)
	}
	className := vm.currentMethod.ClassName
	if className == "" {
		className = receiver.Type
	}
	class, ok := vm.lookupClass(className)
	if !ok {
		return Null, fmt.Errorf("%s constructor call requires registered class %q", callee, className)
	}
	targetClass := class
	if callee == "super" {
		if class.SuperClass == "" {
			if len(args) == 0 {
				return Null, nil
			}
			return Null, fmt.Errorf("%s has no superclass constructor", receiver.Type)
		}
		var found bool
		targetClass, found = vm.Classes[class.SuperClass]
		if !found {
			if looksManagedQualifiedType(class.SuperClass) {
				return Null, nil
			}
			return Null, fmt.Errorf("unknown superclass %q", class.SuperClass)
		}
	}
	matchArgs := args
	if callee == "super" {
		matchArgs = preferStaticArgumentTypes(args)
	}
	target, found, ambiguous := vm.matchConstructor(targetClass, matchArgs)
	if ambiguous {
		return Null, fmt.Errorf("ambiguous %s constructor with %d argument(s)", targetClass.Name, len(args))
	}
	if !found {
		if len(args) == 0 {
			return Null, nil
		}
		if callee == "super" {
			updated, handled, err := vm.constructFrameworkSObjectDomainBase(targetClass.Name, receiver, args)
			if handled || err != nil {
				if err != nil {
					return Null, err
				}
				vm.Globals["this"] = updated
				return Null, nil
			}
		}
		if isExceptionType(targetClass.Name) {
			handled, err := applyExceptionConstructorArgs(&receiver, args)
			if err != nil {
				return Null, err
			}
			if handled {
				vm.Globals["this"] = receiver
				return Null, nil
			}
		}
		return Null, fmt.Errorf("%s constructor expects 0 arguments", targetClass.Name)
	}
	if callee == "this" && sameConstructorSignature(vm.currentMethod, target) {
		return Null, fmt.Errorf("recursive constructor invocation %s", target.Name)
	}
	if vm.activeConstructors[constructorCallKey(target)] > 0 {
		return Null, fmt.Errorf("recursive constructor invocation %s", target.Name)
	}
	if callee == "super" && vm.isSObjectUnitOfWorkBaseType(targetClass.Name) {
		constructed, err := vm.constructFrameworkSObjectUnitOfWork(args, nil)
		if err != nil {
			return Null, err
		}
		if receiver.Fields == nil {
			receiver.Fields = make(map[string]Value)
		}
		for name, value := range constructed.Fields {
			receiver.Fields[name] = value
		}
		if vm.isSObjectUnitOfWorkRuntimeType(receiver.Type) {
			_ = vm.replayFrameworkSObjectUnitOfWorkTypeRegistrations(receiver)
		}
		vm.Globals["this"] = receiver
		return Null, nil
	}
	if err := vm.callImplicitSuperConstructor(targetClass, target, receiver, result); err != nil {
		return Null, err
	}
	_, err := vm.callConstructorWithReceiver(targetClass, target, receiver, args, result)
	return Null, err
}

func preferStaticArgumentTypes(args []Value) []Value {
	out := make([]Value, len(args))
	copy(out, args)
	for i := range out {
		if out[i].Static == "" {
			continue
		}
		out[i].Type = out[i].Static
		out[i].Runtime = ""
	}
	return out
}

func (vm *VM) callConstructorWithReceiver(class Class, target Method, receiver Value, args []Value, result *Result) (Value, error) {
	constructors := vm.sameSignatureConstructors(class, target, args)
	if len(constructors) == 0 {
		constructors = []Method{target}
	}
	current := receiver
	for _, ctor := range constructors {
		constructed, err := vm.callMethodWithReceiver(ctor, current, args, result)
		if err != nil {
			return Null, err
		}
		if constructed.Kind == ValueObject {
			current = constructed
		}
	}
	return current, nil
}

func (vm *VM) sameSignatureConstructors(class Class, target Method, args []Value) []Method {
	out := make([]Method, 0, len(class.Constructors))
	for _, candidate := range class.Constructors {
		if !sameConstructorSignature(candidate, target) || !vm.methodApplicable(candidate, args) {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func constructorCallKey(method Method) string {
	parts := make([]string, 0, len(method.Params)+2)
	parts = append(parts, method.ClassName, method.Name)
	for _, param := range method.Params {
		parts = append(parts, param.Type)
	}
	return strings.Join(parts, "\x00")
}

func (vm *VM) constructFrameworkSObjectDomainBase(className string, receiver Value, args []Value) (Value, bool, error) {
	if !isFrameworkSObjectDomainClassName(className) {
		return receiver, false, nil
	}
	if len(args) != 1 {
		return receiver, true, fmt.Errorf("%s constructor expects List<SObject>", className)
	}
	records, err := vm.coerceAssignable("List<SObject>", args[0])
	if err != nil {
		return receiver, true, err
	}
	if receiver.Fields == nil {
		receiver.Fields = make(map[string]Value)
	}
	receiver.Fields["Records"] = records
	receiver.Fields["SObjectDescribe"] = Object("DescribeSObjectSharingControl")
	return receiver, true, nil
}

func isFrameworkSObjectDomainClassName(className string) bool {
	switch strings.ToLower(strings.TrimSpace(className)) {
	case "sobjectdomain", "framework_sobjectdomain", "fflib_sobjectdomain":
		return true
	default:
		return false
	}
}

func sameConstructorSignature(left, right Method) bool {
	if !strings.EqualFold(left.Name, right.Name) || len(left.Params) != len(right.Params) {
		return false
	}
	for i := range left.Params {
		if !strings.EqualFold(left.Params[i].Type, right.Params[i].Type) {
			return false
		}
	}
	return true
}

func (vm *VM) resolveInstanceMethod(typeName, method string) (Method, bool) {
	return vm.resolveInstanceMethodSeen(typeName, method, make(map[string]bool))
}

func (vm *VM) resolveInstanceMethodSeen(typeName, method string, seen map[string]bool) (Method, bool) {
	var interfaces []string
	for typeName != "" {
		if seen[typeName] {
			return Method{}, false
		}
		seen[typeName] = true
		if target, ok := vm.Methods[typeName+"."+method]; ok {
			return target, true
		}
		class, ok := vm.lookupClass(typeName)
		if !ok {
			break
		}
		interfaces = append(interfaces, class.Interfaces...)
		typeName = class.SuperClass
	}
	for _, iface := range interfaces {
		if target, ok := vm.resolveInterfaceMethodSeen(iface, method, seen); ok {
			return target, true
		}
	}
	return Method{}, false
}

func (vm *VM) resolveInstanceMethodForArgs(typeName, method string, args []Value) (Method, bool, bool) {
	typeName = vm.resolveRuntimeTypeName(typeName)
	cacheKey := vm.methodResolveCacheKey("i", typeName, method, args)
	if cached, ok := vm.methodResolveCache[cacheKey]; ok {
		return cached.Method, cached.OK, false
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgsSeen(typeName, method, args, make(map[string]bool))
	if !ambiguous {
		if vm.methodResolveCache == nil {
			vm.methodResolveCache = make(map[string]methodResolution)
		}
		vm.methodResolveCache[cacheKey] = methodResolution{Method: target, OK: ok}
	}
	return target, ok, ambiguous
}

func (vm *VM) resolveInstanceMethodByArity(typeName, method string, arity int) (Method, bool, bool) {
	typeName = vm.resolveRuntimeTypeName(typeName)
	return vm.resolveInstanceMethodByAritySeen(typeName, method, arity, make(map[string]bool))
}

func (vm *VM) resolveRuntimeTypeName(typeName string) string {
	if resolved := vm.resolveTypeNameInClass(vm.currentClass, typeName); resolved != "" {
		return resolved
	}
	if resolved, ok := vm.resolveClassName(typeName); ok {
		if !strings.Contains(typeName, ".") && !strings.Contains(resolved, ".") {
			if nested, nestedOK := vm.resolveUniqueNestedTypeName(typeName); nestedOK {
				return nested
			}
		}
		return resolved
	}
	if resolved, ok := vm.resolveUniqueNestedTypeName(typeName); ok {
		return resolved
	}
	return typeName
}

func (vm *VM) resolveInstanceMethodByAritySeen(typeName, method string, arity int, seen map[string]bool) (Method, bool, bool) {
	var interfaces []string
	for typeName != "" {
		if seen[typeName] {
			return Method{}, false, false
		}
		seen[typeName] = true
		if target, ok, ambiguous := vm.matchRegisteredInstanceMethodByArity(typeName+"."+method, arity); ok || ambiguous {
			return target, ok, ambiguous
		}
		class, ok := vm.lookupClass(typeName)
		if !ok {
			break
		}
		interfaces = append(interfaces, class.Interfaces...)
		typeName = class.SuperClass
	}
	for _, iface := range interfaces {
		if target, ok, ambiguous := vm.resolveInstanceMethodByAritySeen(iface, method, arity, seen); ok || ambiguous {
			return target, ok, ambiguous
		}
	}
	return Method{}, false, false
}

func (vm *VM) matchRegisteredMethodByArity(name string, arity int) (Method, bool, bool) {
	candidates := vm.registeredMethodCandidates(name)
	if len(candidates) == 0 {
		if method, ok := vm.Methods[name]; ok && len(method.Params) == arity {
			return method, true, false
		}
		return Method{}, false, false
	}
	var found Method
	count := 0
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if len(candidate.Params) != arity {
			continue
		}
		signature := methodSignature(candidate)
		if seen[signature] {
			continue
		}
		seen[signature] = true
		if count == 0 {
			found = candidate
		}
		count++
	}
	switch count {
	case 0:
		return Method{}, false, false
	case 1:
		return found, true, false
	default:
		return found, false, true
	}
}

func (vm *VM) matchRegisteredInstanceMethodByArity(name string, arity int) (Method, bool, bool) {
	candidates := vm.registeredMethodCandidates(name)
	if len(candidates) == 0 {
		if method, ok := vm.Methods[name]; ok && !method.IsStatic && len(method.Params) == arity {
			return method, true, false
		}
		return Method{}, false, false
	}
	var found Method
	count := 0
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if candidate.IsStatic || len(candidate.Params) != arity {
			continue
		}
		signature := methodSignature(candidate)
		if seen[signature] {
			continue
		}
		seen[signature] = true
		if count == 0 {
			found = candidate
		}
		count++
	}
	switch count {
	case 0:
		return Method{}, false, false
	case 1:
		return found, true, false
	default:
		return found, false, true
	}
}

func (vm *VM) firstRegisteredMethodByArity(typeName, method string, args []Value) (Method, bool) {
	return vm.firstRegisteredMethodByAritySeen(typeName, method, args, make(map[string]bool))
}

func (vm *VM) firstRegisteredMethodByAritySeen(typeName, method string, args []Value, seen map[string]bool) (Method, bool) {
	var interfaces []string
	for typeName != "" {
		if seen[typeName] {
			return Method{}, false
		}
		seen[typeName] = true
		if target, ok := vm.firstApplicableInstanceMethodByArity(typeName+"."+method, args); ok {
			return target, true
		}
		class, ok := vm.lookupClass(typeName)
		if !ok {
			break
		}
		interfaces = append(interfaces, class.Interfaces...)
		typeName = class.SuperClass
	}
	for _, iface := range interfaces {
		if target, ok := vm.firstRegisteredMethodByAritySeen(iface, method, args, seen); ok {
			return target, true
		}
	}
	return Method{}, false
}

func (vm *VM) firstMethodByArity(name string, arity int) (Method, bool) {
	candidates := vm.registeredMethodCandidates(name)
	for _, candidate := range candidates {
		if len(candidate.Params) == arity {
			return candidate, true
		}
	}
	if method, ok := vm.Methods[name]; ok && len(method.Params) == arity {
		return method, true
	}
	return Method{}, false
}

func (vm *VM) firstInstanceMethodByArity(name string, arity int) (Method, bool) {
	candidates := vm.registeredMethodCandidates(name)
	for _, candidate := range candidates {
		if !candidate.IsStatic && len(candidate.Params) == arity {
			return candidate, true
		}
	}
	if method, ok := vm.Methods[name]; ok && !method.IsStatic && len(method.Params) == arity {
		return method, true
	}
	return Method{}, false
}

func (vm *VM) firstApplicableInstanceMethodByArity(name string, args []Value) (Method, bool) {
	candidates := vm.registeredMethodCandidates(name)
	for _, candidate := range candidates {
		if candidate.IsStatic || len(candidate.Params) != len(args) || !vm.methodApplicable(candidate, args) {
			continue
		}
		return candidate, true
	}
	if method, ok := vm.Methods[name]; ok && !method.IsStatic && len(method.Params) == len(args) && vm.methodApplicable(method, args) {
		return method, true
	}
	return Method{}, false
}

func (vm *VM) dispatchAccessMethod(staticType string, target Method, method string, args []Value) Method {
	staticType = vm.resolveTypeNameInClass(vm.currentClass, staticType)
	if staticType == "" {
		return target
	}
	surface, ok, ambiguous := vm.resolveInstanceMethodForArgs(staticType, method, args)
	if !ok || ambiguous {
		return target
	}
	if strings.EqualFold(surface.ClassName, target.ClassName) || vm.isSubclass(target.ClassName, surface.ClassName) {
		return surface
	}
	return target
}

func (vm *VM) uniqueConcreteOverride(receiver Value, baseType, method string, args []Value) (Method, bool) {
	var found Method
	count := 0
	var fieldMatched Method
	fieldMatchCount := 0
	var qualifierMatched Method
	qualifierMatchCount := 0
	for className, class := range vm.Classes {
		if strings.EqualFold(className, baseType) || (!vm.typeMatches(className, baseType, make(map[string]bool)) && !classExtendsType(class, baseType)) {
			continue
		}
		if vm.classIsTestScoped(className) && !sameLexicalTopLevel(vm.currentClass, className) {
			continue
		}
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(className, method, args)
		if (!ok || methodHasModifier(target.Modifiers, "abstract")) && !ambiguous {
			if staticTarget, found := vm.staticConcreteOverrideForAbstract(baseType, className, method, args); found {
				target = staticTarget
				ok = true
			}
		}
		if !ok || ambiguous || methodHasModifier(target.Modifiers, "abstract") {
			continue
		}
		count++
		found = target
		if sameTypeQualifier(baseType, className) {
			qualifierMatched = target
			qualifierMatchCount++
		}
		if class, ok := vm.Classes[className]; ok && receiverHasClassField(receiver, class) {
			fieldMatched = target
			fieldMatchCount++
		}
	}
	if count > 1 {
		if qualifierMatchCount == 1 {
			return qualifierMatched, true
		}
		return fieldMatched, fieldMatchCount == 1
	}
	return found, count == 1
}

func (vm *VM) staticConcreteOverrideForAbstract(baseType, className, method string, args []Value) (Method, bool) {
	baseClass, ok := vm.lookupClass(baseType)
	if !ok || (!baseClass.IsInterface && !baseClass.IsAbstract) {
		return Method{}, false
	}
	target, ok, ambiguous := vm.resolveStaticMethodForArgs(className, method, args)
	if !ok || ambiguous || methodHasModifier(target.Modifiers, "abstract") {
		return Method{}, false
	}
	return target, true
}

func sameTypeQualifier(baseType, className string) bool {
	baseDot := strings.IndexByte(baseType, '.')
	classDot := strings.IndexByte(className, '.')
	if baseDot < 0 {
		return classDot < 0
	}
	if classDot < 0 {
		return false
	}
	return strings.EqualFold(baseType[:baseDot], className[:classDot])
}

func classExtendsType(class Class, baseType string) bool {
	return strings.EqualFold(class.SuperClass, baseType) || strings.EqualFold(shortTypeName(class.SuperClass), shortTypeName(baseType))
}

func receiverHasClassField(receiver Value, class Class) bool {
	for name := range class.Fields {
		if _, _, ok := objectFieldValue(receiver, name); ok {
			return true
		}
	}
	return false
}

func (vm *VM) concreteOverrideByReceiverFields(receiver Value, baseType, method string, args []Value) (Method, bool) {
	var found Method
	count := 0
	for className, class := range vm.Classes {
		if strings.EqualFold(className, baseType) || vm.classIsTestScoped(className) || !receiverHasClassField(receiver, class) {
			continue
		}
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(className, method, args)
		if !ok || ambiguous || methodHasModifier(target.Modifiers, "abstract") {
			continue
		}
		count++
		if count > 1 {
			return Method{}, false
		}
		found = target
	}
	return found, count == 1
}

func (vm *VM) classIsTestScoped(className string) bool {
	if class, ok := vm.Classes[className]; ok && class.IsTest {
		return true
	}
	top, nested := lexicalTopLevel(className)
	if !nested {
		return strings.Contains(top, "Test")
	}
	class, ok := vm.Classes[top]
	return (ok && class.IsTest) || strings.Contains(top, "Test")
}

func (vm *VM) resolveInstanceMethodForArgsSeen(typeName, method string, args []Value, seen map[string]bool) (Method, bool, bool) {
	var interfaces []string
	for typeName != "" {
		if seen[typeName] {
			return Method{}, false, false
		}
		seen[typeName] = true
		if target, ok, ambiguous := vm.matchRegisteredInstanceMethod(typeName+"."+method, args); ok || ambiguous {
			return target, ok, ambiguous
		}
		class, ok := vm.lookupClass(typeName)
		if !ok {
			break
		}
		interfaces = append(interfaces, class.Interfaces...)
		typeName = class.SuperClass
	}
	for _, iface := range interfaces {
		if target, ok, ambiguous := vm.resolveInterfaceMethodForArgsSeen(iface, method, args, seen); ok || ambiguous {
			return target, ok, ambiguous
		}
	}
	return Method{}, false, false
}

func (vm *VM) resolveInterfaceMethodSeen(typeName, method string, seen map[string]bool) (Method, bool) {
	if typeName == "" || seen[typeName] {
		return Method{}, false
	}
	seen[typeName] = true
	if target, ok := vm.Methods[typeName+"."+method]; ok && !target.IsStatic {
		return target, true
	}
	class, ok := vm.Classes[typeName]
	if !ok {
		return Method{}, false
	}
	for _, iface := range class.Interfaces {
		if target, ok := vm.resolveInterfaceMethodSeen(iface, method, seen); ok {
			return target, true
		}
	}
	return Method{}, false
}

func (vm *VM) resolveInterfaceMethodForArgsSeen(typeName, method string, args []Value, seen map[string]bool) (Method, bool, bool) {
	if typeName == "" || seen[typeName] {
		return Method{}, false, false
	}
	seen[typeName] = true
	if target, ok, ambiguous := vm.matchRegisteredInstanceMethod(typeName+"."+method, args); ok || ambiguous {
		return target, ok, ambiguous
	}
	class, ok := vm.lookupClass(typeName)
	if !ok {
		return Method{}, false, false
	}
	for _, iface := range class.Interfaces {
		if target, ok, ambiguous := vm.resolveInterfaceMethodForArgsSeen(iface, method, args, seen); ok || ambiguous {
			return target, ok, ambiguous
		}
	}
	return Method{}, false, false
}

func (vm *VM) resolveStaticMethodForArgs(typeName, method string, args []Value) (Method, bool, bool) {
	candidates := vm.staticTypeNameCandidates(typeName)
	cacheKey := vm.methodResolveCacheKey("s", strings.Join(candidates, "|"), method, args)
	if cached, ok := vm.methodResolveCache[cacheKey]; ok {
		return cached.Method, cached.OK, false
	}
	for _, candidateType := range candidates {
		for searchType := candidateType; searchType != ""; {
			target, ok, ambiguous := vm.matchRegisteredStaticMethod(searchType+"."+method, args)
			if ambiguous {
				return Method{}, false, true
			}
			if ok {
				if vm.methodResolveCache == nil {
					vm.methodResolveCache = make(map[string]methodResolution)
				}
				vm.methodResolveCache[cacheKey] = methodResolution{Method: target, OK: true}
				return target, true, false
			}
			class, ok := vm.lookupClass(searchType)
			if !ok {
				break
			}
			searchType = class.SuperClass
		}
	}
	if vm.methodResolveCache == nil {
		vm.methodResolveCache = make(map[string]methodResolution)
	}
	vm.methodResolveCache[cacheKey] = methodResolution{}
	return Method{}, false, false
}

func (vm *VM) staticTypeNameCandidates(typeName string) []string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return nil
	}
	out := make([]string, 0, 4)
	appendUnique := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		for _, existing := range out {
			if strings.EqualFold(existing, name) {
				return
			}
		}
		out = append(out, name)
	}
	if !strings.Contains(typeName, ".") {
		if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
			appendUnique(namespace + "." + typeName)
			if class, ok := vm.lookupClassInNamespace(namespace, typeName); ok {
				appendUnique(class.Name)
				if class.Namespace != "" {
					appendUnique(class.Namespace + "." + class.Name)
				}
			}
		}
	}
	appendUnique(typeName)
	return out
}

func (vm *VM) currentExecutionNamespace() string {
	if vm == nil {
		return ""
	}
	if strings.TrimSpace(vm.currentNamespace) != "" {
		return strings.TrimSpace(vm.currentNamespace)
	}
	if strings.TrimSpace(vm.currentClass) == "" {
		return ""
	}
	if class, ok := vm.Classes[vm.currentClass]; ok {
		return strings.TrimSpace(class.Namespace)
	}
	for _, triggers := range vm.Triggers {
		for _, trigger := range triggers {
			if strings.EqualFold(trigger.Name, vm.currentClass) {
				return strings.TrimSpace(trigger.Namespace)
			}
		}
	}
	return ""
}

func (vm *VM) matchRegisteredStaticMethod(name string, args []Value) (Method, bool, bool) {
	filterStatic := func(candidates []Method) []Method {
		static := make([]Method, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.IsStatic {
				static = append(static, candidate)
			}
		}
		return static
	}
	if candidates := filterStatic(vm.registeredMethodCandidates(name)); len(candidates) > 0 {
		return vm.matchMethodByArgs(candidates, args)
	}
	method, ok := vm.Methods[name]
	if !ok || !method.IsStatic || len(method.Params) != len(args) {
		return Method{}, false, false
	}
	for i, param := range method.Params {
		paramType := vm.resolveTypeNameInClass(method.ClassName, param.Type)
		if paramType == "" {
			paramType = param.Type
		}
		if err := vm.ensureAssignable(paramType, args[i]); err != nil {
			return Method{}, false, false
		}
	}
	return method, true, false
}

func (vm *VM) matchRegisteredMethod(name string, args []Value) (Method, bool, bool) {
	if strings.Contains(name, ".") && vm.currentClass != "" {
		if callerNS := vm.classNamespace(vm.currentClass); callerNS != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(callerNS)+".") {
			if method, ok, ambiguous := vm.matchRegisteredMethodInNamespace(callerNS, name, args); ok || ambiguous {
				return method, ok, ambiguous
			}
		}
	}
	if candidates := vm.registeredMethodCandidates(name); len(candidates) > 0 {
		return vm.matchMethodByArgs(candidates, args)
	}
	method, ok := vm.Methods[name]
	if !ok {
		return Method{}, false, false
	}
	if len(method.Params) != len(args) {
		return Method{}, false, false
	}
	for i, param := range method.Params {
		if err := vm.ensureAssignable(param.Type, args[i]); err != nil {
			return Method{}, false, false
		}
	}
	return method, true, false
}

func (vm *VM) matchRegisteredMethodInNamespace(namespace, name string, args []Value) (Method, bool, bool) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return Method{}, false, false
	}
	qualified := namespace + "." + name
	if candidates := vm.registeredMethodCandidates(qualified); len(candidates) > 0 {
		return vm.matchMethodByArgs(candidates, args)
	}
	method, ok := vm.Methods[qualified]
	if !ok {
		return Method{}, false, false
	}
	if len(method.Params) != len(args) {
		return Method{}, false, false
	}
	for i, param := range method.Params {
		if err := vm.ensureAssignable(param.Type, args[i]); err != nil {
			return Method{}, false, false
		}
	}
	return method, true, false
}

func (vm *VM) matchRegisteredInstanceMethod(name string, args []Value) (Method, bool, bool) {
	if candidates := vm.registeredMethodCandidates(name); len(candidates) > 0 {
		instance := make([]Method, 0, len(candidates))
		for _, candidate := range candidates {
			if !candidate.IsStatic {
				instance = append(instance, candidate)
			}
		}
		return vm.matchMethodByArgs(instance, args)
	}
	method, ok := vm.Methods[name]
	if !ok || method.IsStatic {
		return Method{}, false, false
	}
	if len(method.Params) != len(args) {
		return Method{}, false, false
	}
	for i, param := range method.Params {
		if err := vm.ensureAssignable(param.Type, args[i]); err != nil {
			return Method{}, false, false
		}
	}
	return method, true, false
}

func (vm *VM) registeredMethodCandidates(name string) []Method {
	if vm.methodCandidates != nil {
		if candidates, ok := vm.methodCandidates[name]; ok {
			return candidates
		}
	} else {
		vm.methodCandidates = make(map[string][]Method)
	}
	exact := vm.MethodOverloads[name]
	folded := vm.MethodFolded[strings.ToLower(name)]
	if len(exact) == 0 {
		vm.methodCandidates[name] = folded
		return folded
	}
	if len(folded) == 0 {
		vm.methodCandidates[name] = exact
		return exact
	}
	out := make([]Method, 0, len(exact)+len(folded))
	seen := make(map[string]bool, len(exact)+len(folded))
	appendUnique := func(method Method) {
		key := registeredMethodCandidateKey(method)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, method)
	}
	for _, method := range exact {
		appendUnique(method)
	}
	for _, method := range folded {
		appendUnique(method)
	}
	vm.methodCandidates[name] = out
	return out
}

func (vm *VM) methodResolveCacheKey(kind, typeName, method string, args []Value) string {
	var b strings.Builder
	b.WriteString(kind)
	if vm.currentMethod.Dependency {
		b.WriteString(":dependency")
	} else {
		b.WriteString(":project")
	}
	b.WriteByte('|')
	b.WriteString(strings.ToLower(typeName))
	b.WriteByte('|')
	b.WriteString(strings.ToLower(method))
	for _, arg := range args {
		b.WriteByte('|')
		b.WriteString(valueShape(arg))
	}
	return b.String()
}

func (vm *VM) matchMethodByArgs(candidates []Method, args []Value) (Method, bool, bool) {
	applicable := make([]Method, 0, len(candidates))
	seen := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		if vm.methodApplicable(candidate, args) {
			signature := candidate.ClassName + "\x00" + methodParamSignature(candidate) + "\x00" + strconv.FormatBool(candidate.IsStatic)
			if index, exists := seen[signature]; exists {
				if vm.preferMethodCandidate(candidate, applicable[index]) {
					applicable[index] = candidate
				}
				continue
			}
			seen[signature] = len(applicable)
			applicable = append(applicable, candidate)
		}
	}
	vm.sortMethodCandidatesForCaller(applicable)
	target, ok, ambiguous := vm.bestMethodBySpecificity(applicable, args)
	if ambiguous {
		vm.lastAmbiguous = &overloadDiagnostic{
			Args:       append([]Value(nil), args...),
			Candidates: append([]Method(nil), applicable...),
		}
	} else {
		vm.lastAmbiguous = nil
	}
	return target, ok, ambiguous
}

func (vm *VM) preferMethodCandidate(candidate, existing Method) bool {
	if vm.currentMethod.Dependency {
		return methodOrigin(candidate) == symbolOriginDependency && methodOrigin(existing) != symbolOriginDependency
	}
	return methodOrigin(candidate) == symbolOriginProject && methodOrigin(existing) != symbolOriginProject
}

func (vm *VM) sortMethodCandidatesForCaller(methods []Method) {
	sort.SliceStable(methods, func(i, j int) bool {
		left := vm.methodCandidateRank(methods[i])
		right := vm.methodCandidateRank(methods[j])
		if left != right {
			return left < right
		}
		if methods[i].ClassName != methods[j].ClassName {
			return methods[i].ClassName < methods[j].ClassName
		}
		if methods[i].Name != methods[j].Name {
			return methods[i].Name < methods[j].Name
		}
		return methods[i].File < methods[j].File
	})
}

func (vm *VM) methodCandidateRank(method Method) int {
	return dependencyPreferenceRank(methodOrigin(method), vm.currentMethod.Dependency)
}

func (vm *VM) methodApplicable(candidate Method, args []Value) bool {
	if len(candidate.Params) != len(args) {
		return false
	}
	for i, param := range candidate.Params {
		paramType := vm.resolveTypeNameInClass(candidate.ClassName, param.Type)
		if vm.conversionScore(paramType, args[i]) < 0 {
			return false
		}
	}
	return true
}

func (vm *VM) ambiguousOverloadError(callee string, args []Value) error {
	message := fmt.Sprintf("ambiguous overload for call %q", callee)
	diag := vm.lastAmbiguous
	if diag == nil || len(diag.Candidates) == 0 {
		diag = &overloadDiagnostic{Args: append([]Value(nil), args...)}
	}
	argTypes := runtimeArgTypes(diag.Args)
	if len(argTypes) == 0 && len(args) > 0 {
		argTypes = runtimeArgTypes(args)
	}
	if len(argTypes) > 0 {
		message += "; argument types: " + strings.Join(argTypes, ", ")
	}
	if len(diag.Candidates) > 0 {
		signatures := make([]string, 0, len(diag.Candidates))
		for _, candidate := range diag.Candidates {
			signatures = append(signatures, methodSignature(candidate))
		}
		sort.Strings(signatures)
		message += "; candidates: " + strings.Join(signatures, "; ")
	}
	return fmt.Errorf("%s", message)
}

func runtimeArgTypes(args []Value) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, 0, len(args))
	for i, arg := range args {
		out = append(out, fmt.Sprintf("%d:%s", i+1, valueTypeName(arg)))
	}
	return out
}

func methodSignature(method Method) string {
	name := method.Name
	if name == "" && method.ClassName != "" {
		name = method.ClassName
	}
	return name + methodParamSignature(method)
}

func methodParamSignature(method Method) string {
	params := make([]string, 0, len(method.Params))
	for _, param := range method.Params {
		paramType := strings.TrimSpace(param.Type)
		if paramType == "" {
			paramType = "Object"
		}
		if param.Name != "" {
			params = append(params, param.Name+" "+paramType)
		} else {
			params = append(params, paramType)
		}
	}
	return "(" + strings.Join(params, ", ") + ")"
}

func (vm *VM) bestMethodBySpecificity(applicable []Method, args []Value) (Method, bool, bool) {
	if len(applicable) == 0 {
		return Method{}, false, false
	}
	bestIndex := -1
	for i, candidate := range applicable {
		moreSpecificThanAll := true
		for j, other := range applicable {
			if i == j {
				continue
			}
			switch vm.compareMethodSpecificityForArgs(candidate, other, args) {
			case -1, 2:
				moreSpecificThanAll = false
			}
			if !moreSpecificThanAll {
				break
			}
		}
		if moreSpecificThanAll {
			if bestIndex >= 0 && vm.compareMethodSpecificityForArgs(candidate, applicable[bestIndex], args) == 0 {
				continue
			}
			if bestIndex >= 0 {
				return Method{}, false, true
			}
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		if scored, ok := vm.bestMethodByConversionScore(applicable, args); ok {
			return scored, true, false
		}
		return Method{}, false, true
	}
	return applicable[bestIndex], true, false
}

func (vm *VM) bestMethodByConversionScore(applicable []Method, args []Value) (Method, bool) {
	bestIndex := -1
	bestScore := math.MinInt
	for i, candidate := range applicable {
		if len(candidate.Params) != len(args) {
			return Method{}, false
		}
		score := 0
		for j, param := range candidate.Params {
			paramType := vm.resolveTypeNameInClass(candidate.ClassName, param.Type)
			part := vm.conversionScore(paramType, args[j])
			if part < 0 {
				return Method{}, false
			}
			score += part
		}
		if score > bestScore {
			bestScore = score
			bestIndex = i
			continue
		}
		if score == bestScore {
			bestIndex = -1
		}
	}
	if bestIndex < 0 {
		return Method{}, false
	}
	return applicable[bestIndex], true
}

func (vm *VM) compareMethodSpecificityForArgs(left, right Method, args []Value) int {
	if len(args) != len(left.Params) || len(args) != len(right.Params) {
		return vm.compareMethodSpecificity(left, right)
	}
	leftBetter := false
	rightBetter := false
	for i, arg := range args {
		leftType := vm.resolveTypeNameInClass(left.ClassName, left.Params[i].Type)
		rightType := vm.resolveTypeNameInClass(right.ClassName, right.Params[i].Type)
		leftScore := vm.conversionScore(leftType, arg)
		rightScore := vm.conversionScore(rightType, arg)
		switch {
		case leftScore > rightScore:
			leftBetter = true
		case rightScore > leftScore:
			rightBetter = true
		}
		if leftBetter && rightBetter {
			return 2
		}
	}
	switch {
	case leftBetter:
		return 1
	case rightBetter:
		return -1
	default:
		return vm.compareMethodSpecificity(left, right)
	}
}

func (vm *VM) compareMethodSpecificity(left, right Method) int {
	leftBetter := false
	rightBetter := false
	for i := range left.Params {
		leftType := vm.resolveTypeNameInClass(left.ClassName, left.Params[i].Type)
		rightType := vm.resolveTypeNameInClass(right.ClassName, right.Params[i].Type)
		switch vm.compareTypeSpecificity(leftType, rightType) {
		case 1:
			leftBetter = true
		case -1:
			rightBetter = true
		case 2:
			return 2
		}
		if leftBetter && rightBetter {
			return 2
		}
	}
	switch {
	case leftBetter:
		return 1
	case rightBetter:
		return -1
	default:
		return 0
	}
}

func (vm *VM) resolveTypeNameInClass(className, typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	if base := collectionBase(typeName); base != "" {
		element, ok := collectionElementType(typeName)
		if !ok {
			return typeName
		}
		return base + "<" + vm.resolveTypeNameInClass(className, element) + ">"
	}
	if keyType, valueType, ok := mapTypeArgs(typeName); ok {
		return "Map<" + vm.resolveTypeNameInClass(className, keyType) + "," + vm.resolveTypeNameInClass(className, valueType) + ">"
	}
	if strings.Contains(typeName, ".") || className == "" {
		return typeName
	}
	if resolved, ok := vm.resolveNestedTypeInClassHierarchy(className, typeName); ok {
		return resolved
	}
	return typeName
}

func (vm *VM) resolveNestedTypeInClassHierarchy(className, typeName string) (string, bool) {
	for owner := className; owner != ""; {
		for _, ownerCandidate := range lexicalOwnerCandidates(owner) {
			candidate := ownerCandidate + "." + typeName
			if class, ok := vm.lookupClass(candidate); ok {
				if isNamespaceClassAlias(candidate, class) {
					return class.Name, true
				}
				if strings.Contains(candidate, ".") && !strings.Contains(class.Name, ".") {
					return candidate, true
				}
				return class.Name, true
			}
		}
		seenSupers := map[string]bool{}
		for super := vm.superClassName(owner); super != ""; super = vm.superClassName(super) {
			key := strings.ToLower(super)
			if seenSupers[key] {
				break
			}
			seenSupers[key] = true
			for _, ownerCandidate := range lexicalOwnerCandidates(super) {
				candidate := ownerCandidate + "." + typeName
				if class, ok := vm.lookupClass(candidate); ok {
					if isNamespaceClassAlias(candidate, class) {
						return class.Name, true
					}
					if strings.Contains(candidate, ".") && !strings.Contains(class.Name, ".") {
						return candidate, true
					}
					return class.Name, true
				}
			}
		}
		dot := strings.LastIndex(owner, ".")
		if dot < 0 {
			break
		}
		owner = owner[:dot]
	}
	return "", false
}

func isNamespaceClassAlias(candidate string, class Class) bool {
	return class.Namespace != "" &&
		!strings.Contains(class.Name, ".") &&
		strings.EqualFold(candidate, class.Namespace+"."+class.Name)
}

func lexicalOwnerCandidates(owner string) []string {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil
	}
	candidates := []string{owner}
	if short := shortTypeName(owner); short != "" && !strings.EqualFold(short, owner) {
		candidates = append(candidates, short)
	}
	return candidates
}

func (vm *VM) superClassName(className string) string {
	class, ok := vm.lookupClass(className)
	if !ok {
		return ""
	}
	return class.SuperClass
}

func (vm *VM) compareTypeSpecificity(left, right string) int {
	if strings.EqualFold(left, right) {
		return 0
	}
	leftToRight := vm.typeAssignableTo(left, right)
	rightToLeft := vm.typeAssignableTo(right, left)
	switch {
	case leftToRight && !rightToLeft:
		return 1
	case rightToLeft && !leftToRight:
		return -1
	case !leftToRight && !rightToLeft:
		return 2
	default:
		return 0
	}
}

func (vm *VM) typeAssignableTo(from, to string) bool {
	from = canonicalRuntimePlatformType(from)
	to = canonicalRuntimePlatformType(to)
	if strings.EqualFold(from, to) || strings.EqualFold(to, "Object") {
		return true
	}
	if namespaceQualifiedTypeEquivalent(from, to) {
		return true
	}
	if strings.EqualFold(to, "ApexPages.Component") && strings.HasPrefix(strings.ToLower(strings.TrimSpace(from)), "component.") {
		return true
	}
	if platformTokenTypeAlias(from, to) {
		return true
	}
	if frameworkMockSupportTypesMatch(from, to) {
		return true
	}
	if (strings.EqualFold(from, "String") && strings.EqualFold(to, "Id")) ||
		(strings.EqualFold(from, "Id") && strings.EqualFold(to, "String")) {
		return true
	}
	if messagingEmailAssignable(from, to) {
		return true
	}
	if strings.EqualFold(to, "Cache.Partition") &&
		(strings.EqualFold(from, "Cache.OrgPartition") || strings.EqualFold(from, "Cache.SessionPartition")) {
		return true
	}
	if strings.EqualFold(from, "Date") && strings.EqualFold(to, "Datetime") {
		return true
	}
	if strings.EqualFold(to, "sObject") && vm.isSObjectLikeType(from) {
		return true
	}
	if vm.isSObjectLikeType(from) && vm.isSObjectLikeType(to) && sObjectTypeNamespaceEquivalent(from, to) {
		return true
	}
	if collectionBase(from) != "" && strings.EqualFold(collectionBase(from), collectionBase(to)) {
		if vm.sObjectCollectionDowncastAssignable(from, to) {
			return true
		}
		fromElement, fromOK := collectionElementType(from)
		toElement, toOK := collectionElementType(to)
		if fromOK && toOK {
			return vm.typeAssignableTo(fromElement, toElement)
		}
	}
	if collectionBase(to) == "Iterable" && (collectionBase(from) == "List" || collectionBase(from) == "Set") {
		fromElement, fromOK := collectionElementType(from)
		toElement, toOK := collectionElementType(to)
		if fromOK && toOK {
			return vm.typeAssignableTo(fromElement, toElement)
		}
	}
	if fromKey, fromValue, fromOK := mapTypeArgs(from); fromOK {
		toKey, toValue, toOK := mapTypeArgs(to)
		if toOK {
			return vm.typeAssignableTo(fromKey, toKey) && vm.typeAssignableTo(fromValue, toValue)
		}
	}
	if numericConversionScore(to, from) >= 0 {
		return true
	}
	if _, ok := vm.typeDistance(from, to, make(map[string]bool)); ok {
		return true
	}
	return false
}

func namespaceQualifiedTypeEquivalent(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return strings.EqualFold(stripLeadingTypeNamespace(left), right) ||
		strings.EqualFold(left, stripLeadingTypeNamespace(right))
}

func stripLeadingTypeNamespace(typeName string) string {
	first := strings.Index(typeName, ".")
	if first < 0 {
		return typeName
	}
	rest := typeName[first+1:]
	if strings.Contains(rest, ".") {
		return rest
	}
	return typeName
}

func sObjectTypeNamespaceEquivalent(left, right string) bool {
	leftBase := stripSObjectNamespacePrefix(canonicalRuntimePlatformType(left))
	rightBase := stripSObjectNamespacePrefix(canonicalRuntimePlatformType(right))
	return strings.EqualFold(leftBase, rightBase)
}

func stripSObjectNamespacePrefix(typeName string) string {
	if strings.Contains(typeName, ".") {
		typeName = shortTypeName(typeName)
	}
	if !strings.Contains(typeName, "__") {
		return typeName
	}
	if strings.HasSuffix(strings.ToLower(typeName), "__c") ||
		strings.HasSuffix(strings.ToLower(typeName), "__mdt") ||
		strings.HasSuffix(strings.ToLower(typeName), "__e") ||
		strings.HasSuffix(strings.ToLower(typeName), "__x") {
		parts := strings.SplitN(typeName, "__", 2)
		if len(parts) == 2 && strings.Contains(parts[1], "__") {
			return parts[1]
		}
	}
	return typeName
}

func messagingEmailAssignable(from, to string) bool {
	if !strings.EqualFold(to, "Messaging.Email") {
		return false
	}
	return strings.EqualFold(from, "Messaging.SingleEmailMessage") ||
		strings.EqualFold(from, "Messaging.MassEmailMessage")
}

func collectionBase(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if strings.HasSuffix(typeName, "[]") && strings.TrimSpace(strings.TrimSuffix(typeName, "[]")) != "" {
		return "List"
	}
	base, ok := genericBaseName(typeName)
	if !ok {
		return ""
	}
	switch {
	case strings.EqualFold(base, "List"):
		return "List"
	case strings.EqualFold(base, "Set"):
		return "Set"
	case strings.EqualFold(base, "Iterator"):
		return "Iterator"
	case strings.EqualFold(base, "Iterable"):
		return "Iterable"
	default:
		return ""
	}
}

func isMapType(typeName string) bool {
	base, ok := genericBaseName(typeName)
	return ok && strings.EqualFold(base, "Map")
}

func genericBaseName(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	open := strings.IndexByte(typeName, '<')
	if open < 0 || !strings.HasSuffix(typeName, ">") {
		return "", false
	}
	base := strings.TrimSpace(typeName[:open])
	if rest, ok := stripLeadingSystemNamespace(base); ok {
		base = rest
	}
	return base, true
}

func (vm *VM) conversionScore(paramType string, value Value) int {
	if value.Kind == ValueNull {
		if value.Type != "" {
			if strings.EqualFold(paramType, value.Type) {
				return 1000
			}
			if collectionBase(value.Type) != "" && collectionBase(paramType) != "" {
				if vm.typeAssignableTo(value.Type, paramType) {
					return 900
				}
				if vm.sObjectCollectionDowncastAssignable(value.Type, paramType) {
					return 850
				}
				return -1
			}
			if isMapType(value.Type) && isMapType(paramType) {
				if vm.typeAssignableTo(value.Type, paramType) {
					return 900
				}
				return -1
			}
			if vm.typeAssignableTo(value.Type, paramType) {
				return 900
			}
			return 1
		}
		if collectionBase(paramType) != "" || isMapType(paramType) {
			return 0
		}
		return 1
	}
	if value.Static != "" {
		if strings.EqualFold(paramType, value.Static) {
			return 1000
		}
		if platformTokenTypeAlias(value.Static, paramType) {
			return 1000
		}
		if vm.typeAssignableTo(value.Static, paramType) {
			return 900
		}
	}
	valueType := valueTypeName(value)
	if strings.EqualFold(paramType, valueType) {
		return 1000
	}
	if platformTokenTypeAlias(valueType, paramType) {
		return 1000
	}
	if isDescribeSObjectResultType(paramType) && isSObjectTypeToken(value) {
		return 900
	}
	if collectionBase(valueType) != "" && collectionBase(paramType) != "" {
		if vm.typeAssignableTo(valueType, paramType) {
			return 900
		}
		if vm.sObjectCollectionDowncastAssignable(valueType, paramType) {
			return 850
		}
		if vm.collectionElementsAssignable(paramType, value) {
			return 850
		}
		return -1
	}
	if isMapType(valueType) && isMapType(paramType) {
		if vm.typeAssignableTo(valueType, paramType) {
			return 900
		}
		if vm.mapEntriesAssignable(paramType, value) {
			return 850
		}
		return -1
	}
	if score := numericConversionScore(paramType, valueType); score >= 0 {
		return score
	}
	if value.Kind == ValueObject && strings.EqualFold(paramType, value.Type) {
		return 950
	}
	if value.Kind == ValueObject && value.Runtime != "" && strings.EqualFold(paramType, value.Runtime) {
		return 925
	}
	if vm.typeAssignableTo(valueType, paramType) {
		return 900
	}
	if strings.EqualFold(paramType, "Object") {
		return 10
	}
	if value.Kind == ValueObject {
		if distance, ok := vm.typeDistance(value.Type, paramType, make(map[string]bool)); ok {
			return 800 - distance
		}
	}
	if value.Kind == ValueList && collectionBase(paramType) == "" && vm.isSObjectLikeType(paramType) {
		return -1
	}
	if err := vm.ensureAssignable(paramType, value); err != nil {
		return -1
	}
	return 1
}

func (vm *VM) sObjectCollectionDowncastAssignable(fromType, toType string) bool {
	fromElement, fromOK := collectionElementType(fromType)
	toElement, toOK := collectionElementType(toType)
	if !fromOK || !toOK {
		return false
	}
	if !strings.EqualFold(fromElement, "sObject") {
		return false
	}
	return vm.isSObjectLikeType(toElement)
}

func (vm *VM) collectionElementsAssignable(paramType string, value Value) bool {
	elementType, ok := collectionElementType(paramType)
	if !ok {
		return false
	}
	paramBase := collectionBase(paramType)
	switch value.Kind {
	case ValueList:
		if paramBase != "List" && paramBase != "Iterable" {
			return false
		}
		if len(value.List) == 0 {
			return false
		}
		for _, item := range value.List {
			if err := vm.ensureAssignable(elementType, item); err != nil {
				return false
			}
		}
		return true
	case ValueSet:
		if paramBase != "Set" && paramBase != "Iterable" {
			return false
		}
		if len(value.Set) == 0 {
			return false
		}
		for _, item := range value.Set {
			if err := vm.ensureAssignable(elementType, item); err != nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (vm *VM) mapEntriesAssignable(paramType string, value Value) bool {
	keyType, valueType, ok := mapTypeArgs(paramType)
	if !ok || value.Kind != ValueMap {
		return false
	}
	if len(value.Map) == 0 {
		// Local DTO JSON fields can carry untyped empty maps; overload specificity still picks the surface method.
		return true
	}
	for rawKey, item := range value.Map {
		if err := vm.ensureAssignable(keyType, valueFromMapKey(rawKey)); err != nil {
			return false
		}
		if err := vm.ensureAssignable(valueType, item); err != nil {
			return false
		}
	}
	return true
}

func numericConversionScore(paramType, valueType string) int {
	switch valueType {
	case "Integer":
		switch paramType {
		case "Long":
			return 900
		case "Decimal":
			return 800
		case "Double":
			return 700
		}
	case "Long":
		switch paramType {
		case "Decimal":
			return 800
		case "Double":
			return 700
		}
	case "Decimal":
		if paramType == "Double" {
			return 800
		}
	}
	return -1
}

func (vm *VM) typeDistance(typeName, target string, seen map[string]bool) (int, bool) {
	typeName = systemInterfaceAlias(typeName)
	target = systemInterfaceAlias(target)
	if resolved, ok := vm.resolveClassName(typeName); ok {
		typeName = resolved
	}
	if resolved, ok := vm.resolveClassName(target); ok {
		target = resolved
	}
	if typeName == "" || seen[typeName] {
		return 0, false
	}
	if strings.EqualFold(typeName, target) {
		return 0, true
	}
	seen[typeName] = true
	class, ok := vm.lookupClass(typeName)
	if !ok {
		return 0, false
	}
	best := 0
	found := false
	if distance, ok := vm.typeDistance(class.SuperClass, target, seen); ok {
		best = distance + 1
		found = true
	}
	for _, iface := range class.Interfaces {
		if distance, ok := vm.typeDistance(iface, target, seen); ok {
			distance++
			if !found || distance < best {
				best = distance
				found = true
			}
		}
		if !strings.Contains(iface, ".") {
			if distance, ok := vm.typeDistance(class.Name+"."+iface, target, seen); ok {
				distance++
				if !found || distance < best {
					best = distance
					found = true
				}
			}
		}
	}
	return best, found
}

func valueTypeName(value Value) string {
	switch value.Kind {
	case ValueInt:
		if value.Type != "" {
			return value.Type
		}
		return "Integer"
	case ValueDecimal:
		return "Decimal"
	case ValueBool:
		return "Boolean"
	case ValueString:
		if value.Type != "" {
			return value.Type
		}
		return "String"
	case ValueList:
		if value.Type != "" {
			return value.Type
		}
		return "List"
	case ValueSet:
		if value.Type != "" {
			return value.Type
		}
		return "Set"
	case ValueMap:
		if value.Type != "" {
			return value.Type
		}
		return "Map"
	case ValueObject:
		if value.Static != "" {
			return value.Static
		}
		return value.Type
	case ValueNull:
		return "null"
	default:
		return string(value.Kind)
	}
}

func runtimeValueTypeName(value Value) string {
	if value.Kind == ValueObject {
		if runtime := runtimeObjectType(value); runtime != "" {
			return runtime
		}
		if value.Static != "" {
			return value.Static
		}
		return "Object"
	}
	if value.Kind == ValueList || value.Kind == ValueSet || value.Kind == ValueMap {
		if typed := typeExceptionCollectionName(value.Type); typed != "" {
			return typed
		}
		if value.Kind == ValueMap {
			keyType := "ANY"
			for _, key := range value.MapKeys {
				if key.Kind == ValueString {
					keyType = "String"
					break
				}
			}
			return "Map<" + keyType + ",ANY>"
		}
		if value.Kind == ValueList {
			return "List<ANY>"
		}
		return "Set<ANY>"
	}
	return valueTypeName(value)
}

func newExceptionError(typeName, message string) error {
	value := Object(typeName)
	value.Fields["message"] = String(message)
	return &apexThrowError{value: value}
}

func newExceptionErrorWithContext(typeName, message, context string) error {
	value := Object(typeName)
	value.Fields["message"] = String(message)
	if strings.TrimSpace(context) != "" {
		value.Fields["__diagnosticContext"] = String(context)
	}
	return &apexThrowError{value: value}
}

func newNullDereferenceError(context string) error {
	return newExceptionErrorWithContext("NullPointerException", "Attempt to de-reference a null object", context)
}

func listIndexException(index int) error {
	return newExceptionError("ListException", fmt.Sprintf("List index out of bounds: %d", index))
}

func annotateException(value Value, stack []callFrame) Value {
	if value.Kind != ValueObject || !isExceptionType(value.Type) {
		return value
	}
	value.Fields["__thrown"] = Bool(true)
	if len(stack) == 0 {
		return value
	}
	if _, ok := value.Fields["__stackTrace"]; ok {
		return value
	}
	value.Fields["__lineNumber"] = Int(int64(stack[len(stack)-1].Line))
	value.Fields["__stackTrace"] = String(stackTraceString(stack))
	return value
}

func stackTraceString(stack []callFrame) string {
	frames := stackFrames(stack)
	lines := make([]string, 0, len(frames))
	for _, frame := range frames {
		if frame.Line > 0 {
			line := apexStackFrameSymbol(frame.Symbol) + ": line " + strconv.Itoa(frame.Line)
			if frame.Column > 0 {
				line += ", column " + strconv.Itoa(frame.Column)
			}
			lines = append(lines, line)
			continue
		}
		lines = append(lines, apexStackFrameSymbol(frame.Symbol))
	}
	return strings.Join(lines, "\n")
}

func apexStackFrameSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return "AnonymousBlock"
	}
	lower := strings.ToLower(symbol)
	if strings.HasPrefix(lower, "class.") || strings.HasPrefix(lower, "trigger.") || symbol == "AnonymousBlock" {
		return symbol
	}
	if strings.Contains(symbol, ".") {
		return "Class." + symbol
	}
	return symbol
}

func catchTypes(inst ir.Instruction) []string {
	if len(inst.CatchTypes) > 0 {
		return inst.CatchTypes
	}
	return []string{inst.Type}
}

func vmCatchClauses(inst ir.Instruction) []ir.CatchClause {
	if len(inst.Catches) > 0 {
		return inst.Catches
	}
	if len(inst.Catch) == 0 {
		return nil
	}
	return []ir.CatchClause{{Types: catchTypes(inst), Name: inst.Name, Body: inst.Catch, Pos: inst.Pos}}
}

func (vm *VM) exceptionMatchesAny(catchTypes []string, thrown Value) bool {
	for _, catchType := range catchTypes {
		if vm.exceptionMatches(catchType, thrown) {
			return true
		}
	}
	return false
}

func (vm *VM) exceptionMatches(catchType string, thrown Value) bool {
	if catchType == "" || exceptionTypeName(catchType) == "Exception" || strings.EqualFold(catchType, "Object") {
		return true
	}
	if thrown.Kind == ValueObject {
		if vm.typeMatches(thrown.Type, catchType, make(map[string]bool)) {
			return true
		}
		return strings.EqualFold(lastTypeSegment(exceptionTypeName(thrown.Type)), lastTypeSegment(exceptionTypeName(catchType)))
	}
	return false
}

func lastTypeSegment(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if i := strings.LastIndex(typeName, "."); i >= 0 {
		return typeName[i+1:]
	}
	return typeName
}

func (vm *VM) typeMatches(typeName, target string, seen map[string]bool) bool {
	typeName = systemInterfaceAlias(typeName)
	target = systemInterfaceAlias(target)
	if resolved, ok := vm.resolveClassName(typeName); ok {
		typeName = resolved
	}
	if resolved, ok := vm.resolveClassName(target); ok {
		target = resolved
	}
	if vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(typeName); ok {
			typeName = resolved
		}
		if resolved, ok := vm.resolveObjectName(target); ok {
			target = resolved
		}
	}
	if typeName == "" || seen[typeName] {
		return false
	}
	if strings.EqualFold(typeName, target) {
		return true
	}
	if vm.classNamesReferToSameRuntimeType(typeName, target) {
		return true
	}
	if !strings.Contains(target, ".") && strings.EqualFold(shortTypeName(typeName), target) {
		return true
	}
	if platformTokenTypeAlias(typeName, target) {
		return true
	}
	if frameworkMockSupportTypesMatch(typeName, target) {
		return true
	}
	if strings.EqualFold(target, "sObject") && vm.isSObjectLikeType(typeName) {
		return true
	}
	if builtinExceptionTypeMatches(typeName, target) {
		return true
	}
	seen[typeName] = true
	class, ok := vm.lookupClass(typeName)
	if !ok {
		return false
	}
	if class.SuperClass != "" {
		if !strings.Contains(class.SuperClass, ".") {
			if nestedSuperClass, ok := nestedSiblingTypeName(class.Name, class.SuperClass); ok && vm.typeMatches(nestedSuperClass, target, seen) {
				return true
			}
		}
		if vm.typeMatches(class.SuperClass, target, seen) {
			return true
		}
	}
	for _, iface := range class.Interfaces {
		if strings.EqualFold(shortTypeName(iface), shortTypeName(target)) {
			return true
		}
		if vm.typeMatches(iface, target, seen) {
			return true
		}
		if !strings.Contains(iface, ".") && vm.typeMatches(class.Name+"."+iface, target, seen) {
			return true
		}
	}
	return false
}

func (vm *VM) classNamesReferToSameRuntimeType(left, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	if !strings.EqualFold(shortTypeName(left), shortTypeName(right)) {
		return false
	}
	leftClass, leftOK := vm.lookupClass(left)
	rightClass, rightOK := vm.lookupClass(right)
	if leftOK && rightOK {
		return strings.EqualFold(vm.classTypeToken(leftClass), vm.classTypeToken(rightClass))
	}
	if leftOK {
		return strings.EqualFold(vm.classTypeToken(leftClass), right)
	}
	if rightOK {
		return strings.EqualFold(left, vm.classTypeToken(rightClass))
	}
	return false
}

func nestedSiblingTypeName(className, typeName string) (string, bool) {
	if strings.TrimSpace(typeName) == "" || strings.Contains(typeName, ".") {
		return "", false
	}
	dot := strings.LastIndex(className, ".")
	if dot <= 0 || dot == len(className)-1 {
		return "", false
	}
	return className[:dot+1] + typeName, true
}

func platformTokenTypeAlias(typeName, target string) bool {
	switch {
	case strings.EqualFold(typeName, "Schema.SObjectType") && strings.EqualFold(target, "SObjectType"):
		return true
	case strings.EqualFold(typeName, "SObjectType") && strings.EqualFold(target, "Schema.SObjectType"):
		return true
	case strings.EqualFold(typeName, "Schema.SObjectField") && strings.EqualFold(target, "SObjectField"):
		return true
	case strings.EqualFold(typeName, "SObjectField") && strings.EqualFold(target, "Schema.SObjectField"):
		return true
	case strings.EqualFold(typeName, "Schema.FieldSet") && strings.EqualFold(target, "FieldSet"):
		return true
	case strings.EqualFold(typeName, "FieldSet") && strings.EqualFold(target, "Schema.FieldSet"):
		return true
	case strings.EqualFold(typeName, "Schema.FieldSetMember") && strings.EqualFold(target, "FieldSetMember"):
		return true
	case strings.EqualFold(typeName, "FieldSetMember") && strings.EqualFold(target, "Schema.FieldSetMember"):
		return true
	case strings.EqualFold(typeName, "Schema.DescribeFieldResult") && strings.EqualFold(target, "DescribeFieldResult"):
		return true
	case strings.EqualFold(typeName, "DescribeFieldResult") && strings.EqualFold(target, "Schema.DescribeFieldResult"):
		return true
	case strings.EqualFold(typeName, "Schema.DescribeSObjectResult") && strings.EqualFold(target, "DescribeSObjectResult"):
		return true
	case strings.EqualFold(typeName, "DescribeSObjectResult") && strings.EqualFold(target, "Schema.DescribeSObjectResult"):
		return true
	case strings.EqualFold(typeName, "PageReference") && strings.EqualFold(target, "ApexPages.PageReference"):
		return true
	case strings.EqualFold(typeName, "ApexPages.PageReference") && strings.EqualFold(target, "PageReference"):
		return true
	default:
		return false
	}
}

func shortTypeName(typeName string) string {
	if i := strings.LastIndex(typeName, "."); i >= 0 {
		return typeName[i+1:]
	}
	return typeName
}

func systemInterfaceAlias(typeName string) string {
	switch strings.TrimSpace(typeName) {
	case "System.Callable":
		return "Callable"
	case "System.StubProvider":
		return "StubProvider"
	default:
		return typeName
	}
}

func (vm *VM) evalInstanceOf(value Value, target string) Value {
	target = strings.TrimSpace(target)
	if target == "" {
		return Bool(false)
	}
	if value.Kind == ValueNull {
		return Bool(strings.EqualFold(value.Type, "Object") && (collectionBase(target) != "" || isMapType(target)))
	}
	if strings.EqualFold(target, "Id") && value.Kind == ValueString {
		return Bool(validateApexIDShape(value.Text) == nil)
	}
	if strings.EqualFold(target, "String") && value.Kind == ValueString && strings.EqualFold(value.Type, "Id") {
		return Bool(true)
	}
	if strings.EqualFold(target, "Datetime") && value.Kind == ValueObject && strings.EqualFold(value.Type, "Date") {
		return Bool(true)
	}
	if value.Kind == ValueObject {
		return Bool(vm.typeMatches(runtimeObjectType(value), target, make(map[string]bool)))
	}
	if collectionBase(target) != "" || isMapType(target) {
		valueType := valueTypeName(value)
		if vm.typeAssignableTo(valueType, target) {
			return Bool(true)
		}
		if collectionBase(target) != "" && vm.collectionElementsAssignable(target, value) {
			return Bool(true)
		}
		if isMapType(target) && vm.mapEntriesAssignable(target, value) {
			return Bool(true)
		}
		return Bool(false)
	}
	if strings.EqualFold(target, "Object") {
		return Bool(true)
	}
	valueType := valueTypeName(value)
	if strings.EqualFold(valueType, target) {
		return Bool(true)
	}
	return Bool(numericConversionScore(target, valueType) >= 0)
}

func builtinExceptionTypeMatches(typeName, target string) bool {
	typeName = exceptionTypeName(typeName)
	target = exceptionTypeName(target)
	if typeName == "" || target == "" {
		return false
	}
	for current := typeName; current != ""; current = builtinExceptionParent(current) {
		if current == target {
			return true
		}
	}
	return false
}

func builtinExceptionParent(typeName string) string {
	typeName = exceptionTypeName(typeName)
	if parent, ok := builtinExceptionParents[typeName]; ok {
		return parent
	}
	if strings.HasSuffix(typeName, "Exception") {
		return "Exception"
	}
	return ""
}

func isBuiltinExceptionType(typeName string) bool {
	typeName = exceptionTypeName(typeName)
	for known := range builtinExceptionParents {
		if strings.EqualFold(typeName, known) {
			return true
		}
	}
	return false
}

var builtinExceptionParents = map[string]string{
	"Exception": "Object",

	"AssertException":                 "Exception",
	"AuraHandledException":            "Exception",
	"AsyncException":                  "Exception",
	"CalloutException":                "Exception",
	"DmlException":                    "Exception",
	"EmailException":                  "Exception",
	"ExternalObjectException":         "Exception",
	"IllegalArgumentException":        "Exception",
	"IllegalStateException":           "Exception",
	"InvalidParameterValueException":  "Exception",
	"JSONException":                   "Exception",
	"LimitException":                  "Exception",
	"ListException":                   "Exception",
	"MathException":                   "Exception",
	"NoAccessException":               "Exception",
	"NoDataFoundException":            "Exception",
	"NoSuchElementException":          "Exception",
	"NullPointerException":            "Exception",
	"PatternSyntaxException":          "IllegalArgumentException",
	"QueryException":                  "Exception",
	"RequiredFeatureMissingException": "Exception",
	"SearchException":                 "Exception",
	"SecurityException":               "Exception",
	"SerializationException":          "Exception",
	"SObjectException":                "Exception",
	"StringException":                 "Exception",
	"TypeException":                   "Exception",
	"VisualforceException":            "Exception",
	"XmlException":                    "Exception",
}

func exceptionTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimPrefix(typeName, "System.")
	return typeName
}

func exceptionQualifiedTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if !strings.Contains(typeName, ".") && isBuiltinExceptionType(typeName) {
		return "System." + typeName
	}
	return typeName
}

func (vm *VM) exceptionQualifiedTypeName(typeName string) string {
	qualified := exceptionQualifiedTypeName(typeName)
	if class, ok := vm.lookupClass(typeName); ok && strings.TrimSpace(class.Namespace) != "" {
		prefix := class.Namespace + "."
		if !strings.HasPrefix(strings.ToLower(qualified), strings.ToLower(prefix)) {
			return prefix + qualified
		}
	}
	return qualified
}

func exceptionToString(value Value) string {
	typeName := exceptionTypeName(value.Type)
	if typeName == "" {
		typeName = "Exception"
	}
	message := ""
	if raw, ok := value.Fields["message"]; ok && raw.Kind == ValueString {
		message = raw.Text
	}
	prefix := "System." + typeName
	if message == "" {
		return prefix
	}
	return prefix + ": " + message
}

func typeValueName(value Value) string {
	if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
		return raw.Text
	}
	return value.Text
}

var loggingLevelNames = []string{"NONE", "ERROR", "WARN", "INFO", "DEBUG", "FINE", "FINER", "FINEST"}

var apexPagesSeverityNames = []string{"CONFIRM", "INFO", "WARNING", "ERROR", "FATAL"}

var triggerOperationNames = []string{"BEFORE_INSERT", "BEFORE_UPDATE", "BEFORE_DELETE", "AFTER_INSERT", "AFTER_UPDATE", "AFTER_DELETE", "AFTER_UNDELETE"}

var metadataDeployStatusNames = []string{"Succeeded", "SUCCEEDED", "Failed", "FAILED", "InProgress", "INPROGRESS", "Pending", "PENDING", "Canceling", "CANCELING", "Canceled", "CANCELED"}

var metadataMetadataTypeNames = []string{"CustomMetadata"}

func isLoggingLevelName(level string) bool {
	_, ok := canonicalLoggingLevelName(level)
	return ok
}

func canonicalLoggingLevelName(level string) (string, bool) {
	for _, name := range loggingLevelNames {
		if strings.EqualFold(level, name) {
			return name, true
		}
	}
	return "", false
}

func isLoggingLevelValue(value Value) bool {
	if value.Kind != ValueObject || value.Type != "LoggingLevel" {
		return false
	}
	return isLoggingLevelName(value.Text)
}

func (vm *VM) coerceAssignable(typeName string, value Value) (Value, error) {
	canonicalTypeName := typeName
	if rest, ok := stripLeadingSystemNamespace(canonicalTypeName); ok {
		canonicalTypeName = rest
	}
	canonicalValueType := value.Type
	if rest, ok := stripLeadingSystemNamespace(canonicalValueType); ok {
		canonicalValueType = rest
	}
	if canonicalValueType != "" && strings.EqualFold(canonicalValueType, canonicalTypeName) {
		text := ""
		if value.Kind == ValueString {
			text = value.Text
		} else if objectText, ok := platformScalarObjectText(value); ok {
			text = objectText
		} else if value.Kind == ValueObject && value.Text != "" {
			text = value.Text
		}
		if text != "" {
			switch {
			case strings.EqualFold(canonicalTypeName, "Date") && strings.EqualFold(strings.TrimSpace(text), "Today()"):
				return platformScalar("Date", vm.fakeNow.Format("2006-01-02")), nil
			case strings.EqualFold(canonicalTypeName, "Date"):
				parsed, err := parseDateText(text)
				if err == nil {
					return platformScalar("Date", parsed.Format("2006-01-02")), nil
				}
			case strings.EqualFold(canonicalTypeName, "Datetime") && strings.EqualFold(strings.TrimSpace(text), "Now()"):
				return platformScalar("Datetime", formatPlatformDatetime(vm.fakeNow)), nil
			}
		}
		return value, nil
	}
	if strings.EqualFold(canonicalTypeName, "Datetime") && strings.EqualFold(canonicalValueType, "Date") {
		text := ""
		if value.Kind == ValueString {
			text = value.Text
		} else if objectText, ok := platformScalarObjectText(value); ok {
			text = objectText
		} else if value.Kind == ValueObject && value.Text != "" {
			text = value.Text
		}
		if strings.EqualFold(strings.TrimSpace(text), "Today()") {
			today := time.Date(vm.fakeNow.Year(), vm.fakeNow.Month(), vm.fakeNow.Day(), 0, 0, 0, 0, time.UTC)
			return platformScalar("Datetime", formatPlatformDatetime(today)), nil
		}
	}
	if value.Type != "" && strings.EqualFold(value.Type, typeName) {
		return value, nil
	}
	if value.Kind == ValueNull {
		value.Type = typeName
		return value, nil
	}
	if value.Kind == ValueMap && (vm.isSObjectLikeType(typeName) || vm.isJSONTypedObjectTarget(typeName)) {
		return vm.typedValueFromJSON(typeName, jsonFromValue(value, false), false)
	}
	if strings.EqualFold(typeName, "String") {
		if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
			idText, ok := platformScalarObjectText(value)
			if !ok {
				return String(""), nil
			}
			if len(idText) == 15 {
				return String(apexIDTo18(idText)), nil
			}
			return String(idText), nil
		}
	}
	if value.Kind == ValueString {
		if strings.EqualFold(typeName, "String") && strings.EqualFold(value.Type, "Id") {
			if len(value.Text) == 15 {
				return String(apexIDTo18(value.Text)), nil
			}
			return String(value.Text), nil
		}
		if class, ok := vm.resolveEnumClass(typeName); ok {
			valueText := value.Text
			if dot := strings.LastIndexByte(valueText, '.'); dot >= 0 {
				valueText = valueText[dot+1:]
			}
			for _, enumValue := range class.EnumValues {
				if enumValue == valueText {
					return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
				}
			}
		}
		if apexIdentifierStartsUpper(typeName) && !isBuiltinTypeName(typeName) && !platformScalarObject(typeName) && stringEnumCoercionTarget(typeName) {
			valueText := value.Text
			if dot := strings.LastIndexByte(valueText, '.'); dot >= 0 {
				valueText = valueText[dot+1:]
			}
			return Value{Kind: ValueObject, Type: typeName, Text: valueText}, nil
		}
		switch typeName {
		case "Id":
			if err := validateApexIDShape(value.Text); err != nil {
				return Null, newExceptionError("StringException", strings.TrimPrefix(err.Error(), "System.StringException: "))
			}
			value.Type = "Id"
			return value, nil
		case "Date":
			if strings.EqualFold(strings.TrimSpace(value.Text), "Today()") {
				return platformScalar("Date", vm.fakeNow.Format("2006-01-02")), nil
			}
			parsed, err := parseDateText(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Date", parsed.Format("2006-01-02")), nil
		case "Datetime":
			if strings.EqualFold(strings.TrimSpace(value.Text), "Now()") {
				return platformScalar("Datetime", formatPlatformDatetime(vm.fakeNow)), nil
			}
			if strings.EqualFold(value.Type, "Date") {
				parsed, err := parseDateText(value.Text)
				if err != nil {
					return Null, err
				}
				return platformScalar("Datetime", formatPlatformDatetime(parsed)), nil
			}
			parsed, err := parseDatetimeTextAllowDateOnly(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Datetime", formatPlatformDatetime(parsed)), nil
		case "Time":
			parsed, err := parseTimeText(value.Text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Time", parsed), nil
		}
	}
	if value.Kind == ValueObject {
		if isStubProxy(value) && vm.typeMatches(value.Type, typeName, make(map[string]bool)) {
			if !strings.EqualFold(typeName, "Object") && !strings.EqualFold(value.Type, typeName) {
				if value.Runtime == "" {
					value.Runtime = value.Type
				}
				value.Static = typeName
			}
			return value, nil
		}
		if strings.EqualFold(typeName, "Database.QueryLocator") && value.Type == "Database.QueryLocator" {
			return value, nil
		}
		if isDescribeSObjectResultType(typeName) && isSObjectTypeToken(value) {
			return vm.describeFromSObjectTypeToken(value)
		}
		if isDescribeFieldResultType(typeName) && isSObjectFieldTokenType(value.Type) {
			return vm.describeFromSObjectFieldToken(value)
		}
		if class, ok := vm.resolveEnumClass(typeName); ok && len(class.EnumValues) > 0 {
			if strings.EqualFold(value.Type, class.Name) {
				return Value{Kind: ValueObject, Type: class.Name, Text: value.Text}, nil
			}
		}
		if strings.EqualFold(typeName, "Date") && (strings.EqualFold(value.Type, "Datetime") || strings.EqualFold(value.Type, "DateTime")) {
			parsed, err := parsePlatformDatetime(value)
			if err != nil {
				return Null, err
			}
			return platformScalar("Date", parsed.Format("2006-01-02")), nil
		}
		if strings.EqualFold(typeName, "Datetime") && strings.EqualFold(value.Type, "Date") {
			text, err := platformScalarText(value, "Date")
			if err != nil {
				return Null, err
			}
			parsed, err := parseDateText(text)
			if err != nil {
				return Null, err
			}
			return platformScalar("Datetime", formatPlatformDatetime(parsed)), nil
		}
		if strings.EqualFold(typeName, "String") && (strings.EqualFold(value.Type, "Id") || strings.EqualFold(value.Type, "String") || strings.EqualFold(value.Type, "UUID")) {
			text, err := platformScalarText(value, value.Type)
			if err != nil {
				return Null, err
			}
			return String(text), nil
		}
		if strings.EqualFold(typeName, "Id") && strings.EqualFold(value.Type, "String") {
			text, err := platformScalarText(value, value.Type)
			if err != nil {
				return Null, err
			}
			return platformScalar("Id", text), nil
		}
		if collectionBase(typeName) == "Iterator" && isIteratorValue(value) {
			value.Type = typeName
			return value, nil
		}
		if collectionBase(typeName) == "Iterable" && value.Type == "Database.QueryLocator" {
			return vm.queryLocatorIterable(typeName, value)
		}
		if strings.EqualFold(typeName, "Object") || vm.typeAssignableTo(value.Type, typeName) || vm.typeMatches(value.Type, typeName, make(map[string]bool)) {
			if !strings.EqualFold(typeName, "Object") && !strings.EqualFold(value.Type, typeName) {
				if value.Runtime == "" {
					value.Runtime = value.Type
				}
				value.Static = typeName
			}
			return value, nil
		}
		if managedAPIVersionedTypeAssignable(value.Type, typeName) {
			if value.Runtime == "" {
				value.Runtime = value.Type
			}
			value.Static = typeName
			return value, nil
		}
		return Null, fmt.Errorf("cannot assign %s to %s", value.Type, typeName)
	}
	if value.Kind == ValueList && vm.isSObjectLikeType(typeName) {
		if len(value.List) == 0 {
			return Null, newExceptionError("QueryException", "List has no rows for assignment to SObject")
		}
		if len(value.List) > 1 {
			return Null, newExceptionError("QueryException", "List has more than 1 row for assignment to SObject")
		}
		return vm.coerceAssignable(typeName, value.List[0])
	}
	if (strings.EqualFold(typeName, "Decimal") || strings.EqualFold(typeName, "Double")) && value.Kind == ValueInt {
		decimal := Decimal(float64(value.Int))
		decimal.Text = strconv.FormatInt(value.Int, 10)
		return decimal, nil
	}
	if (strings.EqualFold(typeName, "Integer") || strings.EqualFold(typeName, "Long")) && untypedIntegralDecimalLiteral(value) {
		if value.Decimal < float64(math.MinInt64) || value.Decimal > float64(math.MaxInt64) {
			return Null, fmt.Errorf("cannot assign decimal to %s", typeName)
		}
		return Int(int64(value.Decimal)), nil
	}
	if collectionBase(typeName) == "List" && value.Kind == ValueList {
		sourceTypes := []string{value.Type, value.Runtime, value.Static}
		if value.Runtime == "" && value.Type != "" && !strings.EqualFold(value.Type, typeName) {
			value.Runtime = value.Type
		}
		value.Type = typeName
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		if strings.EqualFold(elementType, "SObject") {
			return value, nil
		}
		if len(value.List) == 0 && vm.isSObjectLikeType(elementType) {
			for _, sourceType := range sourceTypes {
				sourceElementType, ok := collectionElementType(sourceType)
				if !ok || strings.EqualFold(sourceElementType, "SObject") || strings.EqualFold(sourceElementType, "Object") {
					continue
				}
				if vm.typeAssignableTo(sourceElementType, elementType) {
					continue
				}
				return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", typeExceptionAnyName(sourceType), typeExceptionAnyName(typeName)))
			}
		}
		for _, sourceType := range []string{value.Runtime, value.Static} {
			if sourceElementType, ok := collectionElementType(sourceType); ok &&
				strings.EqualFold(sourceElementType, "SObject") && vm.isSObjectLikeType(elementType) {
				return value, nil
			}
		}
		for i, item := range value.List {
			if strings.EqualFold(elementType, "Id") && item.Kind == ValueString && item.Text == "" {
				continue
			}
			coerced, err := vm.coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			value.List[i] = coerced
		}
		return value, nil
	}
	if collectionBase(typeName) == "List" && value.Kind == ValueMap {
		if records, ok := queryResultRecordsList(value); ok {
			return vm.coerceAssignable(typeName, records)
		}
	}
	if strings.EqualFold(typeName, "Database.QueryLocator") && value.Kind == ValueList {
		if locator, ok := value.Fields["__queryLocator"]; ok && locator.Kind == ValueObject && locator.Type == "Database.QueryLocator" {
			return locator, nil
		}
	}
	if collectionBase(typeName) == "Set" && value.Kind == ValueSet {
		value.Type = typeName
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		out := make([]Value, 0, len(value.Set))
		for _, item := range value.Set {
			coerced, err := vm.coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			if !containsValue(out, coerced) {
				out = append(out, coerced)
			}
		}
		value.Set = out
		return value, nil
	}
	if collectionBase(typeName) == "Iterable" && (value.Kind == ValueList || value.Kind == ValueSet) {
		value.Type = typeName
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		items := collectionMembers(value)
		out := make([]Value, 0, len(items))
		for _, item := range items {
			coerced, err := vm.coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			out = append(out, coerced)
		}
		if value.Kind == ValueSet {
			value.Set = out
		} else {
			value.List = out
		}
		return value, nil
	}
	if collectionBase(typeName) == "Iterable" && value.Kind == ValueObject && value.Type == "Database.QueryLocator" {
		return vm.queryLocatorIterable(typeName, value)
	}
	if isMapType(typeName) && value.Kind == ValueMap {
		sourceType := value.Type
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			value.Type = typeName
			return value, nil
		}
		type coercedEntry struct {
			key      string
			keyValue Value
			value    Value
		}
		entries := make([]coercedEntry, 0, len(value.Map))
		for _, rawKey := range orderedValueMapKeys(value) {
			item := value.Map[rawKey]
			keyValue := mapStoredKey(value, rawKey)
			coercedKey, err := vm.coerceAssignable(keyType, keyValue)
			if err != nil {
				return Null, fmt.Errorf("key: %w", err)
			}
			coercedValue, err := vm.coerceAssignable(valueType, item)
			if err != nil {
				return Null, fmt.Errorf("value: %w", err)
			}
			entries = append(entries, coercedEntry{key: mapKey(coercedKey), keyValue: coercedKey, value: coercedValue})
		}
		for rawKey := range value.Map {
			delete(value.Map, rawKey)
		}
		value.MapKeys = make(map[string]Value, len(entries))
		value.MapOrder = make([]string, 0, len(entries))
		for _, entry := range entries {
			if _, exists := value.Map[entry.key]; !exists {
				value.MapOrder = append(value.MapOrder, entry.key)
			}
			value.Map[entry.key] = entry.value
			value.MapKeys[entry.key] = entry.keyValue
		}
		if strings.EqualFold(valueType, "sObject") && mapConcreteSObjectValueType(sourceType) != "" {
			value.Type = sourceType
		} else {
			value.Type = typeName
		}
		return value, nil
	}
	return coerceAssignable(typeName, value)
}
