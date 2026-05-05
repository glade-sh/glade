package storage

// EnsureStandardObjectFields adds public Salesforce standard fields for objects
// whose project metadata commonly only carries custom-field deltas.
func EnsureStandardObjectFields(definition *ObjectDefinition) {
	if definition == nil {
		return
	}
	if definition.Fields == nil {
		definition.Fields = make(map[string]Field)
	}
	for _, field := range standardFieldsForObject(definition.APIName) {
		if _, ok := ResolveFieldName(*definition, "", field.APIName); ok {
			continue
		}
		definition.Fields[field.APIName] = field
	}
}

func standardFieldsForObject(objectName string) []Field {
	switch {
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
	default:
		return nil
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
