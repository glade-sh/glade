package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// personAccountEvidenceSkipDirs are directories that never hold object
// metadata, or hold unrelated trees. Skipping them keeps the scan to a
// directory listing of the metadata folders on large repositories.
var personAccountEvidenceSkipDirs = map[string]bool{
	".git": true, ".sfdx": true, ".sf": true, ".glade": true, "node_modules": true,
	"classes": true, "triggers": true, "lwc": true, "aura": true, "pages": true,
	"components": true, "staticresources": true, "flows": true, "layouts": true,
	"flexipages": true, "profiles": true, "permissionsets": true, "labels": true,
	"email": true, "documents": true, "reports": true, "dashboards": true,
	"workflows": true, "tabs": true, "applications": true, "quickActions": true,
}

const personAccountEvidenceMaxDepth = 12

// personAccountRecordTypePaths returns retrieved PersonAccount record type
// files under root. An org exposes the PersonAccount object, and so its record
// types, only when Person Accounts are enabled, so a source tree that carries
// one comes from an org with the feature.
func personAccountRecordTypePaths(root string) []string {
	if root == "" {
		return nil
	}
	var found []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		if path != root {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil || strings.Count(rel, string(os.PathSeparator)) >= personAccountEvidenceMaxDepth {
				return filepath.SkipDir
			}
		}
		name := entry.Name()
		if personAccountEvidenceSkipDirs[name] {
			return filepath.SkipDir
		}
		if name != "objects" {
			return nil
		}
		recordTypes := filepath.Join(path, "PersonAccount", "recordTypes")
		items, readErr := os.ReadDir(recordTypes)
		if readErr == nil {
			for _, item := range items {
				if !item.IsDir() && strings.HasSuffix(item.Name(), ".recordType-meta.xml") {
					found = append(found, filepath.Join(recordTypes, item.Name()))
				}
			}
		}
		return filepath.SkipDir
	})
	return found
}
