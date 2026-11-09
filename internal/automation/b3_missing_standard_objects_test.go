package automation

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestB3AutoResponseRulesInitializeStandardObjects(t *testing.T) {
	org := storage.NewOrgState()
	ApplyToOrg(&org, Index{AutoResponseRules: []storage.AutoResponseRule{
		{Object: "Lead", Name: "LeadRule", Active: true},
		{Object: "Case", Name: "CaseRule", Active: true},
	}})
	for _, name := range []string{"Lead", "Case"} {
		if len(org.Objects[name].Definition.AutoResponseRules) != 1 {
			t.Errorf("%s response rules were lost before standard object initialization", name)
		}
	}
}
