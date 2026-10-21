package storage

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCurrencyOrgFeatureSurvivesPersistence(t *testing.T) {
	org := NewOrgState()
	ApplyOrgShape(&org, []string{"MultiCurrency"})
	check := func(t *testing.T, copy OrgState) {
		t.Helper()
		value, ok := CurrencyDefaultForField(&copy, "", Field{APIName: "CurrencyIsoCode", Type: FieldPicklist})
		if !ok || value.String != "USD" {
			t.Fatalf("currency profile lost: value=%+v enabled=%t", value, ok)
		}
	}
	for name, clone := range map[string]func() OrgState{
		"clone": org.Clone, "runtime": org.CloneRuntime, "frozen": org.CloneRuntimeFrozenDefinition,
		"shared": org.CloneRuntimeFrozenShared, "rollback": org.CloneRollbackSnapshot,
	} {
		t.Run(name, func(t *testing.T) { check(t, clone()) })
	}
	t.Run("json", func(t *testing.T) {
		raw, err := json.Marshal(org)
		if err != nil {
			t.Fatal(err)
		}
		var copy OrgState
		if err := json.Unmarshal(raw, &copy); err != nil {
			t.Fatal(err)
		}
		check(t, copy)
	})
	t.Run("fixture", func(t *testing.T) {
		var data bytes.Buffer
		if err := WriteFixture(&data, FixtureFromOrg(org)); err != nil {
			t.Fatal(err)
		}
		fixture, err := ReadFixture(&data)
		if err != nil {
			t.Fatal(err)
		}
		copy := NewOrgState()
		if err := ApplyFixture(&copy, fixture); err != nil {
			t.Fatal(err)
		}
		check(t, copy)
	})
	t.Run("sqlite", func(t *testing.T) {
		store, err := OpenSQLite(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		if err := store.Save(org); err != nil {
			t.Fatal(err)
		}
		copy, err := store.Load()
		if err != nil {
			t.Fatal(err)
		}
		check(t, copy)
	})
}

func TestOrgFeatureProfileAppliesToLateStandardObject(t *testing.T) {
	org := NewOrgState()
	ApplyOrgShape(&org, []string{"MultiCurrency"})
	EnsureStandardObject(&org, "User")
	if _, exists := ResolveFieldName(org.Objects["User"].Definition, "", "DefaultCurrencyIsoCode"); !exists {
		t.Fatal("late User is missing the native multi-currency DefaultCurrencyIsoCode field")
	}
}

func TestOrgFeatureProfileAccumulatesFeatures(t *testing.T) {
	org := NewOrgState()
	ApplyOrgShape(&org, []string{"MultiCurrency"})
	ApplyOrgShape(&org, []string{"Chatter"})
	if _, ok := CurrencyDefaultForField(&org, "", Field{APIName: "CurrencyIsoCode", Type: FieldPicklist}); !ok {
		t.Fatal("applying an additional feature dropped the enabled currency capability")
	}
	EnsureStandardObject(&org, "User")
	if _, exists := ResolveFieldName(org.Objects["User"].Definition, "", "DefaultCurrencyIsoCode"); !exists {
		t.Fatal("applying an additional feature dropped the currency field profile")
	}
}
