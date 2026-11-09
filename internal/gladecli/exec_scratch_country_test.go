package gladecli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
)

// The scratch definition country sets the anonymous user's locale and time
// zone, as it does for tests (org captures scratch_country_locale_*). Explicit
// test.user settings win.
func TestRunExecScratchCountryUser(t *testing.T) {
	for _, tc := range []struct {
		name, country, config, locale, zone, orgCountry string
	}{
		{"none", "", "", "en_US", "America/Los_Angeles", ""},
		{"unlisted", "FR", "", "en_US", "America/Los_Angeles", ""},
		{"de", "DE", "", "de_DE", "Europe/Berlin", "DE"},
		{"gb", "GB", "", "en_GB", "Europe/London", "GB"},
		{"no", "NO", "", "no_NO", "Europe/Paris", "NO"},
		{"explicit", "DE", "test:\n  user:\n    locale: en_GB\n", "en_GB", "Europe/Berlin", "DE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := execStandardOrgProject(t, tc.config)
			if tc.country != "" {
				writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), fmt.Sprintf(`{"orgName":"x","edition":"Developer","country":%q}`, tc.country))
			}
			body := fmt.Sprintf(`System.assertEquals('%s', UserInfo.getLocale()); System.assertEquals('%s', UserInfo.getTimeZone().getID()); Organization o = [SELECT Country FROM Organization LIMIT 1]; String c = o.Country; System.assertEquals('%s', c == null ? '' : c);`, tc.locale, tc.zone, tc.orgCountry)
			if err := runExec(context.Background(), []string{"--project", root, "--json", body}, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}
