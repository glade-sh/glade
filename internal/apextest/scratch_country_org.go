package apextest

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// applyScratchCountryOrg gives the Organization row and the default admin user
// the country, locale and time zone a scratch definition country implies (org
// captures scratch_country_locale_no, _de and _gb, API 62 and 67). Per-user
// test settings are applied separately and may override the user's values.
func applyScratchCountryOrg(org *storage.OrgState, root string) {
	country, defaults, ok := project.ScratchCountryDefaults(root)
	if !ok || org == nil {
		return
	}
	organizations := org.Objects["Organization"]
	for id, record := range organizations.Records {
		record = record.Clone()
		record.Fields["Country"] = storage.StringValue(country)
		record.Fields["DefaultLocaleSidKey"] = storage.StringValue(defaults.Locale)
		record.Fields["TimeZoneSidKey"] = storage.StringValue(defaults.TimeZone)
		organizations.PutRecord(id, record)
	}
	org.Objects["Organization"] = organizations
	users := org.Objects["User"]
	for _, id := range []storage.ID{"005-local-user", storage.ID(storage.DefaultUserID)} {
		if record, found := users.Records[id]; found {
			record = record.Clone()
			record.Fields["Country"] = storage.StringValue(country)
			users.PutRecord(id, record)
		}
	}
	org.Objects["User"] = users
}

// ApplyScratchCountryAnonymousOrg applies the scratch definition country to an
// org that anonymous routes (glade exec, server executeAnonymous, dap) run on.
// The org captures record the same country locale and time zone for the
// anonymous and named routes. Explicit test.user settings win. A project
// without a captured country leaves the org unchanged.
func ApplyScratchCountryAnonymousOrg(org *storage.OrgState, p project.Project) {
	if org == nil {
		return
	}
	if _, _, ok := project.ScratchCountryDefaults(p.Root); !ok {
		return
	}
	applyScratchCountryOrg(org, p.Root)
	user := project.ScratchCountryUserDefaults(p.Root, p.Test.User)
	users := org.Objects["User"]
	for _, id := range []storage.ID{"005-local-user", storage.ID(storage.DefaultUserID)} {
		record, found := users.Records[id]
		if !found {
			continue
		}
		record = record.Clone()
		for field, value := range map[string]string{
			"TimeZoneSidKey": user.TimeZone, "LocaleSidKey": user.Locale,
			"LanguageLocaleKey": user.Language, "DefaultCurrencyIsoCode": user.DefaultCurrencyISOCode,
		} {
			if value != "" {
				record.Fields[field] = storage.StringValue(value)
			}
		}
		users.PutRecord(id, record)
		org.Objects["User"] = users
		return
	}
}
