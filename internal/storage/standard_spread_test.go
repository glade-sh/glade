package storage

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// eagerSpreadOrg builds the org the way `glade exec` did when it started from
// the whole catalog: spread first, then the project shape.
func eagerSpreadOrg(features []string) OrgState {
	org := NewOrgState()
	org.OrgID = DefaultOrgID
	EnsureStandardObjectSpread(&org)
	EnsureDeterministicPlatformData(&org)
	ApplyProjectOrgShape(&org, features)
	return org
}

// lazySpreadOrg builds the lean org `glade exec` starts from now: the standard
// objects the lean construction installs are seeded from the catalog first, the
// rest of the catalog is pending.
func lazySpreadOrg(features []string) OrgState {
	lean := NewOrgState()
	EnsureDeterministicPlatformData(&lean)
	ApplyProjectOrgShape(&lean, features)
	org := NewOrgState()
	org.OrgID = DefaultOrgID
	for _, name := range StandardSpreadObjectsOf(lean) {
		EnsureStandardObject(&org, name)
	}
	EnsureDeterministicPlatformData(&org)
	ApplyProjectOrgShape(&org, features)
	org.EnableLazyStandardSpread()
	return org
}

// firstDifference shows where two JSON encodings diverge.
func firstDifference(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := max(i-120, 0)
	return fmt.Sprintf("at byte %d\n  eager: %s\n  lazy:  %s", i, a[lo:min(i+200, len(a))], b[lo:min(i+200, len(b))])
}

