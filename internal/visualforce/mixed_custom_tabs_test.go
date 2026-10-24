package visualforce

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

func TestVisualforceProjectRenderWithMixedCustomTabs(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/vf_project_render", func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../apextest/testdata/conformance/recent_access_fields")); err != nil {
				t.Fatal(err)
			}
			if err := os.CopyFS(root, os.DirFS("../schema/testdata/mixed_custom_tabs")); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/pages/MixedTabs.page"), `<apex:page><apex:outputText value="{!'mixed tabs ready'}"/></apex:page>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.TabFiles) != 8 {
				t.Fatalf("discovered %d tabs, want 8", len(p.TabFiles))
			}
			// Strict page indexing reaches schema loading through expression validation.
			if _, err := LoadProject(p); err != nil {
				t.Fatalf("VF project load: %v", err)
			}
			html, err := RenderPageForTest(vm.New(nil), root, "MixedTabs")
			if err != nil {
				t.Fatalf("VF render: %v", err)
			}
			if !strings.Contains(html, "mixed tabs ready") {
				t.Fatalf("VF output = %q", html)
			}
		})
	}
}
