package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestEventSubscriberTriggerIDScope(t *testing.T) {
	for _, tc := range []struct{ name, object, status, want string }{
		{"ordinary", "Account", "Active", "01q000000000010"},
		{"inactive-event", "Notice__e", "Inactive", "01q000000000010"},
		{"active-event", "Notice__e", "Active", "01q00000000000A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var paths []string
			for i := 0; i < 10; i++ {
				path := filepath.Join(dir, fmt.Sprintf("Trigger%02d.trigger", i))
				body := fmt.Sprintf("trigger Trigger%02d on %s (after insert) {}", i, tc.object)
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path+"-meta.xml", []byte("<ApexTrigger><status>"+tc.status+"</status></ApexTrigger>"), 0600); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, path)
			}
			m, err := NewSourceMetadataFromProject(project.Project{Root: dir, ApexFiles: paths, SourceAPIVersion: "62.0"})
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range m.ToolingOrg.Objects["ApexTrigger"].Records {
				if row.Fields["Name"].String == "Trigger09" {
					if string(row.ID) != tc.want {
						t.Fatalf("id = %s, want %s", row.ID, tc.want)
					}
					return
				}
			}
			t.Fatal("missing tenth trigger")
		})
	}
}
