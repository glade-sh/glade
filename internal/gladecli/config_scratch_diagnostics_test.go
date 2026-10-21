package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConfigValidateReportsUnreadScratchSettings(t *testing.T) {
	for _, tc := range []struct {
		name, definition, configFile, cumulus string
		warnings                              []string
	}{
		{
			name:       "default definition",
			configFile: "config/project-scratch-def.json",
			definition: `{"features":["MultiCurrency"],"settings":{"currencySettings":{"enableMultiCurrency":true,"enableUnmodeledCurrencyOption":false},"nameSettings":{"enableMiddleName":true}}}`,
			warnings:   []string{"settings.currencySettings.enableUnmodeledCurrencyOption", "settings.nameSettings.enableMiddleName"},
		},
		{
			name:       "alternate definition",
			configFile: "config/hc-project-scratch-def.json",
			definition: `{"settings":{"userManagementSettings":{"enableNewUserEmailVerification":true}}}`,
			warnings:   []string{"settings.userManagementSettings.enableNewUserEmailVerification"},
		},
		{
			name:       "linked definition",
			configFile: "orgs/dev.json",
			cumulus:    "orgs:\n  scratch:\n    dev:\n      config_file: orgs/dev.json\n",
			definition: `{"settings":{"securitySettings":{"sessionTimeout":"opaque-setting-value"}}}`,
			warnings:   []string{"settings.securitySettings.sessionTimeout"},
		},
		{
			name:       "mapped false settings",
			configFile: "config/project-scratch-def.json",
			definition: `{"settings":{"communitiesSettings":{"enableNetworksEnabled":false,"enableOotbProfExtUserOpsEnable":false},"chatterSettings":{"enableChatter":false},"lightningExperienceSettings":{"enableS1DesktopEnabled":false},"currencySettings":{"enableMultiCurrency":false},"quoteSettings":{"enableQuote":false}}}`,
		},
		{
			name:       "malformed definition",
			configFile: "config/project-scratch-def.json",
			definition: `{"settings":`,
			warnings:   []string{"cannot read scratch definition"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  packageDirs: [force-app]\n")
			writeTestFile(t, filepath.Join(root, tc.configFile), tc.definition)
			if tc.cumulus != "" {
				writeTestFile(t, filepath.Join(root, "cumulusci.yml"), tc.cumulus)
				writeTestFile(t, filepath.Join(root, "cumulusci.template.yml"), tc.cumulus)
			}
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"config", "validate", "--project", root}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, stderr.String())
			}
			got := stdout.String()
			if !strings.Contains(got, "status: ok") {
				t.Fatalf("validation status missing: %s", got)
			}
			if count := strings.Count(got, "warning:"); count != len(tc.warnings) {
				t.Errorf("warning count=%d, want %d: %s", count, len(tc.warnings), got)
			}
			for _, warning := range tc.warnings {
				if !strings.Contains(got, warning) || !strings.Contains(got, filepath.ToSlash(tc.configFile)) {
					t.Errorf("missing warning %q with source %s: %s", warning, tc.configFile, got)
				}
			}
			if strings.Contains(got, "opaque-setting-value") {
				t.Fatal("diagnostic exposed a setting value")
			}
		})
	}
}
