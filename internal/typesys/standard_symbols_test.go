package typesys

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

func TestStandardPlatformSymbolsMergeProductNamespaceDeclarations(t *testing.T) {
	symbols := StandardPlatformSymbols()

	operations := requireStandardSymbol(t, symbols, "Metadata.Operations")
	requireStandardMethod(t, operations, "retrieve", []string{"Metadata.MetadataType", "List<String>", "Boolean"}, true)
	requireStandardMethod(t, operations, "checkDeployStatus", []string{"Id", "Boolean"}, true)

	container := requireStandardSymbol(t, symbols, "Metadata.DeployContainer")
	requireStandardConstructor(t, container, []string{})
	requireStandardMethod(t, container, "addMetadata", []string{"Metadata.Metadata"}, false)

	settings := requireStandardSymbol(t, symbols, "ConnectApi.OrganizationSettings")
	requireStandardProperty(t, settings, "userSettings", "ConnectApi.UserSettings")

	visibility := requireStandardSymbol(t, symbols, "Cache.Visibility")
	if visibility.Kind != apexast.DeclarationEnum {
		t.Fatalf("Cache.Visibility kind = %q, want enum", visibility.Kind)
	}
	requireStandardProperty(t, visibility, "ALL", "Cache.Visibility")
}

func TestStandardPlatformSymbolsReturnsIndependentSlices(t *testing.T) {
	first := StandardPlatformSymbols()
	if len(first) == 0 {
		t.Fatal("missing standard symbols")
	}
	firstName := first[0].Name
	first[0].Name = "__mutated__"
	if again := StandardPlatformSymbols(); again[0].Name != firstName {
		t.Fatalf("standard symbol slice shares element mutation: got %q want %q", again[0].Name, firstName)
	}

	memberType := -1
	for i, symbol := range first {
		if len(symbol.Members) > 0 {
			memberType = i
			break
		}
	}
	if memberType < 0 {
		t.Fatal("missing standard symbol members")
	}
	second := StandardPlatformSymbols()
	memberName := second[memberType].Members[0].Name
	second[memberType].Members[0].Name = "__mutated_member__"
	if again := StandardPlatformSymbols(); again[memberType].Members[0].Name != memberName {
		t.Fatalf("standard symbol members share mutation: got %q want %q", again[memberType].Members[0].Name, memberName)
	}
}

func TestCloneTypeSymbolsCopiesNestedModifiers(t *testing.T) {
	input := []TypeSymbol{{
		Name:       "Trail",
		Modifiers:  []string{"public"},
		Interfaces: []string{"Runnable"},
		Members: []MemberSymbol{{
			Name:      "run",
			Modifiers: []string{"public"},
			Parameters: []apexast.Parameter{{
				Name:      "scope",
				Modifiers: []string{"final"},
			}},
			Accessors: []apexast.Accessor{{
				Kind:      "get",
				Modifiers: []string{"public"},
			}},
		}},
	}}

	copy := cloneTypeSymbols(input)
	copy[0].Modifiers[0] = "private"
	copy[0].Interfaces[0] = "Changed"
	copy[0].Members[0].Modifiers[0] = "private"
	copy[0].Members[0].Parameters[0].Modifiers[0] = "var"
	copy[0].Members[0].Accessors[0].Modifiers[0] = "private"

	if input[0].Modifiers[0] != "public" ||
		input[0].Interfaces[0] != "Runnable" ||
		input[0].Members[0].Modifiers[0] != "public" ||
		input[0].Members[0].Parameters[0].Modifiers[0] != "final" ||
		input[0].Members[0].Accessors[0].Modifiers[0] != "public" {
		t.Fatalf("cloneTypeSymbols shared nested modifier slices: %#v", input)
	}
}

func TestStandardSymbolsFromSpecsMergesDuplicateTypesCaseInsensitively(t *testing.T) {
	symbols := StandardSymbolsFromSpecs([]StandardSymbolSpec{{
		Name: "ConnectApi.Organization",
		Methods: []StandardMethodSpec{{
			Name:       "getSettings",
			ReturnType: "ConnectApi.OrganizationSettings",
			Static:     true,
		}},
	}, {
		Name: "connectapi.organization",
		Methods: []StandardMethodSpec{{
			Name:       "getSettings",
			ReturnType: "ConnectApi.OrganizationSettings",
			Static:     true,
		}, {
			Name:       "getHealth",
			ReturnType: "Object",
			Static:     true,
		}},
	}})

	if len(symbols) != 1 {
		t.Fatalf("symbol count = %d, want 1: %#v", len(symbols), symbols)
	}
	requireStandardMethod(t, symbols[0], "getSettings", nil, true)
	requireStandardMethod(t, symbols[0], "getHealth", nil, true)
}

func TestStandardPlatformSymbolsIncludeInstallVersion(t *testing.T) {
	symbols := StandardPlatformSymbols()

	version := requireStandardSymbol(t, symbols, "Version")
	requireStandardConstructor(t, version, []string{"Integer", "Integer"})
	requireStandardConstructor(t, version, []string{"Integer", "Integer", "Integer"})
	requireStandardMethod(t, version, "compareTo", []string{"Version"}, false)

	installContext := requireStandardSymbol(t, symbols, "InstallContext")
	requireStandardMethod(t, installContext, "previousVersion", nil, false)
}

func TestStandardPlatformSymbolsQualifyDatabaseDMLOptionsParameters(t *testing.T) {
	symbols := StandardPlatformSymbols()
	database := requireStandardSymbol(t, symbols, "Database")

	for _, member := range database.Members {
		if member.Kind != apexast.DeclarationMethod {
			continue
		}
		for _, param := range member.Parameters {
			if param.Type == "DMLOptions" {
				t.Fatalf("Database.%s has bare DMLOptions parameter; use Database.DMLOptions", member.Name)
			}
		}
	}
	requireStandardMethod(t, database, "insert", []string{"Object", "Database.DMLOptions"}, true)
}

func TestStandardPlatformSymbolsIncludeBaseExceptionConstructors(t *testing.T) {
	symbols := StandardPlatformSymbols()

	exception := requireStandardSymbol(t, symbols, "Exception")
	requireStandardConstructor(t, exception, []string{})
	requireStandardConstructor(t, exception, []string{"Exception"})
	requireStandardConstructor(t, exception, []string{"String"})
	requireStandardConstructor(t, exception, []string{"String", "Exception"})
}

