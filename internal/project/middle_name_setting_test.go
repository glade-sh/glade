package project

import (
	"path/filepath"
	"testing"
)

func TestMiddleNameSettingProfileRoutes(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, route := range []string{"helper", "sfdx", "cumulusci"} {
			t.Run(route+"/"+map[bool]string{true: "enabled", false: "disabled"}[enabled], func(t *testing.T) {
				settings := map[string]any{"nameSettings": map[string]any{"enableMiddleName": enabled}}
				got := scratchSettingsFeatures(settings)
				if route != "helper" {
					root := t.TempDir()
					config := "config/project-scratch-def.json"
					if route == "cumulusci" {
						config = "orgs/dev.json"
						writeFile(t, filepath.Join(root, "cumulusci.yml"), "orgs:\n  scratch:\n    dev:\n      config_file: orgs/dev.json\n")
					}
					value := "false"
					if enabled {
						value = "true"
					}
					writeFile(t, filepath.Join(root, config), `{"edition":"Enterprise","settings":{"nameSettings":{"enableMiddleName":`+value+`}}}`)
					got = OrgShapeFeatures(root)
				}
				found := false
				for _, feature := range got {
					found = found || feature == "MiddleNameEnabled"
				}
				if found != enabled {
					t.Fatalf("enableMiddleName=%t: features %v", enabled, got)
				}
			})
		}
	}
}
