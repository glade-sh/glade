package apextest

import (
	"path/filepath"
	"testing"
)

func TestProfileInventorySeedIntegrity(t *testing.T) {
	root := copyPermsetMutingProject(t, filepath.Join("testdata", "conformance", "standard_profile_inventory", "project"), "67.0")
	index := loadTestIndex(t, root)
	org := orgFromIndex(index)
	for id, profile := range org.Objects["Profile"].Records {
		if profile.Fields["Name"].String == "" {
			t.Errorf("profile seed %s has no name: %+v", id, profile)
		}
	}
}