func TestStandardPlatformSymbolsIncludeLabelLimitsDecimalAndTargetExceptionShapes(t *testing.T) {
	symbols := StandardPlatformSymbols()

	decimal := requireStandardSymbol(t, symbols, "Decimal")
	requireStandardMethod(t, decimal, "divide", []string{"Decimal", "Integer", "RoundingMode"}, false)

	label := requireStandardSymbol(t, symbols, "Label")
	requireStandardMethod(t, label, "get", []string{"String", "String"}, true)
	requireStandardMethod(t, label, "get", []string{"String", "String", "String"}, true)
	requireStandardMethod(t, label, "translationExists", []string{"String", "String", "String"}, true)

	systemLabel := requireStandardSymbol(t, symbols, "System.Label")
	requireStandardMethod(t, systemLabel, "get", []string{"String", "String"}, true)
	requireStandardMethod(t, systemLabel, "get", []string{"String", "String", "String"}, true)
	requireStandardMethod(t, systemLabel, "translationExists", []string{"String", "String", "String"}, true)

	limits := requireStandardSymbol(t, symbols, "Limits")
	requireStandardMethodType(t, limits, "getAsyncCalls", "Integer")
	requireStandardMethodType(t, limits, "getLimitAsyncCalls", "Integer")

	invalidParameter := requireStandardSymbol(t, symbols, "InvalidParameterValueException")
	requireStandardConstructor(t, invalidParameter, []string{})
	requireStandardConstructor(t, invalidParameter, []string{"Exception"})
	requireStandardConstructor(t, invalidParameter, []string{"String"})

	for _, name := range []string{"NoAccessException", "NoDataFoundException", "NullPointerException"} {
		exceptionType := requireStandardSymbol(t, symbols, name)
		requireStandardConstructor(t, exceptionType, []string{"Exception"})
		requireStandardConstructor(t, exceptionType, []string{"String"})
		requireStandardConstructor(t, exceptionType, []string{"String", "Exception"})
	}

	touchHandled := requireStandardSymbol(t, symbols, "TouchHandledException")
	requireStandardConstructor(t, touchHandled, []string{})
	requireStandardConstructor(t, touchHandled, []string{"String", "Exception"})
}

func TestStandardPlatformSymbolsIncludeServiceBackedSystemAndStdlibShapes(t *testing.T) {
	symbols := StandardPlatformSymbols()

	requireStandardSymbol(t, symbols, "Canvas")
	requireStandardSymbol(t, symbols, "DataSource")

	asyncSaveCallback := requireStandardSymbol(t, symbols, "DataSource.AsyncSaveCallback")
	if asyncSaveCallback.Kind != apexast.DeclarationInterface {
		t.Fatalf("DataSource.AsyncSaveCallback kind = %q, want interface", asyncSaveCallback.Kind)
	}
	requireStandardMethod(t, asyncSaveCallback, "processSave", []string{"Database.SaveResult"}, false)

	asyncDeleteCallback := requireStandardSymbol(t, symbols, "DataSource.AsyncDeleteCallback")
	if asyncDeleteCallback.Kind != apexast.DeclarationInterface {
		t.Fatalf("DataSource.AsyncDeleteCallback kind = %q, want interface", asyncDeleteCallback.Kind)
	}
	requireStandardMethod(t, asyncDeleteCallback, "processDelete", []string{"Database.DeleteResult"}, false)

	applicationContext := requireStandardSymbol(t, symbols, "Canvas.ApplicationContext")
	requireStandardMethod(t, applicationContext, "getCanvasUrl", nil, false)
	requireStandardMethod(t, applicationContext, "getDeveloperName", nil, false)
	requireStandardMethod(t, applicationContext, "getName", nil, false)
	requireStandardMethod(t, applicationContext, "getNamespace", nil, false)
	requireStandardMethod(t, applicationContext, "getVersion", nil, false)
	requireStandardMethod(t, applicationContext, "setCanvasUrlPath", []string{"String"}, false)

	environmentContext := requireStandardSymbol(t, symbols, "Canvas.EnvironmentContext")
	requireStandardMethod(t, environmentContext, "addEntityField", []string{"String"}, false)
	requireStandardMethod(t, environmentContext, "addEntityFields", []string{"Set<String>"}, false)
	requireStandardMethod(t, environmentContext, "getDisplayLocation", nil, false)
	requireStandardMethod(t, environmentContext, "getEntityFields", nil, false)
	requireStandardMethod(t, environmentContext, "getLocationUrl", nil, false)
	requireStandardMethod(t, environmentContext, "getParametersAsJSON", nil, false)
	requireStandardMethod(t, environmentContext, "getSublocation", nil, false)
	requireStandardMethod(t, environmentContext, "setParametersAsJSON", []string{"String"}, false)

	renderContext := requireStandardSymbol(t, symbols, "Canvas.RenderContext")
	requireStandardMethod(t, renderContext, "getApplicationContext", nil, false)
	requireStandardMethod(t, renderContext, "getEnvironmentContext", nil, false)

	canvasLifecycleHandler := requireStandardSymbol(t, symbols, "Canvas.CanvasLifecycleHandler")
	requireStandardMethod(t, canvasLifecycleHandler, "excludeContextTypes", nil, false)
	requireStandardMethod(t, canvasLifecycleHandler, "onRender", []string{"Canvas.RenderContext"}, false)

	nlpResponse := requireStandardSymbol(t, symbols, "industriesNlpSvc.NlpResponse")
	requireStandardProperty(t, nlpResponse, "summarizationResult", "industriesNlpSvc.NlpSummarizationResult")
	requireStandardProperty(t, nlpResponse, "errors", "List<String>")

	nlpSummarizationResult := requireStandardSymbol(t, symbols, "industriesNlpSvc.NlpSummarizationResult")
	requireStandardProperty(t, nlpSummarizationResult, "summary", "String")

	eventBus := requireStandardSymbol(t, symbols, "EventBus")
	requireStandardMethod(t, eventBus, "publishWithAccessLevel", []string{"SObject", "AccessLevel"}, true)
	requireStandardMethod(t, eventBus, "publishWithAccessLevel", []string{"SObject", "Object", "AccessLevel"}, true)
	requireStandardMethod(t, eventBus, "publishWithAccessLevel", []string{"List<SObject>", "Object", "AccessLevel"}, true)

	pushPayload := requireStandardSymbol(t, symbols, "Messaging.PushNotificationPayload")
	requireStandardMethod(t, pushPayload, "apple", []string{"String", "String", "Integer", "Map<String,Object>"}, true)
	requireStandardMethod(t, pushPayload, "apple", []string{"String", "String", "String", "List<String>", "String", "String", "Integer", "Map<String,Object>"}, true)

	push := requireStandardSymbol(t, symbols, "Messaging.PushNotification")
	requireStandardConstructor(t, push, []string{})
	requireStandardConstructor(t, push, []string{"Map<String,Object>"})
	requireStandardMethod(t, push, "send", []string{"String", "Set<String>"}, false)
	requireStandardMethod(t, push, "setPayload", []string{"Map<String,Object>"}, false)
	requireStandardMethod(t, push, "setTtl", []string{"Integer"}, false)
}

