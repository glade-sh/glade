package startupcache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// unpackaged/pre and unpackaged/post are deployed before tests run, so the
// warm startup cache must track them like package roots: creating a stage,
// adding a file, or editing one without changing size or time is stale.
func TestSetupStageRootsInvalidateWarmStartupCache(t *testing.T) {
	const object = `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><recordTypes><fullName>Owned</fullName><label>Owned</label><active>true</active></recordTypes></CustomObject>`
	for _, route := range []string{"new-stage", "new-file", "same-metadata-edit"} {
		for _, stage := range []string{"pre", "post"} {
			t.Run(route+"/"+stage, func(t *testing.T) {
				root := t.TempDir()
				writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
				writeStartupCacheTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
				writeStartupCacheTestFile(t, filepath.Join(root, "force-app/main/default/classes/Owned.cls"), "public class Owned {}")
				stageDir := filepath.Join(root, "unpackaged", stage)
				existing := filepath.Join(stageDir, "owned/objects/Contact.object")
				if route != "new-stage" {
					writeStartupCacheTestFile(t, existing, object)
				}
				loaded, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				manifest := BuildManifest(root, loaded, typesys.Index{})
				if route != "new-stage" && !manifestHasDirectory(manifest.PackageRoots, "unpackaged/"+stage) {
					t.Fatalf("setup stage absent from manifest package roots: %#v", manifest.PackageRoots)
				}
				entry := NewEntry(root, loaded, typesys.Index{}, storage.NewOrgState(), CompiledRuntime{})
				if !Fresh(&entry, root, Version) {
					t.Fatal("initial cache entry is stale")
				}
				before, err := ValidateInputWithSourceDigests(root, nil)
				if err != nil {
					t.Fatal(err)
				}
				switch route {
				case "new-stage", "new-file":
					writeStartupCacheTestFile(t, filepath.Join(stageDir, "owned/objects/Account.object"), object)
				case "same-metadata-edit":
					info, err := os.Stat(existing)
					if err != nil {
						t.Fatal(err)
					}
					edited := strings.Replace(object, "<label>Owned</label>", "<label>Other</label>", 1)
					if err := os.WriteFile(existing, []byte(edited), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(existing, info.ModTime(), info.ModTime()); err != nil {
						t.Fatal(err)
					}
				}
				if Fresh(&entry, root, Version) {
					t.Fatal("setup stage change did not invalidate the warm startup cache")
				}
				after, err := ValidateInputWithSourceDigests(root, nil)
				if err != nil {
					t.Fatal(err)
				}
				if before.Digest() == after.Digest() {
					t.Fatal("setup stage change kept the validated input digest")
				}
			})
		}
	}
}
