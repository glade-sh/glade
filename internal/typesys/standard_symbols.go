package typesys

import (
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
)

type StandardSymbolSpec struct {
	Name             string
	Kind             apexast.DeclarationKind
	SuperClass       string
	EnumHashBase     *int64 // stable platform enum family hash seed, when acquired
	Modifiers        []string
	Interfaces       []string
	Constructors     [][]string
	ConstructorSpecs []StandardConstructorSpec
	// ReplaceConstructors makes this standard-platform spec authoritative for
	// source-visible constructors, including an intentional empty set.
	ReplaceConstructors bool
	Methods             []StandardMethodSpec
	Properties          []StandardPropertySpec
}

type StandardConstructorSpec struct {
	Parameters []StandardParameterSpec
}

type StandardParameterSpec struct {
	Name string
	Type string
}

type StandardMethodSpec struct {
	Name           string
	ReturnType     string
	Parameters     []string
	ParameterSpecs []StandardParameterSpec
	Modifiers      []string
	Static         bool
}

type StandardPropertySpec struct {
	Name   string
	Type   string
	Static bool
	Force  bool
}

func standardEnumProperties(typeName string, names ...string) []StandardPropertySpec {
	props := make([]StandardPropertySpec, 0, len(names))
	for _, name := range names {
		props = append(props, StandardPropertySpec{Name: name, Type: typeName, Static: true})
	}
	return props
}

func standardEnumHashBase(value int64) *int64 {
	return &value
}

var (
	standardPlatformSymbolsOnce  sync.Once
	standardPlatformSymbolsCache []TypeSymbol
)

func StandardPlatformSymbols() []TypeSymbol {
	standardPlatformSymbolsOnce.Do(func() {
		standardPlatformSymbolsCache = buildStandardPlatformSymbols()
	})
	return cloneTypeSymbols(standardPlatformSymbolsCache)
}

// StandardPlatformSymbolView returns cached platform symbols for read-only walkers.
// Callers that may mutate symbols or nested slices must use StandardPlatformSymbols.
func StandardPlatformSymbolView() []TypeSymbol {
	standardPlatformSymbolsOnce.Do(func() {
		standardPlatformSymbolsCache = buildStandardPlatformSymbols()
	})
	return standardPlatformSymbolsCache
}

var (
	standardSystemNamespaceTypeNamesOnce  sync.Once
	standardSystemNamespaceTypeNamesCache []string
	standardSchemaNamespaceTypeNamesOnce  sync.Once
	standardSchemaNamespaceTypeNamesCache []string
)

func StandardSystemNamespaceTypeNames() []string {
	standardSystemNamespaceTypeNamesOnce.Do(func() {
		standardSystemNamespaceTypeNamesCache = buildStandardSystemNamespaceTypeNames()
	})
	return append([]string(nil), standardSystemNamespaceTypeNamesCache...)
}

func StandardSchemaNamespaceTypeNames() []string {
	standardSchemaNamespaceTypeNamesOnce.Do(func() {
		standardSchemaNamespaceTypeNamesCache = buildStandardSchemaNamespaceTypeNames()
	})
	return append([]string(nil), standardSchemaNamespaceTypeNamesCache...)
}

func buildStandardSystemNamespaceTypeNames() []string {
	seen := map[string]string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || strings.Contains(name, ".") {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; !ok {
			seen[key] = name
		}
	}
	for _, specs := range [][]StandardSymbolSpec{
		standardPlatformSymbolSpecs,
		systemStubSymbolSpecs,
		standardPlatformSymbolOverlays,
	} {
		for _, spec := range specs {
			add(spec.Name)
		}
	}
	return sortedNamespaceNames(seen)
}

func buildStandardSchemaNamespaceTypeNames() []string {
	seen := map[string]string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		const prefix = "Schema."
		if !strings.HasPrefix(name, prefix) {
			return
		}
		short := strings.TrimPrefix(name, prefix)
		if short == "" || strings.Contains(short, ".") {
			return
		}
		key := strings.ToLower(short)
		if _, ok := seen[key]; !ok {
			seen[key] = short
		}
	}
	for _, specs := range [][]StandardSymbolSpec{
		standardPlatformSymbolSpecs,
		systemStubSymbolSpecs,
		standardPlatformSymbolOverlays,
	} {
		for _, spec := range specs {
			add(spec.Name)
		}
	}
	return sortedNamespaceNames(seen)
}

func sortedNamespaceNames(names map[string]string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, name)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func buildStandardPlatformSymbols() []TypeSymbol {
	specs := append([]StandardSymbolSpec(nil), standardPlatformSymbolSpecs...)
	specs = append(specs, productNamespaceSymbolSpecs...)
	specs = append(specs, systemStubSymbolSpecs...)
	specs = append(specs, dataSourcePlatformSymbolOverlays...)
	specs = append(specs, standardPlatformSymbolOverlays...)
	for _, name := range standardPlatformTypeNames {
		if standardSpecExists(specs, name) {
			continue
		}
		specs = append(specs, StandardSymbolSpec{Name: name, Kind: apexast.DeclarationClass})
	}
	return StandardSymbolsFromSpecs(specs)
}

func cloneTypeSymbols(symbols []TypeSymbol) []TypeSymbol {
	out := make([]TypeSymbol, len(symbols))
	for i, symbol := range symbols {
		out[i] = symbol
		out[i].Modifiers = append([]string(nil), symbol.Modifiers...)
		out[i].Interfaces = append([]string(nil), symbol.Interfaces...)
		out[i].Members = cloneMemberSymbols(symbol.Members)
	}
	return out
}

func cloneMemberSymbols(members []MemberSymbol) []MemberSymbol {
	out := make([]MemberSymbol, len(members))
	for i, member := range members {
		out[i] = member
		out[i].Modifiers = append([]string(nil), member.Modifiers...)
		out[i].Parameters = cloneParameters(member.Parameters)
		out[i].Accessors = cloneAccessors(member.Accessors)
	}
	return out
}

func cloneParameters(parameters []apexast.Parameter) []apexast.Parameter {
	out := make([]apexast.Parameter, len(parameters))
	for i, parameter := range parameters {
		out[i] = parameter
		out[i].Modifiers = append([]string(nil), parameter.Modifiers...)
	}
	return out
}

func cloneAccessors(accessors []apexast.Accessor) []apexast.Accessor {
	out := make([]apexast.Accessor, len(accessors))
	for i, accessor := range accessors {
		out[i] = accessor
		out[i].Modifiers = append([]string(nil), accessor.Modifiers...)
	}
	return out
}

func StandardSObjectSymbols(names []string) []TypeSymbol {
	seen := make(map[string]bool, len(names))
	specs := make([]StandardSymbolSpec, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		specs = append(specs, StandardSymbolSpec{Name: name, Kind: apexast.DeclarationClass, SuperClass: "SObject"})
	}
	return StandardSymbolsFromSpecs(specs)
}

func StandardTypeNameSymbols(names []string) []TypeSymbol {
	specs := make([]StandardSymbolSpec, 0, len(names))
	for _, name := range names {
		specs = append(specs, StandardSymbolSpec{Name: name, Kind: apexast.DeclarationClass})
	}
	return StandardSymbolsFromSpecs(specs)
}

func StandardSymbolsFromSpecs(specs []StandardSymbolSpec) []TypeSymbol {
	specs = mergeStandardSymbolSpecs(specs)
	out := make([]TypeSymbol, 0, len(specs))
	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		if spec.Name == "" {
			continue
		}
		key := strings.ToLower(spec.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		kind := spec.Kind
		if kind == "" {
			kind = apexast.DeclarationClass
		}
		superClass := spec.SuperClass
		if kind != apexast.DeclarationClass {
			superClass = ""
		}
		sym := TypeSymbol{
			Kind:                      kind,
			Name:                      spec.Name,
			File:                      "<standard-platform>",
			Dependency:                true,
			ConstructorsAuthoritative: spec.ReplaceConstructors,
			EnumHashBase:              spec.EnumHashBase,
			SuperClass:                superClass,
			Modifiers:                 append([]string(nil), spec.Modifiers...),
			Interfaces:                append([]string(nil), spec.Interfaces...),
		}
		if namespace, localName, ok := splitStandardSymbolName(spec.Name); ok {
			sym.Namespace = namespace
			sym.Name = localName
		}
		if kind == apexast.DeclarationClass {
			for _, ctor := range standardConstructorSpecs(spec) {
				sym.Members = append(sym.Members, MemberSymbol{
					Kind:       apexast.DeclarationConstructor,
					Name:       localStandardSymbolName(spec.Name),
					Modifiers:  []string{"public", "passive-generated"},
					Parameters: standardSpecParameters(ctor.Parameters),
				})
			}
		}
		for _, prop := range spec.Properties {
			modifiers := []string{"public"}
			if prop.Static {
				modifiers = append(modifiers, "static")
			}
			sym.Members = append(sym.Members, MemberSymbol{
				Kind:      apexast.DeclarationProperty,
				Name:      prop.Name,
				Type:      prop.Type,
				Modifiers: modifiers,
			})
		}
		for _, method := range spec.Methods {
			modifiers := []string{"public"}
			modifiers = append(modifiers, method.Modifiers...)
			if method.Static {
				modifiers = append(modifiers, "static")
			}
			sym.Members = append(sym.Members, MemberSymbol{
				Kind:       apexast.DeclarationMethod,
				Name:       method.Name,
				Type:       method.ReturnType,
				Modifiers:  modifiers,
				Parameters: standardMethodParameters(spec.Name, method),
			})
		}
		out = append(out, sym)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

func standardConstructorSpecs(spec StandardSymbolSpec) []StandardConstructorSpec {
	out := append([]StandardConstructorSpec(nil), spec.ConstructorSpecs...)
	for _, ctor := range spec.Constructors {
		out = append(out, StandardConstructorSpec{Parameters: standardParameterSpecs(ctor)})
	}
	return out
}

func mergeStandardSymbolSpecs(specs []StandardSymbolSpec) []StandardSymbolSpec {
	out := make([]StandardSymbolSpec, 0, len(specs))
	byName := make(map[string]int, len(specs))
	for _, spec := range specs {
		if spec.Name == "" {
			continue
		}
		key := strings.ToLower(spec.Name)
		existingIndex, ok := byName[key]
		if !ok {
			byName[key] = len(out)
			out = append(out, spec)
			continue
		}
		existing := &out[existingIndex]
		if spec.Kind != "" {
			existing.Kind = spec.Kind
		}
		if existing.SuperClass == "" {
			existing.SuperClass = spec.SuperClass
		}
		if spec.EnumHashBase != nil {
			existing.EnumHashBase = spec.EnumHashBase
		}
		existing.Modifiers = appendUniqueStandardStrings(existing.Modifiers, spec.Modifiers)
		existing.Interfaces = appendUniqueStandardStrings(existing.Interfaces, spec.Interfaces)
		if spec.ReplaceConstructors {
			existing.ReplaceConstructors = true
			existing.Constructors = append([][]string(nil), spec.Constructors...)
			existing.ConstructorSpecs = append([]StandardConstructorSpec(nil), spec.ConstructorSpecs...)
		} else if !existing.ReplaceConstructors {
			existing.Constructors = appendUniqueStandardConstructors(existing.Constructors, spec.Constructors)
			existing.ConstructorSpecs = appendUniqueStandardConstructorSpecs(existing.ConstructorSpecs, spec.ConstructorSpecs)
		}
		existing.Methods = appendUniqueStandardMethods(existing.Methods, spec.Methods)
		existing.Properties = appendUniqueStandardProperties(existing.Properties, spec.Properties)
	}
	return out
}

func appendUniqueStandardStrings(values, additions []string) []string {
	for _, addition := range additions {
		if addition == "" {
			continue
		}
		found := false
		for _, value := range values {
			if strings.EqualFold(value, addition) {
				found = true
				break
			}
		}
		if !found {
			values = append(values, addition)
		}
	}
	return values
}

func appendUniqueStandardConstructors(values, additions [][]string) [][]string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[standardTypeListKey(value)] = true
	}
	for _, addition := range additions {
		key := standardTypeListKey(addition)
		if seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, addition)
	}
	return values
}

func appendUniqueStandardConstructorSpecs(values, additions []StandardConstructorSpec) []StandardConstructorSpec {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[standardConstructorSpecKey(value)] = true
	}
	for _, addition := range additions {
		key := standardConstructorSpecKey(addition)
		if seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, addition)
	}
	return values
}

func appendUniqueStandardMethods(values, additions []StandardMethodSpec) []StandardMethodSpec {
	seen := make(map[string]bool, len(values)+len(additions))
	byKey := make(map[string]int, len(values))
	for i, value := range values {
		seen[standardMethodKey(value)] = true
		byKey[standardMethodKey(value)] = i
	}
	for _, addition := range additions {
		key := standardMethodKey(addition)
		if seen[key] {
			if index, ok := byKey[key]; ok && shouldReplaceStandardMethod(values[index], addition) {
				values[index] = addition
			}
			continue
		}
		seen[key] = true
		byKey[key] = len(values)
		values = append(values, addition)
	}
	return values
}

func shouldReplaceStandardMethod(existing, addition StandardMethodSpec) bool {
	if standardModifierContains(addition.Modifiers, "abstract") && !standardModifierContains(existing.Modifiers, "abstract") {
		return true
	}
	existingType := strings.TrimSpace(existing.ReturnType)
	additionType := strings.TrimSpace(addition.ReturnType)
	if additionType == "" || strings.EqualFold(additionType, "Object") {
		return false
	}
	return existingType == "" || strings.EqualFold(existingType, "Object")
}

func standardModifierContains(modifiers []string, wanted string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(strings.TrimSpace(modifier), wanted) {
			return true
		}
	}
	return false
}

func appendUniqueStandardProperties(values, additions []StandardPropertySpec) []StandardPropertySpec {
	seen := make(map[string]bool, len(values)+len(additions))
	byKey := make(map[string]int, len(values))
	for i, value := range values {
		key := standardPropertyKey(value)
		seen[key] = true
		byKey[key] = i
	}
	for _, addition := range additions {
		key := standardPropertyKey(addition)
		if seen[key] {
			if index, ok := byKey[key]; ok && shouldReplaceStandardProperty(values[index], addition) {
				values[index] = addition
			}
			continue
		}
		seen[key] = true
		byKey[key] = len(values)
		values = append(values, addition)
	}
	return values
}

func shouldReplaceStandardProperty(existing, addition StandardPropertySpec) bool {
	existingType := strings.TrimSpace(existing.Type)
	additionType := strings.TrimSpace(addition.Type)
	if additionType == "" || strings.EqualFold(additionType, "Object") {
		return false
	}
	if addition.Force {
		return true
	}
	if strings.EqualFold(additionType, "Id") &&
		strings.EqualFold(existingType, "String") &&
		strings.HasSuffix(strings.ToLower(existing.Name), "id") {
		return true
	}
	return existingType == "" || strings.EqualFold(existingType, "Object")
}

func standardMethodKey(method StandardMethodSpec) string {
	types := make([]string, 0, len(method.ParameterSpecs))
	for _, param := range method.ParameterSpecs {
		types = append(types, param.Type)
	}
	if len(types) == 0 {
		types = method.Parameters
	}
	return strings.ToLower(method.Name) + "|" + strconv.FormatBool(method.Static) + "|" + standardTypeListKey(types)
}

func standardConstructorSpecKey(ctor StandardConstructorSpec) string {
	types := make([]string, 0, len(ctor.Parameters))
	for _, param := range ctor.Parameters {
		types = append(types, param.Type)
	}
	return standardTypeListKey(types)
}

func standardPropertyKey(prop StandardPropertySpec) string {
	return strings.ToLower(prop.Name) + "|" + strconv.FormatBool(prop.Static)
}

func standardTypeListKey(types []string) string {
	normalized := make([]string, 0, len(types))
	for _, typ := range types {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(typ)))
	}
	return strings.Join(normalized, ",")
}

func standardSpecExists(specs []StandardSymbolSpec, name string) bool {
	for _, spec := range specs {
		if strings.EqualFold(spec.Name, name) {
			return true
		}
	}
	return false
}

func splitStandardSymbolName(name string) (string, string, bool) {
	idx := strings.LastIndexByte(name, '.')
	if idx <= 0 || idx == len(name)-1 {
		return "", name, false
	}
	return name[:idx], name[idx+1:], true
}

func localStandardSymbolName(name string) string {
	_, local, ok := splitStandardSymbolName(name)
	if ok {
		return local
	}
	return name
}

func standardParameters(types []string) []apexast.Parameter {
	return standardSpecParameters(standardParameterSpecs(types))
}

func standardMethodParameters(typeName string, method StandardMethodSpec) []apexast.Parameter {
	if len(method.ParameterSpecs) != 0 {
		return standardSpecParameters(normalizeStandardMethodParameterSpecs(typeName, method))
	}
	return standardParameters(method.Parameters)
}

func normalizeStandardMethodParameterSpecs(typeName string, method StandardMethodSpec) []StandardParameterSpec {
	specs := append([]StandardParameterSpec(nil), method.ParameterSpecs...)
	for i := range specs {
		if strings.EqualFold(specs[i].Name, "accessLevel") && specs[i].Type == "Object" {
			specs[i].Name = "accessLevel"
			specs[i].Type = "AccessLevel"
			continue
		}
		if strings.EqualFold(typeName, "Database") && strings.EqualFold(specs[i].Name, "callback") && specs[i].Type == "Object" {
			switch strings.ToLower(method.Name) {
			case "deleteasync":
				specs[i].Type = "DataSource.AsyncDeleteCallback"
			case "insertasync", "updateasync":
				specs[i].Type = "DataSource.AsyncSaveCallback"
			}
		}
		if strings.EqualFold(specs[i].Name, "dmlOptions") && specs[i].Type == "Object" {
			specs[i].Name = "dmlOptions"
			specs[i].Type = "Database.DMLOptions"
			continue
		}
		if strings.EqualFold(typeName, "Search") && strings.EqualFold(method.Name, "suggest") &&
			strings.EqualFold(specs[i].Name, "options") && specs[i].Type == "Object" {
			specs[i].Type = "Search.SuggestionOption"
		}
	}
	return specs
}