func TestStandardPlatformSymbolsIncludeSearchQuery(t *testing.T) {
	symbols := StandardPlatformSymbols()

	search := requireStandardSymbol(t, symbols, "Search")
	requireStandardMethod(t, search, "query", []string{"String"}, true)
	requireStandardMethod(t, search, "query", []string{"String", "AccessLevel"}, true)
	requireStandardMethod(t, search, "find", []string{"String", "AccessLevel"}, true)
	requireStandardMethod(t, search, "suggest", []string{"String", "String", "Search.SuggestionOption"}, true)
	requireStandardMethod(t, search, "suggest", []string{"String", "String", "Search.SuggestionOption", "AccessLevel"}, true)
	requireStandardMethodType(t, search, "query", "List<List<SObject>>")

	date := requireStandardSymbol(t, symbols, "Date")
	requireStandardMethod(t, date, "daysInMonth", []string{"Integer", "Integer"}, true)
}

func TestStandardPlatformSymbolsIncludeDataSourceCompileShapes(t *testing.T) {
	symbols := StandardPlatformSymbols()

	for _, name := range []string{
		"DataSource.AuthenticationCapability",
		"DataSource.AuthenticationProtocol",
		"DataSource.Capability",
		"DataSource.DataType",
		"DataSource.FilterType",
		"DataSource.IdentityType",
		"DataSource.OrderDirection",
		"DataSource.QueryAggregation",
	} {
		enum := requireStandardSymbol(t, symbols, name)
		if enum.Kind != apexast.DeclarationEnum {
			t.Fatalf("%s kind = %q, want enum", name, enum.Kind)
		}
	}

	requireStandardProperty(t, requireStandardSymbol(t, symbols, "DataSource.DataType"), "STRING_SHORT_TYPE", "DataSource.DataType")
	requireStandardProperty(t, requireStandardSymbol(t, symbols, "DataSource.OrderDirection"), "ASCENDING", "DataSource.OrderDirection")

	column := requireStandardSymbol(t, symbols, "DataSource.Column")
	if column.SuperClass != "DataSource.DataSourceUtil" {
		t.Fatalf("DataSource.Column superclass = %q, want DataSource.DataSourceUtil", column.SuperClass)
	}
	requireStandardMethod(t, column, "text", []string{"String", "String", "Integer"}, true)
	requireStandardMethod(t, column, "picklist", []string{"String", "List<Map<String,String>>", "Boolean", "Boolean"}, true)
	requireStandardProperty(t, column, "type", "DataSource.DataType")
	requireStandardProperty(t, column, "picklistValues", "List<Map<String,String>>")

	table := requireStandardSymbol(t, symbols, "DataSource.Table")
	requireStandardMethod(t, table, "get", []string{"String", "String", "List<DataSource.Column>"}, true)
	requireStandardProperty(t, table, "columns", "List<DataSource.Column>")

	tableResult := requireStandardSymbol(t, symbols, "DataSource.TableResult")
	requireStandardMethod(t, tableResult, "get", []string{"DataSource.QueryContext", "List<Map<String,Object>>"}, true)
	requireStandardProperty(t, tableResult, "rows", "List<Map<String,Object>>")

	params := requireStandardSymbol(t, symbols, "DataSource.ConnectionParams")
	requireStandardProperty(t, params, "protocol", "DataSource.AuthenticationProtocol")
	requireStandardProperty(t, params, "principalType", "DataSource.IdentityType")

	filter := requireStandardSymbol(t, symbols, "DataSource.Filter")
	requireStandardProperty(t, filter, "type", "DataSource.FilterType")
	requireStandardProperty(t, filter, "subfilters", "List<DataSource.Filter>")

	order := requireStandardSymbol(t, symbols, "DataSource.Order")
	requireStandardMethod(t, order, "get", []string{"String", "String", "DataSource.OrderDirection"}, true)
	requireStandardProperty(t, order, "direction", "DataSource.OrderDirection")

	queryContext := requireStandardSymbol(t, symbols, "DataSource.QueryContext")
	requireStandardMethod(t, queryContext, "get", []string{"List<DataSource.Table>", "Integer", "Integer", "DataSource.TableSelection"}, true)
	requireStandardProperty(t, queryContext, "tableSelection", "DataSource.TableSelection")

	searchContext := requireStandardSymbol(t, symbols, "DataSource.SearchContext")
	requireStandardConstructor(t, searchContext, []string{})
	requireStandardConstructor(t, searchContext, []string{"List<DataSource.Table>", "Integer", "Integer", "List<DataSource.TableSelection>", "String"})
	requireStandardProperty(t, searchContext, "tableSelections", "List<DataSource.TableSelection>")

	deleteResult := requireStandardSymbol(t, symbols, "DataSource.DeleteResult")
	requireStandardMethod(t, deleteResult, "success", []string{"String"}, true)
	requireStandardMethod(t, deleteResult, "failure", []string{"String", "String"}, true)
	requireStandardProperty(t, deleteResult, "success", "Boolean")

	upsertResult := requireStandardSymbol(t, symbols, "DataSource.UpsertResult")
	requireStandardMethod(t, upsertResult, "success", []string{"String"}, true)
	requireStandardMethod(t, upsertResult, "failure", []string{"String", "String"}, true)
	requireStandardProperty(t, upsertResult, "success", "Boolean")

	provider := requireStandardSymbol(t, symbols, "DataSource.Provider")
	if provider.SuperClass != "DataSource.DataSourceUtil" {
		t.Fatalf("DataSource.Provider superclass = %q, want DataSource.DataSourceUtil", provider.SuperClass)
	}
	requireStandardMethodType(t, provider, "getCapabilities", "List<DataSource.Capability>")
	requireStandardMethodType(t, provider, "getConnection", "DataSource.Connection")

	connection := requireStandardSymbol(t, symbols, "DataSource.Connection")
	if connection.SuperClass != "DataSource.DataSourceUtil" {
		t.Fatalf("DataSource.Connection superclass = %q, want DataSource.DataSourceUtil", connection.SuperClass)
	}
	requireStandardMethodType(t, connection, "sync", "List<DataSource.Table>")
	requireStandardMethodType(t, connection, "query", "DataSource.TableResult")
	requireStandardMethodType(t, connection, "search", "List<DataSource.TableResult>")

	dataSourceUtil := requireStandardSymbol(t, symbols, "DataSource.DataSourceUtil")
	requireStandardMethod(t, dataSourceUtil, "logWarning", []string{"String"}, false)
	requireStandardMethod(t, dataSourceUtil, "throwException", []string{"String"}, false)

	database := requireStandardSymbol(t, symbols, "Database")
	for _, method := range []string{"insertAsync", "updateAsync"} {
		requireStandardMethod(t, database, method, []string{"Object", "AccessLevel"}, true)
		requireStandardMethod(t, database, method, []string{"List<Object>", "AccessLevel"}, true)
		requireStandardMethod(t, database, method, []string{"Object", "Database.AllowCallouts", "AccessLevel"}, true)
		requireStandardMethod(t, database, method, []string{"List<Object>", "Database.AllowCallouts", "AccessLevel"}, true)
	}
	requireStandardMethod(t, database, "deleteAsync", []string{"Object", "AccessLevel"}, true)
	requireStandardMethod(t, database, "deleteAsync", []string{"List<Object>", "AccessLevel"}, true)
	requireStandardMethod(t, database, "deleteAsync", []string{"Object", "Database.AllowCallouts", "AccessLevel"}, true)
	requireStandardMethod(t, database, "deleteAsync", []string{"List<Object>", "Database.AllowCallouts", "AccessLevel"}, true)
}

