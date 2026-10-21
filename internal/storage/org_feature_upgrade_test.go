package storage

import (
	"encoding/json"
	"testing"
)

func legacyCurrencyOrg(t *testing.T) OrgState {
	t.Helper()
	org := NewOrgState()
	ApplyOrgShape(&org, []string{"MultiCurrency"})
	EnsureStandardObject(&org, "Organization")
	object := org.Objects["Organization"]
	object.Definition.Fields["IsMultiCurrencyEnabled"] = Field{APIName: "IsMultiCurrencyEnabled", Type: FieldBoolean}
	object.Records[ID(org.OrgID)] = Record{ID: ID(org.OrgID), Object: "Organization", Fields: map[string]Value{"IsMultiCurrencyEnabled": BooleanValue(true)}}
	org.Objects["Organization"] = object
	for name, object := range org.Objects {
		delete(object.Definition.Metadata, standardFieldsOverlayMarker)
		org.Objects[name] = object
	}
	raw, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	delete(envelope, "features")
	raw, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var legacy OrgState
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	return legacy
}

func TestLegacyOrgFeatureProfileUpgrade(t *testing.T) {
	check := func(t *testing.T, org OrgState) {
		t.Helper()
		if value, ok := CurrencyDefaultForField(&org, "", Field{APIName: "CurrencyIsoCode", Type: FieldPicklist}); !ok || value.String != "USD" {
			t.Fatalf("legacy currency feature lost: value=%+v enabled=%t", value, ok)
		}
		if _, exists := ResolveFieldName(org.Objects["Organization"].Definition, "", "IsMultiCurrencyEnabled"); exists {
			t.Fatal("legacy Organization flag remains in schema")
		}
		for _, record := range org.Objects["Organization"].Records {
			if _, exists := record.GetField("IsMultiCurrencyEnabled"); exists {
				t.Fatal("legacy Organization flag remains in records")
			}
		}
	}
	t.Run("json", func(t *testing.T) { check(t, legacyCurrencyOrg(t)) })
	t.Run("sqlite", func(t *testing.T) {
		store, err := OpenSQLite(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		if err := store.Save(legacyCurrencyOrg(t)); err != nil {
			t.Fatal(err)
		}
		org, err := store.Load()
		if err != nil {
			t.Fatal(err)
		}
		check(t, org)
	})
	t.Run("fixture", func(t *testing.T) {
		org := NewOrgState()
		if err := ApplyFixture(&org, FixtureFromOrg(legacyCurrencyOrg(t))); err != nil {
			t.Fatal(err)
		}
		check(t, org)
	})
}

func TestApplyOrgShapeKeepsFrozenDefinitionPrivate(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "User")
	clone := org.CloneRuntimeFrozenDefinition()
	ApplyOrgShape(&clone, []string{"MultiCurrency"})
	if _, exists := ResolveFieldName(clone.Objects["User"].Definition, "", "DefaultCurrencyIsoCode"); !exists {
		t.Fatal("clone did not receive currency feature")
	}
	if _, exists := ResolveFieldName(org.Objects["User"].Definition, "", "DefaultCurrencyIsoCode"); exists {
		t.Fatal("feature application mutated the frozen parent definition")
	}
}
