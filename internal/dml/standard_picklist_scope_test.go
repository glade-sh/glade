package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// These are enforcement scope fences: a captured catalog is not the effective
// value source for organization configuration. They do not claim Salesforce
// accepts the synthetic value used here.
func TestStandardRestrictedPicklistConfigurationScope(t *testing.T) {
	for _, row := range []struct {
		object string
		fields []string
	}{
		{"Account", []string{"Division", "CurrencyIsoCode", "BillingCountryCode", "BillingStateCode", "ShippingCountryCode", "ShippingStateCode", "PersonMailingCountryCode", "PersonMailingStateCode", "PersonOtherCountryCode", "PersonOtherStateCode"}},
		{"Contact", []string{"Division", "CurrencyIsoCode", "MailingCountryCode", "MailingStateCode", "OtherCountryCode", "OtherStateCode"}},
		{"Lead", []string{"Division", "CurrencyIsoCode", "CountryCode", "StateCode"}},
		{"User", []string{"DefaultDivision", "CurrencyIsoCode", "DefaultCurrencyIsoCode", "LocaleSidKey", "LanguageLocaleKey", "CountryCode", "StateCode"}},
		{"Contract", []string{"Division", "CurrencyIsoCode", "BillingCountryCode", "BillingStateCode", "ShippingCountryCode", "ShippingStateCode"}},
		{"Order", []string{"Division", "CurrencyIsoCode", "BillingCountryCode", "BillingStateCode", "ShippingCountryCode", "ShippingStateCode"}},
		{"Quote", []string{"Division", "CurrencyIsoCode", "BillingCountryCode", "BillingStateCode", "ShippingCountryCode", "ShippingStateCode", "QuoteToCountryCode", "QuoteToStateCode", "AdditionalCountryCode", "AdditionalStateCode"}},
		{"Task", []string{"Division", "CurrencyIsoCode", "RecurrenceTimeZoneSidKey"}},
		{"Event", []string{"Division", "CurrencyIsoCode", "RecurrenceTimeZoneSidKey"}},
	} {
		t.Run(row.object, func(t *testing.T) {
			definition := storage.ObjectDefinition{APIName: row.object}
			// Division and DefaultDivision are absent from a clean org's describe;
			// the standard field policy keeps them only for an unspecified org
			// shape, where the compatibility catalog stays complete. Request that
			// shape so the captured configuration fields exist to be fenced.
			storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{project.UnspecifiedStandardFieldShape, "MultiCurrency", "PersonAccounts", "StateAndCountryPicklist"})
			for _, name := range row.fields {
				t.Run(name, func(t *testing.T) {
					field, ok := definition.Fields[name]
					if !ok {
						t.Fatal("configuration field missing from standard catalog")
					}
					if !field.RestrictedPicklist || field.Type != storage.FieldPicklist {
						t.Fatalf("describe restriction was cleared: type=%s restricted=%v", field.Type, field.RestrictedPicklist)
					}
					for _, names := range [][2]string{{row.object, name}, {strings.ToLower(row.object), strings.ToLower(name)}} {
						if err := validateRestrictedPicklistField(names[0], names[1], field, storage.StringValue("NotInCapturedCatalog")); err != nil {
							t.Errorf("configuration field %s.%s enforced captured values: %v", names[0], names[1], err)
						}
					}
				})
			}
		})
	}
}