func standardParameterSpecs(types []string) []StandardParameterSpec {
	params := make([]StandardParameterSpec, 0, len(types))
	for i, typ := range types {
		params = append(params, StandardParameterSpec{Name: standardParameterName(i), Type: typ})
	}
	return params
}

func standardSpecParameters(specs []StandardParameterSpec) []apexast.Parameter {
	params := make([]apexast.Parameter, 0, len(specs))
	for i, spec := range specs {
		name := spec.Name
		if name == "" {
			name = standardParameterName(i)
		}
		params = append(params, apexast.Parameter{Name: name, Type: spec.Type})
	}
	return params
}

func standardParameterName(index int) string {
	return "arg" + strconv.Itoa(index)
}

var standardPlatformTypeNames = []string{
	"AccessLevel",
	"AccessType",
	"AggregateResult",
	"Address",
	"Assert",
	"ApexPages",
	"ApexPages.Message",
	"ApexPages.StandardController",
	"ApexPages.StandardSetController",
	"Approval",
	"Auth",
	"Auth.AuthConfiguration",
	"Auth.AuthConfig",
	"Auth.AuthToken",
	"Auth.AuthProviderCallbackState",
	"Auth.AuthProviderPlugin",
	"Auth.AuthProviderPluginClass",
	"Auth.AuthProviderTokenResponse",
	"Auth.CommunitiesUtil",
	"Auth.ConfigurableSelfRegHandler",
	"Auth.ConfirmUserRegistrationHandler",
	"Auth.ConnectedAppPlugin",
	"Auth.CustomOneTimePasswordDeliveryHandler",
	"Auth.CustomOneTimePasswordDeliveryResult",
	"Auth.ExternalClientAppOauthHandler",
	"Auth.GeneratedUserData",
	"Auth.HeadlessSelfRegistrationHandler",
	"Auth.HeadlessUserDiscoveryHandler",
	"Auth.HeadlessUserDiscoveryResponse",
	"Auth.HttpCalloutMockUtil",
	"Auth.IntegratingAppType",
	"Auth.InvocationContext",
	"Auth.JWS",
	"Auth.JWT",
	"Auth.JWTBearerTokenExchange",
	"Auth.JWTUtil",
	"Auth.JsonValueOutput",
	"Auth.LightningLoginEligibility",
	"Auth.LoginDiscoveryHandler",
	"Auth.LoginDiscoveryMethod",
	"Auth.MyDomainLoginDiscoveryHandler",
	"Auth.OAuth2TokenExchangeType",
	"Auth.OAuthRefreshResult",
	"Auth.Oauth2TokenExchangeHandler",
	"Auth.OauthToken",
	"Auth.OauthTokenType",
	"Auth.RegistrationHandler",
	"Auth.SamlJitHandler",
	"Auth.SessionLevel",
	"Auth.SessionManagement",
	"Auth.TokenValidationResult",
	"Auth.UserData",
	"Auth.VerificationAction",
	"Auth.VerificationMethod",
	"Auth.VerificationPolicy",
	"Auth.VerificationResult",
	"Cache",
	"Cache.CacheBuilder",
	"Cache.Org",
	"Cache.OrgPartition",
	"Cache.Session",
	"Cache.SessionPartition",
	"Cache.Visibility",
	"Callable",
	"Canvas",
	"ConnectApi",
	"ConnectApi.ChatterFeeds",
	"ConnectApi.Communities",
	"ConnectApi.Community",
	"ConnectApi.Organization",
	"ConnectApi.OrganizationSettings",
	"ConnectApi.TimeZone",
	"ConnectApi.UserProfiles",
	"ConnectApi.UserSettings",
	"Comparator",
	"CronJobDetail",
	"CronTrigger",
	"Datacloud",
	"Datacloud.AdditionalInformationMap",
	"Datacloud.DuplicateResult",
	"Datacloud.FieldDiff",
	"Datacloud.MatchRecord",
	"Datacloud.MatchResult",
	"Database",
	"Database.BatchableContext",
	"Database.DeleteResult",
	"Database.DMLOptions",
	"Database.EmptyRecycleBinResult",
	"Database.AssignmentRuleHeader",
	"Database.DuplicateRuleHeader",
	"Database.EmailHeader",
	"Database.LocaleOptions",
	"Database.Error",
	"Database.DuplicateError",
	"Database.LeadConvert",
	"Database.LeadConvertResult",
	"Database.MergeResult",
	"Database.QueryLocator",
	"Database.SaveResult",
	"Database.Savepoint",
	"Database.UndeleteResult",
	"Database.UnitOfWork",
	"Database.UpsertResult",
	"DataSource",
	"DataWeave.Result",
	"DataWeave.Script",
	"DataWeaveScriptException",
	"DataWeaveScriptResource",
	"Dom",
	"Dom.Document",
	"Dom.XmlNode",
	"Dom.XmlNodeType",
	"EntityDefinition",
	"EntityParticle",
	"FieldDefinition",
	"FeatureManagement",
	"Finalizer",
	"FinalizerContext",
	"Http",
	"HttpCalloutMock",
	"HttpRequest",
	"HttpResponse",
	"WebServiceCallout",
	"WebServiceMock",
	"InstallContext",
	"InstallHandler",
	"Folder",
	"Iterable",
	"Iterator",
	"JSONGenerator",
	"JSONParser",
	"JSONToken",
	"Limits",
	"LoggingLevel",
	"Matcher",
	"Messaging",
	"Messaging.AttachmentRetrievalOption",
	"Messaging.Email",
	"Messaging.MassEmailMessage",
	"Messaging.SendEmailResult",
	"Messaging.SingleEmailMessage",
	"Metadata",
	"Metadata.AsyncResult",
	"Metadata.CustomField",
	"Metadata.CustomMetadata",
	"Metadata.CustomMetadataValue",
	"Metadata.CustomObject",
	"Metadata.DeployCallback",
	"Metadata.DeployCallbackContext",
	"Metadata.DeployContainer",
	"Metadata.DeployDetails",
	"Metadata.DeployMessage",
	"Metadata.DeployResult",
	"Metadata.DeployStatus",
	"Metadata.Metadata",
	"Metadata.MetadataType",
	"Metadata.Operations",
	"MultiStaticResourceCalloutMock",
	"Network",
	"Note",
	"PageReference",
	"Pattern",
	"PicklistEntry",
	"Queueable",
	"QueueableContext",
	"RecentlyViewed",
	"RecordTypeInfo",
	"RestRequest",
	"RestResponse",
	"Schedulable",
	"SchedulableContext",
	"Schema",
	"Schema.ChildRelationship",
	"Schema.DescribeColorResult",
	"Schema.DescribeFieldResult",
	"Schema.DescribeIconResult",
	"Schema.DescribeSObjectResult",
	"Schema.DescribeTabResult",
	"Schema.DescribeTabSetResult",
	"Schema.FieldSet",
	"Schema.FieldSetMap",
	"Schema.FieldSetMember",
	"Schema.PicklistEntry",
	"Schema.RecordTypeInfo",
	"Schema.DisplayType",
	"Schema.SObjectField",
	"Schema.SObjectType",
	"Schema.SObjectTypeFields",
	"Schema.SObjectTypeFieldSets",
	"Schema.SoapType",
	"SObjectDescribeOptions",
	"Label",
	"Search",
	"Security",
	"SelectOption",
	"Site",
	"Site.ExternalUserCreateException",
	"Site.UrlRewriter",
	"StatusCode",
	"FieldSet",
	"FieldSetMap",
	"FieldSetMember",
	"SObjectAccessDecision",
	"SObjectField",
	"SObjectType",
	"SoapType",
	"DisplayType",
	"System",
	"System.Assert",
	"System.Label",
	"Test",
	"TimeZone",
	"TriggerOperation",
	"UserEntityAccess",
	"UserFieldAccess",
	"UserInfo",
	"UserManagement",
	"UUID",
}

