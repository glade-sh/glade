package surfaceledger

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/capability"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
)

func BuildGladeSnapshot() []SurfaceLedgerRow {
	byID := map[string]SurfaceLedgerRow{}
	for _, symbol := range typesys.StandardPlatformSymbolView() {
		namespace, typeName := splitTypeName(symbol.Namespace, symbol.Name)
		id := ApexTypeID(namespace, typeName)
		byID[surfaceIDKey(id)] = RowFromGladeShape(SurfaceLedgerRow{
			SurfaceID:     id,
			Product:       ProductApex,
			Area:          AreaRuntime,
			Namespace:     namespace,
			TypeName:      typeName,
			Kind:          KindType,
			GladeBehavior: behaviorForStandardType(namespace, typeName),
			Sources:       []string{"standard-symbols"},
		})
		for _, member := range symbol.Members {
			params := memberParameterTypes(member.Parameters)
			memberName := member.Name
			if memberName == "" {
				memberName = typeName
			}
			kind := gladeMemberKind(string(member.Kind))
			if string(member.Kind) == "constructor" {
				memberName = gladeConstructorMemberName(namespace, typeName, memberName)
			}
			if kind == KindProperty {
				params = nil
			}
			memberID := ApexMemberID(namespace, typeName, memberName, params)
			row := RowFromGladeShape(SurfaceLedgerRow{
				SurfaceID:  memberID,
				Product:    ProductApex,
				Area:       AreaRuntime,
				Namespace:  namespace,
				TypeName:   typeName,
				MemberName: memberName,
				Kind:       kind,
				ReturnType: member.Type,
				Parameters: params,
				Sources:    []string{"standard-symbols"},
			})
			if string(member.Kind) == "constructor" && messagingInboundEmailDTOType(namespace, typeName) {
				row.GladeBehavior = BehaviorPassive
			}
			byID[surfaceIDKey(memberID)] = row
		}
	}
	for _, entry := range capability.StdlibMatrix() {
		id := idFromStdlibAPI(entry.API)
		key := surfaceIDKey(id)
		row := byID[key]
		if row.SurfaceID == "" {
			row = SurfaceLedgerRow{SurfaceID: id, Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, Sources: []string{"stdlib-matrix"}}
			fillFromApexID(&row)
		}
		row.GladeBehavior = behaviorFromCapabilityStatus(entry.Status)
		row.Notes = entry.Notes
		row.Sources = mergeStrings(row.Sources, []string{"stdlib-matrix"})
		byID[key] = withDefaults(row)
	}
	for _, entry := range capability.BuildStubBehaviorReport().Entries {
		id := idFromStubBehavior(entry)
		key := surfaceIDKey(id)
		row := byID[key]
		if row.SurfaceID == "" {
			row = SurfaceLedgerRow{SurfaceID: id, Product: ProductApex, Area: AreaRuntime, Kind: gladeMemberKind(entry.Kind), Sources: []string{"stub-behavior"}}
			fillFromApexID(&row)
		}
		row.GladeBehavior = mergeGladeBehavior(row.GladeBehavior, behaviorFromStubStatus(entry.Status))
		row.ReturnType = firstNonEmpty(row.ReturnType, entry.ReturnType)
		if len(row.Parameters) == 0 {
			row.Parameters = append([]string(nil), entry.Parameters...)
		}
		row.Notes = firstNonEmpty(row.Notes, entry.Notes)
		row.Sources = mergeStrings(row.Sources, []string{"stub-behavior"})
		byID[key] = withDefaults(row)
	}
	addDataReferenceGladeRows(byID)
	addLocalTestLWCGladeRows(byID)
	addLocalTestVisualforceComponentRows(byID)
	addLocalTestAuraMetadataRows(byID)
	addUnsupportedQueryRuntimeGladeRows(byID)
	addFixtureBackedStdlibAliasRows(byID)
	addFixtureBackedSystemAliasRows(byID)
	addFixtureBackedApexAliasRows(byID)
	rows := make([]SurfaceLedgerRow, 0, len(byID))
	for _, row := range byID {
		rows = append(rows, withDefaults(row))
	}
	sortRows(rows)
	return rows
}

type fixtureBackedStdlibAlias struct {
	TypeName string
	Methods  []string
}

