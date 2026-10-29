package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCommerceSettingNativeProfileProjectAndDirectRoutes(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, route := range []string{"settings-helper", "sfdx-scratch-definition", "cumulusci-scratch-definition"} {
			name := "disabled"
			if enabled {
				name = "enabled"
			}
			t.Run(name+"/"+route, func(t *testing.T) {
				settings := map[string]any{"commerceSettings": map[string]any{"commerceEnabled": enabled}}
				got := scratchSettingsFeatures(settings)
				if route != "settings-helper" {
					root := t.TempDir()
					raw, err := os.ReadFile("../apextest/testdata/conformance/standard_describe_profiles/profiles/P10.json")
					if err != nil {
						t.Fatal(err)
					}
					var def map[string]any
					if err := json.Unmarshal(raw, &def); err != nil {
						t.Fatal(err)
					}
					def["settings"] = settings
					raw, err = json.Marshal(def)
					if err != nil {
						t.Fatal(err)
					}
					config := "config/project-scratch-def.json"
					if route == "cumulusci-scratch-definition" {
						config = "orgs/dev.json"
						writeFile(t, filepath.Join(root, "cumulusci.yml"), "orgs:\n  scratch:\n    dev:\n      config_file: orgs/dev.json\n")
					}
					writeFile(t, filepath.Join(root, config), string(raw))
					got = OrgShapeFeatures(root)
				}
				found := false
				for _, feature := range got {
					found = found || feature == "CommerceEnabled"
				}
				if found != enabled {
					t.Fatalf("commerceEnabled=%t; features=%v", enabled, got)
				}
			})
		}
	}
}
