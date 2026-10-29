package storage

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/schema"
)

// Custom Address components share the platform State/Country picklists and
// geocode accuracy choices. Native describes at API 62.0 and 67.0 list the
// same values, labels, order and validFor bitmaps as the catalog's standard
// Account Billing components, so the catalog is the source for both.
var customAddressPicklistSource = map[string]string{
	"StateCode":       "BillingStateCode",
	"CountryCode":     "BillingCountryCode",
	"GeocodeAccuracy": "BillingGeocodeAccuracy",
}

// CustomAddressComponentPicklistValues returns the picklist values of a
// custom Address picklist component.
func CustomAddressComponentPicklistValues(compoundName, componentName string) ([]PicklistValue, bool) {
	component, ok := schema.CustomAddressComponentFor(compoundName, componentName)
	if !ok || !component.Picklist {
		return nil, false
	}
	entry, ok := standardObjectCatalogEntryForName("Account")
	if !ok {
		return nil, false
	}
	source, ok := entry.Definition.Fields[customAddressPicklistSource[component.Suffix]]
	if !ok || len(source.PicklistValues) == 0 {
		return nil, false
	}
	return append([]PicklistValue(nil), source.PicklistValues...), true
}

// CustomAddressComponentKey returns the System.Address property backed by a
// component of a custom Address field.
func CustomAddressComponentKey(compoundName, componentName string) (string, bool) {
	component, ok := schema.CustomAddressComponentFor(compoundName, componentName)
	if !ok {
		return "", false
	}
	return component.AddressKey, true
}

// CustomAddressComponentName returns the component field for a suffix such as
// StateCode, or "" when compoundName is not a custom Address field name.
func CustomAddressComponentName(compoundName, suffix string) string {
	if !strings.HasSuffix(strings.ToLower(compoundName), "__c") || len(compoundName) <= 3 {
		return ""
	}
	return schema.CustomAddressComponentName(compoundName, suffix)
}

type customAddressStateCountryData struct {
	countries map[string]string
	// states maps upper-case country code, then upper-case state code, to label.
	states map[string]map[string]string
}

var customAddressStateCountry struct {
	once sync.Once
	data customAddressStateCountryData
}

// CustomAddressCountryLabel returns the State and Country picklist label for
// a country code.
func CustomAddressCountryLabel(code string) (string, bool) {
	data := loadCustomAddressStateCountryData()
	label, ok := data.countries[strings.ToUpper(strings.TrimSpace(code))]
	return label, ok
}

// CustomAddressStateLabel returns the label of a state code valid for the
// given country code.
func CustomAddressStateLabel(countryCode, stateCode string) (string, bool) {
	data := loadCustomAddressStateCountryData()
	label, ok := data.states[strings.ToUpper(strings.TrimSpace(countryCode))][strings.ToUpper(strings.TrimSpace(stateCode))]
	return label, ok
}

func loadCustomAddressStateCountryData() customAddressStateCountryData {
	customAddressStateCountry.once.Do(func() {
		customAddressStateCountry.data = decodeCustomAddressStateCountryData()
	})
	return customAddressStateCountry.data
}

type customAddressCatalogPicklistValue struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Active   bool   `json:"active"`
	ValidFor string `json:"validFor"`
}

type customAddressCatalogField struct {
	Name           string                              `json:"name"`
	PicklistValues []customAddressCatalogPicklistValue `json:"picklistValues"`
}

// decodeCustomAddressStateCountryData streams the embedded describe catalog
// and decodes only Account's Billing State/Country picklists. Dependent
// validFor bitmaps index the controlling CountryCode values in order.
func decodeCustomAddressStateCountryData() customAddressStateCountryData {
	out := customAddressStateCountryData{countries: map[string]string{}, states: map[string]map[string]string{}}
	file, err := standardDescribeCatalogFS.Open("standard_describe_catalog.json.gz")
	if err != nil {
		return out
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return out
	}
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	if !customAddressSeekObjectKey(decoder, "describes") || !customAddressSeekObjectKey(decoder, "Account") ||
		!customAddressSeekObjectKey(decoder, "fields") {
		return out
	}
	var fields []customAddressCatalogField
	if err := decoder.Decode(&fields); err != nil {
		return out
	}
	var countries, states []customAddressCatalogPicklistValue
	for _, field := range fields {
		switch field.Name {
		case "BillingCountryCode":
			countries = field.PicklistValues
		case "BillingStateCode":
			states = field.PicklistValues
		}
	}
	for _, country := range countries {
		if country.Active {
			out.countries[strings.ToUpper(country.Value)] = country.Label
		}
	}
	for _, state := range states {
		if !state.Active {
			continue
		}
		bits, err := base64.StdEncoding.DecodeString(state.ValidFor)
		if err != nil {
			continue
		}
		for index, country := range countries {
			if index/8 >= len(bits) || bits[index/8]&(0x80>>(index%8)) == 0 {
				continue
			}
			code := strings.ToUpper(country.Value)
			if out.states[code] == nil {
				out.states[code] = map[string]string{}
			}
			out.states[code][strings.ToUpper(state.Value)] = state.Label
		}
	}
	return out
}

// customAddressSeekObjectKey advances the decoder into the current JSON
// object until the value of key is next, skipping other members.
func customAddressSeekObjectKey(decoder *json.Decoder, key string) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return false
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		name, _ := token.(string)
		if name == key {
			return true
		}
		var skip json.RawMessage
		if err := decoder.Decode(&skip); err != nil {
			return false
		}
	}
	return false
}
