package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func missingObjectError(name string) error {
	return &soql.QueryError{Message: soql.MissingObjectMessage(name), Err: errors.New("unknown object " + name)}
}

// An object name that no catalog object can answer to is a final miss: the
// retry that completes the spread would fail the same way, so the spread stays
// lazy.
func TestMissingObjectKeepsLazySpreadWhenNoCatalogObjectMatches(t *testing.T) {
	for _, features := range [][]string{nil, {"MultiCurrency", "Quotes"}} {
		for _, name := range []string{"NoSuchThing__c", "NoSuchThing", "Fooo__Share", "zzz", "Foo__r", "ns__Thing__c", "Nope__mdt", "Nope__x", "Nope__e", "kav", "ns__Account", "ns__Opportunity__Feed"} {
			org := shapedLazyOrg(features)
			machine := New(nil)
			machine.SetOrg(&org)
			if machine.lazySpreadCompletesMissingObject(missingObjectError(name)) {
				t.Errorf("%v %q: a miss no catalog object can answer completed the spread", features, name)
			}
			if !org.LazyStandardSpreadEnabled() {
				t.Errorf("%v %q: the lazy spread was completed", features, name)
			}
		}
	}
}

// A name a catalog object can answer to, in any case, with or without a
// namespace token, or one the catalog holds under a namespace token, keeps the
// retry: the spread completes.
func TestMissingObjectCompletesSpreadWhenCatalogObjectMatches(t *testing.T) {
	for _, name := range []string{"Account", "account", "ACCOUNTSHARE", "Time_Dependency__c", "FSL__Time_Dependency__c", "ns__Time_Dependency__c", "Knowledge__kav", "Knowledge__Feed", "AccountFeed"} {
		org := shapedLazyOrg(nil)
		machine := New(nil)
		machine.SetOrg(&org)
		// Hold the object back so the executor would have missed it.
		delete(org.Objects, "Account")
		if !machine.lazySpreadCompletesMissingObject(missingObjectError(name)) {
			t.Errorf("%q: a catalog name did not complete the spread", name)
		}
		if org.LazyStandardSpreadEnabled() {
			t.Errorf("%q: the spread is still lazy", name)
		}
	}
}

// Other failures keep their behavior: a non-object message still completes the
// spread, as before.
func TestMissingObjectUnparsedMessageCompletesSpread(t *testing.T) {
	org := shapedLazyOrg(nil)
	machine := New(nil)
	machine.SetOrg(&org)
	if !machine.lazySpreadCompletesMissingObject(&soql.QueryError{Message: "The SOQL FIELDS function only supports arguments of (all), (custom), (standard); (x) is not supported."}) {
		t.Fatal("an unrecognized not-supported message no longer completes the spread")
	}
}

// With an org namespace the resolution also tries the namespaced and stripped
// forms of a name. A miss the catalog answers under any of them keeps the retry.
func TestMissingObjectWithOrgNamespace(t *testing.T) {
	cases := []struct {
		name     string
		complete bool
	}{
		{"NoSuchThing__c", false},
		{"nosuchthing__c", false},
		{"Acount", false},
		{"Time_Dependency__c", true},
		{"time_dependency__c", true},
		{"FSL__Time_Dependency__c", true},
		{"Knowledge__kav", true},
		{"knowledge__kav", true},
		{"ContentVersion", true},
		{"contentversion", true},
	}
	for _, c := range cases {
		org := shapedLazyOrg(nil)
		org.Namespace = "ns"
		machine := New(nil)
		machine.SetOrg(&org)
		delete(org.Objects, "ContentVersion")
		got := machine.lazySpreadCompletesMissingObject(missingObjectError(c.name))
		if got != c.complete {
			t.Errorf("%q: completed=%v, want %v", c.name, got, c.complete)
		}
		if got == org.LazyStandardSpreadEnabled() {
			t.Errorf("%q: lazy flag %v disagrees with completion %v", c.name, org.LazyStandardSpreadEnabled(), got)
		}
	}
}

// The guard is sound when no name that an eagerly built org resolves is called
// final. Every catalog name is tried in several case, namespace, suffix and typo
// forms, with and without an org namespace. The eager org is the oracle, so a
// misclassified answerable name fails here whatever the executor does first.
func TestMissingObjectFinalNeverHidesAnEagerHit(t *testing.T) {
	suffixes := []string{"", "__Share", "__History", "__Feed", "__ChangeEvent", "__kav", "__ka", "__x", "__mdt", "__e", "__c", "__r"}
	for _, namespace := range []string{"", "ns"} {
		eager := shapedLazyOrg(nil)
		eager.Namespace = namespace
		storage.MaterializeStandardObjectSpread(&eager)
		oracle := New(nil)
		oracle.SetOrg(&eager)
		lazy := shapedLazyOrg(nil)
		lazy.Namespace = namespace
		guard := New(nil)
		guard.SetOrg(&lazy)

		seen := make(map[string]struct{})
		check := func(name string) {
			if _, done := seen[name]; done {
				return
			}
			seen[name] = struct{}{}
			resolved, ok := oracle.resolveObjectName(name)
			if !ok {
				_, ok = storage.ResolveObjectName(eager, name)
			}
			if ok && guard.lazyMissingObjectIsFinal(name) {
				t.Errorf("namespace %q: %q resolves to %q on the eager org but the guard calls it final", namespace, name, resolved)
			}
		}
		for _, base := range storage.KnownStandardObjectNames() {
			stripped := storage.StripAnyNamespaceToken(base)
			for _, form := range []string{base, stripped, "ns__" + stripped, "ns__" + base} {
				for _, suffix := range suffixes {
					for _, name := range []string{form + suffix, strings.ToLower(form + suffix), strings.ToUpper(form + suffix)} {
						check(name)
					}
				}
			}
			if len(base) > 2 {
				check(base[:len(base)-1])
				check(base + "x")
			}
		}
		if len(seen) < 100000 {
			t.Fatalf("namespace %q: only %d names were checked", namespace, len(seen))
		}
	}
}
