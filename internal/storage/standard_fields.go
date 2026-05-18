package storage

import (
	"sort"
	"strings"
	"sync"
)

const standardFieldsOverlayMarker = "__oaer_standard_fields_overlay"

// EnsureStandardObjectFields adds public Salesforce standard fields for objects
// whose project metadata commonly only carries custom-field deltas.
func EnsureStandardObjectFields(definition *ObjectDefinition) {
	EnsureStandardObjectFieldsForFeatures(definition, nil)
}

// EnsureStandardObjectFieldsForFeatures adds the base standard object overlay,
// plus feature-gated standard fields and record types for enabled org features.
func EnsureStandardObjectFieldsForFeatures(definition *ObjectDefinition, features []string) {
	if definition == nil {
		return
	}
	featureSignature := canonicalFeatureSignature(features)
	if standardFieldsOverlayApplied(*definition, featureSignature) {
		return
	}
	stateAndCountryPicklistEnabled := hasCanonicalFeature(features, "StateAndCountryPicklist")
	if definition.Fields == nil {
		definition.Fields = make(map[string]Field)
	}
	if field, ok := definition.Fields["Id"]; !ok || field.APIName == "" {
		definition.Fields["Id"] = Field{APIName: "Id", Label: "Record ID", Type: FieldID}
	}
	ensureCoreSystemFields(definition)
	mergeStandardObjectDefinition(definition, features)
	mergeStandardSObjectStubFields(definition, features)
	mergeStandardSObjectStubRelationships(definition, features)
	applyStandardObjectCompatibilityOverlays(definition)
	EnsureRecordTypeIDField(definition)
	ensureCommonRecordTypeField(definition)
	fields := standardFieldsForObject(definition.APIName)
	for _, field := range fields {
		if _, ok := ResolveFieldName(*definition, "", field.APIName); ok {
			continue
		}
		definition.Fields[field.APIName] = field
	}
	for _, field := range fields {
		ensureStandardRelationship(definition, field)
	}
	if !stateAndCountryPicklistEnabled {
		removeStateAndCountryPicklistFields(definition)
	}
	for _, field := range definition.Fields {
		ensureStandardRelationship(definition, field)
	}
	markStandardFieldsOverlay(definition, featureSignature)
}

func standardFieldsOverlayApplied(definition ObjectDefinition, featureSignature string) bool {
	if definition.Metadata == nil {
		return false
	}
	return definition.Metadata[standardFieldsOverlayMarker] == featureSignature
}

func markStandardFieldsOverlay(definition *ObjectDefinition, featureSignature string) {
	if definition == nil {
		return
	}
	if definition.Metadata == nil {
		definition.Metadata = make(map[string]string)
	}
	definition.Metadata[standardFieldsOverlayMarker] = featureSignature
}

func canonicalFeatureSignature(features []string) string {
	if len(features) == 0 {
		return ""
	}
	normalized := make([]string, 0, len(features))
	for _, feature := range features {
		feature = canonicalFeatureName(feature)
		if feature != "" {
			normalized = append(normalized, feature)
		}
	}
	if len(normalized) == 0 {
		return ""
	}
	sort.Strings(normalized)
	out := normalized[:0]
	last := ""
	for _, feature := range normalized {
		if feature == last {
			continue
		}
		out = append(out, feature)
		last = feature
	}
	return strings.Join(out, ",")
}

