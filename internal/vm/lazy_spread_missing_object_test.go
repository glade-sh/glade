package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
)

func missingObjectError(name string) error {
	return &soql.QueryError{Message: soql.MissingObjectMessage(name), Err: errors.New("unknown object " + name)}
}

// An object name that no catalog object can answer to is a final miss: the
// retry that completes the spread would fail the same way, so the spread stays
// lazy.
func TestMissingObjectKeepsLazySpreadWhenNoCatalogObjectMatches(t *testing.T) {
	for _, features := range [][]string{nil, {"MultiCurrency", "Quotes"}} {
		for _, name := range []string{"NoSuchThing__c", "NoSuchThing", "Fooo__Share", "zzz", "Foo__r", "ns__Thing__c", "Nope__mdt", "Nope__x", "Nope__e"} {
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
	for _, name := range []string{"Account", "account", "ACCOUNTSHARE", "Time_Dependency__c", "FSL__Time_Dependency__c", "kav", "Knowledge__kav", "ns__Account", "ns__Opportunity__Feed"} {
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
