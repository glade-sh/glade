package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRuntimeKeyTracksVisualforceSidecarPresence(t *testing.T) {
	for _, tc := range []struct{ source, metadata string }{
		{"pages/Home.page", `<ApexPage><apiVersion>62.0</apiVersion></ApexPage>`},
		{"components/Part.component", `<ApexComponent><apiVersion>62.0</apiVersion></ApexComponent>`},
	} {
		t.Run(tc.source, func(t *testing.T) {
			root := t.TempDir()
			write := func(path, value string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(value), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
			write(filepath.Join(root, "src", "pages", "Home.page"), `<apex:page/>`)
			write(filepath.Join(root, "src", "components", "Part.component"), `<apex:component/>`)
			key := func() string {
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				s, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index, _ := typesys.BuildWithArtifacts(p, s)
				return RuntimeContentKey(index, nil)
			}
			before := key()
			meta := filepath.Join(root, "src", tc.source) + "-meta.xml"
			write(meta, tc.metadata)
			if after := key(); after == before {
				t.Fatal("adding sidecar did not change runtime key")
			}
			if err := os.Remove(meta); err != nil {
				t.Fatal(err)
			}
			if after := key(); after != before {
				t.Fatal("removing sidecar did not restore runtime key")
			}
		})
	}
}