func definitionJSON(t *testing.T, definition ObjectDefinition) string {
	t.Helper()
	data, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var lazySpreadShapes = map[string][]string{
	"unspecified":  {project.UnspecifiedStandardFieldShape},
	"communities":  {"Communities"},
	"networks":     {"Communities", "NetworksEnabled", "Quotes", "MultiCurrency"},
	"orderManaged": {"OrderManagement", "OrderManagementEnabled"},
	"mixed":        {"Sites", "PersonAccounts", "StateAndCountryPicklist", "ContactsToMultipleAccounts", "PlatformCache", "Territory2", "KnowledgeEnabled"},
}

// Each object the eager spread installs is also what the lazy spread installs
// on first reference, with the same definition, and nothing else resolves.
func TestLazyStandardSpreadMatchesEagerObjectByObject(t *testing.T) {
	for name, features := range lazySpreadShapes {
		t.Run(name, func(t *testing.T) {
			eager := eagerSpreadOrg(features)
			lazy := lazySpreadOrg(features)
			if len(lazy.Objects) >= len(eager.Objects)/4 {
				t.Fatalf("lazy org starts with %d objects, eager has %d", len(lazy.Objects), len(eager.Objects))
			}
			for _, objectName := range sortedObjectNames(eager) {
				if _, present := lazy.Objects[objectName]; !present {
					canonical, ok := EnsureLazyStandardObject(&lazy, objectName)
					if !ok || canonical != objectName {
						t.Errorf("%s: lazy spread does not add it (ok=%v canonical=%q)", objectName, ok, canonical)
						continue
					}
				}
				want := definitionJSON(t, eager.Objects[objectName].Definition)
				got := definitionJSON(t, lazy.Objects[objectName].Definition)
				if want != got {
					t.Errorf("%s: lazy definition differs %s", objectName, firstDifference(want, got))
				}
			}
			for _, objectName := range sortedObjectNames(lazy) {
				if _, present := eager.Objects[objectName]; !present {
					t.Errorf("%s: lazy org installed an object the eager spread lacks", objectName)
				}
			}
			for _, objectName := range []string{"Network", "NetworkMember", "Quote", "WebStore", "CampaignInfluence", "NoSuchObject__c", "Account__ChangeEvent"} {
				if _, eagerHas := eager.Objects[objectName]; eagerHas {
					continue
				}
				if canonical, ok := LazyStandardObjectName(&lazy, objectName); ok {
					t.Errorf("%s: lazy spread offers %q, eager spread does not install it", objectName, canonical)
				}
			}
		})
	}
}

// Completing a lazy org yields the eager org: same objects, definitions and
// RecordType rows.
func TestMaterializeStandardObjectSpreadMatchesEager(t *testing.T) {
	for name, features := range lazySpreadShapes {
		t.Run(name, func(t *testing.T) {
			eager := eagerSpreadOrg(features)
			lazy := lazySpreadOrg(features)
			// Touch a few objects first; completion must not depend on order.
			for _, objectName := range []string{"Opportunity", "AsyncApexJob", "Case"} {
				EnsureLazyStandardObject(&lazy, objectName)
			}
			if !MaterializeStandardObjectSpread(&lazy) {
				t.Fatal("materialize reported no change on a lazy org")
			}
			if lazy.LazyStandardSpreadEnabled() {
				t.Fatal("lazy flag survives materialization")
			}
			if MaterializeStandardObjectSpread(&lazy) {
				t.Fatal("second materialize reported a change")
			}
			eagerNames, lazyNames := sortedObjectNames(eager), sortedObjectNames(lazy)
			if len(eagerNames) != len(lazyNames) {
				t.Fatalf("object counts differ: eager %d, lazy %d", len(eagerNames), len(lazyNames))
			}
			for i, objectName := range eagerNames {
				if lazyNames[i] != objectName {
					t.Fatalf("object sets differ at %d: eager %s, lazy %s", i, objectName, lazyNames[i])
				}
				want := definitionJSON(t, eager.Objects[objectName].Definition)
				got := definitionJSON(t, lazy.Objects[objectName].Definition)
				if want != got {
					t.Errorf("%s: definition differs after materialization %s", objectName, firstDifference(want, got))
				}
			}
			eagerRecordTypes := eager.Objects["RecordType"].Records
			lazyRecordTypes := lazy.Objects["RecordType"].Records
			if len(eagerRecordTypes) != len(lazyRecordTypes) {
				t.Fatalf("RecordType rows differ: eager %d, lazy %d", len(eagerRecordTypes), len(lazyRecordTypes))
			}
			for id, record := range eagerRecordTypes {
				got, ok := lazyRecordTypes[id]
				if !ok {
					t.Errorf("RecordType %s missing after materialization", id)
					continue
				}
				if definitionJSON(t, ObjectDefinition{Label: record.Fields["SobjectType"].String + "/" + record.Fields["DeveloperName"].String}) !=
					definitionJSON(t, ObjectDefinition{Label: got.Fields["SobjectType"].String + "/" + got.Fields["DeveloperName"].String}) {
					t.Errorf("RecordType %s names a different record type", id)
				}
			}
		})
	}
}

// Disallowed gated objects stay out of a lazy org and an unspecified shape keeps
// the compatibility fallback, exactly as AllowsStandardObject decides.
func TestLazyStandardObjectNameFollowsShape(t *testing.T) {
	plain := lazySpreadOrg([]string{"Communities"})
	if _, ok := LazyStandardObjectName(&plain, "quote"); ok {
		t.Error("Quote offered without Quotes")
	}
	if _, ok := LazyStandardObjectName(&plain, "network"); ok {
		t.Error("Network offered without the NetworksEnabled setting")
	}
	if name, ok := LazyStandardObjectName(&plain, "asyncapexjob"); !ok || name != "AsyncApexJob" {
		t.Errorf("AsyncApexJob: ok=%v name=%q", ok, name)
	}
	networks := lazySpreadOrg([]string{"Communities", "NetworksEnabled"})
	if _, present := networks.Objects["Network"]; !present {
		if name, ok := LazyStandardObjectName(&networks, "network"); !ok || name != "Network" {
			t.Errorf("Network with NetworksEnabled: ok=%v name=%q", ok, name)
		}
	}
	// The gated objects an unspecified shape allows are installed by the
	// shape itself; the lazy spread owes nothing more for them.
	unspecified := lazySpreadOrg([]string{project.UnspecifiedStandardFieldShape})
	if _, present := unspecified.Objects["Quote"]; !present {
		t.Error("Quote missing from an unspecified shape")
	}
	if _, ok := LazyStandardObjectName(&unspecified, "Quote"); ok {
		t.Error("an installed object is still offered lazily")
	}
	eager := NewOrgState()
	if _, ok := LazyStandardObjectName(&eager, "AsyncApexJob"); ok {
		t.Error("a non-lazy org offers lazy objects")
	}
	if MaterializeStandardObjectSpread(&eager) {
		t.Error("materializing a non-lazy org reported a change")
	}
}
