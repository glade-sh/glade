package storage

import "testing"

// Native rows P05 and A02: System.Address reports picklist labels for the
// stored codes, resolving shared state codes through the country.
func TestCustomAddressStateCountryLabels(t *testing.T) {
	for _, tc := range []struct{ country, state, countryLabel, stateLabel string }{
		{"US", "CA", "United States", "California"},
		{"CA", "BC", "Canada", "British Columbia"},
	} {
		if got, ok := CustomAddressCountryLabel(tc.country); !ok || got != tc.countryLabel {
			t.Fatalf("country %s = %q %v", tc.country, got, ok)
		}
		if got, ok := CustomAddressStateLabel(tc.country, tc.state); !ok || got != tc.stateLabel {
			t.Fatalf("state %s/%s = %q %v", tc.country, tc.state, got, ok)
		}
	}
	// Row P20: XX is not a US state.
	if _, ok := CustomAddressStateLabel("US", "XX"); ok {
		t.Fatal("US/XX resolved")
	}
	values, ok := CustomAddressComponentPicklistValues("Home__c", "Home__StateCode__s")
	if !ok || len(values) != 384 {
		t.Fatalf("StateCode values = %d %v", len(values), ok)
	}
}

func TestResolveFieldNameNamespacedCustomAddressComponent(t *testing.T) {
	definition := ObjectDefinition{Fields: map[string]Field{
		"ns__Home__City__s": {APIName: "ns__Home__City__s"},
		"Plain__City__s":    {APIName: "Plain__City__s"},
		"ns__Other__c":      {APIName: "ns__Other__c"},
	}}
	for _, name := range []string{"Home__City__s", "home__city__s", "ns__Home__City__s"} {
		if got, ok := ResolveFieldName(definition, "ns", name); !ok || got != "ns__Home__City__s" {
			t.Errorf("ResolveFieldName(ns, %q) = %q, %v", name, got, ok)
		}
	}
	if got, ok := ResolveFieldName(definition, "ns", "Plain__City__s"); !ok || got != "Plain__City__s" {
		t.Errorf("exact unnamespaced component = %q, %v; want Plain__City__s", got, ok)
	}
	if got, ok := ResolveFieldName(definition, "", "Home__City__s"); ok {
		t.Errorf("without a namespace Home__City__s resolved to %q", got)
	}
	if got, ok := ResolveFieldName(definition, "other", "Home__City__s"); ok {
		t.Errorf("namespace other resolved Home__City__s to %q", got)
	}
}
