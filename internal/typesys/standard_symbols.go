package typesys

import (
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
)

type StandardSymbolSpec struct {
	Name             string
	Kind             apexast.DeclarationKind
	SuperClass       string
	Interfaces       []string
	Constructors     [][]string
	ConstructorSpecs []StandardConstructorSpec
	Methods          []StandardMethodSpec
	Properties       []StandardPropertySpec
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
	Static         bool
}

type StandardPropertySpec struct {
	Name   string
	Type   string
	Static bool
}

func standardEnumProperties(typeName string, names ...string) []StandardPropertySpec {
	props := make([]StandardPropertySpec, 0, len(names))
	for _, name := range names {
		props = append(props, StandardPropertySpec{Name: name, Type: typeName, Static: true})
	}
	return props
}

func StandardPlatformSymbols() []TypeSymbol {
	specs := append([]StandardSymbolSpec(nil), standardPlatformSymbolSpecs...)
	specs = append(specs, productNamespaceSymbolSpecs...)
	specs = append(specs, systemStubSymbolSpecs...)
	for _, name := range standardPlatformTypeNames {
		if standardSpecExists(specs, name) {
			continue
		}
		specs = append(specs, StandardSymbolSpec{Name: name, Kind: apexast.DeclarationClass})
	}
	return StandardSymbolsFromSpecs(specs)
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
			Kind:       kind,
			Name:       spec.Name,
			File:       "<standard-platform>",
			Dependency: true,
			SuperClass: superClass,
			Interfaces: append([]string(nil), spec.Interfaces...),
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
			if method.Static {
				modifiers = append(modifiers, "static")
			}
			sym.Members = append(sym.Members, MemberSymbol{
				Kind:       apexast.DeclarationMethod,
				Name:       method.Name,
				Type:       method.ReturnType,
				Modifiers:  modifiers,
				Parameters: standardMethodParameters(method),
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
		if existing.Kind == "" {
			existing.Kind = spec.Kind
		}
		if existing.SuperClass == "" {
			existing.SuperClass = spec.SuperClass
		}
		existing.Interfaces = appendUniqueStandardStrings(existing.Interfaces, spec.Interfaces)
		existing.Constructors = appendUniqueStandardConstructors(existing.Constructors, spec.Constructors)
		existing.ConstructorSpecs = appendUniqueStandardConstructorSpecs(existing.ConstructorSpecs, spec.ConstructorSpecs)
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
	for _, value := range values {
		seen[standardMethodKey(value)] = true
	}
	for _, addition := range additions {
		key := standardMethodKey(addition)
		if seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, addition)
	}
	return values
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

func standardMethodParameters(method StandardMethodSpec) []apexast.Parameter {
	if len(method.ParameterSpecs) != 0 {
		return standardSpecParameters(method.ParameterSpecs)
	}
	return standardParameters(method.Parameters)
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
	"Auth.AuthToken",
	"Auth.CommunitiesUtil",
	"Auth.JWT",
	"Auth.RegistrationHandler",
	"Auth.SessionManagement",
	"Auth.UserData",
	"Auth.VerificationMethod",
	"Auth.VerificationResult",
	"Cache",
	"Cache.CacheBuilder",
	"Cache.Org",
	"Cache.OrgPartition",
	"Cache.Session",
	"Cache.SessionPartition",
	"Cache.Visibility",
	"Callable",
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
	"Database.Error",
	"Database.DuplicateError",
	"Database.LeadConvert",
	"Database.LeadConvertResult",
	"Database.MergeResult",
	"Database.QueryLocator",
	"Database.SaveResult",
	"Database.Savepoint",
	"Database.UndeleteResult",
	"Database.UpsertResult",
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
	"Schema.SoapType",
	"SObjectDescribeOptions",
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
	{Name: "Version", Constructors: [][]string{{"Integer", "Integer"}, {"Integer", "Integer", "Integer"}}, Methods: []StandardMethodSpec{{Name: "compareTo", ReturnType: "Integer", Parameters: []string{"Version"}}}},
	{Name: "InstallContext", Methods: []StandardMethodSpec{{Name: "previousVersion", ReturnType: "Version"}, {Name: "isPush", ReturnType: "Boolean"}, {Name: "installerId", ReturnType: "Id"}}, Properties: []StandardPropertySpec{{Name: "installerId", Type: "Id"}, {Name: "InstallerId", Type: "Id"}}},
	{Name: "URL", Constructors: [][]string{{"URL", "String"}}, Methods: []StandardMethodSpec{{Name: "getSalesforceBaseUrl", ReturnType: "URL", Static: true}, {Name: "toExternalForm", ReturnType: "String"}}},
	{Name: "PageReference", Constructors: [][]string{{"String"}}, Methods: []StandardMethodSpec{{Name: "getUrl", ReturnType: "String"}, {Name: "setRedirect", ReturnType: "PageReference", Parameters: []string{"Boolean"}}, {Name: "getParameters", ReturnType: "Map<String,String>"}}},
	{Name: "SelectOption", Constructors: [][]string{{"String", "String"}, {"String", "String", "Boolean"}, {"String", "String", "Boolean", "Boolean"}}},
	{Name: "Search", Methods: []StandardMethodSpec{{Name: "query", ReturnType: "List<List<SObject>>", Parameters: []string{"String"}, Static: true}}},
	{Name: "DataWeave.Script", Constructors: [][]string{{}}, Methods: []StandardMethodSpec{{Name: "createScript", ReturnType: "DataWeave.Script", Parameters: []string{"String"}, Static: true}, {Name: "execute", ReturnType: "DataWeave.Result", Parameters: []string{"Map<String,Object>"}}}},
	{Name: "DataWeave.Result", Constructors: [][]string{{}}, Methods: []StandardMethodSpec{{Name: "getValue", ReturnType: "Object"}, {Name: "getValueAsString", ReturnType: "String"}, {Name: "getMimeType", ReturnType: "String"}}, Properties: []StandardPropertySpec{{Name: "value", Type: "Object"}, {Name: "valueAsString", Type: "String"}, {Name: "mimeType", Type: "String"}}},
	{Name: "DataWeaveScriptResource", Constructors: [][]string{{}}},
	{Name: "DataWeaveScriptException", SuperClass: "Exception", Constructors: [][]string{{}, {"String"}}},
	{Name: "ConnectApi.ExternalCredential", Properties: []StandardPropertySpec{{Name: "principals", Type: "List<ConnectApi.ExternalCredentialPrincipal>"}}},
	{Name: "ConnectApi.ExternalCredentialInput", Properties: []StandardPropertySpec{{Name: "principals", Type: "List<ConnectApi.ExternalCredentialPrincipalInput>"}}},
	{Name: "Messaging.BinaryAttachment", Properties: []StandardPropertySpec{{Name: "body", Type: "Blob"}, {Name: "fileName", Type: "String"}, {Name: "headers", Type: "List<Messaging.InboundEmail.Header>"}, {Name: "mimeTypeSubType", Type: "String"}}},
	{Name: "Messaging.InboundEmail", Properties: []StandardPropertySpec{{Name: "authenticationResults", Type: "List<Messaging.AuthenticationResultField>"}, {Name: "binaryAttachments", Type: "List<Messaging.InboundEmail.BinaryAttachment>"}, {Name: "ccAddresses", Type: "List<String>"}, {Name: "fromAddress", Type: "String"}, {Name: "fromName", Type: "String"}, {Name: "headers", Type: "List<Messaging.InboundEmail.Header>"}, {Name: "htmlBody", Type: "String"}, {Name: "htmlBodyIsTruncated", Type: "Boolean"}, {Name: "inReplyTo", Type: "String"}, {Name: "messageId", Type: "String"}, {Name: "plainTextBody", Type: "String"}, {Name: "plainTextBodyIsTruncated", Type: "Boolean"}, {Name: "references", Type: "String"}, {Name: "replyTo", Type: "String"}, {Name: "subject", Type: "String"}, {Name: "textAttachments", Type: "List<Messaging.InboundEmail.TextAttachment>"}, {Name: "toAddresses", Type: "List<String>"}}},
	{Name: "Messaging.InboundEmail.BinaryAttachment", SuperClass: "Messaging.BinaryAttachment", Constructors: [][]string{{}}},
	{Name: "Messaging.InboundEmail.TextAttachment", SuperClass: "Messaging.TextAttachment", Constructors: [][]string{{}}},
	{Name: "Messaging.InboundEmail.Header", Properties: []StandardPropertySpec{{Name: "name", Type: "String"}, {Name: "value", Type: "String"}}},
	{Name: "Messaging.InboundEmailResult", Properties: []StandardPropertySpec{{Name: "message", Type: "String"}, {Name: "success", Type: "Boolean"}}},
	{Name: "Messaging.InboundEnvelope", Properties: []StandardPropertySpec{{Name: "fromAddress", Type: "String"}, {Name: "toAddress", Type: "String"}}},
	{Name: "Messaging.TextAttachment", Properties: []StandardPropertySpec{{Name: "body", Type: "String"}, {Name: "bodyIsTruncated", Type: "Boolean"}, {Name: "charset", Type: "String"}, {Name: "fileName", Type: "String"}, {Name: "headers", Type: "List<Messaging.InboundEmail.Header>"}, {Name: "mimeTypeSubType", Type: "String"}}},
	{Name: "System", Methods: []StandardMethodSpec{{Name: "now", ReturnType: "Datetime", Static: true}, {Name: "today", ReturnType: "Date", Static: true}, {Name: "debug", ReturnType: "void", Parameters: []string{"Object"}, Static: true}, {Name: "assert", ReturnType: "void", Parameters: []string{"Boolean"}, Static: true}, {Name: "assert", ReturnType: "void", Parameters: []string{"Boolean", "Object"}, Static: true}, {Name: "assertEquals", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true}, {Name: "assertEquals", ReturnType: "void", Parameters: []string{"Object", "Object", "Object"}, Static: true}, {Name: "assertNotEquals", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true}, {Name: "assertNotEquals", ReturnType: "void", Parameters: []string{"Object", "Object", "Object"}, Static: true}}},
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
	{Name: "LoggingLevel", Kind: apexast.DeclarationEnum, Properties: []StandardPropertySpec{{Name: "NONE", Type: "LoggingLevel", Static: true}, {Name: "ERROR", Type: "LoggingLevel", Static: true}, {Name: "WARN", Type: "LoggingLevel", Static: true}, {Name: "INFO", Type: "LoggingLevel", Static: true}, {Name: "DEBUG", Type: "LoggingLevel", Static: true}, {Name: "FINE", Type: "LoggingLevel", Static: true}, {Name: "FINER", Type: "LoggingLevel", Static: true}, {Name: "FINEST", Type: "LoggingLevel", Static: true}}},
	{Name: "Limits", Methods: []StandardMethodSpec{{Name: "getQueries", ReturnType: "Integer", Static: true}, {Name: "getLimitQueries", ReturnType: "Integer", Static: true}, {Name: "getQueryRows", ReturnType: "Integer", Static: true}, {Name: "getLimitQueryRows", ReturnType: "Integer", Static: true}, {Name: "getDmlStatements", ReturnType: "Integer", Static: true}, {Name: "getLimitDmlStatements", ReturnType: "Integer", Static: true}, {Name: "getDMLStatements", ReturnType: "Integer", Static: true}, {Name: "getLimitDMLStatements", ReturnType: "Integer", Static: true}, {Name: "getDmlRows", ReturnType: "Integer", Static: true}, {Name: "getLimitDmlRows", ReturnType: "Integer", Static: true}, {Name: "getDMLRows", ReturnType: "Integer", Static: true}, {Name: "getLimitDMLRows", ReturnType: "Integer", Static: true}, {Name: "getHeapSize", ReturnType: "Integer", Static: true}, {Name: "getLimitHeapSize", ReturnType: "Integer", Static: true}, {Name: "getCpuTime", ReturnType: "Integer", Static: true}, {Name: "getLimitCpuTime", ReturnType: "Integer", Static: true}, {Name: "getCallouts", ReturnType: "Integer", Static: true}, {Name: "getLimitCallouts", ReturnType: "Integer", Static: true}, {Name: "getBatchJobs", ReturnType: "Integer", Static: true}, {Name: "getLimitBatchJobs", ReturnType: "Integer", Static: true}, {Name: "getEmailInvocations", ReturnType: "Integer", Static: true}, {Name: "getLimitEmailInvocations", ReturnType: "Integer", Static: true}}},
	{Name: "Assert", Methods: standardAssertMethods()},
	{Name: "System.Assert", Methods: standardAssertMethods()},
	{Name: "Pattern", Methods: []StandardMethodSpec{{Name: "compile", ReturnType: "Pattern", Parameters: []string{"String"}, Static: true}, {Name: "compile", ReturnType: "Pattern", Parameters: []string{"String", "Integer"}, Static: true}, {Name: "matches", ReturnType: "Boolean", Parameters: []string{"String", "String"}, Static: true}, {Name: "quote", ReturnType: "String", Parameters: []string{"String"}, Static: true}, {Name: "matcher", ReturnType: "Matcher", Parameters: []string{"String"}}, {Name: "pattern", ReturnType: "String"}, {Name: "split", ReturnType: "List<String>", Parameters: []string{"String"}}, {Name: "split", ReturnType: "List<String>", Parameters: []string{"String", "Integer"}}}, Properties: []StandardPropertySpec{{Name: "CASE_INSENSITIVE", Type: "Integer", Static: true}, {Name: "COMMENTS", Type: "Integer", Static: true}, {Name: "MULTILINE", Type: "Integer", Static: true}, {Name: "LITERAL", Type: "Integer", Static: true}, {Name: "DOTALL", Type: "Integer", Static: true}, {Name: "UNICODE_CASE", Type: "Integer", Static: true}, {Name: "UNIX_LINES", Type: "Integer", Static: true}, {Name: "CANON_EQ", Type: "Integer", Static: true}}},
	{Name: "Matcher", Methods: []StandardMethodSpec{{Name: "find", ReturnType: "Boolean"}, {Name: "find", ReturnType: "Boolean", Parameters: []string{"Integer"}}, {Name: "matches", ReturnType: "Boolean"}, {Name: "lookingAt", ReturnType: "Boolean"}, {Name: "group", ReturnType: "String"}, {Name: "group", ReturnType: "String", Parameters: []string{"Integer"}}, {Name: "groupCount", ReturnType: "Integer"}, {Name: "start", ReturnType: "Integer"}, {Name: "start", ReturnType: "Integer", Parameters: []string{"Integer"}}, {Name: "end", ReturnType: "Integer"}, {Name: "end", ReturnType: "Integer", Parameters: []string{"Integer"}}, {Name: "replaceAll", ReturnType: "String", Parameters: []string{"String"}}, {Name: "replaceFirst", ReturnType: "String", Parameters: []string{"String"}}, {Name: "reset", ReturnType: "Matcher"}, {Name: "reset", ReturnType: "Matcher", Parameters: []string{"String"}}, {Name: "region", ReturnType: "Matcher", Parameters: []string{"Integer", "Integer"}}, {Name: "regionStart", ReturnType: "Integer"}, {Name: "regionEnd", ReturnType: "Integer"}, {Name: "hasAnchoringBounds", ReturnType: "Boolean"}, {Name: "hasTransparentBounds", ReturnType: "Boolean"}, {Name: "useAnchoringBounds", ReturnType: "Matcher", Parameters: []string{"Boolean"}}, {Name: "useTransparentBounds", ReturnType: "Matcher", Parameters: []string{"Boolean"}}, {Name: "usePattern", ReturnType: "Matcher", Parameters: []string{"Pattern"}}, {Name: "pattern", ReturnType: "Pattern"}, {Name: "quoteReplacement", ReturnType: "String", Parameters: []string{"String"}, Static: true}}},
	{Name: "ApexPages", Methods: []StandardMethodSpec{{Name: "currentPage", ReturnType: "PageReference", Static: true}, {Name: "addMessage", ReturnType: "void", Parameters: []string{"ApexPages.Message"}, Static: true}, {Name: "getMessages", ReturnType: "List<ApexPages.Message>", Static: true}, {Name: "hasMessages", ReturnType: "Boolean", Static: true}, {Name: "hasMessages", ReturnType: "Boolean", Parameters: []string{"ApexPages.Severity"}, Static: true}}},
	{Name: "ApexPages.Message", Constructors: [][]string{{"ApexPages.Severity", "String"}, {"ApexPages.Severity", "String", "String"}}, Methods: []StandardMethodSpec{{Name: "getSummary", ReturnType: "String"}, {Name: "getDetail", ReturnType: "String"}}},
	{Name: "ApexPages.StandardController", Constructors: [][]string{{"SObject"}}, Methods: []StandardMethodSpec{{Name: "view", ReturnType: "PageReference"}, {Name: "save", ReturnType: "PageReference"}, {Name: "delete", ReturnType: "PageReference"}, {Name: "edit", ReturnType: "PageReference"}, {Name: "cancel", ReturnType: "PageReference"}, {Name: "reset", ReturnType: "void"}, {Name: "getId", ReturnType: "Id"}, {Name: "addFields", ReturnType: "void", Parameters: []string{"List<String>"}}}},
	{Name: "ApexPages.StandardSetController", Constructors: [][]string{{"List<SObject>"}, {"Database.QueryLocator"}}, Methods: []StandardMethodSpec{{Name: "getRecords", ReturnType: "List<SObject>"}, {Name: "getResultSize", ReturnType: "Integer"}, {Name: "next", ReturnType: "void"}, {Name: "previous", ReturnType: "void"}, {Name: "getHasNext", ReturnType: "Boolean"}, {Name: "getHasPrevious", ReturnType: "Boolean"}}},
	{Name: "Database", Methods: []StandardMethodSpec{
		{Name: "setSavepoint", ReturnType: "Savepoint", Static: true},
		{Name: "rollback", ReturnType: "void", Parameters: []string{"Savepoint"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"String"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"String", "System.AccessLevel"}, Static: true},
		{Name: "getQueryLocatorWithBinds", ReturnType: "Database.QueryLocator", Parameters: []string{"String", "Map<String,Object>", "System.AccessLevel"}, Static: true},
		{Name: "countQuery", ReturnType: "Integer", Parameters: []string{"String", "System.AccessLevel"}, Static: true},
		{Name: "countQueryWithBinds", ReturnType: "Integer", Parameters: []string{"String", "Map<String,Object>", "System.AccessLevel"}, Static: true},
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
	{Name: "Datetime", Methods: []StandardMethodSpec{{Name: "now", ReturnType: "Datetime", Static: true}, {Name: "newInstance", ReturnType: "Datetime", Parameters: []string{"Integer", "Integer", "Integer"}, Static: true}, {Name: "newInstance", ReturnType: "Datetime", Parameters: []string{"Long"}, Static: true}, {Name: "newInstance", ReturnType: "Datetime", Parameters: []string{"Integer", "Integer", "Integer", "Integer", "Integer", "Integer"}, Static: true}, {Name: "newInstance", ReturnType: "Datetime", Parameters: []string{"Date", "Time"}, Static: true}, {Name: "newInstanceGmt", ReturnType: "Datetime", Parameters: []string{"Integer", "Integer", "Integer"}, Static: true}, {Name: "newInstanceGmt", ReturnType: "Datetime", Parameters: []string{"Integer", "Integer", "Integer", "Integer", "Integer", "Integer"}, Static: true}, {Name: "newInstanceGmt", ReturnType: "Datetime", Parameters: []string{"Date", "Time"}, Static: true}, {Name: "valueOf", ReturnType: "Datetime", Parameters: []string{"String"}, Static: true}, {Name: "valueOf", ReturnType: "Datetime", Parameters: []string{"Object"}, Static: true}, {Name: "valueOfGmt", ReturnType: "Datetime", Parameters: []string{"String"}, Static: true}, {Name: "addDays", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addMonths", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addYears", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addHours", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addMinutes", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addSeconds", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addMilliseconds", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "format", ReturnType: "String"}, {Name: "format", ReturnType: "String", Parameters: []string{"String"}}, {Name: "format", ReturnType: "String", Parameters: []string{"String", "String"}}, {Name: "formatGmt", ReturnType: "String"}, {Name: "formatGmt", ReturnType: "String", Parameters: []string{"String"}}, {Name: "date", ReturnType: "Date"}, {Name: "dateGmt", ReturnType: "Date"}, {Name: "time", ReturnType: "Time"}, {Name: "timeGmt", ReturnType: "Time"}, {Name: "day", ReturnType: "Integer"}, {Name: "month", ReturnType: "Integer"}, {Name: "year", ReturnType: "Integer"}, {Name: "hour", ReturnType: "Integer"}, {Name: "minute", ReturnType: "Integer"}, {Name: "second", ReturnType: "Integer"}, {Name: "millisecond", ReturnType: "Integer"}, {Name: "toString", ReturnType: "String"}}},
	{Name: "Schema", Methods: []StandardMethodSpec{{Name: "getGlobalDescribe", ReturnType: "Map<String,Schema.SObjectType>", Static: true}, {Name: "describeSObjects", ReturnType: "List<Schema.DescribeSObjectResult>", Parameters: []string{"List<String>"}, Static: true}, {Name: "describeTabs", ReturnType: "List<Schema.DescribeTabSetResult>", Static: true}}},
	{Name: "Schema.SObjectType", Methods: []StandardMethodSpec{{Name: "getDescribe", ReturnType: "Schema.DescribeSObjectResult"}, {Name: "getDescribe", ReturnType: "Schema.DescribeSObjectResult", Parameters: []string{"SObjectDescribeOptions"}}, {Name: "newSObject", ReturnType: "SObject"}}},
	{Name: "Schema.SObjectField", Methods: []StandardMethodSpec{{Name: "getDescribe", ReturnType: "Schema.DescribeFieldResult"}}, Properties: []StandardPropertySpec{{Name: "label", Type: "String"}, {Name: "name", Type: "String"}}},
	{Name: "Schema.DescribeSObjectResult", Methods: []StandardMethodSpec{{Name: "getName", ReturnType: "String"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getLabelPlural", ReturnType: "String"}, {Name: "getKeyPrefix", ReturnType: "String"}, {Name: "getFields", ReturnType: "Map<String,Schema.SObjectField>"}, {Name: "getFieldSets", ReturnType: "Schema.FieldSetMap"}, {Name: "getRecordTypeInfos", ReturnType: "List<Schema.RecordTypeInfo>"}, {Name: "getRecordTypeInfosByName", ReturnType: "Map<String,Schema.RecordTypeInfo>"}, {Name: "getRecordTypeInfosByDeveloperName", ReturnType: "Map<String,Schema.RecordTypeInfo>"}, {Name: "getRecordTypeInfosById", ReturnType: "Map<Id,Schema.RecordTypeInfo>"}, {Name: "getChildRelationships", ReturnType: "List<Schema.ChildRelationship>"}, {Name: "getSObjectType", ReturnType: "Schema.SObjectType"}, {Name: "isAccessible", ReturnType: "Boolean"}, {Name: "isCreateable", ReturnType: "Boolean"}, {Name: "isUpdateable", ReturnType: "Boolean"}, {Name: "isDeletable", ReturnType: "Boolean"}, {Name: "isQueryable", ReturnType: "Boolean"}, {Name: "isSearchable", ReturnType: "Boolean"}}, Properties: []StandardPropertySpec{{Name: "fields", Type: "Map<String,Schema.SObjectField>"}, {Name: "fieldSets", Type: "Schema.FieldSetMap"}}},
	{Name: "Schema.FieldSetMap", Methods: []StandardMethodSpec{{Name: "get", ReturnType: "Schema.FieldSet", Parameters: []string{"String"}}, {Name: "getMap", ReturnType: "Map<String,Schema.FieldSet>"}}},
	{Name: "Schema.FieldSet", Methods: []StandardMethodSpec{{Name: "getDescription", ReturnType: "String"}, {Name: "getFields", ReturnType: "List<Schema.FieldSetMember>"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getName", ReturnType: "String"}, {Name: "getNamespace", ReturnType: "String"}, {Name: "getNameSpace", ReturnType: "String"}, {Name: "getSObjectType", ReturnType: "Schema.SObjectType"}}},
	{Name: "Schema.FieldSetMember", Methods: []StandardMethodSpec{{Name: "getFieldPath", ReturnType: "String"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getRequired", ReturnType: "Boolean"}, {Name: "getDBRequired", ReturnType: "Boolean"}, {Name: "getType", ReturnType: "Schema.DisplayType"}, {Name: "getSObjectField", ReturnType: "Schema.SObjectField"}}},
	{Name: "Schema.DescribeTabSetResult", Methods: []StandardMethodSpec{{Name: "getDescription", ReturnType: "String"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getLogoUrl", ReturnType: "String"}, {Name: "getName", ReturnType: "String"}, {Name: "getNamespace", ReturnType: "String"}, {Name: "getTabSetId", ReturnType: "String"}, {Name: "getTabs", ReturnType: "List<Schema.DescribeTabResult>"}, {Name: "isSelected", ReturnType: "Boolean"}}, Properties: []StandardPropertySpec{{Name: "name", Type: "String"}}},
	{Name: "Schema.DescribeTabResult", Methods: []StandardMethodSpec{{Name: "getColors", ReturnType: "List<Schema.DescribeColorResult>"}, {Name: "getIconUrl", ReturnType: "String"}, {Name: "getIcons", ReturnType: "List<Schema.DescribeIconResult>"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getMiniIconUrl", ReturnType: "String"}, {Name: "getMobileUrl", ReturnType: "String"}, {Name: "getName", ReturnType: "String"}, {Name: "getSObjectName", ReturnType: "String"}, {Name: "getTabEnumOrId", ReturnType: "String"}, {Name: "getUrl", ReturnType: "String"}, {Name: "isCustom", ReturnType: "Boolean"}}},
	{Name: "Schema.DescribeColorResult", Methods: []StandardMethodSpec{{Name: "getColor", ReturnType: "String"}, {Name: "getContext", ReturnType: "String"}, {Name: "getTheme", ReturnType: "String"}}},
	{Name: "Schema.DescribeFieldResult", Methods: []StandardMethodSpec{{Name: "getName", ReturnType: "String"}, {Name: "getType", ReturnType: "Schema.DisplayType"}, {Name: "getSoapType", ReturnType: "Schema.SoapType"}, {Name: "getSObjectType", ReturnType: "Schema.SObjectType"}, {Name: "getCompoundFieldName", ReturnType: "String"}, {Name: "isAccessible", ReturnType: "Boolean"}, {Name: "isCreateable", ReturnType: "Boolean"}, {Name: "isUpdateable", ReturnType: "Boolean"}, {Name: "isCalculated", ReturnType: "Boolean"}}, Properties: []StandardPropertySpec{{Name: "compoundFieldName", Type: "String"}}},
	{Name: "Schema.DisplayType", Kind: apexast.DeclarationEnum, Properties: standardEnumProperties("Schema.DisplayType", "ADDRESS", "ANYTYPE", "BASE64", "BOOLEAN", "COMBOBOX", "COMPLEXVALUE", "CURRENCY", "DATACATEGORYGROUPREFERENCE", "DATE", "DATETIME", "DOUBLE", "EMAIL", "ENCRYPTEDSTRING", "FLOATARRAY", "ID", "INTEGER", "JSON", "LOCATION", "LONG", "MULTIPICKLIST", "PERCENT", "PHONE", "PICKLIST", "REFERENCE", "SOBJECT", "STRING", "TEXTAREA", "TEXTARRAY", "TIME", "URL")},
	{Name: "Schema.FieldDescribeOptions", Kind: apexast.DeclarationEnum, Properties: standardEnumProperties("Schema.FieldDescribeOptions", "DEFAULT", "FULL_DESCRIBE")},
	{Name: "Schema.SObjectDescribeOptions", Kind: apexast.DeclarationEnum, Properties: standardEnumProperties("Schema.SObjectDescribeOptions", "DEFAULT", "DEFERRED", "FULL")},
	{Name: "SObjectDescribeOptions", Kind: apexast.DeclarationEnum, Properties: standardEnumProperties("SObjectDescribeOptions", "DEFAULT", "DEFERRED", "FULL")},
	{Name: "Schema.ChildRelationship", Methods: []StandardMethodSpec{{Name: "getRelationshipName", ReturnType: "String"}, {Name: "getChildSObject", ReturnType: "Schema.SObjectType"}, {Name: "getField", ReturnType: "Schema.SObjectField"}, {Name: "isCascadeDelete", ReturnType: "Boolean"}}},
	{Name: "Security", Methods: []StandardMethodSpec{{Name: "stripInaccessible", ReturnType: "SObjectAccessDecision", Parameters: []string{"AccessType", "List<SObject>"}, Static: true}, {Name: "stripInaccessible", ReturnType: "SObjectAccessDecision", Parameters: []string{"AccessType", "List<SObject>", "Boolean"}, Static: true}, {Name: "stripInaccessible", ReturnType: "SObjectAccessDecision", Parameters: []string{"AccessType", "List<SObject>", "Boolean", "Id"}, Static: true}}},
	{Name: "SObjectAccessDecision", Constructors: [][]string{{}}, Methods: []StandardMethodSpec{{Name: "clone", ReturnType: "Object"}, {Name: "getModifiedIndexes", ReturnType: "Set<Integer>"}, {Name: "getRecords", ReturnType: "List<SObject>"}, {Name: "getRemovedFields", ReturnType: "Map<String,Set<String>>"}}},
	{Name: "Address", Methods: []StandardMethodSpec{{Name: "getStreet", ReturnType: "String"}, {Name: "getCity", ReturnType: "String"}, {Name: "getState", ReturnType: "String"}, {Name: "getStateCode", ReturnType: "String"}, {Name: "getPostalCode", ReturnType: "String"}, {Name: "getCountry", ReturnType: "String"}, {Name: "getCountryCode", ReturnType: "String"}, {Name: "getLatitude", ReturnType: "Double"}, {Name: "getLongitude", ReturnType: "Double"}, {Name: "getGeocodeAccuracy", ReturnType: "String"}}},
	{Name: "Metadata.MetadataType", Kind: apexast.DeclarationEnum, Properties: []StandardPropertySpec{{Name: "CustomMetadata", Type: "Metadata.MetadataType", Static: true}}},
	{Name: "Metadata.Operations", Methods: []StandardMethodSpec{{Name: "retrieve", ReturnType: "List<Metadata.CustomMetadata>", Parameters: []string{"Metadata.MetadataType", "List<String>"}, Static: true}, {Name: "enqueueDeployment", ReturnType: "Id", Parameters: []string{"Metadata.DeployContainer", "Metadata.DeployCallback"}, Static: true}, {Name: "checkDeployStatus", ReturnType: "Metadata.DeployResult", Parameters: []string{"Id"}, Static: true}}},
	{Name: "Messaging.SingleEmailMessage", SuperClass: "Messaging.Email", Methods: []StandardMethodSpec{{Name: "setToAddresses", ReturnType: "void", Parameters: []string{"List<String>"}}, {Name: "setSubject", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setPlainTextBody", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setWhatId", ReturnType: "void", Parameters: []string{"Id"}}}},
	{Name: "Messaging", Methods: []StandardMethodSpec{{Name: "sendEmail", ReturnType: "List<Messaging.SendEmailResult>", Parameters: []string{"List<Messaging.Email>"}, Static: true}}},
	{Name: "ConnectApi.Organization", Methods: []StandardMethodSpec{{Name: "getSettings", ReturnType: "ConnectApi.OrganizationSettings", Static: true}}},
	{Name: "ConnectApi.ChatterFeeds", Methods: []StandardMethodSpec{{Name: "postFeedElement", ReturnType: "ConnectApi.FeedElement", Parameters: []string{"String", "ConnectApi.FeedElementInput"}, Static: true}, {Name: "postFeedElement", ReturnType: "ConnectApi.FeedElement", Parameters: []string{"String", "String", "ConnectApi.FeedElementType", "String"}, Static: true}}},
	{Name: "ConnectApi.UserProfiles", Methods: []StandardMethodSpec{{Name: "getUserProfile", ReturnType: "ConnectApi.UserProfile", Parameters: []string{"String", "String"}, Static: true}}},
	{Name: "Auth.AuthConfiguration", Constructors: [][]string{{}, {"String", "String"}}, Methods: []StandardMethodSpec{{Name: "getAuthConfig", ReturnType: "Auth.AuthConfiguration", Static: true}, {Name: "getAuthProviders", ReturnType: "List<AuthProvider>"}, {Name: "getFooterText", ReturnType: "String"}, {Name: "getBackgroundColor", ReturnType: "String"}, {Name: "getStartUrl", ReturnType: "String"}, {Name: "isCommunityUsingSiteAsContainer", ReturnType: "Boolean"}}},
	{Name: "Http", Methods: []StandardMethodSpec{{Name: "send", ReturnType: "HttpResponse", Parameters: []string{"HttpRequest"}}}},
	{Name: "HttpRequest", Constructors: [][]string{{}}, Methods: []StandardMethodSpec{{Name: "setEndpoint", ReturnType: "void", Parameters: []string{"String"}}, {Name: "getEndpoint", ReturnType: "String"}, {Name: "setMethod", ReturnType: "void", Parameters: []string{"String"}}, {Name: "getMethod", ReturnType: "String"}, {Name: "setHeader", ReturnType: "void", Parameters: []string{"String", "String"}}, {Name: "getHeader", ReturnType: "String", Parameters: []string{"String"}}, {Name: "setBody", ReturnType: "void", Parameters: []string{"String"}}, {Name: "getBody", ReturnType: "String"}, {Name: "setTimeout", ReturnType: "void", Parameters: []string{"Integer"}}}},
	{Name: "HttpResponse", Methods: []StandardMethodSpec{{Name: "getStatusCode", ReturnType: "Integer"}, {Name: "getStatus", ReturnType: "String"}, {Name: "getBody", ReturnType: "String"}}},
	{Name: "WebServiceCallout", Methods: []StandardMethodSpec{{Name: "invoke", ReturnType: "void", Parameters: []string{"Object", "Object", "Map<String,Object>", "List<String>"}, Static: true}}},
	{Name: "WebServiceMock", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "doInvoke", ReturnType: "void", Parameters: []string{"Object", "Object", "Map<String,Object>", "String", "String", "String", "String", "String", "String"}}}},
	{Name: "UserInfo", Methods: []StandardMethodSpec{{Name: "getUserId", ReturnType: "Id", Static: true}, {Name: "getProfileId", ReturnType: "Id", Static: true}, {Name: "getUserName", ReturnType: "String", Static: true}, {Name: "getName", ReturnType: "String", Static: true}, {Name: "getFirstName", ReturnType: "String", Static: true}, {Name: "getLastName", ReturnType: "String", Static: true}, {Name: "getUserEmail", ReturnType: "String", Static: true}, {Name: "getOrganizationId", ReturnType: "Id", Static: true}, {Name: "getUserType", ReturnType: "String", Static: true}, {Name: "getSessionId", ReturnType: "String", Static: true}, {Name: "getLocale", ReturnType: "String", Static: true}, {Name: "getLanguage", ReturnType: "String", Static: true}, {Name: "getTimeZone", ReturnType: "TimeZone", Static: true}, {Name: "isMultiCurrencyOrganization", ReturnType: "Boolean", Static: true}}},
	{Name: "UUID", Methods: []StandardMethodSpec{{Name: "randomUUID", ReturnType: "String", Static: true}}},
	{Name: "Callable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "call", ReturnType: "Object", Parameters: []string{"String", "Map<String,Object>"}}}},
	{Name: "Comparator", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "compare", ReturnType: "Integer", Parameters: []string{"Object", "Object"}}}},
	{Name: "Database.Batchable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "start", ReturnType: "Iterable", Parameters: []string{"Database.BatchableContext"}}, {Name: "execute", ReturnType: "void", Parameters: []string{"Database.BatchableContext", "List<Object>"}}, {Name: "finish", ReturnType: "void", Parameters: []string{"Database.BatchableContext"}}}},
	{Name: "Database.Stateful", Kind: apexast.DeclarationInterface},
	{Name: "HttpCalloutMock", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "respond", ReturnType: "HttpResponse", Parameters: []string{"HttpRequest"}}}},
	{Name: "Iterable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "iterator", ReturnType: "Iterator"}}},
	{Name: "Iterator", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "hasNext", ReturnType: "Boolean"}, {Name: "next", ReturnType: "Object"}}},
	{Name: "Queueable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "execute", ReturnType: "void", Parameters: []string{"QueueableContext"}}}},
	{Name: "Schedulable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "execute", ReturnType: "void", Parameters: []string{"SchedulableContext"}}}},
	{Name: "AsyncOptions", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "DuplicateSignature", Type: "Object"}, {Name: "MaximumQueueableStackDepth", Type: "Integer"}, {Name: "MinimumQueueableDelayInMinutes", Type: "Integer"}}},
	{Name: "Database.DMLOptions", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "AllowFieldTruncation", Type: "Boolean"}, {Name: "AssignmentRuleHeader", Type: "Database.AssignmentRuleHeader"}, {Name: "DuplicateRuleHeader", Type: "Database.DuplicateRuleHeader"}, {Name: "EmailHeader", Type: "Database.EmailHeader"}, {Name: "LocaleOptions", Type: "Object"}, {Name: "LocalizeErrors", Type: "Boolean"}, {Name: "OptAllOrNone", Type: "Boolean"}}},
	{Name: "Database.AssignmentRuleHeader", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "AssignmentRuleId", Type: "Id"}, {Name: "UseDefaultRule", Type: "Boolean"}}},
	{Name: "Database.DuplicateRuleHeader", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "AllowSave", Type: "Boolean"}, {Name: "RunAsCurrentUser", Type: "Boolean"}}},
	{Name: "Database.EmailHeader", Constructors: [][]string{{}}, Properties: []StandardPropertySpec{{Name: "TriggerAutoResponseEmail", Type: "Boolean"}, {Name: "TriggerOtherEmail", Type: "Boolean"}, {Name: "TriggerUserEmail", Type: "Boolean"}}},
	{Name: "Database.Error", Methods: []StandardMethodSpec{{Name: "getFields", ReturnType: "List<String>"}, {Name: "getMessage", ReturnType: "String"}, {Name: "getStatusCode", ReturnType: "StatusCode"}, {Name: "getExtendedErrorDetails", ReturnType: "List<Object>"}}},
	{Name: "Database.DuplicateError", SuperClass: "Database.Error", Methods: []StandardMethodSpec{{Name: "getDuplicateResult", ReturnType: "Datacloud.DuplicateResult"}, {Name: "getFields", ReturnType: "List<String>"}, {Name: "getMessage", ReturnType: "String"}, {Name: "getStatusCode", ReturnType: "StatusCode"}}},
	{Name: "Datacloud.DuplicateResult", Methods: []StandardMethodSpec{{Name: "getDuplicateRule", ReturnType: "String"}, {Name: "getDuplicateRuleEntityType", ReturnType: "String"}, {Name: "getErrorMessage", ReturnType: "String"}, {Name: "getMatchResults", ReturnType: "List<Datacloud.MatchResult>"}, {Name: "isAllowSave", ReturnType: "Boolean"}}},
	{Name: "Datacloud.MatchResult", Methods: []StandardMethodSpec{{Name: "getEntityType", ReturnType: "String"}, {Name: "getErrors", ReturnType: "List<Database.Error>"}, {Name: "getMatchEngine", ReturnType: "String"}, {Name: "getMatchRecords", ReturnType: "List<Datacloud.MatchRecord>"}, {Name: "getRule", ReturnType: "String"}, {Name: "getSize", ReturnType: "Integer"}, {Name: "isSuccess", ReturnType: "Boolean"}}},
	{Name: "Datacloud.MatchRecord", Methods: []StandardMethodSpec{{Name: "getAdditionalInformation", ReturnType: "List<Datacloud.AdditionalInformationMap>"}, {Name: "getFieldDiffs", ReturnType: "List<Datacloud.FieldDiff>"}, {Name: "getMatchConfidence", ReturnType: "Double"}, {Name: "getRecord", ReturnType: "SObject"}}},
	{Name: "Datacloud.FieldDiff", Methods: []StandardMethodSpec{{Name: "getDifference", ReturnType: "String"}, {Name: "getName", ReturnType: "String"}}},
	{Name: "System.Callable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "call", ReturnType: "Object", Parameters: []string{"String", "Map<String,Object>"}}}},
	{Name: "System.StubProvider", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "handleMethodCall", ReturnType: "Object", Parameters: []string{"Object", "String", "Type", "List<Type>", "List<String>", "List<Object>"}}}},
}

func standardAssertMethods() []StandardMethodSpec {
	return []StandardMethodSpec{
		{Name: "areEqual", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true},
		{Name: "areEqual", ReturnType: "void", Parameters: []string{"Object", "Object", "Object"}, Static: true},
		{Name: "areNotEqual", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true},
		{Name: "areNotEqual", ReturnType: "void", Parameters: []string{"Object", "Object", "Object"}, Static: true},
		{Name: "isTrue", ReturnType: "void", Parameters: []string{"Boolean"}, Static: true},
		{Name: "isTrue", ReturnType: "void", Parameters: []string{"Boolean", "Object"}, Static: true},
		{Name: "isFalse", ReturnType: "void", Parameters: []string{"Boolean"}, Static: true},
		{Name: "isFalse", ReturnType: "void", Parameters: []string{"Boolean", "Object"}, Static: true},
		{Name: "isNull", ReturnType: "void", Parameters: []string{"Object"}, Static: true},
		{Name: "isNull", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true},
		{Name: "isNotNull", ReturnType: "void", Parameters: []string{"Object"}, Static: true},
		{Name: "isNotNull", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true},
		{Name: "fail", ReturnType: "void", Static: true},
		{Name: "fail", ReturnType: "void", Parameters: []string{"Object"}, Static: true},
	}
}
