package typesys

import (
	"sort"
	"strconv"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
)

type StandardSymbolSpec struct {
	Name         string
	Kind         apexast.DeclarationKind
	SuperClass   string
	Interfaces   []string
	Constructors [][]string
	Methods      []StandardMethodSpec
	Properties   []StandardPropertySpec
}

type StandardMethodSpec struct {
	Name       string
	ReturnType string
	Parameters []string
	Static     bool
}

type StandardPropertySpec struct {
	Name   string
	Type   string
	Static bool
}

func StandardPlatformSymbols() []TypeSymbol {
	specs := append([]StandardSymbolSpec(nil), standardPlatformSymbolSpecs...)
	specs = append(specs, productNamespaceSymbolSpecs...)
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
		sym := TypeSymbol{
			Kind:       kind,
			Name:       spec.Name,
			File:       "<standard-platform>",
			Dependency: true,
			SuperClass: spec.SuperClass,
			Interfaces: append([]string(nil), spec.Interfaces...),
		}
		if namespace, localName, ok := splitStandardSymbolName(spec.Name); ok {
			sym.Namespace = namespace
			sym.Name = localName
		}
		for _, ctor := range spec.Constructors {
			sym.Members = append(sym.Members, MemberSymbol{
				Kind:       apexast.DeclarationConstructor,
				Name:       localStandardSymbolName(spec.Name),
				Modifiers:  []string{"public"},
				Parameters: standardParameters(ctor),
			})
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
				Parameters: standardParameters(method.Parameters),
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
	for _, value := range values {
		seen[standardPropertyKey(value)] = true
	}
	for _, addition := range additions {
		key := standardPropertyKey(addition)
		if seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, addition)
	}
	return values
}

func standardMethodKey(method StandardMethodSpec) string {
	return strings.ToLower(method.Name) + "|" + strconv.FormatBool(method.Static) + "|" + standardTypeListKey(method.Parameters)
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
	params := make([]apexast.Parameter, 0, len(types))
	for i, typ := range types {
		params = append(params, apexast.Parameter{Name: standardParameterName(i), Type: typ})
	}
	return params
}

func standardParameterName(index int) string {
	return "arg" + strconv.Itoa(index)
}

var standardPlatformTypeNames = []string{
	"AccessLevel",
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
	"CronJobDetail",
	"CronTrigger",
	"Database",
	"Database.BatchableContext",
	"Database.DeleteResult",
	"Database.DMLOptions",
	"Database.LeadConvert",
	"Database.LeadConvertResult",
	"Database.MergeResult",
	"Database.QueryLocator",
	"Database.SaveResult",
	"Database.Savepoint",
	"Database.UndeleteResult",
	"Database.UpsertResult",
	"Dom",
	"Dom.Document",
	"Dom.XmlNode",
	"Dom.XmlNodeType",
	"FeatureManagement",
	"Http",
	"HttpCalloutMock",
	"HttpRequest",
	"HttpResponse",
	"InstallContext",
	"InstallHandler",
	"Folder",
	"Iterable",
	"Iterator",
	"JSONGenerator",
	"JSONParser",
	"JSONToken",
	"Limits",
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
	"Schema.DescribeFieldResult",
	"Schema.DescribeSObjectResult",
	"Schema.DescribeTabResult",
	"Schema.DescribeTabSetResult",
	"Schema.FieldSet",
	"Schema.FieldSetMember",
	"Schema.PicklistEntry",
	"Schema.RecordTypeInfo",
	"Schema.DisplayType",
	"Schema.SObjectField",
	"Schema.SObjectType",
	"Schema.SoapType",
	"Search",
	"SelectOption",
	"Site",
	"Site.ExternalUserCreateException",
	"Site.UrlRewriter",
	"SObjectField",
	"SObjectType",
	"SoapType",
	"DisplayType",
	"System",
	"System.Assert",
	"Test",
	"TimeZone",
	"TriggerOperation",
	"UserInfo",
	"UserManagement",
}

