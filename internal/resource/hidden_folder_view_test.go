package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestHiddenFolderItemsAreObserved(t *testing.T) {
	cases := []struct {
		name, folderDir, folderSuffix, item, body, companion string
		load                                                 func(inputs, *storage.OrgState, string, string) error
	}{
		{"report", "reports", ".reportFolder-meta.xml", ".Hidden.report-meta.xml", `<Report><name>Hidden</name><format>Tabular</format></Report>`, "", loadFolderReports},
		{"document", "documents", ".documentFolder-meta.xml", ".Hidden.document-meta.xml", `<Document><name>Hidden</name></Document>`, ".Hidden.txt", loadFolderDocuments},
		{"dashboard", "dashboards", ".dashboardFolder-meta.xml", ".Hidden.dashboard-meta.xml", `<Dashboard><title>Hidden</title></Dashboard>`, "", loadFolderDashboards},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, tc.folderDir, "Shared")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			folder := filepath.Join(root, tc.folderDir, "Shared"+tc.folderSuffix)
			if err := os.WriteFile(folder, []byte(`<Folder><name>Shared</name></Folder>`), 0644); err != nil {
				t.Fatal(err)
			}
			item := filepath.Join(dir, tc.item)
			if err := os.WriteFile(item, []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			if tc.companion != "" {
				if err := os.WriteFile(filepath.Join(dir, tc.companion), []byte("body"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			scan := func(v *inputview.View, expected int) {
				t.Helper()
				p := (project.Project{Root: root}).WithInputs(v)
				org := storage.NewOrgState()
				if err := ensureFolderObject(p, &org, []string{folder}, ""); err != nil {
					t.Fatal(err)
				}
				if err := tc.load(p, &org, folder, ""); err != nil {
					t.Fatal(err)
				}
				object := org.Objects[map[string]string{"report": "Report", "document": "Document", "dashboard": "Dashboard"}[tc.name]]
				if len(object.Records) != expected {
					t.Fatalf("got %d folder items, want %d", len(object.Records), expected)
				}
			}
			v := inputview.New()
			scan(v, 1)
			if err := os.Remove(item); err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err == nil {
				t.Fatal("hidden item removal was not observed")
			}
			v = inputview.New()
			scan(v, 0)
			if err := os.WriteFile(item, []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err == nil {
				t.Fatal("hidden item addition was not observed")
			}
		})
	}
}