func TestStandardPlatformSymbolsIncludeDatabaseAccessLevelAliasShapes(t *testing.T) {
	symbols := StandardPlatformSymbols()
	database := requireStandardSymbol(t, symbols, "Database")

	requireStandardMethod(t, database, "queryWithBinds", []string{"String", "Map", "AccessLevel"}, true)
	requireStandardMethod(t, database, "countQueryWithBinds", []string{"String", "Map", "AccessLevel"}, true)
	requireStandardMethod(t, database, "getCursor", []string{"String", "AccessLevel"}, true)
	requireStandardMethod(t, database, "getCursorWithBinds", []string{"String", "Map", "AccessLevel"}, true)
	requireStandardMethod(t, database, "getPaginationCursor", []string{"String", "AccessLevel"}, true)
	requireStandardMethod(t, database, "getPaginationCursorWithBinds", []string{"String", "Map", "AccessLevel"}, true)
}

func TestStandardPlatformSymbolsIncludeMessagingPageReferenceAndSObjectOptionRows(t *testing.T) {
	symbols := StandardPlatformSymbols()

	messaging := requireStandardSymbol(t, symbols, "Messaging")
	requireStandardMethod(t, messaging, "sendEmail", []string{"List<Messaging.Email>", "Boolean"}, true)
	requireStandardMethod(t, messaging, "renderStoredEmailTemplate", []string{"String", "String", "String", "Messaging.AttachmentRetrievalOption"}, true)
	requireStandardMethod(t, messaging, "renderStoredEmailTemplate", []string{"String", "String", "String", "Messaging.AttachmentRetrievalOption", "Boolean"}, true)

	attachmentOption := requireStandardSymbol(t, symbols, "Messaging.AttachmentRetrievalOption")
	if attachmentOption.Kind != apexast.DeclarationEnum {
		t.Fatalf("Messaging.AttachmentRetrievalOption kind = %q, want enum", attachmentOption.Kind)
	}
	requireStandardPropertyStatic(t, attachmentOption, "METADATA_ONLY", "Messaging.AttachmentRetrievalOption", true)
	requireStandardPropertyStatic(t, attachmentOption, "METADATA_WITH_BODY", "Messaging.AttachmentRetrievalOption", true)
	requireStandardPropertyStatic(t, attachmentOption, "NONE", "Messaging.AttachmentRetrievalOption", true)

	page := requireStandardSymbol(t, symbols, "PageReference")
	requireStandardConstructor(t, page, []string{"String"})
	requireStandardConstructor(t, page, []string{"SObject"})
	requireStandardMethod(t, page, "setCookies", []string{"List<Cookie>"}, false)

	sobject := requireStandardSymbol(t, symbols, "SObject")
	requireStandardMethod(t, sobject, "setOptions", []string{"Database.DMLOptions"}, false)

	httpRequest := requireStandardSymbol(t, symbols, "HttpRequest")
	requireStandardMethod(t, httpRequest, "setBodyDocument", []string{"Dom.Document"}, false)

	testClass := requireStandardSymbol(t, symbols, "Test")
	requireStandardMethod(t, testClass, "setFixedSearchResults", []string{"List<Id>"}, true)
}

func TestStandardPlatformSymbolsTypeAuthProviders(t *testing.T) {
	symbols := StandardPlatformSymbols()

	authConfiguration := requireStandardSymbol(t, symbols, "Auth.AuthConfiguration")
	requireStandardMethodType(t, authConfiguration, "getAuthProviders", "List<AuthProvider>")
	requireNoStandardSymbol(t, symbols, "Auth.Auth")
	requireNoStandardSymbol(t, symbols, "Approval.Approval")
}

func TestStandardPlatformSymbolsIncludeUserInfoStubMethodsAndFieldTokenProperties(t *testing.T) {
	symbols := StandardPlatformSymbols()

	userInfo := requireStandardSymbol(t, symbols, "UserInfo")
	requireStandardMethod(t, userInfo, "getOrganizationName", nil, true)
	requireStandardMethod(t, userInfo, "isCurrentUserLicensed", []string{"String"}, true)

	limits := requireStandardSymbol(t, symbols, "Limits")
	requireStandardMethodType(t, limits, "getBatchJobs", "Integer")
	requireStandardMethodType(t, limits, "getLimitBatchJobs", "Integer")

	field := requireStandardSymbol(t, symbols, "Schema.SObjectField")
	requireStandardProperty(t, field, "label", "String")
}

func TestStandardPlatformSymbolsIncludeSchemaDescribeShape(t *testing.T) {
	symbols := StandardPlatformSymbols()

	schema := requireStandardSymbol(t, symbols, "Schema")
	requireStandardMethodType(t, schema, "describeTabs", "List<Schema.DescribeTabSetResult>")

	describe := requireStandardSymbol(t, symbols, "Schema.DescribeSObjectResult")
	requireStandardProperty(t, describe, "fieldSets", "Schema.FieldSetMap")
	requireStandardMethodType(t, describe, "getFieldSets", "Schema.FieldSetMap")

	fieldSetMap := requireStandardSymbol(t, symbols, "Schema.FieldSetMap")
	requireStandardMethodType(t, fieldSetMap, "getMap", "Map<String,Schema.FieldSet>")
	requireStandardMethod(t, fieldSetMap, "get", []string{"String"}, false)

	fieldSet := requireStandardSymbol(t, symbols, "Schema.FieldSet")
	requireStandardMethodType(t, fieldSet, "getName", "String")
	requireStandardMethodType(t, fieldSet, "getNameSpace", "String")
	requireStandardMethodType(t, fieldSet, "getSObjectType", "Schema.SObjectType")

	fieldSetMember := requireStandardSymbol(t, symbols, "Schema.FieldSetMember")
	requireStandardMethodType(t, fieldSetMember, "getSObjectField", "Schema.SObjectField")
}

