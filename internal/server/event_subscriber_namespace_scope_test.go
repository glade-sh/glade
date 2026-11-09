package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestEventSubscriberTriggerIDNamespacedScope(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for i := 0; i < 10; i++ {
		path := filepath.Join(root, fmt.Sprintf("Trigger%02d.trigger", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("trigger Trigger%02d on Notice__e (after insert) {}", i)), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+"-meta.xml", []byte("<ApexTrigger><status>Active</status></ApexTrigger>"), 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	m, err := NewSourceMetadataFromProject(project.Project{Root: root, Namespace: "pkg", ApexFiles: paths, SourceAPIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range m.ToolingOrg.Objects["ApexTrigger"].Records {
		if row.Fields["Name"].String == "Trigger09" {
			if string(row.ID) != "01q000000000010" {
				t.Fatalf("namespaced trigger id = %s, want 01q000000000010", row.ID)
			}
			return
		}
	}
	t.Fatal("missing tenth trigger")
}
