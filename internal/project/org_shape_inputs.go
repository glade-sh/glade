package project

import "path/filepath"

// OrgShapeInputFiles returns scratch-definition, CumulusCI and person-account
// record type paths consulted by the org shape loader, including absent config
// paths whose creation can change the shape. Other runner inputs are excluded.
func OrgShapeInputFiles(root string) []string {
	paths := []string{
		filepath.Join(root, "cumulusci.yml"),
		filepath.Join(root, "cumulusci.template.yml"),
	}
	paths = append(paths, scratchOrgConfigFiles(root)...)
	return dedupeStrings(append(paths, personAccountRecordTypePaths(root)...))
}