func TestStandardPlatformSymbolsIncludeGeneratedSystemStubBreadth(t *testing.T) {
	symbols := StandardPlatformSymbols()

	testClass := requireStandardSymbol(t, symbols, "Test")
	requireStandardMethod(t, testClass, "createStubQueryRow", []string{"Schema.SObjectType", "Map<String,Object>"}, true)
	requireStandardMethod(t, testClass, "setCurrentPageReference", []string{"Object"}, true)

	trigger := requireStandardSymbol(t, symbols, "Trigger")
	for _, property := range []string{"isExecuting", "isInsert", "isUpdate", "isDelete", "isBefore", "isAfter", "isUndelete"} {
		requireStandardPropertyStatic(t, trigger, property, "Boolean", true)
	}
	requireStandardPropertyStatic(t, trigger, "new", "List<SObject>", true)
	requireStandardPropertyStatic(t, trigger, "old", "List<SObject>", true)
	requireStandardPropertyStatic(t, trigger, "newMap", "Map<Id,SObject>", true)
	requireStandardPropertyStatic(t, trigger, "oldMap", "Map<Id,SObject>", true)
	requireStandardPropertyStatic(t, trigger, "operationType", "TriggerOperation", true)
	requireStandardPropertyStatic(t, trigger, "size", "Integer", true)

	stringClass := requireStandardSymbol(t, symbols, "String")
	requireStandardMethod(t, stringClass, "equals", []string{"String"}, false)
	requireStandardMethod(t, stringClass, "format", []string{"String", "List<Object>"}, true)
	requireStandardMethod(t, stringClass, "template", []string{"Map<String,Object>"}, false)

	systemClass := requireStandardSymbol(t, symbols, "System")
	requireStandardMethod(t, systemClass, "debug", []string{"LoggingLevel", "Object"}, true)

	installHandler := requireStandardSymbol(t, symbols, "InstallHandler")
	requireStandardMethod(t, installHandler, "onInstall", []string{"InstallContext"}, false)

	uninstallHandler := requireStandardSymbol(t, symbols, "UninstallHandler")
	requireStandardMethod(t, uninstallHandler, "onUninstall", []string{"UninstallContext"}, false)

	integrationTest := requireStandardSymbol(t, symbols, "IntegrationTest")
	requireStandardMethod(t, integrationTest, "commitTestOnly", nil, true)

	displayType := requireStandardSymbol(t, symbols, "Schema.DisplayType")
	if displayType.Kind != apexast.DeclarationEnum {
		t.Fatalf("Schema.DisplayType kind = %q, want enum", displayType.Kind)
	}
	requireStandardProperty(t, displayType, "ANYTYPE", "Schema.DisplayType")
	requireStandardProperty(t, displayType, "LOCATION", "Schema.DisplayType")

	saveResult := requireStandardSymbol(t, symbols, "Database.SaveResult")
	requireStandardMethod(t, saveResult, "getErrors", nil, false)
	requireStandardMethod(t, saveResult, "isSuccess", nil, false)
	requireStandardProperty(t, saveResult, "errors", "List<Database.Error>")
	requireStandardProperty(t, saveResult, "id", "Id")
	requireStandardProperty(t, saveResult, "success", "Boolean")

	databaseError := requireStandardSymbol(t, symbols, "Database.Error")
	requireStandardMethodType(t, databaseError, "getExtendedErrorDetails", "List<Object>")

	database := requireStandardSymbol(t, symbols, "Database")
	requireStandardMethod(t, database, "insert", []string{"SObject", "Object"}, true)
	requireStandardMethod(t, database, "update", []string{"List<SObject>", "Object"}, true)
	requireStandardMethod(t, database, "countQuery", []string{"String", "System.AccessLevel"}, true)
	requireStandardMethod(t, database, "countQueryWithBinds", []string{"String", "Map<String,Object>", "System.AccessLevel"}, true)
	requireStandardMethod(t, database, "getQueryLocator", []string{"String", "System.AccessLevel"}, true)
	requireStandardMethod(t, database, "getQueryLocatorWithBinds", []string{"String", "Map<String,Object>", "System.AccessLevel"}, true)
	requireStandardMethod(t, database, "queryWithBinds", []string{"String", "Map<String,Object>", "System.AccessLevel"}, true)
	requireStandardMethodType(t, database, "countQueryWithBinds", "Integer")
	requireStandardMethodType(t, database, "getQueryLocatorWithBinds", "Database.QueryLocator")

	batchable := requireStandardSymbol(t, symbols, "Database.Batchable")
	if batchable.Kind != apexast.DeclarationInterface {
		t.Fatalf("Database.Batchable kind = %q, want interface", batchable.Kind)
	}
	if batchable.SuperClass != "" {
		t.Fatalf("Database.Batchable superclass = %q, want empty interface superclass", batchable.SuperClass)
	}
	for _, member := range batchable.Members {
		if member.Kind == apexast.DeclarationConstructor {
			t.Fatalf("Database.Batchable has generated-stub constructor %q, want no interface constructors", member.Name)
		}
	}
	requireStandardMethodType(t, batchable, "start", "Iterable")

	for _, name := range []string{"Database.Stateful", "HttpCalloutMock", "Iterable", "Iterator", "Queueable", "Schedulable"} {
		symbol := requireStandardSymbol(t, symbols, name)
		if symbol.Kind != apexast.DeclarationInterface {
			t.Fatalf("%s kind = %q, want interface", name, symbol.Kind)
		}
		if symbol.SuperClass != "" {
			t.Fatalf("%s superclass = %q, want empty interface superclass", name, symbol.SuperClass)
		}
		for _, member := range symbol.Members {
			if member.Kind == apexast.DeclarationConstructor {
				t.Fatalf("%s has generated-stub constructor %q, want no interface constructors", name, member.Name)
			}
		}
	}

	statusCode := requireStandardSymbol(t, symbols, "StatusCode")
	if statusCode.Kind != apexast.DeclarationEnum {
		t.Fatalf("StatusCode kind = %q, want enum", statusCode.Kind)
	}
	requireStandardProperty(t, statusCode, "APEX_FAILED", "StatusCode")

	typeClass := requireStandardSymbol(t, symbols, "Type")
	requireStandardMethod(t, typeClass, "isAssignableFrom", []string{"Type"}, false)

	canvasTest := requireStandardSymbol(t, symbols, "Canvas.Test")
	requireStandardPropertyStatic(t, canvasTest, "KEY_CANVAS_URL", "Object", true)

	batchResult := requireStandardSymbol(t, symbols, "ConnectApi.BatchResult")
	requireStandardProperty(t, batchResult, "isSuccess", "Boolean")
	externalCredential := requireStandardSymbol(t, symbols, "ConnectApi.ExternalCredential")
	requireStandardProperty(t, externalCredential, "principals", "List<ConnectApi.ExternalCredentialPrincipal>")
	externalCredentialInput := requireStandardSymbol(t, symbols, "ConnectApi.ExternalCredentialInput")
	requireStandardProperty(t, externalCredentialInput, "principals", "List<ConnectApi.ExternalCredentialPrincipalInput>")
	inboundEmail := requireStandardSymbol(t, symbols, "Messaging.InboundEmail")
	requireStandardProperty(t, inboundEmail, "authenticationResults", "List<Messaging.InboundEmail.AuthenticationResult>")
	requireStandardProperty(t, inboundEmail, "binaryAttachments", "List<Messaging.InboundEmail.BinaryAttachment>")
	requireStandardProperty(t, inboundEmail, "fromName", "String")
	inboundEmailResult := requireStandardSymbol(t, symbols, "Messaging.InboundEmailResult")
	requireStandardProperty(t, inboundEmailResult, "success", "Boolean")
	requireStandardProperty(t, inboundEmailResult, "message", "String")

	cursorDeleteFilter := requireStandardSymbol(t, symbols, "Database.Cursor.DeleteFilter")
	requireStandardPropertyStatic(t, cursorDeleteFilter, "NO_FILTER", "Database.Cursor.DeleteFilter", true)
	paginationDeleteFilter := requireStandardSymbol(t, symbols, "Database.PaginationCursor.DeleteFilter")
	requireStandardPropertyStatic(t, paginationDeleteFilter, "NO_FILTER", "Database.PaginationCursor.DeleteFilter", true)
	accountEngagementType := requireStandardSymbol(t, symbols, "sfdatakit.DeployComponentBundleAccountEngagementConfig.AccountEngagmentDataStreamTypeEnum")
	if accountEngagementType.Kind != apexast.DeclarationEnum {
		t.Fatalf("sfdatakit account engagement enum kind = %q, want enum", accountEngagementType.Kind)
	}
	requireStandardPropertyStatic(t, accountEngagementType, "EmailActivity", "sfdatakit.DeployComponentBundleAccountEngagementConfig.AccountEngagmentDataStreamTypeEnum", true)
}