var standardPlatformSymbolSpecs = []StandardSymbolSpec{
	{Name: "AccessLevel", Kind: apexast.DeclarationEnum, Methods: []StandardMethodSpec{{Name: "withPermissionSetId", ReturnType: "AccessLevel", Parameters: []string{"String"}}}},
	{Name: "EventBus", Methods: []StandardMethodSpec{
		{Name: "publishWithAccessLevel", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "AccessLevel"}, Static: true},
		{Name: "publishWithAccessLevel", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "Object", "AccessLevel"}, Static: true},
		{Name: "publishWithAccessLevel", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "AccessLevel"}, Static: true},
		{Name: "publishWithAccessLevel", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Object", "AccessLevel"}, Static: true},
	}},
	{Name: "Version", Constructors: [][]string{{"Integer", "Integer"}, {"Integer", "Integer", "Integer"}}, Methods: []StandardMethodSpec{{Name: "compareTo", ReturnType: "Integer", Parameters: []string{"Version"}}}},
	{Name: "InstallContext", Methods: []StandardMethodSpec{{Name: "previousVersion", ReturnType: "Version"}, {Name: "isPush", ReturnType: "Boolean"}, {Name: "installerId", ReturnType: "Id"}}, Properties: []StandardPropertySpec{{Name: "installerId", Type: "Id"}, {Name: "InstallerId", Type: "Id"}}},
	{Name: "UninstallContext", Methods: []StandardMethodSpec{{Name: "organizationId", ReturnType: "Id"}}, Properties: []StandardPropertySpec{{Name: "organizationId", Type: "Id"}, {Name: "OrganizationId", Type: "Id"}}},
	{Name: "InstallHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "onInstall", ReturnType: "void", Parameters: []string{"InstallContext"}}}},
	{Name: "UninstallHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "onUninstall", ReturnType: "void", Parameters: []string{"UninstallContext"}}}},
	{Name: "IntegrationTest", Methods: []StandardMethodSpec{{Name: "commitTestOnly", ReturnType: "void", Static: true}}},
	{Name: "URL", Constructors: [][]string{{"URL", "String"}}, Methods: []StandardMethodSpec{{Name: "getSalesforceBaseUrl", ReturnType: "URL", Static: true}, {Name: "toExternalForm", ReturnType: "String"}}},
	{Name: "PageReference", Constructors: [][]string{{"String"}, {"SObject"}}, Methods: []StandardMethodSpec{{Name: "getUrl", ReturnType: "String"}, {Name: "setRedirect", ReturnType: "PageReference", Parameters: []string{"Boolean"}}, {Name: "getParameters", ReturnType: "Map<String,String>"}, {Name: "setCookies", ReturnType: "void", Parameters: []string{"List<Cookie>"}}}},
	{Name: "Search", Methods: []StandardMethodSpec{
		{Name: "query", ReturnType: "List<List<SObject>>", Parameters: []string{"String"}, Static: true},
		{Name: "query", ReturnType: "List<List<SObject>>", Parameters: []string{"String", "AccessLevel"}, Static: true},
		{Name: "find", ReturnType: "Search.SearchResults", Parameters: []string{"String"}, Static: true},
		{Name: "find", ReturnType: "Search.SearchResults", Parameters: []string{"String", "AccessLevel"}, Static: true},
		{Name: "suggest", ReturnType: "Search.SuggestionResults", Parameters: []string{"String", "String", "Search.SuggestionOption"}, Static: true},
		{Name: "suggest", ReturnType: "Search.SuggestionResults", Parameters: []string{"String", "String", "Search.SuggestionOption", "AccessLevel"}, Static: true},
	}},
	{Name: "Exception", Constructors: [][]string{{}, {"Exception"}, {"String"}, {"String", "Exception"}}},
	{Name: "Decimal", Methods: []StandardMethodSpec{{Name: "divide", ReturnType: "Decimal", Parameters: []string{"Decimal", "Integer", "RoundingMode"}}}},
	{Name: "Label", Methods: []StandardMethodSpec{{Name: "get", ReturnType: "String", Parameters: []string{"String", "String"}, Static: true}, {Name: "get", ReturnType: "String", Parameters: []string{"String", "String", "String"}, Static: true}, {Name: "translationExists", ReturnType: "Boolean", Parameters: []string{"String", "String", "String"}, Static: true}}},
	{Name: "System.Label", Methods: []StandardMethodSpec{{Name: "get", ReturnType: "String", Parameters: []string{"String", "String"}, Static: true}, {Name: "get", ReturnType: "String", Parameters: []string{"String", "String", "String"}, Static: true}, {Name: "translationExists", ReturnType: "Boolean", Parameters: []string{"String", "String", "String"}, Static: true}}},
	{Name: "InvalidParameterValueException", SuperClass: "Exception", Constructors: [][]string{{}, {"Exception"}, {"String"}}},
	{Name: "NoAccessException", SuperClass: "Exception", Constructors: [][]string{{"Exception"}, {"String"}, {"String", "Exception"}}},
	{Name: "NoDataFoundException", SuperClass: "Exception", Constructors: [][]string{{"Exception"}, {"String"}, {"String", "Exception"}}},
	{Name: "NullPointerException", SuperClass: "Exception", Constructors: [][]string{{"Exception"}, {"String"}, {"String", "Exception"}}},
	{Name: "TouchHandledException", SuperClass: "Exception", Constructors: [][]string{{}, {"String", "Exception"}}},
	{Name: "Answers", Methods: []StandardMethodSpec{{Name: "findSimilar", ReturnType: "List<Id>", Parameters: []string{"Question"}, Static: true}}},
	{Name: "Approval", Methods: []StandardMethodSpec{
		{Name: "isLocked", ReturnType: "Boolean", Parameters: []string{"Id"}, Static: true},
		{Name: "isLocked", ReturnType: "Map<Id,Boolean>", Parameters: []string{"List<Id>"}, Static: true},
		{Name: "isLocked", ReturnType: "Boolean", Parameters: []string{"SObject"}, Static: true},
		{Name: "isLocked", ReturnType: "Map<Id,Boolean>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "lock", ReturnType: "Approval.LockResult", Parameters: []string{"Id"}, Static: true},
		{Name: "lock", ReturnType: "Approval.LockResult", Parameters: []string{"Id", "Boolean"}, Static: true},
		{Name: "lock", ReturnType: "List<Approval.LockResult>", Parameters: []string{"List<Id>"}, Static: true},
		{Name: "lock", ReturnType: "List<Approval.LockResult>", Parameters: []string{"List<Id>", "Boolean"}, Static: true},
		{Name: "lock", ReturnType: "Approval.LockResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "lock", ReturnType: "Approval.LockResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "lock", ReturnType: "List<Approval.LockResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "lock", ReturnType: "List<Approval.LockResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "unlock", ReturnType: "Approval.UnlockResult", Parameters: []string{"Id"}, Static: true},
		{Name: "unlock", ReturnType: "Approval.UnlockResult", Parameters: []string{"Id", "Boolean"}, Static: true},
		{Name: "unlock", ReturnType: "List<Approval.UnlockResult>", Parameters: []string{"List<Id>"}, Static: true},
		{Name: "unlock", ReturnType: "List<Approval.UnlockResult>", Parameters: []string{"List<Id>", "Boolean"}, Static: true},
		{Name: "unlock", ReturnType: "Approval.UnlockResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "unlock", ReturnType: "Approval.UnlockResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "unlock", ReturnType: "List<Approval.UnlockResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "unlock", ReturnType: "List<Approval.UnlockResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "process", ReturnType: "Approval.ProcessResult", Parameters: []string{"Approval.ProcessRequest"}, Static: true},
		{Name: "process", ReturnType: "Approval.ProcessResult", Parameters: []string{"Approval.ProcessRequest", "Boolean"}, Static: true},
		{Name: "process", ReturnType: "List<Approval.ProcessResult>", Parameters: []string{"List<Approval.ProcessRequest>"}, Static: true},
		{Name: "process", ReturnType: "List<Approval.ProcessResult>", Parameters: []string{"List<Approval.ProcessRequest>", "Boolean"}, Static: true},
		{Name: "process", ReturnType: "Approval.ProcessResult", Parameters: []string{"Approval.ProcessSubmitRequest"}, Static: true},
		{Name: "process", ReturnType: "Approval.ProcessResult", Parameters: []string{"Approval.ProcessSubmitRequest", "Boolean"}, Static: true},
		{Name: "process", ReturnType: "Approval.ProcessResult", Parameters: []string{"Approval.ProcessWorkitemRequest"}, Static: true},
		{Name: "process", ReturnType: "Approval.ProcessResult", Parameters: []string{"Approval.ProcessWorkitemRequest", "Boolean"}, Static: true},
		{Name: "process", ReturnType: "List<Approval.ProcessResult>", Parameters: []string{"List<Approval.ProcessSubmitRequest>"}, Static: true},
		{Name: "process", ReturnType: "List<Approval.ProcessResult>", Parameters: []string{"List<Approval.ProcessWorkitemRequest>"}, Static: true},
	}},
	{Name: "Approval.ProcessSubmitRequest", SuperClass: "Approval.ProcessRequest"},
	{Name: "Approval.ProcessWorkitemRequest", SuperClass: "Approval.ProcessRequest"},
	{Name: "BusinessHours", Methods: []StandardMethodSpec{
		{Name: "add", ReturnType: "Datetime", Parameters: []string{"Id", "Datetime", "Long"}, Static: true},
		{Name: "addGmt", ReturnType: "Datetime", Parameters: []string{"Id", "Datetime", "Long"}, Static: true},
		{Name: "diff", ReturnType: "Long", Parameters: []string{"String", "Datetime", "Datetime"}, Static: true},
		{Name: "isWithin", ReturnType: "Boolean", Parameters: []string{"String", "Datetime"}, Static: true},
		{Name: "nextStartDate", ReturnType: "Datetime", Parameters: []string{"Id", "Datetime"}, Static: true},
	}},
	{Name: "Question", SuperClass: "SObject", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "Title", Type: "String"}, {Name: "CommunityId", Type: "Id"}}},
	{Name: "DataWeave.Script", ReplaceConstructors: true, Methods: []StandardMethodSpec{{Name: "createScript", ReturnType: "DataWeave.Script", Parameters: []string{"String"}, Static: true}, {Name: "execute", ReturnType: "DataWeave.Result", Parameters: []string{"Map<String,Object>"}}}},
	{Name: "DataWeave.Result", ReplaceConstructors: true, Methods: []StandardMethodSpec{{Name: "getValue", ReturnType: "Object"}, {Name: "getValueAsString", ReturnType: "String"}}},
	{Name: "DataWeaveScriptResource", Constructors: [][]string{{}}},
	{Name: "DataWeaveScriptException", SuperClass: "Exception", Constructors: [][]string{{}, {"String"}}},
	{Name: "ConnectApi.ExternalCredential", Properties: []StandardPropertySpec{{Name: "principals", Type: "List<ConnectApi.ExternalCredentialPrincipal>"}}},
	{Name: "ConnectApi.ExternalCredentialInput", Properties: []StandardPropertySpec{
		{Name: "authenticationProtocol", Type: "ConnectApi.CredentialAuthenticationProtocol"},
		{Name: "authenticationProtocolVariant", Type: "ConnectApi.CredentialAuthenticationProtocolVariant"},
		{Name: "customHeaders", Type: "List<ConnectApi.CredentialCustomHeader>"},
		{Name: "developerName", Type: "String"},
		{Name: "masterLabel", Type: "String"},
		{Name: "parameters", Type: "List<ConnectApi.ExternalCredentialParameterInput>"},
		{Name: "principals", Type: "List<ConnectApi.ExternalCredentialPrincipalInput>"},
	}},
	{Name: "ConnectApi.ExternalCredentialPrincipalInput", Properties: []StandardPropertySpec{
		{Name: "id", Type: "String"},
		{Name: "parameters", Type: "List<ConnectApi.ExternalCredentialParameterInput>"},
		{Name: "principalName", Type: "String"},
		{Name: "principalType", Type: "ConnectApi.CredentialPrincipalType"},
		{Name: "sequenceNumber", Type: "Integer"},
	}},
	{Name: "ConnectApi.NamedCredential", Properties: []StandardPropertySpec{{Name: "developerName", Type: "String"}, {Name: "masterLabel", Type: "String"}, {Name: "type", Type: "ConnectApi.NamedCredentialType"}, {Name: "calloutUrl", Type: "String"}, {Name: "calloutOptions", Type: "ConnectApi.NamedCredentialCalloutOptions"}, {Name: "externalCredentials", Type: "List<ConnectApi.ExternalCredential>"}}},
	{Name: "ConnectApi.NamedCredentialInput", Properties: []StandardPropertySpec{{Name: "developerName", Type: "String"}, {Name: "masterLabel", Type: "String"}, {Name: "type", Type: "ConnectApi.NamedCredentialType"}, {Name: "calloutUrl", Type: "String"}, {Name: "calloutOptions", Type: "ConnectApi.NamedCredentialCalloutOptionsInput"}, {Name: "externalCredentials", Type: "List<ConnectApi.ExternalCredentialInput>"}}},
	{Name: "ConnectApi.NamedCredentialCalloutOptions", Properties: []StandardPropertySpec{{Name: "allowMergeFieldsInBody", Type: "Boolean"}, {Name: "allowMergeFieldsInHeader", Type: "Boolean"}, {Name: "generateAuthorizationHeader", Type: "Boolean"}}},
	{Name: "ConnectApi.NamedCredentialCalloutOptionsInput", Properties: []StandardPropertySpec{{Name: "allowMergeFieldsInBody", Type: "Boolean"}, {Name: "allowMergeFieldsInHeader", Type: "Boolean"}, {Name: "generateAuthorizationHeader", Type: "Boolean"}}},
	{Name: "ConnectApi.EinsteinLLMGenerationItemOutput", Properties: []StandardPropertySpec{{Name: "text", Type: "String"}}},
	{Name: "ConnectApi.EinsteinPromptTemplateGenerationsRepresentation", Properties: []StandardPropertySpec{{Name: "generations", Type: "List<ConnectApi.EinsteinLLMGenerationItemOutput>"}}},
	{Name: "ConnectApi.OrchestrationInstance", Properties: []StandardPropertySpec{{Name: "id", Type: "Id"}, {Name: "stageInstances", Type: "List<ConnectApi.OrchestrationStageInstance>"}}},
	{Name: "ConnectApi.OrchestrationStageInstance", Properties: []StandardPropertySpec{
		{Name: "id", Type: "Id"},
		{Name: "label", Type: "String"},
		{Name: "name", Type: "String"},
		{Name: "position", Type: "Integer"},
		{Name: "stageStepInstances", Type: "List<ConnectApi.OrchestrationStepInstance>"},
		{Name: "status", Type: "ConnectApi.OrchestrationInstanceStatus", Force: true},
	}},
	{Name: "ConnectApi.OrchestrationStepInstance", Properties: []StandardPropertySpec{
		{Name: "id", Type: "Id"},
		{Name: "label", Type: "String"},
		{Name: "name", Type: "String"},
		{Name: "status", Type: "ConnectApi.OrchestrationInstanceStatus", Force: true},
		{Name: "type", Type: "ConnectApi.OrchestrationStepType"},
		{Name: "workAssignments", Type: "List<ConnectApi.OrchestrationWorkAssignment>"},
	}},
	{Name: "ConnectApi.OrchestrationWorkAssignment", Properties: []StandardPropertySpec{{Name: "id", Type: "Id"}, {Name: "label", Type: "String"}, {Name: "contextRecordId", Type: "Id"}, {Name: "screenFlowId", Type: "String"}}},
	{Name: "ConnectApi.CommentInput", Properties: []StandardPropertySpec{{Name: "body", Type: "ConnectApi.MessageBodyInput"}}},
	{Name: "ConnectApi.FeedBody", Properties: []StandardPropertySpec{{Name: "messageSegments", Type: "List<ConnectApi.MessageSegment>"}}},
	{Name: "ConnectApi.FeedElement", Properties: []StandardPropertySpec{{Name: "body", Type: "ConnectApi.FeedBody"}, {Name: "id", Type: "Id"}}},
	{Name: "ConnectApi.FeedItemInput", SuperClass: "ConnectApi.FeedElementInput", Properties: []StandardPropertySpec{{Name: "body", Type: "ConnectApi.MessageBodyInput"}, {Name: "feedElementType", Type: "ConnectApi.FeedElementType"}, {Name: "subjectId", Type: "Id"}}},
	{Name: "ConnectApi.MessageBody", Properties: []StandardPropertySpec{{Name: "messageSegments", Type: "List<ConnectApi.MessageSegment>"}}},
	{Name: "ConnectApi.MessageBodyInput", Properties: []StandardPropertySpec{{Name: "messageSegments", Type: "List<ConnectApi.MessageSegmentInput>"}}},
	{Name: "ConnectApi.NBAActionParameter", Properties: []StandardPropertySpec{{Name: "name", Type: "String"}, {Name: "value", Type: "String"}, {Name: "type", Type: "String"}}},
	{Name: "ConnectApi.NBAFlowAction", Properties: []StandardPropertySpec{{Name: "flowType", Type: "ConnectApi.NBAFlowType"}, {Name: "name", Type: "String"}, {Name: "parameters", Type: "List<ConnectApi.NBAActionParameter>"}}},
	{Name: "ConnectApi.NBANativeRecommendation", Properties: []StandardPropertySpec{{Name: "id", Type: "Id"}, {Name: "name", Type: "String"}, {Name: "url", Type: "String"}}},
	{Name: "ConnectApi.NBARecommendation", Properties: []StandardPropertySpec{{Name: "acceptanceLabel", Type: "String"}, {Name: "description", Type: "String"}, {Name: "externalId", Type: "String"}, {Name: "rejectionLabel", Type: "String"}, {Name: "target", Type: "Object"}, {Name: "targetAction", Type: "Object"}}},
	{Name: "ConnectApi.NBARecommendations", Properties: []StandardPropertySpec{{Name: "executionId", Type: "Id"}, {Name: "onBehalfOfId", Type: "Id"}, {Name: "recommendations", Type: "List<ConnectApi.NBARecommendation>"}}},
	{Name: "Metadata.PlatformActionListItem", Properties: []StandardPropertySpec{{Name: "actionName", Type: "String"}}},
	{Name: "Metadata.RelatedListItem", Properties: []StandardPropertySpec{{Name: "relatedList", Type: "String"}}},
	{Name: "ConnectApi.EntityLinkSegment", SuperClass: "ConnectApi.MessageSegment", Properties: []StandardPropertySpec{{Name: "entity", Type: "ConnectApi.Reference"}}},
	{Name: "ConnectApi.EntityLinkSegmentInput", SuperClass: "ConnectApi.MessageSegmentInput", Properties: []StandardPropertySpec{{Name: "entityId", Type: "Id"}}},
	{Name: "ConnectApi.HashtagSegment", SuperClass: "ConnectApi.MessageSegment", Properties: []StandardPropertySpec{{Name: "tag", Type: "String"}}},
	{Name: "ConnectApi.HashtagSegmentInput", SuperClass: "ConnectApi.MessageSegmentInput", Properties: []StandardPropertySpec{{Name: "tag", Type: "String"}}},
	{Name: "ConnectApi.InlineImageSegment", SuperClass: "ConnectApi.MessageSegment", Properties: []StandardPropertySpec{{Name: "altText", Type: "String"}, {Name: "fileId", Type: "Id"}}},
	{Name: "ConnectApi.InlineImageSegmentInput", SuperClass: "ConnectApi.MessageSegmentInput", Properties: []StandardPropertySpec{{Name: "fileId", Type: "Id"}, {Name: "altText", Type: "String"}}},
	{Name: "ConnectApi.LinkSegment", SuperClass: "ConnectApi.MessageSegment", Properties: []StandardPropertySpec{{Name: "url", Type: "String"}}},
	{Name: "ConnectApi.LinkSegmentInput", SuperClass: "ConnectApi.MessageSegmentInput", Properties: []StandardPropertySpec{{Name: "url", Type: "String"}}},
	{Name: "ConnectApi.MarkupBeginSegment", SuperClass: "ConnectApi.MessageSegment", Properties: []StandardPropertySpec{{Name: "markupType", Type: "ConnectApi.MarkupType"}}},
	{Name: "ConnectApi.MarkupBeginSegmentInput", SuperClass: "ConnectApi.MessageSegmentInput", Properties: []StandardPropertySpec{{Name: "markupType", Type: "ConnectApi.MarkupType"}}},
	{Name: "ConnectApi.MarkupEndSegment", SuperClass: "ConnectApi.MessageSegment", Properties: []StandardPropertySpec{{Name: "markupType", Type: "ConnectApi.MarkupType"}}},
	{Name: "ConnectApi.MarkupEndSegmentInput", SuperClass: "ConnectApi.MessageSegmentInput", Properties: []StandardPropertySpec{{Name: "markupType", Type: "ConnectApi.MarkupType"}}},
	{Name: "ConnectApi.MentionSegment", SuperClass: "ConnectApi.MessageSegment", Properties: []StandardPropertySpec{{Name: "record", Type: "ConnectApi.Reference"}}},
	{Name: "ConnectApi.MentionSegmentInput", SuperClass: "ConnectApi.MessageSegmentInput", Properties: []StandardPropertySpec{{Name: "id", Type: "Id"}}},
	{Name: "ConnectApi.Reference", Properties: []StandardPropertySpec{{Name: "id", Type: "Id"}, {Name: "name", Type: "String"}}},
	{Name: "ConnectApi.TextSegment", SuperClass: "ConnectApi.MessageSegment", Properties: []StandardPropertySpec{{Name: "text", Type: "String"}}},
	{Name: "ConnectApi.TextSegmentInput", SuperClass: "ConnectApi.MessageSegmentInput", Properties: []StandardPropertySpec{{Name: "text", Type: "String"}}},
	{Name: "Messaging.BinaryAttachment", Properties: []StandardPropertySpec{{Name: "body", Type: "Blob"}, {Name: "fileName", Type: "String"}, {Name: "headers", Type: "List<Messaging.InboundEmail.Header>"}, {Name: "mimeTypeSubType", Type: "String"}}},
	{Name: "Messaging.InboundEmail", Properties: []StandardPropertySpec{{Name: "authenticationResults", Type: "List<Messaging.InboundEmail.AuthenticationResult>"}, {Name: "binaryAttachments", Type: "List<Messaging.InboundEmail.BinaryAttachment>"}, {Name: "ccAddresses", Type: "List<String>"}, {Name: "fromAddress", Type: "String"}, {Name: "fromName", Type: "String"}, {Name: "headers", Type: "List<Messaging.InboundEmail.Header>"}, {Name: "htmlBody", Type: "String"}, {Name: "htmlBodyIsTruncated", Type: "Boolean"}, {Name: "inReplyTo", Type: "String"}, {Name: "messageId", Type: "String"}, {Name: "plainTextBody", Type: "String"}, {Name: "plainTextBodyIsTruncated", Type: "Boolean"}, {Name: "references", Type: "List<String>"}, {Name: "replyTo", Type: "String"}, {Name: "subject", Type: "String"}, {Name: "textAttachments", Type: "List<Messaging.InboundEmail.TextAttachment>"}, {Name: "toAddresses", Type: "List<String>"}}},
	{Name: "Messaging.InboundEmail.AuthenticationResult", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "authenticationResultFields", Type: "List<Messaging.InboundEmail.AuthenticationResultField>"}, {Name: "method", Type: "String"}, {Name: "result", Type: "String"}}},
	{Name: "Messaging.InboundEmail.AuthenticationResultField", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "name", Type: "String"}, {Name: "value", Type: "String"}}},
	{Name: "Messaging.InboundEmail.BinaryAttachment", SuperClass: "Messaging.BinaryAttachment", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "body", Type: "Blob"}, {Name: "fileName", Type: "String"}, {Name: "headers", Type: "List<Messaging.InboundEmail.Header>"}, {Name: "mimeTypeSubType", Type: "String"}}},
	{Name: "Messaging.InboundEmail.TextAttachment", SuperClass: "Messaging.TextAttachment", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "body", Type: "String"}, {Name: "bodyIsTruncated", Type: "Boolean"}, {Name: "charset", Type: "String"}, {Name: "fileName", Type: "String"}, {Name: "headers", Type: "List<Messaging.InboundEmail.Header>"}, {Name: "mimeTypeSubType", Type: "String"}}},
	{Name: "Messaging.InboundEmail.Header", Properties: []StandardPropertySpec{{Name: "name", Type: "String"}, {Name: "value", Type: "String"}}},
	{Name: "Messaging.InboundEmailResult", Properties: []StandardPropertySpec{{Name: "message", Type: "String"}, {Name: "success", Type: "Boolean"}}},
	{Name: "Messaging.InboundEnvelope", Properties: []StandardPropertySpec{{Name: "fromAddress", Type: "String"}, {Name: "toAddress", Type: "String"}}},
	{Name: "Messaging.TextAttachment", Properties: []StandardPropertySpec{{Name: "body", Type: "String"}, {Name: "bodyIsTruncated", Type: "Boolean"}, {Name: "charset", Type: "String"}, {Name: "fileName", Type: "String"}, {Name: "headers", Type: "List<Messaging.InboundEmail.Header>"}, {Name: "mimeTypeSubType", Type: "String"}}},
	{Name: "VisualEditor.DesignTimePageContext", Properties: []StandardPropertySpec{{Name: "entityName", Type: "String"}}},
	{Name: "Metadata.Layout", Properties: []StandardPropertySpec{{Name: "layoutSections", Type: "List<Metadata.LayoutSection>"}}},
	{Name: "Metadata.LayoutSection", Properties: []StandardPropertySpec{{Name: "layoutColumns", Type: "List<Metadata.LayoutColumn>"}}},
	{Name: "Metadata.LayoutColumn", Properties: []StandardPropertySpec{{Name: "layoutItems", Type: "List<Metadata.LayoutItem>"}}},
	{Name: "Metadata.LayoutItem", Properties: []StandardPropertySpec{{Name: "field", Type: "String"}}},
	{Name: "eventbus.TriggerContext", Properties: []StandardPropertySpec{{Name: "retries", Type: "Integer"}}},
	{Name: "Boolean", Methods: []StandardMethodSpec{{Name: "valueOf", ReturnType: "Boolean", Parameters: []string{"String"}, Static: true}, {Name: "valueOf", ReturnType: "Boolean", Parameters: []string{"Object"}, Static: true}}},
	{Name: "String", Methods: []StandardMethodSpec{{Name: "equals", ReturnType: "Boolean", Parameters: []string{"String"}}, {Name: "template", ReturnType: "String", Parameters: []string{"Map<String,Object>"}}}},
	{Name: "HttpRequest", Methods: []StandardMethodSpec{
		{Name: "getBody", ReturnType: "String"},
		{Name: "getEndpoint", ReturnType: "String"},
	}},
	{Name: "System.HttpRequest", Methods: []StandardMethodSpec{
		{Name: "getBody", ReturnType: "String"},
		{Name: "getEndpoint", ReturnType: "String"},
	}},
	{Name: "System", Methods: []StandardMethodSpec{{Name: "now", ReturnType: "Datetime", Static: true}, {Name: "today", ReturnType: "Date", Static: true}, {Name: "debug", ReturnType: "void", Parameters: []string{"Object"}, Static: true}, {Name: "debug", ReturnType: "void", Parameters: []string{"LoggingLevel", "Object"}, Static: true}, {Name: "assert", ReturnType: "void", Parameters: []string{"Boolean"}, Static: true}, {Name: "assert", ReturnType: "void", Parameters: []string{"Boolean", "Object"}, Static: true}, {Name: "assertEquals", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true}, {Name: "assertEquals", ReturnType: "void", Parameters: []string{"Object", "Object", "Object"}, Static: true}, {Name: "assertNotEquals", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true}, {Name: "assertNotEquals", ReturnType: "void", Parameters: []string{"Object", "Object", "Object"}, Static: true}}},
	{Name: "Test", Methods: []StandardMethodSpec{{Name: "isRunningTest", ReturnType: "Boolean", Static: true}, {Name: "setCurrentPage", ReturnType: "void", Parameters: []string{"PageReference"}, Static: true}, {Name: "setCurrentPageReference", ReturnType: "void", Parameters: []string{"PageReference"}, Static: true}, {Name: "setFixedSearchResults", ReturnType: "void", Parameters: []string{"List<Id>"}, Static: true}}},
	{Name: "Math", Properties: []StandardPropertySpec{{Name: "E", Type: "Decimal", Static: true}, {Name: "PI", Type: "Decimal", Static: true}}},
	{Name: "UserInfo", Constructors: [][]string{{}}, Methods: []StandardMethodSpec{
		{Name: "getCurrentUvid", ReturnType: "String", Static: true},
		{Name: "getDefaultCurrency", ReturnType: "String", Static: true},
		{Name: "getFirstName", ReturnType: "String", Static: true},
		{Name: "getLanguage", ReturnType: "String", Static: true},
		{Name: "getLastName", ReturnType: "String", Static: true},
		{Name: "getLocale", ReturnType: "String", Static: true},
		{Name: "getName", ReturnType: "String", Static: true},
		{Name: "getOrganizationId", ReturnType: "String", Static: true},
		{Name: "getOrganizationName", ReturnType: "String", Static: true},
		{Name: "getProfileId", ReturnType: "String", Static: true},
		{Name: "getSessionId", ReturnType: "String", Static: true},
		{Name: "getTimeZone", ReturnType: "System.TimeZone", Static: true},
		{Name: "getUiTheme", ReturnType: "String", Static: true},
		{Name: "getUiThemeDisplayed", ReturnType: "String", Static: true},
		{Name: "getUserEmail", ReturnType: "String", Static: true},
		{Name: "getUserId", ReturnType: "String", Static: true},
		{Name: "getUserName", ReturnType: "String", Static: true},
		{Name: "getUserRoleId", ReturnType: "String", Static: true},
		{Name: "getUserType", ReturnType: "String", Static: true},
		{Name: "hasPackageLicense", ReturnType: "Boolean", Parameters: []string{"Id"}, Static: true},
		{Name: "isCurrentUserLicensed", ReturnType: "Boolean", Parameters: []string{"String"}, Static: true},
		{Name: "isCurrentUserLicensedForPackage", ReturnType: "Boolean", Parameters: []string{"Id"}, Static: true},
		{Name: "isMultiCurrencyOrganization", ReturnType: "Boolean", Static: true},
	}},
	{Name: "TriggerOperation", Kind: apexast.DeclarationEnum, Properties: []StandardPropertySpec{{Name: "BEFORE_INSERT", Type: "TriggerOperation", Static: true}, {Name: "BEFORE_UPDATE", Type: "TriggerOperation", Static: true}, {Name: "BEFORE_DELETE", Type: "TriggerOperation", Static: true}, {Name: "AFTER_INSERT", Type: "TriggerOperation", Static: true}, {Name: "AFTER_UPDATE", Type: "TriggerOperation", Static: true}, {Name: "AFTER_DELETE", Type: "TriggerOperation", Static: true}, {Name: "AFTER_UNDELETE", Type: "TriggerOperation", Static: true}}},
	{Name: "Trigger", Properties: []StandardPropertySpec{{Name: "isExecuting", Type: "Boolean", Static: true}, {Name: "isInsert", Type: "Boolean", Static: true}, {Name: "isUpdate", Type: "Boolean", Static: true}, {Name: "isDelete", Type: "Boolean", Static: true}, {Name: "isBefore", Type: "Boolean", Static: true}, {Name: "isAfter", Type: "Boolean", Static: true}, {Name: "isUndelete", Type: "Boolean", Static: true}, {Name: "new", Type: "List<SObject>", Static: true}, {Name: "old", Type: "List<SObject>", Static: true}, {Name: "newMap", Type: "Map<Id,SObject>", Static: true}, {Name: "oldMap", Type: "Map<Id,SObject>", Static: true}, {Name: "operationType", Type: "TriggerOperation", Static: true}, {Name: "size", Type: "Integer", Static: true}}},
	{Name: "LoggingLevel", Kind: apexast.DeclarationEnum, Methods: []StandardMethodSpec{{Name: "name", ReturnType: "String"}, {Name: "ordinal", ReturnType: "Integer"}, {Name: "toString", ReturnType: "String"}, {Name: "values", ReturnType: "List<LoggingLevel>", Static: true}, {Name: "valueOf", ReturnType: "LoggingLevel", Parameters: []string{"String"}, Static: true}}, Properties: []StandardPropertySpec{{Name: "NONE", Type: "LoggingLevel", Static: true}, {Name: "ERROR", Type: "LoggingLevel", Static: true}, {Name: "WARN", Type: "LoggingLevel", Static: true}, {Name: "INFO", Type: "LoggingLevel", Static: true}, {Name: "DEBUG", Type: "LoggingLevel", Static: true}, {Name: "FINE", Type: "LoggingLevel", Static: true}, {Name: "FINER", Type: "LoggingLevel", Static: true}, {Name: "FINEST", Type: "LoggingLevel", Static: true}}},
	{Name: "Limits", Methods: []StandardMethodSpec{{Name: "getQueries", ReturnType: "Integer", Static: true}, {Name: "getLimitQueries", ReturnType: "Integer", Static: true}, {Name: "getQueryRows", ReturnType: "Integer", Static: true}, {Name: "getLimitQueryRows", ReturnType: "Integer", Static: true}, {Name: "getDmlStatements", ReturnType: "Integer", Static: true}, {Name: "getLimitDmlStatements", ReturnType: "Integer", Static: true}, {Name: "getDMLStatements", ReturnType: "Integer", Static: true}, {Name: "getLimitDMLStatements", ReturnType: "Integer", Static: true}, {Name: "getDmlRows", ReturnType: "Integer", Static: true}, {Name: "getLimitDmlRows", ReturnType: "Integer", Static: true}, {Name: "getDMLRows", ReturnType: "Integer", Static: true}, {Name: "getLimitDMLRows", ReturnType: "Integer", Static: true}, {Name: "getHeapSize", ReturnType: "Integer", Static: true}, {Name: "getLimitHeapSize", ReturnType: "Integer", Static: true}, {Name: "getCpuTime", ReturnType: "Integer", Static: true}, {Name: "getLimitCpuTime", ReturnType: "Integer", Static: true}, {Name: "getCallouts", ReturnType: "Integer", Static: true}, {Name: "getLimitCallouts", ReturnType: "Integer", Static: true}, {Name: "getAsyncCalls", ReturnType: "Integer", Static: true}, {Name: "getLimitAsyncCalls", ReturnType: "Integer", Static: true}, {Name: "getBatchJobs", ReturnType: "Integer", Static: true}, {Name: "getLimitBatchJobs", ReturnType: "Integer", Static: true}, {Name: "getEmailInvocations", ReturnType: "Integer", Static: true}, {Name: "getLimitEmailInvocations", ReturnType: "Integer", Static: true}}},
	{Name: "Assert", Methods: standardAssertMethods()},
	{Name: "System.Assert", Methods: standardAssertMethods()},
	{Name: "Pattern", Methods: []StandardMethodSpec{{Name: "compile", ReturnType: "Pattern", Parameters: []string{"String"}, Static: true}, {Name: "matches", ReturnType: "Boolean", Parameters: []string{"String", "String"}, Static: true}, {Name: "quote", ReturnType: "String", Parameters: []string{"String"}, Static: true}, {Name: "matcher", ReturnType: "Matcher", Parameters: []string{"String"}}, {Name: "pattern", ReturnType: "String"}, {Name: "split", ReturnType: "List<String>", Parameters: []string{"String"}}, {Name: "split", ReturnType: "List<String>", Parameters: []string{"String", "Integer"}}}},
	{Name: "Matcher", Methods: []StandardMethodSpec{{Name: "find", ReturnType: "Boolean"}, {Name: "find", ReturnType: "Boolean", Parameters: []string{"Integer"}}, {Name: "matches", ReturnType: "Boolean"}, {Name: "lookingAt", ReturnType: "Boolean"}, {Name: "group", ReturnType: "String"}, {Name: "group", ReturnType: "String", Parameters: []string{"Integer"}}, {Name: "groupCount", ReturnType: "Integer"}, {Name: "start", ReturnType: "Integer"}, {Name: "start", ReturnType: "Integer", Parameters: []string{"Integer"}}, {Name: "end", ReturnType: "Integer"}, {Name: "end", ReturnType: "Integer", Parameters: []string{"Integer"}}, {Name: "replaceAll", ReturnType: "String", Parameters: []string{"String"}}, {Name: "replaceFirst", ReturnType: "String", Parameters: []string{"String"}}, {Name: "reset", ReturnType: "Matcher"}, {Name: "reset", ReturnType: "Matcher", Parameters: []string{"String"}}, {Name: "region", ReturnType: "Matcher", Parameters: []string{"Integer", "Integer"}}, {Name: "regionStart", ReturnType: "Integer"}, {Name: "regionEnd", ReturnType: "Integer"}, {Name: "hasAnchoringBounds", ReturnType: "Boolean"}, {Name: "hasTransparentBounds", ReturnType: "Boolean"}, {Name: "useAnchoringBounds", ReturnType: "Matcher", Parameters: []string{"Boolean"}}, {Name: "useTransparentBounds", ReturnType: "Matcher", Parameters: []string{"Boolean"}}, {Name: "usePattern", ReturnType: "Matcher", Parameters: []string{"Pattern"}}, {Name: "pattern", ReturnType: "Pattern"}, {Name: "quoteReplacement", ReturnType: "String", Parameters: []string{"String"}, Static: true}}},
	{Name: "ApexPages", Methods: []StandardMethodSpec{{Name: "currentPage", ReturnType: "PageReference", Static: true}, {Name: "addMessage", ReturnType: "void", Parameters: []string{"ApexPages.Message"}, Static: true}, {Name: "addMessages", ReturnType: "void", Parameters: []string{"Exception"}, Static: true}, {Name: "addMessages", ReturnType: "void", Parameters: []string{"Object"}, Static: true}, {Name: "getMessages", ReturnType: "List<ApexPages.Message>", Static: true}, {Name: "hasMessages", ReturnType: "Boolean", Static: true}, {Name: "hasMessages", ReturnType: "Boolean", Parameters: []string{"ApexPages.Severity"}, Static: true}}},
	{Name: "ApexPages.Message", Constructors: [][]string{{"ApexPages.Severity", "String"}, {"ApexPages.Severity", "String", "String"}, {"ApexPages.Severity", "String", "String", "String"}}, Methods: []StandardMethodSpec{{Name: "getSummary", ReturnType: "String"}, {Name: "getDetail", ReturnType: "String"}, {Name: "getComponentLabel", ReturnType: "String"}}},
	{Name: "ApexPages.StandardController", Constructors: [][]string{{"SObject"}}, Methods: []StandardMethodSpec{{Name: "view", ReturnType: "PageReference"}, {Name: "save", ReturnType: "PageReference"}, {Name: "delete", ReturnType: "PageReference"}, {Name: "edit", ReturnType: "PageReference"}, {Name: "cancel", ReturnType: "PageReference"}, {Name: "reset", ReturnType: "void"}, {Name: "getId", ReturnType: "Id"}, {Name: "addFields", ReturnType: "void", Parameters: []string{"List<String>"}}}},
	{Name: "ApexPages.StandardSetController", Constructors: [][]string{{"List<SObject>"}, {"Database.QueryLocator"}}, Methods: []StandardMethodSpec{{Name: "getRecords", ReturnType: "List<SObject>"}, {Name: "getResultSize", ReturnType: "Integer"}, {Name: "next", ReturnType: "void"}, {Name: "previous", ReturnType: "void"}, {Name: "getHasNext", ReturnType: "Boolean"}, {Name: "getHasPrevious", ReturnType: "Boolean"}}},
	{Name: "Database", Methods: []StandardMethodSpec{
		{Name: "setSavepoint", ReturnType: "Savepoint", Static: true},
		{Name: "rollback", ReturnType: "void", Parameters: []string{"Savepoint"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"String"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"String", "System.AccessLevel"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"List<Object>"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"List<Object>", "System.AccessLevel"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"Object"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"Object", "System.AccessLevel"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"List", "Object"}, Static: true},
		{Name: "getQueryLocatorWithBinds", ReturnType: "Database.QueryLocator", Parameters: []string{"String", "Map<String,Object>", "System.AccessLevel"}, Static: true},
		{Name: "getQueryLocatorWithBinds", ReturnType: "Database.QueryLocator", Parameters: []string{"String", "Map", "System.AccessLevel"}, Static: true},
		{Name: "countQuery", ReturnType: "Integer", Parameters: []string{"String", "System.AccessLevel"}, Static: true},
		{Name: "countQueryWithBinds", ReturnType: "Integer", Parameters: []string{"String", "Map<String,Object>", "System.AccessLevel"}, Static: true},
		{Name: "query", ReturnType: "List<SObject>", Parameters: []string{"String", "System.AccessLevel"}, Static: true},
		{Name: "queryWithBinds", ReturnType: "List<SObject>", Parameters: []string{"String", "Map<String,Object>", "System.AccessLevel"}, Static: true},
		{Name: "insert", ReturnType: "Database.SaveResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "insert", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "insert", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "AccessLevel"}, Static: true},
		{Name: "insert", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "Boolean", "AccessLevel"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "AccessLevel"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Boolean", "AccessLevel"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Database.DMLOptions"}, Static: true},
		{Name: "update", ReturnType: "Database.SaveResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "update", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "update", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "AccessLevel"}, Static: true},
		{Name: "update", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "Boolean", "AccessLevel"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "AccessLevel"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Boolean", "AccessLevel"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Database.DMLOptions"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"SObject", "AccessLevel"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"SObject", "Boolean", "AccessLevel"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<SObject>", "AccessLevel"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<SObject>", "Boolean", "AccessLevel"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"Id"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"Id", "Boolean"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"Id", "AccessLevel"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"Id", "Boolean", "AccessLevel"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Id>"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Id>", "Boolean"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Id>", "AccessLevel"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Id>", "Boolean", "AccessLevel"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "AccessLevel"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "Boolean", "AccessLevel"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "Schema.SObjectField"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "Schema.SObjectField", "AccessLevel"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "Schema.SObjectField", "Boolean"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "Schema.SObjectField", "Boolean", "AccessLevel"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "AccessLevel"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "Boolean", "AccessLevel"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "Schema.SObjectField"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "Schema.SObjectField", "AccessLevel"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "Schema.SObjectField", "Boolean"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "Schema.SObjectField", "Boolean", "AccessLevel"}, Static: true},
		{Name: "undelete", ReturnType: "Database.UndeleteResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "undelete", ReturnType: "Database.UndeleteResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "undelete", ReturnType: "Database.UndeleteResult", Parameters: []string{"SObject", "AccessLevel"}, Static: true},
		{Name: "undelete", ReturnType: "Database.UndeleteResult", Parameters: []string{"SObject", "Boolean", "AccessLevel"}, Static: true},
		{Name: "undelete", ReturnType: "Database.UndeleteResult", Parameters: []string{"Id"}, Static: true},
		{Name: "undelete", ReturnType: "Database.UndeleteResult", Parameters: []string{"Id", "Boolean"}, Static: true},
		{Name: "undelete", ReturnType: "Database.UndeleteResult", Parameters: []string{"Id", "AccessLevel"}, Static: true},
		{Name: "undelete", ReturnType: "Database.UndeleteResult", Parameters: []string{"Id", "Boolean", "AccessLevel"}, Static: true},
		{Name: "undelete", ReturnType: "List<Database.UndeleteResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "undelete", ReturnType: "List<Database.UndeleteResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "undelete", ReturnType: "List<Database.UndeleteResult>", Parameters: []string{"List<SObject>", "AccessLevel"}, Static: true},
		{Name: "undelete", ReturnType: "List<Database.UndeleteResult>", Parameters: []string{"List<SObject>", "Boolean", "AccessLevel"}, Static: true},
		{Name: "undelete", ReturnType: "List<Database.UndeleteResult>", Parameters: []string{"List<Id>"}, Static: true},
		{Name: "undelete", ReturnType: "List<Database.UndeleteResult>", Parameters: []string{"List<Id>", "Boolean"}, Static: true},
		{Name: "undelete", ReturnType: "List<Database.UndeleteResult>", Parameters: []string{"List<Id>", "AccessLevel"}, Static: true},
		{Name: "undelete", ReturnType: "List<Database.UndeleteResult>", Parameters: []string{"List<Id>", "Boolean", "AccessLevel"}, Static: true},
		{Name: "emptyRecycleBin", ReturnType: "Database.EmptyRecycleBinResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "emptyRecycleBin", ReturnType: "Database.EmptyRecycleBinResult", Parameters: []string{"Id"}, Static: true},
		{Name: "emptyRecycleBin", ReturnType: "List<Database.EmptyRecycleBinResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "emptyRecycleBin", ReturnType: "List<Database.EmptyRecycleBinResult>", Parameters: []string{"List<Id>"}, Static: true},
		{Name: "merge", ReturnType: "Database.MergeResult", Parameters: []string{"SObject", "SObject"}, Static: true},
		{Name: "merge", ReturnType: "Database.MergeResult", Parameters: []string{"SObject", "SObject", "Boolean"}, Static: true},
		{Name: "merge", ReturnType: "Database.MergeResult", Parameters: []string{"SObject", "SObject", "AccessLevel"}, Static: true},
		{Name: "merge", ReturnType: "Database.MergeResult", Parameters: []string{"SObject", "SObject", "Boolean", "AccessLevel"}, Static: true},
		{Name: "merge", ReturnType: "Database.MergeResult", Parameters: []string{"SObject", "Id"}, Static: true},
		{Name: "merge", ReturnType: "Database.MergeResult", Parameters: []string{"SObject", "Id", "Boolean"}, Static: true},
		{Name: "merge", ReturnType: "Database.MergeResult", Parameters: []string{"SObject", "Id", "AccessLevel"}, Static: true},
		{Name: "merge", ReturnType: "Database.MergeResult", Parameters: []string{"SObject", "Id", "Boolean", "AccessLevel"}, Static: true},
		{Name: "merge", ReturnType: "List<Database.MergeResult>", Parameters: []string{"SObject", "List<SObject>"}, Static: true},
		{Name: "merge", ReturnType: "List<Database.MergeResult>", Parameters: []string{"SObject", "List<SObject>", "Boolean"}, Static: true},
		{Name: "merge", ReturnType: "List<Database.MergeResult>", Parameters: []string{"SObject", "List<SObject>", "AccessLevel"}, Static: true},
		{Name: "merge", ReturnType: "List<Database.MergeResult>", Parameters: []string{"SObject", "List<SObject>", "Boolean", "AccessLevel"}, Static: true},
		{Name: "merge", ReturnType: "List<Database.MergeResult>", Parameters: []string{"SObject", "List<Id>"}, Static: true},
		{Name: "merge", ReturnType: "List<Database.MergeResult>", Parameters: []string{"SObject", "List<Id>", "Boolean"}, Static: true},
		{Name: "merge", ReturnType: "List<Database.MergeResult>", Parameters: []string{"SObject", "List<Id>", "AccessLevel"}, Static: true},
		{Name: "merge", ReturnType: "List<Database.MergeResult>", Parameters: []string{"SObject", "List<Id>", "Boolean", "AccessLevel"}, Static: true},
	}},
	{Name: "Date", Methods: []StandardMethodSpec{{Name: "today", ReturnType: "Date", Static: true}, {Name: "newInstance", ReturnType: "Date", Parameters: []string{"Integer", "Integer", "Integer"}, Static: true}, {Name: "daysInMonth", ReturnType: "Integer", Parameters: []string{"Integer", "Integer"}, Static: true}, {Name: "valueOf", ReturnType: "Date", Parameters: []string{"String"}, Static: true}, {Name: "valueOf", ReturnType: "Date", Parameters: []string{"Object"}, Static: true}, {Name: "addDays", ReturnType: "Date", Parameters: []string{"Integer"}}, {Name: "addMonths", ReturnType: "Date", Parameters: []string{"Integer"}}, {Name: "addYears", ReturnType: "Date", Parameters: []string{"Integer"}}, {Name: "daysBetween", ReturnType: "Integer", Parameters: []string{"Date"}}, {Name: "day", ReturnType: "Integer"}, {Name: "month", ReturnType: "Integer"}, {Name: "year", ReturnType: "Integer"}, {Name: "toStartOfMonth", ReturnType: "Date"}, {Name: "toEndOfMonth", ReturnType: "Date"}, {Name: "format", ReturnType: "String"}, {Name: "toString", ReturnType: "String"}}},
	{Name: "Schema", Methods: []StandardMethodSpec{{Name: "getGlobalDescribe", ReturnType: "Map<String,Schema.SObjectType>", Static: true}, {Name: "describeSObjects", ReturnType: "List<Schema.DescribeSObjectResult>", Parameters: []string{"List<String>"}, Static: true}, {Name: "describeTabs", ReturnType: "List<Schema.DescribeTabSetResult>", Static: true}}},
	{Name: "Schema.SObjectType", Methods: []StandardMethodSpec{{Name: "getDescribe", ReturnType: "Schema.DescribeSObjectResult"}, {Name: "getDescribe", ReturnType: "Schema.DescribeSObjectResult", Parameters: []string{"SObjectDescribeOptions"}}, {Name: "newSObject", ReturnType: "SObject"}}},
	{Name: "Schema.SObjectField", Methods: []StandardMethodSpec{{Name: "getDescribe", ReturnType: "Schema.DescribeFieldResult"}, {Name: "isAccessible", ReturnType: "Boolean"}, {Name: "isCreateable", ReturnType: "Boolean"}, {Name: "isUpdateable", ReturnType: "Boolean"}}, Properties: []StandardPropertySpec{{Name: "label", Type: "String"}, {Name: "name", Type: "String"}}},
	{Name: "Schema.DescribeSObjectResult", Methods: []StandardMethodSpec{{Name: "getName", ReturnType: "String"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getLabelPlural", ReturnType: "String"}, {Name: "getKeyPrefix", ReturnType: "String"}, {Name: "getFields", ReturnType: "Schema.SObjectTypeFields"}, {Name: "getFieldSets", ReturnType: "Schema.SObjectTypeFieldSets"}, {Name: "getRecordTypeInfos", ReturnType: "List<Schema.RecordTypeInfo>"}, {Name: "getRecordTypeInfosByName", ReturnType: "Map<String,Schema.RecordTypeInfo>"}, {Name: "getRecordTypeInfosByDeveloperName", ReturnType: "Map<String,Schema.RecordTypeInfo>"}, {Name: "getRecordTypeInfosById", ReturnType: "Map<Id,Schema.RecordTypeInfo>"}, {Name: "getChildRelationships", ReturnType: "List<Schema.ChildRelationship>"}, {Name: "getSObjectType", ReturnType: "Schema.SObjectType"}, {Name: "isAccessible", ReturnType: "Boolean"}, {Name: "isCreateable", ReturnType: "Boolean"}, {Name: "isUpdateable", ReturnType: "Boolean"}, {Name: "isDeletable", ReturnType: "Boolean"}, {Name: "isQueryable", ReturnType: "Boolean"}, {Name: "isSearchable", ReturnType: "Boolean"}}, Properties: []StandardPropertySpec{{Name: "fields", Type: "Schema.SObjectTypeFields"}, {Name: "fieldSets", Type: "Schema.SObjectTypeFieldSets"}}},
	{Name: "Schema.SObjectTypeFields", Methods: []StandardMethodSpec{{Name: "get", ReturnType: "Schema.SObjectField", Parameters: []string{"String"}}, {Name: "getMap", ReturnType: "Map<String,Schema.SObjectField>"}}},
	{Name: "Schema.SObjectTypeFieldSets", Methods: []StandardMethodSpec{{Name: "get", ReturnType: "Schema.FieldSet", Parameters: []string{"String"}}, {Name: "getMap", ReturnType: "Map<String,Schema.FieldSet>"}}},
	{Name: "Schema.FieldSetMap", Methods: []StandardMethodSpec{{Name: "get", ReturnType: "Schema.FieldSet", Parameters: []string{"String"}}, {Name: "getMap", ReturnType: "Map<String,Schema.FieldSet>"}}},
	{Name: "Schema.FieldSet", Methods: []StandardMethodSpec{{Name: "getDescription", ReturnType: "String"}, {Name: "getFields", ReturnType: "List<Schema.FieldSetMember>"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getName", ReturnType: "String"}, {Name: "getNamespace", ReturnType: "String"}, {Name: "getNameSpace", ReturnType: "String"}, {Name: "getSObjectType", ReturnType: "Schema.SObjectType"}}},
	{Name: "Schema.FieldSetMember", Methods: []StandardMethodSpec{{Name: "getFieldPath", ReturnType: "String"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getRequired", ReturnType: "Boolean"}, {Name: "getDBRequired", ReturnType: "Boolean"}, {Name: "getType", ReturnType: "Schema.DisplayType"}, {Name: "getSObjectField", ReturnType: "Schema.SObjectField"}}},
	{Name: "Schema.DescribeTabSetResult", Methods: []StandardMethodSpec{{Name: "getDescription", ReturnType: "String"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getLogoUrl", ReturnType: "String"}, {Name: "getName", ReturnType: "String"}, {Name: "getNamespace", ReturnType: "String"}, {Name: "getTabSetId", ReturnType: "String"}, {Name: "getTabs", ReturnType: "List<Schema.DescribeTabResult>"}, {Name: "isSelected", ReturnType: "Boolean"}}, Properties: []StandardPropertySpec{{Name: "name", Type: "String"}}},
	{Name: "Schema.DescribeTabResult", Methods: []StandardMethodSpec{{Name: "getColors", ReturnType: "List<Schema.DescribeColorResult>"}, {Name: "getIconUrl", ReturnType: "String"}, {Name: "getIcons", ReturnType: "List<Schema.DescribeIconResult>"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getMiniIconUrl", ReturnType: "String"}, {Name: "getMobileUrl", ReturnType: "String"}, {Name: "getName", ReturnType: "String"}, {Name: "getSObjectName", ReturnType: "String"}, {Name: "getTabEnumOrId", ReturnType: "String"}, {Name: "getUrl", ReturnType: "String"}, {Name: "isCustom", ReturnType: "Boolean"}}},
	{Name: "Schema.DescribeColorResult", Methods: []StandardMethodSpec{{Name: "getColor", ReturnType: "String"}, {Name: "getContext", ReturnType: "String"}, {Name: "getTheme", ReturnType: "String"}}},
	{Name: "Schema.DescribeFieldResult", Methods: []StandardMethodSpec{{Name: "getName", ReturnType: "String"}, {Name: "getType", ReturnType: "Schema.DisplayType"}, {Name: "getSoapType", ReturnType: "Schema.SoapType"}, {Name: "getSObjectType", ReturnType: "Schema.SObjectType"}, {Name: "getCompoundFieldName", ReturnType: "String"}, {Name: "isAccessible", ReturnType: "Boolean"}, {Name: "isCreateable", ReturnType: "Boolean"}, {Name: "isUpdateable", ReturnType: "Boolean"}, {Name: "isCalculated", ReturnType: "Boolean"}}, Properties: []StandardPropertySpec{{Name: "compoundFieldName", Type: "String"}}},
	{Name: "Schema.DisplayType", Kind: apexast.DeclarationEnum, Properties: standardEnumProperties("Schema.DisplayType", "STRING", "BOOLEAN", "DOUBLE", "INTEGER", "PERCENT", "CURRENCY", "DATE", "DATETIME", "TIME", "PICKLIST", "MULTIPICKLIST", "DATACATEGORYGROUPREFERENCE", "BASE64", "ID", "REFERENCE", "TEXTAREA", "PHONE", "COMBOBOX", "URL", "EMAIL", "ANYTYPE", "LOCATION", "ENCRYPTEDSTRING", "COMPLEXVALUE", "ADDRESS", "SOBJECT", "LONG", "JSON", "FLOATARRAY", "TEXTARRAY")},
	{Name: "Schema.FieldDescribeOptions", Kind: apexast.DeclarationEnum, Properties: standardEnumProperties("Schema.FieldDescribeOptions", "DEFAULT", "FULL_DESCRIBE")},
	{Name: "Schema.SObjectDescribeOptions", Kind: apexast.DeclarationEnum, Properties: standardEnumProperties("Schema.SObjectDescribeOptions", "DEFAULT", "FULL", "DEFERRED")},
	{Name: "SObjectDescribeOptions", Kind: apexast.DeclarationEnum, Properties: standardEnumProperties("SObjectDescribeOptions", "DEFAULT", "FULL", "DEFERRED")},
	{Name: "Schema.ChildRelationship", Methods: []StandardMethodSpec{{Name: "getRelationshipName", ReturnType: "String"}, {Name: "getChildSObject", ReturnType: "Schema.SObjectType"}, {Name: "getField", ReturnType: "Schema.SObjectField"}, {Name: "isCascadeDelete", ReturnType: "Boolean"}}},
	{Name: "Security", Methods: []StandardMethodSpec{{Name: "stripInaccessible", ReturnType: "SObjectAccessDecision", Parameters: []string{"AccessType", "List<SObject>"}, Static: true}, {Name: "stripInaccessible", ReturnType: "SObjectAccessDecision", Parameters: []string{"AccessType", "List<SObject>", "Boolean"}, Static: true}, {Name: "stripInaccessible", ReturnType: "SObjectAccessDecision", Parameters: []string{"AccessType", "List<SObject>", "Boolean", "Id"}, Static: true}}},
	{Name: "SObjectAccessDecision", Constructors: [][]string{{}}, Methods: []StandardMethodSpec{{Name: "clone", ReturnType: "Object"}, {Name: "getModifiedIndexes", ReturnType: "Set<Integer>"}, {Name: "getRecords", ReturnType: "List<SObject>"}, {Name: "getRemovedFields", ReturnType: "Map<String,Set<String>>"}}},
	{Name: "Address", Methods: []StandardMethodSpec{{Name: "getStreet", ReturnType: "String"}, {Name: "getCity", ReturnType: "String"}, {Name: "getState", ReturnType: "String"}, {Name: "getStateCode", ReturnType: "String"}, {Name: "getPostalCode", ReturnType: "String"}, {Name: "getCountry", ReturnType: "String"}, {Name: "getCountryCode", ReturnType: "String"}, {Name: "getLatitude", ReturnType: "Double"}, {Name: "getLongitude", ReturnType: "Double"}, {Name: "getGeocodeAccuracy", ReturnType: "String"}}},
	{Name: "Metadata.MetadataType", Kind: apexast.DeclarationEnum, Properties: []StandardPropertySpec{{Name: "CustomMetadata", Type: "Metadata.MetadataType", Static: true}}},
	{Name: "Metadata.Operations", Methods: []StandardMethodSpec{{Name: "retrieve", ReturnType: "List<Metadata.CustomMetadata>", Parameters: []string{"Metadata.MetadataType", "List<String>"}, Static: true}, {Name: "enqueueDeployment", ReturnType: "Id", Parameters: []string{"Metadata.DeployContainer", "Metadata.DeployCallback"}, Static: true}, {Name: "checkDeployStatus", ReturnType: "Metadata.DeployResult", Parameters: []string{"Id"}, Static: true}}},
	{Name: "Messaging.SingleEmailMessage", SuperClass: "Messaging.Email", Methods: []StandardMethodSpec{{Name: "setToAddresses", ReturnType: "void", Parameters: []string{"List<String>"}}, {Name: "setSubject", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setPlainTextBody", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setWhatId", ReturnType: "void", Parameters: []string{"Id"}}, {Name: "getCustomHeaders", ReturnType: "Map<String,String>"}, {Name: "setCustomHeaders", ReturnType: "void", Parameters: []string{"Map<String,String>"}}}, Properties: []StandardPropertySpec{{Name: "customHeaders", Type: "Map<String,String>"}}},
	{Name: "Messaging.AttachmentRetrievalOption", Kind: apexast.DeclarationEnum, Properties: standardEnumProperties("Messaging.AttachmentRetrievalOption", "METADATA_ONLY", "METADATA_WITH_BODY", "NONE")},
	{Name: "Messaging", Methods: []StandardMethodSpec{
		{Name: "sendEmail", ReturnType: "List<Messaging.SendEmailResult>", Parameters: []string{"List<Messaging.Email>"}, Static: true},
		{Name: "sendEmail", ReturnType: "List<Messaging.SendEmailResult>", Parameters: []string{"List<Messaging.Email>", "Boolean"}, Static: true},
		{Name: "renderStoredEmailTemplate", ReturnType: "Messaging.SingleEmailMessage", Parameters: []string{"String", "String", "String", "Messaging.AttachmentRetrievalOption"}, Static: true},
		{Name: "renderStoredEmailTemplate", ReturnType: "Messaging.SingleEmailMessage", Parameters: []string{"String", "String", "String", "Messaging.AttachmentRetrievalOption", "Boolean"}, Static: true},
	}},
	{Name: "SObject", Methods: []StandardMethodSpec{{Name: "setOptions", ReturnType: "void", Parameters: []string{"Database.DMLOptions"}}, {Name: "getOptions", ReturnType: "Database.DMLOptions"}}},
	{Name: "ConnectApi.Organization", Methods: []StandardMethodSpec{{Name: "getSettings", ReturnType: "ConnectApi.OrganizationSettings", Static: true}}},
	{Name: "ConnectApi.ChatterFeeds", Methods: []StandardMethodSpec{{Name: "postFeedElement", ReturnType: "ConnectApi.FeedElement", Parameters: []string{"String", "ConnectApi.FeedElementInput"}, Static: true}, {Name: "postFeedElement", ReturnType: "ConnectApi.FeedElement", Parameters: []string{"String", "String", "ConnectApi.FeedElementType", "String"}, Static: true}}},
	{Name: "ConnectApi.UserProfiles", Methods: []StandardMethodSpec{{Name: "getUserProfile", ReturnType: "ConnectApi.UserProfile", Parameters: []string{"String", "String"}, Static: true}}},
	{Name: "Auth.AuthConfiguration", Constructors: [][]string{{}, {"String", "String"}}, Methods: []StandardMethodSpec{{Name: "getAuthConfig", ReturnType: "Auth.AuthConfiguration", Static: true}, {Name: "getAuthProviders", ReturnType: "List<AuthProvider>"}, {Name: "getFooterText", ReturnType: "String"}, {Name: "getBackgroundColor", ReturnType: "String"}, {Name: "getStartUrl", ReturnType: "String"}, {Name: "isCommunityUsingSiteAsContainer", ReturnType: "Boolean"},
		{Name: "getAllowInternalUserLoginEnabled", ReturnType: "Boolean"}, {Name: "getAuthConfigProviders", ReturnType: "List<Auth.AuthConfig>"}, {Name: "getAuthProviderSsoDomainUrl", ReturnType: "String", Parameters: []string{"String", "String", "String"}, Static: true}, {Name: "getAuthProviderSsoUrl", ReturnType: "String", Parameters: []string{"String", "String", "String"}, Static: true}, {Name: "getCertificateLoginEnabled", ReturnType: "Boolean", Parameters: []string{"String"}}, {Name: "getCertificateLoginUrl", ReturnType: "String", Parameters: []string{"String", "String"}, Static: true}, {Name: "getDefaultProfileForRegistration", ReturnType: "String"}, {Name: "getForgotPasswordUrl", ReturnType: "String"}, {Name: "getHeadlessForgotPasswordEnabled", ReturnType: "Boolean"}, {Name: "getHeadlessFrgtPswEnabled", ReturnType: "Boolean"}, {Name: "getHeadlessPasswordlessLoginEnabled", ReturnType: "Boolean"}, {Name: "getHeadlessRegistrationEnabled", ReturnType: "Boolean"}, {Name: "getLogoUrl", ReturnType: "String"}, {Name: "getRightFrameUrl", ReturnType: "String"}, {Name: "getSamlProviders", ReturnType: "List<Auth.AuthConfig>"}, {Name: "getSamlSsoUrl", ReturnType: "String", Parameters: []string{"String", "String", "String"}, Static: true}, {Name: "getSelfRegistrationEnabled", ReturnType: "Boolean"}, {Name: "getSelfRegistrationUrl", ReturnType: "String"}, {Name: "getUsernamePasswordEnabled", ReturnType: "Boolean"}}},
	{Name: "Auth.AuthProviderCallbackState", ConstructorSpecs: []StandardConstructorSpec{{Parameters: []StandardParameterSpec{{Name: "headers", Type: "Map<String,String>"}, {Name: "body", Type: "String"}, {Name: "queryParameters", Type: "Map<String,String>"}}}}, Properties: []StandardPropertySpec{{Name: "headers", Type: "Map<String,String>"}, {Name: "body", Type: "String"}, {Name: "queryParameters", Type: "Map<String,String>"}}},
	{Name: "Auth.AuthProviderPlugin", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "initiate", ReturnType: "PageReference", Parameters: []string{"Map<String,String>", "String"}}, {Name: "handleCallback", ReturnType: "Auth.AuthProviderTokenResponse", Parameters: []string{"Map<String,String>", "Auth.AuthProviderCallbackState"}}, {Name: "getUserInfo", ReturnType: "Auth.UserData", Parameters: []string{"Map<String,String>", "Auth.AuthProviderTokenResponse"}}, {Name: "getCustomMetadataType", ReturnType: "String"}}},
	{Name: "Auth.AuthProviderPluginClass", Methods: []StandardMethodSpec{{Name: "initiate", ReturnType: "PageReference", Parameters: []string{"Map<String,String>", "String"}, Modifiers: []string{"virtual"}}, {Name: "handleCallback", ReturnType: "Auth.AuthProviderTokenResponse", Parameters: []string{"Map<String,String>", "Auth.AuthProviderCallbackState"}, Modifiers: []string{"virtual"}}, {Name: "getUserInfo", ReturnType: "Auth.UserData", Parameters: []string{"Map<String,String>", "Auth.AuthProviderTokenResponse"}, Modifiers: []string{"virtual"}}, {Name: "getCustomMetadataType", ReturnType: "String", Modifiers: []string{"virtual"}}, {Name: "refresh", ReturnType: "Auth.OAuthRefreshResult", Parameters: []string{"Map<String,String>", "String"}, Modifiers: []string{"virtual"}}}},
	{Name: "Auth.AuthProviderTokenResponse", Constructors: [][]string{{"String", "String", "String", "String"}}, Properties: []StandardPropertySpec{{Name: "provider", Type: "String"}, {Name: "oauthToken", Type: "String"}, {Name: "oauthSecretOrRefreshToken", Type: "String"}, {Name: "state", Type: "String"}, {Name: "idToken", Type: "String"}}},
	{Name: "Auth.AuthToken", Methods: []StandardMethodSpec{{Name: "getAccessToken", ReturnType: "String", Parameters: []string{"String", "String"}, Static: true}, {Name: "getAccessTokenMap", ReturnType: "Map<String,String>", Parameters: []string{"String", "String"}, Static: true}, {Name: "refreshAccessToken", ReturnType: "Map<String,String>", Parameters: []string{"String", "String", "String"}, Static: true}, {Name: "revokeAccess", ReturnType: "Boolean", Parameters: []string{"String", "String", "String", "String"}, Static: true}}},
	{Name: "Auth.ConfigurableSelfRegHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "createUser", ReturnType: "User", Parameters: []string{"Id", "Id", "Map<Schema.SObjectField,String>", "String"}}}},
	{Name: "Auth.ConfirmUserRegistrationHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "confirmUser", ReturnType: "void", Parameters: []string{"Id", "Id", "Id", "Auth.UserData"}}}},
	{Name: "Auth.CommunitiesUtil", Methods: []StandardMethodSpec{{Name: "getLogoutUrl", ReturnType: "String", Static: true}, {Name: "getUserDisplayName", ReturnType: "String", Static: true}, {Name: "isGuestUser", ReturnType: "Boolean", Static: true}, {Name: "isInternalUser", ReturnType: "Boolean", Static: true}}},
	{Name: "Auth.ConnectedAppPlugin", Methods: []StandardMethodSpec{{Name: "authorize", ReturnType: "Boolean", Parameters: []string{"Id", "Id", "Boolean"}}, {Name: "authorize", ReturnType: "Boolean", Parameters: []string{"Id", "Id", "Boolean", "Auth.InvocationContext"}}, {Name: "customAttributes", ReturnType: "Map<String,String>", Parameters: []string{"Id", "Id", "Map<String,String>"}}, {Name: "customAttributes", ReturnType: "Map<String,String>", Parameters: []string{"Id", "Id", "Map<String,String>", "Auth.InvocationContext"}}, {Name: "modifySAMLResponse", ReturnType: "void", Parameters: []string{"Map<String,String>", "Id", "dom.XmlNode"}}, {Name: "refresh", ReturnType: "Boolean", Parameters: []string{"Id", "Id"}}, {Name: "refresh", ReturnType: "Boolean", Parameters: []string{"Id", "Id", "Auth.InvocationContext"}}}},
	{Name: "Auth.CustomOneTimePasswordDeliveryHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "sendOneTimePassword", ReturnType: "Auth.CustomOneTimePasswordDeliveryResult", Parameters: []string{"Id", "String", "String", "String", "Id", "String"}}}},
	{Name: "Auth.ExternalClientAppOauthHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "authorize", ReturnType: "Boolean", Parameters: []string{"Id", "Id", "Boolean", "Auth.InvocationContext"}}, {Name: "customAttributes", ReturnType: "Map<String,String>", Parameters: []string{"Id", "Id", "Map<String,String>", "Auth.InvocationContext"}}, {Name: "refresh", ReturnType: "Boolean", Parameters: []string{"Id", "Id", "Auth.InvocationContext"}}}},
	{Name: "Auth.GeneratedUserData", Constructors: [][]string{{"String", "String", "String", "String", "String", "String", "String", "String", "String"}}, Properties: []StandardPropertySpec{{Name: "firstName", Type: "String"}, {Name: "lastName", Type: "String"}, {Name: "email", Type: "String"}, {Name: "username", Type: "String"}, {Name: "alias", Type: "String"}, {Name: "localesIdKey", Type: "String"}, {Name: "timeZoneSidKey", Type: "String"}, {Name: "emailEncodingKey", Type: "String"}, {Name: "languageLocaleKey", Type: "String"}}},
	{Name: "Auth.HeadlessSelfRegistrationHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "createUser", ReturnType: "User", Parameters: []string{"Id", "Auth.UserData", "String", "String", "String"}}}},
	{Name: "Auth.HeadlessUserDiscoveryHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "discoverUserFromLoginHint", ReturnType: "Auth.HeadlessUserDiscoveryResponse", Parameters: []string{"Id", "String", "Auth.VerificationAction", "String", "Map<String,String>"}}}},
	{Name: "Auth.HeadlessUserDiscoveryResponse", Constructors: [][]string{{"Set<Id>", "String"}}, Properties: []StandardPropertySpec{{Name: "userIds", Type: "Set<Id>"}, {Name: "customErrorMessage", Type: "String"}}},
	{Name: "Auth.HttpCalloutMockUtil", Methods: []StandardMethodSpec{{Name: "setHttpMock", ReturnType: "void", Parameters: []string{"HttpCalloutMock"}, Static: true}}},
	{Name: "Auth.IntegratingAppType", Kind: apexast.DeclarationEnum},
	{Name: "Auth.JWS", Constructors: [][]string{{"Auth.JWT", "String"}}, Methods: []StandardMethodSpec{{Name: "getCompactSerialization", ReturnType: "String"}, {Name: "clone", ReturnType: "Object"}}},
	{Name: "Auth.JWT", Constructors: [][]string{{}}, Methods: []StandardMethodSpec{{Name: "setIss", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setSub", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setAud", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setNbfClockSkew", ReturnType: "void", Parameters: []string{"Integer"}}, {Name: "setValidityLength", ReturnType: "void", Parameters: []string{"Integer"}}, {Name: "setAdditionalClaims", ReturnType: "void", Parameters: []string{"Map<String,Object>"}}, {Name: "getIss", ReturnType: "String"}, {Name: "getSub", ReturnType: "String"}, {Name: "getAud", ReturnType: "String"}, {Name: "getNbfClockSkew", ReturnType: "Integer"}, {Name: "getValidityLength", ReturnType: "Integer"}, {Name: "getAdditionalClaims", ReturnType: "Map<String,Object>"}, {Name: "toJSONString", ReturnType: "String"}, {Name: "clone", ReturnType: "Object"}}},
	{Name: "Auth.JWTBearerTokenExchange", Constructors: [][]string{{}, {"String", "Auth.JWS"}}, Methods: []StandardMethodSpec{{Name: "getTokenEndpoint", ReturnType: "String"}, {Name: "setTokenEndpoint", ReturnType: "void", Parameters: []string{"String"}}, {Name: "getJWS", ReturnType: "Auth.JWS"}, {Name: "setJWS", ReturnType: "void", Parameters: []string{"Auth.JWS"}}, {Name: "getGrantType", ReturnType: "String"}, {Name: "setGrantType", ReturnType: "void", Parameters: []string{"String"}}, {Name: "getAccessToken", ReturnType: "String"}, {Name: "getHttpResponse", ReturnType: "HttpResponse"}, {Name: "clone", ReturnType: "Object"}}},
	{Name: "Auth.JWTUtil", Methods: []StandardMethodSpec{{Name: "parseJWTFromStringWithoutValidation", ReturnType: "Auth.JWT", Parameters: []string{"String"}, Static: true}, {Name: "validateJWTWithCert", ReturnType: "Auth.JWT", Parameters: []string{"String", "String"}, Static: true}, {Name: "validateJWTWithKey", ReturnType: "Auth.JWT", Parameters: []string{"String", "String"}, Static: true}, {Name: "validateJWTWithKeysEndpoint", ReturnType: "Auth.JWT", Parameters: []string{"String", "String", "Boolean"}, Static: true}}},
	{Name: "Auth.JsonValueOutput", Constructors: [][]string{{"String", "Boolean", "Integer", "Double", "String", "String"}}, Properties: []StandardPropertySpec{{Name: "stringValue", Type: "String"}, {Name: "booleanValue", Type: "Boolean"}, {Name: "integerValue", Type: "Integer"}, {Name: "doubleValue", Type: "Double"}, {Name: "jsonStringValue", Type: "String"}, {Name: "jsonArrayValue", Type: "String"}}},
	{Name: "Auth.LoginDiscoveryHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "login", ReturnType: "PageReference", ParameterSpecs: []StandardParameterSpec{{Type: "String"}, {Type: "String"}, {Name: "requestAttributes", Type: "Map<String,String>"}}}}},
	{Name: "Auth.LoginDiscoveryMethod", Kind: apexast.DeclarationEnum},
	{Name: "Auth.MyDomainLoginDiscoveryHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "login", ReturnType: "PageReference", Parameters: []string{"String", "String", "Map<String,String>"}}}},
	{Name: "Auth.OAuth2TokenExchangeType", Kind: apexast.DeclarationEnum},
	{Name: "Auth.OAuthRefreshResult", Constructors: [][]string{{"String", "String"}, {"String", "String", "String"}}, Properties: []StandardPropertySpec{{Name: "accessToken", Type: "String"}, {Name: "refreshToken", Type: "String"}, {Name: "error", Type: "String"}}},
	{Name: "Auth.Oauth2TokenExchangeHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "getUserForTokenSubject", ReturnType: "User", Parameters: []string{"Id", "Auth.TokenValidationResult", "Boolean", "String", "Auth.IntegratingAppType"}}, {Name: "validateIncomingToken", ReturnType: "Auth.TokenValidationResult", Parameters: []string{"String", "Auth.IntegratingAppType", "String", "Auth.OAuth2TokenExchangeType"}}}},
	{Name: "Auth.OauthToken", Methods: []StandardMethodSpec{{Name: "revokeToken", ReturnType: "void", Parameters: []string{"Auth.OauthTokenType", "String"}, Static: true}}},
	{Name: "Auth.OauthTokenType", Kind: apexast.DeclarationEnum},
	{Name: "Auth.RegistrationHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "createUser", ReturnType: "User", Parameters: []string{"Id", "Auth.UserData"}}, {Name: "updateUser", ReturnType: "void", Parameters: []string{"Id", "Id", "Auth.UserData"}}}, Properties: []StandardPropertySpec{{Name: "User", Type: "User", Static: true}}},
	{Name: "Auth.SamlJitHandler", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "createUser", ReturnType: "User", Parameters: []string{"Id", "Id", "Id", "String", "Map<String,String>", "String"}}, {Name: "updateUser", ReturnType: "void", Parameters: []string{"Id", "Id", "Id", "Id", "String", "Map<String,String>", "String"}}}},
	{Name: "Auth.SessionLevel", Kind: apexast.DeclarationEnum},
	{Name: "Auth.SessionManagement", Methods: []StandardMethodSpec{{Name: "getCurrentSession", ReturnType: "Map<String,String>", Static: true}, {Name: "finishLoginDiscovery", ReturnType: "void", Parameters: []string{"Auth.LoginDiscoveryMethod", "Id"}, Static: true}, {Name: "finishLoginFlow", ReturnType: "PageReference", Static: true}, {Name: "finishLoginFlow", ReturnType: "PageReference", Parameters: []string{"String"}, Static: true}, {Name: "generateVerificationUrl", ReturnType: "String", Parameters: []string{"Auth.VerificationPolicy", "String", "String"}, Static: true}, {Name: "getLightningLoginEligibility", ReturnType: "Auth.LightningLoginEligibility", Parameters: []string{"Id"}, Static: true}, {Name: "getQrCode", ReturnType: "Map<String,String>", Static: true}, {Name: "getRequiredSessionLevelForProfile", ReturnType: "Auth.SessionLevel", Parameters: []string{"String"}, Static: true}, {Name: "ignoreForConcurrentSessionLimit", ReturnType: "void", Parameters: []string{"Object"}, Static: true}, {Name: "inOrgNetworkRange", ReturnType: "Boolean", Parameters: []string{"String"}, Static: true}, {Name: "isIpAllowedForProfile", ReturnType: "Boolean", Parameters: []string{"String", "String"}, Static: true}, {Name: "setSessionLevel", ReturnType: "void", Parameters: []string{"Auth.SessionLevel"}, Static: true}, {Name: "validateTotpTokenForKey", ReturnType: "Boolean", Parameters: []string{"String", "String"}, Static: true}, {Name: "validateTotpTokenForKey", ReturnType: "Boolean", Parameters: []string{"String", "String", "String"}, Static: true}, {Name: "validateTotpTokenForUser", ReturnType: "Boolean", Parameters: []string{"String"}, Static: true}, {Name: "validateTotpTokenForUser", ReturnType: "Boolean", Parameters: []string{"String", "String"}, Static: true}, {Name: "verifyDeviceFlow", ReturnType: "Boolean", Parameters: []string{"String", "String"}, Static: true}}},
	{Name: "Auth.TokenValidationResult", Constructors: [][]string{{"Boolean"}, {"Boolean", "Object", "Auth.UserData", "String", "Auth.OAuth2TokenExchangeType", "String"}}, Methods: []StandardMethodSpec{{Name: "getCustomErrorMessage", ReturnType: "String"}, {Name: "getData", ReturnType: "Object"}, {Name: "getToken", ReturnType: "String"}, {Name: "getTokenType", ReturnType: "Auth.OAuth2TokenExchangeType"}, {Name: "getUserData", ReturnType: "Auth.UserData"}}, Properties: []StandardPropertySpec{{Name: "isValid", Type: "Boolean"}, {Name: "data", Type: "Object"}, {Name: "userData", Type: "Auth.UserData"}, {Name: "customErrorMsg", Type: "String"}, {Name: "tokenType", Type: "Auth.OAuth2TokenExchangeType"}, {Name: "token", Type: "String"}}},
	{Name: "Auth.UserData", Constructors: [][]string{{}, {"String", "String", "String", "String", "String", "String", "String", "String", "String", "String", "Map<String,String>"}, {"String", "String", "String", "String", "String", "String", "String", "String", "String", "String", "Map<String,String>", "String", "String"}}, Properties: []StandardPropertySpec{{Name: "identifier", Type: "String"}, {Name: "firstName", Type: "String"}, {Name: "lastName", Type: "String"}, {Name: "fullName", Type: "String"}, {Name: "email", Type: "String"}, {Name: "link", Type: "String"}, {Name: "username", Type: "String"}, {Name: "locale", Type: "String"}, {Name: "provider", Type: "String"}, {Name: "siteLoginUrl", Type: "String"}, {Name: "attributeMap", Type: "Map<String,String>"}, {Name: "userInfoJSONString", Type: "String"}, {Name: "idToken", Type: "String"}, {Name: "idTokenJSONString", Type: "String"}}},
	{Name: "Auth.VerificationAction", Kind: apexast.DeclarationEnum},
	{Name: "Auth.VerificationMethod", Kind: apexast.DeclarationEnum},
	{Name: "Auth.VerificationPolicy", Kind: apexast.DeclarationEnum},
	{Name: "Auth.VerificationResult", Constructors: [][]string{{}, {"PageReference", "Boolean", "String"}}, Methods: []StandardMethodSpec{{Name: "clone", ReturnType: "Object"}}, Properties: []StandardPropertySpec{{Name: "redirect", Type: "PageReference"}, {Name: "success", Type: "Boolean"}, {Name: "message", Type: "String"}}},
	{Name: "Http", Methods: []StandardMethodSpec{{Name: "send", ReturnType: "HttpResponse", Parameters: []string{"HttpRequest"}}}},
	{Name: "HttpRequest", Constructors: [][]string{{}}, Methods: []StandardMethodSpec{{Name: "setEndpoint", ReturnType: "void", Parameters: []string{"String"}}, {Name: "getEndpoint", ReturnType: "String"}, {Name: "setMethod", ReturnType: "void", Parameters: []string{"String"}}, {Name: "getMethod", ReturnType: "String"}, {Name: "setHeader", ReturnType: "void", Parameters: []string{"String", "String"}}, {Name: "getHeader", ReturnType: "String", Parameters: []string{"String"}}, {Name: "setBody", ReturnType: "void", Parameters: []string{"String"}}, {Name: "getBody", ReturnType: "String"}, {Name: "setBodyDocument", ReturnType: "void", Parameters: []string{"Dom.Document"}}, {Name: "setTimeout", ReturnType: "void", Parameters: []string{"Integer"}}}},
	{Name: "HttpResponse", Methods: []StandardMethodSpec{{Name: "getStatusCode", ReturnType: "Integer"}, {Name: "getStatus", ReturnType: "String"}, {Name: "getBody", ReturnType: "String"}}},
	{Name: "WebServiceCallout", Methods: []StandardMethodSpec{{Name: "invoke", ReturnType: "void", Parameters: []string{"Object", "Object", "Map<String,Object>", "List<String>"}, Static: true}}},
	{Name: "WebServiceMock", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "doInvoke", ReturnType: "void", Parameters: []string{"Object", "Object", "Map<String,Object>", "String", "String", "String", "String", "String", "String"}}}},
	{Name: "UserInfo", Methods: []StandardMethodSpec{{Name: "getUserId", ReturnType: "Id", Static: true}, {Name: "getProfileId", ReturnType: "Id", Static: true}, {Name: "getUserName", ReturnType: "String", Static: true}, {Name: "getName", ReturnType: "String", Static: true}, {Name: "getFirstName", ReturnType: "String", Static: true}, {Name: "getLastName", ReturnType: "String", Static: true}, {Name: "getUserEmail", ReturnType: "String", Static: true}, {Name: "getOrganizationId", ReturnType: "Id", Static: true}, {Name: "getUserType", ReturnType: "String", Static: true}, {Name: "getSessionId", ReturnType: "String", Static: true}, {Name: "getLocale", ReturnType: "String", Static: true}, {Name: "getLanguage", ReturnType: "String", Static: true}, {Name: "getTimeZone", ReturnType: "TimeZone", Static: true}, {Name: "isMultiCurrencyOrganization", ReturnType: "Boolean", Static: true}}},
	{Name: "UUID", Methods: []StandardMethodSpec{{Name: "randomUUID", ReturnType: "String", Static: true}}},
	{Name: "Object", Methods: []StandardMethodSpec{{Name: "equals", ReturnType: "Boolean", Parameters: []string{"Object"}}, {Name: "hashCode", ReturnType: "Integer"}, {Name: "toString", ReturnType: "String"}}},
	{Name: "Enum"},
	{Name: "List", Constructors: [][]string{{}, {"List"}}, Methods: []StandardMethodSpec{{Name: "addToRelationship", ReturnType: "void", Parameters: []string{"List<SObject>"}}, {Name: "addToRelationship", ReturnType: "void", Parameters: []string{"SObject"}}, {Name: "equals", ReturnType: "Boolean", Parameters: []string{"List"}}, {Name: "getAddedToRelationship", ReturnType: "List<SObject>"}, {Name: "getMarkedForDeletion", ReturnType: "List<SObject>"}, {Name: "markForDelete", ReturnType: "void", Parameters: []string{"List<SObject>"}}, {Name: "markForDelete", ReturnType: "void", Parameters: []string{"SObject"}}}},
	{Name: "Map", Constructors: [][]string{{}, {"Map"}, {"List<SObject>"}}, Methods: []StandardMethodSpec{{Name: "equals", ReturnType: "Boolean", Parameters: []string{"Map"}}, {Name: "remove", ReturnType: "Object", Parameters: []string{"Object"}}}},
	{Name: "Set", Constructors: [][]string{{}, {"Set"}}, Methods: []StandardMethodSpec{{Name: "addAll", ReturnType: "Boolean", Parameters: []string{"List<Object>"}}, {Name: "containsAll", ReturnType: "Boolean", Parameters: []string{"List<Object>"}}, {Name: "equals", ReturnType: "Boolean", Parameters: []string{"Set<Object>"}}, {Name: "removeAll", ReturnType: "Boolean", Parameters: []string{"List<Object>"}}, {Name: "retainAll", ReturnType: "Boolean", Parameters: []string{"List<Object>"}}}},
	{Name: "Callable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "call", ReturnType: "Object", Parameters: []string{"String", "Map<String,Object>"}}}},
	{Name: "Comparator", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "compare", ReturnType: "Integer", Parameters: []string{"Object", "Object"}}}},
	{Name: "Comparable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "compareTo", ReturnType: "Integer", Parameters: []string{"Object"}}}},
	{Name: "Database.Batchable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "start", ReturnType: "Iterable", Parameters: []string{"Database.BatchableContext"}}, {Name: "execute", ReturnType: "void", Parameters: []string{"Database.BatchableContext", "List<Object>"}}, {Name: "finish", ReturnType: "void", Parameters: []string{"Database.BatchableContext"}}}},
	{Name: "Database.BatchableContext", Kind: apexast.DeclarationInterface, ReplaceConstructors: true, Methods: []StandardMethodSpec{{Name: "getChildJobId", ReturnType: "Id"}, {Name: "getJobId", ReturnType: "Id"}}},
	{Name: "Database.Stateful", Kind: apexast.DeclarationInterface},
	{Name: "HttpCalloutMock", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "respond", ReturnType: "HttpResponse", Parameters: []string{"HttpRequest"}}}},
	{Name: "Iterable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "iterator", ReturnType: "Iterator"}}},
	{Name: "Iterator", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "hasNext", ReturnType: "Boolean"}, {Name: "next", ReturnType: "Object"}}},
	{Name: "Queueable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "execute", ReturnType: "void", Parameters: []string{"QueueableContext"}}}},
	{Name: "Schedulable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "execute", ReturnType: "void", Parameters: []string{"SchedulableContext"}}}},
	{Name: "AsyncOptions", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "DuplicateSignature", Type: "Object"}, {Name: "MaximumQueueableStackDepth", Type: "Integer"}, {Name: "MinimumQueueableDelayInMinutes", Type: "Integer"}}},
	{Name: "QueueableDuplicateSignature.Builder", Methods: []StandardMethodSpec{{Name: "getMaxSize", ReturnType: "Integer"}, {Name: "getRemainingSize", ReturnType: "Integer"}, {Name: "getSize", ReturnType: "Integer"}}},
	{Name: "RemoteObjectController", Methods: []StandardMethodSpec{{Name: "update", ReturnType: "Map<String,Object>", Parameters: []string{"String", "List<String>", "Map<String,Object>"}, Static: true}}},
	{Name: "Database.DMLOptions", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "AllowFieldTruncation", Type: "Boolean"}, {Name: "AssignmentRuleHeader", Type: "Database.AssignmentRuleHeader"}, {Name: "DuplicateRuleHeader", Type: "Database.DuplicateRuleHeader"}, {Name: "EmailHeader", Type: "Database.EmailHeader"}, {Name: "LocaleOptions", Type: "Database.LocaleOptions"}, {Name: "LocalizeErrors", Type: "Boolean"}, {Name: "OptAllOrNone", Type: "Boolean"}}},
	{Name: "Database.AssignmentRuleHeader", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "AssignmentRuleId", Type: "Id"}, {Name: "UseDefaultRule", Type: "Boolean"}}},
	{Name: "Database.DuplicateRuleHeader", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "AllowSave", Type: "Boolean"}, {Name: "RunAsCurrentUser", Type: "Boolean"}}},
	{Name: "Database.EmailHeader", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "TriggerAutoResponseEmail", Type: "Boolean"}, {Name: "TriggerOtherEmail", Type: "Boolean"}, {Name: "TriggerUserEmail", Type: "Boolean"}}},
	{Name: "Database.LocaleOptions", Constructors: [][]string{{}}},
	{Name: "Database.Error", Methods: []StandardMethodSpec{{Name: "getFields", ReturnType: "List<String>"}, {Name: "getMessage", ReturnType: "String"}, {Name: "getStatusCode", ReturnType: "StatusCode"}, {Name: "getExtendedErrorDetails", ReturnType: "List<Object>"}}},
	{Name: "Database.DuplicateError", SuperClass: "Database.Error", Methods: []StandardMethodSpec{{Name: "getDuplicateResult", ReturnType: "Datacloud.DuplicateResult"}, {Name: "getFields", ReturnType: "List<String>"}, {Name: "getMessage", ReturnType: "String"}, {Name: "getStatusCode", ReturnType: "StatusCode"}}},
	{Name: "Datacloud.DuplicateResult", Methods: []StandardMethodSpec{{Name: "getDuplicateRule", ReturnType: "String"}, {Name: "getDuplicateRuleEntityType", ReturnType: "String"}, {Name: "getErrorMessage", ReturnType: "String"}, {Name: "getMatchResults", ReturnType: "List<Datacloud.MatchResult>"}, {Name: "isAllowSave", ReturnType: "Boolean"}}},
	{Name: "Datacloud.MatchResult", Methods: []StandardMethodSpec{{Name: "getEntityType", ReturnType: "String"}, {Name: "getErrors", ReturnType: "List<Database.Error>"}, {Name: "getMatchEngine", ReturnType: "String"}, {Name: "getMatchRecords", ReturnType: "List<Datacloud.MatchRecord>"}, {Name: "getRule", ReturnType: "String"}, {Name: "getSize", ReturnType: "Integer"}, {Name: "isSuccess", ReturnType: "Boolean"}}},
	{Name: "Datacloud.MatchRecord", Methods: []StandardMethodSpec{{Name: "getAdditionalInformation", ReturnType: "List<Datacloud.AdditionalInformationMap>"}, {Name: "getFieldDiffs", ReturnType: "List<Datacloud.FieldDiff>"}, {Name: "getMatchConfidence", ReturnType: "Double"}, {Name: "getRecord", ReturnType: "SObject"}}},
	{Name: "Datacloud.FieldDiff", Methods: []StandardMethodSpec{{Name: "getDifference", ReturnType: "String"}, {Name: "getName", ReturnType: "String"}}},
	{Name: "System.Callable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "call", ReturnType: "Object", Parameters: []string{"String", "Map<String,Object>"}}}},
	{Name: "System.StubProvider", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "handleMethodCall", ReturnType: "Object", Parameters: []string{"Object", "String", "Type", "List<Type>", "List<String>", "List<Object>"}}}},
}

var standardPlatformSymbolOverlays = []StandardSymbolSpec{
	{Name: "UserProvisioning.UserProvisioningPlugin", Modifiers: []string{"abstract"}, Methods: []StandardMethodSpec{
		{Name: "buildDescribeCall", ReturnType: "Process.PluginDescribeResult", Modifiers: []string{"abstract"}},
	}},
	// API 67 platform enum hashes use a stable family seed plus declaration
	// ordinal. Keep the seeds on the merged type metadata, not on members.
	{Name: "Schema.SoapType", EnumHashBase: standardEnumHashBase(884834318)},
	{Name: "StatusCode", EnumHashBase: standardEnumHashBase(674160322)},
	{Name: "Metadata.StatusCode", EnumHashBase: standardEnumHashBase(-369369150)},
	{Name: "Exception", Modifiers: []string{"abstract"}, ReplaceConstructors: true},
	// Package.Version is a platform value type; API 67 exposes no Apex
	// constructor even though the generated stub includes a default one.
	{Name: "Package.Version", ReplaceConstructors: true},
	{Name: "InvalidParameterValueException", SuperClass: "Exception", Constructors: [][]string{{"String", "String"}}, ReplaceConstructors: true},
	{Name: "NoAccessException", SuperClass: "Exception", Constructors: [][]string{{}}, ReplaceConstructors: true},
	{Name: "NoDataFoundException", SuperClass: "Exception", Constructors: [][]string{{}}, ReplaceConstructors: true},
	{Name: "NullPointerException", SuperClass: "Exception", Constructors: [][]string{{}}, ReplaceConstructors: true},
	{Name: "Cache.CacheBuilder", Kind: apexast.DeclarationInterface},
	{Name: "CommerceExtension.ResolutionStrategy", Kind: apexast.DeclarationInterface},
	{Name: "Finalizer", Kind: apexast.DeclarationInterface},
	{Name: "Readiness.ProductEvaluator", Kind: apexast.DeclarationInterface},
	{Name: "DataSource.AsyncDeleteCallback", Kind: apexast.DeclarationClass, Modifiers: []string{"abstract"}, Methods: []StandardMethodSpec{{Name: "processDelete", ReturnType: "void", Modifiers: []string{"virtual"}, Parameters: []string{"Database.DeleteResult"}}}},
	{Name: "DataSource.AsyncSaveCallback", Kind: apexast.DeclarationClass, Modifiers: []string{"abstract"}, Methods: []StandardMethodSpec{{Name: "processSave", ReturnType: "void", Modifiers: []string{"virtual"}, Parameters: []string{"Database.SaveResult"}}}},
	{Name: "Metadata.DeployResult", Properties: []StandardPropertySpec{
		{Name: "errorMessage", Type: "String"},
		{Name: "errorStatusCode", Type: "Metadata.StatusCode", Force: true},
		{Name: "success", Type: "Boolean"},
	}},
	{Name: "ApexPages.Message", Methods: []StandardMethodSpec{{Name: "getSeverity", ReturnType: "ApexPages.Severity"}}},
	{Name: "AsyncInfo", Methods: []StandardMethodSpec{{Name: "getCurrentQueueableStackDepth", ReturnType: "Integer", Static: true}}},
	{Name: "AsyncOptions", Methods: []StandardMethodSpec{{Name: "getMaximumQueueableStackDepth", ReturnType: "Integer"}, {Name: "setMinimumQueueableDelayInMinutes", ReturnType: "void", Parameters: []string{"Integer"}}}},
	{Name: "Limits", Methods: []StandardMethodSpec{{Name: "getAsyncJobs", ReturnType: "Integer", Static: true}, {Name: "getFutureCalls", ReturnType: "Integer", Static: true}, {Name: "getLimitAsyncJobs", ReturnType: "Integer", Static: true}, {Name: "getQueueableJobs", ReturnType: "Integer", Static: true}}},
	{Name: "QueueableContext", Methods: []StandardMethodSpec{{Name: "getJobId", ReturnType: "Id"}}},
	{Name: "SchedulableContext", Methods: []StandardMethodSpec{{Name: "getTriggerId", ReturnType: "Id"}}},
	{Name: "TxnSecurity.AsyncCondition", Kind: apexast.DeclarationInterface},
	{Name: "Test", Methods: []StandardMethodSpec{{Name: "clearApexPageMessages", ReturnType: "void", Static: true}}},
	{Name: "ConnectApi.ManagedContentVersion", Properties: []StandardPropertySpec{{Name: "contentNodes", Type: "Map<String,ConnectApi.ManagedContentNodeValue>"}}},
	{Name: "ConnectApi.ManagedContentVersionCollection", Properties: []StandardPropertySpec{{Name: "items", Type: "List<ConnectApi.ManagedContentVersion>"}}},
	{Name: "ConnectApi.OrganizationSettings", Properties: []StandardPropertySpec{{Name: "orgId", Type: "Id"}}},
	{Name: "ConnectApi.OrchestrationInstanceCollection", Properties: []StandardPropertySpec{{Name: "instances", Type: "List<ConnectApi.OrchestrationInstance>"}}},
	{Name: "ApexPages.StandardController", Methods: []StandardMethodSpec{
		{Name: "getRecord", ReturnType: "SObject"},
		{Name: "quickSave", ReturnType: "PageReference"},
	}},
	{Name: "ApexPages.StandardSetController", Methods: []StandardMethodSpec{
		{Name: "addFields", ReturnType: "void", Parameters: []string{"List<String>"}},
		{Name: "cancel", ReturnType: "PageReference"},
		{Name: "first", ReturnType: "void"},
		{Name: "getCompleteResult", ReturnType: "Boolean"},
		{Name: "getFilterId", ReturnType: "String"},
		{Name: "getListViewOptions", ReturnType: "List<SelectOption>"},
		{Name: "getPageNumber", ReturnType: "Integer"},
		{Name: "getPageSize", ReturnType: "Integer"},
		{Name: "getRecord", ReturnType: "SObject"},
		{Name: "getSelected", ReturnType: "List<Object>"},
		{Name: "last", ReturnType: "void"},
		{Name: "save", ReturnType: "PageReference"},
		{Name: "setFilterID", ReturnType: "void", Parameters: []string{"String"}},
		{Name: "setPageNumber", ReturnType: "void", Parameters: []string{"Integer"}},
		{Name: "setPageSize", ReturnType: "void", Parameters: []string{"Integer"}},
		{Name: "setSelected", ReturnType: "void", Parameters: []string{"List<Object>"}},
	}},
	{Name: "ApexPages.KnowledgeArticleVersionStandardController", Methods: []StandardMethodSpec{
		{Name: "setDataCategory", ReturnType: "void", Parameters: []string{"String", "String"}},
	}},
	{Name: "Dom.XmlNode", Methods: []StandardMethodSpec{
		{Name: "insertBefore", ReturnType: "Dom.XmlNode", Parameters: []string{"Dom.XmlNode", "Dom.XmlNode"}},
		{Name: "removeChild", ReturnType: "Boolean", Parameters: []string{"Dom.XmlNode"}},
	}},
	{Name: "Invocable.Action", Methods: []StandardMethodSpec{
		{Name: "getDescribe", ReturnType: "List<Invocable.Action.DescribeResult>"},
		{Name: "getName", ReturnType: "String"},
		{Name: "getNamespace", ReturnType: "String"},
		{Name: "getType", ReturnType: "String"},
		{Name: "getVersion", ReturnType: "String"},
		{Name: "isStandard", ReturnType: "Boolean"},
	}},
	{Name: "Invocable.Action.AdditionalAttribute", Methods: []StandardMethodSpec{
		{Name: "clone", ReturnType: "Object"},
		{Name: "getApexClass", ReturnType: "String"},
		{Name: "getDataType", ReturnType: "String"},
		{Name: "getIsCollection", ReturnType: "Boolean"},
		{Name: "getName", ReturnType: "String"},
		{Name: "getValue", ReturnType: "Object"},
		{Name: "getValueAsBooleanList", ReturnType: "List<Boolean>"},
		{Name: "getValueAsDateList", ReturnType: "List<Date>"},
		{Name: "getValueAsDoubleList", ReturnType: "List<Double>"},
		{Name: "getValueAsIntegerList", ReturnType: "List<Integer>"},
		{Name: "getValueAsList", ReturnType: "List<Object>"},
		{Name: "getValueAsLongList", ReturnType: "List<Long>"},
		{Name: "getValueAsStringList", ReturnType: "List<String>"},
	}},
	{Name: "Invocable.Action.DescribeResult", Methods: []StandardMethodSpec{
		{Name: "clone", ReturnType: "Object"},
		{Name: "getAction", ReturnType: "Invocable.Action"},
		{Name: "getAllowsTransactionControl", ReturnType: "Boolean"},
		{Name: "getCapabilityTypes", ReturnType: "List<String>"},
		{Name: "getCategory", ReturnType: "String"},
		{Name: "getConfigurationEditor", ReturnType: "String"},
		{Name: "getDescription", ReturnType: "String"},
		{Name: "getGenericTypes", ReturnType: "List<Invocable.Action.GenericType>"},
		{Name: "getHasCallout", ReturnType: "Boolean"},
		{Name: "getHasSystemGeneratedOutput", ReturnType: "Boolean"},
		{Name: "getIconId", ReturnType: "String"},
		{Name: "getIconName", ReturnType: "String"},
		{Name: "getInputs", ReturnType: "List<Invocable.Action.InputParameter>"},
		{Name: "getLabel", ReturnType: "String"},
		{Name: "getMethodDescription", ReturnType: "String"},
		{Name: "getMethodLabel", ReturnType: "String"},
		{Name: "getMethodName", ReturnType: "String"},
		{Name: "getName", ReturnType: "String"},
		{Name: "getOutputs", ReturnType: "List<Invocable.Action.OutputParameter>"},
		{Name: "getTargetEntityName", ReturnType: "String"},
		{Name: "getType", ReturnType: "String"},
	}},
	{Name: "Invocable.Action.GenericType", Methods: []StandardMethodSpec{
		{Name: "clone", ReturnType: "Object"},
		{Name: "getDescription", ReturnType: "String"},
		{Name: "getLabel", ReturnType: "String"},
		{Name: "getName", ReturnType: "String"},
		{Name: "getSuperType", ReturnType: "String"},
	}},
	{Name: "Invocable.Action.InputParameter", Methods: []StandardMethodSpec{
		{Name: "clone", ReturnType: "Object"},
		{Name: "getAdditionalAttributes", ReturnType: "List<Invocable.Action.AdditionalAttribute>"},
		{Name: "getApexClass", ReturnType: "String"},
		{Name: "getByteLength", ReturnType: "Integer"},
		{Name: "getConfiguration", ReturnType: "Boolean"},
		{Name: "getDefaultValue", ReturnType: "Object"},
		{Name: "getDescription", ReturnType: "String"},
		{Name: "getLabel", ReturnType: "String"},
		{Name: "getMaxOccurs", ReturnType: "Integer"},
		{Name: "getName", ReturnType: "String"},
		{Name: "getPicklistValues", ReturnType: "List<Invocable.Action.PicklistValue>"},
		{Name: "getPlaceholderText", ReturnType: "String"},
		{Name: "getRequired", ReturnType: "Boolean"},
		{Name: "getSObjectType", ReturnType: "String"},
		{Name: "getSetupReferenceType", ReturnType: "List<String>"},
		{Name: "getToolingType", ReturnType: "String"},
		{Name: "getType", ReturnType: "String"},
	}},
	{Name: "Invocable.Action.OutputParameter", Methods: []StandardMethodSpec{
		{Name: "clone", ReturnType: "Object"},
		{Name: "getAdditionalAttributes", ReturnType: "List<Invocable.Action.AdditionalAttribute>"},
		{Name: "getApexClass", ReturnType: "String"},
		{Name: "getDescription", ReturnType: "String"},
		{Name: "getLabel", ReturnType: "String"},
		{Name: "getMaxOccurs", ReturnType: "Integer"},
		{Name: "getName", ReturnType: "String"},
		{Name: "getPicklistValues", ReturnType: "List<Invocable.Action.PicklistValue>"},
		{Name: "getSObjectType", ReturnType: "String"},
		{Name: "getType", ReturnType: "String"},
	}},
	{Name: "Invocable.Action.PicklistValue", Methods: []StandardMethodSpec{
		{Name: "clone", ReturnType: "Object"},
		{Name: "getActive", ReturnType: "Boolean"},
		{Name: "getDefaultValue", ReturnType: "Boolean"},
		{Name: "getLabel", ReturnType: "String"},
		{Name: "getValidFor", ReturnType: "String"},
		{Name: "getValue", ReturnType: "String"},
	}},
	{Name: "Invocable.Action.Error", Methods: []StandardMethodSpec{
		{Name: "clone", ReturnType: "Object"},
		{Name: "getCode", ReturnType: "String"},
		{Name: "getMessage", ReturnType: "String"},
	}},
	{Name: "Invocable.Action.Result", Methods: []StandardMethodSpec{
		{Name: "clone", ReturnType: "Object"},
		{Name: "getAction", ReturnType: "Invocable.Action"},
		{Name: "getErrors", ReturnType: "List<Invocable.Action.Error>"},
		{Name: "getInvocationParameters", ReturnType: "Map<String,Object>"},
		{Name: "getOutputParameters", ReturnType: "Map<String,Object>"},
		{Name: "isSuccess", ReturnType: "Boolean"},
	}},
	{Name: "Metadata.Metadata", Methods: []StandardMethodSpec{{Name: "clone", ReturnType: "Object"}}},
	{Name: "Process.PluginDescribeResult", Properties: []StandardPropertySpec{
		{Name: "inputParameters", Type: "List<Process.PluginDescribeResult.InputParameter>"},
		{Name: "outputParameters", Type: "List<Process.PluginDescribeResult.OutputParameter>"},
	}},
	{Name: "Process.PluginDescribeResult.InputParameter", Constructors: [][]string{
		{"String", "Process.PluginDescribeResult.ParameterType", "Boolean"},
		{"String", "String", "Process.PluginDescribeResult.ParameterType", "Boolean"},
	}, Properties: []StandardPropertySpec{
		{Name: "description", Type: "String"},
		{Name: "name", Type: "String"},
		{Name: "parameterType", Type: "Process.PluginDescribeResult.ParameterType"},
		{Name: "required", Type: "Boolean"},
	}},
	{Name: "Process.PluginDescribeResult.OutputParameter", Constructors: [][]string{
		{"String", "Process.PluginDescribeResult.ParameterType"},
		{"String", "String", "Process.PluginDescribeResult.ParameterType"},
	}, Properties: []StandardPropertySpec{
		{Name: "description", Type: "String"},
		{Name: "name", Type: "String"},
		{Name: "parameterType", Type: "Process.PluginDescribeResult.ParameterType"},
	}},
	{Name: "Messaging.Email", Methods: []StandardMethodSpec{
		{Name: "setTemplateID", ReturnType: "void", Parameters: []string{"Id"}},
	}},
	{Name: "Messaging.SingleEmailMessage", Methods: []StandardMethodSpec{
		{Name: "setDocumentAttachments", ReturnType: "void", Parameters: []string{"List<Id>"}},
		{Name: "setFileAttachments", ReturnType: "void", Parameters: []string{"List<Messaging.EmailFileAttachment>"}},
	}},
	{Name: "Support.EmailTemplateSelector", Methods: []StandardMethodSpec{
		{Name: "getDefaultTemplateId", ReturnType: "Id", Parameters: []string{"Id"}},
	}},
	{Name: "Database", Methods: []StandardMethodSpec{
		{Name: "countQueryWithBinds", ReturnType: "Integer", Parameters: []string{"String", "Map", "AccessLevel"}, Static: true},
		{Name: "deleteAsync", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Object>", "DataSource.AsyncDeleteCallback"}, Static: true},
		{Name: "deleteAsync", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Object>", "DataSource.AsyncDeleteCallback", "AccessLevel"}, Static: true},
		{Name: "deleteAsync", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Object>", "AccessLevel"}, Static: true},
		{Name: "deleteAsync", ReturnType: "Database.DeleteResult", Parameters: []string{"Object", "DataSource.AsyncDeleteCallback", "AccessLevel"}, Static: true},
		{Name: "deleteAsync", ReturnType: "Database.DeleteResult", Parameters: []string{"Object", "AccessLevel"}, Static: true},
		{Name: "getAsyncDeleteResult", ReturnType: "Database.DeleteResult", Parameters: []string{"Database.DeleteResult"}, Static: true},
		{Name: "getAsyncSaveResult", ReturnType: "Database.SaveResult", Parameters: []string{"Database.SaveResult"}, Static: true},
		{Name: "getCursor", ReturnType: "Database.Cursor", Parameters: []string{"String", "Object"}, Static: true},
		{Name: "getCursor", ReturnType: "Database.Cursor", Parameters: []string{"String", "AccessLevel"}, Static: true},
		{Name: "getCursorWithBinds", ReturnType: "Database.Cursor", Parameters: []string{"String", "Map", "Object"}, Static: true},
		{Name: "getCursorWithBinds", ReturnType: "Database.Cursor", Parameters: []string{"String", "Map", "AccessLevel"}, Static: true},
		{Name: "getPaginationCursor", ReturnType: "Database.PaginationCursor", Parameters: []string{"String", "Object"}, Static: true},
		{Name: "getPaginationCursor", ReturnType: "Database.PaginationCursor", Parameters: []string{"String", "AccessLevel"}, Static: true},
		{Name: "getPaginationCursorWithBinds", ReturnType: "Database.PaginationCursor", Parameters: []string{"String", "Map", "Object"}, Static: true},
		{Name: "getPaginationCursorWithBinds", ReturnType: "Database.PaginationCursor", Parameters: []string{"String", "Map", "AccessLevel"}, Static: true},
		{Name: "insertAsync", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "DataSource.AsyncSaveCallback"}, Static: true},
		{Name: "insertAsync", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "DataSource.AsyncSaveCallback", "AccessLevel"}, Static: true},
		{Name: "insertAsync", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "AccessLevel"}, Static: true},
		{Name: "insertAsync", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "DataSource.AsyncSaveCallback", "AccessLevel"}, Static: true},
		{Name: "insertAsync", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "AccessLevel"}, Static: true},
		{Name: "insertImmediate", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "AccessLevel"}, Static: true},
		{Name: "insertImmediate", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "AccessLevel"}, Static: true},
		{Name: "insert", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "Database.DMLOptions"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "Database.DMLOptions"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "Database.DMLOptions", "AccessLevel"}, Static: true},
		{Name: "insert", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "Database.DMLOptions", "AccessLevel"}, Static: true},
		{Name: "queryWithBinds", ReturnType: "List<SObject>", Parameters: []string{"String", "Map", "AccessLevel"}, Static: true},
		{Name: "updateAsync", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "DataSource.AsyncSaveCallback"}, Static: true},
		{Name: "updateAsync", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "DataSource.AsyncSaveCallback", "AccessLevel"}, Static: true},
		{Name: "updateAsync", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "AccessLevel"}, Static: true},
		{Name: "updateAsync", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "DataSource.AsyncSaveCallback", "AccessLevel"}, Static: true},
		{Name: "updateAsync", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "AccessLevel"}, Static: true},
		{Name: "updateImmediate", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "AccessLevel"}, Static: true},
		{Name: "updateImmediate", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "AccessLevel"}, Static: true},
		{Name: "update", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "Database.DMLOptions"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "Database.DMLOptions"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<Object>", "Database.DMLOptions", "AccessLevel"}, Static: true},
		{Name: "update", ReturnType: "Database.SaveResult", Parameters: []string{"Object", "Database.DMLOptions", "AccessLevel"}, Static: true},
		{Name: "deleteImmediate", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Object>", "AccessLevel"}, Static: true},
		{Name: "deleteImmediate", ReturnType: "Database.DeleteResult", Parameters: []string{"Object", "AccessLevel"}, Static: true},
	}},
	{Name: "Schema.DescribeFieldResult", Methods: []StandardMethodSpec{
		{Name: "getControllerValues", ReturnType: "Map<String,Integer>"},
	}, Properties: []StandardPropertySpec{{Name: "controllervalues", Type: "Map<String,Integer>"}}},
	{Name: "industriesNlpSvc.NlpResponse", Properties: []StandardPropertySpec{{Name: "summarizationResult", Type: "industriesNlpSvc.NlpSummarizationResult"}, {Name: "errors", Type: "List<String>"}}},
	{Name: "industriesNlpSvc.NlpSummarizationResult", Properties: []StandardPropertySpec{{Name: "summary", Type: "String"}}},
	{Name: "Quiddity", Properties: []StandardPropertySpec{
		{Name: "RUN_INTEGRATION_TESTS", Type: "Quiddity", Static: true},
	}},
	{Name: "String", Methods: []StandardMethodSpec{
		{Name: "join", ReturnType: "String", Parameters: []string{"List<Object>", "String"}, Static: true},
		{Name: "join", ReturnType: "String", Parameters: []string{"Set<Object>", "String"}, Static: true},
		{Name: "template", ReturnType: "String", Parameters: []string{"Map"}},
	}},
	// API 67 rejects no-arg constructors on these 29 Reports
	// types. They are returned by runtime methods; Apex code cannot
	// instantiate them directly.
	{Name: "reports.AggregateColumn", ReplaceConstructors: true},
	{Name: "reports.DetailColumn", ReplaceConstructors: true},
	{Name: "reports.Dimension", ReplaceConstructors: true},
	{Name: "reports.FilterOperator", ReplaceConstructors: true},
	{Name: "reports.FilterValue", ReplaceConstructors: true},
	{Name: "reports.GroupingColumn", ReplaceConstructors: true},
	{Name: "reports.GroupingValue", ReplaceConstructors: true},
	{Name: "reports.ReportCurrency", ReplaceConstructors: true},
	{Name: "reports.ReportDataCell", ReplaceConstructors: true},
	{Name: "reports.ReportDescribeResult", ReplaceConstructors: true},
	{Name: "reports.ReportDetailRow", ReplaceConstructors: true},
	{Name: "reports.ReportDivisionInfo", ReplaceConstructors: true},
	{Name: "reports.ReportExtendedMetadata", ReplaceConstructors: true},
	{Name: "reports.ReportFact", ReplaceConstructors: true},
	{Name: "reports.ReportFactWithDetails", ReplaceConstructors: true},
	{Name: "reports.ReportFactWithSummaries", ReplaceConstructors: true},
	{Name: "reports.ReportInstance", ReplaceConstructors: true},
	{Name: "reports.ReportInstanceAttributes", ReplaceConstructors: true},
	{Name: "reports.ReportResults", ReplaceConstructors: true},
	{Name: "reports.ReportScopeInfo", ReplaceConstructors: true},
	{Name: "reports.ReportScopeValue", ReplaceConstructors: true},
	{Name: "reports.ReportTypeColumn", ReplaceConstructors: true},
	{Name: "reports.ReportTypeColumnCategory", ReplaceConstructors: true},
	{Name: "reports.ReportTypeMetadata", ReplaceConstructors: true},
	{Name: "reports.StandardDateFilterDuration", ReplaceConstructors: true},
	{Name: "reports.StandardDateFilterDurationGroup", ReplaceConstructors: true},
	{Name: "reports.StandardFilterInfo", ReplaceConstructors: true},
	{Name: "reports.StandardFilterInfoPicklist", ReplaceConstructors: true},
	{Name: "reports.SummaryValue", ReplaceConstructors: true},
}

func standardAssertMethods() []StandardMethodSpec {
	return []StandardMethodSpec{
		{Name: "areEqual", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true},
		{Name: "areEqual", ReturnType: "void", Parameters: []string{"Object", "Object", "String"}, Static: true},
		{Name: "areNotEqual", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true},
		{Name: "areNotEqual", ReturnType: "void", Parameters: []string{"Object", "Object", "String"}, Static: true},
		{Name: "isTrue", ReturnType: "void", Parameters: []string{"Boolean"}, Static: true},
		{Name: "isTrue", ReturnType: "void", Parameters: []string{"Boolean", "String"}, Static: true},
		{Name: "isFalse", ReturnType: "void", Parameters: []string{"Boolean"}, Static: true},
		{Name: "isFalse", ReturnType: "void", Parameters: []string{"Boolean", "String"}, Static: true},
		{Name: "isNull", ReturnType: "void", Parameters: []string{"Object"}, Static: true},
		{Name: "isNull", ReturnType: "void", Parameters: []string{"Object", "String"}, Static: true},
		{Name: "isNotNull", ReturnType: "void", Parameters: []string{"Object"}, Static: true},
		{Name: "isNotNull", ReturnType: "void", Parameters: []string{"Object", "String"}, Static: true},
		{Name: "fail", ReturnType: "void", Static: true},
		{Name: "fail", ReturnType: "void", Parameters: []string{"String"}, Static: true},
	}
}
