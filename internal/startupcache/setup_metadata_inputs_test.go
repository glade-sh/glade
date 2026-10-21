package startupcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSetupMetadataInputsInvalidateStartupCache(t *testing.T) {
	for _, path := range []string{"settings/BusinessHours.settings-meta.xml", "duplicateRules/Contact.Grove.duplicateRule-meta.xml"} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			target := filepath.Join(root, "force-app", "main", "default", path)
			writeStartupCacheTestFile(t, target, "<Metadata>before</Metadata>")
			loaded, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			entry := NewEntry(root, loaded, typesys.Index{}, storage.NewOrgState(), CompiledRuntime{})
			if !Fresh(&entry, root, Version) {
				t.Fatal("initial cache entry is stale")
			}
			before, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			writeStartupCacheTestFile(t, target, "<Metadata>after!</Metadata>")
			if err := os.Chtimes(target, before.ModTime(), before.ModTime()); err != nil {
				t.Fatal(err)
			}
			if Fresh(&entry, root, Version) {
				t.Fatal("setup metadata edit did not invalidate startup cache")
			}
		})
	}
}