// user_timezone.json rows the captured rows accept Tokyo/Los Angeles, while
// The captured rows reject Invalid/Zone and UTC at APIs 62/67/68 on both routes.
// This validation predates describe-based catalog enforcement and must survive
// the configuration-field exclusion.
func TestStandardRestrictedPicklistPreservesUserTimezoneValidation(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "User"}
	storage.EnsureStandardObjectFields(&definition)
	field := definition.Fields["TimeZoneSidKey"]
	for _, row := range []struct {
		value string
		valid bool
	}{
		{"Asia/Tokyo", true},
		{"America/Los_Angeles", true},
		{"Invalid/Zone", false},
		{"UTC", false},
	} {
		t.Run(row.value, func(t *testing.T) {
			for _, names := range [][2]string{{"User", "TimeZoneSidKey"}, {"user", "timezonesidkey"}} {
				err := validateRestrictedPicklistField(names[0], names[1], field, storage.StringValue(row.value))
				if row.valid {
					if err != nil {
						t.Errorf("native accepted %s: %v", row.value, err)
					}
					continue
				}
				if err == nil {
					t.Errorf("pre-existing validation accepted %s", row.value)
					continue
				}
				result := resultFromError("", err)
				if result.StatusCode != "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST" || len(result.Fields) != 1 || result.Fields[0] != names[1] {
					t.Errorf("native rejection changed: %+v", result)
				}
			}
		})
	}
}

func TestStandardRestrictedPicklistConfigurationScopeControls(t *testing.T) {
	for _, row := range []struct{ object, field string }{
		{"Account", "BillingGeocodeAccuracy"},
		// Explicit project restrictions retain their pre-catalog behavior,
		// including fields whose catalog values alone are not authoritative.
		{"User", "LocaleSidKey"},
		{"User", "TimeZoneSidKey"},
		{"Account", "CurrencyIsoCode"},
		{"Account", "CountryCode__c"},
		{"Account", "ns__CurrencyIsoCode__c"},
		{"Probe__c", "CurrencyIsoCode"},
		{"Probe__c", "LocaleSidKey"},
		{"Probe__c", "BillingCountryCode"},
		{"ns__Probe__c", "Division"},
	} {
		t.Run(row.object+"."+row.field, func(t *testing.T) {
			field := storage.Field{
				APIName:            row.field,
				Label:              "Owned Choice",
				Type:               storage.FieldPicklist,
				RestrictedPicklist: true,
				PicklistValues:     []storage.PicklistValue{{Value: "Allowed", Active: true}},
			}
			if err := validateRestrictedPicklistField(row.object, row.field, field, storage.StringValue("Allowed")); err != nil {
				t.Fatalf("declared value rejected: %v", err)
			}
			err := validateRestrictedPicklistField(row.object, row.field, field, storage.StringValue("NotInCapturedCatalog"))
			if err == nil {
				t.Fatal("restricted control accepted an undeclared value")
			}
			if got, want := err.Error(), "Owned Choice: bad value for restricted picklist field: NotInCapturedCatalog"; got != want {
				t.Fatalf("error=%q; want %q", got, want)
			}
		})
	}
}

// The owned captures cover Account.BillingGeocodeAccuracy but do not establish
// the behavior of every other catalog field or every Order.Status definition.
func TestStandardRestrictedPicklistEvidenceScope(t *testing.T) {
	for _, row := range []struct {
		object, name, value string
		catalog, rejected   bool
	}{
		{"Account", "BillingGeocodeAccuracy", "Unknown", true, true},
		{"Account", "UncapturedChoice", "Unknown", true, false},
		{"Product2", "Type", "Unknown", true, false},
		{"EmailTemplate", "TemplateType", "Unknown", true, false},
		{"Order", "Status", "Draft", false, true},
		{"Order", "Status", "Draft", true, true},
		{"Order", "Status", "Unknown", false, true},
	} {
		t.Run(row.object+"/"+row.name+"/"+row.value, func(t *testing.T) {
			field := storage.Field{APIName: row.name, Label: row.name, Type: storage.FieldPicklist,
				RestrictedPicklist: true, RestrictedPicklistFromCatalog: row.catalog,
				PicklistValues: []storage.PicklistValue{{Value: "Other", Active: true}}}
			err := validateRestrictedPicklistField(row.object, row.name, field, storage.StringValue(row.value))
			if (err != nil) != row.rejected {
				t.Fatalf("rejected=%v, want %v: %v", err != nil, row.rejected, err)
			}
		})
	}
}

