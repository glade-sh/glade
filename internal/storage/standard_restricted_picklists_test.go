package storage

import "testing"

func TestStandardRestrictedPicklistCatalogRoutes(t *testing.T) {
	// Native DML and describe observations: standard_restricted_picklists/org.tsv:2-5.
	for _, row := range []struct {
		object, field string
		restricted    bool
	}{
		{"Account", "BillingGeocodeAccuracy", true},
		{"Account", "ShippingGeocodeAccuracy", true},
		{"Contact", "MailingGeocodeAccuracy", true},
		{"Lead", "GeocodeAccuracy", true},
		{"Task", "CallType", true},
		{"Account", "Industry", false},
	} {
		t.Run(row.object+"."+row.field, func(t *testing.T) {
			entry, ok := standardObjectCatalogEntryForName(row.object)
			if !ok {
				t.Fatal("missing catalog")
			}
			packed, ok, err := lookupStandardDescribeCatalogV2(row.object)
			if err != nil || !ok {
				t.Fatal("missing packed catalog", err)
			}
			fields := map[string]Field{"catalog": entry.Definition.Fields[row.field], "packed": describeFieldMap(packed.Fields)[row.field]}
			definition := ObjectDefinition{APIName: row.object}
			EnsureStandardObjectFields(&definition)
			fields["overlay"] = definition.Fields[row.field]
			stale := definition.Fields[row.field]
			stale.RestrictedPicklist = false
			definition.Fields[row.field] = stale
			RefreshStandardObjectFieldOverlay(&definition)
			fields["persisted"] = definition.Fields[row.field]
			for route, field := range fields {
				if field.RestrictedPicklist != row.restricted {
					t.Errorf("%s restricted=%v; want %v", route, field.RestrictedPicklist, row.restricted)
				}
			}
		})
	}
}

func TestStandardRestrictedPicklistPlaceholderLabels(t *testing.T) {
	// Native default labels are retained in the DML errors at org.tsv:2-5.
	for _, row := range []struct{ object, field, label string }{
		{"Account", "BillingGeocodeAccuracy", "Billing Geocode Accuracy"},
		{"Contact", "MailingGeocodeAccuracy", "Mailing Geocode Accuracy"},
		{"Task", "CallType", "Call Type"},
	} {
		t.Run(row.object+"."+row.field, func(t *testing.T) {
			definition := ObjectDefinition{APIName: row.object, Fields: map[string]Field{row.field: {APIName: row.field, Label: row.field, Type: FieldAny}}}
			EnsureStandardObjectFields(&definition)
			if got := definition.Fields[row.field].Label; got != row.label {
				t.Fatalf("placeholder label=%q; want %q", got, row.label)
			}
			field := definition.Fields[row.field]
			field.Label = row.field
			definition.Fields[row.field] = field
			RefreshStandardObjectFieldOverlay(&definition)
			if got := definition.Fields[row.field].Label; got != row.label {
				t.Fatalf("persisted label=%q; want %q", got, row.label)
			}
		})
	}
}
