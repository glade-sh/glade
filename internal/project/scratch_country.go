package project

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/config"
)

// scratchCountryUsers maps a scratch definition country to the locale and time
// zone native gives the default admin user of a fresh org. The captures
// scratch_country_locale_no, _de and _gb record these pairs at API 62 and 67
// with language en_US. A country that is not listed keeps the test defaults.
var scratchCountryUsers = map[string]config.TestUserConfig{
	"NO": {Locale: "no_NO", TimeZone: "Europe/Paris"},
	"DE": {Locale: "de_DE", TimeZone: "Europe/Berlin"},
	"GB": {Locale: "en_GB", TimeZone: "Europe/London"},
}

// ScratchCountryDefaults returns the upper-case country of the project's
// scratch definition and the locale and time zone native assigns it, when the
// country is one of the captured ones.
func ScratchCountryDefaults(root string) (string, config.TestUserConfig, bool) {
	if strings.TrimSpace(root) == "" {
		return "", config.TestUserConfig{}, false
	}
	defPath := scratchOrgConfigFiles(root)[0]
	if !PathInsideRoot(root, defPath) {
		return "", config.TestUserConfig{}, false
	}
	data, err := os.ReadFile(defPath) // #nosec G304 -- fixed config path under the loaded project root.
	if err != nil {
		return "", config.TestUserConfig{}, false
	}
	var def struct {
		Country string `json:"country"`
	}
	if json.Unmarshal(data, &def) != nil {
		return "", config.TestUserConfig{}, false
	}
	country := strings.ToUpper(strings.TrimSpace(def.Country))
	defaults, ok := scratchCountryUsers[country]
	return country, defaults, ok
}

// ScratchCountryUserDefaults fills the user locale and time zone that the
// project's scratch definition country implies. Settings the project already
// configures keep their value.
func ScratchCountryUserDefaults(root string, user config.TestUserConfig) config.TestUserConfig {
	if user.Locale != "" && user.TimeZone != "" {
		return user
	}
	_, defaults, ok := ScratchCountryDefaults(root)
	if !ok {
		return user
	}
	if user.Locale == "" {
		user.Locale = defaults.Locale
	}
	if user.TimeZone == "" {
		user.TimeZone = defaults.TimeZone
	}
	return user
}
