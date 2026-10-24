package project

import "path/filepath"

// OrgShapeInputFiles returns scratch-definition, CumulusCI and retrieved
// PersonAccount record type paths consulted by the org shape loader, including absent paths whose creation can change
// the shape. Other configuration and runner inputs are outside this inventory.
func OrgShapeInputFiles(root string) []string {
	paths := []string{
		filepath.Join(root, "cumulusci.yml"),
		filepath.Join(root, "cumulusci.template.yml"),
	}
	paths = append(paths, scratchOrgConfigFiles(root)...)
	return dedupeStrings(append(paths, personAccountRecordTypePaths(root)...))
}
