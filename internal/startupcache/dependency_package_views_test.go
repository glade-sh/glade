package startupcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestBuildManifestTracksDisjointDependencyPackageViews(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "application")
	supplier := filepath.Join(workspace, "supplier")
	writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true,"package":"GroveApplication","dependencies":[{"package":"GroveConsumerA"},{"package":"GroveConsumerB"}]}]
}`)
	writeStartupCacheTestFile(t, filepath.Join(root, "force-app/main/default/classes/GroveApplication.cls"), "public class GroveApplication {}\n")
	for _, consumer := range []struct{ directory, name, dependency string }{
		{"consumer-a", "GroveConsumerA", "GroveFirst"},
		{"consumer-b", "GroveConsumerB", "GroveSecond"},
	} {
		consumerRoot := filepath.Join(workspace, consumer.directory)
		writeStartupCacheTestFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true,"package":"`+consumer.name+`","dependencies":[{"package":"`+consumer.dependency+`"}]}]}`)
		writeStartupCacheTestFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes", consumer.name+".cls"), "public class "+consumer.name+" {}\n")
	}
	writeStartupCacheTestFile(t, filepath.Join(supplier, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"scratch","default":true},{"path":"first","package":"GroveFirst"},{"path":"second","package":"GroveSecond"}]
}`)
	writeStartupCacheTestFile(t, filepath.Join(supplier, "scratch/.gitkeep"), "")
	firstSource := filepath.Join(supplier, "first/main/default/classes/GroveFirst.cls")
	secondSource := filepath.Join(supplier, "second/main/default/classes/GroveSecond.cls")
	writeStartupCacheTestFile(t, firstSource, "public class GroveFirst {}\n")
	writeStartupCacheTestFile(t, secondSource, "public class GroveSecond { public static String value = 'old'; }\n")
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ManagedPackageDependencies) != 2 {
		t.Fatalf("expected two consumers, got %#v", loaded.ManagedPackageDependencies)
	}
	var views []*project.Project
	for _, consumer := range loaded.ManagedPackageDependencies {
		if consumer.Project == nil || len(consumer.Project.ManagedPackageDependencies) != 1 {
			t.Fatalf("consumer did not load its supplier: %#v", consumer)
		}
		views = append(views, consumer.Project.ManagedPackageDependencies[0].Project)
	}
	if views[0] == nil || views[1] == nil || views[0] == views[1] || views[0].Root != supplier || views[1].Root != supplier {
		t.Fatal("expected distinct supplier views sharing a source root")
	}
	if len(views[0].ApexFiles) != 1 || views[0].ApexFiles[0] != firstSource || len(views[1].ApexFiles) != 1 || views[1].ApexFiles[0] != secondSource {
		t.Fatalf("expected disjoint supplier sources: first=%v second=%v", views[0].ApexFiles, views[1].ApexFiles)
	}

	t.Run("manifest_includes_both_views", func(t *testing.T) {
		manifest := BuildManifest(root, loaded, typesys.Index{})
		if !manifest.Complete {
			t.Fatal("fixture produced an incomplete manifest")
		}
		for _, relative := range []string{"../supplier/first/main/default/classes/GroveFirst.cls", "../supplier/second/main/default/classes/GroveSecond.cls"} {
			if !manifestHasFile(manifest.Files, relative) {
				t.Errorf("supplier source missing from manifest: %s", relative)
			}
		}
		for _, relative := range []string{"../supplier/first", "../supplier/second"} {
			if !manifestHasDirectory(manifest.PackageRoots, relative) {
				t.Errorf("supplier package missing from manifest roots: %s", relative)
			}
		}
	})
	t.Run("second_view_edit_invalidates_cache", func(t *testing.T) {
		entry := NewEntry(root, loaded, typesys.Index{}, storage.NewOrgState(), CompiledRuntime{})
		if !Fresh(&entry, root, Version) {
			t.Fatal("new cache entry is stale before source edit")
		}
		before, err := os.Stat(secondSource)
		if err != nil {
			t.Fatal(err)
		}
		writeStartupCacheTestFile(t, secondSource, "public class GroveSecond { public static String value = 'new'; }\n")
		if err := os.Chtimes(secondSource, before.ModTime(), before.ModTime()); err != nil {
			t.Fatal(err)
		}
		if Fresh(&entry, root, Version) {
			t.Fatal("cache remains fresh after editing the second supplier view")
		}
	})
}
