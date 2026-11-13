package project

import "path/filepath"

// OrgShapeInputFiles returns scratch-definition, CumulusCI and person-account
// record type paths consulted by the org shape loader, including absent config
// paths whose creation can change the shape. Other runner inputs are excluded.
func OrgShapeInputFiles(root string) []string {
	paths := []string{
		filepath.Join(root, "cumulusci.yml"),
		filepath.Join(root, "cumulusci.template.yml"),
		filepath.Join(root, "sfdx-project.json"),
	}
	paths = append(paths, scratchOrgConfigFiles(nil, root)...)
	paths = append(paths, extraScratchDefinitionFiles(nil, root)...)
	settingsFiles, personAccounts := orgShapeEvidenceScan(nil, root)
	paths = append(paths, personAccounts...)
	return dedupeStrings(append(paths, settingsFiles...))
}
