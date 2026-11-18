package storage

import "testing"

func prefixPlanEagerOrg() OrgState {
	org := NewOrgState()
	org.OrgID = DefaultOrgID
	EnsureStandardObjectSpread(&org)
	EnsureUniqueKeyPrefixes(&org)
	return org
}

func prefixPlanLazyOrg() OrgState {
	org := NewOrgState()
	org.OrgID = DefaultOrgID
	org.EnableStandardSpreadSource()
	EnsureUniqueKeyPrefixes(&org)
	return org
}

// Clearing the runtime schema stamp resets the validation flag. The cold
// objects keep the prefixes the unique assignment gave them, so a lookup by one
// still names the object the complete spread holds.
func TestSpreadKeyPrefixOwnerSurvivesStampClear(t *testing.T) {
	eager := prefixPlanEagerOrg()
	lazy := prefixPlanLazyOrg()
	lazy.ClearRuntimeSchemaStamp()
	checked := 0
	for name, state := range eager.Objects {
		prefix := state.Definition.KeyPrefix
		if prefix == "" || StandardKeyPrefix(name) == prefix {
			continue
		}
		if _, held := lazy.Objects[name]; held {
			continue
		}
		if got := lazy.SpreadKeyPrefixOwner(prefix); got != name {
			t.Fatalf("prefix %q after a stamp clear: lazy owner %q, eager spread holds %q", prefix, got, name)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no cold object with a reassigned prefix")
	}
}

// Objects added after the clear re-plan the prefixes. The lazy org installs
// and then completes with the prefixes the eager org holds.
func TestSpreadPrefixesAfterStampClearAndAddedObject(t *testing.T) {
	eager := prefixPlanEagerOrg()
	lazy := prefixPlanLazyOrg()
	lazy.ClearRuntimeSchemaStamp()
	// A custom object that claims a prefix a cold object holds.
	var taken string
	for name, state := range eager.Objects {
		if p := state.Definition.KeyPrefix; p != "" && StandardKeyPrefix(name) != p && taken == "" {
			if _, held := lazy.Objects[name]; !held {
				taken = p
			}
		}
	}
	add := func(org *OrgState) {
		org.Objects["Zed__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Zed__c", Label: "Zed", PluralLabel: "Zeds", KeyPrefix: taken, Fields: map[string]Field{"Id": {APIName: "Id", Type: FieldID}}}}
		org.ClearRuntimeSchemaStamp()
		EnsureUniqueKeyPrefixes(org)
	}
	add(&eager)
	add(&lazy)
	MaterializeStandardObjectSpread(&lazy)
	if len(lazy.Objects) != len(eager.Objects) {
		t.Fatalf("lazy holds %d objects, eager %d", len(lazy.Objects), len(eager.Objects))
	}
	for name, state := range eager.Objects {
		if got := lazy.Objects[name].Definition.KeyPrefix; got != state.Definition.KeyPrefix {
			t.Fatalf("%s: lazy prefix %q, eager %q", name, got, state.Definition.KeyPrefix)
		}
	}
}
