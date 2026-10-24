package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrgShapeCommunitiesDisabledNativeDefinitionRoutes(t *testing.T) {
	source, err := os.ReadFile("../apextest/testdata/conformance/site_network_test_context/project/config/project-scratch-def.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"default_scratch", "alternate_scratch", "cumulusci", "cumulusci_template"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			path := "config/project-scratch-def.json"
			switch route {
			case "alternate_scratch":
				path = "config/hc-project-scratch-def.json"
			case "cumulusci", "cumulusci_template":
				path = "orgs/synthetic.json"
				config := "cumulusci.yml"
				if route == "cumulusci_template" {
					config = "cumulusci.template.yml"
				}
				writeFile(t, filepath.Join(root, config), "orgs:\n  scratch:\n    dev:\n      config_file: orgs/synthetic.json\n")
			}
			writeFile(t, filepath.Join(root, path), string(source))
			features := OrgShapeFeatures(root)
			sites := false
			for _, feature := range features {
				if feature == "Communities" {
					t.Fatalf("native Networks disabled: Communities remained enabled in %v", features)
				}
				if feature == "Sites" {
					sites = true
				}
			}
			if !sites {
				t.Fatal("Sites capability must remain available")
			}
		})
	}
}