var standardPlatformSymbolSpecs = []StandardSymbolSpec{
	{Name: "Version", Constructors: [][]string{{"Integer", "Integer"}, {"Integer", "Integer", "Integer"}}, Methods: []StandardMethodSpec{{Name: "compareTo", ReturnType: "Integer", Parameters: []string{"Version"}}}},
	{Name: "InstallContext", Methods: []StandardMethodSpec{{Name: "previousVersion", ReturnType: "Version"}}},
	{Name: "URL", Constructors: [][]string{{"URL", "String"}}, Methods: []StandardMethodSpec{{Name: "getSalesforceBaseUrl", ReturnType: "URL", Static: true}, {Name: "toExternalForm", ReturnType: "String"}}},
	{Name: "PageReference", Constructors: [][]string{{"String"}}, Methods: []StandardMethodSpec{{Name: "getUrl", ReturnType: "String"}, {Name: "setRedirect", ReturnType: "PageReference", Parameters: []string{"Boolean"}}, {Name: "getParameters", ReturnType: "Map<String,String>"}}},
	{Name: "SelectOption", Constructors: [][]string{{"String", "String"}, {"String", "String", "Boolean"}, {"String", "String", "Boolean", "Boolean"}}},
	{Name: "Search", Methods: []StandardMethodSpec{{Name: "query", ReturnType: "List<List<SObject>>", Parameters: []string{"String"}, Static: true}}},
	{Name: "System", Methods: []StandardMethodSpec{{Name: "now", ReturnType: "Datetime", Static: true}, {Name: "today", ReturnType: "Date", Static: true}, {Name: "debug", ReturnType: "void", Parameters: []string{"Object"}, Static: true}, {Name: "assert", ReturnType: "void", Parameters: []string{"Boolean"}, Static: true}, {Name: "assert", ReturnType: "void", Parameters: []string{"Boolean", "Object"}, Static: true}, {Name: "assertEquals", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true}, {Name: "assertEquals", ReturnType: "void", Parameters: []string{"Object", "Object", "Object"}, Static: true}, {Name: "assertNotEquals", ReturnType: "void", Parameters: []string{"Object", "Object"}, Static: true}, {Name: "assertNotEquals", ReturnType: "void", Parameters: []string{"Object", "Object", "Object"}, Static: true}}},
	{Name: "TriggerOperation", Kind: apexast.DeclarationEnum, Properties: []StandardPropertySpec{{Name: "BEFORE_INSERT", Type: "TriggerOperation", Static: true}, {Name: "BEFORE_UPDATE", Type: "TriggerOperation", Static: true}, {Name: "BEFORE_DELETE", Type: "TriggerOperation", Static: true}, {Name: "AFTER_INSERT", Type: "TriggerOperation", Static: true}, {Name: "AFTER_UPDATE", Type: "TriggerOperation", Static: true}, {Name: "AFTER_DELETE", Type: "TriggerOperation", Static: true}, {Name: "AFTER_UNDELETE", Type: "TriggerOperation", Static: true}}},
	{Name: "Limits", Methods: []StandardMethodSpec{{Name: "getQueries", ReturnType: "Integer", Static: true}, {Name: "getLimitQueries", ReturnType: "Integer", Static: true}, {Name: "getQueryRows", ReturnType: "Integer", Static: true}, {Name: "getLimitQueryRows", ReturnType: "Integer", Static: true}, {Name: "getDmlStatements", ReturnType: "Integer", Static: true}, {Name: "getLimitDmlStatements", ReturnType: "Integer", Static: true}, {Name: "getDMLStatements", ReturnType: "Integer", Static: true}, {Name: "getLimitDMLStatements", ReturnType: "Integer", Static: true}, {Name: "getDmlRows", ReturnType: "Integer", Static: true}, {Name: "getLimitDmlRows", ReturnType: "Integer", Static: true}, {Name: "getDMLRows", ReturnType: "Integer", Static: true}, {Name: "getLimitDMLRows", ReturnType: "Integer", Static: true}, {Name: "getHeapSize", ReturnType: "Integer", Static: true}, {Name: "getLimitHeapSize", ReturnType: "Integer", Static: true}, {Name: "getCpuTime", ReturnType: "Integer", Static: true}, {Name: "getLimitCpuTime", ReturnType: "Integer", Static: true}, {Name: "getCallouts", ReturnType: "Integer", Static: true}, {Name: "getLimitCallouts", ReturnType: "Integer", Static: true}, {Name: "getEmailInvocations", ReturnType: "Integer", Static: true}, {Name: "getLimitEmailInvocations", ReturnType: "Integer", Static: true}}},
	{Name: "Assert", Methods: standardAssertMethods()},
	{Name: "System.Assert", Methods: standardAssertMethods()},
	{Name: "Pattern", Methods: []StandardMethodSpec{{Name: "compile", ReturnType: "Pattern", Parameters: []string{"String"}, Static: true}, {Name: "compile", ReturnType: "Pattern", Parameters: []string{"String", "Integer"}, Static: true}, {Name: "matches", ReturnType: "Boolean", Parameters: []string{"String", "String"}, Static: true}, {Name: "quote", ReturnType: "String", Parameters: []string{"String"}, Static: true}, {Name: "matcher", ReturnType: "Matcher", Parameters: []string{"String"}}, {Name: "pattern", ReturnType: "String"}, {Name: "split", ReturnType: "List<String>", Parameters: []string{"String"}}, {Name: "split", ReturnType: "List<String>", Parameters: []string{"String", "Integer"}}}, Properties: []StandardPropertySpec{{Name: "CASE_INSENSITIVE", Type: "Integer", Static: true}, {Name: "COMMENTS", Type: "Integer", Static: true}, {Name: "MULTILINE", Type: "Integer", Static: true}, {Name: "LITERAL", Type: "Integer", Static: true}, {Name: "DOTALL", Type: "Integer", Static: true}, {Name: "UNICODE_CASE", Type: "Integer", Static: true}, {Name: "UNIX_LINES", Type: "Integer", Static: true}, {Name: "CANON_EQ", Type: "Integer", Static: true}}},
	{Name: "Matcher", Methods: []StandardMethodSpec{{Name: "find", ReturnType: "Boolean"}, {Name: "find", ReturnType: "Boolean", Parameters: []string{"Integer"}}, {Name: "matches", ReturnType: "Boolean"}, {Name: "lookingAt", ReturnType: "Boolean"}, {Name: "group", ReturnType: "String"}, {Name: "group", ReturnType: "String", Parameters: []string{"Integer"}}, {Name: "groupCount", ReturnType: "Integer"}, {Name: "start", ReturnType: "Integer"}, {Name: "start", ReturnType: "Integer", Parameters: []string{"Integer"}}, {Name: "end", ReturnType: "Integer"}, {Name: "end", ReturnType: "Integer", Parameters: []string{"Integer"}}, {Name: "replaceAll", ReturnType: "String", Parameters: []string{"String"}}, {Name: "replaceFirst", ReturnType: "String", Parameters: []string{"String"}}, {Name: "reset", ReturnType: "Matcher"}, {Name: "reset", ReturnType: "Matcher", Parameters: []string{"String"}}, {Name: "region", ReturnType: "Matcher", Parameters: []string{"Integer", "Integer"}}, {Name: "regionStart", ReturnType: "Integer"}, {Name: "regionEnd", ReturnType: "Integer"}, {Name: "hasAnchoringBounds", ReturnType: "Boolean"}, {Name: "hasTransparentBounds", ReturnType: "Boolean"}, {Name: "useAnchoringBounds", ReturnType: "Matcher", Parameters: []string{"Boolean"}}, {Name: "useTransparentBounds", ReturnType: "Matcher", Parameters: []string{"Boolean"}}, {Name: "usePattern", ReturnType: "Matcher", Parameters: []string{"Pattern"}}, {Name: "pattern", ReturnType: "Pattern"}, {Name: "quoteReplacement", ReturnType: "String", Parameters: []string{"String"}, Static: true}}},
	{Name: "ApexPages", Methods: []StandardMethodSpec{{Name: "currentPage", ReturnType: "PageReference", Static: true}, {Name: "addMessage", ReturnType: "void", Parameters: []string{"ApexPages.Message"}, Static: true}, {Name: "getMessages", ReturnType: "List<ApexPages.Message>", Static: true}, {Name: "hasMessages", ReturnType: "Boolean", Static: true}, {Name: "hasMessages", ReturnType: "Boolean", Parameters: []string{"ApexPages.Severity"}, Static: true}}},
	{Name: "ApexPages.Message", Constructors: [][]string{{"ApexPages.Severity", "String"}, {"ApexPages.Severity", "String", "String"}}, Methods: []StandardMethodSpec{{Name: "getSummary", ReturnType: "String"}, {Name: "getDetail", ReturnType: "String"}}},
	{Name: "ApexPages.StandardController", Constructors: [][]string{{"SObject"}}, Methods: []StandardMethodSpec{{Name: "view", ReturnType: "PageReference"}, {Name: "save", ReturnType: "PageReference"}, {Name: "delete", ReturnType: "PageReference"}, {Name: "edit", ReturnType: "PageReference"}, {Name: "cancel", ReturnType: "PageReference"}, {Name: "reset", ReturnType: "void"}, {Name: "getId", ReturnType: "Id"}, {Name: "addFields", ReturnType: "void", Parameters: []string{"List<String>"}}}},
	{Name: "ApexPages.StandardSetController", Constructors: [][]string{{"List<SObject>"}, {"Database.QueryLocator"}}, Methods: []StandardMethodSpec{{Name: "getRecords", ReturnType: "List<SObject>"}, {Name: "next", ReturnType: "void"}, {Name: "previous", ReturnType: "void"}, {Name: "getHasNext", ReturnType: "Boolean"}, {Name: "getHasPrevious", ReturnType: "Boolean"}}},
	{Name: "Database", Methods: []StandardMethodSpec{
		{Name: "setSavepoint", ReturnType: "Savepoint", Static: true},
		{Name: "rollback", ReturnType: "void", Parameters: []string{"Savepoint"}, Static: true},
		{Name: "getQueryLocator", ReturnType: "Database.QueryLocator", Parameters: []string{"String"}, Static: true},
		{Name: "insert", ReturnType: "Database.SaveResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "insert", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "insert", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Database.DMLOptions"}, Static: true},
		{Name: "update", ReturnType: "Database.SaveResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "update", ReturnType: "Database.SaveResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "update", ReturnType: "List<Database.SaveResult>", Parameters: []string{"List<SObject>", "Database.DMLOptions"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"Id"}, Static: true},
		{Name: "delete", ReturnType: "Database.DeleteResult", Parameters: []string{"Id", "Boolean"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Id>"}, Static: true},
		{Name: "delete", ReturnType: "List<Database.DeleteResult>", Parameters: []string{"List<Id>", "Boolean"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "Boolean"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "Schema.SObjectField"}, Static: true},
		{Name: "upsert", ReturnType: "Database.UpsertResult", Parameters: []string{"SObject", "Schema.SObjectField", "Boolean"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "Boolean"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "Schema.SObjectField"}, Static: true},
		{Name: "upsert", ReturnType: "List<Database.UpsertResult>", Parameters: []string{"List<SObject>", "Schema.SObjectField", "Boolean"}, Static: true},
	}},
	{Name: "Date", Methods: []StandardMethodSpec{{Name: "today", ReturnType: "Date", Static: true}, {Name: "newInstance", ReturnType: "Date", Parameters: []string{"Integer", "Integer", "Integer"}, Static: true}, {Name: "valueOf", ReturnType: "Date", Parameters: []string{"String"}, Static: true}, {Name: "addDays", ReturnType: "Date", Parameters: []string{"Integer"}}, {Name: "addMonths", ReturnType: "Date", Parameters: []string{"Integer"}}, {Name: "addYears", ReturnType: "Date", Parameters: []string{"Integer"}}, {Name: "daysBetween", ReturnType: "Integer", Parameters: []string{"Date"}}, {Name: "day", ReturnType: "Integer"}, {Name: "month", ReturnType: "Integer"}, {Name: "year", ReturnType: "Integer"}, {Name: "toStartOfMonth", ReturnType: "Date"}, {Name: "toEndOfMonth", ReturnType: "Date"}, {Name: "format", ReturnType: "String"}, {Name: "toString", ReturnType: "String"}}},
	{Name: "Datetime", Methods: []StandardMethodSpec{{Name: "now", ReturnType: "Datetime", Static: true}, {Name: "newInstance", ReturnType: "Datetime", Parameters: []string{"Integer", "Integer", "Integer"}, Static: true}, {Name: "newInstance", ReturnType: "Datetime", Parameters: []string{"Long"}, Static: true}, {Name: "newInstance", ReturnType: "Datetime", Parameters: []string{"Integer", "Integer", "Integer", "Integer", "Integer", "Integer"}, Static: true}, {Name: "newInstance", ReturnType: "Datetime", Parameters: []string{"Date", "Time"}, Static: true}, {Name: "newInstanceGmt", ReturnType: "Datetime", Parameters: []string{"Integer", "Integer", "Integer", "Integer", "Integer", "Integer"}, Static: true}, {Name: "newInstanceGmt", ReturnType: "Datetime", Parameters: []string{"Date", "Time"}, Static: true}, {Name: "valueOf", ReturnType: "Datetime", Parameters: []string{"String"}, Static: true}, {Name: "valueOfGmt", ReturnType: "Datetime", Parameters: []string{"String"}, Static: true}, {Name: "addDays", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addMonths", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addYears", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addHours", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addMinutes", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addSeconds", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "addMilliseconds", ReturnType: "Datetime", Parameters: []string{"Integer"}}, {Name: "format", ReturnType: "String"}, {Name: "format", ReturnType: "String", Parameters: []string{"String"}}, {Name: "format", ReturnType: "String", Parameters: []string{"String", "String"}}, {Name: "formatGmt", ReturnType: "String"}, {Name: "formatGmt", ReturnType: "String", Parameters: []string{"String"}}, {Name: "date", ReturnType: "Date"}, {Name: "dateGmt", ReturnType: "Date"}, {Name: "time", ReturnType: "Time"}, {Name: "timeGmt", ReturnType: "Time"}, {Name: "day", ReturnType: "Integer"}, {Name: "month", ReturnType: "Integer"}, {Name: "year", ReturnType: "Integer"}, {Name: "hour", ReturnType: "Integer"}, {Name: "minute", ReturnType: "Integer"}, {Name: "second", ReturnType: "Integer"}, {Name: "millisecond", ReturnType: "Integer"}, {Name: "toString", ReturnType: "String"}}},
	{Name: "Schema", Methods: []StandardMethodSpec{{Name: "getGlobalDescribe", ReturnType: "Map<String,Schema.SObjectType>", Static: true}, {Name: "describeSObjects", ReturnType: "List<Schema.DescribeSObjectResult>", Parameters: []string{"List<String>"}, Static: true}}},
	{Name: "Schema.SObjectType", Methods: []StandardMethodSpec{{Name: "getDescribe", ReturnType: "Schema.DescribeSObjectResult"}, {Name: "newSObject", ReturnType: "SObject"}}},
	{Name: "Schema.SObjectField", Methods: []StandardMethodSpec{{Name: "getDescribe", ReturnType: "Schema.DescribeFieldResult"}}},
	{Name: "Schema.DescribeSObjectResult", Methods: []StandardMethodSpec{{Name: "getName", ReturnType: "String"}, {Name: "getLabel", ReturnType: "String"}, {Name: "getLabelPlural", ReturnType: "String"}, {Name: "getKeyPrefix", ReturnType: "String"}, {Name: "getFields", ReturnType: "Map<String,Schema.SObjectField>"}, {Name: "getRecordTypeInfos", ReturnType: "List<Schema.RecordTypeInfo>"}, {Name: "getRecordTypeInfosByName", ReturnType: "Map<String,Schema.RecordTypeInfo>"}, {Name: "getRecordTypeInfosByDeveloperName", ReturnType: "Map<String,Schema.RecordTypeInfo>"}, {Name: "getRecordTypeInfosById", ReturnType: "Map<Id,Schema.RecordTypeInfo>"}, {Name: "getChildRelationships", ReturnType: "List<Schema.ChildRelationship>"}, {Name: "getSObjectType", ReturnType: "Schema.SObjectType"}, {Name: "isAccessible", ReturnType: "Boolean"}, {Name: "isCreateable", ReturnType: "Boolean"}, {Name: "isUpdateable", ReturnType: "Boolean"}, {Name: "isDeletable", ReturnType: "Boolean"}, {Name: "isQueryable", ReturnType: "Boolean"}, {Name: "isSearchable", ReturnType: "Boolean"}}, Properties: []StandardPropertySpec{{Name: "fields", Type: "Map<String,Schema.SObjectField>"}}},
	{Name: "Schema.DescribeFieldResult", Methods: []StandardMethodSpec{{Name: "getName", ReturnType: "String"}, {Name: "getType", ReturnType: "Schema.DisplayType"}, {Name: "getSOAPType", ReturnType: "Schema.SoapType"}, {Name: "getSoapType", ReturnType: "Schema.SoapType"}, {Name: "isAccessible", ReturnType: "Boolean"}, {Name: "isCreateable", ReturnType: "Boolean"}, {Name: "isUpdateable", ReturnType: "Boolean"}, {Name: "isCalculated", ReturnType: "Boolean"}}},
	{Name: "Schema.DisplayType", Kind: apexast.DeclarationEnum, Properties: []StandardPropertySpec{{Name: "BOOLEAN", Type: "Schema.DisplayType", Static: true}, {Name: "CURRENCY", Type: "Schema.DisplayType", Static: true}, {Name: "DATE", Type: "Schema.DisplayType", Static: true}, {Name: "DATETIME", Type: "Schema.DisplayType", Static: true}, {Name: "DOUBLE", Type: "Schema.DisplayType", Static: true}, {Name: "ID", Type: "Schema.DisplayType", Static: true}, {Name: "INTEGER", Type: "Schema.DisplayType", Static: true}, {Name: "PERCENT", Type: "Schema.DisplayType", Static: true}, {Name: "PICKLIST", Type: "Schema.DisplayType", Static: true}, {Name: "REFERENCE", Type: "Schema.DisplayType", Static: true}, {Name: "STRING", Type: "Schema.DisplayType", Static: true}, {Name: "TEXTAREA", Type: "Schema.DisplayType", Static: true}}},
	{Name: "Schema.ChildRelationship", Methods: []StandardMethodSpec{{Name: "getRelationshipName", ReturnType: "String"}, {Name: "getChildSObject", ReturnType: "Schema.SObjectType"}, {Name: "getField", ReturnType: "Schema.SObjectField"}, {Name: "isCascadeDelete", ReturnType: "Boolean"}}},
	{Name: "Metadata.MetadataType", Kind: apexast.DeclarationEnum, Properties: []StandardPropertySpec{{Name: "CustomMetadata", Type: "Metadata.MetadataType", Static: true}}},
	{Name: "Metadata.Operations", Methods: []StandardMethodSpec{{Name: "retrieve", ReturnType: "List<Metadata.CustomMetadata>", Parameters: []string{"Metadata.MetadataType", "List<String>"}, Static: true}, {Name: "enqueueDeployment", ReturnType: "Id", Parameters: []string{"Metadata.DeployContainer", "Metadata.DeployCallback"}, Static: true}, {Name: "checkDeployStatus", ReturnType: "Metadata.DeployResult", Parameters: []string{"Id"}, Static: true}}},
	{Name: "Messaging.SingleEmailMessage", SuperClass: "Messaging.Email", Methods: []StandardMethodSpec{{Name: "setToAddresses", ReturnType: "void", Parameters: []string{"List<String>"}}, {Name: "setSubject", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setPlainTextBody", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setWhatId", ReturnType: "void", Parameters: []string{"Id"}}}},
	{Name: "Messaging", Methods: []StandardMethodSpec{{Name: "sendEmail", ReturnType: "List<Messaging.SendEmailResult>", Parameters: []string{"List<Messaging.Email>"}, Static: true}}},
	{Name: "ConnectApi.Organization", Methods: []StandardMethodSpec{{Name: "getSettings", ReturnType: "ConnectApi.OrganizationSettings", Static: true}}},
	{Name: "ConnectApi.ChatterFeeds", Methods: []StandardMethodSpec{{Name: "postFeedElement", ReturnType: "ConnectApi.FeedElement", Parameters: []string{"String", "ConnectApi.FeedElementInput"}, Static: true}, {Name: "postFeedElement", ReturnType: "ConnectApi.FeedElement", Parameters: []string{"String", "String", "ConnectApi.FeedElementType", "String"}, Static: true}}},
	{Name: "ConnectApi.UserProfiles", Methods: []StandardMethodSpec{{Name: "getUserProfile", ReturnType: "ConnectApi.UserProfile", Parameters: []string{"String", "String"}, Static: true}}},
	{Name: "Auth.AuthConfiguration", Constructors: [][]string{{}}, Methods: []StandardMethodSpec{{Name: "getAuthConfig", ReturnType: "Auth.AuthConfiguration", Static: true}, {Name: "getAuthProviders", ReturnType: "List<String>"}, {Name: "getFooterText", ReturnType: "String"}, {Name: "getBackgroundColor", ReturnType: "String"}}},
	{Name: "Http", Methods: []StandardMethodSpec{{Name: "send", ReturnType: "HttpResponse", Parameters: []string{"HttpRequest"}}}},
	{Name: "HttpRequest", Methods: []StandardMethodSpec{{Name: "setEndpoint", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setMethod", ReturnType: "void", Parameters: []string{"String"}}, {Name: "setHeader", ReturnType: "void", Parameters: []string{"String", "String"}}, {Name: "setBody", ReturnType: "void", Parameters: []string{"String"}}}},
	{Name: "HttpResponse", Methods: []StandardMethodSpec{{Name: "getStatusCode", ReturnType: "Integer"}, {Name: "getStatus", ReturnType: "String"}, {Name: "getBody", ReturnType: "String"}}},
	{Name: "UserInfo", Methods: []StandardMethodSpec{{Name: "getUserId", ReturnType: "Id", Static: true}, {Name: "getProfileId", ReturnType: "Id", Static: true}, {Name: "getUserName", ReturnType: "String", Static: true}, {Name: "getName", ReturnType: "String", Static: true}, {Name: "getFirstName", ReturnType: "String", Static: true}, {Name: "getLastName", ReturnType: "String", Static: true}, {Name: "getUserEmail", ReturnType: "String", Static: true}, {Name: "getOrganizationId", ReturnType: "Id", Static: true}, {Name: "getUserType", ReturnType: "String", Static: true}, {Name: "getSessionId", ReturnType: "String", Static: true}, {Name: "getLocale", ReturnType: "String", Static: true}, {Name: "getLanguage", ReturnType: "String", Static: true}, {Name: "getTimeZone", ReturnType: "TimeZone", Static: true}, {Name: "isMultiCurrencyOrganization", ReturnType: "Boolean", Static: true}}},
	{Name: "Callable", Kind: apexast.DeclarationInterface, Methods: []StandardMethodSpec{{Name: "call", ReturnType: "Object", Parameters: []string{"String", "Map<String,Object>"}}}},
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
