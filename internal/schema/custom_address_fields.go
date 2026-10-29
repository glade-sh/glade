package schema

import "strings"

// CustomAddressComponent is one platform-generated component of a custom
// Address field. Source metadata declares only the compound field; Salesforce
// describes these components as ordinary fields named <Base>__<Suffix>__s.
// Facets come from native describe captures at API 62.0 and 67.0
// (testdata/conformance/custom_address_fields).
type CustomAddressComponent struct {
	Suffix     string
	LabelPart  string
	Type       string
	Length     int
	Precision  int
	Scale      int
	Picklist   bool
	Groupable  bool
	AddressKey string
}

// CustomAddressComponents lists the components in native describe order.
var CustomAddressComponents = []CustomAddressComponent{
	{Suffix: "Street", LabelPart: "Street", Type: "TextArea", Length: 255, Groupable: true, AddressKey: "street"},
	{Suffix: "City", LabelPart: "City", Type: "Text", Length: 40, Groupable: true, AddressKey: "city"},
	{Suffix: "PostalCode", LabelPart: "ZIP/Postal Code", Type: "Text", Length: 20, Groupable: true, AddressKey: "postalCode"},
	{Suffix: "StateCode", LabelPart: "State/Province", Type: "Picklist", Length: 10, Picklist: true, Groupable: true, AddressKey: "stateCode"},
	{Suffix: "CountryCode", LabelPart: "Country/Territory", Type: "Picklist", Length: 10, Picklist: true, Groupable: true, AddressKey: "countryCode"},
	{Suffix: "Latitude", LabelPart: "Latitude", Type: "Number", Precision: 18, Scale: 15, AddressKey: "latitude"},
	{Suffix: "Longitude", LabelPart: "Longitude", Type: "Number", Precision: 18, Scale: 15, AddressKey: "longitude"},
	{Suffix: "GeocodeAccuracy", LabelPart: "Geocode Accuracy", Type: "Picklist", Length: 40, Picklist: true, Groupable: true, AddressKey: "geocodeAccuracy"},
}

// IsCustomAddressField reports whether a source field is a custom Address.
func IsCustomAddressField(field Field) bool {
	return strings.EqualFold(strings.TrimSpace(field.Type), "Address") && hasSuffixFold(field.Name, "__c") && len(field.Name) > 3
}

// CustomAddressComponentName returns <Base>__<Suffix>__s for a compound
// custom Address field named <Base>__c.
func CustomAddressComponentName(compoundName, suffix string) string {
	return compoundName[:len(compoundName)-len("__c")] + "__" + suffix + "__s"
}

// CustomAddressComponentFor matches a component field name to its compound
// custom Address field name, case-insensitively.
func CustomAddressComponentFor(compoundName, componentName string) (CustomAddressComponent, bool) {
	if !hasSuffixFold(compoundName, "__c") || len(compoundName) <= 3 {
		return CustomAddressComponent{}, false
	}
	for _, component := range CustomAddressComponents {
		if strings.EqualFold(CustomAddressComponentName(compoundName, component.Suffix), componentName) {
			return component, true
		}
	}
	return CustomAddressComponent{}, false
}

func customAddressComponentFields(compound Field) []Field {
	label := strings.TrimSpace(compound.Label)
	if label == "" {
		label = strings.TrimSuffix(compound.Name, "__c")
	}
	yes, no := true, false
	out := make([]Field, 0, len(CustomAddressComponents))
	for _, component := range CustomAddressComponents {
		groupable := no
		if component.Groupable {
			groupable = yes
		}
		field := Field{
			Name:                  CustomAddressComponentName(compound.Name, component.Suffix),
			File:                  compound.File,
			Label:                 label + " (" + component.LabelPart + ")",
			Type:                  component.Type,
			Length:                component.Length,
			Precision:             component.Precision,
			Scale:                 component.Scale,
			ScaleSpecified:        component.Scale != 0,
			RestrictedPicklist:    component.Picklist,
			CompoundFieldName:     compound.Name,
			Nillable:              &yes,
			DefaultedOnCreate:     &no,
			Createable:            &yes,
			Updateable:            &yes,
			Filterable:            &yes,
			Sortable:              &yes,
			Groupable:             &groupable,
			Aggregatable:          &yes,
			DescribeCaseSensitive: &no,
		}
		if component.Suffix == "StateCode" {
			field.PicklistController = CustomAddressComponentName(compound.Name, "CountryCode")
		}
		out = append(out, field)
	}
	return out
}

// addCustomAddressComponentFields expands every custom Address field into its
// platform-generated components and fixes the compound field's native facets
// (read-only, filterable only through DISTANCE, not sortable or groupable).
func addCustomAddressComponentFields(objects map[string]*Object) {
	for _, object := range objects {
		var components []Field
		for i := range object.Fields {
			field := &object.Fields[i]
			if !IsCustomAddressField(*field) {
				continue
			}
			yes, no := true, false
			field.Nillable, field.Createable, field.Updateable = &yes, &no, &no
			field.Filterable, field.Sortable, field.Groupable, field.Aggregatable = &yes, &no, &no, &no
			for _, component := range customAddressComponentFields(*field) {
				if fieldIndex(object.Fields, component.Name) < 0 && fieldIndex(components, component.Name) < 0 {
					components = append(components, component)
				}
			}
		}
		object.Fields = append(object.Fields, components...)
	}
}

func hasSuffixFold(value, suffix string) bool {
	return len(value) >= len(suffix) && strings.EqualFold(value[len(value)-len(suffix):], suffix)
}
