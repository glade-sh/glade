package storage

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDataWeaveMetadataSurvivesOrgClonesAndPersistence(t *testing.T) {
	org := NewOrgState()
	org.Metadata.DataWeaveResources = []DataWeaveResourceMetadata{{Name: "Owned", Namespace: "a", Content: "%dw 2.0\r\n---\r\n3 + 4  \r\n", APIVersion: "53.0", ContentPath: "owned/Owned.dwl", MetadataPath: "owned/Owned.dwl-meta.xml"}, {Name: "Owned", Namespace: "b", APIVersion: "67.0", MetadataPath: "dependency/Owned.dwl-meta.xml"}}
	for name, clone := range map[string]OrgState{"clone": org.Clone(), "runtime": org.CloneRuntime(), "frozenDefinition": org.CloneRuntimeFrozenDefinition(), "frozenShared": org.CloneRuntimeFrozenShared(), "rollback": org.CloneRollbackSnapshot(), "snapshot": SnapshotRuntimeOrg(&org)} {
		if !reflect.DeepEqual(clone.Metadata.DataWeaveResources, org.Metadata.DataWeaveResources) {
			t.Fatalf("%s lost source metadata", name)
		}
	}
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "dataweave.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Save(org); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Metadata.DataWeaveResources, org.Metadata.DataWeaveResources) {
		t.Fatalf("persisted source metadata changed: %+v", loaded.Metadata.DataWeaveResources)
	}
}
