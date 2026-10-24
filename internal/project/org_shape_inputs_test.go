package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestOrgShapeInputFilesAbsentAndReferencedDefinitionRoutes(t *testing.T) {
	root := t.TempDir()
	fixed := []string{"cumulusci.yml", "cumulusci.template.yml", "config/project-scratch-def.json", "config/hc-project-scratch-def.json"}
	for _, file := range fixed {
		if !slices.Contains(OrgShapeInputFiles(root), filepath.Join(root, file)) {
			t.Errorf("missing absent configuration path %s", file)
		}
	}
	for _, config := range []string{"cumulusci.yml", "cumulusci.template.yml"} {
		t.Run(config, func(t *testing.T) {
			body := "orgs:\n  scratch:\n    dev:\n      config_file: 'orgs/not-created.json'\n"
			if err := os.WriteFile(filepath.Join(root, config), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(OrgShapeInputFiles(root), filepath.Join(root, "orgs/not-created.json")) {
				t.Fatal("inventory must use the loader's config_file discovery even before the definition exists")
			}
			if err := os.Remove(filepath.Join(root, config)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
