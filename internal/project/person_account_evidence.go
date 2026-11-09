package project

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	_, found := orgShapeEvidenceScan(root)
	return found
}

// personAccountRecordTypesIn lists the Person Accounts record type evidence
// under one objects directory.
func personAccountRecordTypesIn(objects string) []string {
	var found []string
	for _, object := range []string{"PersonAccount", "Account"} {
		recordTypes := filepath.Join(objects, object, "recordTypes")
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
	return found
}

// orgShapeEvidenceScan walks the project tree once for both kinds of org shape
// evidence: deployed settings files (settings/*.settings-meta.xml, from the
// outermost settings directory on a path) and Person Accounts record types
// (under objects directories, which the walk does not enter further). The
// metadata-folder skip list and depth bound apply as before; the settings list
// is sorted and the record type list keeps walk order.
func orgShapeEvidenceScan(root string) (settings, personAccounts []string) {
	if root == "" {
		return nil, nil
	}
	settingsSubtree := ""
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
		if settingsSubtree != "" && !strings.HasPrefix(path, settingsSubtree+string(os.PathSeparator)) {
			settingsSubtree = ""
		}
		switch name {
		case "objects":
			personAccounts = append(personAccounts, personAccountRecordTypesIn(path)...)
			return filepath.SkipDir
		case "settings":
			if settingsSubtree == "" {
				settings = append(settings, sourceSettingsFilesIn(path)...)
				settingsSubtree = path
			}
		}
		return nil
	})
	sort.Strings(settings)
	return settings, personAccounts
}
