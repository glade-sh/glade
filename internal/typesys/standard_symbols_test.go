package typesys

import (
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
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

func TestStandardPlatformSymbolsIncludeSearchQuery(t *testing.T) {
	symbols := StandardPlatformSymbols()

	search := requireStandardSymbol(t, symbols, "Search")
	requireStandardMethod(t, search, "query", []string{"String"}, true)
	requireStandardMethodType(t, search, "query", "List<List<SObject>>")

	date := requireStandardSymbol(t, symbols, "Date")
	requireStandardMethod(t, date, "daysInMonth", []string{"Integer", "Integer"}, true)
}

func TestStandardPlatformSymbolsIncludeUserInfoStubMethodsAndFieldTokenProperties(t *testing.T) {
	symbols := StandardPlatformSymbols()

	userInfo := requireStandardSymbol(t, symbols, "UserInfo")
	requireStandardMethod(t, userInfo, "getOrganizationName", nil, true)
	requireStandardMethod(t, userInfo, "isCurrentUserLicensed", []string{"String"}, true)

	field := requireStandardSymbol(t, symbols, "Schema.SObjectField")
	requireStandardProperty(t, field, "label", "String")
}

func TestStandardPlatformSymbolsIncludeGeneratedSystemStubBreadth(t *testing.T) {
	symbols := StandardPlatformSymbols()

	testClass := requireStandardSymbol(t, symbols, "Test")
	requireStandardMethod(t, testClass, "createStubQueryRow", []string{"Schema.SObjectType", "Map<String,Object>"}, true)
	requireStandardMethod(t, testClass, "setCurrentPageReference", []string{"Object"}, true)

	stringClass := requireStandardSymbol(t, symbols, "String")
	requireStandardMethod(t, stringClass, "format", []string{"String", "List<Object>"}, true)

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

	database := requireStandardSymbol(t, symbols, "Database")
	requireStandardMethod(t, database, "insert", []string{"SObject", "Object"}, true)
	requireStandardMethod(t, database, "update", []string{"List<SObject>", "Object"}, true)
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

	for _, name := range []string{"HttpCalloutMock", "Iterable", "Iterator", "Queueable", "Schedulable"} {
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
