package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestOrgAccountContactAvailabilityNative(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false},
		{"feature", []string{"ContactsToMultipleAccounts"}, true},
		{"unrelated", []string{"Quotes"}, false},
		{"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			ApplyOrgShape(&org, tc.features)
			for _, name := range []string{"AccountContactRelation", "accountcontactrelation"} {
				if got := org.AllowsStandardObject(name); got != tc.want {
					t.Errorf("%s available=%v, want %v", name, got, tc.want)
				}
			}
			if got := org.AllowsStandardObject("Account"); !got {
				t.Error("Account boundary unavailable")
			}
		})
	}
}

func TestOrgAccountContactAvailabilityRefresh(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "AccountContactRelation")
	ApplyOrgShape(&org, nil)
	if _, ok := org.Objects["AccountContactRelation"]; ok {
		t.Fatal("feature-off object remains hydrated")
	}
	ApplyOrgShape(&org, []string{"ContactsToMultipleAccounts"})
	if _, ok := org.Objects["AccountContactRelation"]; !ok {
		t.Fatal("feature-on object was not restored")
	}
}
