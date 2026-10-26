package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Standard objects come from a describe export of one clean org. Standard
// fields that export omits (for example Campaign.CampaignImageId) must still be
// accepted as constructor initializers, in any letter case.
func TestStandardObjectConstructorAcceptsStandardFieldMissingFromCatalog(t *testing.T) {
	index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: "67.0"}}
	for _, source := range []string{
		"Account a = new Account(Name = 'x', Tradestyle = 'y');",
		"Account a = (Account) new Account(Name = 'x', TradeStyle = 'y');",
		"Account a = new Account(Name = 'x', TRADESTYLE = 'y');",
		"Campaign c = new Campaign(Name = 'x', CampaignImageId = null);",
		"Campaign c = (Campaign) new Campaign(Name = 'x', CampaignImageID = null);",
		"Campaign c = new Campaign(Name = 'x', CAMPAIGNIMAGEID = null);",
	} {
		if result := AnalyzeAnonymous(index, source, "67.0"); result.HasErrors() {
			t.Errorf("%q: %v", source, result.Diagnostics)
		}
	}
}

// Fields that exist only under an org feature (Person Account names, the state
// and country picklists, optional gated fields) are not catalog gaps: with the
// feature off, native rejects them at compile time, so Glade must too.
func TestStandardObjectConstructorStillRejectsFeatureGatedFields(t *testing.T) {
	index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: "67.0"}}
	for _, source := range []string{
		"Account a = new Account(FirstName = 'x');",
		"Account a = new Account(LastName = 'x', PersonEmail = 'y');",
		"Contact c = new Contact(LastName = 'x', IsPersonAccount = true);",
		"Account a = new Account(Name = 'x', BillingStateCode = 'CA');",
	} {
		if result := AnalyzeAnonymous(index, source, "67.0"); !result.HasErrors() {
			t.Errorf("%q: feature-gated standard field was accepted", source)
		}
	}
}

// Only ordinary standard fields the catalog omits are opened. Custom and
// relationship custom fields on a standard object stay strict (metadata is
// complete for them), at every checked API version.
func TestStandardObjectConstructorKeepsCustomFieldsStrictAtEveryAPI(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}}
		for _, source := range []string{
			"Account a = new Account(Name = 'x', Missing__c = 'y');",
			"Account a = new Account(Name = 'x', Missing__r = null);",
		} {
			if result := AnalyzeAnonymous(index, source, api); !result.HasErrors() {
				t.Errorf("API %s %q: undefined custom field was accepted", api, source)
			}
		}
		if result := AnalyzeAnonymous(index, "Account a = new Account(Name = 'x', Tradestyle = 'y');", api); result.HasErrors() {
			t.Errorf("API %s: ordinary missing standard field was rejected: %v", api, result.Diagnostics)
		}
	}
}
