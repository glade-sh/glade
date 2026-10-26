package startupcache

import (
	"path/filepath"
	"testing"
)

func TestDependencyPathsValidatedInputTracksConfigAndExternalSources(t *testing.T) {
	root := t.TempDir()
	consumer := filepath.Join(root, "work", "consumer")
	library := filepath.Join(root, "external", "library")
	writeStartupCacheTestFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"SharedLibrary"}]}],"sourceApiVersion":"67.0"}`)
	config := filepath.Join(consumer, "glade.yml")
	writeStartupCacheTestFile(t, config, "project:\n  dependencyPaths: [\"../../external\"]\n")
	digest := func() string {
		t.Helper()
		input, err := ValidateInputWithSourceDigests(consumer, nil)
		if err != nil {
			t.Fatal(err)
		}
		return input.Digest()
	}
	missing := digest()
	writeStartupCacheTestFile(t, filepath.Join(library, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true,"package":"SharedLibrary"}]}`)
	source := filepath.Join(library, "force-app/Shared.cls")
	writeStartupCacheTestFile(t, source, "public class Shared {}")
	loaded := digest()
	if missing == loaded {
		t.Fatal("external dependency appearance did not change cache input")
	}
	writeStartupCacheTestFile(t, source, "public class Shared { public static String value() { return 'ready'; } }")
	changed := digest()
	if loaded == changed {
		t.Fatal("external source edit did not change cache input")
	}
	writeStartupCacheTestFile(t, config, "project:\n  dependencyPaths: [\"../../missing\"]\n")
	if digest() == changed {
		t.Fatal("dependencyPaths edit did not change cache input")
	}
}
