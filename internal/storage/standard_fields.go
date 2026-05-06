package storage

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
	if definition.Fields == nil {
		definition.Fields = make(map[string]Field)
	}
	if field, ok := definition.Fields["Id"]; !ok || field.APIName == "" {
		definition.Fields["Id"] = Field{APIName: "Id", Label: "Record ID", Type: FieldID}
	}
	mergeStandardObjectDefinition(definition, features)
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
	for _, field := range definition.Fields {
		ensureStandardRelationship(definition, field)
	}
}

func standardFieldsForObject(objectName string) []Field {
	if _, ok := standardObjectCatalogEntryFor(objectName); ok {
		return nil
	}
	switch {
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
	case stringsEqualFold(objectName, "Account"):
		return []Field{
			{APIName: "Name", Label: "Account Name", Type: FieldString},
			{APIName: "AccountNumber", Label: "Account Number", Type: FieldString},
			{APIName: "AnnualRevenue", Label: "Annual Revenue", Type: FieldDecimal},
			{APIName: "BillingStreet", Label: "Billing Street", Type: FieldString},
			{APIName: "BillingCity", Label: "Billing City", Type: FieldString},
			{APIName: "BillingState", Label: "Billing State", Type: FieldString},
			{APIName: "BillingPostalCode", Label: "Billing Zip/Postal Code", Type: FieldString},
			{APIName: "BillingCountry", Label: "Billing Country", Type: FieldString},
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
			{APIName: "PersonEmail", Label: "Person Email", Type: FieldString},
			{APIName: "PersonMailingStreet", Label: "Mailing Street", Type: FieldString},
			{APIName: "PersonMailingCity", Label: "Mailing City", Type: FieldString},
			{APIName: "PersonMailingState", Label: "Mailing State", Type: FieldString},
			{APIName: "PersonMailingPostalCode", Label: "Mailing Zip/Postal Code", Type: FieldString},
			{APIName: "PersonMailingCountry", Label: "Mailing Country", Type: FieldString},
			{APIName: "PersonOtherStreet", Label: "Other Street", Type: FieldString},
			{APIName: "PersonOtherCity", Label: "Other City", Type: FieldString},
			{APIName: "PersonOtherState", Label: "Other State", Type: FieldString},
			{APIName: "PersonOtherPostalCode", Label: "Other Zip/Postal Code", Type: FieldString},
			{APIName: "PersonOtherCountry", Label: "Other Country", Type: FieldString},
			{APIName: "Phone", Label: "Account Phone", Type: FieldString},
			{APIName: "Rating", Label: "Rating", Type: FieldPicklist},
			{APIName: "RecordTypeId", Label: "Record Type ID", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
			{APIName: "ShippingStreet", Label: "Shipping Street", Type: FieldString},
			{APIName: "ShippingCity", Label: "Shipping City", Type: FieldString},
			{APIName: "ShippingState", Label: "Shipping State", Type: FieldString},
			{APIName: "ShippingPostalCode", Label: "Shipping Zip/Postal Code", Type: FieldString},
			{APIName: "ShippingCountry", Label: "Shipping Country", Type: FieldString},
			{APIName: "ShippingLatitude", Label: "Shipping Latitude", Type: FieldDecimal},
			{APIName: "ShippingLongitude", Label: "Shipping Longitude", Type: FieldDecimal},
			{APIName: "Sic", Label: "SIC Code", Type: FieldString},
			{APIName: "Site", Label: "Account Site", Type: FieldString},
			{APIName: "TickerSymbol", Label: "Ticker Symbol", Type: FieldString},
			{APIName: "Type", Label: "Account Type", Type: FieldPicklist},
			{APIName: "Website", Label: "Website", Type: FieldString},
		}
	case stringsEqualFold(objectName, "Contact"):
		return []Field{
			{APIName: "Name", Label: "Full Name", Type: FieldString},
			{APIName: "FirstName", Label: "First Name", Type: FieldString},
			{APIName: "LastName", Label: "Last Name", Type: FieldString},
			{APIName: "Salutation", Label: "Salutation", Type: FieldPicklist},
			{APIName: "Title", Label: "Title", Type: FieldString},
			{APIName: "Email", Label: "Email", Type: FieldString},
			{APIName: "HomePhone", Label: "Home Phone", Type: FieldString},
			{APIName: "Phone", Label: "Business Phone", Type: FieldString},
			{APIName: "MailingStreet", Label: "Mailing Street", Type: FieldString},
			{APIName: "MailingCity", Label: "Mailing City", Type: FieldString},
			{APIName: "MailingState", Label: "Mailing State", Type: FieldString},
			{APIName: "MailingPostalCode", Label: "Mailing Zip/Postal Code", Type: FieldString},
			{APIName: "MailingCountry", Label: "Mailing Country", Type: FieldString},
			{APIName: "OtherStreet", Label: "Other Street", Type: FieldString},
			{APIName: "OtherCity", Label: "Other City", Type: FieldString},
			{APIName: "OtherState", Label: "Other State", Type: FieldString},
			{APIName: "OtherPostalCode", Label: "Other Zip/Postal Code", Type: FieldString},
			{APIName: "OtherCountry", Label: "Other Country", Type: FieldString},
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
			{APIName: "TemplateStyle", Label: "Template Style", Type: FieldString},
			{APIName: "TemplateType", Label: "Template Type", Type: FieldString},
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
			{APIName: "RecordTypeId", Label: "Record Type ID", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
		}
	default:
		return nil
	}
}

func EnsureStandardObject(org *OrgState, objectName string) {
	if org == nil || objectName == "" {
		return
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
	org.Objects[objectName] = state
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
	mergeStandardRecordTypes(definition, entry.Definition.RecordTypes)
	for _, feature := range features {
		feature = canonicalFeatureName(feature)
		if feature == "" {
			continue
		}
		mergeStandardFields(definition, entry.FeatureFields[feature])
		mergeStandardRecordTypes(definition, entry.FeatureRecordTypes[feature])
	}
}

func mergeStandardFields(definition *ObjectDefinition, fields map[string]Field) {
	for _, field := range fields {
		if _, ok := ResolveFieldName(*definition, "", field.APIName); ok {
			continue
		}
		definition.Fields[field.APIName] = cloneField(field)
	}
}

func mergeStandardRelationships(definition *ObjectDefinition, relationships []Relationship) {
	for _, relationship := range relationships {
		if relationship.Field == "" {
			continue
		}
		found := false
		for _, existing := range definition.Relations {
			if stringsEqualFold(existing.Field, relationship.Field) {
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

func canonicalFeatureName(feature string) string {
	switch {
	case stringsEqualFold(feature, "PersonAccounts"):
		return "PersonAccounts"
	case stringsEqualFold(feature, "MultiCurrency"):
		return "MultiCurrency"
	default:
		return feature
	}
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
	for i, relation := range definition.Relations {
		if relation.Field == field.APIName {
			definition.Relations[i].ParentRelationship = relationshipName
			definition.Relations[i].ParentObjects = append([]string(nil), field.ReferenceTo...)
			return
		}
		if relation.ParentRelationship == relationshipName {
			return
		}
	}
	definition.Relations = append(definition.Relations, Relationship{
		Field:              field.APIName,
		ParentObjects:      append([]string(nil), field.ReferenceTo...),
		ParentRelationship: relationshipName,
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