// project-valuesets/org.tsv rows 2-5 cover this exact deployed value set.
func TestOrderStatusDeployedValueSetRetainsDraft(t *testing.T) {
	field := storage.Field{APIName: "Status", Type: storage.FieldPicklist, RestrictedPicklist: true,
		RestrictedPicklistFromCatalog: true, StandardValueSetApplied: true, PicklistValues: []storage.PicklistValue{
			{Value: "CustomOrder", Active: true}, {Value: "Activated", Active: true},
		}}
	if err := validateRestrictedPicklistField("Order", "Status", field, storage.StringValue("Draft")); err != nil {
		t.Fatal(err)
	}
	field.RestrictedPicklistFromCatalog = false
	if err := validateRestrictedPicklistField("Order", "Status", field, storage.StringValue("Draft")); err == nil {
		t.Fatal("explicit project restriction accepted undeclared Draft")
	}
}

// locale-boundary/org.tsv:2-5 and locale_sid_key/org.tsv:2-3 capture these DML results.
func TestUserLocaleSidKeyCapturedValues(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "User"}
	storage.EnsureStandardObjectFields(&definition)
	field := definition.Fields["LocaleSidKey"]
	for _, row := range []struct {
		value string
		valid bool
	}{
		{"en_US", true}, {"pl_PL", true}, {"af", true}, {"haw", true}, {"en_AD", true},
		{"en", false}, {"az", false}, {"NotAnAllowedValue", false},
		{"en-GB", false}, {"en_FOO", false}, {"en_XY", false}, {"xx_XX", false}, {"zz_US", false},
	} {
		t.Run(row.value, func(t *testing.T) {
			err := validateRestrictedPicklistField("User", "LocaleSidKey", field, storage.StringValue(row.value))
			if row.valid && err != nil {
				t.Fatalf("valid value: %v", err)
			}
			if !row.valid {
				if err == nil {
					t.Fatal("expected restricted picklist error")
				}
				result := resultFromError("", err)
				if result.StatusCode != "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST" || result.Error != "Locale: bad value for restricted picklist field: "+row.value {
					t.Fatalf("unexpected result: %+v", result)
				}
			}
		})
	}
}

// The existing locale-boundary capture supplies update behavior; these local
// controls exercise the shared engine insert, update, and upsert paths.
func TestUserLocaleSidKeyEngineRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"insert", "update", "upsert-insert", "upsert-update"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureDeterministicPlatformData(&org)
				storage.EnsureStandardObject(&org, "User")
				engine := NewEngine(&org)
				record := storage.Record{Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue("NotAnAllowedValue")}}
				if route == "update" || route == "upsert-update" {
					record.ID = storage.ID(storage.DefaultUserID)
				}
				var result []Result
				switch route {
				case "insert":
					result = engine.Insert([]storage.Record{record})
				case "update":
					result = engine.Update([]storage.Record{record})
				default:
					result = engine.Upsert([]storage.Record{record})
				}
				if len(result) != 1 || result[0].StatusCode != "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST" || result[0].Error != "Locale: bad value for restricted picklist field: NotAnAllowedValue" {
					t.Fatalf("locale rejection: %+v", result)
				}
			})
		}
	}
}

// No native row establishes whether these unprobed values are accepted.
// They must not be rejected solely because a partial capture omits them.
func TestUserLocaleSidKeyUnprobedValuesAreNotRejected(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "User"}
	storage.EnsureStandardObjectFields(&definition)
	field := definition.Fields["LocaleSidKey"]
	for _, value := range []string{"unprobed_locale", "zz_ZZ", "es_XY"} {
		if err := validateRestrictedPicklistField("User", "LocaleSidKey", field, storage.StringValue(value)); err != nil {
			t.Errorf("unprobed value %q was rejected: %v", value, err)
		}
	}
}
