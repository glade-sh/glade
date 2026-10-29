package dml

import (
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// validateCustomAddressComponents applies the native checks Salesforce runs on
// custom Address components (testdata/conformance/custom_address_fields rows
// P16, P18, P20, P21 and followup L, U). It runs only when the DML row writes a component.
func validateCustomAddressComponents(definition storage.ObjectDefinition, namespace string, record storage.Record, existing []storage.Record) error {
	compounds := make([]string, 0)
	for name, field := range definition.Fields {
		if field.Type == storage.FieldAddress && storage.CustomAddressComponentName(name, "Latitude") != "" {
			compounds = append(compounds, name)
		}
	}
	sort.Strings(compounds)
	for _, compound := range compounds {
		component := func(suffix string) string {
			name := storage.CustomAddressComponentName(compound, suffix)
			if canonical, ok := storage.ResolveFieldName(definition, namespace, name); ok {
				return canonical
			}
			return name
		}
		names := map[string]string{}
		touched := false
		for _, suffix := range []string{"Street", "City", "PostalCode", "StateCode", "CountryCode", "Latitude", "Longitude", "GeocodeAccuracy"} {
			names[suffix] = component(suffix)
			if _, ok := record.GetField(names[suffix]); ok || record.HasExplicitNull(names[suffix]) {
				touched = true
			}
		}
		if !touched {
			continue
		}
		effective := func(name string) storage.Value {
			if record.HasExplicitNull(name) {
				return storage.NullValue()
			}
			if value, ok := record.GetField(name); ok {
				return value
			}
			if len(existing) > 0 && !existing[0].HasExplicitNull(name) {
				if value, ok := existing[0].GetField(name); ok {
					return value
				}
			}
			return storage.NullValue()
		}
		latitude, longitude := effective(names["Latitude"]), effective(names["Longitude"])
		// Native order (followup rows L03b, L03d, L03e): latitude range,
		// longitude range, then the missing half of the pair.
		if text, outside := customAddressOutsideRange(latitude, 90); outside {
			return dmlErrorf("NUMBER_OUTSIDE_VALID_RANGE", []string{names["Latitude"]}, "%s: value outside of valid range on numeric field: %s", customAddressComponentLabel(definition, names["Latitude"]), text)
		}
		if text, outside := customAddressOutsideRange(longitude, 180); outside {
			return dmlErrorf("NUMBER_OUTSIDE_VALID_RANGE", []string{names["Longitude"]}, "%s: value outside of valid range on numeric field: %s", customAddressComponentLabel(definition, names["Longitude"]), text)
		}
		if latitude.Kind != storage.ValueNull && longitude.Kind == storage.ValueNull {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{names["Longitude"]}, "Longitude value is missing : %s", customAddressComponentLabel(definition, names["Longitude"]))
		}
		if longitude.Kind != storage.ValueNull && latitude.Kind == storage.ValueNull {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{names["Latitude"]}, "Latitude value is missing : %s", customAddressComponentLabel(definition, names["Latitude"]))
		}
		// Native validates StateCode only when the row writes a value that
		// differs from the stored one; changing the country of a record that
		// keeps its state does not revalidate it (followup rows U13, U14,
		// U20 against U12, U18).
		written, ok := record.GetField(names["StateCode"])
		if !ok {
			continue
		}
		if len(existing) > 0 && !existing[0].HasExplicitNull(names["StateCode"]) {
			if stored, ok := existing[0].GetField(names["StateCode"]); ok && stored.Kind == storage.ValueString && written.Kind == storage.ValueString && strings.EqualFold(stored.String, written.String) {
				continue
			}
		}
		state := effective(names["StateCode"])
		if state.Kind != storage.ValueString || strings.TrimSpace(state.String) == "" {
			continue
		}
		country := effective(names["CountryCode"])
		if country.Kind != storage.ValueString || strings.TrimSpace(country.String) == "" {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{names["StateCode"]}, "A country/territory must be specified before specifying a state value for field")
		}
		if _, ok := storage.CustomAddressCountryLabel(country.String); !ok {
			// The restricted CountryCode picklist reports an unknown country.
			continue
		}
		if _, ok := storage.CustomAddressStateLabel(country.String, state.String); !ok {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{names["StateCode"]}, "The %s field contains %s, which isn't an available state. Your administrator can add it for you.", names["StateCode"], state.String)
		}
	}
	return nil
}

// isCustomAddressStateCode reports the StateCode component of a custom
// Address field. Its validity depends on CountryCode (row P20), so the
// generic restricted-picklist check does not apply to it.
func isCustomAddressStateCode(definition storage.ObjectDefinition, fieldName string) bool {
	field, ok := definition.Fields[fieldName]
	if !ok || field.CompoundFieldName == "" {
		return false
	}
	compound, ok := definition.Fields[field.CompoundFieldName]
	return ok && compound.Type == storage.FieldAddress && strings.EqualFold(storage.CustomAddressComponentName(field.CompoundFieldName, "StateCode"), fieldName)
}

func customAddressComponentLabel(definition storage.ObjectDefinition, name string) string {
	if label := strings.TrimSpace(definition.Fields[name].Label); label != "" {
		return label
	}
	return name
}

// customAddressOutsideRange reports a latitude (limit 90) or longitude
// (limit 180) beyond its inclusive native bound (followup rows bundle metadata versioned compilation and template grammar, conditional rendering).
func customAddressOutsideRange(value storage.Value, limit int64) (string, bool) {
	var text string
	switch value.Kind {
	case storage.ValueInteger:
		text = strconv.FormatInt(value.Integer, 10)
	case storage.ValueDecimal:
		text = value.Decimal
	default:
		return "", false
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok {
		return "", false
	}
	return text, new(big.Rat).Abs(number).Cmp(big.NewRat(limit, 1)) > 0
}
