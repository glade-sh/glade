package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTestEnvironmentConfig(t *testing.T) {
	source := "test:\n  user:\n    timeZone: Europe/Berlin\n    locale: en_GB\n    language: en_US\n    defaultCurrencyIsoCode: EUR\n  clock:\n    start: '2026-10-20T22:00:00+00:00'\n  permissionSets: [Example_Access]\n"
	cfg, err := parseYAMLSubset(source)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(cfg.Test)
	for _, want := range []string{`"timeZone":"Europe/Berlin"`, `"locale":"en_GB"`, `"language":"en_US"`, `"defaultCurrencyIsoCode":"EUR"`, `"start":"2026-10-20T22:00:00+00:00"`, `"permissionSets":["Example_Access"]`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("config %s lacks %s", raw, want)
		}
	}
}

func TestTestEnvironmentRejectsInvalidSettings(t *testing.T) {
	for _, tc := range []struct{ section, key, value string }{
		{"user", "timeZone", "Mars/Olympus"}, {"user", "timeZone", "Local"},
		{"user", "locale", "zz_ZZ"}, {"user", "locale", "en_XX"}, {"user", "locale", "en"},
		{"user", "language", "zz"}, {"user", "defaultCurrencyIsoCode", "ZZZ"},
		{"clock", "start", "2026-05-02"}, {"clock", "start", "2026-05-02T00:00:00"},
		{"clock", "start", "2026-02-30T00:00:00Z"}, {"clock", "typo", "true"},
		{"user", "timeZone", "[]"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			_, err := parseYAMLSubset("test:\n  " + tc.section + ":\n    " + tc.key + ": " + tc.value + "\n")
			if err == nil || !strings.Contains(err.Error(), "test."+tc.section+"."+tc.key) {
				t.Fatalf("error=%v, want qualified setting error", err)
			}
		})
	}
}

func TestTestEnvironmentClockNormalizationAndPermissionRoutes(t *testing.T) {
	cfg, err := parseYAMLSubset("test:\n  user:\n    locale: en_GB\n  clock:\n    start: '2026-05-05T00:30:00.1239+02:00'\n  permissionSets: [Calendar_Access]\n  permissionSetGroups: [Calendar_Group]\n")
	if err != nil {
		t.Fatal(err)
	}
	env := cfg.Test.EffectiveEnvironment()
	if env.Clock.Start != "2026-05-04T22:30:00.123Z" || env.User.TimeZone != "America/Los_Angeles" || env.User.Locale != "en_GB" || strings.Join(cfg.Test.PermissionSets, ",") != "Calendar_Access" || strings.Join(cfg.Test.PermissionSetGroups, ",") != "Calendar_Group" {
		t.Fatalf("environment=%+v permissions=%+v", env, cfg.Test)
	}
	if !cfg.Test.HasEnvironment() || (TestConfig{}).HasEnvironment() {
		t.Fatal("environment presence does not follow configured settings")
	}
}
