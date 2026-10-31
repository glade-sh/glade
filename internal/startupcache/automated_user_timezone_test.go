package startupcache

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAutomatedUserTimezoneRejectsOldWarmCache(t *testing.T) {
	root := t.TempDir()
	writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	entry := NewEntry(root, loaded, typesys.Index{}, org, CompiledRuntime{})
	if !Fresh(&entry, root, Version) {
		t.Fatal("current entry unexpectedly stale")
	}
	entry.Version = 14
	if Fresh(&entry, root, Version) {
		t.Fatal("version 14 cache containing old automated-user seed is still accepted")
	}
}
