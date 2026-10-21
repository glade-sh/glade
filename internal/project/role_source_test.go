package project

import (
	"path/filepath"
	"testing"
)

func TestLoadRoleMetadataAcrossOverlappingSourceRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true},{"path":"force-app/main"}]}`)
	parent := filepath.Join(root, "force-app/main/default/roles/ZParent.role-meta.xml")
	child := filepath.Join(root, "force-app/main/default/roles/AChild.role")
	writeFile(t, parent, `<Role><name>Parent</name></Role>`)
	writeFile(t, child, `<Role><name>Child</name><parentRole>ZParent</parentRole></Role>`)
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.RoleFiles) != 2 || p.RoleFiles[0] != child || p.RoleFiles[1] != parent {
		t.Fatalf("role discovery/order/deduplication: %v", p.RoleFiles)
	}
}
