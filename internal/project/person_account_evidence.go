package project

import (
	"encoding/xml"
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

// personAccountRecordTypePaths accepts both the retrieved PersonAccount layout
// and the default Account.PersonAccount identity exposed by listMetadata.
// Other Account record types do not establish that Person Accounts are enabled.
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
		for _, object := range []string{"PersonAccount", "Account"} {
			recordTypes := filepath.Join(path, object, "recordTypes")
			items, _ := os.ReadDir(recordTypes)
			for _, item := range items {
				if item.IsDir() || !strings.HasSuffix(item.Name(), ".recordType-meta.xml") {
					continue
				}
				file := filepath.Join(recordTypes, item.Name())
				if object == "Account" {
					var recordType struct {
						XMLName  xml.Name `xml:"RecordType"`
						FullName string   `xml:"fullName"`
					}
					data, err := os.ReadFile(file)
					if err != nil || xml.Unmarshal(data, &recordType) != nil || strings.TrimSpace(recordType.FullName) != "PersonAccount" {
						continue
					}
				}
				found = append(found, file)
			}
		}
		return filepath.SkipDir
	})
	return found
}
