package config

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // Keep IANA validation available in standalone offline builds.

	"golang.org/x/text/currency"
)

type TestUserConfig struct {
	TimeZone               string `json:"timeZone,omitempty"`
	Locale                 string `json:"locale,omitempty"`
	Language               string `json:"language,omitempty"`
	DefaultCurrencyISOCode string `json:"defaultCurrencyIsoCode,omitempty"`
}

// DefaultTestClockStart is the deterministic clock start of a run that sets
// no test.clock.start. Salesforce runs a test at the real date; Glade fixes the
// instant so results and cached runs repeat. The default is a Wednesday in
// mid-month and mid-quarter at 12:00 UTC: in every time zone from UTC-12 to
// UTC+14 that day and the days just before and after it are weekdays, and a
// week either side stays in the same month. 2026-05-02, the default before
// v0.3.5, was a Saturday.
const DefaultTestClockStart = "2026-05-13T12:00:00Z"

type TestClockConfig struct {
	Start string `json:"start,omitempty"`
}

type TestEnvironment struct {
	User  TestUserConfig  `json:"user"`
	Clock TestClockConfig `json:"clock"`
}

func (c TestConfig) HasEnvironment() bool {
	return c.User != (TestUserConfig{}) || c.Clock.Start != ""
}

// EffectiveEnvironment fills omitted settings with the deterministic test defaults.
func (c TestConfig) EffectiveEnvironment() TestEnvironment {
	user := c.User
	if user.TimeZone == "" {
		user.TimeZone = "America/Los_Angeles"
	}
	if user.Locale == "" {
		user.Locale = "en_US"
	}
	if user.Language == "" {
		user.Language = "en_US"
	}
	if user.DefaultCurrencyISOCode == "" {
		user.DefaultCurrencyISOCode = "USD"
	}
	clock := c.Clock
	if clock.Start == "" {
		clock.Start = DefaultTestClockStart
	} else if parsed, err := time.Parse(time.RFC3339Nano, clock.Start); err == nil {
		clock.Start = parsed.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano)
	}
	return TestEnvironment{User: user, Clock: clock}
}

func ValidateTestEnvironment(c TestConfig) error {
	if zone := c.User.TimeZone; zone != "" {
		if _, err := time.LoadLocation(zone); err != nil || zone == "Local" {
			return fmt.Errorf("test.user.timeZone: unknown time zone %q; use an IANA zone, UTC or GMT", zone)
		}
	}
	if locale := c.User.Locale; locale != "" {
		if !testUserKeyKnown(testUserLocaleKeys, locale) {
			return fmt.Errorf("test.user.locale: unknown locale %q; expected a language_region key such as en_US", locale)
		}
	}
	if lang := c.User.Language; lang != "" {
		if !testUserKeyKnown(testUserLanguageKeys, lang) {
			return fmt.Errorf("test.user.language: unknown language %q", lang)
		}
	}
	if code := c.User.DefaultCurrencyISOCode; code != "" {
		if _, err := currency.ParseISO(code); err != nil || code != strings.ToUpper(code) {
			return fmt.Errorf("test.user.defaultCurrencyIsoCode: unknown ISO currency %q", code)
		}
	}
	if start := c.Clock.Start; start != "" {
		parsed, err := time.Parse(time.RFC3339Nano, start)
		if err != nil || parsed.UTC().Year() < 1 || parsed.UTC().Year() > 9999 {
			return fmt.Errorf("test.clock.start: invalid timestamp %q; expected RFC3339 with Z or an explicit offset", start)
		}
	}
	return nil
}

func testUserKeyKnown(keys, value string) bool {
	for _, key := range strings.Fields(keys) {
		if key == value {
			return true
		}
	}
	return false
}
