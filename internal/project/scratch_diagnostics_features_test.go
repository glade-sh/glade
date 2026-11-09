package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestScratchDefinitionWarningsReportsUnreadFeatures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configFile string
		cumulus    string
		definition map[string]any
		warnings   []string
	}{
		{
			name:       "unread features array in default definition",
			configFile: "config/project-scratch-def.json",
			definition: map[string]any{
				"features": []string{"MultiCurrency", "ServiceCloud", "Knowledge"},
				"settings": map[string]any{
					"currencySettings": map[string]any{
						"enableMultiCurrency": true,
						"unreadCurrencyKey":   false,
					},
				},
			},
			warnings: []string{
				"config/project-scratch-def.json: features.ServiceCloud is not read by Glade",
				"config/project-scratch-def.json: settings.currencySettings.unreadCurrencyKey is not read by Glade",
			},
		},
		{
			name:       "unread single string feature",
			configFile: "config/hc-project-scratch-def.json",
			definition: map[string]any{
				"features": "DevelopmentWave",
			},
			warnings: []string{
				"config/hc-project-scratch-def.json: features.DevelopmentWave is not read by Glade",
			},
		},
		{
			name:       "unread feature in cumulusci linked definition",
			configFile: "orgs/dev.json",
			cumulus:    "orgs:\n  scratch:\n    dev:\n      config_file: orgs/dev.json\n",
			definition: map[string]any{
				"features": []string{"ServiceCloud"},
			},
			warnings: []string{
				"orgs/dev.json: features.ServiceCloud is not read by Glade",
			},
		},
		{
			name:       "policy-gate features are read",
			configFile: "config/project-scratch-def.json",
			definition: map[string]any{
				"features": []string{"B2BCommerce", "CommerceEnabled", "LiveAgent", "LiveAgentEnabled", "OrderManagement", "HealthCloud:2"},
			},
		},
		{
			name:       "suffixed names other than AddCustomApps are unread",
			configFile: "config/project-scratch-def.json",
			definition: map[string]any{
				"features": []string{"MultiCurrency:1", "AddCustomApps"},
			},
			warnings: []string{
				"config/project-scratch-def.json: features.AddCustomApps is not read by Glade",
				"config/project-scratch-def.json: features.MultiCurrency:1 is not read by Glade",
			},
		},
		{
			name:       "mapped features produce no warning",
			configFile: "config/project-scratch-def.json",
			definition: map[string]any{
				"features": []string{
					"PersonAccounts",
					"MultiCurrency",
					"multicurrency",
					"Sites",
					"Communities",
					"FlowSites",
					"StateAndCountryPicklist",
					"ContactsToMultipleAccounts",
					"PlatformCache",
					"ProviderFreePlatformCache",
					"EnableSetPasswordInApi",
					"AddCustomApps:5",
					"AnalyticsAdminPerms",
					"HealthCloudAddOn",
					"Chatter",
					"LightningExperience",
					"Quotes",
					"OrderManagement",
					"ApexUserModeWithPermset",
				},
			},
			warnings: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, filepath.FromSlash(tc.configFile))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(tc.definition)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if tc.cumulus != "" {
				if err := os.WriteFile(filepath.Join(root, "cumulusci.yml"), []byte(tc.cumulus), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got := ScratchDefinitionWarnings(root)
			if len(got) != len(tc.warnings) {
				t.Fatalf("ScratchDefinitionWarnings() count=%d, want %d; got=%v", len(got), len(tc.warnings), got)
			}
			for i, want := range tc.warnings {
				if got[i] != want {
					t.Errorf("warning[%d]=%q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestScratchDefinitionWarningsDeduplicatesUnreadFeatures(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config", "project-scratch-def.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{
		"features": []string{"ServiceCloud", "servicecloud", "Knowledge", "multicurrency"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	warnings := ScratchDefinitionWarnings(root)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 deduplicated warning, got %d: %v", len(warnings), warnings)
	}
	if !slices.Contains(warnings, "config/project-scratch-def.json: features.ServiceCloud is not read by Glade") {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if strings.Contains(strings.Join(warnings, " "), "private-value") {
		t.Fatal("warning exposed a value")
	}
}