func TestStandardPlatformSymbolsIncludeCoreRuntimeCollectionObjectShapes(t *testing.T) {
	symbols := StandardPlatformSymbols()

	objectClass := requireStandardSymbol(t, symbols, "Object")
	requireStandardMethod(t, objectClass, "equals", []string{"Object"}, false)
	requireStandardMethodType(t, objectClass, "equals", "Boolean")
	requireStandardMethod(t, objectClass, "hashCode", nil, false)
	requireStandardMethodType(t, objectClass, "hashCode", "Integer")
	requireStandardMethod(t, objectClass, "toString", nil, false)
	requireStandardMethodType(t, objectClass, "toString", "String")

	enumClass := requireStandardSymbol(t, symbols, "Enum")
	if enumClass.Kind != apexast.DeclarationClass {
		t.Fatalf("Enum kind = %q, want class", enumClass.Kind)
	}

	comparator := requireStandardSymbol(t, symbols, "Comparator")
	if comparator.Kind != apexast.DeclarationInterface {
		t.Fatalf("Comparator kind = %q, want interface", comparator.Kind)
	}
	requireStandardMethod(t, comparator, "compare", []string{"Object", "Object"}, false)

	listClass := requireStandardSymbol(t, symbols, "List")
	requireStandardConstructor(t, listClass, nil)
	requireStandardConstructor(t, listClass, []string{"List"})
	requireStandardMethod(t, listClass, "equals", []string{"List"}, false)

	mapClass := requireStandardSymbol(t, symbols, "Map")
	requireStandardConstructor(t, mapClass, nil)
	requireStandardConstructor(t, mapClass, []string{"Map"})
	requireStandardConstructor(t, mapClass, []string{"List<SObject>"})
	requireStandardMethod(t, mapClass, "equals", []string{"Map"}, false)
	requireStandardMethod(t, mapClass, "remove", []string{"Object"}, false)

	setClass := requireStandardSymbol(t, symbols, "Set")
	requireStandardConstructor(t, setClass, nil)
	requireStandardConstructor(t, setClass, []string{"Set"})
	requireStandardMethod(t, setClass, "addAll", []string{"List<Object>"}, false)
	requireStandardMethod(t, setClass, "containsAll", []string{"List<Object>"}, false)
	requireStandardMethod(t, setClass, "equals", []string{"Set<Object>"}, false)
	requireStandardMethod(t, setClass, "removeAll", []string{"List<Object>"}, false)
	requireStandardMethod(t, setClass, "retainAll", []string{"List<Object>"}, false)
}

func TestStandardSymbolsFromSpecsKeepsRicherGeneratedPropertyTypes(t *testing.T) {
	symbols := StandardSymbolsFromSpecs([]StandardSymbolSpec{{
		Name: "Database.SaveResult",
		Properties: []StandardPropertySpec{{
			Name: "errors",
			Type: "Object",
		}},
	}, {
		Name: "database.saveresult",
		Properties: []StandardPropertySpec{{
			Name: "errors",
			Type: "List<Database.Error>",
		}},
	}})

	saveResult := requireStandardSymbol(t, symbols, "Database.SaveResult")
	requireStandardProperty(t, saveResult, "errors", "List<Database.Error>")
}

