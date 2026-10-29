package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOrgShapePersonAccountEvidenceRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/person_account_evidence/evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	var shapes []struct {
		Name    string
		Enabled bool
		Files   map[string]string
	}
	if err := json.Unmarshal(raw, &shapes); err != nil {
		t.Fatal(err)
	}
	for _, shape := range shapes {
		t.Run(shape.Name, func(t *testing.T) {
			root := t.TempDir()
			for path, source := range shape.Files {
				writeFile(t, filepath.Join(root, path), source)
			}
			found := false
			for _, feature := range OrgShapeFeatures(root) {
				found = found || feature == "PersonAccounts"
			}
			if found != shape.Enabled {
				t.Fatalf("features=%v, want PersonAccounts=%t", OrgShapeFeatures(root), shape.Enabled)
			}
			if shape.Enabled {
				for path := range shape.Files {
					if filepath.Ext(path) != ".xml" {
						continue
					}
					found = false
					for _, input := range OrgShapeInputFiles(root) {
						found = found || input == filepath.Join(root, path)
					}
					if !found {
						t.Fatalf("evidence %s missing from shape input inventory", path)
					}
				}
			}
		})
	}
}