func ensureCoreSystemFields(definition *ObjectDefinition) {
	ensureField(definition, Field{APIName: "CreatedDate", Label: "Created Date", Type: FieldDateTime, DisplayType: "DATETIME"})
	ensureField(definition, Field{APIName: "CreatedById", Label: "Created By ID", Type: FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"})
	ensureField(definition, Field{APIName: "LastModifiedDate", Label: "Last Modified Date", Type: FieldDateTime, DisplayType: "DATETIME"})
	ensureField(definition, Field{APIName: "LastModifiedById", Label: "Last Modified By ID", Type: FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"})
	ensureField(definition, Field{APIName: "SystemModstamp", Label: "System Modstamp", Type: FieldDateTime, DisplayType: "DATETIME"})
	if isOwnerBackedObject(definition.APIName) {
		ensureField(definition, Field{APIName: "OwnerId", Label: "Owner ID", Type: FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "Owner"})
	}
}

func isOwnerBackedObject(objectName string) bool {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" || strings.HasSuffix(strings.ToLower(objectName), "__mdt") {
		return false
	}
	if stringsHasSuffixFold(objectName, "__c") || stringsHasSuffixFold(objectName, "__pc") || stringsHasSuffixFold(objectName, "__pr") {
		return true
	}
	switch {
	case stringsEqualFold(objectName, "Account"),
		stringsEqualFold(objectName, "Asset"),
		stringsEqualFold(objectName, "Campaign"),
		stringsEqualFold(objectName, "Case"),
		stringsEqualFold(objectName, "Contact"),
		stringsEqualFold(objectName, "Contract"),
		stringsEqualFold(objectName, "Event"),
		stringsEqualFold(objectName, "Lead"),
		stringsEqualFold(objectName, "Opportunity"),
		stringsEqualFold(objectName, "Order"),
		stringsEqualFold(objectName, "Task"):
		return true
	default:
		return false
	}
}

func ensureCommonRecordTypeField(definition *ObjectDefinition) {
	switch {
	case stringsEqualFold(definition.APIName, "Opportunity"):
	default:
		return
	}
	if _, ok := ResolveFieldName(*definition, "", "RecordTypeId"); ok {
		return
	}
	definition.Fields["RecordTypeId"] = Field{APIName: "RecordTypeId", Label: "Record Type ID", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}
}

func standardFieldsForObject(objectName string) []Field {
	if _, ok := standardObjectCatalogEntryFor(objectName); ok && !stringsEqualFold(objectName, "Account") {
		return nil
	}
	switch {
	case strings.HasSuffix(objectName, "__e"):
		return []Field{
			{APIName: "EventUuid", Label: "Event UUID", Type: FieldString},
			{APIName: "ReplayId", Label: "Replay ID", Type: FieldString},
		}
	case stringsEqualFold(objectName, "EntityDefinition"):
		return []Field{
			{APIName: "DeveloperName", Label: "Developer Name", Type: FieldString},
			{APIName: "DurableId", Label: "Durable ID", Type: FieldString},
			{APIName: "KeyPrefix", Label: "Key Prefix", Type: FieldString},
			{APIName: "Label", Label: "Label", Type: FieldString},
			{APIName: "NamespacePrefix", Label: "Namespace Prefix", Type: FieldString},
			{APIName: "QualifiedApiName", Label: "Qualified API Name", Type: FieldString},
		}
	case stringsEqualFold(objectName, "FieldDefinition"):
		return []Field{
			{APIName: "DeveloperName", Label: "Developer Name", Type: FieldString},
			{APIName: "DurableId", Label: "Durable ID", Type: FieldString},
			{APIName: "EntityDefinitionId", Label: "Entity Definition ID", Type: FieldReference, ReferenceTo: []string{"EntityDefinition"}, RelationshipName: "EntityDefinition"},
			{APIName: "Label", Label: "Label", Type: FieldString},
			{APIName: "NamespacePrefix", Label: "Namespace Prefix", Type: FieldString},
			{APIName: "QualifiedApiName", Label: "Qualified API Name", Type: FieldString},
		}
	case stringsEqualFold(objectName, "Group"):
		return []Field{
			{APIName: "Name", Label: "Group Name", Type: FieldString, Required: true},
			{APIName: "DeveloperName", Label: "Developer Name", Type: FieldString},
			{APIName: "Type", Label: "Type", Type: FieldPicklist},
		}
	case stringsEqualFold(objectName, "Account"):
		return withoutPersonAccountFields([]Field{
			{APIName: "Name", Label: "Account Name", Type: FieldString},
			{APIName: "AccountNumber", Label: "Account Number", Type: FieldString},
			{APIName: "AnnualRevenue", Label: "Annual Revenue", Type: FieldDecimal},
			{APIName: "BillingStreet", Label: "Billing Street", Type: FieldString},
			{APIName: "BillingCity", Label: "Billing City", Type: FieldString},
			{APIName: "BillingState", Label: "Billing State", Type: FieldString},
			{APIName: "BillingStateCode", Label: "Billing State/Province Code", Type: FieldString},
			{APIName: "BillingPostalCode", Label: "Billing Zip/Postal Code", Type: FieldString},
			{APIName: "BillingCountry", Label: "Billing Country", Type: FieldString},
			{APIName: "BillingCountryCode", Label: "Billing Country Code", Type: FieldString},
			{APIName: "BillingLatitude", Label: "Billing Latitude", Type: FieldDecimal},
			{APIName: "BillingLongitude", Label: "Billing Longitude", Type: FieldDecimal},
			{APIName: "Description", Label: "Account Description", Type: FieldString},
			{APIName: "Fax", Label: "Fax", Type: FieldString},
			{APIName: "FirstName", Label: "First Name", Type: FieldString},
			{APIName: "Industry", Label: "Industry", Type: FieldPicklist},
			{APIName: "IsPersonAccount", Label: "Is Person Account", Type: FieldBoolean},
			{APIName: "LastName", Label: "Last Name", Type: FieldString},
			{APIName: "MasterRecordId", Label: "Master Record ID", Type: FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "MasterRecord"},
			{APIName: "NumberOfEmployees", Label: "Employees", Type: FieldInteger},
			{APIName: "PersonBirthdate", Label: "Birthdate", Type: FieldDate},
			{APIName: "PersonDepartment", Label: "Department", Type: FieldString},
			{APIName: "PersonDoNotCall", Label: "Do Not Call", Type: FieldBoolean},
			{APIName: "PersonEmail", Label: "Person Email", Type: FieldString},
			{APIName: "PersonEmailBouncedDate", Label: "Email Bounced Date", Type: FieldDateTime},
			{APIName: "PersonEmailBouncedReason", Label: "Email Bounced Reason", Type: FieldString},
			{APIName: "PersonHasOptedOutOfEmail", Label: "Email Opt Out", Type: FieldBoolean},
			{APIName: "PersonHasOptedOutOfFax", Label: "Fax Opt Out", Type: FieldBoolean},
			{APIName: "PersonHomePhone", Label: "Home Phone", Type: FieldString},
			{APIName: "PersonMailingStreet", Label: "Mailing Street", Type: FieldString},
			{APIName: "PersonMailingCity", Label: "Mailing City", Type: FieldString},
			{APIName: "PersonMailingState", Label: "Mailing State", Type: FieldString},
			{APIName: "PersonMailingStateCode", Label: "Mailing State/Province Code", Type: FieldString},
			{APIName: "PersonMailingPostalCode", Label: "Mailing Zip/Postal Code", Type: FieldString},
			{APIName: "PersonMailingCountry", Label: "Mailing Country", Type: FieldString},
			{APIName: "PersonMailingCountryCode", Label: "Mailing Country Code", Type: FieldString},
			{APIName: "PersonMobilePhone", Label: "Mobile Phone", Type: FieldString},
			{APIName: "PersonOtherStreet", Label: "Other Street", Type: FieldString},
			{APIName: "PersonOtherCity", Label: "Other City", Type: FieldString},
			{APIName: "PersonOtherState", Label: "Other State", Type: FieldString},
			{APIName: "PersonOtherStateCode", Label: "Other State/Province Code", Type: FieldString},
			{APIName: "PersonOtherPostalCode", Label: "Other Zip/Postal Code", Type: FieldString},
			{APIName: "PersonOtherCountry", Label: "Other Country", Type: FieldString},
			{APIName: "PersonOtherCountryCode", Label: "Other Country Code", Type: FieldString},
			{APIName: "PersonTitle", Label: "Title", Type: FieldString},
			{APIName: "Phone", Label: "Account Phone", Type: FieldString},
			{APIName: "Rating", Label: "Rating", Type: FieldPicklist},
			{APIName: "RecordTypeId", Label: "Record Type ID", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
			{APIName: "ShippingStreet", Label: "Shipping Street", Type: FieldString},
			{APIName: "ShippingCity", Label: "Shipping City", Type: FieldString},
			{APIName: "ShippingState", Label: "Shipping State", Type: FieldString},
			{APIName: "ShippingStateCode", Label: "Shipping State/Province Code", Type: FieldString},
			{APIName: "ShippingPostalCode", Label: "Shipping Zip/Postal Code", Type: FieldString},
			{APIName: "ShippingCountry", Label: "Shipping Country", Type: FieldString},
			{APIName: "ShippingCountryCode", Label: "Shipping Country Code", Type: FieldString},
			{APIName: "ShippingLatitude", Label: "Shipping Latitude", Type: FieldDecimal},
			{APIName: "ShippingLongitude", Label: "Shipping Longitude", Type: FieldDecimal},
			{APIName: "Sic", Label: "SIC Code", Type: FieldString},
			{APIName: "Site", Label: "Account Site", Type: FieldString},
			{APIName: "TickerSymbol", Label: "Ticker Symbol", Type: FieldString},
			{APIName: "Type", Label: "Account Type", Type: FieldPicklist},
			{APIName: "Website", Label: "Website", Type: FieldString},
		})
	case stringsEqualFold(objectName, "Contact"):
		return []Field{
			{APIName: "Name", Label: "Full Name", Type: FieldString},
			{APIName: "FirstName", Label: "First Name", Type: FieldString},
			{APIName: "LastName", Label: "Last Name", Type: FieldString},
			{APIName: "Salutation", Label: "Salutation", Type: FieldPicklist},
			{APIName: "Title", Label: "Title", Type: FieldString},
			{APIName: "Email", Label: "Email", Type: FieldString},
			{APIName: "EmailBouncedDate", Label: "Email Bounced Date", Type: FieldDateTime},
			{APIName: "EmailBouncedReason", Label: "Email Bounced Reason", Type: FieldString},
			{APIName: "Birthdate", Label: "Birthdate", Type: FieldDate},
			{APIName: "Department", Label: "Department", Type: FieldString},
			{APIName: "DoNotCall", Label: "Do Not Call", Type: FieldBoolean},
			{APIName: "HasOptedOutOfEmail", Label: "Email Opt Out", Type: FieldBoolean},
			{APIName: "HasOptedOutOfFax", Label: "Fax Opt Out", Type: FieldBoolean},
			{APIName: "HomePhone", Label: "Home Phone", Type: FieldString},
			{APIName: "MobilePhone", Label: "Mobile Phone", Type: FieldString},
			{APIName: "Phone", Label: "Business Phone", Type: FieldString},
			{APIName: "MailingStreet", Label: "Mailing Street", Type: FieldString},
			{APIName: "MailingCity", Label: "Mailing City", Type: FieldString},
			{APIName: "MailingState", Label: "Mailing State", Type: FieldString},
			{APIName: "MailingStateCode", Label: "Mailing State/Province Code", Type: FieldString},
			{APIName: "MailingPostalCode", Label: "Mailing Zip/Postal Code", Type: FieldString},
			{APIName: "MailingCountry", Label: "Mailing Country", Type: FieldString},
			{APIName: "MailingCountryCode", Label: "Mailing Country Code", Type: FieldString},
			{APIName: "OtherStreet", Label: "Other Street", Type: FieldString},
			{APIName: "OtherCity", Label: "Other City", Type: FieldString},
			{APIName: "OtherState", Label: "Other State", Type: FieldString},
			{APIName: "OtherStateCode", Label: "Other State/Province Code", Type: FieldString},
			{APIName: "OtherPostalCode", Label: "Other Zip/Postal Code", Type: FieldString},
			{APIName: "OtherCountry", Label: "Other Country", Type: FieldString},
			{APIName: "OtherCountryCode", Label: "Other Country Code", Type: FieldString},
		}
	case stringsEqualFold(objectName, "OpportunityContactRole"):
		return []Field{
			{APIName: "ContactId", Label: "Contact ID", Type: FieldReference, ReferenceTo: []string{"Contact"}, RelationshipName: "Contact"},
			{APIName: "OpportunityId", Label: "Opportunity ID", Type: FieldReference, ReferenceTo: []string{"Opportunity"}, RelationshipName: "Opportunity", Required: true},
			{APIName: "Role", Label: "Role", Type: FieldPicklist},
			{APIName: "IsPrimary", Label: "Primary", Type: FieldBoolean},
		}
	case stringsEqualFold(objectName, "OpportunityLineItem"):
		return []Field{
			{APIName: "OpportunityId", Label: "Opportunity ID", Type: FieldReference, ReferenceTo: []string{"Opportunity"}, RelationshipName: "Opportunity", Required: true},
			{APIName: "PricebookEntryId", Label: "Price Book Entry ID", Type: FieldReference, ReferenceTo: []string{"PricebookEntry"}, RelationshipName: "PricebookEntry", Required: true},
			{APIName: "Product2Id", Label: "Product ID", Type: FieldReference, ReferenceTo: []string{"Product2"}, RelationshipName: "Product2"},
			{APIName: "Quantity", Label: "Quantity", Type: FieldDecimal},
			{APIName: "UnitPrice", Label: "Sales Price", Type: FieldDecimal},
			{APIName: "TotalPrice", Label: "Total Price", Type: FieldDecimal},
		}
	case stringsEqualFold(objectName, "PricebookEntry"):
		return []Field{
			{APIName: "Name", Label: "Price Book Entry Name", Type: FieldString},
			{APIName: "Pricebook2Id", Label: "Price Book ID", Type: FieldReference, ReferenceTo: []string{"Pricebook2"}, RelationshipName: "Pricebook2", Required: true},
			{APIName: "Product2Id", Label: "Product ID", Type: FieldReference, ReferenceTo: []string{"Product2"}, RelationshipName: "Product2", Required: true},
			{APIName: "UnitPrice", Label: "List Price", Type: FieldDecimal, Required: true},
			{APIName: "IsActive", Label: "Active", Type: FieldBoolean},
			{APIName: "UseStandardPrice", Label: "Use Standard Price", Type: FieldBoolean},
		}
	case stringsEqualFold(objectName, "EmailTemplate"):
		return []Field{
			{APIName: "ApiVersion", Label: "API Version", Type: FieldDecimal},
			{APIName: "Body", Label: "Body", Type: FieldString},
			{APIName: "BrandTemplateId", Label: "Letterhead ID", Type: FieldReference, ReferenceTo: []string{"BrandTemplate"}},
			{APIName: "Description", Label: "Description", Type: FieldString},
			{APIName: "DeveloperName", Label: "Developer Name", Type: FieldString},
			{APIName: "Encoding", Label: "Encoding", Type: FieldString},
			{APIName: "FolderId", Label: "Folder ID", Type: FieldReference, ReferenceTo: []string{"Folder"}},
			{APIName: "HtmlValue", Label: "HTML Value", Type: FieldString},
			{APIName: "IsActive", Label: "Active", Type: FieldBoolean},
			{APIName: "LastUsedDate", Label: "Last Used Date", Type: FieldDateTime},
			{APIName: "Markup", Label: "Markup", Type: FieldString},
			{APIName: "Name", Label: "Email Template Name", Type: FieldString},
			{APIName: "NamespacePrefix", Label: "Namespace Prefix", Type: FieldString},
			{APIName: "OwnerId", Label: "Owner ID", Type: FieldReference, ReferenceTo: []string{"User"}},
			{APIName: "Subject", Label: "Subject", Type: FieldString},
			{APIName: "TemplateStyle", Label: "Template Style", Type: FieldString, DefaultValue: "none"},
			{APIName: "TemplateType", Label: "Template Type", Type: FieldString, DefaultValue: "text"},
			{APIName: "TimesUsed", Label: "Times Used", Type: FieldInteger},
		}
	case stringsEqualFold(objectName, "KnowledgeArticleVersion") || stringsHasSuffixFold(objectName, "__kav"):
		return []Field{
			{APIName: "ArticleNumber", Label: "Article Number", Type: FieldString},
			{APIName: "FirstPublishedDate", Label: "First Published Date", Type: FieldDateTime},
			{APIName: "IsLatestVersion", Label: "Is Latest Version", Type: FieldBoolean},
			{APIName: "IsVisibleInCsp", Label: "Visible in Customer Portal", Type: FieldBoolean},
			{APIName: "IsVisibleInPkb", Label: "Visible in Public Knowledge Base", Type: FieldBoolean},
			{APIName: "IsVisibleInPrm", Label: "Visible in Partner Portal", Type: FieldBoolean},
			{APIName: "KnowledgeArticleId", Label: "Knowledge Article ID", Type: FieldReference, ReferenceTo: []string{"KnowledgeArticle"}},
			{APIName: "Language", Label: "Language", Type: FieldString},
			{APIName: "LastPublishedDate", Label: "Last Published Date", Type: FieldDateTime},
			{APIName: "PublishStatus", Label: "Publish Status", Type: FieldPicklist},
			{APIName: "RecordTypeId", Label: "Record Type ID", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
			{APIName: "Summary", Label: "Summary", Type: FieldString},
			{APIName: "Title", Label: "Title", Type: FieldString},
			{APIName: "UrlName", Label: "URL Name", Type: FieldString},
			{APIName: "VersionNumber", Label: "Version Number", Type: FieldInteger},
		}
	case stringsHasSuffixFold(objectName, "__mdt"):
		// Public custom metadata records expose metadata identity fields.
		// Plain custom objects and custom settings do not get these by suffix.
		return []Field{
			{APIName: "DeveloperName", Label: "Developer Name", Type: FieldString},
			{APIName: "Label", Label: "Label", Type: FieldString},
			{APIName: "Language", Label: "Language", Type: FieldString},
			{APIName: "MasterLabel", Label: "Master Label", Type: FieldString},
			{APIName: "NamespacePrefix", Label: "Namespace Prefix", Type: FieldString},
			{APIName: "QualifiedApiName", Label: "Qualified API Name", Type: FieldString},
		}
	case stringsHasSuffixFold(objectName, "__c"):
		return []Field{
			{APIName: "Name", Label: "Name", Type: FieldString},
			{APIName: "LastActivityDate", Label: "Last Activity", Type: FieldDate, DisplayType: "DATE"},
			{APIName: "RecordTypeId", Label: "Record Type ID", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
		}
	default:
		return nil
	}
}

func withoutPersonAccountFields(fields []Field) []Field {
	out := fields[:0]
	for _, field := range fields {
		if isPersonAccountField(field.APIName) {
			continue
		}
		out = append(out, field)
	}
	return out
}

func EnsureStandardObject(org *OrgState, objectName string) {
	if org == nil || objectName == "" {
		return
	}
	if canonical, ok := ResolveKnownStandardObjectName(objectName); ok {
		objectName = canonical
	}
	if org.Objects == nil {
		org.Objects = make(map[string]ObjectState)
	}
	state := org.Objects[objectName]
	if state.Definition.APIName == "" {
		state.Definition.APIName = objectName
	}
	if state.Definition.Label == "" {
		state.Definition.Label = objectName
	}
	if state.Definition.PluralLabel == "" {
		state.Definition.PluralLabel = objectName + "s"
	}
	if state.Definition.KeyPrefix == "" {
		state.Definition.KeyPrefix = StandardKeyPrefixes()[objectName]
	}
	if state.Records == nil {
		state.Records = make(map[ID]Record)
	}
	if state.Indexes == nil {
		state.Indexes = make(map[string]IndexSet)
	}
	EnsureStandardObjectFields(&state.Definition)
	if entry, ok := standardObjectCatalogEntryFor(objectName); ok {
		if state.Definition.Label == objectName && entry.Definition.Label != "" {
			state.Definition.Label = entry.Definition.Label
		}
		if state.Definition.PluralLabel == objectName+"s" && entry.Definition.PluralLabel != "" {
			state.Definition.PluralLabel = entry.Definition.PluralLabel
		}
	}
	if state.Definition.KeyPrefix == "" {
		state.Definition.KeyPrefix = AssignDeterministicPrefixes([]string{objectName}, nil)[objectName]
	}
	org.Objects[objectName] = state
	if len(state.Definition.RecordTypes) > 0 {
		ensureRecordTypeObject(org)
		ensureRecordTypeRecords(org)
	}
}

var knownStandardObjectCache struct {
	once          sync.Once
	names         []string
	canonicalByLC map[string]string
	catalogByLC   map[string]standardObjectCatalogEntry
}

func IsKnownStandardObject(objectName string) bool {
	_, ok := ResolveKnownStandardObjectName(objectName)
	return ok
}

func isKnownStandardObjectExact(objectName string) bool {
	if objectName == "" {
		return false
	}
	if StandardKeyPrefixes()[objectName] != "" {
		return true
	}
	if _, ok := standardObjectCatalogData[objectName]; ok {
		return true
	}
	if _, ok := standardSObjectStubFieldData[objectName]; ok {
		return true
	}
	if stringsHasSuffixFold(objectName, "__c") || stringsHasSuffixFold(objectName, "__mdt") {
		return false
	}
	return len(standardFieldsForObject(objectName)) > 0
}

func ResolveKnownStandardObjectName(objectName string) (string, bool) {
	objectName = strings.TrimSpace(objectName)
	if isKnownStandardObjectExact(objectName) {
		return objectName, true
	}
	initKnownStandardObjectCache()
	if candidate, ok := knownStandardObjectCache.canonicalByLC[standardObjectLookupKey(objectName)]; ok {
		return candidate, true
	}
	return "", false
}

func KnownStandardObjectNames() []string {
	initKnownStandardObjectCache()
	return append([]string(nil), knownStandardObjectCache.names...)
}

func initKnownStandardObjectCache() {
	knownStandardObjectCache.once.Do(func() {
		names := buildKnownStandardObjectNameSet()
		out := make([]string, 0, len(names))
		for name := range names {
			out = append(out, name)
		}
		sort.Strings(out)
		canonicalByLC := make(map[string]string, len(out))
		for _, name := range out {
			canonicalByLC[standardObjectLookupKey(name)] = name
		}
		catalogByLC := make(map[string]standardObjectCatalogEntry, len(standardObjectCatalogData))
		for name, entry := range standardObjectCatalogData {
			catalogByLC[standardObjectLookupKey(name)] = entry
		}
		knownStandardObjectCache.names = out
		knownStandardObjectCache.canonicalByLC = canonicalByLC
		knownStandardObjectCache.catalogByLC = catalogByLC
	})
}

func standardObjectLookupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func buildKnownStandardObjectNameSet() map[string]bool {
	names := make(map[string]bool)
	for name := range StandardKeyPrefixes() {
		names[name] = true
	}
	for name := range standardObjectCatalogData {
		names[name] = true
	}
	for _, name := range standardSObjectStubNames() {
		names[name] = true
	}
	for _, name := range []string{
		"Account",
		"Contact",
		"EmailTemplate",
		"EntityDefinition",
		"FieldDefinition",
		"KnowledgeArticleVersion",
		"OpportunityContactRole",
		"OpportunityLineItem",
		"PricebookEntry",
	} {
		names[name] = true
	}
	return names
}

func StandardObjectDefinition(objectName string) (ObjectDefinition, bool) {
	canonical, ok := ResolveKnownStandardObjectName(objectName)
	if !ok {
		return ObjectDefinition{}, false
	}
	objectName = canonical
	definition := ObjectDefinition{APIName: objectName}
	EnsureStandardObjectFields(&definition)
	if definition.Label == "" {
		definition.Label = objectName
	}
	if definition.PluralLabel == "" {
		definition.PluralLabel = objectName + "s"
	}
	if definition.KeyPrefix == "" {
		definition.KeyPrefix = StandardKeyPrefixes()[objectName]
	}
	return definition, true
}

func mergeStandardObjectDefinition(definition *ObjectDefinition, features []string) {
	entry, ok := standardObjectCatalogEntryFor(definition.APIName)
	if !ok {
		return
	}
	if definition.APIName == "" {
		definition.APIName = entry.Definition.APIName
	}
	if definition.Label == "" {
		definition.Label = entry.Definition.Label
	}
	if definition.PluralLabel == "" {
		definition.PluralLabel = entry.Definition.PluralLabel
	}
	if definition.KeyPrefix == "" {
		definition.KeyPrefix = entry.Definition.KeyPrefix
	}
	mergeStandardFields(definition, entry.Definition.Fields)
	mergeStandardRelationships(definition, entry.Definition.Relations)
	if len(definition.RecordTypes) == 0 {
		mergeStandardRecordTypes(definition, entry.Definition.RecordTypes)
	}
	for _, feature := range features {
		feature = canonicalFeatureName(feature)
		if feature == "" {
			continue
		}
		mergeStandardFields(definition, entry.FeatureFields[feature])
		mergeStandardRecordTypes(definition, entry.FeatureRecordTypes[feature])
	}
}

func mergeStandardSObjectStubFields(definition *ObjectDefinition, features []string) {
	mergeStandardSObjectStubObjectInfo(definition)
	fields, ok := standardSObjectStubFieldsFor(definition.APIName)
	if !ok {
		return
	}
	if stringsEqualFold(definition.APIName, "Account") && !hasCanonicalFeature(features, "PersonAccounts") {
		fields = withoutPersonAccountFieldMap(fields)
	}
	mergeStandardFields(definition, fields)
	applyStandardSObjectStubReadOnlyFields(definition)
}

func mergeStandardSObjectStubObjectInfo(definition *ObjectDefinition) {
	info, ok := standardSObjectStubObjectInfoFor(definition.APIName)
	if !ok {
		return
	}
	if definition.Label == "" || stringsEqualFold(definition.Label, definition.APIName) {
		definition.Label = info.Label
	}
	if definition.PluralLabel == "" || stringsEqualFold(definition.PluralLabel, definition.APIName+"s") {
		definition.PluralLabel = info.PluralLabel
	}
}

func applyStandardSObjectStubReadOnlyFields(definition *ObjectDefinition) {
	readOnlyFields, ok := standardSObjectStubReadOnlyFieldsFor(definition.APIName)
	if !ok {
		return
	}
	for _, name := range readOnlyFields {
		field, ok := definition.Fields[name]
		if !ok {
			continue
		}
		if field.Createable == nil {
			field.Createable = BoolFlag(false)
		}
		if field.Updateable == nil {
			field.Updateable = BoolFlag(false)
		}
		definition.Fields[name] = field
	}
}

func mergeStandardSObjectStubRelationships(definition *ObjectDefinition, features []string) {
	relationships, ok := standardSObjectStubRelationshipsFor(definition.APIName)
	if !ok {
		return
	}
	if !hasCanonicalFeature(features, "PersonAccounts") {
		relationships = withoutPersonAccountRelationships(relationships)
	}
	mergeStandardRelationships(definition, relationships)
}

func withoutPersonAccountFieldMap(fields map[string]Field) map[string]Field {
	filtered := make(map[string]Field, len(fields))
	for name, field := range fields {
		if isPersonAccountField(name) {
			continue
		}
		filtered[name] = field
	}
	return filtered
}

func withoutPersonAccountRelationships(relationships []Relationship) []Relationship {
	filtered := make([]Relationship, 0, len(relationships))
	for _, relationship := range relationships {
		if isPersonAccountRelationship(relationship) {
			continue
		}
		filtered = append(filtered, relationship)
	}
	return filtered
}

func isPersonAccountRelationship(relationship Relationship) bool {
	if !parentObjectsContain(relationship.ParentObjects, "Account") {
		return false
	}
	return isPersonAccountField(relationship.ChildRelationship) || stringsEqualFold(relationship.Field, "ContactId") || stringsEqualFold(relationship.Field, "WhoId")
}

func parentObjectsContain(values []string, want string) bool {
	for _, value := range values {
		if stringsEqualFold(value, want) {
			return true
		}
	}
	return false
}

func isPersonAccountField(name string) bool {
	switch {
	case strings.HasPrefix(name, "Person"):
		return true
	case stringsEqualFold(name, "FirstName"),
		stringsEqualFold(name, "LastName"),
		stringsEqualFold(name, "MiddleName"),
		stringsEqualFold(name, "Suffix"),
		stringsEqualFold(name, "Salutation"),
		stringsEqualFold(name, "IsPersonAccount"):
		return true
	default:
		return false
	}
}

func applyStandardObjectCompatibilityOverlays(definition *ObjectDefinition) {
	switch {
	case stringsEqualFold(definition.APIName, "Account"):
		markFieldRequired(definition, "Name")
	case stringsEqualFold(definition.APIName, "Asset"):
		ensureField(definition, Field{APIName: "ExternalIdentifier", Label: "External Identifier", Type: FieldString, DisplayType: "STRING", Length: 255, Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		ensureField(definition, Field{APIName: "CurrentMrr", Label: "Current MRR", Type: FieldDecimal, DisplayType: "CURRENCY", Precision: 18, Scale: 2, Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		ensureField(definition, Field{APIName: "Uuid", Label: "UUID", Type: FieldString, DisplayType: "STRING", Length: 255, Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		ensureField(definition, Field{APIName: "StatusReason", Label: "Status Reason", Type: FieldPicklist, DisplayType: "PICKLIST", Length: 255, Createable: BoolFlag(true), Updateable: BoolFlag(true)})
	case stringsEqualFold(definition.APIName, "Attachment"):
		ensureReferenceTarget(definition, "ParentId", "User")
	case stringsEqualFold(definition.APIName, "Document"):
		ensureReferenceTarget(definition, "FolderId", "User")
	case stringsEqualFold(definition.APIName, "ContentVersion"):
		allowGeneratedContentDocument(definition)
	case stringsEqualFold(definition.APIName, "ContentDistribution"):
		markFieldCreateable(definition, "ContentVersionId")
	case stringsEqualFold(definition.APIName, "Case"):
		markFieldOptional(definition, "BusinessHoursId")
	case stringsEqualFold(definition.APIName, "EmailMessage"):
		ensureField(definition, Field{APIName: "RelatedToId", Label: "Related To ID", Type: FieldReference, Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		ensureField(definition, Field{APIName: "ToIds", Label: "To IDs", Type: FieldAny, Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		markFieldOptional(definition, "ParentId")
		markFieldCreateable(definition, "Incoming")
		markFieldCreateable(definition, "IsClientManaged")
	case stringsEqualFold(definition.APIName, "EmailMessageRelation"):
		markFieldCreateable(definition, "EmailMessageId")
		markFieldCreateable(definition, "RelationId")
		markFieldCreateable(definition, "RelationType")
		markFieldCreateable(definition, "RelationAddress")
	case stringsEqualFold(definition.APIName, "EmailTemplate"):
		ensureFieldDefault(definition, "TemplateStyle", "none")
		ensureFieldDefault(definition, "TemplateType", "text")
	case stringsEqualFold(definition.APIName, "Lead"):
		markFieldRequired(definition, "Company")
		markFieldRequired(definition, "LastName")
		ensureField(definition, Field{APIName: "DoNotCall", Label: "Do Not Call", Type: FieldBoolean, DisplayType: "BOOLEAN", DefaultValue: "false", Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		ensureField(definition, Field{APIName: "HasOptedOutOfEmail", Label: "Email Opt Out", Type: FieldBoolean, DisplayType: "BOOLEAN", DefaultValue: "false", Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		ensureField(definition, Field{APIName: "HasOptedOutOfFax", Label: "Fax Opt Out", Type: FieldBoolean, DisplayType: "BOOLEAN", DefaultValue: "false", Createable: BoolFlag(true), Updateable: BoolFlag(true)})
	case stringsEqualFold(definition.APIName, "User"):
		ensureField(definition, Field{APIName: "AssistantName", Label: "Assistant", Type: FieldString, DisplayType: "STRING", Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		ensureField(definition, Field{APIName: "LeadSource", Label: "Lead Source", Type: FieldPicklist, DisplayType: "PICKLIST", Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		ensureField(definition, Field{APIName: "Salutation", Label: "Salutation", Type: FieldPicklist, DisplayType: "PICKLIST", Createable: BoolFlag(true), Updateable: BoolFlag(true)})
		ensureFieldDefault(definition, "Alias", "local")
		ensureFieldDefault(definition, "Email", "local-user@example.invalid")
		ensureFieldDefault(definition, "EmailEncodingKey", "UTF-8")
		ensureFieldDefault(definition, "LanguageLocaleKey", "en_US")
		ensureFieldDefault(definition, "LocaleSidKey", "en_US")
		ensureFieldDefault(definition, "ProfileId", "00e000000000001")
		ensureFieldDefault(definition, "TimeZoneSidKey", "UTC")
		ensureFieldDefault(definition, "Username", "local-user@example.invalid")
	case stringsEqualFold(definition.APIName, "EntityDefinition"):
		ensureReferenceShape(definition, "RunningUserEntityAccessId", []string{"UserEntityAccess"}, "RunningUserEntityAccess")
	case stringsEqualFold(definition.APIName, "EntityParticle"):
		ensureReferenceShape(definition, "FieldDefinitionId", []string{"FieldDefinition"}, "FieldDefinition")
	case stringsEqualFold(definition.APIName, "FieldDefinition"):
		ensureReferenceShape(definition, "RunningUserFieldAccessId", []string{"UserFieldAccess"}, "RunningUserFieldAccess")
	case stringsEqualFold(definition.APIName, "Event"):
		removeField(definition, "Name")
		ensureField(definition, Field{APIName: "Type", Label: "Type", Type: FieldPicklist, DisplayType: "PICKLIST", Length: 255, Createable: BoolFlag(true), Updateable: BoolFlag(true)})
	case stringsEqualFold(definition.APIName, "PermissionSetGroupComponent"):
		markFieldCreateable(definition, "PermissionSetGroupId")
		markFieldCreateable(definition, "PermissionSetId")
	}
}

func ensureField(definition *ObjectDefinition, field Field) {
	if definition == nil || field.APIName == "" {
		return
	}
	if _, ok := ResolveFieldName(*definition, "", field.APIName); ok {
		return
	}
	definition.Fields[field.APIName] = field
}

func ensureReferenceShape(definition *ObjectDefinition, fieldName string, referenceTo []string, relationshipName string) {
	if definition == nil {
		return
	}
	resolved, ok := ResolveFieldName(*definition, "", fieldName)
	if !ok {
		ensureField(definition, Field{APIName: fieldName, Type: FieldReference, ReferenceTo: referenceTo, RelationshipName: relationshipName})
		return
	}
	field := definition.Fields[resolved]
	if field.Type == "" || field.Type == FieldAny {
		field.Type = FieldReference
	}
	if len(field.ReferenceTo) == 0 {
		field.ReferenceTo = append([]string(nil), referenceTo...)
	} else {
		field.ReferenceTo = appendUniqueStringsFold(field.ReferenceTo, referenceTo...)
	}
	if field.RelationshipName == "" {
		field.RelationshipName = relationshipName
	}
	definition.Fields[resolved] = field
}

func removeField(definition *ObjectDefinition, fieldName string) {
	resolved, ok := ResolveFieldName(*definition, "", fieldName)
	if !ok {
		return
	}
	delete(definition.Fields, resolved)
}

func markFieldRequired(definition *ObjectDefinition, fieldName string) {
	resolved, ok := ResolveFieldName(*definition, "", fieldName)
	if !ok {
		return
	}
	field := definition.Fields[resolved]
	field.Required = true
	definition.Fields[resolved] = field
}

func allowGeneratedContentDocument(definition *ObjectDefinition) {
	markFieldOptional(definition, "ContentDocumentId")
}

func markFieldOptional(definition *ObjectDefinition, fieldName string) {
	resolved, ok := ResolveFieldName(*definition, "", fieldName)
	if !ok {
		return
	}
	field := definition.Fields[resolved]
	field.Required = false
	definition.Fields[resolved] = field
}

func markFieldCreateable(definition *ObjectDefinition, fieldName string) {
	resolved, ok := ResolveFieldName(*definition, "", fieldName)
	if !ok {
		return
	}
	field := definition.Fields[resolved]
	field.Createable = BoolFlag(true)
	definition.Fields[resolved] = field
}

func ensureFieldDefault(definition *ObjectDefinition, fieldName string, defaultValue string) {
	resolved, ok := ResolveFieldName(*definition, "", fieldName)
	if !ok {
		return
	}
	field := definition.Fields[resolved]
	if field.DefaultValue == "" {
		field.DefaultValue = defaultValue
		definition.Fields[resolved] = field
	}
}

func ensureReferenceTarget(definition *ObjectDefinition, fieldName string, targetName string) {
	if field, ok := definition.Fields[fieldName]; ok {
		field.ReferenceTo = appendUniqueStringsFold(field.ReferenceTo, targetName)
		definition.Fields[fieldName] = field
	}
	for i := range definition.Relations {
		if stringsEqualFold(definition.Relations[i].Field, fieldName) {
			definition.Relations[i].ParentObjects = appendUniqueStringsFold(definition.Relations[i].ParentObjects, targetName)
		}
	}
}

func appendUniqueStringsFold(values []string, additions ...string) []string {
	for _, addition := range additions {
		if addition == "" {
			continue
		}
		found := false
		for _, value := range values {
			if stringsEqualFold(value, addition) {
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

func mergeStandardFields(definition *ObjectDefinition, fields map[string]Field) {
	for _, field := range fields {
		if existingName, ok := ResolveFieldName(*definition, "", field.APIName); ok {
			existing := definition.Fields[existingName]
			enrichStandardField(&existing, field)
			definition.Fields[existingName] = existing
			continue
		}
		definition.Fields[field.APIName] = cloneField(field)
	}
}

func enrichStandardField(existing *Field, field Field) {
	if existing.APIName == "" {
		existing.APIName = field.APIName
	}
	if existing.Label == "" {
		existing.Label = field.Label
	}
	if existing.Type == "" || existing.Type == FieldAny {
		existing.Type = field.Type
	}
	if existing.DisplayType == "" {
		existing.DisplayType = field.DisplayType
	}
	if existing.Length == 0 {
		existing.Length = field.Length
	}
	if existing.Precision == 0 {
		existing.Precision = field.Precision
	}
	if existing.Scale == 0 {
		existing.Scale = field.Scale
	}
	if existing.Formula == "" {
		existing.Formula = field.Formula
	}
	if existing.DefaultValue == "" {
		existing.DefaultValue = field.DefaultValue
	}
	if existing.CompoundFieldName == "" {
		existing.CompoundFieldName = field.CompoundFieldName
	}
	if existing.DisplayFormat == "" {
		existing.DisplayFormat = field.DisplayFormat
	}
	if existing.SummarizedField == "" {
		existing.SummarizedField = field.SummarizedField
	}
	if existing.SummaryForeignKey == "" {
		existing.SummaryForeignKey = field.SummaryForeignKey
	}
	if existing.SummaryOperation == "" {
		existing.SummaryOperation = field.SummaryOperation
	}
	if len(existing.SummaryFilterItems) == 0 && len(field.SummaryFilterItems) != 0 {
		existing.SummaryFilterItems = append([]SummaryFilterItem(nil), field.SummaryFilterItems...)
	}
	if field.AutoNumber {
		existing.AutoNumber = true
	}
	if field.ExternalID {
		existing.ExternalID = true
	}
	if field.Unique {
		existing.Unique = true
	}
	if field.Encrypted {
		existing.Encrypted = true
	}
	if field.CaseSensitive {
		existing.CaseSensitive = true
	}
	if existing.Nillable == nil {
		existing.Nillable = cloneBoolFlag(field.Nillable)
	}
	if existing.DefaultedOnCreate == nil {
		existing.DefaultedOnCreate = cloneBoolFlag(field.DefaultedOnCreate)
	}
	if existing.Accessible == nil {
		existing.Accessible = cloneBoolFlag(field.Accessible)
	}
	if existing.Createable == nil {
		existing.Createable = cloneBoolFlag(field.Createable)
	}
	if existing.Updateable == nil {
		existing.Updateable = cloneBoolFlag(field.Updateable)
	}
	if existing.Filterable == nil {
		existing.Filterable = cloneBoolFlag(field.Filterable)
	}
	if existing.Groupable == nil {
		existing.Groupable = cloneBoolFlag(field.Groupable)
	}
	if existing.Sortable == nil {
		existing.Sortable = cloneBoolFlag(field.Sortable)
	}
	if existing.Aggregatable == nil {
		existing.Aggregatable = cloneBoolFlag(field.Aggregatable)
	}
	if existing.Permissionable == nil {
		existing.Permissionable = cloneBoolFlag(field.Permissionable)
	}
	if existing.DeprecatedAndHidden == nil {
		existing.DeprecatedAndHidden = cloneBoolFlag(field.DeprecatedAndHidden)
	}
	if existing.ChildRelationshipName == "" {
		existing.ChildRelationshipName = field.ChildRelationshipName
	}
	if existing.RelationshipName == "" {
		existing.RelationshipName = field.RelationshipName
	}
	if len(existing.ReferenceTo) == 0 && len(field.ReferenceTo) != 0 {
		existing.ReferenceTo = append([]string(nil), field.ReferenceTo...)
	} else if len(field.ReferenceTo) != 0 {
		existing.ReferenceTo = appendUniqueStringsFold(existing.ReferenceTo, field.ReferenceTo...)
	}
	if len(existing.PicklistValues) == 0 && len(field.PicklistValues) != 0 {
		existing.PicklistValues = append([]PicklistValue(nil), field.PicklistValues...)
	}
}

func mergeStandardRelationships(definition *ObjectDefinition, relationships []Relationship) {
	for _, relationship := range relationships {
		if relationship.Field == "" {
			continue
		}
		found := false
		for i, existing := range definition.Relations {
			if sameStandardRelationship(existing, relationship) {
				definition.Relations[i].ParentObjects = appendUniqueStringsFold(definition.Relations[i].ParentObjects, relationship.ParentObjects...)
				if definition.Relations[i].ParentRelationship == "" && relationship.ParentRelationship != "" {
					definition.Relations[i].ParentRelationship = relationship.ParentRelationship
				}
				if definition.Relations[i].ChildRelationship == "" && relationship.ChildRelationship != "" {
					definition.Relations[i].ChildRelationship = relationship.ChildRelationship
				}
				if relationship.Polymorphic {
					definition.Relations[i].Polymorphic = true
				}
				found = true
				break
			}
		}
		if !found {
			copy := relationship
			copy.ParentObjects = append([]string(nil), relationship.ParentObjects...)
			definition.Relations = append(definition.Relations, copy)
		}
	}
}

func sameStandardRelationship(left, right Relationship) bool {
	if !stringsEqualFold(left.Field, right.Field) {
		return false
	}
	if left.ParentRelationship != "" && right.ParentRelationship != "" && !stringsEqualFold(left.ParentRelationship, right.ParentRelationship) {
		return false
	}
	if left.ChildRelationship != "" && right.ChildRelationship != "" && !stringsEqualFold(left.ChildRelationship, right.ChildRelationship) {
		return false
	}
	if (left.ChildRelationship == "") != (right.ChildRelationship == "") && len(left.ParentObjects) != 0 && len(right.ParentObjects) != 0 && !parentObjectsOverlap(left.ParentObjects, right.ParentObjects) {
		return false
	}
	return true
}

func parentObjectsOverlap(left, right []string) bool {
	for _, l := range left {
		for _, r := range right {
			if stringsEqualFold(l, r) {
				return true
			}
		}
	}
	return false
}

func mergeStandardRecordTypes(definition *ObjectDefinition, recordTypes []RecordTypeInfo) {
	for _, recordType := range recordTypes {
		if recordType.DeveloperName == "" {
			continue
		}
		found := false
		for _, existing := range definition.RecordTypes {
			if stringsEqualFold(existing.DeveloperName, recordType.DeveloperName) {
				found = true
				break
			}
		}
		if !found {
			definition.RecordTypes = append(definition.RecordTypes, recordType)
		}
	}
}

func cloneField(field Field) Field {
	field.ReferenceTo = append([]string(nil), field.ReferenceTo...)
	field.PicklistValues = append([]PicklistValue(nil), field.PicklistValues...)
	return field
}

func cloneBoolFlag(flag *bool) *bool {
	if flag == nil {
		return nil
	}
	value := *flag
	return &value
}

func canonicalFeatureName(feature string) string {
	if idx := strings.IndexByte(feature, ':'); idx >= 0 {
		feature = feature[:idx]
	}
	switch {
	case stringsEqualFold(feature, "PersonAccounts"):
		return "PersonAccounts"
	case stringsEqualFold(feature, "MultiCurrency"):
		return "MultiCurrency"
	case stringsEqualFold(feature, "Sites"):
		return "Sites"
	case stringsEqualFold(feature, "Communities") || stringsEqualFold(feature, "FlowSites"):
		return "Communities"
	case stringsEqualFold(feature, "StateAndCountryPicklist"):
		return "StateAndCountryPicklist"
	case stringsEqualFold(feature, "ContactsToMultipleAccounts"):
		return "ContactsToMultipleAccounts"
	case stringsEqualFold(feature, "PlatformCache") || stringsEqualFold(feature, "ProviderFreePlatformCache"):
		return "PlatformCache"
	case stringsEqualFold(feature, "EnableSetPasswordInApi"):
		return "EnableSetPasswordInApi"
	case stringsEqualFold(feature, "AddCustomApps"):
		return "AddCustomApps"
	case stringsEqualFold(feature, "AnalyticsAdminPerms"):
		return "AnalyticsAdminPerms"
	case strings.HasPrefix(strings.ToLower(feature), "healthcloud"):
		return "HealthCloud"
	case stringsEqualFold(feature, "LightningExperience"):
		return "LightningExperience"
	case stringsEqualFold(feature, "Chatter"):
		return "Chatter"
	default:
		return feature
	}
}

func hasCanonicalFeature(features []string, want string) bool {
	for _, feature := range features {
		if canonicalFeatureName(feature) == want {
			return true
		}
	}
	return false
}

func removeStateAndCountryPicklistFields(definition *ObjectDefinition) {
	for fieldName := range definition.Fields {
		if isStateAndCountryPicklistField(fieldName) {
			delete(definition.Fields, fieldName)
		}
	}
}

func isStateAndCountryPicklistField(fieldName string) bool {
	return strings.HasSuffix(fieldName, "StateCode") || strings.HasSuffix(fieldName, "CountryCode")
}

func stringsEqualFold(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		l, r := left[i], right[i]
		if l >= 'A' && l <= 'Z' {
			l += 'a' - 'A'
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		if l != r {
			return false
		}
	}
	return true
}

func stringsHasSuffixFold(value, suffix string) bool {
	if len(value) < len(suffix) {
		return false
	}
	return stringsEqualFold(value[len(value)-len(suffix):], suffix)
}

func ensureStandardRelationship(definition *ObjectDefinition, field Field) {
	relationshipName := ParentRelationshipName(field)
	if relationshipName == "" || len(field.ReferenceTo) == 0 {
		return
	}
	matchingFields := 0
	for _, relation := range definition.Relations {
		if stringsEqualFold(relation.Field, field.APIName) {
			matchingFields++
		}
	}
	childRelationshipName := ChildRelationshipName(field)
	if matchingFields > 1 && childRelationshipName == "" {
		return
	}
	for i, relation := range definition.Relations {
		if stringsEqualFold(relation.Field, field.APIName) {
			if childRelationshipName != "" && relation.ChildRelationship != "" && !stringsEqualFold(relation.ChildRelationship, childRelationshipName) {
				continue
			}
			definition.Relations[i].ParentRelationship = relationshipName
			definition.Relations[i].ParentObjects = appendUniqueStringsFold(definition.Relations[i].ParentObjects, field.ReferenceTo...)
			if definition.Relations[i].ChildRelationship == "" && childRelationshipName != "" {
				definition.Relations[i].ChildRelationship = childRelationshipName
			}
			return
		}
		if stringsEqualFold(relation.ParentRelationship, relationshipName) {
			if definition.Relations[i].ChildRelationship == "" && childRelationshipName != "" {
				definition.Relations[i].ChildRelationship = childRelationshipName
			}
			return
		}
	}
	definition.Relations = append(definition.Relations, Relationship{
		Field:              field.APIName,
		ParentObjects:      append([]string(nil), field.ReferenceTo...),
		ParentRelationship: relationshipName,
		ChildRelationship:  childRelationshipName,
		Polymorphic:        len(field.ReferenceTo) > 1,
	})
}

func ParentRelationshipName(field Field) string {
	switch {
	case stringsHasSuffixFold(field.APIName, "__c"):
		return field.APIName[:len(field.APIName)-len("__c")] + "__r"
	case field.RelationshipName != "":
		return field.RelationshipName
	case stringsHasSuffixFold(field.APIName, "Id"):
		return field.APIName[:len(field.APIName)-len("Id")]
	default:
		return ""
	}
}

func ChildRelationshipName(field Field) string {
	if field.ChildRelationshipName != "" {
		return field.ChildRelationshipName
	}
	if stringsHasSuffixFold(field.APIName, "__c") && field.RelationshipName != "" && !stringsHasSuffixFold(field.RelationshipName, "__r") {
		return field.RelationshipName + "__r"
	}
	return ""
}