func TestStandardPlatformSymbolsIncludeGeneratedProductNamespaceStubBreadth(t *testing.T) {
	symbols := StandardPlatformSymbols()

	org := requireStandardSymbol(t, symbols, "ConnectApi.Organization")
	requireStandardMethod(t, org, "getSettings", nil, true)

	commerce := requireStandardSymbol(t, symbols, "commercepromotions.PromotionRequest")
	requireStandardProperty(t, commerce, "buyerAccountId", "String")
	requireStandardConstructorParams(t, commerce,
		[]string{"SObject", "String", "String", "List<String>"},
		[]string{"salesTransaction", "buyerAccountId", "webStoreId", "couponCodes"},
	)

	providerSummary := requireStandardSymbol(t, symbols, "AiCopilot.ProviderSummarySchema")
	requireStandardProperty(t, providerSummary, "keyInfo", "List<AiCopilot.PrvdSumSectionInfo>")
	requireStandardProperty(t, providerSummary, "changeInfo", "List<AiCopilot.PrvdSumSectionInfo>")

	action := requireStandardSymbol(t, symbols, "Invocable.Action")
	requireStandardMethodParams(t, action, "createCustomAction",
		[]string{"String", "String", "String"},
		[]string{"type", "namespace", "name"},
		true,
	)

	audience := requireStandardSymbol(t, symbols, "ConnectApi.AudienceCriteriaType")
	requireStandardPropertyStatic(t, audience, "Audience", "ConnectApi.AudienceCriteriaType", true)

	couponError := requireStandardSymbol(t, symbols, "commercepromotions.CouponInfo.ErrorCode")
	requireStandardPropertyStatic(t, couponError, "INVALIDCOUPON", "commercepromotions.CouponInfo.ErrorCode", true)

	slackBuilder := requireStandardSymbol(t, symbols, "Slack.ApiTestRequest.Builder")
	requireStandardMethodType(t, slackBuilder, "build", "Slack.ApiTestRequest")
	requireStandardMethodParams(t, slackBuilder, "foo", []string{"String"}, []string{"foo"}, false)
}

func TestStandardPlatformSymbolsResolveInvocableActionDTOReferences(t *testing.T) {
	symbols := StandardPlatformSymbols()
	for _, name := range []string{
		"Invocable.Action.AdditionalAttribute",
		"Invocable.Action.GenericType",
		"Invocable.Action.OutputParameter",
		"Invocable.Action.PicklistValue",
	} {
		requireStandardSymbol(t, symbols, name)
	}

	describe := requireStandardSymbol(t, symbols, "Invocable.Action.DescribeResult")
	requireStandardMethodType(t, describe, "getGenericTypes", "List<Invocable.Action.GenericType>")
	requireStandardMethodType(t, describe, "getOutputs", "List<Invocable.Action.OutputParameter>")

	input := requireStandardSymbol(t, symbols, "Invocable.Action.InputParameter")
	requireStandardMethodType(t, input, "getAdditionalAttributes", "List<Invocable.Action.AdditionalAttribute>")
	requireStandardMethodType(t, input, "getPicklistValues", "List<Invocable.Action.PicklistValue>")
}

func TestStandardPlatformSymbolsUseQualifiedEmailAttachmentType(t *testing.T) {
	symbols := StandardPlatformSymbols()
	message := requireStandardSymbol(t, symbols, "Messaging.SingleEmailMessage")
	requireStandardMethod(t, message, "setFileAttachments", []string{"List<Messaging.EmailFileAttachment>"}, false)
	for _, member := range message.Members {
		if !strings.EqualFold(member.Name, "setFileAttachments") {
			continue
		}
		for _, param := range member.Parameters {
			if strings.EqualFold(param.Type, "List<EmailFileAttachment>") {
				t.Fatalf("setFileAttachments has unqualified attachment parameter: %#v", member.Parameters)
			}
		}
	}
}

func TestGeneratedStubSpecsCaseInsensitiveBreadthAudit(t *testing.T) {
	auditGeneratedStubSpecs(t, "system", systemStubSymbolSpecs)
	auditGeneratedStubSpecs(t, "product namespace", productNamespaceSymbolSpecs)
}

func auditGeneratedStubSpecs(t *testing.T, label string, specs []StandardSymbolSpec) {
	t.Helper()
	byName := make(map[string]StandardSymbolSpec, len(specs))
	for _, spec := range specs {
		if spec.Name == "" {
			continue
		}
		key := strings.ToLower(spec.Name)
		if existing, ok := byName[key]; ok {
			t.Fatalf("%s generated specs contain duplicate type %q and %q differing only by case", label, existing.Name, spec.Name)
		}
		byName[key] = spec
		auditGeneratedStubSpecMembers(t, label, spec)
		if spec.Kind == apexast.DeclarationEnum {
			for _, prop := range spec.Properties {
				if prop.Name == "" {
					continue
				}
				if !prop.Static {
					t.Fatalf("%s enum %s constant %s is not static", label, spec.Name, prop.Name)
				}
				if !strings.EqualFold(prop.Type, spec.Name) {
					t.Fatalf("%s enum %s constant %s type = %q, want %q", label, spec.Name, prop.Name, prop.Type, spec.Name)
				}
			}
		}
	}
	for _, spec := range specs {
		for _, aliasName := range generatedEnumAliasNames(spec) {
			alias, ok := byName[strings.ToLower(aliasName)]
			if !ok {
				t.Fatalf("%s enum %s has missing nested alias %s", label, spec.Name, aliasName)
			}
			if alias.Kind != apexast.DeclarationEnum {
				t.Fatalf("%s nested alias %s kind = %q, want enum", label, aliasName, alias.Kind)
			}
			if len(alias.Properties) == 0 {
				t.Fatalf("%s nested alias %s has no constants", label, aliasName)
			}
			for _, prop := range alias.Properties {
				if !prop.Static || !strings.EqualFold(prop.Type, alias.Name) {
					t.Fatalf("%s nested alias %s constant %s = type %q static=%v, want static %s", label, aliasName, prop.Name, prop.Type, prop.Static, alias.Name)
				}
			}
		}
	}
}

func auditGeneratedStubSpecMembers(t *testing.T, label string, spec StandardSymbolSpec) {
	t.Helper()
	methodCases := make(map[string]string, len(spec.Methods))
	for _, method := range spec.Methods {
		key := strings.ToLower(method.Name)
		if existing, ok := methodCases[key]; ok && existing != method.Name {
			t.Fatalf("%s generated spec %s has methods %q and %q differing only by case", label, spec.Name, existing, method.Name)
		}
		methodCases[key] = method.Name
	}
	propertyCases := make(map[string]string, len(spec.Properties))
	for _, prop := range spec.Properties {
		key := strings.ToLower(prop.Name)
		if existing, ok := propertyCases[key]; ok && existing != prop.Name {
			t.Fatalf("%s generated spec %s has properties %q and %q differing only by case", label, spec.Name, existing, prop.Name)
		}
		propertyCases[key] = prop.Name
	}
}