var fixtureBackedStdlibAliases = []fixtureBackedStdlibAlias{
	{TypeName: "Blob", Methods: []string{"toString"}},
	{TypeName: "Datetime", Methods: []string{"newInstanceGmt", "valueOfGmt"}},
	{TypeName: "Decimal", Methods: []string{"abs", "longValue", "pow", "valueOf"}},
	{TypeName: "Double", Methods: []string{"valueOf"}},
	{TypeName: "Id", Methods: []string{"getSObjectType", "to15", "to18", "valueOf"}},
	{TypeName: "Integer", Methods: []string{"doubleValue", "valueOf"}},
	{TypeName: "JSON", Methods: []string{"createParser"}},
	{TypeName: "JSONParser", Methods: []string{"getBooleanValue", "getDateValue", "getText"}},
	{TypeName: "List", Methods: []string{"copyConstructor", "indexOf", "sort"}},
	{TypeName: "Long", Methods: []string{"valueOf"}},
	{TypeName: "Map", Methods: []string{"containsKey", "containsValue", "copyConstructor", "deepClone", "keySet", "toString"}},
	{TypeName: "Object", Methods: []string{"equals", "hashCode", "toString"}},
	{TypeName: "Pattern", Methods: []string{"pattern", "split"}},
	{TypeName: "RestRequest", Methods: []string{"getHeader", "getParameter"}},
	{TypeName: "RoundingMode", Methods: []string{"name", "ordinal", "toString", "valueOf", "values"}},
	{TypeName: "Set", Methods: []string{"copyConstructor", "deepClone", "remove"}},
	{TypeName: "String", Methods: []string{
		"codePointAt",
		"commonPrefix",
		"escapeCsv",
		"escapeEcmaScript",
		"escapeHtml3",
		"escapeHtml4",
		"escapeJava",
		"escapeSingleQuotes",
		"escapeUnicode",
		"escapeXml",
		"escapeXml10",
		"escapeXml11",
		"format",
		"getChars",
		"lastIndexOfAny",
		"lastOrdinalIndexOf",
		"ordinalIndexOf",
		"overlay",
		"remove",
		"removeIgnoreCase",
		"replaceAll",
		"replaceFirst",
		"replaceIgnoreCase",
		"replaceOnce",
		"rotate",
		"strip",
		"stripAll",
		"stripEnd",
		"stripStart",
		"stripToEmpty",
		"stripToNull",
		"swapCase",
		"unescapeCsv",
		"unescapeEcmaScript",
		"unescapeHtml3",
		"unescapeHtml4",
		"unescapeJava",
		"unescapeUnicode",
		"unescapeXml",
		"unescapeXml10",
		"unescapeXml11",
	}},
	{TypeName: "Time", Methods: []string{"valueOf"}},
	{TypeName: "Type", Methods: []string{"forName"}},
	{TypeName: "URL", Methods: []string{"getAuthority", "getCurrentRequestUrl", "getDefaultPort", "getHost"}},
}

func addFixtureBackedStdlibAliasRows(byID map[string]SurfaceLedgerRow) {
	for _, alias := range fixtureBackedStdlibAliases {
		for _, method := range alias.Methods {
			id := ApexMemberID("System", alias.TypeName, method, nil)
			byID[surfaceIDKey(id)] = RowFromGladeShape(SurfaceLedgerRow{
				SurfaceID:     id,
				Product:       ProductApex,
				Area:          AreaRuntime,
				Namespace:     "System",
				TypeName:      alias.TypeName,
				MemberName:    method,
				Kind:          KindMethod,
				GladeBehavior: BehaviorSupported,
				Sources:       []string{"stdlib-fixture-alias"},
				Notes:         "fixture-backed docs shorthand for a System stdlib method whose typed overloads are implemented locally",
			})
		}
	}
}

type fixtureBackedSystemAliasRow struct {
	SurfaceID string
	Kind      string
	Behavior  BehaviorState
	Notes     string
}

