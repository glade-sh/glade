package project

import (
	"path/filepath"
	"testing"
)

func TestLoadGroupMetadataAcrossOverlappingSourceRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true},{"path":"force-app/main"}]}`)
	parent := filepath.Join(root, "force-app/main/default/groups/ZParent.group-meta.xml")
	child := filepath.Join(root, "force-app/main/default/groups/AChild.group")
	writeFile(t, parent, `<Group><name>Parent</name><doesIncludeBosses>true</doesIncludeBosses></Group>`)
	writeFile(t, child, `<Group><name>Child</name><doesIncludeBosses>false</doesIncludeBosses></Group>`)
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.GroupFiles) != 2 || p.GroupFiles[0] != child || p.GroupFiles[1] != parent {
		t.Fatalf("group discovery/order/deduplication: %v", p.GroupFiles)
	}
}
