package packageartifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestSourceHashTracksSetupMetadata(t *testing.T) {
	for _, relative := range []string{"settings/BusinessHours.settings-meta.xml", "duplicateRules/Contact.Grove.duplicateRule-meta.xml"} {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`), 0600); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, "force-app", "main", "default", relative)
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("<Metadata>before</Metadata>"), 0600); err != nil {
				t.Fatal(err)
			}
			loaded, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			before, err := SourceHash(loaded)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("<Metadata>after!</Metadata>"), 0600); err != nil {
				t.Fatal(err)
			}
			after, err := SourceHash(loaded)
			if err != nil {
				t.Fatal(err)
			}
			if before == after {
				t.Fatal("setup metadata edit did not change the package artifact source hash")
			}
		})
	}
}
