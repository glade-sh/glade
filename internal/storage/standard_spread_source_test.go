package storage

import (
	"reflect"
	"sort"
	"testing"
)

func sourceSpreadEagerOrg(features []string) OrgState {
	org := NewOrgState()
	org.OrgID = DefaultOrgID
	EnsureStandardObjectSpread(&org)
	ApplyProjectOrgShape(&org, features)
	return org
}

func sourceSpreadLazyOrg(features []string) OrgState {
	org := NewOrgState()
	org.OrgID = DefaultOrgID
	org.EnableStandardSpreadSource()
	ApplyProjectOrgShape(&org, features)
	return org
}

func compareCompleteOrgs(t *testing.T, eager, lazy OrgState) {
	t.Helper()
	eagerNames, lazyNames := sortedObjectNames(eager), sortedObjectNames(lazy)
	if !reflect.DeepEqual(eagerNames, lazyNames) {
		t.Fatalf("object sets differ: eager %d, lazy %d; first only-eager/only-lazy: %v", len(eagerNames), len(lazyNames), symmetricDifference(eagerNames, lazyNames))
	}
	for _, name := range eagerNames {
		want := definitionJSON(t, eager.Objects[name].Definition)
		got := definitionJSON(t, lazy.Objects[name].Definition)
		if want != got {
			t.Errorf("%s: definition differs %s", name, firstDifference(want, got))
		}
		if len(eager.Objects[name].Records) != len(lazy.Objects[name].Records) {
			t.Errorf("%s: record counts differ: eager %d, lazy %d", name, len(eager.Objects[name].Records), len(lazy.Objects[name].Records))
		}
	}
	if !reflect.DeepEqual(eager.Features, lazy.Features) {
		t.Errorf("features differ: eager %v, lazy %v", eager.Features, lazy.Features)
	}
	for id, record := range eager.Objects["RecordType"].Records {
		got, ok := lazy.Objects["RecordType"].Records[id]
		if !ok || !reflect.DeepEqual(record.Fields, got.Fields) {
			t.Errorf("RecordType row %s differs", id)
		}
	}
}

func symmetricDifference(a, b []string) []string {
	in := map[string]int{}
	for _, n := range a {
		in[n] |= 1
	}
	for _, n := range b {
		in[n] |= 2
	}
	var out []string
	for n, bits := range in {
		if bits != 3 {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

// A source-backed lazy org that is completed holds exactly what the eager
// spread then shape holds, whichever objects were touched first.
func TestStandardSpreadSourceMatchesEagerAfterCompletion(t *testing.T) {
	for name, features := range lazySpreadShapes {
		t.Run(name, func(t *testing.T) {
			eager := sourceSpreadEagerOrg(features)
			lazy := sourceSpreadLazyOrg(features)
			EnsureUniqueKeyPrefixes(&eager)
			EnsureUniqueKeyPrefixes(&lazy)
			if len(lazy.Objects) >= len(eager.Objects)/4 {
				t.Fatalf("lazy org starts with %d objects, eager has %d", len(lazy.Objects), len(eager.Objects))
			}
			stampBefore := lazy.RuntimeSchemaStamp
			for _, objectName := range []string{"opportunity", "AsyncApexJob", "Case"} {
				if resolved, ok := ResolveObjectName(lazy, objectName); !ok {
					t.Fatalf("%s did not resolve on a lazy org (resolved %q)", objectName, resolved)
				}
			}
			EnsureStandardObject(&lazy, "Entitlement")
			if lazy.RuntimeSchemaStamp != stampBefore {
				t.Fatal("installing from the source changed the runtime schema stamp")
			}
			if !MaterializeStandardObjectSpread(&lazy) {
				t.Fatal("materialize reported no change")
			}
			if lazy.LazyStandardSpreadEnabled() || MaterializeStandardObjectSpread(&lazy) {
				t.Fatal("org still lazy after materialize")
			}
			compareCompleteOrgs(t, eager, lazy)
		})
	}
}

// Each object resolved on demand carries the definition the eager org holds.
func TestStandardSpreadSourceObjectByObject(t *testing.T) {
	for name, features := range lazySpreadShapes {
		t.Run(name, func(t *testing.T) {
			eager := sourceSpreadEagerOrg(features)
			lazy := sourceSpreadLazyOrg(features)
			EnsureUniqueKeyPrefixes(&eager)
			EnsureUniqueKeyPrefixes(&lazy)
			for _, objectName := range sortedObjectNames(eager) {
				if _, ok := ResolveObjectName(lazy, objectName); !ok {
					t.Errorf("%s: not resolved on the lazy org", objectName)
					continue
				}
				want := definitionJSON(t, eager.Objects[objectName].Definition)
				got := definitionJSON(t, lazy.Objects[objectName].Definition)
				if want != got {
					t.Errorf("%s: definition differs %s", objectName, firstDifference(want, got))
				}
			}
			for _, objectName := range []string{"Network", "NetworkMember", "Quote", "CampaignInfluence", "NoSuchObject__c"} {
				if _, eagerHas := eager.Objects[objectName]; eagerHas {
					continue
				}
				if _, ok := ResolveObjectName(lazy, objectName); ok {
					t.Errorf("%s: resolves on the lazy org but not on the eager org", objectName)
				}
			}
		})
	}
}

// The prefix assignment depends on the objects outside the catalog too: a
// custom object takes the next free prefix in name order. A lazy org that has
// custom objects must assign what the eager org assigns.
func TestStandardSpreadSourceKeyPrefixesWithCustomObjects(t *testing.T) {
	addCustom := func(org *OrgState) {
		for _, name := range []string{"Alpha__c", "Zeta__c", "Mid__mdt"} {
			org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name, Label: name, KeyPrefix: "a00"}, Records: map[ID]Record{}}
		}
	}
	eager := sourceSpreadEagerOrg(nil)
	lazy := sourceSpreadLazyOrg(nil)
	addCustom(&eager)
	addCustom(&lazy)
	EnsureUniqueKeyPrefixes(&eager)
	EnsureUniqueKeyPrefixes(&lazy)
	for _, name := range []string{"CaseOwnerSharingRule", "Entitlement"} {
		if _, ok := ResolveObjectName(lazy, name); !ok {
			t.Fatalf("%s did not resolve", name)
		}
	}
	MaterializeStandardObjectSpread(&lazy)
	compareCompleteOrgs(t, eager, lazy)
}
