package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConfigValidateReportsUnreadScratchFeatures(t *testing.T) {
	for _, tc := range []struct {
		name, definition, configFile, cumulus string
		warnings                              []string
	}{
		{
			name:       "unread features in default definition",
			configFile: "config/project-scratch-def.json",
			definition: `{"features":["MultiCurrency","ServiceCloud","Knowledge"],"settings":{"currencySettings":{"enableMultiCurrency":true}}}`,
			warnings:   []string{"features.Knowledge", "features.ServiceCloud"},
		},
		{
			name:       "unread single string feature",
			configFile: "config/hc-project-scratch-def.json",
			definition: `{"features":"DevelopmentWave"}`,
			warnings:   []string{"features.DevelopmentWave"},
		},
		{
			name:       "unread feature in cumulusci linked definition",
			configFile: "orgs/dev.json",
			cumulus:    "orgs:\n  scratch:\n    dev:\n      config_file: orgs/dev.json\n",
			definition: `{"features":["ServiceCloud"]}`,
			warnings:   []string{"features.ServiceCloud"},
		},
		{
			name:       "mapped features produce no warning",
			configFile: "config/project-scratch-def.json",
			definition: `{"features":["PersonAccounts","Sites","Communities","FlowSites","StateAndCountryPicklist","ContactsToMultipleAccounts","PlatformCache","ProviderFreePlatformCache","EnableSetPasswordInApi","AddCustomApps:5","AnalyticsAdminPerms","HealthCloudAddOn","Chatter","LightningExperience","Quotes","OrderManagement"]}`,
			warnings:   nil,
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
		})
	}
}