func generatedEnumAliasNames(spec StandardSymbolSpec) []string {
	if !hasGeneratedEnumConstants(spec) {
		return nil
	}
	out := []string{}
	for _, method := range spec.Methods {
		if !method.Static || method.Name != "valueOf" || method.ReturnType == "" || strings.EqualFold(method.ReturnType, spec.Name) {
			continue
		}
		if hasGeneratedEnumValuesMethod(spec, method.ReturnType) {
			out = append(out, method.ReturnType)
		}
	}
	return out
}

func hasGeneratedEnumConstants(spec StandardSymbolSpec) bool {
	for _, prop := range spec.Properties {
		if prop.Static && prop.Name != "" {
			return true
		}
	}
	return false
}

func hasGeneratedEnumValuesMethod(spec StandardSymbolSpec, typeName string) bool {
	for _, method := range spec.Methods {
		if method.Static && method.Name == "values" && strings.EqualFold(method.ReturnType, "List<"+typeName+">") {
			return true
		}
	}
	return false
}

func requireStandardSymbol(t *testing.T, symbols []TypeSymbol, name string) TypeSymbol {
	t.Helper()
	for _, symbol := range symbols {
		if strings.EqualFold(standardSymbolFullName(symbol), name) {
			return symbol
		}
	}
	t.Fatalf("missing standard symbol %s", name)
	return TypeSymbol{}
}

func requireNoStandardSymbol(t *testing.T, symbols []TypeSymbol, name string) {
	t.Helper()
	for _, symbol := range symbols {
		if strings.EqualFold(standardSymbolFullName(symbol), name) {
			t.Fatalf("unexpected standard symbol %s", name)
		}
	}
}

func requireStandardConstructor(t *testing.T, symbol TypeSymbol, params []string) {
	t.Helper()
	for _, member := range symbol.Members {
		if member.Kind != apexast.DeclarationConstructor {
			continue
		}
		if standardMemberParamsEqual(member, params) {
			return
		}
	}
	t.Fatalf("missing constructor on %s with params %#v: %#v", standardSymbolFullName(symbol), params, symbol.Members)
}

func requireStandardConstructorParams(t *testing.T, symbol TypeSymbol, types, names []string) {
	t.Helper()
	if len(types) != len(names) {
		t.Fatalf("constructor assertion mismatch: types=%#v names=%#v", types, names)
	}
	for _, member := range symbol.Members {
		if member.Kind != apexast.DeclarationConstructor || !standardMemberParamsEqual(member, types) {
			continue
		}
		for i, name := range names {
			if !strings.EqualFold(member.Parameters[i].Name, name) {
				t.Fatalf("constructor param %d on %s = %q, want %q: %#v", i, standardSymbolFullName(symbol), member.Parameters[i].Name, name, member.Parameters)
			}
		}
		return
	}
	t.Fatalf("missing constructor on %s with params %#v: %#v", standardSymbolFullName(symbol), types, symbol.Members)
}

func requireStandardMethodParams(t *testing.T, symbol TypeSymbol, methodName string, types, names []string, static bool) {
	t.Helper()
	if len(types) != len(names) {
		t.Fatalf("method assertion mismatch: types=%#v names=%#v", types, names)
	}
	for _, member := range symbol.Members {
		if member.Kind != apexast.DeclarationMethod || !strings.EqualFold(member.Name, methodName) || !standardMemberParamsEqual(member, types) {
			continue
		}
		if memberHasModifier(member, "static") != static {
			continue
		}
		for i, name := range names {
			if !strings.EqualFold(member.Parameters[i].Name, name) {
				t.Fatalf("method param %d on %s.%s = %q, want %q: %#v", i, standardSymbolFullName(symbol), methodName, member.Parameters[i].Name, name, member.Parameters)
			}
		}
		return
	}
	t.Fatalf("missing method %s.%s with params %#v static=%v: %#v", standardSymbolFullName(symbol), methodName, types, static, symbol.Members)
}

func requireStandardMethod(t *testing.T, symbol TypeSymbol, name string, params []string, static bool) {
	t.Helper()
	for _, member := range symbol.Members {
		if member.Kind != apexast.DeclarationMethod || !strings.EqualFold(member.Name, name) {
			continue
		}
		if !standardMemberParamsEqual(member, params) {
			continue
		}
		if memberHasModifier(member, "static") != static {
			continue
		}
		return
	}
	t.Fatalf("missing method %s.%s with params %#v static=%v: %#v", standardSymbolFullName(symbol), name, params, static, symbol.Members)
}

func requireStandardProperty(t *testing.T, symbol TypeSymbol, name, typ string) {
	t.Helper()
	for _, member := range symbol.Members {
		if member.Kind == apexast.DeclarationProperty && strings.EqualFold(member.Name, name) && strings.EqualFold(member.Type, typ) {
			return
		}
	}
	t.Fatalf("missing property %s.%s type %s: %#v", standardSymbolFullName(symbol), name, typ, symbol.Members)
}

func requireStandardPropertyStatic(t *testing.T, symbol TypeSymbol, name, typ string, static bool) {
	t.Helper()
	for _, member := range symbol.Members {
		if member.Kind != apexast.DeclarationProperty || !strings.EqualFold(member.Name, name) || !strings.EqualFold(member.Type, typ) {
			continue
		}
		if memberHasModifier(member, "static") == static {
			return
		}
	}
	t.Fatalf("missing property %s.%s type %s static=%v: %#v", standardSymbolFullName(symbol), name, typ, static, symbol.Members)
}

func requireStandardMethodType(t *testing.T, symbol TypeSymbol, name, typ string) {
	t.Helper()
	for _, member := range symbol.Members {
		if member.Kind == apexast.DeclarationMethod && strings.EqualFold(member.Name, name) && strings.EqualFold(member.Type, typ) {
			return
		}
	}
	t.Fatalf("missing method %s.%s type %s: %#v", standardSymbolFullName(symbol), name, typ, symbol.Members)
}

func standardMemberParamsEqual(member MemberSymbol, params []string) bool {
	if len(member.Parameters) != len(params) {
		return false
	}
	for i := range params {
		if !strings.EqualFold(member.Parameters[i].Type, params[i]) {
			return false
		}
	}
	return true
}

func memberHasModifier(member MemberSymbol, modifier string) bool {
	for _, candidate := range member.Modifiers {
		if strings.EqualFold(candidate, modifier) {
			return true
		}
	}
	return false
}

func standardSymbolFullName(symbol TypeSymbol) string {
	if symbol.Namespace == "" {
		return symbol.Name
	}
	return symbol.Namespace + "." + symbol.Name
}
