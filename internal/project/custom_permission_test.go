package project

import (
	"path/filepath"
	"testing"
)

func TestLoadCustomPermissionMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, suffix := range []string{".customPermission", ".customPermission-meta.xml"} {
		writeFile(t, filepath.Join(root, "force-app/customPermissions/Owned"+suffix), `<CustomPermission><label>Owned</label></CustomPermission>`)
	}
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.CustomPermissionFiles) != 2 {
		t.Fatalf("files=%v", p.CustomPermissionFiles)
	}
}