var fixtureBackedSystemAliasRows = []fixtureBackedSystemAliasRow{
	{SurfaceID: "apex:System.Auth.*", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostics for local Auth token, JWT, OAuth, and cloud surfaces"},
	{SurfaceID: "apex:System.Canvas.*", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostics for local Canvas app integration surfaces"},
	{SurfaceID: "apex:System.Continuation.*", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostics for local Continuation callback and callout surfaces"},
	{SurfaceID: "apex:System.Crypto.areEqualConstantTime(Blob,Blob)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Crypto constant-time Blob comparison"},
	{SurfaceID: "apex:System.CustomMetadataType.getAll", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local custom metadata static getAll access"},
	{SurfaceID: "apex:System.CustomSetting.getInstance", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local custom setting static getInstance access"},
	{SurfaceID: "apex:System.DMLOptions", Kind: KindType, Behavior: BehaviorPassive, Notes: "fixture-backed System-qualified alias for Database.DMLOptions local option shape"},
	{SurfaceID: "apex:System.DMLOptions.allowFieldTruncation", Kind: KindProperty, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Database.DMLOptions.allowFieldTruncation"},
	{SurfaceID: "apex:System.DMLOptions.assignmentRuleHeader", Kind: KindProperty, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Database.DMLOptions.assignmentRuleHeader"},
	{SurfaceID: "apex:System.DMLOptions.emailHeader", Kind: KindProperty, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Database.DMLOptions.emailHeader"},
	{SurfaceID: "apex:System.DMLOptions.localeOptions", Kind: KindProperty, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Database.DMLOptions.localeOptions"},
	{SurfaceID: "apex:System.DMLOptions.optAllOrNone", Kind: KindProperty, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Database.DMLOptions.optAllOrNone"},
	{SurfaceID: "apex:System.EventBus.*", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostics for local platform event delivery surfaces"},
	{SurfaceID: "apex:System.EventBus.publish", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local no-op platform event publish SaveResult behavior"},
	{SurfaceID: "apex:System.EventBus.publishAfterCommit", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostic for after-commit platform event delivery"},
	{SurfaceID: "apex:System.HierarchyCustomSetting.getOrgDefaults", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local hierarchy custom setting org-default lookup"},
	{SurfaceID: "apex:System.HttpRequest.setClientCertificate", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostic for inline client-certificate material"},
	{SurfaceID: "apex:System.HttpRequest.setClientCertificateName", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostic for named client-certificate configuration"},
	{SurfaceID: "apex:System.HttpRequest.setTimeout", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local HttpRequest timeout validation"},
	{SurfaceID: "apex:System.IllegalStateException", Kind: KindType, Behavior: BehaviorPassive, Notes: "fixture-backed System-qualified alias for the local built-in IllegalStateException type"},
	{SurfaceID: "apex:System.Integer.MAX_VALUE", Kind: KindProperty, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Integer.MAX_VALUE"},
	{SurfaceID: "apex:System.Integer.MIN_VALUE", Kind: KindProperty, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Integer.MIN_VALUE"},
	{SurfaceID: "apex:System.Iterator.remove", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostic for mutating collection iterators"},
	{SurfaceID: "apex:System.JSONGenerator.writeRaw(String)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for JSONGenerator.writeRaw"},
	{SurfaceID: "apex:System.JSONGenerator.writeRawField(String,String)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for JSONGenerator.writeRawField"},
	{SurfaceID: "apex:System.JSONGenerator.writeRawValue(String)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for JSONGenerator.writeRawValue"},
	{SurfaceID: "apex:System.Limits.getDmlRows", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Limits.getDmlRows"},
	{SurfaceID: "apex:System.Limits.getDmlStatements", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Limits.getDmlStatements"},
	{SurfaceID: "apex:System.Limits.getEmailInvocations", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Limits.getEmailInvocations"},
	{SurfaceID: "apex:System.Limits.getLimitDmlRows", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Limits.getLimitDmlRows"},
	{SurfaceID: "apex:System.Limits.getLimitDmlStatements", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Limits.getLimitDmlStatements"},
	{SurfaceID: "apex:System.Limits.getLimitScheduledJobs", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Limits.getLimitScheduledJobs"},
	{SurfaceID: "apex:System.Limits.getScheduledJobs", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Limits.getScheduledJobs"},
	{SurfaceID: "apex:System.Long.MAX_VALUE", Kind: KindProperty, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Long.MAX_VALUE"},
	{SurfaceID: "apex:System.Long.MIN_VALUE", Kind: KindProperty, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Long.MIN_VALUE"},
	{SurfaceID: "apex:System.Matcher.appendReplacement", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostic for Java StringBuffer append replacement semantics"},
	{SurfaceID: "apex:System.Matcher.appendTail", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostic for Java StringBuffer append tail semantics"},
	{SurfaceID: "apex:System.Matcher.groupCount", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Matcher.groupCount"},
	{SurfaceID: "apex:System.Matcher.hasAnchoringBounds", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed docs shorthand for Matcher.hasAnchoringBounds()"},
	{SurfaceID: "apex:System.Matcher.hasTransparentBounds", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed docs shorthand for Matcher.hasTransparentBounds()"},
	{SurfaceID: "apex:System.Matcher.lookingAt", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Matcher.lookingAt"},
	{SurfaceID: "apex:System.Matcher.region", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Matcher.region"},
	{SurfaceID: "apex:System.Matcher.replaceAll", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Matcher.replaceAll"},
	{SurfaceID: "apex:System.Matcher.replaceFirst", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Matcher.replaceFirst"},
	{SurfaceID: "apex:System.Matcher.useAnchoringBounds", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed docs shorthand for Matcher.useAnchoringBounds(Boolean)"},
	{SurfaceID: "apex:System.Matcher.usePattern", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local Matcher.usePattern"},
	{SurfaceID: "apex:System.Matcher.useTransparentBounds", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed docs shorthand for Matcher.useTransparentBounds(Boolean)"},
	{SurfaceID: "apex:System.Messaging.MassEmailMessage", Kind: KindType, Behavior: BehaviorPassive, Notes: "fixture-backed System-qualified alias for the local MassEmailMessage DTO shape"},
	{SurfaceID: "apex:System.Messaging.SendEmailResult", Kind: KindType, Behavior: BehaviorPassive, Notes: "fixture-backed System-qualified alias for the local SendEmailResult DTO shape"},
	{SurfaceID: "apex:System.Messaging.sendPushNotification", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostic for messaging transport APIs"},
	{SurfaceID: "apex:System.PatternSyntaxException", Kind: KindType, Behavior: BehaviorPassive, Notes: "fixture-backed System-qualified alias for local PatternSyntaxException values"},
	{SurfaceID: "apex:System.PageReference(record)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed docs shorthand for the PageReference ApexPage record constructor"},
	{SurfaceID: "apex:System.QuickAction.*", Kind: KindMethod, Behavior: BehaviorUnsupported, Notes: "fixture-backed explicit unsupported diagnostics for local quick action UI surfaces"},
	{SurfaceID: "apex:System.RestRequest.getHeader(String)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for RestRequest.getHeader(String)"},
	{SurfaceID: "apex:System.RestRequest.getParameter(String)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for RestRequest.getParameter(String)"},
	{SurfaceID: "apex:System.Schema.describeSObjects(List<String>)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Schema.describeSObjects(List<String>)"},
	{SurfaceID: "apex:System.Schema.getGlobalDescribe()", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Schema.getGlobalDescribe()"},
	{SurfaceID: "apex:System.Time.valueOf(String)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for Time.valueOf(String)"},
	{SurfaceID: "apex:System.TxnSecurity.EventCondition.evaluate(SObject)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local transaction-security event-condition default evaluation"},
	{SurfaceID: "apex:System.TxnSecurity.PolicyCondition.evaluate(TxnSecurity.Event)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed System-qualified alias for local transaction-security policy-condition default evaluation"},
	{SurfaceID: "apex:System.Type.forName(namespace,name)", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed docs shorthand for Type.forName namespace/name lookup"},
}

func addFixtureBackedSystemAliasRows(byID map[string]SurfaceLedgerRow) {
	for _, alias := range fixtureBackedSystemAliasRows {
		row := SurfaceLedgerRow{
			SurfaceID:     alias.SurfaceID,
			Product:       ProductApex,
			Area:          AreaRuntime,
			Kind:          alias.Kind,
			GladeBehavior: alias.Behavior,
			Sources:       []string{"system-fixture-alias"},
			Notes:         alias.Notes,
		}
		fillFromApexID(&row)
		byID[surfaceIDKey(row.SurfaceID)] = RowFromGladeShape(row)
	}
}

var fixtureBackedApexAliasRows = []fixtureBackedSystemAliasRow{
	{SurfaceID: "apex:TxnSecurity.Event.Event()", Kind: KindMethod, Behavior: BehaviorSupported, Notes: "fixture-backed exact TxnSecurity docs constructor for the local passive transaction-security event record"},
}

func addFixtureBackedApexAliasRows(byID map[string]SurfaceLedgerRow) {
	for _, alias := range fixtureBackedApexAliasRows {
		row := SurfaceLedgerRow{
			SurfaceID:     alias.SurfaceID,
			Product:       ProductApex,
			Area:          AreaRuntime,
			Kind:          alias.Kind,
			GladeBehavior: alias.Behavior,
			Sources:       []string{"apex-fixture-alias"},
			Notes:         alias.Notes,
		}
		fillFromApexID(&row)
		byID[surfaceIDKey(row.SurfaceID)] = RowFromGladeShape(row)
	}
}

func mergeGladeBehavior(existing, next BehaviorState) BehaviorState {
	if existing == BehaviorUnsupported {
		return existing
	}
	if existing == BehaviorSupported && (next == BehaviorPassive || next == BehaviorStubNoOp || next == BehaviorUnsupported) {
		return existing
	}
	if existing == BehaviorPartial && (next == BehaviorStubNoOp || next == BehaviorUnsupported) {
		return existing
	}
	if next == "" || next == BehaviorNone {
		return existing
	}
	return next
}

func behaviorForStandardType(namespace, typeName string) BehaviorState {
	if strings.EqualFold(namespace, "Database") && strings.EqualFold(typeName, "Stateful") {
		return BehaviorSupported
	}
	return BehaviorNone
}

func addDataReferenceGladeRows(byID map[string]SurfaceLedgerRow) {
	for _, objectName := range storage.KnownStandardObjectNames() {
		definition, ok := storage.StandardObjectDefinition(objectName)
		if !ok {
			continue
		}
		objectID := DataObjectID(definition.APIName)
		byID[surfaceIDKey(objectID)] = RowFromGeneratedDataReferenceShape(SurfaceLedgerRow{
			SurfaceID:     objectID,
			Product:       ProductDataRef,
			Area:          AreaData,
			TypeName:      definition.APIName,
			Kind:          KindType,
			GladeBehavior: BehaviorSupported,
			Sources:       []string{SourceStandardSObjectGeneratedShape},
		})
		for _, field := range definition.Fields {
			fieldID := DataFieldID(definition.APIName, field.APIName)
			byID[surfaceIDKey(fieldID)] = RowFromGeneratedDataReferenceShape(SurfaceLedgerRow{
				SurfaceID:     fieldID,
				Product:       ProductDataRef,
				Area:          AreaData,
				TypeName:      definition.APIName,
				FieldName:     field.APIName,
				Kind:          KindField,
				ReturnType:    string(field.Type),
				GladeBehavior: BehaviorSupported,
				Sources:       []string{SourceStandardSObjectGeneratedShape},
			})
		}
	}
}

func addLocalTestLWCGladeRows(byID map[string]SurfaceLedgerRow) {
	for _, module := range localTestUnsupportedLWCModules {
		id := LWCModuleID(module)
		byID[surfaceIDKey(id)] = RowFromGladeShape(SurfaceLedgerRow{
			SurfaceID:     id,
			Product:       ProductLWC,
			Area:          AreaUI,
			TypeName:      module,
			Kind:          KindModule,
			GladeBehavior: BehaviorUnsupported,
			Sources:       []string{"uicontroller-import-shape"},
			Notes:         "local Apex tests can index this LWC import shape; browser or service execution is not modeled locally",
		})
	}
}

func addLocalTestVisualforceComponentRows(byID map[string]SurfaceLedgerRow) {
	for _, name := range visualforce.StandardComponentReferenceNames() {
		id := visualforceComponentReferenceID(name)
		byID[surfaceIDKey(id)] = RowFromGladeShape(SurfaceLedgerRow{
			SurfaceID:     id,
			Product:       ProductVisualforce,
			Area:          AreaUI,
			TypeName:      strings.TrimPrefix(id, "visualforce:"),
			Kind:          KindGuide,
			GladeShape:    ShapeGenerated,
			GladeBehavior: BehaviorPassive,
			Sources:       []string{"visualforce-component-doc-shape"},
			Notes:         "local Visualforce indexing accepts this component reference as metadata shape; browser rendering is not modeled locally",
		})
	}
}

func visualforceComponentReferenceID(name string) string {
	if name == "" {
		return "visualforce:pages_compref"
	}
	return "visualforce:pages_compref_" + name
}

func addLocalTestAuraMetadataRows(byID map[string]SurfaceLedgerRow) {
	for _, id := range localTestAuraMetadataSurfaceIDs {
		byID[surfaceIDKey(id)] = RowFromGladeShape(SurfaceLedgerRow{
			SurfaceID:     id,
			Product:       ProductAura,
			Area:          AreaUI,
			TypeName:      strings.TrimPrefix(id, "unknown:"),
			Kind:          KindGuide,
			GladeShape:    ShapeGenerated,
			GladeBehavior: BehaviorPassive,
			Sources:       []string{"aura-metadata-doc-shape"},
			Notes:         "local UI controller discovery can index this Aura metadata shape; browser rendering and Lightning services are not modeled locally",
		})
	}
}

type unsupportedQueryRuntimeRow struct {
	ID   string
	Note string
}

var unsupportedQueryRuntimeRows = []unsupportedQueryRuntimeRow{
	{ID: "unknown:MockSOQLTestsForDMOs", Note: "Data Cloud DMO mock SOQL tests require Data Cloud services and are not executed by the local Apex runtime."},
	{ID: "unknown:apex_connector_external_objects_mock_soql_tests", Note: "Salesforce Connect external-object mock SOQL tests require connector services outside the local Apex runtime."},
	{ID: "unknown:salesforce_app_limits_platform_soslsoql", Note: "SOQL and SOSL platform limit tables document remote service limits, not local Apex query execution behavior."},
	{ID: "unknown:sforce_api_calls_describesoqllistview", Note: "SOAP describeSObject list-view metadata calls are API metadata surfaces, not local Apex SOQL execution."},
	{ID: "unknown:sforce_api_calls_describesoqllistview_soqlwherecondition", Note: "List-view SOQL where-condition metadata is returned by describe API calls, not executed as local Apex SOQL."},
	{ID: "unknown:sforce_api_calls_describesoqllistviewparams", Note: "DescribeSObject list-view request parameters are SOAP API metadata, not local Apex runtime behavior."},
	{ID: "unknown:sforce_api_calls_describesoqllistviewresult", Note: "DescribeSObject list-view result DTOs are SOAP API metadata, not local Apex runtime behavior."},
	{ID: "unknown:sforce_api_calls_describesoqllistviews", Note: "DescribeSObject list-view collection metadata is a SOAP API surface outside local Apex execution."},
	{ID: "unknown:sforce_api_calls_describesoqllistviewsrequest", Note: "DescribeSoqlListViewsRequest is SOAP API metadata input, not local Apex SOQL execution."},
	{ID: "unknown:sforce_api_calls_soql_changing_batch_size", Note: "API query batch-size negotiation is a remote API cursor setting and has no local Apex test-runner equivalent."},
	{ID: "unknown:sforce_api_calls_soql_feeds_url_syntax", Note: "Syndication feed SOQL mapping is public-site feed configuration outside local Apex query execution."},
	{ID: "unknown:sforce_api_calls_soql_relationships_query_datacat", Note: "DataCategorySelection article relationships depend on Knowledge data category services outside the local runtime."},
	{ID: "unknown:sforce_api_calls_soql_relationships_query_hist", Note: "History relationship queries depend on Salesforce field-history tracking services outside the local runtime."},
	{ID: "unknown:sforce_api_calls_soql_select_set_options", Note: "SOQL SET OPTIONS targets Data Cloud DLO and DMO service behavior outside local Apex query execution."},
	{ID: "unknown:sforce_api_calls_soql_select_with_datacategory", Note: "SOQL WITH DATA CATEGORY filters Knowledge and Question visibility services outside the local runtime."},
	{ID: "unknown:sforce_api_calls_soql_select_with_datacategory_catselection", Note: "SOQL data category selection syntax depends on Knowledge category services outside the local runtime."},
	{ID: "unknown:sforce_api_calls_soql_select_with_recordvisibilitycontext", Note: "RecordVisibilityContext uses Salesforce visibility service descriptors outside local Apex query execution."},
	{ID: "unknown:sforce_api_calls_soql_typos", Note: "SOQL typographical conventions are documentation syntax guidance, not a runtime surface."},
	{ID: "unknown:sforce_api_calls_sosl_limits_external_objects", Note: "SOSL external-object limits depend on Salesforce Connect services outside the local runtime."},
	{ID: "unknown:sforce_api_calls_sosl_typos", Note: "SOSL typographical conventions are documentation syntax guidance, not a runtime surface."},
	{ID: "unknown:sforce_api_calls_sosl_update_tracking", Note: "SOSL UPDATE TRACKING records Salesforce Knowledge search analytics outside the local runtime."},
	{ID: "unknown:sforce_api_calls_sosl_update_viewstat", Note: "SOSL UPDATE VIEWSTAT records Salesforce Knowledge article view statistics outside the local runtime."},
	{ID: "unknown:sforce_api_calls_sosl_using_listview", Note: "SOSL USING ListView depends on Salesforce list-view metadata and service filtering outside the local runtime."},
	{ID: "unknown:sforce_api_calls_sosl_with", Note: "SOSL WITH DivisionFilter depends on Salesforce division filtering services outside the local runtime."},
	{ID: "unknown:sforce_api_calls_sosl_with_data_category", Note: "SOSL WITH DATA CATEGORY filters Knowledge and Question category visibility outside the local runtime."},
	{ID: "unknown:sforce_api_calls_sosl_with_metadata", Note: "SOSL WITH METADATA returns service response labels outside local Apex search execution."},
	{ID: "unknown:supported_soql", Note: "Supported PushTopic query rules are Streaming API behavior outside local Apex query execution."},
	{ID: "unknown:unsupported_soql_statements", Note: "Unsupported PushTopic query rules are Streaming API behavior outside local Apex query execution."},
}

func addUnsupportedQueryRuntimeGladeRows(byID map[string]SurfaceLedgerRow) {
	for _, item := range unsupportedQueryRuntimeRows {
		byID[surfaceIDKey(item.ID)] = RowFromGladeShape(SurfaceLedgerRow{
			SurfaceID:     item.ID,
			Product:       ProductApex,
			Area:          AreaRuntime,
			Kind:          KindMethod,
			GladeBehavior: BehaviorUnsupported,
			Sources:       []string{"query-runtime-explicit-unsupported"},
			Notes:         item.Note,
		})
	}
}

var localTestUnsupportedLWCModules = []string{
	"Decorators",
	"HTML",
	"LWC",
	"PageReference",
	"Salesforce",
	"Standard",
	"XML",
	"`@salesforce`",
	"`experience/blockBuilderApi`",
	"`experience/cms*Api`",
	"`experience/cmsEditorApi`",
	"`lightning/analyticsWaveApi`",
	"`lightning/graphql`",
	"`lightning/industriesEducationPublicApi`",
	"`lightning/mobileCapabilities`",
	"`lightning/serviceKnowledgeApi`",
	"`lightning/ui*Api`",
	"`lightning/uiGraphQLApi`",
	"`notifyRecordUpdateAvailable(recordIds)`",
	"lightning/cmsDeliveryApi",
}

var localTestAuraMetadataSurfaceIDs = strings.Fields(`
unknown:apex_classes_annotation_AuraEnabled
unknown:code_sample_lightning_cmp
unknown:meta_auradefinitionbundle
unknown:meta_lightningbolt
unknown:meta_lightningcomponentbundle
unknown:meta_lightningexperiencesettings
unknown:meta_lightningexperiencetheme
unknown:meta_lightningmessagechannel
unknown:meta_lightningonboardingconfig
unknown:meta_lightningtypebundle
unknown:ref_attr_types_aura
unknown:ref_attr_types_aura_action
unknown:ref_aura_application
unknown:ref_aura_attribute
unknown:ref_aura_event
unknown:ref_aura_interface
`)

func splitTypeName(namespace, name string) (string, string) {
	if namespace != "" {
		return namespace, strings.TrimPrefix(name, namespace+".")
	}
	if idx := strings.LastIndex(name, "."); idx > 0 {
		return name[:idx], name[idx+1:]
	}
	return "System", name
}

func gladeConstructorMemberName(namespace, typeName, memberName string) string {
	if !strings.EqualFold(memberName, lastTypeSegment(typeName)) {
		return memberName
	}
	if strings.EqualFold(namespace, "Messaging.InboundEmail") {
		return "InboundEmail." + typeName
	}
	return typeName
}

func messagingInboundEmailDTOType(namespace, typeName string) bool {
	if !strings.EqualFold(namespace, "Messaging.InboundEmail") {
		return false
	}
	switch typeName {
	case "AuthenticationResult", "AuthenticationResultField", "BinaryAttachment", "TextAttachment":
		return true
	default:
		return false
	}
}

func lastTypeSegment(typeName string) string {
	if idx := strings.LastIndexByte(typeName, '.'); idx >= 0 && idx < len(typeName)-1 {
		return typeName[idx+1:]
	}
	return typeName
}

func memberParameterTypes(params []apexast.Parameter) []string {
	out := make([]string, 0, len(params))
	for _, param := range params {
		out = append(out, param.Type)
	}
	return cleanList(out)
}

func gladeMemberKind(kind string) string {
	switch kind {
	case "method", "constructor":
		return KindMethod
	case "property":
		return KindProperty
	case "field":
		return KindField
	default:
		return KindType
	}
}

func idFromStdlibAPI(api string) string {
	api = strings.TrimSpace(api)
	parts := strings.SplitN(api, ".", 2)
	if len(parts) != 2 {
		return ApexTypeID("System", api)
	}
	params := []string(nil)
	member := parts[1]
	if open := strings.IndexByte(member, '('); open >= 0 && strings.HasSuffix(member, ")") {
		rawParams := strings.TrimSuffix(member[open+1:], ")")
		member = member[:open]
		params = splitSurfaceParameterList(rawParams)
	} else if member == "contains" {
		params = []string{"String"}
	}
	return ApexMemberID("System", parts[0], member, params)
}

func splitSurfaceParameterList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var params []string
	depth := 0
	start := 0
	for i, r := range raw {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				params = append(params, strings.TrimSpace(raw[start:i]))
				start = i + len(string(r))
			}
		}
	}
	params = append(params, strings.TrimSpace(raw[start:]))
	return params
}

func idFromStubBehavior(entry capability.StubBehaviorEntry) string {
	namespace, typeName := splitTypeName("", entry.Type)
	if entry.Member == "" {
		return ApexTypeID(namespace, typeName)
	}
	if gladeMemberKind(entry.Kind) == KindProperty {
		return ApexMemberID(namespace, typeName, entry.Member, nil)
	}
	return ApexMemberID(namespace, typeName, entry.Member, entry.Parameters)
}

func behaviorFromCapabilityStatus(status capability.Status) BehaviorState {
	switch status {
	case capability.StatusSupported:
		return BehaviorSupported
	case capability.StatusPartial:
		return BehaviorPartial
	case capability.StatusUnsupported:
		return BehaviorUnsupported
	case capability.StatusStub:
		return BehaviorStubNoOp
	default:
		return BehaviorNone
	}
}

func behaviorFromStubStatus(status capability.StubBehaviorStatus) BehaviorState {
	switch status {
	case capability.StubBehaviorImplemented:
		return BehaviorSupported
	case capability.StubBehaviorPassiveDefault:
		return BehaviorPassive
	case capability.StubBehaviorStubNoOp:
		return BehaviorStubNoOp
	case capability.StubBehaviorUnsupported:
		return BehaviorUnsupported
	default:
		return BehaviorNone
	}
}

func fillFromApexID(row *SurfaceLedgerRow) {
	if row == nil || !strings.HasPrefix(row.SurfaceID, "apex:") {
		return
	}
	rest := strings.TrimPrefix(row.SurfaceID, "apex:")
	if dot := strings.LastIndex(rest, "."); dot > 0 {
		row.Namespace = rest[:dot]
		member := rest[dot+1:]
		if paren := strings.Index(member, "("); paren >= 0 {
			row.MemberName = member[:paren]
			typePart := rest[:dot]
			if typeDot := strings.LastIndex(typePart, "."); typeDot > 0 {
				row.Namespace = typePart[:typeDot]
				row.TypeName = typePart[typeDot+1:]
			}
			return
		}
		if row.Kind == KindMethod || row.Kind == KindProperty || row.Kind == KindField {
			row.MemberName = member
			typePart := rest[:dot]
			if typeDot := strings.LastIndex(typePart, "."); typeDot > 0 {
				row.Namespace = typePart[:typeDot]
				row.TypeName = typePart[typeDot+1:]
			}
			return
		}
		row.TypeName = member
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
